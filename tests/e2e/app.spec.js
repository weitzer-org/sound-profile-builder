import { test, expect } from '@playwright/test';

test('QC-2 HTMX Dashboard UI Integration Test', async ({ page }) => {
  // Intercept all API requests and append mock=true so backend uses mock mode
  await page.route('**/api/preset/*', async route => {
    const request = route.request();
    if (request.method() === 'POST') {
      const postData = request.postData() || '';
      const newPostData = postData ? postData + '&mock=true' : 'mock=true';
      await route.continue({ postData: newPostData });
    } else {
      // For GET requests like /api/preset/view
      const url = new URL(request.url());
      url.searchParams.set('mock', 'true');
      await route.continue({ url: url.toString() });
    }
  });

  // Navigate to the local Go server
  await page.goto('/login?mock=true');
  await expect(page).toHaveTitle('Login - QC-2 Modeler');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // Wait for redirect to index
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler', { timeout: 10000 });

  // Verify HTMX page loading and root constraints
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler');
  await expect(page.locator('h1')).toHaveText('Spin Up a Tone');

  // Test Tonal Prompt
  await page.fill('input[name="prompt"]', 'Generate a Hendrix style Fuzz Face matrix.');
  await page.click('button#submit-btn');

  // Wait for mock data response (which includes Draft Preset in header)
  await expect(page.locator('button:has-text("Finalize Save")')).toBeVisible({ timeout: 10000 });

  // 1. Rename & Save the generated draft
  await page.fill('input[name="preset_name"]', 'Awesome Hendrix Tone');
  await page.click('button:has-text("Finalize Save")');

  // Wait for sidebar update
  const presetList = page.locator('#preset-list-container li');
  await expect(presetList.filter({ hasText: 'Awesome Hendrix Tone' }).first()).toBeVisible({ timeout: 10000 });

  // 2. Adjust Preset (Wait for adjustment form to appear in workspace)
  await page.fill('#chat-input', 'Make it brighter.');
  await page.click('button:has-text("Adjust")');
  
  // Wait for model chat log to appear
  await expect(page.locator('.workspace-wrapper')).toContainText('Make it brighter.');

  // 3. Workspace Rename
  await page.click('button:has-text("Rename")');
  await page.fill('form[hx-post="/api/preset/rename"] input[name="preset_name"]', 'Brighter Hendrix Tone');
  await page.click('form[hx-post="/api/preset/rename"] button[type="submit"]');

  // Wait for title update
  await expect(page.locator('h2', { hasText: 'Brighter Hendrix Tone' })).toBeVisible({ timeout: 10000 });

  // 4. Copy Preset into Workspace
  const uniqueDuplicateName = `Hendrix Duplicate ${Date.now()}`;
  const awesomeToneListItem = presetList.filter({ hasText: 'Brighter Hendrix Tone' }).first();
  await awesomeToneListItem.locator('button:has-text("Copy")').click();
  
  // Wait for workspace to shift into Copy mode
  await expect(page.locator('h3', { hasText: 'Duplicate Preset' })).toBeVisible({ timeout: 10000 });
  const workspaceCopyForm = page.locator('form[hx-post="/api/preset/copy"]');
  await workspaceCopyForm.locator('input[name="new_name"]').fill(uniqueDuplicateName);
  await workspaceCopyForm.locator('button:has-text("Confirm Duplicate")').click();

  // Wait for duplicate to appear in sidebar
  await expect(presetList.filter({ hasText: uniqueDuplicateName }).first()).toBeVisible({ timeout: 30000 });
  // Ensure the workspace also loads the new duplicate via the HX-Target replacement
  await expect(page.locator('h2', { hasText: uniqueDuplicateName }).first()).toBeVisible({ timeout: 10000 });

  // 5. Delete Preset in sidebar
  const duplicateToneListItem = presetList.filter({ hasText: uniqueDuplicateName }).first();
  const deleteBtn = duplicateToneListItem.locator('button[id^="delete-btn-"]');
  await deleteBtn.click();
  // Ensure it turns to "Confirm?" and click again
  await expect(deleteBtn).toHaveText('Confirm?');
  await deleteBtn.click();

  // Wait for UI sequence to complete (duplicate is destroyed)
  await expect(presetList.filter({ hasText: uniqueDuplicateName }).first()).not.toBeVisible({ timeout: 10000 });
});
