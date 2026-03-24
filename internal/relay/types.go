package relay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultLoopNum = 15

	IssueStatusTodo        = "todo"
	IssueStatusPlanning    = "planning"
	IssueStatusRunning     = "running"
	IssueStatusDone        = "done"
	IssueStatusFailed      = "failed"
	IssueStatusInterrupted = "interrupted"
	IssueStatusDeleted     = "deleted"
)

type Pipeline struct {
	Name         string `json:"name" yaml:"name"`
	InitCommand  string `json:"init_command" yaml:"init_command"`
	LoopNum      int    `json:"loop_num" yaml:"loop_num"`
	PlanPrompt   string `json:"plan_prompt" yaml:"plan_prompt"`
	CodingPrompt string `json:"coding_prompt" yaml:"coding_prompt"`
}

type Issue struct {
	ID                 string `json:"id"`
	PipelineName       string `json:"pipeline_name"`
	Goal               string `json:"goal"`
	Description        string `json:"description"`
	Status             string `json:"status"`
	CurrentLoop        int    `json:"current_loop"`
	ArtifactDir        string `json:"artifact_dir"`
	WorkspacePath      string `json:"workspace_path"`
	RepoPath           string `json:"repo_path"`
	ActivePhase        string `json:"active_phase"`
	ActivePIDs         []int  `json:"active_pids"`
	LastError          string `json:"last_error"`
	InterruptRequested bool   `json:"interrupt_requested"`
}

type FeatureItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Passes      bool   `json:"passes"`
	Notes       string `json:"notes"`
}

func (p *Pipeline) Normalize() error {
	if p.LoopNum <= 0 {
		p.LoopNum = DefaultLoopNum
	}
	return p.Validate()
}

func (p Pipeline) Validate() error {
	if p.Name == "" {
		return errors.New("pipeline.name is required")
	}
	if strings.ContainsAny(p.Name, `/\`) {
		return fmt.Errorf("pipeline.name %q must not contain path separators", p.Name)
	}
	if p.InitCommand == "" {
		return errors.New("pipeline.init_command is required")
	}
	if p.PlanPrompt == "" {
		return errors.New("pipeline.plan_prompt is required")
	}
	if p.CodingPrompt == "" {
		return errors.New("pipeline.coding_prompt is required")
	}
	if p.LoopNum <= 0 {
		return fmt.Errorf("pipeline.loop_num must be positive, got %d", p.LoopNum)
	}
	return nil
}

func (i *Issue) Normalize() error {
	if i.ID == "" {
		i.ID = generateIssueID()
	}
	if i.Status == "" {
		i.Status = IssueStatusTodo
	}
	return i.ValidateForCreate()
}

func (i Issue) ValidateForCreate() error {
	if i.ID == "" {
		return errors.New("issue.id is required")
	}
	if i.PipelineName == "" {
		return errors.New("issue.pipeline_name is required")
	}
	if i.Goal == "" {
		return errors.New("issue.goal is required")
	}
	if i.Status != IssueStatusTodo {
		return fmt.Errorf("issue.status must start as %q, got %q", IssueStatusTodo, i.Status)
	}
	return nil
}

func (i Issue) TemplateJSON() string {
	payload := struct {
		ID                 string `json:"id"`
		PipelineName       string `json:"pipeline_name"`
		Goal               string `json:"goal"`
		Description        string `json:"description"`
		Status             string `json:"status"`
		CurrentLoop        int    `json:"current_loop"`
		ArtifactDir        string `json:"artifact_dir"`
		WorkspacePath      string `json:"workspace_path"`
		RepoPath           string `json:"repo_path"`
		ActivePhase        string `json:"active_phase"`
		ActivePIDs         []int  `json:"active_pids"`
		InterruptRequested bool   `json:"interrupt_requested"`
	}{
		ID:                 i.ID,
		PipelineName:       i.PipelineName,
		Goal:               i.Goal,
		Description:        i.Description,
		Status:             i.Status,
		CurrentLoop:        i.CurrentLoop,
		ArtifactDir:        i.ArtifactDir,
		WorkspacePath:      i.WorkspacePath,
		RepoPath:           i.RepoPath,
		ActivePhase:        i.ActivePhase,
		ActivePIDs:         append([]int(nil), i.ActivePIDs...),
		InterruptRequested: i.InterruptRequested,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func generateIssueID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "issue-unknown"
	}
	return "issue-" + hex.EncodeToString(buf)
}

func IsIssueActiveStatus(status string) bool {
	return status == IssueStatusPlanning || status == IssueStatusRunning
}

func IsIssueTerminalStatus(status string) bool {
	switch status {
	case IssueStatusDone, IssueStatusFailed, IssueStatusInterrupted, IssueStatusDeleted:
		return true
	default:
		return false
	}
}
