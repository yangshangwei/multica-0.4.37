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
    const config = parseRuntimeConfig(raw);
    // The parser validated the object. Migrate only the missing field so
    // operator-owned and future fields survive the first upgraded launch.
    const stored = JSON.parse(raw) as Record<string, unknown>;
    if (stored.updateUrl === undefined) {
      try {
        await writeRuntimeConfig({ ...stored, updateUrl: config.updateUrl }, configPath);
      } catch (error) {
        // A read-only file must not block a valid business connection. The
        // derived URL still works in memory; retry persistence on next launch.
        console.warn(`Could not persist updateUrl in ${configPath}:`, error);
      }
    }
    return { ok: true, source: "configured", config };
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
  await writeRuntimeConfig(normalized, configPath);
  return normalized;
}

async function writeRuntimeConfig(config: object, configPath: string): Promise<void> {
  const dir = dirname(configPath);
  await mkdir(dir, { recursive: true });
  const tempPath = `${configPath}.${process.pid}.${Date.now()}.tmp`;
  try {
    await writeFile(tempPath, `${JSON.stringify(config, null, 2)}\n`, {
      encoding: "utf-8",
      mode: 0o600,
    });
    await rename(tempPath, configPath);
  } catch (error) {
    try { await unlink(tempPath); } catch { /* best effort */ }
    throw error;
  }
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
