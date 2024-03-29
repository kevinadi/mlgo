package main

import (
	"flag"
	"fmt"
	"os"
)

const Datadir = "mlgo-data"

var (
	version string
	date    string
)

func main() {
	// Subcommands
	standaloneCommand := flag.NewFlagSet("standalone", flag.ExitOnError)
	replsetCommand := flag.NewFlagSet("replset", flag.ExitOnError)
	shardedCommand := flag.NewFlagSet("sharded", flag.ExitOnError)

	// Standalone
	standaloneAuthPtr := standaloneCommand.Bool("auth", false, "use auth")
	standalonePortPtr := standaloneCommand.Int("port", 27017, "start on this port")
	standaloneScriptPtr := standaloneCommand.Bool("script", false, "print deployment script")

	// Replica set
	replsetAuthPtr := replsetCommand.Bool("auth", false, "use auth")
	replsetPortPtr := replsetCommand.Int("port", 27017, "start on this port")
	replsetNumPtr := replsetCommand.Int("num", 1, "run this many nodes")
	replsetConfigPtr := replsetCommand.String("cfg", "", "configuration of the set")
	replsetNamePtr := replsetCommand.String("name", "replset", "name of the set")
	replsetScriptPtr := replsetCommand.Bool("script", false, "print deployment script")
	replsetInitPtr := replsetCommand.Bool("noinit", false, "don't initiate the replica set")
	replset1Ptr := replsetCommand.Bool("1", false, "initiate a single-node replica set")
	replset3Ptr := replsetCommand.Bool("3", false, "initiate a three-node replica set")
	replset5Ptr := replsetCommand.Bool("5", false, "initiate a five-node replica set")
	replset7Ptr := replsetCommand.Bool("7", false, "initiate a seven-node replica set")

	// Sharded cluster
	shardedAuthPtr := shardedCommand.Bool("auth", false, "use auth")
	shardedPortPtr := shardedCommand.Int("port", 27017, "start on this port")
	shardedNumPtr := shardedCommand.Int("num", 2, "run this many shards")
	shardedShardsvrPtr := shardedCommand.Int("shardsvr", 1, "run this many nodes per shard")
	shardedConfigSvrPtr := shardedCommand.Int("configsvr", 1, "run this many config servers")
	shardedShardsvrConfigPtr := shardedCommand.String("shardcfg", "", "configuration of the shard replica set")
	shardedScriptPtr := shardedCommand.Bool("script", false, "print deployment script")

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		var helptext string
		helptext = fmt.Sprintf("%s %s %s\n\n", os.Args[0], version, date)
		helptext += "Usage:\n"
		helptext += "  standalone (st) -- run a standalone node\n"
		helptext += "  replset (rs) -- run a replica set\n"
		helptext += "  sharded (sh) -- run a sharded cluster\n"
		helptext += "\n"
		helptext += "  start [criteria] -- start some mongod/mongos using the start.sh script\n"
		helptext += "  kill/stop [all|criteria] -- kill running mongod/mongos under the current directory\n"
		helptext += "  rm -- remove the data directory\n"
		helptext += "\n"
		helptext += "  [criteria] for ps, start, and kill is an expression that will restrict the output or operations of the command\n"
		helptext += "\n"
		helptext += "Examples:\n"
		helptext += "  mlgo rs                # Start a basic single-node replica set\n"
		helptext += "  mlgo rs -cfg PSA       # Start a 3-node replica set with Primary-Secondary-Arbiter configuration\n"
		helptext += "  mlgo rs -cfg PSH       # Start a 3-node replica set with Primary-Secondary-Hidden configuration\n"
		helptext += "  mlgo rs -3             # Start a 3-node replica set\n"
		helptext += "  mlgo sh                # Start a basic 2 shards, 1 node per shard, 1 config server\n"
		helptext += "  mlgo sh -shardcfg PSA  # Start a 2 shards, PSA configuration on shards, 1 config server\n"
		fmt.Println(helptext)
		if ps := Util_ps(""); ps != "" {
			fmt.Println(Util_ps_pretty())
		} else {
			fmt.Println("No running processes")
		}
		os.Exit(0)
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	// FlagSet.Parse() requires a set of arguments to parse as input
	// os.Args[2:] will be all arguments starting after the subcommand at os.Args[1]
	switch os.Args[1] {
	case "install-m":
		if len(os.Args) == 3 {
			fmt.Println(Util_install_m(os.Args[2]))
		} else {
			fmt.Println(Util_install_m(""))
		}
	case "start":
		if len(os.Args) == 3 {
			Util_start_process(os.Args[2])
		} else {
			Util_start_process("")
		}
	case "kill", "stop":
		if len(os.Args) == 3 {
			Util_kill(os.Args[2])
		} else {
			Util_kill("")
		}

	case "rm":
		Util_rm()
	case "standalone", "st":
		standaloneCommand.Parse(os.Args[2:])
	case "replset", "rs":
		replsetCommand.Parse(os.Args[2:])
	case "sharded", "sh":
		shardedCommand.Parse(os.Args[2:])
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Standalone
	if standaloneCommand.Parsed() {
		m := new(Mongod)
		m.Init(*standalonePortPtr, *standaloneAuthPtr, "", *standaloneScriptPtr)
		m.Deploy()
	}

	// Replica set
	if replsetCommand.Parsed() {
		var rs_num int
		rs := new(ReplSet)
		if *replset1Ptr {
			rs_num = 1
		} else if *replset3Ptr {
			rs_num = 3
		} else if *replset5Ptr {
			rs_num = 5
		} else if *replset7Ptr {
			rs_num = 7
		} else {
			rs_num = *replsetNumPtr
		}
		rs.Init(rs_num, *replsetPortPtr, *replsetConfigPtr, *replsetNamePtr, *replsetAuthPtr, *replsetInitPtr, *replsetScriptPtr)
		rs.Deploy()
	}

	// Sharded cluster
	if shardedCommand.Parsed() {
		sh := new(Sharded)
		sh.Init(*shardedPortPtr, *shardedNumPtr, *shardedShardsvrPtr, *shardedShardsvrConfigPtr, *shardedConfigSvrPtr, *shardedAuthPtr, *shardedScriptPtr)
		sh.Deploy()
	}
}
