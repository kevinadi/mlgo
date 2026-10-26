package deploy

import (
	"context"
	"github.com/kevinadi/mlgo/internal/mongocmd"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevinadi/mlgo/internal/topology"
)

type scripted struct {
	out []byte
	err error
}

type fake struct {
	runs    [][]string
	failRun map[string]int
	outputs []scripted
	outN    int
	sleeps  int
}

func (f *fake) Run(_ context.Context, argv []string) error {
	f.runs = append(f.runs, append([]string{}, argv...))
	if len(argv) == 0 {
		return nil
	}
	if f.failRun[argv[0]] > 0 {
		f.failRun[argv[0]]--
		return os.ErrInvalid
	}
	return nil
}

func (f *fake) Output(_ context.Context, argv []string) ([]byte, error) {
	if f.outN < len(f.outputs) {
		s := f.outputs[f.outN]
		f.outN++
		return s.out, s.err
	}
	return []byte("true\n"), nil
}

func (f *fake) sleep(time.Duration) { f.sleeps++ }

func countArg0(runs [][]string, name string) int {
	n := 0
	for _, argv := range runs {
		if len(argv) > 0 && argv[0] == name {
			n++
		}
	}
	return n
}

func TestRenderMatchesDeployment(t *testing.T) {
	opt := Options{CWD: "/work", Shell: "mongosh", Cache: "1"}
	sharded, err := BuildSharded(27017, 2, 1, 1, "PSA", true, false, opt)
	if err != nil {
		t.Fatal(err)
	}
	script := sharded.Render()
	if strings.Count(script, "sh.addShard(") != 2 {
		t.Fatalf("script is missing addShard:\n%s", script)
	}
	if strings.Count(script, "rs.initiate(") != 3 {
		t.Fatalf("want initiate for 2 shards and config, script:\n%s", script)
	}
	if !strings.Contains(script, "--shardsvr") || !strings.Contains(script, "--configsvr") {
		t.Fatalf("missing server roles:\n%s", script)
	}
	if strings.Contains(script, "openssl") || strings.Contains(script, "--keyFile") || strings.Contains(script, "--auth") {
		t.Fatalf("auth steps present without -auth:\n%s", script)
	}
	if strings.Contains(script, "--wiredTigerCacheSizeGB") && !strings.Contains(script, "--wiredTigerCacheSizeGB 1") {
		t.Fatalf("cache flag:\n%s", script)
	}

	authOpt := opt
	authOpt.Auth = true
	authed, err := BuildSharded(27017, 1, 1, 1, "", false, false, authOpt)
	if err != nil {
		t.Fatal(err)
	}
	authScript := authed.Render()
	if !strings.Contains(authScript, "openssl rand -base64 32") || !strings.Contains(authScript, "--keyFile") {
		t.Fatalf("auth script missing keyfile:\n%s", authScript)
	}
	if strings.Contains(authScript, "--auth") {
		t.Fatalf("replica members should use keyFile, not --auth:\n%s", authScript)
	}
	if strings.Count(authScript, "createUser") != 1 {
		t.Fatalf("user should be created once, script:\n%s", authScript)
	}

	noUser := strings.Count(authScript, "createUser")
	if noUser != 1 {
		t.Fatal(noUser)
	}
}

func TestReplicaNoInitSkipsShell(t *testing.T) {
	plan, err := BuildReplica("configuration", 27017, "", 1, false, false, Options{
		CWD: "/work", Shell: "mongosh", Cache: "1", NoInit: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	script := plan.Render()
	if strings.Contains(script, "rs.initiate") || strings.Contains(script, "--configsvr") {
		t.Fatalf("script = %s", script)
	}
	if plan.NeedsShell() {
		t.Fatal("noinit without auth does not need a shell")
	}
}

func TestConflictingConfig(t *testing.T) {
	_, err := BuildReplica("replset", 27017, "PSA", 2, true, true, Options{CWD: "/work"})
	if err == nil {
		t.Fatal("expected conflict")
	}
}

func TestExecuteRetriesShellOnly(t *testing.T) {
	dir := t.TempDir()
	plan, err := BuildReplica("replset", 27017, "P", 1, true, false, Options{
		CWD: dir, Shell: "mongosh", Cache: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := &fake{
		failRun: map[string]int{"mongosh": 2},
		outputs: []scripted{{out: []byte("false\n")}, {out: []byte("true\n")}},
	}
	if err := Execute(context.Background(), plan, f, f.sleep); err != nil {
		t.Fatal(err)
	}
	if got := countArg0(f.runs, "mongod"); got != 1 {
		t.Fatalf("mongod runs = %d", got)
	}
	if got := countArg0(f.runs, "mongosh"); got != 3 {
		t.Fatalf("mongosh runs = %d, want 2 failures + success; runs=%v", got, f.runs)
	}
	if f.sleeps != 1+2 { // one wait sleep, two retry sleeps
		t.Fatalf("sleeps = %d", f.sleeps)
	}
	start, err := os.ReadFile(filepath.Join(dir, topology.DataDir, "start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(start), "--fork") || strings.Contains(string(start), "rs.initiate") {
		t.Fatalf("start.sh = %s", start)
	}
}

func TestMongodFailureStops(t *testing.T) {
	plan, err := BuildStandalone(27017, Options{CWD: t.TempDir(), Cache: "1", Shell: "mongosh"})
	if err != nil {
		t.Fatal(err)
	}
	f := &fake{failRun: map[string]int{"mongod": 5}}
	if err := Execute(context.Background(), plan, f, f.sleep); err == nil {
		t.Fatal("expected mongod failure")
	}
	if got := countArg0(f.runs, "mongod"); got != 1 {
		t.Fatalf("mongod was retried %d times", got)
	}
	if f.sleeps != 0 {
		t.Fatalf("sleeps = %d", f.sleeps)
	}
}

func TestKeyFileIsRandomAndPrivate(t *testing.T) {
	var bodies []string
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		plan, err := BuildStandalone(27017, Options{CWD: dir, Auth: true, Cache: "1", Shell: "mongosh"})
		if err != nil {
			t.Fatal(err)
		}
		// Standalone auth waits for ping, then createUser. No keyfile on standalone.
		f := &fake{outputs: []scripted{{out: []byte("pong\n")}}}
		if err := Execute(context.Background(), plan, f, f.sleep); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(f.runs[1], " ")
		if !strings.Contains(joined, "--auth") || strings.Contains(joined, "--keyFile") {
			t.Fatalf("standalone argv = %s", joined)
		}
	}

	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		plan, err := BuildReplica("replset", 27017, "", 1, false, false, Options{
			CWD: dir, Auth: true, Cache: "1", Shell: "mongosh",
		})
		if err != nil {
			t.Fatal(err)
		}
		f := &fake{outputs: []scripted{{out: []byte("true\n")}}}
		if err := Execute(context.Background(), plan, f, f.sleep); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, topology.DataDir, "keyfile.txt")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(strings.TrimSpace(string(body))) < 6 {
			t.Fatalf("keyfile too short: %q", body)
		}
		bodies = append(bodies, string(body))
		var sawKey bool
		for _, argv := range f.runs {
			if len(argv) > 0 && argv[0] == "mongod" && strings.Contains(strings.Join(argv, " "), "--keyFile") {
				sawKey = true
			}
		}
		if !sawKey {
			t.Fatalf("mongod missing keyFile: %v", f.runs)
		}
	}
	if bodies[0] == bodies[1] {
		t.Fatal("keyfiles were identical")
	}
}

func TestNoForkDetachesAndOmitsFlag(t *testing.T) {
	dir := t.TempDir()
	var started [][]string
	var ports []int
	orig := startBackground
	startBackground = func(argv []string, port int, timeout time.Duration) (int, error) {
		started = append(started, append([]string{}, argv...))
		ports = append(ports, port)
		if timeout <= 0 {
			t.Fatal("timeout")
		}
		return 1, nil
	}
	t.Cleanup(func() { startBackground = orig })

	opt := Options{CWD: dir, Shell: "mongosh", Cache: "0.5", NoFork: true}
	plan, err := BuildSharded(27017, 1, 1, 1, "P", true, false, opt)
	if err != nil {
		t.Fatal(err)
	}
	script := plan.Render()
	if hasForkArg(script) {
		t.Fatalf("script passes --fork:\n%s", script)
	}
	if !strings.Contains(script, "setsid") || !strings.Contains(script, "port_ready") {
		t.Fatalf("script does not wait for the server:\n%s", script)
	}
	if strings.Count(script, "Starting mongod on port") != 2 || !strings.Contains(script, "Starting mongos on port 27017") {
		t.Fatalf("script:\n%s", script)
	}
	if !strings.Contains(script, "rejects --fork") {
		t.Fatalf("missing summary:\n%s", script)
	}
	if strings.Contains(script, "mongod.log 2>&1") || strings.Contains(script, "mongos.log 2>&1") {
		t.Fatalf("script appends onto --logpath:\n%s", script)
	}
	if !strings.Contains(script, "mongod.log.launch") || !strings.Contains(script, "mongos.log.launch") {
		t.Fatalf("script missing launch sidecar:\n%s", script)
	}
	if strings.Contains(script, "grep -q -w") || !strings.Contains(script, "grep -q -x -F --") {
		t.Fatalf("readiness grep is not a whole line:\n%s", script)
	}

	f := &fake{outputs: []scripted{{out: []byte("true\n")}, {out: []byte("true\n")}, {out: []byte("pong\n")}}}
	if err := Execute(context.Background(), plan, f, f.sleep); err != nil {
		t.Fatal(err)
	}
	if countArg0(f.runs, "mongod") != 0 || countArg0(f.runs, "mongos") != 0 {
		t.Fatalf("executor ran a server in the foreground: %v", f.runs)
	}
	if len(started) != 3 {
		t.Fatalf("detached starts = %d", len(started))
	}
	for _, argv := range started {
		if strings.Contains(strings.Join(argv, " "), "--fork") {
			t.Fatalf("detached argv has --fork: %v", argv)
		}
	}
	if ports[0] == 0 || ports[2] != 27017 {
		t.Fatalf("ports = %v", ports)
	}
	start, err := os.ReadFile(filepath.Join(dir, topology.DataDir, "start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(start)
	if strings.Contains(text, "--fork") || strings.Contains(text, "nohup") || strings.Contains(text, "setsid") {
		t.Fatalf("start.sh should be plain argv:\n%s", text)
	}
	if strings.Count(text, "\nmongod ") != 2 || !strings.Contains(text, "\nmongos ") {
		t.Fatalf("start.sh = %s", text)
	}
	info, err := os.Stat(filepath.Join(dir, topology.DataDir, "start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 != 0 {
		t.Fatalf("start.sh is executable: %v", info.Mode())
	}
}

func TestScriptStopsOnFailure(t *testing.T) {
	plan, err := BuildStandalone(27017, Options{CWD: t.TempDir(), Shell: "mongosh", Cache: "1"})
	if err != nil {
		t.Fatal(err)
	}
	script := plan.Render()
	if !strings.HasPrefix(script, "#!/bin/sh\nset -e\n") {
		t.Fatalf("script does not stop on a failed command:\n%s", script)
	}
	fork := plan
	fork.Options.NoFork = true
	detached := fork.Render()
	if strings.Contains(detached, "return $?") {
		t.Fatalf("a failing lsof would abort under set -e:\n%s", detached)
	}
	if !strings.Contains(detached, "if lsof ") {
		t.Fatalf("listen check:\n%s", detached)
	}
}

func TestWaitScriptMatchesWholeLine(t *testing.T) {
	got := waitScript("mongo", 27017, "print(1)", "1", "mongod")
	if !strings.Contains(got, "grep -q -x -F -- 1") {
		t.Fatalf("wait script = %s", got)
	}
	if strings.Contains(got, "grep -q -w") {
		t.Fatalf("word match would treat \"exiting with code 1\" as ready:\n%s", got)
	}
}

func hasForkArg(script string) bool {
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

func TestExecArgvForkFlag(t *testing.T) {
	step := Step{Background: true, Argv: []string{"mongod", "--port", "27017"}}
	if !strings.Contains(strings.Join(ExecArgv(step, Options{}), " "), "--fork") {
		t.Fatal("unix argv lost --fork")
	}
	if strings.Contains(strings.Join(ExecArgv(step, Options{NoFork: true}), " "), "--fork") {
		t.Fatal("NoFork argv still has --fork")
	}
	win := strings.Join(ExecArgv(step, Options{Windows: true, NoFork: true}), " ")
	if !strings.Contains(win, "start /b") || strings.Contains(win, "--fork") {
		t.Fatalf("windows argv = %s", win)
	}
	saved := strings.Join(CommandArgv(step, Options{}), " ")
	if !strings.Contains(saved, "--fork") {
		t.Fatal("start.sh line lost --fork")
	}
}

func TestOutputSaysDoesNotTreatFalseAsTrue(t *testing.T) {
	if outputSays([]byte("false\n"), "true") {
		t.Fatal("false contains the letters true")
	}
	if !outputSays([]byte("true\n"), "true") {
		t.Fatal("expected true")
	}
}

func TestWaitScriptChecksShellStatus(t *testing.T) {
	got := waitScript("mongo", 27017, mongocmd.PingEval, mongocmd.PingToken, "mongod")
	if !strings.Contains(got, `if out=$(`) || !strings.Contains(got, "grep -q -x -F -- pong") {
		t.Fatalf("wait script =\n%s", got)
	}
	if strings.Contains(got, "grep -q -w 1") {
		t.Fatalf("wait script matches a bare 1:\n%s", got)
	}
}
