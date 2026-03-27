import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const workflow = readFileSync('../.github/workflows/release-policy.yml', 'utf8');

test('official release workflow uses Node 24 compatible actions', () => {
  assert.match(workflow, /uses:\s+actions\/checkout@v5\b/);
  assert.match(workflow, /uses:\s+actions\/setup-go@v6\b/);
  assert.match(workflow, /uses:\s+actions\/setup-node@v6\b/);
  assert.doesNotMatch(workflow, /uses:\s+softprops\/action-gh-release@/);
});

test('official release workflow uploads assets and publishes npm from release policy', () => {
  assert.match(workflow, /name:\s+Build, upload, and publish release artifacts/);
  assert.match(workflow, /gh release upload "\$VERSION" dist\/\*\.tar\.gz --clobber/);
  assert.ok(workflow.includes('npm --prefix npm run publish-release -- \\\n'));
});
