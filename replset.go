package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func RS_create_keyfile() {
	Util_runcommand_string([]string{"mkdir", "data"})

	outfile, err := os.Create("data/keyfile.txt")
	defer outfile.Close()
	Check(err)

	_, err = outfile.WriteString(Util_randstring(40))
	Check(err)

	if runtime.GOOS != "windows" {
		err = outfile.Chmod(0600)
		Check(err)
	}
}

func RS_create_dbpath(num int, port int) [][]string {
	var cmdline [][]string
	for i := port; i < port+num; i++ {
		cmdline = append(cmdline, ST_mkdir_cmd(i))
	}
	return cmdline
}

func RS_init_replset(config string, port int, replsetname string) []string {
	var conf string
	var members []string
	var initcmd []string

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
	eval := fmt.Sprintf("rs.initiate(%s)", conf)
	initcmd = append(initcmd, []string{"mongo", "--port", strconv.Itoa(port), "--eval", eval}...)

	return initcmd
}

func RS_wait_for_primary(port int) {
	var cmdline []string
	cmdline = append(cmdline, []string{
		"mongo",
		"--port", strconv.Itoa(port),
		"--quiet",
		"--eval", "db.isMaster()",
	}...)

	fmt.Println("Waiting for primary...")
	primary := false
	i := 0
	for !primary && i < 10 {
		com := exec.Command(cmdline[0], cmdline[1:]...)
		out, err := com.CombinedOutput()
		if err != nil {
			log.Fatalf("isMaster failed with %s\n", err)
		}
		primary = strings.Contains(string(out), "\"ismaster\" : true")
		time.Sleep(2 * time.Second)
		i += 1
	}

	if !primary {
		fmt.Println("Primary not found after 20 seconds")
		os.Exit(1)
	}
}

func RS_call_mongod(num int, port int, config string, replsetname string, auth bool) [][]string {
	var cmdlines [][]string

	for i := 0; i < num; i++ {
		mongod_cmd := ST_mongod_cmd(port+i, auth)
		mongod_cmd = append(mongod_cmd, []string{"--replSet", replsetname}...)
		if auth {
			mongod_cmd = append(mongod_cmd, []string{"--keyFile", "data/keyfile.txt"}...)
		}
		if strings.HasPrefix(replsetname, "shard") {
			mongod_cmd = append(mongod_cmd, []string{"--shardsvr"}...)
		}
		if strings.HasPrefix(replsetname, "config") {
			mongod_cmd = append(mongod_cmd, []string{"--configsvr"}...)
		}
		cmdlines = append(cmdlines, mongod_cmd)
	}

	return cmdlines
}

func RS_commandlines(num int, port int, config string, replsetname string, auth bool) ([][]string, string) {
	var cmdlines [][]string
	var hosts []string

	cmdlines = append(cmdlines, RS_create_dbpath(num, port)...)
	cmdlines = append(cmdlines, RS_call_mongod(num, port, config, replsetname, auth)...)
	cmdlines = append(cmdlines, RS_init_replset(config, port, replsetname))

	for i := 0; i < num; i++ {
		hosts = append(hosts, fmt.Sprintf("localhost:%d", port+i))
	}
	connstr := replsetname + "/" + strings.Join(hosts, ",")

	return cmdlines, connstr
}

func RS_deploy_replset(num int, port int, config string, replsetname string, auth bool, script bool) {
	fmt.Printf("# Auth: %t\n", auth)
	fmt.Printf("# Replica set nodes: %d\n", num)
	fmt.Printf("# Nodes configuration: %s\n\n", config)

	cmdlines, connstr := RS_commandlines(num, port, config, replsetname, auth)

	if script {
		fmt.Println(Util_cmd_script(cmdlines))
	} else {
		if auth {
			RS_create_keyfile()
		}
		Util_runcommand_string_string(cmdlines)
		RS_wait_for_primary(port)
		if auth {
			Util_runcommand_string(ST_mongo_adduser(port))
		}
		Util_create_start_script(cmdlines)
		fmt.Println(" -- ", "connstr :", connstr)
	}
}
