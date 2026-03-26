# Layered Verification Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the first practical layered verification system for Relay CLI with documentation plus representative golden, subprocess, and contract coverage.

**Architecture:** Extend the existing Go CLI test surface instead of replacing it. Keep pure logic in package-level tests, add a small reusable subprocess harness for real command execution, snapshot a stable help surface with checked-in golden files, validate machine-consumed JSON artifacts via fixtures, and document how contributors move from targeted checks to broader proof.

**Tech Stack:** Go, Cobra CLI, standard library subprocess/file helpers, checked-in `testdata` fixtures, repository docs under `docs/`.

---

### Task 1: Audit current verification layers

**Files:**
- Inspect: `internal/cli/app_test.go`
- Inspect: `e2e/README.md`
- Inspect: `internal/relay/features.go`
- Inspect: `internal/relay/issue.go`
- Inspect: `README.md`

**Step 1:** Read the current CLI/unit coverage and identify stable help surfaces and existing gaps.

**Step 2:** Read the current artifact loading/validation code to anchor fixture tests on existing validation paths.

**Step 3:** Read the repo E2E contract to decide whether this change should add a scenario or document why repo E2E remains a gap.

**Step 4:** Capture the findings in the new verification strategy guide instead of leaving them implicit.

### Task 2: Add golden help output coverage

**Files:**
- Modify: `internal/cli/app_test.go`
- Create: `internal/cli/testdata/help.golden`
- Create: `internal/cli/testdata/help_pipeline.golden`

**Step 1:** Write a failing test that renders `relay help` and `relay help pipeline` using the real Cobra app.

**Step 2:** Compare normalized stdout against checked-in golden files and add a `-update` refresh flag or similarly small refresh mechanism.

**Step 3:** Generate the golden files from the real command output.

**Step 4:** Run the targeted CLI test package to verify the goldens pass.

### Task 3: Add subprocess command coverage

**Files:**
- Create or modify: `internal/cli/subprocess_test.go`
- Reuse: `cmd/relay`

**Step 1:** Write a subprocess-style test helper that runs `go run ./cmd/relay` with temporary `HOME`/state/workspace directories.

**Step 2:** Cover one stable real command path that proves exit code, stdout/stderr, and filesystem side effects, preferably `relay issue add` in a temp repo workspace.

**Step 3:** Assert the resulting artifact structure through on-disk checks rather than only in-memory state.

**Step 4:** Run the targeted subprocess test and keep the helper maintainable for future commands.

### Task 4: Add artifact contract fixtures

**Files:**
- Create or modify: `internal/relay/features_test.go`
- Create: `internal/relay/testdata/feature_list_valid.json`
- Create: `internal/relay/testdata/feature_list_invalid_missing_fields.json`

**Step 1:** Write fixture-driven tests against existing feature list validation/loading logic.

**Step 2:** Cover a valid fixture plus at least one schema/failure case that would catch contract drift in review.

**Step 3:** Run the targeted relay package tests and keep failure messages specific.

### Task 5: Document contributor verification workflow

**Files:**
- Create: `docs/verification.md`
- Modify: `README.md`

**Step 1:** Write the layered verification guide with current coverage audit, test boundaries, targeted commands, `go test ./...`, CLI smoke commands, and explicit remaining gaps.

**Step 2:** Add a concise README pointer so contributors can find the guide.

**Step 3:** Explicitly decide whether repo-level `relay-e2e` changes are warranted for this task and document the outcome.

### Task 6: Verify, update artifacts, and ship

**Files:**
- Update via shell: `/Users/arc/.relay/issues/issue-94d81e0b35e5c1f6/feature_list.json`
- Append via shell: `/Users/arc/.relay/issues/issue-94d81e0b35e5c1f6/progress.txt`

**Step 1:** Run targeted package tests for each changed layer.

**Step 2:** Run `go test ./...` because shared CLI and state handling are changing.

**Step 3:** Run local CLI smoke checks such as `go run ./cmd/relay help`, `go run ./cmd/relay help pipeline`, and the chosen subprocess-covered command.

**Step 4:** Update `feature_list.json` based on verified evidence only, preserving prior truth values.

**Step 5:** Append a concise execution/handoff note to `progress.txt`.

**Step 6:** Commit, push the task branch, ensure an open PR exists, and report any remaining gaps honestly.
