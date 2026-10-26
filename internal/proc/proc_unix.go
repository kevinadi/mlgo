//go:build !windows

package proc

import (
	"os"
	"syscall"
)

func signal(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
