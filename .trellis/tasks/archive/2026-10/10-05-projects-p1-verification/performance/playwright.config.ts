import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "performance.spec.ts",
  workers: 1,
  retries: 0,
  timeout: 70 * 60_000,
  reporter: [["list"]],
  outputDir: "../../../../.omx/projects-p1-performance/playwright",
});
