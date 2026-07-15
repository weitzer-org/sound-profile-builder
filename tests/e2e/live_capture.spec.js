// Live-mode capture: drives the real Gemini pipeline (not MOCK_MODE) once per
// viewport, screenshots the redesigned UI with genuine generated content, and
// exercises the View/Edit toggle end-to-end against real persisted data.
// Not a pass/fail regression spec -- just captures evidence. Run against a
// server started with `./startup_mobile.sh --live` (real GEMINI_API_KEY).
import { test, expect } from '@playwright/test';
import { withFullMobileCapture } from './helpers.js';

test.setTimeout(600000);

async function runLiveJourney(page, { prompt, presetName, mobile }) {
  await page.goto(mobile ? '/login?mode=standalone' : '/login');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

  const shot = async (name) => {
    const opts = { fullPage: true };
    if (mobile) {
      await withFullMobileCapture(page, () => page.screenshot({ path: `live-shots/${name}.png`, ...opts }));
    } else {
      await page.screenshot({ path: `live-shots/${name}.png`, ...opts });
    }
  };

  await shot(`${mobile ? 'mobile' : 'desktop'}-live-1-blank-generator`);

  await page.fill('input[name="prompt"]', prompt);
  await page.click('#gen-submit-btn');

  console.log(`[${mobile ? 'mobile' : 'desktop'}] waiting on real Gemini generation...`);
  await page.waitForSelector('.tweaking-workspace', { timeout: 300000 });
  await page.waitForTimeout(500);
  await shot(`${mobile ? 'mobile' : 'desktop'}-live-2-generated`);

  // Save it (Finalize Save -> handleRenamePreset, no extra API call).
  await page.fill('input[placeholder="Enter custom name..."]', presetName);
  await page.click('button:has-text("Finalize Save")');
  await page.waitForTimeout(1000);

  // Library: find the saved preset and open it (Adjust preset -> Edit mode by default).
  if (mobile) {
    await page.click('.mobile-tab-btn[data-view="view-library"]');
  } else {
    await page.click('button.top-nav-btn:has-text("Preset Library")');
  }
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });
  await shot(`${mobile ? 'mobile' : 'desktop'}-live-3-library-list`);

  const row = page.locator(`#library-list-container li:has-text("${presetName}")`).first();
  await row.locator('button:has-text("Adjust preset")').click();
  await page.waitForSelector('#library-editor-workspace .param-group, #library-editor-workspace table.grid-matrix', { timeout: 15000 });
  await page.waitForTimeout(300);
  await shot(`${mobile ? 'mobile' : 'desktop'}-live-4-edit-mode`);

  // Flip to View mode -- proves the real persisted rationale table survives save.
  const viewBtn = page.locator('#library-editor-workspace button:has-text("View")').first();
  if (await viewBtn.count()) {
    await viewBtn.click();
    await page.waitForTimeout(500);
    await shot(`${mobile ? 'mobile' : 'desktop'}-live-5-view-mode`);
  } else {
    console.log(`[${mobile ? 'mobile' : 'desktop'}] no View/Edit toggle found (unexpected)`);
  }
}

test.describe('Desktop live capture', () => {
  test.use({ viewport: { width: 1280, height: 960 } });
  test('real Gemini generation, save, view/edit', async ({ page }) => {
    await runLiveJourney(page, { prompt: 'Texas Flood by Stevie Ray Vaughan', presetName: 'Live Capture - Texas Flood (Desktop)', mobile: false });
  });
});

test.describe('Mobile live capture', () => {
  test.use({
    viewport: { width: 402, height: 874 },
    deviceScaleFactor: 3,
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
    isMobile: true,
    hasTouch: true,
  });
  test('real Gemini generation, save, view/edit', async ({ page }) => {
    await runLiveJourney(page, { prompt: 'Texas Flood by Stevie Ray Vaughan', presetName: 'Live Capture - Texas Flood (Mobile)', mobile: true });
  });
});
