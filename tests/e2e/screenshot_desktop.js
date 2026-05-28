const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

(async () => {
  const outputDir = path.join(__dirname, 'output', 'desktop');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  console.log('Launching headless desktop browser...');
  const browser = await chromium.launch();
  
  // Standard Desktop viewport configuration
  const context = await browser.newContext({
    viewport: { width: 1280, height: 960 },
    deviceScaleFactor: 1
  });

  const page = await context.newPage();

  // 1. Capture Desktop Login Screen (Full screen)
  console.log('1. Navigating to desktop login...');
  await page.goto('http://localhost:8083/login');
  await page.waitForSelector('input[name="password"]');
  await page.screenshot({ path: path.join(outputDir, '1_login_screen_desktop.png'), fullPage: true });

  // Authenticate
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // 2. Capture Desktop Empty Dashboard
  console.log('2. Waiting for desktop index page redirect...');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '2_blank_generator_desktop.png'), fullPage: true });

  // 3. Trigger dynamic Agent generation with BB King rig target
  console.log('3. Submitting BB King prompt (BB King The Thrill Is Gone)...');
  await page.fill('input[name="prompt"]', 'BB King The Thrill Is Gone');
  await page.click('button#gen-submit-btn');

  // Wait for progress panel to show up
  await page.waitForSelector('#gen-progress-area .progress-panel', { timeout: 5000 });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(outputDir, '3_agent_progress_desktop.png'), fullPage: true });

  // 4. Wait for real evaluation workspace to build
  console.log('4. Waiting for BB King evaluation workspace to build...');
  await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: path.join(outputDir, '4_workspace_draft_matrix_desktop.png'), fullPage: true });

  // 4d. Switch the active tab to secondary guitar (Gibson ES-339 Humbuckers) on desktop
  console.log('4d. Switching pickup tabs to Gibson ES-339 Humbuckers...');
  await page.click('button:has-text("Gibson ES-339 Humbuckers")');
  await page.waitForTimeout(800);
  await page.screenshot({ path: path.join(outputDir, '4d_workspace_second_guitar_desktop.png'), fullPage: true });

  // 5. Navigate to Preset Library tab (Split horizontal columns list dashboard!)
  console.log('5. Navigating to the Preset Library tab...');
  await page.click('button:has-text("Preset Library")');
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '5_preset_library_list_desktop.png'), fullPage: true });

  // 5b. Open the first saved preset in the library feed
  console.log('5b. Clicking the Adjust button of the first saved preset in the library feed...');
  const firstAdjustButton = page.locator('#library-list-container li button').first();
  await firstAdjustButton.click();
  await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: path.join(outputDir, '5b_preset_editor_active_desktop.png'), fullPage: true });

  // 5c. Detect layout type and perform responsive desktop user interaction
  const sliderCount = await page.locator('#library-editor-workspace input[type="range"]').count();
  if (sliderCount > 0) {
      console.log('5c. Tweaking the first active block input range slider programmatically...');
      const firstSlider = page.locator('#library-editor-workspace input[type="range"]').first();
      await firstSlider.evaluate(el => el.value = "8.2");
      await firstSlider.dispatchEvent('change');
      await page.waitForTimeout(800);
      await page.screenshot({ path: path.join(outputDir, '5c_preset_parameter_tweaked_desktop.png'), fullPage: true });
  } else {
      console.log('5c. No block range sliders found. Taking backup visual parameters editor panel capture instead...');
      await page.screenshot({ path: path.join(outputDir, '5c_preset_parameter_settled_desktop.png'), fullPage: true });
  }

  // 6. Navigate to Learned Rules tab
  console.log('6. Navigating to the Learned Rules tab...');
  await page.click('button:has-text("Learned Rules")');
  await page.waitForSelector('#rules-list-container', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '6_learned_rules_desktop.png'), fullPage: true });

  console.log('All automated Desktop user journey screenshots captured successfully!');
  await browser.close();
})();
