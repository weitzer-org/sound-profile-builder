const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

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

  // 1. Capture PWA Standalone Login Screen
  console.log('1. Navigating to login with mode=standalone...');
  await page.goto('http://localhost:8082/login?mode=standalone');
  await page.waitForSelector('input[name="password"]');
  await page.screenshot({ path: path.join(outputDir, '1_login_screen.png') });

  // Authenticate
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // 2. Wait for redirect and capture PWA blank generator dashboard
  console.log('2. Waiting for PWA index page redirect (sessionStorage persisted)...');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
  await page.waitForTimeout(1000); // Let PWA flex bounds load
  await page.screenshot({ path: path.join(outputDir, '2_blank_generator.png') });

  // 3. Trigger dynamic Agent generation and capture progress indicator
  console.log('3. Submitting the real-data evaluation query hook (TEST_EVAL_SRV_CLEAN)...');
  await page.fill('input[name="prompt"]', 'TEST_EVAL_SRV_CLEAN');
  await page.click('button#gen-submit-btn');

  // Wait for progress panel to show up
  await page.waitForSelector('#gen-progress-area .progress-panel', { timeout: 5000 });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(outputDir, '3_agent_progress.png') });

  // 4. Wait for real evaluation workspace to build
  console.log('4. Waiting for real evaluation workspace to build...');
  await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
  await page.waitForTimeout(1500); // Settle time
  
  // Capture Top Half of real matrix grid
  await page.screenshot({ path: path.join(outputDir, '4_workspace_draft_matrix.png') });

  // Scroll .main-view container down so the Live DSP Matrix is fully in focus!
  console.log('4b. Scrolling vertical main-view to focus on the Live DSP Matrix card...');
  await page.evaluate(() => {
    const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-generator');
    if (activeView) activeView.scrollTop = 390; // Scroll down 390px to align the card!
  });
  await page.waitForTimeout(500);

  // Capture initial Compact Scene A (Rhythm) table view (with zero horizontal overflow!)
  console.log('4b. Capturing initial Compact Scene A (Rhythm) view (zero-horizontal-scroll!)...');
  await page.screenshot({ path: path.join(outputDir, '4b_workspace_matrix_scene_a.png') });

  // 4c. Tapping the "Scene B (Lead)" Segmented control button to switch the parameters view!
  console.log('4c. Tapping the "Scene B (Lead)" Segmented control button...');
  await page.click('.mobile-scene-toggle-bar button:has-text("Scene B (Lead)")');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(outputDir, '4c_workspace_matrix_scene_b.png') });

  // 4d. Switching pickup tabs to Gibson ES-339 Humbuckers!
  console.log('4d. Switching pickup tabs to Gibson ES-339 Humbuckers...');
  // Restore Scene A button state first for clean baseline
  await page.click('.mobile-scene-toggle-bar button:has-text("Scene A (Rhythm)")');
  await page.waitForTimeout(200);
  
  // Click the secondary guitar tab button inside our segmented control tabs header!
  await page.click('button:has-text("Gibson ES-339 Humbuckers")');
  await page.waitForTimeout(800);
  await page.screenshot({ path: path.join(outputDir, '4d_workspace_second_guitar.png') });

  // 5. Navigate to Preset Library tab
  console.log('5. Navigating to the Preset Library tab...');
  // Reset main view scroll to top first so sticky header tab buttons click cleanly
  await page.evaluate(() => {
    const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-generator');
    if (activeView) activeView.scrollTop = 0;
  });
  await page.click('button:has-text("Preset Library")');
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '5_preset_library_list.png') });

  // 5b. Select a dynamic preset from the library feed to load its structured editor!
  console.log('5b. Clicking the Adjust button of the first saved preset in the library feed...');
  const firstAdjustButton = page.locator('#library-list-container li button').first();
  await firstAdjustButton.click();
  await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '5b_preset_editor_active.png') });

  // 5c. Detect layout type and perform responsive user interaction
  const sliderCount = await page.locator('#library-editor-workspace input[type="range"]').count();
  if (sliderCount > 0) {
      console.log('5c. Tweaking the first active block input range slider programmatically...');
      const firstSlider = page.locator('#library-editor-workspace input[type="range"]').first();
      await firstSlider.evaluate(el => el.value = "8.2"); // Set value to 8.2 (tactile slider touch)
      await firstSlider.dispatchEvent('change'); // Trigger HTMX parameter update post!
      await page.waitForTimeout(800); // Let layout update settle
      await page.screenshot({ path: path.join(outputDir, '5c_preset_parameter_tweaked.png') });
  } else {
      console.log('5c. No block range sliders found (Legacy Table view active). Tweaking the chat instruction input instead...');
      const chatInput = page.locator('#library-editor-workspace textarea#chat-input').first();
      await chatInput.fill('Add more spring reverb mix to Scene A rhythm channel');
      await page.waitForTimeout(500);
      await page.screenshot({ path: path.join(outputDir, '5c_preset_chat_input_filled.png') });
  }

  // 6. Navigate to Learned Rules tab
  console.log('6. Navigating to the Learned Rules tab...');
  // Reset scroll state first
  await page.evaluate(() => {
    const activeView = document.querySelector('.main-view[style*="display: block;"]') || document.querySelector('#view-library');
    if (activeView) activeView.scrollTop = 0;
  });
  await page.click('button:has-text("Learned Rules")');
  await page.waitForSelector('#rules-list-container', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '6_learned_rules.png') });

  console.log('All automated iPhone 16 Pro user journey screenshots captured successfully!');
  await browser.close();
})();
