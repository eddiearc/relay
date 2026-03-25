# Relay

English | [中文](./README.zh-CN.md)

### What Relay Is

Relay is a Go-based harness for long-running software-engineering agents.

It is not built around a single prompt-response loop. It is built around execution structure: persistent task state, repo initialization, planning before coding, looped fresh-agent runs, and explicit completion checks.

Relay is designed for coding tasks that need more than one model turn and more than one context window.

### Product Idea

Relay acts as an execution layer for coding agents:

- `pipeline` defines the project-level execution contract.
- `issue` defines one concrete task to run.
- `serve` polls the issue queue and drives planning plus coding loops.
- `feature_list.json` is the structured source of truth for completion.
- `progress.txt` is the handoff log between runs.
- `events.log`, `runs/`, and `issue.json` make failures inspectable.

The current runner executes the local `codex` CLI directly.

### Inspiration

This project is strongly influenced by two essays:

- OpenAI: [Harness Engineering](https://openai.com/en/index/harness-engineering/)
- Anthropic: [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

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
- `loop_num`
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

### Quick Start

Create a pipeline:

```bash
relay pipeline add demo-pipeline \
  --init-command 'git clone https://example.com/repo.git app' \
  --loop-num 3 \
  --plan-prompt-file plan.md \
  --coding-prompt-file coding.md
```

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

Versioning is controlled by git tags in CI. The GitHub Actions workflow:

- reuses the local `Makefile`, so CI and local packaging stay aligned
- runs only when a GitHub Release is published
- runs `make test`
- runs `make package-all VERSION=<release-tag>`
- uploads `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` archives to that release

If you want a local build with explicit version metadata, use:

```bash
make package VERSION=v0.1.0
```

To trigger packaging in GitHub, publish a release for a tag such as `v0.1.0`. For example:

```bash
gh release create v0.1.0 --generate-notes
```

Windows packages are not published yet because the current runtime assumes Unix tools such as `zsh` and `SIGKILL`.

### Scope

Relay is currently a focused harness for real coding tasks on real repositories.

The goal is not to be a generic chat shell. The goal is to provide a durable execution model for software-engineering agents working across planning, coding, persistence, and verification.
