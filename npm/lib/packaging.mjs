const PACKAGE_SCOPE = '@eddiearc';
const PACKAGE_BASENAME = 'relay';

const PLATFORMS = [
  { os: 'darwin', arch: 'arm64' },
  { os: 'darwin', arch: 'x64' },
  { os: 'linux', arch: 'arm64' },
  { os: 'linux', arch: 'x64' },
];

const GO_ARCH_BY_NPM_ARCH = new Map([
  ['x64', 'amd64'],
  ['arm64', 'arm64'],
]);

export function normalizeNpmVersion(version) {
  return version.startsWith('v') ? version.slice(1) : version;
}

export function supportedPlatforms() {
  return PLATFORMS.map((platform) => ({ ...platform }));
}

export function packageNameForPlatform(os, arch) {
  return `${PACKAGE_SCOPE}/${PACKAGE_BASENAME}-${os}-${arch}`;
}

export function platformPackageDirName(os, arch) {
  return `${PACKAGE_BASENAME}-${os}-${arch}`;
}

export function goArchForNpmArch(arch) {
  const goArch = GO_ARCH_BY_NPM_ARCH.get(arch);
  if (!goArch) {
    throw new Error(`Unsupported npm architecture: ${arch}`);
  }
  return goArch;
}

export function archiveNameForPlatform(versionTag, os, arch) {
  return `${PACKAGE_BASENAME}_${versionTag}_${os}_${goArchForNpmArch(arch)}.tar.gz`;
}

export function platformKey(os, arch) {
  return `${os}:${arch}`;
}

export function rootPackageName() {
  return `${PACKAGE_SCOPE}/${PACKAGE_BASENAME}`;
}
