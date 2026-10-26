//go:build windows

package proc

import (
	"os"
	"os/exec"
	"strconv"
)

func signal(pid int) error {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
