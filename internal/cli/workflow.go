package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eddiearc/relay/internal/relay"
	"gopkg.in/yaml.v3"
)

func runPipelineShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineShowUsage)
		fs.PrintDefaults()
	}
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 1 {
		_, _ = io.WriteString(stderr, "pipeline show requires a pipeline name argument\n")
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	pipeline, err := store.LoadPipeline(fs.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load pipeline %q: %v\n", fs.Arg(0), err)
		return 1
	}
	data, err := yaml.Marshal(pipeline)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "render pipeline yaml: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "summary:\n")
	_, _ = fmt.Fprintf(stdout, "- name: %s\n", pipeline.Name)
	_, _ = fmt.Fprintf(stdout, "- init_strategy: %s\n", summarizeBlock(pipeline.InitCommand))
	_, _ = fmt.Fprintf(stdout, "- agent_runner: %s\n", summarizeAgentRunner(pipeline.AgentRunner))
	_, _ = fmt.Fprintf(stdout, "- loop_limit: %d\n", pipeline.LoopNum)
	_, _ = fmt.Fprintf(stdout, "- plan_constraints: %s\n", strings.Join(firstMeaningfulLines(pipeline.PlanPrompt, 3), " | "))
	_, _ = fmt.Fprintf(stdout, "- coding_constraints: %s\n", strings.Join(firstMeaningfulLines(pipeline.CodingPrompt, 3), " | "))
	_, _ = fmt.Fprintf(stdout, "\nyaml:\n%s", string(data))
	return 0
}

func summarizeAgentRunner(value string) string {
	resolved, err := relay.ResolveAgentRunner("", value)
	if err != nil {
		return value
	}
	return resolved
}

func summarizeBlock(value string) string {
	lines := firstMeaningfulLines(value, 1)
	if len(lines) == 0 {
		return "(empty)"
	}
	return lines[0]
}

func firstMeaningfulLines(value string, limit int) []string {
	lines := strings.Split(value, "\n")
	result := make([]string, 0, limit)
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
		if len(result) == limit {
			break
		}
	}
	return result
}

func runWatch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, watchUsage)
		fs.PrintDefaults()
	}
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	pollInterval := fs.Duration("poll-interval", 2*time.Second, "watch polling interval")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *issueID == "" {
		_, _ = io.WriteString(stderr, "watch requires -issue\n")
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = io.WriteString(stderr, "watch does not take positional arguments\n")
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	var previous relay.Issue
	var havePrevious bool
	var eventOffset int64
	var progressSnapshot string
	var latestRunSummary string

	for {
		issue, err := store.LoadIssue(*issueID)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load issue: %v\n", err)
			return 1
		}

		if !havePrevious {
			_, _ = fmt.Fprintf(stdout, "status=%s loop=%d\n", issue.Status, issue.CurrentLoop)
		} else if issue.Status != previous.Status || issue.CurrentLoop != previous.CurrentLoop {
			_, _ = fmt.Fprintf(stdout, "status_change=%s->%s loop=%d\n", previous.Status, issue.Status, issue.CurrentLoop)
		}
		havePrevious = true
		previous = issue

		if summary := summarizeProgressFile(issue.ArtifactDir); summary != "" && summary != progressSnapshot {
			progressSnapshot = summary
			_, _ = fmt.Fprintf(stdout, "progress=%s\n", summary)
		}

		eventsPath := store.EventsPath(issue.ID)
		if nextOffset, lines, err := readNewEventLines(eventsPath, eventOffset); err == nil {
			eventOffset = nextOffset
			for _, line := range lines {
				_, _ = fmt.Fprintf(stdout, "event=%s\n", line)
			}
		}

		if relay.IsIssueTerminalStatus(issue.Status) {
			if summary := summarizeLatestRunFailure(store, issue.ID); summary != "" && summary != latestRunSummary {
				latestRunSummary = summary
				_, _ = fmt.Fprintf(stdout, "latest_run=%s\n", summary)
			}
			switch issue.Status {
			case relay.IssueStatusDone:
				_, _ = io.WriteString(stdout, "terminal_status=done\n")
				return 0
			case relay.IssueStatusFailed, relay.IssueStatusInterrupted:
				_, _ = fmt.Fprintf(stdout, "terminal_status=%s\n", issue.Status)
				return 2
			default:
				_, _ = fmt.Fprintf(stdout, "terminal_status=%s\n", issue.Status)
				return 0
			}
		}

		time.Sleep(*pollInterval)
	}
}

func summarizeProgressFile(artifactDir string) string {
	data, err := os.ReadFile(relay.ProgressPath(artifactDir))
	if err != nil {
		return ""
	}
	latest := ""
	entries := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entries++
		latest = line
	}
	if entries == 0 {
		return "progress.txt exists but is empty"
	}
	return fmt.Sprintf("progress.txt entries=%d latest=%s", entries, latest)
}

func readNewEventLines(path string, offset int64) (int64, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return offset, nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return offset, nil, err
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return offset, nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return offset, nil, err
	}
	return info.Size(), lines, nil
}

func summarizeLatestRunFailure(store *relay.Store, issueID string) string {
	runDir := store.RunDir(issueID)
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return ""
	}

	type candidate struct {
		path string
		time time.Time
	}
	var stderrCandidates []candidate
	var finalCandidates []candidate
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(runDir, entry.Name())
		switch {
		case strings.HasSuffix(entry.Name(), ".stderr.log"):
			stderrCandidates = append(stderrCandidates, candidate{path: path, time: info.ModTime()})
		case strings.HasSuffix(entry.Name(), ".final.txt"):
			finalCandidates = append(finalCandidates, candidate{path: path, time: info.ModTime()})
		}
	}
	sort.Slice(stderrCandidates, func(i, j int) bool { return stderrCandidates[i].time.After(stderrCandidates[j].time) })
	sort.Slice(finalCandidates, func(i, j int) bool { return finalCandidates[i].time.After(finalCandidates[j].time) })

	for _, set := range [][]candidate{stderrCandidates, finalCandidates} {
		for _, candidate := range set {
			data, err := os.ReadFile(candidate.path)
			if err != nil {
				continue
			}
			summary := summarizeLogSnippet(string(data))
			if summary != "" {
				return filepath.Base(candidate.path) + ": " + summary
			}
		}
	}
	return ""
}

func summarizeLogSnippet(value string) string {
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line
		}
	}
	return ""
}
