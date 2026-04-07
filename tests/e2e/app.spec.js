import { test, expect } from '@playwright/test';

test('QC-2 HTMX Dashboard UI Integration Test', async ({ page }) => {
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
  await expect(page).toHaveTitle('Login - QC-2 Modeler');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // Wait for redirect to index
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler', { timeout: 10000 });

  // Listen for HTMX target errors and log details
  await page.evaluate(() => {
    window.addEventListener('htmx:targetError', (evt) => {
      console.log('HTMX TARGET ERROR DETAILS:', {
        target: evt.detail.target,
        content: evt.detail.content,
        error: evt.detail.error
      });
    });
  });

  // Verify HTMX page loading and root constraints
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler');
  await expect(page.locator('h1')).toHaveText('Spin Up a Tone');

  // Test Tonal Prompt
  await page.fill('input[name="prompt"]', 'Generate a Hendrix style Fuzz Face matrix.');
  await page.click('button#gen-submit-btn');

  // Wait for mock data response (which includes Draft Preset in header)
  await expect(page.locator('button:has-text("Finalize Save")')).toBeVisible({ timeout: 30000 });

  // 1. Rename & Save the generated draft
  await page.fill('input[name="preset_name"]', 'Awesome Hendrix Tone');
  await page.click('button:has-text("Finalize Save")');

  // Wait for sidebar update
  const presetList = page.locator('#library-list-container li');
  await expect(presetList.filter({ hasText: 'Awesome Hendrix Tone' }).first()).toBeAttached({ timeout: 10000 });

  // Wait for workspace header update
  await expect(page.locator('.workspace-wrapper h2').first()).toContainText('Awesome Hendrix Tone', { timeout: 10000 });

  // 2. Adjust Preset (Wait for adjustment form to appear in workspace)
  await page.fill('#chat-input', 'Make it brighter.');
  await page.click('#chat-submit-btn');
  
  // Wait for model chat log to appear
  await expect(page.locator('#view-generator .workspace-wrapper').first()).toContainText('Make it brighter.');

  // Wait for mock server to process and HTMX to settle
  await page.waitForTimeout(2000);

  // 3. Workspace Rename
  await page.click('button:has-text("Rename")');
  await page.fill('form[hx-post="/api/preset/rename"] input[name="preset_name"]', 'Brighter Hendrix Tone');
  await expect(page.locator('form[hx-post="/api/preset/rename"] button[type="submit"]')).toBeVisible();
  await page.locator('form[hx-post="/api/preset/rename"] button[type="submit"]').click();

  // Wait for title update
  await expect(page.locator('h2', { hasText: 'Brighter Hendrix Tone' })).toBeVisible({ timeout: 10000 });

  // 4. Copy Preset into Workspace
  const uniqueDuplicateName = `Hendrix Duplicate ${Date.now()}`;
  // Switch to Preset Library tab to make sidebar visible
  await page.click('button:has-text("Preset Library")');
  await expect(presetList.filter({ hasText: 'Brighter Hendrix Tone' }).first()).toBeVisible({ timeout: 10000 });

  const awesomeToneListItem = presetList.filter({ hasText: 'Brighter Hendrix Tone' }).first();
  await awesomeToneListItem.locator('button:has-text("Copy")').click();
  
  // Wait for workspace to shift into Copy mode
  await expect(page.locator('h3', { hasText: 'Duplicate Preset' })).toBeVisible({ timeout: 10000 });
  const workspaceCopyForm = page.locator('form[hx-post="/api/preset/copy"]');
  await workspaceCopyForm.locator('input[name="new_name"]').fill(uniqueDuplicateName);
  
  // Debug: Check if #library-editor-workspace exists
  const exists = await page.evaluate(() => !!document.getElementById('library-editor-workspace'));
  console.log('#library-editor-workspace exists in DOM:', exists);

  // Wait a bit for HTMX to initialize listeners on the new form
  await page.waitForTimeout(1000);

  // Use dispatchEvent to ensure HTMX catches the submit event
  await workspaceCopyForm.evaluate(form => {
    form.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));
  });

  // Debug: Log all preset names in the list after copy
  await page.waitForTimeout(2000); // Wait a bit for swap
  const listTextsAfter = await presetList.allInnerTexts();
  console.log('Preset list after copy confirm:', listTextsAfter);

  // Wait for duplicate to appear in sidebar
  await expect(presetList.filter({ hasText: uniqueDuplicateName }).first()).toBeVisible({ timeout: 30000 });
  
  // Ensure the workspace also loads the new duplicate (we stay in the Library tab)
  await expect(page.locator('h2', { hasText: uniqueDuplicateName }).first()).toBeVisible({ timeout: 10000 });

  // 5. Delete Preset in sidebar
  const duplicateToneListItem = presetList.filter({ hasText: uniqueDuplicateName }).first();
  const deleteBtn = duplicateToneListItem.locator('button[id^="delete-btn-"]');
  await deleteBtn.click();
  // Ensure it turns to "Confirm?" and click again
  await expect(deleteBtn).toHaveText('Confirm?');
  await deleteBtn.click();

  // Wait for UI sequence to complete (duplicate is destroyed)
  await expect(presetList.filter({ hasText: uniqueDuplicateName }).first()).not.toBeVisible({ timeout: 30000 });
});
