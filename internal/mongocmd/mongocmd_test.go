package mongocmd

import (
	"strings"
	"testing"

	"github.com/kevinadi/mlgo/internal/topology"
)

func TestMongodRoleAndAuth(t *testing.T) {
	data := Mongod(Node{Port: 27017, DBPath: "/db", LogPath: "/db/mongod.log", ReplSet: "configuration", Cache: "1"})
	joined := strings.Join(data, " ")
	if strings.Contains(joined, "--shardsvr") || strings.Contains(joined, "--configsvr") || strings.Contains(joined, "--auth") {
		t.Fatalf("data node argv = %s", joined)
	}

	shard := strings.Join(Mongod(Node{Port: 27018, ReplSet: "s", Role: topology.RoleShard, KeyFile: "/k", Cache: "1"}), " ")
	if !strings.Contains(shard, "--shardsvr") || !strings.Contains(shard, "--keyFile /k") || strings.Contains(shard, "--auth") {
		t.Fatalf("shard argv = %s", shard)
	}

	cfg := strings.Join(Mongod(Node{Port: 27019, ReplSet: "config", Role: topology.RoleConfig, Cache: "0.5"}), " ")
	if !strings.Contains(cfg, "--configsvr") || !strings.Contains(cfg, "--wiredTigerCacheSizeGB 0.5") {
		t.Fatalf("config argv = %s", cfg)
	}

	standalone := strings.Join(Mongod(Node{Port: 27017, Auth: true, Cache: "1"}), " ")
	if !strings.Contains(standalone, "--auth") || strings.Contains(standalone, "--keyFile") || strings.Contains(standalone, "--replSet") {
		t.Fatalf("standalone argv = %s", standalone)
	}

	mongos := strings.Join(Mongos(27017, "config/localhost:27019", "/log", ""), " ")
	if strings.Contains(mongos, "wiredTiger") || strings.Contains(mongos, "--keyFile") {
		t.Fatalf("mongos argv = %s", mongos)
	}
}

func TestInitiateEval(t *testing.T) {
	members, err := topology.MembersFromConfig("PH", 27017)
	if err != nil {
		t.Fatal(err)
	}
	eval, err := InitiateEval("replset", members)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(eval, `"hidden":true`) || !strings.Contains(eval, `"votes":0`) || strings.Contains(eval, "'") {
		t.Fatalf("eval = %s", eval)
	}
}

func TestHelloEvalChecksForAFunction(t *testing.T) {
	if strings.Contains(HelloEval, "(db.hello||db.isMaster)") {
		t.Fatalf("a legacy collection named hello is truthy: %s", HelloEval)
	}
	if !strings.Contains(HelloEval, `typeof db.hello=="function"`) || !strings.Contains(HelloEval, "db.isMaster") {
		t.Fatalf("eval = %s", HelloEval)
	}
}
