import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import path from 'node:path';

test('npm launcher is not ignored by git', () => {
  const result = spawnSync('git', ['check-ignore', 'npm/bin/relay.js'], {
    cwd: path.resolve(process.cwd(), '..'),
    encoding: 'utf8',
  });

  assert.notEqual(
    result.status,
    0,
    `npm/bin/relay.js must be tracked, but git ignore matched:\n${result.stdout}${result.stderr}`,
  );
});
