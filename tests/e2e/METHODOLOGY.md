# E2E Testing Methodology & Playwright Best Practices

This document outlines the testing methodology used for validating complex multi-agent features in the Sound Profile Builder (like the Agent Progress Indicator). It serves as a guide for writing future tests and capturing visual recordings for review.

---

## 🏗️ Philosophy
Our E2E tests target holistic visual features and states. Because we orchestrate a highly asynchronous pipeline driven by large language models, our tests prioritize:
1.  **State Determinism**: Using mock query params vs. live APIs controlled clearly by state.
2.  **Visual Proof**: Extensive screenshots and full-viewport videos to analyze the active UI on task boundaries.
3.  **Human Visibility**: Intentional pacing so that humans can follow video replays comfortably.

---

## 🎥 Recording Guidelines - Enhancing Video Review
To make execution recordings useful for debugging layout shifts and complex animations, strictly follow these practices:

### 1. Intentional Dwell Time (Pacing)
Do not immediately chain actions or assertions after a high-stakes UI element transitions. **Stay on the page for a brief moment** after key steps complete so that viewers can assess the final state of rendered matrices.
*   **Best Practice**: Insert dedicated `wait` timers before complex page transitions or element targeting or right after screenshots are dumped.
*   *Example in Playwright:*
    ```javascript
    // Allow state to settle so reviewers can read the finished matrix on video
    await page.waitForTimeout(2000); 
    ```

### 2. Full Page Viewport Expansion
Continuous layouts driven by HTMX need larger viewports to capture all loaded components without strictly relying on fast-action scroll methods.
*   Force large frame height at the beginning of localized tests:
    ```javascript
    await page.setViewportSize({ width: 1280, height: 1200 });
    ```
*   Use manual evaluation calls to capture infinite-styled feeds below the visible fold lines:
    ```javascript
    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
    ```

---

## 🔀 Strategies: Mock Mode vs. Live Mode

### 🎭 Mock Mode (Fast Track)
Use mock modes for continuous regression tests, fast validation of UI element presence, and scoped selector testing without blowing API tokens.
*   **Workflow**: Append `?mock=true` to URL paths or intercept HTTP responses to insert controlled matrix objects.
*   **Timeframes**: Usually executes localized suites in `~10-20 seconds`.

### ⚡ Live Mode (System-wide Reality)
Crucial before releasing or changing hard code in core agent components to ensure model responses don't break dynamic HTML layouts (e.g. LLM hallucinates UI wrappers or bad table entries).
*   **Workflow**: Detach route interceptors and forcefully unset continuous environment variables like `MOCK_MODE`. Add high extension boundaries on base timeouts (minimum `300,000` ms).
*   **Timeframes**: Standard runs usually hit boundaries between `3 to 5 minutes`.

---

## 🛠️ Typical Workspace Selection Target Handlers
Due to complex multiple-guitar configurations scoping identical table patterns, standard CSS query parameters will fail strict rules in Playwright. Always chain `first()` filters:
```javascript
// Correctly avoids strict mode violation across multiple matrix components
await expect(page.locator('#gen-workspace-wrapper .grid-matrix').first()).toBeVisible();
```
