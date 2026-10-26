package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/kevinadi/mlgo/internal/deploy"
	"github.com/kevinadi/mlgo/internal/mongocmd"
	"github.com/kevinadi/mlgo/internal/proc"
)

var (
	version string
	date    string
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		fmt.Print(usage())
		return printProcesses()
	}
	switch args[0] {
	case "start":
		what, err := optionalArg(args)
		if err != nil {
			return err
		}
		return proc.Start(what)
	case "kill", "stop":
		if err := extraArgs(args); err != nil {
			return err
		}
		spec, err := killSpec(args)
		if err != nil {
			return err
		}
		return proc.Kill(spec)
	case "rm":
		if len(args) > 1 {
			return fmt.Errorf("unexpected argument %q", args[1])
		}
		return proc.RemoveData()
	case "standalone", "st":
		return runStandalone(args[1:])
	case "replset", "rs":
		return runReplica(args[1:])
	case "sharded", "sh":
		return runSharded(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage())
	}
}

type commonFlags struct {
	auth   *bool
	port   *int
	script *bool
}

func newSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", name)
		fs.PrintDefaults()
	}
	return fs
}

func addCommon(fs *flag.FlagSet) commonFlags {
	return commonFlags{
		auth:   fs.Bool("auth", false, "enable authentication and create user \"user\" with password \"password\""),
		port:   fs.Int("port", 27017, "first port (mongos port for a sharded cluster)"),
		script: fs.Bool("script", false, "print the deployment script instead of running it"),
	}
}

func parseSet(fs *flag.FlagSet, args []string) (help bool, err error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	if fs.NArg() != 0 {
		return false, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return false, nil
}

func runStandalone(args []string) error {
	fs := newSet("standalone")
	common := addCommon(fs)
	help, err := parseSet(fs, args)
	if help || err != nil {
		return err
	}
	return launch(deploy.BuildStandalone(*common.port, options(*common.auth, *common.script)))
}

func runReplica(args []string) error {
	fs := newSet("replset")
	common := addCommon(fs)
	num := fs.Int("num", 1, "member count used when -cfg is omitted")
	cfg := fs.String("cfg", "", "member roles: P, then S, then optional A or H (for example PSA or PSH)")
	name := fs.String("name", "replset", "replica set name")
	init := fs.Bool("init", true, "initiate the replica set (pass -init=false to skip)")
	help, err := parseSet(fs, args)
	if help || err != nil {
		return err
	}
	cfgSet, numSet := visited(fs, "cfg", "num")
	opt := options(*common.auth, *common.script)
	opt.NoInit = !*init
	return launch(deploy.BuildReplica(*name, *common.port, *cfg, *num, cfgSet, numSet, opt))
}

func runSharded(args []string) error {
	fs := newSet("sharded")
	common := addCommon(fs)
	num := fs.Int("num", 2, "number of shards")
	shardsvr := fs.Int("shardsvr", 1, "members per shard when -shardcfg is omitted")
	configsvr := fs.Int("configsvr", 1, "config servers")
	shardcfg := fs.String("shardcfg", "", "shard member roles: P, then S, then optional A or H")
	help, err := parseSet(fs, args)
	if help || err != nil {
		return err
	}
	cfgSet, numSet := visited(fs, "shardcfg", "shardsvr")
	return launch(deploy.BuildSharded(*common.port, *num, *shardsvr, *configsvr, *shardcfg, cfgSet, numSet, options(*common.auth, *common.script)))
}

func visited(fs *flag.FlagSet, a, b string) (bool, bool) {
	gotA, gotB := false, false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case a:
			gotA = true
		case b:
			gotB = true
		}
	})
	return gotA, gotB
}

func options(auth, script bool) deploy.Options {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	shell := deploy.FindShell()
	if shell == "" {
		shell = "mongosh"
	}
	return deploy.Options{
		Auth:    auth,
		Script:  script,
		Cache:   cacheSize(),
		Shell:   shell,
		Windows: runtime.GOOS == "windows",
		CWD:     cwd,
	}
}

func cacheSize() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	size, err := mongocmd.DetectCacheSize(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaving the WiredTiger cache at the server default: %v\n", err)
		return ""
	}
	return size
}

func launch(plan deploy.Plan, err error) error {
	if err != nil {
		return err
	}
	if plan.Options.Script {
		fmt.Print(plan.Render())
		return nil
	}
	if plan.NeedsShell() {
		if _, err := exec.LookPath(plan.Options.Shell); err != nil {
			return fmt.Errorf("mongosh or mongo is required on PATH")
		}
	}
	return deploy.Execute(context.Background(), plan, deploy.Local{}, nil)
}

func optionalArg(args []string) (string, error) {
	if err := extraArgs(args); err != nil {
		return "", err
	}
	if len(args) == 2 {
		return args[1], nil
	}
	return "", nil
}

func extraArgs(args []string) error {
	if len(args) > 2 {
		return fmt.Errorf("unexpected argument %q", args[2])
	}
	return nil
}

func printProcesses() error {
	procs, err := proc.List()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Print(proc.Format(procs, cwd))
	return nil
}

func killSpec(args []string) (proc.Spec, error) {
	if len(args) < 2 {
		cwd, err := os.Getwd()
		if err != nil {
			return proc.Spec{}, err
		}
		return proc.Spec{CWD: cwd}, nil
	}
	if args[1] == "all" {
		return proc.Spec{All: true}, nil
	}
	return proc.Spec{Criteria: args[1]}, nil
}

func usage() string {
	text := fmt.Sprintf("%s %s %s\n\n", os.Args[0], version, date)
	text += "Usage:\n"
	text += "  standalone (st)  run one mongod\n"
	text += "  replset (rs)     run a replica set\n"
	text += "  sharded (sh)     run a sharded cluster\n"
	text += "\n"
	text += "  start [criteria]       start commands from mlgo-data/start.sh\n"
	text += "  kill|stop [all|criteria]  signal mongod/mongos (default: this directory)\n"
	text += "  rm                     remove mlgo-data when nothing is running\n"
	text += "\n"
	text += "  criteria is a literal substring of the command, or an exact pid, port, or replica set name\n"
	text += "\n"
	text += "  -cfg and -shardcfg take P, then any number of S, then any number of A or H.\n"
	text += "  When both -cfg and -num are set they must describe the same number of members.\n"
	text += "\n"
	text += "Examples:\n"
	text += "  mlgo rs                         # one-node replica set on port 27017\n"
	text += "  mlgo rs -num 3                  # primary and two secondaries\n"
	text += "  mlgo rs -cfg PSA                # primary, secondary, arbiter\n"
	text += "  mlgo rs -cfg PSH                # primary, secondary, hidden\n"
	text += "  mlgo rs -init=false             # start mongod processes without rs.initiate\n"
	text += "  mlgo sh                         # two shards, one mongod each, one config server\n"
	text += "  mlgo sh -shardcfg PSA           # two shards with a PSA set on each\n"
	text += "  mlgo sh -num 3 -shardsvr 3 -configsvr 3\n"
	return text
}
