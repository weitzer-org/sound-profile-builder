import { defineConfig } from '@playwright/test';

export default defineConfig({
  timeout: 60000,
  testDir: '.',
  testMatch: '**/*.spec.js',
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8082',
    headless: true,
    video: 'on',
    // PW_CHROMIUM_PATH points at a preinstalled Chromium (e.g. a cloud sandbox's
    // /opt/pw-browsers build) instead of the one `playwright install` downloads.
    launchOptions: process.env.PW_CHROMIUM_PATH ? { executablePath: process.env.PW_CHROMIUM_PATH } : {},
  },
  forbidOnly: !!process.env.CI,
});
