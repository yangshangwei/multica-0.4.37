import { execFile, spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { createInterface } from "node:readline";
import { nativeAcceptanceHome, nativeInstalledLayout } from "./collaboration-installed-layout.mjs";

/** One synthetic host identity per disposable container; never read the host identity. */
export function createInstalledMachineIdentity(root: string) {
  const parent = path.join(realpathSync(root), ".omx", "private", "installed-main-identities");
  mkdirSync(parent, { recursive: true, mode: 0o700 });
  const directory = mkdtempSync(path.join(parent, "container-"));
  const file = path.join(directory, "machine-id");
  const discoveryCLI = path.join(directory, "claude");
  try {
    writeFileSync(file, `${randomUUID().replaceAll("-", "")}\n`, { mode: 0o444, flag: "wx" });
    writeFileSync(discoveryCLI, "#!/bin/sh\nif [ \"$#\" -eq 1 ] && [ \"$1\" = --version ]; then\n  printf '2.1.5 (Claude Code)\\n'\n  exit 0\nfi\nprintf 'unexpected agent execution\\n' >> \"${MULTICA_INSTALLED_DISCOVERY_MARKER:?explicit test marker required}\"\nexit 97\n", { mode: 0o555, flag: "wx" });
  }
  catch (error) { rmSync(directory, { recursive: true, force: true }); throw error; }
  let removed = false;
  return { mount: `${realpathSync(file)}:/etc/machine-id:ro`, discoveryMount: `${realpathSync(discoveryCLI)}:/acceptance/discovery/claude:ro`, cleanup() {
    if (removed) return;
    rmSync(directory, { recursive: true, force: true });
    removed = true;
  } };
}

export interface InstalledMainMetadata {
  main: { packaged: boolean; appPath: string; name: string; version: string; executable: string;
    home: string; userData: string; platform: string; arch: string; electron: string };
  native: { appInfo: { version: string; os: string }; daemon: { state: string }; preferences: { autoStart: boolean; autoStop: boolean };
    cliInstalled: boolean; updater: { automaticUpdates: boolean }; config: { ok: boolean; source: string; config: { apiUrl: string; appUrl: string } } };
  tabs: { activeScope: string; activeWorkspaceSlug: string | null; accountTabs: { id: string; url: string }[] } | null;
  artifactSha256: string; executableSha256: string; rendererUrl: string; errors: string[];
  accountRequests: { path: string; method: string; workspaceId: string | null; workspaceSlug: string | null;
    clientVersion: string | null; clientOS: string | null }[];
  responses: { path: string; status: number; method: string }[];
  bundledCLI?: { version: string; sha256: string; command: string[] };
  installation?: { kind: "deb"; packageName: string; version: string; architecture: string; status: string;
    debSha256: string; executableSha256: string; asarSha256: string; cliSha256: string;
    desktopEntry: string; desktopLauncher: string; cliPathUnclaimed: boolean; standaloneCliPreserved: boolean;
    packageManager: { command: string[]; exitCode: number; stdout: string; stderr: string } };
}

export interface InstalledMainWrite<Receipt, Input = unknown> { receipt: Receipt; requestId: string; input: Input; resultText: string }

const installedCommands = new Set(["install", "init", "login", "navigate", "read", "control", "comment", "cancel", "history",
  "host-records", "handoff", "workflow", "reload", "restart", "capture", "status", "close"]);
const installedPhases = new Set(["command", "restart.close", "restart.launch", "restart.refresh", "restart.metadata",
  "close.stop-daemon", "close.application", "eof.stop-daemon", "eof.application"]);
const installedOutcomes = new Set(["started", "completed", "failed", "timeout", "write-failed", "process-exit", "process-error"]);
export interface InstalledMainTimelineEvent {
  id: number;
  command: string;
  phase: string;
  outcome: string;
  elapsedMs: number;
  source: "client" | "controller";
}

/** Project only fixed-schema metadata; even a malformed reply must not expose payload or errors. */
export function installedTimelineEvent(value: unknown, source: InstalledMainTimelineEvent["source"]): InstalledMainTimelineEvent | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const entry = value as Record<string, unknown>;
  if (!Number.isSafeInteger(entry.id) || (entry.id as number) < 0 || typeof entry.command !== "string"
    || (!installedCommands.has(entry.command) && entry.command !== "unknown") || typeof entry.phase !== "string"
    || !installedPhases.has(entry.phase) || typeof entry.outcome !== "string" || !installedOutcomes.has(entry.outcome)
    || typeof entry.elapsedMs !== "number" || !Number.isFinite(entry.elapsedMs) || entry.elapsedMs < 0) return null;
  return { id: entry.id as number, command: entry.command, phase: entry.phase, outcome: entry.outcome,
    elapsedMs: entry.elapsedMs, source };
}

/** Every independent cleanup runs; earlier failures retain their original objects and order. */
export async function finishInstalledMainJourney(failures: unknown[], cleanups: (() => Promise<unknown>)[]): Promise<void> {
  const retained = [...failures];
  for (const cleanup of cleanups) {
    try { await cleanup(); } catch (error) { retained.push(error); }
  }
  if (retained.length === 1) throw retained[0];
  if (retained.length > 1) throw new AggregateError(retained, "Installed Main journey and cleanup failures", { cause: retained[0] });
}

/** Credentials travel only in stdin to one fresh container. Docker arguments,
 * replies, reports and Playwright traces contain no fixture password or token. */
export class InstalledMainClient {
  readonly containerName = `multica-collaboration-main-${randomUUID().slice(0, 12)}`;
  readonly packageRoot: string;
  private readonly child: ChildProcessWithoutNullStreams;
  private sequence = 0;
  private readonly pending = new Map<number, { command: string; resolve(value: unknown): void; reject(error: Error): void; timer: NodeJS.Timeout }>();
  private readonly timeline: InstalledMainTimelineEvent[] = [];
  private timelineTruncated = false;
  private readonly timelineStarted = performance.now();
  private ended = false;
  private stderrBytes = 0;
  private ignoredOutputLines = 0;
  private nativeMode = false;
  private machineIdentity?: ReturnType<typeof createInstalledMachineIdentity>;

  constructor(options: { image?: string; executablePath?: string; acceptanceHome?: string } = {}) {
    if (options.executablePath) {
      const layout = nativeInstalledLayout(options.executablePath);
      const acceptanceHome = nativeAcceptanceHome(options.acceptanceHome);
      this.nativeMode = true;
      this.packageRoot = layout.resources;
      const repo = realpathSync(path.resolve(import.meta.dirname, "../.."));
      this.child = spawn(process.execPath, [path.join(repo, "e2e/fixtures/collaboration-installed-main.mjs")], {
        stdio: "pipe",
        detached: process.platform !== "win32",
        env: { ...process.env, COLLABORATION_INSTALLED_MAIN_EXECUTABLE: realpathSync(options.executablePath),
          MULTICA_PACKAGED_ACCEPTANCE: "1", MULTICA_PACKAGED_ACCEPTANCE_HOME: acceptanceHome },
      });
      this.attachProtocolHandlers();
      return;
    }
    const appDir = process.env.COLLABORATION_INSTALLED_APP_DIR;
    const appVolume = process.env.COLLABORATION_INSTALLED_APP_VOLUME;
    const deb = process.env.COLLABORATION_INSTALLED_DEB;
    const debSha256 = process.env.COLLABORATION_INSTALLED_DEB_SHA256;
    const image = options.image ?? process.env.COLLABORATION_INSTALLED_IMAGE;
    if ((!appDir && !appVolume && !deb) || !image) throw new Error("Installed Main requires an explicit task-owned Docker image and directory package or .deb");
    if (deb && (appDir || appVolume)) throw new Error("Select either an installed .deb or a directory package, never both");
    if (deb && (!deb.endsWith(".deb") || !/^[0-9a-f]{64}$/.test(debSha256 ?? ""))) throw new Error("Installed Main requires an explicitly pinned .deb SHA-256");
    if (appVolume && !/^multica-gap-full-main-package-[a-z0-9-]+$/.test(appVolume)) throw new Error("Installed Main requires a named task-owned package volume");
    this.packageRoot = deb ? "/opt/Multica" : "/opt/multica";
    const repo = realpathSync(path.resolve(import.meta.dirname, "../.."));
    const packageMount = deb
      ? ["--volume", `${realpathSync(deb)}:/acceptance/package.deb:ro`, "--env", `COLLABORATION_INSTALLED_DEB_SHA256=${debSha256}`]
      : appVolume
        ? ["--mount", `type=volume,source=${appVolume},target=/opt/multica,readonly`]
        : ["--volume", `${realpathSync(appDir!)}:/opt/multica:ro`];
    this.machineIdentity = createInstalledMachineIdentity(repo);
    try {
      this.child = spawn("docker", ["run", "--rm", "--init", "-i", "--name", this.containerName,
      "--volume", this.machineIdentity.mount, "--volume", this.machineIdentity.discoveryMount,
      "--add-host", "host.docker.internal:host-gateway", "--volume", `${repo}:/workspace:ro`,
      ...packageMount, "--workdir", "/workspace", image,
      "xvfb-run", "-a", "-s", "-screen 0 1600x1400x24", "node", "/workspace/e2e/fixtures/collaboration-installed-main.mjs"],
    { stdio: "pipe" });
    } catch (error) { this.machineIdentity.cleanup(); throw error; }
    this.attachProtocolHandlers();
  }

  private attachProtocolHandlers() {
    // Native logging is intentionally counted, never forwarded or attached.
    this.child.stderr.on("data", (chunk: Buffer) => { this.stderrBytes += chunk.length; });
    const lines = createInterface({ input: this.child.stdout });
    lines.on("line", (line) => {
      const eventPrefix = "MULTICA_INSTALLED_EVENT ";
      if (line.startsWith(eventPrefix)) {
        try {
          const event = installedTimelineEvent(JSON.parse(line.slice(eventPrefix.length)), "controller");
          if (event) this.appendTimeline(event);
          else this.ignoredOutputLines += 1;
        } catch { this.ignoredOutputLines += 1; }
        return;
      }
      const prefix = "MULTICA_INSTALLED_REPLY ";
      if (!line.startsWith(prefix)) { this.ignoredOutputLines += 1; return; }
      let reply: { id: number; value?: unknown; error?: string };
      try { reply = JSON.parse(line.slice(prefix.length)); }
      catch { this.rejectAll(new Error("Installed Main emitted an invalid protocol reply")); return; }
      const pending = this.pending.get(reply.id);
      if (!pending) return;
      clearTimeout(pending.timer);
      this.pending.delete(reply.id);
      this.recordCommand(reply.id, pending.command, reply.error ? "failed" : "completed");
      if (reply.error) pending.reject(new Error(reply.error));
      else pending.resolve(reply.value);
    });
    this.child.on("error", () => {
      this.machineIdentity?.cleanup();
      this.rejectAll(new Error("Could not launch the configured installed Main container"), "process-error");
    });
    this.child.on("close", (code, signal) => {
      this.ended = true;
      this.rejectAll(new Error(`Installed Main container exited: code=${code}, signal=${signal}, private stderr bytes=${this.stderrBytes}`), "process-exit");
      if (this.machineIdentity) void this.cleanupStoppedContainerIdentity();
    });
  }

  async command<T>(command: string, input: Record<string, unknown> = {}, timeout = 120_000): Promise<T> {
    if (this.ended) throw new Error("Installed Main container is no longer running");
    const id = ++this.sequence;
    this.recordCommand(id, command, "started");
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        this.recordCommand(id, command, "timeout");
        reject(new Error(`Installed Main ${command} exceeded ${timeout}ms`));
      }, timeout);
      this.pending.set(id, { command, resolve: (value) => resolve(value as T), reject, timer });
      this.child.stdin.write(`${JSON.stringify({ id, command, ...input })}\n`, (error) => {
        if (error && this.pending.delete(id)) {
          clearTimeout(timer);
          this.recordCommand(id, command, "write-failed");
          reject(new Error("Installed Main stdin command could not be delivered"));
        }
      });
    });
  }

  get transportEvidence() { return { containerName: this.containerName, ignoredOutputLines: this.ignoredOutputLines,
    privateStderrBytes: this.stderrBytes, credentialTransport: "stdin only", homeOverride: this.nativeMode, nativeIPCStubs: false,
    commandTimeline: this.timeline.map((event) => ({ ...event })), timelineTruncated: this.timelineTruncated }; }

  private appendTimeline(event: InstalledMainTimelineEvent) {
    // Preserve the first failure context, rather than evicting it during cleanup.
    if (this.timeline.length >= 512) { this.timelineTruncated = true; return; }
    this.timeline.push(event);
  }

  private recordCommand(id: number, command: string, outcome: string) {
    const event = installedTimelineEvent({ id, command: installedCommands.has(command) ? command : "unknown",
      phase: "command", outcome, elapsedMs: performance.now() - this.timelineStarted }, "client");
    if (event) this.appendTimeline(event);
  }

  private async cleanupStoppedContainerIdentity(stopRunning = false) {
    if (!this.machineIdentity) return;
    const inspect = () => new Promise<"stopped" | "running" | "unknown">((resolve) => {
      execFile("docker", ["inspect", "--format", "{{.State.Running}}", this.containerName], { timeout: 10_000 }, (error, stdout, stderr) => {
        // A disconnected Docker client is not proof that its container stopped.
        if ((!error && stdout.trim() === "false") || (error && /No such (object|container)/i.test(stderr))) resolve("stopped");
        else resolve(!error && stdout.trim() === "true" ? "running" : "unknown");
      });
    });
    let state = await inspect();
    if (state === "running" && stopRunning) {
      await new Promise<void>((resolve) => {
        execFile("docker", ["stop", "--time", "5", this.containerName], { timeout: 10_000 }, () => resolve());
      });
      state = await inspect();
    }
    if (state === "stopped") this.machineIdentity.cleanup();
    // Unknown ownership/daemon reachability retains the mounted file for root cleanup.
  }

  async close() {
    if (this.ended) {
      await this.cleanupStoppedContainerIdentity(true);
      return;
    }
    const waitForClose = () => new Promise<void>((resolve) => {
      if (this.ended) { resolve(); return; }
      const timer = setTimeout(() => { this.child.off("close", closed); resolve(); }, 10_000);
      const closed = () => { clearTimeout(timer); resolve(); };
      this.child.once("close", closed);
    });
    const failures: unknown[] = [];
    try { await this.command("close", {}, 30_000); } catch (error) { failures.push(error); }
    await finishInstalledMainJourney(failures, [
      async () => { this.child.stdin.end(); await waitForClose(); },
      async () => {
        if (!this.ended) {
          if (this.nativeMode) {
            const pid = this.child.pid;
            if (!pid) throw new Error("Installed Main controller has no owned process ID");
            // Only the controller this instance spawned, including its Electron
            // children. Never use a process name or touch another installed app.
            if (process.platform === "win32") {
              await new Promise<void>((resolve, reject) => {
                execFile("taskkill", ["/PID", String(pid), "/T", "/F"], { timeout: 10_000 }, (error) => {
                  if (error && !this.ended) reject(new Error("Could not terminate this installed Main controller"));
                  else resolve();
                });
              });
            } else {
              try { process.kill(-pid, "SIGTERM"); } catch (error) { if ((error as NodeJS.ErrnoException).code !== "ESRCH") throw error; }
            }
            await waitForClose();
            if (!this.ended) throw new Error("Installed Main controller did not exit after termination");
          } else {
            // Stop only the UUID-named container this instance started. The Docker
            // client can outlive the container, so killing its PID is insufficient.
            await new Promise<void>((resolve) => {
              const stop = spawn("docker", ["stop", "--time", "5", this.containerName], { stdio: "ignore" });
              stop.once("error", () => resolve());
              stop.once("close", () => resolve());
            });
          }
        }
      },
      () => this.cleanupStoppedContainerIdentity(true),
    ]);
  }

  private rejectAll(error: Error, outcome = "failed") {
    for (const [id, pending] of this.pending) {
      clearTimeout(pending.timer);
      this.recordCommand(id, pending.command, outcome);
      pending.reject(error);
    }
    this.pending.clear();
  }
}

export function dockerTaskURL(value: string) {
  const url = new URL(value);
  if (!["127.0.0.1", "localhost"].includes(url.hostname) || url.username || url.password) {
    throw new Error("Installed Main requires the explicit loopback task service URLs");
  }
  url.hostname = "host.docker.internal";
  return url.origin;
}
