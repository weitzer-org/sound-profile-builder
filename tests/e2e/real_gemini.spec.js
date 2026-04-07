import { test, expect } from '@playwright/test';
import * as path from 'path';

const screenshotDir = '/usr/local/google/home/benweitzer/.gemini/jetski/brain/b1f3d854-6bb3-48ea-b72c-5c29ab278f0f/screenshots';

test('Real Gemini API Test', async ({ page }) => {
  test.setTimeout(600000); // 10 minutes
  // DO NOT mock API routes here! We want to use the real backend.

  // Navigate to the local server (assuming port 8080 based on logs)
  await page.goto('http://localhost:8080/login');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // Wait for redirect to index
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler', { timeout: 15000 });
  await page.waitForLoadState('networkidle');

  // === Case 1: Initial Generation ===
  console.log('Starting Initial Generation for BB King');
  await page.fill('input[name="prompt"]', 'BB King');
  await page.click('#gen-submit-btn');
  
  // Wait for generation (real API is slow, wait up to 5 minutes)
  const table = page.locator('.grid-matrix').first();
  await table.waitFor({ state: 'visible', timeout: 300000 });
  
  // Assert column headers
  await expect(table.locator('th').nth(0)).toHaveText(/EFFECT TYPE & NAME/i);
  await expect(table.locator('th').nth(1)).toHaveText(/SCENE A \(RHYTHM\)/i);
  await expect(table.locator('th').nth(2)).toHaveText(/SCENE B \(LEAD\)/i);
  
  await page.evaluate(() => { const h = document.querySelector('.nav-header'); if(h) h.style.position = 'static'; });
  await page.screenshot({ path: path.join(screenshotDir, 'real_bb_king_initial.png'), fullPage: true });

  // === Case 2: Adjustment ===
  console.log('Starting Adjustment: make it grittier');
  const chatInput = page.locator('#chat-input').first();
  await chatInput.fill('make it grittier');
  await page.click('#chat-submit-btn');
  
  // Wait for response (wait up to 3 minutes)
  // Wait for the submit button to stop showing the spinner (HTMX removes htmx-request)
  const submitBtn = page.locator('#chat-submit-btn');
  await expect(submitBtn).not.toHaveClass(/htmx-request/, { timeout: 180000 });
  
  // Also wait for the table to be visible again just in case
  await table.waitFor({ state: 'visible', timeout: 10000 });
  
  // Assert column headers again
  await expect(table.locator('th').nth(0)).toHaveText(/EFFECT TYPE & NAME/i);
  await expect(table.locator('th').nth(1)).toHaveText(/SCENE A \(RHYTHM\)/i);
  await expect(table.locator('th').nth(2)).toHaveText(/SCENE B \(LEAD\)/i);
  
  await page.evaluate(() => { const h = document.querySelector('.nav-header'); if(h) h.style.position = 'static'; });
  await page.screenshot({ path: path.join(screenshotDir, 'real_bb_king_grittier.png'), fullPage: true });

  // === Case 3: Save Preset ===
  console.log('Saving Preset');
  await page.fill('input[placeholder="Enter custom name..."]', 'BB King Real');
  await page.click('button:has-text("Finalize Save")');
  await page.waitForTimeout(5000); // Wait for save
  
  await page.evaluate(() => { const h = document.querySelector('.nav-header'); if(h) h.style.position = 'static'; });
  await page.screenshot({ path: path.join(screenshotDir, 'real_bb_king_saved.png'), fullPage: true });

  // === Case 4: Navigate to Preset in Library ===
  console.log('Navigating to Preset Library');
  await page.click('button:has-text("Preset Library")');
  await page.evaluate(() => {
    const genView = document.getElementById('view-generator');
    if (genView) genView.remove();
  });
  await page.waitForSelector('#library-list-container', { timeout: 10000 });
  const adjustBtn = page.locator('#library-list-container button:has-text("Adjust preset")').first();
  await adjustBtn.waitFor({ state: 'visible', timeout: 15000 });
  await adjustBtn.click();
  await page.waitForLoadState('networkidle');
  await page.locator('#library-editor-workspace .effect-block').first().waitFor({ state: 'visible', timeout: 30000 });
  
  await page.evaluate(() => { const h = document.querySelector('.nav-header'); if(h) h.style.position = 'static'; });
  await page.screenshot({ path: path.join(screenshotDir, 'real_bb_king_library_view.png'), fullPage: true });

  // === Case 5: Adjustment in Library ===
  console.log('Making another adjustment in library');
  const libChatInput = page.locator('#library-editor-workspace #chat-input').first();
  await libChatInput.waitFor({ state: 'visible', timeout: 15000 });
  await libChatInput.fill('add a delay');
  await page.click('#library-editor-workspace #chat-submit-btn');
  
  // Wait for response
  await expect(page.locator('#library-editor-workspace #chat-submit-btn').first()).not.toHaveClass(/htmx-request/, { timeout: 180000 });
  await page.locator('#library-editor-workspace .grid-matrix').first().waitFor({ state: 'visible', timeout: 30000 });
  
  await page.evaluate(() => { const h = document.querySelector('.nav-header'); if(h) h.style.position = 'static'; });
  await page.screenshot({ path: path.join(screenshotDir, 'real_bb_king_library_adjusted.png'), fullPage: true });
});
