package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelaySubprocessIssueAddCreatesArtifacts(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	pipelineFile := filepath.Join(t.TempDir(), "pipeline.yaml")
	if err := os.WriteFile(pipelineFile, []byte(""+
		"name: demo-subprocess\n"+
		"init_command: git init repo\n"+
		"loop_num: 2\n"+
		"plan_prompt: plan\n"+
		"coding_prompt: code\n"), 0o644); err != nil {
		t.Fatalf("write pipeline yaml: %v", err)
	}

	importResult := runRelaySubprocess(t, "pipeline", "import", "-file", pipelineFile, "-state-dir", stateDir)
	if importResult.exitCode != 0 {
		t.Fatalf("pipeline import exit=%d stderr=%s", importResult.exitCode, importResult.stderr.String())
	}
	if !strings.Contains(importResult.stdout.String(), "pipeline demo-subprocess imported") {
		t.Fatalf("expected pipeline import stdout, got %s", importResult.stdout.String())
	}

	result := runRelaySubprocess(t,
		"issue", "add",
		"--id", "issue-subprocess",
		"--pipeline", "demo-subprocess",
		"--goal", "Verify subprocess command behavior",
		"--description", "Capture exit code, stdout, stderr, and issue.json side effects.",
		"-state-dir", stateDir,
	)
	if result.exitCode != 0 {
		t.Fatalf("issue add exit=%d stderr=%s", result.exitCode, result.stderr.String())
	}
	if !strings.Contains(result.stdout.String(), "\"id\": \"issue-subprocess\"") {
		t.Fatalf("expected issue json on stdout, got %s", result.stdout.String())
	}
	for _, want := range []string{
		"Process the queue once: relay serve --once -state-dir ",
		"Watch progress: relay watch -issue issue-subprocess -state-dir ",
	} {
		if !strings.Contains(result.stderr.String(), want) {
			t.Fatalf("expected stderr to contain %q, got %s", want, result.stderr.String())
		}
	}

	issuePath := filepath.Join(stateDir, "issues", "issue-subprocess", "issue.json")
	issueJSON, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue artifact: %v", err)
	}
	if !strings.Contains(string(issueJSON), "\"pipeline_name\": \"demo-subprocess\"") {
		t.Fatalf("expected persisted issue artifact, got %s", string(issueJSON))
	}
	if _, err := os.Stat(filepath.Join(stateDir, "pipelines", "demo-subprocess.yaml")); err != nil {
		t.Fatalf("expected imported pipeline to exist: %v", err)
	}
}

func TestRelaySubprocessStatusReadsIssueSnapshot(t *testing.T) {
	t.Parallel()

	stateDir := setupSubprocessIssue(t)
	result := runRelaySubprocess(t, "status", "-issue", "issue-subprocess", "-state-dir", stateDir)
	if result.exitCode != 0 {
		t.Fatalf("status exit=%d stderr=%s", result.exitCode, result.stderr.String())
	}
	if result.stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", result.stderr.String())
	}
	for _, want := range []string{
		"issue=issue-subprocess status=todo loop=0",
		"artifact=" + filepath.Join(stateDir, "issues", "issue-subprocess"),
	} {
		if !strings.Contains(result.stdout.String(), want) {
			t.Fatalf("expected stdout to contain %q, got %s", want, result.stdout.String())
		}
	}
}

func setupSubprocessIssue(t *testing.T) string {
	t.Helper()
	stateDir := t.TempDir()
	pipelineFile := filepath.Join(t.TempDir(), "pipeline.yaml")
	if err := os.WriteFile(pipelineFile, []byte(""+
		"name: demo-subprocess\n"+
		"init_command: git init repo\n"+
		"loop_num: 2\n"+
		"plan_prompt: plan\n"+
		"coding_prompt: code\n"), 0o644); err != nil {
		t.Fatalf("write pipeline yaml: %v", err)
	}
	if result := runRelaySubprocess(t, "pipeline", "import", "-file", pipelineFile, "-state-dir", stateDir); result.exitCode != 0 {
		t.Fatalf("pipeline import exit=%d stderr=%s", result.exitCode, result.stderr.String())
	}
	if result := runRelaySubprocess(t,
		"issue", "add",
		"--id", "issue-subprocess",
		"--pipeline", "demo-subprocess",
		"--goal", "Verify subprocess status behavior",
		"--description", "Exercise a second real CLI flow after issue creation.",
		"-state-dir", stateDir,
	); result.exitCode != 0 {
		t.Fatalf("issue add exit=%d stderr=%s", result.exitCode, result.stderr.String())
	}
	return stateDir
}

type relaySubprocessResult struct {
	exitCode int
	stdout   bytes.Buffer
	stderr   bytes.Buffer
}

func runRelaySubprocess(t *testing.T, args ...string) relaySubprocessResult {
	t.Helper()

	cmd := exec.Command("go", append([]string{"run", "./cmd/relay"}, args...)...)
	cmd.Dir = repoRoot(t)
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)
	cmd.Env = os.Environ()

	err := cmd.Run()
	result := relaySubprocessResult{
		stdout: *cmd.Stdout.(*bytes.Buffer),
		stderr: *cmd.Stderr.(*bytes.Buffer),
	}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run relay subprocess: %v", err)
	}
	result.exitCode = exitErr.ExitCode()
	return result
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root")
		}
		dir = parent
	}
}
