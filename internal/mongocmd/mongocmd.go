// Package mongocmd builds argv for mongod, mongos, and the MongoDB shell.
package mongocmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/kevinadi/mlgo/internal/topology"
)

// Node is one mongod process.
type Node struct {
	Port    int
	DBPath  string
	LogPath string
	ReplSet string
	Role    topology.ServerRole
	// Auth adds --auth. Replica-set members use KeyFile instead, because
	// a keyFile already enables authentication.
	Auth    bool
	KeyFile string
	// Cache is the --wiredTigerCacheSizeGB value. An empty Cache omits the flag.
	Cache string
}

// NodeDBPath is the dbPath for a node under cwd.
func NodeDBPath(cwd string, port int) string {
	return filepath.Join(cwd, topology.DataDir, strconv.Itoa(port))
}

// KeyFilePath is the shared keyFile for a deployment under cwd.
func KeyFilePath(cwd string) string {
	return filepath.Join(cwd, topology.DataDir, "keyfile.txt")
}

// Mongod returns the argv for a mongod process, without --fork.
func Mongod(n Node) []string {
	argv := []string{
		"mongod",
		"--port", strconv.Itoa(n.Port),
		"--dbpath", n.DBPath,
		"--logpath", n.LogPath,
	}
	if n.Cache != "" {
		argv = append(argv, "--wiredTigerCacheSizeGB", n.Cache)
	}
	if n.ReplSet == "" && n.Auth {
		argv = append(argv, "--auth")
	}
	if n.ReplSet != "" {
		argv = append(argv, "--replSet", n.ReplSet)
		if n.KeyFile != "" {
			argv = append(argv, "--keyFile", n.KeyFile)
		}
		switch n.Role {
		case topology.RoleShard:
			argv = append(argv, "--shardsvr")
		case topology.RoleConfig:
			argv = append(argv, "--configsvr")
		}
	}
	return argv
}

// Mongos returns the argv for a mongos process, without --fork.
func Mongos(port int, configDB, logPath, keyFile string) []string {
	argv := []string{
		"mongos",
		"--port", strconv.Itoa(port),
		"--configdb", configDB,
		"--logpath", logPath,
	}
	if keyFile != "" {
		argv = append(argv, "--keyFile", keyFile)
	}
	return argv
}

// Mkdir returns the argv that creates path.
func Mkdir(path string, windows bool) []string {
	if windows {
		return []string{"mkdir", path}
	}
	return []string{"mkdir", "-p", path}
}

// Shell returns the argv for one --eval against the legacy shell or mongosh.
func Shell(bin string, port int, eval string) []string {
	return []string{bin, "--port", strconv.Itoa(port), "--quiet", "--eval", eval}
}

const (
	// HelloEval prints true when this member or mongos can take writes.
	// typeof rejects the legacy shell, where a missing db.hello is a
	// collection object and is therefore truthy. A real helper is a function.
	HelloEval = `var cmd=(typeof db.hello=="function")?db.hello:db.isMaster; var h=cmd.call(db); print(!!(h.isWritablePrimary||h.ismaster));`

	// PingEval prints PingToken on its own line when mongod is accepting
	// commands. A legacy shell's connection error never prints that line.
	PingEval  = `print(db.runCommand({ping:1}).ok==1?"pong":"down");`
	PingToken = "pong"

	// CreateUserEval creates the fixed local test user on admin.
	CreateUserEval = `db.getSiblingDB("admin").createUser({user:"user",pwd:"password",roles:["root"]})`
)

// InitiateEval is the rs.initiate() call for these members.
func InitiateEval(name string, members []topology.Member) (string, error) {
	docs := make([]map[string]any, len(members))
	for i, member := range members {
		docs[i] = member.Doc()
	}
	raw, err := json.Marshal(map[string]any{
		"_id":     name,
		"members": docs,
	})
	if err != nil {
		return "", err
	}
	return "rs.initiate(" + string(raw) + ")", nil
}

// AddShardEval is the sh.addShard() call for one shard connection string.
func AddShardEval(conn string) string {
	return fmt.Sprintf("sh.addShard(%q)", conn)
}
