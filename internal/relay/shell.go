package relay

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	FinalDir string
}

type ShellRunRequest struct {
	Workdir string
	Command string
	OnStart func(pid int)
}

type ShellRunner interface {
	Run(ctx context.Context, req ShellRunRequest) (CommandResult, error)
}

type ZshRunner struct{}

func (ZshRunner) Run(ctx context.Context, req ShellRunRequest) (CommandResult, error) {
	finalDirFile, err := os.CreateTemp("", "relay-final-dir-*")
	if err != nil {
		return CommandResult{ExitCode: -1}, fmt.Errorf("create final-dir temp file: %w", err)
	}
	finalDirPath := finalDirFile.Name()
	if closeErr := finalDirFile.Close(); closeErr != nil {
		return CommandResult{ExitCode: -1}, fmt.Errorf("close final-dir temp file: %w", closeErr)
	}
	defer os.Remove(finalDirPath)

	wrapped := fmt.Sprintf("trap 'pwd > %s' EXIT\n%s", shSingleQuote(finalDirPath), req.Command)
	cmd := exec.CommandContext(ctx, "zsh", "-lc", wrapped)
	cmd.Dir = req.Workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: -1}, fmt.Errorf("run command %q: %w", req.Command, err)
	}
	if req.OnStart != nil && cmd.Process != nil {
		req.OnStart(cmd.Process.Pid)
	}
	err = cmd.Wait()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if data, readErr := os.ReadFile(finalDirPath); readErr == nil {
		result.FinalDir = strings.TrimSpace(string(data))
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result, fmt.Errorf("run command %q: %w", req.Command, err)
}

func shSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
