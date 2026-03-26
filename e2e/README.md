# Relay E2E Scenarios

This directory stores project-specific E2E scenarios for the global `relay-e2e` skill.

The global skill is responsible for orchestration:
- discover this directory
- create temporary Relay state and workspace roots
- materialize pipeline/issue inputs from `scenario.yaml`
- run `relay serve --once`
- collect artifacts and logs
- run an independent verification agent

The repository is responsible for project-specific definitions:
- `scenario.yaml`: scenario contract, init command, issue metadata, verification targets
- `plan_prompt.md`: planning prompt for the execution agent
- `coding_prompt.md`: coding prompt for the execution agent
- `verify_prompt.md`: prompt for the independent verification agent

The bundled `relay-e2e` skill should treat these Markdown prompt files as the canonical contract.

Those prompt files should mirror Relay's shipped harness defaults:
- planning stays broader, with closed-loop features and explicit verification boundaries
- each coding loop stays narrower, ideally one main slice plus verification for that slice
- verification reads artifacts first, then runs an independent proof against the produced repo

Current scenarios:
- `go-http-kv` (`scenario.yaml`): minimal Go HTTP key-value server using the default Codex runner
- `go-http-kv-claude` (`scenario-claude.yaml`): same scenario using `agent_runner: claude`

## Current gap for planning-vs-coding proof

The current shared `relay-e2e` skill only reports PASS when the Relay issue finishes as `done` and every item in `feature_list.json` is `passes: true`.

That means the bundled flow is good for end-to-end completion scenarios, but it is not yet a valid acceptance layer for the healthy intermediate state that this repository now wants to encourage: a narrow coding loop finishing one intended slice while later rollout features remain explicitly pending.

Until the shared skill can encode an expected non-terminal state, document that limitation and use repository-native tests plus local CLI commands as the source of truth for this specific planning-vs-coding split.

The intended entrypoint is the global `relay-e2e` skill, not `go test ./e2e`.
