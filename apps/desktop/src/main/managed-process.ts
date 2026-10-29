import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { readFile, readlink } from "node:fs/promises";
import { join } from "node:path";
import { promisify } from "node:util";

const execute = promisify(execFile);
let identity: Promise<{ hostId: string; bootId: string }> | undefined;
const digest = (value: string) => createHash("sha256").update(value).digest("hex");
// Endpoint security hooks injected into powershell.exe can append their own
// lines to stdout, so the identity is read only from the marked line.
const WINDOWS_IDENTITY_MARKER = "MULTICA_IDENTITY:";
// server/internal/daemon/managed_process.go runs the same command; both sides must parse the same output.
const WINDOWS_IDENTITY_COMMAND = `$o=Get-CimInstance Win32_OperatingSystem;$h=(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography').MachineGuid;'${WINDOWS_IDENTITY_MARKER}'+(@{host=$h;boot=$o.LastBootUpTime.ToUniversalTime().ToString('o')}|ConvertTo-Json -Compress)`;

// execFile errors carry stdout, which can hold the unhashed identifiers.
async function run(file: string, args: string[], options: { timeout: number; maxBuffer?: number }): Promise<string> {
  try { return (await execute(file, args, options)).stdout; } catch { throw new Error("OS boot identity unavailable"); }
}

/** Parses the Windows identity command output; errors never echo stdout. */
export function parseWindowsIdentity(stdout: string): { hostId: string; bootId: string } {
  const lines = stdout.split(/\r?\n/).map((line) => line.trim()).filter((line) => line.startsWith(WINDOWS_IDENTITY_MARKER));
  let value: unknown;
  try { value = lines.length === 1 ? JSON.parse(lines[0]!.slice(WINDOWS_IDENTITY_MARKER.length)) : undefined; } catch { value = undefined; }
  if (!value || typeof value !== "object" || !("host" in value) || !("boot" in value) || typeof value.host !== "string" || typeof value.boot !== "string" || !value.host || !value.boot) throw new Error("OS boot identity unavailable");
  return { hostId: digest(`windows-host:${value.host.toLowerCase()}`), bootId: digest(`windows-boot:${value.boot}`) };
}

/** Local lock provenance only; host/boot identifiers are hashed and never reported. */
export function managementProcessIdentity(): Promise<{ hostId: string; bootId: string }> {
  // Cache only a success: one slow PowerShell start must not disable managed
  // locking until the app restarts.
  identity ??= readManagementProcessIdentity().catch((error: unknown) => { identity = undefined; throw error; });
  return identity;
}

async function readManagementProcessIdentity(): Promise<{ hostId: string; bootId: string }> {
  if (process.platform === "darwin") {
    const [platform, boot] = await Promise.all([
      run("/usr/sbin/ioreg", ["-rd1", "-c", "IOPlatformExpertDevice"], { timeout: 5_000, maxBuffer: 256 * 1024 }),
      run("/usr/sbin/sysctl", ["-n", "kern.bootsessionuuid"], { timeout: 5_000 }),
    ]);
    const host = platform.match(/"IOPlatformUUID"\s*=\s*"([0-9a-f-]+)"/i)?.[1];
    if (!host || !/^[0-9a-f-]{36}$/i.test(boot.trim())) throw new Error("OS boot identity unavailable");
    return { hostId: digest(`darwin-host:${host.toLowerCase()}`), bootId: digest(`darwin-boot:${boot.trim().toLowerCase()}`) };
  }
  if (process.platform === "linux") {
    const [host, boot, namespace] = await Promise.all([readFile("/etc/machine-id", "utf8"), readFile("/proc/sys/kernel/random/boot_id", "utf8"), readlink("/proc/self/ns/pid")]);
    return { hostId: digest(`linux-host:${host.trim().toLowerCase()}\0${namespace}`), bootId: digest(`linux-boot:${boot.trim().toLowerCase()}`) };
  }
  if (process.platform === "win32") {
    const powershell = join(process.env.SystemRoot ?? "C:\\Windows", "System32/WindowsPowerShell/v1.0/powershell.exe");
    return parseWindowsIdentity(await run(powershell, ["-NoProfile", "-NonInteractive", "-Command", WINDOWS_IDENTITY_COMMAND], { timeout: 10_000 }));
  }
  throw new Error("Managed locking is unavailable on this OS");
}
