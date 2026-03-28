const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

  // Intercept all API requests and append mock=true so backend uses mock mode
  await page.route('**/api/preset/*', async route => {
    const request = route.request();
    if (request.method() === 'POST') {
      const postData = request.postData() || '';
      const newPostData = postData ? postData + '&mock=true' : 'mock=true';
      await route.continue({ postData: newPostData });
    } else {
      const url = new URL(request.url());
      url.searchParams.set('mock', 'true');
      await route.continue({ url: url.toString() });
    }
  });
  
  console.log('Navigating to http://localhost:8081/?mock=true...');
  await page.goto('http://localhost:8081/?mock=true');
  
  console.log('Generating Draft...');
  await page.fill('input[name="prompt"]', 'Fuzz Face Matrix');
  await page.click('button#submit-btn');
  
  // Wait for Draft to appear
  await page.waitForSelector('input[name="preset_name"]', { state: 'visible', timeout: 10000 });
  await page.fill('input[name="preset_name"]', 'My Custom Tone');
  
  console.log('Taking Draft screenshot...');
  // Take screenshot of Draft Rename
  await page.screenshot({ path: '/usr/local/google/home/benweitzer/.gemini/jetski/brain/0786b0be-67e4-490c-9d82-72f3a50fd820/screenshot_draft.png' });
  
  await page.click('button:has-text("Finalize Save")');
  
  // Wait for preset list
  console.log('Waiting for finalized save...');
  await page.waitForSelector('#preset-list-container li', { timeout: 10000 });
  
  // Open workspace rename
  console.log('Taking Workspace Rename screenshot...');
  // Wait for rename button to appear
  await page.waitForSelector('button:has-text("Rename")', { timeout: 10000 });
  await page.click('button:has-text("Rename")');
  await page.fill('form[hx-post="/api/preset/rename"] input[name="preset_name"]', 'Brighter Tone');
  
  // Take screenshot of Workspace Rename
  await page.screenshot({ path: '/usr/local/google/home/benweitzer/.gemini/jetski/brain/0786b0be-67e4-490c-9d82-72f3a50fd820/screenshot_workspace.png' });

  // Open sidebar Copy
  console.log('Taking Sidebar Copy screenshot...');
  const awesomeToneListItem = page.locator('#preset-list-container li').filter({ hasText: 'My Custom Tone' }).first();
  await awesomeToneListItem.locator('button:has-text("Copy")').click();
  
  // Fill in the new name
  const copyForm = awesomeToneListItem.locator('form[hx-post="/api/preset/copy"]');
  await copyForm.locator('input[name="new_name"]').fill('Hendrix Duplicate');
  
  await page.screenshot({ path: '/usr/local/google/home/benweitzer/.gemini/jetski/brain/0786b0be-67e4-490c-9d82-72f3a50fd820/screenshot_sidebar_filled.png' });

  await browser.close();
  console.log('Screenshots captured successfully.');
})();
