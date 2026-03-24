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
	"sync"
	"syscall"

	acp "github.com/coder/acp-go-sdk"
)

type AgentRunRequest struct {
	Phase    string
	RepoPath string
	Prompt   string
	IssueID  string
	LoopID   string
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

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return AgentRunResult{}, fmt.Errorf("open runner stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return AgentRunResult{}, fmt.Errorf("open runner stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return AgentRunResult{}, fmt.Errorf("start codex acp bridge: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	client := newACPClient(req)
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	if _, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs:       acp.FileSystemCapability{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
		},
	}); err != nil {
		return client.result(stderr.String(), exitCodeFromErr(err)), fmt.Errorf("initialize codex acp: %w", err)
	}
	session, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        req.RepoPath,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return client.result(stderr.String(), exitCodeFromErr(err)), fmt.Errorf("create codex acp session: %w", err)
	}
	if _, err := conn.Prompt(ctx, acp.PromptRequest{
		SessionId: session.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(req.Prompt)},
	}); err != nil {
		return client.result(stderr.String(), exitCodeFromErr(err)), fmt.Errorf("run codex acp prompt: %w", err)
	}
	return client.result(stderr.String(), 0), nil
}

func (r CodexRunner) commandSpec() (commandSpec, error) {
	if r.Command != "" {
		return commandSpec{Command: r.Command, Args: append([]string(nil), r.Args...)}, nil
	}
	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if bridgePath, err := lookPath("codex-acp"); err == nil {
		return commandSpec{Command: bridgePath}, nil
	}
	if _, err := lookPath("npx"); err == nil {
		return commandSpec{
			Command: "npx",
			Args:    []string{"-y", "@zed-industries/codex-acp@latest"},
		}, nil
	}
	return commandSpec{}, errors.New("codex ACP bridge not found: install codex-acp or make npx available for @zed-industries/codex-acp")
}

type commandSpec struct {
	Command string
	Args    []string
}

type relayACPClient struct {
	req       AgentRunRequest
	terminals *terminalStore

	mu           sync.Mutex
	stdout       bytes.Buffer
	finalMessage bytes.Buffer
}

func newACPClient(req AgentRunRequest) *relayACPClient {
	return &relayACPClient{
		req:       req,
		terminals: newTerminalStore(),
	}
}

func (c *relayACPClient) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	data, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	content := string(data)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = *params.Line - 1
			if start < 0 {
				start = 0
			}
			if start > len(lines) {
				start = len(lines)
			}
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 && start+*params.Limit < end {
			end = start + *params.Limit
		}
		content = strings.Join(lines[start:end], "\n")
	}
	if content == "" {
		content = "\n"
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

func (c *relayACPClient) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	return acp.WriteTextFileResponse{}, os.WriteFile(params.Path, []byte(params.Content), 0o644)
}

func (c *relayACPClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if len(params.Options) == 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Cancelled: &acp.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Selected: &acp.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId},
		},
	}, nil
}

func (c *relayACPClient) SessionUpdate(_ context.Context, notification acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	update := notification.Update
	switch {
	case update.AgentMessageChunk != nil:
		if block := update.AgentMessageChunk.Content.Text; block != nil {
			c.stdout.WriteString(block.Text)
			c.finalMessage.WriteString(block.Text)
		}
	case update.AgentThoughtChunk != nil:
		if block := update.AgentThoughtChunk.Content.Text; block != nil {
			c.stdout.WriteString(block.Text)
		}
	case update.ToolCall != nil:
		fmt.Fprintf(&c.stdout, "\n[tool] %s (%s)\n", update.ToolCall.Title, update.ToolCall.Status)
	case update.ToolCallUpdate != nil:
		fmt.Fprintf(&c.stdout, "\n[tool] %s -> %v\n", update.ToolCallUpdate.ToolCallId, update.ToolCallUpdate.Status)
	}
	return nil
}

func (c *relayACPClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return c.terminals.create(ctx, params)
}

func (c *relayACPClient) KillTerminalCommand(_ context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return c.terminals.kill(params.TerminalId)
}

func (c *relayACPClient) TerminalOutput(_ context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return c.terminals.output(params.TerminalId)
}

func (c *relayACPClient) ReleaseTerminal(_ context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return c.terminals.release(params.TerminalId)
}

func (c *relayACPClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return c.terminals.wait(ctx, params.TerminalId)
}

func (c *relayACPClient) result(stderr string, exitCode int) AgentRunResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return AgentRunResult{
		Stdout:       c.stdout.String(),
		Stderr:       stderr,
		ExitCode:     exitCode,
		FinalMessage: c.finalMessage.String(),
	}
}

type terminalStore struct {
	mu        sync.Mutex
	nextID    int
	terminals map[string]*terminalProcess
}

func newTerminalStore() *terminalStore {
	return &terminalStore{terminals: map[string]*terminalProcess{}}
}

func (s *terminalStore) create(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	s.mu.Lock()
	s.nextID++
	terminalID := fmt.Sprintf("term-%d", s.nextID)
	s.mu.Unlock()

	cmd := exec.CommandContext(ctx, params.Command, params.Args...)
	if params.Cwd != nil {
		cmd.Dir = *params.Cwd
	}
	cmd.Env = os.Environ()
	for _, env := range params.Env {
		cmd.Env = append(cmd.Env, env.Name+"="+env.Value)
	}

	limit := 64 * 1024
	if params.OutputByteLimit != nil && *params.OutputByteLimit > 0 {
		limit = *params.OutputByteLimit
	}
	writer := newLimitedTailBuffer(limit)
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return acp.CreateTerminalResponse{}, err
	}

	process := &terminalProcess{
		cmd:    cmd,
		output: writer,
		done:   make(chan struct{}),
	}
	go process.wait()

	s.mu.Lock()
	s.terminals[terminalID] = process
	s.mu.Unlock()

	return acp.CreateTerminalResponse{TerminalId: terminalID}, nil
}

func (s *terminalStore) kill(terminalID string) (acp.KillTerminalCommandResponse, error) {
	process, err := s.lookup(terminalID)
	if err != nil {
		return acp.KillTerminalCommandResponse{}, err
	}
	if process.cmd.Process != nil {
		if err := process.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return acp.KillTerminalCommandResponse{}, err
		}
	}
	return acp.KillTerminalCommandResponse{}, nil
}

func (s *terminalStore) output(terminalID string) (acp.TerminalOutputResponse, error) {
	process, err := s.lookup(terminalID)
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}
	exitStatus := process.exitStatus()
	output := process.output.String()
	if output == "" {
		output = "\n"
	}
	return acp.TerminalOutputResponse{
		Output:     output,
		Truncated:  process.output.Truncated(),
		ExitStatus: exitStatus,
	}, nil
}

func (s *terminalStore) release(terminalID string) (acp.ReleaseTerminalResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.terminals, terminalID)
	return acp.ReleaseTerminalResponse{}, nil
}

func (s *terminalStore) wait(ctx context.Context, terminalID string) (acp.WaitForTerminalExitResponse, error) {
	process, err := s.lookup(terminalID)
	if err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}
	select {
	case <-ctx.Done():
		return acp.WaitForTerminalExitResponse{}, ctx.Err()
	case <-process.done:
		return process.waitResponse(), nil
	}
}

func (s *terminalStore) lookup(terminalID string) (*terminalProcess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	process, ok := s.terminals[terminalID]
	if !ok {
		return nil, fmt.Errorf("unknown terminal: %s", terminalID)
	}
	return process, nil
}

type terminalProcess struct {
	cmd    *exec.Cmd
	output *limitedTailBuffer
	done   chan struct{}

	mu       sync.Mutex
	exitCode *int
	signal   *string
}

func (p *terminalProcess) wait() {
	defer close(p.done)
	err := p.cmd.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	if err == nil {
		code := 0
		p.exitCode = &code
		return
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		p.exitCode = &code
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			sig := status.Signal().String()
			p.signal = &sig
		}
		return
	}
	code := -1
	p.exitCode = &code
}

func (p *terminalProcess) exitStatus() *acp.TerminalExitStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exitCode == nil && p.signal == nil {
		return nil
	}
	return &acp.TerminalExitStatus{
		ExitCode: p.exitCode,
		Signal:   p.signal,
	}
}

func (p *terminalProcess) waitResponse() acp.WaitForTerminalExitResponse {
	p.mu.Lock()
	defer p.mu.Unlock()
	return acp.WaitForTerminalExitResponse{
		ExitCode: p.exitCode,
		Signal:   p.signal,
	}
}

type limitedTailBuffer struct {
	limit     int
	truncated bool

	mu  sync.Mutex
	buf []byte
}

func newLimitedTailBuffer(limit int) *limitedTailBuffer {
	return &limitedTailBuffer{limit: limit}
}

func (b *limitedTailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.limit {
		b.truncated = true
		b.buf = append([]byte(nil), b.buf[len(b.buf)-b.limit:]...)
	}
	return len(p), nil
}

func (b *limitedTailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.buf...))
}

func (b *limitedTailBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
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
