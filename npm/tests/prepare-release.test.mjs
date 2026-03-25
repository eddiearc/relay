import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const scriptPath = path.resolve('scripts/prepare-release.mjs');

async function writeFakeArchive(distDir, version, osName, arch) {
  const packageDir = path.join(distDir, `relay_${version}_${osName}_${arch}`);
  await fs.mkdir(packageDir, { recursive: true });
  await fs.writeFile(path.join(packageDir, 'relay'), `#!/bin/sh\necho ${osName}-${arch}\n`);
  const archivePath = path.join(distDir, `relay_${version}_${osName}_${arch}.tar.gz`);
  spawnSync('tar', ['-czf', archivePath, '-C', distDir, path.basename(packageDir)], {
    stdio: 'inherit',
  });
  return archivePath;
}

test('prepare-release generates the main package and platform packages from dist archives', async () => {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'relay-npm-test-'));
  const distDir = path.join(tempDir, 'dist');
  const outDir = path.join(tempDir, 'out');
  await fs.mkdir(distDir, { recursive: true });

  for (const [osName, arch] of [
    ['darwin', 'arm64'],
    ['darwin', 'amd64'],
    ['linux', 'arm64'],
    ['linux', 'amd64'],
  ]) {
    await writeFakeArchive(distDir, 'v1.2.3', osName, arch);
  }

  const result = spawnSync(
    process.execPath,
    [scriptPath, '--version', 'v1.2.3', '--dist-dir', distDir, '--out-dir', outDir],
    {
      cwd: path.resolve('.'),
      encoding: 'utf8',
    },
  );

  assert.equal(result.status, 0, result.stderr || result.stdout);

  const mainPackage = JSON.parse(
    await fs.readFile(path.join(outDir, 'relay', 'package.json'), 'utf8'),
  );
  assert.equal(mainPackage.name, '@eddiearc/relay');
  assert.equal(mainPackage.version, '1.2.3');
  assert.equal(mainPackage.bin.relay, './bin/relay.js');
  assert.ok(mainPackage.files.includes('skills'));

  const skillContent = await fs.readFile(
    path.join(outDir, 'relay', 'skills', 'relay-operator', 'SKILL.md'),
    'utf8',
  );
  assert.match(skillContent, /Relay Operator/);

  const platformBinary = await fs.readFile(
    path.join(outDir, 'relay-darwin-arm64', 'bin', 'relay'),
    'utf8',
  );
  assert.match(platformBinary, /darwin-arm64/);

  const platformPackage = JSON.parse(
    await fs.readFile(path.join(outDir, 'relay-linux-x64', 'package.json'), 'utf8'),
  );
  assert.equal(platformPackage.cpu[0], 'x64');
  assert.equal(platformPackage.os[0], 'linux');
});
