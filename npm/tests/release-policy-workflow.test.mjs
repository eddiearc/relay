import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const workflow = readFileSync('../.github/workflows/release-policy.yml', 'utf8');

test('release policy workflow runs on main pushes and manual dry runs', () => {
  assert.match(workflow, /push:\s+branches:\s+- main/s);
  assert.match(workflow, /workflow_dispatch:/);
  assert.match(workflow, /dry_run:/);
  assert.match(workflow, /published_release_tag:/);
});

test('release policy workflow inspects release decisions before publishing', () => {
  assert.match(workflow, /fetch-depth:\s+0/);
  assert.match(workflow, /git fetch --force --tags origin/);
  assert.match(workflow, /gh release list --limit 200 --exclude-drafts --json tagName,isPrerelease/);
  assert.match(workflow, /go run \.\/cmd\/relay release inspect \\\s+--repo "\$GITHUB_WORKSPACE"/s);
  assert.match(workflow, /--format json/);
  assert.match(workflow, /if: \$\{\{ env\.DRY_RUN != 'true' && steps\.inspect\.outputs\.action == 'publish-explicit-tag' \}\}/);
  assert.match(workflow, /if: \$\{\{ env\.DRY_RUN != 'true' && steps\.inspect\.outputs\.action == 'auto-cut-patch' \}\}/);
  assert.match(workflow, /gh release create "\$\{\{ steps\.inspect\.outputs\.tag \}\}" --target "\$TARGET_REF"/);
});
