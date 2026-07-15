// Ad-hoc verification for the Builder Statement mobile clamp/expand fix.
// Uses MOCK_MODE (free, fast) since this is checking CSS/JS behavior, not
// generation content. Not part of the tracked suite.
import { test, expect } from '@playwright/test';

test.use({
  viewport: { width: 402, height: 874 },
  deviceScaleFactor: 3,
  userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
  isMobile: true,
  hasTouch: true,
});

test('builder statement clamp/expand on mobile', async ({ page }) => {
  await page.goto('/login?mode=standalone');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

  await page.fill('input[name="prompt"]', 'brighter tone with more edge');
  await page.click('#gen-submit-btn');
  await page.waitForSelector('.tweaking-workspace', { timeout: 30000 });

  const statement = page.locator('.builder-statement-text').first();
  await statement.scrollIntoViewIfNeeded();
  await page.screenshot({ path: 'live-shots/builder-statement-1-generation-clamped.png' });

  await statement.click();
  await page.waitForTimeout(200);
  await page.screenshot({ path: 'live-shots/builder-statement-2-generation-expanded.png' });

  // Save it, then re-check on the saved-preset Adjust workspace too.
  await page.click('button:has-text("Finalize Save")');
  await page.waitForTimeout(1000);
  await page.click('.mobile-tab-btn[data-view="view-library"]');
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });
  await page.locator('#library-list-container li').first().locator('button:has-text("Adjust preset")').click();
  await page.waitForSelector('#library-editor-workspace .builder-statement-text', { timeout: 10000 });

  const savedStatement = page.locator('#library-editor-workspace .builder-statement-text').first();
  await savedStatement.scrollIntoViewIfNeeded();
  await page.screenshot({ path: 'live-shots/builder-statement-3-saved-clamped.png' });

  await savedStatement.click();
  await page.waitForTimeout(200);
  await page.screenshot({ path: 'live-shots/builder-statement-4-saved-expanded.png' });

  // #view-library (not window/body -- body is position:fixed/overflow:hidden
  // on mobile, a scroll-lock; .main-view is the actual scroll container) --
  // scroll it to the bottom and confirm the last bit of real content (the
  // ADK log accordion, at the very end of the DSP matrix card) is fully
  // visible above the docked panel, not hidden behind it.
  await page.evaluate(() => {
    const view = document.getElementById('view-library');
    view.scrollTop = view.scrollHeight;
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: 'live-shots/builder-statement-5-saved-scrolled-to-bottom.png' });

  const logAccordion = page.locator('#library-editor-workspace summary:has-text("View ADK Processing Log")');
  await expect(logAccordion).toBeInViewport();
  const accordionBox = await logAccordion.boundingBox();
  const panelBox = await page.locator('#library-editor-workspace .chat-control-panel').boundingBox();
  if (accordionBox && panelBox && accordionBox.y + accordionBox.height > panelBox.y) {
    throw new Error(`ADK log accordion (bottom ${accordionBox.y + accordionBox.height}) overlaps the docked panel (top ${panelBox.y})`);
  }
});
