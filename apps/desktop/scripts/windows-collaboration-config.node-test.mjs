import { constants as cryptoConstants, createDecipheriv, generateKeyPairSync, privateDecrypt } from "node:crypto";
import { mkdtemp, mkdir, writeFile, rm, realpath, symlink, readFile, lstat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { encryptedCollaborationDiagnostic } from "./windows-collaboration-encrypted-diagnostic.mjs";
import { readFileSync } from "node:fs";
import { daemonStartupDiagnostic, projectDaemonStartupDiagnostic } from "../../../e2e/fixtures/collaboration-daemon-startup-diagnostics.mjs";
import assert from "node:assert/strict";
import test from "node:test";
import http from "node:http";
import { once } from "node:events";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { windowsCollaborationPlan, windowsCollaborationTestPattern, windowsDiscoveryEnvironment } from "./windows-collaboration-config.mjs";

import { collaborationDiagnostic, collaborationResults, daemonFailureProjection } from "./windows-collaboration-diagnostics.mjs";

import { collaborationProcessEvidence, probeCollaborationApi } from "./windows-collaboration-services.mjs";

function fixture() {
  const env = { GITHUB_ACTIONS: "true", RUNNER_ENVIRONMENT: "github-hosted", GITHUB_SHA: "a".repeat(40),
    GITHUB_WORKSPACE: "D:\\a\\multica", RUNNER_TEMP: "D:\\runner temp" };
  const options = { phase: "upgrade_candidate", stateDirectory: "D:\\runner temp\\collaboration", installedDirectory: "D:\\runner temp\\lifecycle\\installed",
    report: "D:\\runner temp\\collaboration-upgrade.json" };
  const setup = { sourceCommit: env.GITHUB_SHA, repo: env.GITHUB_WORKSPACE, api: "http://127.0.0.1:18081",
    environment: { DATABASE_URL: "postgres://postgres@127.0.0.1:15432/multica_acceptance?sslmode=disable", MULTICA_KNOWLEDGE_CONTEXT_ENABLED: "false", MULTICA_AUTH_MODE: "password" },
    collaboration: { webUrl: "http://127.0.0.1:13001", daemon: "D:\\runner temp\\collaboration\\bin\\daemon.exe", provider: "D:\\runner temp\\collaboration\\bin\\provider.exe", cli: "D:\\runner temp\\collaboration\\bin\\cli.exe", discovery: "D:\\runner temp\\collaboration\\bin\\discovery-only.exe" } };
  return { env, options, setup, run: () => windowsCollaborationPlan(options, setup, env, "win32", "x64") };
}

test("builds one isolated candidate journey from pinned source and explicit services", () => {
  const f = fixture(), plan = f.run();
  assert.equal(plan.layout.cli, "D:\\runner temp\\lifecycle\\installed\\resources\\app.asar.unpacked\\resources\\bin\\multica.exe");
  assert.equal(plan.phaseRoot, "D:\\runner temp\\collaboration\\upgrade_candidate");
  assert.equal(plan.home, "D:\\runner temp\\collaboration\\upgrade_candidate\\home");
  assert.equal(plan.sourceCommit, f.env.GITHUB_SHA);
});

test("refuses a baseline, persistent runner, unsupported host and mismatched source", () => {
  for (const mutate of [f => { f.options.phase = "baseline"; }, f => { f.env.RUNNER_ENVIRONMENT = "self-hosted"; },
    f => { f.setup.sourceCommit = "b".repeat(40); }, f => { f.setup.repo = "D:\\another-repo"; }]) {
    const f = fixture(); mutate(f); assert.throws(f.run);
  }
  const f = fixture();
  assert.throws(() => windowsCollaborationPlan(f.options, f.setup, f.env, "darwin", "arm64"));
  assert.throws(() => windowsCollaborationPlan(f.options, f.setup, f.env, "win32", "arm64"));
});

test("refuses escaped roots, foreign binaries, business databases and external endpoints", () => {
  for (const mutate of [f => { f.options.installedDirectory = "C:\\Users\\person\\Multica"; },
    f => { f.options.stateDirectory = "D:\\runner temp\\..\\business"; }, f => { f.options.report = "D:\\outside.json"; },
    f => { f.setup.collaboration.daemon = "D:\\other\\daemon.exe"; }, f => { f.setup.api = "https://business.example"; },
    f => { f.setup.collaboration.webUrl = "http://name:13001"; }, f => { f.setup.environment.DATABASE_URL = "postgres://postgres@127.0.0.1:15432/multica"; },
    f => { f.setup.environment.MULTICA_KNOWLEDGE_CONTEXT_ENABLED = "true"; }]) {
    const f = fixture(); mutate(f); assert.throws(f.run);
  }
});

test("the real driver refuses a non-disposable host before opening its private setup or launching a process", () => {
  const result = spawnSync(process.execPath, [fileURLToPath(new URL("./windows-collaboration.mjs", import.meta.url)),
    "--phase", "upgrade_candidate", "--installed-directory", "C:\\missing\\installed", "--state-dir", "C:\\missing\\state", "--report", "C:\\missing\\report.json"],
    { env: { ...process.env, GITHUB_ACTIONS: "true", RUNNER_ENVIRONMENT: "self-hosted" }, encoding: "utf8", timeout: 10_000 });
  assert.ifError(result.error);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /Native Windows required|Disposable GitHub runner required/);
  assert(!result.stderr.includes("ENOENT"), "The driver must refuse before attempting to read private state");
});


test("failure diagnostics expose fixed stages and source coordinates without private payload", () => {
  const secret = "mct_private-synthetic-token";
  const repo = "D:\\a\\multica";
  const file = `${repo}\\e2e\\collaboration-host-installed.spec.ts`;
  const failure = { name: "AssertionError", code: "ERR_ASSERTION", message: secret, expected: secret, actual: secret,
    stack: `AssertionError: ${secret}\n    at privateFunction (${file}:93:7)\n    at secret (D:\\private\\${secret}.ts:1:1)` };
  const stdout = JSON.stringify({ stats: { expected: 0, unexpected: 1, skipped: 0, flaky: 0 }, errors: [],
    config: { secret }, suites: [{ title: secret, specs: [{ tests: [{ results: [{ status: "failed", error: failure,
      errorLocation: { file, line: 93, column: 7 }, stdout: [{ text: secret }], attachments: [{ body: secret }] }] }] }] }] });
  const result = collaborationDiagnostic({ stage: "playwright", error: { code: 1, message: secret }, stdout,
    stderr: secret, repo, journey: { boot: { password: secret }, transport: { secret }, unknown: secret } });
  assert.equal(result.process_exit_code, 1);
  assert.deepEqual(result.locations, [{ file: "e2e/collaboration-host-installed.spec.ts", line: 93, column: 7 }]);
  assert(result.error_kinds.includes("AssertionError"));
  assert.deepEqual(result.error_codes, ["ERR_ASSERTION"]);
  assert.equal(result.milestones.boot, true);
  assert.equal(result.milestones.login, false);
  for (const forbidden of [secret, "privateFunction", repo, "password", "attachments", "stdout"]) assert(!JSON.stringify(result).includes(forbidden));
});

test("failure diagnostics fail closed for malformed reports and unknown free text", () => {
  const secret = "private-fixture-password";
  const result = collaborationDiagnostic({ stage: secret, repo: "D:/a/repo", stdout: secret, stderr: secret,
    error: { name: secret, code: secret, signal: secret, message: secret, stack: secret } });
  assert.equal(result.stage, "unknown");
  assert.equal(result.json_report, "unavailable");
  assert.equal(result.process_error_code, null);
  assert.equal(result.signal, null);
  assert.deepEqual(result.locations, []);
  assert(!JSON.stringify(result).includes(secret));
});

test("Windows selection includes only the complete knowledge-OFF installed journey", () => {
  const pattern = new RegExp(windowsCollaborationTestPattern);
  assert(pattern.test("chromium e2e/collaboration-host-installed.spec.ts Packaged full Main controls a real native host run and explicitly resumes the shared Agent"));
  assert(!pattern.test("Packaged full Main with default knowledge ON isolates each native run through Stop, restart and Resume"));
});


test("diagnostics tolerate malformed nested report data and cap result inventories", () => {
  assert.deepEqual(collaborationResults({ suites: [null, { specs: "secret", suites: [{ specs: [{ tests: [null, { results: [null] }] }] }] }] }), []);
  const result = collaborationDiagnostic({ stage: "playwright", repo: "D:/repo", error: { code: "ENOENT", name: "Error" },
    stdout: JSON.stringify({ suites: [{ specs: [{ tests: [{ results: Array.from({ length: 30 }, () => ({ status: "private-status" })) }] }] }], errors: "private-errors" }) });
  assert.equal(result.process_error_code, "ENOENT");
  assert.equal(result.result_statuses.length, 16);
  assert(result.result_statuses.every(value => value === "unknown"));
  assert(!JSON.stringify(result).includes("private-"));
});


test("service evidence rejects foreign ownership and strips private process data", () => {
  const secret = "private-password";
  const flags = { present: true, alive: true, path_matches: true, start_matches: true };
  const result = collaborationProcessEvidence({ source_matches: true, api: { ...flags, command: secret }, web: { ...flags, start_matches: false }, secret });
  assert.equal(result.api.path_matches, true);
  assert.equal(result.web.start_matches, false);
  assert(!JSON.stringify(result).includes(secret));
  assert.equal(collaborationProcessEvidence({ api: { alive: secret } }).api.alive, false);
});

test("API preflight binds health/config to the owned process and exposes only fixed refusal causes", async () => {
  let foreign = false;
  const server = http.createServer((request, response) => {
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify(request.url === "/health" ? { status: "ok", pid: foreign ? 999 : 123, commit: "a".repeat(40), secret: "private-token" }
      : request.url === "/healthz" ? { status: "ok" }
        : { auth_mode: "password", managed_installation_supported: true, deployment_id: "private-deployment", secret: "private-password" }));
  });
  server.listen(0, "127.0.0.1"); await once(server, "listening");
  const setup = { api: `http://127.0.0.1:${server.address().port}`, pid: 123, sourceCommit: "a".repeat(40), environment: { MULTICA_DEPLOYMENT_ID: "private-deployment" } };
  try {
    const valid = await probeCollaborationApi(setup);
    assert(Object.values(valid).every(item => item.valid));
    assert(!JSON.stringify(valid).includes("private-"));
    foreign = true;
    assert.equal((await probeCollaborationApi(setup)).health.valid, false);
  } finally { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); }
  const stopped = await probeCollaborationApi(setup);
  for (const item of Object.values(stopped)) {
    assert.equal(item.valid, false); assert.equal(item.status, null); assert(["ECONNREFUSED", "ECONNRESET", "UND_ERR_SOCKET"].includes(item.cause_code), "A stopped listener must expose only its fixed transport failure code");
  }
  assert(!JSON.stringify(stopped).includes(setup.api));
});


test("daemon failure marker projects only fixed state and ownership booleans", () => {
  const secret = "mct_private-fixture-secret";
  const value = { expectedState: "running", expectedProfile: secret, authenticated: true, managedDeploymentAdvertised: true,
    actual: { state: "stopped", profileMatches: false, serverMatches: true, pid: 12345, token: secret,
      installation: { present: true, serverMatches: false, userMatches: true, user: secret } },
    profileFiles: { "config.json": { exists: true, outside_expected_home: false, diagnostic_read_failed: false, text: secret },
      ".desktop-user-id": { exists: false }, "daemon.pid": { diagnostic_read_failed: true, pid: 12345 },
      "daemon.log": { text: secret }, [secret]: { exists: true } }, password: secret };
  const result = daemonFailureProjection([{ message: `${secret}\nInstalled daemon diagnostic: ${JSON.stringify(value)}\n${secret}` }]);
  assert.equal(result.status, "parsed"); assert.equal(result.expected_state, "running"); assert.equal(result.actual_state, "stopped");
  assert.equal(result.profile_matches, false); assert.equal(result.installation_user_matches, true);
  assert.deepEqual(result.files.config, { exists: true, outside_home: false, read_failed: false, server_matches: null, deployment_matches: null, deployment_present: null, credential_present: null });
  assert.deepEqual(result.files.pid, { exists: null, outside_home: null, read_failed: true });
  assert(!JSON.stringify(result).includes(secret)); assert(!JSON.stringify(result).includes("12345"));
  const publicReport = collaborationDiagnostic({ stage: "playwright", repo: "D:/repo", stdout: JSON.stringify({
    suites: [{ specs: [{ tests: [{ results: [{ error: { message: `Installed daemon diagnostic: ${JSON.stringify(value)}` } }] }] }] }],
  }) });
  assert.deepEqual(publicReport.daemon, result);

  const wrongTypes = daemonFailureProjection([{ stack: 'Installed daemon diagnostic: {"expectedState":"private","actual":{"state":"private","profileMatches":"true"}}' }]);
  assert.equal(wrongTypes.expected_state, "unknown"); assert.equal(wrongTypes.actual_state, "unknown"); assert.equal(wrongTypes.profile_matches, null);
});

test("daemon diagnostics retain every emitted state and reject unsupported states", () => {
  for (const state of ["stopped", "starting", "running", "stopping", "installing_cli", "cli_not_found", "recovery_paused", "auth_expired"]) {
    const result = daemonFailureProjection([{ message: `Installed daemon diagnostic: ${JSON.stringify({ expectedState: state, actual: { state } })}` }]);
    assert.equal(result.expected_state, state); assert.equal(result.actual_state, state);
  }
  for (const state of ["error", "private-unknown-state", true, null]) {
    const result = daemonFailureProjection([{ message: `Installed daemon diagnostic: ${JSON.stringify({ expectedState: state, actual: { state } })}` }]);
    assert.equal(result.expected_state, "unknown"); assert.equal(result.actual_state, "unknown");
  }
});

test("daemon failure projection rejects malformed and oversized markers and ignores unrelated text", () => {
  for (const json of ["not-json", "null", "[]", JSON.stringify({ ignored: "x".repeat(65536) })]) {
    assert.deepEqual(daemonFailureProjection([{ message: `Installed daemon diagnostic: ${json}` }]), { status: "invalid" });
  }
  assert.deepEqual(daemonFailureProjection([{ message: 'prefix Installed daemon diagnostic: {"authenticated":true}' }]), { status: "unavailable" });
  const result = collaborationDiagnostic({ stage: "playwright", repo: "D:/repo", stdout: 'Installed daemon diagnostic: {"authenticated":true}',
    stderr: 'Installed daemon diagnostic: {"authenticated":true}' });
  assert.deepEqual(result.daemon, { status: "unavailable" });
});


test("daemon log diagnostics retain only controller allowlisted codes and markers", () => {
  const secret = "mct_private-fixture-token";
  const diagnostic = { mainErrorCodes: ["EPERM", secret, "EPERM", { code: "ENOENT" }], profileFiles: {
    "daemon.err.log": { exists: true, text: secret, tail_sha256: secret, errorCodes: ["ENOENT", secret, "ERR_PRIVATE", "ENOENT"],
      markers: ["not logged in", secret, "permission denied", "not logged in", { text: "address already in use" }] },
    "daemon.log": { errorCodes: secret, markers: { secret } },
    "config.json": { errorCodes: ["EPERM"], markers: ["not logged in"], token: secret },
  } };
  const result = daemonFailureProjection([{ message: `Installed daemon diagnostic: ${JSON.stringify(diagnostic)}` }]);
  assert.deepEqual(result.main_error_codes, ["EPERM"]);
  assert.deepEqual(result.files.err_log.error_codes, ["ENOENT"]);
  assert.deepEqual(result.files.err_log.markers, ["not logged in", "permission denied"]);
  assert.deepEqual(result.files.log.error_codes, []); assert.deepEqual(result.files.log.markers, []);
  assert.equal(result.files.config.error_codes, undefined);
  for (const forbidden of [secret, "ERR_PRIVATE", "tail_sha256", "text", "token"]) assert(!JSON.stringify(result).includes(forbidden));
});


test("daemon setup diagnostics project fixed stages/status and configuration comparisons", () => {
  const secret = "private-password";
  const value = { actual: { diagnostics: {
    cli: { code: "cli_validation_failed", stage: "cli_setup", httpStatus: 503, message: secret },
    connection: { code: secret, stage: secret, httpStatus: secret, secret },
  } }, profileFiles: {
    "config.json": { serverMatches: false, deploymentMatches: true, deploymentPresent: true, credentialPresent: true, token: secret },
    ".desktop-user-id": { userMatches: true, text: secret },
  } };
  const result = daemonFailureProjection([{ message: `Installed daemon diagnostic: ${JSON.stringify(value)}` }]);
  assert.deepEqual(result.connection_diagnostics, {
    cli: { code: "cli_validation_failed", stage: "cli_setup", http_status: 503 }, connection: { code: "unknown", stage: "unknown", http_status: null },
  });
  assert.equal(result.files.config.server_matches, false); assert.equal(result.files.config.deployment_matches, true);
  assert.equal(result.files.config.deployment_present, true); assert.equal(result.files.config.credential_present, true);
  assert.equal(result.files.owner.user_matches, true);
  assert(!JSON.stringify(result).includes(secret)); assert(!JSON.stringify(result).includes("token"));
});


test("Main-derived startup codes remain public without broadening log vocabulary", () => {
  const derived = ["managed_setup_failed", "pat_mint_failed", "cli_auto_install_failed", "credential_restart_failed"];
  const secret = "private-startup-text";
  const result = daemonFailureProjection([{ message: `Installed daemon diagnostic: ${JSON.stringify({
    mainErrorCodes: [...derived, "EPERM", secret], profileFiles: { "daemon.err.log": { errorCodes: [...derived, "EPERM", secret] } },
  })}` }]);
  assert.deepEqual(result.main_error_codes, [...derived, "EPERM"]);
  assert.deepEqual(result.files.err_log.error_codes, ["EPERM"]);
  assert(!JSON.stringify(result).includes(secret));
});


test("startup classification locates prelogger gates without emitting log paths or values", () => {
  const secret = "mct_private-fixture-token";
  const cases = [
    ["read CLI config: open C:/private/config.json: secret", "profile_config_read"], ["parse CLI config: secret", "profile_config_parse"],
    ["OS boot identity unavailable", "os_boot_identity"], ["invalid management lock owner", "lock_owner"],
    ["installation identity has a live lock owner", "lock_busy"], ["invalid managed daemon PID evidence", "pid_invalid"],
    ["managed daemon process exit is not confirmed", "pid_live"], ["managed control process exit is not confirmed", "control_live"],
    ["managed startup handoff size is invalid", "handoff_size"], ["invalid managed startup handoff", "handoff_json"],
    ["noncanonical management JSON envelope", "noncanonical_json"], ["invalid managed startup scope", "handoff_scope"],
    ["invalid managed startup account version", "handoff_account"], ["invalid management server URL", "handoff_server"],
    ["invalid or duplicated managed workspace", "handoff_workspace"], ["unknown flag: --private", "command_flag"],
    ["panic: private", "go_panic"], ["fatal error: private", "go_fatal"],
  ];
  for (const [message, expected] of cases) {
    const result = daemonStartupDiagnostic(`${message} ${secret}`);
    assert(result.categories.includes(expected)); assert(!JSON.stringify(result).includes(secret));
  }
  const io = daemonStartupDiagnostic(`open C:/private/.installation-locks/${secret}.json: confidential OS message`);
  assert.deepEqual(io.io, [{ operation: "open", target: "lock" }]);
  assert(!JSON.stringify(io).includes(secret)); assert(!JSON.stringify(io).includes("private"));
  assert.deepEqual(daemonStartupDiagnostic(""), { output_present: false, categories: [], io: [] });
  assert.deepEqual(daemonStartupDiagnostic(secret), { output_present: true, categories: [], io: [] });
  const forged = projectDaemonStartupDiagnostic({ output_present: true, categories: [secret, "handoff_size"],
    io: [{ operation: "open", target: "pid", path: secret }, { operation: secret, target: "other" }], raw: secret });
  assert.deepEqual(forged, { output_present: true, categories: ["handoff_size"], io: [{ operation: "open", target: "pid" }] });
});


test("startup classifier covers every stable error literal in the three prelogger managed gates", () => {
  const files = ["server/internal/daemon/managed_pid.go", "server/internal/daemon/managed_lock.go", "server/internal/daemon/managed_handoff.go",
    "server/internal/daemon/managed_process.go", "server/cmd/multica/managed_handoff.go", "server/cmd/multica/managed_control.go",
    "server/internal/daemon/managed_control.go", "server/internal/daemon/managed_identity.go", "server/internal/daemon/managed_lifecycle.go"];
  for (const file of files) {
    let source = readFileSync(new URL(`../../../${file}`, import.meta.url), "utf8");
    if (file === "server/cmd/multica/managed_control.go") source = source.slice(source.indexOf("func managedProcessProfile("));
    if (file === "server/internal/daemon/managed_control.go") source = source.slice(source.indexOf("func readManagementControlFile("));
    if (file.endsWith("managed_identity.go")) source = source.slice(0, source.indexOf("func readManagedIdentity("));
    if (file.endsWith("managed_lifecycle.go")) source = source.slice(0, source.indexOf("func managedProfileUser("));
    for (const match of source.matchAll(/errors\.New\("([^"\n]+)"\)/g)) {
      // Handoff.Validate also emits errors after logger startup; the same fixed vocabulary covers them.
      assert(daemonStartupDiagnostic(match[1]).categories.length > 0, `${file}: stable error is not classified`);
    }
  }
});


test("startup IO classification uses only exact path components before the error separator", () => {
  for (const [line, operation, target] of [
    ["open C:/fixture/config.json.backup: error", "open", "other"],
    ["open C:/fixture/other: error mentions /daemon.pid", "open", "other"],
    ["open C:\\Runner Temp\\profile\\config.json: private OS message", "open", "config"],
    ["lstat D:/Runner Temp/profile/daemon.pid: private OS message", "lstat", "pid"],
    ["mkdir D:\\Runner Temp\\profile\\.installation-locks: private OS message", "mkdir", "lock"],
    ["open D:/Runner Temp/.installation-locks/id.json: private OS message", "open", "lock"],
    ["open D:/Runner Temp/.installation-locks.backup/id.json: private OS message", "open", "other"],
    ["rename C:/one/config.json C:/two/config.json: private OS message", "rename", "other"],
    ["open C:/fixture/config.json", "open", "other"],
  ]) assert.deepEqual(daemonStartupDiagnostic(line).io, [{ operation, target }]);
});


test("encrypted failure evidence authenticates source/phase and never publishes private material", async () => {
  const { publicKey, privateKey } = generateKeyPairSync("rsa", { modulusLength: 3072 });
  const encodedKey = publicKey.export({ type: "spki", format: "der" }).toString("base64");
  const root = await realpath(await mkdtemp(join(tmpdir(), "encrypted-collaboration-")));
  const home = join(root, "owned home");
  const profile = join(home, ".multica", "profiles", "desktop-127.0.0.1-18081");
  await mkdir(profile, { recursive: true });
  const secret = "private-daemon-token";
  const log = Buffer.from("x".repeat(70 * 1024) + secret);
  await writeFile(join(profile, "daemon.err.log"), log);
  const options = { publicKey: encodedKey, phaseRoot: root, home, apiUrl: "http://127.0.0.1:18081", sourceCommit: "a".repeat(40), phase: "upgrade_candidate",
    stdout: JSON.stringify({ business: "excluded-business-payload", suites: [{ specs: [{ tests: [{ results: [{ error: { message: secret, stack: `Error: ${secret}` } }] }] }] }] }),
    stderr: "z".repeat(20 * 1024) + secret };
  const decrypt = (envelope, recipient = privateKey) => {
    const key = privateDecrypt({ key: recipient, oaepHash: "sha256", padding: cryptoConstants.RSA_PKCS1_OAEP_PADDING }, Buffer.from(envelope.wrapped_key, "base64"));
    const decipher = createDecipheriv("aes-256-gcm", key, Buffer.from(envelope.iv, "base64"));
    decipher.setAAD(Buffer.from(JSON.stringify(envelope.aad))); decipher.setAuthTag(Buffer.from(envelope.tag, "base64"));
    return JSON.parse(Buffer.concat([decipher.update(Buffer.from(envelope.ciphertext, "base64")), decipher.final()]).toString("utf8"));
  };
  try {
    const envelope = await encryptedCollaborationDiagnostic(options);
    assert.equal(envelope.status, "sealed");
    assert(!JSON.stringify(envelope).includes(secret)); assert(!JSON.stringify(envelope).includes(home));
    const plaintext = decrypt(envelope);
    assert.equal(plaintext.daemon_error_log.status, "captured"); assert.equal(plaintext.daemon_error_log.truncated, true);
    assert.deepEqual(Buffer.from(plaintext.daemon_error_log.data, "base64"), log.subarray(-65536));
    assert(plaintext.controller_failure.includes(secret)); assert(Buffer.byteLength(plaintext.subprocess_stderr) <= 16384);
    assert(!JSON.stringify(plaintext).includes("excluded-business-payload"));
    for (const unicode of ["汉".repeat(10000), "🙂".repeat(5000) + "汉"]) {
      const unicodeEnvelope = await encryptedCollaborationDiagnostic({ ...options, stderr: unicode,
        stdout: JSON.stringify({ suites: [{ specs: [{ tests: [{ results: [{ error: { message: unicode, stack: unicode } }] }] }] }] }) });
      const unicodePlaintext = decrypt(unicodeEnvelope);
      for (const field of ["controller_failure", "subprocess_stderr"]) {
        assert(Buffer.byteLength(unicodePlaintext[field], "utf8") <= 16384);
        assert(!unicodePlaintext[field].includes("\uFFFD"), "The bounded tail must not split a Unicode code point");
      }
    }
    for (const field of ["ciphertext", "tag"]) {
      const changed = Buffer.from(envelope[field], "base64"); changed[0] ^= 1;
      assert.throws(() => decrypt({ ...envelope, [field]: changed.toString("base64") }));
    }
    for (const aad of [{ ...envelope.aad, source_commit: "b".repeat(40) }, { ...envelope.aad, phase: "reinstall_candidate" }]) {
      assert.throws(() => decrypt({ ...envelope, aad }));
    }
    const wrong = generateKeyPairSync("rsa", { modulusLength: 3072 });
    assert.throws(() => decrypt(envelope, wrong.privateKey));
    const second = await encryptedCollaborationDiagnostic(options);
    assert.notEqual(second.iv, envelope.iv); assert.notEqual(second.ciphertext, envelope.ciphertext);
    assert.equal(await encryptedCollaborationDiagnostic({}), undefined);
    assert.deepEqual(await encryptedCollaborationDiagnostic({ publicKey: "not-key" }), { status: "invalid_key" });
    assert.deepEqual(await encryptedCollaborationDiagnostic({ publicKey: privateKey.export({ type: "pkcs8", format: "der" }).toString("base64") }), { status: "invalid_key" });
    const weak = generateKeyPairSync("rsa", { modulusLength: 2048 });
    assert.deepEqual(await encryptedCollaborationDiagnostic({ publicKey: weak.publicKey.export({ type: "spki", format: "der" }).toString("base64") }), { status: "invalid_key" });
    assert.deepEqual(await encryptedCollaborationDiagnostic({ ...options, sourceCommit: "invalid" }), { status: "invalid_context" });
    assert.equal(decrypt(await encryptedCollaborationDiagnostic({ ...options, home: join(root, "missing") })).daemon_error_log.status, "unavailable");
    assert.equal(decrypt(await encryptedCollaborationDiagnostic({ ...options, home: tmpdir() })).daemon_error_log.status, "rejected");
    const alias = join(root, "alias");
    await symlink(home, alias, process.platform === "win32" ? "junction" : "dir");
    assert.equal(decrypt(await encryptedCollaborationDiagnostic({ ...options, home: alias })).daemon_error_log.status, "rejected");
    assert.equal(decrypt(await encryptedCollaborationDiagnostic({ ...options, apiUrl: "https://external.example" })).daemon_error_log.status, "rejected");
  } finally { await rm(root, { recursive: true, force: true }); }
});


test("native discovery inventory explicitly isolates Claude and refuses ambient providers", () => {
  const f = fixture(); const plan = f.run();
  const probe = readFileSync(new URL("../../../server/internal/daemon/agents_probe.go", import.meta.url), "utf8");
  const descriptors = readFileSync(new URL("../../../server/pkg/agent/builtin_runtimes.go", import.meta.url), "utf8");
  const env = windowsDiscoveryEnvironment(plan, probe, descriptors);
  assert.equal(env.MULTICA_CLAUDE_PATH, plan.binaries.discovery);
  assert(Object.keys(env).length >= 25);
  for (const [key, value] of Object.entries(env)) {
    if (key !== "MULTICA_CLAUDE_PATH") assert(value.startsWith(`${plan.phaseRoot}\\missing-providers\\`) && value.endsWith(`${key}.exe`));
  }
  assert.throws(() => windowsDiscoveryEnvironment(plan, "", ""));
  f.setup.collaboration.discovery = f.setup.collaboration.provider;
  assert.throws(f.run, /distinct/);
  f.setup.collaboration.discovery = "D:\\outside\\discovery.exe";
  assert.throws(f.run, /owned runner root/);
});

test("the real native discovery binary permits only version checks and records every execution refusal", { timeout: 60_000 }, async () => {
  const root = await realpath(await mkdtemp(join(tmpdir(), "native-discovery-")));
  const executable = join(root, process.platform === "win32" ? "discovery.exe" : "discovery");
  const marker = `${executable}.unexpected`;
  const source = fileURLToPath(new URL("../../../e2e/fixtures/collaboration-discovery-only.go", import.meta.url));
  const commit = "a".repeat(40);
  try {
    const build = spawnSync("go", ["build", "-ldflags", `-X main.sourceCommit=${commit}`, "-o", executable, source], { encoding: "utf8", timeout: 45_000 });
    assert.equal(build.status, 0, build.stderr);
    const info = spawnSync("go", ["version", "-m", executable], { encoding: "utf8" });
    assert.equal(info.status, 0, info.stderr); assert(info.stdout.includes(`main.sourceCommit=${commit}`));
    const version = spawnSync(executable, ["--version"], { encoding: "utf8", timeout: 5000 });
    assert.equal(version.status, 0, version.stderr); assert.equal(version.stdout, "2.1.5 (Claude Code)\n");
    await assert.rejects(lstat(marker), { code: "ENOENT" });
    const secret = "private-synthetic-task-input";
    for (const args of [[], ["--print", secret], ["--version", secret], ["--fixture-capabilities"]]) {
      const refused = spawnSync(executable, args, { encoding: "utf8", timeout: 5000 });
      assert.equal(refused.status, 97, refused.stderr); assert.equal(refused.stdout, ""); assert.equal(refused.stderr, "");
    }
    const contents = await readFile(marker, "utf8");
    assert.equal(contents, "unexpected agent execution\n".repeat(4)); assert(!contents.includes(secret));
    const after = spawnSync(executable, ["--version"], { encoding: "utf8", timeout: 5000 });
    assert.equal(after.status, 0); assert.equal(await readFile(marker, "utf8"), contents);
  } finally { await rm(root, { recursive: true, force: true }); }
});
