package relay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AgentRunRequest struct {
	Phase   string
	Workdir string
	Prompt  string
	IssueID string
	LoopID  string
	OnPID   func(pid int)
}

type AgentRunResult struct {
	Stdout       string
	Stderr       string
	ExitCode     int
	FinalMessage string
}

type AgentRunner interface {
	Run(context.Context, AgentRunRequest) (AgentRunResult, error)
}

type CodexRunner struct {
	Command  string
	Args     []string
	LookPath func(string) (string, error)
}

func (r CodexRunner) Run(ctx context.Context, req AgentRunRequest) (AgentRunResult, error) {
	spec, err := r.commandSpec()
	if err != nil {
		return AgentRunResult{}, err
	}

	tempDir, err := os.MkdirTemp("", "relay-codex-run-*")
	if err != nil {
		return AgentRunResult{}, fmt.Errorf("create codex temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	finalMessagePath := filepath.Join(tempDir, "final-message.txt")
	args := append([]string{}, spec.Args...)
	args = append(args, "exec", "--dangerously-bypass-approvals-and-sandbox")
	if req.Workdir != "" {
		args = append(args, "-C", req.Workdir)
	}
	args = append(args, "-o", finalMessagePath, "-")

	cmd := exec.CommandContext(ctx, spec.Command, args...)
	if req.Workdir != "" {
		cmd.Dir = req.Workdir
	}
	cmd.Stdin = strings.NewReader(req.Prompt)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return AgentRunResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: -1,
		}, fmt.Errorf("start codex exec: %w", err)
	}
	if req.OnPID != nil && cmd.Process != nil {
		req.OnPID(cmd.Process.Pid)
	}

	err = cmd.Wait()
	result := AgentRunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCodeFromErr(err),
	}
	if data, readErr := os.ReadFile(finalMessagePath); readErr == nil {
		result.FinalMessage = string(data)
	}
	if err != nil {
		return result, fmt.Errorf("run codex exec: %w", err)
	}
	return result, nil
}

func (r CodexRunner) commandSpec() (commandSpec, error) {
	return resolveCommandSpec(r.Command, r.Args, "codex", r.LookPath, "codex CLI not found: install codex and make it available in PATH")
}

type ClaudeRunner struct {
	Command  string
	Args     []string
	LookPath func(string) (string, error)
}

func (r ClaudeRunner) Run(ctx context.Context, req AgentRunRequest) (AgentRunResult, error) {
	spec, err := r.commandSpec()
	if err != nil {
		return AgentRunResult{}, err
	}

	args := append([]string{}, spec.Args...)
	args = append(args, "-p", "--dangerously-skip-permissions", "--output-format", "text")

	cmd := exec.CommandContext(ctx, spec.Command, args...)
	if req.Workdir != "" {
		cmd.Dir = req.Workdir
	}
	cmd.Stdin = strings.NewReader(req.Prompt)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return AgentRunResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: -1,
		}, fmt.Errorf("start claude print: %w", err)
	}
	if req.OnPID != nil && cmd.Process != nil {
		req.OnPID(cmd.Process.Pid)
	}

	err = cmd.Wait()
	result := AgentRunResult{
		Stdout:       stdout.String(),
		Stderr:       stderr.String(),
		ExitCode:     exitCodeFromErr(err),
		FinalMessage: stdout.String(),
	}
	if err != nil {
		return result, fmt.Errorf("run claude print: %w", err)
	}
	return result, nil
}

func (r ClaudeRunner) commandSpec() (commandSpec, error) {
	return resolveCommandSpec(r.Command, r.Args, "claude", r.LookPath, "claude CLI not found: install claude and make it available in PATH")
}

type commandSpec struct {
	Command string
	Args    []string
}

func ResolveAgentRunner(issueRunner, pipelineRunner string) (string, error) {
	if err := validateAgentRunner("issue.agent_runner", issueRunner); err != nil {
		return "", err
	}
	if err := validateAgentRunner("pipeline.agent_runner", pipelineRunner); err != nil {
		return "", err
	}
	switch {
	case issueRunner != "":
		return issueRunner, nil
	case pipelineRunner != "":
		return pipelineRunner, nil
	default:
		return DefaultAgentRunner, nil
	}
}

func NewAgentRunner(name string) (AgentRunner, error) {
	if err := validateAgentRunner("agent_runner", name); err != nil {
		return nil, err
	}
	switch name {
	case "", AgentRunnerCodex:
		return CodexRunner{}, nil
	case AgentRunnerClaude:
		return ClaudeRunner{}, nil
	default:
		return nil, fmt.Errorf("unsupported agent_runner %q", name)
	}
}

func resolveCommandSpec(command string, args []string, binary string, lookPath func(string) (string, error), missingErr string) (commandSpec, error) {
	if command != "" {
		return commandSpec{
			Command: command,
			Args:    append([]string(nil), args...),
		}, nil
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	resolved, err := lookPath(binary)
	if err != nil {
		return commandSpec{}, errors.New(missingErr)
	}
	return commandSpec{
		Command: resolved,
		Args:    append([]string(nil), args...),
	}, nil
}

func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
