import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

// Real pipeline output in the preset views. Build fixtures from saved eval outputs with
// cmd/e2e_eval_fixtures, seed them (E2E_SEED=1 E2E_SEED_EXTRA=<dir>), and point EVAL_FIXTURES
// at the same directory. For every catalog block of every guitar, the View must show every
// device control where the fixture says: as a parameter row (the preset's own name) or by
// name in the "At device default" line. Screenshots of each guitar's View and of Edit go to
// SCREENSHOT_DIR. Without EVAL_FIXTURES every test here is skipped. Mock mode: no Gemini
// calls (the presets are already generated).

const shots = process.env.SCREENSHOT_DIR || path.join('test-results', 'eval-presets');
const dir = process.env.EVAL_FIXTURES || '';
const fixtures = dir
  ? fs.readdirSync(dir).filter((f) => f.endsWith('.json')).sort().map((f) => JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8')))
  : [];
const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

test.use({ viewport: { width: 1280, height: 960 }, deviceScaleFactor: 2 });

test('eval fixtures are configured', async () => {
  test.skip(fixtures.length === 0, 'set EVAL_FIXTURES to a directory made by cmd/e2e_eval_fixtures');
});

for (const fx of fixtures) {
  test(`real preset: ${fx.name}`, async ({ page }) => {
    await page.goto('/login?mock=true');
    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

    await page.locator('button:has-text("Preset Library")').first().click();
    await page.waitForSelector('#library-list-container li', { timeout: 15000 }).catch(() => {});
    const item = () => page.locator('#library-list-container li', { hasText: fx.name }).first();
    const more = page.locator('#library-list-container button:has-text("Load More")');
    for (let i = 0; i < 5 && !(await item().count()) && (await more.count()); i++) {
      await more.first().click();
      await item().waitFor({ timeout: 3000 }).catch(() => {});
    }
    test.skip(!(await item().count()), `"${fx.name}" not in the library: seed it first`);
    await item().locator('button:has-text("Adjust preset")').click();
    const ws = page.locator('#library-editor-workspace');
    await ws.locator('.card').first().waitFor({ timeout: 15000 });

    await ws.locator('button:has-text("View")').first().click();
    await ws.locator('table.grid-matrix').first().waitFor({ timeout: 10000 });
    let checked = 0;
    for (const guitar of Object.keys(fx.expected_controls).sort()) {
      await ws.locator('.tab-btn:visible', { hasText: guitar }).first().click();
      await page.waitForTimeout(300); // let the tab switch finish before reading and the screenshot
      const pane = ws.locator('.tab-pane:visible').first();
      const seen = {};
      for (const blk of fx.expected_controls[guitar]) {
        // The nth row whose block heading is "Type: Model" (a guitar can carry the same model
        // twice; another block's rationale may mention this one, so match the heading only).
        const heading = `${blk.type}: ${blk.model}`;
        const n = (seen[heading] = (seen[heading] ?? -1) + 1);
        const row = pane.locator('tbody tr').filter({ has: page.locator('td:first-child', { hasText: new RegExp('^\\s*' + escapeRe(heading)) }) }).nth(n);
        const text = await row.innerText();
        const missing = blk.controls.filter((c) => !text.includes(c));
        expect(missing, `${guitar} / ${blk.model}: device controls not shown`).toEqual([]);
        checked += blk.controls.length;
      }
      await ws.screenshot({ path: path.join(shots, `${slug(fx.name)}-view-${slug(guitar)}.png`) });
    }
    test.info().annotations.push({ type: 'device controls checked', description: String(checked) });
    console.log(`${fx.name}: ${checked} device controls found on screen`);

    await ws.locator('button:has-text("Edit")').first().click();
    await ws.locator('.effect-block').first().waitFor({ timeout: 10000 });
    await page.waitForTimeout(300);
    await ws.screenshot({ path: path.join(shots, `${slug(fx.name)}-edit.png`) });
  });
}
