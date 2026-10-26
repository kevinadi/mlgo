package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinadi/mlgo/internal/mongocmd"
	"github.com/kevinadi/mlgo/internal/shellquote"
	"github.com/kevinadi/mlgo/internal/topology"
)

// ExecArgv is the argv passed to the operating system for a step.
// A background step on Unix gets --fork unless Options.NoFork is set.
// Windows uses start /b either way. The NoFork path does not go through
// ExecArgv: the caller detaches the process itself.
func ExecArgv(step Step, opt Options) []string {
	argv := append([]string{}, step.Argv...)
	if step.Background {
		switch {
		case opt.Windows:
			argv = append([]string{"start", "/b"}, argv...)
		case !opt.NoFork:
			argv = append(argv, "--fork")
		}
	}
	if opt.Windows {
		argv = append([]string{"cmd", "/c"}, argv...)
	}
	return argv
}

// CommandArgv is the argv as it appears in start.sh. The script is already
// interpreted by a shell, so the Windows cmd /c wrapper is omitted. When
// NoFork is set the line is the plain server argv: `mlgo start` detaches it.
func CommandArgv(step Step, opt Options) []string {
	argv := append([]string{}, step.Argv...)
	if !step.Background {
		return argv
	}
	if opt.Windows {
		return append([]string{"start", "/b"}, argv...)
	}
	if opt.NoFork {
		return argv
	}
	return append(argv, "--fork")
}

func formatArgv(argv []string) string {
	parts := make([]string, len(argv))
	for i, arg := range argv {
		parts[i] = shellquote.Quote(arg)
	}
	return strings.Join(parts, " ")
}

// Render returns a shell script for the plan.
func (p Plan) Render() string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	for _, line := range p.Summary {
		fmt.Fprintf(&b, "%s\n", line)
	}
	fmt.Fprintln(&b)
	if p.detachInShell() {
		b.WriteString(detachHelpers)
	}
	for _, step := range p.Steps {
		switch step.Kind {
		case StepKeyFile:
			fmt.Fprintln(&b, keyFileScript(mongocmd.KeyFilePath(p.Options.CWD)))
		case StepWaitPrimary:
			fmt.Fprintln(&b, waitScript(p.Options.Shell, step.Port, mongocmd.HelloEval, "true", "primary "+step.Label))
		case StepWaitPing:
			fmt.Fprintln(&b, waitScript(p.Options.Shell, step.Port, mongocmd.PingEval, mongocmd.PingToken, step.Label))
		case StepExec:
			if step.Background && p.detachInShell() {
				fmt.Fprintln(&b, detachScript(step))
				continue
			}
			line := formatArgv(CommandArgv(step, p.Options))
			if step.Retries > 0 {
				fmt.Fprintln(&b, retryScript(line, step.Retries+1))
			} else {
				fmt.Fprintln(&b, line)
			}
		}
	}
	return b.String()
}

func (p Plan) detachInShell() bool {
	return p.Options.NoFork && !p.Options.Windows
}

// detachHelpers backgrounds a server in a new session and waits until that
// process is listening. A bare `nohup ... &` returns before mongod accepts
// connections, so the next step (rs.initiate, sh.addShard) would race it.
const detachHelpers = `start_detached() {
  # exec so $! is the server. A child perl would leave $! on the shell,
  # and the listen check would watch the wrong pid.
  if command -v perl >/dev/null 2>&1; then
    exec perl -MPOSIX -e 'POSIX::setsid(); exec @ARGV or die $!' "$@"
  elif command -v python3 >/dev/null 2>&1; then
    exec python3 -c 'import os,sys; os.setsid(); os.execvp(sys.argv[1], sys.argv[1:])' "$@"
  else
    exec nohup "$@"
  fi
}

port_ready() {
  port=$1
  pid=$2
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -a -p "$pid" -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
    return $?
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "$port" >/dev/null 2>&1
    return $?
  fi
  return 1
}

`

func detachScript(step Step) string {
	argv := append([]string{}, step.Argv...)
	line := formatArgv(argv)
	name := "server"
	if len(argv) > 0 {
		name = filepath.Base(argv[0])
	}
	log := flagValue(argv, "--logpath")
	redir := "</dev/null >/dev/null 2>&1"
	tail := ""
	if log != "" {
		// Keep early stderr off --logpath. mongod renames or truncates that
		// file when it opens it, which drops the lines we just appended.
		launch := shellquote.Quote(log + ".launch")
		quoted := shellquote.Quote(log)
		redir = "</dev/null >>" + launch + " 2>&1"
		tail = "\n    tail -n 40 " + launch + " >&2\n    tail -n 40 " + quoted + " >&2"
	}
	return fmt.Sprintf(`echo "Starting %s on port %d..."
start_detached %s %s &
pid=$!
ok=0
i=0
while [ "$i" -lt 120 ]; do
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "%s exited before listening on port %d" >&2%s
    exit 1
  fi
  if port_ready %d "$pid"; then
    ok=1
    break
  fi
  sleep 1
  i=$((i + 1))
done
if [ "$ok" -ne 1 ]; then
  echo "%s not listening on port %d after 120 seconds (pid $pid)" >&2%s
  exit 1
fi`, name, step.Port, line, redir, name, step.Port, tail, step.Port, name, step.Port, tail)
}

func flagValue(argv []string, name string) string {
	prefix := name + "="
	for i, arg := range argv {
		if arg == name && i+1 < len(argv) {
			return argv[i+1]
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func keyFileScript(path string) string {
	dir := filepath.Dir(path)
	return fmt.Sprintf("mkdir -p %s\n( umask 077; openssl rand -base64 32 > %s )\nchmod 600 %s",
		shellquote.Quote(dir), shellquote.Quote(path), shellquote.Quote(path))
}

func retryScript(line string, attempts int) string {
	return fmt.Sprintf(`ok=0
i=0
while [ "$i" -lt %d ]; do
  if %s; then
    ok=1
    break
  fi
  echo "Retrying command..."
  sleep 2
  i=$((i + 1))
done
if [ "$ok" -ne 1 ]; then
  echo "command failed" >&2
  exit 1
fi`, attempts, line)
}

func waitScript(shell string, port int, eval, want, label string) string {
	probe := formatArgv([]string{shell, "--port", fmt.Sprintf("%d", port), "--quiet", "--eval", eval})
	return fmt.Sprintf(`echo "Waiting for %s..."
ok=0
i=0
while [ "$i" -lt 20 ]; do
  if out=$(%s 2>/dev/null) && printf '%%s\n' "$out" | grep -q -x -F -- %s; then
    ok=1
    break
  fi
  sleep 2
  i=$((i + 1))
done
if [ "$ok" -ne 1 ]; then
  echo "%s not ready on port %d after 40 seconds" >&2
  exit 1
fi`, label, probe, shellquote.Quote(want), label, port)
}

// WriteStartScript writes the mongod and mongos argv used by `mlgo start`.
// The file is not executable. Running it as a shell script would block on
// the first server when that server has no --fork.
func WriteStartScript(plan Plan) error {
	dir := filepath.Join(plan.Options.CWD, topology.DataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# argv for `mlgo start`. Do not execute this file.\n")
	for _, step := range plan.Steps {
		if !step.Background {
			continue
		}
		fmt.Fprintln(&b, formatArgv(CommandArgv(step, plan.Options)))
	}
	path := filepath.Join(dir, "start.sh")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
