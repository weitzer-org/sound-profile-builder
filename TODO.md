# Project Backlog & To-Do

## Features & Enhancements

- **Interactive Post-Generation Tweaking**: Build a follow-up conversational thread UI/architecture where the user can chat with the Orchestrator LLM to recursively tweak the generated results, give feedback, or ask questions about the generated DSP blocks.
- **Free Cloud Capture Mapping**: Investigate a mechanism to keep a local copy of Cloud Navigator content specifically tailored to enforce sourcing *only* free presets on the Cortex Cloud. (The initial `cloud_captures_free.json` map concept was paused pending further UX/architectural refinement).
- **Progress Indicator**: Add a progress indicator while the agent is running.
- **Multi-Guitar Run**: Run all guitars at the same time with a tabbed experience.
- **Bug Fix**: Fix bugs with save preset functionality.
- **UI Improvement**: Improve the UI around the agent log.
- **Token Optimization**: Optimize overall prompt/API token usage.
- **Token Monitoring**: `TokenUsage` (`internal/agents/orchestrator.go`) only tracks aggregate `InputTokens`/`OutputTokens` across the whole pipeline run, plus a call *count* per model (`ModelsUsed`) -- there's no per-model token breakdown and no timing at all. Per-call numbers do get logged (`AGENT_TOKEN_LOG: Agent=X In=Y Out=Z Model=W`) but aren't persisted or surfaced in the UI. Needs:
    - Per-model input/output token totals, not just an aggregate across every model combined.
    - Total wall-clock time for the run, and ideally per-agent/per-call timing (nothing is timed anywhere today).
    - Surface both (aggregate + per-model + timing) in the UI, not just the current combined "Input: X | Output: Y | Models: A (n), B (m)" line.
- **Gemini Flash Profile Generation**: Test whether using a Gemini Flash call to generate preset metadata (Profile Name, Inspired By / Goal, Core Tone Source, Key Characteristics, Good For) yields better results.
- **Default Preset Name**: Have the preset name defaulted to the tone prompt prior to saving.
- **Temporary Presets Section**: Include non-saved presets as a separate section in the saved presets for X time.
- **Preset Library**: Build out a preset library.
- **Track Adjustments**: Only the capture half of this exists. Every chat refinement already auto-saves a "Learned Rule" memory (`handlers_preset.go:1025`, via `memoryStore.Save`), and `/api/memories` displays them -- but nothing ever reads them back. Confirmed `memoryStore.List` is only ever called from the read-only display endpoint (`handleGetMemories`), never from the orchestrator or the generation path. So today Learned Rules are captured and shown to the user, but have zero influence on future generations. The actual remaining work is wiring memory retrieval into `RunPipeline`/`RefineChat` so past corrections actually inform new output.
- **Capture Toggle**: Add a UI toggle to allow or restrict the multi-agent pipeline from sourcing Factory Captures during preset generation.
- **Paid Plugin Toggle**: Add a UI configuration to selectively enable/disable routing through Paid Plugin architectures (e.g., Nameless, Cory Wong, Plini).
- **YouTube Tone Analysis Agent**: Research integrating an agent to search for YouTube videos representing the prompt and analyze the tone from the video audio/context to inform preset generation.
- **Change Default Presets**: Add/Update standard presets to include: Rhythm, Clean Boost, Overdrive, and Comp.
- **Performance Optimization**: Improve performance of the Preset Library loading. It currently feels slow. Options include:
    - *Option A (Easy)*: Implement an in-memory TTL cache (e.g., 30 seconds) on the listing handler to eliminate repeated GCS reads.
    - *Option B (Structured)*: Refactor storage to extract thin metadata index files for lists, instead of handling flat heavy-file scans.
    - *Option C (Test-Specific)*: Hardcode dry mock returns specifically labeled for E2E speed checks in testing routes.
- **CSRF Protection**: No CSRF protection exists on any state-changing route (save/delete/copy/rename preset, generate, chat-refine) -- confirmed zero `csrf` references anywhere in `internal/api`. The HMAC session cookie alone doesn't defend against cross-site request forgery.
- **Rate Limiting**: No rate limiting exists on `/api/preset/generate` or `/api/preset/chat`, both of which call the paid Gemini API. Nothing currently stops a compromised session or a client-side bug from hammering either endpoint and running up API cost.
- **Per-User Ownership Model**: `storage.Preset` has no `owner`/`user_id` field -- confirmed via grep. Every authenticated session shares one `MOCK_PASSWORD` and can view/edit/delete any preset. Decide whether this is acceptable long-term (single-team internal tool) or needs per-user scoping.
- **Live-Mode Playwright Test Convention**: `tests/e2e/live_capture.spec.js` and `live_recheck.spec.js` (real-Gemini-data verification, used repeatedly during the mobile UX redesign work) are uncommitted with no decision made on whether they're a permanent fixture. Either commit them under a clear naming/gating convention (e.g. `*.live.spec.js`, excluded from the normal mock-mode run since they cost real API calls) or delete them.

## Known Bugs & Investigations

- [ ] Add effect rationale to the fallback table renderer in `handlers_preset.go` when available in StructuredPreset.
- [ ] Fix `#workspace-wrapper` duplicate-ID bug (confirmed, not just suspected): `renderTweakingWorkspaceHTML` hardcodes `id="workspace-wrapper"` on its root div and is called for both the Generator draft workspace and the Library Adjust workspace, both of which can be present in the DOM at once (one merely `display:none`). The refinement chat form's `hx-target="#workspace-wrapper"` (`handlers_preset.go:792`) always resolves to the first match in DOM order -- the Generator's -- regardless of which workspace the user is actually refining. Refining a saved preset from the Library can silently write the response into the hidden Generator draft instead of the visible Library workspace.
- [ ] Dead code: `refinementSummaryHtml` (`handlers_preset.go` ~line 758) checks `lastMsg.Role == "architect"` to decide whether to show a prominent "Latest Refinement Result" callout, but `ChatHistory` entries are only ever written with role `"user"` or `"model"` (confirmed via grep across `internal/api`/`internal/agents`) -- that condition can never be true, so the callout never renders. Either wire a real `"architect"`-role message in, or remove the dead branch.
- [ ] `s.tasks` (in-memory generate/chat task map in `server.go`) never evicts entries -- confirmed zero `delete(s.tasks, ...)` calls anywhere. Every generation and chat-refinement task ID plus its full rendered HTML `Result` string stays in memory for the process's lifetime. Low urgency given Fly.io scale-to-zero, but unbounded growth on a longer-lived instance.
- [ ] Add a `gofmt -l` check to CI (`.github/workflows/ci.yml`) -- confirmed no gofmt check exists there and no local pre-commit hook either (`.git/hooks/pre-commit` doesn't exist). Pre-existing whitespace drift in `internal/api/handlers_preset.go` and others has been accumulating and has to be carefully avoided on every unrelated diff.
- [ ] Root-cause the vanished "Live Capture - Texas Flood (Desktop)" preset from the mobile UX redesign session -- it disappeared from the store after a container rebuild/redeploy and was never explained; worked around at the time by reusing the "(Mobile)" preset's data instead. Not confirmed as a one-off.
