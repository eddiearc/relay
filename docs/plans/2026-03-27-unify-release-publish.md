# Unify Release Publish Implementation Plan

> **For Codex:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move stable GitHub release creation and npm publication into one official workflow while keeping a separate smoke-only packaging path.

**Architecture:** Keep `release-policy.yml` as the policy/decision entrypoint on `main`, but extend it so the same workflow performs the packaging and npm publication after it decides or creates the stable release. Keep `release-smoke.yml` as the explicit dry-run workflow for packaging validation, and update workflow-level tests plus maintainer docs so the new trigger model is obvious and verifiable.

**Tech Stack:** GitHub Actions YAML, Node workflow tests under `npm/tests`, Go CLI release inspection, Markdown docs.

---

### Task 1: Lock the new workflow contract in tests

**Files:**
- Modify: `npm/tests/release-policy-workflow.test.mjs`
- Modify: `npm/tests/release-smoke-workflow.test.mjs`

**Step 1: Write failing workflow assertions**

Add assertions that `release-policy.yml` performs checkout, test, packaging, release asset upload, npm package generation, and npm publication itself, and that it does not depend on a `release.published` trigger.

**Step 2: Run tests to verify they fail**

Run: `npm --prefix npm test -- --test-name-pattern="release policy|release smoke"`
Expected: FAIL because `release-policy.yml` still only creates releases and `release.yml` still owns packaging/publish.

**Step 3: Keep smoke-path assertions focused**

Retain assertions that `release-smoke.yml` creates a temporary draft release, uploads assets, prepares npm packages, validates them, and does not publish to npm.

**Step 4: Re-run targeted workflow tests**

Run: `npm --prefix npm test -- --test-name-pattern="release policy|release smoke"`
Expected: still failing until workflow YAML is updated.

**Step 5: Commit**

Run after implementation: `git add npm/tests/release-policy-workflow.test.mjs npm/tests/release-smoke-workflow.test.mjs && git commit -m "test: lock unified release workflow contract"`

### Task 2: Unify the official release workflow

**Files:**
- Modify: `.github/workflows/release-policy.yml`
- Modify: `.github/workflows/release.yml`

**Step 1: Update the policy workflow**

Make `release-policy.yml` responsible for release decisioning plus official packaging/publishing. It should compute the tag, optionally create the release, then run tests, build archives, upload assets, generate npm packages, and publish them when not in dry-run mode.

**Step 2: Preserve inspectability**

Keep job or step boundaries clear so maintainers can see where release decisioning ends and packaging/publish begins.

**Step 3: Remove the official dependency on `release.published`**

Either retire `release.yml` or convert it away from being the official publish path so the stable release-to-npm path no longer waits for a downstream release event.

**Step 4: Run targeted workflow tests**

Run: `npm --prefix npm test -- --test-name-pattern="release policy|release smoke"`
Expected: PASS.

**Step 5: Commit**

Run after implementation: `git add .github/workflows/release-policy.yml .github/workflows/release.yml && git commit -m "ci: unify release creation and npm publish"`

### Task 3: Update maintainer docs

**Files:**
- Modify: `README.md`
- Modify: `npm/README.md`

**Step 1: Rewrite the release flow docs**

Describe the single official `release-policy.yml` path, explain how Trusted Publishing fits into that workflow, and keep the smoke workflow instructions explicit.

**Step 2: Keep local inspection guidance**

Retain the `relay release inspect` local verification path and connect it to the GitHub workflow behavior.

**Step 3: Run targeted doc-adjacent checks**

Run: `rg -n "release\.yml|release-policy\.yml|release-smoke\.yml|Trusted Publisher|Trusted Publishing" README.md npm/README.md`
Expected: Docs consistently describe one official release workflow plus one smoke workflow.

**Step 4: Commit**

Run after implementation: `git add README.md npm/README.md && git commit -m "docs: explain unified release publishing flow"`

### Task 4: Verify end to end as far as locally realistic

**Files:**
- Verify: `.github/workflows/release-policy.yml`
- Verify: `.github/workflows/release-smoke.yml`
- Verify: `internal/release/...`
- Verify: `cmd/relay/...`

**Step 1: Run targeted tests**

Run: `npm --prefix npm test`
Expected: PASS.

**Step 2: Run Go verification**

Run: `go test ./internal/release ./internal/cli`
Expected: PASS.

**Step 3: Run broader Go coverage**

Run: `go test ./...`
Expected: PASS.

**Step 4: Exercise the local CLI inspection path**

Run: `go run ./cmd/relay release inspect --repo . --main-ref HEAD --published-release-tag v0.2.2`
Expected: prints an inspectable release decision without needing GitHub release events.

**Step 5: Record the E2E decision**

State explicitly that `relay-e2e` is not a good fit for this change because GitHub Actions release publication, release asset upload, and npm Trusted Publishing require external GitHub/npm infrastructure that local Relay E2E scenarios do not emulate.
