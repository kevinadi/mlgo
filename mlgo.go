package main

import (
	"flag"
	"fmt"
	"os"
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
	replsetNumPtr := replsetCommand.Int("num", 3, "run this many nodes")
	replsetConfigPtr := replsetCommand.String("cfg", "PSS", "configuration of the set")
	replsetNamePtr := replsetCommand.String("name", "replset", "name of the set")
	replsetScriptPtr := replsetCommand.Bool("script", false, "print deployment script")

	// Sharded cluster
	shardedAuthPtr := shardedCommand.Bool("auth", false, "use auth")
	shardedPortPtr := shardedCommand.Int("port", 27017, "start on this port")
	shardedNumPtr := shardedCommand.Int("num", 2, "run this many shards")
	shardedShardsvrPtr := shardedCommand.Int("shardsvr", 1, "run this many nodes per shard")
	shardedConfigSvrPtr := shardedCommand.Int("configsvr", 1, "run this many config servers")
	shardedShardsvrConfigPtr := shardedCommand.String("shardcfg", "P", "configuration of the shard replica set")
	shardedScriptPtr := shardedCommand.Bool("script", false, "print deployment script")

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		var helptext string
		helptext = "Usage:\n"
		helptext += "  standalone (st) -- run a standalone node\n"
		helptext += "  replset (rs) -- run a replica set\n"
		helptext += "  sharded (sh) -- run a sharded cluster\n"
		helptext += "\n"
		helptext += "  ps [criteria] -- show running mongod/mongos\n"
		helptext += "  start [criteria] -- start some mongod/mongos using the start.sh script\n"
		helptext += "  kill [criteria] -- kill running mongod/mongos\n"
		helptext += "  rm -- remove the data/ directory\n"
		helptext += "\n"
		helptext += "  [criteria] for ps, start, and kill is an expression that will restrict the output or operations of the command\n"
		fmt.Println(helptext)
		os.Exit(1)
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
	case "ps":
		if len(os.Args) == 3 {
			fmt.Println(Util_ps(os.Args[2]))
		} else {
			fmt.Println(Util_ps(""))
		}
	case "start":
		if len(os.Args) == 3 {
			Util_start_process(os.Args[2])
		} else {
			Util_start_process("")
		}
	case "kill":
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
		rs := new(ReplSet)
		rs.Init(*replsetNumPtr, *replsetPortPtr, *replsetConfigPtr, *replsetNamePtr, *replsetAuthPtr, *replsetScriptPtr)
		rs.Deploy()
	}

	// Sharded cluster
	if shardedCommand.Parsed() {
		sh := new(Sharded)
		sh.Init(*shardedPortPtr, *shardedNumPtr, *shardedShardsvrPtr, *shardedShardsvrConfigPtr, *shardedConfigSvrPtr, *shardedAuthPtr, *shardedScriptPtr)
		sh.Deploy()
	}
}
