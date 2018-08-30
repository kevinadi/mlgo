package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	// Subcommands
	standaloneCommand := flag.NewFlagSet("standalone", flag.ExitOnError)
	replsetCommand := flag.NewFlagSet("replset", flag.ExitOnError)
	psCommand := flag.NewFlagSet("ps", flag.ExitOnError)
	killCommand := flag.NewFlagSet("kill", flag.ExitOnError)
	rmCommand := flag.NewFlagSet("rm", flag.ExitOnError)

	// Standalone
	standaloneAuthPtr := standaloneCommand.Bool("auth", false, "use auth")
	standalonePortPtr := standaloneCommand.Int("port", 27017, "start on this port")

	// Replica set
	replsetAuthPtr := replsetCommand.Bool("auth", false, "use auth")
	replsetPortPtr := replsetCommand.Int("port", 27017, "start on this port")
	replsetNumPtr := replsetCommand.Int("num", 3, "run this many nodes")
	replsetConfigPtr := replsetCommand.String("cfg", "PSS", "configuration of the set")
	replsetNamePtr := replsetCommand.String("name", "replset", "name of the set")

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
		psCommand.Parse(os.Args[2:])
	case "kill":
		killCommand.Parse(os.Args[2:])
	case "rm":
		rmCommand.Parse(os.Args[2:])
	case "standalone", "st":
		standaloneCommand.Parse(os.Args[2:])
	case "replset", "rs":
		replsetCommand.Parse(os.Args[2:])
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}

	// ps
	if psCommand.Parsed() {
		Util_ps()
	}

	if killCommand.Parsed() {
		Util_kill()
	}

	if rmCommand.Parsed() {
		Util_rm()
	}

	// Standalone
	if standaloneCommand.Parsed() {
		Run_standalone(*standalonePortPtr, *standaloneAuthPtr)
	}

	// Replica set
	if replsetCommand.Parsed() {
		var rsNum int = *replsetNumPtr
		var rsCfg string = *replsetConfigPtr

		re_config, _ := regexp.Compile("(?i)PS*A*")
		re_num, _ := regexp.Compile("[0-9]+")

		if len(os.Args) > 2 {
			cmd := os.Args[2]
			switch {
			case re_config.MatchString(cmd):
				rsNum = len(cmd)
				rsCfg = cmd
			case re_num.MatchString(cmd):
				rsNum, _ = strconv.Atoi(cmd)
				rsCfg = "P" + strings.Repeat("S", rsNum-1)
			default:
				replsetCommand.PrintDefaults()
				os.Exit(1)
			}
		}

		Run_replset(rsNum, *replsetPortPtr, rsCfg, *replsetNamePtr, *replsetAuthPtr)
	}

}
