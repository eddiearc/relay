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

type ShellRunner interface {
	Run(ctx context.Context, workdir, command string) (CommandResult, error)
}

type ZshRunner struct{}

func (ZshRunner) Run(ctx context.Context, workdir, command string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, "zsh", "-lc", command)
	cmd.Dir = workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
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
	return result, fmt.Errorf("run command %q: %w", command, err)
}
