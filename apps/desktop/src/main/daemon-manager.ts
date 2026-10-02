import { app, ipcMain, BrowserWindow, shell } from "electron";
import { execFile } from "child_process";
import { randomUUID } from "node:crypto";
import {
  readFile,
  writeFile,
  rename,
  mkdir,
  rm,
  open,
  stat,
} from "fs/promises";
import {
  existsSync,
  watchFile,
  unwatchFile,
  type StatsListener,
} from "fs";
import { join } from "path";
import { homedir, hostname } from "os";
import type {
  DaemonStatus,
  DaemonPrefs,
  LocalRuntimeProbe,
} from "../shared/daemon-types";
import { daemonStatusAlive } from "../shared/daemon-types";
import { ensureManagedCli, managedCliPath } from "./cli-bootstrap";
import { decideVersionAction } from "./version-decision";
import {
  deriveProfileName,
  healthPortForProfile,
  profileArgs,
  profileConfigPath,
  profileDir,
  profileLogPath,
  profilePidPath,
  profileUserIdPath,
} from "./daemon-profile";
import {
  DaemonOperationGate,
  DaemonRecoveryPolicy,
  daemonProcessExists,
  parseDaemonPid,
  recoveryStartAllowed,
  runDaemonRecoveryAttempt,
} from "./daemon-recovery";
import {
  daemonLifecycleUnreachable,
  isDaemonExternallyManaged,
  normalizeHostOS,
} from "./daemon-os";
import {
  classifyAuthProbe,
  isAuthStatusError,
  type AuthProbeResult,
} from "./daemon-auth-probe";
import { createDaemonQuitHandler } from "./daemon-quit";
import { readManagementPrivateFile } from "./managed-installation";
import { prepareManagedInstallationSession, type ManagedInstallationSession } from "./managed-session";
import { transitionManagedDaemon, type ManagedPendingReason, type ManagedTransitionRecovery } from "./managed-transition";
import type { InstallationMetadataProof } from "../shared/managed-installation";

const POLL_INTERVAL_MS = 5_000;
const PREFS_PATH = join(homedir(), ".multica", "desktop_prefs.json");
const LOG_TAIL_RETRY_MS = 2_000;
const LOG_TAIL_MAX_RETRIES = 5;
// How long a start may sit in "starting" (with no /health) before we probe the
// token to find out whether login expired. The daemon's own startup can legitimately
// take a while (it renews the PAT and lists workspaces before serving /health), so we
// wait past the common case to avoid probing healthy-but-slow starts.
const AUTH_PROBE_GRACE_MS = 10_000;
// `multica daemon start` blocks until the daemon reports ready, polling /health
// for up to its own startup timeout (45s in server/cmd/multica/cmd_daemon.go) to
// cover cold-start agent-version detection. This execFile timeout MUST stay
// above that — otherwise Electron kills the CLI supervisor mid-startup and a
// healthy-but-slow start is misreported as a failure (the detached daemon child
// keeps running, so the UI flashes "stopped" then "running").
const DAEMON_START_EXEC_TIMEOUT_MS = 60_000;
const HEALTH_PROBE_TIMEOUT_MS = 2_000;
// Five times the UI probe and equal to the auth-probe grace: a daemon that
// misses this second independent window is no longer treated as merely busy.
const RECOVERY_HEALTH_PROBE_TIMEOUT_MS = 10_000;

const DEFAULT_PREFS: DaemonPrefs = { autoStart: true, autoStop: false };

// Always a resolved Desktop-owned profile. "Not resolved yet" is represented by
// `null` at every call site, never by an empty name — see daemon-profile.ts.
interface ActiveProfile {
  name: string;
  port: number;
}

let statusPollTimer: ReturnType<typeof setInterval> | null = null;
let logTailWatcher: { path: string; listener: StatsListener } | null = null;
let currentState: DaemonStatus["state"] = "installing_cli";
let getMainWindow: () => BrowserWindow | null = () => null;
let statusPollInProgress = false;
let cachedCliBinary: string | null | undefined = undefined;
let cliResolvePromise: Promise<string | null> | null = null;
let cachedCliBinaryVersion: string | null | undefined = undefined;
// Set when a CLI version mismatch was detected but the running daemon is
// busy executing tasks. The poll loop retries the check on each tick and
// fires the restart once active_task_count drops to 0.
let pendingVersionRestart = false;
let targetApiBaseUrl: string | null = null;
let activeProfile: ActiveProfile | null = null;
// Recovery is intentionally process-local: it keeps a daemon alive while the
// Desktop main process is running, but is not an OS service/watchdog.
let desiredDaemonRunning = false;
let daemonQuitting = false;
// Once a foreign-OS daemon (for example WSL2) is observed on this profile, do
// not replace it with a native daemon if its forwarded health endpoint drops.
let externalDaemonObserved = false;
const recoveryPolicy = new DaemonRecoveryPolicy();
const lifecycleOperations = new DaemonOperationGate();

// Auth-probe state for the current start attempt. When a start fails to reach
// "running", we probe the daemon's token once (after AUTH_PROBE_GRACE_MS) to
// decide whether the cause is an expired/invalid login. `authExpired` is sticky
// until the next start attempt or a successful /health, so the UI keeps showing
// the re-login prompt instead of flapping back to "starting". See #3512.
let startingSince: number | null = null;
let authProbeDone = false;
let authExpired = false;

// Serialize all writes to any profile config file. Multiple paths
// (syncToken, resolveActiveProfile, clearToken, watch/unwatch handlers)
// may try to write concurrently; chaining them avoids interleaved writes
// corrupting the JSON.
let configWriteChain: Promise<void> = Promise.resolve();
let managedSession: ManagedInstallationSession | null = null;
let managedPreparation: Promise<ManagedInstallationSession | null> | null = null;
let managedGeneration = 0;
let managedSetupError: string | null = null;
let pendingManagedScopeRefresh = false;
let managedScopeRefreshInProgress = false;
let managedPendingReason: ManagedPendingReason | null = null;
let managedRetryAfter = 0;
const managedTransitionRecoveries = new Map<string, ManagedTransitionRecovery>();

function publishInstallationMetadata(value: InstallationMetadataProof | null): void {
  for (const window of BrowserWindow.getAllWindows()) {
    if (!window.isDestroyed()) window.webContents.send("daemon:installation-metadata", value);
  }
}

function resetManagedSession(): void {
  managedGeneration += 1;
  managedSession?.dispose();
  managedSession = null;
  managedPreparation = null;
  managedSetupError = null;
  pendingManagedScopeRefresh = false;
  managedPendingReason = null;
  managedRetryAfter = 0;
  publishInstallationMetadata(null);
}

async function reconcileManagedSession(active: ActiveProfile, session: ManagedInstallationSession): Promise<{ accepted: boolean; reason?: ManagedPendingReason }> {
  if (managedScopeRefreshInProgress) return { accepted: false, reason: "switching" };
  managedScopeRefreshInProgress = true;
  try {
    const key = `${active.name}:${active.port}`;
    const recovery = managedTransitionRecoveries.get(key) ?? { stopRequested: false };
    managedTransitionRecoveries.set(key, recovery);
    const isStopped = async () => !daemonStatusAlive((await fetchHealthAtPort(active.port))?.status) && await daemonPidIsConfirmedAbsent(active.name, true);
    const isCurrent = () => session === managedSession && managedTransitionRecoveries.get(key) === recovery && activeProfile?.name === active.name && desiredDaemonRunning && !daemonQuitting && !serverSwitchBlocked();
    const result = await transitionManagedDaemon({ session, port: active.port, recovery, isStopped,
      isCurrent,
      waitForStop: async () => {
        for (let attempt = 0; attempt < 30; attempt += 1) {
          if (await isStopped()) return true;
          await new Promise((resolve) => setTimeout(resolve, 250));
        }
        return false;
      },
      start: (handoff) => startDaemon(undefined, handoff, isCurrent),
    });
    if (session === managedSession) {
      pendingManagedScopeRefresh = !result.accepted;
      managedPendingReason = result.accepted ? null : result.reason;
      managedRetryAfter = Date.now() + 15_000;
      scheduleStatusRefresh();
    }
    return result;
  } finally { managedScopeRefreshInProgress = false; }
}

async function refreshManagedScopes(): Promise<{ managed: boolean; accepted: boolean }> {
  const session = managedSession; const active = activeProfile;
  if (!session || !active) return { managed: false, accepted: false };
  pendingManagedScopeRefresh = true;
  if (managedScopeRefreshInProgress || lifecycleOperations.inProgress || serverSwitchBlocked()) return { managed: true, accepted: false };
  const result = await lifecycleOperations.runBackground(() => reconcileManagedSession(active, session));
  return { managed: true, accepted: "accepted" in result && result.accepted === true };
}

async function configureManagedSession(token: string, userId: string, daemonToken: string, active: ActiveProfile, previousDeploymentId?: string): Promise<void> {
  resetManagedSession();
  if (!targetApiBaseUrl) return;
  const generation = managedGeneration;
  pendingManagedScopeRefresh = true;
  managedPendingReason = "switching";
  const target = targetApiBaseUrl;
  const pending = prepareManagedInstallationSession({
    homeDirectory: homedir(), profile: active.name, apiBaseUrl: target, userToken: token, daemonToken, userId,
    desktopVersion: app.getVersion(), os: process.platform === "darwin" ? "macos" : normalizeHostOS(process.platform), previousDeploymentId,
    onMetadataProof: (value) => {
      if (generation === managedGeneration && activeProfile?.name === active.name && targetApiBaseUrl === target) publishInstallationMetadata(value);
    },
    onHeartbeat: () => {
      if (generation === managedGeneration && pendingManagedScopeRefresh) void refreshManagedScopes();
    },
  });
  managedPreparation = pending;
  try {
    const session = await pending;
    if (generation !== managedGeneration || activeProfile?.name !== active.name || targetApiBaseUrl !== target) { session?.dispose(); return; }
    managedSession = session;
    if (!session) { pendingManagedScopeRefresh = false; managedPendingReason = null; }
    publishInstallationMetadata(session?.metadataProof ?? null);
    session?.startHeartbeat();
  } catch (error) {
    if (generation === managedGeneration) managedSetupError = "Managed installation setup is unavailable; no legacy downgrade was attempted";
    throw error;
  } finally { if (managedPreparation === pending) managedPreparation = null; }
}


async function readProfileUserId(profile: string): Promise<string | null> {
  try {
    const raw = await readFile(profileUserIdPath(profile), "utf-8");
    const trimmed = raw.trim();
    return trimmed || null;
  } catch {
    return null;
  }
}

async function writeProfileUserId(
  profile: string,
  userId: string,
): Promise<void> {
  await mkdir(profileDir(profile), { recursive: true });
  await writeFile(profileUserIdPath(profile), userId, "utf-8");
}

async function removeProfileUserId(profile: string, strict = false): Promise<void> {
  try {
    await rm(profileUserIdPath(profile));
  } catch (error) {
    if (strict && !(error && typeof error === "object" && "code" in error && error.code === "ENOENT")) throw error;
  }
}

function normalizeUrl(u: string): string {
  if (!u) return "";
  try {
    const parsed = new URL(u);
    return `${parsed.protocol}//${parsed.host}`.toLowerCase();
  } catch {
    return u.replace(/\/+$/, "").toLowerCase();
  }
}

function urlsMatch(a: string, b: string): boolean {
  const na = normalizeUrl(a);
  const nb = normalizeUrl(b);
  return na.length > 0 && na === nb;
}

function sendStatus(status: DaemonStatus): void {
  const win = getMainWindow();
  win?.webContents.send("daemon:status", status);
}

interface HealthPayload {
  status?: string;
  pid?: number;
  /** Daemon's runtime.GOOS. Absent on daemons older than the #3916 fix. */
  os?: string;
  uptime?: string;
  daemon_id?: string;
  device_name?: string;
  server_url?: string;
  cli_version?: string;
  active_task_count?: number;
  agents?: string[];
  workspaces?: unknown[];
}

async function fetchHealthAtPort(
  port: number,
  timeoutMs = HEALTH_PROBE_TIMEOUT_MS,
): Promise<HealthPayload | null> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(`http://127.0.0.1:${port}/health`, {
      signal: controller.signal,
    });
    if (!res.ok) return null;
    return (await res.json()) as HealthPayload;
  } catch {
    return null;
  } finally {
    clearTimeout(timeout);
  }
}

/**
 * Validates the daemon profile's token against the backend to find out whether
 * a stuck start is an auth problem. Hits the same endpoint `multica auth status`
 * uses (GET /api/me) with the exact token the daemon loads from config.json, so
 * the verdict matches what the daemon itself would get from the server.
 *
 * Only the HTTP status is inspected (never the body) so a future change to the
 * /api/me response shape can't break this — a 401 means the token is rejected,
 * a 2xx means it's fine, and a thrown request means the network is the problem,
 * not auth. See classifyAuthProbe for the full rule set.
 */
async function probeTokenValidity(profile: string): Promise<AuthProbeResult> {
  if (!targetApiBaseUrl) return "unknown";
  const cfg = await readProfileConfig(profile);
  const token = typeof cfg.token === "string" ? cfg.token : "";
  if (!token) return classifyAuthProbe({ noToken: true });
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 4_000);
    const res = await fetch(`${targetApiBaseUrl.replace(/\/+$/, "")}/api/me`, {
      headers: { Authorization: `Bearer ${token}` },
      credentials: "omit",
      redirect: "error",
      signal: controller.signal,
    });
    clearTimeout(timeout);
    return classifyAuthProbe({ status: res.status });
  } catch {
    return classifyAuthProbe({ networkError: true });
  }
}

async function readProfileConfig(
  profile: string,
  strict = false,
): Promise<Record<string, unknown>> {
  try {
    const raw = await readFile(profileConfigPath(profile), "utf-8");
    const parsed = JSON.parse(raw);
    if (strict && (!parsed || typeof parsed !== "object" || Array.isArray(parsed))) throw new Error("Invalid daemon profile configuration");
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (error) {
    if (strict && !(error && typeof error === "object" && "code" in error && error.code === "ENOENT")) throw error;
    return {};
  }
}

async function writeProfileConfig(
  profile: string,
  cfg: Record<string, unknown>,
): Promise<void> {
  const op = async () => {
    await mkdir(profileDir(profile), { recursive: true });
    const path = profileConfigPath(profile);
    const temporary = `${path}.${randomUUID()}.tmp`;
    const file = await open(temporary, "wx", 0o600);
    try {
      try {
        await file.writeFile(JSON.stringify(cfg, null, 2), "utf-8");
        await file.sync();
      } finally { await file.close(); }
      // The daemon requires private credentials and may be reading this file.
      await rename(temporary, path);
    } finally { await rm(temporary, { force: true }); }
  };
  const next = configWriteChain.catch(() => {}).then(op);
  configWriteChain = next.catch(() => {});
  return next;
}

/**
 * Returns the Desktop-owned profile for the current target API URL. Creates
 * the profile's config.json on demand with `server_url` pinned to the target.
 *
 * Returns `null` until the renderer reports its `apiUrl`. There is no profile
 * to act on in that window, and callers must do nothing rather than reach for
 * the user's default CLI profile at `~/.multica/` — neither its files nor its
 * health port.
 */
async function resolveActiveProfile(): Promise<ActiveProfile | null> {
  const target = targetApiBaseUrl;
  if (!target) return null;

  const name = deriveProfileName(target);
  const cfg = await readProfileConfig(name);

  if (cfg.server_url !== target) {
    cfg.server_url = target;
    await writeProfileConfig(name, cfg);
    console.log(`[daemon] initialized profile "${name}" → ${target}`);
  }

  return { name, port: healthPortForProfile(name) };
}

async function ensureActiveProfile(): Promise<ActiveProfile | null> {
  if (activeProfile) return activeProfile;
  // Only a resolved profile is cached; the target URL arrives over IPC shortly
  // after startup, and caching "unresolved" would persist until something
  // happened to invalidate it.
  activeProfile = await resolveActiveProfile();
  return activeProfile;
}

function invalidateActiveProfile(): void {
  activeProfile = null;
  externalDaemonObserved = false;
  recoveryPolicy.reset();
}

function setDesiredDaemonRunning(desired: boolean, explicit = false): void {
  if ((daemonQuitting || serverSwitchBlocked()) && desired) return;
  if (desiredDaemonRunning === desired && !explicit) return;
  desiredDaemonRunning = desired;
  recoveryPolicy.reset();
}

function observeDaemonBoundary(status: DaemonStatus): void {
  if (status.state !== "running") return;
  externalDaemonObserved = status.externallyManaged === true;
  if (externalDaemonObserved) {
    recoveryPolicy.reset();
  }
}

async function fetchHealth(): Promise<DaemonStatus> {
  // While the CLI is being downloaded or has permanently failed, short-circuit
  // polling — there's nothing to probe yet and /health calls would just return
  // "stopped", which would overwrite the correct setup state in the UI.
  if (currentState === "installing_cli" || currentState === "cli_not_found") {
    return { state: currentState };
  }

  const active = await ensureActiveProfile();
  // No profile yet means no daemon of ours to probe. Reporting "stopped" is the
  // honest answer; probing the default port would surface the user's own CLI
  // daemon as if it were Desktop's.
  if (!active) return { state: "stopped" };
  const data = await fetchHealthAtPort(active.port);

  if (!data || data.status !== "running") {
    // A start that never reaches "running" is the symptom; an expired/invalid
    // login is the most common cause and the one with no other signal (the
    // daemon exits before it can serve /health, so we can't read the reason
    // from it). Probe the token once per attempt, after a grace period, to
    // surface a re-login prompt instead of spinning on "starting" forever.
    if (
      currentState === "starting" &&
      !authExpired &&
      !authProbeDone &&
      startingSince !== null &&
      Date.now() - startingSince >= AUTH_PROBE_GRACE_MS
    ) {
      authProbeDone = true;
      if ((await probeTokenValidity(active.name)) === "auth_expired") {
        authExpired = true;
      }
    }
    // Sticky: once login is known-expired, keep reporting it (even after
    // currentState flips away from "starting") until the next start attempt or
    // a successful /health clears the flag.
    if (authExpired) {
      return { state: "auth_expired", profile: active.name };
    }
    // The daemon binds /health before preflight finishes and self-reports
    // "starting" until it's ready. Trust that over our own currentState, so a
    // daemon booting on its own — or started via the CLI — surfaces as
    // "starting" instead of "stopped".
    if (data?.status === "starting") {
      return {
        state: "starting",
        profile: active.name,
      };
    }
    return {
      state:
        currentState === "starting"
          ? "starting"
          : recoveryPolicy.isPaused
            ? "recovery_paused"
            : "stopped",
      profile: active.name,
    };
  }

  // A live, authenticated daemon clears any prior auth-failure verdict so the
  // re-login prompt disappears once the user reconnects.
  authExpired = false;
  startingSince = null;

  // A running daemon whose OS differs from this host's is one we can't drive
  // via the native lifecycle CLI (e.g. Linux-in-WSL2 behind a Windows desktop,
  // reachable only over localhost forwarding). Surface it so the UI disables
  // the auto-start/auto-stop toggles instead of letting them silently no-op,
  // and so before-quit skips a stop that would never land. See #3916.
  const externallyManaged = isDaemonExternallyManaged(
    data.os,
    normalizeHostOS(process.platform),
  );

  // Safety: if we have a target URL and the daemon on our port reports a
  // different server_url, it's not "our" daemon — drop it and re-resolve.
  if (
    targetApiBaseUrl &&
    data.server_url &&
    !urlsMatch(data.server_url, targetApiBaseUrl)
  ) {
    invalidateActiveProfile();
    return { state: "stopped" };
  }

  return {
    state: pendingManagedScopeRefresh && managedPendingReason ? "starting" : "running",
    managementPendingReason: managedPendingReason ?? undefined,
    pid: data.pid,
    uptime: data.uptime,
    daemonId: data.daemon_id,
    deviceName: data.device_name,
    agents: data.agents ?? [],
    workspaceCount: Array.isArray(data.workspaces)
      ? data.workspaces.length
      : 0,
    profile: active.name,
    serverUrl: data.server_url,
    externallyManaged,
  };
}

function findCliOnPath(): string | null {
  const candidates = process.platform === "win32" ? ["multica.exe"] : ["multica"];
  const paths = (process.env["PATH"] ?? "").split(
    process.platform === "win32" ? ";" : ":",
  );
  if (process.platform === "darwin") {
    paths.push("/opt/homebrew/bin", "/usr/local/bin");
  }
  for (const name of candidates) {
    for (const dir of paths) {
      const full = join(dir, name);
      if (existsSync(full)) return full;
    }
  }
  return null;
}

/**
 * Returns the path to the CLI binary bundled inside the Desktop app.
 *
 * - Dev (`electron-vite dev`): `app.getAppPath()` → `apps/desktop`, resolving
 *   to `apps/desktop/resources/bin/multica`. `bundle-cli.mjs` populates this
 *   before dev starts, so iterating on Go changes is "make build → restart".
 * - Packaged: `app.getAppPath()` → `<Multica.app>/Contents/Resources/app.asar`.
 *   electron-builder's `asarUnpack: resources/**` extracts the binary to
 *   `app.asar.unpacked/`, so we swap the path segment to execute it.
 */
function bundledCliPath(): string {
  const binName = process.platform === "win32" ? "multica.exe" : "multica";
  return join(app.getAppPath(), "resources", "bin", binName).replace(
    "app.asar",
    "app.asar.unpacked",
  );
}

async function probeCliBinary(
  bin: string,
  source: "bundled" | "managed" | "path",
): Promise<string | null> {
  try {
    const stdout = await new Promise<string>((resolve, reject) => {
      execFile(
        bin,
        ["version", "--output", "json"],
        { timeout: 5_000 },
        (err, out) => {
          if (err) reject(err);
          else resolve(out);
        },
      );
    });
    const parsed = JSON.parse(stdout) as { version?: string };
    if (typeof parsed.version === "string" && parsed.version.length > 0) {
      return parsed.version;
    }
    console.warn(
      `[daemon] ignoring ${source} CLI at ${bin}: version output was missing or invalid`,
    );
    return null;
  } catch (err) {
    console.warn(`[daemon] ignoring ${source} CLI at ${bin}:`, err);
    return null;
  }
}

/**
 * Returns a usable `multica` binary path. Priority:
 *   1. Cached result from a previous successful resolve.
 *   2. Bundled binary shipped with the Desktop app (`bundle-cli.mjs`).
 *   3. Managed binary already installed in userData (`managedCliPath`).
 *   4. Download + install latest release into userData.
 *   5. `multica` on PATH (dev convenience / user-installed via brew).
 * Returns `null` only when all of the above fail.
 *
 * Bundled is preferred so Desktop iterates in lockstep with Go changes in
 * the same repo — avoids the 404 / stale-API problem when the Desktop's
 * TS side is ahead of the last published CLI release.
 *
 * This function is idempotent and safe to call concurrently — in-flight
 * installs are de-duplicated via `cliResolvePromise`.
 */
async function resolveCliBinary(): Promise<string | null> {
  if (cachedCliBinary !== undefined) return cachedCliBinary;
  if (cliResolvePromise) return cliResolvePromise;

  cliResolvePromise = (async () => {
    const bundled = bundledCliPath();
    if (existsSync(bundled)) {
      const version = await probeCliBinary(bundled, "bundled");
      if (version) {
        console.log(`[daemon] using bundled CLI at ${bundled}`);
        cachedCliBinary = bundled;
        cachedCliBinaryVersion = version;
        return bundled;
      }
    }

    const managed = managedCliPath();
    if (existsSync(managed)) {
      const version = await probeCliBinary(managed, "managed");
      if (version) {
        cachedCliBinary = managed;
        cachedCliBinaryVersion = version;
        return managed;
      }
    }

    try {
      const installed = await ensureManagedCli({
        forceInstall: existsSync(managed),
      });
      const version = await probeCliBinary(installed, "managed");
      if (version) {
        cachedCliBinary = installed;
        cachedCliBinaryVersion = version;
        return installed;
      }
      console.warn(
        `[daemon] managed CLI at ${installed} failed validation after install`,
      );
    } catch (err) {
      console.warn("[daemon] CLI auto-install failed, falling back to PATH:", err);
    }

    const onPath = findCliOnPath();
    if (onPath) {
      const version = await probeCliBinary(onPath, "path");
      if (version) {
        cachedCliBinary = onPath;
        cachedCliBinaryVersion = version;
        return onPath;
      }
    }

    cachedCliBinary = null;
    cachedCliBinaryVersion = null;
    return null;
  })();

  try {
    return await cliResolvePromise;
  } finally {
    cliResolvePromise = null;
  }
}

/**
 * Reads the version of the currently resolved CLI binary. Cached for the
 * process lifetime — the bundled binary doesn't change after bundle time.
 * Returns null on any failure (unknown `go` at bundle time, broken binary,
 * wrong-arch bundled binary, etc.) so callers can fail open.
 */
async function getCliBinaryVersion(): Promise<string | null> {
  if (cachedCliBinaryVersion !== undefined) return cachedCliBinaryVersion;
  const bin = await resolveCliBinary();
  if (!bin) {
    cachedCliBinaryVersion = null;
    return null;
  }
  cachedCliBinaryVersion = await probeCliBinary(bin, "path");
  return cachedCliBinaryVersion;
}

/**
 * Compares the running daemon's `cli_version` against the CLI binary we
 * would use to spawn a new one, and restarts only when safe. The decision
 * logic itself is in `version-decision.ts` (pure, unit-tested); this
 * wrapper handles the async plumbing and side effects.
 *
 * Restart is only fired when ALL of:
 *   - a daemon is actually running on the active profile's port
 *   - both sides report a version and the strings differ
 *   - `active_task_count` is 0 (no in-flight agent work would be killed)
 *
 * On a confirmed mismatch while the daemon is busy, `pendingVersionRestart`
 * is set; the poll loop retries this function on each 5s tick and will fire
 * the restart as soon as the daemon drains.
 */
async function ensureRunningDaemonVersionMatches(): Promise<
  "restarted" | "deferred" | "ok" | "not_running"
> {
  const active = await ensureActiveProfile();
  if (!active) return "not_running";
  const running = await fetchHealthAtPort(active.port);

  // Don't try to version-match a daemon we can't restart (e.g. WSL2). Treat it
  // as up-to-date — restartDaemon would no-op anyway, and skipping here avoids
  // a misleading "restarting daemon" log on every auto-start. #3916.
  if (isDaemonExternallyManaged(running?.os, normalizeHostOS(process.platform))) {
    pendingVersionRestart = false;
    return "ok";
  }

  const bundled = await getCliBinaryVersion();
  const action = decideVersionAction(bundled, running);

  switch (action) {
    case "not_running":
      pendingVersionRestart = false;
      return "not_running";
    case "ok":
      pendingVersionRestart = false;
      return "ok";
    case "defer": {
      if (!pendingVersionRestart) {
        const activeTasks = running?.active_task_count ?? 0;
        console.log(
          `[daemon] CLI version mismatch (bundled=${bundled} running=${running?.cli_version}); deferring restart until ${activeTasks} active task(s) finish`,
        );
      }
      pendingVersionRestart = true;
      return "deferred";
    }
    case "restart":
      console.log(
        `[daemon] CLI version mismatch (bundled=${bundled} running=${running?.cli_version}) — restarting daemon`,
      );
      pendingVersionRestart = false;
      await restartDaemon();
      return "restarted";
  }
}

/**
 * Exchange the user's JWT for a long-lived PAT via POST /api/tokens. The
 * daemon needs a PAT (or `mul_` / `mdt_` token) because JWTs expire in 30
 * days and signatures are tied to a specific backend instance.
 */
async function mintPat(jwt: string): Promise<string> {
  if (!targetApiBaseUrl) {
    throw new Error("mint PAT: target API URL not set");
  }
  const url = `${targetApiBaseUrl.replace(/\/+$/, "")}/api/tokens`;
  const res = await fetch(url, {
    method: "POST",
    credentials: "omit",
    redirect: "error",
    signal: AbortSignal.timeout(15000),
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${jwt}`,
    },
    // Omit expires_in_days → server treats as null → non-expiring PAT.
    body: JSON.stringify({ name: "Multica Desktop" }),
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    // Attach the status so callers can tell a genuine auth rejection (401 — the
    // session token is dead) apart from a transient failure (5xx, etc.) without
    // string-matching the message.
    throw Object.assign(
      new Error(`mint PAT failed: ${res.status} ${res.statusText} ${body}`),
      { status: res.status },
    );
  }
  const data = (await res.json()) as { token?: unknown };
  if (typeof data.token !== "string" || !data.token.startsWith("mul_")) {
    throw new Error("mint PAT: response missing token");
  }
  return data.token;
}

/**
 * Ensure the active profile's config.json has a usable token for the daemon.
 *
 * - Input from the renderer is the user's JWT (from localStorage) plus the
 *   current user's id, so we can detect session changes.
 * - If the profile already has a cached PAT (`mul_...`) AND the sidecar user
 *   id matches the caller, reuse it — minting fresh on every launch would
 *   accumulate garbage in the user's tokens page.
 * - On user mismatch (or first run) call POST /api/tokens with the JWT to
 *   mint a fresh PAT, overwriting any stale cached PAT. This is the critical
 *   path: without it, a previous user's PAT would be used by a new session.
 * - If the caller happens to pass a PAT directly, write it through.
 * - Reports a user mismatch to the caller; the IPC boundary owns the gated
 *   restart so internal callers such as reauthenticate never re-enter it.
 */
async function syncToken(
  tokenFromRenderer: string,
  userId: string,
): Promise<{ active: ActiveProfile; userChanged: boolean }> {
  const active = await ensureActiveProfile();
  if (!active) {
    // Writing here would land the token and server_url in the user's default
    // CLI config. The renderer awaits setTargetApiUrl before calling this, so
    // reaching this branch is a real error rather than a normal startup race.
    throw new Error("daemon profile is not resolved yet; token sync skipped");
  }
  const config = await readProfileConfig(active.name);
  const previousUserId = await readProfileUserId(active.name);
  const userChanged = Boolean(previousUserId) && previousUserId !== userId;
  const sameUserWithCachedPat =
    !userChanged &&
    previousUserId === userId &&
    typeof config.token === "string" &&
    config.token.startsWith("mul_");

  let finalToken: string;
  if (tokenFromRenderer.startsWith("mul_")) {
    finalToken = tokenFromRenderer;
  } else if (sameUserWithCachedPat) {
    finalToken = config.token as string;
  } else {
    try {
      finalToken = await mintPat(tokenFromRenderer);
      console.log(
        `[daemon] minted PAT for profile "${active.name}" (user_changed=${userChanged})`,
      );
    } catch (err) {
      console.error("[daemon] failed to mint PAT:", err);
      throw err;
    }
  }

  config.token = finalToken;
  if (targetApiBaseUrl) config.server_url = targetApiBaseUrl;
  await writeProfileConfig(active.name, config);
  await writeProfileUserId(active.name, userId);
  const previousDeploymentId = typeof config.management_deployment_id === "string" ? config.management_deployment_id : undefined;
  try {
    await configureManagedSession(tokenFromRenderer, userId, finalToken, active, previousDeploymentId);
  } catch (error) {
    if (sameUserWithCachedPat && isAuthStatusError(error)) {
      finalToken = await mintPat(tokenFromRenderer);
      config.token = finalToken;
      await writeProfileConfig(active.name, config);
      await configureManagedSession(tokenFromRenderer, userId, finalToken, active, previousDeploymentId);
    } else { throw error; }
  }

  return { active, userChanged };
}

async function restartDaemonAfterUserSwitch(
  active: ActiveProfile,
): Promise<void> {
  // If we just rotated credentials onto a running daemon, restart it so the
  // in-memory token in the Go process matches the new config.
  const existing = await fetchHealthAtPort(active.port);
  if (managedSession) {
    setDesiredDaemonRunning(true);
    const session = managedSession;
    await lifecycleOperations.runForeground(() => reconcileManagedSession(active, session));
    return;
  }
  if (daemonStatusAlive(existing?.status)) {
    // Restart whether it's "running" or still "starting" — a booting daemon
    // already loaded the old token at startup, so it must be restarted to
    // pick up the rotated credentials.
    console.log(
      "[daemon] user switched — restarting daemon with new credentials",
    );
    // Credential rotation is a one-shot login intent, not poll-driven
    // maintenance: wait for bootstrap/recovery instead of dropping it.
    const restarted = await lifecycleOperations.runForeground(() =>
      restartDaemon(),
    );
    if (!restarted.success) {
      console.warn(
        `[daemon] restart-on-user-switch failed: ${restarted.error ?? "unknown error"}`,
      );
    }
  }
}

async function loadPrefs(): Promise<DaemonPrefs> {
  try {
    const raw = await readFile(PREFS_PATH, "utf-8");
    const parsed = JSON.parse(raw);
    return { ...DEFAULT_PREFS, ...parsed };
  } catch {
    return { ...DEFAULT_PREFS };
  }
}

async function savePrefs(prefs: DaemonPrefs): Promise<void> {
  const dir = join(homedir(), ".multica");
  await mkdir(dir, { recursive: true });
  await writeFile(PREFS_PATH, JSON.stringify(prefs, null, 2), "utf-8");
}

async function clearToken(strict = false): Promise<void> {
  resetManagedSession();
  const active = await ensureActiveProfile();
  // Nothing of ours to clear yet, and the default CLI profile is not ours to
  // strip a token from.
  if (!active) return;
  const config = await readProfileConfig(active.name, strict);
  if ("token" in config) {
    delete config.token;
    await writeProfileConfig(active.name, config);
  }
  // Always drop the sidecar so a subsequent syncToken from any user is
  // treated as a fresh mint, not a reuse of a stale cached PAT.
  await removeProfileUserId(active.name, strict);
}

// Result of a user-initiated daemon re-authentication. The distinction matters:
// only `session_invalid` justifies signing the user out of the whole app; a
// `transient` failure must keep them logged in so they can retry.
export type ReauthResult =
  | { ok: true }
  | { ok: false; reason: "session_invalid" }
  | { ok: false; reason: "transient"; message: string };

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/**
 * Recover the local daemon from the "auth_expired" state. Drops the stale
 * cached PAT, mints a fresh one from the current session token, and restarts
 * the daemon so it loads the new credential.
 *
 * Failures are classified rather than collapsed: a 401 from the mint means the
 * session token itself is dead (`session_invalid` → the renderer drives a full
 * re-login); anything else — mint 5xx, a network blip, a config write error, a
 * restart hiccup — is `transient`, leaving the user signed in so they can retry.
 * This mirrors the conservative classification the startup probe already uses.
 */
async function reauthenticate(
  token: string,
  userId: string,
): Promise<ReauthResult> {
  try {
    await clearToken();
    // syncToken mints a fresh PAT because clearToken just removed any cache.
    await syncToken(token, userId);
  } catch (err) {
    if (isAuthStatusError(err)) return { ok: false, reason: "session_invalid" };
    return { ok: false, reason: "transient", message: errorMessage(err) };
  }
  const restart = await restartDaemon();
  if (!restart.success) {
    return {
      ok: false,
      reason: "transient",
      message: restart.error ?? "failed to restart daemon",
    };
  }
  return { ok: true };
}

function successfulRuntimeProbe(
  providers: string[],
  daemonRunning: boolean,
): Extract<LocalRuntimeProbe, { probeResult: "success" }> {
  const providerSummary: Record<string, number> = {};
  for (const rawProvider of providers) {
    const provider = rawProvider.trim().toLowerCase();
    if (!/^[a-z0-9][a-z0-9_-]{0,63}$/.test(provider)) continue;
    providerSummary[provider] = (providerSummary[provider] ?? 0) + 1;
  }
  const runtimeCount = Object.values(providerSummary).reduce(
    (sum, count) => sum + count,
    0,
  );
  return {
    probeResult: "success",
    runtimeCount,
    providerSummary,
    onlineCount: daemonRunning ? runtimeCount : 0,
    offlineCount: daemonRunning ? 0 : runtimeCount,
  };
}

async function probeLocalRuntimes(): Promise<LocalRuntimeProbe> {
  const health = await fetchHealth();
  if (health.state === "running") {
    return successfulRuntimeProbe(health.agents ?? [], true);
  }

  const bin = await resolveCliBinary();
  if (!bin) return { probeResult: "error" };
  const active = await ensureActiveProfile();
  if (!active) return { probeResult: "error" };
  return new Promise((resolve) => {
    execFile(
      bin,
      ["daemon", "probe-runtimes", ...profileArgs(active.name)],
      { timeout: 15_000, env: desktopSpawnEnv(), maxBuffer: 64 * 1024 },
      (error, stdout) => {
        if (error) {
          resolve({ probeResult: "error" });
          return;
        }
        try {
          const parsed = JSON.parse(stdout) as {
            probe_result?: unknown;
            runtime_count?: unknown;
            provider_summary?: unknown;
          };
          if (
            parsed.probe_result !== "success" ||
            typeof parsed.runtime_count !== "number" ||
            !parsed.provider_summary ||
            typeof parsed.provider_summary !== "object" ||
            Array.isArray(parsed.provider_summary)
          ) {
            resolve({ probeResult: "error" });
            return;
          }
          const providers: string[] = [];
          for (const [provider, count] of Object.entries(
            parsed.provider_summary as Record<string, unknown>,
          )) {
            if (
              !Number.isInteger(count) ||
              (count as number) < 0 ||
              (count as number) > 1000
            ) {
              resolve({ probeResult: "error" });
              return;
            }
            providers.push(...Array<string>(count as number).fill(provider));
          }
          const probe = successfulRuntimeProbe(providers, false);
          resolve(
            probe.runtimeCount === parsed.runtime_count
              ? probe
              : { probeResult: "error" },
          );
        } catch {
          resolve({ probeResult: "error" });
        }
      },
    );
  });
}

// Env passed to every CLI child so the daemon process knows it was spawned
// by the Desktop app. The server uses this to mark runtimes as managed and
// hide CLI self-update UI. Computed lazily so it picks up the PATH fix
// applied by fix-path in main/index.ts — as a top-level const it would
// snapshot process.env at import time, before that block runs.
function desktopSpawnEnv(): NodeJS.ProcessEnv {
  return { ...process.env, MULTICA_LAUNCHED_BY: "desktop" };
}

function scheduleStatusRefresh(): void {
  setTimeout(() => void pollOnce(), 0);
}

async function startDaemon(
  recoveryProfile?: ActiveProfile,
  preparedHandoff?: string,
  managedStartupGuard?: () => boolean,
): Promise<{ success: boolean; error?: string }> {
  if (daemonQuitting || serverSwitchBlocked()) return { success: false, error: "Desktop is quitting or switching servers" };
  const bin = await resolveCliBinary();
  if (!bin) return { success: false, error: "multica CLI is not installed" };

  const active = await ensureActiveProfile();
  if (!active) {
    return { success: false, error: "Waiting for the service address" };
  }
  if (
    recoveryProfile &&
    !recoveryStartAllowed({
      desiredRunning: desiredDaemonRunning,
      externalDaemonObserved,
      expected: recoveryProfile,
      current: active,
    })
  ) {
    return { success: false, error: "Daemon recovery was superseded" };
  }
  try { if (managedPreparation) await managedPreparation; } catch { return { success: false, error: managedSetupError ?? "Managed installation setup failed" }; }
  if (managedSetupError) return { success: false, error: managedSetupError };
  const startupSession = managedSession;
  const startupGeneration = managedGeneration;
  const existing = await fetchHealthAtPort(active.port);
  if (daemonQuitting || serverSwitchBlocked()) return { success: false, error: "Desktop is quitting or switching servers" };
  if (daemonStatusAlive(existing?.status)) {
    // A daemon is already up ("running") or booting ("starting") on this port —
    // don't spawn a second one (the CLI rejects that as "already running").
    // Let polling track it through to "running".
    externalDaemonObserved = isDaemonExternallyManaged(
      existing?.os,
      normalizeHostOS(process.platform),
    );
    if (managedSession) {
      const result = await reconcileManagedSession(active, managedSession);
      scheduleStatusRefresh();
      return result.accepted ? { success: true } : { success: false, error: `Managed daemon transition pending (${result.reason ?? "switching"})` };
    }
    scheduleStatusRefresh();
    return { success: true };
  }
  if (
    recoveryProfile &&
    !recoveryStartAllowed({
      desiredRunning: desiredDaemonRunning,
      externalDaemonObserved,
      expected: recoveryProfile,
      current: active,
    })
  ) {
    return { success: false, error: "Daemon recovery was superseded" };
  }

  try {
    if (await managedProfileRequiresConfirmedExit(active.name) && !await daemonPidIsConfirmedAbsent(active.name, true)) {
      managedPendingReason = "switching";
      pendingManagedScopeRefresh = managedSession !== null;
      return { success: false, error: "Managed daemon process exit is not confirmed; startup remains pending" };
    }
  } catch { return { success: false, error: "Managed daemon process evidence is unavailable; startup remains pending" }; }

  let handoff: string | null = preparedHandoff ?? null;
  if (managedSession && handoff === null) {
    try { handoff = await managedSession.createHandoff(); }
    catch { return { success: false, error: "Managed installation proof could not be prepared" }; }
  }

  if (recoveryProfile) recoveryPolicy.recordRecoveryAttempt(Date.now());
  currentState = "starting";
  // Begin a fresh auth-probe window for this attempt.
  startingSince = Date.now();
  authProbeDone = false;
  authExpired = false;
  sendStatus({ state: "starting" });

  const args = ["daemon", "start", ...profileArgs(active.name), ...(handoff ? ["--managed-handoff-stdin"] : [])];

  return new Promise((resolve) => {
    // A delayed proof/start from an earlier login must not start a process
    // after a newer credential or server selection superseded it.
    if (startupGeneration !== managedGeneration || startupSession !== managedSession || (managedStartupGuard && !managedStartupGuard()) || activeProfile?.name !== active.name) {
      resolve({ success: false, error: "Managed startup was superseded" });
      return;
    }
    // Shutdown may start while CLI/profile/health preflight is awaiting.
    if (daemonQuitting) {
      resolve({ success: false, error: "Desktop is quitting" });
      return;
    }
    const child = execFile(
      bin,
      args,
      { timeout: DAEMON_START_EXEC_TIMEOUT_MS, env: desktopSpawnEnv() },
      (err) => {
        if (err) {
          currentState = "stopped";
          sendStatus({ state: "stopped" });
          resolve({ success: false, error: err.message });
          return;
        }
        // Stay in "starting" until pollOnce confirms /health — the CLI
        // returning 0 only means the supervisor was spawned, not that the
        // daemon process is already listening.
        scheduleStatusRefresh();
        resolve({ success: true });
      },
    );
    if (handoff) {
      child.stdin?.on("error", () => undefined);
      child.stdin?.end(handoff);
    }
  });
}

/**
 * Fresh boundary preflight for stop/restart: read the active profile's CURRENT
 * /health and decide whether the daemon runs somewhere the app can't drive
 * (WSL2 etc.). Done per call rather than off the poll cache, so a lifecycle op
 * never shells out to a CLI that can't reach the daemon's process — even on
 * paths that didn't just poll (e.g. restart-on-user-switch in syncToken, which
 * calls restartDaemon directly). See #3916.
 */
async function lifecycleBlockedByForeignDaemon(): Promise<boolean> {
  const active = await ensureActiveProfile();
  if (!active) return false;
  return daemonLifecycleUnreachable(
    async () => (await fetchHealthAtPort(active.port))?.os,
    normalizeHostOS(process.platform),
  );
}

async function stopDaemon(): Promise<{ success: boolean; error?: string }> {
  // Central lifecycle guard: a daemon running in an environment we can't drive
  // (e.g. Linux in WSL2 behind a Windows desktop) can't be stopped by the
  // native CLI — it would act on the host process namespace and no-op, while
  // still flipping our state to "stopped". Bail as a successful no-op so every
  // caller (logout, quit, restart, the Runtime card) is covered in one place
  // rather than each remembering to check. Preflighted against live /health so
  // it holds even when no poll ran first. #3916.
  if (await lifecycleBlockedByForeignDaemon()) return { success: true };

  const bin = await resolveCliBinary();
  if (!bin) return { success: false, error: "multica CLI is not installed" };

  const active = await ensureActiveProfile();
  if (!active) return { success: true };
  currentState = "stopping";
  // An explicit stop is a clean reset — drop any pending auth-failure verdict.
  authExpired = false;
  startingSince = null;
  sendStatus({ state: "stopping" });

  const args = ["daemon", "stop", ...profileArgs(active.name)];

  return new Promise((resolve) => {
    execFile(bin, args, { timeout: 15_000 }, (err) => {
      if (err) {
        void pollOnce();
        resolve({ success: false, error: err.message });
        return;
      }
      resolve({ success: true });
      currentState = "stopped";
      sendStatus({ state: "stopped" });
    });
  });
}

async function restartDaemon(): Promise<{ success: boolean; error?: string }> {
  if (daemonQuitting || serverSwitchBlocked()) return { success: false, error: "Desktop is quitting or switching servers" };
  // Same central, live-preflighted guard as stopDaemon: we can neither stop nor
  // start a daemon we don't manage, so don't try (user-switch, reauth,
  // first-workspace, and any future restart caller all route through here).
  // #3916.
  if (await lifecycleBlockedByForeignDaemon()) return { success: true };
  try { if (managedPreparation) await managedPreparation; } catch { return { success: false, error: managedSetupError ?? "Managed setup is unavailable" }; }
  if (managedSetupError) return { success: false, error: managedSetupError };
  if (managedSession) {
    const active = await ensureActiveProfile();
    if (!active) return { success: false, error: "Managed profile is unavailable" };
    const config = await readProfileConfig(active.name);
    const existing = await fetchHealthAtPort(active.port);
    if (daemonStatusAlive(existing?.status) && typeof config.management_deployment_id !== "string") {
      return { success: false, error: "Existing legacy daemon has no authenticated management control; it remains unassociated" };
    }
    try { await managedSession.createHandoff(); } catch { return { success: false, error: "Managed restart proof could not be prepared" }; }
  }
  const stopResult = await stopDaemon();
  if (!stopResult.success) return stopResult;
  return startDaemon();
}

async function managedProfileRequiresConfirmedExit(profile: string): Promise<boolean> {
  const config = await readProfileConfig(profile, true);
  if (managedSession || managedPreparation || "management_deployment_id" in config || [...managedTransitionRecoveries.keys()].some((key) => key.startsWith(`${profile}:`))) return true;
  return (await readManagementPrivateFile(join(profileDir(profile), "management-control.json"), 4096)) !== null;
}

async function daemonPidIsConfirmedAbsent(profile: string, managed = false): Promise<boolean> {
  try {
    let raw: string | null = null;
    try { raw = await readFile(profilePidPath(profile), "utf-8"); }
    catch (error) { if (!(error && typeof error === "object" && "code" in error && error.code === "ENOENT")) throw error; }
    if (raw !== null) {
      const pid = parseDaemonPid(raw);
      if (pid === null || daemonProcessExists(pid)) return false;
    }
    if (managed) {
      const control = await readManagementPrivateFile(join(profileDir(profile), "management-control.json"), 4096);
      if (control !== null) {
        const value: unknown = JSON.parse(control.toString("utf8"));
        if (!value || typeof value !== "object" || !("version" in value) || value.version !== "1" || !("pid" in value) || typeof value.pid !== "number" || !Number.isSafeInteger(value.pid) || value.pid <= 0 || value.pid > 0xffffffff || daemonProcessExists(value.pid)) return false;
      }
    }
    return true;
  } catch (err) {
    console.warn("[daemon] recovery deferred: unable to verify daemon PID:", err);
    return false;
  }
}

async function attemptDaemonRecovery(active: ActiveProfile): Promise<void> {
  let managed: boolean;
  try { managed = await managedProfileRequiresConfirmedExit(active.name); }
  catch { return; }
  const startAllowed = () =>
    recoveryStartAllowed({
      desiredRunning: desiredDaemonRunning,
      externalDaemonObserved,
      expected: active,
      current: activeProfile,
    });
  const outcome = await runDaemonRecoveryAttempt({
    startAllowed,
    // A normal UI poll intentionally gives up after 2s. Before treating that
    // as process death, use an independent longer probe so a busy daemon is
    // not restarted merely because one health request was slow.
    confirmAlive: async () => {
      const health = await fetchHealthAtPort(
        active.port,
        RECOVERY_HEALTH_PROBE_TIMEOUT_MS,
      );
      if (!daemonStatusAlive(health?.status)) return false;
      externalDaemonObserved = isDaemonExternallyManaged(
        health?.os,
        normalizeHostOS(process.platform),
      );
      recoveryPolicy.observe({
        desiredRunning: desiredDaemonRunning,
        externalDaemonObserved,
        lifecycleBusy: lifecycleOperations.inProgress,
        state: health?.status === "running" ? "running" : "starting",
        now: Date.now(),
      });
      scheduleStatusRefresh();
      return true;
    },
    pidConfirmedAbsent: () => daemonPidIsConfirmedAbsent(active.name, managed),
    recordPidDeferral: () => { const fallback = recoveryPolicy.recordPidDeferral(Date.now()); return !managed && fallback; },
    recordPidAbsent: () => recoveryPolicy.recordPidAbsent(),
    // Force-kill leaves daemon.pid behind, and Windows can reuse the number
    // for another process. After three spaced deferrals, let the CLI recheck
    // health and attempt the start; an occupied port then fails safely into
    // the normal retry backoff and absolute budget.
    onPidFallback: () =>
      console.warn(
        "[daemon] daemon PID stayed unverifiable; proceeding through CLI safety checks",
      ),
    start: () => {
      if (managed && managedSession) {
        return reconcileManagedSession(active, managedSession).then((result) => ({ success: result.accepted }));
      }
      return startDaemon(active);
    },
    desiredRunning: () => desiredDaemonRunning,
    stop: () => stopDaemon(),
  });

  if (outcome.kind === "alive") {
    console.log("[daemon] recovery cancelled: longer health probe succeeded");
  } else if (outcome.kind === "pid_deferred") {
    console.warn(
      "[daemon] recovery deferred: daemon PID is still alive or could not be verified",
    );
  } else if (outcome.kind === "start_failed") {
    console.warn(
      `[daemon] recovery failed: ${outcome.error ?? "unknown error"}`,
    );
  }
}

function recoveryDecision(status: DaemonStatus) {
  return recoveryPolicy.observe({
    desiredRunning: desiredDaemonRunning,
    externalDaemonObserved,
    lifecycleBusy: lifecycleOperations.inProgress,
    state: status.state,
    now: Date.now(),
  });
}

async function pollOnce(): Promise<void> {
  if (daemonQuitting || statusPollInProgress) return;
  statusPollInProgress = true;
  try {
    const status = await fetchHealth();
    if (daemonQuitting) return;
    currentState = status.state;
    observeDaemonBoundary(status);
    const decision = recoveryDecision(status);
    sendStatus(
      decision === "pause" ? { ...status, state: "recovery_paused" } : status,
    );
    if (decision === "confirm") {
      const active = await ensureActiveProfile();
      if (active) {
        // Recovery can spend 10s confirming health plus 60s in the CLI start.
        // Do not hold the poll lock across it: startDaemon publishes
        // "starting" immediately, and subsequent polls keep that visible.
        void lifecycleOperations
          .runBackground(() => attemptDaemonRecovery(active))
          .catch((err) => {
            console.warn("[daemon] background recovery failed:", err);
          });
      }
    }
    if (pendingManagedScopeRefresh && managedSession && desiredDaemonRunning && Date.now() >= managedRetryAfter) {
      void refreshManagedScopes();
    }
    // Retry a deferred version-mismatch restart once the daemon drains. Route
    // it through the same singleflight guard as user and recovery operations.
    if (pendingVersionRestart && status.state === "running") {
      void lifecycleOperations
        .runBackground(() => ensureRunningDaemonVersionMatches())
        .catch((err) => {
          console.warn("[daemon] deferred version restart failed:", err);
        });
    }
  } finally {
    statusPollInProgress = false;
  }
}

function startPolling(): void {
  if (daemonQuitting || statusPollTimer) return;
  void pollOnce();
  statusPollTimer = setInterval(() => void pollOnce(), POLL_INTERVAL_MS);
}

/**
 * Ensures the CLI binary is available, then transitions into the normal
 * stopped/running state machine. Called once at startup and again on
 * user-triggered `daemon:retry-install`.
 */
async function bootstrapCli(): Promise<void> {
  const bin = await resolveCliBinary();
  if (!bin) {
    currentState = "cli_not_found";
    sendStatus({ state: "cli_not_found" });
    return;
  }
  currentState = "stopped";
  sendStatus({ state: "stopped" });
  startPolling();
}

function stopPolling(): void {
  if (statusPollTimer) {
    clearInterval(statusPollTimer);
    statusPollTimer = null;
  }
}

const LOG_TAIL_INITIAL_WINDOW_BYTES = 32 * 1024;
const LOG_TAIL_INITIAL_LINES = 200;
const LOG_TAIL_POLL_MS = 500;

async function readLogRange(
  path: string,
  startAt: number,
  length: number,
): Promise<string> {
  const handle = await open(path, "r");
  try {
    const buffer = Buffer.alloc(length);
    const { bytesRead } = await handle.read(buffer, 0, length, startAt);
    return buffer.subarray(0, bytesRead).toString("utf-8");
  } finally {
    await handle.close();
  }
}

function sendLines(win: BrowserWindow, text: string): void {
  const lines = text.split("\n").filter((line) => line.length > 0);
  for (const line of lines) {
    win.webContents.send("daemon:log-line", line);
  }
}

// Cross-platform tail -f replacement: read the tail of the file once, then
// poll its stat with fs.watchFile and forward any new bytes since the last
// known offset. watchFile works on macOS, Linux, and Windows; spawn("tail")
// would silently fail on Windows.
function startLogTail(win: BrowserWindow, retryCount = 0): void {
  stopLogTail();

  void ensureActiveProfile().then(async (active) => {
    // Before the renderer reports its apiUrl there is no Desktop-owned profile
    // yet, and therefore no log file of ours to tail. Retry rather than reach
    // for the default profile's log.
    const logPath = active ? profileLogPath(active.name) : null;
    if (!logPath || !existsSync(logPath)) {
      if (retryCount < LOG_TAIL_MAX_RETRIES) {
        setTimeout(() => startLogTail(win, retryCount + 1), LOG_TAIL_RETRY_MS);
      }
      return;
    }

    let position = 0;
    try {
      const initialStats = await stat(logPath);
      const windowBytes = Math.min(
        initialStats.size,
        LOG_TAIL_INITIAL_WINDOW_BYTES,
      );
      const startAt = initialStats.size - windowBytes;
      if (windowBytes > 0) {
        const text = await readLogRange(logPath, startAt, windowBytes);
        const lines = text
          .split("\n")
          .filter((line) => line.length > 0)
          .slice(-LOG_TAIL_INITIAL_LINES);
        for (const line of lines) {
          win.webContents.send("daemon:log-line", line);
        }
      }
      position = initialStats.size;
    } catch (err) {
      console.warn("[daemon] log tail initial read failed:", err);
      return;
    }

    const listener: StatsListener = (curr) => {
      const target = getMainWindow();
      if (!target) return;
      // File rotated/truncated — restart from the new beginning.
      if (curr.size < position) position = 0;
      if (curr.size === position) return;
      const from = position;
      const length = curr.size - from;
      position = curr.size;
      readLogRange(logPath, from, length)
        .then((text) => sendLines(target, text))
        .catch((err) => {
          console.warn("[daemon] log tail read failed:", err);
        });
    };

    watchFile(logPath, { interval: LOG_TAIL_POLL_MS }, listener);
    logTailWatcher = { path: logPath, listener };
  });
}

function stopLogTail(): void {
  if (logTailWatcher) {
    unwatchFile(logTailWatcher.path, logTailWatcher.listener);
    logTailWatcher = null;
  }
}

let serverSwitchBlocked = () => false;
const pendingCredentialOperations = new Set<Promise<unknown>>();

/** Drain already-authorized writes before clearing the old profile. */
export async function resetDaemonForServerSwitch(): Promise<void> {
  setDesiredDaemonRunning(false, true);
  await Promise.allSettled([...pendingCredentialOperations]);
  await lifecycleOperations.runForeground(async () => {
    const active = await ensureActiveProfile();
    if (!active) return;
    if (await lifecycleBlockedByForeignDaemon()) throw new Error("Stop the externally managed daemon before switching servers");
    const running = await fetchHealthAtPort(active.port);
    if (!daemonStatusAlive(running?.status) && await daemonPidIsConfirmedAbsent(active.name, await managedProfileRequiresConfirmedExit(active.name))) { await clearToken(true); return; }
    const stopped = await stopDaemon();
    if (!stopped.success) throw new Error(stopped.error || "Could not stop the previous server daemon");
    await clearToken(true);
  });
}

export function setupDaemonManager(
  windowGetter: () => BrowserWindow | null,
  isServerSwitchBlocked: () => boolean = () => false,
): void {
  serverSwitchBlocked = isServerSwitchBlocked;
  getMainWindow = windowGetter;

  ipcMain.handle("daemon:set-target-api-url", async (_e, url: string) => {
    if (serverSwitchBlocked()) throw new Error("Server switch in progress");
    const normalized = url || null;
    if (targetApiBaseUrl !== normalized) {
      console.log(`[daemon] target API URL set to ${normalized ?? "(none)"}`);
      setDesiredDaemonRunning(false);
      resetManagedSession();
      targetApiBaseUrl = normalized;
      invalidateActiveProfile();
      await pollOnce();
    }
  });
  ipcMain.handle("daemon:start", () => {
    if (serverSwitchBlocked()) throw new Error("Server switch in progress");
    externalDaemonObserved = false;
    setDesiredDaemonRunning(true, true);
    return lifecycleOperations.runForeground(() => startDaemon());
  });
  ipcMain.handle("daemon:stop", () => {
    pendingManagedScopeRefresh = false; managedPendingReason = null; managedTransitionRecoveries.clear();
    setDesiredDaemonRunning(false, true);
    return lifecycleOperations.runForeground(() => stopDaemon());
  });
  ipcMain.handle("daemon:restart", () => {
    if (serverSwitchBlocked()) throw new Error("Server switch in progress");
    externalDaemonObserved = false;
    setDesiredDaemonRunning(true, true);
    return lifecycleOperations.runForeground(() => restartDaemon());
  });
  ipcMain.handle("daemon:get-status", () => fetchHealth());
  ipcMain.handle("daemon:get-installation-metadata", () => managedSession?.metadataProof ?? null);
  ipcMain.handle("daemon:end-installation-session", () => resetManagedSession());
  ipcMain.handle("daemon:refresh-managed-workspaces", () => { setDesiredDaemonRunning(true); return refreshManagedScopes(); });
  ipcMain.handle("daemon:probe-runtimes", () => probeLocalRuntimes());
  // The host's OS name, available regardless of daemon state. The Runtimes
  // page uses it as a fallback identity for "this machine" when no
  // app-managed daemon is reporting a device name (e.g. the daemon runs
  // out-of-band in WSL2). See desktop-runtimes-page.tsx.
  ipcMain.handle("daemon:get-host-name", () => hostname());
  ipcMain.handle(
    "daemon:sync-token",
    async (_event, token: string, userId: string) => {
      if (serverSwitchBlocked()) throw new Error("Server switch in progress");
      const operation = (async () => {
        const result = await syncToken(token, userId);
        if ((result.userChanged || managedSession !== null) && !serverSwitchBlocked()) {
          await restartDaemonAfterUserSwitch(result.active);
        }
      })();
      pendingCredentialOperations.add(operation);
      try { await operation; } finally { pendingCredentialOperations.delete(operation); }
    },
  );
  ipcMain.handle("daemon:clear-token", () => {
    setDesiredDaemonRunning(false, true);
    return clearToken();
  });
  ipcMain.handle(
    "daemon:reauthenticate",
    async (_event, token: string, userId: string): Promise<ReauthResult> => {
      if (serverSwitchBlocked()) throw new Error("Server switch in progress");
      setDesiredDaemonRunning(true, true);
      return lifecycleOperations.runForeground(() => {
        if (serverSwitchBlocked()) throw new Error("Server switch in progress");
        return reauthenticate(token, userId);
      });
    },
  );
  ipcMain.handle("daemon:is-cli-installed", async () => {
    const bin = await resolveCliBinary();
    return bin !== null;
  });
  ipcMain.handle("daemon:retry-install", async () => {
    cachedCliBinary = undefined;
    cliResolvePromise = null;
    // A retry-install may land a new CLI at a different version; drop the
    // cached version string so the next check re-reads the binary.
    cachedCliBinaryVersion = undefined;
    await lifecycleOperations.runForeground(() => bootstrapCli());
  });
  ipcMain.handle("daemon:get-prefs", () => loadPrefs());
  ipcMain.handle(
    "daemon:set-prefs",
    (_event, prefs: Partial<DaemonPrefs>) =>
      loadPrefs().then((cur) => {
        const merged = { ...cur, ...prefs };
        // Changing the preference still affects the next logged-in launch; it
        // does not start/stop the current session. The Settings copy explicitly
        // states that a launched daemon is supervised while Desktop stays open.
        return savePrefs(merged).then(() => merged);
      }),
  );
  ipcMain.handle("daemon:auto-start", async () => {
    const prefs = await loadPrefs();
    setDesiredDaemonRunning(prefs.autoStart);
    if (!prefs.autoStart) return;
    // Login auto-start is emitted once per session. Queue it behind bootstrap
    // instead of dropping it like a retryable poll operation.
    await lifecycleOperations.runForeground(async () => {
      const bin = await resolveCliBinary();
      if (!bin) return;
      const health = await fetchHealth();
      observeDaemonBoundary(health);
      if (health.state === "running") {
        // Daemon is up but may be running an older CLI than the one we just
        // bundled. Restart it so the new binary actually takes effect.
        await ensureRunningDaemonVersionMatches();
        return;
      }
      await startDaemon();
    });
  });

  ipcMain.on("daemon:start-log-stream", () => {
    const win = getMainWindow();
    if (win) startLogTail(win);
  });

  ipcMain.on("daemon:stop-log-stream", () => {
    stopLogTail();
  });

  // Reveal the daemon's log file in the user's default editor / Console
  // app. Acts as the escape hatch when the in-app log viewer isn't enough
  // (full history, complex search, copy-to-clipboard at scale).
  ipcMain.handle("daemon:open-log-file", async () => {
    const active = await ensureActiveProfile();
    const logPath = active ? profileLogPath(active.name) : null;
    if (!logPath || !existsSync(logPath)) {
      return { success: false, error: "Log file not found yet" };
    }
    // shell.openPath returns "" on success, error string on failure.
    const error = await shell.openPath(logPath);
    return error === "" ? { success: true } : { success: false, error };
  });

  // First-run CLI install kicks off here. Status bar shows "Setting up…"
  // until the managed binary is on disk (instant on subsequent launches).
  currentState = "installing_cli";
  sendStatus({ state: "installing_cli" });
  void lifecycleOperations.runBackground(() => bootstrapCli());

  app.on(
    "before-quit",
    createDaemonQuitHandler({
      stopBackgroundActivity: () => {
        daemonQuitting = true;
        setDesiredDaemonRunning(false);
        stopPolling();
        stopLogTail();
      },
      runExclusive: (operation) => lifecycleOperations.runForeground(operation),
      loadPrefs,
      // stopDaemon owns the guard for externally-managed daemons (WSL2 etc.).
      stopDaemon,
      quit: () => app.quit(),
      warn: (message, error) => console.warn(message, error),
    }),
  );
}
