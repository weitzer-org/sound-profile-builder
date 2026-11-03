# PRD: QC-2 Modeler MCP Server

**Status:** Draft · **Date:** 2026-11-03 · **Owner:** bweitzer
**Companion docs:** [DESIGN.md](DESIGN.md) · [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md)

## 1. Problem

The QC-2 Multi-Agent Modeler is only usable through its HTMX dashboard. The main
way the owner wants to use it is from a Claude chat: describe a tone in
conversation, have Claude kick off the 13-agent pipeline, discuss the result,
and refine it. None of that is possible today. There is no machine-readable
API. Every endpoint returns HTML fragments, and auth is a browser session
cookie plus a CSRF double-submit token.

The pipeline is also a black box from outside. Its two most useful
debugging artifacts never reach the user:

- **The prompts.** These are the versioned system prompt each agent ran with
  (`internal/agents/prompts/*.md`, selected by `config.json`'s
  `agent_prompts`), plus the user-side context each agent received, including
  the `GLOBAL USER CONFIG FLAG` prefixes `RunPipeline` adds.
- **The processing log.** This is the phase timeline and each agent's output,
  model, latency, tokens and errors. Agent outputs are already written to
  `logs/<execID>/<agent>.json`, but `execID` is not linked to the resulting
  preset, and the inputs and timeline are not stored at all.

## 2. Goals

| # | Goal | Measured by |
|---|------|-------------|
| G1 | Generate a preset from a Claude chat with no browser step | A full generate → view → refine → save loop completes in claude.ai |
| G2 | Show the prompts used for any run | Claude can retrieve, for a given run, each agent's system prompt (name, version, text) and user-side input |
| G3 | Show the processing log for any run | Claude can retrieve a timestamped phase/agent event log, per-agent outputs, and token/latency totals, both while the run is in progress and after it finishes |
| G4 | Simple auth with no OAuth build | One static secret, set with `fly secrets`, gates every MCP call. No authorization server, no client registration, no user DB |
| G5 | Leave the existing UI alone | The HTMX dashboard behaves the same. Existing unit and e2e suites pass unchanged |

## 3. Non-goals (v1)

- OAuth 2.1, multi-user accounts, or per-user data isolation. This stays a
  single-owner app, as it is today with `UI_PASSWORD`.
- Editing individual parameters or removing blocks through MCP. Refine by
  natural language (`refine_preset`) covers this use case. Direct edits can come
  in v2.
- Managing memory rules (`/api/memories`). Deferred to v2.
- Exporting to the physical Quad Cortex. That isn't possible today either.
- Editing or A/B-ing prompt versions from chat. v1 can read prompts but not
  change them.
- A local stdio MCP server. The target is a claude.ai **remote** connector, so
  the server runs on Fly next to the app.

## 4. Users & primary journeys

Single user: the project owner, working from claude.ai on web, desktop or mobile.
Secondary: Claude Code sessions in this repo, for debugging pipeline quality.

**J1: Design a tone in chat.**
"Make me a Mayer *Gravity* clean-to-lead preset for my ES-339, no paid plugins."
Claude calls `generate_preset`, keeps the user posted on progress, then shows
the signal chain per guitar as a readable table with the Architect's builder
statement.

**J2: Refine.**
"Less gain on the lead scene, and swap the delay for a tape echo."
Claude calls `refine_preset`, then shows what changed.

**J3: Keep it.**
"Save it as *Gravity Live*." Claude calls `save_preset`. This matters because
the app silently deletes all but the 3 newest drafts (`cleanUpOldDrafts`).

**J4: Why did it pick that?**
"Why a Deluxe Reverb and not a Twin? Show me what the Librarian was told."
Claude calls `get_run_log` and `get_run_prompts` for the run, reads the
Librarian's input and output, and explains.

**J5: Debug a bad run.**
"That run failed. What happened?" Claude reads the log, finds the failing
agent, its error, and which fallback models it tried.

## 5. Functional requirements

### 5.1 Tools (must have)
| Tool | Purpose |
|------|---------|
| `generate_preset` | Start a full pipeline run from a prompt plus optional capture and plugin flags. Returns a `run_id`, and the finished preset if the run ends within the wait window |
| `get_run_status` | Long-poll a run: status, current phase, elapsed time, and `preset_id` once done |
| `get_run_log` | Ordered event log for a run (phases, agent start/finish/error, model fallbacks, critic findings) with per-agent usage. Supports `since_seq` for incremental reads |
| `get_run_prompts` | For a run and an optional agent: the prompt key and version, the system prompt text, and the user-side input. Paginated |
| `get_agent_output` | One agent's raw output for a run. Paginated |
| `list_presets` | Saved presets and drafts: id, name, updated time, builder statement |
| `get_preset` | One preset as chat-friendly Markdown (per-guitar signal chain, Scene A/B values, rationale) plus a structured JSON form. Includes its `run_id`s |
| `refine_preset` | Natural-language refinement through the Refinement Architect. Also a run, so it is logged too |
| `save_preset` | Name a draft so it isn't auto-deleted |

### 5.2 Resources (should have)
Read-only mirrors of the same data, for clients that support MCP resources:
`qc://runs/{run_id}/log`, `qc://runs/{run_id}/prompts/{agent}`,
`qc://presets/{preset_id}`, and `qc://prompt-templates/{agent}/{version}` (the
static versioned prompt library). **The tools are the main access path.**
Whether claude.ai shows resources is not verified (see Open Questions), so
nothing in G1–G3 may depend on resources.

### 5.3 MCP prompt (nice to have)
One MCP prompt, `design_tone`, with `song`, `artist`, `guitar` and `notes`
arguments. It gives Claude a reusable starting template for J1.

### 5.4 Prompt and log exposure details
- The **effective prompt** (after the flag prefixes) and the **raw user
  prompt** are both recorded and returned.
- System prompts are returned **as actually sent** for that run, including a
  `_flash` variant if the model routing chose one, not just the template on
  disk today. A run's prompts stay readable even after a template changes.
- The log is readable **while the run is in progress**, phase by phase.
- Runs started from the HTMX UI are logged the same way. The log is a
  property of the pipeline, not of MCP.

## 6. Non-functional requirements

| Area | Requirement |
|------|-------------|
| Auth | A single `MCP_TOKEN` secret of at least 32 random bytes. The server accepts it as `Authorization: Bearer` **or** as a secret URL path segment (see DESIGN §4 for why both). If the token is unset, the MCP routes are not mounted |
| Cost safety | MCP runs share the existing per-IP LLM rate limiter, plus a cap on concurrent pipeline runs (default 2). A leaked token must not be able to run up an unbounded Gemini bill |
| Latency | A tool call never blocks for more than about 50 s. Long runs use start-then-poll, because a full pipeline is several sequential Gemini phases and likely takes minutes. The exact duration is an unverified assumption, to be measured in Plan Phase 0 |
| Durability | Run state and logs persist to the bucket through `storage.Client`, so they survive Fly scale-to-zero and restarts. A run interrupted by a machine stop shows up as `abandoned`, not `running` forever |
| Response size | Large text (prompts, agent outputs; the Librarian's input alone carries the ~81 KB `coros_map.json` dictionary) is paginated with `max_chars` and `offset`, so no single tool result is huge |
| Mock mode | Every tool accepts `mock: true`, which returns canned data (as `?mock=true` does today), so the full loop can be tested with no Gemini key |
| Portability | All persistence goes through `storage.Client`. The GCS path keeps working, per the CLAUDE.md conventions |

## 7. Success criteria

1. J1–J5 work end to end in claude.ai against prod.
2. `go test ./...` passes, plus a new MCP integration test that drives the
   server with the Go SDK's client in mock mode.
3. The existing Playwright e2e suite passes unchanged.
4. A request without the token, with a wrong token, or with a token of the
   wrong length gets a 401, and the attempted token never appears in logs.

## 8. Open questions / unverified assumptions

These are labelled per `verification-discipline.md`. Plan Phase 0 resolves the
first three before any code is written.

1. **Does claude.ai's "Add custom connector" dialog allow a static
   `Authorization` header?** Unverified. A web search found one third-party
   mention of "request headers (beta)" and nothing official; the Anthropic
   help-center fetch was rate-limited. The design supports both a header token
   and a URL-path token, so either answer works. Phase 0 decides which one the
   setup docs recommend.
2. **Does claude.ai show MCP resources and prompts, or only tools?**
   Unverified. Tools are therefore the main path.
3. **How long does a real full-pipeline run take?** Unverified. It sets the
   default `wait_seconds` and the abandonment timeout.
4. **Does Fly stop a machine that is running a background goroutine but has no
   open HTTP connection?** Unverified, but it is plausible given
   `auto_stop_machines = "stop"`. The design mitigates it with long-polling and
   abandonment detection either way.
5. **What is claude.ai's maximum tool result size?** Unverified. The default
   `max_chars` of 20,000 is a conservative guess.
