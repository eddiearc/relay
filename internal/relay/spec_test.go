package relay

import "testing"

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
