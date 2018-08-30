package main

import (
	"fmt"
)

func Run_standalone(port int, auth bool) {
	var cmdline string

	Util_create_dbpath()

	cmdline = fmt.Sprintf("mongod --dbpath data --port %d ", port)
	cmdline += "--logpath data/mongod.log --fork "

	if auth {
		cmdline += "--auth "
	}

	fmt.Println(cmdline)
}
