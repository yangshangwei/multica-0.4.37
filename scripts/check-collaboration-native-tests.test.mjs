import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const checker = fileURLToPath(new URL("./check-collaboration-native-tests.mjs", import.meta.url));
const required = [
  "TestHostRunnerReusesOrdinaryProviderAndOriginalPermitAndProof",
  "TestHostRunnerEOFDoesNotEndExecutionBeforeRealWait",
  "TestHostRunnerFreshRetryGetsNewTokenAndStopForbidsFallback",
  "TestHostRunnerOpenCodeEmptyExitDoesNotInventBusinessOutcome",
  "TestHostRunnerUsesSelectedCLIUnderCanonicalCommandName",
  "TestHostSessionLockKeepsUnknownOwnershipAfterSupervisorRestart",
  "TestHostLifecycleCodexInternalRetryUsesNewGateAfterWait",
  "TestHostLifecycleCodexTimeoutDoesNotLaunchUnpermittedDiagnostic",
  "TestHostLifecycleRepeatedStartKeepsActiveLaunch",
  "TestHostGitPreparationRegistersEachHelperAndOnlyDeliversSelectedCommit",
  "TestCollaborationHostPollerUsesNativeModeAndSharedLedger",
];
const windows = [
  "TestHostWindowsLedgerProtectsFilesAndLocksAcrossProcesses",
  "TestHostWindowsLedgerRejectsSharedACL",
  "TestHostWindowsLedgerSupportsLongPaths",
  "TestHostLifecycleWindowsWrapperRequiresJobBeforeResume",
  "TestHostLifecycleWindowsWrapperWaitRequiresManagedChildEnd",
];
const passing = (names = required) => names.map((name) => ({ Action: "pass", Test: name }));

function check(t, events, runnerArgs = ["macOS"]) {
  const directory = mkdtempSync(join(tmpdir(), "multica-native-gate-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const path = join(directory, "tests.jsonl");
  writeFileSync(path, events.map((event) => JSON.stringify(event)).join("\n") + "\n");
  const result = spawnSync(process.execPath, [checker, path, ...runnerArgs], { encoding: "utf8", timeout: 10_000 });
  assert.ifError(result.error);
  assert.notEqual(result.status, null);
  return result;
}

test("accepts required parent and child passes", (t) => {
  const child = `${required[9]}/fixed_commit`;
  assert.equal(check(t, [...passing(), { Action: "pass", Test: child }]).status, 0);
});

test("rejects a skipped required descendant even when its parent passes", (t) => {
  for (const suffix of ["fixed_commit", "fixed_commit/native"]) {
    const child = `${required[9]}/${suffix}`;
    const result = check(t, [{ Action: "skip", Test: child }, ...passing()]);
    assert.equal(result.status, 1);
    assert.ok(result.stderr.includes(child));
  }
});

test("keeps unrelated skipped tests outside required subtrees", (t) => {
  assert.equal(check(t, [...passing(), { Action: "skip", Test: `${required[9]}Optional/platform` }]).status, 0);
});

test("rejects missing and skipped required parents", (t) => {
  assert.equal(check(t, passing(required.slice(1))).status, 1);
  assert.equal(check(t, [...passing(required.slice(1)), { Action: "skip", Test: required[0] }]).status, 1);
});

test("requires every Windows-specific case", (t) => {
  assert.equal(check(t, passing(), ["Windows"]).status, 1);
  assert.equal(check(t, passing([...required, ...windows]), ["Windows"]).status, 0);
  const child = `${windows[0]}/peer_lock`;
  assert.equal(check(t, [{ Action: "skip", Test: child }, ...passing([...required, ...windows])], ["Windows"]).status, 1);
});

test("rejects absent or unsupported runner metadata", (t) => {
  for (const runnerArgs of [[], [""], ["windows"], ["other"], ["Linux", "extra"]]) {
    assert.notEqual(check(t, passing(), runnerArgs).status, 0);
  }
});

test("retains test and package failure checks", (t) => {
  for (const failure of [{ Action: "fail", Test: "TestOptional" }, { Action: "fail", Package: "fixture/native" }]) {
    assert.equal(check(t, [...passing(), failure]).status, 1);
  }
});
