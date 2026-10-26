//go:build darwin

package deploy

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kevinadi/mlgo/internal/mongocmd"
	"github.com/kevinadi/mlgo/internal/proc"
	"github.com/kevinadi/mlgo/internal/topology"
)

func TestLiveForkRejectedAndDetachedStart(t *testing.T) {
	if _, err := exec.LookPath("mongod"); err != nil {
		t.Skip("mongod is not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	text, err := mongocmd.VersionText(ctx)
	if err != nil {
		t.Skip(err)
	}
	if mongocmd.ForkSupported("darwin", text) {
		t.Skip("this mongod still accepts --fork")
	}

	dir := t.TempDir()
	forkDB := filepath.Join(dir, "fork-db")
	if err := os.MkdirAll(forkDB, 0o755); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	orig := startBackground
	var pid int
	t.Cleanup(func() {
		startBackground = orig
		var pids []int
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGTERM)
			pids = append(pids, pid)
		}
		pids = append(pids, stopByDBPath(forkDB)...)
		stopOwned(dir, port)
		waitDead(pids)
	})
	logPath := filepath.Join(dir, "fork.log")
	fork := exec.Command("mongod",
		"--port", strconv.Itoa(port),
		"--dbpath", forkDB,
		"--logpath", logPath,
		"--fork",
		"--bind_ip", "127.0.0.1",
		"--wiredTigerCacheSizeGB", "0.5",
	)
	out, err := fork.CombinedOutput()
	if err == nil {
		t.Fatalf("--fork succeeded:\n%s", out)
	}
	if !strings.Contains(string(out), "incompatible with macOS") {
		t.Fatalf("--fork error = %v\n%s", err, out)
	}

	startBackground = func(argv []string, gotPort int, timeout time.Duration) (int, error) {
		var startErr error
		pid, startErr = orig(argv, gotPort, timeout)
		return pid, startErr
	}
	plan, err := BuildStandalone(port, Options{
		CWD: dir, Cache: "0.5", Shell: "mongosh", NoFork: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Execute(context.Background(), plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	start, err := os.ReadFile(filepath.Join(dir, topology.DataDir, "start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(start), "--fork") {
		t.Fatalf("start.sh =\n%s", start)
	}

	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()

	if _, err := exec.LookPath("mongosh"); err == nil {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer pingCancel()
		var pingOut []byte
		var pingErr error
		for i := 0; i < 10; i++ {
			cmd := exec.CommandContext(pingCtx, "mongosh", "--port", strconv.Itoa(port), "--quiet", "--eval", mongocmd.PingEval)
			pingOut, pingErr = cmd.CombinedOutput()
			if pingErr == nil && strings.Contains(string(pingOut), "1") {
				pingErr = nil
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if pingErr != nil || !strings.Contains(string(pingOut), "1") {
			t.Fatalf("ping: %v\n%s", pingErr, pingOut)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func stopByDBPath(db string) []int {
	procs, err := proc.List()
	if err != nil {
		return nil
	}
	want := filepath.Clean(db)
	var pids []int
	for _, p := range procs {
		if filepath.Clean(p.DBPath) != want {
			continue
		}
		pids = append(pids, p.PID)
		_ = syscall.Kill(p.PID, syscall.SIGTERM)
	}
	return pids
}

func waitDead(pids []int) {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		alive := false
		for _, pid := range pids {
			if pid > 0 && syscall.Kill(pid, 0) == nil {
				alive = true
			}
		}
		if !alive {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func stopOwned(dir string, port int) {
	procs, err := proc.List()
	if err != nil {
		return
	}
	root := filepath.Clean(filepath.Join(dir, topology.DataDir))
	want := strconv.Itoa(port)
	var pids []int
	for _, p := range procs {
		if p.Port != want {
			continue
		}
		db := filepath.Clean(p.DBPath)
		if db != root && !strings.HasPrefix(db, root+string(filepath.Separator)) {
			continue
		}
		pids = append(pids, p.PID)
	}
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		alive := false
		for _, pid := range pids {
			if syscall.Kill(pid, 0) == nil {
				alive = true
			}
		}
		if !alive {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
