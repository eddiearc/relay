package relay

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	planHarnessContract = `You are the planning phase of Relay.

You must work from the repository and task context, then write the task artifacts to the exact absolute paths provided below.

Requirements:
- Understand the issue goal, description, and repository context before planning.
- Create a non-empty feature_list.json at FEATURE_LIST_PATH.
- Create or initialize progress.txt at PROGRESS_PATH.
- feature_list.json is the only source of truth for completion.
- progress.txt is the handoff log for future runs.
- Write files directly; do not only describe them in your response.
- Validate that feature_list.json is valid JSON and non-empty before finishing.`
	codingHarnessContract = `You are the coding phase of Relay.

You must work inside the repository, but all task artifacts live outside the repository at the absolute paths provided below.

Requirements:
- Read and update FEATURE_LIST_PATH based on actual progress.
- Append the current loop summary to PROGRESS_PATH before finishing.
- feature_list.json is the only source of truth for completion.
- Do not remove existing features.
- Do not change any feature passes value from true back to false.
- Make code changes in REPO_PATH as needed.
- If you modify repository code, commit those repo changes before finishing.`
)

func RenderPrompt(template string, issue Issue, phase string, loopIndex int) string {
	replacements := map[string]string{
		"{{issue}}":             issue.TemplateJSON(),
		"{{goal}}":              issue.Goal,
		"{{description}}":       issue.Description,
		"{{phase}}":             phase,
		"{{loop_index}}":        strconv.Itoa(loopIndex),
		"{{artifact_dir}}":      issue.ArtifactDir,
		"{{issue_path}}":        IssueFilePath(issue.ArtifactDir),
		"{{feature_list_path}}": FeatureListPath(issue.ArtifactDir),
		"{{progress_path}}":     ProgressPath(issue.ArtifactDir),
		"{{workspace_path}}":    issue.WorkspacePath,
		"{{repo_path}}":         issue.RepoPath,
	}
	rendered := template
	for needle, value := range replacements {
		rendered = strings.ReplaceAll(rendered, needle, value)
	}
	return rendered
}

func BuildPrompt(issue Issue, phase string, loopIndex int, pipelinePrompt string) string {
	harness := codingHarnessContract
	if phase == "plan" {
		harness = planHarnessContract
	}
	var sections []string
	sections = append(sections, harness)
	sections = append(sections, fmt.Sprintf(
		"Paths:\nISSUE_PATH=%s\nFEATURE_LIST_PATH=%s\nPROGRESS_PATH=%s\nREPO_PATH=%s\nWORKSPACE_PATH=%s",
		IssueFilePath(issue.ArtifactDir),
		FeatureListPath(issue.ArtifactDir),
		ProgressPath(issue.ArtifactDir),
		issue.RepoPath,
		issue.WorkspacePath,
	))
	rendered := RenderPrompt(pipelinePrompt, issue, phase, loopIndex)
	if strings.TrimSpace(rendered) != "" {
		sections = append(sections, rendered)
	}
	return strings.Join(sections, "\n\n")
}

func TailContext(artifactDir string) string {
	var chunks []string
	if data, err := os.ReadFile(filepath.Join(artifactDir, "feature_list.json")); err == nil {
		chunks = append(chunks, "Current feature_list.json:\n"+string(data))
	}
	if data, err := os.ReadFile(filepath.Join(artifactDir, "progress.txt")); err == nil {
		lines := strings.Split(string(data), "\n")
		const maxLines = 80
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		chunks = append(chunks, "Recent progress.txt:\n"+strings.Join(lines, "\n"))
	}
	if len(chunks) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(chunks, "\n\n")
}
