#!/usr/bin/env node
import { lstatSync, readFileSync, unlinkSync } from "node:fs";
import { atomicWrite, isMain, parseArguments, readBounded, readFeed, runMain, stageWrite } from "./changelog-lib.mjs";

export const CONTAINER_CHANGELOG_FILE = "/app/data/changelog/changelog.json";

function regularFile(path, optional = false) {
  try {
    const stat = lstatSync(path);
    if (!stat.isFile()) throw new Error(`${path} must be a regular file, not a directory or symlink`);
    return stat;
  } catch (error) {
    if (optional && error.code === "ENOENT") return null;
    throw error;
  }
}

function configuredEnv(contents, directory, images) {
  // Preserve full unrelated assignments, including multiline quoted values.
  // Only selected assignments are replaced; the file is never sourced.
  const replacedKeys = new Set(["CHANGELOG_FILE", "CHANGELOG_DIRECTORY", ...Object.keys(images)]);
  const lines = contents.match(/[^\n]*(?:\n|$)/g).filter(Boolean);
  const kept = [];
  let quote = null;
  let dropping = false;
  for (const line of lines) {
    if (!quote) dropping = replacedKeys.has(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=/.exec(line)?.[1]);
    if (!dropping) kept.push(line);
    let start = 0;
    if (!quote) {
      const assignment = /^\s*(?:export\s+)?[A-Za-z_][A-Za-z0-9_]*\s*=\s*/.exec(line);
      if (!assignment) continue;
      start = assignment[0].length;
      if (!["'", '"'].includes(line[start])) continue;
    }
    for (let index = start; index < line.length; index++) {
      const char = line[index];
      if (char === "\\") { index++; continue; }
      if (quote && char === quote) { quote = null; break; }
      if (!quote && (char === "'" || char === '"')) quote = char;
    }
  }
  if (quote) throw new Error("deployment .env has an unterminated quoted value");
  const text = kept.join("");
  // Compose keeps backslashes literally in single quotes, making trailing
  // backslashes ambiguous. Double quotes support escaped slashes/quotes;
  // doubled dollars prevent dotenv interpolation from changing a path.
  const escaped = (value) => value.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\$/g, () => "$$");
  const imageValues = Object.entries(images).map(([key, value]) => `${key}="${escaped(value)}"\n`).join("");
  return `${text}${text && !text.endsWith("\n") ? "\n" : ""}CHANGELOG_FILE='${CONTAINER_CHANGELOG_FILE}'\nCHANGELOG_DIRECTORY="${escaped(directory)}"\n${imageValues}`;
}

export function installChangelog(options) {
  if (options["current-file"] && options["current-file"] !== CONTAINER_CHANGELOG_FILE) throw new Error("existing CHANGELOG_FILE is incompatible with the bundled directory mount");
  if (!options.directory?.startsWith("/") || /[\r\n]/.test(options.directory) || options.directory.includes("\0")) throw new Error("changelog directory must be an absolute single-line host path");
  const imageOptions = ["backend-image", "web-image", "image-tag"];
  const images = {};
  if (imageOptions.some((key) => Object.hasOwn(options, key))) {
    if (imageOptions.some((key) => typeof options[key] !== "string" || !options[key].trim() || /[\r\n\0]/.test(options[key]))) {
      throw new Error("image selections require nonempty single-line backend-image, web-image and image-tag values");
    }
    images.MULTICA_BACKEND_IMAGE = options["backend-image"];
    images.MULTICA_WEB_IMAGE = options["web-image"];
    images.MULTICA_IMAGE_TAG = options["image-tag"];
  }
  const { bytes, feed } = readFeed(options.input);
  const envStat = regularFile(options["deployment-env"]);
  const oldStat = regularFile(options.destination, true);
  const previous = oldStat ? readBounded(options.destination) : null;
  const env = configuredEnv(readFileSync(options["deployment-env"], "utf8"), options.directory, images);
  let stagedEnv;
  let stagedFeed;
  let replaced = false;
  try {
    // Both parent directories must accept a flushed temporary file before a
    // visible change. If config finalization fails, restore the old feed.
    stagedEnv = stageWrite(options["deployment-env"], env, envStat.mode & 0o777);
    stagedFeed = stageWrite(options.destination, bytes);
    stagedFeed.commit();
    replaced = true;
    stagedEnv.commit();
  } catch (error) {
    if (replaced) {
      try {
        if (previous) atomicWrite(options.destination, previous, oldStat.mode & 0o777);
        else unlinkSync(options.destination);
      } catch (rollbackError) {
        throw new Error(`configuration failed (${error.message}); feed rollback also failed (${rollbackError.message}); restore the deployment backup before retrying`);
      }
    }
    throw error;
  } finally {
    stagedEnv?.discard();
    stagedFeed?.discard();
  }
  return feed;
}

if (isMain(import.meta.url)) runMain(() => {
  const options = parseArguments(process.argv.slice(2), ["input", "destination", "deployment-env", "directory", "current-file", "backend-image", "web-image", "image-tag"], ["input", "destination", "deployment-env", "directory"]);
  const feed = installChangelog(options);
  process.stdout.write(`Installed ${feed.releases.length} changelog entries and saved deployment feed configuration\n`);
});
