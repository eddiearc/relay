package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eddiearc/relay/internal/relay"
)

func TestResolveStateDirDefaultsToHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("home directory unavailable")
	}
	got := resolveStateDir("")
	want := filepath.Join(home, ".relay")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveStateDirResolvesRelativePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	got := resolveStateDir("tmp-state")
	want := filepath.Join(cwd, "tmp-state")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestHelpIncludesVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := run([]string{"help"}, &stdout, io.Discard)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("version")) {
		t.Fatalf("expected help output to include version command, got %s", stdout.String())
	}
}

func TestHelpIncludesUpgradeCommand(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := run([]string{"help"}, &stdout, io.Discard)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("upgrade")) {
		t.Fatalf("expected help output to include upgrade command, got %s", stdout.String())
	}
}

func TestTopLevelHelpIncludesWorkflowGuidance(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := run([]string{"help"}, &stdout, io.Discard)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	output := stdout.String()
	for _, want := range []string{
		"Workflow:",
		"relay help",
		"relay version",
		"relay help pipeline",
		"relay help issue",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected help output to contain %q, got %s", want, output)
		}
	}
	for _, unwanted := range []string{
		"relay help pipeline match",
		"relay help issue evaluate",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected help output to omit %q, got %s", unwanted, output)
		}
	}
}

func TestPipelineHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"help", "pipeline"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"manage persisted pipelines",
		"relay pipeline import -file pipeline.yaml",
		"relay pipeline template",
		"relay help pipeline <subcommand>",
		"repo-level E2E, integration, or CLI end-to-end checks",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected pipeline help output to contain %q, got %s", want, output)
		}
	}
	for _, unwanted := range []string{
		"match",
		"project metadata",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected pipeline help output to omit %q, got %s", unwanted, output)
		}
	}
}

func TestPipelineAddHelpMatchesDetailedUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "add", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"create a pipeline from flags and prompt files",
		"--init-command",
		"--plan-prompt-file",
		"--coding-prompt-file",
		"frontend pipelines should usually preserve browser-driven E2E",
		"ask whether the direction looks right",
		"15 is a reasonable upper bound",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected pipeline add help output to contain %q, got %s", want, output)
		}
	}
	for _, unwanted := range []string{
		"--project-key",
		"--project-path",
		"--project-remote-url",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected pipeline add help output to omit %q, got %s", unwanted, output)
		}
	}
}

func TestPipelineTemplateHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"help", "pipeline", "template"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"print a complete pipeline YAML template",
		"relay pipeline template > pipeline.yaml",
		"Template:",
		"name: repo-name",
		"coding_prompt:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected pipeline template help output to contain %q, got %s", want, output)
		}
	}
}

func TestPipelineTemplateCommandPrintsTemplate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "template"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"name: repo-name",
		"init_command:",
		"plan_prompt:",
		"coding_prompt:",
		"Ensure the branch has an open pull request",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected pipeline template output to contain %q, got %s", want, output)
		}
	}
	if strings.Contains(output, "project:") {
		t.Fatalf("expected pipeline template output to omit project block, got %s", output)
	}
}

func TestIssueHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"help", "issue"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"manage persisted issues",
		"relay issue add --pipeline demo",
		"relay issue template",
		"relay help issue <subcommand>",
		"treat missing repo-level verification as a harness gap",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected issue help output to contain %q, got %s", want, output)
		}
	}
	if strings.Contains(output, "evaluate") {
		t.Fatalf("expected issue help output to omit evaluate, got %s", output)
	}
}

func TestIssueAddHelpMatchesDetailedUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "add", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"create an issue from flags",
		"--pipeline",
		"--goal",
		"--description",
		"feature_list.json rules",
		"frontend browser flows with simulated clicks",
		"CLI command sequences that exercise the built or local binary",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected issue add help output to contain %q, got %s", want, output)
		}
	}
}

func TestIssueTemplateHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"help", "issue", "template"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"print a complete issue JSON template",
		"relay issue template > issue.json",
		"Template:",
		"\"pipeline_name\": \"repo-name\"",
		"\"description\": \"Describe scope, constraints, validation commands, reusable verification assets, missing E2E or unit-test gaps, non-goals, and any known context that should shape feature planning.\"",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected issue template help output to contain %q, got %s", want, output)
		}
	}
}

func TestIssueTemplateCommandPrintsTemplate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "template"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"\"pipeline_name\": \"repo-name\"",
		"\"goal\": \"Describe the end state in one sentence.\"",
		"\"description\": \"Describe scope, constraints, validation commands, reusable verification assets, missing E2E or unit-test gaps, non-goals, and any known context that should shape feature planning.\"",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected issue template output to contain %q, got %s", want, output)
		}
	}
}

func TestServeHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"serve", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"start the polling orchestrator",
		"--once",
		"--poll-interval",
		"Recommended startup sequence:",
		"Diagnostic workflow:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected serve help output to contain %q, got %s", want, output)
		}
	}
}

func TestUpgradeHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"upgrade", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("upgrade the relay CLI")) {
		t.Fatalf("expected upgrade help output, got %s", stdout.String())
	}
}

func TestDetectInstallMethod(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		goBin      string
		goPath     string
		wantMethod installMethod
	}{
		{
			name:       "npm install",
			path:       "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay",
			wantMethod: installMethodNPM,
		},
		{
			name:       "go install with gobin",
			path:       "/Users/test/bin/relay",
			goBin:      "/Users/test/bin",
			wantMethod: installMethodGoInstall,
		},
		{
			name:       "go install with gopath",
			path:       "/Users/test/go/bin/relay",
			goPath:     "/Users/test/go",
			wantMethod: installMethodGoInstall,
		},
		{
			name:       "local build",
			path:       "/tmp/relay/bin/relay",
			wantMethod: installMethodLocalBuild,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectInstallMethod(tc.path, tc.goBin, tc.goPath)
			if got != tc.wantMethod {
				t.Fatalf("expected %q, got %q", tc.wantMethod, got)
			}
		})
	}
}

func TestUpgradeLocalBuildUnavailable(t *testing.T) {
	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/tmp/relay/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	called := false
	restoreRunner := setUpgradeCommandRunnerForTesting(func(name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreRunner()
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"upgrade"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if called {
		t.Fatalf("expected no upgrade command for local builds")
	}
	if !strings.Contains(stdout.String(), "self-upgrade is unavailable for local builds") {
		t.Fatalf("expected local build message, got %s", stdout.String())
	}
}

func TestUpgradeRunsNPMCommand(t *testing.T) {
	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	var gotName string
	var gotArgs []string
	restoreRunner := setUpgradeCommandRunnerForTesting(func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("updated"), nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		if method != installMethodNPM {
			t.Fatalf("expected npm lookup, got %q", method)
		}
		return "v9.9.9", nil
	})
	restoreVersionLookup := setUpgradeVersionLookupForTesting(func(executable string) (string, error) {
		return "v9.9.9", nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreRunner()
		restoreLatestVersionLookup()
		restoreVersionLookup()
	})

	var stdout bytes.Buffer
	if exitCode := run([]string{"upgrade"}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if gotName != "npm" {
		t.Fatalf("expected npm command, got %q", gotName)
	}
	if diff := strings.Join(gotArgs, " "); diff != "update -g @eddiearc/relay" {
		t.Fatalf("expected npm update args, got %q", diff)
	}
}

func TestUpgradeRunsGoInstallCommand(t *testing.T) {
	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/Users/test/go/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "/Users/test/go", nil
	})
	var gotName string
	var gotArgs []string
	restoreRunner := setUpgradeCommandRunnerForTesting(func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("installed"), nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		if method != installMethodGoInstall {
			t.Fatalf("expected go-install lookup, got %q", method)
		}
		return "v9.9.9", nil
	})
	restoreVersionLookup := setUpgradeVersionLookupForTesting(func(executable string) (string, error) {
		return "v9.9.9", nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreRunner()
		restoreLatestVersionLookup()
		restoreVersionLookup()
	})

	var stdout bytes.Buffer
	if exitCode := run([]string{"upgrade"}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if gotName != "go" {
		t.Fatalf("expected go command, got %q", gotName)
	}
	if diff := strings.Join(gotArgs, " "); diff != "install github.com/eddiearc/relay/cmd/relay@latest" {
		t.Fatalf("expected go install args, got %q", diff)
	}
}

func TestUpgradeReportsAlreadyUpToDate(t *testing.T) {
	previousVersion := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = previousVersion
	})

	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	called := false
	restoreRunner := setUpgradeCommandRunnerForTesting(func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte("updated"), nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		return "v1.2.3", nil
	})
	restoreVersionLookup := setUpgradeVersionLookupForTesting(func(executable string) (string, error) {
		return "v1.2.3", nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreRunner()
		restoreLatestVersionLookup()
		restoreVersionLookup()
	})

	var stdout bytes.Buffer
	if exitCode := run([]string{"upgrade"}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if called {
		t.Fatalf("expected no upgrade command when already up to date")
	}
	if diff := strings.TrimSpace(stdout.String()); diff != "Already up to date (v1.2.3)" {
		t.Fatalf("expected exact up-to-date output, got %q", diff)
	}
}

func TestUpgradeReportsTransition(t *testing.T) {
	previousVersion := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = previousVersion
	})

	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	restoreRunner := setUpgradeCommandRunnerForTesting(func(name string, args ...string) ([]byte, error) {
		return []byte("updated"), nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		return "v1.2.4", nil
	})
	restoreVersionLookup := setUpgradeVersionLookupForTesting(func(executable string) (string, error) {
		return "v1.2.4", nil
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreRunner()
		restoreLatestVersionLookup()
		restoreVersionLookup()
	})

	var stdout bytes.Buffer
	if exitCode := run([]string{"upgrade"}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}
	if diff := strings.TrimSpace(stdout.String()); diff != "Upgraded: v1.2.3 → v1.2.4" {
		t.Fatalf("expected exact upgrade output, got %q", diff)
	}
}

func TestUpgradeReturnsLatestVersionLookupErrors(t *testing.T) {
	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	restoreLatestVersionLookup := setUpgradeLatestVersionLookupForTesting(func(method installMethod) (string, error) {
		return "", errors.New("registry unavailable")
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreLatestVersionLookup()
	})

	var stderr bytes.Buffer
	if exitCode := run([]string{"upgrade"}, io.Discard, &stderr); exitCode == 0 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(stderr.String(), "check latest relay version") {
		t.Fatalf("expected latest-version lookup error, got %s", stderr.String())
	}
}

func TestUpgradeReturnsCommandErrors(t *testing.T) {
	restoreExecutable := setUpgradeExecutableForTesting(func() (string, error) {
		return "/usr/local/lib/node_modules/@eddiearc/relay-darwin-arm64/bin/relay", nil
	})
	restoreGoPaths := setUpgradeGoPathsForTesting(func() (string, string, error) {
		return "", "", nil
	})
	restoreRunner := setUpgradeCommandRunnerForTesting(func(name string, args ...string) ([]byte, error) {
		return []byte("permission denied"), errors.New("exit status 1")
	})
	t.Cleanup(func() {
		restoreExecutable()
		restoreGoPaths()
		restoreRunner()
	})

	var stderr bytes.Buffer
	if exitCode := run([]string{"upgrade"}, io.Discard, &stderr); exitCode == 0 {
		t.Fatalf("expected failure")
	}
	output := stderr.String()
	for _, want := range []string{"upgrade failed", "npm update -g @eddiearc/relay", "permission denied"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected error output to contain %q, got %s", want, output)
		}
	}
}

func TestVersionCommandPrintsInjectedBuildMetadata(t *testing.T) {
	previousVersion := version
	previousCommit := commit
	previousDate := buildDate
	version = "v1.2.3"
	commit = "abc1234"
	buildDate = "2026-03-24T12:34:56Z"
	t.Cleanup(func() {
		version = previousVersion
		commit = previousCommit
		buildDate = previousDate
	})

	var stdout bytes.Buffer
	exitCode := run([]string{"version"}, &stdout, io.Discard)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d", exitCode)
	}

	output := stdout.String()
	for _, want := range []string{
		"relay v1.2.3",
		"commit: abc1234",
		"built: 2026-03-24T12:34:56Z",
	} {
		if !bytes.Contains([]byte(output), []byte(want)) {
			t.Fatalf("expected version output to contain %q, got %s", want, output)
		}
	}
}

func TestPipelineAddSavesYAMLPipeline(t *testing.T) {
	stateDir := t.TempDir()
	planPrompt := writeTempFile(t, "plan.md", "plan {{issue}}")
	codingPrompt := writeTempFile(t, "coding.md", "code {{issue}}")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"pipeline", "add",
		"--init-command", "git init repo",
		"--loop-num", "2",
		"--plan-prompt-file", planPrompt,
		"--coding-prompt-file", codingPrompt,
		"-state-dir", stateDir,
		"demo",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "pipelines", "demo.yaml")); err != nil {
		t.Fatalf("expected yaml pipeline file to be saved: %v", err)
	}
	for _, want := range []string{
		"relay pipeline show demo",
		"relay issue add --pipeline demo",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected pipeline add hint %q, got stderr=%s stdout=%s", want, stderr.String(), stdout.String())
		}
	}
}

func TestPipelineImportSavesYAMLPipeline(t *testing.T) {
	stateDir := t.TempDir()
	pipelineFile := writeTempFile(t, "pipeline.yaml", ""+
		"name: demo-import\n"+
		"init_command: git init repo\n"+
		"loop_num: 2\n"+
		"plan_prompt: plan {{issue}}\n"+
		"coding_prompt: code {{issue}}\n")

	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "import", "-file", pipelineFile, "-state-dir", stateDir}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "pipelines", "demo-import.yaml")); err != nil {
		t.Fatalf("expected imported pipeline file to be saved: %v", err)
	}
}

func TestIssueAddCreatesPerIssueDirectory(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"issue", "add",
		"--id", "issue-add",
		"--pipeline", "demo",
		"--goal", "ship feature",
		"--description", "test issue",
		"-state-dir", stateDir,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "issues", "issue-add", "issue.json")); err != nil {
		t.Fatalf("expected issue.json to be created: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"artifact_dir"`)) {
		t.Fatalf("expected artifact dir in output, got %s", stdout.String())
	}
	for _, want := range []string{
		"relay serve --once -state-dir " + stateDir,
		"relay serve -state-dir " + stateDir,
		"relay watch -issue issue-add -state-dir " + stateDir,
		"relay status -issue issue-add -state-dir " + stateDir,
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected issue add hint %q, got %s", want, stderr.String())
		}
	}
}

func TestPipelineShowPrintsNextStepHints(t *testing.T) {
	stateDir := t.TempDir()
	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name:         "demo-show-hint",
		InitCommand:  "git clone --depth 1 https://github.com/example/repo .",
		LoopNum:      15,
		PlanPrompt:   "Read the repository before planning.",
		CodingPrompt: "Verify progress with real commands.",
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "show", "-state-dir", stateDir, "demo-show-hint"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	for _, want := range []string{
		"relay issue add --pipeline demo-show-hint",
		"relay pipeline edit demo-show-hint",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected pipeline show hint %q, got stderr=%s stdout=%s", want, stderr.String(), stdout.String())
		}
	}
}

func TestPipelineListSuggestsHowToContinue(t *testing.T) {
	stateDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "list", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	for _, want := range []string{
		"relay pipeline template > pipeline.yaml",
		"relay pipeline add <name>",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected empty pipeline list hint %q, got %s", want, stderr.String())
		}
	}

	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name:         "demo-list",
		InitCommand:  "git init repo",
		LoopNum:      2,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	})

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"pipeline", "list", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "relay pipeline show demo-list") {
		t.Fatalf("expected pipeline list hint to inspect first pipeline, got %s", stderr.String())
	}
}

func TestServeOncePrintsWatchHintForRunningIssue(t *testing.T) {
	stateDir := t.TempDir()
	savePipelineForTest(t, stateDir, relay.Pipeline{
		Name: "demo-serve-hint",
		InitCommand: "mkdir repo && cd repo && git init && git config user.email relay@example.com && git config user.name relay && " +
			"printf 'init\\n' > README.md && git add README.md && git commit -m init",
		LoopNum:      1,
		PlanPrompt:   "plan",
		CodingPrompt: "code",
	})
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-serve-hint",
		PipelineName: "demo-serve-hint",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusTodo,
	})

	previousRunner := newServeRunner
	newServeRunner = func() relay.AgentRunner { return scriptedServeRunner{t: t} }
	t.Cleanup(func() {
		newServeRunner = previousRunner
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"serve", "--once", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	for _, want := range []string{
		"running issue issue-serve-hint",
		"relay watch -issue issue-serve-hint -state-dir " + stateDir,
		"relay status -issue issue-serve-hint -state-dir " + stateDir,
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected serve hint %q, got stderr=%s stdout=%s", want, stderr.String(), stdout.String())
		}
	}
	if !strings.Contains(stdout.String(), `"status": "done"`) {
		t.Fatalf("expected serve output to include completed issue JSON, got %s", stdout.String())
	}
}

func TestIssueImportCreatesPerIssueDirectory(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo")

	issueFile := writeTempFile(t, "issue.json", `{
  "id": "issue-import",
  "pipeline_name": "demo",
  "goal": "ship feature",
  "description": "test issue"
}`)

	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "import", "-file", issueFile, "-state-dir", stateDir}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "issues", "issue-import", "issue.json")); err != nil {
		t.Fatalf("expected issue.json to be created: %v", err)
	}
}

func TestIssueEditAllowsRunningIssue(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-running-edit")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-running-edit",
		PipelineName: "demo-running-edit",
		Goal:         "old goal",
		Description:  "old desc",
		Status:       relay.IssueStatusRunning,
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"issue", "edit",
		"--id", "issue-running-edit",
		"--goal", "new goal",
		"--description", "new desc",
		"-state-dir", stateDir,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("issue edit failed: %s", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status": "running"`)) {
		t.Fatalf("expected running issue output, got %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"goal": "new goal"`)) {
		t.Fatalf("expected updated goal, got %s", stdout.String())
	}
}

func TestIssueDeleteFailsWhenRunning(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-running-delete")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-running-delete",
		PipelineName: "demo-running-delete",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
	})

	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "delete", "--id", "issue-running-delete", "-state-dir", stateDir}, io.Discard, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected issue delete to fail for running issue")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("cannot be deleted")) {
		t.Fatalf("expected running issue delete error, got %s", stderr.String())
	}
}

func TestIssueInterruptRequestsStopForRunningIssue(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-running-interrupt")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-running-interrupt",
		PipelineName: "demo-running-interrupt",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"issue", "interrupt", "--id", "issue-running-interrupt", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("issue interrupt failed: %s", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status": "running"`)) {
		t.Fatalf("expected running status until current loop ends, got %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"interrupt_requested": true`)) {
		t.Fatalf("expected interrupt request flag, got %s", stdout.String())
	}
}

func TestRecoverActiveIssuesMarksOrphanedRunsTodo(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-recover")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-recover",
		PipelineName: "demo-recover",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
		ActivePhase:  "coding",
		ActivePIDs:   []int{12345},
	})

	store := relay.NewStore(stateDir)
	recovered, err := recoverActiveIssues(store)
	if err != nil {
		t.Fatalf("recoverActiveIssues: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered issue, got %d", recovered)
	}
	issue := loadIssueSnapshot(t, stateDir, "issue-recover")
	if issue.Status != relay.IssueStatusTodo {
		t.Fatalf("expected todo after recovery, got %q", issue.Status)
	}
	if issue.ActivePhase != "" || len(issue.ActivePIDs) != 0 {
		t.Fatalf("expected active runtime fields to be cleared, got phase=%q pids=%v", issue.ActivePhase, issue.ActivePIDs)
	}
}

func TestPipelineDeleteFailsWhenIssueRunning(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-pipeline-running-delete")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-pipeline-running",
		PipelineName: "demo-pipeline-running-delete",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
	})

	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "delete", "-state-dir", stateDir, "demo-pipeline-running-delete"}, io.Discard, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected pipeline delete to fail for running issue")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("active issue")) {
		t.Fatalf("expected active issue error, got %s", stderr.String())
	}
}

func TestPipelineDeleteAllowsTodoIssue(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-pipeline-todo-delete")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-pipeline-todo",
		PipelineName: "demo-pipeline-todo-delete",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusTodo,
	})

	var stderr bytes.Buffer
	exitCode := run([]string{"pipeline", "delete", "-state-dir", stateDir, "demo-pipeline-todo-delete"}, io.Discard, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected pipeline delete to succeed for todo issue: %s", stderr.String())
	}
}

func TestStatusReadsIssueDirectoryState(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-status")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:            "issue-status",
		PipelineName:  "demo-status",
		Goal:          "goal",
		Description:   "desc",
		Status:        relay.IssueStatusRunning,
		ArtifactDir:   filepath.Join(stateDir, "issues", "issue-status"),
		WorkspacePath: "/tmp/workspace",
		WorkdirPath:   "/tmp/workspace/app",
	})

	var stdout bytes.Buffer
	if exitCode := run([]string{"status", "-issue", "issue-status", "-state-dir", stateDir}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("status command failed")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("artifact=")) {
		t.Fatalf("expected artifact path in status output, got %s", stdout.String())
	}
}

func TestReportListsArtifactsAndEventsLog(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-report")
	issue := relay.Issue{
		ID:           "issue-report",
		PipelineName: "demo-report",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusDone,
	}
	saveIssueSnapshot(t, stateDir, issue)
	store := relay.NewStore(stateDir)
	if err := os.WriteFile(relay.FeatureListPath(store.IssueDir(issue.ID)), []byte("[]"), 0o644); err != nil {
		t.Fatalf("write feature_list.json: %v", err)
	}
	if err := os.WriteFile(relay.ProgressPath(store.IssueDir(issue.ID)), []byte("done"), 0o644); err != nil {
		t.Fatalf("write progress.txt: %v", err)
	}
	if err := store.AppendEvent(issue.ID, "issue completed"); err != nil {
		t.Fatalf("append event: %v", err)
	}

	var stdout bytes.Buffer
	if exitCode := run([]string{"report", "-issue", "issue-report", "-state-dir", stateDir}, &stdout, io.Discard); exitCode != 0 {
		t.Fatalf("report command failed")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("artifacts:")) {
		t.Fatalf("expected artifacts section, got %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("events.log")) {
		t.Fatalf("expected events.log path, got %s", stdout.String())
	}
}

func TestKillTerminatesTrackedIssueProcesses(t *testing.T) {
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-kill")

	cmd := exec.Command("zsh", "-lc", "sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}

	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:           "issue-kill",
		PipelineName: "demo-kill",
		Goal:         "goal",
		Description:  "desc",
		Status:       relay.IssueStatusRunning,
		ActivePhase:  "coding",
		ActivePIDs:   []int{cmd.Process.Pid},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"kill", "-issue", "issue-kill", "-state-dir", stateDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("kill failed: %s", stderr.String())
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	select {
	case <-time.After(3 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		t.Fatalf("expected process %d to exit after kill", cmd.Process.Pid)
	case <-waitDone:
	}

	issue := loadIssueSnapshot(t, stateDir, "issue-kill")
	if issue.Status != relay.IssueStatusFailed {
		t.Fatalf("expected failed issue after kill, got %q", issue.Status)
	}
	if issue.ActivePhase != "" || len(issue.ActivePIDs) != 0 {
		t.Fatalf("expected runtime fields to be cleared after kill, got phase=%q pids=%v", issue.ActivePhase, issue.ActivePIDs)
	}
}

func importTestPipeline(t *testing.T, stateDir, name string) {
	t.Helper()
	pipelineFile := writeTempFile(t, "pipeline.yaml", ""+
		"name: "+name+"\n"+
		"init_command: git init repo\n"+
		"loop_num: 2\n"+
		"plan_prompt: plan {{issue}}\n"+
		"coding_prompt: code {{issue}}\n")
	if exitCode := run([]string{"pipeline", "import", "-file", pipelineFile, "-state-dir", stateDir}, io.Discard, io.Discard); exitCode != 0 {
		t.Fatalf("pipeline import failed")
	}
}

func saveIssueSnapshot(t *testing.T, stateDir string, issue relay.Issue) {
	t.Helper()
	store := relay.NewStore(stateDir)
	store.WorkspaceRoot = filepath.Join(stateDir, "relay-workspaces")
	if err := store.Ensure(); err != nil {
		t.Fatalf("ensure store: %v", err)
	}
	if err := store.SaveIssue(issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}
}

type scriptedServeRunner struct {
	t *testing.T
}

func (r scriptedServeRunner) Run(_ context.Context, req relay.AgentRunRequest) (relay.AgentRunResult, error) {
	r.t.Helper()
	featureListPath := mustPromptPathValue(r.t, req.Prompt, "FEATURE_LIST_PATH=")
	progressPath := mustPromptPathValue(r.t, req.Prompt, "PROGRESS_PATH=")

	switch req.Phase {
	case "plan":
		writeFeatureListJSON(r.t, featureListPath, `[{"id":"feature-1","title":"Feature","description":"desc","priority":1,"passes":false,"notes":""}]`)
		writeProgressEntries(r.t, progressPath, "planned initial features")
	case "coding":
		writeFeatureListJSON(r.t, featureListPath, `[{"id":"feature-1","title":"Feature","description":"desc","priority":1,"passes":true,"notes":"verified"}]`)
		appendProgressEntry(r.t, progressPath, "implemented and verified feature-1")
	default:
		r.t.Fatalf("unexpected phase %q", req.Phase)
	}

	return relay.AgentRunResult{Stdout: "ok", FinalMessage: "done"}, nil
}

func mustPromptPathValue(t *testing.T, prompt, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	t.Fatalf("missing %s in prompt", prefix)
	return ""
}

func writeFeatureListJSON(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir feature list dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write feature list: %v", err)
	}
}

func writeProgressEntries(t *testing.T, path string, entries ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir progress dir: %v", err)
	}
	data := strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write progress: %v", err)
	}
}

func appendProgressEntry(t *testing.T, path, entry string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open progress: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(entry + "\n"); err != nil {
		t.Fatalf("append progress: %v", err)
	}
}

func loadIssueSnapshot(t *testing.T, stateDir, issueID string) relay.Issue {
	t.Helper()
	store := relay.NewStore(stateDir)
	issue, err := store.LoadIssue(issueID)
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	return issue
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}
