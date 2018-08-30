package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	// Subcommands
	standaloneCommand := flag.NewFlagSet("standalone", flag.ExitOnError)
	replsetCommand := flag.NewFlagSet("replset", flag.ExitOnError)
	shardedCommand := flag.NewFlagSet("sharded", flag.ExitOnError)

	// Standalone
	standaloneAuthPtr := standaloneCommand.Bool("auth", false, "use auth")
	standalonePortPtr := standaloneCommand.Int("port", 27017, "start on this port")

	// Replica set
	replsetAuthPtr := replsetCommand.Bool("auth", false, "use auth")
	replsetPortPtr := replsetCommand.Int("port", 27017, "start on this port")
	replsetNumPtr := replsetCommand.Int("num", 3, "run this many nodes")
	replsetConfigPtr := replsetCommand.String("cfg", "PSS", "configuration of the set")
	replsetNamePtr := replsetCommand.String("name", "replset", "name of the set")

	// Sharded cluster
	shardedAuthPtr := shardedCommand.Bool("auth", false, "use auth")
	shardedPortPtr := shardedCommand.Int("port", 27017, "start on this port")
	shardedNumPtr := shardedCommand.Int("num", 2, "run this many shards")
	shardedShardsvrPtr := shardedCommand.Int("shardsvr", 1, "run this many nodes per shard")
	shardedConfigSvrPtr := shardedCommand.Int("configsvr", 1, "run this many config servers")
	shardedShardsvrConfigPtr := shardedCommand.String("shardcfg", "P", "configuration of the shard replica set")

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
		helptext += "  ps -- show all running mongod/mongos\n"
		helptext += "  kill -- kill all running mongod/mongos\n"
		helptext += "  rm -- remove the data/ directory\n"
		fmt.Println(helptext)
		os.Exit(1)
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	// FlagSet.Parse() requires a set of arguments to parse as input
	// os.Args[2:] will be all arguments starting after the subcommand at os.Args[1]
	switch os.Args[1] {
	case "ps":
		Util_ps()
	case "kill":
		Util_kill()
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
		ST_deploy_standalone(*standalonePortPtr, *standaloneAuthPtr)
	}

	re_config, _ := regexp.Compile("(?i)PS*A*")

	// Replica set
	if replsetCommand.Parsed() {
		var rsNum int = *replsetNumPtr
		var rsCfg string = strings.ToUpper(*replsetConfigPtr)

		switch {
		case rsNum != 3:
			rsCfg = "P" + strings.Repeat("S", rsNum-1)
		case rsCfg != "PSS" && re_config.MatchString(rsCfg):
			rsNum = len(rsCfg)
		case !re_config.MatchString(rsCfg):
			fmt.Println("Invalid replica set configuration.")
			replsetCommand.PrintDefaults()
			os.Exit(1)
		}

		RS_deploy_replset(rsNum, *replsetPortPtr, rsCfg, *replsetNamePtr, *replsetAuthPtr)
	}

	// Sharded cluster
	if shardedCommand.Parsed() {

		var shNum int = *shardedShardsvrPtr
		var shCfg string = strings.ToUpper(*shardedShardsvrConfigPtr)

		switch {
		case shNum != 1:
			shCfg = "P" + strings.Repeat("S", shNum-1)
		case shCfg != "P" && re_config.MatchString(shCfg):
			shNum = len(shCfg)
		case !re_config.MatchString(shCfg):
			fmt.Println("Invalid replica set configuration.")
			shardedCommand.PrintDefaults()
			os.Exit(1)
		}

		fmt.Println("# Auth:", *shardedAuthPtr)
		fmt.Println("# Port:", *shardedPortPtr)
		fmt.Println("# Shards:", *shardedNumPtr)
		fmt.Println("# ShardSvr Num:", shNum)
		fmt.Println("# ShardSvr Config:", shCfg)
		fmt.Println("# Config servers:", *shardedConfigSvrPtr)

		shardservers := SH_deploy_shardsvr(*shardedNumPtr, shNum, shCfg, *shardedPortPtr+1, *shardedAuthPtr)

		configservers := SH_deploy_configsvr(*shardedConfigSvrPtr, *shardedPortPtr+(shNum*(*shardedNumPtr))+1, *shardedAuthPtr)

		SH_deploy_mongos(configservers, shardservers, *shardedPortPtr, *shardedAuthPtr)
	}

}
