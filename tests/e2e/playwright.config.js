import { defineConfig } from '@playwright/test';

export default defineConfig({
  timeout: 60000,
  testDir: '.',
  testMatch: '**/*.spec.js',
  use: {
    baseURL: 'http://localhost:8083',
    headless: true,
    video: 'on',
  },
  forbidOnly: !!process.env.CI,
});
