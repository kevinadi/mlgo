package main

import (
	"fmt"
	"strings"
)

func SH_shard_address(num int, port int, replset string) string {
	var serverlist []string
	for i := 0; i < num; i++ {
		serverlist = append(serverlist, fmt.Sprintf("localhost:%d", port+i))
	}
	return fmt.Sprintf("%s/%s", replset, strings.Join(serverlist, ","))
}

func SH_deploy_shardsvr(numshards int, shardsvr int, shardcfg string, port int, auth bool) []string {
	var shardservers []string
	var cmdline string = ""
	var cmd string = ""
	var shardname string
	var shardport int

	for shard := 0; shard < numshards; shard++ {
		shardname = fmt.Sprintf("shard%02d", shard)
		shardport = port + (shard * shardsvr)
		cmd = RS_create_dbpath(shardsvr, shardport)
		cmd += RS_call_mongod(shardsvr, shardport, shardcfg, shardname, auth)
		shardservers = append(shardservers, SH_shard_address(shardsvr, shardport, shardname))
		cmdline += cmd
		cmdline += RS_init_replset(shardcfg, shardport, shardname)
		cmdline += RS_wait_for_primary(shardport)
	}

	fmt.Print(cmdline)
	return shardservers
}

func SH_deploy_configsvr(num int, port int, auth bool) string {
	var cmdline string = ""

	config := "P" + strings.Repeat("S", num-1)
	cmdline = RS_create_dbpath(num, port)
	cmdline += RS_call_mongod(num, port, config, "config", auth)
	cmdline += RS_init_replset(config, port, "config")
	cmdline += RS_wait_for_primary(port)

	fmt.Print(cmdline)
	return SH_shard_address(num, port, "config")
}

func SH_deploy_mongos(configsvr string, shardsvr []string, port int, auth bool) {
	var cmdline string = ""

	cmdline = fmt.Sprintf("mongos --configdb %s ", configsvr)
	cmdline += fmt.Sprintf("--port %d ", port)
	cmdline += fmt.Sprintf("--logpath data/mongos.log --fork ")

	if auth {
		cmdline += "--keyFile data/keyfile.txt "
	}

	cmdline += "\n"

	for _, shard := range shardsvr {
		cmdline += fmt.Sprintf("mongo --eval \"sh.addShard('%s')\"\n", shard)
	}

	fmt.Println(cmdline)
}
