import test from 'node:test';
import assert from 'node:assert/strict';

import {
  normalizeNpmVersion,
  packageNameForPlatform,
  supportedPlatforms,
} from '../lib/packaging.mjs';

test('normalizeNpmVersion strips a leading v from git tags', () => {
  assert.equal(normalizeNpmVersion('v1.2.3'), '1.2.3');
  assert.equal(normalizeNpmVersion('1.2.3'), '1.2.3');
});

test('packageNameForPlatform maps darwin arm64 to the expected package name', () => {
  assert.equal(
    packageNameForPlatform('darwin', 'arm64'),
    '@eddiearc/relay-darwin-arm64',
  );
});

test('supportedPlatforms lists the release platforms in publish order', () => {
  assert.deepEqual(supportedPlatforms(), [
    { os: 'darwin', arch: 'arm64' },
    { os: 'darwin', arch: 'x64' },
    { os: 'linux', arch: 'arm64' },
    { os: 'linux', arch: 'x64' },
  ]);
});
