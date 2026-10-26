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
func ExecArgv(step Step, windows bool) []string {
	argv := append([]string{}, step.Argv...)
	if step.Background {
		if windows {
			argv = append([]string{"start", "/b"}, argv...)
		} else {
			argv = append(argv, "--fork")
		}
	}
	if windows {
		argv = append([]string{"cmd", "/c"}, argv...)
	}
	return argv
}

// CommandArgv is the argv as it appears in a shell script. The script is
// already interpreted by a shell, so the Windows cmd /c wrapper is omitted.
func CommandArgv(step Step, windows bool) []string {
	argv := append([]string{}, step.Argv...)
	if !step.Background {
		return argv
	}
	if windows {
		return append([]string{"start", "/b"}, argv...)
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
	for _, step := range p.Steps {
		switch step.Kind {
		case StepKeyFile:
			fmt.Fprintln(&b, keyFileScript(mongocmd.KeyFilePath(p.Options.CWD)))
		case StepWaitPrimary:
			fmt.Fprintln(&b, waitScript(p.Options.Shell, step.Port, mongocmd.HelloEval, "true", "primary "+step.Label))
		case StepWaitPing:
			fmt.Fprintln(&b, waitScript(p.Options.Shell, step.Port, mongocmd.PingEval, mongocmd.PingToken, step.Label))
		case StepExec:
			line := formatArgv(CommandArgv(step, p.Options.Windows))
			if step.Retries > 0 {
				fmt.Fprintln(&b, retryScript(line, step.Retries+1))
			} else {
				fmt.Fprintln(&b, line)
			}
		}
	}
	return b.String()
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
  if out=$(%s 2>/dev/null) && printf '%%s\n' "$out" | grep -q -x %s; then
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

// WriteStartScript writes the mongod and mongos lines used by `mlgo start`.
func WriteStartScript(plan Plan) error {
	dir := filepath.Join(plan.Options.CWD, topology.DataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	for _, step := range plan.Steps {
		if !step.Background {
			continue
		}
		fmt.Fprintln(&b, formatArgv(CommandArgv(step, plan.Options.Windows)))
	}
	path := filepath.Join(dir, "start.sh")
	return os.WriteFile(path, []byte(b.String()), 0o755)
}
