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
- `plan_prompt.txt`: planning prompt for the execution agent
- `coding_prompt.txt`: coding prompt for the execution agent
- `verify_prompt.txt`: prompt for the independent verification agent

Current scenario:
- `go-http-kv`: minimal Go HTTP key-value server with `GET /set` and `GET /get`

The intended entrypoint is the global `relay-e2e` skill, not `go test ./e2e`.
