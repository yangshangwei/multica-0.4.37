import { execFile, type ExecFileException } from "node:child_process";

const WATCHER = "org.kde.StatusNotifierWatcher";
const WATCHER_PATH = "/StatusNotifierWatcher";
const HOST_PROPERTY = "IsStatusNotifierHostRegistered";

function queryHost(command: string, args: string[]): Promise<{
  error: ExecFileException | null;
  output: string;
}> {
  return new Promise((resolve) => {
    execFile(command, args, {
      encoding: "utf8",
      timeout: 1500,
      maxBuffer: 4096,
      killSignal: "SIGKILL",
    }, (error, stdout) => resolve({ error, output: stdout.trim() }));
  });
}

/** Probe the session's actual StatusNotifier host without creating an icon. */
export async function isTrayEnvironmentSupported(): Promise<boolean> {
  if (process.platform === "win32") return true;
  if (process.platform !== "linux") return false;

  const result = await queryHost("gdbus", [
    "call", "--session", "--dest", WATCHER,
    "--object-path", WATCHER_PATH, "--method", "org.freedesktop.DBus.Properties.Get",
    WATCHER, HOST_PROPERTY, "--timeout", "1",
  ]);
  if (!result.error) return result.output === "(<true>,)";
  if (result.error.code !== "ENOENT") return false;

  const fallback = await queryHost("busctl", [
    "--user", "--timeout=1s", "get-property", WATCHER,
    WATCHER_PATH, WATCHER, HOST_PROPERTY,
  ]);
  return !fallback.error && fallback.output === "b true";
}
