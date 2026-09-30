// @vitest-environment node
import { execFile, type ExecFileException, type ExecFileOptions } from "node:child_process";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { isTrayEnvironmentSupported } from "./tray";

vi.mock("node:child_process", () => ({ execFile: vi.fn() }));

function reply(error: ExecFileException | null, stdout = "") {
  vi.mocked(execFile).mockImplementationOnce(((_file: string, _args: string[], _options: ExecFileOptions,
    callback: (error: ExecFileException | null, stdout: string, stderr: string) => void) => {
    callback(error, stdout, "");
  }) as typeof execFile);
}

beforeEach(() => {
  vi.stubGlobal("process", { ...process, platform: "linux" });
  vi.mocked(execFile).mockReset();
});
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

describe("isTrayEnvironmentSupported", () => {
  it.each(["x11", "wayland"])("supports GNOME %s when a host is registered", async (session) => {
    vi.stubEnv("XDG_CURRENT_DESKTOP", "GNOME");
    vi.stubEnv("XDG_SESSION_TYPE", session);
    reply(null, "(<true>,)\n");
    expect(await isTrayEnvironmentSupported()).toBe(true);
    expect(execFile).toHaveBeenCalledWith("gdbus", [
      "call", "--session", "--dest", "org.kde.StatusNotifierWatcher",
      "--object-path", "/StatusNotifierWatcher", "--method", "org.freedesktop.DBus.Properties.Get",
      "org.kde.StatusNotifierWatcher", "IsStatusNotifierHostRegistered", "--timeout", "1",
    ], expect.objectContaining({ timeout: 1500, maxBuffer: 4096, killSignal: "SIGKILL" }), expect.any(Function));
  });

  it.each(["GNOME", "KDE", "XFCE", ""])("rejects %s without a registered host", async (desktop) => {
    vi.stubEnv("XDG_CURRENT_DESKTOP", desktop);
    vi.stubEnv("XDG_SESSION_TYPE", "x11");
    reply(null, "(<false>,)\n");
    expect(await isTrayEnvironmentSupported()).toBe(false);
  });

  it.each(["", "true", "(<false>,)\ntrue", "('true',)"])("rejects malformed output %j", async (output) => {
    reply(null, output);
    expect(await isTrayEnvironmentSupported()).toBe(false);
  });

  it("uses busctl when gdbus is not installed", async () => {
    reply(Object.assign(new Error("not found"), { code: "ENOENT" }));
    reply(null, "b true\n");
    expect(await isTrayEnvironmentSupported()).toBe(true);
    expect(execFile).toHaveBeenNthCalledWith(2, "busctl", [
      "--user", "--timeout=1s", "get-property", "org.kde.StatusNotifierWatcher",
      "/StatusNotifierWatcher", "org.kde.StatusNotifierWatcher", "IsStatusNotifierHostRegistered",
    ], expect.objectContaining({ timeout: 1500, maxBuffer: 4096, killSignal: "SIGKILL" }), expect.any(Function));
  });

  it.each(["b false\n", "true\n", "b true\nnoise"])("rejects an unsupported or malformed busctl result %j", async (output) => {
    reply(Object.assign(new Error("not found"), { code: "ENOENT" }));
    reply(null, output);
    expect(await isTrayEnvironmentSupported()).toBe(false);
  });

  it("reports unsupported when neither D-Bus client is installed", async () => {
    reply(Object.assign(new Error("not found"), { code: "ENOENT" }));
    reply(Object.assign(new Error("not found"), { code: "ENOENT" }));
    expect(await isTrayEnvironmentSupported()).toBe(false);
  });

  it.each(["ETIMEDOUT", "EACCES", 1])("fails closed on a D-Bus error %s without trying another client", async (code) => {
    reply(Object.assign(new Error("probe failed"), { code }));
    expect(await isTrayEnvironmentSupported()).toBe(false);
    expect(execFile).toHaveBeenCalledTimes(1);
  });

  it.each([["darwin", false], ["win32", true], ["freebsd", false]] as const)("returns %s capability without invoking D-Bus", async (platform, expected) => {
    vi.stubGlobal("process", { ...process, platform });
    expect(await isTrayEnvironmentSupported()).toBe(expected);
    expect(execFile).not.toHaveBeenCalled();
  });
});
