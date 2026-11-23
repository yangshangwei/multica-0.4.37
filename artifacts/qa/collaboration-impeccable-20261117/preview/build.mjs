import { build } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const preview = dirname(fileURLToPath(import.meta.url));
let root = preview;
while (!existsSync(resolve(root, "AGENTS.md")) || !existsSync(resolve(root, "package.json"))) {
  const parent = dirname(root);
  if (parent === root) throw new Error("Verification fixture must stay inside its project");
  root = parent;
}
const mode = process.argv[2] ?? "regression";
const output = resolve(process.argv[3] ?? `/tmp/collaboration-impeccable-${mode}-${process.pid}`);
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
    const originalImporter = importer;
    const found = await this.resolve(internal(source) ?? source, originalImporter, { ...options, skipSelf: true });
    if (!found) return null;
    return found;
  },
};
await mkdir(output, { recursive: true });
await writeFile(resolve(preview, "style.css"), `@import "${root}/apps/desktop/node_modules/tailwindcss/index.css";\n@import "${root}/node_modules/tw-animate-css/dist/tw-animate.css";\n@import "${root}/packages/ui/styles/tokens.css";\n@import "${root}/packages/ui/styles/base.css";\n@custom-variant dark (&:is(.dark *));\n@source "${root}/packages/views";\n@source "${root}/packages/ui";\n:root { --font-sans: Inter, system-ui, sans-serif; --font-serif: serif; --font-mono: monospace; }\n`);
const bundle = await build({ root: preview, configFile: false, envFile: false, publicDir: false, plugins: [resolver, react(), tailwindcss()],
  resolve: { dedupe: ["react", "react-dom", "@tanstack/react-query"] },
  build: { outDir: output, emptyOutDir: false, minify: false, sourcemap: false },
  logLevel: "warn", customLogger: { hasWarned: false, info() {}, warn(message) { if (!message.includes('"use client"')) process.stderr.write(message + "\n"); }, warnOnce(message) { this.warn(message); }, error(message) { process.stderr.write(message + "\n"); }, clearScreen() {}, hasErrorLogged() { return false; } },
});
const paths = [
  "packages/views/collaboration/creation-form.tsx", "packages/views/collaboration/creation-form.test.tsx", "packages/views/collaboration/account-picker.tsx",
  "packages/views/collaboration/attachment-picker.tsx", "packages/views/collaboration/creation-description-assist.tsx",
  "packages/views/modals/issue-description-assist.tsx", "packages/core/api/issue-description-stream.ts",
  "packages/views/locales/en/modals.json", "packages/views/locales/zh-Hans/modals.json", "packages/views/collaboration/field-inputs.tsx",
  "packages/views/collaboration/form-styles.ts", "packages/views/collaboration/focus.ts",
  "packages/views/issues/components/pickers/property-picker.tsx",
  "packages/views/locales/en/collaboration.json", "packages/views/locales/zh-Hans/collaboration.json",
  "packages/ui/styles/tokens.css", "packages/ui/styles/base.css",
  "packages/ui/components/ui/dialog.tsx", "packages/ui/components/ui/input.tsx",
  "packages/ui/components/ui/textarea.tsx", "packages/ui/components/ui/button.tsx",
  "packages/ui/components/ui/popover.tsx", "packages/views/modals/create-issue.tsx",
  ...["main.tsx", "index.html", "style.css", "verify.mjs", "regress.mjs", "build.mjs"].map((name) => relative(root, resolve(preview, name))),
];
const outputs = (Array.isArray(bundle) ? bundle : [bundle]).flatMap((item) => item.output);
const bundledPaths = outputs.filter((item) => item.type === "chunk").flatMap((item) => Object.keys(item.modules))
  .map((id) => id.split("?")[0]).filter((id) => id.startsWith(`${root}/`) && !id.includes("/node_modules/") && existsSync(id))
  .map((id) => relative(root, id));
const sourcePaths = [...new Set([...paths, ...bundledPaths])].sort();
const manifest = { mode, output, builtAt: new Date().toISOString(), buildPid: process.pid, root,
  sources: await Promise.all(sourcePaths.map(async (path) => ({ path, sha256: createHash("sha256").update(await readFile(resolve(root, path))).digest("hex") }))),
  assets: await Promise.all(outputs.map(async (item) => ({ path: item.fileName, sha256: createHash("sha256").update(await readFile(resolve(output, item.fileName))).digest("hex") }))) };
await writeFile(resolve(output, "provenance.json"), JSON.stringify(manifest, null, 2) + "\n");
await writeFile(resolve(preview, `${mode}-build.json`), JSON.stringify(manifest, null, 2) + "\n");
process.stdout.write(JSON.stringify({ mode, output }) + "\n");
