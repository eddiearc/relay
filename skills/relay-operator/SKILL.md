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
- `relay pipeline match --repo <path>`
- `relay pipeline show <name>`
- `relay pipeline template`
- `relay issue evaluate --pipeline <name> --goal "..." --description "..."`
- `relay issue template`
- `relay watch -issue <id>`
- `relay help upgrade`

Rules:

- do not restate Relay workflows from memory when CLI help already covers them
- first resolve pipeline state through `pipeline match` / `pipeline show` before inventing project guidance
- evaluate issue quality with `relay issue evaluate` before creating long-running work
- use `relay watch` for user-facing monitoring; keep `relay serve` as the executor
- do not duplicate templates or deep operational guidance in this skill
- if CLI help is insufficient, improve the CLI help instead of expanding this file
- if the bundled skill itself should be refreshed, use:
  `npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y`
