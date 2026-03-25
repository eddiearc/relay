import { createRequire } from 'node:module';

import { packageNameForPlatform, platformKey, supportedPlatforms } from './packaging.mjs';

const require = createRequire(import.meta.url);

const PACKAGE_BY_PLATFORM = new Map(
  supportedPlatforms().map(({ os, arch }) => [platformKey(os, arch), packageNameForPlatform(os, arch)]),
);

export function packageNameForRuntime(platform = process.platform, arch = process.arch) {
  const packageName = PACKAGE_BY_PLATFORM.get(platformKey(platform, arch));
  if (!packageName) {
    const supported = supportedPlatforms()
      .map(({ os, arch: supportedArch }) => `${os}/${supportedArch}`)
      .join(', ');
    throw new Error(
      `relay does not publish a binary for ${platform}/${arch}. Supported platforms: ${supported}.`,
    );
  }
  return packageName;
}

export function resolveBinaryPath(options = {}) {
  const packageName = packageNameForRuntime(options.platform, options.arch);
  try {
    return require.resolve(`${packageName}/bin/relay`);
  } catch {
    throw new Error(
      [
        `relay binary package "${packageName}" is not installed.`,
        'Reinstall without --omit=optional so npm can install the matching platform package.',
      ].join(' '),
    );
  }
}
