import { app } from "electron";
import { mkdir, readFile, rename, unlink, writeFile } from "fs/promises";
import { dirname, join } from "path";
import {
  parseRuntimeConfig,
  runtimeConfigFromDevEnv,
  type RuntimeConfig,
  type RuntimeConfigEnv,
  type RuntimeConfigResult,
} from "../shared/runtime-config";

export async function loadRuntimeConfig(options: {
  isDev: boolean;
  env: RuntimeConfigEnv;
  configPath?: string;
}): Promise<RuntimeConfigResult> {
  if (options.isDev) {
    try {
      return { ok: true, source: "dev", config: runtimeConfigFromDevEnv(options.env) };
    } catch (err) {
      return { ok: false, error: { message: errorMessage(err) } };
    }
  }

  const configPath = options.configPath ?? desktopConfigPath();
  try {
    const raw = await readFile(configPath, "utf-8");
    return { ok: true, source: "configured", config: parseRuntimeConfig(raw) };
  } catch (err) {
    if (isMissingFileError(err)) {
      return {
        ok: false,
        needsSetup: true,
        error: { message: "Runtime config is not configured" },
      };
    }
    return {
      ok: false,
      error: {
        message: `Invalid ${configPath}: ${errorMessage(err)}`,
      },
    };
  }
}

export async function saveRuntimeConfig(
  config: RuntimeConfig,
  configPath = desktopConfigPath(),
): Promise<RuntimeConfig> {
  const normalized = parseRuntimeConfig(JSON.stringify(config));
  const dir = dirname(configPath);
  await mkdir(dir, { recursive: true });
  const tempPath = `${configPath}.${process.pid}.${Date.now()}.tmp`;
  try {
    await writeFile(tempPath, `${JSON.stringify(normalized, null, 2)}\n`, {
      encoding: "utf-8",
      mode: 0o600,
    });
    await rename(tempPath, configPath);
  } catch (error) {
    try { await unlink(tempPath); } catch { /* best effort */ }
    throw error;
  }
  return normalized;
}

export function desktopConfigPath(): string {
  return join(app.getPath("home"), ".multica", "desktop.json");
}

function isMissingFileError(err: unknown): boolean {
  return Boolean(
    err &&
      typeof err === "object" &&
      "code" in err &&
      (err as NodeJS.ErrnoException).code === "ENOENT",
  );
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export type { RuntimeConfig, RuntimeConfigResult };
