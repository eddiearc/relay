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

func TestPipelineNormalizeAcceptsAgentRunnerValues(t *testing.T) {
	for _, value := range []string{"", AgentRunnerCodex, AgentRunnerClaude} {
		t.Run("agent_runner="+value, func(t *testing.T) {
			pipeline := Pipeline{
				Name:         "demo",
				InitCommand:  "git init repo",
				LoopNum:      2,
				PlanPrompt:   "plan",
				CodingPrompt: "code",
				AgentRunner:  value,
			}
			if err := pipeline.Normalize(); err != nil {
				t.Fatalf("Normalize: %v", err)
			}
		})
	}
}

func TestPipelineNormalizeRejectsInvalidAgentRunner(t *testing.T) {
	pipeline := Pipeline{
		Name:         "demo",
		InitCommand:  "git init repo",
		LoopNum:      2,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
		AgentRunner:  "cursor",
	}
	err := pipeline.Normalize()
	if err == nil {
		t.Fatal("expected invalid agent runner to fail")
	}
	if !strings.Contains(err.Error(), "pipeline.agent_runner") {
		t.Fatalf("expected pipeline.agent_runner error, got %v", err)
	}
}

func TestLoadPipelineRejectsInvalidAgentRunner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipeline.yaml")
	data := `name: demo
init_command: git clone repo .
loop_num: 2
agent_runner: cursor
plan_prompt: plan
coding_prompt: code
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write pipeline: %v", err)
	}

	_, err := LoadPipeline(path)
	if err == nil {
		t.Fatal("expected invalid agent_runner to fail")
	}
	if !strings.Contains(err.Error(), "pipeline.agent_runner") {
		t.Fatalf("expected error to mention pipeline.agent_runner, got %v", err)
	}
}

func TestIssueNormalizeAcceptsAgentRunnerValues(t *testing.T) {
	for _, value := range []string{"", AgentRunnerCodex, AgentRunnerClaude} {
		t.Run("agent_runner="+value, func(t *testing.T) {
			issue := Issue{
				PipelineName: "demo",
				Goal:         "ship",
				AgentRunner:  value,
			}
			if err := issue.Normalize(); err != nil {
				t.Fatalf("Normalize: %v", err)
			}
		})
	}
}

func TestIssueNormalizeRejectsInvalidAgentRunner(t *testing.T) {
	issue := Issue{
		PipelineName: "demo",
		Goal:         "ship",
		AgentRunner:  "cursor",
	}
	err := issue.Normalize()
	if err == nil {
		t.Fatal("expected invalid agent runner to fail")
	}
	if !strings.Contains(err.Error(), "issue.agent_runner") {
		t.Fatalf("expected issue.agent_runner error, got %v", err)
	}
}

func TestLoadIssueRejectsInvalidAgentRunner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issue.json")
	data := `{
  "id": "issue-invalid-runner",
  "pipeline_name": "demo",
  "goal": "ship",
  "description": "desc",
  "agent_runner": "cursor"
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write issue: %v", err)
	}

	_, err := LoadIssue(path)
	if err == nil {
		t.Fatal("expected invalid agent_runner to fail")
	}
	if !strings.Contains(err.Error(), "issue.agent_runner") {
		t.Fatalf("expected error to mention issue.agent_runner, got %v", err)
	}
}

func TestIssueTemplateJSONPreservesAgentRunner(t *testing.T) {
	issue := Issue{
		ID:           "issue-1",
		PipelineName: "demo",
		Goal:         "ship",
		Description:  "desc",
		Status:       IssueStatusRunning,
		AgentRunner:  AgentRunnerClaude,
	}
	if !strings.Contains(issue.TemplateJSON(), `"agent_runner": "claude"`) {
		t.Fatalf("expected TemplateJSON to include agent_runner, got %s", issue.TemplateJSON())
	}
}
