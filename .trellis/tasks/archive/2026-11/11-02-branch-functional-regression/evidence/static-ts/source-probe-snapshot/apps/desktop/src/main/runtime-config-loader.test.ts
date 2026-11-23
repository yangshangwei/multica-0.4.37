// @vitest-environment node
import { mkdtemp, readFile, rename, rm, writeFile } from "fs/promises";
import { join } from "path";
import { tmpdir } from "os";
import { afterEach, describe, expect, it, vi } from "vitest";
import { loadRuntimeConfig, saveRuntimeConfig } from "./runtime-config-loader";
import { mergeRuntimeConfigInput, parseRuntimeConfig } from "../shared/runtime-config";

vi.mock("fs/promises", async (importOriginal) => {
  const actual = await importOriginal<typeof import("fs/promises")>();
  return { ...actual, rename: vi.fn(actual.rename) };
});

const directories: string[] = [];
afterEach(async () => {
  vi.restoreAllMocks();
  await Promise.all(directories.splice(0).map((dir) => rm(dir, { recursive: true, force: true })));
});

async function configDirectory(): Promise<string> {
  const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
  directories.push(dir);
  return dir;
}

describe("loadRuntimeConfig", () => {
  it("uses dev env and ignores desktop.json during electron-vite dev", async () => {
    const dir = await configDirectory();
    const configPath = join(dir, "desktop.json");
    await writeFile(
      configPath,
      JSON.stringify({ schemaVersion: 1, apiUrl: "https://prod.example.com" }),
    );

    await expect(
      loadRuntimeConfig({
        isDev: true,
        configPath,
        env: {
          apiUrl: "http://localhost:8080",
          wsUrl: "ws://localhost:8080/ws",
          appUrl: "http://localhost:3000",
        },
      }),
    ).resolves.toEqual({
      ok: true,
      source: "dev",
      config: {
        schemaVersion: 1,
        apiUrl: "http://localhost:8080",
        wsUrl: "ws://localhost:8080/ws",
        appUrl: "http://localhost:3000",
      },
    });
    expect(JSON.parse(await readFile(configPath, "utf8"))).not.toHaveProperty("updateUrl");
  });

  it("marks packaged config as private-only when absent", async () => {
    const dir = await configDirectory();
    await expect(
      loadRuntimeConfig({
        isDev: false,
        configPath: join(dir, "missing.json"),
        env: {},
      }),
    ).resolves.toEqual({
      ok: false,
      needsSetup: true,
      error: { message: "Runtime config is not configured" },
    });
  });

  it("parses a valid packaged desktop.json", async () => {
    const dir = await configDirectory();
    const configPath = join(dir, "desktop.json");
    await writeFile(
      configPath,
      JSON.stringify({ schemaVersion: 1, apiUrl: "https://api.example.com" }),
    );

    await expect(
      loadRuntimeConfig({ isDev: false, configPath, env: {} }),
    ).resolves.toEqual({
      ok: true,
      source: "configured",
      config: {
        schemaVersion: 1,
        apiUrl: "https://api.example.com",
        wsUrl: "wss://api.example.com/ws",
        appUrl: "https://example.com",
        updateUrl: "http://api.example.com:18080/desktop",
      },
    });
  });

  it("fails closed when packaged desktop.json is invalid", async () => {
    const dir = await configDirectory();
    const configPath = join(dir, "desktop.json");
    await writeFile(configPath, "{");

    const result = await loadRuntimeConfig({ isDev: false, configPath, env: {} });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.message).toContain(configPath);
      expect(result.error.message).toContain("Invalid desktop runtime config JSON");
    }
  });

  it("writes the default update URL when saving the first server address", async () => {
    const configPath = join(await configDirectory(), "desktop.json");
    const config = parseRuntimeConfig(JSON.stringify(
      mergeRuntimeConfigInput(null, { apiUrl: "https://10.10.10.20:18443" }),
    ));

    await saveRuntimeConfig(config, configPath);

    expect(JSON.parse(await readFile(configPath, "utf8"))).toEqual({
      schemaVersion: 1,
      apiUrl: "https://10.10.10.20:18443",
      wsUrl: "wss://10.10.10.20:18443/ws",
      appUrl: "https://10.10.10.20:18443",
      updateUrl: "http://10.10.10.20:18080/desktop",
    });
  });

  it("backfills old packaged files while preserving all existing fields", async () => {
    const configPath = join(await configDirectory(), "desktop.json");
    const original = {
      schemaVersion: 1,
      apiUrl: "https://10.10.10.20:18443",
      appUrl: "https://web.example.com",
      wsUrl: "wss://socket.example.com/ws",
      extra: { preserved: true },
    };
    await writeFile(configPath, JSON.stringify(original));

    await loadRuntimeConfig({ isDev: false, configPath, env: {} });

    expect(JSON.parse(await readFile(configPath, "utf8"))).toEqual({
      ...original,
      updateUrl: "http://10.10.10.20:18080/desktop",
    });
  });

  it("does not rewrite an existing explicit update source", async () => {
    const configPath = join(await configDirectory(), "desktop.json");
    const raw = '{"schemaVersion":1,"apiUrl":"https://api.example.com","updateUrl":"https://updates.example.com/custom/","extra":true}\n';
    await writeFile(configPath, raw);

    const result = await loadRuntimeConfig({ isDev: false, configPath, env: {} });

    expect(result).toMatchObject({ ok: true, config: { updateUrl: "https://updates.example.com/custom" } });
    expect(await readFile(configPath, "utf8")).toBe(raw);
  });

  it("uses valid configuration even when persisting the missing field fails", async () => {
    const configPath = join(await configDirectory(), "desktop.json");
    const raw = '{"schemaVersion":1,"apiUrl":"https://api.example.com"}';
    await writeFile(configPath, raw);
    const error = new Error("Read-only configuration directory");
    vi.mocked(rename).mockRejectedValueOnce(error);
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    const result = await loadRuntimeConfig({ isDev: false, configPath, env: {} });

    expect(result).toMatchObject({ ok: true, config: { updateUrl: "http://api.example.com:18080/desktop" } });
    expect(await readFile(configPath, "utf8")).toBe(raw);
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("updateUrl"), error);
  });

});
