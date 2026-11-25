const sourceFiles = [
  "e2e/collaboration-host-installed.spec.ts",
  "e2e/fixtures/collaboration-installed-main.ts",
  "e2e/fixtures/collaboration-installed-main.mjs",
  "e2e/fixtures/collaboration-installed-layout.mjs",
  "e2e/fixtures/collaboration-host-runtime.ts",
  "e2e/fixtures/collaboration-recipient-picker.mjs",
  "e2e/fixtures.ts",
  "e2e/fixtures/collaboration-disclosures.ts",
  "packages/core/collaboration/schema.ts",
  "apps/desktop/scripts/windows-collaboration.mjs",
];
const stages = new Set(["phase_directory", "package_files", "source_commit", "artifact_hashes", "service_probe", "playwright", "result_contract", "journey_evidence", "screenshots"]);
const errorNames = new Set(["Error", "AssertionError", "TypeError", "SyntaxError", "ReferenceError", "TimeoutError", "AggregateError"]);
const errorCodes = new Set(["ENOENT", "EACCES", "EPERM", "ETIMEDOUT", "ERR_ASSERTION", "ERR_MODULE_NOT_FOUND", "MODULE_NOT_FOUND", "ERR_CHILD_PROCESS_STDIO_MAXBUFFER"]);
const statuses = new Set(["passed", "failed", "timedOut", "skipped", "interrupted"]);
const list = value => Array.isArray(value) ? value.slice(0, 16) : [];
const integer = value => Number.isSafeInteger(value) && value >= 0 && value <= 1_000_000 ? value : null;

export function parseCollaborationReport(stdout) {
  try {
    const report = JSON.parse(stdout);
    return report && typeof report === "object" && !Array.isArray(report) ? report : null;
  } catch { return null; }
}

export function collaborationResults(report) {
  const results = [];
  function visit(suites, depth = 0) {
    if (!Array.isArray(suites) || depth > 8) return;
    for (const suite of list(suites)) {
      for (const spec of list(suite?.specs)) {
        for (const test of list(spec?.tests)) results.push(...list(test?.results));
      }
      visit(suite?.suites, depth + 1);
    }
  }
  visit(report?.suites);
  return results.filter(result => result && typeof result === "object").slice(0, 16);
}

/** Public failure data is a projection, never a scrubbed copy of private text. */
export function collaborationDiagnostic({ stage, error, stdout = "", stderr = "", repo, journey }) {
  const report = parseCollaborationReport(stdout);
  const results = collaborationResults(report);
  const root = repo.replaceAll("\\", "/").replace(/\/$/, "");
  const locations = [];
  const location = value => {
    if (typeof value?.file !== "string") return;
    const file = value.file.replaceAll("\\", "/").replace(/^file:\/\/\//, "");
    const relative = sourceFiles.find(candidate => file === candidate || file.toLowerCase() === `${root}/${candidate}`.toLowerCase());
    if (!relative || !integer(value.line) || !integer(value.column)) return;
    const projected = { file: relative, line: value.line, column: value.column };
    if (locations.length < 8 && !locations.some(item => JSON.stringify(item) === JSON.stringify(projected))) locations.push(projected);
  };
  const failures = [error, ...list(report?.errors).slice(0, 8), ...results.map(result => result.error)].filter(Boolean);
  const kinds = new Set();
  const codes = new Set();
  for (const failure of failures) {
    if (errorNames.has(failure.name)) kinds.add(failure.name);
    else {
      const name = /^([A-Za-z]+)(?:\s*\[[^\]]*\])?:/.exec(failure.message ?? "")?.[1];
      kinds.add(errorNames.has(name) ? name : "unknown");
    }
    if (errorCodes.has(failure.code)) codes.add(failure.code);
    location(failure.location);
  }
  for (const result of results) location(result.errorLocation);
  // Parse only stack frame lines. Never emit the message, function name, root,
  // arbitrary filename, assertion expected/actual, or source snippet.
  for (const text of [...failures.map(item => item.stack), stderr]) {
    if (typeof text !== "string") continue;
    for (const line of text.split(/\r?\n/).slice(0, 128)) {
      const frame = /^\s+at (?:[^()]*\()?(.+):(\d+):(\d+)\)?$/.exec(line);
      if (frame) location({ file: frame[1], line: Number(frame[2]), column: Number(frame[3]) });
    }
  }
  return {
    stage: stages.has(stage) ? stage : "unknown",
    process_exit_code: integer(error?.code),
    process_error_code: errorCodes.has(error?.code) ? error.code : null,
    signal: ["SIGTERM", "SIGKILL", "SIGABRT"].includes(error?.signal) ? error.signal : null,
    json_report: report ? "parsed" : "unavailable",
    stats: Object.fromEntries(["expected", "unexpected", "skipped", "flaky"].map(key => [key, integer(report?.stats?.[key])])),
    result_statuses: results.map(result => statuses.has(result.status) ? result.status : "unknown"),
    error_kinds: [...kinds], error_codes: [...codes], locations,
    milestones: Object.fromEntries(["boot", "runtime", "login", "stop", "restart_while_paused", "resume", "final", "packaged_status", "transport"]
      .map(key => [key, Boolean(journey && Object.hasOwn(journey, key))])),
  };
}
