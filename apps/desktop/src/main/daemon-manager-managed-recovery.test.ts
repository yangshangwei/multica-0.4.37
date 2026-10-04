// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { chmod, mkdtemp, mkdir, open, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript";
import * as recovery from "./daemon-recovery";
import * as daemonOS from "./daemon-os";
import * as daemonTypes from "../shared/daemon-types";
import { readManagementPrivateFile } from "./managed-installation";
import * as transitions from "./managed-transition";

const nativeRequire = createRequire(import.meta.url);
const roots: string[] = [];
afterEach(async () => { await Promise.all(roots.splice(0).map(root => rm(root, { recursive: true, force: true }))); });

async function managerFixture(pid: string, withSession: boolean, pendingDrain: boolean, platform: NodeJS.Platform = process.platform, env: NodeJS.ProcessEnv = {}) {
  const root = await mkdtemp(join(tmpdir(), "managed-manager-wiring-")); roots.push(root);
  const profile = { name: "desktop-fixture", port: 19001 };
  const directory = join(root, profile.name); await mkdir(directory);
  await writeFile(join(directory, "config.json"), JSON.stringify({ management_deployment_id: "11111111-1111-4111-8111-111111111111" }));
  await writeFile(join(directory, "daemon.pid"), pid);
  const spawn = vi.fn((_file, _args, _options, callback) => { queueMicrotask(() => callback(null, JSON.stringify({ probe_result: "success", runtime_count: 1, provider_summary: { claude: 1 } }))); return { stdin: { end: vi.fn(), on: vi.fn() } }; });
  const session = { createHandoff: async () => JSON.stringify({ bindings: [] }), connectRunningDaemon: async () => null, matchesDaemonScope: () => true };
  const prepareSession = vi.fn(async () => null);
  const modules: Record<string, unknown> = {
    electron: { app: { getVersion: () => "0.5.0" }, ipcMain: { handle: vi.fn() }, BrowserWindow: { getAllWindows: () => [] }, shell: {} },
    child_process: { execFile: spawn },
    os: { ...nativeRequire("node:os"), homedir: () => root },
    "./daemon-recovery": recovery,
    "./daemon-os": daemonOS,
    "../shared/daemon-types": daemonTypes,
    "./managed-transition": transitions,
    "./daemon-profile": {
      profileDir: (name: string) => join(root, name),
      profilePidPath: (name: string) => join(root, name, "daemon.pid"),
      profileConfigPath: (name: string) => join(root, name, "config.json"),
      profileArgs: (name: string) => ["--profile", name],
    },
    "./managed-installation": { readManagementPrivateFile },
    "./cli-bootstrap": {}, "./version-decision": {}, "./daemon-auth-probe": {}, "./daemon-quit": {}, "./managed-session": { prepareManagedInstallationSession: prepareSession },
  };
  // Load the actual manager and expose only its private orchestration entrypoints
  // inside this isolated test VM. Production code has no test-only exports.
  const source = await readFile(join(__dirname, "daemon-manager.ts"), "utf8");
  const setup = `\nmodule.exports.fixture = {
    recover: () => attemptDaemonRecovery(activeProfile),
    prepareSession: () => { targetApiBaseUrl="http://localhost:18001"; return configureManagedSession("fixture-user-token", "fixture-user", "fixture-pat", activeProfile); },
    writeConfig: (config) => writeProfileConfig(activeProfile.name, config),
    start: () => startDaemon(),
    stop: () => stopDaemon(),
    probe: () => probeLocalRuntimes(),
    exhaustDeferrals: () => { for (let n=0;n<2;n++) recoveryPolicy.recordPidDeferral(Date.now()); },
    configure: (profile, session, pending) => { activeProfile=profile; managedSession=session; cachedCliBinary='fake-cli'; desiredDaemonRunning=true; currentState='stopped'; pendingManagedScopeRefresh=pending; managedPendingReason=pending?'switching':null; if(pending)managedTransitionRecoveries.set(profile.name+':'+profile.port,{stopRequested:true,intentId:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'}); }
  };`;
  const compiled = ts.transpileModule(source + setup, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const module = { exports: {} as { fixture: { recover(): Promise<void>; prepareSession(): Promise<void>; writeConfig(config: Record<string, unknown>): Promise<void>; start(): Promise<{ success: boolean }>; stop(): Promise<{ success: boolean }>; probe(): Promise<{ probeResult: string }>; exhaustDeferrals(): void; configure(profile: unknown, session: unknown, pending: boolean): void } } };
  runInNewContext(compiled, {
    module, exports: module.exports, require: (name: string) => name in modules ? modules[name] : nativeRequire(name),
    process: { env, platform, pid: process.pid, kill: process.kill }, console: { warn: vi.fn(), log: vi.fn(), error: vi.fn() }, URL, AbortController,
    setTimeout: (callback: () => void, delay: number) => delay === 0 ? undefined : setTimeout(callback, delay), clearTimeout,
    fetch: async () => new Response(null, { status: 503 }),
  });
  module.exports.fixture.configure(profile, withSession ? session : null, pendingDrain);
  return { ...module.exports.fixture, configPath: join(directory, "config.json"), prepareSessionCall: prepareSession, spawn, clearPID: () => rm(join(directory, "daemon.pid")), writeControl: (pid: unknown) => writeFile(join(directory, "management-control.json"), JSON.stringify({ version: "1", pid }), { mode: 0o600 }) };
}

it.each([
  ["invalid", true, true], [String(process.pid), true, true],
  ["invalid", false, false], [String(process.pid), false, false],
] as const)("does not spawn managed recovery with PID %s, session %s, drain %s after exhausted deferrals", async (pid, session, draining) => {
  const manager = await managerFixture(pid, session, draining);
  manager.exhaustDeferrals();
  for (let attempt = 0; attempt < 4; attempt++) await manager.recover();
  expect(manager.spawn.mock.calls.length).toBe(0);
});

it.each(["invalid", String(process.pid)])("manual managed Start also requires definite PID absence (%s)", async pid => {
  const manager = await managerFixture(pid, true, true);
  expect(await manager.start()).toMatchObject({ success: false });
  expect(manager.spawn.mock.calls.length).toBe(0);
});

it.each(["invalid", process.pid])("does not infer process absence from missing PID when private control says %s", async pid => {
  const manager = await managerFixture("temporary", true, true);
  await manager.clearPID(); await manager.writeControl(pid);
  manager.exhaustDeferrals(); await manager.recover();
  expect(await manager.start()).toMatchObject({ success: false });
  expect(manager.spawn.mock.calls.length).toBe(0);
});

it("allows a managed manual start when neither PID source identifies a live process", async () => {
  const manager = await managerFixture("temporary", true, false);
  await manager.clearPID();
  expect(await manager.start()).toMatchObject({ success: true });
  expect(manager.spawn.mock.calls.length).toBe(1);
});

for (const operation of ["start", "stop", "probe"] as const) {
  it.each(["'ws://localhost:18393/ws'", "https://old-server.example"])(`${operation} uses the Desktop profile instead of inherited server URL %s`, async inheritedURL => {
    const env = {
      MULTICA_SERVER_URL: inheritedURL,
      PATH: "/fixture/agent-tools:/usr/bin",
      MULTICA_CLAUDE_PATH: "/fixture/agent-tools/claude",
    };
    const manager = await managerFixture("temporary", false, false, process.platform, env);
    await manager.clearPID();

    const result = await manager[operation]();
    expect(result).toMatchObject(operation === "probe" ? { probeResult: "success" } : { success: true });
    expect(manager.spawn).toHaveBeenCalledExactlyOnceWith(
      "fake-cli",
      ["daemon", operation === "probe" ? "probe-runtimes" : operation, "--profile", "desktop-fixture"],
      expect.objectContaining({
        env: {
          PATH: env.PATH,
          MULTICA_CLAUDE_PATH: env.MULTICA_CLAUDE_PATH,
          MULTICA_LAUNCHED_BY: "desktop",
        },
      }),
      expect.any(Function),
    );
    expect(env.MULTICA_SERVER_URL).toBe(inheritedURL);
  });
}

it.each([
  ["darwin", "macos"], ["win32", "windows"], ["linux", "linux"],
] as const)("uses installation protocol OS %s -> %s when preparing the managed session", async (platform, expected) => {
  const manager = await managerFixture("temporary", false, false, platform);
  await manager.prepareSession();
  expect(manager.prepareSessionCall).toHaveBeenCalledWith(expect.objectContaining({ os: expected }));
});

it.skipIf(process.platform === "win32").each(["new", "existing"] as const)("keeps %s daemon profile credentials private for managed startup", async kind => {
  const manager = await managerFixture("temporary", false, false);
  if (kind === "new") await rm(manager.configPath);
  else await chmod(manager.configPath, 0o644);
  const config = { server_url: "http://localhost:18001", auth_token: "synthetic-daemon-credential", management_deployment_id: "11111111-1111-4111-8111-111111111111" };
  await manager.writeConfig(config);
  expect((await stat(manager.configPath)).mode & 0o777).toBe(0o600);
  expect(JSON.parse(await readFile(manager.configPath, "utf8"))).toEqual(config);
});

it("does not truncate the profile while the daemon has a reader open", async () => {
  const manager = await managerFixture("temporary", false, false);
  const previous = await readFile(manager.configPath, "utf8");
  const reader = await open(manager.configPath, "r");
  try {
    await manager.writeConfig({ server_url: "http://localhost:18001", auth_token: "updated-synthetic-credential" });
    expect(await reader.readFile("utf8")).toBe(previous);
    expect(JSON.parse(await readFile(manager.configPath, "utf8")).auth_token).toBe("updated-synthetic-credential");
  } finally { await reader.close(); }
});
