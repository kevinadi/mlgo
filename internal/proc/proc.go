// Package proc lists, starts, and stops local mongod and mongos processes.
package proc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kevinadi/mlgo/internal/daemon"
	"github.com/kevinadi/mlgo/internal/mongocmd"
	"github.com/kevinadi/mlgo/internal/topology"
	"github.com/shirou/gopsutil/v4/process"
)

// Process is one running mongod or mongos.
type Process struct {
	PID     int
	Name    string
	Argv    []string
	Cmdline string
	DBPath  string
	LogPath string
	Port    string
	ReplSet string
	WorkDir string
}

// Spec selects processes.
//
// An empty Spec with CWD set matches servers whose dbpath or logpath is
// under that directory's mlgo-data. All matches every mongod and mongos.
// Criteria is a literal substring of the command line, or an exact pid,
// port, or replica-set name.
type Spec struct {
	All      bool
	CWD      string
	Criteria string
}

// List returns running mongod and mongos processes.
func List() ([]Process, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}
	var out []Process
	for _, proc := range procs {
		name, err := proc.Name()
		if err != nil || !isServer(name) {
			continue
		}
		argv, err := proc.CmdlineSlice()
		raw, rawErr := proc.Cmdline()
		if (err != nil || len(argv) == 0) && rawErr == nil {
			argv, _ = Split(raw)
		}
		p := ParseArgs(int(proc.Pid), argv)
		p.Name = name
		if rawErr == nil && raw != "" {
			p.Cmdline = raw
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out, nil
}

func isServer(name string) bool {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".exe")
	return base == "mongod" || base == "mongos"
}

// ParseArgs reads the fields mlgo cares about from a mongod or mongos argv.
func ParseArgs(pid int, argv []string) Process {
	p := Process{PID: pid, Argv: append([]string{}, argv...), Cmdline: strings.Join(argv, " ")}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		val := ""
		hasVal := false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				val = arg[eq+1:]
				arg = arg[:eq]
				hasVal = true
			} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
				val = argv[i]
				hasVal = true
			}
		}
		if !hasVal {
			continue
		}
		switch arg {
		case "--dbpath":
			p.DBPath = val
		case "--logpath":
			p.LogPath = val
		case "--port":
			p.Port = val
		case "--replSet":
			p.ReplSet = val
		}
	}
	p.WorkDir = WorkDir(p.DBPath)
	if p.WorkDir == "" {
		p.WorkDir = WorkDir(p.LogPath)
	}
	return p
}

// WorkDir is the directory that contains the mlgo-data path, if path is one.
func WorkDir(path string) string {
	if path == "" {
		return ""
	}
	dir := filepath.Clean(path)
	for {
		if filepath.Base(dir) == topology.DataDir {
			parent := filepath.Dir(dir)
			if parent == dir {
				return ""
			}
			return parent
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Filter applies spec to procs.
func Filter(procs []Process, spec Spec) []Process {
	var out []Process
	for _, p := range procs {
		switch {
		case spec.All:
			out = append(out, p)
		case spec.Criteria != "":
			if matchesCriteria(p, spec.Criteria) {
				out = append(out, p)
			}
		default:
			if inWorkspace(p, spec.CWD) {
				out = append(out, p)
			}
		}
	}
	return out
}

func matchesCriteria(p Process, criteria string) bool {
	return criteria == strconv.Itoa(p.PID) ||
		criteria == p.Port ||
		criteria == p.ReplSet ||
		strings.Contains(p.Cmdline, criteria)
}

func inWorkspace(p Process, cwd string) bool {
	if cwd == "" {
		return false
	}
	if p.WorkDir != "" && samePath(p.WorkDir, cwd) {
		return true
	}
	return underData(cwd, p.DBPath) || underData(cwd, p.LogPath)
}

func underData(cwd, path string) bool {
	if path == "" {
		return false
	}
	return hasPrefixPath(path, filepath.Join(cwd, topology.DataDir))
}

// samePath reports whether a and b are the same directory after resolving
// symlinks. macOS and Windows also compare case-insensitively. A lexical
// compare treats /tmp and /private/tmp as different directories, so kill
// and rm miss a server that was started through the other path.
func samePath(a, b string) bool {
	a, b = resolvePath(a), resolvePath(b)
	if a == b {
		return true
	}
	return foldPathCase() && strings.EqualFold(a, b)
}

func hasPrefixPath(path, root string) bool {
	path, root = resolvePath(path), resolvePath(root)
	if path == root {
		return true
	}
	sep := string(filepath.Separator)
	if strings.HasPrefix(path, root+sep) {
		return true
	}
	return foldPathCase() && strings.HasPrefix(strings.ToLower(path), strings.ToLower(root)+sep)
}

func foldPathCase() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

// resolvePath cleans path and evaluates symlinks. When the leaf does not
// exist, the longest existing ancestor is resolved and the rest is rejoined.
func resolvePath(path string) string {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	var suffix []string
	dir := path
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		suffix = append(suffix, filepath.Base(dir))
		resolved, err := filepath.EvalSymlinks(parent)
		if err != nil {
			dir = parent
			continue
		}
		for i := len(suffix) - 1; i >= 0; i-- {
			resolved = filepath.Join(resolved, suffix[i])
		}
		return filepath.Clean(resolved)
	}
}

// Format groups processes by working directory in a stable order.
func Format(procs []Process, cwd string) string {
	if len(procs) == 0 {
		return "No running processes\n"
	}
	groups := map[string][]Process{}
	var keys []string
	for _, p := range procs {
		if _, ok := groups[p.WorkDir]; !ok {
			keys = append(keys, p.WorkDir)
		}
		groups[p.WorkDir] = append(groups[p.WorkDir], p)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, wd := range keys {
		label := wd
		if label == "" {
			label = "(unknown directory)"
		} else if samePath(label, cwd) {
			label += " (current directory)"
		}
		fmt.Fprintln(&b, label)
		ps := groups[wd]
		sort.Slice(ps, func(i, j int) bool { return ps[i].PID < ps[j].PID })
		for _, p := range ps {
			fmt.Fprintf(&b, "%d %s\n", p.PID, p.Cmdline)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// Kill signals every process selected by spec.
func Kill(spec Spec) error {
	procs, err := List()
	if err != nil {
		return err
	}
	matched := Filter(procs, spec)
	if len(matched) == 0 {
		if spec.Criteria == "" && !spec.All {
			return fmt.Errorf("no processes running under current directory")
		}
		return fmt.Errorf("no matching mongod or mongos processes")
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].PID < matched[j].PID })
	for _, p := range matched {
		fmt.Printf("%d %s\n", p.PID, p.Cmdline)
	}
	fmt.Print("kill")
	for _, p := range matched {
		fmt.Printf(" %d", p.PID)
	}
	fmt.Println()
	var failed []error
	for _, p := range matched {
		if err := signal(p.PID); err != nil {
			failed = append(failed, fmt.Errorf("kill %d: %w", p.PID, err))
		}
	}
	if len(failed) > 0 {
		return failed[0]
	}
	return nil
}

// startBackground launches a server line that has no --fork. Tests replace it.
var startBackground = daemon.StartAndWait

// startDetached launches a Windows server without cmd.exe. Tests replace it.
var startDetached = daemon.StartDetached

// forkRejected reports that this machine's mongod must not be passed --fork.
// Tests replace it. An unreadable version on darwin is rejected too.
var forkRejected = func() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	text, err := mongocmd.VersionText(ctx)
	if err != nil {
		return true
	}
	return !mongocmd.ForkSupported(runtime.GOOS, text)
}

// Start runs commands from mlgo-data/start.sh. criteria is a literal
// substring. Matching is not a shell pipeline.
func Start(criteria string) error {
	path := filepath.Join(topology.DataDir, "start.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var cmds [][]string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if criteria != "" && !strings.Contains(line, criteria) {
			continue
		}
		argv, err := Split(line)
		if err != nil {
			return err
		}
		if len(argv) == 0 {
			continue
		}
		cmds = append(cmds, argv)
	}
	if len(cmds) == 0 {
		return fmt.Errorf("no matching commands in %s", path)
	}
	if criteria == "" {
		fmt.Println("Starting all processes ...")
	} else {
		fmt.Println("Starting", criteria, "...")
	}
	rejectFork := forkRejected()
	if rejectFork {
		fmt.Fprintln(os.Stderr, "this mongod rejects --fork on macOS; waiting until each server is listening")
	}
	for _, argv := range cmds {
		if rejectFork {
			argv = stripFork(argv)
		}
		fmt.Println(">>>", strings.Join(argv, " "))
		var err error
		if shouldDetach(argv) {
			err = detach(argv)
		} else {
			err = run(argv)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
		}
	}
	return nil
}

func stripFork(argv []string) []string {
	out := make([]string, 0, len(argv))
	for _, arg := range argv {
		if arg == "--fork" || arg == "--fork=true" {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func hasForkFlag(argv []string) bool {
	for _, arg := range argv {
		if arg == "--fork" || arg == "--fork=true" {
			return true
		}
	}
	return false
}

func shouldDetach(argv []string) bool {
	if runtime.GOOS == "windows" || len(argv) == 0 || !isServer(argv[0]) {
		return false
	}
	return !hasForkFlag(argv)
}

func detach(argv []string) error {
	parsed := ParseArgs(0, argv)
	port, err := strconv.Atoi(parsed.Port)
	if err != nil {
		return fmt.Errorf("no --port in command")
	}
	_, err = startBackground(argv, port, daemon.WaitTimeout)
	return err
}

func run(argv []string) error {
	if runtime.GOOS == "windows" {
		detach, direct := launchDirect(argv)
		if detach {
			_, err := startDetached(direct)
			return err
		}
		argv = direct
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// launchDirect strips a legacy `start /b` prefix. A server is started
// with CreateProcess so cmd.exe cannot split the path on & or %.
func launchDirect(argv []string) (detach bool, out []string) {
	out = stripStartBuiltin(argv)
	if len(out) > 0 && isServer(out[0]) {
		return true, out
	}
	return false, out
}

func stripStartBuiltin(argv []string) []string {
	if len(argv) >= 3 && argv[0] == "start" && strings.EqualFold(argv[1], "/b") {
		return append([]string{}, argv[2:]...)
	}
	return argv
}

// RemoveData deletes mlgo-data. It refuses when a server is still using it.
func RemoveData() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	procs, err := List()
	if err != nil {
		return err
	}
	if n := len(Filter(procs, Spec{CWD: cwd})); n > 0 {
		return fmt.Errorf("refusing to remove %s while %d mongod/mongos processes are still running; stop them first", topology.DataDir, n)
	}
	if _, err := os.Stat(topology.DataDir); os.IsNotExist(err) {
		fmt.Println("no data directory")
		return nil
	} else if err != nil {
		return err
	}
	fmt.Println("removing", topology.DataDir)
	return os.RemoveAll(topology.DataDir)
}

// Split splits a shell command line. It understands single quotes, double
// quotes, and backslash escapes, which is what mlgo writes into start.sh.
func Split(line string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quoting byte
	inWord := false
	flush := func() {
		args = append(args, b.String())
		b.Reset()
		inWord = false
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quoting != 0 {
			if c == quoting {
				quoting = 0
				continue
			}
			if quoting == '"' && c == '\\' && i+1 < len(line) {
				i++
				b.WriteByte(line[i])
				continue
			}
			b.WriteByte(c)
			continue
		}
		switch c {
		case ' ', '\t':
			if inWord {
				flush()
			}
		case '\'', '"':
			quoting = c
			inWord = true
		case '\\':
			if i+1 < len(line) {
				i++
				b.WriteByte(line[i])
				inWord = true
			}
		default:
			b.WriteByte(c)
			inWord = true
		}
	}
	if quoting != 0 {
		return nil, fmt.Errorf("unterminated quote in %q", line)
	}
	if inWord {
		flush()
	}
	return args, nil
}
