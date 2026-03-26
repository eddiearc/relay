package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiearc/relay/internal/cli"
	"github.com/eddiearc/relay/internal/relay"
)

func TestCLIWorkflowWithTempDirsAndFakeRunner(t *testing.T) {
	requireGit(t)

	stateDir := t.TempDir()
	workspaceRoot := filepath.Join(t.TempDir(), "workspaces")

	restore := cli.SetServeRunnerForTesting(func(string) (relay.AgentRunner, error) {
		return &fakeEndToEndRunner{t: t}, nil
	})
	t.Cleanup(restore)

	runTodoWorkflow(t, stateDir, workspaceRoot)
}

func TestServeRealCodexE2E(t *testing.T) {
	if os.Getenv("RELAY_REAL_E2E") == "" {
		t.Skip("set RELAY_REAL_E2E=1 to run the real Codex E2E")
	}
	requireGit(t)

	stateDir := t.TempDir()
	workspaceRoot := filepath.Join(t.TempDir(), "workspaces")

	runTodoWorkflow(t, stateDir, workspaceRoot)
}

func TestWatchReportsCompletedIssueAfterEndToEndServe(t *testing.T) {
	requireGit(t)

	stateDir := t.TempDir()
	workspaceRoot := filepath.Join(t.TempDir(), "workspaces")

	restore := cli.SetServeRunnerForTesting(func(string) (relay.AgentRunner, error) {
		return &fakeEndToEndRunner{t: t}, nil
	})
	t.Cleanup(restore)

	runTodoWorkflow(t, stateDir, workspaceRoot)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := cli.RunWithIO([]string{
		"watch",
		"-issue", "issue-todo-e2e",
		"--poll-interval", "10ms",
		"-state-dir", stateDir,
	}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("watch failed: %d: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"status=done loop=1",
		"progress=progress.txt entries=2 latest=loop 1 complete",
		"event=",
		"terminal_status=done",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected watch output to contain %q, got %s", want, output)
		}
	}
}

func TestWatchReportsFailedIssueAfterEndToEndServe(t *testing.T) {
	requireGit(t)

	stateDir := t.TempDir()
	workspaceRoot := filepath.Join(t.TempDir(), "workspaces")

	restore := cli.SetServeRunnerForTesting(func(string) (relay.AgentRunner, error) {
		return &failingEndToEndRunner{t: t}, nil
	})
	t.Cleanup(restore)

	runFailingWorkflow(t, stateDir, workspaceRoot)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := cli.RunWithIO([]string{
		"watch",
		"-issue", "issue-todo-e2e-failed",
		"--poll-interval", "10ms",
		"-state-dir", stateDir,
	}, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("watch failed to report terminal failure: %d: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"status=failed loop=1",
		"progress=progress.txt entries=1 latest=planning complete",
		"event=",
		"latest_run=loop-01.stderr.log: synthetic coding failure",
		"terminal_status=failed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected watch output to contain %q, got %s", want, output)
		}
	}
}

func TestServeUsesIssueAgentRunnerOverrideEndToEnd(t *testing.T) {
	requireGit(t)

	stateDir := t.TempDir()
	workspaceRoot := filepath.Join(t.TempDir(), "workspaces")

	var runnerNames []string
	restore := cli.SetServeRunnerForTesting(func(name string) (relay.AgentRunner, error) {
		runnerNames = append(runnerNames, name)
		return &fakeEndToEndRunner{t: t}, nil
	})
	t.Cleanup(restore)

	runTodoWorkflowWithRunners(t, stateDir, workspaceRoot, relay.AgentRunnerCodex, relay.AgentRunnerClaude)

	if len(runnerNames) == 0 {
		t.Fatal("expected serve to request a runner")
	}
	if runnerNames[0] != relay.AgentRunnerClaude {
		t.Fatalf("expected issue override to select claude, got %v", runnerNames)
	}
}

func runTodoWorkflow(t *testing.T, stateDir, workspaceRoot string) {
	t.Helper()
	runTodoWorkflowWithRunners(t, stateDir, workspaceRoot, "", "")
}

func runTodoWorkflowWithRunners(t *testing.T, stateDir, workspaceRoot, pipelineRunner, issueRunner string) {
	t.Helper()

	planPrompt := writeTempFile(t, "plan.md", realPlanPrompt)
	codingPrompt := writeTempFile(t, "coding.md", realCodingPrompt)

	pipelineArgs := []string{
		"pipeline", "add",
		"--init-command", todoInitCommand(),
		"--loop-num", "1",
		"--plan-prompt-file", planPrompt,
		"--coding-prompt-file", codingPrompt,
		"-state-dir", stateDir,
	}
	if pipelineRunner != "" {
		pipelineArgs = append(pipelineArgs, "--agent-runner", pipelineRunner)
	}
	pipelineArgs = append(pipelineArgs, "todo-e2e")

	var stderr bytes.Buffer
	if exitCode := cli.RunWithIO(pipelineArgs, io.Discard, &stderr); exitCode != 0 {
		t.Fatalf("pipeline add failed: %s", stderr.String())
	}

	issueArgs := []string{
		"issue", "add",
		"--id", "issue-todo-e2e",
		"--pipeline", "todo-e2e",
		"--goal", "Support persistent todo add/list commands",
		"--description", "Upgrade the sample Go CLI todo app to persist todos and list them.",
		"-state-dir", stateDir,
	}
	if issueRunner != "" {
		issueArgs = append(issueArgs, "--agent-runner", issueRunner)
	}

	stderr.Reset()
	if exitCode := cli.RunWithIO(issueArgs, io.Discard, &stderr); exitCode != 0 {
		t.Fatalf("issue add failed: %s", stderr.String())
	}

	var serveStdout bytes.Buffer
	stderr.Reset()
	if exitCode := cli.RunWithIO([]string{
		"serve",
		"--once",
		"-state-dir", stateDir,
		"--workspace-root", workspaceRoot,
	}, &serveStdout, &stderr); exitCode != 0 {
		t.Fatalf("serve --once failed: %s", stderr.String())
	}

	issue := loadIssueSnapshot(t, stateDir, "issue-todo-e2e")
	if issue.Status != relay.IssueStatusDone {
		t.Fatalf("expected done issue, got %q", issue.Status)
	}
	if issue.ArtifactDir == "" || issue.WorkspacePath == "" || issue.WorkdirPath == "" {
		t.Fatalf("expected artifact/workspace/workdir paths to be set, got %+v", issue)
	}
	if !strings.HasPrefix(issue.ArtifactDir, filepath.Join(stateDir, "issues")) {
		t.Fatalf("expected issue artifact dir under temp state dir, got %s", issue.ArtifactDir)
	}
	if !strings.HasPrefix(issue.WorkspacePath, workspaceRoot) {
		t.Fatalf("expected workspace under temp workspace root, got %s", issue.WorkspacePath)
	}
	if !strings.HasPrefix(issue.WorkdirPath, issue.WorkspacePath) {
		t.Fatalf("expected workdir under workspace, got workdir=%s workspace=%s", issue.WorkdirPath, issue.WorkspacePath)
	}
	if issue.ActivePhase != "" || len(issue.ActivePIDs) != 0 {
		t.Fatalf("expected runtime fields cleared after completion, got phase=%q pids=%v", issue.ActivePhase, issue.ActivePIDs)
	}
	if _, err := os.Stat(relay.FeatureListPath(issue.ArtifactDir)); err != nil {
		t.Fatalf("feature_list.json missing: %v", err)
	}
	if _, err := os.Stat(relay.ProgressPath(issue.ArtifactDir)); err != nil {
		t.Fatalf("progress.txt missing: %v", err)
	}

	addOutput := runCommand(t, issue.WorkdirPath, "go", "run", ".", "add", "buy-milk")
	if strings.TrimSpace(addOutput) != "added: buy-milk" {
		t.Fatalf("unexpected add output: %q", addOutput)
	}
	listOutput := runCommand(t, issue.WorkdirPath, "go", "run", ".", "list")
	if strings.TrimSpace(listOutput) != "1. buy-milk" {
		t.Fatalf("unexpected list output: %q", listOutput)
	}

	featureData, err := os.ReadFile(relay.FeatureListPath(issue.ArtifactDir))
	if err != nil {
		t.Fatalf("read feature_list.json: %v", err)
	}
	var items []relay.FeatureItem
	if err := json.Unmarshal(featureData, &items); err != nil {
		t.Fatalf("parse feature_list.json: %v", err)
	}
	if len(items) == 0 || !relay.AllFeaturesPassed(items) {
		t.Fatalf("expected all features passed, got %+v", items)
	}

	progressData, err := os.ReadFile(relay.ProgressPath(issue.ArtifactDir))
	if err != nil {
		t.Fatalf("read progress.txt: %v", err)
	}
	progress := string(progressData)
	if !strings.Contains(progress, "planning complete") || !strings.Contains(progress, "loop 1 complete") {
		t.Fatalf("unexpected progress.txt: %s", progress)
	}
}

type fakeEndToEndRunner struct {
	t      *testing.T
	coding int
}

func (f *fakeEndToEndRunner) Run(_ context.Context, req relay.AgentRunRequest) (relay.AgentRunResult, error) {
	if req.OnPID != nil {
		req.OnPID(42000 + f.coding + 1)
	}
	switch req.Phase {
	case "plan":
		writeRunOutputs(tHelper{f.t}, req.LogDir, req.LoopID, "ok", "", "planned")
		artifactDir := filepath.Dir(mustExtractPromptPath(f.t, req.Prompt, "FEATURE_LIST_PATH="))
		writeFeatureList(tHelper{f.t}, artifactDir, []relay.FeatureItem{
			{ID: "F-1", Title: "persist todos", Description: "store todo items in todos.txt when adding", Priority: 1, Passes: false},
			{ID: "F-2", Title: "list todos", Description: "print stored todo items in order", Priority: 2, Passes: false},
		})
		appendProgress(tHelper{f.t}, artifactDir, "planning complete")
	case "coding":
		f.coding++
		if f.coding != 1 {
			f.t.Fatalf("unexpected coding run %d", f.coding)
		}
		writeRunOutputs(tHelper{f.t}, req.LogDir, req.LoopID, "ok", "", "done")
		artifactDir := filepath.Dir(mustExtractPromptPath(f.t, req.Prompt, "FEATURE_LIST_PATH="))
		writeFeatureList(tHelper{f.t}, artifactDir, []relay.FeatureItem{
			{ID: "F-1", Title: "persist todos", Description: "store todo items in todos.txt when adding", Priority: 1, Passes: true},
			{ID: "F-2", Title: "list todos", Description: "print stored todo items in order", Priority: 2, Passes: true},
		})
		appendProgress(tHelper{f.t}, artifactDir, "loop 1 complete")
		updateTodoCLIRepo(f.t, req.Workdir)
	default:
		f.t.Fatalf("unexpected phase %q", req.Phase)
	}
	return relay.AgentRunResult{Stdout: "ok", FinalMessage: "done"}, nil
}

type failingEndToEndRunner struct {
	t      *testing.T
	coding int
}

func (f *failingEndToEndRunner) Run(_ context.Context, req relay.AgentRunRequest) (relay.AgentRunResult, error) {
	if req.OnPID != nil {
		req.OnPID(43000 + f.coding + 1)
	}
	switch req.Phase {
	case "plan":
		writeRunOutputs(tHelper{f.t}, req.LogDir, req.LoopID, "ok", "", "planned")
		artifactDir := filepath.Dir(mustExtractPromptPath(f.t, req.Prompt, "FEATURE_LIST_PATH="))
		writeFeatureList(tHelper{f.t}, artifactDir, []relay.FeatureItem{
			{ID: "F-1", Title: "persist todos", Description: "store todo items in todos.txt when adding", Priority: 1, Passes: false},
			{ID: "F-2", Title: "list todos", Description: "print stored todo items in order", Priority: 2, Passes: false},
		})
		appendProgress(tHelper{f.t}, artifactDir, "planning complete")
		return relay.AgentRunResult{Stdout: "ok", FinalMessage: "planned"}, nil
	case "coding":
		f.coding++
		writeRunOutputs(tHelper{f.t}, req.LogDir, req.LoopID, "attempted work", "synthetic coding failure\nmore detail", "")
		return relay.AgentRunResult{
			Stdout:   "attempted work",
			Stderr:   "synthetic coding failure\nmore detail",
			ExitCode: 1,
		}, context.DeadlineExceeded
	default:
		f.t.Fatalf("unexpected phase %q", req.Phase)
		return relay.AgentRunResult{}, nil
	}
}

type tHelper struct{ *testing.T }

func mustExtractPromptPath(t *testing.T, prompt, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("missing %s in prompt", prefix)
	return ""
}

func writeFeatureList(t tHelper, artifactDir string, items []relay.FeatureItem) {
	t.Helper()
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatalf("marshal feature list: %v", err)
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(relay.FeatureListPath(artifactDir), data, 0o644); err != nil {
		t.Fatalf("write feature list: %v", err)
	}
}

func appendProgress(t tHelper, artifactDir, text string) {
	t.Helper()
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	file, err := os.OpenFile(relay.ProgressPath(artifactDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open progress.txt: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(text + "\n"); err != nil {
		t.Fatalf("append progress: %v", err)
	}
}

func writeRunOutputs(t tHelper, logDir, loopID, stdout, stderr, finalMessage string) {
	t.Helper()
	if logDir == "" || loopID == "" {
		return
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, loopID+".stdout.log"), []byte(stdout), 0o644); err != nil {
		t.Fatalf("write stdout log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, loopID+".stderr.log"), []byte(stderr), 0o644); err != nil {
		t.Fatalf("write stderr log: %v", err)
	}
	if finalMessage != "" {
		if err := os.WriteFile(filepath.Join(logDir, loopID+".final.txt"), []byte(finalMessage), 0o644); err != nil {
			t.Fatalf("write final message: %v", err)
		}
	}
}

func updateTodoCLIRepo(t *testing.T, repoPath string) {
	t.Helper()
	mainGo := `package main

import (
	"bufio"
	"fmt"
	"os"
)

const storeFile = "todos.txt"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: todo <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("usage: todo add <item>")
			os.Exit(1)
		}
		file, err := os.OpenFile(storeFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open store: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		if _, err := fmt.Fprintln(file, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "write todo: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("added: %s\n", os.Args[2])
	case "list":
		file, err := os.Open(storeFile)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			fmt.Fprintf(os.Stderr, "open store: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		index := 1
		for scanner.Scan() {
			fmt.Printf("%d. %s\n", index, scanner.Text())
			index++
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "scan store: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(filepath.Join(repoPath, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runCommand(t, repoPath, "git", "add", "main.go")
	runCommand(t, repoPath, "git", "commit", "-m", "feat: persist and list todos")
}

func todoInitCommand() string {
	return `set -e
mkdir repo
cd repo
git init
git config user.email relay@example.com
git config user.name relay
cat > go.mod <<'EOF'
module todoapp

go 1.24.0
EOF
cat > main.go <<'EOF'
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: todo <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("usage: todo add <item>")
			os.Exit(1)
		}
		fmt.Printf("added: %s\n", os.Args[2])
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
EOF
git add go.mod main.go
git commit -m 'init todo cli'`
}

func loadIssueSnapshot(t *testing.T, stateDir, issueID string) relay.Issue {
	t.Helper()
	store := relay.NewStore(stateDir)
	issue, err := store.LoadIssue(issueID)
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	return issue
}

func runCommand(t *testing.T, workdir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
	return string(output)
}

func runFailingWorkflow(t *testing.T, stateDir, workspaceRoot string) {
	t.Helper()

	planPrompt := writeTempFile(t, "plan.md", realPlanPrompt)
	codingPrompt := writeTempFile(t, "coding.md", realCodingPrompt)

	var stderr bytes.Buffer
	if exitCode := cli.RunWithIO([]string{
		"pipeline", "add",
		"--init-command", todoInitCommand(),
		"--loop-num", "1",
		"--plan-prompt-file", planPrompt,
		"--coding-prompt-file", codingPrompt,
		"-state-dir", stateDir,
		"todo-e2e-failed",
	}, io.Discard, &stderr); exitCode != 0 {
		t.Fatalf("pipeline add failed: %s", stderr.String())
	}

	stderr.Reset()
	if exitCode := cli.RunWithIO([]string{
		"issue", "add",
		"--id", "issue-todo-e2e-failed",
		"--pipeline", "todo-e2e-failed",
		"--goal", "Support persistent todo add/list commands",
		"--description", "Upgrade the sample Go CLI todo app to persist todos and list them.",
		"-state-dir", stateDir,
	}, io.Discard, &stderr); exitCode != 0 {
		t.Fatalf("issue add failed: %s", stderr.String())
	}

	var serveStdout bytes.Buffer
	stderr.Reset()
	if exitCode := cli.RunWithIO([]string{
		"serve",
		"--once",
		"-state-dir", stateDir,
		"--workspace-root", workspaceRoot,
	}, &serveStdout, &stderr); exitCode == 0 {
		t.Fatalf("expected serve --once to fail")
	}

	issue := loadIssueSnapshot(t, stateDir, "issue-todo-e2e-failed")
	if issue.Status != relay.IssueStatusFailed {
		t.Fatalf("expected failed issue, got %q", issue.Status)
	}
	if _, err := os.Stat(relay.ProgressPath(issue.ArtifactDir)); err != nil {
		t.Fatalf("progress.txt missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(relay.ProgressPath(issue.ArtifactDir)), "runs", "loop-01.stderr.log")); err != nil {
		t.Fatalf("loop stderr log missing: %v", err)
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

const realPlanPrompt = `Understand the repository and the task.

Write a non-empty feature_list.json to FEATURE_LIST_PATH and initialize progress.txt at PROGRESS_PATH.
The task is done only when the Go TODO CLI supports both:
- persistent "add" behavior that stores todos
- a "list" command that prints stored todos in order
`

const realCodingPrompt = `Implement the requested Go TODO CLI behavior in WORKDIR_PATH.

Requirements:
- "go run . add <item>" should persist the todo item
- "go run . list" should print stored todo items in order as "1. item"
- update FEATURE_LIST_PATH to reflect actual completion
- append a summary to PROGRESS_PATH
- commit repository changes before finishing
`
