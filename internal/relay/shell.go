package relay

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
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
	cmd := exec.CommandContext(ctx, "zsh", "-lc", req.Command)
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
	err := cmd.Wait()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
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
