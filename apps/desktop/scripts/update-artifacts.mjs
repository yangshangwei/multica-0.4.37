#!/usr/bin/env node
import { createHash, randomUUID } from "node:crypto";
import { constants, createReadStream } from "node:fs";
import { chmod, copyFile, link, lstat, mkdir, readFile, readdir, realpath, rename, rm, writeFile } from "node:fs/promises";
import { basename, dirname, isAbsolute, join, relative, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parseUpdateInfo } from "electron-updater/out/providers/Provider.js";

export const metadataName = /^latest(?:-[A-Za-z0-9]+)*\.yml$/;
const artifactName = /^multica-desktop-[A-Za-z0-9][A-Za-z0-9._+-]*\.(?:exe|dmg|zip|AppImage|deb|rpm|7z)(?:\.blockmap)?$/;
const versionPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

async function directory(path, mode = 0o755) {
  await mkdir(path, { recursive: true, mode });
  if (!(await lstat(path)).isDirectory()) throw new Error(`Expected a directory, not a symlink: ${path}`);
}

async function canonicalPath(path) {
  try {
    return await realpath(path);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    return join(await canonicalPath(dirname(path)), basename(path));
  }
}

async function locations(source, destination) {
  if (!source || !destination) throw new Error("Both --source and --destination are required");
  const from = await realpath(resolve(source));
  const to = await canonicalPath(resolve(destination));
  const contains = (parent, child) => {
    const path = relative(parent, child);
    return path === "" || (!path.startsWith(`..${process.platform === "win32" ? "\\" : "/"}`) && path !== ".." && !isAbsolute(path));
  };
  if (contains(from, to) || contains(to, from)) throw new Error("Source and destination must not overlap");
  if (!(await lstat(from)).isDirectory()) throw new Error(`Source is not a directory: ${from}`);
  return { source: from, destination: to };
}

async function fingerprint(path) {
  if (!(await lstat(path)).isFile()) throw new Error(`Expected a regular file, not a symlink: ${path}`);
  const hash = createHash("sha512");
  let size = 0;
  for await (const chunk of createReadStream(path)) {
    hash.update(chunk);
    size += chunk.length;
  }
  return { sha512: hash.digest("base64"), size };
}

function sameFile(left, right) {
  return left.size === right.size && left.sha512 === right.sha512;
}

async function inventory(source) {
  const files = new Map();
  async function visit(path) {
    for (const entry of (await readdir(path, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name))) {
      const file = join(path, entry.name);
      if (entry.isDirectory()) {
        if (!entry.name.startsWith(".") && !/(?:-unpacked|\.app)$/.test(entry.name) && entry.name !== "node_modules") await visit(file);
        continue;
      }
      if (!metadataName.test(entry.name) && !artifactName.test(entry.name)) continue;
      const info = { name: entry.name, path: file, metadata: metadataName.test(entry.name), ...await fingerprint(file) };
      if (files.has(entry.name) && !sameFile(files.get(entry.name), info)) throw new Error(`Conflicting duplicate artifact: ${entry.name}`);
      files.set(entry.name, info);
    }
  }
  await visit(source);
  if (![...files.values()].some((file) => file.metadata)) throw new Error("No generated latest*.yml update metadata found in source");
  return files;
}

function validateUpdateFilename(name, version) {
  if (typeof name !== "string" || !artifactName.test(name) || basename(name) !== name) throw new Error(`Unsafe update filename: ${String(name)}`);
  if (!name.startsWith(`multica-desktop-${version}-`)) throw new Error(`Update filename does not match version ${version}: ${name}`);
}

export function validateUpdateVersion(value, allowPrerelease = false) {
  const version = typeof value === "string" && versionPattern.exec(value);
  if (!version) throw new Error(`Invalid update version: ${String(value)}`);
  if (version[4] && !allowPrerelease) throw new Error(`Stable versions only: ${value}; set a stable VERSION or add --allow-prerelease for intentional test builds`);
}

// Share the metadata contract between offline publication and HTTP verification.
export function updateReferences(info, allowPrerelease = false) {
  validateUpdateVersion(info?.version, allowPrerelease);
  if (!Array.isArray(info.files) || info.files.length === 0) throw new Error("No update files in metadata");
  const references = new Map();
  function add(reference, key, requireSize = true) {
    if (!reference || typeof reference !== "object" || Array.isArray(reference)) throw new Error("Invalid update file record");
    const name = reference[key];
    validateUpdateFilename(name, info.version);
    const { size, sha512 } = reference;
    if ((requireSize || size !== undefined) && (!Number.isSafeInteger(size) || size <= 0)) throw new Error(`Invalid update size: ${name}`);
    if (typeof sha512 !== "string" || !/^[A-Za-z0-9+/]{86}==$/.test(sha512)) throw new Error(`Invalid update SHA-512: ${name}`);
    const previous = references.get(name);
    if (previous && (previous.sha512 !== sha512 || (previous.size !== undefined && size !== undefined && previous.size !== size))) {
      throw new Error(`Conflicting update references: ${name}`);
    }
    references.set(name, { name, size: size ?? previous?.size, sha512 });
  }
  for (const reference of info.files) add(reference, "url");
  if (info.path !== undefined) add(info, "path", false);
  if (info.packages !== undefined) {
    if (!info.packages || typeof info.packages !== "object" || Array.isArray(info.packages)) throw new Error("Invalid update packages");
    for (const reference of Object.values(info.packages)) add(reference, "path");
  }
  return [...references.values()];
}

async function validate(files, allowPrerelease) {
  for (const file of files.values()) {
    if (!file.metadata) continue;
    const info = parseUpdateInfo(await readFile(file.path, "utf8"), file.name, pathToFileURL(file.path));
    for (const reference of updateReferences(info, allowPrerelease)) {
      const artifact = files.get(reference.name);
      if (!artifact || artifact.metadata) throw new Error(`Missing referenced update artifact: ${reference.name}`);
      if (reference.size !== undefined && reference.size !== artifact.size) throw new Error(`Update size mismatch: ${reference.name}`);
      if (reference.sha512 !== artifact.sha512) throw new Error(`Update SHA-512 mismatch: ${reference.name}`);
    }
  }
}

async function stageFiles(files, staging, allowPrerelease, publicFiles) {
  await directory(staging, 0o700);
  for (const file of files.values()) {
    const path = join(staging, file.name);
    await copyFile(file.path, path, constants.COPYFILE_EXCL);
    if (publicFiles) await chmod(path, 0o644);
    if (!sameFile(file, await fingerprint(path))) throw new Error(`Artifact changed while copying: ${file.name}`);
  }
  const staged = await inventory(staging);
  await validate(staged, allowPrerelease);
  return staged;
}

async function existingFile(path) {
  try {
    return await fingerprint(path);
  } catch (error) {
    if (error.code === "ENOENT") return null;
    throw error;
  }
}

async function checkImmutable(files, destination, replaceMetadata) {
  for (const file of files.values()) {
    const existing = await existingFile(join(destination, file.name));
    if (existing && !(replaceMetadata && file.metadata) && !sameFile(file, existing)) throw new Error(`Refusing to overwrite immutable artifact: ${file.name}`);
  }
}

async function installImmutable(file, destination) {
  const path = join(destination, file.name);
  try {
    // A hard link atomically exposes complete bytes and cannot overwrite a
    // concurrent writer's file. Staging and public live on the same filesystem.
    await link(file.path, path);
  } catch (error) {
    if (error.code !== "EEXIST") throw error;
    if (!sameFile(file, await fingerprint(path))) throw new Error(`Refusing to overwrite immutable artifact: ${file.name}`);
  }
}

async function expose(files, destination, replaceMetadata) {
  for (const file of files.values()) if (!file.metadata) await installImmutable(file, destination);
  // Each channel is atomic; different platform channels may switch separately.
  for (const file of files.values()) {
    if (!file.metadata) continue;
    if (replaceMetadata) await rename(file.path, join(destination, file.name));
    else await installImmutable(file, destination);
  }
}

export async function collectArtifacts({ source, destination, allowPrerelease = false }) {
  const paths = await locations(source, destination);
  const files = await inventory(paths.source);
  await validate(files, allowPrerelease);
  await directory(paths.destination);
  await checkImmutable(files, paths.destination, false);
  const staging = join(paths.destination, `.collect-${randomUUID()}`);
  try {
    const staged = await stageFiles(files, staging, allowPrerelease, false);
    await expose(staged, paths.destination, false);
  } finally {
    await rm(staging, { recursive: true, force: true });
  }
  return { directory: paths.destination, files: [...files.keys()].sort() };
}

async function snapshotMetadata(publicDirectory, backups, id) {
  const metadata = (await readdir(publicDirectory)).filter((name) => metadataName.test(name));
  if (metadata.length === 0) return;
  const snapshot = join(backups, id);
  await directory(snapshot, 0o700);
  for (const name of metadata) {
    const from = join(publicDirectory, name);
    if (!(await lstat(from)).isFile()) throw new Error(`Expected regular metadata file: ${from}`);
    await copyFile(from, join(snapshot, name), constants.COPYFILE_EXCL);
  }
}

export async function publishArtifacts({ source, destination, allowPrerelease = false }) {
  const paths = await locations(source, destination);
  await directory(paths.destination);
  const lock = join(paths.destination, ".publish.lock");
  try {
    await mkdir(lock, { mode: 0o700 });
  } catch (error) {
    if (error.code === "EEXIST") throw new Error(`Another publisher holds ${lock}; inspect owner.json and remove a stale lock only after verifying its process has stopped`);
    throw error;
  }
  let staging;
  try {
    await writeFile(join(lock, "owner.json"), JSON.stringify({ pid: process.pid, startedAt: new Date().toISOString(), source: paths.source }) + "\n", { flag: "wx", mode: 0o600 });
    const publicDirectory = join(paths.destination, "public");
    const backups = join(paths.destination, "backups");
    await directory(publicDirectory);
    await directory(backups, 0o700);
    await directory(join(paths.destination, "staging"), 0o700);
    const files = await inventory(paths.source);
    await validate(files, allowPrerelease);
    await checkImmutable(files, publicDirectory, true);
    const id = `${new Date().toISOString().replaceAll(":", "-")}-${randomUUID()}`;
    staging = join(paths.destination, "staging", id);
    const staged = await stageFiles(files, staging, allowPrerelease, true);
    await snapshotMetadata(publicDirectory, backups, id);
    await expose(staged, publicDirectory, true);
    return { directory: publicDirectory, files: [...files.keys()].sort() };
  } finally {
    try {
      if (staging) await rm(staging, { recursive: true, force: true });
    } finally {
      await rm(lock, { recursive: true, force: true });
    }
  }
}

async function main(args) {
  if (args.includes("--help") || args.includes("-h")) {
    console.log("Usage: node apps/desktop/scripts/update-artifacts.mjs <collect|publish> --source DIR --destination DIR [--allow-prerelease]\nPublish writes DIR/public; staging, backups and the publisher lock stay private.");
    return;
  }
  const command = args.shift();
  if (command !== "collect" && command !== "publish") throw new Error("Expected collect or publish command");
  const options = {};
  while (args.length) {
    const arg = args.shift();
    if (arg === "--allow-prerelease") options.allowPrerelease = true;
    else if ((arg === "--source" || arg === "--destination") && args[0] && !args[0].startsWith("--")) options[arg.slice(2)] = args.shift();
    else throw new Error(`Unknown argument or missing value: ${arg}`);
  }
  const result = await (command === "collect" ? collectArtifacts(options) : publishArtifacts(options));
  console.log(`${command === "collect" ? "Collected" : "Published"} ${result.files.length} verified files to ${result.directory}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main(process.argv.slice(2)).catch((error) => {
    console.error(`Desktop updates: ${error.message}`);
    process.exitCode = 1;
  });
}
