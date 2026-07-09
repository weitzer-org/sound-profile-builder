import { defineConfig } from '@playwright/test';

export default defineConfig({
  timeout: 60000,
  testDir: '.',
  testMatch: '**/*.spec.js',
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8082',
    headless: true,
    video: 'on',
  },
  forbidOnly: !!process.env.CI,
});
