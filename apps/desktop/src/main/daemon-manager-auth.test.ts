// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { runInNewContext } from "node:vm";
import ts from "typescript";
import * as authProbe from "./daemon-auth-probe";
import * as recovery from "./daemon-recovery";
import * as daemonOS from "./daemon-os";
import * as daemonQuit from "./daemon-quit";
import * as daemonProfile from "./daemon-profile";
import * as diagnostics from "./daemon-diagnostics";
import * as daemonTypes from "../shared/daemon-types";
import type { DaemonStatus } from "../shared/daemon-types";
import type { DaemonReauthResult as ReauthResult } from "../shared/daemon-types";

const nativeRequire = createRequire(import.meta.url);
const roots: string[] = [];
const apiUrl = "https://password.example";
const profile = { name: "desktop-password.example", port: 19001 };
const cachedToken = "mul_cached-fixture";
const renewedToken = "mul_renewed-fixture";
const sessionToken = "fixture-session-jwt";
const userId = "fixture-user";

afterEach(async () => {
  await Promise.all(roots.splice(0).map((root) => rm(root, { recursive: true, force: true })));
});

type IPCHandler = (event: unknown, ...args: string[]) => unknown;
type LifecycleResult = { success: boolean; error?: string };

async function managerFixture() {
  const root = await mkdtemp(join(tmpdir(), "daemon-auth-"));
  roots.push(root);
  const directory = join(root, profile.name);
  await mkdir(directory);
  const configPath = join(directory, "config.json");
  const ownerPath = join(directory, ".desktop-user-id");
  const originalConfig = { token: cachedToken, server_url: apiUrl, watch_workspaces: ["fixture-workspace"] };
  await writeFile(configPath, JSON.stringify(originalConfig));
  await writeFile(ownerPath, userId);

  const handlers = new Map<string, IPCHandler>();
  const send = vi.fn();
  const probe = vi.fn(async () => new Response(null, { status: 200 }));
  const mint = vi.fn(async () => Response.json({ token: renewedToken }));
  const prepareSession = vi.fn(async () => null);
  const installCli = vi.fn(async () => "unusable-fixture-cli");
  let running = false;
  const health = vi.fn(async () => running ? Response.json({ status: "running" }) : new Response(null, { status: 503 }));
  const fetch = vi.fn(async (url: string, options?: RequestInit) => {
    if (url === `${apiUrl}/api/me`) return probe();
    if (url === `${apiUrl}/api/tokens`) return mint();
    if (url.startsWith("http://127.0.0.1:") && url.endsWith("/health")) {
      return health();
    }
    throw new Error(`Unexpected fixture request: ${url} (${options?.method ?? "GET"})`);
  });
  const spawn = vi.fn((_file: string, args: string[], _options: unknown, callback: (error: Error | null, stdout?: string) => void) => {
    if (args[1] === "stop") running = false;
    queueMicrotask(() => callback(null));
    return { stdin: { end: vi.fn(), on: vi.fn() } };
  });
  const modules: Record<string, unknown> = {
    electron: {
      app: { getVersion: () => "0.5.0", on: vi.fn(), quit: vi.fn() },
      ipcMain: { handle: (channel: string, handler: IPCHandler) => handlers.set(channel, handler), on: vi.fn() },
      BrowserWindow: { getAllWindows: () => [] }, shell: {},
    },
    child_process: { execFile: spawn },
    os: { ...nativeRequire("node:os"), homedir: () => root },
    "./daemon-recovery": recovery,
    "./daemon-os": daemonOS,
    "./daemon-auth-probe": authProbe,
    "./daemon-quit": daemonQuit,
    "../shared/daemon-types": daemonTypes,
    "./daemon-diagnostics": diagnostics,
    "./daemon-profile": {
      ...daemonProfile,
      profileDir: (name: string) => join(root, name),
      profileConfigPath: (name: string) => join(root, name, "config.json"),
      profileUserIdPath: (name: string) => join(root, name, ".desktop-user-id"),
      profilePidPath: (name: string) => join(root, name, "daemon.pid"),
      profileArgs: (name: string) => ["--profile", name],
    },
    "./managed-installation": { readManagementPrivateFile: async () => null },
    "./managed-session": { prepareManagedInstallationSession: prepareSession },
    "./cli-bootstrap": { managedCliPath: () => join(root, "missing-cli"), ensureManagedCli: installCli }, "./version-decision": {}, "./managed-transition": {},
  };
  // Run the real manager and IPC handlers against isolated disk/network/process
  // boundaries. Private setup hooks stay in this VM, never in production exports.
  const source = await readFile(join(__dirname, "daemon-manager.ts"), "utf8");
  const setup = `
    module.exports.fixture = {
      configure: (profile, url) => {
        activeProfile = profile; targetApiBaseUrl = url; cachedCliBinary = "fixture-cli";
        statusPollTimer = 1; desiredDaemonRunning = true;
      },
      ready: () => lifecycleOperations.runForeground(async () => {}),
      holdLifecycle: (pending) => lifecycleOperations.runForeground(() => pending),
      resolveCli: () => { cachedCliBinary = undefined; bundledCliPath = () => "missing-bundled-fixture"; findCliOnPath = () => null; return resolveCliBinary(); },
      resolveWithPath: () => { cachedCliBinary = undefined; bundledCliPath = () => "missing-bundled-fixture"; findCliOnPath = () => "path-fixture-cli"; return resolveCliBinary(); },
      failCli: () => { cachedCliBinary = null; return bootstrapCli(); },
      repairCli: () => { cachedCliBinary = "fixture-cli"; return bootstrapCli(); },
      poll: () => pollOnce(),
      setDesiredRunning: (desired) => { desiredDaemonRunning = desired; },
      supersede: () => { resetManagedSession(); currentState = "running"; },
      blockServerSwitch: () => { serverSwitchBlocked = () => true; },
    };
  `;
  const compiled = ts.transpileModule(source + setup, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const module = { exports: {} as {
    setupDaemonManager(getWindow: () => unknown): void;
    fixture: {
      configure(active: typeof profile, url: string): void;
      ready(): Promise<void>;
      holdLifecycle(pending: Promise<void>): Promise<void>;
      resolveCli(): Promise<string | null>;
      resolveWithPath(): Promise<string | null>;
      failCli(): Promise<void>;
      repairCli(): Promise<void>;
      poll(): Promise<void>;
      setDesiredRunning(desired: boolean): void;
      supersede(): void;
      blockServerSwitch(): void;
    };
  } };
  runInNewContext(compiled, {
    module, exports: module.exports,
    require: (name: string) => name in modules ? modules[name] : nativeRequire(name),
    process: { env: {}, platform: process.platform, pid: process.pid, kill: process.kill },
    console: { log: vi.fn(), warn: vi.fn(), error: vi.fn() },
    URL, AbortController, AbortSignal, fetch,
    setTimeout: vi.fn(() => 0), clearTimeout: vi.fn(), setInterval: vi.fn(() => 0), clearInterval: vi.fn(),
  });
  module.exports.fixture.configure(profile, apiUrl);
  module.exports.setupDaemonManager(() => ({ webContents: { send } }));
  await module.exports.fixture.ready();
  send.mockClear();

  return {
    probe, mint, fetch, spawn, send, prepareSession, health, originalConfig, installCli,
    ...module.exports.fixture,
    setRunning: () => { running = true; },
    sync: (user = userId) => handlers.get("daemon:sync-token")!(null, sessionToken, user) as Promise<void>,
    clear: () => handlers.get("daemon:clear-token")!(null) as Promise<void>,
    setTarget: (url: string) => handlers.get("daemon:set-target-api-url")!(null, url) as Promise<void>,
    reauthenticate: () => handlers.get("daemon:reauthenticate")!(null, sessionToken, userId) as Promise<ReauthResult>,
    start: () => handlers.get("daemon:start")!(null) as Promise<LifecycleResult>,
    status: () => handlers.get("daemon:get-status")!(null) as Promise<DaemonStatus>,
    config: async () => JSON.parse(await readFile(configPath, "utf8")),
    owner: () => readFile(ownerPath, "utf8"),
  };
}

describe("cached daemon credentials in password deployments", () => {
  it("validates and preserves the same user's accepted PAT without minting or restarting", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    await manager.sync();
    expect(manager.fetch).toHaveBeenCalledWith(`${apiUrl}/api/me`, expect.objectContaining({ headers: { Authorization: `Bearer ${cachedToken}` } }));
    expect(manager.mint).not.toHaveBeenCalled();
    expect(manager.spawn).not.toHaveBeenCalled();
    expect(await manager.config()).toEqual(manager.originalConfig);
  });

  it("replaces a revoked cached PAT using the current session even without managed enrollment", async () => {
    const manager = await managerFixture();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    await manager.sync();
    expect(manager.fetch).toHaveBeenCalledWith(`${apiUrl}/api/tokens`, expect.objectContaining({ method: "POST", headers: expect.objectContaining({ Authorization: `Bearer ${sessionToken}` }) }));
    expect(manager.mint).toHaveBeenCalledTimes(1);
    expect(await manager.config()).toEqual({ ...manager.originalConfig, token: renewedToken });
    expect(await manager.owner()).toBe(userId);
    expect(manager.prepareSession).toHaveBeenCalledWith(expect.objectContaining({ daemonToken: renewedToken }));
  });

  it.each([403, 503, "network"])("preserves the cached PAT when its probe is inconclusive (%s)", async (outcome) => {
    const manager = await managerFixture();
    if (outcome === "network") manager.probe.mockRejectedValue(new Error("offline"));
    else manager.probe.mockResolvedValue(new Response(null, { status: Number(outcome) }));
    await manager.sync();
    expect(manager.probe).toHaveBeenCalledTimes(1);
    expect(manager.mint).not.toHaveBeenCalled();
    expect(manager.spawn).not.toHaveBeenCalled();
    expect(await manager.config()).toEqual(manager.originalConfig);
  });

  it.each([401, 503])("leaves disk credentials unchanged when replacement mint fails (%s)", async (status) => {
    const manager = await managerFixture();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    manager.mint.mockResolvedValue(new Response(null, { status }));
    await expect(manager.sync()).rejects.toMatchObject({ status });
    expect(await manager.config()).toEqual(manager.originalConfig);
    expect(await manager.owner()).toBe(userId);
    expect(manager.spawn).not.toHaveBeenCalled();
  });

  it("restarts an already-running daemon after rotating the same user's token", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    await manager.sync();
    expect(manager.spawn.mock.calls.map((call) => call[1])).toEqual([
      ["daemon", "stop", "--profile", profile.name],
      ["daemon", "start", "--profile", profile.name],
    ]);
    expect(await manager.config()).toMatchObject({ token: renewedToken });
  });

  it("does not restart after logout supersedes the credential-change health check", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    let respond!: (value: Response) => void;
    manager.health.mockReturnValueOnce(new Promise((resolve) => { respond = resolve; }));
    const syncing = manager.sync().catch((error: unknown) => error);
    await vi.waitFor(() => expect(manager.health).toHaveBeenCalledTimes(1));
    await manager.clear();
    respond(Response.json({ status: "running" }));
    expect(await syncing).toMatchObject({ message: expect.stringMatching(/superseded/i) });
    expect(manager.spawn).not.toHaveBeenCalled();
    expect(await manager.config()).not.toHaveProperty("token");
  });

  it("does not restart a credential change queued before logout", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    let release!: () => void;
    const held = manager.holdLifecycle(new Promise<void>((resolve) => { release = resolve; }));
    const syncing = manager.sync().catch((error: unknown) => error);
    await vi.waitFor(() => expect(manager.health).toHaveBeenCalledTimes(1));
    await manager.clear();
    release();
    await held;
    expect(await syncing).toMatchObject({ message: expect.stringMatching(/superseded/i) });
    expect(manager.spawn).not.toHaveBeenCalled();
    expect(await manager.config()).not.toHaveProperty("token");
  });

  it("allows login credential rotation before auto-start sets desired-running", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    manager.setDesiredRunning(false);
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    await manager.sync();
    expect(manager.spawn.mock.calls.map((call) => call[1][1])).toEqual(["stop", "start"]);
  });

  it("does not mint after a pending cached-token probe is superseded by a server switch", async () => {
    const manager = await managerFixture();
    let respond!: (value: Response) => void;
    manager.probe.mockReturnValue(new Promise((resolve) => { respond = resolve; }));
    const syncing = manager.sync().catch((error: unknown) => error);
    await vi.waitFor(() => expect(manager.probe).toHaveBeenCalledTimes(1));
    manager.blockServerSwitch();
    respond(new Response(null, { status: 401 }));
    expect(await syncing).toMatchObject({ message: expect.stringMatching(/superseded|switch/i) });
    expect(manager.mint).not.toHaveBeenCalled();
    expect(await manager.config()).toEqual(manager.originalConfig);
  });

  it("does not restore credentials after logout supersedes a pending replacement mint", async () => {
    const manager = await managerFixture();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    let respond!: (value: Response) => void;
    manager.mint.mockReturnValue(new Promise((resolve) => { respond = resolve; }));
    const syncing = manager.sync().catch((error: unknown) => error);
    await vi.waitFor(() => expect(manager.mint).toHaveBeenCalledTimes(1));
    await manager.clear();
    respond(Response.json({ token: renewedToken }));
    expect(await syncing).toMatchObject({ message: expect.stringMatching(/superseded/i) });
    expect(await manager.config()).not.toHaveProperty("token");
    await expect(manager.owner()).rejects.toMatchObject({ code: "ENOENT" });
    expect(manager.spawn).not.toHaveBeenCalled();
  });

  it("retries managed enrollment once if the cached token is revoked after a successful probe", async () => {
    const manager = await managerFixture();
    manager.prepareSession.mockRejectedValueOnce(Object.assign(new Error("revoked"), { status: 401 }));
    await manager.sync();
    expect(manager.mint).toHaveBeenCalledTimes(1);
    expect(manager.prepareSession).toHaveBeenCalledTimes(2);
    expect(await manager.config()).toMatchObject({ token: renewedToken });
  });

  it("does not repeatedly mint when managed enrollment rejects a newly refreshed token", async () => {
    const manager = await managerFixture();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    manager.prepareSession.mockRejectedValue(Object.assign(new Error("session rejected"), { status: 401 }));
    await expect(manager.sync()).rejects.toMatchObject({ status: 401 });
    expect(manager.mint).toHaveBeenCalledTimes(1);
    expect(manager.prepareSession).toHaveBeenCalledTimes(1);
    expect(manager.spawn).not.toHaveBeenCalled();
  });
});

describe("manual daemon reauthentication", () => {
  it("does not mint or start a queued reauthentication after logout", async () => {
    const manager = await managerFixture();
    let finishStart!: (error: Error | null) => void;
    manager.spawn.mockImplementationOnce((_file, _args, _options, callback) => {
      finishStart = callback;
      return { stdin: { end: vi.fn(), on: vi.fn() } };
    });
    const starting = manager.start();
    await vi.waitFor(() => expect(manager.spawn).toHaveBeenCalledTimes(1));
    const reauthenticating = manager.reauthenticate();
    await manager.clear();
    finishStart(null);
    await starting;
    expect(await reauthenticating).toMatchObject({ ok: false, reason: "transient" });
    expect(manager.mint).not.toHaveBeenCalled();
    expect(manager.spawn).toHaveBeenCalledTimes(1);
    expect(await manager.config()).not.toHaveProperty("token");
  });

  it("does not restart if logout occurs during the manual restart preflight", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    let respond!: (value: Response) => void;
    manager.health.mockReturnValueOnce(new Promise((resolve) => { respond = resolve; }));
    const reauthenticating = manager.reauthenticate();
    await vi.waitFor(() => expect(manager.health).toHaveBeenCalledTimes(1));
    await manager.clear();
    respond(Response.json({ status: "running" }));
    expect(await reauthenticating).toMatchObject({ ok: false, reason: "transient" });
    expect(manager.spawn).not.toHaveBeenCalled();
    expect(await manager.config()).not.toHaveProperty("token");
  });

  it.each([401, 503, "network"])("keeps the existing credentials and process on mint failure (%s)", async (outcome) => {
    const manager = await managerFixture();
    manager.setRunning();
    if (outcome === "network") manager.mint.mockRejectedValue(new Error("offline"));
    else manager.mint.mockResolvedValue(new Response(null, { status: Number(outcome) }));
    expect(await manager.reauthenticate()).toMatchObject({ ok: false, reason: outcome === 401 ? "session_invalid" : "transient" });
    expect(await manager.config()).toEqual(manager.originalConfig);
    expect(await manager.owner()).toBe(userId);
    expect(manager.spawn).not.toHaveBeenCalled();
  });

  it("replaces even an accepted cached token before restarting the daemon", async () => {
    const manager = await managerFixture();
    manager.setRunning();
    expect(await manager.reauthenticate()).toEqual({ ok: true });
    expect(manager.mint).toHaveBeenCalledTimes(1);
    expect(await manager.config()).toEqual({ ...manager.originalConfig, token: renewedToken });
    expect(manager.spawn.mock.calls.map((call) => call[1][1])).toEqual(["stop", "start"]);
  });

  it("preserves the profile when the mint response is malformed", async () => {
    const manager = await managerFixture();
    manager.mint.mockResolvedValue(Response.json({ token: 42 }));
    expect(await manager.reauthenticate()).toMatchObject({ ok: false, reason: "transient" });
    expect(await manager.config()).toEqual(manager.originalConfig);
    expect(await manager.owner()).toBe(userId);
    expect(manager.spawn).not.toHaveBeenCalled();
  });
});

describe("fast daemon startup failures", () => {
  function failStart(manager: Awaited<ReturnType<typeof managerFixture>>) {
    manager.spawn.mockImplementationOnce((_file, _args, _options, callback) => {
      queueMicrotask(() => callback(new Error("daemon exited")));
      return { stdin: { end: vi.fn(), on: vi.fn() } };
    });
  }

  it("reports auth_expired immediately for a confirmed 401 before the grace period", async () => {
    const manager = await managerFixture();
    manager.probe.mockResolvedValue(new Response(null, { status: 401 }));
    failStart(manager);
    expect(await manager.start()).toMatchObject({ success: false });
    expect(manager.send).toHaveBeenLastCalledWith("daemon:status", expect.objectContaining({ state: "auth_expired" }));
    expect(await manager.status()).toMatchObject({ state: "auth_expired" });
    expect(manager.probe).toHaveBeenCalledTimes(1);
  });

  it.each([200, 403, 503, "network"])("reports stopped, not an auth failure, for a probe outcome of %s", async (outcome) => {
    const manager = await managerFixture();
    if (outcome === "network") manager.probe.mockRejectedValue(new Error("offline"));
    else manager.probe.mockResolvedValue(new Response(null, { status: Number(outcome) }));
    failStart(manager);
    expect(await manager.start()).toMatchObject({ success: false });
    expect(manager.probe).toHaveBeenCalledTimes(1);
    expect(await manager.status()).toMatchObject({ state: "stopped" });
  });

  it.each(["session", "server-switch", "healthy-daemon"])("does not publish a late auth failure after %s supersedes the start", async (reason) => {
    const manager = await managerFixture();
    let respond!: (value: Response) => void;
    manager.probe.mockReturnValue(new Promise((resolve) => { respond = resolve; }));
    failStart(manager);
    const starting = manager.start();
    await vi.waitFor(() => expect(manager.probe).toHaveBeenCalledTimes(1));
    if (reason === "session") manager.supersede();
    else if (reason === "server-switch") manager.blockServerSwitch();
    else {
      manager.setRunning();
      expect(await manager.status()).toMatchObject({ state: "running" });
    }
    manager.send.mockClear();
    respond(new Response(null, { status: 401 }));
    expect(await starting).toMatchObject({ success: false });
    expect(manager.send).not.toHaveBeenCalled();
  });
});

describe("persistent daemon diagnostics", () => {
  it("keeps CLI setup and challenge failures independently through polling and status reads", async () => {
    const manager = await managerFixture();
    await manager.failCli();
    manager.prepareSession.mockRejectedValue(new diagnostics.DaemonDiagnosticError("challenge_expired", "registration"));
    const result = await manager.reauthenticate();
    expect(result).toMatchObject({ ok: false, reason: "transient", diagnostic: { code: "challenge_expired", stage: "registration" } });
    const status = await manager.status();
    expect(status).toMatchObject({ state: "cli_not_found", diagnostics: {
      cli: { code: "cli_unavailable", stage: "cli_setup" },
      connection: { code: "challenge_expired", stage: "registration" },
    } });
    await manager.poll();
    expect(await manager.status()).toEqual(status);
    expect(manager.send).toHaveBeenLastCalledWith("daemon:status", status);
    await manager.repairCli();
    expect((await manager.status()).diagnostics).toEqual({ connection: status.diagnostics!.connection });
    manager.prepareSession.mockResolvedValue(null);
    expect(await manager.reauthenticate()).toEqual({ ok: true });
    expect((await manager.status()).diagnostics).toBeUndefined();
  });

  it("keeps a failed connection during a same-account retry, but clears it on logout", async () => {
    const manager = await managerFixture();
    manager.prepareSession.mockRejectedValue(new diagnostics.DaemonDiagnosticError("challenge_scope_mismatch", "registration", ["account"]));
    await manager.reauthenticate();
    const previous = (await manager.status()).diagnostics;
    let reject!: (error: unknown) => void;
    manager.prepareSession.mockImplementationOnce(() => new Promise((_resolve, fail) => { reject = fail; }));
    const retry = manager.reauthenticate();
    await vi.waitFor(() => expect(manager.prepareSession).toHaveBeenCalledTimes(2));
    expect((await manager.status()).diagnostics).toEqual(previous);
    await manager.clear();
    reject(new diagnostics.DaemonDiagnosticError("challenge_expired", "registration"));
    expect(await retry).toMatchObject({ ok: false, reason: "transient" });
    expect((await manager.status()).diagnostics).toBeUndefined();
  });

  it("clears another account's failure before its new credential request settles", async () => {
    const manager = await managerFixture();
    manager.mint.mockRejectedValueOnce(Object.assign(new Error("fixture secret response"), { status: 503 }));
    expect(await manager.reauthenticate()).toMatchObject({ diagnostic: { stage: "credentials", code: "request_failed", httpStatus: 503 } });
    manager.mint.mockReturnValueOnce(new Promise(() => {}));
    void manager.sync("another-fixture-user");
    await vi.waitFor(async () => expect((await manager.status()).diagnostics).toBeUndefined());
  });

  it("projects safe unknown errors without response text or extra fields", async () => {
    const manager = await managerFixture();
    manager.mint.mockRejectedValue(Object.assign(new Error("fixture-token-private-body"), { token: "private", code: "challenge_expired", status: 503 }));
    const result = await manager.reauthenticate();
    expect(result).toEqual({ ok: false, reason: "transient", diagnostic: { code: "request_failed", stage: "credentials", occurredAt: expect.any(Number), httpStatus: 503 } });
    expect(JSON.stringify(await manager.status())).not.toContain("private");
  });
});


describe("CLI resolution diagnostics", () => {
  it("records validation failure only after no candidate is usable, and clears it on a usable PATH fallback", async () => {
    const manager = await managerFixture();
    expect(await manager.resolveCli()).toBeNull();
    expect((await manager.status()).diagnostics?.cli).toMatchObject({ code: "cli_validation_failed" });
    manager.spawn.mockImplementation((_bin, args, _options, callback) => {
      queueMicrotask(() => callback(null, args[0] === "version" && _bin === "path-fixture-cli" ? JSON.stringify({ version: "test-version" }) : "invalid version"));
      return { stdin: { end: vi.fn(), on: vi.fn() } };
    });
    expect(await manager.resolveWithPath()).toBe("path-fixture-cli");
    expect((await manager.status()).diagnostics?.cli).toBeUndefined();
  });

  it("preserves the typed installer failure after all fallbacks fail", async () => {
    const manager = await managerFixture();
    manager.installCli.mockRejectedValue(new diagnostics.DaemonDiagnosticError("cli_checksum_failed", "cli_setup"));
    expect(await manager.resolveCli()).toBeNull();
    expect((await manager.status()).diagnostics?.cli).toMatchObject({ code: "cli_checksum_failed", stage: "cli_setup" });
  });
});


it("preserves external-management guards and runtime metadata when a diagnostic is published", async () => {
  const manager = await managerFixture();
  manager.health.mockResolvedValue(Response.json({ status: "running", os: process.platform === "linux" ? "windows" : "linux", server_url: apiUrl, pid: 123, daemon_id: "fixture-daemon", device_name: "fixture-device", uptime: "2h3m" }));
  const prior = await manager.status();
  expect(prior).toMatchObject({ state: "running", externallyManaged: true, pid: 123 });
  manager.mint.mockResolvedValue(new Response(null, { status: 503 }));
  await manager.reauthenticate();
  expect(manager.send).toHaveBeenLastCalledWith("daemon:status", { ...prior, diagnostics: { connection: expect.objectContaining({ code: "request_failed", stage: "credentials" }) } });
});

it("does not replay an old profile through diagnostics or a pending health read after the target is cleared", async () => {
  const manager = await managerFixture();
  const oldHealth = { status: "running", pid: 123, daemon_id: "previous-daemon", device_name: "previous-device" };
  manager.health.mockResolvedValue(Response.json(oldHealth));
  expect(await manager.status()).toMatchObject({ state: "running", profile: profile.name, pid: 123 });
  let respond!: (value: Response) => void;
  manager.health.mockReturnValueOnce(new Promise((resolve) => { respond = resolve; }));
  const polling = manager.poll();
  await vi.waitFor(() => expect(manager.health).toHaveBeenCalledTimes(2));
  await manager.setTarget("");
  expect(manager.send).toHaveBeenLastCalledWith("daemon:status", { state: "stopped", diagnostics: undefined });
  expect(await manager.reauthenticate()).toMatchObject({ ok: false, reason: "transient" });
  expect(manager.send).toHaveBeenLastCalledWith("daemon:status", {
    state: "stopped", diagnostics: { connection: expect.objectContaining({ stage: "credentials" }) },
  });
  respond(Response.json(oldHealth));
  await polling;
  expect(manager.send).toHaveBeenLastCalledWith("daemon:status", {
    state: "stopped", diagnostics: { connection: expect.objectContaining({ stage: "credentials" }) },
  });
});

it("keeps the new profile snapshot when a previous server's health response arrives late", async () => {
  const manager = await managerFixture();
  let respond!: (value: Response) => void;
  manager.health.mockReturnValueOnce(new Promise((resolve) => { respond = resolve; }));
  const polling = manager.poll();
  await vi.waitFor(() => expect(manager.health).toHaveBeenCalledOnce());
  const nextUrl = "https://second.example";
  await manager.setTarget(nextUrl);
  manager.health.mockImplementation(async () => Response.json({ status: "running", server_url: nextUrl, pid: 456, daemon_id: "current-daemon" }));
  const current = await manager.status();
  expect(current).toMatchObject({ profile: daemonProfile.deriveProfileName(nextUrl), pid: 456 });
  respond(Response.json({ status: "running", server_url: apiUrl, pid: 123, daemon_id: "previous-daemon" }));
  await polling;
  expect(manager.send).toHaveBeenLastCalledWith("daemon:status", current);
  expect(await manager.status()).toEqual(current);
});
