import { readFileSync } from "node:fs";

const args = process.argv.slice(2);
if (args.length !== 2 || !args[0] || !["Linux", "macOS", "Windows"].includes(args[1])) {
  console.error("Usage: check-collaboration-native-tests.mjs <tests.jsonl> <Linux|macOS|Windows>");
  process.exit(2);
}
const [path, runnerOS] = args;
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
if (runnerOS === "Windows") {
  required.push(
    "TestHostWindowsLedgerProtectsFilesAndLocksAcrossProcesses",
    "TestHostWindowsLedgerRejectsSharedACL",
    "TestHostWindowsLedgerSupportsLongPaths",
    "TestHostLifecycleWindowsWrapperRequiresJobBeforeResume",
    "TestHostLifecycleWindowsWrapperWaitRequiresManagedChildEnd",
  );
}
const results = new Map();
const skippedRequired = new Set();
let failed = false;
for (const line of readFileSync(path, "utf8").split("\n")) {
  if (!line.trim()) continue;
  const event = JSON.parse(line);
  if (event.Action === "fail") failed = true;
  // Go reports a parent as passed even when one or every nested case skips.
  // Required coverage must include those native preconditions as well.
  if (event.Action === "skip" && typeof event.Test === "string" &&
      required.some((name) => event.Test === name || event.Test.startsWith(`${name}/`))) {
    skippedRequired.add(event.Test);
  }
  if (event.Test && ["pass", "fail", "skip"].includes(event.Action)) {
    results.set(event.Test, event.Action);
  }
}
const missing = required.filter((name) => results.get(name) !== "pass");
if (failed || missing.length || skippedRequired.size) {
  console.error("Native collaboration verification failed or skipped:", [...new Set([...missing, ...skippedRequired])].join(", "));
  process.exitCode = 1;
} else {
  console.log(`${runnerOS}: ${required.length} required native collaboration cases passed.`);
}
