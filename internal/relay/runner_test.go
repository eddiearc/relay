package relay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestCodexRunnerCommandSpecPrefersCodexACPBinary(t *testing.T) {
	runner := CodexRunner{
		LookPath: func(name string) (string, error) {
			switch name {
			case "codex-acp":
				return "/usr/local/bin/codex-acp", nil
			default:
				return "", errors.New("not found")
			}
		},
	}

	spec, err := runner.commandSpec()
	if err != nil {
		t.Fatalf("commandSpec: %v", err)
	}
	if spec.Command != "/usr/local/bin/codex-acp" {
		t.Fatalf("expected codex-acp binary, got %+v", spec)
	}
}

func TestCodexRunnerCommandSpecFallsBackToNpx(t *testing.T) {
	runner := CodexRunner{
		LookPath: func(name string) (string, error) {
			switch name {
			case "npx":
				return "/usr/bin/npx", nil
			default:
				return "", errors.New("not found")
			}
		},
	}

	spec, err := runner.commandSpec()
	if err != nil {
		t.Fatalf("commandSpec: %v", err)
	}
	if spec.Command != "npx" {
		t.Fatalf("expected npx fallback, got %+v", spec)
	}
	if len(spec.Args) < 2 || spec.Args[1] != "@zed-industries/codex-acp@latest" {
		t.Fatalf("unexpected npx args: %+v", spec.Args)
	}
}

func TestCodexRunnerRunViaACPBridge(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "agent-output.txt")
	t.Setenv("GO_WANT_HELPER_ACP_AGENT", "1")
	t.Setenv("ACP_TEST_FILE", testFile)

	runner := CodexRunner{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperACPAgentProcess", "--"},
	}
	result, err := runner.Run(context.Background(), AgentRunRequest{
		Phase:    "coding",
		RepoPath: t.TempDir(),
		Prompt:   "hello from relay",
		IssueID:  "issue-1",
		LoopID:   "loop-01",
	})
	if err != nil {
		t.Fatalf("Run: %v\nstderr=%s", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected zero exit code, got %d", result.ExitCode)
	}
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("read ACP-written file: %v", err)
	}
	if string(data) != "written by helper agent" {
		t.Fatalf("unexpected ACP-written file contents: %q", string(data))
	}
}

func TestHelperACPAgentProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_ACP_AGENT") != "1" {
		return
	}

	agent := &helperACPAgent{testFile: os.Getenv("ACP_TEST_FILE")}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

type helperACPAgent struct {
	conn     *acp.AgentSideConnection
	testFile string
}

func (a *helperACPAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *helperACPAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: false,
		},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

func (a *helperACPAgent) Cancel(context.Context, acp.CancelNotification) error {
	return nil
}

func (a *helperACPAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "session-1"}, nil
}

func (a *helperACPAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	if _, err := a.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
		SessionId: params.SessionId,
		Path:      a.testFile,
		Content:   "written by helper agent",
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	term, err := a.conn.CreateTerminal(ctx, acp.CreateTerminalRequest{
		SessionId: params.SessionId,
		Command:   "/bin/sh",
		Args:      []string{"-c", "printf terminal-ok"},
		Cwd:       acp.Ptr("/"),
	})
	if err != nil {
		return acp.PromptResponse{}, err
	}
	waitResp, err := a.conn.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{
		SessionId:  params.SessionId,
		TerminalId: term.TerminalId,
	})
	if err != nil {
		return acp.PromptResponse{}, err
	}
	out, err := a.conn.TerminalOutput(ctx, acp.TerminalOutputRequest{
		SessionId:  params.SessionId,
		TerminalId: term.TerminalId,
	})
	if err != nil {
		return acp.PromptResponse{}, err
	}
	_ = waitResp
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: params.SessionId,
		Update:    acp.UpdateAgentMessageText("helper:" + out.Output),
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *helperACPAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

var _ acp.Agent = (*helperACPAgent)(nil)

func TestCodexRunnerCommandSpecErrorsWithoutBridge(t *testing.T) {
	runner := CodexRunner{
		LookPath: func(name string) (string, error) {
			return "", errors.New("not found")
		},
	}
	if _, err := runner.commandSpec(); err == nil {
		t.Fatalf("expected missing bridge error")
	}
}

func TestACPClientTerminalLifecycle(t *testing.T) {
	client := newACPClient(AgentRunRequest{})
	createResp, err := client.CreateTerminal(context.Background(), acp.CreateTerminalRequest{
		SessionId: "session-1",
		Command:   "/bin/sh",
		Args:      []string{"-c", "printf terminal-lifecycle"},
		Cwd:       acp.Ptr("/"),
	})
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if _, err := client.WaitForTerminalExit(context.Background(), acp.WaitForTerminalExitRequest{
		SessionId:  "session-1",
		TerminalId: createResp.TerminalId,
	}); err != nil {
		t.Fatalf("WaitForTerminalExit: %v", err)
	}
	output, err := client.TerminalOutput(context.Background(), acp.TerminalOutputRequest{
		SessionId:  "session-1",
		TerminalId: createResp.TerminalId,
	})
	if err != nil {
		t.Fatalf("TerminalOutput: %v", err)
	}
	if output.Output == "" {
		t.Fatalf("expected terminal output")
	}
	if _, err := client.ReleaseTerminal(context.Background(), acp.ReleaseTerminalRequest{
		SessionId:  "session-1",
		TerminalId: createResp.TerminalId,
	}); err != nil {
		t.Fatalf("ReleaseTerminal: %v", err)
	}
}
