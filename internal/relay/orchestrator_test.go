package relay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrchestratorCompletesIssueFromIssueArtifacts(t *testing.T) {
	requireGit(t)

	root := t.TempDir()
	store := NewStore(filepath.Join(root, ".relay"))
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	pipeline, issue := testRunInput()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	orchestrator := NewOrchestrator(store, ZshRunner{}, &fakeAgentRunner{
		t: t,
		plan: func(req AgentRunRequest) {
			artifactDir := mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH=")
			writeFeatureList(t, filepath.Dir(artifactDir), []FeatureItem{
				{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: false},
				{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: false},
			})
			appendProgress(t, filepath.Dir(artifactDir), "planning complete")
		},
		coding: []func(AgentRunRequest){
			func(req AgentRunRequest) {
				artifactDir := mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH=")
				writeFeatureList(t, filepath.Dir(artifactDir), []FeatureItem{
					{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: true},
					{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: false},
				})
				appendProgress(t, filepath.Dir(artifactDir), "loop 1 complete")
				writeRepoChangeAndCommit(t, req.RepoPath, "first.txt", "loop 1\n", "feat: finish first feature")
			},
			func(req AgentRunRequest) {
				artifactDir := mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH=")
				writeFeatureList(t, filepath.Dir(artifactDir), []FeatureItem{
					{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: true},
					{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: true},
				})
				appendProgress(t, filepath.Dir(artifactDir), "loop 2 complete")
				writeRepoChangeAndCommit(t, req.RepoPath, "second.txt", "loop 2\n", "feat: finish all features")
			},
		},
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}

	issue, err = orchestrator.RunIssue(context.Background(), pipeline, issue)
	if err != nil {
		t.Fatalf("RunIssue failed: %v", err)
	}
	if issue.Status != IssueStatusDone {
		t.Fatalf("expected done issue, got %q", issue.Status)
	}
	if issue.CurrentLoop != 2 {
		t.Fatalf("expected 2 loops, got %d", issue.CurrentLoop)
	}
	if issue.ArtifactDir != filepath.Join(root, ".relay", "issues", issue.ID) {
		t.Fatalf("unexpected artifact dir: %s", issue.ArtifactDir)
	}
	if got := filepath.Dir(issue.WorkspacePath); got != filepath.Join(root, "relay-workspaces") {
		t.Fatalf("expected workspace root under relay-workspaces, got %s", got)
	}
	if _, err := os.Stat(FeatureListPath(issue.ArtifactDir)); err != nil {
		t.Fatalf("feature_list.json missing: %v", err)
	}
	if _, err := os.Stat(ProgressPath(issue.ArtifactDir)); err != nil {
		t.Fatalf("progress.txt missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(issue.ArtifactDir, "runs", "plan.stdout.log")); err != nil {
		t.Fatalf("expected run log in issue dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(issue.ArtifactDir, "runs", "init.stdout.log")); err != nil {
		t.Fatalf("expected init stdout log in issue dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(issue.ArtifactDir, "runs", "init.stderr.log")); err != nil {
		t.Fatalf("expected init stderr log in issue dir: %v", err)
	}
	eventsData, err := os.ReadFile(filepath.Join(issue.ArtifactDir, "events.log"))
	if err != nil {
		t.Fatalf("expected events.log in issue dir: %v", err)
	}
	events := string(eventsData)
	for _, needle := range []string{
		"issue started",
		"workspace created",
		"init_command completed",
		"repo discovered",
		"planning started",
		"coding loop=1 completed",
		"issue completed",
	} {
		if !strings.Contains(events, needle) {
			t.Fatalf("expected events.log to contain %q, got %s", needle, events)
		}
	}
	items, err := LoadFeatureList(issue.ArtifactDir)
	if err != nil {
		t.Fatalf("LoadFeatureList: %v", err)
	}
	if !AllFeaturesPassed(items) {
		t.Fatalf("expected all features passed")
	}
}

func TestOrchestratorFailsWhenPlanningDoesNotWriteArtifacts(t *testing.T) {
	requireGit(t)

	root := t.TempDir()
	store := NewStore(filepath.Join(root, ".relay"))
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	pipeline, issue := testRunInput()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	orchestrator := NewOrchestrator(store, ZshRunner{}, &fakeAgentRunner{
		t: t,
		plan: func(req AgentRunRequest) {
			_ = req
		},
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}

	issue, err = orchestrator.RunIssue(context.Background(), pipeline, issue)
	if err == nil {
		t.Fatalf("expected failure")
	}
	if issue.Status != IssueStatusFailed {
		t.Fatalf("expected failed issue, got %q", issue.Status)
	}
	eventsData, readErr := os.ReadFile(filepath.Join(issue.ArtifactDir, "events.log"))
	if readErr != nil {
		t.Fatalf("read events.log: %v", readErr)
	}
	events := string(eventsData)
	for _, needle := range []string{
		"planning started",
		"planning validation failed",
		"issue failed",
	} {
		if !strings.Contains(events, needle) {
			t.Fatalf("expected failure events to contain %q, got %s", needle, events)
		}
	}
}

func TestOrchestratorStopsAfterCurrentLoopWhenInterruptRequested(t *testing.T) {
	requireGit(t)

	root := t.TempDir()
	store := NewStore(filepath.Join(root, ".relay"))
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	pipeline, issue := testRunInput()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	orchestrator := NewOrchestrator(store, ZshRunner{}, &fakeAgentRunner{
		t: t,
		plan: func(req AgentRunRequest) {
			artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
			writeFeatureList(t, artifactDir, []FeatureItem{
				{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: false},
				{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: false},
			})
			appendProgress(t, artifactDir, "planning complete")
		},
		coding: []func(AgentRunRequest){
			func(req AgentRunRequest) {
				artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
				writeFeatureList(t, artifactDir, []FeatureItem{
					{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: true},
					{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: false},
				})
				appendProgress(t, artifactDir, "loop 1 complete")
				writeRepoChangeAndCommit(t, req.RepoPath, "first.txt", "loop 1\n", "feat: finish first feature")

				latest, err := store.LoadIssue(issue.ID)
				if err != nil {
					t.Fatalf("load issue: %v", err)
				}
				latest.InterruptRequested = true
				latest.LastError = "interrupt requested by user"
				if err := store.SaveIssue(latest); err != nil {
					t.Fatalf("save interrupt request: %v", err)
				}
			},
		},
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}

	issue, err = orchestrator.RunIssue(context.Background(), pipeline, issue)
	if err != nil {
		t.Fatalf("RunIssue failed: %v", err)
	}
	if issue.Status != IssueStatusInterrupted {
		t.Fatalf("expected interrupted issue, got %q", issue.Status)
	}
	if issue.CurrentLoop != 1 {
		t.Fatalf("expected interrupt after loop 1, got %d", issue.CurrentLoop)
	}
	if issue.InterruptRequested {
		t.Fatalf("expected interrupt request to be cleared")
	}
}

func TestOrchestratorContinuesAfterCodingLoopError(t *testing.T) {
	requireGit(t)

	root := t.TempDir()
	store := NewStore(filepath.Join(root, ".relay"))
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	pipeline, issue := testRunInput()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	orchestrator := NewOrchestrator(store, ZshRunner{}, &fakeAgentRunner{
		t: t,
		plan: func(req AgentRunRequest) {
			artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
			writeFeatureList(t, artifactDir, []FeatureItem{
				{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: false},
				{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: false},
			})
			appendProgress(t, artifactDir, "planning complete")
		},
		coding: []func(AgentRunRequest){
			func(req AgentRunRequest) {
				artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
				appendProgress(t, artifactDir, "loop 1 failed before completing work")
			},
			func(req AgentRunRequest) {
				artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
				writeFeatureList(t, artifactDir, []FeatureItem{
					{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: true},
					{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: true},
				})
				appendProgress(t, artifactDir, "loop 2 complete")
				writeRepoChangeAndCommit(t, req.RepoPath, "second.txt", "loop 2\n", "feat: finish all features")
			},
		},
		codingErrs: []error{
			errors.New("runner crashed"),
			nil,
		},
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}

	issue, err = orchestrator.RunIssue(context.Background(), pipeline, issue)
	if err != nil {
		t.Fatalf("RunIssue failed: %v", err)
	}
	if issue.Status != IssueStatusDone {
		t.Fatalf("expected done issue, got %q", issue.Status)
	}
	if issue.CurrentLoop != 2 {
		t.Fatalf("expected 2 loops after first loop error, got %d", issue.CurrentLoop)
	}

	eventsData, readErr := os.ReadFile(filepath.Join(issue.ArtifactDir, "events.log"))
	if readErr != nil {
		t.Fatalf("read events.log: %v", readErr)
	}
	events := string(eventsData)
	for _, needle := range []string{
		"coding loop=1 failed: runner crashed",
		"coding loop=1 aborted; advancing to next loop",
		"coding loop=2 completed done=true",
		"issue completed loop=2",
	} {
		if !strings.Contains(events, needle) {
			t.Fatalf("expected events.log to contain %q, got %s", needle, events)
		}
	}
}

func TestOrchestratorContinuesAfterCodingLoopPanic(t *testing.T) {
	requireGit(t)

	root := t.TempDir()
	store := NewStore(filepath.Join(root, ".relay"))
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	pipeline, issue := testRunInput()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	orchestrator := NewOrchestrator(store, ZshRunner{}, &fakeAgentRunner{
		t: t,
		plan: func(req AgentRunRequest) {
			artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
			writeFeatureList(t, artifactDir, []FeatureItem{
				{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: false},
				{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: false},
			})
			appendProgress(t, artifactDir, "planning complete")
		},
		coding: []func(AgentRunRequest){
			func(req AgentRunRequest) {
				artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
				appendProgress(t, artifactDir, "loop 1 panicked")
				panic("runner panic")
			},
			func(req AgentRunRequest) {
				artifactDir := filepath.Dir(mustExtractPromptPath(t, req.Prompt, "FEATURE_LIST_PATH="))
				writeFeatureList(t, artifactDir, []FeatureItem{
					{ID: "F-1", Title: "bootstrap", Description: "bootstrap repo", Priority: 1, Passes: true},
					{ID: "F-2", Title: "finish", Description: "finish issue", Priority: 2, Passes: true},
				})
				appendProgress(t, artifactDir, "loop 2 complete")
				writeRepoChangeAndCommit(t, req.RepoPath, "second.txt", "loop 2\n", "feat: finish all features")
			},
		},
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}

	issue, err = orchestrator.RunIssue(context.Background(), pipeline, issue)
	if err != nil {
		t.Fatalf("RunIssue failed: %v", err)
	}
	if issue.Status != IssueStatusDone {
		t.Fatalf("expected done issue, got %q", issue.Status)
	}
	if issue.CurrentLoop != 2 {
		t.Fatalf("expected 2 loops after first loop panic, got %d", issue.CurrentLoop)
	}

	eventsData, readErr := os.ReadFile(filepath.Join(issue.ArtifactDir, "events.log"))
	if readErr != nil {
		t.Fatalf("read events.log: %v", readErr)
	}
	events := string(eventsData)
	for _, needle := range []string{
		"coding loop=1 failed: panic: runner panic",
		"coding loop=1 aborted; advancing to next loop",
		"coding loop=2 completed done=true",
	} {
		if !strings.Contains(events, needle) {
			t.Fatalf("expected events.log to contain %q, got %s", needle, events)
		}
	}
}

func TestDiscoverRepoRootFallsBackToSingleGitDirectory(t *testing.T) {
	workspace := t.TempDir()
	repoPath := filepath.Join(workspace, "app")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	found, err := DiscoverRepoRoot(context.Background(), workspace)
	if err != nil {
		t.Fatalf("DiscoverRepoRoot failed: %v", err)
	}
	if found != repoPath {
		t.Fatalf("expected %s, got %s", repoPath, found)
	}
}

func TestDiscoverRepoRootRejectsMultipleRepos(t *testing.T) {
	workspace := t.TempDir()
	for _, name := range []string{"app1", "app2"} {
		if err := os.MkdirAll(filepath.Join(workspace, name, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir %s/.git: %v", name, err)
		}
	}
	if _, err := DiscoverRepoRoot(context.Background(), workspace); err == nil {
		t.Fatalf("expected multiple repo error")
	}
}

type fakeAgentRunner struct {
	t      *testing.T
	plan   func(req AgentRunRequest)
	coding []func(req AgentRunRequest)
	planErr error
	codingErrs []error
	index  int
}

func (f *fakeAgentRunner) Run(_ context.Context, req AgentRunRequest) (AgentRunResult, error) {
	switch req.Phase {
	case "plan":
		if f.plan == nil {
			f.t.Fatalf("unexpected plan run")
		}
		f.plan(req)
		if f.planErr != nil {
			return AgentRunResult{}, f.planErr
		}
	case "coding":
		if f.index >= len(f.coding) {
			f.t.Fatalf("unexpected coding run %d", f.index)
		}
		index := f.index
		f.index++
		f.coding[index](req)
		if index < len(f.codingErrs) && f.codingErrs[index] != nil {
			err := f.codingErrs[index]
			return AgentRunResult{}, err
		}
	default:
		f.t.Fatalf("unexpected phase %q", req.Phase)
	}
	return AgentRunResult{Stdout: "ok", FinalMessage: "done"}, nil
}

func testRunInput() (Pipeline, Issue) {
	pipeline := Pipeline{
		Name:         "pipe-1",
		InitCommand:  "mkdir repo && cd repo && git init && git config user.email relay@example.com && git config user.name relay && printf 'init\\n' > README.md && git add README.md && git commit -m init",
		LoopNum:      3,
		PlanPrompt:   "plan {{issue}}",
		CodingPrompt: "code {{issue}}",
	}
	if err := pipeline.Normalize(); err != nil {
		panic(err)
	}
	issue := Issue{
		ID:           "issue-1",
		PipelineName: pipeline.Name,
		Goal:         "implement relay",
		Description:  "testing",
		Status:       IssueStatusTodo,
	}
	return pipeline, issue
}

func writeFeatureList(t *testing.T, artifactDir string, items []FeatureItem) {
	t.Helper()
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatalf("marshal feature list: %v", err)
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(FeatureListPath(artifactDir), data, 0o644); err != nil {
		t.Fatalf("write feature list: %v", err)
	}
}

func appendProgress(t *testing.T, artifactDir, text string) {
	t.Helper()
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	file, err := os.OpenFile(ProgressPath(artifactDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open progress: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(text + "\n"); err != nil {
		t.Fatalf("append progress: %v", err)
	}
}

func writeRepoChangeAndCommit(t *testing.T, repoPath, fileName, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoPath, fileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}
	runGit(t, repoPath, "add", fileName)
	runGit(t, repoPath, "commit", "-m", message)
}

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

func runGit(t *testing.T, repoPath string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}
