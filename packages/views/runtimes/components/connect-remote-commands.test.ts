// @vitest-environment node

import { describe, expect, it } from "vitest";
import { connectionCommands, installCommand } from "./connect-remote-commands";

describe("remote computer commands", () => {
  it("uses the published installer for each shell", () => {
    expect(installCommand("windows", {}, "")).toMatch(/^irm .*install\.ps1 \| iex$/);
    for (const platform of ["macos", "linux"] as const) {
      expect(installCommand(platform, {}, "")).toMatch(/^curl .*install\.sh \| bash$/);
    }
  });

  it("never substitutes a public installer for a missing custom platform", () => {
    expect(installCommand("windows", {}, "internal-unix-install")).toBeNull();
    expect(installCommand("linux", { windows: "internal-windows" }, "")).toBeNull();
    expect(installCommand("linux", {}, "internal-unix-install")).toBe("internal-unix-install");
    expect(installCommand("macos", { macos: "explicit-mac" }, "legacy")).toBe("explicit-mac");
  });

  it.each([
    "http://localhost:8080", "http://api.localhost", "http://localhost.",
    "http://127.2.3.4", "http://127.1", "http://0x7f000001", "http://0.0.0.0",
    "http://[::1]", "http://[::]", "http://[::ffff:127.0.0.1]",
    "http://[::ffff:0.0.0.0]", "http://[0:0:0:0:0:ffff:7f00:1]", "http://[::ffff:0:1]",
    "https://*.example.com", "file:///tmp/server", "https:/api.example.com", "https:api.example.com",
    "//api.example.com", "https://api.example.com/path\\query",
    "https://user:password@example.com", "https://api.example.com\n/path", "not a URL",
  ])("blocks unsafe or inaccessible service URL %s", (url) => {
    expect(connectionCommands("linux", url, "https://app.example.com")).toBeNull();
    expect(connectionCommands("windows", "https://api.example.com", url)).toBeNull();
  });

  it("blocks partial configuration without falling back to cloud", () => {
    expect(connectionCommands("macos", "https://api.example.com", "")).toBeNull();
    expect(connectionCommands("macos", "", "https://app.example.com")).toBeNull();
  });

  it.each(["https://127.api.example.com", "https://0.packages.internal"])(
    "accepts a remote DNS host that only shares an IPv4 prefix: %s",
    (url) => {
      expect(connectionCommands("linux", url, "https://app.example.com")?.setup)
        .toContain(`--server-url '${url}'`);
      expect(connectionCommands("windows", "https://api.example.com", url)?.setup)
        .toContain(`--app-url '${url}'`);
    },
  );

  it.each(["\n", "\r", "\t", "\u0000", "\u007f"])(
    "does not mistake a control-only service address for absent cloud configuration: %j",
    (value) => {
      expect(connectionCommands("linux", value, "")).toBeNull();
      expect(connectionCommands("windows", "", value)).toBeNull();
    },
  );

  it("normalizes remote URLs and quotes Unix shell metacharacters", () => {
    expect(connectionCommands("linux", " https://api.example.com/ ", "https://app.example.com/a'b?x=$HOME&y=1")?.setup)
      .toBe("multica setup self-host --server-url 'https://api.example.com' --app-url 'https://app.example.com/a'\"'\"'b?x=$HOME&y=1'");
  });

  it("quotes PowerShell URL arguments and keeps interactive commands separate", () => {
    const commands = connectionCommands("windows", "https://api.example.com", "https://app.example.com/a'b");
    expect(commands?.setup).toContain("--app-url 'https://app.example.com/a''b'");
    expect(commands?.configure).toBe("multica config set server_url 'https://api.example.com'\nmultica config set app_url 'https://app.example.com/a''b'");
    expect(commands?.configure).not.toContain("login");
    expect(commands?.login).toBe("multica login --token");
    expect(commands?.start).toBe("multica daemon start");
  });

  it("uses cloud setup only when neither service address is configured", () => {
    const commands = connectionCommands("macos", "", "");
    expect(commands?.setup).toBe("multica setup");
    expect(commands?.configure).toContain("'https://api.multica.ai'");
    expect(commands?.configure).toContain("'https://multica.ai'");
    expect(connectionCommands("linux", " ", "  ")?.setup).toBe("multica setup");
  });
});
