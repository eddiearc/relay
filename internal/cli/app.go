package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/eddiearc/relay/internal/relay"
)

var usage = `relay is a goal-driven supervisor CLI.

Usage:
  relay <command> [arguments]
  relay help [command] [subcommand]

Workflow:
  1. relay help
  2. relay version
  3. relay upgrade --check
  4. relay help pipeline
  5. relay help issue
  6. relay serve --once

Examples:
  relay help
  relay version
  relay upgrade --check
  relay pipeline import -file pipeline.yaml
  relay pipeline show demo
  relay issue add --pipeline demo --goal "Implement X" --description "Scope and verification."
  relay serve --once
  relay watch -issue issue-123
  relay status -issue issue-123
  relay report -issue issue-123

Commands:
  serve    Start the polling orchestrator
  watch    Follow one issue's persisted state and logs
  pipeline Create, inspect, and print pipeline templates
  issue    Create, inspect, and print issue templates
  status   Show saved issue status
  report   Print a saved issue report
  kill     Mark a saved issue as failed
  upgrade  Upgrade the relay CLI
  version  Show build version information
  help     Show detailed help for a command or subcommand

More help:
  relay help serve
  relay help watch
  relay help pipeline add
  relay help pipeline show
  relay help pipeline template
  relay help issue add
  relay help issue template

Skill refresh:
  npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y
`

var upgradeUsage = `upgrade the relay CLI.

Usage:
  relay upgrade
  relay upgrade --check

The command detects whether relay was installed via npm or go install and
runs the matching self-update command. Local builds are not self-upgradable.

With --check, Relay only reports:
  - current version
  - install method
  - latest version
  - recommended upgrade command
  - skill refresh command

Exit codes for --check:
  0  already up to date
  2  update available
  1  check failed

If you use the bundled relay-operator skill, refresh it separately with:
  npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y

Examples:
  relay upgrade
  relay upgrade --check
`

var serveUsage = `start the polling orchestrator.

Usage:
  relay serve [flags]

What it does:
  - loads persisted pipelines and issues from the Relay state directory
  - recovers orphaned active issues
  - polls for todo issues
  - resolves agent_runner as issue override -> pipeline setting -> default codex
  - runs planning once, then coding loops until completion or failure with the resolved local runner

Recommended startup sequence:
  1. relay serve --once
  2. relay serve
  3. move the service under nohup, launchd, systemd, or another supervisor only after foreground validation succeeds

Mode selection:
  - single pass for debugging: relay serve --once
  - foreground validation: relay serve
  - lightweight background process: nohup relay serve >> ~/.relay/logs/serve.log 2>&1 &
  - production-style supervision: launchd, systemd, or another service manager

Runner requirements:
  - supported values are codex and claude
  - if neither issue nor pipeline config sets agent_runner, Relay uses codex
  - install the selected local CLI and keep it available in PATH

Diagnostic workflow:
  - relay issue list
  - relay status -issue <issue-id>
  - relay watch -issue <issue-id>
  - relay report -issue <issue-id>
  - inspect feature_list.json, progress.txt, events.log, and runs/ under the issue artifact directory
  - confirm host process state with ps, launchctl, systemctl, or journalctl as appropriate

Examples:
  relay serve --once
  relay serve --poll-interval 10s
  relay serve --workspace-root /tmp/relay-workspaces
`

var pipelineUsage = `manage persisted pipelines.

Usage:
  relay pipeline <subcommand> [arguments]
  relay help pipeline <subcommand>

Subcommands:
  add      Create a pipeline from flags and prompt files
  edit     Update a saved pipeline
  import   Import a pipeline from YAML
  show     Print a saved pipeline with a readable summary
  template Print a full pipeline YAML template
  list     List saved pipelines
  delete   Delete a saved pipeline

Typical flow:
  1. Start with "relay pipeline template" or prepare prompt files for "relay pipeline add".
  2. Save it with "relay pipeline import -file pipeline.yaml" or "relay pipeline add ...".
  3. Confirm it with "relay pipeline list".

Before writing a pipeline, inspect:
  - repository clone URL and default branch
  - whether the repo is a monorepo
  - package manager and build toolchain
  - smallest verification commands that prove progress
  - whether repo-level E2E, integration, or CLI end-to-end checks already exist
  - which reusable test scripts, skills, or fixtures can be reused
  - whether the task needs browser interaction, service startup, or local binary execution for realistic verification
  - whether codegen, Docker, DB setup, env vars, or private registries are required
  - which agent_runner should execute the task: codex, claude, or the default codex fallback

Examples:
  relay pipeline template
  relay pipeline import -file pipeline.yaml
  relay pipeline show demo
  relay pipeline add demo --init-command 'git clone --depth 1 https://github.com/owner/repo .' --agent-runner claude --plan-prompt-file plan.md --coding-prompt-file coding.md
  relay pipeline edit demo --loop-num 15 --agent-runner codex
`

var pipelineAddUsage = `create a pipeline from flags and prompt files.

Usage:
  relay pipeline add <name> --init-command <command> --plan-prompt-file <file> --coding-prompt-file <file> [flags]

Required:
  <name>                 Saved pipeline name
  --init-command         Shell command that prepares a fresh issue workspace
  --agent-runner         Optional runner override: "", codex, or claude
  --plan-prompt-file     Planner prompt template file
  --coding-prompt-file   Coding prompt template file

Pipeline rules:
  - init_command should usually create a fresh workspace checkout
  - agent_runner is optional; leave it empty to default to codex
  - prefer shallow clones unless the task needs history
  - plan_prompt should force verifiable features and branch setup before coding
  - coding_prompt should force evidence-based FEATURE_LIST_PATH updates and PROGRESS_PATH handoff entries
  - for meaningful behavior changes, default to the strongest realistic verification path for the project
  - when a narrower verification path is enough, say that explicitly and explain the exception
  - frontend pipelines should usually preserve browser-driven E2E or explicitly call out that it is missing
  - backend pipelines should usually preserve startup or deployment verification plus integration checks
  - CLI pipelines should usually preserve runnable local end-to-end command verification
  - mobile or desktop pipelines should usually preserve simulator, emulator, or UI automation coverage
  - library or SDK pipelines should usually preserve consumer-facing integration coverage
  - worker or data pipelines should usually preserve fixture-driven end-to-end job or output checks
  - infrastructure pipelines should usually preserve plan or dry-run validation plus smoke checks
  - before using a pipeline for a real task, summarize the planning focus, coding focus, verification path, and missing coverage for the user and ask whether the direction looks right
  - real repo work should usually set loop_num explicitly; 15 is a reasonable upper bound

Examples:
  relay pipeline add demo \
    --init-command 'git clone --depth 1 https://github.com/owner/repo .' \
    --agent-runner claude \
    --plan-prompt-file plan.md \
    --coding-prompt-file coding.md
`

var pipelineEditUsage = `update a saved pipeline.

Usage:
  relay pipeline edit <name> [flags]

What can be changed:
  --init-command
  --agent-runner
  --loop-num
  --plan-prompt-file
  --coding-prompt-file

Examples:
  relay pipeline edit demo --loop-num 15
  relay pipeline edit demo --agent-runner claude
  relay pipeline edit demo --coding-prompt-file coding-v2.md
`

var pipelineImportUsage = `import a pipeline from YAML.

Usage:
  relay pipeline import -file <pipeline.yaml> [flags]

Required pipeline YAML fields:
  - name
  - init_command
  - plan_prompt
  - coding_prompt

Optional pipeline YAML fields:
  - agent_runner (empty, codex, or claude; defaults to codex when omitted)
  - loop_num

Use "relay pipeline template" to print a complete starting template.

Examples:
  relay pipeline import -file pipeline.yaml
`

var pipelineShowUsage = `print a saved pipeline with a readable summary.

Usage:
  relay pipeline show <name> [flags]

What it prints:
  - init strategy summary
  - effective agent runner
  - loop limit
  - key plan and coding constraints
  - full saved YAML

Examples:
  relay pipeline show demo
`

var pipelineTemplateUsage = `print a complete pipeline YAML template.

Usage:
  relay pipeline template

What it prints:
  - a full pipeline.yaml skeleton
  - optional runner selection with codex defaulting
  - branch and PR guidance in the embedded prompts
  - default loop_num guidance suitable for real repository work

Examples:
  relay pipeline template > pipeline.yaml
`

var pipelineListUsage = `list saved pipelines.

Usage:
  relay pipeline list [flags]

Examples:
  relay pipeline list
`

var pipelineDeleteUsage = `delete a saved pipeline.

Usage:
  relay pipeline delete <name> [flags]

The command refuses to delete a pipeline that is still referenced by an active issue.

Examples:
  relay pipeline delete demo
`

var issueUsage = `manage persisted issues.

Usage:
  relay issue <subcommand> [arguments]
  relay help issue <subcommand>

Subcommands:
  add        Create an issue from flags
  edit       Update a saved issue
  interrupt  Request interruption for a running issue
  import     Import an issue from JSON
  template   Print a full issue JSON template
  list       List saved issues
  delete     Mark an inactive issue as deleted

Typical flow:
  1. Start with "relay issue template" or write flags for "relay issue add".
  2. Create or import an issue bound to an existing pipeline.
  2. Run "relay serve --once" to validate the setup.
  3. Inspect progress with "relay status" and "relay report".

Issue writing rules:
  - agent_runner is optional; when omitted, the issue inherits the pipeline runner and then codex by default
  - goal should describe the end state in one sentence
  - description should preserve scope, constraints, non-goals, and verification signals
  - acceptance criteria should come from observable commands, API behavior, UI behavior, files, or service events
  - if the task description is weak, say what is missing before creating the issue
  - treat missing repo-level verification as a harness gap, not a detail to ignore
  - avoid vague criteria such as "implemented the logic" or "mostly done"

Examples:
  relay issue template
  relay issue add --pipeline demo --agent-runner claude --goal "Implement X" --description "Scope and verification."
  relay issue list
  relay issue interrupt --id issue-123
`

var issueAddUsage = `create an issue from flags.

Usage:
  relay issue add --pipeline <name> --goal <goal> --description <description> [flags]

Required:
  --pipeline      Existing pipeline name
  --goal          One-line end state
  --description   Scope, constraints, and verification details

Optional:
  --agent-runner  Optional runner override: "", codex, or claude

Good description inputs include:
  - verification commands such as go test ./..., npm run build, or tsc --noEmit
  - expected API status codes or response bodies
  - explicit non-goals and exclusions
  - reusable scripts, skills, or fixtures that should be used for verification
  - frontend browser flows with simulated clicks when UI behavior matters
  - backend startup commands and integration checks against the running service
  - CLI command sequences that exercise the built or local binary
  - mobile or desktop app automation steps against a simulator, emulator, or packaged app shell
  - library or SDK consumer examples that prove the public API works in a real caller context
  - worker, queue, or data-pipeline fixture runs that prove emitted jobs or persisted outputs

feature_list.json rules for downstream planning:
  - Relay requires a JSON array
  - each item must use exactly: id, title, description, priority, passes, notes
  - passes can only become true after verification

Examples:
  relay issue add \
    --pipeline demo \
    --agent-runner claude \
    --goal "Add CLI summary output" \
    --description "Update the command output and verify with go test ./..."
`

var issueEditUsage = `update a saved issue.

Usage:
  relay issue edit --id <issue-id> [flags]

Examples:
  relay issue edit --id issue-123 --agent-runner codex
  relay issue edit --id issue-123 --goal "Refine summary output"
  relay issue edit --id issue-123 --description "Updated scope and verification commands"
`

var issueInterruptUsage = `interrupt an issue.

Usage:
  relay issue interrupt --id <issue-id> [flags]

If the issue is active, Relay records an interrupt request and the worker loop will stop at the next safe boundary.
If the issue is not active yet, Relay marks it interrupted immediately.

Examples:
  relay issue interrupt --id issue-123
`

var issueImportUsage = `import an issue from JSON.

Usage:
  relay issue import -file <issue.json> [flags]

Optional issue JSON fields:
  - agent_runner (empty, codex, or claude; overrides the pipeline runner)

Use "relay issue template" to print a complete starting template.

Examples:
  relay issue import -file issue.json
`

var issueTemplateUsage = `print a complete issue JSON template.

Usage:
  relay issue template

What it prints:
  - a full issue.json skeleton
  - an optional runner override field
  - the expected shape for goal and description inputs

Examples:
  relay issue template > issue.json
`

var issueListUsage = `list saved issues.

Usage:
  relay issue list [flags]

Examples:
  relay issue list
`

var issueDeleteUsage = `mark an inactive issue as deleted.

Usage:
  relay issue delete --id <issue-id> [flags]

The command refuses to delete an active issue.

Examples:
  relay issue delete --id issue-123
`

var statusUsage = `show saved issue status.

Usage:
  relay status -issue <issue-id> [flags]

What it prints:
  - issue id
  - status
  - current loop number
  - workspace path
  - workdir path
  - artifact directory
  - active phase and pids when present
  - last error and interrupt status when present

Examples:
  relay status -issue issue-123
`

var reportUsage = `print a saved issue report.

Usage:
  relay report -issue <issue-id> [flags]

What it prints:
  - the persisted issue JSON
  - artifact paths for feature_list.json, progress.txt, and events.log
  - run log file paths when available

Examples:
  relay report -issue issue-123
`

var killUsage = `mark a saved issue as failed and terminate tracked processes.

Usage:
  relay kill -issue <issue-id> [flags]

Use this when an issue is stuck and you want Relay to stop the tracked worker pids and persist a failed state.

Examples:
  relay kill -issue issue-123
`

var watchUsage = `follow one issue's persisted state and logs.

Usage:
  relay watch -issue <issue-id> [flags]

What it watches:
  - issue.json state transitions
  - progress.txt summaries
  - new events.log lines
  - latest run stderr/final output when a run fails

Exit codes:
  0  done
  2  failed or interrupted
  1  watch error

Examples:
  relay watch -issue issue-123
  relay watch -issue issue-123 --poll-interval 1s
`

var versionUsage = `show build version information.

Usage:
  relay version

Examples:
  relay version
`

var pipelineTemplateYAML = `name: repo-name
init_command: |
  set -e
  git clone --depth 1 https://github.com/owner/repo .

  if [ -f pnpm-lock.yaml ]; then
    pnpm install --frozen-lockfile
  elif [ -f package-lock.json ]; then
    npm ci
  elif [ -f yarn.lock ]; then
    yarn install --frozen-lockfile
  elif [ -f go.mod ]; then
    go mod download
  fi

  if [ -f package.json ]; then
    npm run build --if-present
  fi
agent_runner: ""
loop_num: 15
plan_prompt: |
  Read the repository before planning.
  If the current branch is main or master, create and switch to a task branch before finishing planning.
  Use a readable branch name derived from the task goal, for example relay/<short-slug>.
  Break the goal into the smallest meaningful features that can be verified.
  Each feature description must include an observable acceptance condition.
  Prefer evidence from tests, commands, API behavior, UI behavior, or generated files.
  Keep features stable across loops. Avoid vague or overlapping features.
coding_prompt: |
  Do not make task changes directly on main or master.
  Stay on the task branch created during planning. If no task branch exists yet, create one before editing code.
  Make the smallest correct change in WORKDIR_PATH.
  Verify progress with real commands where possible.
  Commit the current loop's work before finishing.
  Push the task branch before finishing.
  Ensure the branch has an open pull request. Create one if it does not exist yet; otherwise update the existing PR instead of opening duplicates.
  Update FEATURE_LIST_PATH based on verified state, not intention.
  Record evidence or blockers in notes.
  Append a concise handoff entry to PROGRESS_PATH before finishing.
`

var issueTemplateJSON = `{
  "pipeline_name": "repo-name",
  "agent_runner": "",
  "goal": "Describe the end state in one sentence.",
  "description": "Describe scope, constraints, validation commands, reusable verification assets, missing E2E or unit-test gaps, non-goals, and any known context that should shape feature planning."
}
`

func pipelineTemplateHelpText() string {
	return pipelineTemplateUsage + "\nTemplate:\n\n" + pipelineTemplateYAML
}

func issueTemplateHelpText() string {
	return issueTemplateUsage + "\nTemplate:\n\n" + issueTemplateJSON
}

type installMethod string

const (
	installMethodLocalBuild installMethod = "local-build"
	installMethodNPM        installMethod = "npm"
	installMethodGoInstall  installMethod = "go-install"
)

type upgradeCommand struct {
	name string
	args []string
}

type optionalStringFlag struct {
	value string
	set   bool
}

func (f *optionalStringFlag) String() string {
	return f.value
}

func (f *optionalStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

var newServeRunner = func(name string) (relay.AgentRunner, error) {
	return relay.NewAgentRunner(name)
}

var upgradeExecutable = os.Executable

var upgradeGoPaths = func() (string, string, error) {
	gobin := strings.TrimSpace(os.Getenv("GOBIN"))
	gopath := strings.TrimSpace(os.Getenv("GOPATH"))

	if gobin == "" {
		if out, err := exec.Command("go", "env", "GOBIN").Output(); err == nil {
			gobin = strings.TrimSpace(string(out))
		}
	}
	if gopath == "" {
		if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
			gopath = strings.TrimSpace(string(out))
		}
	}

	return gobin, gopath, nil
}

var upgradeCommandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

var upgradeVersionLookup = func(executable string) (string, error) {
	out, err := exec.Command(executable, "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", errors.New("empty version output")
	}
	firstLine := strings.Split(line, "\n")[0]
	return normalizeVersion(strings.TrimSpace(strings.TrimPrefix(firstLine, "relay "))), nil
}

var upgradeLatestVersionLookup = func(method installMethod) (string, error) {
	var out []byte
	var err error

	switch method {
	case installMethodNPM:
		out, err = exec.Command("npm", "view", "@eddiearc/relay", "version").CombinedOutput()
	case installMethodGoInstall:
		out, err = exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/eddiearc/relay@latest").CombinedOutput()
	default:
		return "", fmt.Errorf("unsupported install method %q", method)
	}
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, message)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", errors.New("empty latest version output")
	}
	firstLine := strings.Split(line, "\n")[0]
	return normalizeVersion(strings.TrimSpace(firstLine)), nil
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
func SetServeRunnerForTesting(factory func(string) (relay.AgentRunner, error)) func() {
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
	case "watch":
		return runWatch(args[1:], stdout, stderr)
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
	case "upgrade":
		return runUpgrade(args[1:], stdout, stderr)
	case "version":
		return runVersion(stdout)
	case "help", "-h", "--help":
		return runHelp(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 1
	}
}

func runVersion(stdout io.Writer) int {
	_, _ = fmt.Fprintf(stdout, "relay %s\ncommit: %s\nbuilt: %s\n", version, commit, buildDate)
	return 0
}

func runHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, usage)
		return 0
	}

	switch args[0] {
	case "serve":
		_, _ = io.WriteString(stdout, serveUsage)
		return 0
	case "watch":
		_, _ = io.WriteString(stdout, watchUsage)
		return 0
	case "pipeline":
		return runPipelineHelp(args[1:], stdout, stderr)
	case "issue":
		return runIssueHelp(args[1:], stdout, stderr)
	case "status":
		_, _ = io.WriteString(stdout, statusUsage)
		return 0
	case "report":
		_, _ = io.WriteString(stdout, reportUsage)
		return 0
	case "kill":
		_, _ = io.WriteString(stdout, killUsage)
		return 0
	case "upgrade":
		_, _ = io.WriteString(stdout, upgradeUsage)
		return 0
	case "version":
		_, _ = io.WriteString(stdout, versionUsage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown help topic %q\n\n%s", args[0], usage)
		return 1
	}
}

func runPipelineHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, pipelineUsage)
		return 0
	}
	switch args[0] {
	case "add":
		_, _ = io.WriteString(stdout, pipelineAddUsage)
		return 0
	case "edit":
		_, _ = io.WriteString(stdout, pipelineEditUsage)
		return 0
	case "import":
		_, _ = io.WriteString(stdout, pipelineImportUsage)
		return 0
	case "show":
		_, _ = io.WriteString(stdout, pipelineShowUsage)
		return 0
	case "template":
		_, _ = io.WriteString(stdout, pipelineTemplateHelpText())
		return 0
	case "list":
		_, _ = io.WriteString(stdout, pipelineListUsage)
		return 0
	case "delete":
		_, _ = io.WriteString(stdout, pipelineDeleteUsage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown pipeline help topic %q\n\n%s", args[0], pipelineUsage)
		return 1
	}
}

func runIssueHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, issueUsage)
		return 0
	}
	switch args[0] {
	case "add":
		_, _ = io.WriteString(stdout, issueAddUsage)
		return 0
	case "edit":
		_, _ = io.WriteString(stdout, issueEditUsage)
		return 0
	case "interrupt":
		_, _ = io.WriteString(stdout, issueInterruptUsage)
		return 0
	case "import":
		_, _ = io.WriteString(stdout, issueImportUsage)
		return 0
	case "template":
		_, _ = io.WriteString(stdout, issueTemplateHelpText())
		return 0
	case "list":
		_, _ = io.WriteString(stdout, issueListUsage)
		return 0
	case "delete":
		_, _ = io.WriteString(stdout, issueDeleteUsage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown issue help topic %q\n\n%s", args[0], issueUsage)
		return 1
	}
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func runUpgrade(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stdout)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, upgradeUsage)
	}
	checkOnly := fs.Bool("check", false, "print version freshness information without upgrading")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = io.WriteString(stderr, "upgrade does not take positional arguments\n")
		return 1
	}

	executable, err := upgradeExecutable()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "locate relay executable: %v\n", err)
		return 1
	}
	gobin, gopath, _ := upgradeGoPaths()
	method := detectInstallMethod(executable, gobin, gopath)
	command, ok := upgradeCommandForMethod(method)
	currentVersion := normalizeVersion(version)
	if *checkOnly {
		latestVersion, err := lookupLatestVersionForCheck(method)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "check latest relay version: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "current_version=%s\ninstall_method=%s\nlatest_version=%s\nrecommended_upgrade_command=%s\nskill_refresh_command=%s\n",
			currentVersion,
			method,
			latestVersion,
			recommendedUpgradeCommand(method, ok, command),
			"npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y",
		)
		if latestVersion == currentVersion {
			return 0
		}
		return 2
	}
	if !ok {
		_, _ = io.WriteString(stdout, "self-upgrade is unavailable for local builds; reinstall via npm or go install instead.\n")
		return 0
	}

	latestVersion, err := upgradeLatestVersionLookup(method)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "check latest relay version: %v\n", err)
		return 1
	}
	if latestVersion == currentVersion {
		_, _ = fmt.Fprintf(stdout, "Already up to date (%s)\n", currentVersion)
		return 0
	}

	out, err := upgradeCommandRunner(command.name, command.args...)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "upgrade failed while running %q: %v\n", commandString(command.name, command.args), err)
		if msg := strings.TrimSpace(string(out)); msg != "" {
			_, _ = fmt.Fprintln(stderr, msg)
		}
		_, _ = io.WriteString(stderr, "Try running the command manually to inspect permissions or network access.\n")
		return 1
	}

	newVersion, err := upgradeVersionLookup(executable)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "upgrade completed but version check failed: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Upgraded: %s → %s\n", currentVersion, newVersion)
	return 0
}

func detectInstallMethod(executable, gobin, gopath string) installMethod {
	path := filepath.ToSlash(filepath.Clean(executable))
	if strings.Contains(path, "/node_modules/@eddiearc/relay-") || strings.Contains(path, "/node_modules/@eddiearc/relay/") {
		return installMethodNPM
	}
	if gobin != "" && filepath.Clean(executable) == filepath.Join(filepath.Clean(gobin), "relay") {
		return installMethodGoInstall
	}
	if gopath != "" && filepath.Clean(executable) == filepath.Join(filepath.Clean(gopath), "bin", "relay") {
		return installMethodGoInstall
	}
	return installMethodLocalBuild
}

func upgradeCommandForMethod(method installMethod) (upgradeCommand, bool) {
	switch method {
	case installMethodNPM:
		return upgradeCommand{name: "npm", args: []string{"update", "-g", "@eddiearc/relay"}}, true
	case installMethodGoInstall:
		return upgradeCommand{name: "go", args: []string{"install", "github.com/eddiearc/relay/cmd/relay@latest"}}, true
	default:
		return upgradeCommand{}, false
	}
}

func lookupLatestVersionForCheck(method installMethod) (string, error) {
	if method == installMethodLocalBuild {
		if latest, err := upgradeLatestVersionLookup(installMethodNPM); err == nil {
			return latest, nil
		}
		return upgradeLatestVersionLookup(installMethodGoInstall)
	}
	return upgradeLatestVersionLookup(method)
}

func recommendedUpgradeCommand(method installMethod, ok bool, command upgradeCommand) string {
	if ok {
		return commandString(command.name, command.args)
	}
	if method == installMethodLocalBuild {
		return "reinstall via npm or go install"
	}
	return "unavailable"
}

func commandString(name string, args []string) string {
	return strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
}

func normalizeVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "v") {
		return trimmed
	}
	if trimmed[0] >= '0' && trimmed[0] <= '9' {
		return "v" + trimmed
	}
	return trimmed
}

func setUpgradeExecutableForTesting(fn func() (string, error)) func() {
	previous := upgradeExecutable
	upgradeExecutable = fn
	return func() {
		upgradeExecutable = previous
	}
}

func setUpgradeGoPathsForTesting(fn func() (string, string, error)) func() {
	previous := upgradeGoPaths
	upgradeGoPaths = fn
	return func() {
		upgradeGoPaths = previous
	}
}

func setUpgradeCommandRunnerForTesting(fn func(name string, args ...string) ([]byte, error)) func() {
	previous := upgradeCommandRunner
	upgradeCommandRunner = fn
	return func() {
		upgradeCommandRunner = previous
	}
}

func setUpgradeVersionLookupForTesting(fn func(executable string) (string, error)) func() {
	previous := upgradeVersionLookup
	upgradeVersionLookup = fn
	return func() {
		upgradeVersionLookup = previous
	}
}

func setUpgradeLatestVersionLookupForTesting(fn func(method installMethod) (string, error)) func() {
	previous := upgradeLatestVersionLookup
	upgradeLatestVersionLookup = fn
	return func() {
		upgradeLatestVersionLookup = previous
	}
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, serveUsage)
		fs.PrintDefaults()
	}
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	workspaceRoot := fs.String("workspace-root", "", "directory for relay workspaces (default: ~/relay-workspaces or RELAY_WORKSPACE_ROOT)")
	pollInterval := fs.Duration("poll-interval", 5*time.Second, "issue polling interval")
	runOnce := fs.Bool("once", false, "process the current todo queue once and exit")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	orchestrator := relay.NewOrchestrator(store, relay.ZshRunner{}, nil)
	ctx := context.Background()
	if recovered, err := recoverActiveIssues(store); err != nil {
		_, _ = fmt.Fprintf(stderr, "recover active issues: %v\n", err)
		return 1
	} else if recovered > 0 {
		_, _ = fmt.Fprintf(stdout, "recovered %d orphaned active issue(s)\n", recovered)
	}
	for {
		processed, failed := processTodoIssues(ctx, orchestrator, store, *stateDir, stdout, stderr)
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
		_, _ = io.WriteString(stdout, pipelineUsage)
		return 0
	}
	if isHelpArg(args[0]) {
		return runPipelineHelp(args[1:], stdout, stderr)
	}
	switch args[0] {
	case "add":
		return runPipelineAdd(args[1:], stdout, stderr)
	case "edit":
		return runPipelineEdit(args[1:], stdout, stderr)
	case "import":
		return runPipelineImport(args[1:], stdout, stderr)
	case "show":
		return runPipelineShow(args[1:], stdout, stderr)
	case "template":
		return runPipelineTemplate(args[1:], stdout, stderr)
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineAddUsage)
		fs.PrintDefaults()
	}
	loopNum := fs.Int("loop-num", relay.DefaultLoopNum, "maximum coding loop iterations")
	initCommand := fs.String("init-command", "", "shell command used to initialize the workspace repository")
	agentRunner := fs.String("agent-runner", "", `optional agent runner override: "", codex, or claude`)
	planPromptFile := fs.String("plan-prompt-file", "", "path to plan prompt template file")
	codingPromptFile := fs.String("coding-prompt-file", "", "path to coding prompt template file")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	resolvedRunner := *agentRunner
	if resolvedRunner == "" {
		resolvedRunner = relay.DetectDefaultRunner()
		if resolvedRunner != "" {
			_, _ = fmt.Fprintf(stderr, "detected agent runner: %s\n", resolvedRunner)
		}
		available := relay.DetectAvailableRunners()
		if len(available) > 1 {
			_, _ = fmt.Fprintf(stderr, "other available runners: %s (use --agent-runner to override)\n", strings.Join(available[1:], ", "))
		}
		if len(available) == 0 {
			_, _ = fmt.Fprintf(stderr, "warning: no agent runner found in PATH (codex or claude); serve will fail unless one is installed\n")
		}
	}
	pipeline := relay.Pipeline{
		Name:         fs.Arg(0),
		InitCommand:  *initCommand,
		AgentRunner:  resolvedRunner,
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
	writePipelineContinuationHints(stderr, pipeline.Name, *stateDir)
	return 0
}

func runPipelineEdit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineEditUsage)
		fs.PrintDefaults()
	}
	loopNum := fs.Int("loop-num", 0, "maximum coding loop iterations")
	initCommand := fs.String("init-command", "", "shell command used to initialize the workspace repository")
	var agentRunner optionalStringFlag
	fs.Var(&agentRunner, "agent-runner", `optional agent runner override: "", codex, or claude`)
	planPromptFile := fs.String("plan-prompt-file", "", "path to plan prompt template file")
	codingPromptFile := fs.String("coding-prompt-file", "", "path to coding prompt template file")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	if agentRunner.set {
		pipeline.AgentRunner = agentRunner.value
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
	writePipelineContinuationHints(stderr, pipeline.Name, *stateDir)
	return 0
}

func runPipelineImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineImportUsage)
		fs.PrintDefaults()
	}
	filePath := fs.String("file", "", "path to pipeline YAML")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	writePipelineContinuationHints(stderr, pipeline.Name, *stateDir)
	return 0
}

func runPipelineTemplate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline template", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineTemplateHelpText())
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = io.WriteString(stderr, "pipeline template does not take positional arguments\n")
		return 1
	}
	_, _ = io.WriteString(stdout, pipelineTemplateYAML)
	return 0
}

func runPipelineDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pipeline delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineDeleteUsage)
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, pipelineListUsage)
		fs.PrintDefaults()
	}
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	firstName := ""
	if len(pipelines) > 0 {
		firstName = pipelines[0].Name
	}
	writePipelineListHints(stderr, firstName, *stateDir)
	return 0
}

func runIssue(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, issueUsage)
		return 0
	}
	if isHelpArg(args[0]) {
		return runIssueHelp(args[1:], stdout, stderr)
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
	case "template":
		return runIssueTemplate(args[1:], stdout, stderr)
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueAddUsage)
		fs.PrintDefaults()
	}
	id := fs.String("id", "", "optional issue id")
	pipelineName := fs.String("pipeline", "", "pipeline name")
	agentRunner := fs.String("agent-runner", "", `optional agent runner override: "", codex, or claude`)
	goal := fs.String("goal", "", "issue goal")
	description := fs.String("description", "", "issue description")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	issue := relay.Issue{
		ID:           *id,
		PipelineName: *pipelineName,
		AgentRunner:  *agentRunner,
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
	writeIssueExecutionHints(stderr, issue.ID, *stateDir)
	return 0
}

func runIssueEdit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueEditUsage)
		fs.PrintDefaults()
	}
	id := fs.String("id", "", "issue id")
	pipelineName := fs.String("pipeline", "", "pipeline name")
	var agentRunner optionalStringFlag
	fs.Var(&agentRunner, "agent-runner", `optional agent runner override: "", codex, or claude`)
	goal := fs.String("goal", "", "issue goal")
	description := fs.String("description", "", "issue description")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	if agentRunner.set {
		issue.AgentRunner = agentRunner.value
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueInterruptUsage)
		fs.PrintDefaults()
	}
	id := fs.String("id", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueImportUsage)
		fs.PrintDefaults()
	}
	filePath := fs.String("file", "", "path to issue JSON")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	writeIssueExecutionHints(stderr, issue.ID, *stateDir)
	return 0
}

func runIssueTemplate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue template", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueTemplateHelpText())
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		_, _ = io.WriteString(stderr, "issue template does not take positional arguments\n")
		return 1
	}
	_, _ = io.WriteString(stdout, issueTemplateJSON)
	return 0
}

func runIssueDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueDeleteUsage)
		fs.PrintDefaults()
	}
	id := fs.String("id", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, issueListUsage)
		fs.PrintDefaults()
	}
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, statusUsage)
		fs.PrintDefaults()
	}
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	_, _ = fmt.Fprintf(stdout, "issue=%s status=%s loop=%d workdir=%s workspace=%s artifact=%s\n", issue.ID, issue.Status, issue.CurrentLoop, issue.WorkdirPath, issue.WorkspacePath, issue.ArtifactDir)
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, reportUsage)
		fs.PrintDefaults()
	}
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	fs.Usage = func() {
		_, _ = io.WriteString(stdout, killUsage)
		fs.PrintDefaults()
	}
	issueID := fs.String("issue", "", "issue id")
	stateDir := fs.String("state-dir", "", "directory for relay state (default: ~/.relay)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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

func processTodoIssues(ctx context.Context, orchestrator *relay.Orchestrator, store *relay.Store, rawStateDir string, stdout, stderr io.Writer) (processed bool, failed bool) {
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
		writeServeWatchHint(stderr, issue, rawStateDir)
		pipeline, err := store.LoadPipeline(issue.PipelineName)
		if err != nil {
			issue.Status = relay.IssueStatusFailed
			issue.LastError = fmt.Sprintf("load pipeline %q: %v", issue.PipelineName, err)
			_ = store.SaveIssue(issue)
			_, _ = fmt.Fprintf(stderr, "issue %s failed: %s\n", issue.ID, issue.LastError)
			failed = true
			continue
		}
		runnerName, err := relay.ResolveAgentRunner(issue.AgentRunner, pipeline.AgentRunner)
		if err != nil {
			issue.Status = relay.IssueStatusFailed
			issue.LastError = err.Error()
			_ = store.SaveIssue(issue)
			_, _ = fmt.Fprintf(stderr, "issue %s failed: %s\n", issue.ID, issue.LastError)
			failed = true
			continue
		}
		runner, err := newServeRunner(runnerName)
		if err != nil {
			issue.Status = relay.IssueStatusFailed
			issue.LastError = err.Error()
			_ = store.SaveIssue(issue)
			_, _ = fmt.Fprintf(stderr, "issue %s failed: %s\n", issue.ID, issue.LastError)
			failed = true
			continue
		}
		orchestrator.Runner = runner
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
