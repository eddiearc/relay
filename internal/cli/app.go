package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/eddiearc/relay/internal/relay"
)

var usage = `relay is a goal-driven supervisor CLI.

Usage:
  relay <command> [arguments]

Commands:
  serve    Start the polling orchestrator
  pipeline Add a persisted pipeline
  issue    Add or inspect issues
  status   Show saved issue status
  report   Print a saved issue report
  kill     Mark a saved issue as failed
  version  Show build version information
  help     Show this help text
`

var newServeRunner = func() relay.AgentRunner {
	return relay.CodexRunner{}
}

// Run executes the relay CLI and returns a process exit code.
func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

// RunWithIO executes the relay CLI with explicit stdout/stderr writers.
func RunWithIO(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr)
}

// SetServeRunnerForTesting overrides the serve runner factory until the returned restore function is called.
func SetServeRunnerForTesting(factory func() relay.AgentRunner) func() {
	previous := newServeRunner
	newServeRunner = factory
	return func() {
		newServeRunner = previous
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 1
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "pipeline":
		return runPipeline(args[1:], stdout, stderr)
	case "issue":
		return runIssue(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "kill":
		return runKill(args[1:], stdout, stderr)
	case "version":
		return runVersion(stdout)
	case "help", "-h", "--help":
		_, _ = io.WriteString(stdout, usage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 1
	}
}

func runVersion(stdout io.Writer) int {
	_, _ = fmt.Fprintf(stdout, "relay %s\ncommit: %s\nbuilt: %s\n", version, commit, buildDate)
	return 0
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	workspaceRoot := fs.String("workspace-root", "", "directory for relay workspaces (default: ~/relay-workspaces or RELAY_WORKSPACE_ROOT)")
	pollInterval := fs.Duration("poll-interval", 5*time.Second, "issue polling interval")
	runOnce := fs.Bool("once", false, "process the current todo queue once and exit")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	if *workspaceRoot != "" {
		store.WorkspaceRoot = resolvePath(*workspaceRoot)
	}
	if err := store.Ensure(); err != nil {
		_, _ = fmt.Fprintf(stderr, "prepare state dir: %v\n", err)
		return 1
	}
	orchestrator := relay.NewOrchestrator(store, relay.ZshRunner{}, newServeRunner())
	ctx := context.Background()
	if recovered, err := recoverActiveIssues(store); err != nil {
		_, _ = fmt.Fprintf(stderr, "recover active issues: %v\n", err)
		return 1
	} else if recovered > 0 {
		_, _ = fmt.Fprintf(stdout, "recovered %d orphaned active issue(s)\n", recovered)
	}
	for {
		processed, failed := processTodoIssues(ctx, orchestrator, store, stdout, stderr)
		if *runOnce {
			if failed {
				return 1
			}
			return 0
		}
		if !processed {
			time.Sleep(*pollInterval)
		}
	}
}

func runPipeline(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, "pipeline requires a subcommand\n")
		return 1
	}
	switch args[0] {
	case "add":
		return runPipelineAdd(args[1:], stdout, stderr)
	case "edit":
		return runPipelineEdit(args[1:], stdout, stderr)
	case "import":
		return runPipelineImport(args[1:], stdout, stderr)
	case "list":
		return runPipelineList(args[1:], stdout, stderr)
	case "delete":
		return runPipelineDelete(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown pipeline subcommand %q\n", args[0])
		return 1
	}
}

func runPipelineAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	loopNum := fs.Int("loop-num", relay.DefaultLoopNum, "maximum coding loop iterations")
	initCommand := fs.String("init-command", "", "shell command used to initialize the workspace repository")
	planPromptFile := fs.String("plan-prompt-file", "", "path to plan prompt template file")
	codingPromptFile := fs.String("coding-prompt-file", "", "path to coding prompt template file")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		_, _ = io.WriteString(stderr, "pipeline add requires a pipeline name argument\n")
		return 1
	}
	planPrompt, err := os.ReadFile(*planPromptFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read plan prompt: %v\n", err)
		return 1
	}
	codingPrompt, err := os.ReadFile(*codingPromptFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read coding prompt: %v\n", err)
		return 1
	}
	pipeline := relay.Pipeline{
		Name:         fs.Arg(0),
		InitCommand:  *initCommand,
		LoopNum:      *loopNum,
		PlanPrompt:   string(planPrompt),
		CodingPrompt: string(codingPrompt),
	}
	if err := pipeline.Normalize(); err != nil {
		_, _ = fmt.Fprintf(stderr, "build pipeline: %v\n", err)
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	if err := store.Ensure(); err != nil {
		_, _ = fmt.Fprintf(stderr, "prepare state dir: %v\n", err)
		return 1
	}
	if err := store.SavePipeline(pipeline); err != nil {
		_, _ = fmt.Fprintf(stderr, "save pipeline: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "pipeline %s saved to %s\n", pipeline.Name, store.PipelinePath(pipeline.Name))
	return 0
}

func runPipelineEdit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	loopNum := fs.Int("loop-num", 0, "maximum coding loop iterations")
	initCommand := fs.String("init-command", "", "shell command used to initialize the workspace repository")
	planPromptFile := fs.String("plan-prompt-file", "", "path to plan prompt template file")
	codingPromptFile := fs.String("coding-prompt-file", "", "path to coding prompt template file")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		_, _ = io.WriteString(stderr, "pipeline edit requires a pipeline name argument\n")
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	pipeline, err := store.LoadPipeline(fs.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load pipeline %q: %v\n", fs.Arg(0), err)
		return 1
	}
	if *initCommand != "" {
		pipeline.InitCommand = *initCommand
	}
	if *loopNum > 0 {
		pipeline.LoopNum = *loopNum
	}
	if *planPromptFile != "" {
		planPrompt, err := os.ReadFile(*planPromptFile)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "read plan prompt: %v\n", err)
			return 1
		}
		pipeline.PlanPrompt = string(planPrompt)
	}
	if *codingPromptFile != "" {
		codingPrompt, err := os.ReadFile(*codingPromptFile)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "read coding prompt: %v\n", err)
			return 1
		}
		pipeline.CodingPrompt = string(codingPrompt)
	}
	if err := pipeline.Normalize(); err != nil {
		_, _ = fmt.Fprintf(stderr, "build pipeline: %v\n", err)
		return 1
	}
	if err := store.SavePipeline(pipeline); err != nil {
		_, _ = fmt.Fprintf(stderr, "save pipeline: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "pipeline %s saved to %s\n", pipeline.Name, store.PipelinePath(pipeline.Name))
	return 0
}

func runPipelineImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filePath := fs.String("file", "", "path to pipeline YAML")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *filePath == "" {
		_, _ = io.WriteString(stderr, "pipeline import requires -file\n")
		return 1
	}
	pipeline, err := relay.LoadPipeline(*filePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load pipeline: %v\n", err)
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	if err := store.Ensure(); err != nil {
		_, _ = fmt.Fprintf(stderr, "prepare state dir: %v\n", err)
		return 1
	}
	if err := store.SavePipeline(pipeline); err != nil {
		_, _ = fmt.Fprintf(stderr, "save pipeline: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "pipeline %s imported to %s\n", pipeline.Name, store.PipelinePath(pipeline.Name))
	return 0
}

func runPipelineDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		_, _ = io.WriteString(stderr, "pipeline delete requires a pipeline name argument\n")
		return 1
	}
	name := fs.Arg(0)
	store := relay.NewStore(resolveStateDir(*stateDir))
	if _, err := store.LoadPipeline(name); err != nil {
		_, _ = fmt.Fprintf(stderr, "load pipeline %q: %v\n", name, err)
		return 1
	}
	issues, err := store.ListIssues()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list issues: %v\n", err)
		return 1
	}
	for _, issue := range issues {
		if issue.PipelineName == name && relay.IsIssueActiveStatus(issue.Status) {
			_, _ = fmt.Fprintf(stderr, "pipeline %s is still referenced by active issue %s\n", name, issue.ID)
			return 1
		}
	}
	if err := os.Remove(store.PipelinePath(name)); err != nil {
		_, _ = fmt.Fprintf(stderr, "delete pipeline: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "pipeline %s deleted\n", name)
	return 0
}

func runPipelineList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	pipelines, err := store.ListPipelines()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list pipelines: %v\n", err)
		return 1
	}
	for _, pipeline := range pipelines {
		_, _ = fmt.Fprintf(stdout, "%s\t%d\t%s\n", pipeline.Name, pipeline.LoopNum, pipeline.InitCommand)
	}
	return 0
}

func runIssue(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, "issue requires a subcommand\n")
		return 1
	}
	switch args[0] {
	case "add":
		return runIssueAdd(args[1:], stdout, stderr)
	case "edit":
		return runIssueEdit(args[1:], stdout, stderr)
	case "interrupt":
		return runIssueInterrupt(args[1:], stdout, stderr)
	case "import":
		return runIssueImport(args[1:], stdout, stderr)
	case "list":
		return runIssueList(args[1:], stdout, stderr)
	case "delete":
		return runIssueDelete(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown issue subcommand %q\n", args[0])
		return 1
	}
}

func runIssueAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "optional issue id")
	pipelineName := fs.String("pipeline", "", "pipeline name")
	goal := fs.String("goal", "", "issue goal")
	description := fs.String("description", "", "issue description")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	issue := relay.Issue{
		ID:           *id,
		PipelineName: *pipelineName,
		Goal:         *goal,
		Description:  *description,
	}
	if err := issue.Normalize(); err != nil {
		_, _ = fmt.Fprintf(stderr, "build issue: %v\n", err)
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	if err := store.Ensure(); err != nil {
		_, _ = fmt.Fprintf(stderr, "prepare state dir: %v\n", err)
		return 1
	}
	issue.ArtifactDir = store.IssueDir(issue.ID)
	if err := saveNewIssue(store, issue, stderr); err != nil {
		return 1
	}
	_ = writeIssue(stdout, issue)
	return 0
}

func runIssueEdit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "issue id")
	pipelineName := fs.String("pipeline", "", "pipeline name")
	goal := fs.String("goal", "", "issue goal")
	description := fs.String("description", "", "issue description")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		_, _ = io.WriteString(stderr, "issue edit requires --id\n")
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	issue, err := store.LoadIssue(*id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue %q: %v\n", *id, err)
		return 1
	}
	if issue.Status == relay.IssueStatusDeleted {
		_, _ = fmt.Fprintf(stderr, "issue %s is deleted\n", issue.ID)
		return 1
	}
	if *pipelineName != "" {
		if _, err := store.LoadPipeline(*pipelineName); err != nil {
			_, _ = fmt.Fprintf(stderr, "load pipeline %q: %v\n", *pipelineName, err)
			return 1
		}
		issue.PipelineName = *pipelineName
	}
	if *goal != "" {
		issue.Goal = *goal
	}
	if *description != "" {
		issue.Description = *description
	}
	if err := store.SaveIssue(issue); err != nil {
		_, _ = fmt.Fprintf(stderr, "save issue: %v\n", err)
		return 1
	}
	_ = writeIssue(stdout, issue)
	return 0
}

func runIssueInterrupt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue interrupt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		_, _ = io.WriteString(stderr, "issue interrupt requires --id\n")
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	issue, err := store.LoadIssue(*id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue %q: %v\n", *id, err)
		return 1
	}
	if issue.Status == relay.IssueStatusDeleted {
		_, _ = fmt.Fprintf(stderr, "issue %s is deleted\n", issue.ID)
		return 1
	}
	if relay.IsIssueTerminalStatus(issue.Status) {
		_, _ = fmt.Fprintf(stderr, "issue %s is already terminal with status %s\n", issue.ID, issue.Status)
		return 1
	}
	if relay.IsIssueActiveStatus(issue.Status) {
		issue.InterruptRequested = true
		issue.LastError = "interrupt requested by user"
	} else {
		issue.Status = relay.IssueStatusInterrupted
		issue.LastError = "interrupted by user"
		issue.InterruptRequested = false
	}
	if err := store.SaveIssue(issue); err != nil {
		_, _ = fmt.Fprintf(stderr, "save issue: %v\n", err)
		return 1
	}
	_ = writeIssue(stdout, issue)
	return 0
}

func runIssueImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filePath := fs.String("file", "", "path to issue JSON")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *filePath == "" {
		_, _ = io.WriteString(stderr, "issue import requires -file\n")
		return 1
	}

	issue, err := relay.LoadIssue(*filePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue: %v\n", err)
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	if err := store.Ensure(); err != nil {
		_, _ = fmt.Fprintf(stderr, "prepare state dir: %v\n", err)
		return 1
	}
	issue.ArtifactDir = store.IssueDir(issue.ID)
	if err := saveNewIssue(store, issue, stderr); err != nil {
		return 1
	}
	_ = writeIssue(stdout, issue)
	return 0
}

func runIssueDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		_, _ = io.WriteString(stderr, "issue delete requires --id\n")
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	issue, err := store.LoadIssue(*id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue %q: %v\n", *id, err)
		return 1
	}
	if relay.IsIssueActiveStatus(issue.Status) {
		_, _ = fmt.Fprintf(stderr, "issue %s is running and cannot be deleted\n", issue.ID)
		return 1
	}
	issue.Status = relay.IssueStatusDeleted
	issue.LastError = "deleted by user"
	if err := store.SaveIssue(issue); err != nil {
		_, _ = fmt.Fprintf(stderr, "save issue: %v\n", err)
		return 1
	}
	_ = writeIssue(stdout, issue)
	return 0
}

func saveNewIssue(store *relay.Store, issue relay.Issue, stderr io.Writer) error {
	if _, err := store.LoadPipeline(issue.PipelineName); err != nil {
		_, _ = fmt.Fprintf(stderr, "load pipeline %q: %v\n", issue.PipelineName, err)
		return err
	}
	if _, err := store.LoadIssue(issue.ID); err == nil {
		_, _ = fmt.Fprintf(stderr, "issue %s already exists\n", issue.ID)
		return errors.New("issue already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		_, _ = fmt.Fprintf(stderr, "check issue existence: %v\n", err)
		return err
	}
	if err := store.SaveIssue(issue); err != nil {
		_, _ = fmt.Fprintf(stderr, "save issue: %v\n", err)
		return err
	}
	return nil
}

func runIssueList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	store := relay.NewStore(resolveStateDir(*stateDir))
	issues, err := store.ListIssues()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list issues: %v\n", err)
		return 1
	}
	for _, issue := range issues {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", issue.ID, issue.Status, issue.PipelineName, issue.Goal)
	}
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *issueID == "" {
		_, _ = io.WriteString(stderr, "status requires -issue\n")
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	issue, err := store.LoadIssue(*issueID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "issue=%s status=%s loop=%d repo=%s workspace=%s artifact=%s\n", issue.ID, issue.Status, issue.CurrentLoop, issue.RepoPath, issue.WorkspacePath, issue.ArtifactDir)
	if issue.ActivePhase != "" {
		_, _ = fmt.Fprintf(stdout, "active_phase=%s active_pids=%v\n", issue.ActivePhase, issue.ActivePIDs)
	}
	if issue.InterruptRequested {
		_, _ = io.WriteString(stdout, "interrupt_requested=true\n")
	}
	if issue.LastError != "" {
		_, _ = fmt.Fprintf(stdout, "last_error=%s\n", issue.LastError)
	}
	return 0
}

func runReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *issueID == "" {
		_, _ = io.WriteString(stderr, "report requires -issue\n")
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	issue, err := store.LoadIssue(*issueID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue: %v\n", err)
		return 1
	}
	if err := writeIssue(stdout, issue); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "\nartifacts:\n- %s\n- %s\n- %s\n", relay.FeatureListPath(issue.ArtifactDir), relay.ProgressPath(issue.ArtifactDir), store.EventsPath(issue.ID))
	runDir := store.RunDir(issue.ID)
	entries, err := os.ReadDir(runDir)
	if err == nil {
		_, _ = fmt.Fprintf(stdout, "\nlogs:\n")
		for _, entry := range entries {
			_, _ = fmt.Fprintf(stdout, "- %s\n", filepath.Join(runDir, entry.Name()))
		}
	}
	return 0
}

func runKill(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("kill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *issueID == "" {
		_, _ = io.WriteString(stderr, "kill requires -issue\n")
		return 1
	}

	store := relay.NewStore(resolveStateDir(*stateDir))
	issue, err := store.LoadIssue(*issueID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load issue: %v\n", err)
		return 1
	}
	if err := terminateIssueProcesses(issue.ActivePIDs); err != nil {
		_, _ = fmt.Fprintf(stderr, "kill issue processes: %v\n", err)
		return 1
	}
	issue.Status = relay.IssueStatusFailed
	issue.LastError = "killed by user"
	issue.ActivePhase = ""
	issue.ActivePIDs = nil
	if err := store.SaveIssue(issue); err != nil {
		_, _ = fmt.Fprintf(stderr, "save issue: %v\n", err)
		return 1
	}
	_ = store.AppendEvent(issue.ID, "issue killed by user")
	_, _ = fmt.Fprintf(stdout, "issue %s marked as failed\n", issue.ID)
	return 0
}

func writeIssue(w io.Writer, issue relay.Issue) error {
	data, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func resolveStateDir(path string) string {
	return resolvePathWithDefault(path, func() string {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".relay")
		}
		return ".relay"
	}())
}

func resolvePath(path string) string {
	return resolvePathWithDefault(path, "")
}

func resolvePathWithDefault(path, fallback string) string {
	if path == "" {
		path = fallback
	}
	if filepath.IsAbs(path) {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	return filepath.Join(cwd, path)
}

func processTodoIssues(ctx context.Context, orchestrator *relay.Orchestrator, store *relay.Store, stdout, stderr io.Writer) (processed bool, failed bool) {
	issues, err := store.ListIssues()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list issues: %v\n", err)
		return false, true
	}
	for _, issue := range issues {
		if issue.Status != relay.IssueStatusTodo {
			continue
		}
		processed = true
		pipeline, err := store.LoadPipeline(issue.PipelineName)
		if err != nil {
			issue.Status = relay.IssueStatusFailed
			issue.LastError = fmt.Sprintf("load pipeline %q: %v", issue.PipelineName, err)
			_ = store.SaveIssue(issue)
			_, _ = fmt.Fprintf(stderr, "issue %s failed: %s\n", issue.ID, issue.LastError)
			failed = true
			continue
		}
		updated, err := orchestrator.RunIssue(ctx, pipeline, issue)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "issue %s failed: %v\n", issue.ID, err)
			failed = true
		}
		_ = writeIssue(stdout, updated)
	}
	return processed, failed
}

func recoverActiveIssues(store *relay.Store) (int, error) {
	issues, err := store.ListIssues()
	if err != nil {
		return 0, err
	}

	recovered := 0
	for _, issue := range issues {
		if !relay.IsIssueActiveStatus(issue.Status) {
			continue
		}
		issue.Status = relay.IssueStatusTodo
		issue.ActivePhase = ""
		issue.ActivePIDs = nil
		issue.InterruptRequested = false
		issue.LastError = "relay serve restarted while issue was active; previous run discarded"
		if err := store.SaveIssue(issue); err != nil {
			return recovered, err
		}
		_ = store.AppendEvent(issue.ID, "recovered orphaned active issue after service restart")
		recovered++
	}
	return recovered, nil
}

func terminateIssueProcesses(pids []int) error {
	var firstErr error
	seen := map[int]struct{}{}
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
