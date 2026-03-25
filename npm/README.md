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

The GitHub release workflow already builds `dist/*.tar.gz`. After that it:

1. runs `npm --prefix npm run prepare-release`
2. publishes the four platform packages
3. publishes `@eddiearc/relay` last

## npm Setup

Preferred:

1. Create the scoped packages under the npm account or organization that owns `@eddiearc`.
2. For each package, configure npm Trusted Publisher to trust this GitHub repository and the release workflow file.
3. Keep the workflow permission `id-token: write` enabled.

Bootstrap or fallback:

1. Create an npm automation token that can publish the `@eddiearc` scope.
2. Add it to the GitHub repository secrets as `NPM_TOKEN`.
3. Re-run the release workflow.

The workflow supports both modes. Trusted publishing is preferred for steady-state releases; `NPM_TOKEN` is the fallback when bootstrapping or recovering package ownership.

## First Publish Checklist

1. Ensure the npm scope `@eddiearc` exists and your publisher account has access.
2. Publish one release tag such as `v0.1.0`.
3. Confirm all five packages exist on npm.
4. Configure Trusted Publisher on each package if the first publish used `NPM_TOKEN`.

## Important Runtime Notes

- Relay currently supports macOS and Linux only.
- The installed CLI still requires `codex` to be available in `PATH`.
