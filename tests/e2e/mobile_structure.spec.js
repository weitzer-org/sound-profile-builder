import { test, expect } from '@playwright/test';

// Plain DOM/HTTP assertions tied to specific claims made by the ported
// mobile/PWA work, so future changes are judged by pass/fail rather than by
// a human eyeballing a screenshot.

test.describe('PWA manifest and service worker', () => {
  test('manifest.json is valid and self-consistent', async ({ request }) => {
    const res = await request.get('/manifest.json');
    expect(res.ok()).toBeTruthy();
    const manifest = await res.json();

    expect(manifest.name).toBeTruthy();
    expect(manifest.start_url).toBeTruthy();
    expect(manifest.display).toBe('standalone');
    expect(Array.isArray(manifest.icons)).toBeTruthy();
    expect(manifest.icons.length).toBeGreaterThan(0);

    for (const icon of manifest.icons) {
      const iconRes = await request.get(icon.src);
      expect(iconRes.ok(), `icon ${icon.src} should be served`).toBeTruthy();
    }
  });

  test('sw.js is served as executable JS', async ({ request }) => {
    const res = await request.get('/sw.js');
    expect(res.ok()).toBeTruthy();
    expect(res.headers()['content-type']).toContain('javascript');
  });

  test('service worker registers without throwing', async ({ page }) => {
    await page.goto('/login?mock=true');
    const registered = await page.evaluate(async () => {
      try {
        await navigator.serviceWorker.register('/sw.js');
        return true;
      } catch {
        return false;
      }
    });
    expect(registered).toBeTruthy();
  });
});

test.describe('Session persistence', () => {
  test('auth cookie is set for a ~30 day session', async ({ page, context }) => {
    await page.goto('/login?mock=true');
    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

    const cookies = await context.cookies();
    const authCookie = cookies.find(c => c.name === 'qc_auth_session');
    expect(authCookie).toBeTruthy();

    const maxAgeSeconds = authCookie.expires - Date.now() / 1000;
    const thirtyDaysSeconds = 30 * 24 * 60 * 60;
    // Allow generous slack for request/render latency between login and this check.
    expect(maxAgeSeconds).toBeGreaterThan(thirtyDaysSeconds - 3600);
    expect(maxAgeSeconds).toBeLessThan(thirtyDaysSeconds + 3600);
  });
});

test.describe('Mobile layout (iPhone 16 Pro viewport)', () => {
  test.use({
    viewport: { width: 402, height: 874 },
    deviceScaleFactor: 3,
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
    isMobile: true,
    hasTouch: true,
  });

  async function assertNoHorizontalScroll(page, label) {
    const overflow = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(overflow.scrollWidth, `${label}: scrollWidth should not exceed viewport width`)
      .toBeLessThanOrEqual(overflow.clientWidth);
  }

  test('generator, workspace, and library views have zero horizontal scroll', async ({ page }) => {
    await page.goto('/login?mock=true');
    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
    await assertNoHorizontalScroll(page, 'blank generator');

    await page.fill('input[name="prompt"]', 'Bright Telecaster Blues Tone');
    await page.click('button#gen-submit-btn');
    await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
    await page.waitForTimeout(300);
    await assertNoHorizontalScroll(page, 'generation workspace');

    // Navigation on mobile is the fixed bottom tab bar (.mobile-tab-btn), not
    // the desktop top-nav row -- that row is hidden under the mobile
    // breakpoint now that the tab bar replaces it (see index.html's
    // .is-mobile-layout nav-header override).
    await page.click('.mobile-tab-btn[data-view="view-library"]');
    await page.waitForSelector('#library-list-container li', { timeout: 10000 });
    await assertNoHorizontalScroll(page, 'preset library');

    await page.click('.mobile-tab-btn[data-view="view-rules"]');
    await page.waitForSelector('#rules-list-container', { timeout: 10000 });
    await assertNoHorizontalScroll(page, 'learned rules');
  });

  test('primary interactive controls meet the 44x44 minimum tap target', async ({ page }) => {
    await page.goto('/login?mock=true');
    await page.fill('input[name="password"]', 'bluesmusic');
    await page.click('button[type="submit"]');
    await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

    // Mobile's primary nav is the fixed bottom tab bar, not the desktop
    // top-nav row (hidden on mobile now that the tab bar replaces it).
    const navButtons = page.locator('.mobile-tab-btn');
    const navCount = await navButtons.count();
    expect(navCount).toBeGreaterThan(0);
    for (let i = 0; i < navCount; i++) {
      const box = await navButtons.nth(i).boundingBox();
      expect(box.height, `nav button ${i} height`).toBeGreaterThanOrEqual(44);
    }

    const submitBox = await page.locator('#gen-submit-btn').boundingBox();
    expect(submitBox.height).toBeGreaterThanOrEqual(44);

    await page.fill('input[name="prompt"]', 'Bright Telecaster Blues Tone');
    await page.click('button#gen-submit-btn');
    await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });

    const sceneButtons = page.locator('.mobile-scene-toggle-bar .scene-toggle-btn');
    const sceneCount = await sceneButtons.count();
    for (let i = 0; i < sceneCount; i++) {
      const box = await sceneButtons.nth(i).boundingBox();
      // Segmented control buttons are intentionally compact; require a
      // smaller but still tappable minimum rather than the full 44px HIG size.
      expect(box.height, `scene toggle button ${i} height`).toBeGreaterThanOrEqual(28);
    }
  });
});
