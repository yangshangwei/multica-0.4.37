// @vitest-environment node
import { mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, expect, it } from "vitest";
import { configureUpdates } from "./configure-updates.mjs";

const directories = [];
afterEach(async () => {
  await Promise.all(directories.splice(0).map((dir) => rm(dir, { recursive: true, force: true })));
});

async function fixture(raw = JSON.stringify({ schemaVersion: 1, apiUrl: "http://localhost:8080", appUrl: "http://localhost:3000", extra: { preserved: true } })) {
  const dir = await mkdtemp(join(tmpdir(), "desktop-update-config-"));
  directories.push(dir);
  const file = join(dir, "desktop.json");
  await writeFile(file, raw);
  return { dir, file, raw };
}

it("preserves business configuration and unknown fields, backs up exact bytes, and normalizes the update URL", async () => {
  const { file, raw } = await fixture();
  const result = await configureUpdates(file, "http://127.0.0.1:18080/desktop/");
  expect(JSON.parse(await readFile(file, "utf8"))).toEqual({ ...JSON.parse(raw), updateUrl: "http://127.0.0.1:18080/desktop" });
  expect(await readFile(result.backupPath, "utf8")).toBe(raw);
});

it.each(["file:///tmp/update", "https://user:password@example.test/desktop", "not-a-url"])("rejects invalid update URL %s without touching the file", async (url) => {
  const { dir, file, raw } = await fixture();
  await expect(configureUpdates(file, url)).rejects.toThrow();
  expect(await readFile(file, "utf8")).toBe(raw);
  expect(await readdir(dir)).toEqual(["desktop.json"]);
});

it.each(["{", "[]", '{"schemaVersion":2,"apiUrl":"http://localhost"}', '{"schemaVersion":1}'])("does not replace invalid existing business configuration: %s", async (raw) => {
  const { file } = await fixture(raw);
  await expect(configureUpdates(file, "http://localhost:18080/desktop")).rejects.toThrow();
  expect(await readFile(file, "utf8")).toBe(raw);
});

it("does not invent missing business configuration", async () => {
  const { file } = await fixture();
  await rm(file);
  await expect(configureUpdates(file, "http://localhost:18080/desktop")).rejects.toThrow();
});

it("does not create backups or rewrite when configuration already matches", async () => {
  const { dir, file } = await fixture();
  await configureUpdates(file, "http://localhost:18080/desktop");
  const before = await readdir(dir);
  expect((await configureUpdates(file, "http://localhost:18080/desktop/")).changed).toBe(false);
  expect(await readdir(dir)).toEqual(before);
});
