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
	Script      bool
	Noinit      bool
}

func (rs *ReplSet) Init(num int, port int, config string, replsetname string, auth bool, noinit bool, script bool) {
	rs.Num = num
	rs.ReplSetName = replsetname
	rs.Port = port
	rs.Auth = auth
	rs.Config = config
	rs.Script = script
	rs.Noinit = noinit
	rs.parse_config()

	for i := 0; i < rs.Num; i++ {
		m_i := new(Mongod)
		m_i.Init(port+i, auth, replsetname, false)
		rs.Mongod = append(rs.Mongod, m_i)
	}

	rs.Cmdlines = rs.Cmd_mongod()
	if !rs.Noinit {
		rs.Cmdlines = append(rs.Cmdlines, rs.Cmd_init())
	}

	var hosts []string
	for i := 0; i < num; i++ {
		hosts = append(hosts, fmt.Sprintf("localhost:%d", port+i))
	}
	rs.Connstr = replsetname + "/" + strings.Join(hosts, ",")
}

func (rs *ReplSet) parse_config() {
	re_config := regexp.MustCompile("(?i)^PS*A*$")
	switch {
	case rs.Config == "" || rs.Num != 3:
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
	for !primary && i < 20 {
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
		fmt.Println("Primary not found after 40 seconds")
		os.Exit(1)
	}
}

func (rs *ReplSet) Create_keyfile() {
	Util_runcommand_string([]string{"mkdir", fmt.Sprintf("%s", Datadir)})

	outfile, err := os.Create(fmt.Sprintf("%s/keyfile.txt", Datadir))
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

func (rs *ReplSet) Deploy() {
	fmt.Println("# Port:", rs.Port)
	fmt.Println("# Num nodes:", rs.Num)
	fmt.Println("# Configuration:", rs.Config)
	fmt.Println("# Auth:", rs.Auth)
	fmt.Println("# Connection string:", rs.Connstr)
	fmt.Println()

	if rs.Script {
		fmt.Println(Util_cmd_script(rs.Cmdlines))
	} else {
		if rs.Auth {
			rs.Create_keyfile()
		}
		Util_runcommand_string_string(rs.Cmdlines)
		if !rs.Noinit {
			rs.Wait_for_primary()
		}
		if rs.Auth {
			rs.Cmd_adduser()
		}
		Util_create_start_script(rs.Cmdlines)
		fmt.Println(" -- ", "connstr :", rs.Connstr)
	}
}
