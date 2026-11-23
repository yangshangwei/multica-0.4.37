import { defineConfig } from '@playwright/test';
export default defineConfig({testDir:'.',testMatch:'drawer.spec.ts',timeout:180000,workers:1,retries:0,outputDir:'./output',reporter:[['list'],['json',{outputFile:'./results.json'}]],use:{actionTimeout:15000,trace:'off',screenshot:'only-on-failure'}});
