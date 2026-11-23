import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import test from "node:test";

const source = readFileSync(new URL("./collaboration-host-runtime.ts", import.meta.url), "utf8");

function closeFixture(platform, forceFallback = false) {
  const start = source.indexOf("  const close = async () => {");
  const end = source.indexOf("\n  try {\n    await expect.poll", start);
  assert(start > 0 && end > start, "Read the actual native Host close function");
  const closeSource = source.slice(start, end).replaceAll("new Promise<void>", "new Promise");
  const signals = [], actions = [], messages = [], ledger = [{ phase: "proof_acknowledged", proof_state: "host_exit_observed" }];
  let exited;
  const exit = new Promise(resolve => { exited = resolve; });
  const child = { exitCode: null,
    kill(signal) {
      signals.push(signal);
      if (platform === "win32" && ["SIGCONT", "SIGSTOP"].includes(signal)) throw new TypeError(`Unknown signal: ${signal}`);
      if (signal === "SIGKILL") { child.exitCode = 9; exited(9); }
      return true;
    },
    stdin: { end() {
      actions.push("stdin_eof");
      if (!forceFallback) {
        messages.push({ claims: 3, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
        child.exitCode = 0; exited(0);
      }
    } },
  };
  let fallback;
  const timer = { callback: null, cleared: false };
  const factory = vm.compileFunction(`let closed = false; ${closeSource}; return close;`,
    ["process", "child", "exit", "sockets", "server", "directory", "runtime", "messages", "errors", "stderr", "hostLedgerEvidence", "rmSync", "join", "setTimeout", "clearTimeout"]);
  const close = factory({ platform }, child, exit, new Set([{ destroy() { actions.push("socket_closed"); } }]),
    { close(done) { actions.push("callback_closed"); done(); } }, "/fixture", { deploymentId: "deployment", installationId: "installation" }, messages, [], "",
    () => ledger, () => actions.push("private_directory_removed"), (...parts) => parts.join("/"),
    (callback, milliseconds) => { assert.equal(milliseconds, 20_000); fallback = callback; return timer; },
    value => { assert.equal(value, timer); timer.cleared = true; });
  return { close, signals, actions, messages, ledger, timer, fireFallback: () => fallback() };
}

test("Windows Host close delivers EOF and retains the graceful final summary without POSIX signals", async () => {
  const f = closeFixture("win32");
  const result = await f.close();
  assert.deepEqual(f.signals, []);
  assert.deepEqual(f.actions, ["stdin_eof", "socket_closed", "callback_closed", "private_directory_removed"]);
  assert.equal(result.exit_code, 0);
  assert.deepEqual(result.messages, [{ claims: 3, invalid_capacity: false, available_slots: 1, active_tasks: 0 }]);
  assert.equal(result.ledger, f.ledger);
  assert.deepEqual(result.errors, []);
  assert.equal(f.timer.cleared, true);
  await assert.rejects(f.close(), /cleanup was invoked twice/);
});

test("POSIX Host close still resumes a suspended daemon before EOF", async () => {
  const f = closeFixture("darwin");
  const result = await f.close();
  assert.deepEqual(f.signals, ["SIGCONT"]);
  assert.equal(result.exit_code, 0);
  assert.equal(result.messages.length, 1);
  assert.equal(f.timer.cleared, true);
});

test("Windows fallback remains bounded and an abrupt exit cannot become a graceful pass", async () => {
  const f = closeFixture("win32", true);
  const pending = f.close();
  // A baseline portability failure must remain observed even if the next
  // assertion catches that EOF was never reached.
  void pending.catch(() => undefined);
  assert.deepEqual(f.actions, ["stdin_eof"]);
  f.fireFallback();
  const result = await pending;
  assert.deepEqual(f.signals, ["SIGKILL"]);
  assert.equal(result.exit_code, 9);
  assert.deepEqual(result.messages, []);
  assert.equal(f.timer.cleared, true);
});

test("Windows suspend and resume helpers refuse before sending unsupported signals", () => {
  const signals = [];
  const child = { exitCode: null, kill(signal) { signals.push(signal); return true; } };
  for (const name of ["freeze", "thaw"]) {
    const body = source.match(new RegExp(`      ${name}\\(\\) \\{([\\s\\S]*?)\\n      \\},`))?.[1];
    assert(body, `Read the actual ${name} helper`);
    const invoke = vm.compileFunction(body, ["process", "closed", "child"]);
    assert.throws(() => invoke({ platform: "win32" }, false, child), /unsupported on Windows/);
  }
  assert.deepEqual(signals, []);
});
