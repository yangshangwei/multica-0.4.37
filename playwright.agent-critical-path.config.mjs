import { defineConfig } from "@playwright/test";
import { join } from "node:path";
import { criticalPathCases } from "./scripts/check-agent-critical-path.mjs";

const runDir = process.env.MULTICA_SMOKE_RUN_DIR;
const baseURL = process.env.PLAYWRIGHT_BASE_URL;
if (!runDir || !baseURL || !process.env.DATABASE_URL) {
  throw new Error("Run this smoke through scripts/agent-critical-path.mjs with an owned allocation");
}
const escapeRegExp = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

export default defineConfig({
  testDir: "./e2e",
  testMatch: [...new Set(criticalPathCases.map(({ file }) => file))],
  grep: new RegExp(criticalPathCases.map(({ title }) => `${escapeRegExp(title)}$`).join("|")),
  timeout: 60_000,
  globalTimeout: 10 * 60_000,
  workers: 1,
  retries: 0,
  repeatEach: 1,
  forbidOnly: true,
  outputDir: join(runDir, "test-results"),
  reporter: [["line"], ["json", { outputFile: join(runDir, "playwright.json") }]],
  use: { baseURL, headless: true, trace: "retain-on-failure", screenshot: "only-on-failure" },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
