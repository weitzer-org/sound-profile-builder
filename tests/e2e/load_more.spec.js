import { test, expect } from '@playwright/test';

test('Preset Library Load More Test', async ({ page }) => {
  // Capture browser logs
  page.on('console', msg => console.log('BROWSER LOG:', msg.text()));

  // Intercept all API requests and append mock=true so backend uses mock mode
  await page.route('**/api/preset/*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    url.searchParams.set('mock', 'true');
    await route.continue({ url: url.toString() });
  });

  // Navigate to the local Go server
  await page.goto('/login?mock=true');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // Wait for redirect to index
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler', { timeout: 10000 });

  // Switch to Preset Library tab
  await page.click('button:has-text("Preset Library")');

  const presetList = page.locator('#library-list-container li');
  
  // Wait for presets to load
  await expect(presetList.first()).toBeVisible({ timeout: 10000 });

  // Count initial visible items
  const initialCount = await presetList.count();
  console.log(`Initial items count: ${initialCount}`);

  // Find Load More button
  const loadMoreBtn = page.locator('button:has-text("Load More...")');
  await expect(loadMoreBtn).toBeVisible();

  // Click Load More
  await loadMoreBtn.click();

  // Wait a bit for swap
  await page.waitForTimeout(1000);

  // Verify that count increased!
  const afterCount = await presetList.count();
  console.log(`Items count after Load More: ${afterCount}`);
  
  expect(afterCount).toBeGreaterThan(initialCount);

  // Verify that Load More button is now HIDDEN or gone
  await expect(loadMoreBtn).toBeHidden();
});
