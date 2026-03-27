# Relay Verification Strategy

Relay should not rely on one verification style. This repository uses a layered model so contributors can pick the smallest realistic proof during iteration, widen to repository-wide proof before handoff, and still finish with real CLI checks for stable user-visible behavior.

## Default Proof Path

For most Relay changes, use this order unless the task clearly justifies a stronger project-level path:

1. targeted package tests for the layer you changed
2. `go test ./...` before commit or PR update
3. local CLI verification for the affected user-facing commands

That default keeps iteration fast without skipping the final proof that the actual `relay` command still behaves as expected.

## Current Layers

### 1. Unit and package-level tests

Use package tests for pure logic and validation rules that do not need a real CLI process.

Good fits in this repository:

- `internal/relay/spec_test.go` for pipeline and issue normalization
- `internal/relay/runner_test.go` for runner command construction and log handling
- `internal/relay/feature_test.go` for `feature_list.json` validation and transition rules
- `internal/relay/state_test.go` for persisted issue snapshot loading and contract checks
- `internal/relay/orchestrator_test.go` for orchestration behavior with controlled test doubles

Use this layer when you are checking:

- normalization and validation logic
- state transitions with in-memory or temp-dir setup
- persisted artifact schema drift
- pure formatting or helper behavior
- failure modes that do not require a subprocess boundary

Targeted commands:

- `go test ./internal/relay -run TestLoadFeatureListFixtures`
- `go test ./internal/relay -run TestStoreLoadIssueFixtures`
- `go test ./internal/relay -run TestValidateFeatureTransitionRejectsDeletionAndRegression`

### 2. In-process CLI tests

Use in-process CLI tests when the command surface matters, but a subprocess boundary adds little value. These tests call `run(...)` or `RunWithIO(...)` directly and are fast enough for local iteration.

Good fits in this repository:

- command routing and usage text branches in `internal/cli/app_test.go`
- hint text and report formatting with temp Relay state
- serve/watch flows that swap in test runners through `SetServeRunnerForTesting`

Use this layer when you are checking:

- command parsing and exit code behavior inside one process
- help topics and detailed usage branches before snapshotting
- orchestration flows that need injected runners or fake shell execution

Targeted commands:

- `go test ./internal/cli -run TestPipelineHelp`
- `go test ./internal/cli -run TestIssueAddCreatesPerIssueDirectory`

### 3. Golden snapshots for stable user-visible output

Use checked-in golden files for stable CLI text that users read directly and that should change only intentionally.

Current coverage:

- `internal/cli/golden_test.go`
- `internal/cli/testdata/help.golden`
- `internal/cli/testdata/help_pipeline.golden`
- `internal/cli/testdata/status.golden`
- `internal/cli/testdata/report.golden`

This layer is the right fit for:

- top-level `relay help`
- stable subcommand help like `relay help pipeline`
- reviewable `relay status` output against persisted issue state
- reviewable `relay report` output that summarizes issue artifacts and logs

Run and refresh:

- verify: `go test ./internal/cli -run TestCommandOutputGolden`
- refresh intentionally changed output: `go test ./internal/cli -run TestCommandOutputGolden -args -update`

Do not use goldens for highly dynamic output, timestamps, or paths that churn every run unless the test normalizes them first.

### 4. Subprocess command integration tests

Use a real subprocess when the process boundary itself matters: exit code propagation, stdout/stderr separation, current working directory behavior, and on-disk side effects.

Current coverage:

- `internal/cli/subprocess_test.go` runs `go run ./cmd/relay`
- covered flows import a pipeline, create an issue, and then read that issue back through `relay status`

This layer is the right fit for:

- commands that create or mutate persisted state on disk
- smoke checks for the real compiled or `go run` CLI surface
- output split across stdout and stderr
- stable command flows that should survive package-level refactors

Targeted commands:

- `go test ./internal/cli -run TestRelaySubprocess`

### 5. Contract fixtures for persisted machine-consumed artifacts

Use checked-in fixtures for JSON or YAML artifacts that other Relay phases or external tooling consume structurally.

Current coverage:

- `internal/relay/testdata/feature_list_valid.json`
- `internal/relay/testdata/feature_list_invalid_missing_title.json`
- `internal/relay/testdata/issue_valid.json`
- `internal/relay/testdata/issue_invalid_missing_goal.json`
- `internal/relay/testdata/issue_invalid_agent_runner.json`

This layer is the right fit for:

- `feature_list.json`
- `issue.json`
- template or persisted formats where schema drift should be obvious in review

Targeted commands:

- `go test ./internal/relay -run TestLoadFeatureListFixtures`
- `go test ./internal/relay -run TestStoreLoadIssueFixtures`

## Practical Workflow

Use the smallest loop first, then widen proof before you ship.

For harness-prompt or execution-policy changes, keep the same ordering discipline explicit in both docs and prompts:

1. pick the narrowest repository-native proof that directly covers the edited behavior
2. widen to `go test ./...` before handoff
3. finish with the smallest real `go run ./cmd/relay ...` commands that exercise the changed CLI surface
4. use `relay-e2e` only when the scenario can prove the intended end state instead of approximating it

### Fast local iteration

Start with the narrowest package test that proves the change you just made.

```bash
go test ./internal/cli -run TestCommandOutputGolden
go test ./internal/cli -run TestRelaySubprocess
go test ./internal/relay -run TestStoreLoadIssueFixtures
```

Pick the smallest relevant command from that list instead of running all of them by default.

### Broader pre-PR verification

Before committing or updating a PR, widen to the full repository test suite.

```bash
go test ./...
```

That is the current repository-wide proof step for Relay.

### Local CLI verification

After the broader test pass, run the smallest real `relay` commands that cover the user-facing surface you changed.

```bash
go run ./cmd/relay help
go run ./cmd/relay help status
go run ./cmd/relay help report
```

If you changed status/report behavior against temp Relay state, also run a matching local command flow or keep the subprocess test focused on that path.

If you intentionally change snapshot-protected output, refresh the goldens first, review the snapshot diff, and then rerun `go test ./internal/cli -run TestCommandOutputGolden` without `-update`.

## Where Each Change Belongs

- use unit/package tests for validation logic, normalization, and transition rules
- use in-process CLI tests for command dispatch and flows that need injected test doubles
- use subprocess tests for real command execution, exit codes, stdout/stderr separation, and filesystem side effects
- use goldens for stable long-form help text plus stable `status` and `report` output
- use contract fixtures for persisted JSON/YAML artifacts that later Relay phases or tooling consume

## `relay-e2e` Decision for This Change

I evaluated the repo-level `relay-e2e` layer for this work and did not extend it.

Reason: the bundled `e2e/` scenarios validate Relay operating on a target repository through `relay serve --once` and an independent verification pass. That is useful for orchestration smoke coverage, but this prompt-split work needs proof about Relay's own execution philosophy: broader phased planning, narrower coding loops, and explicit unfinished scope in `feature_list.json`. The current shared `relay-e2e` skill still declares failure unless the issue reaches `done` and every feature passes, so it cannot yet grade the healthy intermediate state where one coding loop lands one slice and deliberately leaves later features pending.

That makes `relay-e2e` the wrong acceptance layer for this specific gap today, even though it remains valuable smoke coverage for finished scenarios. For now, use repository-native tests plus real CLI commands as the source of truth, and keep the E2E limitation documented rather than pretending the current harness can express expected-partial completion.

Future viability condition: add a dedicated prompt-split scenario only after the shared E2E contract can encode an expected non-terminal state or otherwise distinguish "correctly unfinished after this loop" from a plain failure.

## Remaining Phase 3+ Gaps

- add more stable golden coverage only when the output is intentionally reviewable, such as future `watch` summaries or carefully normalized failure/help surfaces
- add failure-path subprocess coverage for missing issue state, interrupted runs, and other stderr-heavy flows that should stay stable across refactors
- extend persisted artifact fixtures if later phases begin to depend on more structured files under issue artifact directories
- add cross-platform smoke checks where path handling or shell behavior could diverge
- keep `relay-e2e` focused on orchestration until there is a clear, deterministic self-hosted scenario for Relay’s own CLI UX
