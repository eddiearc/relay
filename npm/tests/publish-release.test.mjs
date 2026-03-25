import test from 'node:test';
import assert from 'node:assert/strict';

import { packagePublishOrder } from '../scripts/publish-release.mjs';

test('packagePublishOrder publishes platform packages before the root package', () => {
  assert.deepEqual(packagePublishOrder('/tmp/out'), [
    '/tmp/out/relay-darwin-arm64',
    '/tmp/out/relay-darwin-x64',
    '/tmp/out/relay-linux-arm64',
    '/tmp/out/relay-linux-x64',
    '/tmp/out/relay',
  ]);
});
