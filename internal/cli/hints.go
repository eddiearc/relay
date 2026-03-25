package cli

import (
	"fmt"
	"io"

	"github.com/eddiearc/relay/internal/relay"
)

func stateDirHintSuffix(rawStateDir string) string {
	if rawStateDir == "" {
		return ""
	}
	return fmt.Sprintf(" -state-dir %s", resolvePath(rawStateDir))
}

func writeHintBlock(w io.Writer, title string, steps ...string) {
	if w == nil || len(steps) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "%s:\n", title)
	for _, step := range steps {
		_, _ = fmt.Fprintf(w, "- %s\n", step)
	}
}

func writePipelineContinuationHints(w io.Writer, pipelineName, rawStateDir string) {
	suffix := stateDirHintSuffix(rawStateDir)
	writeHintBlock(
		w,
		"Next",
		fmt.Sprintf("Inspect it: relay pipeline show %s%s", pipelineName, suffix),
		fmt.Sprintf("If the direction looks right, create an issue: relay issue add --pipeline %s --goal \"...\" --description \"...\"%s", pipelineName, suffix),
	)
}

func writePipelineShowHints(w io.Writer, pipelineName, rawStateDir string) {
	suffix := stateDirHintSuffix(rawStateDir)
	writeHintBlock(
		w,
		"Next",
		fmt.Sprintf("If this pipeline fits, create an issue: relay issue add --pipeline %s --goal \"...\" --description \"...\"%s", pipelineName, suffix),
		fmt.Sprintf("If it needs changes, edit it: relay pipeline edit %s%s", pipelineName, suffix),
	)
}

func writePipelineListHints(w io.Writer, firstPipelineName, rawStateDir string) {
	suffix := stateDirHintSuffix(rawStateDir)
	if firstPipelineName == "" {
		writeHintBlock(
			w,
			"Next",
			"Create a starter file: relay pipeline template > pipeline.yaml",
			fmt.Sprintf("Or add one directly: relay pipeline add <name> --init-command '...' --plan-prompt-file plan.md --coding-prompt-file coding.md%s", suffix),
		)
		return
	}
	writeHintBlock(
		w,
		"Next",
		fmt.Sprintf("Inspect a saved pipeline: relay pipeline show %s%s", firstPipelineName, suffix),
	)
}

func writeIssueExecutionHints(w io.Writer, issueID, rawStateDir string) {
	suffix := stateDirHintSuffix(rawStateDir)
	writeHintBlock(
		w,
		"Next",
		fmt.Sprintf("Process the queue once: relay serve --once%s", suffix),
		fmt.Sprintf("Keep consuming issues: relay serve%s", suffix),
		fmt.Sprintf("Watch progress: relay watch -issue %s%s", issueID, suffix),
		fmt.Sprintf("Inspect current state: relay status -issue %s%s", issueID, suffix),
	)
}

func writeServeWatchHint(w io.Writer, issue relay.Issue, rawStateDir string) {
	suffix := stateDirHintSuffix(rawStateDir)
	_, _ = fmt.Fprintf(w, "running issue %s (pipeline=%s)\n", issue.ID, issue.PipelineName)
	writeHintBlock(
		w,
		"Follow",
		fmt.Sprintf("Watch progress: relay watch -issue %s%s", issue.ID, suffix),
		fmt.Sprintf("Inspect state: relay status -issue %s%s", issue.ID, suffix),
	)
}

func writeWatchStartHint(w io.Writer, issueID, rawStateDir string) {
	suffix := stateDirHintSuffix(rawStateDir)
	writeHintBlock(
		w,
		fmt.Sprintf("Issue %s has no recorded execution yet", issueID),
		fmt.Sprintf("Start a single pass: relay serve --once%s", suffix),
		fmt.Sprintf("Or keep consuming issues: relay serve%s", suffix),
	)
}
