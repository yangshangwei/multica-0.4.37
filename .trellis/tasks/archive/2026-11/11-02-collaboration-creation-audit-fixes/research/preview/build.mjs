import { build } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const preview = dirname(fileURLToPath(import.meta.url));
let root = preview;
while (!existsSync(resolve(root, "AGENTS.md")) || !existsSync(resolve(root, "package.json"))) {
  const parent = dirname(root);
  if (parent === root) throw new Error("Verification fixture must stay inside its project");
  root = parent;
}
const mode = process.argv[2] ?? "current";
const output = resolve(process.argv[3] ?? `/tmp/collaboration-creation-fix-${mode}-${process.pid}`);
const snapshot = resolve(preview, "../pre-implementation");
const pinned = JSON.parse(await readFile(resolve(snapshot, "manifest.json"), "utf8"));
const snapshotMap = new Map(mode === "baseline" ? pinned.map((item) => [resolve(root, item.path), resolve(snapshot, item.path)]) : []);
const exports = new Map();
for (const name of ["core", "ui", "views"]) exports.set(name, JSON.parse(await readFile(resolve(root, `packages/${name}/package.json`), "utf8")).exports);
function internal(source) {
  const verification = source.match(/^@verification\/(core|ui|views)\/(.*)$/);
  if (verification) return resolve(root, `packages/${verification[1]}`, verification[2]);
  const match = source.match(/^@multica\/(core|ui|views)(?:\/(.*))?$/);
  if (!match) return null;
  const subpath = match[2] ? `./${match[2]}` : ".";
  const entries = exports.get(match[1]);
  if (typeof entries[subpath] === "string") return resolve(root, `packages/${match[1]}`, entries[subpath]);
  for (const [key, value] of Object.entries(entries)) {
    if (!key.includes("*") || typeof value !== "string") continue;
    const [prefix, suffix] = key.split("*");
    if (subpath.startsWith(prefix) && subpath.endsWith(suffix)) return resolve(root, `packages/${match[1]}`, value.replace("*", subpath.slice(prefix.length, suffix ? -suffix.length : undefined)));
  }
  return null;
}
const resolver = {
  name: "task-owned-original-source-resolver", enforce: "pre",
  async resolveId(source, importer, options) {
    if (source.startsWith("\0")) return null;
    const originalImporter = importer?.startsWith(`${snapshot}/`) ? resolve(root, importer.slice(snapshot.length + 1)) : importer;
    const found = await this.resolve(internal(source) ?? source, originalImporter, { ...options, skipSelf: true });
    if (!found) return null;
    return snapshotMap.has(found.id) ? { ...found, id: snapshotMap.get(found.id) } : found;
  },
};
await mkdir(output, { recursive: true });
await writeFile(resolve(preview, "style.css"), `@import "${root}/apps/desktop/node_modules/tailwindcss/index.css";\n@import "${root}/node_modules/tw-animate-css/dist/tw-animate.css";\n@import "${root}/packages/ui/styles/tokens.css";\n@import "${root}/packages/ui/styles/base.css";\n@custom-variant dark (&:is(.dark *));\n@source "${root}/packages/views";\n@source "${root}/packages/ui";\n@source "${snapshot}/packages/views";\n:root { --font-sans: Inter, system-ui, sans-serif; --font-serif: serif; --font-mono: monospace; }\n`);
await build({ root: preview, configFile: false, envFile: false, publicDir: false, plugins: [resolver, react(), tailwindcss()],
  resolve: { dedupe: ["react", "react-dom", "@tanstack/react-query"] },
  build: { outDir: output, emptyOutDir: false, minify: false, sourcemap: false },
  logLevel: "warn", customLogger: { hasWarned: false, info() {}, warn(message) { if (!message.includes('"use client"')) process.stderr.write(message + "\n"); }, warnOnce(message) { this.warn(message); }, error(message) { process.stderr.write(message + "\n"); }, clearScreen() {}, hasErrorLogged() { return false; } },
});
const extra = ["packages/ui/styles/tokens.css", "packages/ui/styles/base.css", "packages/ui/components/ui/dialog.tsx", "packages/ui/components/ui/input.tsx", "packages/ui/components/ui/textarea.tsx", "packages/ui/components/ui/button.tsx", "packages/ui/components/ui/popover.tsx"];
if (mode !== "baseline") extra.push("packages/views/collaboration/form-styles.ts");
const paths = [...new Set([...pinned.map((item) => item.path), ...extra])];
const manifest = { mode, output, builtAt: new Date().toISOString(), buildPid: process.pid, root,
  sources: await Promise.all(paths.map(async (path) => ({ path, sha256: createHash("sha256").update(await readFile(snapshotMap.get(resolve(root, path)) ?? resolve(root, path))).digest("hex") }))) };
await writeFile(resolve(output, "provenance.json"), JSON.stringify(manifest, null, 2) + "\n");
await writeFile(resolve(preview, `${mode}-build.json`), JSON.stringify(manifest, null, 2) + "\n");
process.stdout.write(JSON.stringify({ mode, output }) + "\n");
