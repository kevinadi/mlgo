package main

import (
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kevinadi/mlgo/internal/mongocmd"
)

func TestUsageSeparatesStartAndKill(t *testing.T) {
	text := usage()
	if strings.Contains(text, "criteria is a literal substring of the command, or an exact pid") {
		t.Fatalf("start and kill share one criteria sentence:\n%s", text)
	}
	if !strings.Contains(text, "start criteria matches a literal substring") || !strings.Contains(text, "kill criteria is an exact pid") {
		t.Fatalf("usage =\n%s", text)
	}
	if !strings.Contains(text, "-shardcfg and -shardsvr") {
		t.Fatalf("usage =\n%s", text)
	}
}

func TestFlagErrorIsPrintedOnce(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	runErr := run([]string{"rs", "-3"})
	writeError(runErr)
	w.Close()
	os.Stderr = old
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if runErr == nil || !isShown(runErr) {
		t.Fatalf("err = %v", runErr)
	}
	n := strings.Count(string(body), "flag provided but not defined: -3")
	if n != 1 {
		t.Fatalf("printed %d times:\n%s", n, body)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	if err := run([]string{"install-m"}); err == nil {
		t.Fatal("install-m was removed")
	}
	if err := run([]string{"rs", "-3", "-script"}); err == nil {
		t.Fatal("-3 was removed; use -num")
	}
	if err := run([]string{"rs", "-cfg", "PSA", "-num", "2", "-script"}); err == nil {
		t.Fatal("expected -cfg and -num to conflict")
	}
	if err := run([]string{"sh", "-shardsvr", "2", "-shardcfg", "PSA", "-script"}); err == nil {
		t.Fatal("expected shard configuration conflict")
	}
	if err := run([]string{"rs", "-cfg", "P|", "-script"}); err == nil {
		t.Fatal("expected invalid configuration")
	}
	if err := run([]string{"rm", "extra"}); err == nil {
		t.Fatal("rm does not take arguments")
	}
}

func TestNoInitOmitsInitiate(t *testing.T) {
	body := captureStdout(t, func() error {
		return run([]string{"rs", "-init=false", "-script"})
	})
	if strings.Contains(body, "rs.initiate") {
		t.Fatalf("script initiated the set:\n%s", body)
	}
	if !strings.Contains(body, "# Num nodes: 1") {
		t.Fatalf("default replica set is one node:\n%s", body)
	}
}

func TestShardCfgDeterminesSize(t *testing.T) {
	body := captureStdout(t, func() error {
		return run([]string{"sh", "-shardcfg", "PSH", "-script"})
	})
	if !strings.Contains(body, "# ShardSvr Cfg: PSH") || !strings.Contains(body, `"hidden":true`) {
		t.Fatalf("script:\n%s", body)
	}
	if strings.Count(body, "sh.addShard(") != 2 {
		t.Fatalf("addShard count:\n%s", body)
	}
	if !strings.Contains(body, "--shardsvr") || !strings.Contains(body, "--configsvr") {
		t.Fatalf("missing server roles:\n%s", body)
	}
}

func TestScriptOmitsForkWhenUnsupported(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("fork rejection is a macOS mongod 8.2+ behavior")
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
	body := captureStdout(t, func() error {
		return run([]string{"st", "-script"})
	})
	if scriptPassesFork(body) {
		t.Fatalf("script still passes --fork:\n%s", body)
	}
	if !strings.Contains(body, "setsid") || !strings.Contains(body, "rejects --fork") {
		t.Fatalf("script:\n%s", body)
	}
}

func scriptPassesFork(script string) bool {
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "--fork") {
			return true
		}
	}
	return false
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	type result struct {
		body []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		body, err := io.ReadAll(r)
		done <- result{body, err}
	}()
	stdout := os.Stdout
	os.Stdout = w
	var runErr error
	func() {
		defer func() {
			os.Stdout = stdout
			w.Close()
		}()
		runErr = fn()
	}()
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if runErr != nil {
		t.Fatal(runErr)
	}
	return string(got.body)
}
