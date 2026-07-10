// withFullMobileCapture temporarily relaxes standard mobile standalone
// viewport caps so a fullPage screenshot/assertion captures the whole
// scrollable layout without clipping, then restores the inline styles
// afterwards. Pass the toHaveScreenshot() assertion (or any screenshot call)
// as `takeShot`.
export async function withFullMobileCapture(page, takeShot) {
  await page.evaluate(() => {
    const views = document.querySelectorAll('.main-view');
    views.forEach(v => {
      v.style.setProperty('height', 'auto', 'important');
      v.style.setProperty('overflow', 'visible', 'important');
    });
    document.body.style.setProperty('height', 'auto', 'important');
    document.body.style.setProperty('overflow', 'visible', 'important');
    document.body.style.setProperty('position', 'static', 'important');

    // Temporarily draw the fixed bottom docked chat card inline so it flows naturally between cards in the capture
    const chatCard = document.querySelector('.workspace-wrapper > .card.chat-control-panel');
    if (chatCard) {
      chatCard.style.setProperty('position', 'static', 'important');
      chatCard.style.setProperty('margin-top', '1.5rem', 'important');
      chatCard.style.setProperty('box-shadow', 'none', 'important');
      chatCard.style.setProperty('padding', '1.5rem', 'important');
      chatCard.style.setProperty('width', '100%', 'important');
      chatCard.style.setProperty('border-radius', '12px', 'important');
    }

    const wrapper = document.querySelector('.workspace-wrapper');
    if (wrapper) {
      wrapper.style.setProperty('padding-bottom', '1.5rem', 'important');
    }
  });

  try {
    await takeShot();
  } finally {
    await page.evaluate(() => {
      const views = document.querySelectorAll('.main-view');
      views.forEach(v => {
        v.style.removeProperty('height');
        v.style.removeProperty('overflow');
      });
      document.body.style.removeProperty('height');
      document.body.style.removeProperty('overflow');
      document.body.style.removeProperty('position');

      const chatCard = document.querySelector('.workspace-wrapper > .card.chat-control-panel');
      if (chatCard) {
        chatCard.style.removeProperty('position');
        chatCard.style.removeProperty('margin-top');
        chatCard.style.removeProperty('box-shadow');
        chatCard.style.removeProperty('padding');
        chatCard.style.removeProperty('width');
        chatCard.style.removeProperty('border-radius');
      }

      const wrapper = document.querySelector('.workspace-wrapper');
      if (wrapper) {
        wrapper.style.removeProperty('padding-bottom');
      }
    });
  }
}

// Logs in against the deterministic mock pipeline (MOCK_MODE=true server-side,
// ?mock=true client-side) and submits a generation prompt. Canned output comes
// from testdata/e2e_mocks/architect_generate.json regardless of prompt text,
// so this is pixel-stable across runs.
export async function loginAndGenerate(page, prompt = 'Bright Telecaster Blues Tone') {
  await page.goto('/login?mock=true');
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });

  await page.fill('input[name="prompt"]', prompt);
  await page.click('button#gen-submit-btn');
  await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
  await page.waitForTimeout(500);
}
