// @vitest-environment node
import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

const scriptPath = [
  resolve(process.cwd(), "scripts/bundle-cli.mjs"),
  resolve(process.cwd(), "apps/desktop/scripts/bundle-cli.mjs"),
].find((candidate) => existsSync(candidate));
const fixtures = [];

afterEach(() => {
  while (fixtures.length) rmSync(fixtures.pop(), { recursive: true, force: true });
});

function bundle(platform, arch) {
  const root = mkdtempSync(join(tmpdir(), "multica-bundle-cli-test-"));
  fixtures.push(root);
  const script = join(root, "apps/desktop/scripts/bundle-cli.mjs");
  mkdirSync(dirname(script), { recursive: true });
  copyFileSync(scriptPath, script);
  const loader = join(root, "fake-build-tools.mjs");
  // Exercise the real bundling entry point in an isolated tree. The loader
  // replaces only external build tools; filesystem staging/copying stays real.
  writeFileSync(loader, `
import childProcess from "node:child_process";
import { writeFileSync } from "node:fs";
import { syncBuiltinESMExports } from "node:module";
childProcess.execSync = (command) => {
  if (command === "go version") return "go version test-toolchain";
  if (command.startsWith("codesign ")) return "";
  throw new Error("Unexpected shell command: " + command);
};
childProcess.execFileSync = (command, args, options) => {
  if (command === "git") {
    if (args[0] === "rev-parse") return "123456789";
    if (args[0] === "describe") return "v0.4.46";
  }
  if (command === "go" && args[0] === "build") {
    const output = args[args.indexOf("-o") + 1];
    writeFileSync(output, JSON.stringify({
      goos: options.env.GOOS,
      goarch: options.env.GOARCH,
      cgo: options.env.CGO_ENABLED,
      args,
    }));
    return "";
  }
  throw new Error("Unexpected executable: " + command);
};
syncBuiltinESMExports();
`);
  const result = spawnSync(process.execPath, [
    "--import", loader, script,
    "--target-platform", platform,
    "--target-arch", arch,
  ], { cwd: root, encoding: "utf8" });
  return { root, result, cli: join(root, "apps/desktop/resources/bin/multica.exe") };
}

describe("CLI bundling target architecture", () => {
  it.each([
    ["ia32", "386"],
    ["x64", "amd64"],
    ["arm64", "arm64"],
  ])("stages the matching Windows CLI for %s", (arch, goarch) => {
    const { result, cli } = bundle("win32", arch);

    expect(result.status, result.stderr).toBe(0);
    const compiled = JSON.parse(readFileSync(cli, "utf8"));
    expect(compiled).toMatchObject({ goos: "windows", goarch, cgo: "0" });
    expect(compiled.args).toContain("./cmd/multica");
    expect(compiled.args[compiled.args.indexOf("-ldflags") + 1]).toContain("-X main.version=v0.4.46");
  });

  it.each(["darwin", "linux"])("rejects an ia32 CLI for %s before building or staging", (platform) => {
    const { root, result, cli } = bundle(platform, "ia32");

    expect(result.status).not.toBe(0);
    expect(result.stderr).toMatch(/ia32.*win32/i);
    expect(existsSync(join(root, "server/bin"))).toBe(false);
    expect(existsSync(cli)).toBe(false);
  });
});
