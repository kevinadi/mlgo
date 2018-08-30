package main

import (
	"fmt"
	"strings"
)

func RS_create_dbpath(num int, port int) string {
	var cmdline string
	cmdline = Util_create_dbpath()
	for i := port; i < port+num; i++ {
		cmdline += fmt.Sprintf("mkdir data/%d\n", i)
	}
	return cmdline
}

func RS_init_replset(config string, port int, replsetname string) string {
	var conf string
	var members []string

	for i, m := range config {
		switch strings.ToUpper(string(m)) {
		case "P":
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', priority:2}", i, port+i))
		case "S":
			switch {
			case i > 0 && i < 7:
				members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d'}", i, port+i))
			case i >= 7:
				members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', priority:0, votes:0}", i, port+i))
			}
		case "A":
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', arbiterOnly:1}", i, port+i))
		}
	}

	conf += fmt.Sprintf("{_id:'%s', members:[%s]}", replsetname, strings.Join(members, ", "))
	conf = fmt.Sprintf("mongo --port %d --eval \"rs.initiate(%s)\"", port, conf)
	return conf + "\n"
}

func RS_wait_for_primary(port int) string {
	var cmdline string
	cmdline = fmt.Sprintf("mongo --port %d --quiet --eval \"db.isMaster()\"", port)
	cmdline = fmt.Sprintf("until %s | grep '\"ismaster\".*:.*true'; do sleep 2; echo waiting for primary...; done", cmdline)
	return cmdline + "\n"
}

func RS_call_mongod(num int, port int, config string, replsetname string, auth bool) string {
	cmdline := ""
	for i := 0; i < num; i++ {
		cmdline += fmt.Sprintf("mongod --dbpath data/%d --port %d ", port+i, port+i)
		cmdline += fmt.Sprintf("--logpath data/%d/mongod.log --fork ", port+i)
		cmdline += fmt.Sprintf("--replSet %s ", replsetname)

		if auth {
			cmdline += "--keyFile data/keyfile.txt "
		}

		if strings.HasPrefix(replsetname, "shard") {
			cmdline += "--shardsvr"
		}

		if strings.HasPrefix(replsetname, "config") {
			cmdline += "--configsvr"
		}

		cmdline += "\n"
	}
	return cmdline
}

func RS_deploy_replset(num int, port int, config string, replsetname string, auth bool) {
	var cmdline string

	cmdline += RS_create_dbpath(num, port)

	if auth {
		cmdline += Util_create_keyfile()
	}

	cmdline += RS_call_mongod(num, port, config, replsetname, auth)

	cmdline += RS_init_replset(config, port, replsetname)
	cmdline += RS_wait_for_primary(port)

	if auth {
		cmdline += Util_create_first_user(port)
	}

	fmt.Print(cmdline)
}
