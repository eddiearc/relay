#!/usr/bin/env node

import fs from 'node:fs/promises';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

import { normalizeNpmVersion, platformPackageDirName, supportedPlatforms } from '../lib/packaging.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const npmDir = path.resolve(__dirname, '..');
const entryPath = fileURLToPath(import.meta.url);

function parseArgs(argv) {
  const args = new Map();
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith('--')) {
      continue;
    }
    args.set(token.slice(2), argv[index + 1]);
    index += 1;
  }
  return args;
}

export function packagePublishOrder(packagesDir) {
  const platformDirs = supportedPlatforms().map(({ os, arch }) =>
    path.join(packagesDir, platformPackageDirName(os, arch)),
  );
  return [...platformDirs, path.join(packagesDir, 'relay')];
}

function packageAlreadyPublished(name, version) {
  try {
    execFileSync('npm', ['view', `${name}@${version}`, 'version', '--json'], {
      stdio: 'pipe',
      encoding: 'utf8',
    });
    return true;
  } catch {
    return false;
  }
}

async function readManifest(packageDir) {
  const raw = await fs.readFile(path.join(packageDir, 'package.json'), 'utf8');
  return JSON.parse(raw);
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const versionTag = args.get('version') ?? process.env.RELAY_VERSION;
  if (!versionTag) {
    throw new Error('Missing required --version <tag> argument');
  }
  const version = normalizeNpmVersion(versionTag);
  const packagesDir = path.resolve(args.get('packages-dir') ?? path.join(npmDir, 'out'));

  for (const packageDir of packagePublishOrder(packagesDir)) {
    const manifest = await readManifest(packageDir);
    if (manifest.version !== version) {
      throw new Error(
        `Package ${manifest.name} has version ${manifest.version}, expected ${version}. Run prepare-release first.`,
      );
    }

    if (packageAlreadyPublished(manifest.name, version)) {
      console.log(`Skipping ${manifest.name}@${version}; already published.`);
      continue;
    }

    console.log(`Publishing ${manifest.name}@${version}`);
    execFileSync('npm', ['publish', packageDir, '--access', 'public'], {
      stdio: 'inherit',
    });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === entryPath) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}
