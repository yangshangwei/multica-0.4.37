#!/usr/bin/env node
// Fails a pull request that adds a bitmap over IMAGE_BUDGET_BYTES, or grows an
// existing one past it, unless the PR description carries an exemption line.
//
// MUL-6352 took the repo from 21.7MB of PNG/JPG to ~4MB of WebP; this keeps it
// there. Every committed bitmap is paid for on every clone, every Docker build
// context, and every deploy upload — costs a runtime image optimizer like
// `next/image` never touches, because it optimizes what visitors download, not
// what the repo carries.
//
// Shrinking a file is always allowed: the check compares against the size at
// `--base`, so a PR that recompresses an oversized image passes even while the
// result is still over budget.
//
// Usage:
//   node scripts/check-image-budget.mjs --base origin/main
//
// Reads PR_BODY from the environment for the exemption escape hatch, and emits
// GitHub Actions annotations when GITHUB_ACTIONS is set.

import { execFileSync, spawn } from "node:child_process";
import { statSync } from "node:fs";

const IMAGE_BUDGET_BYTES = 300 * 1024;
const BITMAP_EXTENSIONS = /\.(png|jpe?g|gif|bmp|tiff?|webp|avif)$/i;
const EXEMPTION_PATTERN = /^\s*Oversized image exemption:\s*\S/im;

const baseRef = readBaseRef();
const offenders = [];

for await (const { status, file } of changedFiles(baseRef)) {
  if (!BITMAP_EXTENSIONS.test(file)) continue;
  const bytes = statSync(file).size;
  if (bytes <= IMAGE_BUDGET_BYTES) continue;
  const was = status === "A" ? 0 : blobSize(baseRef, file);
  if (bytes <= was) continue; // recompressed downward — an improvement
  offenders.push({ file, bytes, was });
}

if (offenders.length === 0) {
  console.log(`No bitmap added or grown past ${kb(IMAGE_BUDGET_BYTES)} against ${baseRef}.`);
  process.exit(0);
}

for (const { file, bytes, was } of offenders) {
  const from = was === 0 ? "new file" : `up from ${kb(was)}`;
  const message = `${file} is ${kb(bytes)} (${from}), over the ${kb(IMAGE_BUDGET_BYTES)} image budget`;
  console.log(process.env.GITHUB_ACTIONS ? `::error file=${file}::${message}` : `  ${message}`);
}

if (EXEMPTION_PATTERN.test(process.env.PR_BODY ?? "")) {
  console.log("\nPR description carries an exemption line — allowing.");
  process.exit(0);
}

console.error(
  [
    "",
    `${offenders.length} bitmap(s) exceed the ${kb(IMAGE_BUDGET_BYTES)} budget.`,
    "",
    "Shrink them first. Lossy WebP capped at 1920px for decorative art and",
    "1600px for screenshots strips most of the weight with no visible loss:",
    "",
    "  cwebp -q 78 -m 6 -sharp_yuv -metadata none -resize 1600 0 in.png -o out.webp",
    "",
    "Flat, palettized UI screenshots sometimes encode smaller losslessly",
    "(`cwebp -lossless -z 9`) — compare both and keep the smaller file.",
    "",
    "If the size is genuinely required, add a line to the PR description:",
    "",
    "  Oversized image exemption: <why this file has to ship at this size>",
  ].join("\n"),
);
process.exit(1);

// Added and modified paths between `base` and the working tree. Deletions
// cannot regress the budget, so the filter drops them.
async function* changedFiles(base) {
  // Stream NUL-delimited fields: neither large diffs nor quoted paths should
  // stop the budget check before it reaches the bitmaps.
  const child = spawn("git", ["diff", "--name-status", "--diff-filter=AM", "-z", base, "--"], {
    stdio: ["ignore", "pipe", "inherit"],
  });
  /** @type {Promise<Error | undefined>} */
  const closed = new Promise((resolve) => {
    child.once("error", resolve);
    child.once("close", (code, signal) => {
      resolve(code === 0 ? undefined : new Error(`git diff failed (${signal ?? code})`));
    });
  });
  child.stdout.setEncoding("utf8");
  let pending = "";
  let status;
  try {
    for await (const chunk of child.stdout) {
      pending += chunk;
      let end;
      while ((end = pending.indexOf("\0")) !== -1) {
        const field = pending.slice(0, end);
        pending = pending.slice(end + 1);
        if (status === undefined) {
          status = field;
        } else {
          yield { status, file: field };
          status = undefined;
        }
      }
    }
    const error = await closed;
    if (error) throw error;
    if (pending || status !== undefined) throw new Error("git diff returned an incomplete path record");
  } finally {
    if (child.exitCode === null && child.signalCode === null) child.kill();
    await closed;
  }
}

// Byte size of `file` as of `ref`, or 0 when it did not exist there.
function blobSize(ref, file) {
  try {
    return Number(execFileSync("git", ["cat-file", "-s", `${ref}:${file}`], { encoding: "utf8" }));
  } catch {
    return 0;
  }
}

function readBaseRef() {
  const i = process.argv.indexOf("--base");
  if (i !== -1 && process.argv[i + 1]) return process.argv[i + 1];
  if (process.env.GITHUB_BASE_REF) return `origin/${process.env.GITHUB_BASE_REF}`;
  throw new Error("no base ref: pass --base <ref> or set GITHUB_BASE_REF");
}

function kb(bytes) {
  return `${(bytes / 1024).toFixed(1)}KB`;
}
