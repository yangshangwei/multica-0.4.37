import assert from "node:assert/strict";
import vm from "node:vm";

// Exercise Node's embedded JavaScript validation against Windows signal names,
// without creating or signaling a process. Native Windows execution is unrun.
const source = process.binding("natives")["internal/util"];
const start = source.indexOf("function convertToValidSignal(signal) {");
const end = source.indexOf("\nfunction getConstructorOf", start);
assert(start > 0 && end > start);
const windowsSignals = { SIGHUP: 1, SIGINT: 2, SIGQUIT: 3, SIGILL: 4, SIGABRT: 22,
  SIGFPE: 8, SIGKILL: 9, SIGSEGV: 11, SIGTERM: 15, SIGBREAK: 21, SIGWINCH: 28 };
class UnknownSignal extends Error { constructor(signal) { super(`Unknown signal: ${signal}`); this.code = "ERR_UNKNOWN_SIGNAL"; } }
const convert = vm.compileFunction(`${source.slice(start, end)}; return convertToValidSignal;`,
  ["signals", "getSignalsToNamesMapping", "StringPrototypeToUpperCase", "ERR_UNKNOWN_SIGNAL"])(
  windowsSignals, () => Object.fromEntries(Object.entries(windowsSignals).map(([key, value]) => [value, key])),
  (value) => value.toUpperCase(), UnknownSignal);
assert.equal(convert("SIGTERM"), 15);
assert.throws(() => convert("SIGCONT"), { code: "ERR_UNKNOWN_SIGNAL" });
console.log(JSON.stringify({ node: process.version, embedded_function: "internal/util.convertToValidSignal",
  supported_control: "SIGTERM", rejected: "SIGCONT", native_windows_executed: false }));
