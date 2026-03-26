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
		Phase:   "coding",
		Workdir: repoPath,
		Prompt:  "hello from relay",
		IssueID: "issue-1",
		LoopID:  "loop-01",
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

func TestResolveAgentRunnerPrefersIssueThenPipelineThenDefault(t *testing.T) {
	tests := []struct {
		name           string
		pipelineRunner string
		issueRunner    string
		want           string
	}{
		{
			name: "default codex",
			want: AgentRunnerCodex,
		},
		{
			name:           "pipeline runner",
			pipelineRunner: AgentRunnerClaude,
			want:           AgentRunnerClaude,
		},
		{
			name:           "issue override",
			pipelineRunner: AgentRunnerCodex,
			issueRunner:    AgentRunnerClaude,
			want:           AgentRunnerClaude,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveAgentRunner(tc.issueRunner, tc.pipelineRunner)
			if err != nil {
				t.Fatalf("ResolveAgentRunner: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestResolveAgentRunnerRejectsInvalidValues(t *testing.T) {
	if _, err := ResolveAgentRunner("cursor", ""); err == nil {
		t.Fatal("expected invalid issue agent_runner to fail")
	}
	if _, err := ResolveAgentRunner("", "cursor"); err == nil {
		t.Fatal("expected invalid pipeline agent_runner to fail")
	}
}

func TestClaudeRunnerCommandSpecPrefersClaudeBinary(t *testing.T) {
	runner := ClaudeRunner{
		LookPath: func(name string) (string, error) {
			switch name {
			case "claude":
				return "/usr/local/bin/claude", nil
			default:
				return "", errors.New("not found")
			}
		},
	}

	spec, err := runner.commandSpec()
	if err != nil {
		t.Fatalf("commandSpec: %v", err)
	}
	if spec.Command != "/usr/local/bin/claude" {
		t.Fatalf("expected claude binary, got %+v", spec)
	}
}

func TestClaudeRunnerCommandSpecUsesExplicitCommand(t *testing.T) {
	runner := ClaudeRunner{
		Command: "/tmp/custom-claude",
		Args:    []string{"--model", "sonnet"},
	}

	spec, err := runner.commandSpec()
	if err != nil {
		t.Fatalf("commandSpec: %v", err)
	}
	if spec.Command != "/tmp/custom-claude" {
		t.Fatalf("expected explicit command, got %+v", spec)
	}
	if len(spec.Args) != 2 || spec.Args[0] != "--model" || spec.Args[1] != "sonnet" {
		t.Fatalf("unexpected explicit args: %+v", spec.Args)
	}
}

func TestClaudeRunnerCommandSpecErrorsWithoutClaude(t *testing.T) {
	runner := ClaudeRunner{
		LookPath: func(name string) (string, error) {
			return "", errors.New("not found")
		},
	}
	if _, err := runner.commandSpec(); err == nil {
		t.Fatalf("expected missing claude error")
	}
}

func TestClaudeRunnerRunDirectCLI(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	recordPath := filepath.Join(tempDir, "args.txt")
	promptPath := filepath.Join(tempDir, "prompt.txt")
	pwdPath := filepath.Join(tempDir, "pwd.txt")
	claudePath := writeFakeClaude(t, tempDir)

	t.Setenv("RUNNER_RECORD_FILE", recordPath)
	t.Setenv("RUNNER_PROMPT_FILE", promptPath)
	t.Setenv("RUNNER_PWD_FILE", pwdPath)
	t.Setenv("RUNNER_STDOUT_TEXT", "claude-stdout")
	t.Setenv("RUNNER_STDERR_TEXT", "claude-stderr")
	t.Setenv("RUNNER_FINAL_TEXT", "claude-final")

	var seenPID int
	runner := ClaudeRunner{Command: claudePath}
	result, err := runner.Run(context.Background(), AgentRunRequest{
		Phase:   "coding",
		Workdir: repoPath,
		Prompt:  "hello from relay",
		IssueID: "issue-1",
		LoopID:  "loop-01",
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
	if result.Stdout != "claude-stdout" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
	if result.Stderr != "claude-stderr" {
		t.Fatalf("unexpected stderr: %q", result.Stderr)
	}
	if result.FinalMessage != "claude-stdout" {
		t.Fatalf("expected final message to mirror stdout, got %q", result.FinalMessage)
	}

	argsData, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	argsText := string(argsData)
	for _, fragment := range []string{"-p", "--dangerously-skip-permissions", "--output-format", "text"} {
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

func TestCodexRunnerStreamsOutputToLogDir(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	logDir := filepath.Join(tempDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	codexPath := writeFakeCodex(t, tempDir)
	t.Setenv("RUNNER_RECORD_FILE", filepath.Join(tempDir, "args.txt"))
	t.Setenv("RUNNER_PROMPT_FILE", filepath.Join(tempDir, "prompt.txt"))
	t.Setenv("RUNNER_PWD_FILE", filepath.Join(tempDir, "pwd.txt"))
	t.Setenv("RUNNER_STDOUT_TEXT", "codex-streamed-stdout")
	t.Setenv("RUNNER_STDERR_TEXT", "codex-streamed-stderr")
	t.Setenv("RUNNER_FINAL_TEXT", "codex-final")

	runner := CodexRunner{Command: codexPath}
	result, err := runner.Run(context.Background(), AgentRunRequest{
		Phase:   "coding",
		Workdir: repoPath,
		Prompt:  "test prompt",
		IssueID: "issue-stream",
		LoopID:  "loop-01",
		LogDir:  logDir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stdout != "codex-streamed-stdout" {
		t.Fatalf("expected stdout in result, got %q", result.Stdout)
	}

	stdoutLog, err := os.ReadFile(filepath.Join(logDir, "loop-01.stdout.log"))
	if err != nil {
		t.Fatalf("read stdout log: %v", err)
	}
	if string(stdoutLog) != "codex-streamed-stdout" {
		t.Fatalf("expected streamed stdout log, got %q", string(stdoutLog))
	}

	stderrLog, err := os.ReadFile(filepath.Join(logDir, "loop-01.stderr.log"))
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	if string(stderrLog) != "codex-streamed-stderr" {
		t.Fatalf("expected streamed stderr log, got %q", string(stderrLog))
	}
}

func TestClaudeRunnerStreamsOutputToLogDir(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	logDir := filepath.Join(tempDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	claudePath := writeFakeClaude(t, tempDir)
	t.Setenv("RUNNER_RECORD_FILE", filepath.Join(tempDir, "args.txt"))
	t.Setenv("RUNNER_PROMPT_FILE", filepath.Join(tempDir, "prompt.txt"))
	t.Setenv("RUNNER_PWD_FILE", filepath.Join(tempDir, "pwd.txt"))
	t.Setenv("RUNNER_STDOUT_TEXT", "claude-streamed-stdout")
	t.Setenv("RUNNER_STDERR_TEXT", "claude-streamed-stderr")

	runner := ClaudeRunner{Command: claudePath}
	result, err := runner.Run(context.Background(), AgentRunRequest{
		Phase:   "coding",
		Workdir: repoPath,
		Prompt:  "test prompt",
		IssueID: "issue-stream",
		LoopID:  "loop-01",
		LogDir:  logDir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	stdoutLog, err := os.ReadFile(filepath.Join(logDir, "loop-01.stdout.log"))
	if err != nil {
		t.Fatalf("read stdout log: %v", err)
	}
	if string(stdoutLog) != "claude-streamed-stdout" {
		t.Fatalf("expected streamed stdout log, got %q", string(stdoutLog))
	}

	stderrLog, err := os.ReadFile(filepath.Join(logDir, "loop-01.stderr.log"))
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	if string(stderrLog) != "claude-streamed-stderr" {
		t.Fatalf("expected streamed stderr log, got %q", string(stderrLog))
	}

	if result.FinalMessage != "claude-streamed-stdout" {
		t.Fatalf("expected final message to mirror stdout, got %q", result.FinalMessage)
	}
}

func writeFakeCodex(t *testing.T, dir string) string {
	t.Helper()
	return writeFakeRunner(t, dir, "codex")
}

func writeFakeClaude(t *testing.T, dir string) string {
	t.Helper()
	return writeFakeRunner(t, dir, "claude")
}

func writeFakeRunner(t *testing.T, dir, name string) string {
	t.Helper()

	scriptPath := filepath.Join(dir, name)
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
		*)
			shift
			;;
	esac
done

cat > "$RUNNER_PROMPT_FILE"

printf '%s' "${RUNNER_STDOUT_TEXT:-runner-stdout}"
printf '%s' "${RUNNER_STDERR_TEXT:-runner-stderr}" >&2
if [ -n "$output_file" ]; then
	printf '%s' "${RUNNER_FINAL_TEXT:-runner-final}" > "$output_file"
fi
`

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake runner: %v", err)
	}
	return scriptPath
}
