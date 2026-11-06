import { test, expect } from '@playwright/test';
import * as fs from 'fs';

// The "Download device JSON" link (plan P2): one per guitar in the library editor, serving
// that guitar's Quad Cortex device export as a JSON attachment. Uses the synthetic v6
// preset in testdata/e2e_presets (run with `E2E_SEED=1 scripts/e2e-native.sh
// device_export.spec.js`); skipped when the fixture isn't loaded. Mock mode: no Gemini.

const V6 = 'Data grid: v6-style preset';

async function login(page) {
  await page.goto('/login?mock=true');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
}

async function openPreset(page, name) {
  await page.locator('button:has-text("Preset Library")').first().click();
  await page.waitForSelector('#library-list-container li', { timeout: 15000 }).catch(() => {});
  const item = () => page.locator('#library-list-container li', { hasText: name }).first();
  if (!(await item().count())) {
    const more = page.locator('#library-list-container button:has-text("Load More")');
    if (await more.count()) {
      await more.first().click();
      await item().waitFor({ timeout: 5000 }).catch(() => {});
    }
  }
  test.skip(!(await item().count()), `fixture "${name}" not in the library: run with E2E_SEED=1`);
  await item().locator('button:has-text("Adjust preset")').click();
  await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
}

test('library editor offers a device JSON download per guitar', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 1200 });
  await login(page);
  await openPreset(page, V6);

  const links = page.locator('#library-editor-workspace a.device-export-link');
  await expect(links.first()).toBeVisible();
  const labels = await links.allTextContents();
  expect(labels.length).toBeGreaterThanOrEqual(1);
  for (const l of labels) expect(l).toMatch(/^Download device JSON \(.+\)$/);

  const [download] = await Promise.all([page.waitForEvent('download'), links.first().click()]);
  expect(download.suggestedFilename()).toMatch(/^qc2-device-[a-z0-9-]+\.json$/);
  const body = JSON.parse(fs.readFileSync(await download.path(), 'utf8'));
  expect(body.format).toBe('qc2-device-export/1');
  expect(body.catalog.sha256).toMatch(/^[0-9a-f]{64}$/);
  expect(body.rows[0]).toMatchObject({ row: 0, in_port: 1, out_port: 19 });
  expect(body.scene_sources).toEqual(['A', 'B', 'A', 'A', 'A', 'A', 'A', 'A']);
  for (const b of body.rows[0].blocks) {
    expect(b.bypass).toHaveLength(8);
    for (const p of b.params) {
      expect(p.display).toHaveLength(8);
      expect(p.wire).toHaveLength(8);
    }
  }
  expect(Array.isArray(body.unresolved)).toBe(true);
  await page.waitForTimeout(800);
});
