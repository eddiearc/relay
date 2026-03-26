# Relay Verification Strategy

Relay should not rely on one verification style. This repository now uses a layered model so contributors can pick the smallest realistic proof while still preserving an end-to-end path before merge.

## Current Layers

### 1. Unit and package-level tests

Use package tests for pure logic and validation rules that do not need a real CLI process.

Good fits in this repository:

- `internal/relay/spec_test.go` for pipeline and issue normalization
- `internal/relay/runner_test.go` for runner command construction and log handling
- `internal/relay/feature_test.go` for `feature_list.json` validation and transition rules
- `internal/relay/orchestrator_test.go` for orchestration behavior with controlled test doubles

Use this layer when you are checking:

- normalization and validation logic
- state transitions with in-memory or temp-dir setup
- pure formatting or helper behavior
- failure modes that do not require a subprocess boundary

Targeted commands:

- `go test ./internal/relay -run TestLoadFeatureListFixtures`
- `go test ./internal/relay -run TestValidateFeatureTransitionRejectsDeletionAndRegression`

### 2. In-process CLI tests

Use in-process CLI tests when the command surface matters, but a subprocess boundary adds little value. These tests call `run(...)` or `RunWithIO(...)` directly and are fast enough for iteration.

Good fits in this repository:

- command routing and usage text branches in `internal/cli/app_test.go`
- hint text and report formatting
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

This layer is the right fit for:

- top-level `relay help`
- stable subcommand help like `relay help pipeline`
- other long, human-facing text output where review should show exact drift

Run and refresh:

- verify: `go test ./internal/cli -run TestHelpOutputGolden`
- refresh intentionally changed output: `go test ./internal/cli -run TestHelpOutputGolden -args -update`

Do not use goldens for highly dynamic output, timestamps, or paths that churn every run.

### 4. Subprocess command integration tests

Use a real subprocess when the process boundary itself matters: exit code propagation, stdout/stderr separation, current working directory behavior, and on-disk side effects.

Current coverage:

- `internal/cli/subprocess_test.go` runs `go run ./cmd/relay`
- the covered path imports a pipeline and then runs `relay issue add` against a temporary state directory

This layer is the right fit for:

- commands that create or mutate persisted state on disk
- smoke checks for the real compiled or `go run` CLI surface
- output split across stdout and stderr

Targeted command:

- `go test ./internal/cli -run TestRelaySubprocessIssueAddCreatesArtifacts`

### 5. Contract fixtures for persisted machine-consumed artifacts

Use checked-in fixtures for JSON or YAML artifacts that other Relay phases or external tooling consume structurally.

Current coverage:

- `internal/relay/testdata/feature_list_valid.json`
- `internal/relay/testdata/feature_list_invalid_missing_title.json`
- `internal/relay/feature_test.go` loading fixtures through `LoadFeatureList`

This layer is the right fit for:

- `feature_list.json`
- `issue.json`
- template or persisted formats where schema drift should be obvious in review

Targeted command:

- `go test ./internal/relay -run TestLoadFeatureListFixtures`

## Practical Workflow

Use the smallest loop first, then widen proof before you ship.

1. Run the most targeted test for the layer you changed.
2. Run adjacent package tests if you changed shared helpers.
3. Run local CLI smoke commands for user-facing commands.
4. Run `go test ./...` before commit or PR update.

Recommended command sequence for this repository:

```bash
go test ./internal/cli -run TestHelpOutputGolden
go test ./internal/cli -run TestRelaySubprocessIssueAddCreatesArtifacts
go test ./internal/relay -run TestLoadFeatureListFixtures
go test ./...
go run ./cmd/relay help
go run ./cmd/relay help pipeline
```

If you intentionally change help text, refresh the goldens first, review the snapshot diff, and then rerun `go test ./internal/cli -run TestHelpOutputGolden` without `-update`.

## Where Each Change Belongs

- use unit/package tests for validation logic, normalization, and transition rules
- use in-process CLI tests for command dispatch and flows that need injected test doubles
- use subprocess tests for real command execution, exit codes, stdout/stderr separation, and filesystem side effects
- use goldens for stable long-form help text and other intentionally reviewed user-visible output
- use contract fixtures for persisted JSON/YAML artifacts that other loops or tools consume

## `relay-e2e` Decision for This Change

I evaluated the repo-level `relay-e2e` layer for this work and did not extend it.

Reason: the bundled `e2e/` scenarios validate Relay operating on a target repository through `relay serve --once` and an independent verification pass. That is useful for orchestration smoke coverage, but this task primarily changes Relay's own contributor verification guidance and adds repository-native tests for help output, subprocess command behavior, and artifact contracts. Those are better covered directly in `internal/cli` and `internal/relay` because they need exact assertions on help text, stderr/stdout splits, and fixture schema drift.

Current gap: Relay still does not have a repository-level scenario that self-hosts these exact CLI verification layers end to end. For now, treat `relay-e2e` as complementary smoke coverage for orchestration, not as a replacement for the repository-native layers above.

## Remaining Gaps

- only a first stable help surface is snapshot-protected today; other user-visible reports can be added later if they prove stable enough
- subprocess coverage currently exercises `pipeline import` and `issue add`; `serve`, `watch`, and failure paths still lean mostly on in-process tests
- contract fixtures currently cover `feature_list.json`; `issue.json` and other persisted artifacts can gain dedicated fixtures if their schema becomes more important to external tooling
- repo-level `relay-e2e` remains an orchestration smoke layer rather than a full self-hosted verification path for Relay's own CLI UX
