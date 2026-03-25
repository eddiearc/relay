---
name: relay-operator
description: Use when an agent needs to operate the Relay CLI end-to-end on a real repository.
---

# Relay Operator

This skill is only the agent adapter layer.

Treat `relay-operator` as the current best-practice path, not as a frozen or exclusive workflow.
If you find a path that is clearer, safer, or more effective than the current one, prefer improving Relay itself and sending a GitHub PR to refine this skill and the CLI guidance.

Before doing Relay work:

```bash
relay help
relay version
```

For concrete operations, use the Relay CLI as the source of truth:

- `relay help`
- `relay help serve`
- `relay help pipeline`
- `relay help issue`
- `relay pipeline template`
- `relay issue template`
- `relay help upgrade`

Rules:

- do not restate Relay workflows from memory when CLI help already covers them
- do not duplicate templates or deep operational guidance in this skill
- if CLI help is insufficient, improve the CLI help instead of expanding this file
- if the bundled skill itself should be refreshed, use:
  `npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y`
