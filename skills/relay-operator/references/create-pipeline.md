# Create Pipeline

Use this reference when the user specifically needs deeper pipeline design guidance than the top-level `relay-operator` workflow provides.

## Inspect Before Writing

Inspect at least:

- the repository clone URL and default branch
- whether the repo is a monorepo
- the package manager and build toolchain
- the smallest verification commands that prove progress
- whether codegen, Docker, DB setup, env vars, or private registries are required

## Pipeline Rules

- `name` should be stable and human-readable
- `init_command` should usually populate a fresh issue workspace by cloning the target repository into `.`
- `init_command` must be repeatable, non-interactive, and fail fast
- prefer shallow clones such as `git clone --depth 1 <repo-url> .` unless the task truly needs full history
- treat each issue as a new workspace; do not rely on a reused checkout unless the user explicitly wants that behavior
- do not hide long-running foreground services inside `init_command` unless service orchestration is part of the task itself
- `loop_num` may be omitted and Relay will default to `20`, but most pipelines should set it explicitly
- `plan_prompt` should force the planner to produce verifiable features and establish a non-`main` working branch before coding starts
- `coding_prompt` should force the coding agent to stay off `main`, commit every loop, push every loop, and maintain one open PR for the task branch

## `loop_num` Guidance

- omit it only if the default behavior is intentional
- `3`: small bugfix or tightly scoped single-area change
- `5` to `8`: smaller tasks with predictable verification
- `15`: recommended upper bound for real repository work when the task may need several loops
- treat `15` as a deliberate max loop count, not as a guarantee that weak issue decomposition is acceptable

## Authoring Checklist

1. Read the repository root files and build scripts.
2. Infer the clone URL, default branch, and final working directory inside the fresh workspace.
3. Infer the verification commands that prove a feature is done.
4. Decide the branch naming pattern for task work, for example `relay/<short-slug>`.
5. Write a planning prompt that produces observable acceptance conditions and creates the task branch before planning finishes.
6. Write a coding prompt that updates `FEATURE_LIST_PATH` and `PROGRESS_PATH` from evidence and never lands task changes on `main`.

## Create or Import

Prefer a file-backed pipeline:

```bash
relay pipeline import -file /path/to/pipeline.yaml
```

Or create it directly:

```bash
relay pipeline add <name> \
  --init-command '...' \
  --loop-num 15 \
  --plan-prompt-file /path/to/plan_prompt.md \
  --coding-prompt-file /path/to/coding_prompt.md
```

## Common Mistakes

- copying a generic pipeline without reading the repository
- relying on an existing checkout instead of a fresh issue workspace
- leaving planning vague so feature acceptance cannot be judged
- allowing coding work to happen on `main`
- forgetting that coding loops should `commit + push + maintain PR`
