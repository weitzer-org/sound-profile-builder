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

  // 1. Capture Desktop Login Screen
  console.log('1. Navigating to desktop login...');
  await page.goto('http://localhost:8082/login');
  await page.waitForSelector('input[name="password"]');
  await page.screenshot({ path: path.join(outputDir, '1_login_screen_desktop.png') });

  // Authenticate
  await page.fill('input[name="password"]', 'bluesmusic');
  await page.click('button[type="submit"]');

  // 2. Capture Desktop Empty Dashboard (prompt form at top, workspace placeholder separate below!)
  console.log('2. Waiting for desktop index page redirect...');
  await page.waitForSelector('h1:has-text("Spin Up a Tone")', { timeout: 15000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '2_blank_generator_desktop.png') });

  // 3. Trigger dynamic Agent generation and capture progress indicator
  console.log('3. Submitting the real-data evaluation query hook (TEST_EVAL_SRV_CLEAN)...');
  await page.fill('input[name="prompt"]', 'TEST_EVAL_SRV_CLEAN');
  await page.click('button#gen-submit-btn');

  // Wait for progress panel to show up
  await page.waitForSelector('#gen-progress-area .progress-panel', { timeout: 5000 });
  await page.waitForTimeout(200);
  await page.screenshot({ path: path.join(outputDir, '3_agent_progress_desktop.png') });

  // 4. Wait for real evaluation workspace to build (Prompt form card STAYS visible at top!)
  console.log('4. Waiting for real evaluation workspace to build...');
  await page.waitForSelector('.tweaking-workspace', { timeout: 20000 });
  await page.waitForTimeout(1500);
  
  // Scroll down a bit to see both the active workspace and the prompt form card nicely aligned
  await page.evaluate(() => window.scrollTo(0, 100));
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(outputDir, '4_workspace_draft_matrix_desktop.png') });

  // 5. Navigate to Preset Library tab (Split 1fr 2.5fr columns dashboard!)
  console.log('5. Navigating to the Preset Library tab...');
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.click('button:has-text("Preset Library")');
  await page.waitForSelector('#library-list-container li', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '5_preset_library_list_desktop.png') });

  // 5b. Open the first saved preset in the library feed
  console.log('5b. Clicking the Adjust button of the first saved preset in the library feed...');
  const firstAdjustButton = page.locator('#library-list-container li button').first();
  await firstAdjustButton.click();
  await page.waitForSelector('#library-editor-workspace .card', { timeout: 15000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '5b_preset_editor_active_desktop.png') });

  // 6. Navigate to Learned Rules tab
  console.log('6. Navigating to the Learned Rules tab...');
  await page.click('button:has-text("Learned Rules")');
  await page.waitForSelector('#rules-list-container', { timeout: 10000 });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: path.join(outputDir, '6_learned_rules_desktop.png') });

  console.log('All automated Desktop user journey screenshots captured successfully!');
  await browser.close();
})();
