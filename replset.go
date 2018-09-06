package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ReplSet struct {
	Mongod      []*Mongod
	ReplSetName string
	Port        int
	Auth        bool
	Config      string
	Num         int
	Connstr     string
	Cmdlines    [][]string
}

func (rs *ReplSet) Init(num int, port int, config string, replsetname string, auth bool) {
	rs.Num = num
	rs.ReplSetName = replsetname
	rs.Port = port
	rs.Auth = auth
	rs.Config = config
	//rs.parse_config()

	for i := 0; i < rs.Num; i++ {
		m_i := new(Mongod)
		m_i.Init(port+i, auth, replsetname)
		rs.Mongod = append(rs.Mongod, m_i)
	}

	rs.Cmdlines = rs.Cmd_mongod()
	rs.Cmdlines = append(rs.Cmdlines, rs.Cmd_init())

	var hosts []string
	for i := 0; i < num; i++ {
		hosts = append(hosts, fmt.Sprintf("localhost:%d", port+i))
	}
	rs.Connstr = replsetname + "/" + strings.Join(hosts, ",")
}

func (rs *ReplSet) parse_config() {
	re_config := regexp.MustCompile("(?i)^PS*A*$")
	switch {
	case rs.Num != 3:
		rs.Config = "P" + strings.Repeat("S", rs.Num-1)
	case rs.Config != "PSS" && re_config.MatchString(rs.Config):
		rs.Num = len(rs.Config)
	case !re_config.MatchString(rs.Config):
		fmt.Println("Invalid replica set configuration:", rs.Config)
		os.Exit(1)
	}
}

func (rs *ReplSet) Cmd_mongod() [][]string {
	var cmdline [][]string
	for i := 0; i < rs.Num; i++ {
		cmdline = append(cmdline, rs.Mongod[i].Cmd_mkdir())
		cmdline = append(cmdline, rs.Mongod[i].Cmd_mongod())
	}
	return cmdline
}

func (rs *ReplSet) Cmd_init() []string {
	var conf string
	var members []string
	var initcmd []string

	for i, m := range rs.Config {
		switch strings.ToUpper(string(m)) {
		case "P":
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', priority:2}", i, rs.Port+i))
		case "S":
			switch {
			case i > 0 && i < 7:
				members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d'}", i, rs.Port+i))
			case i >= 7:
				members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', priority:0, votes:0}", i, rs.Port+i))
			}
		case "A":
			members = append(members, fmt.Sprintf("{_id:%d, host:'localhost:%d', arbiterOnly:1}", i, rs.Port+i))
		}
	}

	conf += fmt.Sprintf("{_id:'%s', members:[%s]}", rs.ReplSetName, strings.Join(members, ", "))
	eval := fmt.Sprintf("rs.initiate(%s)", conf)
	initcmd = append(initcmd, []string{"mongo", "--port", strconv.Itoa(rs.Port), "--eval", eval}...)

	return initcmd
}

func (rs *ReplSet) Wait_for_primary() {
	var cmdline []string
	cmdline = append(cmdline, []string{
		"mongo",
		"--port", strconv.Itoa(rs.Port),
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

func (rs *ReplSet) Create_keyfile() {
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

func (rs *ReplSet) Cmd_adduser() {
	Util_runcommand_string(rs.Mongod[0].Cmd_adduser())
}

func RS_deploy_replset(num int, port int, config string, replsetname string, auth bool, script bool) {
	fmt.Printf("# Auth: %t\n", auth)
	fmt.Printf("# Replica set nodes: %d\n", num)
	fmt.Printf("# Nodes configuration: %s\n\n", config)

	rs := new(ReplSet)
	rs.Init(num, port, config, replsetname, auth)

	if script {
		fmt.Println(Util_cmd_script(rs.Cmdlines))
	} else {
		if auth {
			rs.Create_keyfile()
		}
		Util_runcommand_string_string(rs.Cmdlines)
		rs.Wait_for_primary()
		if auth {
			rs.Cmd_adduser()
		}
		Util_create_start_script(rs.Cmdlines)
		fmt.Println(" -- ", "connstr :", rs.Connstr)
	}
}
