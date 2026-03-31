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
- **Capture Toggle**: Add a UI toggle to allow or restrict the multi-agent pipeline from sourcing Factory Captures during preset generation.
- **Paid Plugin Toggle**: Add a UI configuration to selectively enable/disable routing through Paid Plugin architectures (e.g., Nameless, Cory Wong, Plini).
- **Cyclic Agent Deliberation**: Explore transitioning the Orchestrator from a linear DAG to a cyclic feedback graph, allowing the Librarian to pass a "Gear Not Found" exception back to the Tone Historian iteratively to select the best historically accurate fallback amp.
