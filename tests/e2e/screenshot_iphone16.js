const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

// captureFullMobilePage temporarily relaxes standard mobile standalone viewport caps 
// to let Playwright take a perfect full-height portrait scroll capture, keeping layout structures intact.
async function captureFullMobilePage(page, outputPath) {
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

  await page.screenshot({ path: outputPath, fullPage: true });

  // Restore dynamic standard physical viewport boundaries and safe-area notch docks
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

(async () => {
  const outputDir = path.join(__dirname, 'output', 'iphone16');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  console.log('Launching headless browser...');
  const browser = await chromium.launch();
  
  // iPhone 16 Pro configuration:
  // Viewport: 402 x 874, DPR: 3, User Agent: iOS 18 Safari, isMobile: true, hasTouch: true
  const context = await browser.newContext({
    viewport: { width: 402, height: 874 },
    deviceScaleFactor: 3,
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
    isMobile: true,
    hasTouch: true
  });

  const page = await context.newPage();

  // 1. Capture PWA Standalone Login Screen (Full screen)
  console.log('1. Navigating to login with mode=standalone...');
  await page.goto('http://localhost:8083/login?mode=standalone');
  await page.waitForSelector('input[name="password"]');
  await page.screenshot({ path: path.join(outputDir, '1_login_screen.png'), fullPage: true });

  // Authenticate
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // 2. Wait for redirect and capture PWA blank generator dashboard
  console.log('2. Waiting for PWA index page redirect (sessionStorage persisted)...');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '2_blank_generator.png'), fullPage: true });

  // 3. Trigger dynamic Agent generation with BB King rig target
  console.log('3. Submitting BB King prompt (BB King The Thrill Is Gone)...');
  await page.fill('input[name="prompt"]', 'BB King The Thrill Is Gone');
  await page.click('button#gen-submit-btn');

  // Wait for progress panel to show up
  await page.waitForSelector('#gen-progress-area .progress-panel', { timeout: 5000 });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(outputDir, '3_agent_progress.png'), fullPage: true });

  // 4. Wait for real evaluation workspace to build
  console.log('4. Waiting for BB King evaluation workspace to build...');
  await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
  await page.waitForTimeout(1500);
  
  // Capture Top Half of workspace
  await page.screenshot({ path: path.join(outputDir, '4_workspace_draft_matrix.png') });

  // 4b. Capture full-height Compact Scene A Rhythm table view (zero-horizontal-scroll!)
  console.log('4b. Capturing full-height Compact Scene A Rhythm table view...');
  // Scroll vertical main-view down to reveal table card first
  await page.evaluate(() => {
    const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-generator');
    if (activeView) activeView.scrollTop = 390;
  });
  await page.waitForTimeout(500);
  await captureFullMobilePage(page, path.join(outputDir, '4b_workspace_matrix_scene_a.png'));

  // 4b-modal. Tap the first effect's Info Icon ⓘ to open the Glassmorphic Agent Rationale popup modal!
  console.log('4b-modal. Tapping the Info Icon ⓘ of the first table row...');
  const firstInfoIcon = page.locator('table.grid-matrix td .info-icon').first();
  await firstInfoIcon.click();
  await page.waitForSelector('.reasoning-popover-card', { timeout: 5000 });
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(outputDir, '4b_workspace_matrix_scene_a_reasoning.png') });
  
  // Dismiss popover
  await page.click('.reasoning-popover-close');
  await page.waitForTimeout(300);

  // 4c. Tapping the "Scene B (Lead)" segmented button
  console.log('4c. Tapping the "Scene B (Lead)" Segmented control button...');
  await page.click('.mobile-scene-toggle-bar button:has-text("Scene B (Lead)")');
  await page.waitForTimeout(500);
  await captureFullMobilePage(page, path.join(outputDir, '4c_workspace_matrix_scene_b.png'));

  // 4d. Switching pickup tabs to Gibson ES-339 Humbuckers
  console.log('4d. Switching pickup tabs to Gibson ES-339 Humbuckers...');
  // Restore Scene A button state first for clean baseline
  await page.click('.mobile-scene-toggle-bar button:has-text("Scene A (Rhythm)")');
  await page.waitForTimeout(200);
  
  // Click guitar tab button
  await page.click('button:has-text("Gibson ES-339 Humbuckers")');
  await page.waitForTimeout(800);
  await captureFullMobilePage(page, path.join(outputDir, '4d_workspace_second_guitar.png'));

  // 5. Navigate to Preset Library tab
  console.log('5. Navigating to the Preset Library tab...');
  await page.evaluate(() => {
    const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-generator');
    if (activeView) activeView.scrollTop = 0;
  });
  await page.click('button:has-text("Preset Library")');
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await captureFullMobilePage(page, path.join(outputDir, '5_preset_library_list.png'));

  // 5b. Select a preset to load the dynamic workspace editor
  console.log('5b. Clicking the Adjust button of the first saved preset in the library feed...');
  const firstAdjustButton = page.locator('#library-list-container li button').first();
  await firstAdjustButton.click();
  await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
  await page.waitForTimeout(1500);
  await captureFullMobilePage(page, path.join(outputDir, '5b_preset_editor_active.png'));

  // 5c. Detect layout and edit parameter range slider
  const sliderCount = await page.locator('#library-editor-workspace input[type="range"]').count();
  if (sliderCount > 0) {
      console.log('5c. Tweaking the first active block input range slider programmatically...');
      const firstSlider = page.locator('#library-editor-workspace input[type="range"]').first();
      await firstSlider.evaluate(el => el.value = "8.2");
      await firstSlider.dispatchEvent('change');
      await page.waitForTimeout(800);
      await captureFullMobilePage(page, path.join(outputDir, '5c_preset_parameter_tweaked.png'));
  } else {
      console.log('5c. No block range sliders found. Taking backup visual parameters editor panel capture instead...');
      await captureFullMobilePage(page, path.join(outputDir, '5c_preset_parameter_settled.png'));
  }

  // 6. Navigate to Learned Rules tab
  console.log('6. Navigating to the Learned Rules tab...');
  await page.evaluate(() => {
    const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-library');
    if (activeView) activeView.scrollTop = 0;
  });
  await page.click('button:has-text("Learned Rules")');
  await page.waitForSelector('#rules-list-container', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await captureFullMobilePage(page, path.join(outputDir, '6_learned_rules.png'));

  console.log('All automated iPhone 16 Pro user journey screenshots captured successfully!');
  await browser.close();
})();
