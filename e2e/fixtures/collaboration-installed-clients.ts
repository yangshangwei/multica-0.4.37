import { execFile, spawn } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { statSync } from "node:fs";
import { isAbsolute } from "node:path";
import { createInterface } from "node:readline";
import { promisify } from "node:util";
import { expect } from "@playwright/test";

export const installedCLIImage = "sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104";
export const installedCLIVersion = { version: "v0.6.7", commit: "9caea35808afc8d05fb6135bfc74d4bfb0b45cf1",
  date: "2026-10-17T05:56:15Z", go: "go1.26.9", arch: "amd64", os: "linux" };
const execute = promisify(execFile);
const digest = (bytes: string | Buffer) => createHash("sha256").update(bytes).digest("hex");

interface RelayFrame {
  kind: "request" | "result";
  id?: number; method?: string; path?: string; headers?: Record<string, string[]>; body?: string;
  exit_code?: number; stdout?: string; stderr?: string;
}
export interface InstalledCLIRequest {
  method: string; path: string; status: number; body_sha256: string;
  client_version: string; client_platform: string; client_os: string;
}

export async function verifyInstalledCLI() {
  const command = ["run", "--rm", "--pull", "never", "--read-only", "--network", "none", "--platform", "linux/amd64",
    "--entrypoint", "/app/multica", installedCLIImage, "version", "--output", "json"];
  const { stdout } = await execute("docker", command, { timeout: 30_000 });
  const version: unknown = JSON.parse(stdout);
  expect(version).toEqual(installedCLIVersion);
  return { image: installedCLIImage, binary: "/app/multica", version, command,
    scope: "Existing local offline OCI package bytes; not publisher signature or installed Desktop" };
}

/** Docker has no network interface beyond loopback. Only exact expected legacy
 * paths leave the container over private stdio and reach the task-owned API. */
export async function invokeInstalledCLI(input: { apiUrl: string; token: string; workspaceId: string; args: string[];
  expected: { method: string; path: string } }) {
  const api = new URL(input.apiUrl);
  if (api.protocol !== "http:" || !["127.0.0.1", "localhost"].includes(api.hostname) || api.pathname !== "/") {
    throw new Error("Installed CLI requests require the task-owned loopback API origin");
  }
  const relay = process.env.COLLABORATION_INSTALLED_CLI_RELAY;
  if (!relay || !isAbsolute(relay) || !statSync(relay).isFile()) throw new Error("The explicit test-only Linux relay is required");
  if (input.args[0] !== "issue" || input.args.some((arg) => arg.startsWith("--server") || arg.startsWith("--profile"))) {
    throw new Error("The installed-client slice allows only its explicit issue command");
  }
  const name = `multica-installed-cli-${randomUUID()}`;
  const stage = `${name}-stage`;
  const volume = `${name}-relay`;
  await execute("docker", ["volume", "create", volume], { timeout: 20_000 });
  const args = ["create", "--pull", "never", "--name", name, "--read-only", "--network", "none", "--platform", "linux/amd64",
    "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--tmpfs", "/tmp:rw,nosuid,nodev,size=16m",
    "--mount", `type=volume,src=${volume},dst=/fixture,readonly`, "--entrypoint", "/fixture/relay", "-i", installedCLIImage];
  try {
    // Docker rejects cp into a readonly rootfs. Copy into an owned writable
    // volume before the real runtime mounts those exact test bytes readonly.
    await execute("docker", ["create", "--pull", "never", "--name", stage, "--read-only", "--network", "none", "--platform", "linux/amd64",
      "--mount", `type=volume,src=${volume},dst=/fixture`, "--entrypoint", "/bin/true", installedCLIImage], { timeout: 20_000 });
    await execute("docker", ["cp", relay, `${stage}:/fixture/relay`], { timeout: 20_000 });
    await execute("docker", ["rm", stage], { timeout: 10_000 });
    await execute("docker", args, { timeout: 20_000 });
  } catch (error) {
    await execute("docker", ["rm", "--force", name, stage], { timeout: 10_000 }).catch(() => undefined);
    await execute("docker", ["volume", "rm", volume], { timeout: 10_000 }).catch(() => undefined);
    throw error;
  }
  const child = spawn("docker", ["start", "--attach", "--interactive", name], { stdio: "pipe" });
  const requests: InstalledCLIRequest[] = [];
  const pending: Promise<void>[] = [];
  const errors: string[] = [];
  let result: RelayFrame | undefined;
  let diagnostic = "";
  const lines = createInterface({ input: child.stdout });
  child.stderr.on("data", (value: Buffer) => { diagnostic += value.toString(); });
  const writeFrame = (value: unknown) => new Promise<void>((resolve, reject) => {
    child.stdin.write(`${JSON.stringify(value)}\n`, (error) => error ? reject(error) : resolve());
  });
  const forward = async (frame: RelayFrame) => {
    if (frame.method !== input.expected.method || frame.path !== input.expected.path || !frame.id || !frame.headers) {
      throw new Error("Old CLI attempted an unexpected route; no request forwarded");
    }
    const headers = new Headers();
    for (const [key, values] of Object.entries(frame.headers)) {
      if (!["connection", "content-length", "host"].includes(key.toLowerCase())) {
        for (const value of values) headers.append(key, value);
      }
    }
    expect(headers.get("Authorization")).toBe(`Bearer ${input.token}`);
    expect(headers.get("X-Client-Version")).toBe(installedCLIVersion.version);
    expect(headers.get("X-Client-Platform")).toBe("cli");
    expect(headers.get("X-Client-OS")).toBe("linux");
    const body = Buffer.from(frame.body ?? "", "base64");
    const response = await fetch(new URL(frame.path, api), { method: frame.method, headers,
      body: body.length > 0 ? body : undefined, redirect: "manual", signal: AbortSignal.timeout(12_000) });
    const bytes = Buffer.from(await response.arrayBuffer());
    requests.push({ method: frame.method, path: frame.path, status: response.status, body_sha256: digest(body),
      client_version: headers.get("X-Client-Version")!, client_platform: headers.get("X-Client-Platform")!, client_os: headers.get("X-Client-OS")! });
    await writeFrame({ kind: "response", id: frame.id, status: response.status,
      headers: { "Content-Type": [response.headers.get("content-type") ?? "application/json"] }, body: bytes.toString("base64") });
  };
  lines.on("line", (line) => {
    let frame: RelayFrame;
    try { frame = JSON.parse(line); } catch { errors.push("Malformed relay frame"); return; }
    if (frame.kind === "result") { result = frame; return; }
    if (frame.kind !== "request") { errors.push("Unexpected relay frame"); return; }
    pending.push(forward(frame).catch(async () => {
      errors.push("Installed CLI request identity/transport validation failed");
      await writeFrame({ kind: "response", id: frame.id, status: 502, body: Buffer.from("fixture request refused").toString("base64") }).catch(() => undefined);
    }));
  });
  child.stdin.on("error", () => { errors.push("Installed CLI private transport closed"); });
  const closed = new Promise<number | null>((resolve, reject) => {
    child.once("error", reject);
    child.once("close", resolve);
  });
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    await writeFrame({ token: input.token, workspace_id: input.workspaceId, args: input.args });
    const exit = await Promise.race([closed, new Promise<never>((_resolve, reject) => {
      timer = setTimeout(() => reject(new Error("Installed CLI execution exceeded fixture budget")), 45_000);
    })]);
    await Promise.all(pending);
    expect(exit, diagnostic.replaceAll(input.token, "[redacted]")).toBe(0);
    expect(errors).toEqual([]);
    if (!result || typeof result.exit_code !== "number") throw new Error("The actual packaged CLI result is missing");
    expect(requests).toHaveLength(1);
    return { args: input.args, exitCode: result.exit_code, requests, stdout: result.stdout ?? "", stderr: result.stderr ?? "",
      evidence: { args: input.args, exit_code: result.exit_code, requests, stdout_sha256: digest(result.stdout ?? ""),
        stderr_sha256: digest(result.stderr ?? ""), container_network: "none", config_root: "ephemeral task-only /tmp Multica root",
        home_override: false, provider_or_daemon_started: false } };
  } finally {
    if (timer) clearTimeout(timer);
    lines.close();
    child.stdin.end();
    // Only this test's random named container can be removed.
    await execute("docker", ["rm", "--force", name], { timeout: 10_000 }).catch(() => undefined);
    await execute("docker", ["volume", "rm", volume], { timeout: 10_000 }).catch(() => undefined);
    if (child.exitCode === null) child.kill("SIGTERM");
  }
}
