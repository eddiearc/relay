package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eddiearc/relay/internal/relay"
)

func TestHelpIncludesWatchCommand(t *testing.T) {
	var stdout bytes.Buffer
	if exitCode := run([]string{"help"}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "watch") {
		t.Fatalf("expected help output to include watch, got %s", stdout.String())
	}
}

func TestPipelineTemplateIncludesProjectBlock(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := run([]string{"pipeline", "template"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"project:",
		"key: github.com/owner/repo",
		"path: .",
		"remote_url: https://github.com/owner/repo.git",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected pipeline template output to contain %q, got %s", want, output)
		}
	}
}

func TestUpgradeCheckReportsAvailableUpdate(t *testing.T) {
	previousVersion := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = previousVersion
	})

	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		if method != installMethodNPM {
			t.Fatalf("expected npm lookup, got %q", method)
		}
		return "v1.2.4", nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreLatestVersionLookup()
	})

	var stdout bytes.Buffer
	exitCode := run([]string{"upgrade", "--check"}, &stdout, io.Discard)
	if exitCode != 2 {
		t.Fatalf("expected exit 2 for update available, got %d", exitCode)
	}
	output := stdout.String()
	for _, want := range []string{
		"current_version=v1.2.3",
		"install_method=npm",
		"latest_version=v1.2.4",
		"recommended_upgrade_command=npm update -g @eddiearc/relay",
		"skill_refresh_command=npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected upgrade --check output to contain %q, got %s", want, output)
		}
	}
}

func TestUpgradeCheckLocalBuildReportsReinstallPath(t *testing.T) {
	previousVersion := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = previousVersion
	})

	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/tmp/relay/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		if method != installMethodNPM {
			t.Fatalf("expected local-build check to consult npm first, got %q", method)
		}
		return "v1.2.4", nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreLatestVersionLookup()
	})

	var stdout bytes.Buffer
	exitCode := run([]string{"upgrade", "--check"}, &stdout, io.Discard)
	if exitCode != 2 {
		t.Fatalf("expected exit 2 for update available, got %d", exitCode)
	}
	output := stdout.String()
	for _, want := range []string{
		"install_method=local-build",
		"latest_version=v1.2.4",
		"recommended_upgrade_command=reinstall via npm or go install",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected upgrade --check output to contain %q, got %s", want, output)
		}
	}
}

func TestUpgradeCheckReturnsLookupErrors(t *testing.T) {
	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		return "", errors.New("registry unavailable")
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreLatestVersionLookup()
	})

	var stderr bytes.Buffer
	if exitCode := run([]string{"upgrade", "--check"}, io.Discard, &stderr); exitCode == 0 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(stderr.String(), "check latest relay version") {
		t.Fatalf("expected latest-version lookup error, got %s", stderr.String())
	}
}

func TestPipelineMatchExactAndRepoFallback(t *testing.T) {
	stateDir := t.TempDir()
	projectKey := "github.com/example/monorepo"
	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name: "repo-default",
		Project: &relay.PipelineProject{
			Key:       projectKey,
			Path:      ".",
			RemoteURL: "https://github.com/example/monorepo.git",
		},
		InitCommand:  "git clone repo .",
		LoopNum:      15,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	})
	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name: "apps-web",
		Project: &relay.PipelineProject{
			Key:       projectKey,
			Path:      "apps/web",
			RemoteURL: "https://github.com/example/monorepo.git",
		},
		InitCommand:  "git clone repo .",
		LoopNum:      15,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	})
	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name:         "legacy-no-project",
		InitCommand:  "git clone repo .",
		LoopNum:      15,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	})

	repoRoot := filepath.Join(t.TempDir(), "repo")
	mustMkdirAll(t, filepath.Join(repoRoot, "apps", "web"))
	initGitRepo(t, repoRoot, "https://github.com/example/monorepo.git")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "match", "--repo", filepath.Join(repoRoot, "apps", "web"), "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "matched_pipeline=apps-web") || !strings.Contains(stdout.String(), "match_reason=exact-project-path") {
		t.Fatalf("expected exact project-path match, got %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"pipeline", "match", "--repo", repoRoot, "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "matched_pipeline=repo-default") || !strings.Contains(stdout.String(), "match_reason=exact-project-path") {
		t.Fatalf("expected repo-level match, got %s", stdout.String())
	}
}

func TestPipelineMatchReturnsAmbiguousCandidates(t *testing.T) {
	stateDir := t.TempDir()
	projectKey := "github.com/example/repo"
	for _, name := range []string{"demo-a", "demo-b"} {
		savePipelineForTest(t, stateDir, relay.Pipeline{
			Name: name,
			Project: &relay.PipelineProject{
				Key:       projectKey,
				Path:      ".",
				RemoteURL: "https://github.com/example/repo.git",
			},
			InitCommand:  "git clone repo .",
			LoopNum:      15,
			PlanPrompt:   "plan",
			CodingPrompt: "code",
		})
	}

	repoRoot := filepath.Join(t.TempDir(), "repo")
	mustMkdirAll(t, repoRoot)
	initGitRepo(t, repoRoot, "https://github.com/example/repo.git")

	var stdout bytes.Buffer
	exitCode := run([]string{"pipeline", "match", "--repo", repoRoot, "-state-dir", stateDir}, &stdout, io.Discard)
	if exitCode != 2 {
		t.Fatalf("expected exit 2 for ambiguous candidates, got %d", exitCode)
	}
	output := stdout.String()
	for _, want := range []string{"multiple pipeline candidates", "demo-a", "demo-b"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected ambiguous output to contain %q, got %s", want, output)
		}
	}
}

func TestPipelineShowPrintsSummaryAndYAML(t *testing.T) {
	stateDir := t.TempDir()
	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name: "demo-show",
		Project: &relay.PipelineProject{
			Key:       "github.com/example/repo",
			Path:      "apps/web",
			RemoteURL: "https://github.com/example/repo.git",
		},
		InitCommand:  "git clone --depth 1 https://github.com/example/repo .",
		LoopNum:      15,
		PlanPrompt:   "Read the repository before planning.\nBreak the goal into verifiable features.\nEach feature must have observable acceptance conditions.",
		CodingPrompt: "Stay on the task branch.\nVerify progress with real commands where possible.\nUpdate FEATURE_LIST_PATH based on verified state.",
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "show", "-state-dir", stateDir, "demo-show"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"summary:",
		"- project_key: github.com/example/repo",
		"- project_path: apps/web",
		"- verification_path:",
		"yaml:",
		"name: demo-show",
		"coding_prompt:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected pipeline show output to contain %q, got %s", want, output)
		}
	}
}

func TestIssueEvaluateReturnsNotReady(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-evaluate-not-ready")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"issue", "evaluate",
		"--pipeline", "demo-evaluate-not-ready",
		"--goal", "Improve it",
		"--description", "Make things better quickly.",
		"-state-dir", stateDir,
	}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"evaluation=not_ready",
		"goal_clarity=fail",
		"verification_specificity=fail",
		"scope_constraints_non_goals=fail",
		"required_changes:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected evaluate output to contain %q, got %s", want, output)
		}
	}
}

func TestIssueEvaluateReturnsReadyPreview(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-evaluate-ready")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"issue", "evaluate",
		"--pipeline", "demo-evaluate-ready",
		"--goal", "Add a readable CLI summary for saved pipeline metadata",
		"--description", "Only touch the CLI summary output and persisted pipeline presentation. Non-goal: worker runtime changes. Keep the output deterministic. Verify with go test ./... and relay pipeline show demo-evaluate-ready.",
		"-state-dir", stateDir,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"evaluation=ready",
		"goal_clarity=pass",
		"verification_specificity=pass",
		"scope_constraints_non_goals=pass",
		"execution_preview:",
		"- monitor_with: relay watch -issue <new-issue-id>",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected evaluate output to contain %q, got %s", want, output)
		}
	}
}

func TestWatchReportsStateFlowAndCompletes(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-watch")
	store := relay.NewStore(stateDir)
	store.WorkspaceRoot = filepath.Join(stateDir, "workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure store: %v", err)
	}

	issue := relay.Issue{
		ID:           "issue-watch",
		PipelineName: "demo-watch",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusTodo,
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}

	done := make(chan int, 1)
	var stdout bytes.Buffer
	go func() {
		done <- run([]string{"watch", "-issue", issue.ID, "--poll-interval", "10ms", "-state-dir", stateDir}, &stdout, io.Discard)
	}()

	time.Sleep(30 * time.Millisecond)
	issue.Status = relay.IssueStatusPlanning
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save planning issue: %v", err)
	}
	if err := os.WriteFile(relay.ProgressPath(store.IssueDir(issue.ID)), []byte("planned initial features\n"), 0o644); err != nil {
		t.Fatalf("write progress: %v", err)
	}
	if err := store.AppendEvent(issue.ID, "planning started"); err != nil {
		t.Fatalf("append planning event: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
	issue.Status = relay.IssueStatusRunning
	issue.CurrentLoop = 1
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save running issue: %v", err)
	}
	if err := os.WriteFile(relay.ProgressPath(store.IssueDir(issue.ID)), []byte("planned initial features\nloop 1 verified\n"), 0o644); err != nil {
		t.Fatalf("write progress update: %v", err)
	}
	if err := store.AppendEvent(issue.ID, "coding loop=1 started"); err != nil {
		t.Fatalf("append running event: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
	issue.Status = relay.IssueStatusDone
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save done issue: %v", err)
	}
	if err := store.AppendEvent(issue.ID, "issue completed loop=1"); err != nil {
		t.Fatalf("append done event: %v", err)
	}

	select {
	case exitCode := <-done:
		if exitCode != 0 {
			t.Fatalf("expected watch to exit 0, got %d", exitCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not complete")
	}

	output := stdout.String()
	for _, want := range []string{
		"status=todo loop=0",
		"status_change=todo->planning",
		"status_change=planning->running loop=1",
		"progress=progress.txt entries=2 latest=loop 1 verified",
		"event=",
		"terminal_status=done",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected watch output to contain %q, got %s", want, output)
		}
	}
}

func TestWatchReturnsFailureSummary(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-watch-failed")
	store := relay.NewStore(stateDir)
	store.WorkspaceRoot = filepath.Join(stateDir, "workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure store: %v", err)
	}

	issue := relay.Issue{
		ID:           "issue-watch-failed",
		PipelineName: "demo-watch-failed",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusFailed,
		LastError:    "loop failed",
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}
	if err := os.MkdirAll(store.RunDir(issue.ID), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.RunDir(issue.ID), "loop-01.stderr.log"), []byte("command failed\nmore detail\n"), 0o644); err != nil {
		t.Fatalf("write stderr log: %v", err)
	}

	var stdout bytes.Buffer
	exitCode := run([]string{"watch", "-issue", issue.ID, "--poll-interval", "10ms", "-state-dir", stateDir}, &stdout, io.Discard)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	output := stdout.String()
	for _, want := range []string{
		"status=failed loop=0",
		"latest_run=loop-01.stderr.log: command failed",
		"terminal_status=failed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected watch output to contain %q, got %s", want, output)
		}
	}
}

func savePipelineForTest(t *testing.T, stateDir string, pipeline relay.Pipeline) {
	t.Helper()
	store := relay.NewStore(stateDir)
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure store: %v", err)
	}
	if err := store.SavePipeline(pipeline); err != nil {
		t.Fatalf("save pipeline %s: %v", pipeline.Name, err)
	}
}

func initGitRepo(t *testing.T, root, remote string) {
	t.Helper()
	runTestCommand(t, root, "git", "init")
	if remote != "" {
		runTestCommand(t, root, "git", "remote", "add", "origin", remote)
	}
}

func runTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
