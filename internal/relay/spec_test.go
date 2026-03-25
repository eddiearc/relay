package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueCreateNormalizeDefaults(t *testing.T) {
	issue := Issue{
		PipelineName: "pipe",
		Goal:         "goal",
	}
	if err := issue.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if issue.Status != IssueStatusTodo {
		t.Fatalf("expected todo status, got %q", issue.Status)
	}
	if issue.PipelineName != "pipe" {
		t.Fatalf("expected pipeline name to stay set")
	}
}

func TestPipelineNormalizeDefaults(t *testing.T) {
	pipeline := Pipeline{
		Name:         "demo",
		InitCommand:  "git init repo",
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	}
	if err := pipeline.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if pipeline.LoopNum != DefaultLoopNum {
		t.Fatalf("expected default loop num %d, got %d", DefaultLoopNum, pipeline.LoopNum)
	}
}

func TestIssueNormalizeGeneratesID(t *testing.T) {
	issue := Issue{
		PipelineName: "demo",
		Goal:         "ship",
	}
	if err := issue.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if issue.ID == "" {
		t.Fatalf("expected generated issue id")
	}
	if issue.Status != IssueStatusTodo {
		t.Fatalf("expected todo status, got %q", issue.Status)
	}
}

func TestLoadPipelineRejectsUnknownProjectField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipeline.yaml")
	data := `name: demo
project:
  key: github.com/example/repo
init_command: git clone repo .
plan_prompt: plan
coding_prompt: code
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write pipeline: %v", err)
	}

	_, err := LoadPipeline(path)
	if err == nil {
		t.Fatal("expected unknown project field to fail")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("expected error to mention project field, got %v", err)
	}
}
