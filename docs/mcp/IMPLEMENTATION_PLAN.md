# Implementation Plan: QC-2 Modeler MCP Server

**Status:** Draft · **Date:** 2026-11-03
**Companion docs:** [PRD.md](PRD.md) · [DESIGN.md](DESIGN.md)

## 0. How this plan saves quota

This plan follows `.claude/rules/claude-config/agent-routing.md` and the CLAUDE.md
cost policy, applied to this specific work:

1. **The main session runs on Sonnet at medium effort** (`/model sonnet`,
   `/effort medium`) for every coding phase. The design work (the expensive,
   judgment-heavy part) is already done in these docs. What's left is mostly
   following a spec.
2. **Mechanical, well-specified tasks go to Haiku sub-agents.** That covers test
   tables, golden files, Markdown rendering, docs, and config plumbing. Each
   task card below names the files, the spec section, and an acceptance check,
   so a cold Haiku agent can do it without re-deriving context.
3. **Opus is used once, on purpose:** `/security-review` on the auth PR
   (CLAUDE.md requires it for auth diffs; it's a single inline pass, ~1 call).
   No `opus-verifier`, `planner`, or `/code-review high`, unless something in
   Phase 0 overturns the design.
4. **Free reviewers do the PR gating:** GSR, gemini-code-assist and CodeRabbit
   on every PR. Add the `deep-review` label (GSR swarm, zero Claude quota) to
   PR 1 and PR 2, which are the pipeline-logic and auth changes, *instead of*
   `/code-review high`.
5. **`test-runner` (Haiku) runs the suites** and returns only failures, so long
   `go test`/Playwright logs never land in the Sonnet context.
6. **Three small PRs, not one big one.** Each fits a single review pass. A
   rejected design detail costs one PR's rework, not three.
7. **You do the parts that need a browser and an account** (Phase 0 checks,
   connector setup). Those are cheap for you and impossible for an agent.

How to dispatch a Haiku task card: in the Sonnet session, say *"run task card
2.3 with a Haiku subagent"*. That is an `Agent` call with `model: "haiku"` and
the card's text pasted in as the prompt. A bare `Agent` call inherits the main
session's model, so always pass `model` explicitly.

### Rough budget

These are order-of-magnitude estimates, not measurements.

| Phase | Main session (Sonnet) | Haiku sub-agents | Opus |
|---|---|---|---|
| 0 Spikes | ~2 turns | 1 | 0 |
| 1 PR 1: presetsvc + run log | ~10–14 turns | 3–4 | 0 |
| 2 PR 2: MCP skeleton + auth + read tools | ~8–10 turns | 3 | 1 (`/security-review`) |
| 3 PR 3: run tools + prompts/log + resources | ~8–10 turns | 3 | 0 |
| 4 Docs & config | ~1 turn | 1 | 0 |
| 5 Bot-review rounds (×3 PRs) | ~2–4 turns each | `pr-triage` past round 2 | 0 |
| 6 Deploy & connect | ~1 turn | 0 | 0 |

## Phase 0: Spikes (resolve PRD §8 before coding)

**0.1 (Done, 2026-11-03): claude.ai connector dialog.** Step 2 offers Sign in
now / Sign in when needed / **No sign-in**, plus **Request headers** (up to
four, stored securely, never shown again) and a Transport selector
(Streamable HTTP by default). The setup docs lead with the header form. Step 1
probes the URL first; DESIGN §4 says how the server answers that probe.

**0.2 (Done, no further work): pipeline duration.** Measured from
`eval_results/full_pipeline/full_pipeline_gemini36_benchmark_report.md`:
117–166 s, mean about 138 s, under prod routing (PRD §8.3). That sets
`wait_seconds` (200 / 120) and `RUN_ABANDON_AFTER` (10 min). Optional
follow-up: once PR 1 ships, read the real durations from `runs/*.json` and
retune.

**0.3 (Done, via docs): resources and prompts.** claude.ai supports them
(PRD §8.2). Confirm once in Phase 6 that they show up in the attach menu.

## Phase 1: PR 1, `presetsvc` extraction + run log (no MCP yet)

Branch: `claude/intelligent-archimedes-agkyf7` (or a fresh branch per PR).
Behaviour change for UI users: none visible, except that runs are now logged
and capped at 2 concurrent.

| # | Task | Who | Spec | Accept when |
|---|---|---|---|---|
| 1.1 | Create `internal/runs`: `RunRecord`, `RunEvent`, `AgentTrace`, `Store` (Get/Put through `storage.Client` at `runs/<id>.json`), `Recorder` (implements observer, mutex, debounced persist, abandonment check) | **Sonnet** | DESIGN §3.1–3.2 | Compiles; 1.4 tests pass with `-race` |
| 1.2 | `agents` hook: `RunObserver`, `WithRunObserver`/`WithRunID`/`RunIDFrom`; call sites in `RunAgentSplit` (start, attempt beside `recordAttemptUsage`, done) and `RunPipeline` (`onProgress` sites, critic); `execID` from ctx; move `agentNames` → exported `AgentRoleKeys` with the 3 missing entries; mock-mode canned replay | **Sonnet** | DESIGN §3.3 | Existing `internal/agents` tests unchanged and green; nil observer = zero behaviour change |
| 1.3 | Create `internal/presetsvc`: move the generate goroutine body and the API-key resolution out of `server.go`/`handlers_preset.go`; `StartGenerate`/`StartRefine`/`Save`/`Run`/`WaitRun`; semaphore; in-memory run cache with eviction; `Preset.RunIDs` field. Rewire `handleGeneratePreset`, `handleChatPreset` and `handleTaskStatus` to call it, **emitting byte-identical HTML** | **Sonnet** | DESIGN §2 | `go test ./internal/api/...` green with no test edits beyond constructor wiring |
| 1.3b | SIGTERM handler in `cmd/server/main.go` (`signal.NotifyContext` → `http.Server.Shutdown`, plus `presetsvc.AbandonActive("server stopped mid-run…")` with a flush), replacing the bare `ListenAndServe` | **Haiku** (small and well specified) | DESIGN §3.3 "Graceful stop" | A test sends a cancel to the shutdown context and the active mock run reads back `abandoned` |
| 1.4 | Unit tests for `internal/runs`: concurrent `OnAgentDone` from 3 goroutines, persist round-trip on the fake client, abandonment flip, event `Seq` monotonic | **Haiku** (card below) | DESIGN §3, §7 | `go test -race ./internal/runs/...` green |
| 1.5 | Unit tests for `presetsvc`: mock-mode generate reaches `complete` with `PresetID` set and `RunIDs` linked; third concurrent start returns `ErrBusy`; `WaitRun` wakes on state change before the timeout | **Haiku** | DESIGN §2 | green |
| 1.6 | Run the full unit suite + Playwright e2e in the container; report failures only | **`test-runner` (Haiku)** | CLAUDE.md "Tests" | Both green; output quoted in the PR |
| 1.7 | Open the PR, add the `deep-review` label, then reply to the GSR findings per CLAUDE.md | **Sonnet** | | Bots' security-high findings addressed |

**Task card 1.4 (Haiku), paste as-is:**
> Repo `weitzer-org/sound-profile-builder`, Go. Write `internal/runs/recorder_test.go`
> and `store_test.go` for the package in `internal/runs` (read every file in it first).
> Use the in-memory fake `storage.Client` already used in `internal/storage` or
> `internal/api` tests (grep `ListFiles(` in `*_test.go` to find it; reuse it, don't
> write a new one unless none exists). Cover: (1) 3 goroutines calling OnAgentStart/OnAgentDone
> concurrently → 3 AgentTraces, no race under `-race`; (2) events have strictly
> increasing Seq; (3) Put then Get returns an equal RunRecord; (4) a record with
> status=running and UpdatedAt older than the abandon threshold reads back as
> `abandoned`. Table-driven where natural. Run `go test -race ./internal/runs/...`
> and paste the final output. Don't modify non-test files; if a test reveals a bug,
> stop and report it instead of fixing it.

## Phase 2: PR 2, MCP skeleton + token auth + read-only tools

| # | Task | Who | Spec | Accept when |
|---|---|---|---|---|
| 2.1 | `go get github.com/modelcontextprotocol/go-sdk@v1.8.0`; `internal/mcpserver`: `Mount(mux, svc, cfg)`, stateless `NewStreamableHTTPHandler`, `/mcp` + `/mcp/{token}` routes, `RequireToken` middleware, fail-closed when `MCP_TOKEN` is unset or short, existing rate limiter wrapped around both routes | **Sonnet** | DESIGN §4, §5, §6 | `curl` with no token → 401; with token → MCP `initialize` OK |
| 2.2 | Tools: `list_presets`, `get_preset`, `save_preset` (with annotations and model-facing descriptions) | **Sonnet** | DESIGN §5.1 | Integration test (2.5) passes |
| 2.3 | `mcpserver/render.go`: `StructuredPreset` → Markdown, plus golden test against `testdata/e2e_mocks/architect_generate.json` | **Haiku** (card below) | DESIGN §5.4 | Golden test green; output reviewed by eye once in the PR |
| 2.4 | Auth tests: header/path × missing/wrong/short/correct, `MCP_TOKEN_NEXT` accepted, a captured `log` output that never contains the token, the 401 has no `resource_metadata`, and the `/.well-known/oauth-*` paths return 404 | **Haiku** | DESIGN §4, §7 | green |
| 2.5 | Integration test harness: Go SDK client over `httptest.NewServer` in mock mode; initialize → `tools/list` → `list_presets` → `get_preset` → `save_preset` | **Haiku** | DESIGN §7 | green; Phase 3 extends it |
| 2.6 | `/security-review` on the branch diff | **Opus, inline (~1 call)** | CLAUDE.md "Security review" | Findings fixed or answered in the PR |
| 2.7 | `test-runner` full suites → PR → `deep-review` label → bot rounds | **Haiku → Sonnet** | | as 1.6/1.7 |

**Task card 2.3 (Haiku):**
> In `internal/mcpserver/render.go` write `func RenderPresetMarkdown(p *storage.Preset, sp storage.StructuredPreset) string`.
> Output: `## <Name>` heading, then the builder statement as a paragraph, then for each guitar
> (sorted by name) a `### <guitar>` heading and a table `| # | Block | Model | Settings (A / B) |`
> where Settings lists each parameter as `Name value[unit]`, shown as `A→B` when `ValueB` is non-empty
> and differs, with `*` appended when `Basis == "estimate"`, joined by `; ` (plain text, no HTML
> such as `<br>`). Escape `|` as `\|` and replace newlines with spaces in every cell. After each table,
> a `**Why:**` bullet list of `Model: Rationale` for blocks with non-empty Rationale. End with a
> footnote line `* = estimated value` only if any estimate appeared. Deterministic: no map
> iteration without sorting. Write `render_test.go` with a golden file in
> `internal/mcpserver/testdata/preset.golden.md` generated from
> `testdata/e2e_mocks/architect_generate.json`'s `structured_payload` (support `-update` flag). Also
> a test that a Model containing `|` and `\n` is escaped. Run `go test ./internal/mcpserver/...` and
> paste output. Touch only these files.

## Phase 3: PR 3, run tools, prompts and log exposure, resources

| # | Task | Who | Spec | Accept when |
|---|---|---|---|---|
| 3.1 | `generate_preset`, `refine_preset`, `get_run_status`, with long-poll through `WaitRun`, `NotifyProgress` when a progress token is present, and `ErrBusy` → actionable tool error | **Sonnet** | DESIGN §5.1 | Mock loop completes in the integration test |
| 3.2 | `get_run_log`, `get_run_prompts` (index mode + paged mode), `get_agent_output`; a paging helper shared by all three; role-or-key agent lookup through `AgentRoleKeys` | **Sonnet** | DESIGN §5.1, PRD §5.4 | Integration test reads the canned mock prompts back exactly |
| 3.3 | Resource templates (4) + `design_tone` prompt, reusing the 3.2 functions | **Haiku** | DESIGN §5.2–5.3 | `resources/templates/list` and `resources/read` covered in the integration test |
| 3.4 | Extend the integration test to the full PRD J1–J5 loop in mock mode, plus paging edge cases (offset past end, `max_chars` = 1) | **Haiku** | DESIGN §7 | green |
| 3.5 | `test-runner` → PR → bot rounds (no `deep-review` needed: no new auth or pipeline logic) | **Haiku → Sonnet** | | as before |

## Phase 4: Docs & config (fold into PR 3)

**4.1 (Haiku):** update `.env.example`, `docker-compose.yml` (placeholder
`MCP_TOKEN`), the `fly.toml` header comment (`fly secrets set MCP_TOKEN=…`),
README, and a short CLAUDE.md "MCP server" section pointing at `docs/mcp/`.
Include connector setup steps for both forms. Lead with whichever form
Phase 0.1 found claude.ai supports, and give the Claude Code form
`claude mcp add --transport http qc2 https://<app>.fly.dev/mcp --header "Authorization: Bearer $MCP_TOKEN"`.

## Phase 5: Review discipline (each PR)

- Bots are the gate, per CLAUDE.md. Reply inline to GSR's
  security-high/critical/Architecture/Logic findings, and do at least one more
  round if GSR rebuts.
- Once a PR passes round two, run **`pr-triage` (Sonnet, low)** with the ledger
  at `/tmp/pr-<n>-triage.md`.
- Escalate to `/code-review high` only if a bot finds a real concurrency or auth
  bug in PR 1 or PR 2. Ask first; it costs ~17 calls.

## Phase 6: Deploy & connect (You + ~1 Sonnet turn)

1. `fly secrets set MCP_TOKEN=$(openssl rand -hex 32)`. Merging to `main`
   auto-deploys.
2. In claude.ai → Settings → Connectors → Add custom connector:
   - **Step 1:** name `QC-2 Modeler`, URL `https://<app>.fly.dev/mcp`. The
     probe will get a 401 with no OAuth metadata. Click **Continue anyway**.
   - **Step 2:** choose **No sign-in**. Under **Request headers**, set the
     name to `authorization` and the value to `Bearer <token>`. Type the
     `Bearer ` prefix yourself. Leave Transport on Streamable HTTP.
   - **If the dialog won't continue, or insists on OAuth:** remove the
     connector and add it again with URL `https://<app>.fly.dev/mcp/<token>`
     and no header. Then record that result in DESIGN §4.
   - Auth can't be edited later. To rotate, remove the connector and add it
     again (DESIGN §4).
3. Walk through J1–J5 from the PRD in a real chat. This is the only step that
   spends real Gemini money, about one full pipeline run plus one refine.
4. Record Phase 0.3: do resources and prompts appear in claude.ai? If they
   don't, nothing changes, because tools carry the use case.
5. Add follow-ups to TODO.md: retention for `logs/` and `runs/`; v2 tools
   (`update_parameter`, `remove_block`, memories); OAuth upgrade via
   `auth.RequireBearerToken`.

## Risk register

| Risk | Likelihood | Mitigation |
|---|---|---|
| The presetsvc refactor changes UI HTML subtly | Med | "Byte-identical HTML" acceptance on 1.3; Playwright suite; GSR deep-review |
| An observer hook races in concurrent phases | Med | Mutex in `Recorder`; `-race` tests (1.4) |
| Fly stops the machine mid-run | Med (per a summary of Fly's docs: the proxy only sees inbound requests) | A 200 s first wait covers a typical run inside one open request; "poll in the same turn" tool wording; SIGTERM handler marks runs `abandoned` immediately; a run can be *viewed* after a stop but not *finished*, which is acceptable |
| URL token leaks through a log | Low–Med | Fixed route labels in logs; log-capture test (2.4); rotation via `MCP_TOKEN_NEXT` |
| Tool results too large for claude.ai | Low (documented limit ~150k chars) | Paging with a 40 k default and 120 k max; index mode first |
| Claude model mis-sequences tools (e.g. double generate) | Med | Model-facing tool descriptions; `ErrBusy` guard |
