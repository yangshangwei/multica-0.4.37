// @vitest-environment node
import { describe, expect, it } from "vitest";

import { selectPlatformReleaseAssetName } from "./cli-release-asset";

describe("selectPlatformReleaseAssetName", () => {
  it("prefers the versioned archive name when both exist", () => {
    const assetNames = [
      "checksums.txt",
      "multica_darwin_amd64.tar.gz",
      "multica-cli-1.2.3-darwin-amd64.tar.gz",
    ];

    expect(selectPlatformReleaseAssetName(assetNames, "darwin", "x64")).toBe(
      "multica-cli-1.2.3-darwin-amd64.tar.gz",
    );
  });

  it("falls back to the legacy archive name when only legacy is present", () => {
    const assetNames = ["checksums.txt", "multica_darwin_amd64.tar.gz"];

    expect(selectPlatformReleaseAssetName(assetNames, "darwin", "x64")).toBe(
      "multica_darwin_amd64.tar.gz",
    );
  });

  it("matches the renamed darwin archive from release assets", () => {
    const assetNames = [
      "checksums.txt",
      "multica-cli-1.2.3-darwin-amd64.tar.gz",
      "multica-cli-1.2.3-darwin-arm64.tar.gz",
      "multica-cli-1.2.3-linux-amd64.tar.gz",
    ];

    expect(selectPlatformReleaseAssetName(assetNames, "darwin", "x64")).toBe(
      "multica-cli-1.2.3-darwin-amd64.tar.gz",
    );
  });

  it("matches the renamed windows zip archive", () => {
    const assetNames = [
      "multica-cli-1.2.3-windows-amd64.zip",
      "multica-cli-1.2.3-linux-amd64.tar.gz",
    ];

    expect(selectPlatformReleaseAssetName(assetNames, "win32", "x64")).toBe(
      "multica-cli-1.2.3-windows-amd64.zip",
    );
  });

  it("selects a Windows 386 CLI for an ia32 desktop from mixed-architecture assets", () => {
    expect(
      selectPlatformReleaseAssetName([
        "multica-cli-0.4.46-windows-amd64.zip",
        "multica-cli-0.4.46-windows-386.zip",
        "multica-cli-0.4.46-windows-arm64.zip",
        "multica_windows_386.zip",
      ], "win32", "ia32"),
    ).toBe("multica-cli-0.4.46-windows-386.zip");
  });

  it("can select the legacy Windows 386 CLI archive", () => {
    expect(selectPlatformReleaseAssetName(["multica_windows_386.zip"], "win32", "ia32")).toBe(
      "multica_windows_386.zip",
    );
  });

  it("does not substitute an x64 CLI when a Windows 386 archive is missing", () => {
    expect(() => selectPlatformReleaseAssetName([
      "multica-cli-0.4.46-windows-amd64.zip",
      "multica_windows_amd64.zip",
    ], "win32", "ia32")).toThrow(/no release asset found.*windows-386/);
  });

  it.each(["darwin", "linux"] as const)("rejects ia32 CLI installation on %s", (platform) => {
    expect(() => selectPlatformReleaseAssetName([
      `multica-cli-0.4.46-${platform}-386.tar.gz`,
    ], platform, "ia32")).toThrow(/unsupported platform/);
  });

  it("fails when the current platform asset is missing", () => {
    expect(() =>
      selectPlatformReleaseAssetName(
        ["multica-cli-1.2.3-linux-amd64.tar.gz", "multica_linux_amd64.tar.gz"],
        "darwin",
        "arm64",
      ),
    ).toThrow(/no release asset found/);
  });
});
