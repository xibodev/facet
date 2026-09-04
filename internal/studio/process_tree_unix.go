//go:build !windows

package studio

import (
	"errors"
	"os/exec"
	"syscall"
)

type unixProcessTree struct {
	pid int
}

func prepareProcessTree(cmd *exec.Cmd) (processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &unixProcessTree{}, nil
}

func (tree *unixProcessTree) afterStart(cmd *exec.Cmd) error {
	tree.pid = cmd.Process.Pid
	return nil
}

func (tree *unixProcessTree) terminate() error {
	if tree.pid == 0 {
		return nil
	}
	err := syscall.Kill(-tree.pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func (*unixProcessTree) close() error {
	return nil
}
