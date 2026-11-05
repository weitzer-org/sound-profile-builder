import { test, expect } from '@playwright/test';
import * as path from 'path';
import { withFullMobileCapture } from './helpers.js';

// The preset data grid -- the read-only View table and the Edit controls -- on the
// synthetic presets in testdata/e2e_presets (load them with `E2E_SEED=1
// scripts/e2e-native.sh data_grid.spec.js`, or `go run ./cmd/e2e_seed` against the
// server's store). Every use case asserts what it should show and saves a screenshot to
// SCREENSHOT_DIR (default test-results/data-grid). Mock mode: no Gemini calls.

const shots = process.env.SCREENSHOT_DIR || path.join('test-results', 'data-grid');
const V6 = 'Data grid: v6-style preset';
const V4 = 'Data grid: v4-style preset with device-check notes';
const NARRATED = 'Data grid: older preset with a narrated table';

async function login(page, mobile) {
  await page.goto(mobile ? '/login?mode=standalone&mock=true' : '/login?mock=true');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
}

// openPreset opens a seeded preset in the library editor, skipping the test when the
// fixtures weren't loaded.
async function openPreset(page, name, mobile) {
  if (mobile) {
    await page.click('.mobile-tab-btn[data-view="view-library"]');
  } else {
    await page.locator('button:has-text("Preset Library")').first().click();
  }
  // An empty library (fixtures not seeded) shows no items at all: skip rather than time out.
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

const ws = (page) => page.locator('#library-editor-workspace');

async function showView(page) {
  await ws(page).locator('button:has-text("View")').first().click();
  await ws(page).locator('table.grid-matrix').first().waitFor({ timeout: 10000 });
  await page.waitForTimeout(400);
}

async function showEdit(page) {
  await ws(page).locator('button:has-text("Edit")').first().click();
  await ws(page).locator('.effect-block').first().waitFor({ timeout: 10000 });
  await page.waitForTimeout(400);
}

// rowFor is the View table row of the block with this model, in the visible guitar tab.
const rowFor = (page, model) => ws(page).locator('.tab-pane:visible tr', { hasText: model }).first();

test.describe('data grid, desktop', () => {
  test.use({ viewport: { width: 1280, height: 960 }, deviceScaleFactor: 2 });

  test('v6-style preset: View, second guitar, Edit, and an edit flowing into View', async ({ page }) => {
    await login(page, false);
    await openPreset(page, V6, false);

    // 1. View: only changed controls, one "At device default" line, Scene B differences marked.
    await showView(page);
    const comp = rowFor(page, 'Opto Comp');
    await expect(comp.locator('.defaults-note')).toHaveText('At device default: ATTACK, RELEASE, MAKEUP, MIX');
    await expect(comp).toContainText('THRESHOLD: -30 dB');
    const delay = rowFor(page, 'Analog Delay');
    await expect(delay.locator('.param.diff')).toHaveText(['MIX: 30 %', 'DELAY TIME: 380 ms', 'FEEDBACK: 30 %']);
    await expect(delay).not.toContainText('WIDTH: 60');
    await ws(page).screenshot({ path: path.join(shots, '01-v6-view-telecaster.png') });

    // 2. The second guitar's tab.
    await ws(page).locator('button:has-text("Gibson ES-339 Humbuckers")').first().click();
    await page.waitForTimeout(400);
    await expect(rowFor(page, 'Green 808')).toContainText('OVERDRIVE: 2');
    await ws(page).screenshot({ path: path.join(shots, '02-v6-view-gibson.png') });

    // 3. Edit: every device control, the device's ranges and positions, untouched ones tagged.
    await showEdit(page);
    const ratio = ws(page).locator(`input[type="range"][hx-vals*='"param_name":"RATIO"']`).first();
    await expect(ratio).toHaveAttribute('min', '2');
    await expect(ratio).toHaveAttribute('max', '20');
    const trails = ws(page).locator(`.effect-block:has-text("Analog Delay") select[hx-vals*='"param_name":"TRAILS"']`).first();
    await expect(trails).toHaveValue('On');
    await expect(ws(page).locator('.param-default-tag').first()).toBeVisible();
    await ws(page).screenshot({ path: path.join(shots, '03-v6-edit.png') });

    // 4. Change a control that was at its default (the delay's WIDTH, 60 %) and see it
    //    become a row in View and leave the "At device default" line.
    const width = ws(page).locator(`input[type="range"][hx-vals*='"block_id":"t-6"'][hx-vals*='"param_name":"WIDTH"']`).first();
    const saved = page.waitForResponse((r) => r.url().includes('/api/preset/update_parameter') && r.status() === 204);
    await width.fill('75');
    await saved;
    // The live value above the slider follows it, with the control's unit.
    await expect(width.locator('xpath=preceding-sibling::div[1]/span[last()]')).toHaveText('75 %');
    await ws(page).locator('.effect-block:has-text("Analog Delay")').first().screenshot({ path: path.join(shots, '04a-edit-default-control.png') });
    await showView(page);
    const delayAfter = rowFor(page, 'Analog Delay');
    await expect(delayAfter).toContainText('WIDTH: 75 %');
    await expect(delayAfter.locator('.defaults-note')).not.toContainText('WIDTH');
    await ws(page).screenshot({ path: path.join(shots, '04b-view-after-edit.png') });
  });

  test('v4-style preset: device-check notes, a capture, an invented block name', async ({ page }) => {
    await login(page, false);
    await openPreset(page, V4, false);

    // 5. View: the post-generation checks' notes are shown (the table is rendered from the
    //    checked data); a capture keeps every parameter; no defaults line where there's none.
    await showView(page);
    await expect(rowFor(page, 'Green 808')).toContainText("Drive isn't a control on the device's Green 808");
    await expect(rowFor(page, 'Plate Reverb')).toContainText("isn't a block on the device");
    await expect(rowFor(page, '412 Brit 60B V30 95')).toContainText('no Blend control on a cab');
    const capture = rowFor(page, 'US DLX 58');
    await expect(capture).toContainText('Volume: 0.0 dB');
    await expect(capture.locator('.defaults-note')).toHaveCount(0);
    await ws(page).screenshot({ path: path.join(shots, '05-v4-view-device-checks.png') });

    // 6. Edit: the device's controls, plus the preset's own non-device names as text boxes.
    await showEdit(page);
    await expect(ws(page).locator(`.effect-block:has-text("Green 808") input[hx-vals*='"param_name":"Drive"']`)).toHaveCount(1);
    await ws(page).screenshot({ path: path.join(shots, '06-v4-edit.png') });
  });

  test('older preset: its stored narrated table is kept', async ({ page }) => {
    await login(page, false);
    await openPreset(page, NARRATED, false);
    // 7. No block has a rationale in the structured data, so the stored table is shown.
    await showView(page);
    await expect(ws(page)).toContainText('narrated by an older Architect');
    await ws(page).screenshot({ path: path.join(shots, '07-narrated-view.png') });
  });

  test('generation draft (mock run)', async ({ page }) => {
    await login(page, false);
    // 8. A fresh generation's draft workspace, from mock mode's canned Architect output.
    await page.fill('input[name="prompt"]', 'Bright Telecaster blues tone');
    await page.click('button#gen-submit-btn');
    await page.waitForSelector('.tweaking-workspace', { timeout: 30000 });
    await page.waitForTimeout(500);
    await expect(page.locator('.tweaking-workspace table.grid-matrix').first()).toBeVisible();
    await page.locator('.tweaking-workspace').first().screenshot({ path: path.join(shots, '08-generation-draft.png') });
  });
});

test.describe('data grid, phone', () => {
  test.use({
    viewport: { width: 402, height: 874 }, deviceScaleFactor: 3, isMobile: true, hasTouch: true,
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
  });

  test('v6-style preset: View Scene A, Scene B, and Edit', async ({ page }) => {
    await login(page, true);
    await openPreset(page, V6, true);

    // 9. View, Scene A: one chip per changed control; the defaults line under the block name.
    await showView(page);
    await expect(rowFor(page, 'Opto Comp').locator('.defaults-note')).toBeVisible();
    await withFullMobileCapture(page, () => page.screenshot({ path: path.join(shots, '09-phone-view-scene-a.png'), fullPage: true }));

    // 10. View, Scene B: the controls that differ from Scene A are highlighted.
    await ws(page).locator('.mobile-scene-toggle-bar button:has-text("Scene B (Lead)")').first().click();
    await page.waitForTimeout(400);
    await expect(rowFor(page, 'Analog Delay').locator('.param.diff').first()).toBeVisible();
    await withFullMobileCapture(page, () => page.screenshot({ path: path.join(shots, '10-phone-view-scene-b.png'), fullPage: true }));

    // 11. Edit: one block's controls (the full page is very long on a phone).
    await showEdit(page);
    await ws(page).locator('.effect-block:has-text("Analog Delay")').first().screenshot({ path: path.join(shots, '11-phone-edit-delay.png') });
  });
});
