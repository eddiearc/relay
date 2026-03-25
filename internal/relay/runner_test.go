package relay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexRunnerCommandSpecPrefersCodexBinary(t *testing.T) {
	runner := CodexRunner{
		LookPath: func(name string) (string, error) {
			switch name {
			case "codex":
				return "/usr/local/bin/codex", nil
			default:
				return "", errors.New("not found")
			}
		},
	}

	spec, err := runner.commandSpec()
	if err != nil {
		t.Fatalf("commandSpec: %v", err)
	}
	if spec.Command != "/usr/local/bin/codex" {
		t.Fatalf("expected codex binary, got %+v", spec)
	}
}

func TestCodexRunnerCommandSpecUsesExplicitCommand(t *testing.T) {
	runner := CodexRunner{
		Command: "/tmp/custom-codex",
		Args:    []string{"--profile", "relay"},
	}

	spec, err := runner.commandSpec()
	if err != nil {
		t.Fatalf("commandSpec: %v", err)
	}
	if spec.Command != "/tmp/custom-codex" {
		t.Fatalf("expected explicit command, got %+v", spec)
	}
	if len(spec.Args) != 2 || spec.Args[0] != "--profile" || spec.Args[1] != "relay" {
		t.Fatalf("unexpected explicit args: %+v", spec.Args)
	}
}

func TestCodexRunnerCommandSpecErrorsWithoutCodex(t *testing.T) {
	runner := CodexRunner{
		LookPath: func(name string) (string, error) {
			return "", errors.New("not found")
		},
	}
	if _, err := runner.commandSpec(); err == nil {
		t.Fatalf("expected missing codex error")
	}
}

func TestCodexRunnerRunDirectCLI(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	recordPath := filepath.Join(tempDir, "args.txt")
	promptPath := filepath.Join(tempDir, "prompt.txt")
	pwdPath := filepath.Join(tempDir, "pwd.txt")
	codexPath := writeFakeCodex(t, tempDir)

	t.Setenv("RUNNER_RECORD_FILE", recordPath)
	t.Setenv("RUNNER_PROMPT_FILE", promptPath)
	t.Setenv("RUNNER_PWD_FILE", pwdPath)
	t.Setenv("RUNNER_STDOUT_TEXT", "runner-stdout")
	t.Setenv("RUNNER_STDERR_TEXT", "runner-stderr")
	t.Setenv("RUNNER_FINAL_TEXT", "runner-final")

	var seenPID int
	runner := CodexRunner{Command: codexPath}
	result, err := runner.Run(context.Background(), AgentRunRequest{
		Phase:    "coding",
		Workdir:  repoPath,
		Prompt:   "hello from relay",
		IssueID:  "issue-1",
		LoopID:   "loop-01",
		OnPID: func(pid int) {
			seenPID = pid
		},
	})
	if err != nil {
		t.Fatalf("Run: %v\nstderr=%s", err, result.Stderr)
	}
	if seenPID <= 0 {
		t.Fatalf("expected runner to report process pid, got %d", seenPID)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected zero exit code, got %d", result.ExitCode)
	}
	if result.Stdout != "runner-stdout" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
	if result.Stderr != "runner-stderr" {
		t.Fatalf("unexpected stderr: %q", result.Stderr)
	}
	if result.FinalMessage != "runner-final" {
		t.Fatalf("unexpected final message: %q", result.FinalMessage)
	}

	argsData, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	argsText := string(argsData)
	for _, fragment := range []string{"exec", "--dangerously-bypass-approvals-and-sandbox", "-C", repoPath, "-"} {
		if !strings.Contains(argsText, fragment) {
			t.Fatalf("expected args to contain %q, got %q", fragment, argsText)
		}
	}

	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt file: %v", err)
	}
	if string(promptData) != "hello from relay" {
		t.Fatalf("unexpected prompt contents: %q", string(promptData))
	}

	pwdData, err := os.ReadFile(pwdPath)
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if strings.TrimSpace(string(pwdData)) != repoPath {
		t.Fatalf("expected command to run in repo path %q, got %q", repoPath, string(pwdData))
	}
}

func writeFakeCodex(t *testing.T, dir string) string {
	t.Helper()

	scriptPath := filepath.Join(dir, "codex")
	script := `#!/bin/sh
set -eu

: "${RUNNER_RECORD_FILE:?}"
: "${RUNNER_PROMPT_FILE:?}"
: "${RUNNER_PWD_FILE:?}"

printf '%s\n' "$*" > "$RUNNER_RECORD_FILE"
pwd > "$RUNNER_PWD_FILE"

output_file=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-C)
			shift 2
			;;
		-o)
			output_file="$2"
			shift 2
			;;
		-)
			cat > "$RUNNER_PROMPT_FILE"
			shift
			;;
		*)
			shift
			;;
	esac
done

printf '%s' "${RUNNER_STDOUT_TEXT:-runner-stdout}"
printf '%s' "${RUNNER_STDERR_TEXT:-runner-stderr}" >&2
if [ -n "$output_file" ]; then
	printf '%s' "${RUNNER_FINAL_TEXT:-runner-final}" > "$output_file"
fi
`

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return scriptPath
}
