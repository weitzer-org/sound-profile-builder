# QC-2 Multi-Agent Modeler — Project Guide

Go service that orchestrates a 13-agent Google Gemini pipeline (12 generation
agents plus an advisory Preset Critic) to generate physics-accurate Quad
Cortex (QC) guitar presets, served through an HTMX dashboard.

## Run locally (Docker)
Local dev runs the app in a container with a **MinIO** (S3-compatible) store
standing in for Cloudflare R2 — no cloud accounts required.

```bash
cp .env.example .env        # first time
./run-mock.sh               # MOCK mode: canned pipeline, no Gemini key needed
./run-live.sh               # LIVE mode: requires a real GEMINI_API_KEY in .env
```
- App: http://localhost:8080 (log in with the `MOCK_PASSWORD` from `.env`)
- MinIO console: http://localhost:9001 (`minioadmin` / `minioadmin`)
- Stop: `docker compose down` (add `-v` to also wipe the MinIO volume)

## Architecture
- `cmd/server/main.go` — entrypoint; selects the storage backend, builds the server.
- `internal/api` — HTTP handlers, auth middleware (HMAC session cookie), HTMX server.
- `internal/agents` — orchestrator + 13-agent Gemini/Open-LLM pipeline
  (12 generation agents plus a 13th advisory Preset Critic that re-reads the
  Architect's output for prose-vs-data contradictions; `MOCK_MODE=true`
  returns canned data instead of calling the LLM).
- `internal/storage` — `Client` interface
  (`ReadFile`/`WriteFile`/`ListFiles`/`DeleteFile`/`Close`) with two backends:
  - `gcs.go` — Google Cloud Storage (original; **kept for a future GCP move**).
  - `s3.go` — S3-compatible (Cloudflare R2 in prod, MinIO locally) via AWS SDK Go v2.
  - Presets and memories are JSON blobs keyed by UUID under `presets/` and `memories/`.
- `internal/config` — loads `config.json` with env overrides.

## Capture metadata (`coros_map.json` / `user_captures.json`)
Both are embedded (`//go:embed`, `internal/agents/orchestrator.go`) — changes require a
rebuild/redeploy, not just a file edit.

**Both enrichment tools only ever research new/not-yet-covered entries, never the whole
file again** — important since new factory captures ship with firmware updates and new
user captures get downloaded regularly, and re-researching everything each time would
waste API budget on records that already have a good answer. For `coros_map.json`, an
entry with a non-empty `tonal_archetype` already IS the "done" signal (`cmd/enrich_captures`
only targets entries where it's empty) — no extra field needed. For `user_captures.json`,
every entry has *some* description by construction, so there's no natural empty-field
signal; `description_verified: true` on an entry is what marks it as already covered by a
citation-backed pass. Set it explicitly in `user_captures.json` when merging an approved
`verified_description` from `cmd/verify_user_captures`'s draft file — a merge that doesn't
set it will cause the next run to needlessly re-research that entry.

- **`coros_map.json`** — the factory capture map: real-world gear name -> QC block name
  (`coros_equivalent`), whether it's a pre-trained Neural Capture (`is_capture`), and an
  optional `tonal_archetype` (a real descriptive tonal color, consumed by
  `SelectedCaptureContext` in `internal/agents/fuzzy_matcher.go` to help the Architect pick
  the right relative-dB direction/magnitude on confirmed-capture blocks per Rule 9). An
  entry for a device that ships inside a paid Neural DSP Archetype plugin (Plini, Gojira,
  Cory Wong, John Mayer, ...) also carries `required_plugin` (the short name as it appears
  in `config.json`'s `available_plugins`) — `Orchestrator.RunPipeline` and
  `GetCategorizedAmplifiers` (`internal/agents/orchestrator.go`,
  `internal/agents/fuzzy_matcher.go`) use it to strip that entry out of the Librarian's
  Dictionary/amp menu entirely when the user doesn't own the plugin, rather than relying
  on the "AVAILABLE PLUGINS: ..." prompt line alone to discourage it.
  `tonal_archetype` coverage is sparse today (~51%) — this is a known, tracked gap (see
  TODO.md's Pipeline Quality Work section for the full history), not an oversight to fix
  ad hoc. **Whenever new entries are added to `coros_map.json`** (e.g. after a CorOS/NanOS
  firmware update adds new factory captures), re-run `cmd/enrich_captures` afterward to
  research `tonal_archetype` for any of the new entries that are missing one — don't
  hand-write it inline in the same edit. It's a two-step process by design: the tool writes
  proposed labels (each with a citation) to a sibling draft file, never `coros_map.json`
  directly, and a human reviews/merges them via a normal PR — see `cmd/enrich_captures`'s
  package doc comment and TODO.md for the full rationale (constrained label vocabulary,
  why it's offline/PR-gated rather than live).
  - Do NOT commit a new capture entry with a guessed or hand-invented `tonal_archetype` —
    either leave it unset until `cmd/enrich_captures` researches it, or add it yourself only
    with a real citeable source, same bar the tool holds itself to.
- **`user_captures.json`** — the personal/downloaded 3rd-party (Cortex Cloud) capture
  library. Structurally different from `coros_map.json`: every entry already carries a
  `description` field (coverage is 100%, confirmed by inspecting the file directly), so
  there's no *coverage* gap the way there is for `coros_map.json`. But coverage isn't the
  same as verified accuracy — the original 87 descriptions (commit `62ff925`) were written
  by an earlier Claude Code session inferring gear identity from the (often cryptic,
  Cortex-Cloud-exported) capture name alone, with no search grounding and no citation, the
  same unverified-guess risk `coros_map.json`'s gap represented. `cmd/verify_user_captures`
  (parallel tool to `cmd/enrich_captures`, same offline/draft-file/PR-gated process)
  independently re-derives a citation-backed description from the name alone for every
  entry (deliberately not shown the existing description, to avoid anchoring the new
  answer on a possibly-wrong old one) and writes both side by side in the draft for
  comparison — the new answer isn't assumed better, a human still judges each one.
  **When adding a new user capture, write a real, specific `description` in the same
  edit** (not a placeholder) and run it through `cmd/verify_user_captures` for a
  citation-backed second opinion before treating it as settled, same bar as
  `coros_map.json`.

- **`internal/catalog/qc_catalog.json`** — the Quad Cortex's own device catalog: every block
  model the unit offers, its "Based on ..." real-gear label, and its exact parameters (names,
  ranges, defaults, units, switch positions). Derived from the unit's `ModelRepo.xml` (which
  the device sends over USB) by `cmd/import_catalog`; never edit it by hand, re-run the
  importer after a CorOS update (`-diff` shows what changed). Keep the raw XML out of the
  repo. It's the ground truth for block *names*: `TestCorosMapNamesExistInCatalog` fails if
  a non-capture `coros_map.json` entry's `coros_equivalent` isn't a real model (or mono/stereo
  family) name. Neural Captures aren't in it. Bounds the device gives symbolically (e.g.
  `MAX_CABSIM_DB`) stay symbolic in the JSON (`min_symbol`/`max_symbol`, so the range checks below still
  skip them). `internal/catalog/firmware_constants.go` resolves them with per-entry evidence (only the cab
  bounds are confirmed on CorOS 4.1.0; the rest are pyquadcortex 4.0.1 data, MIT) and lists the unmeasured
  ones (`MIN/MAX_INPUT_TRIM`); `Param.ToWire`/`ToDisplay` (`convert.go`) use it and *refuse* (never clamp)
  out-of-range, NaN, non-float and unmeasured. The catalog also carries each param's `skew` (taper exponent;
  none = linear) and each model's `param_count` (full list, hidden included; the device's array is one longer,
  an unexplained trailing slot, and `Model.ValidParamIndex` rejects any index >= `param_count`). Only one
  clone family (the dual-mic cab) is checked against the device, not all 196.
  It also feeds the pipeline directly:
  - Every catalog model and family name is a valid block (`GetValidNativeBlocks`). Only amp/cab/drive
    names are fuzzy-snap targets; other categories and every plugin model are exact-match only.
  - Catalog amps/cabs/drives that no `coros_map.json` entry names are added to the Librarian's
    Dictionary and amp menu at runtime (`internal/agents/catalog_menus.go`), keyed by family name
    with the device's `based_on` text and no `tonal_archetype`. Uncovered guitar amps sit in the
    amp menu's "Unclassified (no tonal label yet)" bucket. Don't bulk-copy them into
    `coros_map.json`; add a real entry (and run `cmd/enrich_captures`) when one is worth it, and the
    runtime copy drops out by itself.
  - Plugin gating uses the catalog's `plugin_id` through `internal/catalog/plugins.go`'s table
    (short names as in `config.json`'s `available_plugins`). A new `plugin_id` after a CorOS update
    fails `TestPluginNamesCoverCatalog`; until it's added, that plugin's devices are never offered.
  - **Architect parameters** (`internal/agents/catalog_params.go`). For a catalog-aware Architect prompt
    (`12_architect_v6`, see below), the Architect's context gets the chosen blocks' real parameters (names,
    ranges, defaults, switch positions; from the Librarian's picks in generate, the preset's blocks in refine)
    and the real effect names for Rule 3 swaps; v6 is production (config.json). v5 and older get none of it, so
    they stay a like-for-like baseline. `qc_block_schema.json` is still injected for blocks the catalog doesn't cover.
  - **Parameter check.** `FlagCatalogParamMismatches` runs on every generate/refine (last, after
    `FlagUnverifiedStructuredBlocks`) and in `RunMechanicalQualityChecks`. It notes in Rationale, never
    Value: unknown parameter names, values out of a known range, bad switch positions, contradicting
    units, and `confirmed_range` on an unknown range. `Bypass` is always allowed; on cabs `Mic 1`/`Mic 2`
    (the mic model) are allowed and `Blend` is counted separately (the device sets each mic's LEVEL).
    Names compare ignoring case, spaces and inter-letter hyphens, plus a few aliases (Time, Low/High Cut).
    It also counts (and notes) `UnrecognizedBlocks`: a non-capture block named neither a device block
    nor a library entry and not already flagged unverified, i.e. an invented effect name such as
    "Plate Reverb" (the unverified check only covers gear categories). The input block's gate, the
    output block and the global EQ are real but aren't grid blocks, so they're exempt.
  - **Deliberate defaults.** A catalog-aware Architect lists only the controls it changes in
    `parameters` and names the ones it deliberately leaves at the device default in the block's
    `kept_defaults` (`storage.EffectBlock.KeptDefaults`; required in its response schema only).
    In a preset that uses the list, `UnconsideredParams` counts and notes any control every model
    behind the block name has but the block neither sets nor keeps. Older output has no list and
    isn't checked for this, so it scores 0 by construction.
  - **Which prompts get the catalog features** is decided by the prompt's own text (it contains
    `# Device Block Parameters`, `promptUsesCatalogParams`), not its version number, so a prompt
    without that section gets exactly the context and schema it was written for.
  - **Don't guess display scales.** The check skips unitless 0..1 floats and symbolic bounds: how the device
    displays those is unverified. 0..10 dials and dB/Hz/ms/s/% ranges are checked as the catalog gives them, which
    matches how presets already write them (not verified on a device).
  - **Pans** (catalog-aware prompts only). The Architect is shown each pan's catalog range, default and which end is
    L and which R (`MIC 1 PAN 0=L..10=R d5`; `BALANCE -1=L..1=R d0`), taken only from the catalog and only when every
    model behind the block name agrees (else the old `L..R`). What the unit's screen shows is **unverified**. On such
    output (the preset uses `kept_defaults`) the check notes a plain number outside that range; older output isn't
    checked, and a pan is still never `confirmed_range`. The export and read-back take a plain number on the catalog
    range, labelled `scale:catalog-range` († in the dry run); a unit, an out-of-range number or a word other than Center
    is refused. 0 is hard left on a 0..10 pan: center is the default.
  - **Cab families.** A family name (no `(M)`/`(ST)`) merges the mono and stereo cab, and the export places the mono one.
    Controls only one side has (PAN / BALANCE) are marked `[mono cab only]` / `[stereo cab only]` in the Architect's context.

## Quad Cortex device bridge (in progress)
Pushing presets to, and reading them from, the owner's physical Quad Cortex over USB, driven from the phone through
this app's MCP connector. Read these before any device work:
- `docs/research/qc-usb-tools-audit.md` — security and device-safety audit of the unofficial QC USB tools
  (qc-mcp, pyquadcortex, web editor, ampersand, cortex), with verdicts per pinned commit.
- `docs/design/qc-device-bridge-plan.md` — the design and phased plan (Phase 0 hardware gates, P1–P6), the MCP
  tool inventory, which model does what, and the kickoff prompt for each implementation chat (section 11).
- `tools/qc-mcp-trial/` — the allowlist proxy and Windows walkthrough used to trial qc-mcp on the owner's unit.
- `internal/deviceexport` — the device export (plan P2, section 4): one guitar's chain to `qc2-device-export/1` (model ids,
  param indexes, all eight scene values and bypass flags, row 0, capture blocks by name). Pure and deterministic; it
  refuses what has no data at all, and labels every conversion with its source in the param's `evidence` (`:g4` = read on
  the unit; device-XML skews, pyquadcortex bounds and qc-mcp's switch encoding are allowed but labelled, plan 4.1).
  Clone indexes stay limited to `VerifiedCloneBases`. `go run ./cmd/device_census <eval dir>` shows how many eval chains
  are pushable and why not. `send_preset_to_device`,
  `get_preset(format=device)` and `/api/preset/device_export` share one exporter (`api.Server.DeviceExporter`). Goldens:
  `go test ./internal/deviceexport -run Golden -update` (rerun after any catalog re-import: the header carries its hash).
- `internal/stomplayout` — stomp layout (S1 of `docs/design/qc-stomp-layout.md`): which footswitch each block goes on. Pure
  (blocks + current assignments -> plan of `assign`/`remove`/`skipped`/`warnings`); layout in the embedded `stomp_layout.json`.
  It must not import `internal/agents`: the capture library comes in as a `CaptureLookup` (`agents.StompCaptureLookup()`).
  Everything about the device in it is UNVERIFIED until gate G7. Goldens: `go test ./internal/stomplayout -run Golden -update`.
- `internal/deviceexport/readback.go` — the inverse of the export (plan P5): the `qc-device-readback/1` payload (contract
  section 9.1, `devicejobs.ParseReadbackPayload`) becomes a `StructuredPreset` draft. It shares the export's tables and
  refuses rather than guesses (unconverted items carry a reason). `read_device_preset(import=true)` is the only thing
  that stores it (`presetsvc.ImportDraft`, id `device-<job_id>`, `Preset.Origin`). Device text goes through
  `devicejobs.SanitizeText` first. Shared fixtures: `testdata/devicejobs/readback*.json`, read by Go and by
  `deploy/qc-agent/tests/test_readback.py` (`go test ./internal/deviceexport -run RoundTripFixtures -update` rewrites the round-trip ones).
- `deploy/qc-agent/` — the home agent (P4, Python). Its README has the Windows install, daily use and hardware
  acceptance steps. qc-mcp's transport is vendored under `vendor/qc_mcp/` at `f634ecd`, byte for byte and SHA-256
  pinned (test plus a runtime check): never edit those files; re-vendor in its own PR (`vendor/PROVENANCE.md`). The
  agent validates pushes against `internal/catalog/qc_catalog.json` (hash-matched with the export), so a catalog
  change means the PC's clone must be pulled before pushes work again. Captures are placed by exact name from the
  unit's own listing, V1/V2 decided on the unit (`qc_agent/captures.py`, plan 4.2); the only string write the wire
  guard allows is each capture's resolved `file_name`, once. Run its tests with
  `cd deploy/qc-agent && python -m unittest discover -s tests` (needs `protobuf` from `requirements.lock.txt`).
- `internal/devicelib` — what QC-2 knows about the unit's contents (plan P4b): the capture list a `sync_device_library` job
  returned (`device/library.json` in the main bucket, through `storage.Client`), a reviewed alias table
  (`capture_aliases.json`, empty until the first real sync), and the per-cab mic strings seen in `read_device_preset`
  results (`device/cab_mics.json`). `send_preset_to_device`'s dry run resolves each capture against the list with the
  agent's own rules (exact name, then one reviewed alias, never fuzzy; not listed = unresolved, "not on your unit"); a seen
  mic is written exactly as the export's `mics`. All of it is device text: `devicejobs.ParseLibraryPayload` is strict,
  names must survive `SanitizeText` unchanged, and the agent still re-resolves captures on the unit at push time. With no
  synced list the export is byte-identical to before. The agent side is `qc_agent/captures.library_payload` (one
  directory read, shared with pushes) and `jobs.sync_device_library`; `testdata/devicejobs/library.json` pins both.
  The cab mic string write (`set_param_typed`, held to the planned string by the wire guard) is **unverified on the
  unit** until README step 14.
- `docs/design/device-job-contract.md` — the signed job/result/heartbeat contract between QC-2 and the home agent
  (`internal/devicejobs` is the Go reference). `testdata/devicejobs/vectors.json` is shared with the Python agent: changing
  a vector, tag, field or check step is a contract change for both sides.

Owner decisions that bind this work: no pyquadcortex and no containers. The home agent runs natively on the owner's
Windows PC, using qc-mcp's transport vendored at `f634ecd`. It starts from a desktop shortcut only, and is never
always-on. No LLM in the push path. The device never gets global-settings writes, deletes, or overwrites of slots
this system didn't create. Our catalog's parameter indexes for the 196 cloned models (`clones=`) are unverified
until Phase 0 gate G1.

## Storage backend selection
`STORAGE_BACKEND=s3` (default `gcs`). For `s3`, set `S3_ENDPOINT`, `S3_BUCKET`,
`S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_REGION` (`auto` for R2).
See `.env.example`.

## MCP server
- `internal/mcpserver` is mounted in the same process by `cmd/server/main.go` on `/mcp` and `/mcp/{token}`.
- Token auth only: `MCP_TOKEN` (≥32 chars). Unset or short = MCP disabled (routes reject everything). `MCP_TOKEN_NEXT` is accepted during rotation.
- It shares the UI's per-IP rate limiter (`server.RateLimit()`) and `presetsvc`'s concurrency cap (`MCP_MAX_CONCURRENT_RUNS`).
- Run records and logs go through `storage.Client`: `runs/<run_id>.json` and `logs/<run_id>/`.
- `mock=true` or `MOCK_MODE=true` replays `testdata/e2e_mocks/run_events.json`, so every tool can be tested without Gemini.
- Integration tests use the Go SDK client over `httptest` (`internal/mcpserver/*_test.go`).
- **UI parity is enforced.** `generate_preset` must accept every field of the UI's tone prompt form
  (`#gen-prompt-form` in `web/templates/index.html`); `internal/mcpserver/parity_test.go` fails when the form
  gains one that `generateInput` lacks, or when an omitted MCP parameter stops matching the checkbox's default.
  Add the MCP parameter, don't allowlist it.
- **MCP-only `generate_preset` options** (no UI checkbox): `guitars` (1-3 validated names; omitted = the
  Gibson ES-339 + Telecaster pair), `single_amp_mode`, `allow_cloud_captures` (request wins; nil = `config.json`).
  Unlike the `allow_*` flags, config is the default, not a ceiling. `guitars` is untrusted text that lands in
  LLM prompts: keep it behind `validGuitars`.
- **Device tools** (`internal/mcpserver/devicetools.go` over `internal/devicejobs`) are registered only when
  `DEVICE_JOBS_BUCKET` and `DEVICE_JOB_KEY` are set (a bad value logs and leaves them off; the jobs bucket must differ
  from the main one). `sync_device_library` is registered too when the server has a device library (`WithDeviceLibrary`; `cmd/server` always
  adds it with the device tools). Without them the server has exactly the 9 tools the existing tests pin. Everything read back
  from the bridge is untrusted: pass it through `sanitizeDeviceText` and `cell()`. The push path uses P2's
  `internal/deviceexport` (`cmd/server/main.go`); `get_preset(format=device)` works without the device tools.
- `qc://style/preset-display` (`internal/mcpserver/style/preset-display.md`, <=40 lines) is the chat presentation
  guide; the server `instructions` summarise it. If you change `RenderPresetMarkdown`'s columns, update the guide
  and `testdata/preset.golden.md` (`go test ./internal/mcpserver -run Golden -update`).
- Connector setup: the header form (`authorization: Bearer <token>`) comes first. The `/mcp/<token>` path form is the fallback. See README "MCP server" and `docs/mcp/` for details.

**Usage analytics.** Every real agent call funnels through `Orchestrator.
RunAgentSplit` (`internal/agents/orchestrator.go`), which is wrapped by
`recordAttemptUsage`/`recordUsage` (`internal/agents/usage_recorder.go`) to
persist a per-call token/latency/cost/success record to the same bucket
under `usage/<date>/`. See `usage_analytics_reference.md` for the schema and
query recipes (`cmd/usage_report`, or raw `jq`-over-S3/GCS) — read that
before answering any "what's our token spend/error rate" question instead of
re-deriving the storage layout. `usage_recorder.go`'s `geminiPriceSchedule`
(dated: introductory vs. standard rates, e.g. `gemini-4-argon` doubles on
2026-12-01) mirrors `job_tracker`'s `internal/scoring/pricing.go` — keep both
in sync when Gemini prices change. `internal/agents/usage_reporter.go`'s
`UsageReporter` also optionally pushes this same data to the GSR
code-review project's hosted usage dashboard (`GSR_USAGE_INGEST_URL`/
`GSR_USAGE_INGEST_KEY`, see `.env.example`) — opt-in, non-blocking, filtered
to `provider=="gemini"` only.

## Secrets & auth
Setting `MOCK_PASSWORD` uses a local secret fetcher (skips GCP Secret Manager)
and becomes the dashboard login password; `GEMINI_API_KEY` supplies the LLM key.
In prod, secrets are injected as env vars (Fly: `fly secrets`), never committed.
`.env` is git-ignored.

## Tests
- **Unit:** `go test ./...`
- **E2E (Playwright, HTMX UI):** `tests/e2e/` — drives a running app and uses
  `?mock=true`. Node isn't installed locally; run the suite via the
  `mcr.microsoft.com/playwright:v1.58.2` container against the app.
- Always run and show the **unit + e2e results** as proof before claiming the
  application works.

## Code review
Merges to `main` auto-deploy to prod (see Deployment) and there's no CI test
gate, so review before merging is the main safety net.

**Cost policy (Claude quota is a real constraint on this project).** The
bundled `/code-review` spawns 8 finder agents plus up to 8 verifiers — ~17
model calls per run, the single largest discretionary expense in the workflow.
So it is **not** the default. As of 2026-07-15, **`/quick-review` is no
longer run automatically either** — the free GitHub-integrated bots
(gemini-code-assist, CodeRabbit) and the **GSR GitHub Action**
(`.github/workflows/gsr-review.yml`, github.com/weitzer-org/gsr, `basic`
mode) review every PR at zero Claude quota — GSR runs on its own Gemini API
key, not Claude's — and are now the sole default gate for routine PRs.

- **Default — every PR:** open the PR and let GSR + gemini-code-assist +
  CodeRabbit review it. No automatic Claude review step. Read and address
  `security-high`/`security-critical` findings before merging regardless of
  what else ran.
- **Always subscribe to the PR and address its comments.** After opening a PR, call
  `subscribe_pr_activity` on it and keep it subscribed until it is merged or closed; work
  review comments, bot findings and CI as they arrive (owner standing instruction, 2026-11-05).
  Merging stays the owner's call: never merge unless asked.
- **Run `/quick-review` on demand, not automatically** — when you explicitly
  want a local pass before opening a PR, or a bot finding is worth a second
  look. Still the project's own **`/quick-review`** (`.claude/skills/quick-review/`):
  one inline pass, no sub-agents, ~1 call.
- **Escalate to the multi-agent `/code-review high`** only for large or
  architecturally risky changes (auth, storage backend, agent pipeline logic)
  where the fan-out's extra recall is worth ~17 calls. Always pass the effort
  level explicitly. `medium` and `high` both run the same 8 parallel finder
  agents; the level changes candidate volume and verify aggressiveness
  (precision-biased at `medium`, recall-biased at `high`), not agent count.
- For the same class of large/risky change, also consider **GSR's agent-swarm
  mode** (`.github/workflows/gsr-review-deep.yml`) as a zero-Claude-quota
  complement to `/code-review high` — apply the `deep-review` label to the PR
  to trigger it. It runs GSR's full swarm (Architecture, Logic, Security,
  TechDebt, Testing agents + a dedup pass) instead of the `basic` single-pass
  mode that runs automatically on every PR. Label-only, deliberately: GSR's
  entrypoint hard-requires a real `pull_request` event payload, and a
  `workflow_dispatch` manual-trigger path was tried and dropped after two
  different workarounds both failed on live testing for platform-level
  reasons (see the workflow file's comments).
- Reserve `/code-review ultra` (multi-agent cloud review) for substantial
  features before merge — it's billed separately, so don't run it routinely.
- `/code-review --fix` applies findings directly if you want them auto-fixed.
- If a diff feels too big or risky for a single `/quick-review` pass, say so
  and let the user decide whether to budget for the full fan-out — don't
  quietly spawn sub-agents to compensate.

### Responding to GSR's PR comments
`gsr-review.yml`/`gsr-review-deep.yml` run GSR's PR-comment feedback loop
(`feedback-loop: respond`, `feedback-post: true`) — GSR reads replies to its
own findings and can post a rebuttal back if it disagrees. That loop only has
something to classify if a reply exists in the first place, so when GSR
leaves findings on a PR you're working on, reply to at least the
`security-high`/`security-critical`/`Architecture`/`Logic` ones inline (fix
and say so, or push back with a concrete reason) rather than leaving them
unaddressed — don't just silently fix-and-merge. If GSR's feedback loop then
posts a rebuttal to your reply, engage with that too instead of treating it
as the last word or as noise: at least one more round (concede, or restate
why the original reply stands) before moving on. Unlike gemini-code-assist/
CodeRabbit comments (background automation this repo treats as a second
opinion, not something to chase proactively), GSR is this project's own
dogfooded tool — its findings and rebuttals are worth a real reply.

### Security review
The standard review lenses (correctness, cleanup, altitude, conventions) are
not a substitute for an explicit security pass — they check whether a change
does what it intends, not whether an adversary can bend it. A hand-rolled regex
"sanitizer" for agent-authored HTML passed an internal `/code-review high` pass
in this repo and still shipped with several XSS bypasses (HTML-entity-encoded
schemes, attribute selectors without leading whitespace, dangerous attributes
beyond href/src) that only surfaced once GitHub's automated reviewers
(gemini-code-assist, CodeRabbit) looked at the PR with that specific lens.

- Run **`/security-review`** (project skill, `.claude/skills/security-review/`)
  as an optional, additive pass — not a replacement for `/quick-review` —
  whenever a diff touches auth, secrets, storage-backend credentials, or how
  externally-influenced content (user input, Gemini/LLM output, anything
  from the agent pipeline) gets rendered, parsed, or escaped. It runs
  adversarial angles (injection, auth/authz, secrets handling, supply chain)
  the standard lenses don't cover. It was rewritten to a **single inline pass
  (no sub-agents, ~1 call)** to fit the cost policy above — same quota profile
  as `/quick-review`, not the old multi-agent fan-out.
- Don't hand-roll HTML sanitization, escaping, or URL-scheme filtering with
  regex — regex can't safely parse HTML. Use a real parser-based allowlist
  library (this repo uses `github.com/microcosm-cc/bluemonday`).
- Treat the GitHub-integrated bots' comments on open PRs (gemini-code-assist,
  CodeRabbit) as a real second opinion, not noise — they've caught real
  security bugs internal review missed. Read and address `security-high`/
  `security-critical` findings before merging regardless of what level of
  `/code-review` already ran.

## Agents, routing, and verification discipline

Shared across weitzer-org projects and vendored from
`weitzer-org/claude-config` by its `sync-project.sh`. Edit them there, not
here; a sync PR carries changes back into this repo.

- `.claude/rules/claude-config/verification-discipline.md`: the
  verify-or-label rule for any claim about a system outside this repo, and
  the review-round triage ledger. PRs here have run six bot-review rounds, so
  start the ledger once a PR passes round two.
- `.claude/rules/claude-config/agent-routing.md`: which subagent to use for
  which signal, and the `opus-verifier` triggers.
- `.claude/agents/`: the model-pinned agents those rules name. The
  `claude-config.lock` file lists which ones are managed.

Both rules files load automatically in every session, local and cloud.
Where this file is more specific, it wins. In particular, the **Code review**
cost policy above stands: bots are the default PR gate, so don't run the
`reviewer` agent automatically before every PR. Use it, or `/quick-review`,
when you would have asked for a local pass anyway. The same goes for
`security-reviewer` vs `/security-review`.

## Deployment
- **Fly.io** (chosen runtime): `fly.toml`; secrets via `fly secrets`; storage =
  Cloudflare R2 (S3 backend). Scales to zero.
- **GCP** (future / preserved): `cloudbuild.yaml` → Cloud Run + GCS.

## Conventions
- Keep the GCP/GCS path intact — the project may migrate back to GCP.
- Storage is accessed only through the `storage.Client` interface; add backends
  there, don't special-case callers.
