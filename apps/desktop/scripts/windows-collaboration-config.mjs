import assert from "node:assert/strict";
import path from "node:path";
import { nativeAcceptanceHome, nativeInstalledLayout } from "../../../e2e/fixtures/collaboration-installed-layout.mjs";

const phases = ["install_candidate", "upgrade_candidate", "reinstall_candidate"];

/** Validate runner-owned inputs before reading credentials or launching Main. */
export function windowsCollaborationRoots(options, env = process.env, platform = process.platform, arch = process.arch) {
  assert.equal(platform, "win32", "Native Windows required");
  assert.equal(arch, "x64", "Native Windows x64 required");
  assert.equal(env.GITHUB_ACTIONS, "true", "Disposable GitHub runner required");
  assert.equal(env.RUNNER_ENVIRONMENT, "github-hosted", "Disposable GitHub runner required");
  assert(phases.includes(options.phase), "Only the installed candidate is eligible");
  assert.match(env.GITHUB_SHA ?? "", /^[0-9a-f]{40}$/, "Exact candidate source required");
  const local = (value) => nativeAcceptanceHome(value, "win32");
  const under = (root, value) => {
    const relative = path.win32.relative(local(root), local(value));
    assert(relative && !relative.startsWith("..\\") && relative !== ".." && !path.win32.isAbsolute(relative), "Path must stay below the owned runner root");
    return local(value);
  };
  const state = under(env.RUNNER_TEMP, options.stateDirectory);
  const installed = under(env.RUNNER_TEMP, options.installedDirectory);
  const report = under(env.RUNNER_TEMP, options.report);
  assert(report.endsWith(".json"), "The public report must be a JSON file");
  const phaseRoot = under(state, path.win32.join(state, options.phase));
  const layout = nativeInstalledLayout(path.win32.join(installed, "Multica.exe"), "win32");
  return { state, installed, report, phaseRoot, layout, local, under };
}

export function windowsCollaborationPlan(options, setup, env = process.env, platform = process.platform, arch = process.arch) {
  const { state, report, phaseRoot, layout, local, under } = windowsCollaborationRoots(options, env, platform, arch);
  assert.equal(setup.sourceCommit, env.GITHUB_SHA, "Server and installed candidate must use the same source");
  assert.equal(local(setup.repo), local(env.GITHUB_WORKSPACE), "Server checkout must be the selected candidate");
  const endpoint = (value) => {
    const url = new URL(value);
    assert.equal(url.protocol, "http:");
    assert.equal(url.hostname, "127.0.0.1", "Only the task loopback service is eligible");
    assert(!url.username && !url.password && !url.search && !url.hash && (url.pathname === "/" || url.pathname === ""));
    assert(Number(url.port) > 1024, "Explicit task service port required");
    return url.origin;
  };
  const apiUrl = endpoint(setup.api);
  const webUrl = endpoint(setup.collaboration.webUrl);
  assert.notEqual(apiUrl, webUrl);
  const database = new URL(setup.environment.DATABASE_URL);
  assert(["postgres:", "postgresql:"].includes(database.protocol));
  assert.equal(database.hostname, "127.0.0.1");
  assert(Number(database.port) > 1024, "Explicit isolated database port required");
  assert.equal(database.pathname, "/multica_acceptance", "Task-created acceptance database required");
  assert.equal(setup.environment.MULTICA_KNOWLEDGE_CONTEXT_ENABLED, "false", "Knowledge must stay disabled");
  assert.equal(setup.environment.MULTICA_AUTH_MODE, "password");
  const binaries = Object.fromEntries(["daemon", "provider", "cli"].map((key) => [key, under(state, setup.collaboration[key])]));
  return { state, phaseRoot, report, layout, binaries, repo: local(setup.repo), apiUrl, webUrl,
    home: path.win32.join(phaseRoot, "home"), sourceCommit: env.GITHUB_SHA };
}
