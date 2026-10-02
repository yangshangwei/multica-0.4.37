import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtemp, writeFile, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { randomUUID } from 'node:crypto';

const execute = promisify(execFile);
test('contained provider requires ownership and emits one successful task result', { timeout: 60000 }, async () => {
  const root = await mkdtemp(join(tmpdir(), 'multica-provider-test-'));
  const binary = join(root, process.platform === 'win32' ? 'fixture.exe' : 'fixture');
  try {
    await execute('go', ['build', '-o', binary, join(import.meta.dirname, 'windows-business-provider.go')]);
    // Production intentionally removes inherited MULTICA_* before task launch.
    const inherited = { ...process.env, WINDOWS_BUSINESS_STATE_DIR: root, MULTICA_BUSINESS_STATE_DIR: 'must-be-filtered', MULTICA_FIXTURE_UNTRUSTED: 'must-be-filtered' };
    const env = Object.fromEntries(Object.entries(inherited).filter(([key]) => !key.toUpperCase().startsWith('MULTICA_')));
    env.MULTICA_TASK_ID = randomUUID();
    assert.equal(env.MULTICA_BUSINESS_STATE_DIR, undefined);
    assert.equal(env.MULTICA_FIXTURE_UNTRUSTED, undefined);
    await assert.rejects(execute(binary, ['--version'], { env }), error => error.code === 78);
    await writeFile(join(root, 'server-private.json'), '{}');
    assert.match((await execute(binary, ['--version'], { env })).stdout, /Claude Code/);
    await assert.rejects(execute(binary, ['unexpected'], { env }), error => error.code === 64);
    await writeFile(join(root, 'release-' + env.MULTICA_TASK_ID), 'release');
    const result = await execute(binary, ['--output-format', 'stream-json'], { env, timeout: 5000 });
    const events = result.stdout.trim().split('\n').map(JSON.parse);
    assert.deepEqual(events.map(event => event.type), ['system', 'assistant', 'result']);
    assert.equal(events[2].subtype, 'success');
    assert.equal(events[2].total_cost_usd, 0);
    const starts = (await readFile(join(root, 'provider-starts.jsonl'), 'utf8')).trim().split('\n').map(JSON.parse);
    assert.equal(starts.length, 1); assert.equal(starts[0].task_id, env.MULTICA_TASK_ID);
  } finally { await rm(root, { recursive: true }); }
});
