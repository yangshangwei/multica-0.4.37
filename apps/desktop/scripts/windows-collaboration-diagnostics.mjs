import { projectDaemonStartupDiagnostic } from "../../../e2e/fixtures/collaboration-daemon-startup-diagnostics.mjs";

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

const daemonStates = new Set(["stopped", "starting", "running", "stopping", "installing_cli", "cli_not_found", "recovery_paused", "auth_expired"]);
const boolean = value => typeof value === "boolean" ? value : null;
// Exact intersection with the controller's already-public diagnostic vocabulary.
const daemonCodes = new Set(["challenge_expired", "challenge_time_window", "challenge_scope_mismatch", "challenge_invalid",
  "cli_unavailable", "cli_download_failed", "cli_checksum_failed", "cli_extract_failed", "cli_install_failed", "cli_validation_failed",
  "request_failed", "operation_failed", "EACCES", "EPERM", "ENOENT", "ENOEXEC", "EROFS", "EADDRINUSE", "ECONNREFUSED", "ETIMEDOUT"]);
const mainDaemonCodes = new Set([...daemonCodes, "managed_setup_failed", "pat_mint_failed", "cli_auto_install_failed", "credential_restart_failed"]);
const daemonMarkers = new Set(["no agent runtimes could be registered", "not logged in", "permission denied", "read-only file system",
  "exec format error", "address already in use", "no such file or directory", "daemon-managed task"]);
const daemonStages = new Set(["cli_setup", "registration", "binding", "credentials", "restart"]);
const connectionDiagnostic = value => value && typeof value === "object" && !Array.isArray(value) ? {
  code: daemonCodes.has(value.code) ? value.code : "unknown",
  stage: daemonStages.has(value.stage) ? value.stage : "unknown",
  http_status: Number.isInteger(value.httpStatus) && value.httpStatus >= 100 && value.httpStatus <= 599 ? value.httpStatus : null,
} : null;
const allowedValues = (value, allowed) => Array.isArray(value) ? [...new Set(value.filter(item => allowed.has(item)))].slice(0, allowed.size) : [];


export function daemonFailureProjection(failures) {
  const marker = "Installed daemon diagnostic: ";
  for (const failure of failures.slice(0, 16)) {
    for (const text of [failure?.message, failure?.stack]) {
      if (typeof text !== "string") continue;
      // Each eligible input is already a bounded Playwright error, not stdout/stderr.
      for (const line of text.slice(0, 128 * 1024).split(/\r?\n/).slice(0, 128)) {
        if (!line.startsWith(marker)) continue;
        if (line.length > 64 * 1024) return { status: "invalid" };
        let value;
        try { value = JSON.parse(line.slice(marker.length)); } catch { return { status: "invalid" }; }
        if (!value || typeof value !== "object" || Array.isArray(value)) return { status: "invalid" };
        const actual = value.actual;
        return {
          status: "parsed",
          expected_state: daemonStates.has(value.expectedState) ? value.expectedState : "unknown",
          actual_state: daemonStates.has(actual?.state) ? actual.state : "unknown",
          profile_matches: boolean(actual?.profileMatches), server_matches: boolean(actual?.serverMatches),
          authenticated: boolean(value.authenticated), managed_deployment_advertised: boolean(value.managedDeploymentAdvertised),
          installation_present: boolean(actual?.installation?.present),
          installation_server_matches: boolean(actual?.installation?.serverMatches), installation_user_matches: boolean(actual?.installation?.userMatches),
          capture_failed: boolean(value.capture_failed), capture_timed_out: boolean(value.capture_timed_out),
          main_error_codes: allowedValues(value.mainErrorCodes, mainDaemonCodes),
          connection_diagnostics: { cli: connectionDiagnostic(actual?.diagnostics?.cli), connection: connectionDiagnostic(actual?.diagnostics?.connection) },
          files: Object.fromEntries([["config", "config.json"], ["owner", ".desktop-user-id"], ["pid", "daemon.pid"], ["log", "daemon.log"], ["err_log", "daemon.err.log"]]
            .map(([key, name]) => {
              const file = value.profileFiles?.[name];
              return [key, { exists: boolean(file?.exists), outside_home: boolean(file?.outside_expected_home), read_failed: boolean(file?.diagnostic_read_failed),
                ...(key === "config" ? { server_matches: boolean(file?.serverMatches), deployment_matches: boolean(file?.deploymentMatches),
                  deployment_present: boolean(file?.deploymentPresent), credential_present: boolean(file?.credentialPresent) } : {}),
                ...(key === "owner" ? { user_matches: boolean(file?.userMatches) } : {}),
                ...(["log", "err_log"].includes(key) ? { error_codes: allowedValues(file?.errorCodes, daemonCodes), markers: allowedValues(file?.markers, daemonMarkers), startup: projectDaemonStartupDiagnostic(file?.startup) } : {}) }];
            })),
        };
      }
    }
  }
  return { status: "unavailable" };
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
    daemon: daemonFailureProjection(failures),
    milestones: Object.fromEntries(["boot", "runtime", "login", "stop", "restart_while_paused", "resume", "final", "packaged_status", "transport"]
      .map(key => [key, Boolean(journey && Object.hasOwn(journey, key))])),
  };
}
