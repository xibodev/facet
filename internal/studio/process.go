package studio

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/xibodev/facet/internal/studio/engine"
)

// Session is a durable Studio conversation. Each turn uses a fresh CLI process.
type Session struct {
	ID       string
	NativeID string
	Dir      string
	Mode     string
	Engine   string

	mu           sync.Mutex
	valid        bool
	cmd          *exec.Cmd
	cancel       context.CancelFunc
	done         chan struct{}
	turnGate     chan struct{}
	adapter      engine.EngineAdapter
	extraArgs    []string
	environment  []string
	maxTokenSize int
}

const (
	sessionCloseTimeout       = 5 * time.Second
	processOutputDrainTimeout = time.Second
	processOutputDrainGrace   = time.Second
)

type turnResult struct {
	ok         bool
	reason     string
	abnormal   bool
	canceled   bool
	emitFailed bool
}

type turnEvent struct {
	normalized *engine.NormalizedEvent
	payload    map[string]any
}

type processTree interface {
	afterStart(*exec.Cmd) error
	terminate() error
	close() error
}

// Close invalidates the conversation, cancels its current process tree, and
// waits for runTurn (the sole owner of cmd.Wait) to confirm that it was reaped.
func (s *Session) Close() error {
	s.mu.Lock()
	s.valid = false
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}

	timer := time.NewTimer(sessionCloseTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("timed out after %s waiting for Studio session %q process tree to be reaped", sessionCloseTimeout, s.ID)
	}
}

// IsAlive reports whether the Studio conversation may accept and resume turns.
func (s *Session) IsAlive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.valid
}

func (s *Session) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil
}

func (s *Session) setNativeID(id string) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.NativeID != id
	s.NativeID = id
	return changed
}

func (s *Session) status() (nativeID, dir, mode, engineName string, alive bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.NativeID, s.Dir, s.Mode, s.Engine, s.valid
}

func (s *Session) acquireTurn(ctx context.Context) (func(), error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.turnGate:
	}

	release := func() { s.turnGate <- struct{}{} }
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	if !s.IsAlive() {
		release()
		return nil, errors.New("session is not resumable")
	}
	return release, nil
}

func (s *Session) invalidate() {
	s.mu.Lock()
	s.valid = false
	s.mu.Unlock()
}

func (s *Session) runTurn(ctx context.Context, prompt string, emit func(turnEvent) error) turnResult {
	release, err := s.acquireTurn(ctx)
	if err != nil {
		canceled := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		if canceled {
			s.invalidate()
		}
		return turnResult{reason: err.Error(), canceled: canceled}
	}
	defer release()

	turnCtx, cancel := context.WithCancel(ctx)
	turnDone := make(chan struct{})
	s.mu.Lock()
	if !s.valid || turnCtx.Err() != nil {
		if turnCtx.Err() != nil {
			s.valid = false
		}
		s.mu.Unlock()
		cancel()
		if err := turnCtx.Err(); err != nil {
			return turnResult{reason: err.Error(), canceled: true}
		}
		return turnResult{reason: "session is not resumable", canceled: true}
	}
	adapter := s.adapter
	dir := s.Dir
	mode := s.Mode
	nativeID := s.NativeID
	extraArgs := append([]string(nil), s.extraArgs...)
	environment := append([]string(nil), s.environment...)
	maxTokenSize := s.maxTokenSize
	s.cancel = cancel
	s.done = turnDone
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		if s.done == turnDone {
			s.cmd = nil
			s.cancel = nil
			s.done = nil
			close(turnDone)
		}
		s.mu.Unlock()
	}()

	if adapter == nil {
		s.invalidate()
		return turnResult{reason: "session engine adapter is not configured", abnormal: true}
	}
	if strings.TrimSpace(prompt) == "" {
		s.invalidate()
		return turnResult{reason: "prompt is required", abnormal: true}
	}

	args, err := adapter.BuildTurnArgs(dir, mode, prompt, nativeID, extraArgs)
	if err != nil {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("build %s turn command: %v", s.Engine, err), abnormal: true}
	}
	if err := turnCtx.Err(); err != nil {
		s.invalidate()
		return turnResult{reason: err.Error(), canceled: true}
	}

	cmd := exec.Command(adapter.ExecutableName(), args...)
	cmd.Dir = dir
	if environment != nil {
		cmd.Env = environment
	}
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("open %s output: %v", s.Engine, err), abnormal: true}
	}
	defer func() {
		_ = stdout.Close()
		_ = stdoutWriter.Close()
	}()
	cmd.Stdout = stdoutWriter
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("open %s error output: %v", s.Engine, err), abnormal: true}
	}
	defer func() {
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}()
	stderr := &tailBuffer{limit: 2048}
	cmd.Stderr = stderrWriter
	tree, err := prepareProcessTree(cmd)
	if err != nil {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("prepare %s process tree: %v", s.Engine, err), abnormal: true}
	}
	var terminateOnce sync.Once
	var terminateErr error
	terminateTree := func() error {
		terminateOnce.Do(func() {
			terminateErr = tree.terminate()
		})
		return terminateErr
	}
	var closeTreeOnce sync.Once
	var closeTreeErr error
	closeTree := func() error {
		closeTreeOnce.Do(func() {
			closeTreeErr = tree.close()
		})
		return closeTreeErr
	}
	defer func() { _ = closeTree() }()

	s.mu.Lock()
	if !s.valid || turnCtx.Err() != nil {
		if turnCtx.Err() != nil {
			s.valid = false
		}
		s.mu.Unlock()
		if err := turnCtx.Err(); err != nil {
			return turnResult{reason: err.Error(), canceled: true}
		}
		return turnResult{reason: "session is not resumable", canceled: true}
	}
	s.cmd = cmd
	s.mu.Unlock()

	startReady := make(chan struct{})
	watcherStop := make(chan struct{})
	watcherDone := make(chan error, 1)
	var started bool
	var startMu sync.Mutex
	go func() {
		select {
		case <-turnCtx.Done():
			<-startReady
			startMu.Lock()
			wasStarted := started
			startMu.Unlock()
			if wasStarted {
				watcherDone <- terminateTree()
				return
			}
			watcherDone <- nil
		case <-watcherStop:
			watcherDone <- nil
		}
	}()

	if err := turnCtx.Err(); err != nil {
		close(startReady)
		close(watcherStop)
		<-watcherDone
		s.invalidate()
		return turnResult{reason: err.Error(), canceled: true}
	}
	if err := cmd.Start(); err != nil {
		close(startReady)
		close(watcherStop)
		<-watcherDone
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("start engine (%s): %v", s.Engine, err), abnormal: true}
	}
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	stderrDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(stderr, stderrReader)
		stderrDone <- err
	}()
	startMu.Lock()
	started = true
	startMu.Unlock()
	startErr := tree.afterStart(cmd)
	close(startReady)
	if startErr != nil {
		cancel()
		treeErr := <-watcherDone
		waitErr := cmd.Wait()
		closeErr := closeTree()
		_ = stdout.Close()
		_ = stderrReader.Close()
		var stderrDrainErr error
		select {
		case <-stderrDone:
		case <-time.After(processOutputDrainGrace):
			stderrDrainErr = fmt.Errorf("timed out after %s waiting for stderr reader to stop after closing process pipe", processOutputDrainGrace)
		}
		reason := fmt.Sprintf("start %s process tree: %v", s.Engine, startErr)
		if treeErr != nil {
			reason = fmt.Sprintf("%s; terminate process tree: %v", reason, treeErr)
		}
		if waitErr != nil {
			reason = fmt.Sprintf("%s; process exit: %v", reason, waitErr)
		}
		if closeErr != nil {
			reason = fmt.Sprintf("%s; close process tree: %v", reason, closeErr)
		}
		reason = withProcessOutputDrainError(reason, stderrDrainErr)
		s.invalidate()
		return turnResult{reason: withStderr(reason, stderr.String()), abnormal: true}
	}

	var failureReason string
	var normalizeErr error
	var emitErr error
	var successfulTerminal bool
	var engineFailed bool
	// Wait owns process reaping; the scanner remains the sole event emitter.
	type processWaitResult struct {
		waitErr           error
		treeErr           error
		closeErr          error
		stderrErr         error
		drainErr          error
		forcedStdoutClose bool
		forcedStderrClose bool
	}
	waitDone := make(chan processWaitResult, 1)
	rootExited := make(chan struct{})
	scannerDone := make(chan struct{})
	go func() {
		waitErr := cmd.Wait()
		close(rootExited)
		treeErr := terminateTree()
		closeErr := closeTree()

		forcedStdoutClose := false
		forcedStderrClose := false
		var stderrErr error
		timer := time.NewTimer(processOutputDrainTimeout)
		defer timer.Stop()
		scannerWait := scannerDone
		stderrWait := stderrDone
		var drainErr error
		pipesClosed := false
		for scannerWait != nil || stderrWait != nil {
			select {
			case <-scannerWait:
				scannerWait = nil
			case stderrErr = <-stderrWait:
				stderrWait = nil
			case <-timer.C:
				if !pipesClosed {
					pipesClosed = true
					if scannerWait != nil {
						forcedStdoutClose = true
						_ = stdout.Close()
					}
					if stderrWait != nil {
						forcedStderrClose = true
						_ = stderrReader.Close()
					}
					timer.Reset(processOutputDrainGrace)
					continue
				}

				var pending []string
				if scannerWait != nil {
					pending = append(pending, "stdout")
				}
				if stderrWait != nil {
					pending = append(pending, "stderr")
				}
				drainErr = fmt.Errorf("timed out after %s waiting for %s reader to stop after closing process pipes", processOutputDrainGrace, strings.Join(pending, " and "))
				scannerWait = nil
				stderrWait = nil
			}
		}
		waitDone <- processWaitResult{
			waitErr:           waitErr,
			treeErr:           treeErr,
			closeErr:          closeErr,
			stderrErr:         stderrErr,
			drainErr:          drainErr,
			forcedStdoutClose: forcedStdoutClose,
			forcedStderrClose: forcedStderrClose,
		}
	}()

	if maxTokenSize <= 0 {
		maxTokenSize = 32 * 1024 * 1024
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, min(1024*1024, maxTokenSize)), maxTokenSize)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		normalized, eventErr := adapter.NormalizeEvent(line)
		if engineFailed {
			continue
		}
		if eventErr != nil {
			if normalizeErr == nil {
				normalizeErr = eventErr
				failureReason = fmt.Sprintf("normalize %s output: %v", s.Engine, eventErr)
			}
			engineFailed = true
			cancel()
			continue
		}
		if normalized == nil {
			continue
		}
		if normalized.SessionID != "" {
			s.setNativeID(normalized.SessionID)
		}
		if normalized.Type == engine.EventDone && !normalized.IsError {
			successfulTerminal = true
		}

		payload, mapErr := normalizedEventMap(normalized)
		if mapErr != nil {
			if normalizeErr == nil {
				normalizeErr = mapErr
				failureReason = fmt.Sprintf("encode %s output: %v", s.Engine, mapErr)
			}
			engineFailed = true
			cancel()
			continue
		}
		if adapter.Name() == "claude" && normalized.Raw != nil {
			payload = normalized.Raw
		}
		if err := emit(turnEvent{normalized: normalized, payload: payload}); err != nil {
			emitErr = err
			cancel()
			_ = stdout.Close()
			break
		}

		if normalized.Type == engine.EventError || normalized.Type == engine.EventDone && normalized.IsError {
			engineFailed = true
			failureReason = strings.TrimSpace(normalized.Content)
			if failureReason == "" {
				failureReason = fmt.Sprintf("%s reported an unsuccessful turn", s.Engine)
			}
			cancel()
			continue
		}
	}
	scanErr := scanner.Err()
	unexpectedScanErr := scanErr != nil && turnCtx.Err() == nil
	if unexpectedScanErr {
		_ = terminateTree()
		select {
		case <-rootExited:
		default:
			cancel()
		}
	}
	close(scannerDone)
	processResult := <-waitDone
	close(watcherStop)
	watcherErr := <-watcherDone
	treeErr := processResult.treeErr
	if treeErr == nil {
		treeErr = watcherErr
	}
	scanFailed := unexpectedScanErr && !processResult.forcedStdoutClose

	turnCanceled := turnCtx.Err() != nil
	requestCanceled := ctx.Err() != nil

	s.mu.Lock()
	conversationValid := s.valid
	currentNativeID := s.NativeID
	s.mu.Unlock()

	if emitErr != nil {
		s.invalidate()
		reason := fmt.Sprintf("emit Studio event: %v", emitErr)
		reason = withProcessOutputDrainError(reason, processResult.drainErr)
		return turnResult{reason: withProcessTreeError(reason, treeErr), abnormal: true, canceled: true, emitFailed: true}
	}
	if engineFailed || normalizeErr != nil {
		s.invalidate()
		reason := withProcessOutputDrainError(withStderr(failureReason, stderr.String()), processResult.drainErr)
		return turnResult{reason: withProcessTreeError(reason, treeErr)}
	}
	if requestCanceled || (turnCanceled && !scanFailed) {
		reason := "turn canceled"
		if ctx.Err() != nil {
			reason = ctx.Err().Error()
		} else if !conversationValid {
			reason = "session stopped"
		}
		s.invalidate()
		reason = withProcessOutputDrainError(reason, processResult.drainErr)
		return turnResult{reason: withProcessTreeError(reason, treeErr), abnormal: true, canceled: true}
	}
	if scanFailed {
		s.invalidate()
		reason := withStderr(fmt.Sprintf("%s output stream: %v", s.Engine, scanErr), stderr.String())
		reason = withProcessOutputDrainError(reason, processResult.drainErr)
		return turnResult{reason: withProcessTreeError(reason, treeErr), abnormal: true}
	}
	if processResult.drainErr != nil {
		s.invalidate()
		return turnResult{reason: withStderr(processResult.drainErr.Error(), stderr.String()), abnormal: true}
	}
	if processResult.stderrErr != nil && !processResult.forcedStderrClose {
		s.invalidate()
		return turnResult{reason: withStderr(fmt.Sprintf("%s error output stream: %v", s.Engine, processResult.stderrErr), stderr.String()), abnormal: true}
	}
	if processResult.waitErr != nil {
		s.invalidate()
		return turnResult{reason: withStderr(fmt.Sprintf("%s process exited: %v", s.Engine, processResult.waitErr), stderr.String()), abnormal: true}
	}
	if treeErr != nil {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("terminate %s process tree: %v", s.Engine, treeErr), abnormal: true}
	}
	if processResult.closeErr != nil {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("close %s process tree: %v", s.Engine, processResult.closeErr), abnormal: true}
	}
	if !successfulTerminal {
		s.invalidate()
		return turnResult{reason: withStderr(fmt.Sprintf("%s exited successfully without a successful terminal event", s.Engine), stderr.String()), abnormal: true}
	}
	if currentNativeID == "" {
		s.invalidate()
		return turnResult{reason: fmt.Sprintf("%s completed without reporting a native session ID", s.Engine), abnormal: true}
	}
	if !s.IsAlive() {
		return turnResult{reason: "session stopped", abnormal: true, canceled: true}
	}
	return turnResult{ok: true}
}

func withProcessTreeError(reason string, err error) string {
	if err == nil {
		return reason
	}
	return fmt.Sprintf("%s; terminate process tree: %v", strings.TrimSpace(reason), err)
}

func withProcessOutputDrainError(reason string, err error) string {
	if err == nil {
		return reason
	}
	return fmt.Sprintf("%s; %v", strings.TrimSpace(reason), err)
}

func normalizedEventMap(event *engine.NormalizedEvent) (map[string]any, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func withStderr(reason, stderr string) string {
	reason = strings.TrimSpace(reason)
	stderr = strings.TrimSpace(stderr)
	if reason == "" {
		reason = "engine turn failed"
	}
	if stderr == "" || strings.Contains(reason, stderr) {
		return reason
	}
	return reason + "; " + stderr
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	if b.limit <= 0 {
		return written, nil
	}
	if len(p) >= b.limit {
		b.data = append(b.data[:0], p[len(p)-b.limit:]...)
		return written, nil
	}
	b.data = append(b.data, p...)
	if overflow := len(b.data) - b.limit; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:b.limit]
	}
	return written, nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...))
}
