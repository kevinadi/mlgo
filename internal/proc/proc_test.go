package proc

import (
	"strings"
	"testing"
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
