import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, realpathSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import process from "node:process";
import { createInterface } from "node:readline";
import { _electron as electron, expect } from "@playwright/test";

// This process controls the packaged executable. It never imports an alternate
// Main, replaces IPC handlers, injects auth, or changes the OS home directory.
const debSha256 = process.env.COLLABORATION_INSTALLED_DEB_SHA256;
const nativeExecutable = process.env.COLLABORATION_INSTALLED_MAIN_EXECUTABLE;
const nativeMode = Boolean(nativeExecutable);
const nativeContents = nativeMode ? path.resolve(nativeExecutable, "../..") : null;
const packageRoot = nativeMode ? path.join(nativeContents, "Resources") : debSha256 === undefined ? "/opt/multica" : "/opt/Multica";
const executable = nativeMode ? nativeExecutable : `${packageRoot}/multica-desktop`;
const packagePath = nativeMode ? `${packageRoot}/app.asar` : `${packageRoot}/resources/app.asar`;
const bundledCliPath = nativeMode ? `${packagePath}.unpacked/resources/bin/multica` : `${packageRoot}/resources/app.asar.unpacked/resources/bin/multica`;
const protocolPrefix = "MULTICA_INSTALLED_REPLY ";
const secrets = new Set();
const errors = [];
const responses = [];
const accountRequests = [];
const writes = [];
let app;
let page;
let apiUrl;
let appUrl;
let executionViewVersion;
let installation;

function scrub(value) {
  let text = typeof value === "string" ? value : JSON.stringify(value);
  for (const secret of secrets) text = text.replaceAll(secret, "<private-fixture-value>");
  return text.replace(/\b(?:eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+|mdt_[A-Za-z0-9_-]+|pat_[A-Za-z0-9_-]+)\b/g, "<credential>");
}

function reply(id, value, error) {
  process.stdout.write(`${protocolPrefix}${scrub({ id, ...(error ? { error } : { value }) })}\n`);
}

function sha(file) {
  return createHash("sha256").update(readFileSync(file)).digest("hex");
}

function installDeb(expected) {
  assert.match(debSha256, /^[0-9a-f]{64}$/);
  assert.equal(sha("/acceptance/package.deb"), debSha256, "Install only the pinned .deb bytes");
  for (const field of ["executableSha256", "asarSha256", "cliSha256"]) assert.match(expected?.[field], /^[0-9a-f]{64}$/);
  assert(!existsSync(packageRoot), "The real package install requires a fresh container");
  const run = (command, args) => {
    const result = spawnSync(command, args, { encoding: "utf8", timeout: 90_000, maxBuffer: 16 * 1024 * 1024 });
    assert.equal(result.status, 0, `${command} failed: ${result.stderr}`);
    return result;
  };
  const field = (name) => run("dpkg-deb", ["--field", "/acceptance/package.deb", name]).stdout.trim();
  const packageName = field("Package");
  const version = field("Version");
  const architecture = field("Architecture");
  assert.equal(packageName, "multica");
  assert.equal(architecture, "arm64");
  const standaloneCli = "/usr/bin/multica";
  const sentinel = "#!/bin/sh\n# Test-owned standalone CLI placeholder.\nexit 73\n";
  assert(!existsSync(standaloneCli), "The fresh container must not carry a user CLI");
  writeFileSync(standaloneCli, sentinel, { mode: 0o755 });
  const command = ["dpkg", "--install", "/acceptance/package.deb"];
  const installed = run(command[0], command.slice(1));
  const [status, installedVersion, installedArch] = run("dpkg-query", ["--show", "--showformat=${Status}\n${Version}\n${Architecture}", packageName]).stdout.trim().split("\n");
  assert.equal(status, "install ok installed");
  assert.equal(installedVersion, version);
  assert.equal(installedArch, architecture);
  const files = run("dpkg-query", ["--listfiles", packageName]).stdout.trim().split("\n");
  const cli = `${packageRoot}/resources/app.asar.unpacked/resources/bin/multica`;
  for (const file of [executable, packagePath, cli]) assert(files.includes(file), `Installed package must own ${file}`);
  const hashes = { executableSha256: sha(executable), asarSha256: sha(packagePath), cliSha256: sha(cli) };
  assert.deepEqual(hashes, expected);
  const desktopEntry = "/usr/share/applications/multica-desktop.desktop";
  assert(files.includes(desktopEntry));
  const desktop = readFileSync(desktopEntry, "utf8");
  for (const line of ["Name=Multica", "Exec=/opt/Multica/multica-desktop %U", "Icon=multica-desktop", "StartupWMClass=Multica", "MimeType=x-scheme-handler/multica;"]) {
    assert(desktop.split("\n").includes(line), `Installed desktop entry must retain ${line}`);
  }
  for (const size of [16, 24, 32, 48, 64, 128, 256, 512]) {
    assert(files.includes(`/usr/share/icons/hicolor/${size}x${size}/apps/multica-desktop.png`));
  }
  assert.equal(realpathSync("/usr/bin/multica-desktop"), executable);
  assert(!files.includes(standaloneCli), "Desktop must not claim the standalone CLI command");
  assert.equal(readFileSync(standaloneCli, "utf8"), sentinel, "Installing Desktop must preserve the separate CLI");
  return { kind: "deb", packageName, version, architecture, status, debSha256, ...hashes,
    desktopEntry, desktopLauncher: "/usr/bin/multica-desktop", cliPathUnclaimed: true, standaloneCliPreserved: true,
    packageManager: { command, exitCode: installed.status, stdout: installed.stdout, stderr: installed.stderr } };
}

function taskEndpoint(value, field) {
  const url = new URL(value);
  if (!nativeMode) assert.equal(url.hostname, "host.docker.internal", `${field} must use the explicit Docker host task endpoint`);
  assert.equal(url.protocol, "http:");
  assert(!url.username && !url.password && !url.search && !url.hash);
  assert(url.port && Number(url.port) > 1024);
  return url.origin;
}

function createProfileFile(file, value) {
  assert(!existsSync(file), `A fresh container is required; ${file} already exists`);
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
}

function execution() {
  return page.getByRole("region", { name: "AI execution", exact: true });
}

async function waitIssue(issueId, title) {
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible({ timeout: 45_000 });
  await expect.poll(async () => (await read(`/api/collaboration/issues/${issueId}`)).id).toBe(issueId);
}

async function refreshIssue(issueId) {
  const requestPath = `/api/collaboration/issues/${issueId}`;
  // A receipt is visible before Query's invalidation read commits. A fresh
  // renderer and its actual issue GET own the next UI intent's observations.
  // An old invalidation response can arrive while reload is pending. Only a
  // request started in the new main-frame navigation belongs to this baseline.
  let navigated = false;
  const currentRequests = new WeakSet();
  const navigation = (frame) => { if (frame === page.mainFrame()) navigated = true; };
  const request = (value) => {
    if (navigated && value.method() === "GET" && new URL(value.url()).pathname === requestPath) currentRequests.add(value);
  };
  page.on("framenavigated", navigation);
  page.on("request", request);
  try {
    const current = page.waitForResponse((response) => currentRequests.has(response.request()) && response.status() === 200);
    await page.reload();
    // Electron can retire a successful reload response body from CDP even for
    // the new navigation. Keep that real UI GET status, then read current JSON
    // through the authenticated renderer instead of CDP getResponseBody.
    assert.equal((await current).status(), 200);
    const issue = await read(requestPath);
    assert.equal(issue.id, issueId);
    const heading = page.getByRole("heading", { name: issue.title, exact: true });
    await expect(heading).toBeVisible({ timeout: 45_000 });
    const header = heading.locator("..").locator("..");
    await expect(header.locator("dl > div").filter({ has: page.getByText("Handler", { exact: true }) }).locator("dd")).toContainText(issue.handler.name);
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    return issue;
  } finally {
    page.off("framenavigated", navigation);
    page.off("request", request);
  }
}

function observations(issue) {
  return { issue_id: issue.id, collaboration_revision: issue.collaboration_revision, lifecycle_epoch: issue.lifecycle_epoch,
    write_context: issue.write_context, session: issue.session, execution: issue.execution };
}

async function read(requestPath) {
  assert(/^\/api\/(?:collaboration\/issues\/[0-9a-f-]{36}(?:\/(?:commands|participants|activity|mention-targets)(?:\?.*)?)?|me\/(?:workbench|collaboration-notifications)(?:\?.*)?|workspaces|me)$/.test(requestPath), "Only named read-only collaboration paths are available");
  // Host acceptance opts in explicitly. Existing installed-client journeys
  // keep their original unversioned read contract and runtime semantics.
  if (executionViewVersion === 6 && /^\/api\/collaboration\/issues\/[0-9a-f-]{36}$/.test(requestPath)) {
    requestPath += "?execution_view_version=6";
  }
  return page.evaluate(async ({ base, requestPath }) => {
    const token = localStorage.getItem("multica_token");
    if (!token) throw new Error("The real password form has not persisted a desktop session");
    const response = await fetch(`${base}${requestPath}`, { headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok) throw new Error(`Installed client read ${requestPath} returned ${response.status}`);
    return response.json();
  }, { base: apiUrl, requestPath });
}

async function cliAvailable() {
  // The packaged preload exposes only getCliPath since the daemon diagnostics backport (dc60bfc87).
  return page.evaluate(async () => {
    const path = await window.daemonAPI.getCliPath();
    return typeof path === "string" && path.trim().length > 0;
  });
}

async function metadata() {
  const main = await app.evaluate(({ app }) => ({
    packaged: app.isPackaged, appPath: app.getAppPath(), name: app.getName(), version: app.getVersion(),
    executable: process.execPath, home: app.getPath("home"), userData: app.getPath("userData"),
    platform: process.platform, arch: process.arch, electron: process.versions.electron,
  }));
  assert.equal(main.packaged, true);
  assert.equal(main.name, "Multica");
  if (nativeMode) {
    assert.equal(main.home, process.env.MULTICA_PACKAGED_ACCEPTANCE_HOME);
    assert.equal(main.userData, path.join(main.home, ".config", "Multica"));
  } else {
    assert.equal(main.home, "/root");
    assert.equal(main.userData, "/root/.config/Multica");
  }
  assert.equal(main.appPath, packagePath);
  assert.equal(main.executable, executable);
  assert.equal(main.platform, nativeMode ? process.platform : "linux");
  assert.equal(main.arch, nativeMode ? process.arch : "arm64");
  assert.equal(main.electron, "39.8.7");
  const native = await page.evaluate(async () => ({
    appInfo: window.desktopAPI.appInfo, config: window.desktopAPI.runtimeConfig,
    daemon: await window.daemonAPI.getStatus(), preferences: await window.daemonAPI.getPrefs(),
    updater: await window.updater.getPreferences(),
  }));
  native.cliInstalled = await cliAvailable();
  assert.equal(native.appInfo.version, main.version);
  assert.equal(native.appInfo.os, nativeMode ? "macos" : "linux");
  assert.equal(native.config.ok, true);
  assert.equal(native.config.source, "configured");
  assert.equal(native.config.config.apiUrl, apiUrl);
  assert.equal(native.config.config.appUrl, appUrl);
  assert.equal(native.preferences.autoStart, false);
  assert.equal(native.updater.automaticUpdates, false);
  await expect.poll(() => page.evaluate(() => window.daemonAPI.getStatus()), { timeout: 30_000 }).toMatchObject({ state: "stopped" });
  native.daemon = await page.evaluate(() => window.daemonAPI.getStatus());
  assert.equal(native.cliInstalled, true);
  const tabs = await page.evaluate(() => {
    const state = JSON.parse(localStorage.getItem("multica_tabs") ?? "null")?.state;
    if (!state) return null;
    return { activeScope: state.activeScope, activeWorkspaceSlug: state.activeWorkspaceSlug,
      accountTabs: state.accountGroup?.tabs.map(({ id, url }) => ({ id, url })) ?? [] };
  });
  return { main, native, tabs, rendererUrl: page.url(), artifactSha256: sha(packagePath), executableSha256: sha(executable),
    errors: [...errors], accountRequests: [...accountRequests], responses: [...responses], writes: [...writes], ...(installation ? { installation } : {}) };
}

async function launch() {
  app = await electron.launch({ executablePath: executable, args: nativeMode ? [] : ["--no-sandbox"],
    ...(nativeMode ? { env: { ...process.env, MULTICA_PACKAGED_ACCEPTANCE: "1", MULTICA_PACKAGED_ACCEPTANCE_HOME: process.env.MULTICA_PACKAGED_ACCEPTANCE_HOME } } : {}),
    timeout: 60_000 });
  page = await app.firstWindow({ timeout: 45_000 });
  page.setDefaultTimeout(20_000);
  page.on("pageerror", (error) => errors.push(scrub(error.message)));
  page.on("response", (response) => {
    const pathname = new URL(response.url()).pathname;
    if (pathname.startsWith("/api/")) responses.push({ path: pathname, method: response.request().method(), status: response.status() });
  });
  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    if (!/^\/api\/(?:collaboration\/|me\/workbench)/.test(pathname)) return;
    const headers = request.headers();
    accountRequests.push({ path: pathname, method: request.method(), workspaceId: headers["x-workspace-id"] ?? null,
      workspaceSlug: headers["x-workspace-slug"] ?? null, clientVersion: headers["x-client-version"] ?? null,
      clientOS: headers["x-client-os"] ?? null });
  });
  await expect.poll(() => new URL(page.url()).protocol).toBe("file:");
  await expect.poll(() => new URL(page.url()).pathname).toBe(`${packagePath}/out/renderer/index.html`);
  // A fresh packaged acceptance home has no locale preference. The journey
  // uses English labels and must initialize that preference before the first
  // password-form interaction, including after a packaged restart.
  await page.evaluate(() => {
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
  });
  await page.reload();
  await expect.poll(() => new URL(page.url()).pathname).toBe(`${packagePath}/out/renderer/index.html`);
  await expect.poll(() => page.evaluate(() => typeof window.daemonAPI?.getPrefs)).toBe("function");
  await expect.poll(() => page.evaluate(() => window.daemonAPI.getStatus().then((status) => status.state)), { timeout: 90_000 }).toBe("stopped");
  await expect.poll(cliAvailable, { timeout: 90_000 }).toBe(true);
}

async function originalResponse(pending, expectedStatus, baseline) {
  const response = await pending;
  const request = response.request();
  const requestId = request.headers()["idempotency-key"];
  const outcome = { receipt: await response.json(), requestId, input: request.postDataJSON() };
  const diagnostic = { path: new URL(response.url()).pathname, status: response.status(), expectedStatus, ...outcome,
    baseline: baseline ? observations(baseline) : null };
  writes.push(diagnostic);
  if (response.status() !== expectedStatus && baseline) diagnostic.current = observations(await read(`/api/collaboration/issues/${baseline.id}`));
  assert.equal(response.status(), expectedStatus, `Installed original write ${diagnostic.path} must preserve its expected status`);
  assert(requestId && /^[0-9a-f-]{36}$/i.test(requestId), "The real client write must carry its original key");
  assert.equal(outcome.input.expected_lifecycle_epoch, baseline?.lifecycle_epoch);
  assert.equal(outcome.input.expected_participant_epoch, baseline?.write_context?.participant_epoch);
  assert.equal(outcome.input.expected_account_epoch, baseline?.write_context?.account_epoch);
  return outcome;
}

async function dispatch(message) {
  switch (message.command) {
    case "install": {
      installation = installDeb(message.expectedInstallation);
      return installation;
    }
    case "init": {
      assert(message.executionViewVersion === undefined || message.executionViewVersion === 6);
      executionViewVersion = message.executionViewVersion;
      if (nativeMode) {
        assert.equal(process.platform, "darwin");
      } else {
        assert.equal(process.platform, "linux");
        assert.equal(homedir(), "/root");
      }
      if (debSha256 !== undefined) installation = installDeb(message.expectedInstallation);
      assert(existsSync(executable) && existsSync(packagePath), "Mount the current electron-builder directory package");
      apiUrl = taskEndpoint(message.apiUrl, "apiUrl");
      appUrl = taskEndpoint(message.appUrl, "appUrl");
      const home = nativeMode ? process.env.MULTICA_PACKAGED_ACCEPTANCE_HOME : "/root";
      const userData = nativeMode ? path.join(home, ".config", "Multica") : "/root/.config/Multica";
      createProfileFile(path.join(home, ".multica", "desktop.json"), { schemaVersion: 1, apiUrl, appUrl,
        wsUrl: `${apiUrl.replace("http:", "ws:")}/ws`, updateUrl: `${apiUrl}/desktop` });
      createProfileFile(path.join(home, ".multica", "desktop_prefs.json"), { autoStart: false, autoStop: false });
      createProfileFile(path.join(userData, "updater-preferences.json"), { automaticUpdates: false });
      await launch();
      const cli = bundledCliPath;
      const result = spawnSync(cli, ["version", "--output", "json"], { encoding: "utf8", timeout: 10_000 });
      assert.equal(result.status, 0, "The bundled CLI version command must execute on the actual container OS");
      const version = JSON.parse(result.stdout);
      assert.equal(typeof version.version, "string");
      return { ...await metadata(), bundledCLI: { sha256: sha(cli), version: version.version,
        command: ["<packaged-bundled-cli>", "version", "--output", "json"] } };
    }
    case "login": {
      secrets.add(message.username);
      secrets.add(message.password);
      const username = page.locator("#password-username");
      const password = page.locator("#password-value");
      await expect(username).toBeVisible({ timeout: 45_000 });
      await username.fill(message.username);
      await password.fill(message.password);
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === "/auth/login" && response.request().method() === "POST");
      await page.locator("#password-form").getByRole("button", { name: "Sign in", exact: true }).click();
      const response = await pending;
      assert.equal(response.status(), 200);
      const data = await response.json();
      secrets.add(data.token);
      assert.equal(data.user.id, message.userId);
      await expect(username).toHaveCount(0);
      await expect(page.getByRole("heading", { name: "Collaboration", exact: true })).toBeVisible({ timeout: 45_000 });
      return { status: response.status(), userId: data.user.id, actualPasswordForm: true };
    }
    case "navigate": {
      assert(/^\/account(?:\?|$)/.test(message.path));
      const result = await page.evaluate((destination) => window.desktopAPI.openAccount(destination), message.path);
      assert.equal(result.ok, true, "Production Main must accept the authenticated account destination");
      if (message.issueId) await waitIssue(message.issueId, message.title);
      return { result, ...await metadata() };
    }
    case "read": return read(message.path);
    case "control": {
      const baseline = await refreshIssue(message.issueId);
      const panel = execution();
      let button = panel.getByRole("button", { name: message.action === "stop" ? "Stop all" : "Resume all", exact: true });
      if (message.resourceId) {
        const row = panel.getByRole("listitem").filter({ has: page.getByText(message.resourceId, { exact: true }) })
          .filter({ has: page.getByRole("button", { name: "Stop target", exact: true }) });
        button = row.getByRole("button", { name: message.action === "stop" ? "Stop target" : "Resume target", exact: true });
      }
      const requestPath = `/api/collaboration/issues/${message.issueId}/control`;
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === requestPath && response.request().method() === "POST");
      await button.click();
      const outcome = await originalResponse(pending, message.status, baseline);
      const result = panel.getByRole("status", { name: "Control result", exact: true });
      await expect(result).toBeVisible();
      return { ...outcome, resultText: await result.innerText() };
    }
    case "comment": {
      const baseline = await refreshIssue(message.issueId);
      const form = page.getByRole("form", { name: "Comments", exact: true });
      await form.getByLabel("Add comment", { exact: true }).fill(message.marker);
      await form.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: message.targetName, exact: true }).click();
      const content = await form.getByLabel("Add comment", { exact: true }).inputValue();
      assert(content.includes(`mention://agent/${message.resourceId}`));
      const requestPath = `/api/collaboration/issues/${message.issueId}/comments`;
      const previewPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${requestPath}/trigger-preview` && response.request().method() === "POST");
      await form.getByRole("button", { name: "Preview delivery", exact: true }).click();
      const preview = await previewPending;
      assert.equal(preview.status(), 200);
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === requestPath && response.request().method() === "POST");
      await form.getByRole("button", { name: "Send comment", exact: true }).click();
      const outcome = await originalResponse(pending, 201, baseline);
      await expect(form.getByLabel("Add comment", { exact: true })).toHaveValue("");
      return { ...outcome, preview: await preview.json(), content };
    }
    case "cancel": {
      const baseline = await refreshIssue(message.issueId);
      // Sequences are lane-local. Selecting only the translated sequence label
      // can cancel A's input when both A and B have sequence1 in different lanes.
      const target = execution().getByRole("listitem").filter({ has: page.getByText(message.resourceId, { exact: true }) })
        .filter({ has: page.getByRole("button", { name: "Stop target", exact: true }) });
      const lane = target.locator("..").locator("..");
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${message.issueId}/commands/${message.commandId}/cancel`
        && response.request().method() === "POST");
      await lane.getByRole("button", { name: `Cancel waiting input ${message.sequence}`, exact: true }).click();
      const outcome = await originalResponse(pending, 200, baseline);
      await expect(page.getByText("Waiting input cancelled.", { exact: true })).toBeVisible();
      return outcome;
    }
    case "history": {
      const toggle = execution().getByRole("button", { name: "Command history", exact: true });
      if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
      const entry = execution().getByRole("region", { name: "Command history", exact: true }).locator(`[data-command-id="${message.commandId}"]`);
      await expect(entry).toBeVisible();
      await expect(entry).toContainText(message.marker);
      await expect(entry).toContainText(message.state);
      if (message.reason) await expect(entry).toContainText(message.reason);
      return { commandId: message.commandId, text: await entry.innerText() };
    }
    case "host-records": {
      assert.equal(executionViewVersion, 6, "Host records require the explicit v6 fixture read contract");
      assert(/^[0-9a-f-]{36}$/.test(message.runId));
      const issue = await refreshIssue(message.issueId);
      assert.equal(issue.execution.protocol_version, 6);
      assert.equal(issue.execution.execution_gate.enabled, true);
      const run = issue.execution.runs.find((value) => value.id === message.runId);
      assert(run, "The current issue must own the requested host run");
      assert.equal(run.execution_mode, "host_process_supervision_v1");
      assert.equal(run.proof_state, "host_exit_observed");
      assert.equal(run.launch_count, "1");
      assert.equal(run.forced_disposition, message.stopped ? "stopped" : null);
      const panel = execution();
      await expect(panel).not.toContainText("Shared AI execution is unavailable.");
      await expect(panel).not.toContainText("New AI execution is disabled.");
      const disclosure = panel.locator(`[id="run-${message.runId}"]`);
      await expect(disclosure).toContainText(message.stopped ? "Managed execution stopped" : "Managed execution ended");
      const toggle = disclosure.locator(`button[aria-controls="records-${message.runId}"]`);
      if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
      const records = disclosure.getByRole("region", { name: "Run records", exact: true });
      await expect(records).toContainText("Standard host execution");
      await expect(records).toContainText("Direct launches: 1");
      await expect(records).toContainText("Observed cleanup scope");
      await expect(records).not.toContainText("Termination confirmed");
      for (const marker of message.markers ?? []) await expect(records).toContainText(marker);
      await disclosure.evaluate((element) => {
        const root = element.closest('[data-tab-scroll-root="account-detail"]');
        if (!root) throw new Error("Packaged account detail scroll root is missing");
        root.scrollTop += element.getBoundingClientRect().top - root.getBoundingClientRect().top - 12;
      });
      await page.evaluate(async () => { await document.fonts.ready;
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
      await expect(disclosure).toBeInViewport();
      return { run, text: await records.innerText(), display: "packaged production renderer", nativeIPCStubs: false };
    }
    case "handoff": {
      const baseline = await refreshIssue(message.issueId);
      await page.getByRole("button", { name: "Transfer", exact: true }).click();
      const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Transfer", exact: true }) });
      await form.getByLabel("Exact recipient", { exact: true }).fill(message.recipientId);
      await form.getByRole("button", { name: "Find account", exact: true }).click();
      await expect(form.getByText(`Selected account: ${message.recipientName}`, { exact: true })).toBeVisible();
      await form.getByLabel("Note", { exact: true }).fill(message.note);
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${message.issueId}/handoffs`
        && response.request().method() === "POST");
      await form.getByRole("button", { name: "Confirm", exact: true }).click();
      const result = await originalResponse(pending, 200, baseline);
      assert.equal(result.input.expected_collaboration_revision, baseline.collaboration_revision);
      await expect(form).toHaveCount(0);
      return result;
    }
    case "workflow": {
      const label = { request_information: "Request information", return_information: "Return information",
        submit_acceptance: "Submit for acceptance" }[message.action];
      assert(label, "Only the named human workflow branches are available");
      const baseline = await refreshIssue(message.issueId);
      assert.equal(baseline.capabilities[message.action], true);
      const optionsPending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${message.issueId}/status-options`
        && new URL(response.url()).searchParams.get("action") === message.action && response.request().method() === "GET");
      await page.getByRole("button", { name: label, exact: true }).click();
      const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
      const optionsResponse = await optionsPending;
      assert.equal(optionsResponse.status(), 200);
      const options = await optionsResponse.json();
      assert.equal(options.action, message.action);
      assert.equal(options.options.length, 1);
      if (message.action === "request_information") {
        await form.getByLabel("Exact recipient", { exact: true }).fill(message.recipientId);
        await form.getByRole("button", { name: "Find account", exact: true }).click();
        await expect(form.getByText(`Selected account: ${message.recipientName}`, { exact: true })).toBeVisible();
      }
      const select = form.getByLabel("Status", { exact: true });
      await expect(select.locator("option")).toHaveCount(1);
      const selectedStatus = await select.locator("option").getAttribute("value");
      assert(selectedStatus, "The directed workflow must expose its prescribed status");
      assert.equal(selectedStatus, options.default_status);
      await select.selectOption(selectedStatus);
      await form.getByLabel("Note", { exact: true }).fill(message.note);
      const requestPath = `/api/collaboration/issues/${message.issueId}/${message.action === "request_information" ? "handoffs"
        : message.action === "return_information" ? "information-returns" : "acceptance-submissions"}`;
      const pending = page.waitForResponse((response) => new URL(response.url()).pathname === requestPath && response.request().method() === "POST");
      await form.getByRole("button", { name: "Confirm", exact: true }).click();
      const outcome = await originalResponse(pending, 200, baseline);
      assert.equal(outcome.input.expected_collaboration_revision, baseline.collaboration_revision);
      assert.equal(outcome.input.expected_status, selectedStatus);
      assert.equal(outcome.input.status_options_version, options.status_options_version);
      if (message.action === "return_information") assert.equal(outcome.input.information_request_id, baseline.current_information_request.id);
      await expect(form).toHaveCount(0);
      return { ...outcome, selectedStatus, baseline };
    }
    case "reload": {
      await refreshIssue(message.issueId);
      return metadata();
    }
    case "restart": {
      await app.close();
      await launch();
      await refreshIssue(message.issueId);
      await expect(page.locator("#password-username")).toHaveCount(0);
      return metadata();
    }
    case "capture": {
      await page.evaluate(async () => { await document.fonts.ready;
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
      const window = await app.browserWindow(page);
      try {
        return { png: await window.evaluate(async (target) => (await target.webContents.capturePage()).toPNG().toString("base64")),
          width: await page.evaluate(() => innerWidth), height: await page.evaluate(() => innerHeight),
          noHorizontalOverflow: await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth) };
      } finally { await window.dispose(); }
    }
    case "status": return metadata();
    case "close": {
      if (app) await app.close();
      app = undefined;
      return { closed: true, errors: [...errors], accountRequests: [...accountRequests], responses: [...responses] };
    }
    default: throw new Error("Unknown installed-client fixture command");
  }
}

const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
for await (const line of input) {
  let message;
  try {
    message = JSON.parse(line);
    assert(Number.isInteger(message.id) && typeof message.command === "string");
    reply(message.id, await dispatch(message));
    if (message.command === "close") break;
  } catch (error) {
    reply(message?.id ?? null, undefined, scrub(error instanceof Error ? error.stack ?? error.message : String(error)));
  }
}
if (app) await app.close().catch(() => undefined);
