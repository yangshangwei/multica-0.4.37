import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { windowsCollaborationPlan } from "./windows-collaboration-config.mjs";

function fixture() {
  const env = { GITHUB_ACTIONS: "true", RUNNER_ENVIRONMENT: "github-hosted", GITHUB_SHA: "a".repeat(40),
    GITHUB_WORKSPACE: "D:\\a\\multica", RUNNER_TEMP: "D:\\runner temp" };
  const options = { phase: "upgrade_candidate", stateDirectory: "D:\\runner temp\\collaboration", installedDirectory: "D:\\runner temp\\lifecycle\\installed",
    report: "D:\\runner temp\\collaboration-upgrade.json" };
  const setup = { sourceCommit: env.GITHUB_SHA, repo: env.GITHUB_WORKSPACE, api: "http://127.0.0.1:18081",
    environment: { DATABASE_URL: "postgres://postgres@127.0.0.1:15432/multica_acceptance?sslmode=disable", MULTICA_KNOWLEDGE_CONTEXT_ENABLED: "false", MULTICA_AUTH_MODE: "password" },
    collaboration: { webUrl: "http://127.0.0.1:13001", daemon: "D:\\runner temp\\collaboration\\bin\\daemon.exe", provider: "D:\\runner temp\\collaboration\\bin\\provider.exe", cli: "D:\\runner temp\\collaboration\\bin\\cli.exe" } };
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
