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

The current repository must contain at least one scenario file and prompt files:

- `e2e/scenario.yaml` (default codex scenario)
- `e2e/scenario-claude.yaml` (claude scenario, optional)
- `e2e/plan_prompt.md`
- `e2e/coding_prompt.md`
- `e2e/verify_prompt.md`

The prompt files referenced in `files:` within each scenario YAML are the canonical names.

## Flow

### 0. Detect Available Runners

Before selecting a scenario, detect which agent runners are installed:

```bash
which codex 2>/dev/null && echo "codex available"
which claude 2>/dev/null && echo "claude available"
```

Runner selection logic:
- If the user explicitly requested a runner (e.g., `/relay-e2e codex` or `/relay-e2e claude`), use that runner. Error if not installed.
- If no runner specified, auto-detect: prefer `claude` if available, fall back to `codex`.
- If neither is installed, stop and report.

Map runner to scenario file:
- `codex` → `e2e/scenario.yaml`
- `claude` → `e2e/scenario-claude.yaml`

If the selected scenario file does not exist, fall back to `e2e/scenario.yaml` (the default scenario works with any runner — it just omits `agent_runner` and defaults to codex).

### 1. Read the Scenario

Read the selected scenario YAML and the prompt files referenced in its `files:` section.

Extract:
- pipeline name
- agent_runner (if present)
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
- `agent_runner` (from scenario, if present)
- `init_command`
- `loop_num`
- `plan_prompt` loaded from the prompt file
- `coding_prompt` loaded from the prompt file

The temporary issue must embed:
- `id`
- `pipeline_name`
- `agent_runner` (from scenario issue, if present)
- `goal`
- `description`

### 4. Run Relay with Current Repo Code

Use the current repository code, not an installed binary from elsewhere.

Build a temporary binary first (faster for the serve step):

```bash
go build -o /tmp/relay-e2e-bin ./cmd/relay
```

Then run:

```bash
/tmp/relay-e2e-bin pipeline import -file <pipeline.yaml> -state-dir <state>
/tmp/relay-e2e-bin issue import -file <issue.json> -state-dir <state>
RELAY_WORKSPACE_ROOT=<workspaces> /tmp/relay-e2e-bin serve --once -state-dir <state>
```

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

Also verify streaming logs exist:
- `runs/<phase>.stdout.log` files should exist and be non-empty for at least the plan and coding phases

Always record the absolute issue artifact directory in the final report.

### 6. Perform Independent Verification

Read the verify prompt file, then perform a separate verification pass against the produced repository.

Use the current agent as the verifier. Do not treat the execution agent's self-report as sufficient proof.

Verification rules:
- start the produced app from the final `workdir_path`
- follow the verification contract from the scenario YAML
- run the HTTP checks described there
- compare actual status codes and bodies with the scenario contract

This verification pass must be logically separate from the execution pass:
- read artifacts first
- inspect the final repo
- then verify the running program

### 7. Final Output

Return a concise report with:
- scenario name and runner used
- PASS or FAIL
- temp state directory
- workspace root
- issue artifact directory
- final repo path
- streaming log file sizes
- executed Relay commands
- verification commands and HTTP checks
- failing responses and relevant log paths when applicable

## Default Decisions

- Use the repo's `e2e/` files as the contract.
- Auto-detect runner if not specified.
- Build a temporary binary from the current repo code.
- Use temporary directories only.
- Use the current agent for the verification pass.
- Treat `feature_list.json` and the independent verification pass as required for success.

## Common Mistakes

- Reusing `~/.relay` instead of temporary state.
- Editing the scenario instead of testing it.
- Trusting `progress.txt` without independently starting the final program.
- Forgetting to include artifact and log paths in failure output.
- Ignoring `agent_runner` field when materializing pipeline/issue inputs.
- Not checking that streaming log files were created during execution.
