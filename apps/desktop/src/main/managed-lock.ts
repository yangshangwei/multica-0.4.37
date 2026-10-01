import { randomUUID } from "node:crypto";
import { lstat, mkdir, open, readdir, readFile, rename, unlink } from "node:fs/promises";
import { join } from "node:path";
import { managementProcessIdentity } from "./managed-process";

interface Ticket { version: 1; pid: number; owner_nonce: string; host_id: string; boot_id: string; ticket: string | null }
const MAX_TICKET = 9_223_372_036_854_775_807n;

function ownerIsDead(pid: number): boolean {
  try { process.kill(pid, 0); return false; }
  catch (error) { return (error as NodeJS.ErrnoException).code === "ESRCH"; }
}

async function publish(path: string, value: Ticket) {
  const temporary = `${path}.${randomUUID()}.tmp`;
  const file = await open(temporary, "wx", 0o600);
  try { await file.writeFile(JSON.stringify(value)); await file.sync(); }
  finally { await file.close(); }
  try { await rename(temporary, path); }
  catch (error) { await unlink(temporary); throw error; }
}

async function tickets(directory: string): Promise<Ticket[]> {
  const result: Ticket[] = [];
  const current = await managementProcessIdentity();
  for (const name of await readdir(directory)) {
    if (!/^[0-9a-f-]{36}\.json$/.test(name)) continue;
    const path = join(directory, name);
    let value: Ticket;
    try {
      const info = await lstat(path);
      if (!info.isFile() || info.isSymbolicLink() || info.size > 1024 || (process.platform !== "win32" && (info.mode & 0o077) !== 0) || (process.getuid && info.uid !== process.getuid())) throw new Error("Invalid installation lock record");
      value = JSON.parse(await readFile(path, "utf8"));
    } catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") continue; throw error; }
    if (value.version !== 1 || !Number.isSafeInteger(value.pid) || value.pid <= 0 || value.pid > 0xffffffff || !/^[0-9a-f]{64}$/.test(value.host_id) || !/^[0-9a-f]{64}$/.test(value.boot_id) || name !== `${value.owner_nonce}.json` ||
      !(value.ticket === null || (typeof value.ticket === "string" && /^[1-9][0-9]*$/.test(value.ticket) && BigInt(value.ticket) <= MAX_TICKET))) throw new Error("Invalid installation lock owner");
    if (value.host_id === current.hostId && (value.boot_id !== current.bootId || ownerIsDead(value.pid))) {
      // Owner paths are never reused. Removing a dead owner's unique record
      // cannot race deletion of a new live owner's lock (the fixed-path ABA).
      await unlink(path).catch((error: NodeJS.ErrnoException) => { if (error.code !== "ENOENT") throw error; });
      continue;
    }
    result.push(value);
  }
  return result;
}

/** Lamport tickets serialize Node/Go peers and survive process death without TTL stealing. */
export async function withManagementFileLock<T>(parent: string, operation: () => Promise<T>, timeoutMs = 5_000): Promise<T> {
  const directory = join(parent, ".installation-locks");
  await mkdir(directory, { mode: 0o700 }).catch((error: NodeJS.ErrnoException) => { if (error.code !== "EEXIST") throw error; });
  const info = await lstat(directory);
  if (!info.isDirectory() || info.isSymbolicLink() || (process.platform !== "win32" && (info.mode & 0o077) !== 0) || (process.getuid && info.uid !== process.getuid())) throw new Error("Invalid installation lock directory");
  const current = await managementProcessIdentity();
  const owner: Ticket = { version: 1, pid: process.pid, owner_nonce: randomUUID(), host_id: current.hostId, boot_id: current.bootId, ticket: null };
  const path = join(directory, `${owner.owner_nonce}.json`);
  await publish(path, owner);
  try {
    let maximum = 0n;
    for (const other of await tickets(directory)) if (other.ticket !== null && BigInt(other.ticket) > maximum) maximum = BigInt(other.ticket);
    if (maximum === MAX_TICKET) throw new Error("Installation lock ticket exhausted");
    owner.ticket = String(maximum + 1n);
    await publish(path, owner);
    const deadline = Date.now() + timeoutMs;
    for (;;) {
      const blocked = (await tickets(directory)).some((other) => other.owner_nonce !== owner.owner_nonce && (other.ticket === null ||
        BigInt(other.ticket) < BigInt(owner.ticket!) || (other.ticket === owner.ticket && other.owner_nonce < owner.owner_nonce)));
      if (!blocked) return await operation();
      if (Date.now() >= deadline) throw new Error("Installation identity has a live lock owner");
      await new Promise((resolve) => setTimeout(resolve, 25));
    }
  } finally { await unlink(path).catch((error: NodeJS.ErrnoException) => { if (error.code !== "ENOENT") throw error; }); }
}
