import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createServer } from "node:http";
import { mkdtemp, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout } from "node:timers/promises";
import test from "node:test";

async function fixture(t) {
  const directory = await mkdtemp(join(tmpdir(), "multica-check-renderer-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  return directory;
}

function launch(t, directory, port) {
  const ready = join(directory, "ready.json");
  const child = spawn(process.execPath, [new URL("./check-desktop-renderer.mjs", import.meta.url).pathname, directory, String(port), ready], { stdio: ["ignore", "pipe", "pipe"] });
  const closed = once(child, "close");
  let output = "";
  child.stdout.on("data", data => { output += data; });
  child.stderr.on("data", data => { output += data; });
  t.after(async () => { if (child.exitCode === null) child.kill(); await closed; });
  return { child, closed, ready, output: () => output };
}

// Use real HTTP and the installed preview server: no mocked readiness or files.
test("serves the built renderer and assets, records its listener, and stops", { timeout: 15_000 }, async t => {
  const directory = await fixture(t);
  const html = '<!doctype html><div id="root">candidate</div>';
  await writeFile(join(directory, "index.html"), html);
  await writeFile(join(directory, "candidate.js"), 'console.log("candidate");');
  const process = launch(t, directory, 0);
  let ready;
  for (let i = 0; i < 100; i++) {
    try { ready = JSON.parse(await readFile(process.ready, "utf8")); break; } catch { /* Startup not finished. */ }
    assert.equal(process.child.exitCode, null, process.output());
    await setTimeout(50);
  }
  assert.ok(ready, process.output());
  assert.equal(ready.pid, process.child.pid);
  assert.equal(ready.renderer, await realpath(directory));
  assert.match(ready.index_sha256, /^[a-f0-9]{64}$/);
  assert.equal(new URL(ready.origin).hostname, "127.0.0.1");
  assert.equal(await (await fetch(ready.origin)).text(), html);
  assert.equal(await (await fetch(ready.origin + "/candidate.js")).text(), 'console.log("candidate");');
  process.child.kill();
  assert.equal((await process.closed)[0], 0);
  await assert.rejects(fetch(ready.origin));
});

test("missing build exits before creating readiness evidence", { timeout: 15_000 }, async t => {
  const directory = await fixture(t);
  const process = launch(t, directory, 0);
  assert.notEqual((await process.closed)[0], 0);
  await assert.rejects(readFile(process.ready));
});

test("a busy port fails without replacing the existing server", { timeout: 15_000 }, async t => {
  const directory = await fixture(t);
  await writeFile(join(directory, "index.html"), "candidate");
  const existing = createServer((_req, res) => res.end("original"));
  existing.listen(0, "127.0.0.1");
  await once(existing, "listening");
  t.after(() => new Promise(resolve => existing.close(resolve)));
  const port = existing.address().port;
  const process = launch(t, directory, port);
  assert.notEqual((await process.closed)[0], 0);
  await assert.rejects(readFile(process.ready));
  assert.equal(await (await fetch(`http://127.0.0.1:${port}`)).text(), "original");
});
