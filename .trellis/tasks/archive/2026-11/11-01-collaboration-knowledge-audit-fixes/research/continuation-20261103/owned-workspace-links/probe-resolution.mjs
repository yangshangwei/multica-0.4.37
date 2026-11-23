import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { createRequire } from "node:module";

const root = fs.realpathSync(process.argv[2]);
const phase = process.argv[3];
const output = process.argv[4];
const evidenceRoot = "/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/continuation-20261103/owned-workspace-links";
if (!output || path.dirname(path.resolve(output)) !== evidenceRoot) {
  throw new Error("Output must be a new file in the assigned evidence directory");
}
const sha = (file) => crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
function identity(file) {
  try {
    const real = fs.realpathSync(file);
    const stat = fs.statSync(file);
    const lst = fs.lstatSync(file);
    return {
      path: file, realpath: real, device: stat.dev, inode: stat.ino,
      symlink: lst.isSymbolicLink(), link: lst.isSymbolicLink() ? fs.readlinkSync(file) : null,
      owned: real === root || real.startsWith(`${root}/`),
      sha256: stat.isFile() ? sha(file) : null,
    };
  } catch (error) {
    return { path: file, absent: true, error_code: error.code };
  }
}
const entrypoints = {
  "@multica/core": "@multica/core/auth",
  "@multica/views": "@multica/views/auth",
  "@multica/ui": "@multica/ui/lib/utils",
  "@multica/tsconfig": "@multica/tsconfig/react-library.json",
  "@multica/eslint-config": "@multica/eslint-config/base",
  "@multica/plugin-sdk": "@multica/plugin-sdk",
};
const workspacePaths = Object.fromEntries(Object.keys(entrypoints).map((name) => [name, `packages/${name.split("/")[1]}`]));
const consumerPaths = ["."];
for (const group of ["apps", "packages"]) {
  for (const name of fs.readdirSync(path.join(root, group)).sort()) {
    if (fs.existsSync(path.join(root, group, name, "package.json"))) consumerPaths.push(`${group}/${name}`);
  }
}
const rows = [];
const declared = [];
const external = [];
const moduleDirectories = [];
for (const consumer of consumerPaths) {
  const base = path.resolve(root, consumer);
  const manifestPath = path.join(base, "package.json");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const req = createRequire(manifestPath);
  moduleDirectories.push({ consumer, ...identity(path.join(base, "node_modules")) });
  const dependencies = { ...manifest.dependencies, ...manifest.devDependencies, ...manifest.optionalDependencies };
  for (const [name, specifier] of Object.entries(dependencies)) {
    const link = identity(path.join(base, "node_modules", name));
    if (specifier.startsWith("workspace:")) {
      const expected = fs.realpathSync(path.join(root, workspacePaths[name]));
      declared.push({ consumer, name, specifier, expected_realpath: expected, link,
        matches_expected: link.realpath === expected });
    } else if (!name.startsWith("@multica/")) {
      const packageJson = path.join(base, "node_modules", name, "package.json");
      const packageIdentity = identity(packageJson);
      const installed = packageIdentity.absent ? null : JSON.parse(fs.readFileSync(packageJson, "utf8"));
      external.push({ consumer, name, specifier, installed_name: installed?.name,
        installed_version: installed?.version, package: packageIdentity, link });
    }
  }
  for (const [name, specifier] of Object.entries(entrypoints)) {
    try {
      const resolved = req.resolve(specifier);
      rows.push({ consumer, name, specifier, declared: dependencies[name]?.startsWith("workspace:") === true,
        result: identity(resolved) });
    } catch (error) {
      rows.push({ consumer, name, specifier, declared: dependencies[name]?.startsWith("workspace:") === true,
        error_code: error.code, error: error.message.split("\n")[0] });
    }
  }
}
const graph = [];
const coreProvider = path.join(root, "packages/core/platform/core-provider.tsx");
const directAuth = path.join(root, "packages/core/auth/index.ts");
for (const from of ["apps/web/components/web-providers.tsx", "packages/core/platform/core-provider.tsx", "packages/views/auth/password-form.tsx"]) {
  const req = createRequire(path.join(root, from));
  try { graph.push({ from, specifier: "@multica/core/auth", result: identity(req.resolve("@multica/core/auth")) }); }
  catch (error) { graph.push({ from, specifier: "@multica/core/auth", error_code: error.code }); }
}
graph.push({ from: "packages/core/platform/core-provider.tsx", specifier: "../auth/index.ts",
  note: "explicit extension for a Node read-only probe of the bundler's relative ../auth import",
  result: identity(createRequire(coreProvider).resolve("../auth/index.ts")) });
const record = {
  schema_version: 1, recorded_at: new Date().toISOString(), root, phase,
  mode: "read-only resolve/stat/hash; no application modules executed",
  command: [process.execPath, process.argv[1], ...process.argv.slice(2)],
  node: { version: process.version, platform: process.platform, arch: process.arch, binary: identity(process.execPath) },
  directories: moduleDirectories, declared_workspace_links: declared, workspace_exports: rows,
  external_direct_dependencies: external, auth_graph: graph, direct_auth: identity(directAuth),
  declared_link_mismatches: declared.filter((row) => !row.matches_expected).length,
  declared_export_failures: rows.filter((row) => row.declared && (!row.result || !row.result.owned)).length,
  resolved_workspace_exports_outside_root: rows.filter((row) => row.result && !row.result.owned).length,
};
fs.writeFileSync(output, `${JSON.stringify(record, null, 2)}\n`, { flag: "wx" });
console.log(JSON.stringify({ root, phase, declared_edges: declared.length, mismatched_edges: record.declared_link_mismatches,
  failed_declared_exports: record.declared_export_failures, outside_exports: record.resolved_workspace_exports_outside_root,
  auth_graph: graph, output }, null, 2));
if (phase === "owned-after" && (record.declared_link_mismatches || record.declared_export_failures || record.resolved_workspace_exports_outside_root)) process.exitCode = 1;
