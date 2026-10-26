// Package deploy turns a topology plan into ordered steps and runs them.
//
// The same plan is executed and rendered as a shell script, so --script
// cannot drift from what the tool actually does.
package deploy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevinadi/mlgo/internal/mongocmd"
	"github.com/kevinadi/mlgo/internal/topology"
)

// Options controls how a deployment is built and run.
type Options struct {
	Auth    bool
	NoInit  bool
	Script  bool
	Cache   string
	Shell   string
	Windows bool
	CWD     string
}

// StepKind is one unit of work in a deployment.
type StepKind int

const (
	StepExec StepKind = iota
	StepKeyFile
	StepWaitPrimary
	StepWaitPing
)

// Step is one action. Argv is the real program arguments. --fork and the
// Windows start wrapper are added when the step runs or is rendered.
type Step struct {
	Kind       StepKind
	Argv       []string
	Background bool
	Retries    int
	Port       int
	Label      string
}

// Plan is a complete deployment.
type Plan struct {
	Summary []string
	Steps   []Step
	Options Options
	Conn    string
}

// Executor runs a command. Tests substitute a fake.
type Executor interface {
	Run(ctx context.Context, argv []string) error
	Output(ctx context.Context, argv []string) ([]byte, error)
}

// Local runs commands on this machine.
type Local struct{}

func (Local) Run(ctx context.Context, argv []string) error {
	fmt.Println(">>>", strings.Join(argv, " "))
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (Local) Output(ctx context.Context, argv []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	return cmd.CombinedOutput()
}

// FindShell returns "mongosh" or "mongo", preferring mongosh.
func FindShell() string {
	for _, name := range []string{"mongosh", "mongo"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// NeedsShell reports whether executing the plan requires a MongoDB shell.
func (p Plan) NeedsShell() bool {
	for _, step := range p.Steps {
		switch step.Kind {
		case StepWaitPrimary, StepWaitPing:
			return true
		case StepExec:
			if len(step.Argv) > 0 && step.Argv[0] == p.Options.Shell {
				return true
			}
		}
	}
	return false
}

func normalize(opt Options) Options {
	if opt.Shell == "" {
		opt.Shell = "mongosh"
	}
	return opt
}

func cacheSummary(cache string) string {
	if cache == "" {
		return "# WiredTiger cache: server default"
	}
	return "# WiredTiger cache: " + cache + " GB"
}

// BuildStandalone plans one mongod.
func BuildStandalone(port int, opt Options) (Plan, error) {
	if err := topology.ValidatePort(port); err != nil {
		return Plan{}, err
	}
	opt = normalize(opt)
	steps := appendMongod(nil, opt, port, "", topology.RoleData, "")
	if opt.Auth {
		steps = append(steps, Step{Kind: StepWaitPing, Port: port, Label: "mongod"})
		steps = append(steps, Step{Kind: StepExec, Argv: mongocmd.Shell(opt.Shell, port, mongocmd.CreateUserEval)})
	}
	conn := fmt.Sprintf("localhost:%d", port)
	return Plan{
		Summary: []string{
			fmt.Sprintf("# Port: %d", port),
			fmt.Sprintf("# Auth: %t", opt.Auth),
			fmt.Sprintf("# Connection string: %s", conn),
			cacheSummary(opt.Cache),
		},
		Steps:   steps,
		Options: opt,
		Conn:    conn,
	}, nil
}

// BuildReplica plans one replica set.
func BuildReplica(name string, port int, cfg string, num int, cfgSet, numSet bool, opt Options) (Plan, error) {
	spec, err := topology.PlanReplica(name, port, cfg, num, cfgSet, numSet)
	if err != nil {
		return Plan{}, err
	}
	opt = normalize(opt)
	var steps []Step
	key := ""
	if opt.Auth {
		key = mongocmd.KeyFilePath(opt.CWD)
		steps = append(steps, Step{Kind: StepKeyFile})
	}
	for _, member := range spec.Members {
		steps = appendMongod(steps, opt, member.Port, spec.Name, spec.Role, key)
	}
	if !opt.NoInit {
		steps, err = appendInitiate(steps, opt, spec.Port, spec.Name, spec.Members)
		if err != nil {
			return Plan{}, err
		}
		if opt.Auth {
			steps = append(steps, Step{Kind: StepExec, Argv: mongocmd.Shell(opt.Shell, spec.Port, mongocmd.CreateUserEval)})
		}
	}
	conn := topology.ConnString(spec.Name, spec.Members)
	summary := []string{
		fmt.Sprintf("# Port: %d", spec.Port),
		fmt.Sprintf("# Num nodes: %d", len(spec.Members)),
		fmt.Sprintf("# Configuration: %s", spec.Config),
		fmt.Sprintf("# Auth: %t", opt.Auth),
		fmt.Sprintf("# Connection string: %s", conn),
		cacheSummary(opt.Cache),
	}
	if opt.NoInit && opt.Auth {
		summary = append(summary, "# User not created because the replica set is not being initiated")
	}
	return Plan{Summary: summary, Steps: steps, Options: opt, Conn: conn}, nil
}

// BuildSharded plans a sharded cluster.
func BuildSharded(port, numShards, nodesPerShard, numConfig int, shardCfg string, cfgSet, numSet bool, opt Options) (Plan, error) {
	cluster, err := topology.PlanSharded(port, numShards, nodesPerShard, numConfig, shardCfg, cfgSet, numSet)
	if err != nil {
		return Plan{}, err
	}
	opt = normalize(opt)
	var steps []Step
	key := ""
	if opt.Auth {
		key = mongocmd.KeyFilePath(opt.CWD)
		steps = append(steps, Step{Kind: StepKeyFile})
	}
	for _, shard := range cluster.Shards {
		for _, member := range shard.Members {
			steps = appendMongod(steps, opt, member.Port, shard.Name, topology.RoleShard, key)
		}
		steps, err = appendInitiate(steps, opt, shard.Port, shard.Name, shard.Members)
		if err != nil {
			return Plan{}, err
		}
	}
	for _, member := range cluster.Config.Members {
		steps = appendMongod(steps, opt, member.Port, cluster.Config.Name, topology.RoleConfig, key)
	}
	steps, err = appendInitiate(steps, opt, cluster.Config.Port, cluster.Config.Name, cluster.Config.Members)
	if err != nil {
		return Plan{}, err
	}

	db := mongocmd.NodeDBPath(opt.CWD, cluster.MongosPort)
	configDB := topology.ConnString(cluster.Config.Name, cluster.Config.Members)
	steps = append(steps, Step{Kind: StepExec, Argv: mongocmd.Mkdir(db, opt.Windows)})
	steps = append(steps, Step{
		Kind:       StepExec,
		Background: true,
		Argv:       mongocmd.Mongos(cluster.MongosPort, configDB, filepath.Join(db, "mongos.log"), key),
	})
	steps = append(steps, Step{Kind: StepWaitPing, Port: cluster.MongosPort, Label: "mongos"})
	for _, shard := range cluster.Shards {
		conn := topology.ConnString(shard.Name, shard.Members)
		steps = append(steps, Step{
			Kind:    StepExec,
			Retries: 4,
			Argv:    mongocmd.Shell(opt.Shell, cluster.MongosPort, mongocmd.AddShardEval(conn)),
		})
	}
	if opt.Auth {
		steps = append(steps, Step{Kind: StepExec, Argv: mongocmd.Shell(opt.Shell, cluster.MongosPort, mongocmd.CreateUserEval)})
	}

	conn := fmt.Sprintf("localhost:%d", cluster.MongosPort)
	summary := []string{
		fmt.Sprintf("# Port: %d", cluster.MongosPort),
		fmt.Sprintf("# Num Shards: %d", len(cluster.Shards)),
		fmt.Sprintf("# ShardSvr Num: %d", len(cluster.Shards[0].Members)),
		fmt.Sprintf("# ShardSvr Cfg: %s", cluster.Shards[0].Config),
		fmt.Sprintf("# ConfigSvr Num: %d", len(cluster.Config.Members)),
		fmt.Sprintf("# Auth: %t", opt.Auth),
		fmt.Sprintf("# Connection string: %s", conn),
		cacheSummary(opt.Cache),
	}
	return Plan{Summary: summary, Steps: steps, Options: opt, Conn: conn}, nil
}

func appendMongod(steps []Step, opt Options, port int, replset string, role topology.ServerRole, key string) []Step {
	db := mongocmd.NodeDBPath(opt.CWD, port)
	steps = append(steps, Step{Kind: StepExec, Argv: mongocmd.Mkdir(db, opt.Windows)})
	steps = append(steps, Step{
		Kind:       StepExec,
		Background: true,
		Argv: mongocmd.Mongod(mongocmd.Node{
			Port:    port,
			DBPath:  db,
			LogPath: filepath.Join(db, "mongod.log"),
			ReplSet: replset,
			Role:    role,
			Auth:    opt.Auth && replset == "",
			KeyFile: key,
			Cache:   opt.Cache,
		}),
	})
	return steps
}

func appendInitiate(steps []Step, opt Options, port int, name string, members []topology.Member) ([]Step, error) {
	eval, err := mongocmd.InitiateEval(name, members)
	if err != nil {
		return nil, err
	}
	steps = append(steps,
		Step{Kind: StepExec, Argv: mongocmd.Shell(opt.Shell, port, eval), Retries: 4},
		Step{Kind: StepWaitPrimary, Port: port, Label: name},
	)
	return steps, nil
}

// Execute runs the plan and writes the start script.
func Execute(ctx context.Context, plan Plan, ex Executor, sleep func(time.Duration)) error {
	if ex == nil {
		ex = Local{}
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	for _, line := range plan.Summary {
		fmt.Println(line)
	}
	fmt.Println()
	for _, step := range plan.Steps {
		if err := runStep(ctx, plan, step, ex, sleep); err != nil {
			return err
		}
	}
	if err := WriteStartScript(plan); err != nil {
		return err
	}
	if plan.Conn != "" {
		fmt.Println(" -- ", "connstr :", plan.Conn)
	}
	return nil
}

func runStep(ctx context.Context, plan Plan, step Step, ex Executor, sleep func(time.Duration)) error {
	switch step.Kind {
	case StepKeyFile:
		return writeKeyFile(plan.Options.CWD)
	case StepWaitPrimary:
		return waitReady(ctx, ex, plan.Options.Shell, step.Port, mongocmd.HelloEval, "true", "primary "+step.Label, sleep)
	case StepWaitPing:
		return waitReady(ctx, ex, plan.Options.Shell, step.Port, mongocmd.PingEval, mongocmd.PingToken, step.Label, sleep)
	case StepExec:
		return runExec(ctx, plan, step, ex, sleep)
	default:
		return fmt.Errorf("unknown deployment step %d", step.Kind)
	}
}

func runExec(ctx context.Context, plan Plan, step Step, ex Executor, sleep func(time.Duration)) error {
	argv := ExecArgv(step, plan.Options.Windows)
	attempts := step.Retries + 1
	var err error
	for i := 0; i < attempts; i++ {
		err = ex.Run(ctx, argv)
		if err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		fmt.Println("Retrying command...")
		sleep(2 * time.Second)
	}
	return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
}

func waitReady(ctx context.Context, ex Executor, shell string, port int, eval, want, label string, sleep func(time.Duration)) error {
	fmt.Printf("Waiting for %s...\n", label)
	var last error
	for i := 0; i < 20; i++ {
		out, err := ex.Output(ctx, mongocmd.Shell(shell, port, eval))
		if err == nil && outputSays(out, want) {
			return nil
		}
		last = err
		if i < 19 {
			sleep(2 * time.Second)
		}
	}
	if last != nil {
		return fmt.Errorf("%s not ready on port %d after 40 seconds: %w", label, port, last)
	}
	return fmt.Errorf("%s not ready on port %d after 40 seconds", label, port)
}

func outputSays(out []byte, want string) bool {
	for _, field := range strings.Fields(string(out)) {
		if field == want {
			return true
		}
	}
	return false
}

func writeKeyFile(cwd string) error {
	dir := filepath.Join(cwd, topology.DataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	path := mongocmd.KeyFilePath(cwd)
	body := base64.StdEncoding.EncodeToString(buf) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
