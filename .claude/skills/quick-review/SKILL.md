---
name: quick-review
description: Low-cost, single-pass code review of the current branch diff. Runs entirely inline with NO sub-agents (~1 model call instead of the bundled /code-review's ~17), so it's the default pre-merge review for this repo. Use for routine diffs; escalate to /code-review high or /security-review only for genuinely high-risk changes.
---

# Quick review

The default pre-merge review for this repo. The bundled `/code-review` skill
fans out to **8 finder agents plus up to 8 verifiers** — roughly 17 model
calls per run — which is this project's single largest discretionary quota
expense. This skill does the same core job in **one inline pass, spawning no
agents at all**.

That tradeoff is deliberate and it is real: a single pass finds fewer issues
than 8 specialized angles. It is affordable *because* this repo already has a
free automated second opinion — the GitHub-integrated bots
(gemini-code-assist, CodeRabbit) review every PR at zero Claude quota, and
they have repeatedly caught real bugs (including all four findings on PR #71).
This skill is the local first pass; the bots are the backstop.

**Do not spawn sub-agents while running this skill.** If you find yourself
wanting to, that's the signal to stop and tell the user this diff warrants a
full `/code-review high` instead.

## Phase 1 — Gather the diff

Run `git diff @{upstream}...HEAD` (fall back to `git diff main...HEAD`). If
there are uncommitted changes or the range diff is empty, also run
`git diff HEAD` — review often runs before the commit. If a PR number, branch,
or path was passed as an argument, review that instead.

## Phase 2 — Single-pass review

Read every hunk. For each one, Read the enclosing function — bugs in unchanged
lines of a touched function are in scope. Work through these lenses in one
pass, in priority order. Correctness always outranks the rest.

**Correctness (the priority):**
- Inverted/wrong conditions, off-by-one, nil deref, falsy-zero checks,
  wrong-variable copy-paste, errors swallowed in a catch, unescaped regex
  metacharacters.
- **Removed behavior:** for every deleted or replaced line, name the invariant
  it enforced and find where the new code re-establishes it. If you can't, that's
  a finding.
- **Call sites:** for each changed function, Grep for its callers and check the
  change doesn't break them (new precondition, changed return shape, new error
  path, ordering dependency).

**Repo-specific traps (this codebase has been bitten by each of these):**
- **Never mutate `Value`/`ValueB`** in a `Flag*`/critic path — those are the
  editable UI source of truth. Findings annotate `Rationale` via
  `appendRationaleNote`. Silently "fixing" data is a data-corruption bug.
- **Prompt versioning:** prompt changes ship as a new `_vN.md` file, never
  edited in place (see the doc comment in `internal/agents/prompts.go`).
- **Bypass semantics are inverted:** `Bypass: On` = bypassed/inactive,
  `Bypass: Off` = active. Reason carefully before flagging anything about it.
- **No hand-rolled HTML/URL sanitization by regex** — use `bluemonday`.
- **Gemini API-key auth only** (`BackendGeminiAPI`); never introduce anything
  requiring a GCP project.
- **Every parameter needs a real, specific value** — no ranges, no hedging.
- Agent-facing changes must respect `MaxOutputTokens` caps in `schemas.go`.

**Cleanup (only if it's clearly worth the maintainer's time):** duplicated
logic that an existing helper already covers (name the helper), needless
complexity, wasted repeated I/O.

**Conventions:** clear violations of the repo-root `CLAUDE.md`. Only flag when
you can quote the exact rule and the exact offending line.

## Phase 3 — Self-verify inline

There is no separate verifier agent, so verify each candidate yourself before
reporting it. Re-read the relevant lines and keep only findings where you can
name the concrete inputs/state that trigger the wrong output. **Drop anything
you can't substantiate** — with no verify pass to catch you, a confident wrong
finding is worse than a missed one. If a mechanism is real but the trigger is
uncertain, keep it and say so explicitly.

## Output

Report findings most-severe first, each with file, line, a one-sentence
summary, and a concrete failure scenario. Cap at ~8. If nothing survives
verification, say so plainly rather than padding.

Then state, in one line, what this pass did **not** cover — so the user knows
what the GitHub bots still need to catch. For example: "Single-pass only; no
dedicated security/injection angle — `/security-review` (also a multi-agent
fan-out) or the PR bots remain the check for that."

## When to escalate instead

Tell the user to run the more expensive review when the diff touches:
- auth, session/HMAC handling, secrets, or storage-backend credentials
- how externally-influenced content (user input, Gemini/agent output) is
  rendered, parsed, or escaped → `/security-review`
- a large or architecturally risky change → `/code-review high`
