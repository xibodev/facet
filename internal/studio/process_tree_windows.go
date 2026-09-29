//go:build windows

package studio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation               = 9
	jobObjectLimitKillOnJobClose                    = 0x00002000
	createSuspended                                 = 0x00000004
	processSetQuota                                 = 0x00000100
	processTerminate                                = 0x00000001
	threadSuspendResume                             = 0x00000002
	th32csSnapThread                                = 0x00000004
	errorNotFound                     syscall.Errno = 1168
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = kernel32.NewProc("TerminateJobObject")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procThread32First            = kernel32.NewProc("Thread32First")
	procThread32Next             = kernel32.NewProc("Thread32Next")
	procOpenThread               = kernel32.NewProc("OpenThread")
	procResumeThread             = kernel32.NewProc("ResumeThread")
)

type threadEntry32 struct {
	size           uint32
	usage          uint32
	threadID       uint32
	ownerProcessID uint32
	basePri        int32
	deltaPri       int32
	flags          uint32
}

type windowsProcessTree struct {
	job syscall.Handle
}

func prepareProcessTree(cmd *exec.Cmd) (processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createSuspended}

	job, _, callErr := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create Windows Job Object: %w", windowsCallError(callErr))
	}
	tree := &windowsProcessTree{job: syscall.Handle(job)}
	infoSize := 112
	if unsafe.Sizeof(uintptr(0)) == 8 {
		infoSize = 144
	}
	limits := make([]byte, infoSize)
	binary.LittleEndian.PutUint32(limits[16:], jobObjectLimitKillOnJobClose)
	ok, _, callErr := procSetInformationJobObject.Call(
		job,
		jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits[0])),
		uintptr(len(limits)),
	)
	if ok == 0 {
		_ = tree.close()
		return nil, fmt.Errorf("configure Windows Job Object: %w", windowsCallError(callErr))
	}
	return tree, nil
}

func (tree *windowsProcessTree) afterStart(cmd *exec.Cmd) error {
	pid := cmd.Process.Pid
	handle, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		return tree.failStart(cmd, 0, false, fmt.Errorf("obtain process handle: %w", err))
	}
	ok, _, callErr := procAssignProcessToJobObject.Call(uintptr(tree.job), uintptr(handle))
	if ok == 0 {
		return tree.failStart(cmd, handle, false, fmt.Errorf("assign process to Windows Job Object: %w", windowsCallError(callErr)))
	}
	if err := resumeProcessThreads(pid); err != nil {
		return tree.failStart(cmd, handle, true, fmt.Errorf("resume process in Windows Job Object: %w", err))
	}
	if err := syscall.CloseHandle(handle); err != nil {
		return tree.failStart(cmd, handle, true, fmt.Errorf("close process handle after Windows Job Object setup: %w", err))
	}
	return nil
}

func (tree *windowsProcessTree) failStart(cmd *exec.Cmd, root syscall.Handle, assigned bool, cause error) error {
	errs := []error{cause}
	var terminateErr error
	if assigned {
		terminateErr = tree.terminate()
	} else if root != 0 {
		terminateErr = syscall.TerminateProcess(root, 1)
	} else {
		terminateErr = cmd.Process.Kill()
	}
	if terminateErr != nil && !errors.Is(terminateErr, os.ErrProcessDone) {
		errs = append(errs, fmt.Errorf("terminate suspended process: %w", terminateErr))
	}
	if root != 0 {
		if err := syscall.CloseHandle(root); err != nil {
			errs = append(errs, fmt.Errorf("close process handle: %w", err))
		}
	}
	if err := tree.closeJob(); err != nil {
		errs = append(errs, fmt.Errorf("close Windows Job Object: %w", err))
	}
	return errors.Join(errs...)
}

func (tree *windowsProcessTree) terminate() error {
	if tree.job != 0 {
		ok, _, callErr := procTerminateJobObject.Call(uintptr(tree.job), 1)
		if ok != 0 {
			return nil
		}
		if err := windowsCallError(callErr); !errors.Is(err, errorNotFound) {
			return err
		}
	}
	return nil
}

func (tree *windowsProcessTree) close() error {
	return tree.closeJob()
}

func (tree *windowsProcessTree) closeJob() error {
	if tree.job == 0 {
		return nil
	}
	job := tree.job
	tree.job = 0
	return syscall.CloseHandle(job)
}

func resumeProcessThreads(pid int) error {
	snapshot, _, callErr := procCreateToolhelp32Snapshot.Call(th32csSnapThread, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return windowsCallError(callErr)
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	entry := threadEntry32{size: uint32(unsafe.Sizeof(threadEntry32{}))}
	ok, _, callErr := procThread32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		if entry.ownerProcessID == uint32(pid) {
			thread, _, openErr := procOpenThread.Call(threadSuspendResume, 0, uintptr(entry.threadID))
			if thread == 0 {
				return fmt.Errorf("open process thread %d: %w", entry.threadID, windowsCallError(openErr))
			}
			previous, _, resumeErr := procResumeThread.Call(thread)
			_ = syscall.CloseHandle(syscall.Handle(thread))
			if previous == ^uintptr(0) {
				return fmt.Errorf("resume process thread %d: %w", entry.threadID, windowsCallError(resumeErr))
			}
			return nil
		}
		entry.size = uint32(unsafe.Sizeof(threadEntry32{}))
		ok, _, callErr = procThread32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return fmt.Errorf("find process thread for PID %d: %w", pid, windowsCallError(callErr))
}

func windowsCallError(err error) error {
	if err == nil || errors.Is(err, syscall.Errno(0)) {
		return errors.New("Windows call failed")
	}
	return err
}
