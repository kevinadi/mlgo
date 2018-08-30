package main

import (
	"fmt"
	"strings"
)

func create_dbpath(num int, port int) string {
	var cmdline string
	Util_create_dbpath()
	for i := port; i < port+num; i++ {
		cmdline += fmt.Sprintf("mkdir data/%d\n", i)
	}
	return cmdline
}

func init_replset(num int, port int, replsetname string) string {
	var conf string
	var members []string
	for i := 0; i < num; i++ {
		switch {
		case i == 0:
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', priority:2}", i, port+i))
		case i > 0 && i < 7:
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d'}", i, port+i))
		case i >= 7:
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', priority:0, votes:0}", i, port+i))
		}
	}

	conf += fmt.Sprintf("{_id:'%s', members:[%s]}", replsetname, strings.Join(members, ", "))
	conf = fmt.Sprintf("mongo --port %d --eval \"rs.initiate(%s)\"", port, conf)
	return conf + "\n"
}

func wait_for_primary(port int) string {
	var cmdline string
	cmdline = fmt.Sprintf("mongo --port %d --quiet --eval \"db.isMaster()\"", port)
	cmdline = fmt.Sprintf("until %s | grep '\"ismaster\".*:.*true'; do sleep 2; echo waiting for primary...; done", cmdline)
	return cmdline + "\n"
}

func Run_replset(num int, port int, config string, replsetname string, auth bool) {
	var cmdline string

	cmdline += create_dbpath(num, port)

	for i := 0; i < num; i++ {
		cmdline += fmt.Sprintf("mongod --dbpath data/%d --port %d ", port+i, port+i)
		cmdline += fmt.Sprintf("--logpath data/%d/mongod.log --fork ", port+i)
		cmdline += fmt.Sprintf("--replSet %s ", replsetname)

		if auth {
			cmdline += "--auth "
		}

		cmdline += "\n"
	}

	cmdline += init_replset(num, port, replsetname)
	cmdline += wait_for_primary(port)

	fmt.Print(cmdline)
}
