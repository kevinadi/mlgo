package topology

import "testing"

func TestResolveRoles(t *testing.T) {
	tests := []struct {
		name    string
		cfg     string
		num     int
		cfgSet  bool
		numSet  bool
		want    string
		wantErr bool
	}{
		{name: "default one node", num: 1, want: "P"},
		{name: "num only", num: 3, numSet: true, want: "PSS"},
		{name: "cfg only", cfg: "psa", cfgSet: true, num: 1, want: "PSA"},
		{name: "cfg and num agree", cfg: "PSA", num: 3, cfgSet: true, numSet: true, want: "PSA"},
		{name: "hidden", cfg: "PSH", cfgSet: true, want: "PSH"},
		{name: "disagree", cfg: "PSA", num: 2, cfgSet: true, numSet: true, wantErr: true},
		{name: "pipe is not a role", cfg: "P|", cfgSet: true, wantErr: true},
		{name: "secondary first", cfg: "S", cfgSet: true, wantErr: true},
		{name: "empty cfg", cfg: "  ", cfgSet: true, wantErr: true},
		{name: "zero nodes", num: 0, numSet: true, wantErr: true},
		{name: "too many", num: MaxMembers + 1, numSet: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveRoles(tt.cfg, tt.num, tt.cfgSet, tt.numSet)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMembersFromConfig(t *testing.T) {
	members, err := MembersFromConfig("PSAH", 27017)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 4 {
		t.Fatalf("len = %d", len(members))
	}
	if members[0].Role != RolePrimary || *members[0].Priority != 2 || members[0].Votes != nil {
		t.Fatalf("primary = %+v", members[0])
	}
	if members[1].Role != RoleSecondary || members[1].Priority != nil || members[1].Votes != nil {
		t.Fatalf("secondary = %+v", members[1])
	}
	if !members[2].Arbiter || members[2].Priority != nil {
		t.Fatalf("arbiter = %+v", members[2])
	}
	if !members[3].Hidden || *members[3].Priority != 0 || *members[3].Votes != 0 {
		t.Fatalf("hidden = %+v", members[3])
	}
	if members[3].Host != "localhost:27020" || members[3].Port != 27020 {
		t.Fatalf("host = %+v", members[3])
	}

	// Eight data members: indexes 0..6 vote, index 7 does not. Total stays at 7.
	eight := "P" + "SSSSSSS"
	members, err = MembersFromConfig(eight, 27017)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 8 {
		t.Fatalf("len = %d", len(members))
	}
	last := members[7]
	if last.Votes == nil || *last.Votes != 0 || last.Priority == nil || *last.Priority != 0 {
		t.Fatalf("8th member still votes: %+v", last)
	}

	// Primary, six voting secondaries, and an arbiter is 8 voters.
	if _, err := MembersFromConfig("PSSSSSSA", 27017); err == nil {
		t.Fatal("expected voting-member error")
	}

	if _, err := MembersFromConfig("P|", 27017); err == nil {
		t.Fatal("expected shape error")
	}
	if _, err := MembersFromConfig("P", 65535); err != nil {
		t.Fatal(err)
	}
	if _, err := MembersFromConfig("PS", 65535); err == nil {
		t.Fatal("expected port overflow")
	}
	if _, err := MembersFromConfig("P", 0); err == nil {
		t.Fatal("expected port error")
	}
}

func TestMemberDoc(t *testing.T) {
	members, err := MembersFromConfig("PH", 27017)
	if err != nil {
		t.Fatal(err)
	}
	hidden := members[1].Doc()
	if hidden["hidden"] != true || hidden["votes"] != 0 || hidden["priority"] != 0 {
		t.Fatalf("doc = %#v", hidden)
	}
	if _, ok := hidden["arbiterOnly"]; ok {
		t.Fatalf("hidden doc has arbiterOnly: %#v", hidden)
	}
}

func TestPlanReplica(t *testing.T) {
	plan, err := PlanReplica("replset", 27017, "", 1, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Config != "P" || plan.Role != RoleData || ConnString(plan.Name, plan.Members) != "replset/localhost:27017" {
		t.Fatalf("plan = %+v conn %s", plan, ConnString(plan.Name, plan.Members))
	}

	plan, err = PlanReplica("replset", 27017, "PSA", 1, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Config != "PSA" || len(plan.Members) != 3 {
		t.Fatalf("cfg-only plan = %+v", plan)
	}

	if _, err := PlanReplica("a/b", 27017, "", 1, false, false); err == nil {
		t.Fatal("expected name error")
	}
	if _, err := PlanReplica("replset", 27017, "PSA", 5, true, true); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestPlanSharded(t *testing.T) {
	cluster, err := PlanSharded(27017, 2, 1, 1, "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if cluster.Shards[0].Port != 27018 || cluster.Shards[1].Port != 27019 || cluster.Config.Port != 27020 {
		t.Fatalf("ports = %d %d %d", cluster.Shards[0].Port, cluster.Shards[1].Port, cluster.Config.Port)
	}
	if cluster.Shards[0].Role != RoleShard || cluster.Config.Role != RoleConfig {
		t.Fatalf("roles = %q %q", cluster.Shards[0].Role, cluster.Config.Role)
	}
	if cluster.Shards[0].Name != "shard00" || cluster.Config.Name != "config" {
		t.Fatalf("names = %s %s", cluster.Shards[0].Name, cluster.Config.Name)
	}

	// -shardcfg PSA with the default -shardsvr 1 follows the configuration.
	cluster, err = PlanSharded(27017, 2, 1, 1, "PSA", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if cluster.Shards[0].Config != "PSA" || len(cluster.Shards[0].Members) != 3 {
		t.Fatalf("PSA shard = %+v", cluster.Shards[0])
	}
	if cluster.Shards[1].Port != 27021 || cluster.Config.Port != 27024 {
		t.Fatalf("PSA ports shard1=%d config=%d", cluster.Shards[1].Port, cluster.Config.Port)
	}

	// Both flags set and equal is fine. Both set and different is an error.
	if _, err := PlanSharded(27017, 2, 3, 1, "PSA", true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanSharded(27017, 2, 2, 1, "PSA", true, true); err == nil {
		t.Fatal("expected shard cfg/num conflict")
	}

	// Hidden members are valid on a shard.
	if _, err := PlanSharded(27017, 1, 1, 1, "PSH", true, false); err != nil {
		t.Fatal(err)
	}

	if _, err := PlanSharded(27017, 0, 1, 1, "", false, false); err == nil {
		t.Fatal("expected shard count error")
	}
	if _, err := PlanSharded(27017, 1, 1, 0, "", false, false); err == nil {
		t.Fatal("expected config count error")
	}
	if _, err := PlanSharded(65535, 1, 1, 1, "", false, false); err == nil {
		t.Fatal("expected port overflow")
	}
}
