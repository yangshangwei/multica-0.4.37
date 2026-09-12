import { randomUUID } from "node:crypto";
import { closeSync, fchmodSync, fsyncSync, openSync, readSync, renameSync, unlinkSync, writeFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const MAX_FEED_BYTES = 2 * 1024 * 1024;
export const COMMIT_PATTERN = /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
export const REPOSITORY_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_.-]*\/[A-Za-z0-9][A-Za-z0-9_.-]*$/;
export const VERSION_PATTERN = /^v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$/;
export const CATEGORIES = ["features", "improvements", "fixes", "other"];

function requireValue(condition, message) {
  if (!condition) throw new Error(message);
}

function object(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function plainText(value) {
  // eslint-disable-next-line no-control-regex -- Reject non-text bytes at the artifact boundary.
  return typeof value === "string" && value.trim().length > 0 && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(value);
}

export function validTimestamp(value) {
  if (typeof value !== "string") return false;
  const parts = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-](\d{2}):(\d{2}))$/.exec(value);
  if (!parts) return false;
  const [, year, month, day, hour, minute, second, offsetHour = "0", offsetMinute = "0"] = parts;
  const days = [31, Number(year) % 4 === 0 && (Number(year) % 100 !== 0 || Number(year) % 400 === 0) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  return Number(month) >= 1 && Number(month) <= 12 && Number(day) >= 1 && Number(day) <= days[Number(month) - 1]
    && Number(hour) < 24 && Number(minute) < 60 && Number(second) < 60 && Number(offsetHour) < 24 && Number(offsetMinute) < 60
    && Number.isFinite(Date.parse(value));
}

export function validateFeed(feed, repository) {
  requireValue(object(feed) && feed.schema_version === 1, "changelog schema_version must be 1");
  requireValue(validTimestamp(feed.generated_at), "changelog generated_at must be an RFC3339 timestamp");
  requireValue(Array.isArray(feed.releases), "changelog releases must be an array");
  const identities = new Set();
  let forkRepository;
  for (const release of feed.releases) {
    requireValue(object(release), "changelog release must be an object");
    const label = `changelog release ${release.id ?? "(missing identity)"}`;
    requireValue(plainText(release.id) && plainText(release.version) && plainText(release.title), `${label}: identity, version and title are required`);
    requireValue(["fork", "upstream"].includes(release.source), `${label}: invalid source`);
    requireValue(["published", "prerelease", "unreleased"].includes(release.status), `${label}: invalid status`);
    const identity = release.id.split(":");
    requireValue(identity.length === 3 && identity[0] === release.source && REPOSITORY_PATTERN.test(identity[1])
      && identity[2] === (release.status === "unreleased" ? "unreleased" : release.version), `${label}: invalid identity`);
    requireValue(!identities.has(release.id), `${label}: duplicate identity`);
    identities.add(release.id);
    if (release.source === "fork") {
      requireValue(!forkRepository || forkRepository === identity[1], `${label}: mixed fork repository history`);
      forkRepository = identity[1];
      requireValue(!repository || repository === forkRepository, `${label}: history repository does not match --repository`);
    }
    requireValue(release.status === "unreleased" ? release.published_at === null : validTimestamp(release.published_at), `${label}: invalid status/publication date combination`);
    for (const key of ["commit", "base_commit"]) {
      requireValue(typeof release[key] === "string" && COMMIT_PATTERN.test(release[key]) || release.source === "upstream" && release[key] === null, `${label}: invalid ${key}`);
    }
    requireValue(Array.isArray(release.sections), `${label}: sections must be an array`);
    for (const section of release.sections) {
      requireValue(object(section) && CATEGORIES.includes(section.category) && Array.isArray(section.items), `${label}: invalid category/items`);
      for (const item of section.items) {
        requireValue(object(item) && plainText(item.text), `${label}: item text is required`);
        requireValue(!Object.hasOwn(item, "commit") || typeof item.commit === "string" && COMMIT_PATTERN.test(item.commit), `${label}: invalid item commit`);
      }
    }
  }
  return feed;
}

export function parseFeed(bytes, repository) {
  requireValue(bytes.byteLength <= MAX_FEED_BYTES, "changelog exceeds the 2 MiB size limit");
  let feed;
  try {
    feed = JSON.parse(new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes));
  } catch (error) {
    throw new Error(`invalid changelog JSON/UTF-8: ${error.message}`);
  }
  return validateFeed(feed, repository);
}

export function readBounded(path, limit = MAX_FEED_BYTES) {
  const fd = openSync(path, "r");
  try {
    const buffer = Buffer.alloc(limit + 1);
    let length = 0;
    while (length <= limit) {
      const count = readSync(fd, buffer, length, buffer.length - length, null);
      if (count === 0) break;
      length += count;
    }
    requireValue(length <= limit, `changelog exceeds the ${limit === MAX_FEED_BYTES ? "2 MiB" : limit + " byte"} size limit`);
    return buffer.subarray(0, length);
  } finally {
    closeSync(fd);
  }
}

export function readFeed(path, repository) {
  const bytes = readBounded(path);
  return { bytes, feed: parseFeed(bytes, repository) };
}

// Replace the directory entry only after all bytes have reached a closed file.
// A mounted directory observes the rename; a single-file bind mount would not.
export function stageWrite(destination, bytes, mode = 0o644) {
  const path = resolve(destination);
  const temporary = join(dirname(path), `.${basename(path)}.${randomUUID()}.tmp`);
  let fd;
  try {
    fd = openSync(temporary, "wx", mode);
    writeFileSync(fd, bytes);
    fchmodSync(fd, mode);
    fsyncSync(fd);
    closeSync(fd);
    fd = undefined;
    return {
      commit: () => renameSync(temporary, path),
      discard: () => { try { unlinkSync(temporary); } catch (error) { if (error.code !== "ENOENT") throw error; } },
    };
  } catch (error) {
    if (fd !== undefined) closeSync(fd);
    try { unlinkSync(temporary); } catch (cleanupError) { if (cleanupError.code !== "ENOENT") throw cleanupError; }
    throw error;
  }
}

export function atomicWrite(destination, bytes, mode = 0o644) {
  const staged = stageWrite(destination, bytes, mode);
  try { staged.commit(); } finally { staged.discard(); }
}

export function parseArguments(argv, allowed, required = []) {
  const values = {};
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    requireValue(key?.startsWith("--") && allowed.includes(key.slice(2)), `unknown option ${key}`);
    requireValue(!Object.hasOwn(values, key.slice(2)), `duplicate option ${key}`);
    const value = argv[index + 1];
    requireValue(typeof value === "string" && value.length > 0 && !value.startsWith("--"), `${key} needs a value`);
    values[key.slice(2)] = value;
  }
  for (const key of required) requireValue(Object.hasOwn(values, key), `--${key} is required`);
  return values;
}

export function isMain(url) {
  return Boolean(process.argv[1]) && fileURLToPath(url) === resolve(process.argv[1]);
}

export function runMain(callback) {
  Promise.resolve().then(callback).catch((error) => {
    process.stderr.write(`Error: ${error.message}\n`);
    process.exitCode = 1;
  });
}

export function markdownText(text) {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/([\\`*_{}[\]()#+.!|~-])/g, "\\$1").replace(/\r?\n/g, " ");
}

export function releaseMarkdown(release, repository) {
  const labels = { features: "Features", improvements: "Improvements", fixes: "Fixes", other: "Other" };
  const lines = [
    `# ${markdownText(release.title)}`,
    "",
    `Version: ${markdownText(release.version)} · Status: ${release.status}`,
    `Repository: ${markdownText(repository)}`,
    `Revision: ${release.commit}`,
    `Base revision: ${release.base_commit}`,
    `Published: ${release.published_at ?? "Unreleased"}`,
  ];
  for (const section of release.sections) {
    lines.push("", `## ${labels[section.category]}`, "");
    for (const item of section.items) lines.push(`- ${markdownText(item.text)}`);
  }
  if (release.sections.length === 0) lines.push("", "No user-facing changes in this commit range.");
  return lines.join("\n") + "\n";
}
