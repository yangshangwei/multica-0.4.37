import { createHash } from "node:crypto";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { PassThrough } from "node:stream";
import vm from "node:vm";
import { mkdtempSync, readFileSync, statSync, existsSync, rmSync, mkdirSync, writeFileSync, realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import path, { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { nativeAcceptanceHome, nativeInstalledLayout } from "./collaboration-installed-layout.mjs";
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
    // Windows reports its read-only attribute through these mode bits, not POSIX ACLs.
    assert.equal(statSync(one).mode & 0o777, 0o444);
    first.cleanup(); first.cleanup();
    assert.equal(existsSync(dirname(one)), false);
    assert.equal(existsSync(two), true);
    second.cleanup();
    assert.equal(existsSync(dirname(two)), false);
  } finally { rmSync(root, { recursive: true, force: true }); }
});


test("container discovery CLI reports its version but refuses and records every task invocation", () => {
  const root = mkdtempSync(join(tmpdir(), "installed-discovery contract-"));
  const identity = createInstalledMachineIdentity(root);
  try {
    assert(identity.discoveryMount.endsWith(":/acceptance/discovery/claude:ro"));
    const executable = identity.discoveryMount.slice(0, -":/acceptance/discovery/claude:ro".length);
    const marker = join(root, "unexpected-invocation");
    const env = { PATH: process.env.PATH, MULTICA_INSTALLED_DISCOVERY_MARKER: marker };
    let command = executable;
    const prefix = [];
    if (process.platform === "win32") {
      // Use Git's POSIX runtime explicitly; PATH bash may resolve to WSL.
      assert.ok(process.env.ProgramFiles, "Windows fixture requires ProgramFiles for Git for Windows");
      const gitBin = join(process.env.ProgramFiles, "Git", "usr", "bin");
      command = join(gitBin, "sh.exe");
      const cygpath = join(gitBin, "cygpath.exe");
      for (const tool of [command, cygpath]) assert.ok(existsSync(tool), `Required Git for Windows fixture tool missing: ${tool}`);
      env.SystemRoot = process.env.SystemRoot;
      const posixPath = path => {
        const converted = spawnSync(cygpath, ["-u", path], { env, encoding: "utf8" });
        assert.ifError(converted.error);
        assert.equal(converted.status, 0, converted.stderr);
        const value = converted.stdout.trimEnd();
        assert.ok(value.startsWith("/") && !/[\r\n]/.test(value), "cygpath must return one absolute POSIX path");
        return value;
      };
      prefix.push(posixPath(executable));
      env.MULTICA_INSTALLED_DISCOVERY_MARKER = posixPath(marker);
    } else {
      assert.equal(statSync(executable).mode & 0o777, 0o555);
    }
    const invoke = args => spawnSync(command, [...prefix, ...args], { env, encoding: "utf8" });
    const version = invoke(["--version"]);
    assert.ifError(version.error);
    assert.equal(version.status, 0, version.stderr);
    assert.equal(version.stdout, "2.1.5 (Claude Code)\n");
    assert.equal(existsSync(marker), false);
    const execution = invoke(["--print", "sensitive-test-input"]);
    assert.ifError(execution.error);
    assert.equal(execution.status, 97, execution.stderr);
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


function rendererAssertion(url, nativeLayout, packagePath) {
  const source = readFileSync(new URL("./collaboration-installed-main.mjs", import.meta.url), "utf8");
  const start = source.indexOf("async function expectInstalledRenderer(");
  const end = source.indexOf("\nasync function launch(", start);
  assert(start >= 0 && end > start, "Read the actual installed renderer assertion");
  const expect = { poll: read => ({ toBe: async expected => assert.equal(await read(), expected) }) };
  return vm.compileFunction(`${source.slice(start, end)}; return expectInstalledRenderer();`,
    ["URL", "expect", "page", "nativeLayout", "packagePath"])(URL, expect, { url: () => url }, nativeLayout, packagePath);
}

test("native launch pins the exact encoded renderer URL including Windows drive and literal percent paths", async () => {
  for (const [platform, executable] of [
    ["win32", "D:\\runner\\installed\\Multica.exe"],
    ["win32", "D:\\Runner %20 #\\installed\\Multica.exe"],
    ["darwin", "/task %20 #/Multica Candidate.app/Contents/MacOS/Multica"],
  ]) {
    const layout = nativeInstalledLayout(executable, platform);
    await rendererAssertion(layout.rendererUrl, layout, layout.asar);
    for (const wrong of [layout.rendererUrl.replace("index.html", "other.html"), layout.rendererUrl.replace("app.asar/", "other.asar/"),
      layout.rendererUrl.replace("file:///", "file://foreign/"), layout.rendererUrl + "?foreign=1", layout.rendererUrl + "#foreign"]) {
      await assert.rejects(rendererAssertion(wrong, layout, layout.asar), assert.AssertionError);
    }
    if (layout.rendererUrl.includes("%2520")) {
      await assert.rejects(rendererAssertion(layout.rendererUrl.replace("%2520", "%20"), layout, layout.asar), assert.AssertionError);
    }
  }
});

test("container launch retains its exact Linux packaged pathname check", async () => {
  for (const packagePath of ["/opt/multica/resources/app.asar", "/opt/Multica/resources/app.asar"]) {
    await rendererAssertion(`file://${packagePath}/out/renderer/index.html`, null, packagePath);
    await assert.rejects(rendererAssertion(`file://${packagePath}/out/renderer/other.html`, null, packagePath), assert.AssertionError);
  }
});


test("native controller children and their descendants share the validated home without changing parent state", () => {
  const source = readFileSync(new URL("./collaboration-installed-main.ts", import.meta.url), "utf8");
  const start = source.indexOf("    if (options.executablePath) {");
  const end = source.indexOf("      this.attachProtocolHandlers();", start);
  assert(start > 0 && end > start, "Read the actual native controller spawn boundary");
  const spawnNative = vm.compileFunction(`${source.slice(start, end).replace("import.meta.dirname", "fixtureDirectory")} }`,
    ["options", "process", "nativeInstalledLayout", "nativeAcceptanceHome", "realpathSync", "path", "spawn", "fixtureDirectory"]);
  const root = realpathSync(mkdtempSync(join(tmpdir(), "installed native home-")));
  const parent = { HOME: process.env.HOME, USERPROFILE: process.env.USERPROFILE };
  const platform = process.platform === "win32" ? "win32" : "darwin";
  const executable = platform === "win32" ? join(root, "Multica.exe") : join(root, "Multica.app", "Contents", "MacOS", "Multica");
  mkdirSync(dirname(executable), { recursive: true }); writeFileSync(executable, "fixture executable path");
  const home = join(root, "owned home %20");
  const profileModule = new URL("../../apps/desktop/src/main/daemon-profile.ts", import.meta.url).href;
  const probe = `import { homedir } from "node:os"; import { profileDir } from ${JSON.stringify(profileModule)}; process.stdout.write(JSON.stringify({ home: homedir(), profile: profileDir("desktop-loopback") }));`;
  const child = `import { homedir } from "node:os"; import { spawnSync } from "node:child_process"; import { profileDir } from ${JSON.stringify(profileModule)};
    const descendant = spawnSync(process.execPath, ["--experimental-strip-types", "--input-type=module", "-e", ${JSON.stringify(probe)}], { env: { ...process.env }, encoding: "utf8" });
    if (descendant.status !== 0) throw new Error(descendant.stderr);
    process.stdout.write(JSON.stringify({ home: homedir(), profile: profileDir("desktop-loopback"), descendant: JSON.parse(descendant.stdout) }));`;
  let launches = 0;
  const spawnProbe = (_command, _args, options) => {
    launches++;
    const result = spawnSync(process.execPath, ["--experimental-strip-types", "--input-type=module", "-e", child], { ...options, detached: false, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    const observed = JSON.parse(result.stdout);
    assert.equal(observed.home, home);
    assert.equal(observed.profile, join(home, ".multica", "profiles", "desktop-loopback"));
    assert.deepEqual(observed.descendant, { home: observed.home, profile: observed.profile });
    assert.equal(options.env.HOME, home); assert.equal(options.env.USERPROFILE, home);
  };
  try {
    const run = acceptanceHome => spawnNative.call({}, { executablePath: executable, acceptanceHome }, process,
      value => nativeInstalledLayout(value, platform), value => nativeAcceptanceHome(value, platform),
      realpathSync, path, spawnProbe, dirname(fileURLToPath(import.meta.url)));
    run(home);
    for (const invalid of [undefined, "relative-home", "bad\nhome"]) assert.throws(() => run(invalid), /absolute/);
    assert.equal(launches, 1, "Invalid homes must fail before spawning");
    assert.deepEqual({ HOME: process.env.HOME, USERPROFILE: process.env.USERPROFILE }, parent);
  } finally { rmSync(root, { recursive: true, force: true }); }
});


test("native discovery guard refuses marker or changed binary while preserving opt-out Mac callers", () => {
  const source = readFileSync(new URL("./collaboration-installed-main.mjs", import.meta.url), "utf8");
  const start = source.indexOf("function verifyDiscoveryOnly(");
  const end = source.indexOf("\nfunction scrub(", start);
  assert(start > 0 && end > start);
  const root = realpathSync(mkdtempSync(join(tmpdir(), "discovery-guard-")));
  const executable = join(root, "discovery.exe"); const marker = `${executable}.unexpected`;
  const sha = file => createHash("sha256").update(readFileSync(file)).digest("hex");
  writeFileSync(executable, "source-bound binary fixture");
  const env = { COLLABORATION_INSTALLED_DISCOVERY_SHA256: sha(executable) };
  const invoke = required => vm.compileFunction(`${source.slice(start, end)}; verifyDiscoveryOnly();`,
    ["discoveryRequired", "discoveryCLI", "discoveryMarker", "nativeMode", "path", "assert", "existsSync", "realpathSync", "sha", "process"])
    (required, executable, marker, true, path, assert, existsSync, realpathSync, sha, { env });
  try {
    invoke(true);
    writeFileSync(marker, "unexpected agent execution\n"); assert.throws(() => invoke(true), /never execute/);
    invoke(false);
    rmSync(marker); writeFileSync(executable, "changed binary"); assert.throws(() => invoke(true), assert.AssertionError);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
