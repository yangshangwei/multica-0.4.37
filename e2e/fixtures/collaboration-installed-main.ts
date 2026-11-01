import { execFile, spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { randomUUID } from "node:crypto";
import { realpathSync } from "node:fs";
import path from "node:path";
import { createInterface } from "node:readline";
import { nativeAcceptanceHome, nativeInstalledLayout } from "./collaboration-installed-layout.mjs";

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

/** Credentials travel only in stdin to one fresh container. Docker arguments,
 * replies, reports and Playwright traces contain no fixture password or token. */
export class InstalledMainClient {
  readonly containerName = `multica-collaboration-main-${randomUUID().slice(0, 12)}`;
  readonly packageRoot: string;
  private readonly child: ChildProcessWithoutNullStreams;
  private sequence = 0;
  private readonly pending = new Map<number, { resolve(value: unknown): void; reject(error: Error): void; timer: NodeJS.Timeout }>();
  private ended = false;
  private stderrBytes = 0;
  private ignoredOutputLines = 0;
  private nativeMode = false;

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
    this.child = spawn("docker", ["run", "--rm", "--init", "-i", "--name", this.containerName,
      "--add-host", "host.docker.internal:host-gateway", "--volume", `${repo}:/workspace:ro`,
      ...packageMount, "--workdir", "/workspace", image,
      "xvfb-run", "-a", "-s", "-screen 0 1600x1400x24", "node", "/workspace/e2e/fixtures/collaboration-installed-main.mjs"],
    { stdio: "pipe" });
    this.attachProtocolHandlers();
  }

  private attachProtocolHandlers() {
    // Native logging is intentionally counted, never forwarded or attached.
    this.child.stderr.on("data", (chunk: Buffer) => { this.stderrBytes += chunk.length; });
    const lines = createInterface({ input: this.child.stdout });
    lines.on("line", (line) => {
      const prefix = "MULTICA_INSTALLED_REPLY ";
      if (!line.startsWith(prefix)) { this.ignoredOutputLines += 1; return; }
      let reply: { id: number; value?: unknown; error?: string };
      try { reply = JSON.parse(line.slice(prefix.length)); }
      catch { this.rejectAll(new Error("Installed Main emitted an invalid protocol reply")); return; }
      const pending = this.pending.get(reply.id);
      if (!pending) return;
      clearTimeout(pending.timer);
      this.pending.delete(reply.id);
      if (reply.error) pending.reject(new Error(reply.error));
      else pending.resolve(reply.value);
    });
    this.child.on("error", () => this.rejectAll(new Error("Could not launch the configured installed Main container")));
    this.child.on("close", (code, signal) => {
      this.ended = true;
      this.rejectAll(new Error(`Installed Main container exited: code=${code}, signal=${signal}, private stderr bytes=${this.stderrBytes}`));
    });
  }

  async command<T>(command: string, input: Record<string, unknown> = {}, timeout = 120_000): Promise<T> {
    if (this.ended) throw new Error("Installed Main container is no longer running");
    const id = ++this.sequence;
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error(`Installed Main ${command} exceeded ${timeout}ms`)); }, timeout);
      this.pending.set(id, { resolve: (value) => resolve(value as T), reject, timer });
      this.child.stdin.write(`${JSON.stringify({ id, command, ...input })}\n`, (error) => {
        if (error) { clearTimeout(timer); this.pending.delete(id); reject(new Error("Installed Main stdin command could not be delivered")); }
      });
    });
  }

  get transportEvidence() { return { containerName: this.containerName, ignoredOutputLines: this.ignoredOutputLines,
    privateStderrBytes: this.stderrBytes, credentialTransport: "stdin only", homeOverride: this.nativeMode, nativeIPCStubs: false }; }

  async close() {
    if (this.ended) return;
    const waitForClose = () => new Promise<void>((resolve) => {
      if (this.ended) { resolve(); return; }
      const timer = setTimeout(() => { this.child.off("close", closed); resolve(); }, 10_000);
      const closed = () => { clearTimeout(timer); resolve(); };
      this.child.once("close", closed);
    });
    try { await this.command("close", {}, 30_000); }
    finally {
      this.child.stdin.end();
      await waitForClose();
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
    }
  }

  private rejectAll(error: Error) {
    for (const pending of this.pending.values()) { clearTimeout(pending.timer); pending.reject(error); }
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
