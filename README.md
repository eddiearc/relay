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

That keeps the skill available across your agents and repositories. `relay-operator` is the self-contained default install. If you explicitly want a project-local install instead, remove `-g`.

Treat `relay-operator` as the current best-practice path, not as a frozen or exclusive workflow. The goal is to let the community converge on a clearer default. If you find a better path than the current one, send a GitHub PR to improve the skill and its CLI guidance.

Then prompt any agent that supports installed skills with something like:

```text
Use the installed relay-operator skill to set up Relay for <repository-path>.
Start by running relay help and relay upgrade --check, and summarize whether Relay or the relay-operator skill should be refreshed.
Then inspect the repository, choose or write a repository-specific pipeline, summarize its planning focus, coding focus, verification path, reusable project assets, and any missing E2E or unit-test coverage in a few concise bullets, ask whether that direction sounds right, then rewrite the task as a Relay issue with explicit acceptance criteria, call out any weak goal, scope, non-goals, or verification details that still need correction, and tell me whether to run relay serve --once or relay serve persistently.
```

Before that first agent prompt, run:

```bash
relay help
relay upgrade --check
```

That gives one friendly opening check for:

- current Relay version and install method
- whether a newer Relay version is available
- the canonical command map and workflow
- the exact `relay-operator` skill refresh command

Relay CLI help is the source of truth for concrete operations. Prefer:

```bash
relay help
relay help pipeline
relay help issue
relay help serve
```

The installed skill will guide the agent to:

- run the opening `relay help` and `relay upgrade --check` check
- use `relay help ...` as the operational source of truth
- inspect the target repository
- inspect saved pipelines with `relay pipeline list` and `relay pipeline show`
- select a clearly matching pipeline, ask the user to choose when multiple look plausible, or create a repository-specific pipeline from `relay pipeline template` when none fit
- summarize the selected or proposed pipeline in a few concise bullets before proceeding, including planning focus, coding focus, verification path, reusable project assets, and whether E2E or unit-test coverage is missing
- call out weak or missing issue inputs instead of silently guessing, especially around end state, scope, non-goals, and verification
- explain the key directional choices from the pipeline configuration and prompt intent, then ask the user if that direction sounds right before creating the issue
- rewrite the task as a Relay issue with explicit acceptance criteria
- ask several focused questions in one turn when the issue still lacks a clear end result, scope / non-goals, or verification path
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
- Anthropic: [Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
- Anthropic: [Building agents with the Claude Agent SDK](https://www.anthropic.com/engineering/building-agents-with-the-claude-agent-sdk/)

### Verification Doctrine

Relay treats verification as part of the harness contract, not as an optional afterthought.

- OpenAI's harness engineering write-up argues that reliable agent work comes from designed environments and feedback loops, not from raw prompting alone.
- Anthropic's eval guidance says the thing that matters is the final state in the environment, and that an eval harness should run tasks end to end and grade outcomes there.
- Anthropic's agent SDK write-up explicitly recommends browser automation for rendered pages so agents can test screenshots, viewports, and interactive elements inside the workflow.

From those sources, Relay derives the following operating policy for project work:

- for meaningful behavior changes, agents should default to the strongest realistic project-level verification path
- when a heavier path is not justified, agents may use a narrower verification path, but they should say that explicitly and explain why
- frontend repos should usually prefer browser E2E that simulates clicks and validates real user journeys
- backend repos should usually prefer a local startup or deployment path plus integration checks against the running service
- CLI repos should usually prefer runnable end-to-end command checks against the built or local binary
- mobile or desktop app repos should usually prefer simulator, emulator, or UI automation that drives the real app shell
- library or SDK repos should usually prefer consumer-facing integration tests, example apps, or fixture projects that exercise the public API
- worker, queue, cron, or data-pipeline repos should usually prefer fixture-driven end-to-end runs that assert emitted jobs, persisted outputs, or downstream side effects
- infrastructure or IaC repos should usually prefer reproducible plan or dry-run checks plus smoke validation against the provisioned or simulated target
- if those verification layers do not exist, the agent should say so explicitly, recommend the missing script, test suite, or skill, and avoid pretending the task is fully specified
- missing unit tests are a separate red flag and should be called out even more strongly

For the repository-native testing map and contributor command sequence, see `docs/verification.md`. The default proof path is targeted package tests, then `go test ./...`, then the smallest real `go run ./cmd/relay ...` commands that cover the changed user-facing surface.

The reusable `e2e/plan_prompt.md`, `e2e/coding_prompt.md`, and `e2e/verify_prompt.md` examples should mirror that same philosophy: broader planning, narrower coding loops, and explicit verification ordering.

### Product Idea

Relay acts as an execution layer for coding agents:

- `pipeline` defines the project-level execution contract.
- `issue` defines one concrete task to run.
- `serve` polls the issue queue and drives planning plus coding loops.
- `feature_list.json` is the structured source of truth for completion.
- `progress.txt` is the handoff log between runs.
- `events.log`, `runs/`, and `issue.json` make failures inspectable.

Relay resolves the local runner as issue `agent_runner` override → pipeline `agent_runner` → default `codex`.
Supported local runners are `codex` and `claude`.

### Bundled Agent Skill

This repository ships a self-contained top-level skill for agents that need to operate the Relay CLI:

```text
skills/relay-operator/
```

Installing `relay-operator` is enough for normal use. It covers:

- repository-specific pipeline authoring
- issue decomposition with explicit acceptance criteria
- persistent `relay serve` operation
- artifact plus host-log inspection

Released npm packages also include `skills/relay-operator/skill.json`. That metadata is versioned with the published Relay tag so the skill can track the bundled CLI release and recommend a consistent refresh command.

For skill installation, prefer the `npx skills` distribution flow instead of manually copying files.

### Inspiration

Relay turns those ideas into a concrete product model for real repositories.

The main product takeaways are:

- The harness matters more than the raw prompt.
- Long-running work needs external memory.
- Completion should come from structured state, not just model narration.
- Each run can be fresh if the right artifacts are persisted.
- Real repo execution and verification matter more than toy demos.

### Planning vs. Coding Split

Relay's default execution model intentionally uses planning and coding at different resolutions.

- planning should think in phases, dependencies, verification boundaries, and acceptance boundaries, especially for repo-wide or system-level work
- planned features should be relatively closed loops of user-visible or verifier-visible progress, not scattered tiny file tasks
- each coding loop should usually take one main feature, or at most a very small cluster of tightly related work, and decide its verification path before editing
- unfinished rollout work should stay explicit in `feature_list.json`; `progress.txt` is for handoff context, not for silently shrinking scope
- the default verification sequence is targeted package proof first, then `go test ./...`, then the smallest real `go run ./cmd/relay ...` commands that cover the changed CLI surface

Current E2E gap: the shared `relay-e2e` skill currently treats a run as passing only when the issue reaches `done` and every item in `feature_list.json` is `passes: true`. That makes it good orchestration smoke coverage, but not yet a valid proof for the healthy intermediate state where a narrow coding loop lands one slice and leaves later features explicitly pending.

### Core Concepts

#### Pipeline

Project-level configuration stored at:

```text
~/.relay/pipelines/<name>.yaml
```

Fields:

- `name`
- `init_command`
- `agent_runner` (optional: `codex` or `claude`; defaults to `codex` when omitted)
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
- `agent_runner` (optional override; inherits the pipeline runner and then defaults to `codex`)
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
- `codex` or `claude` installed and available in `PATH`
- if neither the issue nor the pipeline sets `agent_runner`, Relay uses `codex`

If you prefer building from source:

```bash
go install github.com/eddiearc/relay/cmd/relay@latest
```

### Commands

For examples and workflow guidance, prefer `relay help` and `relay help <command>`.

- `relay pipeline add <name> --init-command ... --plan-prompt-file ... --coding-prompt-file ...`
- `relay pipeline edit <name> [--init-command ...] [--agent-runner codex|claude] [--loop-num ...] [--plan-prompt-file ...] [--coding-prompt-file ...]`
- `relay pipeline import -file pipeline.yaml`
- `relay issue add --pipeline <name> [--agent-runner codex|claude] --goal ... --description ...`
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
- `relay upgrade`
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

Versioning is controlled by CI release policy plus git tags. Relay now uses two GitHub Actions workflows:

- `release-policy.yml` runs on pushes to `main` and on manual `workflow_dispatch`
- it inspects the latest published release tag that still reaches `main`
- if `main` is already covered by that published release, it does nothing
- if `HEAD` already has an explicit stable tag such as `v1.3.0`, it publishes that tag first
- otherwise it derives the next patch tag from the latest published release and publishes that release
- if there is no published release baseline yet, the first release stays explicit/manual
- `release.yml` still handles packaging after a GitHub Release is published
- `release.yml` reuses the local `Makefile`, runs `make test`, builds all archives, uploads them, and then publishes npm packages in platform-first order

If you want a local build with explicit version metadata, use:

```bash
make package VERSION=v0.1.0
```

To inspect the policy locally against the current repository state, run:

```bash
go run ./cmd/relay release inspect \
  --repo . \
  --main-ref HEAD \
  --published-release-tag v0.1.0
```

The inspect command prints the chosen action (`noop`, `publish-explicit-tag`, or `auto-cut-patch`), the selected tag when one is needed, and the reason.

To trigger packaging in GitHub with an explicit release tag such as `v0.1.0`, publish a release for that tag. For example:

```bash
gh release create v0.1.0 --generate-notes
```

If you want to validate the policy without publishing anything, run the `Release Policy` workflow manually with `dry_run=true`. You can also set `published_release_tag` to simulate a published baseline without creating a real release first. For packaging-only validation, the `Release Smoke Test` workflow still creates a temporary draft release tag like `v0.0.0-smoke.<run_id>`, uploads the platform archives, generates the npm packages, validates them with `npm pack --dry-run`, and then deletes the temporary release and tag.

For the npm package layout and registry setup, see [`npm/README.md`](./npm/README.md).

The preferred npm publishing mode is Trusted Publishing via GitHub Actions OIDC. The packaging workflow already includes `id-token: write`; configure Trusted Publisher for each `@eddiearc/*` package in npm using workflow filename `release.yml`.

Windows packages are not published yet because the current runtime assumes Unix tools such as `zsh` and `SIGKILL`.

### Scope

Relay is currently a focused harness for real coding tasks on real repositories.

The goal is not to be a generic chat shell. The goal is to provide a durable execution model for software-engineering agents working across planning, coding, persistence, and verification.
