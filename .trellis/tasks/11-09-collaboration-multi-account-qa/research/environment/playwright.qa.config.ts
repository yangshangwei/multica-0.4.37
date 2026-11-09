import { defineConfig } from "@playwright/test";
import path from "node:path";
const outputRoot = "/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/collaboration-multi-account-20261109";
const run = process.env.QA_RUN_ID ?? "run-01";
if (!/^[a-zA-Z0-9_-]+$/.test(run)) throw new Error("Invalid QA run ID");
export default defineConfig({
  testDir: "./e2e", timeout: 60000, workers: 1, retries: 0,
  outputDir: path.join(outputRoot, run, "test-results"),
  reporter: [["list"], ["json", { outputFile: path.join(outputRoot, run, "playwright.json") }]],
  use: { baseURL: process.env.PLAYWRIGHT_BASE_URL, headless: true, trace: "off", screenshot: "only-on-failure" },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
