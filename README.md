# mlgo

Launch a MongoDB deployment for test purposes

## Usage

```
$ mlgo
Usage:
  standalone (st) -- run a standalone node
  replset (rs) -- run a replica set
  sharded (sh) -- run a sharded cluster

  ps [criteria] -- show running mongod/mongos
  start [criteria] -- start some mongod/mongos using the start.sh script
  kill [criteria] -- kill running mongod/mongos
  rm -- remove the data/ directory
```

### Standalone

```
$ mlgo st -h
Usage of standalone:
  -auth
    	use auth
  -port int
    	start on this port (default 27017)
  -script
    	print deployment script
```

### Replica set

```
$ mlgo rs -h
Usage of replset:
  -auth
    	use auth
  -cfg string
    	configuration of the set (default "PSS")
  -name string
    	name of the set (default "replset")
  -num int
    	run this many nodes (default 3)
  -port int
    	start on this port (default 27017)
  -script
    	print deployment script
```

#### Replica set examples

Launch a basic 3-node replica set:

```
$ mlgo rs
# Auth: false
# Replica set nodes: 3
# Nodes configuration: PSS
```

Launch a replica set with a specific node configuration:

```
$ mlgo rs -cfg PSA
# Auth: false
# Replica set nodes: 3
# Nodes configuration: PSA
```

Launch a replica set with a number of nodes:

```
$ mlgo rs -num 5
# Auth: false
# Replica set nodes: 5
# Nodes configuration: PSSSS
```

### Sharded cluster

```
$ mlgo sh -h
Usage of sharded:
  -auth
    	use auth
  -configsvr int
    	run this many config servers (default 1)
  -num int
    	run this many shards (default 2)
  -port int
    	start on this port (default 27017)
  -script
    	print deployment script
  -shardcfg string
    	configuration of the shard replica set (default "P")
  -shardsvr int
    	run this many nodes per shard (default 1)
```

#### Sharded cluster examples:

Launch a basic 2 shard, 1 node per shard, 1 config server:

```
$ mlgo sh
# Auth: false
# mongos port: 27017
# Number of shards: 2
# ShardSvr replica set num: 1
# ShardSvr configuration: P
# Config servers: 1
```

Launch a 3 shard, 3 nodes per shard, 3 config servers:

```
$ mlgo sh -num 3 -shardsvr 3 -configsvr 3
# Auth: false
# mongos port: 27017
# Number of shards: 3
# ShardSvr replica set num: 3
# ShardSvr configuration: PSS
# Config servers: 3
```

Launch a 2 shard, PSA configuration per shard, 1 config server:

```
$ mlgo sh -shardcfg PSA
# Auth: false
# mongos port: 27017
# Number of shards: 2
# ShardSvr replica set num: 3
# ShardSvr configuration: PSA
# Config servers: 1
```

### Starting and killing

`mlgo` can start and kill processes given by the command line criteria. For example, to kill and restart all `mongod` having `shard00` in its command line:

```
$ mlgo sh -shardcfg PSS
# Auth: false
# mongos port: 27017
# Number of shards: 2
# ShardSvr replica set num: 3
# ShardSvr configuration: PSS
# Config servers: 1

...

$  mlgo ps
57384 mongod --dbpath data/27018 --port 27018 --logpath data/27018/mongod.log --fork --replSet shard00 --shardsvr
57387 mongod --dbpath data/27019 --port 27019 --logpath data/27019/mongod.log --fork --replSet shard00 --shardsvr
57390 mongod --dbpath data/27020 --port 27020 --logpath data/27020/mongod.log --fork --replSet shard00 --shardsvr
57421 mongod --dbpath data/27021 --port 27021 --logpath data/27021/mongod.log --fork --replSet shard01 --shardsvr
57424 mongod --dbpath data/27022 --port 27022 --logpath data/27022/mongod.log --fork --replSet shard01 --shardsvr
57427 mongod --dbpath data/27023 --port 27023 --logpath data/27023/mongod.log --fork --replSet shard01 --shardsvr
57453 mongod --dbpath data/27024 --port 27024 --logpath data/27024/mongod.log --fork --replSet config --configsvr
57465 mongos --configdb config/localhost:27024 --port 27017 --logpath data/mongos.log --fork

$ mlgo kill shard00
57384 mongod --dbpath data/27018 --port 27018 --logpath data/27018/mongod.log --fork --replSet shard00 --shardsvr
57387 mongod --dbpath data/27019 --port 27019 --logpath data/27019/mongod.log --fork --replSet shard00 --shardsvr
57390 mongod --dbpath data/27020 --port 27020 --logpath data/27020/mongod.log --fork --replSet shard00 --shardsvr
kill 57384 57387 57390

$ mlgo start shard00
Starting shard00 ...

$ mlgo ps
57421 mongod --dbpath data/27021 --port 27021 --logpath data/27021/mongod.log --fork --replSet shard01 --shardsvr
57424 mongod --dbpath data/27022 --port 27022 --logpath data/27022/mongod.log --fork --replSet shard01 --shardsvr
57427 mongod --dbpath data/27023 --port 27023 --logpath data/27023/mongod.log --fork --replSet shard01 --shardsvr
57453 mongod --dbpath data/27024 --port 27024 --logpath data/27024/mongod.log --fork --replSet config --configsvr
57465 mongos --configdb config/localhost:27024 --port 27017 --logpath data/mongos.log --fork
57496 mongod --dbpath data/27018 --port 27018 --logpath data/27018/mongod.log --fork --replSet shard00 --shardsvr
57499 mongod --dbpath data/27019 --port 27019 --logpath data/27019/mongod.log --fork --replSet shard00 --shardsvr
57502 mongod --dbpath data/27020 --port 27020 --logpath data/27020/mongod.log --fork --replSet shard00 --shardsvr
```
