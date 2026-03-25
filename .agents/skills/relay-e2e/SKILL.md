---
name: relay-e2e
description: Use when validating a Relay repository end-to-end with a real ACP-backed coding agent against a repo-defined e2e scenario.
---

# Relay E2E

## Overview

This skill runs a Relay repository end-to-end against a project-defined E2E scenario.

The skill is the orchestrator. The repository provides the scenario definition under `e2e/`.

## When to Use

Use this when:
- validating the latest Relay code against a real ACP-backed coding run
- checking whether a repo-defined E2E scenario still works end-to-end
- reproducing a Relay execution failure with full artifacts and logs

Do not use this for unit tests or fast local feedback. This is a slower, scenario-driven validation flow.

## Required Repo Files

The current repository must contain:

- `e2e/scenario.yaml`
- `e2e/plan_prompt.txt`
- `e2e/coding_prompt.txt`
- `e2e/verify_prompt.txt`

If any are missing, stop and report the missing file.

## Flow

### 1. Read the Scenario

Read `e2e/scenario.yaml` and the three prompt files.

Extract:
- pipeline name
- loop count
- init command
- issue id, goal, and description
- verification contract

Treat the repo files as the source of truth. Do not invent a different scenario.

### 2. Create Temporary Execution State

Create a fresh temporary directory for this run. Under it, create:
- `state/`
- `workspaces/`
- `inputs/`

Never use `~/.relay` or the user's normal Relay workspace root for this flow.

### 3. Materialize Pipeline and Issue Inputs

Build temporary input files from the scenario:

- `inputs/pipeline.yaml`
- `inputs/issue.json`

The temporary pipeline must embed:
- `name`
- `init_command`
- `loop_num`
- `plan_prompt` loaded from `e2e/plan_prompt.txt`
- `coding_prompt` loaded from `e2e/coding_prompt.txt`

The temporary issue must embed:
- `id`
- `pipeline_name`
- `goal`
- `description`

### 4. Run Relay with Current Repo Code

Use the current repository code, not an installed binary from elsewhere.

Preferred commands:

```bash
go run ./cmd/relay pipeline import -file <pipeline.yaml> -state-dir <state>
go run ./cmd/relay issue import -file <issue.json> -state-dir <state>
go run ./cmd/relay serve --once -state-dir <state> --workspace-root <workspaces>
```

If `go run ./cmd/relay` is too slow or unsuitable, building a temporary binary from the current repo is acceptable.

### 5. Collect Relay Artifacts

After `serve --once`, inspect:
- `issue.json`
- `feature_list.json`
- `progress.txt`
- `events.log`
- `runs/*`

Fail immediately if:
- the issue is not `done`
- `feature_list.json` is missing or not fully passed
- `progress.txt` is missing

Always record the absolute issue artifact directory in the final report.

### 6. Perform Independent Verification

Read `e2e/verify_prompt.txt`, then perform a separate verification pass against the produced repository.

Use the current agent as the verifier. Do not treat the execution agent's self-report as sufficient proof.

Verification rules:
- start the produced app from the final `repo_path`
- follow the verification contract from `scenario.yaml`
- run the HTTP checks described there
- compare actual status codes and bodies with the scenario contract

This verification pass must be logically separate from the execution pass:
- read artifacts first
- inspect the final repo
- then verify the running program

### 7. Final Output

Return a concise report with:
- scenario name
- PASS or FAIL
- temp state directory
- workspace root
- issue artifact directory
- final repo path
- executed Relay commands
- verification commands and HTTP checks
- failing responses and relevant log paths when applicable

## Default Decisions

- Use the repo's `e2e/` files as the contract.
- Use the current repository code via `go run ./cmd/relay`.
- Use temporary directories only.
- Use the current agent for the verification pass.
- Treat `feature_list.json` and the independent verification pass as required for success.

## Common Mistakes

- Reusing `~/.relay` instead of temporary state.
- Editing the scenario instead of testing it.
- Trusting `progress.txt` without independently starting the final program.
- Forgetting to include artifact and log paths in failure output.
