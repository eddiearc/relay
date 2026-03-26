# Layered Verification Phase 2 Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Broaden Relay’s layered verification rollout across stable CLI goldens, subprocess flows, persisted artifact contracts, and contributor guidance without dropping remaining rollout gaps.

**Architecture:** Extend the existing Go CLI and relay package tests instead of adding new infrastructure. Snapshot stable `status` and `report` output with normalized temp paths, add one more real subprocess flow that crosses the CLI process boundary, add fixture-backed `issue.json` contract tests through the persisted store loader, and tighten contributor docs around the default verification sequence and remaining Phase 3 gaps.

**Tech Stack:** Go, standard library test helpers, Cobra/flag-driven CLI, checked-in golden and JSON fixtures, repository docs.

---

### Task 1: Extend stable CLI golden coverage

**Files:**
- Modify: `internal/cli/golden_test.go`
- Create: `internal/cli/testdata/status.golden`
- Create: `internal/cli/testdata/report.golden`

**Step 1: Write the failing test**
- Add table-driven golden cases for `relay status` and `relay report` using temp Relay state.
- Normalize temp paths so the checked-in snapshots stay reviewable.

**Step 2: Run test to verify it fails**
- Run: `go test ./internal/cli -run TestCommandOutputGolden`
- Expected: FAIL because the new fixtures do not exist yet.

**Step 3: Write minimal implementation**
- Reuse existing snapshot setup helpers and expand the golden harness to support per-case setup plus output normalization.

**Step 4: Run test to verify it passes**
- Run: `go test ./internal/cli -run TestCommandOutputGolden -args -update`
- Run: `go test ./internal/cli -run TestCommandOutputGolden`

### Task 2: Broaden subprocess CLI coverage

**Files:**
- Modify: `internal/cli/subprocess_test.go`

**Step 1: Write the failing test**
- Add a subprocess test for one additional realistic `status`/`report` success or failure path with asserted stdout, stderr, and exit code.

**Step 2: Run test to verify it fails**
- Run: `go test ./internal/cli -run TestRelaySubprocess`
- Expected: FAIL until the new assertions and setup are complete.

**Step 3: Write minimal implementation**
- Reuse the existing subprocess harness and temporary Relay state.
- Keep assertions stable and path-aware.

**Step 4: Run test to verify it passes**
- Run: `go test ./internal/cli -run TestRelaySubprocess`

### Task 3: Add persisted `issue.json` contract fixtures

**Files:**
- Modify: `internal/relay/state_test.go`
- Create: `internal/relay/testdata/issue_valid.json`
- Create: `internal/relay/testdata/issue_invalid_missing_goal.json`
- Create: `internal/relay/testdata/issue_invalid_agent_runner.json`

**Step 1: Write the failing test**
- Add fixture-backed tests around `Store.LoadIssue` and/or `LoadIssue` that cover valid persisted issue state plus stable schema failures.

**Step 2: Run test to verify it fails**
- Run: `go test ./internal/relay -run TestStoreLoadIssueFixtures`
- Expected: FAIL until the fixtures and assertions are in place.

**Step 3: Write minimal implementation**
- Route fixtures through the same persisted loader that CLI status/report use.
- Assert success shape for the valid case and stable error snippets for invalid cases.

**Step 4: Run test to verify it passes**
- Run: `go test ./internal/relay -run TestStoreLoadIssueFixtures`

### Task 4: Refresh contributor verification guidance

**Files:**
- Modify: `docs/verification.md`
- Modify: `README.md`

**Step 1: Update docs**
- Clarify fast local iteration vs broader pre-PR proof vs local CLI verification.
- Make the current default proof path explicit: targeted package tests -> `go test ./...` -> local CLI commands.
- Preserve remaining Phase 3 gaps and the `relay-e2e` decision.

**Step 2: Verify docs references**
- Ensure README points to the verification guide and uses the same sequence.

### Task 5: Verify, update Relay artifacts, and ship

**Files:**
- Update via shell: `/Users/arc/.relay/issues/issue-0b99b2a00852c868/feature_list.json`
- Append via shell: `/Users/arc/.relay/issues/issue-0b99b2a00852c868/progress.txt`

**Step 1: Run targeted verification**
- `go test ./internal/cli -run TestCommandOutputGolden`
- `go test ./internal/cli -run TestRelaySubprocess`
- `go test ./internal/relay -run TestStoreLoadIssueFixtures`

**Step 2: Run broader verification**
- `go test ./...`

**Step 3: Run CLI smoke commands**
- `go run ./cmd/relay help`
- `go run ./cmd/relay help status`
- `go run ./cmd/relay help report`

**Step 4: Update issue artifacts from evidence**
- Mark only verified features as passing.
- Keep remaining rollout scope explicit if anything is still pending.

**Step 5: Commit, push, and ensure PR**
- Commit the tracked repo changes.
- Push `relay/layered-verification-phase2`.
- Update or create the PR.
