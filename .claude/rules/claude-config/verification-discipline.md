# Verification discipline

<!-- Vendored from weitzer-org/claude-config by sync-project.sh. Edit upstream, not here. -->

Nearly every significant error in these projects' history has the same shape:
**a confident claim about a system outside the repo, used as a load-bearing
premise, never actually checked.** The reasoning was fine; the input was
invented. Real cases: an Apify "150-result-per-run minimum" that does not
exist; `parseFloat("0..7")` claimed to return `NaN` (it returns `0`) and
posted as a PR review comment; `gh api .../comments` treated as complete when
the default page size of 30 hid 12 real comments; `process.env = X` in tests
dismissed as a false positive across several review rounds when it was a real
bug; `location == "United States"` read as implying remote; two Cloudflare
accounts declared different because a real value was compared against a
**placeholder** in `.env`.

**The rule.** Before a claim about anything outside the repo becomes
load-bearing, do one of two things. That covers third-party pricing or
billing mechanics, an API's pagination or rate limits, language, runtime, or
library semantics, what a log line proves, what an env var actually resolves
to, and what another service can reach on the network:

1. **Run something that demonstrates it** and quote the output, or
2. **Label it explicitly as an unverified assumption** in the same sentence,
   so it is visible when the conclusion is judged.

Never silently pick a third option (assert it and move on). A one-line
command beats a paragraph of plausible inference, and costs a second:
`node -e 'console.log(parseFloat("0..7"))'` would have prevented a wrong
review comment.

**Standing corollaries.** Each of these has burned a real session:

- **Exhaust every page.** Any list from a paged API is incomplete until
  proven otherwise; never conclude "there are no new comments/findings/objects"
  from a single call. The mechanism differs per client: `gh api` needs
  `--paginate` (its default page size of 30 is what hid 12 real comments); the AWS CLI's `s3api` list operations paginate unless you
  pass `--no-paginate`; an SDK `ListObjectsV2` needs its own
  continuation-token loop.
- **Check a value from `.env`, `.env.example`, or docs before relying on it,
  and never paste the raw value anywhere.** A placeholder like
  `https://<account-id>.r2.cloudflarestorage.com` is not an endpoint, but a
  value that isn't a placeholder is usually a live secret. Report the derived
  fact ("the endpoint is real, not the template") or a masked form, never the
  value itself, in a message, commit, log, or PR comment.
- **Measurements must be apples-to-apples.** Confirm identical denominators
  and inclusion criteria before reporting a delta across runs.
- **Say what a test run actually establishes.** Name the suite that ran and
  what it does not cover.
- **Before dismissing a review finding as a false positive, reproduce the
  claim.** A wrong dismissal gets posted publicly and stands.

## Review-round triage ledger

When a PR goes past its second review round, keep a scratch ledger
(`/tmp/pr-<n>-triage.md`, not committed) with one row per finding ID:
`id | verdict (fixed / declined / duplicate) | the evidence`. Consult it
before re-triaging anything. The `pr-triage` agent maintains this format.

- A finding already declined **with posted rationale** gets a pointer back
  to that rationale, but only while the code that rationale rested on is
  unchanged. If a later commit touched that behavior, verify the finding
  again.
- A genuinely new finding gets verified against the current code before it
  is believed. Bot reviewers re-anchor line numbers on old comment IDs.
