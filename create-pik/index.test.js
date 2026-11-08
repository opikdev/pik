import assert from 'node:assert/strict';
import { execFileSync, spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';

const version = '1.2.3';
const os = { darwin: 'darwin', linux: 'linux' }[process.platform];
const arch = { x64: 'amd64', arm64: 'arm64' }[process.arch];
const archive = `pik_${version}_${os}_${arch}.tar.gz`;
const work = mkdtempSync(join(tmpdir(), 'create-pik-'));
const files = new Map();
const requests = [];
let server;
let releases;
let entry;

function buildPackage() {
  const pkg = join(work, 'pkg');
  mkdirSync(pkg, { recursive: true });
  writeFileSync(join(pkg, 'package.json'), JSON.stringify({ name: 'create-pik', version, type: 'module' }));
  writeFileSync(join(pkg, 'index.js'), readFileSync(new URL('./index.js', import.meta.url)));
  return join(pkg, 'index.js');
}

function setArchive(script) {
  const build = mkdtempSync(join(work, 'build-'));
  writeFileSync(join(build, 'pik'), script);
  chmodSync(join(build, 'pik'), 0o755);
  execFileSync('tar', ['-czf', join(build, archive), '-C', build, 'pik']);
  const data = readFileSync(join(build, archive));
  files.set(`/download/v${version}/${archive}`, data);
  files.set(`/download/v${version}/checksums.txt`, `${createHash('sha256').update(data).digest('hex')}  ${archive}\n`);
}

function run(args, cache) {
  return new Promise((resolve) => {
    const child = spawn(process.execPath, [entry, ...args], {
      env: { ...process.env, PIK_DOWNLOAD_URL: releases, XDG_CACHE_HOME: cache },
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.stderr.on('data', (chunk) => {
      stderr += chunk;
    });
    child.on('close', (status) => resolve({ status, stdout, stderr }));
  });
}

const freshCache = () => mkdtempSync(join(work, 'cache-'));

before(async () => {
  entry = buildPackage();
  server = createServer((request, response) => {
    requests.push(request.url);
    const body = files.get(request.url);
    response.statusCode = body === undefined ? 404 : 200;
    response.end(body ?? 'not found');
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  releases = `http://127.0.0.1:${server.address().port}`;
});

after(() => {
  server.close();
  rmSync(work, { recursive: true, force: true });
});

test('downloads pik, verifies it, and runs pik new with the arguments', async () => {
  setArchive('#!/bin/sh\necho "args: $*"\n');
  const cache = freshCache();
  const result = await run(['my-api', '--yes'], cache);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), 'args: new my-api --yes');
  assert.ok(existsSync(join(cache, 'pik', version, 'pik')));
});

test('reuses the cached binary', async () => {
  setArchive('#!/bin/sh\necho cached\n');
  const cache = freshCache();
  await run([], cache);
  const count = requests.length;
  assert.equal((await run([], cache)).stdout.trim(), 'cached');
  assert.equal(requests.length, count);
});

test('passes on the exit code of pik', async () => {
  setArchive('#!/bin/sh\nexit 3\n');
  assert.equal((await run([], freshCache())).status, 3);
});

test('refuses an archive whose checksum does not match', async () => {
  setArchive('#!/bin/sh\necho should-not-run\n');
  files.set(`/download/v${version}/checksums.txt`, `${'0'.repeat(64)}  ${archive}\n`);
  const cache = freshCache();
  const result = await run([], cache);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /doesn't match, so pik wasn't run/);
  assert.equal(result.stdout, '');
  assert.ok(!existsSync(join(cache, 'pik', version, 'pik')));
});

test('fails clearly when the release is missing', async () => {
  files.clear();
  const result = await run([], freshCache());
  assert.equal(result.status, 1);
  assert.match(result.stderr, /HTTP 404/);
});

test('prints its version without downloading', async () => {
  const count = requests.length;
  assert.equal((await run(['--version'], freshCache())).stdout.trim(), version);
  assert.equal(requests.length, count);
});
