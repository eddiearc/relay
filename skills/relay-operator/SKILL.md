---
name: relay-operator
description: Use when an agent needs to operate the Relay CLI end-to-end on a real repository.
---

# Relay Operator

This skill is intentionally thin.

Use it to route the agent into the Relay CLI, not to duplicate Relay product behavior in prompt text.

## Default Opening

Before touching the repository, run:

```bash
relay help
relay version
```

Then summarize:

- which Relay version is installed
- whether Relay should be upgraded
- whether the bundled `relay-operator` skill should be refreshed with `npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y`
- which Relay command family is relevant next

## Source of Truth

For all concrete Relay operations, use CLI help instead of re-explaining from memory:

- `relay help`
- `relay help serve`
- `relay help pipeline`
- `relay help pipeline add`
- `relay help pipeline edit`
- `relay help pipeline import`
- `relay help issue`
- `relay help issue add`
- `relay help issue edit`
- `relay help issue interrupt`
- `relay help report`
- `relay help status`
- `relay help kill`
- `relay help upgrade`

If a user asks how to perform a Relay action, prefer quoting or summarizing the relevant CLI help output after running the matching help command.

## Agent Behavior

- treat `relay help` and `relay version` as the canonical startup path
- use `relay help` output as the operational source of truth
- do not duplicate long pipeline, issue, or monitoring playbooks in this skill
- if the CLI help appears insufficient, improve the CLI help rather than growing this file
