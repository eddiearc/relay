---
name: relay-operator
description: Use when an agent needs to operate the Relay CLI end-to-end on a real repository.
---

# Relay Operator

This skill is only the agent adapter layer.

Treat `relay-operator` as the current best-practice path, not as a frozen or exclusive workflow.
If you find a path that is clearer, safer, or more effective than the current one, prefer improving Relay itself and sending a GitHub PR to refine this skill and the CLI guidance.

Before doing Relay work on first use:

```bash
command -v relay >/dev/null 2>&1 || npm install -g @eddiearc/relay || go install github.com/eddiearc/relay/cmd/relay@latest
relay upgrade --check
relay help
```

For concrete operations, use the Relay CLI as the source of truth:

- `relay help`
- `relay help watch`
- `relay help pipeline`
- `relay help issue`
- `relay pipeline list`
- `relay pipeline show <name>`
- `relay pipeline template`
- `relay issue template`
- `relay issue add --pipeline <name> --goal "..." --description "..."`
- `relay watch -issue <id>`
- `relay help upgrade`

Rules:

- do not restate Relay workflows from memory when CLI help already covers them
- inspect the current repository before choosing a pipeline
- use `relay pipeline list` and `relay pipeline show` to inspect candidates
- if one pipeline is clearly the right fit for the current repo and task, select it directly
- if multiple pipelines are plausible or the fit is unclear, present the candidates and ask the user to choose
- if no saved pipeline is clearly suitable, start from `relay pipeline template`, write a repository-specific pipeline, and import it
- rewrite the task into a Relay-ready issue before creating it
- if the issue is still too vague, ask the user several focused questions in one turn before creating it; those questions should cover:
  - the target end result
  - scope limits and explicit non-goals
  - how completion should be verified
- use `relay watch` for user-facing monitoring; keep `relay serve` as the executor
- do not duplicate templates or deep operational guidance in this skill
- if CLI help is insufficient, improve the CLI help instead of expanding this file
- if the bundled skill itself should be refreshed, use:
  `npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y`
