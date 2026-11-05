import { test, expect } from '@playwright/test';
import * as path from 'path';

// Screenshots of real eval outputs in the preset views, for judging an eval by eye (e.g. the
// Architect v5-vs-v6 comparison). Convert eval_full_pipeline outputs to e2e_seed fixtures,
// seed them with E2E_SEED=1 E2E_SEED_EXTRA=<dir>, and list their names in EVAL_PRESETS
// (comma-separated). Without EVAL_PRESETS every test here is skipped. Mock mode: no Gemini calls.

const shots = process.env.SCREENSHOT_DIR || path.join('test-results', 'eval-presets');
const names = (process.env.EVAL_PRESETS || '').split(',').map((s) => s.trim()).filter(Boolean);
const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

test.use({ viewport: { width: 1280, height: 960 }, deviceScaleFactor: 2 });

test.skip(names.length === 0, 'set EVAL_PRESETS to the seeded preset names');

for (const name of names) {
  test(`eval preset: ${name}`, async ({ page }) => {
    await page.goto('/login?mock=true');
    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

    await page.locator('button:has-text("Preset Library")').first().click();
    await page.waitForSelector('#library-list-container li', { timeout: 15000 });
    const item = () => page.locator('#library-list-container li', { hasText: name }).first();
    const more = page.locator('#library-list-container button:has-text("Load More")');
    for (let i = 0; i < 5 && !(await item().count()) && (await more.count()); i++) {
      await more.first().click();
      await item().waitFor({ timeout: 3000 }).catch(() => {});
    }
    test.skip(!(await item().count()), `"${name}" not in the library: seed it first`);
    await item().locator('button:has-text("Adjust preset")').click();
    const ws = page.locator('#library-editor-workspace');
    await ws.locator('.card').first().waitFor({ timeout: 15000 });

    // View: one full screenshot per guitar tab.
    await ws.locator('button:has-text("View")').first().click();
    await ws.locator('table.grid-matrix').first().waitFor({ timeout: 10000 });
    const tabs = ws.locator('.tab-btn:visible').filter({ hasText: /Gibson|Fender|Telecaster|ES-339/ });
    const n = Math.max(1, await tabs.count());
    for (let i = 0; i < n; i++) {
      if (await tabs.count()) {
        await tabs.nth(i).click();
      }
      await page.waitForTimeout(300); // let the tab switch finish before the screenshot
      await expect(ws.locator('.tab-pane:visible table.grid-matrix').first()).toBeVisible();
      await ws.screenshot({ path: path.join(shots, `${slug(name)}-view-${i + 1}.png`) });
    }

    // Edit: every block's controls.
    await ws.locator('button:has-text("Edit")').first().click();
    await ws.locator('.effect-block').first().waitFor({ timeout: 10000 });
    await page.waitForTimeout(300);
    await ws.screenshot({ path: path.join(shots, `${slug(name)}-edit.png`) });
  });
}
