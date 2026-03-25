# Relay

English | [中文](./README.zh-CN.md)

### What Relay Is

Relay is an agent-first, CLI-first harness framework for long-running software-engineering agents.

It is not built around a single prompt-response loop. It is built around execution structure: persistent task state, repo initialization, planning before coding, looped fresh-agent runs, and explicit completion checks.

Relay is designed for coding tasks that need more than one model turn and more than one context window.

Fastest way to think about it:

- agent-first: the primary UX is to let an agent operate Relay correctly
- CLI-first: the system is controlled through explicit commands and persisted state
- harness framework: Relay provides orchestration, memory, verification, and recovery around coding agents

### Quick Start

Preferred path for agent users: install the skill globally with `npx skills`, then use Relay through that skill from any supported agent.

#### 1. Skill-First Quick Start

Install the Relay CLI globally, then install the `relay-operator` skill globally:

```bash
npm install -g @eddiearc/relay && \
npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y
```

That keeps the skill available across your agents and repositories. If you explicitly want a project-local install instead, remove `-g`.

Then prompt any agent that supports installed skills with something like:

```text
Use the installed relay-operator skill to set up Relay for <repository-path>.
First verify that relay is installed.
Then inspect the repository, write a repository-specific pipeline, rewrite the task as a Relay issue with explicit acceptance criteria, and tell me whether to run relay serve --once or relay serve persistently.
```

The installed skill will guide the agent to:

- inspect the target repository
- write a repository-specific Relay pipeline
- rewrite the task as a Relay issue with explicit acceptance criteria
- run `relay serve --once` or explain how to run `relay serve` persistently
- inspect Relay artifacts and host logs when something goes wrong

#### 2. Direct CLI Quick Start

If you want to use Relay directly, install it:

```bash
npm install -g @eddiearc/relay
```

Then create a pipeline:

```bash
relay pipeline add demo-pipeline \
  --init-command 'git clone https://example.com/repo.git app' \
  --plan-prompt-file plan.md \
  --coding-prompt-file coding.md
```

If `--loop-num` is omitted, Relay defaults to `20`.

Or import one:

```bash
relay pipeline import -file pipeline.yaml
```

Create an issue:

```bash
relay issue add \
  --pipeline demo-pipeline \
  --goal "Implement the requested feature" \
  --description "Use the repository initialized by init_command."
```

Run the orchestrator:

```bash
relay serve
```

Or process the current queue once:

```bash
relay serve --once
```

### Related Essays

Relay is strongly informed by these essays:

- OpenAI: [Harness Engineering](https://openai.com/en/index/harness-engineering/)
- Anthropic: [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

### Product Idea

Relay acts as an execution layer for coding agents:

- `pipeline` defines the project-level execution contract.
- `issue` defines one concrete task to run.
- `serve` polls the issue queue and drives planning plus coding loops.
- `feature_list.json` is the structured source of truth for completion.
- `progress.txt` is the handoff log between runs.
- `events.log`, `runs/`, and `issue.json` make failures inspectable.

The current runner executes the local `codex` CLI directly.

### Bundled Agent Skill

This repository ships a project-local skill for other agents that need to operate the Relay CLI:

```text
skills/relay-operator/
```

That skill covers repository-specific pipeline authoring, issue decomposition with explicit acceptance criteria, persistent `relay serve` operation, and artifact plus host-log inspection.

For skill installation, prefer the `npx skills` distribution flow instead of manually copying files.

### Inspiration

Relay turns those ideas into a concrete product model for real repositories.

The main product takeaways are:

- The harness matters more than the raw prompt.
- Long-running work needs external memory.
- Completion should come from structured state, not just model narration.
- Each run can be fresh if the right artifacts are persisted.
- Real repo execution and verification matter more than toy demos.

### Core Concepts

#### Pipeline

Project-level configuration stored at:

```text
~/.relay/pipelines/<name>.yaml
```

Fields:

- `name`
- `init_command`
- `loop_num` (optional, defaults to `20`)
- `plan_prompt`
- `coding_prompt`

#### Issue

Task-level control object stored at:

```text
~/.relay/issues/<issue-id>/issue.json
```

Important fields:

- `id`
- `pipeline_name`
- `goal`
- `description`
- `status`
- `current_loop`
- `artifact_dir`
- `workspace_path`
- `workdir_path`
- `last_error`
- `interrupt_requested`

#### Workspace

Each issue gets its own temporary workspace, by default under:

```text
~/relay-workspaces/<issue-id>-<hash>/
```

`init_command` runs there, and Relay then persists the resulting `workdir_path` for later agent runs.

#### Issue Artifacts

Each issue keeps its durable execution state under:

```text
~/.relay/issues/<issue-id>/
  issue.json
  feature_list.json
  progress.txt
  events.log
  runs/
```

Where:

- `feature_list.json` is the source of truth for completion.
- `progress.txt` is the handoff log.
- `events.log` records orchestrator-level events.
- `runs/` stores stdout, stderr, and final messages for planning and coding runs.

### Execution Flow

When `relay serve` finds a `todo` issue, it runs this fixed flow:

1. Create a workspace for the issue.
2. Run `init_command` inside the workspace.
3. Record the final working directory after `init_command`.
4. Launch a dedicated planning agent.
5. Have the planning agent create `feature_list.json` and `progress.txt`.
6. Validate those artifacts.
7. Enter the coding loop.
8. Start a fresh coding agent for each loop.
9. Re-read `feature_list.json` after each run.
10. Mark the issue `done` only when every feature passes.

### Why `feature_list.json` and `progress.txt` Are Separate

- `feature_list.json` answers: is the task complete?
- `progress.txt` answers: what should the next run know?

That separation keeps completion logic out of free-form natural language.

### Install

Fastest install for end users:

```bash
npm install -g @eddiearc/relay
```

Requirements:

- macOS or Linux
- `codex` installed and available in `PATH`

If you prefer building from source:

```bash
go install github.com/eddiearc/relay/cmd/relay@latest
```

### Commands

- `relay pipeline add <name> --init-command ... --plan-prompt-file ... --coding-prompt-file ...`
- `relay pipeline edit <name> [--init-command ...] [--loop-num ...] [--plan-prompt-file ...] [--coding-prompt-file ...]`
- `relay pipeline import -file pipeline.yaml`
- `relay pipeline list`
- `relay pipeline delete <name>`
- `relay issue add --pipeline ... --goal ... --description ...`
- `relay issue edit --id ... [--pipeline ...] [--goal ...] [--description ...]`
- `relay issue import -file issue.json`
- `relay issue list`
- `relay issue interrupt --id ...`
- `relay issue delete --id ...`
- `relay serve [--workspace-root /path/to/workspaces]`
- `relay serve --once`
- `relay status -issue <issue-id>`
- `relay report -issue <issue-id>`
- `relay kill -issue <issue-id>`
- `relay version`

### Build and Release

Build the local CLI binary:

```bash
make build
```

Show the embedded build metadata:

```bash
./bin/relay version
```

Create a local release archive for the current platform:

```bash
make package
```

Build all release archives that match CI:

```bash
make package-all
```

Generate npm packages from those release archives:

```bash
npm --prefix npm run prepare-release -- \
  --version v0.1.0 \
  --dist-dir "$PWD/dist" \
  --out-dir "$PWD/npm/out"
```

Publish the generated npm packages:

```bash
npm --prefix npm run publish-release -- \
  --version v0.1.0 \
  --packages-dir "$PWD/npm/out"
```

Versioning is controlled by git tags in CI. The GitHub Actions workflow:

- reuses the local `Makefile`, so CI and local packaging stay aligned
- runs only when a GitHub Release is published
- runs `make test`
- runs `make package-all VERSION=<release-tag>`
- uploads `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` archives to that release
- generates npm packages from those release archives
- publishes the four platform packages first, then publishes `@eddiearc/relay`

If you want a local build with explicit version metadata, use:

```bash
make package VERSION=v0.1.0
```

To trigger packaging in GitHub, publish a release for a tag such as `v0.1.0`. For example:

```bash
gh release create v0.1.0 --generate-notes
```

For the npm package layout and registry setup, see [`npm/README.md`](./npm/README.md).

The preferred npm publishing mode is Trusted Publishing via GitHub Actions OIDC. The release workflow already includes `id-token: write`; configure Trusted Publisher for each `@eddiearc/*` package in npm using workflow filename `release.yml`.

Windows packages are not published yet because the current runtime assumes Unix tools such as `zsh` and `SIGKILL`.

### Scope

Relay is currently a focused harness for real coding tasks on real repositories.

The goal is not to be a generic chat shell. The goal is to provide a durable execution model for software-engineering agents working across planning, coding, persistence, and verification.
