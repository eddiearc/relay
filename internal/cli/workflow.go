package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eddiearc/relay/internal/relay"
	"gopkg.in/yaml.v3"
)

type repoIdentity struct {
	RepoPath    string
	GitRoot     string
	ProjectKey  string
	ProjectPath string
	RemoteURL   string
}

type pipelineMatch struct {
	Pipeline relay.Pipeline
	Reason   string
}

type issueEvaluationDimension struct {
	Name   string
	Ready  bool
	Reason string
	Action string
}

type issueEvaluationResult struct {
	Ready      bool
	Dimensions []issueEvaluationDimension
}

func runPipelineMatch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline match", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineMatchUsage)
		fs.PrintDefaults()
	}
	repoPath := fs.String("repo", "", "repository path or subdirectory to match")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *repoPath == "" {
		_, _ = io.WriteString(stderr, "pipeline match requires --repo\n")
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = io.WriteString(stderr, "pipeline match does not take positional arguments\n")
		return 1
	}

	identity, err := inspectRepo(*repoPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "inspect repo: %v\n", err)
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	pipelines, err := store.ListPipelines()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list pipelines: %v\n", err)
		return 1
	}
	matches, exactCount, fallbackCount := matchPipelines(pipelines, identity.ProjectKey, identity.ProjectPath)
	switch len(matches) {
	case 0:
		_, _ = fmt.Fprintf(stdout, "no pipeline matched repo %s\n", identity.RepoPath)
		_, _ = fmt.Fprintf(stdout, "project_key=%s\nproject_path=%s\nremote_url=%s\nrepo_root=%s\n", identity.ProjectKey, identity.ProjectPath, identity.RemoteURL, identity.GitRoot)
		return 2
	case 1:
		match := matches[0]
		_, _ = fmt.Fprintf(stdout, "matched_pipeline=%s\nmatch_reason=%s\nproject_key=%s\nproject_path=%s\nremote_url=%s\nrepo_root=%s\nrepo_path=%s\n", match.Pipeline.Name, match.Reason, identity.ProjectKey, identity.ProjectPath, identity.RemoteURL, identity.GitRoot, identity.RepoPath)
		return 0
	default:
		_, _ = fmt.Fprintf(stdout, "multiple pipeline candidates matched repo %s\n", identity.RepoPath)
		_, _ = fmt.Fprintf(stdout, "project_key=%s\nproject_path=%s\nexact_candidates=%d\nfallback_candidates=%d\n", identity.ProjectKey, identity.ProjectPath, exactCount, fallbackCount)
		for _, match := range matches {
			path := "."
			remoteURL := ""
			if match.Pipeline.Project != nil {
				path = match.Pipeline.Project.Path
				remoteURL = match.Pipeline.Project.RemoteURL
			}
			_, _ = fmt.Fprintf(stdout, "- %s\treason=%s\tproject_path=%s\tremote_url=%s\n", match.Pipeline.Name, match.Reason, path, remoteURL)
		}
		return 2
	}
}

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

	projectKey := "(unbound)"
	projectPath := "."
	remoteURL := ""
	if pipeline.Project != nil {
		projectKey = pipeline.Project.Key
		projectPath = pipeline.Project.Path
		remoteURL = pipeline.Project.RemoteURL
	}
	_, _ = fmt.Fprintf(stdout, "summary:\n")
	_, _ = fmt.Fprintf(stdout, "- name: %s\n", pipeline.Name)
	_, _ = fmt.Fprintf(stdout, "- project_key: %s\n", projectKey)
	_, _ = fmt.Fprintf(stdout, "- project_path: %s\n", projectPath)
	_, _ = fmt.Fprintf(stdout, "- remote_url: %s\n", remoteURL)
	_, _ = fmt.Fprintf(stdout, "- init_strategy: %s\n", summarizeBlock(pipeline.InitCommand))
	_, _ = fmt.Fprintf(stdout, "- loop_limit: %d\n", pipeline.LoopNum)
	_, _ = fmt.Fprintf(stdout, "- plan_constraints: %s\n", strings.Join(firstMeaningfulLines(pipeline.PlanPrompt, 3), " | "))
	_, _ = fmt.Fprintf(stdout, "- coding_constraints: %s\n", strings.Join(firstMeaningfulLines(pipeline.CodingPrompt, 3), " | "))
	_, _ = io.WriteString(stdout, "- verification_path: planner writes observable acceptance conditions into feature_list.json; coding loops update feature_list.json from verified state and append handoff notes to progress.txt\n")
	_, _ = fmt.Fprintf(stdout, "\nyaml:\n%s", string(data))
	return 0
}

func runIssueEvaluate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue evaluate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueEvaluateUsage)
		fs.PrintDefaults()
	}
	pipelineName := fs.String("pipeline", "", "pipeline name")
	goal := fs.String("goal", "", "issue goal")
	description := fs.String("description", "", "issue description")
	filePath := fs.String("file", "", "path to issue JSON")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = io.WriteString(stderr, "issue evaluate does not take positional arguments\n")
		return 1
	}

	draft, err := loadIssueDraft(*filePath, *pipelineName, *goal, *description)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue draft: %v\n", err)
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	pipeline, err := store.LoadPipeline(draft.PipelineName)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load pipeline %q: %v\n", draft.PipelineName, err)
		return 1
	}

	result := evaluateIssueDraft(draft.Goal, draft.Description)
	if result.Ready {
		_, _ = io.WriteString(stdout, "evaluation=ready\n")
	} else {
		_, _ = io.WriteString(stdout, "evaluation=not_ready\n")
	}
	_, _ = fmt.Fprintf(stdout, "pipeline=%s\n", pipeline.Name)
	for _, dimension := range result.Dimensions {
		state := "fail"
		if dimension.Ready {
			state = "pass"
		}
		_, _ = fmt.Fprintf(stdout, "%s=%s: %s\n", dimension.Name, state, dimension.Reason)
	}
	if result.Ready {
		_, _ = io.WriteString(stdout, "\nexecution_preview:\n")
		_, _ = fmt.Fprintf(stdout, "- pipeline: %s\n", pipeline.Name)
		_, _ = fmt.Fprintf(stdout, "- init_behavior: %s\n", summarizeBlock(pipeline.InitCommand))
		_, _ = fmt.Fprintf(stdout, "- loop_upper_bound: %d\n", pipeline.LoopNum)
		_, _ = io.WriteString(stdout, "- planner_output: planner will turn the goal and description into feature_list.json with observable acceptance conditions\n")
		_, _ = io.WriteString(stdout, "- coding_output: coding loops will update feature_list.json from verified state and append concise handoff entries to progress.txt\n")
		_, _ = io.WriteString(stdout, "- monitor_with: relay watch -issue <new-issue-id>\n")
		return 0
	}

	_, _ = io.WriteString(stdout, "\nrequired_changes:\n")
	for _, dimension := range result.Dimensions {
		if !dimension.Ready && dimension.Action != "" {
			_, _ = fmt.Fprintf(stdout, "- %s\n", dimension.Action)
		}
	}
	return 2
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

func loadIssueDraft(filePath, pipelineName, goal, description string) (relay.Issue, error) {
	if filePath != "" {
		if pipelineName != "" || goal != "" || description != "" {
			return relay.Issue{}, errors.New("use either -file or --pipeline/--goal/--description")
		}
		return relay.LoadIssue(filePath)
	}
	if pipelineName == "" {
		return relay.Issue{}, errors.New("--pipeline is required")
	}
	if goal == "" {
		return relay.Issue{}, errors.New("--goal is required")
	}
	return relay.Issue{
		PipelineName: pipelineName,
		Goal:         goal,
		Description:  description,
		Status:       relay.IssueStatusTodo,
	}, nil
}

func evaluateIssueDraft(goal, description string) issueEvaluationResult {
	goalReady := isConcreteGoal(goal)
	verificationReady := hasVerificationSignals(description)
	scopeReady := hasScopeConstraintsAndNonGoals(description)
	readinessReady := goalReady && verificationReady && scopeReady && enoughExecutionDetail(description)

	result := issueEvaluationResult{
		Ready: goalReady && verificationReady && scopeReady && readinessReady,
		Dimensions: []issueEvaluationDimension{
			{
				Name:   "goal_clarity",
				Ready:  goalReady,
				Reason: chooseReason(goalReady, "goal states a concrete end state Relay can aim for", "rewrite the goal as a concrete end state instead of a vague improvement request"),
				Action: "Rewrite the goal so it names the concrete end state, not just a vague intent.",
			},
			{
				Name:   "verification_specificity",
				Ready:  verificationReady,
				Reason: chooseReason(verificationReady, "description includes externally visible or command-level verification signals", "add explicit verification commands, response checks, UI checks, files, or logs"),
				Action: "Add explicit verification commands or observable external behavior the worker can prove.",
			},
			{
				Name:   "scope_constraints_non_goals",
				Ready:  scopeReady,
				Reason: chooseReason(scopeReady, "description calls out scope limits, constraints, and non-goals", "add explicit scope limits, constraints, and non-goals"),
				Action: "Call out scope limits, constraints, and at least one explicit non-goal or exclusion.",
			},
			{
				Name:   "execution_readiness",
				Ready:  readinessReady,
				Reason: chooseReason(readinessReady, "issue has enough detail to enter Relay without immediate rework", "issue still lacks enough operational detail to enter Relay safely"),
				Action: "Add enough operational detail for the planner to produce stable, verifiable features.",
			},
		},
	}
	return result
}

func inspectRepo(path string) (repoIdentity, error) {
	repoPath := resolvePath(path)
	rootOut, err := exec.Command("git", "-C", repoPath, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return repoIdentity{}, fmt.Errorf("resolve git root: %w: %s", err, strings.TrimSpace(string(rootOut)))
	}
	gitRoot := strings.TrimSpace(string(rootOut))
	if gitRoot == "" {
		return repoIdentity{}, errors.New("empty git root")
	}

	remoteOut, err := exec.Command("git", "-C", repoPath, "config", "--get", "remote.origin.url").CombinedOutput()
	remoteURL := strings.TrimSpace(string(remoteOut))
	if err != nil {
		remoteURL = ""
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		absRepoPath = repoPath
	}
	if resolved, err := filepath.EvalSymlinks(absRepoPath); err == nil {
		absRepoPath = resolved
	}
	absGitRoot, err := filepath.Abs(gitRoot)
	if err != nil {
		absGitRoot = gitRoot
	}
	if resolved, err := filepath.EvalSymlinks(absGitRoot); err == nil {
		absGitRoot = resolved
	}
	relPath, err := filepath.Rel(absGitRoot, absRepoPath)
	if err != nil {
		return repoIdentity{}, fmt.Errorf("derive repo-relative path: %w", err)
	}

	projectKey := normalizeRemoteProjectKey(remoteURL)
	if projectKey == "" {
		projectKey = stableLocalProjectKey(absGitRoot)
	}

	return repoIdentity{
		RepoPath:    absRepoPath,
		GitRoot:     absGitRoot,
		ProjectKey:  projectKey,
		ProjectPath: normalizeRepoRelativePath(relPath),
		RemoteURL:   remoteURL,
	}, nil
}

func normalizeRemoteProjectKey(remote string) string {
	trimmed := strings.TrimSpace(remote)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err == nil && parsed.Host != "" {
			host := strings.ToLower(parsed.Hostname())
			path := strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), ".git")
			path = strings.Trim(path, "/")
			if host != "" && path != "" {
				return host + "/" + path
			}
		}
	}
	if at := strings.Index(trimmed, "@"); at >= 0 && strings.Contains(trimmed[at+1:], ":") {
		parts := strings.SplitN(trimmed[at+1:], ":", 2)
		host := strings.ToLower(strings.TrimSpace(parts[0]))
		path := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
		path = strings.Trim(path, "/")
		if host != "" && path != "" {
			return host + "/" + path
		}
	}
	return ""
}

func stableLocalProjectKey(root string) string {
	clean := strings.Trim(filepath.ToSlash(filepath.Clean(root)), "/")
	return "local/" + clean
}

func normalizeRepoRelativePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == "" {
		return "."
	}
	return strings.TrimPrefix(clean, "./")
}

func matchPipelines(pipelines []relay.Pipeline, projectKey, projectPath string) ([]pipelineMatch, int, int) {
	var exact []pipelineMatch
	var fallback []pipelineMatch
	for _, pipeline := range pipelines {
		if pipeline.Project == nil {
			continue
		}
		if pipeline.Project.Key != projectKey {
			continue
		}
		switch pipeline.Project.Path {
		case projectPath:
			exact = append(exact, pipelineMatch{Pipeline: pipeline, Reason: "exact-project-path"})
		case ".":
			fallback = append(fallback, pipelineMatch{Pipeline: pipeline, Reason: "repo-fallback"})
		}
	}
	if len(exact) > 0 {
		return exact, len(exact), len(fallback)
	}
	return fallback, 0, len(fallback)
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

func chooseReason(ready bool, good, bad string) string {
	if ready {
		return good
	}
	return bad
}

func isConcreteGoal(goal string) bool {
	trimmed := strings.TrimSpace(goal)
	if trimmed == "" {
		return false
	}
	if utf8Len(trimmed) < 12 && len(strings.Fields(trimmed)) < 3 {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, vague := range []string{
		"fix stuff",
		"improve code",
		"handle it",
		"support it",
		"make it better",
		"optimize it",
		"implement feature",
	} {
		if strings.Contains(lower, vague) {
			return false
		}
	}
	return true
}

func hasVerificationSignals(description string) bool {
	lower := strings.ToLower(description)
	for _, needle := range []string{
		"go test", "npm test", "pnpm test", "yarn test", "pytest", "cargo test",
		"npm run build", "pnpm build", "yarn build", "tsc --noemit", "lint",
		"curl", "http", "status code", "response", "returns", "renders",
		"screenshot", "log", "events.log", "progress.txt", "feature_list.json",
		"file", "stdout", "stderr", "verify", "validation",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func hasScopeConstraintsAndNonGoals(description string) bool {
	lower := strings.ToLower(description)
	hasNonGoal := false
	for _, needle := range []string{"non-goal", "non goal", "out of scope", "not required", "no need", "exclude"} {
		if strings.Contains(lower, needle) {
			hasNonGoal = true
			break
		}
	}
	if !hasNonGoal {
		return false
	}
	for _, needle := range []string{"must", "must not", "without", "keep", "preserve", "do not", "don't", "avoid", "only", "touch", "scope"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func enoughExecutionDetail(description string) bool {
	trimmed := strings.TrimSpace(description)
	if utf8Len(trimmed) < 80 {
		return false
	}
	return strings.Contains(trimmed, ".") || strings.Contains(trimmed, "\n") || strings.Contains(trimmed, ";")
}

func utf8Len(value string) int {
	return len([]rune(value))
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
