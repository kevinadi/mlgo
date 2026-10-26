package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

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
