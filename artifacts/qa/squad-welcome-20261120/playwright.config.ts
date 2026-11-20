import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "../../..",
  testMatch: [
    "e2e/squad-welcome.spec.ts",
    "artifacts/qa/squad-welcome-20261120/native.spec.ts",
  ],
  outputDir: "./results",
  timeout: 90_000,
  workers: 1,
  retries: 0,
  reporter: [["json", { outputFile: "./playwright-results.json" }]],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL,
    headless: true,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
