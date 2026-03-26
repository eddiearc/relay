package cli

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiearc/relay/internal/relay"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

func TestCommandOutputGolden(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		command   string
		args      []string
		issueID   string
		file      string
		setup     func(t *testing.T) string
		normalize func(t *testing.T, stateDir string, stdout, stderr []byte) ([]byte, []byte)
	}{
		{name: "help-top-level", args: []string{"help"}, file: "help.golden"},
		{name: "help-pipeline", args: []string{"help", "pipeline"}, file: "help_pipeline.golden"},
		{
			name:    "status",
			command: "status",
			issueID: "issue-status",
			file:    "status.golden",
			setup:   setupStatusGoldenState,
			normalize: func(t *testing.T, stateDir string, stdout, stderr []byte) ([]byte, []byte) {
				t.Helper()
				return normalizeGoldenOutput(stateDir, stdout), normalizeGoldenOutput(stateDir, stderr)
			},
		},
		{
			name:    "report",
			command: "report",
			issueID: "issue-report",
			file:    "report.golden",
			setup:   setupReportGoldenState,
			normalize: func(t *testing.T, stateDir string, stdout, stderr []byte) ([]byte, []byte) {
				t.Helper()
				return normalizeGoldenOutput(stateDir, stdout), normalizeGoldenOutput(stateDir, stderr)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			args := append([]string(nil), tc.args...)
			stateDir := ""
			if tc.setup != nil {
				stateDir = tc.setup(t)
				args = append([]string{tc.command, "-issue", tc.issueID, "-state-dir", stateDir}, args...)
			}

			if exitCode := run(args, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
			}

			gotStdout := stdout.Bytes()
			gotStderr := stderr.Bytes()
			if tc.normalize != nil {
				gotStdout, gotStderr = tc.normalize(t, stateDir, gotStdout, gotStderr)
			}
			assertGoldenFile(t, filepath.Join("testdata", tc.file), gotStdout)
			if len(gotStderr) > 0 {
				assertGoldenFile(t, filepath.Join("testdata", strings.TrimSuffix(tc.file, ".golden")+".stderr.golden"), gotStderr)
			}
		})
	}
}

func setupStatusGoldenState(t *testing.T) string {
	t.Helper()
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-status")
	saveIssueSnapshot(t, stateDir, relay.Issue{
		ID:                 "issue-status",
		PipelineName:       "demo-status",
		Goal:               "summarize status",
		Description:        "Capture a stable status output.",
		Status:             relay.IssueStatusRunning,
		CurrentLoop:        2,
		ArtifactDir:        filepath.Join(stateDir, "issues", "issue-status"),
		WorkspacePath:      filepath.Join(stateDir, "workspace", "repo"),
		WorkdirPath:        filepath.Join(stateDir, "workspace", "repo", "app"),
		ActivePhase:        "coding",
		ActivePIDs:         []int{111, 222},
		LastError:          "retrying status snapshot",
		InterruptRequested: true,
	})
	return stateDir
}

func setupReportGoldenState(t *testing.T) string {
	t.Helper()
	stateDir := t.TempDir()
	importTestPipeline(t, stateDir, "demo-report")
	issue := relay.Issue{
		ID:                 "issue-report",
		PipelineName:       "demo-report",
		Goal:               "render report",
		Description:        "Capture a stable report output.",
		Status:             relay.IssueStatusDone,
		CurrentLoop:        3,
		ArtifactDir:        filepath.Join(stateDir, "issues", "issue-report"),
		WorkspacePath:      filepath.Join(stateDir, "workspace", "repo"),
		WorkdirPath:        filepath.Join(stateDir, "workspace", "repo", "app"),
		InterruptRequested: false,
	}
	saveIssueSnapshot(t, stateDir, issue)
	store := relay.NewStore(stateDir)
	if err := os.WriteFile(relay.FeatureListPath(store.IssueDir(issue.ID)), []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("write feature_list.json: %v", err)
	}
	if err := os.WriteFile(relay.ProgressPath(store.IssueDir(issue.ID)), []byte("planning complete\ncoding complete\n"), 0o644); err != nil {
		t.Fatalf("write progress.txt: %v", err)
	}
	if err := store.AppendEvent(issue.ID, "issue completed"); err != nil {
		t.Fatalf("append event: %v", err)
	}
	runDir := store.RunDir(issue.ID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	for _, name := range []string{"coding-loop-01.stdout.log", "coding-loop-01.stderr.log"} {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(fmt.Sprintf("log=%s\n", name)), 0o644); err != nil {
			t.Fatalf("write run log %s: %v", name, err)
		}
	}
	return stateDir
}

func normalizeGoldenOutput(stateDir string, data []byte) []byte {
	if stateDir == "" {
		return data
	}
	replacer := strings.NewReplacer(
		stateDir, "<STATE_DIR>",
		filepath.ToSlash(stateDir), "<STATE_DIR>",
	)
	return []byte(replacer.Replace(string(data)))
}

func assertGoldenFile(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\nrefresh with: go test ./internal/cli -run TestCommandOutputGolden -args -update", path)
	}
}
