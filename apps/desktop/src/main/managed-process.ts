import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { readFile, readlink } from "node:fs/promises";
import { join } from "node:path";
import { promisify } from "node:util";

const execute = promisify(execFile);
let identity: Promise<{ hostId: string; bootId: string }> | undefined;
const digest = (value: string) => createHash("sha256").update(value).digest("hex");

/** Local lock provenance only; host/boot identifiers are hashed and never reported. */
export function managementProcessIdentity(): Promise<{ hostId: string; bootId: string }> {
  identity ??= (async () => {
    if (process.platform === "darwin") {
      const [{ stdout: platform }, { stdout: boot }] = await Promise.all([
        execute("/usr/sbin/ioreg", ["-rd1", "-c", "IOPlatformExpertDevice"], { timeout: 5_000, maxBuffer: 256 * 1024 }),
        execute("/usr/sbin/sysctl", ["-n", "kern.bootsessionuuid"], { timeout: 5_000 }),
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
      const command = "$o=Get-CimInstance Win32_OperatingSystem;$h=(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography').MachineGuid;@{host=$h;boot=$o.LastBootUpTime.ToUniversalTime().ToString('o')}|ConvertTo-Json -Compress";
      const { stdout } = await execute(powershell, ["-NoProfile", "-NonInteractive", "-Command", command], { timeout: 10_000 });
      const value: unknown = JSON.parse(stdout.trim());
      if (!value || typeof value !== "object" || !("host" in value) || !("boot" in value) || typeof value.host !== "string" || typeof value.boot !== "string" || !value.host || !value.boot) throw new Error("OS boot identity unavailable");
      return { hostId: digest(`windows-host:${value.host.toLowerCase()}`), bootId: digest(`windows-boot:${value.boot}`) };
    }
    throw new Error("Managed locking is unavailable on this OS");
  })();
  return identity;
}
