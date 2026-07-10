import { test, expect } from '@playwright/test';

test.use({
  viewport: { width: 1280, height: 960 },
  deviceScaleFactor: 1,
});

test.describe('Desktop visual regression (mock mode)', () => {
  test('login, generation, and workspace journey', async ({ page }) => {
    await page.goto('/login?mock=true');
    await page.waitForSelector('input[name="password"]');
    await expect(page).toHaveScreenshot('desktop-1-login.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('desktop-2-blank-generator.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    await page.fill('input[name="prompt"]', 'Bright Telecaster Blues Tone');
    await page.click('button#gen-submit-btn');
    await page.waitForSelector('#gen-progress-area .progress-panel', { timeout: 5000 });
    await expect(page).toHaveScreenshot('desktop-3-agent-progress.png', { maxDiffPixelRatio: 0.03 });

    await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('desktop-4-workspace-draft.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    await page.click('button:has-text("Gibson ES-339 Humbuckers")');
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('desktop-5-second-guitar.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    await page.click('button:has-text("Preset Library")');
    await page.waitForSelector('#library-list-container li', { timeout: 10000 });
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('desktop-6-library-list.png', { fullPage: true, maxDiffPixelRatio: 0.02 });

    const firstAdjustButton = page.locator('#library-list-container li button').first();
    await firstAdjustButton.click();
    await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot('desktop-7-preset-editor.png', { fullPage: true, maxDiffPixelRatio: 0.02 });
  });
});
