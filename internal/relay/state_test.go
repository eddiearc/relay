package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreDeleteIssueRemovesOnlyTargetedIssueState(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	issueOne := Issue{
		ID:           "issue-delete-one",
		PipelineName: "demo",
		Goal:         "delete one",
		Description:  "desc",
		Status:       IssueStatusInterrupted,
		WorkspacePath: filepath.Join(store.WorkspaceRoot,
			"issue-delete-one-1234abcd"),
		WorkdirPath: filepath.Join(store.WorkspaceRoot, "issue-delete-one-1234abcd", "repo"),
	}
	issueTwo := Issue{
		ID:           "issue-delete-two",
		PipelineName: "demo",
		Goal:         "keep two",
		Description:  "desc",
		Status:       IssueStatusDone,
		WorkspacePath: filepath.Join(store.WorkspaceRoot,
			"issue-delete-two-5678efgh"),
		WorkdirPath: filepath.Join(store.WorkspaceRoot, "issue-delete-two-5678efgh", "repo"),
	}
	for _, issue := range []Issue{issueOne, issueTwo} {
		if err := store.SaveIssue(issue); err != nil {
			t.Fatalf("SaveIssue(%s): %v", issue.ID, err)
		}
		if err := os.MkdirAll(issue.WorkdirPath, 0o755); err != nil {
			t.Fatalf("mkdir workdir for %s: %v", issue.ID, err)
		}
		if err := os.WriteFile(FeatureListPath(store.IssueDir(issue.ID)), []byte("[]\n"), 0o644); err != nil {
			t.Fatalf("write feature list for %s: %v", issue.ID, err)
		}
		if err := os.WriteFile(ProgressPath(store.IssueDir(issue.ID)), []byte("progress\n"), 0o644); err != nil {
			t.Fatalf("write progress for %s: %v", issue.ID, err)
		}
	}

	result, err := store.DeleteIssue(issueOne.ID)
	if err != nil {
		t.Fatalf("DeleteIssue: %v", err)
	}
	if !result.ArtifactRemoved {
		t.Fatalf("expected artifact dir removal, got %+v", result)
	}
	if !result.WorkspaceRemoved {
		t.Fatalf("expected workspace removal, got %+v", result)
	}
	if _, err := os.Stat(store.IssueDir(issueOne.ID)); !os.IsNotExist(err) {
		t.Fatalf("expected issue one artifact dir removed, err=%v", err)
	}
	if _, err := os.Stat(issueOne.WorkspacePath); !os.IsNotExist(err) {
		t.Fatalf("expected issue one workspace removed, err=%v", err)
	}
	if _, err := store.LoadIssue(issueTwo.ID); err != nil {
		t.Fatalf("expected issue two to remain loadable: %v", err)
	}
	if _, err := os.Stat(store.IssueDir(issueTwo.ID)); err != nil {
		t.Fatalf("expected issue two artifact dir to remain: %v", err)
	}
	if _, err := os.Stat(issueTwo.WorkspacePath); err != nil {
		t.Fatalf("expected issue two workspace to remain: %v", err)
	}
}

func TestStoreDeleteIssueIsIdempotent(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	issue := Issue{
		ID:            "issue-delete-idempotent",
		PipelineName:  "demo",
		Goal:          "delete twice",
		Description:   "desc",
		Status:        IssueStatusInterrupted,
		WorkspacePath: filepath.Join(store.WorkspaceRoot, "issue-delete-idempotent-a1b2c3d4"),
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("SaveIssue: %v", err)
	}
	if err := os.MkdirAll(issue.WorkspacePath, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	first, err := store.DeleteIssue(issue.ID)
	if err != nil {
		t.Fatalf("first DeleteIssue: %v", err)
	}
	if first.Missing {
		t.Fatalf("expected first delete to see persisted issue, got %+v", first)
	}

	second, err := store.DeleteIssue(issue.ID)
	if err != nil {
		t.Fatalf("second DeleteIssue: %v", err)
	}
	if !second.Missing {
		t.Fatalf("expected second delete to report missing issue state, got %+v", second)
	}
	if _, err := os.Stat(store.IssueDir(issue.ID)); !os.IsNotExist(err) {
		t.Fatalf("expected artifact dir to stay removed, err=%v", err)
	}
	if _, err := os.Stat(issue.WorkspacePath); !os.IsNotExist(err) {
		t.Fatalf("expected workspace to stay removed, err=%v", err)
	}
}

func TestStoreDeleteIssueLeavesExternalWorkspaceUntouched(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	externalWorkspace := filepath.Join(t.TempDir(), "issue-delete-external-unsafe")
	if err := os.MkdirAll(externalWorkspace, 0o755); err != nil {
		t.Fatalf("mkdir external workspace: %v", err)
	}
	issue := Issue{
		ID:            "issue-delete-external",
		PipelineName:  "demo",
		Goal:          "delete safely",
		Description:   "desc",
		Status:        IssueStatusDone,
		WorkspacePath: externalWorkspace,
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("SaveIssue: %v", err)
	}

	result, err := store.DeleteIssue(issue.ID)
	if err != nil {
		t.Fatalf("DeleteIssue: %v", err)
	}
	if result.WorkspaceRemoved {
		t.Fatalf("expected external workspace to be preserved, got %+v", result)
	}
	if _, err := os.Stat(externalWorkspace); err != nil {
		t.Fatalf("expected external workspace to remain: %v", err)
	}
	if _, err := os.Stat(store.IssueDir(issue.ID)); !os.IsNotExist(err) {
		t.Fatalf("expected artifact dir removed, err=%v", err)
	}
}

func TestStoreDeleteIssueRemovesStrayArtifactDirWhenIssueFileMissing(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "relay-workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	artifactDir := store.IssueDir("issue-missing-snapshot")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "events.log"), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write events.log: %v", err)
	}

	result, err := store.DeleteIssue("issue-missing-snapshot")
	if err != nil {
		t.Fatalf("DeleteIssue: %v", err)
	}
	if !result.Missing {
		t.Fatalf("expected missing issue snapshot to be reported, got %+v", result)
	}
	if !result.ArtifactRemoved {
		t.Fatalf("expected stray artifact dir to be removed, got %+v", result)
	}
	if _, err := os.Stat(artifactDir); !os.IsNotExist(err) {
		t.Fatalf("expected stray artifact dir removed, err=%v", err)
	}
}

func TestStoreSavesIssuesIntoPerIssueDirectories(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	issue := Issue{
		ID:           "issue-1",
		PipelineName: "demo",
		Goal:         "ship",
		Description:  "desc",
		Status:       IssueStatusTodo,
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("SaveIssue: %v", err)
	}

	path := filepath.Join(root, "issues", "issue-1", "issue.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected issue snapshot at %s: %v", path, err)
	}
	loaded, err := store.LoadIssue("issue-1")
	if err != nil {
		t.Fatalf("LoadIssue: %v", err)
	}
	if loaded.ArtifactDir != filepath.Join(root, "issues", "issue-1") {
		t.Fatalf("unexpected artifact dir: %s", loaded.ArtifactDir)
	}
}

func TestStoreListIssuesScansIssueDirectories(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	for _, issue := range []Issue{
		{ID: "issue-b", PipelineName: "demo", Goal: "b", Description: "desc", Status: IssueStatusRunning},
		{ID: "issue-a", PipelineName: "demo", Goal: "a", Description: "desc", Status: IssueStatusTodo},
	} {
		if err := store.SaveIssue(issue); err != nil {
			t.Fatalf("SaveIssue(%s): %v", issue.ID, err)
		}
	}

	issues, err := store.ListIssues()
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].ID != "issue-a" || issues[1].ID != "issue-b" {
		t.Fatalf("expected sorted issues, got %+v", issues)
	}
}

func TestStoreSavesPipelineAsYAML(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.SavePipeline(Pipeline{
		Name:         "demo",
		InitCommand:  "git init repo",
		LoopNum:      2,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	}); err != nil {
		t.Fatalf("SavePipeline: %v", err)
	}

	path := filepath.Join(root, "pipelines", "demo.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pipeline yaml: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected pipeline yaml to be written")
	}
}

func TestStoreSavesPipelineAgentRunnerToYAML(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.SavePipeline(Pipeline{
		Name:         "demo-runner",
		InitCommand:  "git init repo",
		LoopNum:      2,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
		AgentRunner:  AgentRunnerClaude,
	}); err != nil {
		t.Fatalf("SavePipeline: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "pipelines", "demo-runner.yaml"))
	if err != nil {
		t.Fatalf("read pipeline yaml: %v", err)
	}
	if !strings.Contains(string(data), "agent_runner: claude") {
		t.Fatalf("expected pipeline yaml to persist agent_runner, got %s", string(data))
	}
}

func TestStoreLoadIssueRejectsInvalidAgentRunner(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	issueDir := filepath.Join(root, "issues", "issue-invalid-runner")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("mkdir issue dir: %v", err)
	}
	data := `{
  "id": "issue-invalid-runner",
  "pipeline_name": "demo",
  "goal": "ship",
  "description": "desc",
  "status": "running",
  "agent_runner": "cursor"
}`
	if err := os.WriteFile(filepath.Join(issueDir, "issue.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write issue.json: %v", err)
	}

	_, err := store.LoadIssue("issue-invalid-runner")
	if err == nil {
		t.Fatal("expected invalid agent_runner to fail")
	}
	if !strings.Contains(err.Error(), "issue.agent_runner") {
		t.Fatalf("expected issue.agent_runner error, got %v", err)
	}
}

func TestStoreLoadIssueFixtures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		issueID   string
		fixture   string
		wantErr   string
		wantCheck func(t *testing.T, issue Issue, issueDir string)
	}{
		{
			name:    "valid",
			issueID: "issue-fixture",
			fixture: "issue_valid.json",
			wantCheck: func(t *testing.T, issue Issue, issueDir string) {
				t.Helper()
				if issue.ArtifactDir != issueDir {
					t.Fatalf("expected artifact dir %s, got %s", issueDir, issue.ArtifactDir)
				}
				if issue.Status != IssueStatusRunning {
					t.Fatalf("expected running status, got %s", issue.Status)
				}
				if issue.CurrentLoop != 2 {
					t.Fatalf("expected current loop 2, got %d", issue.CurrentLoop)
				}
				if issue.ActivePhase != "coding" {
					t.Fatalf("expected active phase coding, got %s", issue.ActivePhase)
				}
			},
		},
		{name: "missing-goal", issueID: "issue-missing-goal", fixture: "issue_invalid_missing_goal.json", wantErr: "issue.goal is required"},
		{name: "invalid-agent-runner", issueID: "issue-invalid-runner", fixture: "issue_invalid_agent_runner.json", wantErr: "issue.agent_runner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := NewStore(root)
			store.WorkspaceRoot = filepath.Join(root, "workspaces")
			if err := store.Ensure(); err != nil {
				t.Fatalf("Ensure: %v", err)
			}

			fixturePath := filepath.Join("testdata", tc.fixture)
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fixturePath, err)
			}
			issueDir := store.IssueDir(tc.issueID)
			if err := os.MkdirAll(issueDir, 0o755); err != nil {
				t.Fatalf("mkdir issue dir: %v", err)
			}
			if err := os.WriteFile(IssueFilePath(issueDir), data, 0o644); err != nil {
				t.Fatalf("write issue fixture: %v", err)
			}

			issue, err := store.LoadIssue(tc.issueID)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("LoadIssue: %v", err)
				}
				if tc.wantCheck != nil {
					tc.wantCheck(t, issue, issueDir)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestStoreConcurrentSaveAndLoadIssueDoesNotSeePartialJSON(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.WorkspaceRoot = filepath.Join(root, "workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	issue := Issue{
		ID:           "issue-concurrent",
		PipelineName: "demo",
		Goal:         "ship",
		Description:  strings.Repeat("payload-", 1<<17),
		Status:       IssueStatusRunning,
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("SaveIssue: %v", err)
	}

	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for loop := 0; loop < 200; loop++ {
			issue.CurrentLoop = loop
			if err := store.SaveIssue(issue); err != nil {
				errCh <- err
				return
			}
		}
	}()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case err := <-errCh:
			t.Fatalf("save issue: %v", err)
		case <-done:
			return
		case <-deadline:
			t.Fatal("timed out waiting for concurrent save loop")
		default:
			_, err := store.LoadIssue(issue.ID)
			if err == nil {
				continue
			}
			if strings.Contains(err.Error(), "parse issue.json") {
				t.Fatalf("saw partial issue.json during concurrent load: %v", err)
			}
		}
	}
}
