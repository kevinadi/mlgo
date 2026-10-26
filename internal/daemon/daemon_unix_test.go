//go:build !windows

package daemon

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTimeoutIncludesStderrAndLog(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("perl is required")
	}
	logPath := filepath.Join(t.TempDir(), "mongod.log")
	if err := os.WriteFile(logPath, []byte("log-tail-marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, err := StartAndWait([]string{
		"perl", "-e", `print STDERR "early-fail\n"; sleep 30`,
		"--", "--logpath", logPath,
	}, 1, time.Second)
	if pid > 0 {
		t.Cleanup(func() { stopPid(pid) })
	}
	if err == nil {
		t.Fatal("expected timeout")
	}
	msg := err.Error()
	if !strings.Contains(msg, "early-fail") || !strings.Contains(msg, "log-tail-marker") || !strings.Contains(msg, "left the process running") {
		t.Fatalf("error = %s", msg)
	}
}

func stopPid(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if proc.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestProbeErrorIsReported(t *testing.T) {
	orig := listenProbe
	listenProbe = func(pid, port int) (bool, error) { return false, errors.New("lsof missing") }
	t.Cleanup(func() { listenProbe = orig })
	start := time.Now()
	pid, err := StartAndWait([]string{"sleep", "30"}, 1, 30*time.Second)
	if pid > 0 {
		t.Cleanup(func() { stopPid(pid) })
	}
	if err == nil || !strings.Contains(err.Error(), "lsof missing") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("waited out the timeout instead of reporting the probe error")
	}
}
