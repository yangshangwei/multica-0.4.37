import path from "node:path";

/** Paths for an explicitly selected native package, never an ambient install. */
export function nativeInstalledLayout(executable, platform = process.platform) {
  if (!["darwin", "win32"].includes(platform)) throw new Error("Native packaged Main requires Windows or macOS");
  const paths = platform === "win32" ? path.win32 : path.posix;
  localAbsolute(executable, platform, "executable");
  executable = paths.resolve(executable);
  if (platform === "win32" ? paths.basename(executable).toLowerCase() !== "multica.exe"
    : !/\.app\/Contents\/MacOS\/Multica$/.test(executable)) {
    throw new Error(platform === "win32" ? "The installed executable must be Multica.exe" : "The app must use Contents/MacOS/Multica");
  }
  const resources = platform === "win32" ? paths.join(paths.dirname(executable), "resources") : paths.resolve(executable, "../..", "Resources");
  const asar = paths.join(resources, "app.asar");
  const rendererUrl = new URL("file:///");
  rendererUrl.pathname = `${platform === "win32" ? "/" : ""}${paths.join(asar, "out", "renderer", "index.html").replaceAll("\\", "/")}`;
  return { resources, asar, cli: paths.join(`${asar}.unpacked`, "resources", "bin", platform === "win32" ? "multica.exe" : "multica"),
    rendererUrl: rendererUrl.href, platform, apiOS: platform === "win32" ? "windows" : "macos" };
}

export function nativeAcceptanceHome(home, platform = process.platform) {
  if (!["darwin", "win32"].includes(platform)) throw new Error("Native packaged Main requires Windows or macOS");
  const paths = platform === "win32" ? path.win32 : path.posix;
  localAbsolute(home, platform, "acceptance home");
  return paths.resolve(home);
}

function localAbsolute(value, platform, name) {
  if (typeof value !== "string" || /[\0\r\n]/.test(value) || (platform === "win32" ? !/^[a-z]:[\\/]/i.test(value) : !path.posix.isAbsolute(value))) {
    throw new Error(`Native packaged Main requires an absolute local ${name}`);
  }
}
