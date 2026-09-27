# Agent routing

<!-- Vendored from weitzer-org/claude-config by sync-project.sh. Edit upstream, not here. -->

This is the user's standing instruction for which subagents to use and when.
Claude quota is a real constraint, so the defaults are deliberate: the main
session does the work, cheap agents run freely, and expensive agents run only
on a named trigger.

**Main session:** Sonnet at medium effort is the default build model. Switch
with `/model` or `/effort` only when a task clearly needs it.

| Signal | Agent | Model |
|---|---|---|
| "Where is X / how does Y flow", or a sweep across many files | `scout` | Haiku |
| After a code change, before claiming it works | `test-runner` | Haiku |
| Before opening or updating a PR (routine change) | `reviewer` | Sonnet, medium |
| PR has review comments to work through; always past round two | `pr-triage` | Sonnet, low |
| Diff touches auth, secrets, credentials, storage access, or rendering/parsing of user or LLM content | `security-reviewer`, in addition to `reviewer` | Opus, high |
| Large, cross-cutting, or architecturally risky change, before writing code | `planner` | Opus, high |
| Any verifier trigger below | `opus-verifier` | Opus, high |
| User explicitly asks for Fable, or for a "fundamentally simpler way" review | `fable-reviewer` | Fable, **ask-only** |

**Verifier triggers.** Call `opus-verifier` when any of these is true:
- A plan, diagnosis, or cost estimate rests on a premise about an external
  system that you have not executed.
- You are comparing measurements across runs and the conclusion depends on
  the comparison being valid.
- You are about to dismiss a review finding as a false positive, or to post
  a rebuttal to a bot reviewer.
- You are about to make an irreversible or expensive change: a schema or
  data migration, anything touching auth or secrets, or anything that spends
  real API money.
- The user asks for "an opus subagent" in any phrasing.

**Rules for spawning:**
- Every agent above pins its own model. Call it by `subagent_type` name. A
  bare `Agent` call silently inherits the main session's model; if you must
  use one, pass `model` explicitly.
- Subagents start cold. Hand them the full context they need: the goal, the
  files, the claim, the base branch.
- Report where a subagent disagreed with you, not just its conclusion.
- Never spawn `fable-reviewer` on your own initiative. Fable costs about
  2.5× Opus per token.
- Don't spawn an agent for something one `grep` or file read answers.
- The project's `CLAUDE.md` wins where it is more specific. For example, if
  it defines its own review skills or cost policy, follow those.
