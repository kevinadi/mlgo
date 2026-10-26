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
	if !strings.Contains(got, `if out=$(`) || !strings.Contains(got, "grep -q -x pong") {
		t.Fatalf("wait script =\n%s", got)
	}
	if strings.Contains(got, "grep -q -w 1") {
		t.Fatalf("wait script matches a bare 1:\n%s", got)
	}
}
