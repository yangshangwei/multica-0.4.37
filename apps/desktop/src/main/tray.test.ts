import { describe, expect, it } from "vitest";
import { buildTrayIconPath, isTrayEnvironmentSupported } from "./tray";

describe("isTrayEnvironmentSupported", () => {
  it("returns false on darwin (tray replaced by dock)", () => {
    expect(
      isTrayEnvironmentSupported({}, "darwin"),
    ).toBe(false);
  });

  it("returns true on win32", () => {
    expect(isTrayEnvironmentSupported({}, "win32")).toBe(true);
  });

  it("returns true on Linux KDE", () => {
    expect(
      isTrayEnvironmentSupported(
        { XDG_CURRENT_DESKTOP: "KDE", XDG_SESSION_TYPE: "wayland" },
        "linux",
      ),
    ).toBe(true);
  });

  it("returns true on Linux XFCE (X11)", () => {
    expect(
      isTrayEnvironmentSupported(
        { XDG_CURRENT_DESKTOP: "XFCE", XDG_SESSION_TYPE: "x11" },
        "linux",
      ),
    ).toBe(true);
  });

  it("returns true on Linux GNOME + X11", () => {
    expect(
      isTrayEnvironmentSupported(
        { XDG_CURRENT_DESKTOP: "GNOME", XDG_SESSION_TYPE: "x11" },
        "linux",
      ),
    ).toBe(true);
  });

  it("returns false on Linux GNOME + Wayland (no AppIndicator by default)", () => {
    expect(
      isTrayEnvironmentSupported(
        { XDG_CURRENT_DESKTOP: "GNOME", XDG_SESSION_TYPE: "wayland" },
        "linux",
      ),
    ).toBe(false);
  });

  it("returns false on Ubuntu GNOME (case-insensitive)", () => {
    expect(
      isTrayEnvironmentSupported(
        { XDG_CURRENT_DESKTOP: "ubuntu:GNOME", XDG_SESSION_TYPE: "wayland" },
        "linux",
      ),
    ).toBe(false);
  });

  it("returns true on Cinnamon", () => {
    expect(
      isTrayEnvironmentSupported(
        { XDG_CURRENT_DESKTOP: "X-Cinnamon", XDG_SESSION_TYPE: "x11" },
        "linux",
      ),
    ).toBe(true);
  });

  it("defaults to true on Linux when env is missing (headless X11 / unknown)", () => {
    expect(isTrayEnvironmentSupported({}, "linux")).toBe(true);
  });
});

describe("buildTrayIconPath", () => {
  it("dev: returns <app>/resources/icon.png", () => {
    const path = buildTrayIconPath({
      isDev: true,
      resourcesPath: "/repo/apps/desktop",
      platform: "win32",
    });
    expect(path).toMatch(/apps[\\/]desktop[\\/]resources[\\/]icon\.png$/);
  });

  it("linux dev also returns resources/icon.png", () => {
    const path = buildTrayIconPath({
      isDev: true,
      resourcesPath: "/repo/apps/desktop",
      platform: "linux",
    });
    expect(path).toMatch(/resources[\\/]icon\.png$/);
  });

  it("prod: returns <resourcesPath>/app.asar.unpacked/resources/icon.png", () => {
    const path = buildTrayIconPath({
      isDev: false,
      resourcesPath: "/opt/Multica/resources",
      platform: "linux",
    });
    expect(path).toBe(
      "/opt/Multica/resources/app.asar.unpacked/resources/icon.png",
    );
  });

  it("windows prod: same unified path", () => {
    const path = buildTrayIconPath({
      isDev: false,
      resourcesPath: "C:\\Program Files\\Multica\\resources",
      platform: "win32",
    });
    expect(path).toMatch(/app\.asar\.unpacked/);
    expect(path).toMatch(/resources[\\/]icon\.png$/);
  });
});
