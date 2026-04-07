import { test, expect } from '@playwright/test';
import * as path from 'path';

const screenshotDir = '/usr/local/google/home/benweitzer/.gemini/jetski/brain/b1f3d854-6bb3-48ea-b72c-5c29ab278f0f/screenshots';

test('Capture UI Screenshots', async ({ page }) => {
  // Intercept all API requests and append mock=true so backend uses mock mode
  await page.route('**/api/preset*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    url.searchParams.set('mock', 'true');
    await route.continue({ url: url.toString() });
  });

  // Navigate to the local Go server
  await page.goto('/login?mock=true');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // Wait for redirect to index and network idle
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler', { timeout: 15000 });
  await page.waitForLoadState('networkidle');

  // Screenshot 1: Login Success / Dashboard
  await page.screenshot({ path: path.join(screenshotDir, '1_dashboard.png') });

  // Switch to Preset Library tab
  await page.click('button:has-text("Preset Library")');

  const presetList = page.locator('#library-list-container li');
  // Wait for presets to load with a larger timeout
  await expect(presetList.first()).toBeVisible({ timeout: 20000 });

  // Screenshot 2: Preset Library
  await page.screenshot({ path: path.join(screenshotDir, '2_preset_library.png') });

  // Search for "Hendrix"
  const searchInput = page.locator('#preset-search');
  await searchInput.pressSequentially('Hendrix');
  await page.waitForTimeout(1000); // Give it a bit more time

  // Screenshot 3: Search Results
  await page.screenshot({ path: path.join(screenshotDir, '3_search_results.png') });

  // Clear search
  await searchInput.fill('');
  await searchInput.pressSequentially('a'); // Trigger keyup with something
  await searchInput.fill(''); // Clear again
  await searchInput.pressSequentially(''); // Trigger keyup empty
  await page.waitForTimeout(1000);

  // Find Load More button
  const loadMoreBtn = page.locator('button:has-text("Load More...")');
  await expect(loadMoreBtn).toBeVisible();

  // Click Load More
  await loadMoreBtn.click();
  await page.waitForTimeout(1500); // Give it time to append

  // Screenshot 4: Loaded More Presets
  await page.screenshot({ path: path.join(screenshotDir, '4_loaded_more.png') });
});
