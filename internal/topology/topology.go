// Package topology plans local MongoDB deployments.
//
// Planning is pure: the same inputs always produce the same ports, member
// documents, and server roles. Replica-set role strings are interpreted
// once, for both a single replica set and each shard.
package topology

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// DataDir is the directory under the working directory that holds every
	// node started by mlgo.
	DataDir = "mlgo-data"

	// MaxMembers is the MongoDB replica-set member limit.
	MaxMembers = 50

	// MaxVoting is the MongoDB voting-member limit. Members past this index
	// are recorded with votes: 0, and a configuration that still has more
	// than MaxVoting voters is rejected.
	MaxVoting = 7
)

// Role is a replica-set member role encoded by the configuration string.
type Role int

const (
	RolePrimary Role = iota
	RoleSecondary
	RoleArbiter
	RoleHidden
)

// ServerRole is the mongod process role. It is chosen by the planner.
// It is not inferred from the replica-set name.
type ServerRole string

const (
	RoleData   ServerRole = ""
	RoleShard  ServerRole = "shardsvr"
	RoleConfig ServerRole = "configsvr"
)

// Member is one replica-set member. Nil Priority or Votes means the field
// is left at the server default when the set is initiated.
type Member struct {
	ID       int
	Port     int
	Host     string
	Role     Role
	Priority *int
	Votes    *int
	Arbiter  bool
	Hidden   bool
}

// Doc is the rs.initiate member document for this member.
func (m Member) Doc() map[string]any {
	doc := map[string]any{
		"_id":  m.ID,
		"host": m.Host,
	}
	if m.Priority != nil {
		doc["priority"] = *m.Priority
	}
	if m.Votes != nil {
		doc["votes"] = *m.Votes
	}
	if m.Arbiter {
		doc["arbiterOnly"] = true
	}
	if m.Hidden {
		doc["hidden"] = true
	}
	return doc
}

// ReplicaPlan is one mongod replica set.
type ReplicaPlan struct {
	Name    string
	Config  string
	Port    int
	Members []Member
	Role    ServerRole
}

// Cluster is a sharded deployment: one mongos, one or more shards, and a
// config server replica set.
type Cluster struct {
	MongosPort int
	Shards     []ReplicaPlan
	Config     ReplicaPlan
}

// P followed by any number of secondaries, then any number of arbiters or
// hidden members.
var rolePattern = regexp.MustCompile(`^PS*[AH]*$`)

// ValidatePort reports whether port can be bound by mongod or mongos.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d is outside 1-65535", port)
	}
	return nil
}

// ValidateSetName reports whether name can be used in a replica-set
// connection string.
func ValidateSetName(name string) error {
	if name == "" {
		return fmt.Errorf("replica set name is empty")
	}
	if strings.ContainsAny(name, "/ \t,") {
		return fmt.Errorf("replica set name %q contains a reserved character", name)
	}
	return nil
}

// ResolveRoles turns the CLI's -cfg and -num into one configuration string.
//
// When both were set, they must agree. When only -cfg was set, the number
// of nodes is the length of the configuration. When -cfg was not set, the
// configuration is a primary plus secondaries.
func ResolveRoles(cfg string, num int, cfgSet, numSet bool) (string, error) {
	cfg = strings.ToUpper(strings.TrimSpace(cfg))
	if cfgSet && cfg == "" {
		return "", fmt.Errorf("replica set configuration is empty")
	}
	if cfgSet && numSet && len(cfg) != num {
		return "", fmt.Errorf("configuration %q has %d members but the member count is %d", cfg, len(cfg), num)
	}
	if cfgSet {
		if err := validateShape(cfg); err != nil {
			return "", err
		}
		return cfg, nil
	}
	if num < 1 {
		return "", fmt.Errorf("replica set must have at least 1 member")
	}
	if num > MaxMembers {
		return "", fmt.Errorf("replica set cannot have more than %d members", MaxMembers)
	}
	return "P" + strings.Repeat("S", num-1), nil
}

func validateShape(cfg string) error {
	if !rolePattern.MatchString(cfg) {
		return fmt.Errorf("invalid replica set configuration %q (want P, then S, then optional A or H)", cfg)
	}
	return nil
}

// MembersFromConfig expands a resolved configuration into member documents
// listening on consecutive ports starting at basePort.
func MembersFromConfig(cfg string, basePort int) ([]Member, error) {
	cfg = strings.ToUpper(strings.TrimSpace(cfg))
	if err := validateShape(cfg); err != nil {
		return nil, err
	}
	if len(cfg) > MaxMembers {
		return nil, fmt.Errorf("replica set cannot have more than %d members", MaxMembers)
	}
	if err := ValidatePort(basePort); err != nil {
		return nil, err
	}

	members := make([]Member, 0, len(cfg))
	votes := 0
	for i, c := range cfg {
		port := basePort + i
		if err := ValidatePort(port); err != nil {
			return nil, fmt.Errorf("member %d: %w", i, err)
		}
		m := Member{
			ID:   i,
			Port: port,
			Host: fmt.Sprintf("localhost:%d", port),
		}
		switch c {
		case 'P':
			m.Role = RolePrimary
			p := 2
			m.Priority = &p
			votes++
		case 'S':
			m.Role = RoleSecondary
			if i >= MaxVoting {
				p := 0
				v := 0
				m.Priority = &p
				m.Votes = &v
			} else {
				votes++
			}
		case 'A':
			m.Role = RoleArbiter
			m.Arbiter = true
			votes++
		case 'H':
			m.Role = RoleHidden
			m.Hidden = true
			p := 0
			v := 0
			m.Priority = &p
			m.Votes = &v
		default:
			return nil, fmt.Errorf("invalid replica set configuration %q", cfg)
		}
		members = append(members, m)
	}
	if votes > MaxVoting {
		return nil, fmt.Errorf("configuration %q has %d voting members; MongoDB allows at most %d", cfg, votes, MaxVoting)
	}
	return members, nil
}

// ConnString is the standard replica-set connection string.
func ConnString(name string, members []Member) string {
	hosts := make([]string, len(members))
	for i, m := range members {
		hosts[i] = m.Host
	}
	joined := strings.Join(hosts, ",")
	if name == "" {
		return joined
	}
	return name + "/" + joined
}

// PlanReplica plans one replica set.
func PlanReplica(name string, port int, cfg string, num int, cfgSet, numSet bool) (ReplicaPlan, error) {
	if err := ValidateSetName(name); err != nil {
		return ReplicaPlan{}, err
	}
	resolved, err := ResolveRoles(cfg, num, cfgSet, numSet)
	if err != nil {
		return ReplicaPlan{}, err
	}
	members, err := MembersFromConfig(resolved, port)
	if err != nil {
		return ReplicaPlan{}, err
	}
	return ReplicaPlan{
		Name:    name,
		Config:  resolved,
		Port:    port,
		Members: members,
		Role:    RoleData,
	}, nil
}

// PlanSharded plans a sharded cluster. Mongos listens on mongosPort. Shard
// and config mongod processes take the following ports, in order.
func PlanSharded(mongosPort, numShards, nodesPerShard, numConfig int, shardCfg string, cfgSet, numSet bool) (Cluster, error) {
	if numShards < 1 {
		return Cluster{}, fmt.Errorf("cluster must have at least 1 shard")
	}
	if err := ValidatePort(mongosPort); err != nil {
		return Cluster{}, err
	}
	resolved, err := ResolveRoles(shardCfg, nodesPerShard, cfgSet, numSet)
	if err != nil {
		return Cluster{}, fmt.Errorf("shard: %w", err)
	}
	configCfg, err := ResolveRoles("", numConfig, false, true)
	if err != nil {
		return Cluster{}, fmt.Errorf("config server: %w", err)
	}

	next := mongosPort + 1
	shards := make([]ReplicaPlan, 0, numShards)
	for i := 0; i < numShards; i++ {
		name := fmt.Sprintf("shard%02d", i)
		members, err := MembersFromConfig(resolved, next)
		if err != nil {
			return Cluster{}, fmt.Errorf("%s: %w", name, err)
		}
		shards = append(shards, ReplicaPlan{
			Name:    name,
			Config:  resolved,
			Port:    next,
			Members: members,
			Role:    RoleShard,
		})
		next += len(members)
	}
	configMembers, err := MembersFromConfig(configCfg, next)
	if err != nil {
		return Cluster{}, fmt.Errorf("config server: %w", err)
	}
	return Cluster{
		MongosPort: mongosPort,
		Shards:     shards,
		Config: ReplicaPlan{
			Name:    "config",
			Config:  configCfg,
			Port:    next,
			Members: configMembers,
			Role:    RoleConfig,
		},
	}, nil
}
