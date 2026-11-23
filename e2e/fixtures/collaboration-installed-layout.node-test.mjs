import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { nativeAcceptanceHome, nativeInstalledLayout } from "./collaboration-installed-layout.mjs";

test("Windows uses the installed EXE resources and bundled EXE with an encoded file URL", () => {
  const layout = nativeInstalledLayout("C:\\Runner Temp\\installed\\Multica.exe", "win32");
  assert.deepEqual(layout, { resources: "C:\\Runner Temp\\installed\\resources", asar: "C:\\Runner Temp\\installed\\resources\\app.asar",
    cli: "C:\\Runner Temp\\installed\\resources\\app.asar.unpacked\\resources\\bin\\multica.exe",
    rendererUrl: "file:///C:/Runner%20Temp/installed/resources/app.asar/out/renderer/index.html", platform: "win32", apiOS: "windows" });
});

test("macOS preserves the app Contents layout and encodes package names", () => {
  const layout = nativeInstalledLayout("/task/Multica Candidate.app/Contents/MacOS/Multica", "darwin");
  assert.equal(layout.resources, "/task/Multica Candidate.app/Contents/Resources");
  assert.equal(layout.cli, "/task/Multica Candidate.app/Contents/Resources/app.asar.unpacked/resources/bin/multica");
  assert.equal(layout.rendererUrl, "file:///task/Multica%20Candidate.app/Contents/Resources/app.asar/out/renderer/index.html");
  assert.equal(layout.apiOS, "macos");
});

test("native renderer URLs round-trip literal percent sequences, hashes and spaces as filesystem text", () => {
  for (const { platform, executable, renderer } of [
    { platform: "win32", executable: "C:\\Runner%20Temp\\installed\\Multica.exe",
      renderer: "C:\\Runner%20Temp\\installed\\resources\\app.asar\\out\\renderer\\index.html" },
    { platform: "darwin", executable: "/task/Multica%20Candidate.app/Contents/MacOS/Multica",
      renderer: "/task/Multica%20Candidate.app/Contents/Resources/app.asar/out/renderer/index.html" },
    { platform: "win32", executable: "C:\\Runner%20 % #\\installed\\Multica.exe",
      renderer: "C:\\Runner%20 % #\\installed\\resources\\app.asar\\out\\renderer\\index.html" },
    { platform: "darwin", executable: "/task%2F % #/Multica%20Candidate.app/Contents/MacOS/Multica",
      renderer: "/task%2F % #/Multica%20Candidate.app/Contents/Resources/app.asar/out/renderer/index.html" },
  ]) {
    const layout = nativeInstalledLayout(executable, platform);
    assert.equal(fileURLToPath(layout.rendererUrl, { windows: platform === "win32" }), renderer);
    assert(layout.rendererUrl.includes("%2520"), "A literal percent-escape-shaped name is escaped only once as filesystem text");
    if (renderer.includes("% #")) assert(layout.rendererUrl.includes("%25%20%23"), "Literal percent, space and hash stay in the same path component");
    assert.equal(new URL(layout.rendererUrl).hash, "");
  }
});

test("native layouts reject unsupported platforms and paths outside a supported package shape", () => {
  assert.throws(() => nativeInstalledLayout("/task/multica", "linux"), /Windows or macOS/);
  for (const executable of ["Multica.exe", "C:Multica.exe", "\\\\host\\share\\Multica.exe", "C:\\task\\other.exe"]) {
    assert.throws(() => nativeInstalledLayout(executable, "win32"), /absolute local|Multica.exe/);
  }
  assert.throws(() => nativeInstalledLayout("/task/Multica", "darwin"), /Contents\/MacOS/);
});

test("native homes use platform path rules and reject network or relative roots", () => {
  assert.equal(nativeAcceptanceHome("C:\\task\\home", "win32"), "C:\\task\\home");
  assert.equal(nativeAcceptanceHome("/task/home", "darwin"), "/task/home");
  for (const home of [undefined, "home", "C:home", "\\\\host\\share\\home", "\\home"]) {
    assert.throws(() => nativeAcceptanceHome(home, "win32"), /absolute local|absolute acceptance home/);
  }
});
