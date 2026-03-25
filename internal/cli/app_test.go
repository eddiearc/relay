package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/eddiearc/relay/internal/relay"
)

func TestResolveStateDirDefaultsToHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("home directory unavailable")
	}
	got := resolveStateDir("")
	want := filepath.Join(home, ".relay")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveStateDirResolvesRelativePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	got := resolveStateDir("tmp-state")
	want := filepath.Join(cwd, "tmp-state")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestHelpIncludesVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := run([]string{"help"}, &stdout, io.Discard)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("version")) {
		t.Fatalf("expected help output to include version command, got %s", stdout.String())
	}
}

func TestVersionCommandPrintsInjectedBuildMetadata(t *testing.T) {
	previousVersion := version
	previousCommit := commit
	previousDate := buildDate
	version = "v1.2.3"
	commit = "abc1234"
	buildDate = "2026-03-24T12:34:56Z"
	t.Cleanup(func() {
		version = previousVersion
		commit = previousCommit
		buildDate = previousDate
	})

	var stdout bytes.Buffer
	exitCode := run([]string{"version"}, &stdout, io.Discard)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}

	output := stdout.String()
	for _, want := range []string{
		"relay v1.2.3",
		"commit: abc1234",
		"built: 2026-03-24T12:34:56Z",
	} {
		if !bytes.Contains([]byte(output), []byte(want)) {
			t.Fatalf("expected version output to contain %q, got %s", want, output)
		}
	}
}

func TestPipelineAddSavesYAMLPipeline(t *testing.T) {
	stateDir := t.TempDir()
	planPrompt := writeTempFile(t, "plan.md", "plan {{issue}}")
	codingPrompt := writeTempFile(t, "coding.md", "code {{issue}}")

	var stderr bytes.Buffer
	exitCode := run([]string{
		"pipeline", "add",
		"--init-command", "git init repo",
		"--loop-num", "2",
		"--plan-prompt-file", planPrompt,
		"--coding-prompt-file", codingPrompt,
		"-state-dir", stateDir,
		"demo",
	}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "pipelines", "demo.yaml")); err != nil {
		t.Fatalf("expected yaml pipeline file to be saved: %v", err)
	}
}

func TestPipelineImportSavesYAMLPipeline(t *testing.T) {
	stateDir := t.TempDir()
	pipelineFile := writeTempFile(t, "pipeline.yaml", ""+
		"name: demo-import\n"+
		"init_command: git init repo\n"+
		"loop_num: 2\n"+
		"plan_prompt: plan {{issue}}\n"+
		"coding_prompt: code {{issue}}\n")

	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "import", "-file", pipelineFile, "-state-dir", stateDir}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "pipelines", "demo-import.yaml")); err != nil {
		t.Fatalf("expected imported pipeline file to be saved: %v", err)
	}
}

func TestIssueAddCreatesPerIssueDirectory(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"issue", "add",
		"--id", "issue-add",
		"--pipeline", "demo",
		"--goal", "ship feature",
		"--description", "test issue",
		"-state-dir", stateDir,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "issues", "issue-add", "issue.json")); err != nil {
		t.Fatalf("expected issue.json to be created: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"artifact_dir"`)) {
		t.Fatalf("expected artifact dir in output, got %s", stdout.String())
	}
}

func TestIssueImportCreatesPerIssueDirectory(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo")

	issueFile := writeTempFile(t, "issue.json", `{
  "id": "issue-import",
  "pipeline_name": "demo",
  "goal": "ship feature",
  "description": "test issue"
}`)

	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "import", "-file", issueFile, "-state-dir", stateDir}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "issues", "issue-import", "issue.json")); err != nil {
		t.Fatalf("expected issue.json to be created: %v", err)
	}
}

func TestIssueEditAllowsRunningIssue(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-running-edit")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-running-edit",
		PipelineName: "demo-running-edit",
		Goal:         "old goal",
		Description:  "old desc",
		Status:       relay.IssueStatusRunning,
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"issue", "edit",
		"--id", "issue-running-edit",
		"--goal", "new goal",
		"--description", "new desc",
		"-state-dir", stateDir,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("issue edit failed: %s", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status": "running"`)) {
		t.Fatalf("expected running issue output, got %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"goal": "new goal"`)) {
		t.Fatalf("expected updated goal, got %s", stdout.String())
	}
}

func TestIssueDeleteFailsWhenRunning(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-running-delete")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-running-delete",
		PipelineName: "demo-running-delete",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
	})

	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "delete", "--id", "issue-running-delete", "-state-dir", stateDir}, io.Discard, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected issue delete to fail for running issue")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("cannot be deleted")) {
		t.Fatalf("expected running issue delete error, got %s", stderr.String())
	}
}

func TestIssueInterruptRequestsStopForRunningIssue(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-running-interrupt")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-running-interrupt",
		PipelineName: "demo-running-interrupt",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "interrupt", "--id", "issue-running-interrupt", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("issue interrupt failed: %s", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status": "running"`)) {
		t.Fatalf("expected running status until current loop ends, got %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"interrupt_requested": true`)) {
		t.Fatalf("expected interrupt request flag, got %s", stdout.String())
	}
}

func TestRecoverActiveIssuesMarksOrphanedRunsTodo(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-recover")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-recover",
		PipelineName: "demo-recover",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
		ActivePhase:  "coding",
		ActivePIDs:   []int{12345},
	})

	store := relay.NewStore(stateDir)
	recovered, err := recoverActiveIssues(store)
	if err != nil {
		t.Fatalf("recoverActiveIssues: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered issue, got %d", recovered)
	}
	issue := loadIssueSnapshot(t, stateDir, "issue-recover")
	if issue.Status != relay.IssueStatusTodo {
		t.Fatalf("expected todo after recovery, got %q", issue.Status)
	}
	if issue.ActivePhase != "" || len(issue.ActivePIDs) != 0 {
		t.Fatalf("expected active runtime fields to be cleared, got phase=%q pids=%v", issue.ActivePhase, issue.ActivePIDs)
	}
}

func TestPipelineDeleteFailsWhenIssueRunning(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-pipeline-running-delete")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-pipeline-running",
		PipelineName: "demo-pipeline-running-delete",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
	})

	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "delete", "-state-dir", stateDir, "demo-pipeline-running-delete"}, io.Discard, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected pipeline delete to fail for running issue")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("active issue")) {
		t.Fatalf("expected active issue error, got %s", stderr.String())
	}
}

func TestPipelineDeleteAllowsTodoIssue(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-pipeline-todo-delete")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-pipeline-todo",
		PipelineName: "demo-pipeline-todo-delete",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusTodo,
	})

	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "delete", "-state-dir", stateDir, "demo-pipeline-todo-delete"}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected pipeline delete to succeed for todo issue: %s", stderr.String())
	}
}

func TestStatusReadsIssueDirectoryState(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-status")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:            "issue-status",
		PipelineName:  "demo-status",
		Goal:          "goal",
		Description:   "desc",
		Status:        relay.IssueStatusRunning,
		ArtifactDir:   filepath.Join(stateDir, "issues", "issue-status"),
		WorkspacePath: "/tmp/workspace",
		RepoPath:      "/tmp/repo",
	})

	var stdout bytes.Buffer
	if exitCode := run([]string{"status", "-issue", "issue-status", "-state-dir", stateDir}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("status command failed")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("artifact=")) {
		t.Fatalf("expected artifact path in status output, got %s", stdout.String())
	}
}

func TestReportListsArtifactsAndEventsLog(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-report")
	issue := relay.Issue{
		ID:           "issue-report",
		PipelineName: "demo-report",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusDone,
	}
	saveIssueSnapshot(t, stateDir, issue)
	store := relay.NewStore(stateDir)
	if err := os.WriteFile(relay.FeatureListPath(store.IssueDir(issue.ID)), []byte("[]"), 0o644); err != nil {
		t.Fatalf("write feature_list.json: %v", err)
	}
	if err := os.WriteFile(relay.ProgressPath(store.IssueDir(issue.ID)), []byte("done"), 0o644); err != nil {
		t.Fatalf("write progress.txt: %v", err)
	}
	if err := store.AppendEvent(issue.ID, "issue completed"); err != nil {
		t.Fatalf("append event: %v", err)
	}

	var stdout bytes.Buffer
	if exitCode := run([]string{"report", "-issue", "issue-report", "-state-dir", stateDir}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("report command failed")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("artifacts:")) {
		t.Fatalf("expected artifacts section, got %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("events.log")) {
		t.Fatalf("expected events.log path, got %s", stdout.String())
	}
}

func TestKillTerminatesTrackedIssueProcesses(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-kill")

	cmd := exec.Command("zsh", "-lc", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}

	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-kill",
		PipelineName: "demo-kill",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
		ActivePhase:  "coding",
		ActivePIDs:   []int{cmd.Process.Pid},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"kill", "-issue", "issue-kill", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("kill failed: %s", stderr.String())
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	select {
	case <-time.After(3 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		t.Fatalf("expected process %d to exit after kill", cmd.Process.Pid)
	case <-waitDone:
	}

	issue := loadIssueSnapshot(t, stateDir, "issue-kill")
	if issue.Status != relay.IssueStatusFailed {
		t.Fatalf("expected failed issue after kill, got %q", issue.Status)
	}
	if issue.ActivePhase != "" || len(issue.ActivePIDs) != 0 {
		t.Fatalf("expected runtime fields to be cleared after kill, got phase=%q pids=%v", issue.ActivePhase, issue.ActivePIDs)
	}
}

func importTestPipeline(t *testing.T, stateDir, name string) {
	t.Helper()
	pipelineFile := writeTempFile(t, "pipeline.yaml", ""+
		"name: "+name+"\n"+
		"init_command: git init repo\n"+
		"loop_num: 2\n"+
		"plan_prompt: plan {{issue}}\n"+
		"coding_prompt: code {{issue}}\n")
	if exitCode := run([]string{"pipeline", "import", "-file", pipelineFile, "-state-dir", stateDir}, io.Discard, io.Discard); exitCode != 0 {
		t.Fatalf("pipeline import failed")
	}
}

func saveIssueSnapshot(t *testing.T, stateDir string, issue relay.Issue) {
	t.Helper()
	store := relay.NewStore(stateDir)
	store.WorkspaceRoot = filepath.Join(stateDir, "relay-workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure store: %v", err)
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}
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

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}
