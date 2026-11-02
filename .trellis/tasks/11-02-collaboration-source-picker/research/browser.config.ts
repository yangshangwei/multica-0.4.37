import { defineConfig } from "@playwright/test";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const phase = process.env.SOURCE_PICKER_BROWSER_PHASE ?? "candidate";
const researchDir = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  testDir: researchDir,
  testMatch: "browser.spec.ts",
  timeout: 180_000,
  workers: 1,
  retries: 0,
  reporter: [["list"], ["json", { outputFile: resolve(researchDir, `browser-evidence/${phase}-results.json`) }]],
  outputDir: resolve(researchDir, `browser-evidence/${phase}-test-output`),
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL,
    browserName: "chromium",
    headless: true,
    viewport: { width: 1280, height: 950 },
    actionTimeout: 15_000,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
});
