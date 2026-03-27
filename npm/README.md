# npm Distribution

Relay ships to npm as one launcher package plus four platform-specific binary packages.

## Package Layout

- `@eddiearc/relay`
  - the public entry point
  - installs a thin Node launcher as the `relay` command
  - depends on platform packages through `optionalDependencies`
- `@eddiearc/relay-darwin-arm64`
- `@eddiearc/relay-darwin-x64`
- `@eddiearc/relay-linux-arm64`
- `@eddiearc/relay-linux-x64`

The platform packages contain only the prebuilt `relay` binary. npm selects them through their `os` and `cpu` fields, while the root package resolves the matching binary at runtime.

## Bundled Skill

The top-level `@eddiearc/relay` package also ships a project-local agent skill under:

```text
skills/relay-operator/
```

Use it as the canonical self-contained skill for agents that need to operate the Relay CLI against a repository.

Released packages also rewrite `skills/relay-operator/skill.json` to the published Relay tag so the bundled skill can track the packaged CLI version and expose a stable refresh command.

## Local Packaging

Build release archives first:

```bash
make package-all VERSION=v0.1.0
```

Generate npm packages from `dist/*.tar.gz`:

```bash
npm --prefix npm run prepare-release -- \
  --version v0.1.0 \
  --dist-dir "$PWD/dist" \
  --out-dir "$PWD/npm/out"
```

Publish the generated packages in the correct order:

```bash
npm --prefix npm run publish-release -- \
  --version v0.1.0 \
  --packages-dir "$PWD/npm/out"
```

The publish script is idempotent for an already-published version. It skips packages that already exist on npm.

## CI Publishing

Relay now uses one official release workflow plus one smoke workflow:

1. `release-policy.yml` evaluates `main` against the latest published release.
2. It no-ops when `main` is already covered.
3. It creates an explicit stable release when `HEAD` already carries one.
4. Otherwise it creates the next patch release from the latest published tag.
5. The same `release-policy.yml` run then builds `dist/*.tar.gz`, runs `npm --prefix npm run prepare-release`, publishes the four platform packages, and publishes `@eddiearc/relay` last.
6. `release-smoke.yml` remains the packaging-only dry run that validates generated packages without publishing them to npm.

For a workflow-safe dry run, trigger `release-policy.yml` manually with `dry_run=true`. Use `published_release_tag` if you want to simulate a published baseline without creating a real release first.

## npm Setup

Preferred:

1. Create the scoped packages under the npm account or organization that owns `@eddiearc`.
2. For each package, configure npm Trusted Publisher to trust:
   - GitHub user or org: `eddiearc`
   - Repository: `relay`
   - Workflow filename: `release-policy.yml`
3. Keep the workflow permission `id-token: write` enabled.
4. Publish from GitHub-hosted runners only.

Packages that need Trusted Publisher configured:

- `@eddiearc/relay`
- `@eddiearc/relay-darwin-arm64`
- `@eddiearc/relay-darwin-x64`
- `@eddiearc/relay-linux-arm64`
- `@eddiearc/relay-linux-x64`

Fallback:

1. Create a granular access token with write access and bypass 2FA enabled.
2. Add it to the GitHub repository secrets as `NPM_TOKEN`.
3. Re-add `NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}` to the publish step if you intentionally want token-based publishing again.

Trusted publishing is now the primary path in CI. Token-based publishing is only a fallback if you explicitly wire it back in.

## First Publish Checklist

1. Ensure the npm scope `@eddiearc` exists and your publisher account has access.
2. Configure Trusted Publisher for all five packages using workflow filename `release-policy.yml`.
3. Publish one release tag such as `v0.1.0`.
4. Confirm all five packages exist on npm.

## Important Runtime Notes

- Relay currently supports macOS and Linux only.
- The installed CLI requires the selected local runner (`codex` or `claude`) to be available in `PATH`.
- If neither the issue nor the pipeline sets `agent_runner`, Relay defaults to `codex`.
