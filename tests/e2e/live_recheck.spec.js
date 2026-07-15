// Re-opens the already-saved live presets from live_capture.spec.js (no new
// Gemini call) to verify the slider/unit fix and check the toast-overlap
// artifact the user flagged, with a longer settle wait before screenshotting.
import { test, expect } from '@playwright/test';
import { withFullMobileCapture } from './helpers.js';

test.setTimeout(120000);

async function recheck(page, { presetName, mobile }) {
  await page.goto(mobile ? '/login?mode=standalone' : '/login');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

  if (mobile) {
    await page.click('.mobile-tab-btn[data-view="view-library"]');
  } else {
    await page.click('button.top-nav-btn:has-text("Preset Library")');
  }
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });

  // Let any leftover toast from a prior run fully fade before we start.
  await page.waitForTimeout(3000);

  const row = page.locator(`#library-list-container li:has-text("${presetName}")`).first();
  await row.locator('button:has-text("Adjust preset")').click();
  await page.waitForSelector('#library-editor-workspace .param-group, #library-editor-workspace table.grid-matrix', { timeout: 15000 });
  await page.waitForTimeout(1000); // let any toast from this navigation settle too

  const shot = async (name) => {
    if (mobile) {
      await withFullMobileCapture(page, () => page.screenshot({ path: `live-shots/${name}.png`, fullPage: true }));
    } else {
      await page.screenshot({ path: `live-shots/${name}.png`, fullPage: true });
    }
  };

  await shot(`${mobile ? 'mobile' : 'desktop'}-recheck-edit-mode`);

  const viewBtn = page.locator('#library-editor-workspace button:has-text("View")').first();
  await viewBtn.click();
  await page.waitForTimeout(1000);
  await shot(`${mobile ? 'mobile' : 'desktop'}-recheck-view-mode`);
}

test.describe('Desktop recheck', () => {
  test.use({ viewport: { width: 1280, height: 960 } });
  test('slider fix + toast overlap check', async ({ page }) => {
    await recheck(page, { presetName: 'Live Capture - Texas Flood (Desktop)', mobile: false });
  });
});

test.describe('Mobile recheck', () => {
  test.use({
    viewport: { width: 402, height: 874 },
    deviceScaleFactor: 3,
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
    isMobile: true,
    hasTouch: true,
  });
  test('slider fix + toast overlap check', async ({ page }) => {
    await recheck(page, { presetName: 'Live Capture - Texas Flood (Mobile)', mobile: true });
  });
});
