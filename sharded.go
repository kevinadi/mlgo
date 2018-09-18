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

type Sharded struct {
	ShardSvr    []*ReplSet
	Mongos      []*Mongos
	ConfigSvr   *ReplSet
	Port        int
	Auth        bool
	NumShards   int
	NumConfig   int
	ShardConfig string
	ShardNum    int
	Cmdlines    [][]string
	Script      bool
}

type Mongos struct {
	Auth     bool
	Port     int
	Config   string
	Cmdlines [][]string
}

func (m *Mongos) Init(port int, config string, auth bool) {
	m.Auth = auth
	m.Port = port
	m.Config = config

	var cmdline []string
	cmdline = []string{
		"mongos",
		"--port", strconv.Itoa(port),
		"--configdb", config,
		"--logpath", fmt.Sprintf("%s/mongos.log", Datadir),
	}
	if auth {
		cmdline = append(cmdline, "--keyFile", fmt.Sprintf("%s/keyfile.txt", Datadir))
	}
	switch runtime.GOOS {
	case "windows":
		cmdline = append([]string{"start", "/b"}, cmdline...)
	default:
		cmdline = append(cmdline, []string{"--fork"}...)
	}
	m.Cmdlines = append(m.Cmdlines, cmdline)
}

func (m *Mongos) Wait_for_primary() {
	var cmdline []string
	cmdline = append(cmdline, []string{
		"mongo",
		"--port", strconv.Itoa(m.Port),
		"--quiet",
		"--eval", "db.isMaster()",
	}...)

	fmt.Println("Waiting for mongos...")
	primary := false
	i := 0
	for !primary && i < 20 {
		com := exec.Command(cmdline[0], cmdline[1:]...)
		out, err := com.CombinedOutput()
		if err != nil {
			log.Println("isMaster on mongos failed with %s\n", err)
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

func (sh *Sharded) Init(port int, numshards int, shardnum int, shardcfg string, numconfig int, auth bool, script bool) {
	sh.NumShards = numshards
	sh.ShardNum = shardnum
	sh.ShardConfig = shardcfg
	sh.NumConfig = numconfig
	sh.Port = port
	sh.Auth = auth
	sh.Script = script
	sh.parse_config()

	fmt.Println("# Port:", sh.Port)
	fmt.Println("# Num Shards:", sh.NumShards)
	fmt.Println("# ShardSvr Num:", sh.ShardNum)
	fmt.Println("# ShardSvr Cfg:", sh.ShardConfig)
	fmt.Println("# ConfigSvr Num:", sh.NumConfig)
	fmt.Println("# Auth:", sh.Auth)
	fmt.Println()

	for i := 0; i < sh.NumShards; i++ {
		sh_i := new(ReplSet)
		rsname := fmt.Sprintf("shard%02d", i)
		sh_i.Init(sh.ShardNum, sh.Port+1+(i*sh.ShardNum), sh.ShardConfig, rsname, sh.Auth, sh.Script)
		sh.ShardSvr = append(sh.ShardSvr, sh_i)
	}

	ConfigSvr := new(ReplSet)
	rsconfig := "P" + strings.Repeat("S", numconfig-1)
	ConfigSvr.Init(sh.NumConfig, sh.Port+1+(sh.NumShards*sh.ShardNum), rsconfig, "config", sh.Auth, sh.Script)
	sh.ConfigSvr = ConfigSvr

	mongos := new(Mongos)
	mongos.Init(sh.Port, sh.ConfigSvr.Connstr, sh.Auth)
	sh.Mongos = append(sh.Mongos, mongos)

	for i := 0; i < sh.NumShards; i++ {
		sh.Cmdlines = append(sh.Cmdlines, sh.ShardSvr[i].Cmdlines...)
	}
	sh.Cmdlines = append(sh.Cmdlines, sh.ConfigSvr.Cmdlines...)
	sh.Cmdlines = append(sh.Cmdlines, mongos.Cmdlines...)

}

func (sh *Sharded) parse_config() {
	re_config := regexp.MustCompile("(?i)^PS*A*$")
	switch {
	case sh.ShardConfig == "" || sh.ShardNum != 1:
		sh.ShardConfig = "P" + strings.Repeat("S", sh.ShardNum-1)
	case sh.ShardConfig != "P" && re_config.MatchString(sh.ShardConfig):
		sh.ShardNum = len(sh.ShardConfig)
	case !re_config.MatchString(sh.ShardConfig):
		fmt.Println("Invalid replica set configuration:", sh.ShardConfig)
		os.Exit(1)
	}
}

func (sh *Sharded) Cmd_addshards() [][]string {
	var cmdline [][]string
	var cmd []string
	for _, s := range sh.ShardSvr {
		cmd = []string{
			"mongo",
			"--port", strconv.Itoa(sh.Port),
			"--eval", fmt.Sprintf("sh.addShard('%s')", s.Connstr),
		}
		cmdline = append(cmdline, cmd)
	}
	return cmdline
}

func (sh *Sharded) Cmd_adduser() []string {
	return []string{
		"mongo", "admin",
		"--port", fmt.Sprintf("%d", sh.Port),
		"--eval", "db.createUser({user: 'user', pwd: 'password', roles: ['root']})",
	}
}

func (sh *Sharded) Deploy() {
	if sh.Script {
		fmt.Println(Util_cmd_script(sh.Cmdlines))
	} else {
		sh.ShardSvr[0].Create_keyfile()
		for _, shard := range sh.ShardSvr {
			Util_runcommand_string_string(shard.Cmdlines)
			shard.Wait_for_primary()
		}
		Util_runcommand_string_string(sh.ConfigSvr.Cmdlines)
		sh.ConfigSvr.Wait_for_primary()
		Util_runcommand_string_string(sh.Mongos[0].Cmdlines)
		sh.Mongos[0].Wait_for_primary()
		Util_runcommand_string_string(sh.Cmd_addshards())
		if sh.Auth {
			Util_runcommand_string(sh.Cmd_adduser())
		}
		Util_create_start_script(sh.Cmdlines)
	}
}
