import { test, expect } from '@playwright/test';

test('Preset Library Search Test', async ({ page }) => {
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

  const searchInput = page.locator('#preset-search');
  await expect(searchInput).toBeVisible();

  // Type "Hendrix" in search using pressSequentially to trigger keyup events
  await searchInput.pressSequentially('Hendrix');

  // Wait a bit for filter to apply
  await page.waitForTimeout(500);

  // Verify that only items containing "Hendrix" are visible
  const visibleItems = presetList.filter({ visible: true });
  const visibleCount = await visibleItems.count();
  console.log(`Visible items count after searching 'Hendrix': ${visibleCount}`);

  const visibleTexts = await visibleItems.allInnerTexts();
  console.log('Visible texts:', visibleTexts);

  for (const text of visibleTexts) {
    // Skip checking if it's the Load More button
    if (text.includes('Load More...')) continue;
    expect(text.toLowerCase()).toContain('hendrix');
  }

  // Type something that doesn't match anything
  await searchInput.fill(''); // Clear first
  await searchInput.pressSequentially('NonExistentPresetXYZ');
  await page.waitForTimeout(500);

  // Log visible count to see if it crashed
  const visibleCountEmpty = await presetList.filter({ visible: true }).count();
  console.log(`Visible items count after searching 'NonExistentPresetXYZ': ${visibleCountEmpty}`);
  
  // Verify that only Load More is visible (count should be 1)
  expect(visibleCountEmpty).toBe(1);
});
