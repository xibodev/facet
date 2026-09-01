package studio

import (
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/xibodev/facet/internal/studio/engine"
)

// deniedTools is a denylist for read-only mode.
var deniedTools = engine.ClaudeDeniedTools

// Session represents a persistent CLI process session.
type Session struct {
	ID       string
	ClaudeID string
	Dir      string
	Mode     string
	Engine   string

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	events  chan map[string]any
	alive   bool
	turnMu  sync.Mutex
	adapter engine.EngineAdapter
}

// Close closes the session's stdin and terminates the child process.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.alive = false
	return nil
}

// send writes one user turn as JSON into stdin.
func (s *Session) send(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive || s.stdin == nil {
		return fmt.Errorf("session process is not running")
	}
	adapter := s.adapter
	if adapter == nil {
		adapter = engine.GetAdapter(s.Engine)
		s.adapter = adapter
	}
	b, err := adapter.FormatUserMessage(text)
	if err != nil {
		return err
	}
	_, err = s.stdin.Write(b)
	return err
}
