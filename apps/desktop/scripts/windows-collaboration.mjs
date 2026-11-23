import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { constants } from "node:fs";
import { execFile } from "node:child_process";
import { readFile, writeFile, mkdir, copyFile, realpath, lstat } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";
import { windowsCollaborationPlan, windowsCollaborationRoots } from "./windows-collaboration-config.mjs";

const execute = promisify(execFile);
const argumentsList = process.argv.slice(2);
assert.equal(argumentsList.length, 8, "Expected phase, installed-directory, state-dir and report");
const args = Object.fromEntries(Array.from({ length: 4 }, (_, index) => [argumentsList[index * 2], argumentsList[index * 2 + 1]]));
assert.deepEqual(Object.keys(args).sort(), ["--installed-directory", "--phase", "--report", "--state-dir"]);
// Validate the host/root before opening the private server record.
const options = { phase: args["--phase"], installedDirectory: args["--installed-directory"], stateDirectory: args["--state-dir"], report: args["--report"] };
const roots = windowsCollaborationRoots(options);
const setup = JSON.parse(await readFile(path.join(roots.state, "api", "server-private.json"), "utf8"));
const plan = windowsCollaborationPlan(options, setup);
const report = { schema_version: 1, status: "failed", source_commit: plan.sourceCommit, phase: args["--phase"],
  scope: "native_windows_x64_installed_collaboration_host_fixture", started_at: new Date().toISOString(), real_provider_calls: 0,
  limitations: ["Synthetic Provider and task-created accounts; no paid account or quota proof", "No signed/offline trust or Windows arm64 claim"], artifacts: {}, error: null };
const digest = async (file) => createHash("sha256").update(await readFile(file)).digest("hex");
try {
  // Existing phase directories are preserved, never overwritten after failure.
  await mkdir(plan.phaseRoot, { recursive: false });
  for (const file of [path.join(args["--installed-directory"], "Multica.exe"), plan.layout.asar, plan.layout.cli, ...Object.values(plan.binaries)]) {
    assert((await lstat(file)).isFile(), "Only explicit fixture/package files are eligible");
    assert.equal((await realpath(file)).toLowerCase(), path.resolve(file).toLowerCase(), "Reparse-point fixture files are not eligible");
  }
  const { stdout } = await execute("git", ["-C", plan.repo, "rev-parse", "HEAD"], { timeout: 10_000 });
  assert.equal(stdout.trim(), plan.sourceCommit);
  report.artifacts = { desktop_sha256: await digest(path.join(args["--installed-directory"], "Multica.exe")),
    asar_sha256: await digest(plan.layout.asar), bundled_cli_sha256: await digest(plan.layout.cli),
    ...Object.fromEntries(await Promise.all(Object.entries(plan.binaries).map(async ([name, file]) => [`fixture_${name}_sha256`, await digest(file)]))) };
  const env = { ...process.env, ...setup.environment, E2E_PASSWORD_AUTH: "1", COLLABORATION_HOST_ACCEPTANCE: "1",
    COLLABORATION_E2E_API_URL: plan.apiUrl, NEXT_PUBLIC_API_URL: plan.apiUrl, PLAYWRIGHT_BASE_URL: plan.webUrl,
    COLLABORATION_ELECTRON_RENDERER_URL: plan.webUrl, COLLABORATION_HOST_DAEMON: plan.binaries.daemon,
    COLLABORATION_HOST_PROVIDER: plan.binaries.provider, COLLABORATION_HOST_CLI: plan.binaries.cli,
    COLLABORATION_INSTALLED_MAIN_EXECUTABLE: path.join(args["--installed-directory"], "Multica.exe"),
    MULTICA_PACKAGED_ACCEPTANCE_HOME: plan.home, COLLABORATION_INSTALLED_SOURCE_SHA: plan.sourceCommit,
    COLLABORATION_HOST_INSTALLED_ASAR_SHA256: report.artifacts.asar_sha256,
    COLLABORATION_INSTALLED_EXECUTABLE_SHA256: report.artifacts.desktop_sha256,
    COLLABORATION_INSTALLED_CLI_SHA256: report.artifacts.bundled_cli_sha256 };
  for (const key of ["ELECTRON_RUN_AS_NODE", "MULTICA_RUN_REAL_AGENT_SMOKE", "COLLABORATION_HOST_INSTALLED_IMAGE", "COLLABORATION_INSTALLED_IMAGE", "COLLABORATION_INSTALLED_DEB", "COLLABORATION_INSTALLED_APP_DIR", "COLLABORATION_INSTALLED_APP_VOLUME"]) delete env[key];
  const output = path.join(plan.phaseRoot, "playwright-output");
  // Invoke the locked repository Playwright entry with Node, avoiding a Windows
  // shell shim and keeping private failures out of uploaded stdout artifacts.
  const entry = path.join(plan.repo, "node_modules", "@playwright", "test", "cli.js");
  const result = await execute(process.execPath, [entry, "test", "e2e/collaboration-host-installed.spec.ts", "--workers=1", "--retries=0", "--reporter=json", `--output=${output}`],
    { cwd: plan.repo, env, timeout: 540_000, maxBuffer: 16 * 1024 * 1024 }).catch(async (error) => {
      await writeFile(path.join(plan.phaseRoot, "playwright-private.json"), error.stdout ?? "", { mode: 0o600 });
      throw new Error(`Installed collaboration fixture failed: exit=${error.code ?? "unknown"}; private details retained`);
    });
  await writeFile(path.join(plan.phaseRoot, "playwright-private.json"), result.stdout, { mode: 0o600 });
  const suite = JSON.parse(result.stdout);
  assert.equal(suite.stats.expected, 1); assert.equal(suite.stats.unexpected, 0); assert.equal(suite.stats.skipped, 0); assert.equal(suite.stats.flaky, 0);
  const cases = suite.suites.flatMap((file) => file.specs ?? []);
  assert.equal(cases.length, 1);
  const results = cases[0].tests.flatMap((test) => test.results);
  assert.equal(results.length, 1); assert.equal(results[0].status, "passed");
  const attached = results[0].attachments;
  const evidence = attached.find((item) => item.name === "installed-host-evidence");
  assert(evidence?.path, "The real installed journey must attach its evidence");
  report.journey = JSON.parse(await readFile(evidence.path, "utf8"));
  assert.equal(report.journey.source_sha, plan.sourceCommit);
  report.screenshots = [];
  for (const name of ["installed-host-stopped.png", "installed-host-ended.png"]) {
    const attachment = attached.find((item) => item.name === name);
    assert(attachment?.path, `Missing actual packaged screen ${name}`);
    const destination = plan.report.replace(/\.json$/, `-${name}`);
    await copyFile(attachment.path, destination, constants.COPYFILE_EXCL);
    report.screenshots.push({ file: path.basename(destination), sha256: await digest(destination) });
  }
  report.status = "passed";
} catch (error) {
  report.error = error instanceof Error ? error.message : "Installed collaboration acceptance failed";
  process.exitCode = 1;
} finally {
  report.finished_at = new Date().toISOString();
  const encoded = JSON.stringify(report, null, 2);
  assert(!/\b(?:mcb_|mct_|mdt_|mat_|mul_)[A-Za-z0-9_.-]+/.test(encoded), "Public report contains a credential-shaped value");
  await writeFile(plan.report, `${encoded}\n`, { mode: 0o600, flag: "wx" });
  process.stdout.write(`Installed collaboration ${report.status}: ${args["--phase"]}\n`);
}
