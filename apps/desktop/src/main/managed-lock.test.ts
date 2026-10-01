// @vitest-environment node
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { access, mkdir, mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import ts from "typescript";
import { withManagementFileLock } from "./managed-lock";
import { managementProcessIdentity } from "./managed-process";
import { loadManagedInstallation } from "./managed-installation";

let root: string;
const children: ChildProcess[] = [];
let moduleUrl: string;
beforeEach(async () => {
  root = await mkdtemp(join(tmpdir(), "multica-managed-lock-test-"));
  for (const name of ["managed-lock", "managed-process"]) {
    const source = await readFile(join(__dirname, `${name}.ts`), "utf8");
    const code = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText.replace('"./managed-process"', '"./managed-process.mjs"');
    await writeFile(join(root, `${name}.mjs`), code);
  }
  moduleUrl = pathToFileURL(join(root, "managed-lock.mjs")).href;
});
afterEach(async () => {
  for (const child of children.splice(0)) if (child.exitCode === null && child.signalCode === null) { child.kill("SIGKILL"); await new Promise((resolve) => child.once("exit", resolve)); }
  await rm(root, { recursive: true, force: true });
});

async function waitFile(path: string) {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) { try { await access(path); return; } catch { await new Promise((resolve) => setTimeout(resolve, 10)); } }
  throw new Error("Fake lock process did not reach its barrier");
}

function contender(directory: string, name: string, hold: boolean) {
  const choosing = join(root, `${name}.choosing`); const critical = join(root, `${name}.critical`); const release = join(root, "release-choosing");
  const script = `
import fs from 'node:fs/promises';
import { syncBuiltinESMExports } from 'node:module';
const original = fs.readdir;
let first = true;
fs.readdir = async (...args) => {
  const result = await original(...args);
  if (first && String(args[0]).endsWith('.installation-locks')) {
    first = false; await fs.writeFile(${JSON.stringify(choosing)}, 'ready');
    for (;;) { try { await fs.access(${JSON.stringify(release)}); break; } catch { await new Promise(r => setTimeout(r, 10)); } }
  }
  return result;
};
syncBuiltinESMExports();
const { withManagementFileLock } = await import(${JSON.stringify(moduleUrl)});
await withManagementFileLock(${JSON.stringify(directory)}, async () => {
  const guard = await fs.open(${JSON.stringify(join(root, "critical-guard"))}, 'wx', 0o600);
  await fs.writeFile(${JSON.stringify(critical)}, 'ready');
  let count = 0; try { count = Number(await fs.readFile(${JSON.stringify(join(root, "counter"))}, 'utf8')); } catch {}
  await new Promise(r => setTimeout(r, ${hold ? 60_000 : 100}));
  await fs.writeFile(${JSON.stringify(join(root, "counter"))}, String(count + 1));
  await guard.close(); await fs.unlink(${JSON.stringify(join(root, "critical-guard"))});
});
`;
  const child = spawn(process.execPath, ["--input-type=module"], { stdio: ["pipe", "ignore", "pipe"] });
  children.push(child); let stderr = ""; child.stderr!.on("data", (chunk) => { stderr += String(chunk); });
  const completed = new Promise<void>((resolve, reject) => child.once("exit", (code, signal) => code === 0 ? resolve() : reject(new Error(`Fake contender exited (${code ?? signal}): ${stderr}`))));
  // Crash tests intentionally kill this process, so attach a handler at once.
  void completed.catch(() => undefined);
  child.stdin!.end(script);
  return { child, choosing, critical, completed };
}

it("serializes two processes that publish choosing records concurrently", async () => {
  const directory = join(root, "identity"); await mkdir(directory, { mode: 0o700 });
  const first = contender(directory, "first", false); const second = contender(directory, "second", false);
  await Promise.all([waitFile(first.choosing), waitFile(second.choosing)]);
  const records = await Promise.all((await readdir(join(directory, ".installation-locks"))).filter((name) => name.endsWith(".json")).map(async (name) => JSON.parse(await readFile(join(directory, ".installation-locks", name), "utf8"))));
  expect(records).toHaveLength(2); expect(records.every((record) => record.ticket === null)).toBe(true);
  await writeFile(join(root, "release-choosing"), "release");
  await Promise.all([first.completed, second.completed]);
  expect(await readFile(join(root, "counter"), "utf8")).toBe("2");
}, 25_000);

it("does not steal a live owner and preserves the key after killing its holder", async () => {
  const deployment = "11111111-1111-4111-8111-111111111111";
  const original = await loadManagedInstallation(root, deployment);
  const directory = join(root, ".multica/management", deployment);
  const holder = contender(directory, "holder", true);
  await waitFile(holder.choosing); await writeFile(join(root, "release-choosing"), "release"); await waitFile(holder.critical);
  await expect(withManagementFileLock(directory, async () => undefined, 100)).rejects.toThrow("live lock owner");
  holder.child.kill("SIGKILL"); await holder.completed.catch(() => undefined);
  expect((await loadManagedInstallation(root, deployment)).publicInfo.publicKey).toBe(original.publicInfo.publicKey);
}, 25_000);

it("recovers an old-boot ticket even when its PID has been reused", async () => {
  const directory = join(root, "identity"); const locks = join(directory, ".installation-locks"); await mkdir(locks, { recursive: true, mode: 0o700 });
  const identity = await managementProcessIdentity(); const nonce = randomUUID();
  await writeFile(join(locks, `${nonce}.json`), JSON.stringify({ version: 1, pid: process.pid, owner_nonce: nonce, host_id: identity.hostId, boot_id: "0".repeat(64), ticket: "1" }), { mode: 0o600 });
  await withManagementFileLock(directory, async () => undefined);
  expect((await readdir(locks)).filter((name) => name.endsWith(".json"))).toEqual([]);
});

it.each(["foreign-host", "uninspectable-process"])("preserves a %s choosing owner", async (kind) => {
  const directory = join(root, "identity"); const locks = join(directory, ".installation-locks");
  await mkdir(locks, { recursive: true, mode: 0o700 });
  const identity = await managementProcessIdentity(); const nonce = randomUUID();
  const path = join(locks, `${nonce}.json`);
  await writeFile(path, JSON.stringify({ version: 1, pid: process.pid, owner_nonce: nonce, host_id: kind === "foreign-host" ? "f".repeat(64) : identity.hostId, boot_id: kind === "foreign-host" ? "0".repeat(64) : identity.bootId, ticket: null }), { mode: 0o600 });
  const probe = kind === "uninspectable-process" ? vi.spyOn(process, "kill").mockImplementation(() => { throw Object.assign(new Error("Not permitted"), { code: "EPERM" }); }) : null;
  try {
    await expect(withManagementFileLock(directory, async () => undefined, 60)).rejects.toThrow("live lock owner");
    expect(JSON.parse(await readFile(path, "utf8")).owner_nonce).toBe(nonce);
  } finally { probe?.mockRestore(); }
});
