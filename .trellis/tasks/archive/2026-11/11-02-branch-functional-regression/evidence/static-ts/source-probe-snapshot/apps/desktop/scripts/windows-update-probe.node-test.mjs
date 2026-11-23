import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createUpdateFeed } from './windows-update-probe.mjs';

test('isolated candidate feed serves exact bytes, HEAD, ranges, metadata revalidation and rejects unknown paths', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'windows-update-feed-'));
  await writeFile(join(directory, 'latest.yml'), 'version: 0.5.3-rc.1\n');
  await writeFile(join(directory, 'multica-desktop.exe'), '0123456789');
  const requests = [];
  const { server, url } = await createUpdateFeed(directory, requests);
  try {
    assert.equal(new URL(url).hostname, '127.0.0.1');
    const metadata = await fetch(`${url}/latest.yml`);
    assert.equal(metadata.headers.get('cache-control'), 'no-cache');
    assert.equal(await metadata.text(), 'version: 0.5.3-rc.1\n');
    const head = await fetch(`${url}/multica-desktop.exe`, { method: 'HEAD' });
    assert.equal(head.headers.get('content-length'), '10');
    assert.equal(await head.text(), '');
    const range = await fetch(`${url}/multica-desktop.exe`, { headers: { Range: 'bytes=2-5' } });
    assert.equal(range.status, 206);
    assert.equal(range.headers.get('content-range'), 'bytes 2-5/10');
    assert.equal(await range.text(), '2345');
    assert.equal(await (await fetch(`${url}/multica-desktop.exe`)).text(), '0123456789');
    assert.equal((await fetch(`${url}/multica-desktop.exe`, { headers: { Range: 'bytes=9-20' } })).status, 416);
    assert.equal((await fetch(`${url}/missing.exe`)).status, 404);
    assert.equal((await fetch(`${url}/multica-desktop.exe`, { method: 'POST' })).status, 404);
    assert.equal((await fetch(`${url}/%2e%2e/private`)).status, 404);
    assert.ok(requests.some((request) => request.range === 'bytes=2-5'));
  } finally {
    server.closeAllConnections();
    await new Promise((done) => server.close(done));
    await rm(directory, { recursive: true });
  }
});

// A PS7 parent exports PSModulePath; invoking Windows PowerShell 5 inherited a
// module path that made native CIM/Get-Acl discovery fail in earlier CI runs.
test('owned-process cleanup keeps the hosted PowerShell 7 module environment', async () => {
  const source = await readFile(new URL('./windows-update-probe.mjs', import.meta.url), 'utf8');
  assert.match(source, /await execute\('pwsh\.exe', \['-NoProfile', '-NonInteractive'/);
  assert.doesNotMatch(source, /execute\('powershell\.exe'/);
});

test('owned home refuses existing state and removes only its matching marker', async () => {
  const { claimHomeDirectory } = await import('./windows-update-probe.mjs');
  const root = await mkdtemp(join(tmpdir(), 'multica-update-home-'));
  try {
    const cleanup = await claimHomeDirectory(root, 'run-one');
    await assert.rejects(claimHomeDirectory(root, 'run-two'), { code: 'EEXIST' });
    await writeFile(join(root, '.multica', 'windows-update-owner.json'), 'other-owner');
    await assert.rejects(cleanup(), /ownership/);
    await writeFile(join(root, '.multica', 'windows-update-owner.json'), 'run-one');
    await cleanup();
    await assert.rejects(readFile(join(root, '.multica', 'windows-update-owner.json')), { code: 'ENOENT' });
  } finally { await rm(root, { recursive: true, force: true }); }
});
