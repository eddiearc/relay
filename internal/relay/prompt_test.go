package relay

import (
	"strings"
	"testing"
)

func TestBuildPromptPlanIncludesArtifactSchemaAndRestrictions(t *testing.T) {
	issue := Issue{
		ID:          "issue-1",
		Goal:        "goal",
		Description: "desc",
		ArtifactDir: "/tmp/state/issues/issue-1",
		WorkspacePath: "/tmp/workspaces/issue-1",
		RepoPath:    "/tmp/workspaces/issue-1/repo",
	}

	prompt := BuildPrompt(issue, "plan", 0, "pipeline prompt")

	for _, needle := range []string{
		"Do not use apply_patch with an absolute path",
		"Do not create extra planning files",
		"feature_list.json must be exactly a JSON array",
		"Every item in feature_list.json must have exactly these JSON fields",
		"\"passes\": false",
		"FEATURE_LIST_PATH=/tmp/state/issues/issue-1/feature_list.json",
		"PROGRESS_PATH=/tmp/state/issues/issue-1/progress.txt",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q\n%s", needle, prompt)
		}
	}
}

func TestBuildPromptCodingIncludesArtifactUpdateRules(t *testing.T) {
	issue := Issue{
		ID:          "issue-1",
		Goal:        "goal",
		Description: "desc",
		ArtifactDir: "/tmp/state/issues/issue-1",
		WorkspacePath: "/tmp/workspaces/issue-1",
		RepoPath:    "/tmp/workspaces/issue-1/repo",
	}

	prompt := BuildPrompt(issue, "coding", 2, "pipeline prompt")

	for _, needle := range []string{
		"Do not use apply_patch with absolute paths",
		"FEATURE_LIST_PATH must remain a JSON array",
		"Do not change any feature passes value from true back to false",
		"REPO_PATH=/tmp/workspaces/issue-1/repo",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q\n%s", needle, prompt)
		}
	}
}
