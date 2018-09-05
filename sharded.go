package main

// func SH_shard_address(num int, port int, replset string) string {
// 	var serverlist []string
// 	for i := 0; i < num; i++ {
// 		serverlist = append(serverlist, fmt.Sprintf("localhost:%d", port+i))
// 	}
// 	return fmt.Sprintf("%s/%s", replset, strings.Join(serverlist, ","))
// }

// func SH_deploy_shardsvr(numshards int, shardsvr int, shardcfg string, port int, auth bool) (string, []string, string) {
// 	var shardservers []string
// 	var mongod_calls []string
// 	var cmdline string = ""
// 	var cmd string = ""
// 	var shardname string
// 	var shardport int

// 	for shard := 0; shard < numshards; shard++ {
// 		shardname = fmt.Sprintf("shard%02d", shard)
// 		shardport = port + (shard * shardsvr)
// 		cmd = RS_create_dbpath(shardsvr, shardport) + "\n"
// 		mongod_calls = append(mongod_calls, RS_call_mongod(shardsvr, shardport, shardcfg, shardname, auth))
// 		cmd += mongod_calls[len(mongod_calls)-1]
// 		shardservers = append(shardservers, SH_shard_address(shardsvr, shardport, shardname))
// 		cmdline += cmd + "\n"
// 		cmdline += RS_init_replset(shardcfg, shardport, shardname) + "\n"
// 		cmdline += RS_wait_for_primary(shardport) + "\n"
// 		if auth {
// 			cmdline += Util_create_first_user(shardport)
// 		}
// 	}

// 	return cmdline, shardservers, strings.Join(mongod_calls, "\n")
// }

// func SH_deploy_configsvr(num int, port int, auth bool) (string, string, string) {
// 	var cmdline string = ""
// 	var mongod_calls []string

// 	config := "P" + strings.Repeat("S", num-1)
// 	cmdline = RS_create_dbpath(num, port) + "\n"
// 	mongod_calls = append(mongod_calls, RS_call_mongod(num, port, config, "config", auth))
// 	cmdline += mongod_calls[len(mongod_calls)-1] + "\n"
// 	cmdline += RS_init_replset(config, port, "config") + "\n"
// 	cmdline += RS_wait_for_primary(port) + "\n"

// 	if auth {
// 		cmdline += Util_create_first_user(port)
// 	}

// 	return cmdline, SH_shard_address(num, port, "config"), strings.Join(mongod_calls, "\n")
// }

// func SH_deploy_mongos(configsvr string, shardsvr []string, port int, auth bool) (string, string) {
// 	var cmdline string = ""
// 	var mongos_calls []string
// 	var addshard_cmd string = ""
// 	var addshard_calls []string

// 	cmdline = fmt.Sprintf("mongos --configdb %s ", configsvr)
// 	cmdline += fmt.Sprintf("--port %d ", port)
// 	cmdline += fmt.Sprintf("--logpath data/mongos.log --fork ")

// 	if auth {
// 		cmdline += "--keyFile data/keyfile.txt "
// 	}

// 	mongos_calls = append(mongos_calls, cmdline)
// 	cmdline += "\n"

// 	for _, shard := range shardsvr {
// 		addshard_cmd = fmt.Sprintf("mongo --eval \"sh.addShard('%s')\" ", shard)
// 		if auth {
// 			addshard_cmd += "-u user -p password --authenticationDatabase admin"
// 		}
// 		addshard_calls = append(addshard_calls, addshard_cmd)
// 	}
// 	cmdline += strings.Join(addshard_calls, "\n")

// 	return cmdline, strings.Join(mongos_calls, "\n")
// }
