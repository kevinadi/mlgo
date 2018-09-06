package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

type Sharded struct {
	ShardSvr    []*ReplSet
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

func (sh *Sharded) Init(port int, numshards int, shardnum int, shardcfg string, numconfig int, auth bool, script bool) {
	sh.NumShards = numshards
	sh.ShardNum = shardnum
	sh.ShardConfig = shardcfg
	sh.NumConfig = numconfig
	sh.Port = port
	sh.Auth = auth
	sh.Script = script
	if sh.ShardNum != 1 {
		sh.ShardConfig = "P" + strings.Repeat("S", sh.ShardNum-1)
	}
	if sh.ShardConfig != "P" {
		sh.ShardNum = len(sh.ShardConfig)
	}
	fmt.Println("Port:", sh.Port)
	fmt.Println("Num Shards:", sh.NumShards)
	fmt.Println("ShardSvr Num:", sh.ShardNum)
	fmt.Println("ShardSvr Cfg:", sh.ShardConfig)
	fmt.Println("ConfigSvr Num:", sh.NumConfig)
	fmt.Println("Auth:", sh.Auth)
	fmt.Println()

	for i := 0; i < sh.NumShards; i++ {
		sh_i := new(ReplSet)
		rsname := fmt.Sprintf("shard%02d", i)
		sh_i.Init(sh.ShardNum, sh.Port+1+(i*sh.ShardNum), sh.ShardConfig, rsname, sh.Auth)
		sh.ShardSvr = append(sh.ShardSvr, sh_i)
	}

	ConfigSvr := new(ReplSet)
	rsconfig := "P" + strings.Repeat("S", numconfig-1)
	ConfigSvr.Init(sh.NumConfig, sh.Port+1+(sh.NumShards*sh.ShardNum), rsconfig, "config", sh.Auth)
	sh.ConfigSvr = ConfigSvr

	for i := 0; i < sh.NumShards; i++ {
		sh.Cmdlines = append(sh.Cmdlines, sh.ShardSvr[i].Cmdlines...)
	}
	sh.Cmdlines = append(sh.Cmdlines, sh.ConfigSvr.Cmdlines...)
	sh.Cmdlines = append(sh.Cmdlines, sh.Cmd_mongos())

}

func (sh *Sharded) Cmd_mongos() []string {
	var cmdline []string
	cmdline = []string{
		"mongos",
		"--port", strconv.Itoa(sh.Port),
		"--configdb", sh.ConfigSvr.Connstr,
		"--logpath", "data/mongos.log",
	}
	if sh.Auth {
		cmdline = append(cmdline, "--keyFile", "data/keyfile.txt")
	}
	switch runtime.GOOS {
	case "windows":
		cmdline = append([]string{"start", "/b"}, cmdline...)
	default:
		cmdline = append(cmdline, []string{"--fork"}...)
	}
	return cmdline
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
		}
		Util_runcommand_string_string(sh.ConfigSvr.Cmdlines)
		Util_runcommand_string(sh.Cmd_mongos())
		Util_runcommand_string_string(sh.Cmd_addshards())
		if sh.Auth {
			Util_runcommand_string(sh.Cmd_adduser())
		}
		Util_create_start_script(sh.Cmdlines)
	}
}
