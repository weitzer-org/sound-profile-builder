# Single-Skill vs. Multi-Agent: Analysis (2026-07-29)

Two questions were evaluated: (1) whether this project's 13-agent Gemini
pipeline (`internal/agents/`) would be more effective collapsed into a
single-prompt "skill" call, the same pattern this repo already uses for
`/quick-review` vs. the bundled multi-agent `/code-review`; and (2) the same
question applied to GSR (`github.com/weitzer-org/gsr`, local checkout
`/home/bweitzer/gsr`), the external PR-review bot this repo's
`gsr-review.yml` / `gsr-review-deep.yml` workflows call.

Conclusion up front: **the two systems point in opposite directions.**
sound-profile-builder's pipeline is a poor collapse candidate; GSR's swarm is
a good one. The reason is structural, not aesthetic — see below.

---

## Part 1 — sound-profile-builder's 13-agent pipeline

### The pipeline today
`RunPipeline` (`internal/agents/orchestrator.go`) runs 4 phases with real
fan-out/fan-in **and** sequential dependencies:

- Phase 1 (parallel): Tone Historian, Sonic Profiler, Community Scraper — all
  against the same raw user prompt.
- Phase 2 (sequential dependency): CorOS Librarian consumes Phase 1 output;
  Cloud Navigator consumes the Librarian's output.
- Phase 3 (parallel): Acoustician, Transducer Tech, FOH Optimizer.
- Phase 4 (parallel): Mix Engineer, Control Mapper, DSP Dispatcher.
- Finalization (sequential): Architect synthesizes the concatenation of every
  prior agent's raw text output into the final `structured_payload`; Preset
  Critic (advisory only) then re-reads the Architect's own output for
  prose-vs-data contradictions.

### Why collapsing this is not recommended
1. **Direct evidence that bigger single-shot synthesis is already worse, not
   better.** The Architect is already the closest thing to "one skill" in
   this pipeline. TODO.md's 2026-07-26 full matrix run found the production
   Pro-tier model had 3.8x more Critic-flagged self-contradictions and 6.1x
   more fabricated/unverified block names than a smaller model on the same
   synthesis task, and higher thinking budgets correlated with *more* flags
   for 2 of 3 models tested. Folding 12 more specialized tasks into that same
   call pushes further in the direction already measured to be worse.
2. **Token/timeout blowup is a documented failure mode here**, not a
   hypothetical: injecting full-schema vocabulary into one narrow agent
   (Acoustician) sent its input tokens up 6.6x (1.1k -> 7.4k) and blew a
   3-minute timeout on a golden-set prompt (`prompts.go` comments). A single
   mega-prompt needs the union of every agent's schema/vocabulary plus Google
   Search grounding plus strict JSON-schema output simultaneously — the exact
   combination already known to strain the API (search grounding + JSON mime
   type is already rejected by some fallback models, forcing a degraded
   retry path today).
3. **Real parallelism is lost.** Phases 1, 3, and 4 run 3 agents concurrently
   each; a single call is inherently serial inside the model, so call count
   drops but wall-clock latency may not improve.
4. **Eval isolation is lost.** TODO.md restricts `eval_subagent` to only the
   agents whose input doesn't depend on upstream output, specifically so
   model/prompt regressions can be attributed to one agent; `ABLATE_AGENT_N`
   env vars for ablation studies also depend on per-agent separation.
5. **All-or-nothing failure cost.** A single narrow agent failing today
   retries a cheap ~10-line-prompt call. Collapsed into one skill, any
   failure (truncation, malformed JSON, schema mismatch) means regenerating
   the entire preset at the largest token budget in the pipeline (Architect
   already gets the ceiling, 16k output tokens).

### Why it differs from the `/code-review` -> `/quick-review` precedent
Code review evaluates existing, bounded text (a diff) — lower stakes, easy to
verify. This pipeline synthesizes across genuinely disjoint domains
(historical gear research, community sentiment, acoustics/impedance math, DSP
lane routing, footswitch logic), each with different grounding/schema needs —
and the repo's own Critic-flag data shows synthesis quality degrading, not
improving, as scope per call increases.

### If anything is collapsed
The smaller, ungrounded, non-dependent agents (FOH Optimizer, Mix Engineer,
Control Mapper — 8-12 line prompts, no search tool) are plausible merge
candidates into one post-processing call. The search-grounded/dependency-chain
agents (Historian -> Librarian -> Navigator) and the Architect/Critic split
should stay separate given the hallucination-rate data above.

---

## Part 2 — GSR (`github.com/weitzer-org/gsr`)

### Correction to this repo's own documentation
`CLAUDE.md` describes GSR's `basic` mode (run automatically on every PR via
`gsr-review.yml`) as a single pass. It is not. `GeminiAgent.analyze()`
(`agent.ts`) runs a hidden two-pass pipeline by default (Discovery, then
Remediation) — at least 2 real Gemini API calls per PR, often 3+ with
retries/caching. GSR's own `metrics.calls` field undercounts this: it counts
"tasks," not actual API calls, so basic mode reports `calls: 1` while making
at least 2 real requests.

### The swarm mode
`gsr-review-deep.yml` (triggered by the `deep-review` label) runs GSR's full
swarm — advertised in GSR's README as 5 agents (Architecture, Logic,
Security, TechDebt, Testing) but actually **10**: the above plus CI/CD,
Dependencies, Secrets, PromptSecurity, and Performance
(`adk/prompts/system_prompts/*.md`, 14-24 lines each). Only 4 of the 10 are
path-gated to relevant files (`orchestrator.ts:114-119`); the other 6 run on
every file unconditionally — a documented gap where non-shipping files (e.g.
`design_prd/*.html`) get reviewed by 6 independent agents at full severity.

Each swarm agent runs the same two-pass Discovery+Remediation pipeline as
basic mode, so real API-call count is roughly `2 x (agents matched)` plus one
more call for the Deduplicator — which is itself an LLM call (Gemini,
`deduplicator.ts`), not deterministic merge logic, and is serialized behind a
single lock rather than parallelized.

Critically, **the 10 agents have no dependency chain between them** — pure
fan-out/fan-in against the same diff, merged only once at the Deduplicator.
This is flatter than sound-profile-builder's pipeline: no Librarian-style
sequential handoff exists to lose by collapsing.

### Why collapsing GSR is more promising
1. **No dependency structure to preserve.** Unlike sound-profile-builder, no
   agent's output feeds another agent's prompt — a single pass loses no
   sequential information.
2. **Internal precedent already validates this for the identical task.**
   `/quick-review` in this repo *is* a single inline pass replacing a
   multi-agent fan-out, for the same problem (reviewing a diff), and it's
   already the default gate here.
3. **Removes a real, currently-serialized bottleneck** — the Deduplicator LLM
   call behind a static lock, needed only because independent agents produce
   overlapping findings; a single coherent pass wouldn't generate duplicates
   to reconcile in the first place.
4. **No evidence the swarm's extra recall is worth its cost.** GSR's own
   `review-quality-design.md` documents a real audit (job_tracker, 19 PRs /
   237 findings, 67.5% overall fix rate) that is entirely basic-mode data —
   swarm mode has never run against a real PR. A shadow-mode comparison is
   proposed in that doc but not built.
5. **Basic mode's one documented failure is a wiring bug, not proof
   single-pass is weaker.** `review-quality-design.md` §5 traces a false
   "`applyConfig` is not defined" finding to `useDedup` being positionally
   miswired into `useTriage`, making basic mode review file-by-file with no
   cross-file context — an accident, not an inherent limitation of collapsing
   agents.

### The real risk: instruction dilution
10 distinct checklists (security, secrets, dependency CVEs, prompt-injection,
SOLID, performance, testing, tech debt, CI/CD) held at equal rigor in one
prompt risks silently deprioritizing the less salient categories (tech debt,
CI/CD, prompt-injection) in favor of obvious logic/security hits — a known
LLM failure mode neither repo has data ruling out. Basic mode's existing
hidden Discovery -> Remediation split also suggests GSR's own authors already
found that a single generation without a refinement pass underperforms; a
collapsed skill should keep that two-step shape internally rather than
cutting to one raw call. And the deterministic file-routing layer
(`shouldRun()`, `IGNORE_PATTERNS`, severity gate) must survive the collapse
regardless — it's cheap, correct, and orthogonal to the LLM-call count.

### Recommendation
1. Fix the `useDedup`/`useTriage` wiring bug in basic mode regardless of
   anything else — it is actively producing false findings today.
2. Consolidate the 10 lenses into one structured multi-section prompt (one
   call, keep the Discovery -> Remediation two-step shape) rather than
   assuming a flat merge of all 10 checklists works with equal rigor.
3. Keep the deterministic file-routing/severity-gate layer outside the LLM
   either way.
4. Build the shadow-mode comparison `review-quality-design.md` already
   proposes before fully retiring swarm mode — replacing "cheap mode with a
   known bug" with "cheaper mode never measured against the thing being
   deleted" trades one unverified assumption for another.
