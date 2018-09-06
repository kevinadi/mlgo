package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

type Mongod struct {
	Dbpath   string
	Port     int
	Logpath  string
	ReplSet  string
	Auth     bool
	KeyFile  string
	Cmdlines [][]string
}

func (m *Mongod) Init(port int, auth bool, replset string) {
	m.Dbpath = fmt.Sprintf("data/%d", port)
	m.Port = port
	m.Auth = auth
	if replset != "" {
		m.ReplSet = replset
	}
	if replset != "" && auth {
		m.KeyFile = "data/keyfile.txt"
	}
	m.Logpath = fmt.Sprintf("data/%d/mongod.log", port)

	m.Cmdlines = append(m.Cmdlines, m.Cmd_mkdir())
	m.Cmdlines = append(m.Cmdlines, m.Cmd_mongod())
	if auth {
		m.Cmdlines = append(m.Cmdlines, m.Cmd_adduser())
	}
}

func (m *Mongod) Cmd_mongod() []string {
	var mongod_call []string

	mongod_call = append(mongod_call, []string{
		"mongod",
		"--port", strconv.Itoa(m.Port),
		"--dbpath", m.Dbpath,
		"--logpath", m.Logpath,
	}...)
	if m.ReplSet == "" && m.Auth {
		mongod_call = append(mongod_call, "--auth")
	}
	if m.ReplSet != "" {
		mongod_call = append(mongod_call, []string{
			"--replSet", m.ReplSet,
		}...)
		if m.Auth {
			mongod_call = append(mongod_call, []string{
				"--keyFile", "data/keyfile.txt",
			}...)
		}
		if strings.HasPrefix(m.ReplSet, "shard") {
			mongod_call = append(mongod_call, "--shardsvr")
		}
		if strings.HasPrefix(m.ReplSet, "config") {
			mongod_call = append(mongod_call, "--configsvr")
		}
	}
	switch runtime.GOOS {
	case "windows":
		mongod_call = append([]string{"start", "/b"}, mongod_call...)
	default:
		mongod_call = append(mongod_call, []string{"--fork"}...)
	}

	return mongod_call
}

func (m *Mongod) Cmd_mkdir() []string {
	var cmdline []string
	switch runtime.GOOS {
	case "windows":
		cmdline = []string{"mkdir", fmt.Sprintf("data\\%d", m.Port)}
	default:
		cmdline = []string{"mkdir", "-p", fmt.Sprintf("data/%d", m.Port)}
	}
	return cmdline
}

func (m *Mongod) Cmd_adduser() []string {
	return []string{
		"mongo", "admin",
		"--port", fmt.Sprintf("%d", m.Port),
		"--eval", "db.createUser({user: 'user', pwd: 'password', roles: ['root']})",
	}
}

func ST_deploy_standalone(port int, auth bool, script bool) {
	m := new(Mongod)
	m.Init(port, auth, "")

	if script {
		fmt.Print(Util_cmd_script(m.Cmdlines))
	} else {
		Util_runcommand_string_string(m.Cmdlines)
		Util_create_start_script(m.Cmdlines)
	}
}
