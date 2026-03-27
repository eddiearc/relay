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
  assert.match(workflow, /gh release delete "\$VERSION" --yes/);
  assert.match(workflow, /git ls-remote --exit-code --tags origin "\$VERSION"/);
  assert.match(workflow, /git push origin ":refs\/tags\/\$VERSION"/);
});

test('release smoke workflow prepares npm packages and validates them without publishing', () => {
  assert.ok(workflow.includes('npm --prefix npm run prepare-release -- \\\n'));
  assert.match(workflow, /npm pack --dry-run "\$package_dir"/);
  assert.ok(!workflow.includes('npm --prefix npm run publish-release -- \\\n'));
});
