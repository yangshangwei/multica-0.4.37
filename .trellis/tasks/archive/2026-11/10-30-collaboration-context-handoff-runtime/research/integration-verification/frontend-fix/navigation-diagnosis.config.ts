import { defineConfig } from "@playwright/test";
export default defineConfig({ testDir: ".", testMatch: "navigation-diagnosis.spec.ts", timeout: 90000, workers: 1, retries: 0, use: { baseURL: "http://localhost:13000", trace: "off" }, reporter: [["list"]] });
