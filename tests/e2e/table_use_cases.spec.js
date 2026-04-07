import { test, expect } from '@playwright/test';
import * as path from 'path';

const screenshotDir = '/usr/local/google/home/benweitzer/.gemini/jetski/brain/b1f3d854-6bb3-48ea-b72c-5c29ab278f0f/screenshots';

test('Table Use Cases Screenshots', async ({ page }) => {
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

  // === Case 1: Initial Generation (Static Table) ===
  console.log('Testing Case 1: Initial Generation');
  await page.fill('input[name="prompt"]', 'Generate a Hendrix style tone');
  await page.click('#gen-submit-btn');
  
  // Wait for generation to complete (mock mode is fast)
  const table1 = page.locator('.grid-matrix').first();
  await table1.waitFor({ state: 'visible', timeout: 10000 });
  
  // Strict assertions for column headers
  await expect(table1.locator('th').nth(0)).toHaveText(/EFFECT TYPE & NAME/i);
  await expect(table1.locator('th').nth(1)).toHaveText(/SCENE A \(RHYTHM\)/i);
  await expect(table1.locator('th').nth(2)).toHaveText(/SCENE B \(LEAD\)/i);
  
  await page.screenshot({ path: path.join(screenshotDir, 'use_case_1_generation.png'), fullPage: true });

  // === Case 2: Finalize Save (Still Static Table) ===
  console.log('Testing Case 2: Finalize Save');
  await page.fill('input[name="preset_name"]', 'My Hendrix Tone');
  await page.click('button:has-text("Finalize Save")');
  
  // Wait for save toast or update
  await page.waitForSelector('.toast.show', { timeout: 5000 });
  
  // Verify it's still static with correct columns
  const table2 = page.locator('.grid-matrix').first();
  await expect(table2).toBeVisible();
  await expect(table2.locator('th').nth(0)).toHaveText(/EFFECT TYPE & NAME/i);
  await expect(table2.locator('th').nth(1)).toHaveText(/SCENE A \(RHYTHM\)/i);
  await expect(table2.locator('th').nth(2)).toHaveText(/SCENE B \(LEAD\)/i);
  
  await page.screenshot({ path: path.join(screenshotDir, 'use_case_2_saved_static.png'), fullPage: true });

  // === Case 3: Loading Saved Preset (Editable View) ===
  console.log('Testing Case 3: Loading Saved Preset');
  
  // Switch to Preset Library tab to make sure presets are listed
  await page.click('button:has-text("Preset Library")');
  
  const firstPreset = page.locator('#library-list-container li').first();
  await expect(firstPreset).toBeVisible({ timeout: 10000 });
  
  // Mock the response for a specific preset to return structured data
  await page.route('**/api/preset/view*', async route => {
    await route.fulfill({
      status: 200,
      contentType: 'text/html',
      body: `
        <div id="library-editor-workspace">
          <h2>Loaded Preset</h2>
          <div class="effect-block">
            <h3>Amplifier: Plexi</h3>
            <div class="param-group">
              <span>Gain</span>
              <input type="range" value="7.0">
            </div>
          </div>
          <form hx-post="/api/preset/chat" hx-target="#library-editor-workspace">
            <textarea name="message"></textarea>
            <button type="submit">Refine</button>
          </form>
        </div>
      `
    });
  });

  await firstPreset.locator('button:has-text("Adjust preset")').click();
  await page.waitForSelector('.effect-block', { timeout: 10000 });
  
  // Assert editable controls are visible
  await expect(page.locator('.effect-block')).toBeVisible();
  await expect(page.locator('.param-group')).toBeVisible();
  
  await page.screenshot({ path: path.join(screenshotDir, 'use_case_3_loaded_editable.png'), fullPage: true });

  // === Case 4: Chat Refinement (Static Table) ===
  console.log('Testing Case 4: Chat Refinement');
  
  // Mock the chat response to return legacy HTML
  await page.route('**/api/preset/chat*', async route => {
    await route.fulfill({
      status: 200,
      contentType: 'text/html',
      body: `
        <div id="library-editor-workspace">
          <h2>Loaded Preset</h2>
          <table class="grid-matrix">
            <thead><tr><th>EFFECT TYPE & NAME</th><th>SCENE A (RHYTHM)</th><th>SCENE B (LEAD)</th></tr></thead>
            <tbody><tr><td>Overdrive</td><td>Drive: 5</td><td>Drive: 7</td></tr></tbody>
          </table>
        </div>
      `
    });
  });

  // Send chat message
  const chatInput = page.locator('#library-editor-workspace textarea[name="message"]');
  await expect(chatInput).toBeVisible();
  await chatInput.fill('Make it brighter');
  await page.click('#library-editor-workspace button:has-text("Refine")');
  
  // Wait for chat response (should be static table)
  const table4 = page.locator('#library-editor-workspace .grid-matrix').first();
  await table4.waitFor({ state: 'visible', timeout: 10000 });
  
  // Assert fallback table has correct columns
  await expect(table4.locator('th').nth(0)).toHaveText(/EFFECT TYPE & NAME/i);
  await expect(table4.locator('th').nth(1)).toHaveText(/SCENE A \(RHYTHM\)/i);
  await expect(table4.locator('th').nth(2)).toHaveText(/SCENE B \(LEAD\)/i);
  
  await page.screenshot({ path: path.join(screenshotDir, 'use_case_4_chat_static.png'), fullPage: true });
});
