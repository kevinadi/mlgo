package main

import (
	"fmt"
	"runtime"
)

func ST_mongod_dbpath(port int) []string {
	return []string{"--dbpath", fmt.Sprintf("data/%d", port)}
}

func ST_mongod_port(port int) []string {
	return []string{"--port", fmt.Sprintf("%d", port)}
}

func ST_mongod_logpath(port int) []string {
	return []string{"--logpath", fmt.Sprintf("data/%d/mongod.log", port)}
}

func ST_mongod_fork() []string {
	return []string{"--fork"}
}

func ST_mongod_start() []string {
	return []string{"start", "/b"}
}

func ST_mongo_adduser(port int) []string {
	return []string{
		"mongo", "admin",
		"--port", fmt.Sprintf("%d", port),
		"--eval", "db.createUser({user: 'user', pwd: 'password', roles: ['root']})",
	}
}

func ST_deploy_standalone(port int, auth bool) [][]string {
	var cmdlines [][]string
	var mongo_call []string

	mkdir_cmd := []string{"mkdir", "-p", fmt.Sprintf("data/%d", port)}

	mongo_call = append(mongo_call, "mongod")
	mongo_call = append(mongo_call, ST_mongod_dbpath(port)...)
	mongo_call = append(mongo_call, ST_mongod_port(port)...)
	mongo_call = append(mongo_call, ST_mongod_logpath(port)...)
	if auth {
		mongo_call = append(mongo_call, "--auth")
	}

	if runtime.GOOS != "windows" {
		mongo_call = append(mongo_call, ST_mongod_fork()...)
	} else {
		mongo_call = append(ST_mongod_start(), mongo_call...)
	}

	cmdlines = append(cmdlines, mkdir_cmd)
	cmdlines = append(cmdlines, mongo_call)

	if auth {
		cmdlines = append(cmdlines, ST_mongo_adduser(port))
	}

	return cmdlines
}
