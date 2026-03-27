# QC-2 Modeler: UI & Development Standards

This document outlines the core architectural choices and UI paradigms used in the Sound Profile Builder. Any future UI or logic modifications must adhere to these standards to maintain a cohesive, "premium workstation" experience.

## 1. Architectural Stack
- **Frontend:** Pure HTML/CSS powered by `htmx` for dynamic DOM swapping. No complex frontend frameworks (React, Vue) are used.
- **Backend:** Go (Golang) serving standard `net/http` handlers and managing the Gemini API orchestration pipelines.
- **LLM Pipeline:** Utilizes a highly-concurrent 12-agent structure, communicating primarily with `gemini-3.1-pro-preview` using fallback logic to `gemini-2.5-pro` for rate limiting.

## 2. Visual & Aesthetic Standards
- **Global Theme:** Dark Mode only. The interface must resemble a high-end audio workstation or IDE (like Ableton Live or VS Code).
- **Core Colors:**
  - Background (Main): `var(--bg-main)` (e.g., `#0f172a` deep slate)
  - Cards / Containers: `var(--bg-card)`
  - Accent / Primary Action: `var(--accent)` (e.g., `#3b82f6` or `#2563eb`)
  - Warning / Destructive: `var(--destructive)` (e.g., `#ef4444`)
- **Typography:**
  - UI Elements: Modern sans-serif (Inter, Roboto, UI system fonts).
  - Matrix/Parameter Values: strict `monospace` (`'SFMono-Regular', Consolas, Menlo`) for vertical numeric alignment.

## 3. Component Guidelines
- **Input Controls:**
  - Avoid native `<select>` dropdowns for complex settings. Use modernized Checkbox "Pill" lists where multiple selections are needed.
  - Chat inputs should utilize auto-expanding `<textarea>` fragments rather than single-line `<input type="text">` to support multi-line prompt structures.
- **Buttons & Clicks:**
  - Primary actions should have subtle transition animations on hover (background tint, scale up) and active (scale down) states.
- **Layout & Responsiveness:**
  - Desktop-first layout via CSS Grid `grid-template-columns`.
  - Tablet/Mobile fallback (`@media max-width: 1024px`) must reliably collapse complex multi-column grids down to a single centralized 1-column scroll view.
- **Padding/Breathing Room:** Components containing heavy text or matrices must be isolated into distinct `<div class="card">` elements with substantial `.padding` (at least `1rem 1.5rem`).

## 4. HTMX Integration & Agent Outputs
- **Safety First:** Agent output containing HTML (e.g., Markdown formatting, lists) should NOT be aggressively escaped (`html.EscapeString`) if injected safely by the backend template. We use `strings.ReplaceAll("\n", "<br>")` for safe plaintext ingestion.
- **Asynchronous Loaders:** Features requiring LLM generation time must instantly replace their trigger button with an `htmx-indicator` (typically a CSS spinner) to prevent duplicate submissions.
- **Destructive Confirmations:** All destructive routes (Delete Preset, Discard Draft) must use the native `hx-confirm="..."` attribute to protect against accidental user actions.
- **Accessibility:** All dynamically rendered regions into which ADK output natively swaps must be wrapped within an `aria-live="polite"` or `aria-live="assertive"` property.
