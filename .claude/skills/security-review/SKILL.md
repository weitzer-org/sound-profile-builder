---
name: security-review
description: Optional, single-pass adversarial security review for this repo. Runs inline with NO sub-agents (~1 model call, not a fan-out). Run alongside /quick-review (not instead of it) for diffs touching auth, secrets, storage credentials, or how externally-influenced content (user input, Gemini/agent output) gets rendered, parsed, or escaped.
---

# Security review

`/quick-review`'s (and `/code-review`'s) standard lenses check whether a
change does what it intends -- not whether an adversary can bend it. This
skill exists because that gap is not hypothetical here: a hand-rolled regex
"sanitizer" for agent-authored HTML passed an internal `/code-review high`
pass in this repo and still shipped with real XSS bypasses (HTML-entity-encoded
schemes, attribute selectors without leading whitespace, dangerous attributes
beyond `href`/`src`) that only surfaced once GitHub's automated reviewers
looked at the PR with that specific lens.

**Cost note:** this skill runs as a **single inline pass and spawns no
sub-agents** — deliberately, because Claude quota is a constraint on this
project and a multi-agent fan-out (the old version of this skill, and the
bundled `/code-review`) is the largest discretionary expense in the workflow.
The tradeoff is that one focused pass finds less than several parallel angles;
that is acceptable here *because* the GitHub-integrated bots (gemini-code-assist,
CodeRabbit) provide a free automated second opinion on every PR and have caught
real security bugs internal review missed. This skill is the local adversarial
pass; the bots are the backstop. **Do not spawn Agent-tool sub-agents while
running it** — if the diff feels too big or dangerous for one pass, say so and
recommend the user budget for a full multi-agent review rather than fanning out
here.

This is **optional and additive** -- run it in addition to `/quick-review`,
not as a replacement. Reach for it when:

- The diff touches auth, session/cookie handling, or secrets
  (`GEMINI_API_KEY`, `MOCK_PASSWORD`, S3/GCS credentials).
- The diff touches the `storage.Client` interface or a storage backend
  (`internal/storage/*.go`).
- The diff touches how externally-influenced content gets rendered,
  parsed, escaped, or sanitized -- user input (Builder Statement, chat
  messages) or agent/LLM-authored output (`legacy_html`, `structured`
  payload fields, anything from `internal/agents/prompts/*.md`) that
  reaches the HTMX dashboard (`web/templates/index.html`,
  `internal/api/handlers_*.go`).
- The diff adds a new third-party dependency.
- A user explicitly asks for a security review, or a prior review's
  findings already touch escaping, sanitization, or auth checks.

## Phase 1 — Gather context

Same diff-gathering as `/quick-review`: `git diff main...HEAD` (or the
appropriate upstream/PR range; also `git diff HEAD` if the review runs before
the commit). Additionally, for any diff touching rendering code, read the
`internal/agents/prompts/*.md` file(s) whose output reaches it -- that's this
project's main source of externally-influenced content flowing into HTML
output, and the prompt text tells you what the model is instructed (and
therefore, via prompt injection, can potentially be coaxed) to emit.

## Phase 2 — Adversarial pass (single pass, no agents)

Work through each angle below yourself, in one pass. Every finding needs a
concrete `file`, `line`, `summary`, and a `failure_scenario` naming the actual
attacker input/state that triggers it -- no speculative "this could be risky"
without a constructible trigger.

### Injection
For every point where external or agent-generated content is embedded into
HTML, a shell command, a file/object path, a URL, or a query, assume the
content is adversarial and ask what breaks:
- **HTML/XSS**: is escaping done via a real parser-based allowlist (this
  project standardizes on `github.com/microcosm-cc/bluemonday` — see
  `internal/api/handlers_preset.go`'s `sanitizeAgentHTML`), or via hand-rolled
  regex/string manipulation? Hand-rolled is a finding on sight, not just when a
  bypass is demonstrated -- regex cannot safely parse HTML.
- **Path traversal**: any place a preset ID, guitar name, or other external
  string is concatenated into an S3/GCS object key or filesystem path without
  validation.
- **Command/argument injection**: anything that shells out or builds a command
  line from external input (none currently in this repo's HTTP path, but check
  `cmd/*` tools too).
- **SSRF**: any outbound URL built from user-controlled input.

### Auth & authorization
For every new or changed HTTP handler in `internal/api`:
- Does it run behind the same auth middleware as its siblings (check
  `server.go`'s route registration), or does it accidentally bypass it?
- Does it trust a client-supplied ID (preset ID, guitar name) without checking
  the authenticated session actually owns/should see that resource? (Note: this
  app currently has a single shared dashboard password, not per-user resources
  -- but flag anything that assumes multi-tenancy protections that don't
  actually exist.)
- Does a new query param or form field create a second path to something an
  existing check already gates (e.g. a `?static=true`-style toggle that skips a
  check the default path enforces)?

### Secrets & credential handling
- Does the diff ever log, echo into an HTTP response, or embed in an error
  message an API key, session token, or credential value? (This project's own
  working convention: check presence via a masked grep/sed, never print the
  live value -- apply the same standard to application code, not just shell
  commands.)
- Do new env vars / config fields follow the existing
  `MOCK_PASSWORD`/`GEMINI_API_KEY` pattern -- never committed, `.env`
  git-ignored locally, injected via `fly secrets` in prod?

### Supply chain
For any dependency added in this diff:
- Is it the standard/recommended library for the problem (prefer boring,
  widely-adopted choices), or a one-off/less-maintained package?
- Does `go.sum`'s new transitive tree look proportionate to what was asked for,
  or does it pull in something unexpected?

## Phase 3 — Self-verify and report

There is no separate verifier agent, so verify each candidate yourself before
reporting. Re-read the actual code and settle each on:
- **CONFIRMED** — you can cite the line and name the attacker input that
  triggers it.
- **PLAUSIBLE** — mechanism is real, trigger realistic but not fully pinned.
  Keep it; prompt injection via a text field the model echoes into HTML is
  realistic in this codebase (it already happened once), so don't dismiss it as
  "speculative."
- **REFUTED** — drop it, but only when you can cite the exact line that already
  guards against it.

Report findings via the `ReportFindings` tool if available (ranked
most-severe first). If it isn't available, report as a plain ranked list with
the same `file` / `line` / `summary` / `failure_scenario` fields.

Finally, because this is one pass rather than a fan-out, state plainly what you
did **not** get to (e.g. "did not deep-audit the full bluemonday policy
config") so the user knows what the GitHub bots still need to cover.
