package proc

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kevinadi/mlgo/internal/topology"
)

func TestIsServer(t *testing.T) {
	if !isServer("mongod") || !isServer("mongos") || !isServer("mongod.exe") {
		t.Fatal("expected server names")
	}
	if isServer("mongodump") || isServer("mongo") || isServer("notmongod") {
		t.Fatal("non-servers matched")
	}
}

func TestParseArgsAndWorkDir(t *testing.T) {
	p := ParseArgs(10, []string{
		"mongod", "--port", "27017", "--dbpath", "/work/mlgo-data/27017",
		"--logpath", "/work/mlgo-data/27017/mongod.log", "--replSet", "shard00",
	})
	if p.Port != "27017" || p.ReplSet != "shard00" || p.WorkDir != "/work" {
		t.Fatalf("%+v", p)
	}
	eq := ParseArgs(11, []string{"mongos", "--port=27018", "--logpath=/work/mlgo-data/27018/mongos.log"})
	if eq.Port != "27018" || eq.WorkDir != "/work" || eq.DBPath != "" {
		t.Fatalf("%+v", eq)
	}
	if WorkDir("/other/path") != "" {
		t.Fatal("expected empty workdir")
	}
}

func TestFilter(t *testing.T) {
	procs := []Process{
		ParseArgs(1, []string{"mongod", "--port", "27017", "--dbpath", "/work/mlgo-data/27017", "--replSet", "shard00"}),
		ParseArgs(2, []string{"mongod", "--port", "27018", "--dbpath", "/work/mlgo-data/27018", "--replSet", "shard01"}),
		ParseArgs(3, []string{"mongod", "--port", "27019", "--dbpath", "/else/mlgo-data/27019", "--replSet", "other"}),
	}
	cwd := Filter(procs, Spec{CWD: "/work"})
	if len(cwd) != 2 {
		t.Fatalf("cwd matches = %d", len(cwd))
	}
	if len(Filter(procs, Spec{All: true})) != 3 {
		t.Fatal("all")
	}
	bySet := Filter(procs, Spec{Criteria: "shard00"})
	if len(bySet) != 1 || bySet[0].PID != 1 {
		t.Fatalf("replset match = %+v", bySet)
	}
	byPort := Filter(procs, Spec{Criteria: "27018"})
	if len(byPort) != 1 || byPort[0].PID != 2 {
		t.Fatalf("port match = %+v", byPort)
	}
	if len(Filter(procs, Spec{Criteria: "shard0."})) != 0 {
		t.Fatal("criteria must not be a regular expression")
	}
}

func TestWorkspaceFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating the link needs a privilege this test does not request")
	}
	real := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(real, topology.DataDir, "27017")
	recorded := ParseArgs(1, []string{"mongod", "--port", "27017", "--dbpath", db})
	if !inWorkspace(recorded, link) {
		t.Fatalf("real dbpath %q was not in %q (workdir %q)", db, link, recorded.WorkDir)
	}
	viaLink := filepath.Join(link, topology.DataDir, "27017")
	started := ParseArgs(2, []string{"mongod", "--port", "27017", "--dbpath", viaLink})
	if !inWorkspace(started, real) {
		t.Fatalf("link dbpath %q was not in %q (workdir %q)", viaLink, real, started.WorkDir)
	}
}

func TestWorkspaceFoldsCase(t *testing.T) {
	if !foldPathCase() {
		t.Skip("this filesystem compares paths with case")
	}
	dir := t.TempDir()
	base := filepath.Join(dir, "MlgoCase")
	if err := os.Mkdir(base, 0o755); err != nil {
		t.Fatal(err)
	}
	folded := filepath.Join(dir, "mlgocase")
	if _, err := os.Stat(folded); err != nil {
		t.Skip("filesystem is case sensitive")
	}
	if !inWorkspace(Process{WorkDir: base}, folded) {
		t.Fatalf("%q and %q did not match", base, folded)
	}
}

func TestTmpAndPrivateTmpMatch(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("this is the macOS /tmp symlink")
	}
	info, err := os.Lstat("/tmp")
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Skip("/tmp is not a symlink")
	}
	leaf := "mlgo-workspace-path-missing"
	p := Process{
		WorkDir: "/tmp/" + leaf,
		DBPath:  "/tmp/" + leaf + "/mlgo-data/27017",
	}
	if !inWorkspace(p, "/private/tmp/"+leaf) {
		t.Fatal("/tmp and /private/tmp did not match")
	}
	if !underData("/tmp/"+leaf, "/private/tmp/"+leaf+"/mlgo-data/27017") {
		t.Fatal("dbpath prefix did not follow /tmp")
	}
}

func TestStartStripsForkWhenRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows starts through start /b and does not detach")
	}
	dir := t.TempDir()
	data := filepath.Join(dir, topology.DataDir)
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nmongod --port 27017 --dbpath /data/db --logpath /data/mongod.log --fork\nmongos --port 27018 --configdb config/localhost:27019 --logpath /data/mongos.log --fork\n"
	if err := os.WriteFile(filepath.Join(data, "start.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	origRejected := forkRejected
	forkRejected = func() bool { return true }
	t.Cleanup(func() { forkRejected = origRejected })
	var got [][]string
	var ports []int
	origStart := startBackground
	startBackground = func(argv []string, port int, timeout time.Duration) (int, error) {
		got = append(got, append([]string{}, argv...))
		ports = append(ports, port)
		if timeout <= 0 {
			t.Fatal("timeout")
		}
		return 7, nil
	}
	t.Cleanup(func() { startBackground = origStart })

	t.Chdir(dir)
	if err := Start(""); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || ports[0] != 27017 || ports[1] != 27018 {
		t.Fatalf("starts = %v ports %v", got, ports)
	}
	for _, argv := range got {
		if strings.Contains(strings.Join(argv, " "), "--fork") {
			t.Fatalf("fork flag survived: %v", argv)
		}
	}
}

func TestStripFork(t *testing.T) {
	got := stripFork([]string{"mongod", "--fork", "--port", "1", "--fork=true"})
	if strings.Join(got, " ") != "mongod --port 1" {
		t.Fatalf("%v", got)
	}
	if !hasForkFlag([]string{"mongod", "--fork"}) || hasForkFlag([]string{"mongod", "--port", "1"}) {
		t.Fatal("hasForkFlag")
	}
	if runtime.GOOS != "windows" && !shouldDetach([]string{"mongod", "--port", "1"}) {
		t.Fatal("expected detach")
	}
	if runtime.GOOS == "windows" && shouldDetach([]string{"mongod", "--port", "1"}) {
		t.Fatal("windows uses start /b")
	}
	if shouldDetach([]string{"mongod", "--port", "1", "--fork"}) {
		t.Fatal("forked server should be waited on")
	}
	if shouldDetach([]string{"mongodump", "--port", "1"}) {
		t.Fatal("mongodump is not a server")
	}
}

func TestSplit(t *testing.T) {
	argv, err := Split("mongod --port 27017 --dbpath '/tmp/a b' --fork")
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 6 || argv[4] != "/tmp/a b" {
		t.Fatalf("%#v", argv)
	}
	argv, err = Split(`mongosh --eval 'rs.initiate({"_id":"replset"})'`)
	if err != nil {
		t.Fatal(err)
	}
	if argv[2] != `rs.initiate({"_id":"replset"})` {
		t.Fatalf("%#v", argv)
	}
	if _, err := Split("mongod 'unterminated"); err == nil {
		t.Fatal("expected quote error")
	}
}

func TestFormatIsStable(t *testing.T) {
	procs := []Process{
		{PID: 2, Cmdline: "mongod b", WorkDir: "/b"},
		{PID: 1, Cmdline: "mongod a", WorkDir: "/a"},
	}
	got := Format(procs, "/a")
	if !strings.Contains(got, "/a (current directory)\n1 mongod a\n\n/b\n2 mongod b\n") {
		t.Fatalf("format = %q", got)
	}
}
