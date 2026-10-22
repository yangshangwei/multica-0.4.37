import base from "../../../../../packages/eslint-config/base.js";
import { createRequire } from "node:module";
import path from "node:path";

// E2E is a root workspace surface. Running app-local ESLint with relative
// paths ignores these files, so use the same base rules at the repo boundary.
const requireDesktop = createRequire(path.resolve("apps/desktop/package.json"));
const { ESLint } = requireDesktop("eslint");
const globals = requireDesktop("globals");
const eslint = new ESLint({ cwd: process.cwd(), overrideConfigFile: true, overrideConfig: [
  ...base,
  { files: ["e2e/collaboration-installed-main.spec.ts", "e2e/fixtures/collaboration-installed-main.{ts,mjs}"],
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: { "import-x/no-extraneous-dependencies": ["error", { devDependencies: true }] } },
] });
const files = ["e2e/collaboration-installed-main.spec.ts", "e2e/fixtures/collaboration-installed-main.ts", "e2e/fixtures/collaboration-installed-main.mjs"];
const results = await eslint.lintFiles(files);
const formatter = await eslint.loadFormatter("stylish");
process.stdout.write(formatter.format(results));
const errors = results.reduce((sum, result) => sum + result.errorCount, 0);
const warnings = results.reduce((sum, result) => sum + result.warningCount, 0);
process.stdout.write(`${JSON.stringify({ files: results.map((result) => path.relative(process.cwd(), result.filePath)), errors, warnings })}\n`);
process.exitCode = errors || warnings ? 1 : 0;
