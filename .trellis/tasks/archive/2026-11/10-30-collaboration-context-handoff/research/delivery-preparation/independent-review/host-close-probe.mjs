import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import vm from "node:vm";

const path = new URL("../../../../../../e2e/fixtures/collaboration-host-runtime.ts", import.meta.url);
// Preserve the original reviewed product even after the owner commits a fix.
const reviewedRoot = new URL("../../../../../../", import.meta.url);
const source = execFileSync("git", ["show", "9de37277d:e2e/fixtures/collaboration-host-runtime.ts"], { cwd: reviewedRoot, encoding: "utf8" });
assert(readFileSync(path, "utf8").includes("const close = async () => {"));
const start = source.indexOf("  const close = async () => {");
const end = source.indexOf("\n  try {\n    await expect.poll", start);
assert(start > 0 && end > start, "Read the actual Host fixture close implementation");
// The extracted block contains one TypeScript-only generic, erased here.
const closeSource = source.slice(start, end).replace("new Promise<void>", "new Promise");
let stdinEnded = false;
const signals = [];
// Windows does not provide SIGCONT as a POSIX resume signal. The probe uses
// an unsupported-signal child, without launching any process, API or database.
const child = { exitCode: null, kill(signal) {
  signals.push(signal);
  if (signal === "SIGCONT") {
    const error = new TypeError("Unknown signal: SIGCONT");
    error.code = "ERR_UNKNOWN_SIGNAL";
    throw error;
  }
  return true;
}, stdin: { end() { stdinEnded = true; } } };
const factory = vm.compileFunction(`let closed = false; ${closeSource}; return close;`,
  ["child", "exit", "sockets", "server", "directory", "runtime", "messages", "errors", "stderr", "hostLedgerEvidence", "rmSync", "join", "setTimeout", "clearTimeout"]);
const close = factory(child, Promise.resolve(0), new Set(), { close(done) { done(); } }, "/fixture", { deploymentId: "fixture", installationId: "fixture" },
  [], [], "", () => [], () => {}, (...parts) => parts.join("/"), setTimeout, clearTimeout);
await assert.rejects(close(), { code: "ERR_UNKNOWN_SIGNAL" });
assert.deepEqual(signals, ["SIGCONT"]);
assert.equal(stdinEnded, false);
console.log(JSON.stringify({ actual_source: "9de37277d:e2e/fixtures/collaboration-host-runtime.ts:302", signals,
  stdin_end_reached: stdinEnded, result: "confirmed unsupported-signal failure before graceful EOF; modeled child only, native Windows unrun" }));
