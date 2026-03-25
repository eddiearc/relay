import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const workflow = readFileSync('../.github/workflows/release-smoke.yml', 'utf8');

test('release smoke workflow is manually triggered', () => {
  assert.match(workflow, /workflow_dispatch:/);
});

test('release smoke workflow creates and cleans up a temporary draft release', () => {
  assert.match(workflow, /VERSION:\s+v0\.0\.0-smoke\.\$\{\{ github\.run_id \}\}/);
  assert.match(workflow, /gh release create "\$VERSION".*--draft/s);
  assert.match(workflow, /gh release upload "\$VERSION" dist\/\*\.tar\.gz --clobber/);
  assert.match(workflow, /gh release delete "\$VERSION" --cleanup-tag --yes/);
});

test('release smoke workflow prepares npm packages without publishing them', () => {
  assert.match(workflow, /npm --prefix npm run prepare-release -- \\/);
  assert.doesNotMatch(workflow, /npm --prefix npm run publish-release -- \\/);
});
