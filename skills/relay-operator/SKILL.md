---
name: relay-operator
description: Use when an agent needs to operate the Relay CLI on a real repository, especially to design a repository-specific pipeline, turn a request into a Relay issue with explicit acceptance criteria, run relay serve as a persistent service, or inspect Relay artifacts and host logs to understand execution state.
---

# Relay Operator

## Overview

Treat Relay as a durable orchestration layer, not as a one-shot prompt runner.

The operating model is simple:

- Read the target repository before writing a pipeline.
- Turn vague work into verifiable features.
- Treat `feature_list.json` as the only source of truth for completion.
- Validate `relay serve` in the foreground before running it persistently.
- Debug from Relay artifacts first, then from host-level process and service logs.

This skill is intended to ship with the Relay project itself. Prefer this repository copy over a user-local copy when both exist.

## When to Use

- A user gives you a repository and wants Relay set up for it.
- A user wants a new pipeline or a pipeline revision for a specific repo.
- A user wants an issue broken down into explicit acceptance criteria.
- A user wants `relay serve` to run continuously.
- A user wants to understand what Relay is doing, why it is stuck, or whether it is healthy.

## Output Contract

When the user asks you to set up Relay for a repository, prefer to produce these artifacts:

- A repository-specific `pipeline.yaml`
- A concrete issue description or `issue.json`
- Explicit acceptance criteria that can be verified by commands, API behavior, UI behavior, or generated outputs
- Startup instructions for `relay serve`
- A log inspection checklist for ongoing operations

Reusable templates are available here:

- [pipeline-template.yaml](references/pipeline-template.yaml)
- [issue-template.json](references/issue-template.json)

Do not hand back generic advice without binding it to the actual repository.

## Quick Map

- Pipelines: `~/.relay/pipelines/<name>.yaml`
- Issue state: `~/.relay/issues/<issue-id>/issue.json`
- Issue artifacts:
  - `~/.relay/issues/<issue-id>/feature_list.json`
  - `~/.relay/issues/<issue-id>/progress.txt`
  - `~/.relay/issues/<issue-id>/events.log`
  - `~/.relay/issues/<issue-id>/runs/`
- Default workspaces: `~/relay-workspaces/<issue-id>-<hash>/`

## 0. Verify the CLI Is Installed

Before doing any Relay work, confirm that the `relay` CLI is actually available.

Check:

```bash
command -v relay
relay version
```

If both commands succeed, continue.

If `relay` is missing, stop assuming the environment is ready and switch to installation guidance. Prefer:

```bash
npm install -g @eddiearc/relay
```

Or, if the user prefers building from source:

```bash
go install github.com/eddiearc/relay/cmd/relay@latest
```

Do not start authoring pipelines, creating issues, or debugging `relay serve` until installation is confirmed.

## 1. Design a Pipeline for a Real Repository

Start from repository facts, not from a canned template.

Before writing the pipeline, inspect at least:

- The actual working directory and whether the repo is a monorepo
- The language and package manager: `pnpm`, `npm`, `yarn`, `go`, `cargo`, `uv`, `poetry`, and so on
- The non-interactive setup path
- The smallest useful verification commands: tests, lint, build, typecheck, API smoke tests, CLI smoke tests
- Whether codegen, database setup, Docker, private registries, or environment variables are required

### Pipeline Rules

- `name` should be stable and human-readable.
- `init_command` should only make the workspace usable.
- `init_command` must be repeatable, non-interactive, and fail fast.
- Do not hide long-running foreground services inside `init_command` unless service orchestration is part of the task itself.
- `loop_num` may be omitted and Relay will default to `20`, but most pipelines should set it explicitly.
- `plan_prompt` should force the planner to produce verifiable features.
- `coding_prompt` should force the coding agent to update artifacts based on verified state, not intent.

### `loop_num` Guidance

- Omit it only if the default behavior is intentional.
- `3`: small bugfix or tightly scoped single-area change
- `5`: good default for medium-complexity work
- `7` or `8`: multi-stage work with several validation checkpoints
- Avoid using a very large `loop_num` to compensate for weak issue decomposition.

### Pipeline Authoring Checklist

1. Read the repository root files and build scripts.
2. Infer the setup path that a fresh workspace needs.
3. Infer the verification commands that prove a feature is done.
4. Write a planning prompt that produces observable acceptance conditions.
5. Write a coding prompt that updates `FEATURE_LIST_PATH` and `PROGRESS_PATH` from evidence.

### Pipeline Template

Use this as a starting point, then rewrite it to fit the actual repository. A reusable copy also lives at [pipeline-template.yaml](references/pipeline-template.yaml).

```yaml
name: repo-name
init_command: |
  set -e
  cd "$WORKDIR_PATH"

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
loop_num: 5
plan_prompt: |
  Read the repository before planning.
  Break the goal into the smallest meaningful features that can be verified.
  Each feature description must include an observable acceptance condition.
  Prefer evidence from tests, commands, API behavior, UI behavior, or generated files.
  Keep features stable across loops. Avoid vague or overlapping features.
coding_prompt: |
  Make the smallest correct change in WORKDIR_PATH.
  Verify progress with real commands where possible.
  Update FEATURE_LIST_PATH based on verified state, not intention.
  Record evidence or blockers in notes.
  Append a concise handoff entry to PROGRESS_PATH before finishing.
```

### Import or Create the Pipeline

Prefer a file-backed pipeline:

```bash
relay pipeline import -file /path/to/pipeline.yaml
```

Or create it directly:

```bash
relay pipeline add <name> \
  --init-command '...' \
  --loop-num 5 \
  --plan-prompt-file /path/to/plan_prompt.md \
  --coding-prompt-file /path/to/coding_prompt.md
```

## 2. Break Work into Issues with Explicit Acceptance Criteria

The point of issue authoring is not to write a nice paragraph. The point is to constrain the planner so it produces a good `feature_list.json`.

### A Good Relay Issue Contains

- `goal`: one sentence describing the end state
- `description`: scope, constraints, non-goals, verification signals, and any existing clues
- Any user-provided validation requirement, preserved verbatim when possible
- Clear exclusions when the task boundary matters

### Feature Decomposition Rules

- One feature should map to one observable outcome.
- `title` should describe a result, not an implementation step.
- `description` should describe how to tell whether the result is achieved.
- `priority` should reflect execution order or dependency order.
- `passes` can become `true` only after verification.
- `notes` should contain evidence, blockers, or residual risk.

### Acceptance Criteria Rubric

Good acceptance criteria are observable from outside the model:

- A command passes: `go test ./...`, `pnpm test`, `cargo test`
- A build passes: `npm run build`
- A typecheck passes: `tsc --noEmit`
- An API returns an expected status code or response field
- A UI interaction produces an expected visible state
- A file or report is generated with expected contents
- A service startup produces an expected event or log signal

Bad acceptance criteria are vague and non-testable:

- "Implemented the logic"
- "Mostly done"
- "Code looks correct"
- "Handled the main cases"

### Required `feature_list.json` Shape

Relay requires `feature_list.json` to be a JSON array, and each item must use exactly these fields:

```json
[
  {
    "id": "feature-1",
    "title": "CLI prints the new summary section",
    "description": "Running the target command prints the summary section with non-empty values.",
    "priority": 1,
    "passes": false,
    "notes": ""
  }
]
```

### Recommended Issue Writing Pattern

When the user gives you a vague request, rewrite it into:

- What result should exist when the work is done
- How a user or operator can verify that result
- Which commands, behaviors, or outputs count as evidence
- What is explicitly out of scope

### Create the Issue

For a simple task:

```bash
relay issue add \
  --pipeline <pipeline-name> \
  --goal "Add X" \
  --description "Scope, constraints, verification commands, and non-goals."
```

For a larger task, prefer authoring `issue.json` and importing it:

```bash
relay issue import -file /path/to/issue.json
```

## 3. Run `relay serve` as a Resident Service

### Mode Selection

- Single pass for debugging: `relay serve --once`
- Foreground validation: `relay serve`
- Lightweight background process: `nohup relay serve >> ~/.relay/logs/serve.log 2>&1 &`
- Production-style supervision: `systemd`, `launchd`, or another service manager

### Startup Sequence

Always use this order:

1. Run `relay serve --once` and confirm the issue, pipeline, and artifacts are valid.
2. Run `relay serve` in the foreground and confirm it actually polls, creates workspaces, and writes artifacts.
3. Only then move it into a persistent background or supervised mode.

### Service Rules

- Keep a dedicated stdout/stderr log for `relay serve`.
- Do not casually start multiple `relay serve` processes against the same state directory.
- If you need multiple workers, define state isolation or queue isolation first.
- Prefer a supervisor that can restart the service and expose service logs.

### Minimal Background Example

```bash
mkdir -p ~/.relay/logs
nohup relay serve >> ~/.relay/logs/serve.log 2>&1 &
```

Then check:

```bash
ps -ef | rg "[r]elay serve"
tail -n 100 ~/.relay/logs/serve.log
```

## 4. Inspect Relay State and Host Logs

Use a fixed debugging order:

1. Check issue-level state.
2. Check issue artifacts.
3. Check host process and service logs.

### Check Relay State First

```bash
relay issue list
relay status -issue <issue-id>
relay report -issue <issue-id>
```

Pay attention to:

- Whether `status` is still `todo`, `planning`, `running`, `done`, or `failed`
- Whether `current_loop` is increasing
- Whether `last_error` is non-empty
- Whether the artifact path exists

### Read Issue Artifacts Directly

```bash
cat ~/.relay/issues/<issue-id>/issue.json
cat ~/.relay/issues/<issue-id>/feature_list.json
tail -n 200 ~/.relay/issues/<issue-id>/progress.txt
tail -n 200 ~/.relay/issues/<issue-id>/events.log
ls -la ~/.relay/issues/<issue-id>/runs
```

From these files, answer:

- Did planning actually produce a valid and non-empty `feature_list.json`?
- Which feature is currently blocked?
- Did the latest coding loop append a useful handoff entry?
- Did the failure happen in planning, coding, or post-run validation?
- Does the latest run output show a command failure, schema violation, or prompt failure?

### Common Diagnostic Signals

- `events.log` contains `planning validation failed`
  - Usually `feature_list.json` is empty, malformed, or uses the wrong fields.
- `current_loop` increases but `feature_list.json` barely changes
  - Usually the prompts are weak or the coding agent is not updating artifacts from real evidence.
- `last_error` is non-empty
  - Read `relay report -issue <issue-id>` and the latest run stderr immediately.
- An issue stays `todo` and no new events appear
  - Usually `relay serve` is not running, or it is running against the wrong state directory or environment.
- Code changed but all `passes` values remain `false`
  - Usually verification never happened, or the feature descriptions are too vague to judge.

### Check Host-Level Service State

Confirm the process exists:

```bash
ps -ef | rg "[r]elay serve"
```

On macOS:

```bash
launchctl list | rg relay
log show --last 1h --predicate 'process == "relay"'
```

On Linux with `systemd`:

```bash
systemctl status relay
journalctl -u relay -n 200 --no-pager
```

For a manual `nohup` process:

```bash
tail -n 200 ~/.relay/logs/serve.log
```

Use the logs for different questions:

- Host logs answer whether the process is alive, restarting, or crashing.
- Relay artifacts answer what the task is doing and why it is not complete.
- Do not rely on only one of those sources.

## 5. Operating Pattern for Agents

When a user gives you a repository and asks for Relay help, use this sequence by default:

1. Read the repository and identify setup and verification commands.
2. Write a repository-specific pipeline.
3. Rewrite the user request into a Relay issue with explicit acceptance criteria.
4. Run `relay serve --once` first if you are validating locally.
5. Move to persistent service mode only after a foreground sanity check.
6. If anything goes wrong, debug in the order `issue status -> artifacts -> host logs`.

## Common Mistakes

- Copying a generic pipeline without reading the repository
- Putting interactive login or long-running foreground processes in `init_command`
- Writing issue descriptions that say "implement X" without saying how to verify X
- Creating features that cannot be externally observed
- Starting `relay serve` in the background before validating it in the foreground
- Looking only at `serve.log` and ignoring `feature_list.json`, `progress.txt`, and `events.log`
- Marking `passes` as `true` because code changed, without evidence
- Using a huge `loop_num` to hide poor feature design
