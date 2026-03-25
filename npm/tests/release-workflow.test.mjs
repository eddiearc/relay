import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const workflow = readFileSync('../.github/workflows/release.yml', 'utf8');

test('release workflow uses Node 24 compatible actions', () => {
  assert.match(workflow, /uses:\s+actions\/checkout@v5\b/);
  assert.match(workflow, /uses:\s+actions\/setup-go@v6\b/);
  assert.doesNotMatch(workflow, /uses:\s+softprops\/action-gh-release@/);
});

test('release workflow uploads assets with gh cli', () => {
  assert.match(workflow, /gh release upload "\$VERSION" dist\/\*\.tar\.gz --clobber/);
});
