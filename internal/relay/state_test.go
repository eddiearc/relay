package relay

import (
	"os"
	"path/filepath"
	"testing"
)

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
