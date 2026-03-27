import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.js',
  use: {
    baseURL: 'http://localhost:8082',
    headless: true, // Mirrors the headless style running from gsr
    video: 'on',    // Automatically captures verification videos of the HTMX dashboard rendering
  },
  forbidOnly: !!process.env.CI,
});
