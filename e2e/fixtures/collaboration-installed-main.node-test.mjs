import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { PassThrough } from "node:stream";
import vm from "node:vm";
import { mkdtempSync, readFileSync, statSync, existsSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { InstalledMainClient, createInstalledMachineIdentity, finishInstalledMainJourney, installedTimelineEvent } from "./collaboration-installed-main.ts";

test("native controller shutdown preserves a failed close command after its owned child exits", async () => {
  const client = Object.create(InstalledMainClient.prototype);
  const failure = new Error("owned close failure");
  client.nativeMode = true;
  client.ended = false;
  client.command = async () => { throw failure; };
  const child = new EventEmitter();
  child.stdin = { end: () => { client.ended = true; child.emit("close", 1); } };
  client.child = child;
  await assert.rejects(client.close(), error => error === failure);
});

test("already closed controller does not send another command or touch Docker", async () => {
  const client = Object.create(InstalledMainClient.prototype);
  client.ended = true;
  client.command = () => { throw new Error("must not send"); };
  await client.close();
});


test("container machine identities are unique private read-only mounts with owned cleanup", () => {
  const root = mkdtempSync(join(tmpdir(), "installed-machine-id-contract-"));
  try {
    const first = createInstalledMachineIdentity(root);
    const second = createInstalledMachineIdentity(root);
    const file = (identity) => identity.mount.slice(0, -":/etc/machine-id:ro".length);
    assert(first.mount.endsWith(":/etc/machine-id:ro"));
    assert(second.mount.endsWith(":/etc/machine-id:ro"));
    const one = file(first), two = file(second);
    assert.match(readFileSync(one, "utf8"), /^[a-f0-9]{32}\n$/);
    assert.match(readFileSync(two, "utf8"), /^[a-f0-9]{32}\n$/);
    assert.notEqual(readFileSync(one, "utf8"), readFileSync(two, "utf8"));
    assert.equal(statSync(one).mode & 0o777, 0o444);
    first.cleanup(); first.cleanup();
    assert.equal(existsSync(dirname(one)), false);
    assert.equal(existsSync(two), true);
    second.cleanup();
    assert.equal(existsSync(dirname(two)), false);
  } finally { rmSync(root, { recursive: true, force: true }); }
});


test("container discovery CLI reports its version but refuses and records every task invocation", () => {
  const root = mkdtempSync(join(tmpdir(), "installed-discovery-contract-"));
  const identity = createInstalledMachineIdentity(root);
  try {
    assert(identity.discoveryMount.endsWith(":/acceptance/discovery/claude:ro"));
    const executable = identity.discoveryMount.slice(0, -":/acceptance/discovery/claude:ro".length);
    const marker = join(root, "unexpected-invocation");
    const env = { PATH: process.env.PATH, MULTICA_INSTALLED_DISCOVERY_MARKER: marker };
    const version = spawnSync(executable, ["--version"], { env, encoding: "utf8" });
    assert.equal(version.status, 0);
    assert.equal(version.stdout, "2.1.5 (Claude Code)\n");
    assert.equal(existsSync(marker), false);
    const execution = spawnSync(executable, ["--print", "sensitive-test-input"], { env, encoding: "utf8" });
    assert.equal(execution.status, 97);
    assert.equal(readFileSync(marker, "utf8"), "unexpected agent execution\n");
    assert(!readFileSync(marker, "utf8").includes("sensitive-test-input"));
    identity.cleanup();
    assert.equal(existsSync(executable), false);
  } finally { identity.cleanup(); rmSync(root, { recursive: true, force: true }); }
});


test("journey preserves the primary error and every cleanup failure while attempting all cleanup steps", async () => {
  const primary = new TypeError("original business failure");
  const status = new Error("status capture failed");
  const disconnect = new Error("disconnect failed");
  const close = new Error("native close failed");
  const calls = [];
  const original = [primary, status];
  await assert.rejects(finishInstalledMainJourney(original, [
    async () => { calls.push("disconnect"); throw disconnect; },
    async () => { calls.push("close"); throw close; },
    async () => { calls.push("timeline"); },
  ]), error => {
    assert(error instanceof AggregateError);
    assert.equal(error.cause, primary);
    assert.deepEqual(error.errors, [primary, status, disconnect, close]);
    return true;
  });
  assert.deepEqual(calls, ["disconnect", "close", "timeline"]);
  assert.deepEqual(original, [primary, status]);
});

test("a lone business or cleanup failure remains the exact original object", async () => {
  const primary = new Error("original");
  await assert.rejects(finishInstalledMainJourney([primary], [async () => undefined]), error => error === primary);
  const cleanup = new Error("cleanup");
  await assert.rejects(finishInstalledMainJourney([], [async () => { throw cleanup; }]), error => error === cleanup);
  await finishInstalledMainJourney([], [async () => undefined]);
});

test("native close keeps command, stream and final cleanup failures without skipping owned cleanup", async () => {
  const client = Object.create(InstalledMainClient.prototype);
  const primary = new Error("close command failed");
  const stream = new Error("stdin end failed");
  const cleanup = new Error("owned cleanup failed");
  const calls = [];
  client.ended = false;
  client.command = async () => { throw primary; };
  client.child = { stdin: { end: () => { client.ended = true; throw stream; } } };
  client.cleanupStoppedContainerIdentity = async () => { calls.push("owned cleanup"); throw cleanup; };
  await assert.rejects(client.close(), error => {
    assert(error instanceof AggregateError);
    assert.deepEqual(error.errors, [primary, stream, cleanup]);
    return true;
  });
  assert.deepEqual(calls, ["owned cleanup"]);
});

function protocolClient() {
  const client = Object.create(InstalledMainClient.prototype);
  const child = new EventEmitter();
  child.stdout = new PassThrough();
  child.stderr = new PassThrough();
  child.stdin = { write: (_value, callback) => callback() };
  Object.assign(client, { child, ended: false, pending: new Map(), sequence: 0, timeline: [], timelineStarted: performance.now(),
    timelineTruncated: false, ignoredOutputLines: 0, stderrBytes: 0, nativeMode: true });
  client.attachProtocolHandlers();
  return { client, child };
}

test("command and streamed restart stages survive a timeout without recording secret input or output", async () => {
  const { client, child } = protocolClient();
  const secret = "sensitive-test-password";
  try {
    const pending = client.command("restart", { password: secret }, 15);
    const event = { id: 1, command: "restart", phase: "restart.close", outcome: "started", elapsedMs: 3,
      token: secret, error: secret, response: { password: secret } };
    child.stdout.write(`MULTICA_INSTALLED_EVENT ${JSON.stringify(event)}\n`);
    // Invalid event fields cannot smuggle secrets into any visible timeline field.
    child.stdout.write(`MULTICA_INSTALLED_EVENT ${JSON.stringify({ ...event, phase: secret })}\n`);
    await assert.rejects(pending, /restart exceeded 15ms/);
    const timeline = client.transportEvidence.commandTimeline;
    assert.deepEqual(timeline.map(({ source, phase, outcome }) => ({ source, phase, outcome })), [
      { source: "client", phase: "command", outcome: "started" },
      { source: "controller", phase: "restart.close", outcome: "started" },
      { source: "client", phase: "command", outcome: "timeout" },
    ]);
    assert(!JSON.stringify(client.transportEvidence).includes(secret));
    const safe = installedTimelineEvent(event, "controller");
    assert.deepEqual(Object.keys(safe), ["id", "command", "phase", "outcome", "elapsedMs", "source"]);
    assert.equal(installedTimelineEvent({ ...event, elapsedMs: Infinity }, "controller"), null);
  } finally { child.stdout.end(); child.stderr.end(); }
});

test("protocol completion records only fixed metadata and returns the original response", async () => {
  const { client, child } = protocolClient();
  const secret = "sensitive-test-token";
  try {
    const pending = client.command("read", { authorization: secret });
    child.stdout.write(`MULTICA_INSTALLED_REPLY ${JSON.stringify({ id: 1, value: { token: secret } })}\n`);
    assert.deepEqual(await pending, { token: secret });
    assert.deepEqual(client.transportEvidence.commandTimeline.map(row => row.outcome), ["started", "completed"]);
    assert(!JSON.stringify(client.transportEvidence).includes(secret));
  } finally { child.stdout.end(); child.stderr.end(); }
});


test("controller phase event failure cannot replace the failed operation", async () => {
  // Exercise the actual controller helper without starting its top-level process loop.
  const source = readFileSync(new URL("./collaboration-installed-main.mjs", import.meta.url), "utf8");
  const start = source.indexOf("async function commandPhase(");
  const end = source.indexOf("\nconst secrets =", start);
  assert(start >= 0 && end > start);
  const primary = new Error("operation failed");
  const transport = new Error("event pipe failed");
  const outcomes = [];
  const commandPhase = vm.runInNewContext(`${source.slice(start, end)}; commandPhase`, {
    AggregateError,
    commandEvent: async (_message, _phase, outcome) => {
      outcomes.push(outcome);
      if (outcome === "failed") throw transport;
    },
  });
  await assert.rejects(commandPhase({ id: 1, command: "restart" }, "restart.close", async () => { throw primary; }), error => {
    assert(error instanceof AggregateError);
    assert.equal(error.cause, primary);
    assert.deepEqual(error.errors, [primary, transport]);
    return true;
  });
  assert.deepEqual(outcomes, ["started", "failed"]);
});

test("controller aggregate failure replies retain member errors while scrubbing credentials", () => {
  const source = readFileSync(new URL("./collaboration-installed-main.mjs", import.meta.url), "utf8");
  const start = source.indexOf("function scrub(");
  const end = source.indexOf("\nfunction reply(", start);
  assert(start >= 0 && end > start);
  const secret = "private-synthetic-password";
  const failureText = vm.runInNewContext(`${source.slice(start, end)}; failureText`, { Error, AggregateError, secrets: new Set([secret]) });
  const text = failureText(new AggregateError([new TypeError(`primary ${secret}`), new Error(`cleanup ${secret}`)], "both failed"));
  assert(text.includes("TypeError: primary <private-fixture-value>"));
  assert(text.includes("Error: cleanup <private-fixture-value>"));
  assert(!text.includes(secret));
});
