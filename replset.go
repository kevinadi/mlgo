package main

import (
	"fmt"
)

func Run_replset(num int, port int, auth bool) {
	var cmdline string

	Util_create_dbpath()

	cmdline = fmt.Sprintf("mongod --dbpath data --port %d ", port)
	cmdline += fmt.Sprintf("--logpath data/%d/mongod.log --fork ", port)

	if auth {
		cmdline += "--auth "
	}

	fmt.Println(cmdline)
}
