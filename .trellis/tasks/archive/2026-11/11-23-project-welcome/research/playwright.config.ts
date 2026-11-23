import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: ["project-welcome.browser.ts", "project-welcome.actions.browser.ts"],
  timeout: 180_000,
  workers: 1,
  retries: 0,
  outputDir: process.env.PROJECT_WELCOME_QA_DIR ?? "./qa",
  reporter: [["list"]],
  use: {
    baseURL: process.env.FRONTEND_ORIGIN,
    headless: true,
    browserName: "chromium",
    trace: "retain-on-failure",
  },
});
