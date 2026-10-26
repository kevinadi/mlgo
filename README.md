# mlgo

`mlgo` starts a local MongoDB deployment for tests: one mongod, a replica set, or a sharded cluster. Data lives in `mlgo-data/` under the current directory.

The deployment is planned first. `mlgo rs --script` prints the same steps that `mlgo rs` runs.

## Install

Requires Go 1.27.

```
go install github.com/kevinadi/mlgo/cmd/mlgo@latest
```

From a checkout, `make` builds `./mlgo` and `make test` runs the tests. `mongod` must be on `PATH`. Initiating a set also needs `mongosh` (or the legacy `mongo` shell).

## Commands

```
standalone (st)           one mongod
replset (rs)              a replica set
sharded (sh)              a sharded cluster

start [criteria]          run matching lines from mlgo-data/start.sh
kill|stop [all|criteria]  signal mongod/mongos; default is this directory
rm                        remove mlgo-data when nothing from it is running
```

With no arguments, mlgo prints this help and the mongod and mongos processes it can see.

`criteria` is a literal substring of the command line, or an exact pid, port, or replica set name. It is not a regular expression.

Shared flags: `-port` (default 27017), `-auth`, `-script`.

### Replica set

```
mlgo rs                              # one node, set name "replset"
mlgo rs -num 3                       # primary and two secondaries
mlgo rs -cfg PSA                     # primary, secondary, arbiter
mlgo rs -cfg PSH                     # primary, secondary, hidden
mlgo rs -init=false                  # start the processes, do not call rs.initiate
```

`-cfg` is `P`, then any number of `S`, then any number of `A` or `H`. When `-cfg` and `-num` are both set, they must describe the same number of members. A configuration with more than 7 voting members is rejected. A secondary at the 8th position or later is non-voting (`votes: 0`, `priority: 0`). Arbiters always vote and count toward the 7-voter limit. Hidden members never vote.

### Sharded cluster

```
mlgo sh                              # mongos on 27017, two single-node shards, one config server
mlgo sh -shardcfg PSA                # each shard is a PSA set
mlgo sh -num 3 -shardsvr 3 -configsvr 3
```

`-shardcfg` uses the same role string as `-cfg`. Ports are assigned upward from `-port`: mongos, then each shard, then the config servers.

The planner assigns each process its role, and `--shardsvr` and `--configsvr` come from that role. A replica set started with `rs` never gets either flag, whatever its `-name`.

### Authentication

`-auth` creates `user` / `password` with the `root` role on `admin`.

A standalone mongod is started with `--auth`. The user is created through the localhost exception.

A replica set or sharded cluster gets one key file, `mlgo-data/keyfile.txt`, mode `0600`, filled from `crypto/rand`. Every member and mongos is started with `--keyFile`. The user is created after the set or cluster is up. `--script` prints an `openssl rand` command instead of embedding the secret.

### WiredTiger cache

mlgo passes `--wiredTigerCacheSizeGB` based on `mongod --version`:

| mongod | value |
| --- | --- |
| 3.4 and newer | `0.5` |
| 3.0, 3.1, 3.2, 3.3 | `1` |
| older, or version not detected | flag omitted; mongod keeps its own default |

Fractional cache sizes are accepted starting in MongoDB 3.4 (minimum 0.25 GB). 0.5 GB keeps a multi-node test deployment small without going under that minimum.

### Shell

`rs.initiate`, `sh.addShard`, and `createUser` run through `mongosh` when it is on `PATH`, otherwise `mongo`. Readiness is `db.hello().isWritablePrimary` when that command exists, otherwise `isMaster`. Initiate and `addShard` are retried. A failed `mongod` is not retried.

## Data directory

Each node uses `mlgo-data/<port>/`. `mlgo start` reruns the `mongod` and `mongos` lines saved in `mlgo-data/start.sh`. `mlgo rm` deletes `mlgo-data` only when no server from this directory is still running.
