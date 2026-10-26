package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/kevinadi/mlgo/internal/deploy"
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
	standaloneCommand := flag.NewFlagSet("standalone", flag.ExitOnError)
	replsetCommand := flag.NewFlagSet("replset", flag.ExitOnError)
	shardedCommand := flag.NewFlagSet("sharded", flag.ExitOnError)

	standaloneAuth := standaloneCommand.Bool("auth", false, "use auth")
	standalonePort := standaloneCommand.Int("port", 27017, "start on this port")
	standaloneScript := standaloneCommand.Bool("script", false, "print deployment script")

	replsetAuth := replsetCommand.Bool("auth", false, "use auth")
	replsetPort := replsetCommand.Int("port", 27017, "start on this port")
	replsetNum := replsetCommand.Int("num", 1, "run this many nodes")
	replsetConfig := replsetCommand.String("cfg", "", "configuration of the set")
	replsetName := replsetCommand.String("name", "replset", "name of the set")
	replsetScript := replsetCommand.Bool("script", false, "print deployment script")
	replsetNoInit := replsetCommand.Bool("noinit", false, "don't initiate the replica set")
	replset1 := replsetCommand.Bool("1", false, "initiate a single-node replica set")
	replset3 := replsetCommand.Bool("3", false, "initiate a three-node replica set")
	replset5 := replsetCommand.Bool("5", false, "initiate a five-node replica set")
	replset7 := replsetCommand.Bool("7", false, "initiate a seven-node replica set")

	shardedAuth := shardedCommand.Bool("auth", false, "use auth")
	shardedPort := shardedCommand.Int("port", 27017, "start on this port")
	shardedNum := shardedCommand.Int("num", 2, "run this many shards")
	shardedShardsvr := shardedCommand.Int("shardsvr", 1, "run this many nodes per shard")
	shardedConfigSvr := shardedCommand.Int("configsvr", 1, "run this many config servers")
	shardedShardCfg := shardedCommand.String("shardcfg", "", "configuration of the shard replica set")
	shardedScript := shardedCommand.Bool("script", false, "print deployment script")

	if len(args) < 1 {
		fmt.Print(usage())
		return printProcesses()
	}

	switch args[0] {
	case "install-m":
		where := ""
		if len(args) == 2 {
			where = args[1]
		}
		fmt.Println(installM(where))
		return nil
	case "start":
		what := ""
		if len(args) == 2 {
			what = args[1]
		}
		return proc.Start(what)
	case "kill", "stop":
		spec, err := killSpec(args)
		if err != nil {
			return err
		}
		return proc.Kill(spec)
	case "rm":
		return proc.RemoveData()
	case "standalone", "st":
		standaloneCommand.Parse(args[1:])
	case "replset", "rs":
		replsetCommand.Parse(args[1:])
	case "sharded", "sh":
		shardedCommand.Parse(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage())
	}

	if standaloneCommand.Parsed() {
		return launch(deploy.BuildStandalone(*standalonePort, options(*standaloneAuth, *standaloneScript)))
	}
	if replsetCommand.Parsed() {
		num := *replsetNum
		shortcut := *replset1 || *replset3 || *replset5 || *replset7
		switch {
		case *replset1:
			num = 1
		case *replset3:
			num = 3
		case *replset5:
			num = 5
		case *replset7:
			num = 7
		}
		cfgSet, numSet := false, false
		replsetCommand.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "cfg":
				cfgSet = true
			case "num":
				numSet = true
			}
		})
		opt := options(*replsetAuth, *replsetScript)
		opt.NoInit = *replsetNoInit
		return launch(deploy.BuildReplica(*replsetName, *replsetPort, *replsetConfig, num, cfgSet, numSet || shortcut, opt))
	}
	if shardedCommand.Parsed() {
		cfgSet, numSet := false, false
		shardedCommand.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "shardcfg":
				cfgSet = true
			case "shardsvr":
				numSet = true
			}
		})
		return launch(deploy.BuildSharded(*shardedPort, *shardedNum, *shardedShardsvr, *shardedConfigSvr, *shardedShardCfg, cfgSet, numSet, options(*shardedAuth, *shardedScript)))
	}
	return nil
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
		Cache:   "1",
		Shell:   shell,
		Windows: runtime.GOOS == "windows",
		CWD:     cwd,
	}
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

func usage() string {
	text := fmt.Sprintf("%s %s %s\n\n", os.Args[0], version, date)
	text += "Usage:\n"
	text += "  standalone (st) -- run a standalone node\n"
	text += "  replset (rs) -- run a replica set\n"
	text += "  sharded (sh) -- run a sharded cluster\n"
	text += "\n"
	text += "  start [criteria] -- start some mongod/mongos using the start.sh script\n"
	text += "  kill/stop [all|criteria] -- kill running mongod/mongos under the current directory\n"
	text += "  rm -- remove the data directory\n"
	text += "\n"
	text += "  [criteria] for start and kill is a literal substring of the command, or an exact pid, port, or replica set name\n"
	text += "\n"
	text += "Examples:\n"
	text += "  mlgo rs                # Start a basic single-node replica set\n"
	text += "  mlgo rs -cfg PSA       # Start a 3-node replica set with Primary-Secondary-Arbiter configuration\n"
	text += "  mlgo rs -cfg PSH       # Start a 3-node replica set with Primary-Secondary-Hidden configuration\n"
	text += "  mlgo rs -3             # Start a 3-node replica set\n"
	text += "  mlgo sh                # Start a basic 2 shards, 1 node per shard, 1 config server\n"
	text += "  mlgo sh -shardcfg PSA  # Start a 2 shards, PSA configuration on shards, 1 config server\n"
	return text
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

func installM(where string) string {
	cmdline := "curl https://raw.githubusercontent.com/aheckmann/m/master/bin/m"
	if where != "" {
		return cmdline + " > " + where + "/m && chmod 755 " + where + "/m"
	}
	return cmdline + " > /usr/local/bin/m && chmod 755 /usr/local/bin/m"
}
