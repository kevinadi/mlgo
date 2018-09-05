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
		st_cmd_array := ST_deploy_standalone(*standalonePortPtr, *standaloneAuthPtr)

		if *standaloneScriptPtr {
			Util_print_script(st_cmd_array)
		} else {
			Util_runcommand_string_string(st_cmd_array)
			//Util_create_start_script(st_cmd)
		}
	}

	// Replica set
	re_config, _ := regexp.Compile("(?i)PS*A*")
	if replsetCommand.Parsed() {
		var rs_cmd string = ""
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

		rs_cmd += fmt.Sprintf("# Auth: %t\n", *replsetAuthPtr)
		rs_cmd += fmt.Sprintf("# Replica set nodes: %d\n", rsNum)
		rs_cmd += fmt.Sprintf("# Nodes configuration: %s\n", rsCfg)
		rs_config_summary := rs_cmd

		rs_cmd += fmt.Sprintf("\n")

		rs_cmdlines, rs_calls := RS_deploy_replset(rsNum, *replsetPortPtr, rsCfg, *replsetNamePtr, *replsetAuthPtr)

		rs_cmd += rs_cmdlines + "\n"
		rs_cmd += fmt.Sprintf("\n")
		rs_cmd += fmt.Sprintf("%s\n", Util_start_script(rs_calls))

		if *replsetScriptPtr {
			fmt.Print(rs_cmd)
		} else {
			fmt.Println(rs_config_summary)
			Util_runcommand_stdout(rs_cmd)
		}
	}

	// Sharded cluster
	if shardedCommand.Parsed() {
		var sh_cmd string = ""
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

		sh_cmd += fmt.Sprintf("# Auth: %t\n", *shardedAuthPtr)
		sh_cmd += fmt.Sprintf("# mongos port: %d\n", *shardedPortPtr)
		sh_cmd += fmt.Sprintf("# Number of shards: %d\n", *shardedNumPtr)
		sh_cmd += fmt.Sprintf("# ShardSvr replica set num: %d\n", shNum)
		sh_cmd += fmt.Sprintf("# ShardSvr configuration: %s\n", shCfg)
		sh_cmd += fmt.Sprintf("# Config servers: %d\n", *shardedConfigSvrPtr)
		sh_config_summary := sh_cmd

		sh_cmd += fmt.Sprintf("\n")
		if *shardedAuthPtr {
			sh_cmd += fmt.Sprintf(Util_create_dbpath()) + "\n"
			sh_cmd += fmt.Sprintf(Util_create_keyfile())
		}

		sh_shards, shardservers, shrd_calls := SH_deploy_shardsvr(*shardedNumPtr, shNum, shCfg, *shardedPortPtr+1, *shardedAuthPtr)
		sh_config, configservers, cfg_calls := SH_deploy_configsvr(*shardedConfigSvrPtr, *shardedPortPtr+(shNum*(*shardedNumPtr))+1, *shardedAuthPtr)
		sh_mongos, mongos_calls := SH_deploy_mongos(configservers, shardservers, *shardedPortPtr, *shardedAuthPtr)

		sh_cmd += sh_shards + "\n" + sh_config + "\n" + sh_mongos
		sh_cmd += fmt.Sprintf("\n")
		sh_cmd += fmt.Sprintf(Util_start_script(shrd_calls + "\n" + cfg_calls + "\n" + mongos_calls))

		if *shardedScriptPtr {
			fmt.Println(sh_cmd)
		} else {
			fmt.Println(sh_config_summary)
			Util_runcommand_stdout(sh_cmd)
		}
	}
}
