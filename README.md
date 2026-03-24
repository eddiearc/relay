# Relay

Relay is a goal-driven Go CLI for supervising short-lived AI worker runs.

`relay serve` starts a polling orchestrator. It scans the saved issue
directories, finds `todo` issues, and runs each one through the planning and
coding loop:

- create a workspace under `~/relay-workspaces/` by default
- run the pipeline `init_command`
- discover the initialized git repository
- run a planning agent that writes `feature_list.json` and `progress.txt` into
  the issue artifact directory
- run fresh coding agents until all features pass or `loop_num` is exhausted

Relay keeps control-plane state under `~/.relay/` by default:

```text
~/.relay/
  pipelines/
    <name>.yaml
  issues/
    <issue-id>/
      issue.json
      feature_list.json
      progress.txt
      runs/
```

Workspaces are stored separately under `~/relay-workspaces/` by default. You can
override the workspace root with `RELAY_WORKSPACE_ROOT` or `relay serve
--workspace-root ...`.

Codex execution now goes through an ACP bridge. Relay looks for `codex-acp`
first, then falls back to `npx -y @zed-industries/codex-acp@latest`.

## Pipeline

Add a pipeline with:

```bash
relay pipeline add demo-pipeline \
  --init-command 'git clone https://example.com/repo.git app' \
  --loop-num 15 \
  --plan-prompt-file plan.txt \
  --coding-prompt-file coding.txt
```

You can also import an existing YAML definition:

```bash
relay pipeline import -file pipeline.yaml
```

Pipeline YAML looks like:

```yaml
name: demo-pipeline
init_command: git clone https://example.com/repo.git app
loop_num: 3
plan_prompt: |
  Understand the task and repo. Write feature_list.json and progress.txt to the
  provided artifact paths.
coding_prompt: |
  Continue the task. Update feature_list.json and progress.txt at the provided
  artifact paths and commit repository code changes if you make any.
```

## Issue

Add an issue with:

```bash
relay issue add \
  --pipeline demo-pipeline \
  --goal "Implement the requested feature" \
  --description "Use the repository initialized by init_command."
```

You can also import an existing JSON definition:

```bash
relay issue import -file issue.json
```

Issue JSON looks like:

```json
{
  "pipeline_name": "demo-pipeline",
  "goal": "Implement the requested feature",
  "description": "Use the repository initialized by init_command."
}
```

Each imported or created issue is stored at:

```text
~/.relay/issues/<issue-id>/issue.json
```

The same issue directory also holds:

- `feature_list.json`: structured completion source of truth
- `progress.txt`: free-form handoff log between agent runs
- `runs/`: stdout/stderr/final logs for each planning/coding run

## Commands

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
- `relay serve -once`
- `relay status -issue issue-001`
- `relay report -issue issue-001`
- `relay kill -issue issue-001`

## Mutation Rules

- `issue edit` is allowed even if the issue is currently running.
- `issue delete` is blocked while the issue status is `running`.
- `pipeline delete` is blocked if any running issue still references that pipeline.
- `issue interrupt` does not kill an in-flight agent process. For a running
  issue it sets `interrupt_requested=true`, lets the current planning/coding run
  finish, and then the orchestrator persists the terminal status as
  `interrupted`.
