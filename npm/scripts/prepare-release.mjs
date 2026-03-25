#!/usr/bin/env node

import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

import {
  archiveNameForPlatform,
  normalizeNpmVersion,
  packageNameForPlatform,
  platformPackageDirName,
  rootPackageName,
  supportedPlatforms,
} from '../lib/packaging.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const npmDir = path.resolve(__dirname, '..');
const repoRoot = path.resolve(npmDir, '..');

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

async function ensureCleanDir(targetDir) {
  await fs.rm(targetDir, { recursive: true, force: true });
  await fs.mkdir(targetDir, { recursive: true });
}

async function copyExecutable(sourcePath, targetPath) {
  await fs.mkdir(path.dirname(targetPath), { recursive: true });
  await fs.copyFile(sourcePath, targetPath);
  await fs.chmod(targetPath, 0o755);
}

async function writeJson(targetPath, value) {
  await fs.mkdir(path.dirname(targetPath), { recursive: true });
  await fs.writeFile(targetPath, `${JSON.stringify(value, null, 2)}\n`);
}

async function copyProjectDir(sourcePath, targetPath) {
  await fs.mkdir(path.dirname(targetPath), { recursive: true });
  await fs.cp(sourcePath, targetPath, { recursive: true });
}

async function rewriteBundledSkillMetadata(skillsDir, versionTag) {
  const skillMetadataPath = path.join(skillsDir, 'relay-operator', 'skill.json');
  const content = await fs.readFile(skillMetadataPath, 'utf8');
  const metadata = JSON.parse(content);
  metadata.version = versionTag;
  await writeJson(skillMetadataPath, metadata);
}

function mainPackageManifest(version) {
  const optionalDependencies = Object.fromEntries(
    supportedPlatforms().map(({ os, arch }) => [packageNameForPlatform(os, arch), version]),
  );

  return {
    name: rootPackageName(),
    version,
    description: 'Relay CLI for long-running software-engineering agents',
    bin: {
      relay: './bin/relay.js',
    },
    type: 'module',
    os: ['darwin', 'linux'],
    engines: {
      node: '>=18',
    },
    publishConfig: {
      access: 'public',
    },
    repository: {
      type: 'git',
      url: 'git+https://github.com/eddiearc/relay.git',
    },
    homepage: 'https://github.com/eddiearc/relay',
    bugs: {
      url: 'https://github.com/eddiearc/relay/issues',
    },
    files: ['bin', 'lib', 'README.md', 'skills'],
    optionalDependencies,
    keywords: ['relay', 'cli', 'agent', 'codex', 'orchestrator'],
  };
}

function platformPackageManifest(version, os, arch) {
  return {
    name: packageNameForPlatform(os, arch),
    version,
    description: `Prebuilt Relay binary for ${os}/${arch}`,
    os: [os],
    cpu: [arch],
    publishConfig: {
      access: 'public',
    },
    repository: {
      type: 'git',
      url: 'git+https://github.com/eddiearc/relay.git',
    },
    homepage: 'https://github.com/eddiearc/relay',
    bugs: {
      url: 'https://github.com/eddiearc/relay/issues',
    },
    files: ['bin/relay', 'README.md'],
    preferUnplugged: true,
    keywords: ['relay', 'cli', 'agent', os, arch],
  };
}

async function writeRootReadme(targetPath, versionTag) {
  const rootReadme = await fs.readFile(path.join(repoRoot, 'README.md'), 'utf8');
  const installSection = [
    '## npm Package',
    '',
    `This package was generated from the GitHub release tag \`${versionTag}\`.`,
    '',
    'Install with:',
    '',
    '```bash',
    `npm install -g ${rootPackageName()}`,
    '```',
    '',
  ].join('\n');
  await fs.writeFile(targetPath, `${installSection}${rootReadme}`);
}

async function writePlatformReadme(targetPath, os, arch, versionTag) {
  const content = [
    `# ${packageNameForPlatform(os, arch)}`,
    '',
    `Prebuilt Relay binary for ${os}/${arch}.`,
    '',
    `Generated from GitHub release tag \`${versionTag}\`.`,
    '',
    `Install the top-level package instead: \`${rootPackageName()}\`.`,
    '',
  ].join('\n');
  await fs.writeFile(targetPath, content);
}

async function extractBinaryFromArchive(archivePath, targetPath) {
  const tempRoot = await fs.mkdtemp(path.join(path.dirname(targetPath), 'extract-'));
  try {
    execFileSync('tar', ['-xzf', archivePath, '-C', tempRoot], { stdio: 'inherit' });
    const [extractedDir] = await fs.readdir(tempRoot);
    if (!extractedDir) {
      throw new Error(`Archive ${archivePath} did not contain a package directory`);
    }
    await copyExecutable(path.join(tempRoot, extractedDir, 'relay'), targetPath);
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const versionTag = args.get('version') ?? process.env.RELAY_VERSION;
  if (!versionTag) {
    throw new Error('Missing required --version <tag> argument');
  }

  const distDir = path.resolve(args.get('dist-dir') ?? path.join(repoRoot, 'dist'));
  const outDir = path.resolve(args.get('out-dir') ?? path.join(npmDir, 'out'));
  const version = normalizeNpmVersion(versionTag);

  await ensureCleanDir(outDir);

  const mainDir = path.join(outDir, 'relay');
  await fs.mkdir(path.join(mainDir, 'bin'), { recursive: true });
  await fs.mkdir(path.join(mainDir, 'lib'), { recursive: true });
  await copyExecutable(path.join(npmDir, 'bin', 'relay.js'), path.join(mainDir, 'bin', 'relay.js'));
  await fs.copyFile(
    path.join(npmDir, 'lib', 'resolve-binary.mjs'),
    path.join(mainDir, 'lib', 'resolve-binary.mjs'),
  );
  await fs.copyFile(path.join(npmDir, 'lib', 'packaging.mjs'), path.join(mainDir, 'lib', 'packaging.mjs'));
  await copyProjectDir(path.join(repoRoot, 'skills'), path.join(mainDir, 'skills'));
  await rewriteBundledSkillMetadata(path.join(mainDir, 'skills'), versionTag);
  await writeJson(path.join(mainDir, 'package.json'), mainPackageManifest(version));
  await writeRootReadme(path.join(mainDir, 'README.md'), versionTag);

  for (const { os, arch } of supportedPlatforms()) {
    const packageDir = path.join(outDir, platformPackageDirName(os, arch));
    const archivePath = path.join(distDir, archiveNameForPlatform(versionTag, os, arch));
    await fs.access(archivePath);
    await fs.mkdir(path.join(packageDir, 'bin'), { recursive: true });
    await extractBinaryFromArchive(archivePath, path.join(packageDir, 'bin', 'relay'));
    await writeJson(path.join(packageDir, 'package.json'), platformPackageManifest(version, os, arch));
    await writePlatformReadme(path.join(packageDir, 'README.md'), os, arch, versionTag);
  }
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
