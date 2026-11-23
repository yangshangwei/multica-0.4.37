// @vitest-environment node
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { lstat, mkdir, mkdtemp, readFile, readdir, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { withManagementFileLock } from "./managed-lock";

vi.mock("node:fs/promises", async (importOriginal) => {
  const actual = await importOriginal<typeof import("node:fs/promises")>();
  return { ...actual, lstat: vi.fn(actual.lstat), readdir: vi.fn(actual.readdir), rename: vi.fn(actual.rename) };
});
vi.mock("./managed-process", () => ({
  managementProcessIdentity: async () => ({ hostId: "a".repeat(64), bootId: "b".repeat(64) }),
}));

let root: string;
let originalRename: typeof rename;
let originalLstat: typeof lstat;
beforeEach(async () => {
  root = await mkdtemp(join(tmpdir(), "multica-ticket-publish-"));
  const actual = await vi.importActual<typeof import("node:fs/promises")>("node:fs/promises");
  originalRename = actual.rename;
  originalLstat = actual.lstat;
  vi.mocked(lstat).mockReset().mockImplementation(originalLstat);
  vi.mocked(readdir).mockReset().mockImplementation(actual.readdir);
  vi.mocked(rename).mockReset().mockImplementation(originalRename);
});
afterEach(async () => {
  vi.restoreAllMocks();
  await rm(root, { recursive: true, force: true });
});

function failTicketReplacement(code: string, failures = Infinity) {
  const attempts: Array<[string, string]> = [];
  const previousRecords: string[] = [];
  vi.mocked(rename).mockImplementation(async (temporary, destination) => {
    const content = JSON.parse(await readFile(temporary, "utf8"));
    if (content.ticket !== null) {
      attempts.push([String(temporary), String(destination)]);
      previousRecords.push(await readFile(destination, "utf8"));
      if (attempts.length <= failures) throw Object.assign(new Error("Rename rejected"), { code });
    }
    await originalRename(temporary, destination);
  });
  return { attempts, previousRecords };
}

it.each(["EPERM", "EACCES", "EBUSY"])("retries the same Windows ticket rename after %s without rerunning the operation", async (code) => {
  vi.spyOn(process, "platform", "get").mockReturnValue("win32");
  const { attempts, previousRecords } = failTicketReplacement(code, 2);
  const operation = vi.fn(async () => "locked");
  await expect(withManagementFileLock(root, operation)).resolves.toBe("locked");
  expect(attempts).toHaveLength(3);
  expect(attempts.every((pair) => pair[0] === attempts[0][0] && pair[1] === attempts[0][1])).toBe(true);
  expect(new Set(previousRecords).size).toBe(1);
  expect(JSON.parse(previousRecords[0]).ticket).toBeNull();
  expect(operation).toHaveBeenCalledTimes(1);
  expect(await readdir(join(root, ".installation-locks"))).toEqual([]);
});

it("fails immediately for a nontransient Windows rename error", async () => {
  vi.spyOn(process, "platform", "get").mockReturnValue("win32");
  const { attempts } = failTicketReplacement("ENOSPC");
  const operation = vi.fn();
  await expect(withManagementFileLock(root, operation)).rejects.toMatchObject({ code: "ENOSPC" });
  expect(attempts).toHaveLength(1);
  expect(operation).not.toHaveBeenCalled();
  expect(await readdir(join(root, ".installation-locks"))).toEqual([]);
});

it("bounds persistent Windows sharing failures and preserves the choosing record until owner cleanup", async () => {
  vi.spyOn(process, "platform", "get").mockReturnValue("win32");
  const { attempts, previousRecords } = failTicketReplacement("EPERM");
  const operation = vi.fn();
  const started = Date.now();
  await expect(withManagementFileLock(root, operation)).rejects.toMatchObject({ code: "EPERM" });
  const elapsed = Date.now() - started;
  expect(elapsed).toBeGreaterThanOrEqual(1_000);
  expect(elapsed).toBeLessThan(2_500);
  expect(attempts.length).toBeGreaterThan(1);
  expect(attempts.length).toBeLessThan(150);
  expect(attempts.every((pair) => pair[0] === attempts[0][0] && pair[1] === attempts[0][1])).toBe(true);
  expect(new Set(previousRecords).size).toBe(1);
  expect(JSON.parse(previousRecords[0]).ticket).toBeNull();
  expect(operation).not.toHaveBeenCalled();
  expect(await readdir(join(root, ".installation-locks"))).toEqual([]);
});

it.each(["darwin", "linux"] as const)("does not retry a permission error on %s", async (platform) => {
  vi.spyOn(process, "platform", "get").mockReturnValue(platform);
  // A Windows host does not report POSIX chmod bits. Model private POSIX
  // permissions only in these simulated Unix cases, keeping real Stats methods.
  vi.mocked(lstat).mockImplementation(async (path) => {
    const info = await originalLstat(path);
    info.mode &= ~0o077;
    return info;
  });
  const { attempts } = failTicketReplacement("EPERM");
  await expect(withManagementFileLock(root, vi.fn())).rejects.toMatchObject({ code: "EPERM" });
  expect(attempts).toHaveLength(1);
  expect(await readdir(join(root, ".installation-locks"))).toEqual([]);
});


it.each(["EPERM", "EACCES", "EBUSY"])("restarts a complete Windows ticket scan after transient %s", async (code) => {
  vi.spyOn(process, "platform", "get").mockReturnValue("win32");
  let failures = 0;
  vi.mocked(lstat).mockImplementation(async (path) => {
    if (String(path).endsWith(".json") && failures++ === 0) throw Object.assign(new Error("Reader pending deletion"), { code });
    return originalLstat(path);
  });
  const operation = vi.fn(async () => "locked");
  await expect(withManagementFileLock(root, operation)).resolves.toBe("locked");
  // Choosing, retry with a fresh directory snapshot, and ownership check.
  expect(vi.mocked(readdir)).toHaveBeenCalledTimes(3);
  expect(operation).toHaveBeenCalledTimes(1);
});

it.each(["live", "foreign"])("fails closed on a persistently unreadable Windows %s peer without removing it", async (kind) => {
  vi.spyOn(process, "platform", "get").mockReturnValue("win32");
  const locks = join(root, ".installation-locks");
  await mkdir(locks, { mode: 0o700 });
  const nonce = "00000000-0000-4000-8000-000000000000";
  const peer = join(locks, `${nonce}.json`);
  const content = JSON.stringify({ version: 1, pid: process.pid, owner_nonce: nonce, host_id: (kind === "live" ? "a" : "f").repeat(64), boot_id: "b".repeat(64), ticket: null });
  await writeFile(peer, content, { mode: 0o600 });
  let attempts = 0;
  vi.mocked(lstat).mockImplementation(async (path) => {
    if (String(path) === peer) {
      attempts++;
      throw Object.assign(new Error("Reader pending deletion"), { code: "EACCES" });
    }
    return originalLstat(path);
  });
  const operation = vi.fn();
  const started = Date.now();
  await expect(withManagementFileLock(root, operation)).rejects.toMatchObject({ code: "EACCES" });
  expect(Date.now() - started).toBeGreaterThanOrEqual(1_000);
  expect(Date.now() - started).toBeLessThan(2_500);
  expect(attempts).toBeGreaterThan(1);
  expect(operation).not.toHaveBeenCalled();
  expect(await readFile(peer, "utf8")).toBe(content);
  expect(await readdir(locks)).toEqual([`${nonce}.json`]);
});

it.each(["darwin", "linux"] as const)("fails an unreadable ticket immediately on %s", async (platform) => {
  vi.spyOn(process, "platform", "get").mockReturnValue(platform);
  let attempts = 0;
  vi.mocked(lstat).mockImplementation(async (path) => {
    if (String(path).endsWith(".json")) {
      attempts++;
      throw Object.assign(new Error("Permission denied"), { code: "EACCES" });
    }
    const info = await originalLstat(path);
    info.mode &= ~0o077;
    return info;
  });
  const operation = vi.fn();
  await expect(withManagementFileLock(root, operation)).rejects.toMatchObject({ code: "EACCES" });
  expect(attempts).toBe(1);
  expect(operation).not.toHaveBeenCalled();
});
