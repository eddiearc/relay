# Upgrade Command Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `relay upgrade` command that exposes help, detects supported install methods, executes the correct self-update command, reports versions, and fails clearly.

**Architecture:** Extend `internal/cli/app.go` with a dedicated `runUpgrade` path that depends on small injectable seams for install-source detection, command execution, and version lookup. Cover the behavior with focused tests in `internal/cli/app_test.go` before implementation, then validate the whole repository with the existing Make targets.

**Tech Stack:** Go, standard library `flag`/`os/exec`, existing Relay CLI test harness.

---

### Task 1: Add failing CLI tests for help and detection

**Files:**
- Modify: `internal/cli/app_test.go`
- Modify: `internal/cli/app.go`

**Step 1: Write the failing test**

Add tests that assert `relay help` lists `upgrade`, `relay upgrade --help` returns usage text, install-source detection maps npm and `go install` paths correctly, and local builds print that self-upgrade is unavailable.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run 'Test(UsageListsUpgradeCommand|UpgradeHelp|DetectInstallMethod|UpgradeLocalBuildUnavailable)'`
Expected: FAIL because `upgrade` is not registered yet.

**Step 3: Write minimal implementation**

Register `upgrade` in the top-level command switch and add a small detection helper plus help text.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run 'Test(UsageListsUpgradeCommand|UpgradeHelp|DetectInstallMethod|UpgradeLocalBuildUnavailable)'`
Expected: PASS.

**Step 5: Commit**

Run: `git add internal/cli/app.go internal/cli/app_test.go docs/plans/$(date +%F)-upgrade-command.md && git commit -m "feat: add relay upgrade command"`

### Task 2: Add failing tests for updater execution and version reporting

**Files:**
- Modify: `internal/cli/app_test.go`
- Modify: `internal/cli/app.go`

**Step 1: Write the failing test**

Add tests that assert npm installs run `npm update -g @eddiearc/relay`, `go install` installs run `go install github.com/eddiearc/relay/cmd/relay@latest`, successful upgrades print the current and resulting versions, and execution errors bubble up with actionable messaging.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run 'Test(UpgradeRunsNPMCommand|UpgradeRunsGoInstallCommand|UpgradeReportsVersions|UpgradeReturnsCommandErrors)'`
Expected: FAIL because the execution and reporting logic does not exist yet.

**Step 3: Write minimal implementation**

Add testable seams for command execution and post-upgrade version lookup, then implement the minimal output/error flow needed to satisfy the tests.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run 'Test(UpgradeRunsNPMCommand|UpgradeRunsGoInstallCommand|UpgradeReportsVersions|UpgradeReturnsCommandErrors)'`
Expected: PASS.

**Step 5: Commit**

Run: `git add internal/cli/app.go internal/cli/app_test.go && git commit -m "feat: support relay self-upgrade"`

### Task 3: Full verification and artifact updates

**Files:**
- Modify: `docs/plans/$(date +%F)-upgrade-command.md`
- Modify: `/Users/arc/.relay/issues/issue-7d8705e0fae3f0bf/feature_list.json`
- Modify: `/Users/arc/.relay/issues/issue-7d8705e0fae3f0bf/progress.txt`

**Step 1: Run repository verification**

Run: `make build && make test`
Expected: both commands exit 0.

**Step 2: Verify branch and PR state**

Run: `git branch --show-current && gh pr view --json number,url,state,headRefName || gh pr create --fill`
Expected: task branch stays open and has one PR.

**Step 3: Update Relay artifacts**

Record verified evidence in `feature_list.json` without flipping any passing feature back to false, then append a concise handoff note to `progress.txt`.

**Step 4: Commit and push**

Run: `git add ... && git commit -m "feat: add relay upgrade flow" && git push`
Expected: branch is pushed successfully.
