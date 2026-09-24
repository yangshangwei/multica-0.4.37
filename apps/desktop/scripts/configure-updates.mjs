import { randomUUID } from "node:crypto";
import { readFile, rename, unlink, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";
import { parseRuntimeConfig } from "../src/shared/runtime-config.ts";

export async function configureUpdates(configPath, updateUrl) {
  const original = await readFile(configPath, "utf8");
  parseRuntimeConfig(original);
  const existing = JSON.parse(original);
  const normalized = parseRuntimeConfig(JSON.stringify({ ...existing, updateUrl }));
  if (!normalized.updateUrl) throw new Error("updateUrl must be a non-empty HTTP/HTTPS URL");
  if (existing.updateUrl === normalized.updateUrl) return { changed: false, updateUrl: normalized.updateUrl };

  const suffix = randomUUID();
  const backupPath = `${configPath}.${suffix}.bak`;
  const tempPath = `${configPath}.${suffix}.tmp`;
  await writeFile(backupPath, original, { mode: 0o600, flag: "wx" });
  try {
    await writeFile(tempPath, `${JSON.stringify({ ...existing, updateUrl: normalized.updateUrl }, null, 2)}\n`, { mode: 0o600, flag: "wx" });
    await rename(tempPath, configPath);
  } finally {
    await unlink(tempPath).catch((error) => { if (error.code !== "ENOENT") throw error; });
  }
  return { changed: true, backupPath, updateUrl: normalized.updateUrl };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const { values } = parseArgs({ options: { url: { type: "string" }, config: { type: "string", default: join(homedir(), ".multica", "desktop.json") }, help: { type: "boolean" } } });
    if (values.help) {
      console.log("Usage: node apps/desktop/scripts/configure-updates.mjs --url HTTP_URL [--config PATH]\nRequires Node >=22.18 and an existing Desktop server configuration. Quit Desktop before changing it.");
    } else {
      if (!values.url) throw new Error("--url is required");
      const result = await configureUpdates(resolve(values.config), values.url);
      console.log(JSON.stringify({ configPath: resolve(values.config), ...result }, null, 2));
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
