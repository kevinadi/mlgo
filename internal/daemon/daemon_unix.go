//go:build !windows

package daemon

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// StartAndWait starts argv in a new session and returns when that process
// is listening on port, or when it exits. The process keeps running after
// a successful return. pid is the server's pid.
func StartAndWait(argv []string, port int, timeout time.Duration) (int, error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("empty command")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %d", port)
	}
	if timeout <= 0 {
		timeout = WaitTimeout
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = nil
	buf := &tailBuf{max: 8 << 10}
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(timeout)
	probeFailures := 0
	for {
		select {
		case err := <-exited:
			return 0, exitError(port, err, buf.String(), readTail(argValue(argv, "--logpath"), 4<<10))
		default:
		}
		listening, probeErr := listenProbe(pid, port)
		if listening {
			return pid, nil
		}
		if probeErr != nil {
			probeFailures++
			if probeFailures >= maxProbeFailures {
				return pid, withDetail(fmt.Errorf("cannot check whether pid %d is listening on port %d: %w; left the process running", pid, port, probeErr), buf.String(), readTail(argValue(argv, "--logpath"), 4<<10))
			}
		} else {
			probeFailures = 0
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return pid, timeoutError(port, timeout, pid, buf.String(), readTail(argValue(argv, "--logpath"), 4<<10))
		}
		wait := 200 * time.Millisecond
		if remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case err := <-exited:
			timer.Stop()
			return 0, exitError(port, err, buf.String(), readTail(argValue(argv, "--logpath"), 4<<10))
		case <-timer.C:
		}
	}
}

func exitError(port int, err error, stderr, log string) error {
	if err == nil {
		err = fmt.Errorf("exit 0")
	}
	return withDetail(fmt.Errorf("server on port %d exited: %w", port, err), stderr, log)
}

func timeoutError(port int, timeout time.Duration, pid int, stderr, log string) error {
	return withDetail(fmt.Errorf("still starting on port %d after %s (pid %d); left the process running", port, timeout, pid), stderr, log)
}

func withDetail(err error, stderr, log string) error {
	msg := strings.TrimSpace(stderr)
	log = strings.TrimSpace(log)
	switch {
	case msg != "" && log != "":
		msg = msg + "\n" + log
	case log != "":
		msg = log
	}
	if msg == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, msg)
}

// StartDetached is the Windows launcher. Unix starts a server with
// StartAndWait or with --fork.
func StartDetached(argv []string) (int, error) {
	return 0, fmt.Errorf("detached Windows start is not used on this system")
}

func pidListening(pid, port int) bool {
	ok, _ := listenProbe(pid, port)
	return ok
}

func argValue(argv []string, name string) string {
	prefix := name + "="
	for i, arg := range argv {
		if arg == name && i+1 < len(argv) {
			return argv[i+1]
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func readTail(path string, max int) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return ""
	}
	start := int64(0)
	if info.Size() > int64(max) {
		start = info.Size() - int64(max)
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(max)))
	if err != nil && len(b) == 0 {
		return ""
	}
	return string(b)
}

// tailBuf keeps the last max bytes written. Write never blocks and always
// reports that it accepted p, so a pipe into it cannot stall the server.
type tailBuf struct {
	mu  sync.Mutex
	b   []byte
	max int
}

func (t *tailBuf) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if t.max > 0 && len(t.b) > t.max {
		t.b = append([]byte(nil), t.b[len(t.b)-t.max:]...)
	}
	return len(p), nil
}

func (t *tailBuf) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}

// maxProbeFailures is how many consecutive listen probes may fail (about
// two seconds) before StartAndWait reports the probe error.
const maxProbeFailures = 10

// listenProbe reports whether pid listens on port. A process that is not
// visible yet is not an error; a failing connection probe (for example a
// missing lsof) is.
var listenProbe = func(pid, port int) (bool, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false, nil
	}
	conns, err := proc.Connections()
	if err != nil {
		return false, err
	}
	for _, conn := range conns {
		if strings.EqualFold(conn.Status, "LISTEN") && int(conn.Laddr.Port) == port {
			return true, nil
		}
	}
	return false, nil
}
