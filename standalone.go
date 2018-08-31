package main

import (
	"fmt"
)

func ST_deploy_standalone(port int, auth bool) (string, string) {
	var cmdline string
	var mongod_call string

	cmdline = Util_create_dbpath() + "\n"
	cmdline += fmt.Sprintf("mkdir data/%d\n", port)

	mongod_call = fmt.Sprintf("mongod --dbpath data/%d --port %d ", port, port)
	mongod_call += fmt.Sprintf("--logpath data/%d/mongod.log --fork ", port)

	if auth {
		mongod_call += "--auth\n"
	}

	cmdline += mongod_call + "\n"

	if auth {
		cmdline += Util_create_first_user(port)
	}

	// fmt.Print(cmdline)
	// return mongod_call
	return cmdline, mongod_call
}
