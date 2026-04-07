# Project Backlog & To-Do

- **Interactive Post-Generation Tweaking**: Build a follow-up conversational thread UI/architecture where the user can chat with the Orchestrator LLM to recursively tweak the generated results, give feedback, or ask questions about the generated DSP blocks.
- **Free Cloud Capture Mapping**: Investigate a mechanism to keep a local copy of Cloud Navigator content specifically tailored to enforce sourcing *only* free presets on the Cortex Cloud. (The initial `cloud_captures_free.json` map concept was paused pending further UX/architectural refinement).
- **Progress Indicator**: Add a progress indicator while the agent is running.
- **Multi-Guitar Run**: Run all guitars at the same time with a tabbed experience.
- **Bug Fix**: Fix bugs with save preset functionality.
- **UI Improvement**: Improve the UI around the agent log.
- **Token Optimization**: Optimize overall prompt/API token usage.
- **Token Monitoring**: Show input and output tokens per agent in the UI.
- **Gemini Flash Profile Generation**: Test whether using a Gemini Flash call to generate preset metadata (Profile Name, Inspired By / Goal, Core Tone Source, Key Characteristics, Good For) yields better results.
- **Default Preset Name**: Have the preset name defaulted to the tone prompt prior to saving.
- **Temporary Presets Section**: Include non-saved presets as a separate section in the saved presets for X time.
- **Preset Library**: Build out a preset library.
- **Track Adjustments**: Track user adjustments over time to dynamically inform future preset generation and tweaks.
- **Capture Toggle**: Add a UI toggle to allow or restrict the multi-agent pipeline from sourcing Factory Captures during preset generation.
- **Paid Plugin Toggle**: Add a UI configuration to selectively enable/disable routing through Paid Plugin architectures (e.g., Nameless, Cory Wong, Plini).
- **YouTube Tone Analysis Agent**: Research integrating an agent to search for YouTube videos representing the prompt and analyze the tone from the video audio/context to inform preset generation.
- **Change Default Presets**: Add/Update standard presets to include: Rhythm, Clean Boost, Overdrive, and Comp.
- **Performance Optimization**: Improve performance of the Preset Library loading. It currently feels slow. Options include:
    - *Option A (Easy)*: Implement an in-memory TTL cache (e.g., 30 seconds) on the listing handler to eliminate repeated GCS reads.
    - *Option B (Structured)*: Refactor storage to extract thin metadata index files for lists, instead of handling flat heavy-file scans.
    - *Option C (Test-Specific)*: Hardcode dry mock returns specifically labeled for E2E speed checks in testing routes.

