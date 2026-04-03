import { test, expect } from '@playwright/test';

test('Tab Isolation and Scoping Test', async ({ page }) => {
  // Intecept all API requests and append mock=true so backend uses mock mode
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

  // 0. Pre-requisite: Create a preset in the library so we have something to view
  await page.fill('input[name="prompt"]', 'Base Preset for Scoping Test');
  await page.click('#gen-submit-btn');
  await expect(page.locator('button:has-text("Finalize Save")')).toBeVisible({ timeout: 30000 });
  const uniqueBaseName = `Scoping Test Base ${Date.now()}`;
  await page.fill('input[name="preset_name"]', uniqueBaseName);
  await page.click('button:has-text("Finalize Save")');

  // Wait for it to appear in sidebar (Preset Library tab)
  await page.click('button:has-text("Preset Library")');
  const presetList = page.locator('#library-list-container li');
  await expect(presetList.filter({ hasText: uniqueBaseName }).first()).toBeVisible({ timeout: 15000 });

  // Switch back to Generator to start the concurrent test
  await page.click('button:has-text("Generator")');

  // 1. Start concurrent generation in Generator tab
  await page.fill('input[name="prompt"]', 'Hendrix Fuzz');
  await page.click('#gen-submit-btn');

  // Verify progress panel appears
  await expect(page.locator('#gen-progress-area')).toBeVisible();

  // 2. Switch to Preset Library tab while running
  await page.click('button:has-text("Preset Library")');

  // 3. Verify sidebar loads and click view on first preset (already switched to Library tab in step 2)
  await expect(presetList.first()).toBeVisible({ timeout: 10000 });
  
  // View the first preset in the library
  await presetList.first().locator('button[hx-get^="/api/preset/view"]').click();

  // Verify workspace loads the preset in read-only mode (static table, no sliders)
  await expect(page.locator('#lib-live-matrix-container')).toBeVisible();
  await expect(page.locator('#lib-live-matrix-container input[type="range"]')).not.toBeVisible();

  // Toggle Edit Mode
  await page.click('#lib-workspace-wrapper button:has-text("Enable Edit")');

  // Verify Edit Table loads (sliders visible)
  await expect(page.locator('#lib-live-matrix-container input[type="range"]')).toBeVisible();

  // 4. Switch back to Generator tab
  await page.click('button:has-text("Generator")');

  // Verify generator workspace is still there and progress area is intact
  await expect(page.locator('#gen-workspace-wrapper')).toBeVisible();
  
  // We should see the Draft Preset load eventually
  await expect(page.locator('button:has-text("Finalize Save")')).toBeVisible({ timeout: 30000 });
});
