---
name: relay-operator
description: Use when an agent needs to operate the Relay CLI end-to-end on a real repository, especially when the task spans pipeline creation, issue creation, and service monitoring or when the agent needs to choose the right Relay sub-skill.
---

# Relay Operator

## Overview

Use this as the top-level Relay skill.

It does three things:

- verifies the Relay CLI is installed before any work starts
- contains the baseline workflow for pipeline creation, issue creation, and monitoring
- links to deeper internal reference files when the agent needs more detail

This skill is intentionally self-contained for installation and everyday use. Installers usually target one named skill at a time, so `relay-operator` must remain useful when installed alone.

Prefer this skill when the user says "set up Relay for this repo" or asks for full Relay operation instead of a single narrow task.

## Internal References

- [create-pipeline.md](references/create-pipeline.md): deeper pipeline design guidance
- [pipeline-template.yaml](references/pipeline-template.yaml): reusable pipeline template
- [create-issue.md](references/create-issue.md): deeper issue-writing guidance
- [issue-template.json](references/issue-template.json): reusable issue template
- [monitor.md](references/monitor.md): deeper service and debugging guidance

Treat those files as optional deep dives. This file must still teach the full baseline workflow on its own.

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

## 1. Create a Pipeline

Design pipelines from repository facts, not from canned templates.

Inspect at least:

- the repository clone URL and default branch
- whether the repo is a monorepo
- the package manager and build toolchain
- the smallest verification commands that prove progress
- whether codegen, Docker, DB setup, env vars, or private registries are required

Pipeline rules:

- `init_command` should usually populate a fresh issue workspace by cloning the target repository into `.`
- prefer shallow clones such as `git clone --depth 1 <repo-url> .`
- treat each issue as a new workspace unless the user explicitly wants reuse
- `loop_num` may be omitted and Relay will default to `20`, but `15` is a good explicit upper bound for real repo work
- `plan_prompt` should force the planner to create verifiable features and establish a non-`main` task branch
- `coding_prompt` should force the coding agent to stay off `main`, verify progress, commit every loop, push every loop, and maintain one open PR for the task branch

Recommended branch naming:

- `relay/<short-slug>`
- `feat/<short-slug>`
- `fix/<short-slug>`

Template:

```yaml
name: repo-name
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
loop_num: 15
plan_prompt: |
  Read the repository before planning.
  If the current branch is main or master, create and switch to a task branch before finishing planning.
  Use a readable branch name derived from the task goal, for example relay/<short-slug>.
  Break the goal into the smallest meaningful features that can be verified.
  Each feature description must include an observable acceptance condition.
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
```

Prefer a file-backed pipeline:

```bash
relay pipeline import -file /path/to/pipeline.yaml
```

Deep dive:

- [create-pipeline.md](references/create-pipeline.md)
- [pipeline-template.yaml](references/pipeline-template.yaml)

## 2. Create an Issue

The point of issue authoring is to constrain planning so Relay produces a good `feature_list.json`.

A good issue contains:

- `goal`: one sentence describing the end state
- `description`: scope, constraints, non-goals, verification signals, and any existing clues
- any user-provided validation requirement, preserved verbatim when possible
- clear exclusions when the task boundary matters

Feature rules:

- one feature should map to one observable outcome
- `title` should describe a result, not an implementation step
- `description` should describe how to tell whether the result is achieved
- `passes` can become `true` only after verification
- `notes` should contain evidence, blockers, or residual risk

Good acceptance criteria come from commands, builds, typechecks, API responses, UI behavior, file outputs, or service events.

Bad acceptance criteria are vague:

- "Implemented the logic"
- "Mostly done"
- "Code looks correct"

Required `feature_list.json` shape:

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

When the user gives you a vague request, rewrite it into:

- what result should exist when the work is done
- how a user or operator can verify that result
- which commands, behaviors, or outputs count as evidence
- what is explicitly out of scope

Create the issue:

```bash
relay issue add \
  --pipeline <pipeline-name> \
  --goal "Add X" \
  --description "Scope, constraints, verification commands, and non-goals."
```

Or import:

```bash
relay issue import -file /path/to/issue.json
```

Deep dive:

- [create-issue.md](references/create-issue.md)
- [issue-template.json](references/issue-template.json)

## 3. Run and Monitor Relay

Use this order:

1. run `relay serve --once` and confirm the issue, pipeline, and artifacts are valid
2. run `relay serve` in the foreground and confirm it polls, creates workspaces, and writes artifacts
3. only then move it into persistent background or supervised mode

Mode selection:

- single pass for debugging: `relay serve --once`
- foreground validation: `relay serve`
- lightweight background process: `nohup relay serve >> ~/.relay/logs/serve.log 2>&1 &`
- production-style supervision: `systemd`, `launchd`, or another service manager

Check Relay state first:

```bash
relay issue list
relay status -issue <issue-id>
relay report -issue <issue-id>
```

Then inspect artifacts:

```bash
cat ~/.relay/issues/<issue-id>/issue.json
cat ~/.relay/issues/<issue-id>/feature_list.json
tail -n 200 ~/.relay/issues/<issue-id>/progress.txt
tail -n 200 ~/.relay/issues/<issue-id>/events.log
ls -la ~/.relay/issues/<issue-id>/runs
```

Then inspect host-level service state:

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

Deep dive:

- [monitor.md](references/monitor.md)

## Default End-to-End Sequence

When the user gives you a repository and wants Relay set up correctly, follow this order:

1. Verify the CLI is installed.
2. Use the pipeline section in this skill to write the pipeline.
3. Use the issue section in this skill to create the issue.
4. Use the monitoring section in this skill if the user wants `relay serve` run or diagnosed.

## Output Contract

For end-to-end Relay setup, prefer to produce:

- a repository-specific `pipeline.yaml`
- a concrete issue description or `issue.json`
- explicit acceptance criteria
- startup instructions for `relay serve`
- a monitoring or debugging checklist when needed

Do not hand back generic Relay advice without binding it to the actual repository and task.
