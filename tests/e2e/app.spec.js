import { test, expect } from '@playwright/test';

test('QC-2 HTMX Dashboard Loads and Handles Constraints', async ({ page }) => {
  // Navigate to the local Go server
  await page.goto('/?mock=true');

  // Verify HTMX page loading and root constraints
  await expect(page).toHaveTitle('QC-2 Multi-Agent Modeler');
  await expect(page.locator('h1')).toHaveText('QC-2 Modeler');

  // Test Tonal Prompt
  await page.fill('input[name="prompt"]', 'Generate a Hendrix style Fuzz Face matrix.');
  
  // Submit HTMX request
  await page.click('button#submit-btn');

  // Wait for parallel Phase 1 Go routine execution times (~5-10 seconds context limit)
  await page.waitForTimeout(5000); 

  // Since video: on is set, the Playwright engine will natively preserve the browser state to /test-results/
});
