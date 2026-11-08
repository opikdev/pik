#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

const { version } = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8'));
const releases = process.env.PIK_DOWNLOAD_URL ?? 'https://github.com/opikdev/pik/releases';
const timeoutMs = 60_000;

const platforms = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
const architectures = { x64: 'amd64', arm64: 'arm64' };

function fail(message) {
  console.error(`create-pik: ${message}`);
  process.exit(1);
}

async function download(url) {
  let response;
  try {
    response = await fetch(url, { signal: AbortSignal.timeout(timeoutMs) });
  } catch (error) {
    fail(`couldn't download ${url}: ${error.message}`);
  }
  if (!response.ok) fail(`couldn't download ${url}: HTTP ${response.status}`);
  return Buffer.from(await response.arrayBuffer());
}

async function install(os, arch, binaryPath) {
  const archive = `pik_${version}_${os}_${arch}.${os === 'windows' ? 'zip' : 'tar.gz'}`;
  const base = `${releases}/download/v${version}`;
  const [data, checksums] = await Promise.all([download(`${base}/${archive}`), download(`${base}/checksums.txt`)]);

  const expected = checksums
    .toString('utf8')
    .split('\n')
    .map((line) => line.trim().split(/\s+/))
    .find(([, file]) => file === archive)?.[0];
  if (!expected) fail(`checksums.txt has no entry for ${archive}`);
  const actual = createHash('sha256').update(data).digest('hex');
  if (actual !== expected) fail(`the checksum of ${archive} doesn't match, so pik wasn't run`);

  const cacheDir = join(binaryPath, '..');
  mkdirSync(cacheDir, { recursive: true });
  const staging = mkdtempSync(join(cacheDir, '.download-'));
  try {
    writeFileSync(join(staging, archive), data);
    const binary = os === 'windows' ? 'pik.exe' : 'pik';
    const extract = spawnSync('tar', ['-xf', archive, binary], { cwd: staging, stdio: 'inherit' });
    if (extract.status !== 0) fail(`couldn't extract ${archive}. Make sure tar is installed`);
    chmodSync(join(staging, binary), 0o755);
    renameSync(join(staging, binary), binaryPath);
  } finally {
    rmSync(staging, { recursive: true, force: true });
  }
}

async function main() {
  const args = process.argv.slice(2);
  if (args[0] === '--version') {
    console.log(version);
    return;
  }

  const os = platforms[process.platform];
  const arch = architectures[process.arch];
  if (!os || !arch) fail(`${process.platform}/${process.arch} isn't supported`);

  const cacheRoot = process.env.XDG_CACHE_HOME ?? join(homedir(), '.cache');
  const binaryPath = join(cacheRoot, 'pik', version, os === 'windows' ? 'pik.exe' : 'pik');
  if (!existsSync(binaryPath)) await install(os, arch, binaryPath);

  const result = spawnSync(binaryPath, ['new', ...args], { stdio: 'inherit' });
  if (result.error) fail(`couldn't run pik: ${result.error.message}`);
  process.exit(result.status ?? 1);
}

await main();
