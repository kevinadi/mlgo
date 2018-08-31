package main

import (
	"fmt"
	"strings"
)

func RS_create_dbpath(num int, port int) string {
	var cmdline []string
	cmdline = append(cmdline, Util_create_dbpath())
	for i := port; i < port+num; i++ {
		cmdline = append(cmdline, fmt.Sprintf("mkdir data/%d", i))
	}
	return strings.Join(cmdline, "\n")
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
	return conf
}

func RS_wait_for_primary(port int) string {
	var cmdline string
	cmdline = fmt.Sprintf("mongo --port %d --quiet --eval \"db.isMaster()\"", port)
	cmdline = fmt.Sprintf("until %s | grep '\"ismaster\".*:.*true'; do sleep 2; echo waiting for primary...; done", cmdline)
	return cmdline
}

func RS_call_mongod(num int, port int, config string, replsetname string, auth bool) string {
	var cmdlines []string
	mongod_call := ""
	for i := 0; i < num; i++ {
		mongod_call = fmt.Sprintf("mongod --dbpath data/%d --port %d ", port+i, port+i)
		mongod_call += fmt.Sprintf("--logpath data/%d/mongod.log --fork ", port+i)
		mongod_call += fmt.Sprintf("--replSet %s ", replsetname)

		if auth {
			mongod_call += "--keyFile data/keyfile.txt "
		}

		if strings.HasPrefix(replsetname, "shard") {
			mongod_call += "--shardsvr"
		}

		if strings.HasPrefix(replsetname, "config") {
			mongod_call += "--configsvr"
		}

		cmdlines = append(cmdlines, mongod_call)
	}
	return strings.Join(cmdlines, "\n")
}

func RS_deploy_replset(num int, port int, config string, replsetname string, auth bool) (string, string) {
	var cmdline []string
	var mongodcalls []string

	cmdline = append(cmdline, RS_create_dbpath(num, port))

	if auth {
		cmdline = append(cmdline, Util_create_keyfile())
	}

	mongodcalls = append(mongodcalls, RS_call_mongod(num, port, config, replsetname, auth))
	cmdline = append(cmdline, mongodcalls[len(mongodcalls)-1])

	cmdline = append(cmdline, RS_init_replset(config, port, replsetname))
	cmdline = append(cmdline, RS_wait_for_primary(port))

	if auth {
		cmdline = append(cmdline, Util_create_first_user(port))
	}

	//fmt.Println(strings.Join(cmdline, "\n"))
	return strings.Join(cmdline, "\n"), strings.Join(mongodcalls, "\n")
}
