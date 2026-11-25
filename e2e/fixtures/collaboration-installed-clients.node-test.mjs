import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import test from "node:test";
import { invokeInstalledCLI } from "./collaboration-installed-clients.ts";

async function scenario(mode, run) {
  const root = mkdtempSync(join(tmpdir(), "installed-cli-cleanup-"));
  const previous = { ...process.env };
  const events = join(root, "events");
  const relay = join(root, "relay");
  writeFileSync(relay, "test relay");
  writeFileSync(join(root, "docker"), `#!${process.execPath}
const fs = require('node:fs');
const args=process.argv.slice(2);
fs.appendFileSync(process.env.TEST_EVENTS, JSON.stringify(args)+'\\n');
if (args[0]==='rm' && args[1]==='--force' && process.env.TEST_MODE !== 'success') process.exit(13);
if (args[0]==='volume' && args[1]==='rm' && process.env.TEST_MODE !== 'success') process.exit(14);
if(args[0]==='start') {
 const lines=require('node:readline').createInterface({input:process.stdin});
 let boot;
 lines.on('line', line=>{
  const data=JSON.parse(line);
  if(!boot){boot=data;console.log(JSON.stringify({kind:'request',id:1,method:'PUT',path:'/api/issues/test',headers:{Authorization:['Bearer '+data.token],'X-Client-Version':['v0.6.7'],'X-Client-Platform':['cli'],'X-Client-OS':['linux']}}));}
  else {console.log(JSON.stringify({kind:'result',exit_code:0,stdout:'ok'}));process.exit(process.env.TEST_MODE==='both-fail'?1:0);}
 });
}
`, { mode: 0o700 });
  const server = createServer((_req, res) => { res.end('{}'); });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  process.env.PATH = root + ":" + dirname(process.execPath);
  process.env.COLLABORATION_INSTALLED_CLI_RELAY = relay;
  process.env.TEST_EVENTS = events;
  process.env.TEST_MODE = mode;
  try {
    await run(() => invokeInstalledCLI({ apiUrl: `http://127.0.0.1:${server.address().port}`, token: "synthetic-token",
      workspaceId: "synthetic-workspace", args: ["issue", "update", "test"], expected: { method: "PUT", path: "/api/issues/test" } }));
    const calls = readFileSync(events, "utf8").trim().split("\n").map(JSON.parse);
    assert.equal(calls.filter(args => args[0] === "rm" && args[1] === "--force").length, 1);
    assert.equal(calls.filter(args => args[0] === "volume" && args[1] === "rm").length, 1);
  } finally {
    for (const key of Object.keys(process.env)) if (!(key in previous)) delete process.env[key];
    Object.assign(process.env, previous);
    await new Promise(resolve => server.close(resolve));
    rmSync(root, { recursive: true, force: true });
  }
}

test("installed CLI success retains actual request and cleans both owned resources", () => scenario("success", async invoke => {
  const result = await invoke();
  assert.equal(result.exitCode, 0);
  assert.equal(result.requests.length, 1);
}));

test("a successful CLI cannot hide either cleanup failure", () => scenario("cleanup-fail", async invoke => {
  await assert.rejects(invoke(), error => {
    assert(error instanceof AggregateError);
    assert.equal(error.errors.length, 2);
    assert.equal(error.cause, undefined);
    return true;
  });
}));

test("primary CLI failure survives both cleanup failures", () => scenario("both-fail", async invoke => {
  await assert.rejects(invoke(), error => {
    assert(error instanceof AggregateError);
    assert.equal(error.errors.length, 3);
    assert.equal(error.errors[0], error.cause);
    assert.match(error.cause.message, /toBe/);
    return true;
  });
}));
