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
	Phase    string
	RepoPath string
	Prompt   string
	IssueID  string
	LoopID   string
	OnPID    func(pid int)
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
	if req.RepoPath != "" {
		args = append(args, "-C", req.RepoPath)
	}
	args = append(args, "-o", finalMessagePath, "-")

	cmd := exec.CommandContext(ctx, spec.Command, args...)
	if req.RepoPath != "" {
		cmd.Dir = req.RepoPath
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
	if r.Command != "" {
		return commandSpec{
			Command: r.Command,
			Args:    append([]string(nil), r.Args...),
		}, nil
	}
	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	command, err := lookPath("codex")
	if err != nil {
		return commandSpec{}, errors.New("codex CLI not found: install codex and make it available in PATH")
	}
	return commandSpec{
		Command: command,
		Args:    append([]string(nil), r.Args...),
	}, nil
}

type commandSpec struct {
	Command string
	Args    []string
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
