package main

import (
	"fmt"
)

func ST_deploy_standalone(port int, auth bool) {
	var cmdline string

	cmdline = Util_create_dbpath()

	cmdline += fmt.Sprintf("mongod --dbpath data --port %d ", port)
	cmdline += "--logpath data/mongod.log --fork "

	if auth {
		cmdline += "--auth\n"
		cmdline += Util_create_first_user(port)
	}

	fmt.Print(cmdline)
}
