//go:build windows

package daemon

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const (
	createNewProcessGroup uint32 = 0x00000200
	detachedProcess       uint32 = 0x00000008
)

// StartAndWait is unused on Windows. Servers are started with StartDetached.
func StartAndWait(argv []string, port int, timeout time.Duration) (int, error) {
	return 0, fmt.Errorf("detached start is not used on windows")
}

// StartDetached starts argv in its own process group and returns when the
// process has been created. The caller does not wait for it to exit, and
// the command line is not parsed by cmd.exe.
func StartDetached(argv []string) (int, error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	go func() { _ = cmd.Wait() }()
	return pid, nil
}
