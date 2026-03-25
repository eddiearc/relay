# Relay Upgrade Command Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a testable `relay upgrade` command that reports no-op and successful upgrades with explicit before/after version output.

**Architecture:** Extend the Go CLI dispatcher with an `upgrade` subcommand and keep the upgrade behavior behind injectable seams for version lookup and command execution so tests can exercise the real CLI entry point. The command will compare the currently running version to the latest published npm version, print the required contract, and only invoke the installer path when the versions differ.

**Tech Stack:** Go CLI (`internal/cli`), Node/npm distribution docs, `make` for repo verification.

---

### Task 1: Add CLI coverage for `relay upgrade`

**Files:**
- Modify: `internal/cli/app.go`
- Test: `internal/cli/app_test.go`

**Step 1: Write the failing test**
- Add a test that invokes `run([]string{"upgrade"}, ...)` and asserts the command is recognized through the CLI dispatcher.
- Add output assertions for both `Already up to date (vX.Y.Z)` and `Upgraded: vX.Y.Z → vA.B.C`.

**Step 2: Run test to verify it fails**
- Run: `go test ./internal/cli -run 'TestUpgrade'`
- Expected: FAIL because `upgrade` is currently an unknown command.

**Step 3: Write minimal implementation**
- Add `upgrade` to the help text and command switch.
- Introduce injectable seams for current version lookup, latest version lookup, and upgrade command execution.

**Step 4: Run test to verify it passes**
- Run: `go test ./internal/cli -run 'TestUpgrade'`
- Expected: PASS.

**Step 5: Commit**
- Run: `git add internal/cli/app.go internal/cli/app_test.go && git commit -m 'Add relay upgrade command'`

### Task 2: Implement version transition behavior

**Files:**
- Modify: `internal/cli/app.go`
- Test: `internal/cli/app_test.go`

**Step 1: Write the failing test**
- Add tests for latest-version lookup failure and installer failure so the command preserves non-zero exits and useful stderr.

**Step 2: Run test to verify it fails**
- Run: `go test ./internal/cli -run 'TestUpgrade'`
- Expected: FAIL because errors are not yet handled by the new command.

**Step 3: Write minimal implementation**
- Normalize versions consistently.
- Print `Already up to date (...)` on no-op.
- Print `Upgraded: ... → ...` only after a successful installer invocation.

**Step 4: Run test to verify it passes**
- Run: `go test ./internal/cli -run 'TestUpgrade'`
- Expected: PASS.

**Step 5: Commit**
- Run: `git add internal/cli/app.go internal/cli/app_test.go && git commit -m 'Refine upgrade status messages'`

### Task 3: Verify the repo and update Relay artifacts

**Files:**
- Modify: `docs/plans/2026-03-25-upgrade-command.md`
- Modify: `/Users/arc/.relay/issues/issue-70031ad1f7f3eb34/feature_list.json`
- Append: `/Users/arc/.relay/issues/issue-70031ad1f7f3eb34/progress.txt`

**Step 1: Run targeted tests**
- Run: `go test ./internal/cli -run 'TestUpgrade'`
- Expected: PASS.

**Step 2: Run required verification**
- Run: `make build`
- Expected: exit 0.
- Run: `make test`
- Expected: exit 0.

**Step 3: Update artifacts from verified state**
- Mark feature items as passed only if the command and tests are verified.
- Record concrete verification evidence or blockers in `notes`.

**Step 4: Commit and publish branch state**
- Run: `git add ... && git commit -m 'Improve relay upgrade output'`
- Run: `git push origin HEAD`
- Ensure a PR exists with `gh pr view` / `gh pr create`.
