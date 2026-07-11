import { test, expect } from '@playwright/test';
import { withFullMobileCapture } from './helpers.js';

// iPhone 16 Pro device profile, matching the original manual screenshot script.
test.use({
  viewport: { width: 402, height: 874 },
  deviceScaleFactor: 3,
  userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
  isMobile: true,
  hasTouch: true,
});

test.describe('Mobile visual regression (iPhone 16 Pro, mock mode)', () => {
  test('login, generation, and workspace journey', async ({ page }) => {
    await page.goto('/login?mode=standalone&mock=true');
    await page.waitForSelector('input[name="password"]');
    await expect(page).toHaveScreenshot('mobile-1-login.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('mobile-2-blank-generator.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    await page.fill('input[name="prompt"]', 'Bright Telecaster Blues Tone');
    await page.click('button#gen-submit-btn');
    await page.waitForSelector('#gen-progress-area .progress-panel', { timeout: 5000 });
    await expect(page).toHaveScreenshot('mobile-3-agent-progress.png', { maxDiffPixelRatio: 0.03 });

    await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('mobile-4-workspace-draft.png', { maxDiffPixelRatio: 0.02 });

    // Scene A (Rhythm), zero-horizontal-scroll compact table view.
    await page.evaluate(() => {
      const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-generator');
      if (activeView) activeView.scrollTop = 390;
    });
    await page.waitForTimeout(300);
    await withFullMobileCapture(page, () =>
      expect(page).toHaveScreenshot('mobile-5-scene-a.png', { fullPage: true, maxDiffPixelRatio: 0.02 })
    );

    // Scene B (Lead) segmented toggle.
    await page.click('.mobile-scene-toggle-bar button:has-text("Scene B (Lead)")');
    await page.waitForTimeout(300);
    await withFullMobileCapture(page, () =>
      expect(page).toHaveScreenshot('mobile-7-scene-b.png', { fullPage: true, maxDiffPixelRatio: 0.02 })
    );

    // Second guitar tab.
    await page.click('.mobile-scene-toggle-bar button:has-text("Scene A (Rhythm)")');
    await page.waitForTimeout(200);
    await page.click('button:has-text("Gibson ES-339 Humbuckers")');
    await page.waitForTimeout(500);
    await withFullMobileCapture(page, () =>
      expect(page).toHaveScreenshot('mobile-8-second-guitar.png', { fullPage: true, maxDiffPixelRatio: 0.02 })
    );

    // Preset library.
    await page.evaluate(() => {
      const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-generator');
      if (activeView) activeView.scrollTop = 0;
    });
    // Mobile nav is the fixed bottom tab bar; the desktop top-nav row is
    // hidden under this viewport now that the tab bar replaces it.
    await page.click('.mobile-tab-btn[data-view="view-library"]');
    await page.waitForSelector('#library-list-container li', { timeout: 10000 });
    await page.waitForTimeout(500);
    await withFullMobileCapture(page, () =>
      expect(page).toHaveScreenshot('mobile-9-library-list.png', { fullPage: true, maxDiffPixelRatio: 0.02 })
    );

    // Preset editor.
    const firstAdjustButton = page.locator('#library-list-container li button').first();
    await firstAdjustButton.click();
    await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
    await page.waitForTimeout(500);
    await withFullMobileCapture(page, () =>
      expect(page).toHaveScreenshot('mobile-10-preset-editor.png', { fullPage: true, maxDiffPixelRatio: 0.02 })
    );
  });
});
