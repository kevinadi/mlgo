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

func ST_mongod_cmd(port int, auth bool) []string {
	var mongod_call []string

	mongod_call = append(mongod_call, "mongod")
	mongod_call = append(mongod_call, ST_mongod_dbpath(port)...)
	mongod_call = append(mongod_call, ST_mongod_port(port)...)
	mongod_call = append(mongod_call, ST_mongod_logpath(port)...)

	if auth {
		mongod_call = append(mongod_call, "--auth")
	}

	switch runtime.GOOS {
	case "windows":
		mongod_call = append(ST_mongod_start(), mongod_call...)
	default:
		mongod_call = append(mongod_call, ST_mongod_fork()...)
	}

	return mongod_call
}

func ST_mkdir_cmd(port int) []string {
	var mkdir_cmd []string

	switch runtime.GOOS {
	case "windows":
		mkdir_cmd = []string{"mkdir", fmt.Sprintf("data\\%d", port)}
	default:
		mkdir_cmd = []string{"mkdir", "-p", fmt.Sprintf("data/%d", port)}
	}

	return mkdir_cmd
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

	cmdlines = append(cmdlines, ST_mkdir_cmd(port))
	cmdlines = append(cmdlines, ST_mongod_cmd(port, auth))
	if auth {
		cmdlines = append(cmdlines, ST_mongo_adduser(port))
	}

	return cmdlines
}
