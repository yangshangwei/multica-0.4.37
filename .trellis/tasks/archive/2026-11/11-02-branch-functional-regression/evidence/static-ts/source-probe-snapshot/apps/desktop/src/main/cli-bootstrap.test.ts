// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import * as fsPromises from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript";
import * as diagnostics from "./daemon-diagnostics";

const nativeRequire = createRequire(import.meta.url);
const roots: string[] = [];
afterEach(async () => { await Promise.all(roots.splice(0).map(root => rm(root, { recursive: true, force: true }))); });

it.each(["download", "checksum", "extract", "install", "success"])("retains the observed CLI setup stage: %s", async failure => {
  const root = await mkdtemp(join(tmpdir(), "cli-diagnostics-")); roots.push(root);
  const checksum = createHash("sha256").update("archive").digest("hex");
  const spawn = vi.fn((_file: string, args: string[], _options: unknown, callback: (error: Error | null) => void) => {
    if (failure === "extract") { callback(new Error("sensitive fixture process error")); return; }
    void writeFile(join(args[3]!, "multica.exe"), "fixture binary").then(() => callback(null));
  });
  const modules: Record<string, unknown> = {
    electron: { app: { getPath: () => root } },
    child_process: { execFile: spawn },
    os: { tmpdir: () => root },
    "fs/promises": { ...fsPromises, rename: failure === "install" ? async () => { throw new Error("sensitive fixture path"); } : fsPromises.rename },
    "./daemon-diagnostics": diagnostics,
    "./cli-release-asset": { selectPlatformReleaseAssetName: () => "fixture.zip" },
  };
  const source = await readFile(join(__dirname, "cli-bootstrap.ts"), "utf8");
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const module = { exports: {} as { ensureManagedCli(): Promise<string> } };
  runInNewContext(compiled, {
    module, exports: module.exports, require: (name: string) => name in modules ? modules[name] : nativeRequire(name),
    process: { platform: "win32" }, console: { log: vi.fn(), warn: vi.fn() },
    fetch: async (url: string) => {
      if (failure === "download") throw new Error("sensitive fixture URL");
      if (url.endsWith("checksums.txt")) return new Response(`${failure === "checksum" ? "0".repeat(64) : checksum}  fixture.zip`);
      return new Response("archive");
    },
  });
  if (failure === "success") {
    expect(await module.exports.ensureManagedCli()).toBe(join(root, "bin", "multica.exe"));
    expect(await readFile(join(root, "bin", "multica.exe"), "utf8")).toBe("fixture binary");
  } else {
    await expect(module.exports.ensureManagedCli()).rejects.toMatchObject({ code: `cli_${failure}_failed`, stage: "cli_setup" });
  }
  if (failure === "download" || failure === "checksum") expect(spawn).not.toHaveBeenCalled();
});
