# QC bridge job latency: where the time goes and what to change (issue #164)

Status: investigation, 2026-11-06. Nothing below was timed per part on the unit yet. This change adds
the log lines that will time it (see "Measure first"). Every claim is marked **seen** (from the P6
log in the issue), **read** (from code or docs, with a reference) or **unverified**.

## What was seen

- A `list_slots` job took about 17 s from `started` to `succeeded`. The capture census line, logged
  right after the first listing, came 16 s after `started` (**seen**, issue #164).
- Every later directory read took 16-17 s. A push took 62-72 s end to end (**seen**, issue comment).
- `send_preset_to_device` with `wait_seconds: 120` failed on the MCP client after 60 s, while the job
  ran on (**seen**).

## Where the 16 s probably goes

From the code only. The split is **unverified**: it is what the new log lines will measure.

| Part | Cost | Source |
|---|---|---|
| Connect: open, handshake, firmware read | about 1.0 s of fixed waits in the handshake (`_collect(0.15)` ×3, `sleep(0.3)`, `_collect(0.4)`), plus one Version round trip | **read**: `vendor/qc_mcp/transport.py:309-360` |
| The `File` READ stream itself | probably 11-12 s | **unverified**. Upstream measured 6-12 s for the full READ on a QC Mini (`docs/DIRECTORY.md`, qc-mcp `f634ecd`), and "~12 s" for its ~400-message stream (`transport.list_directory` docstring). The owner's unit lists about 590 folders and 2073 capture files. |
| The quiet tail | at least 3 s on every listing | **read**: `list_directory_raw(quiet_ms=3000)`, `qc_agent/qcmcp_backend.py` |

Is the stream limited by USB throughput or by the unit? **Unknown.** Each report carries at most 126
payload bytes (`protocol.py`: `REPORT_SIZE = 128`). The new log line reports the stream's KB and
seconds, so the throughput can be computed from one listing.

A push lists the directory at least three times: the slot check before writing, the check before
saving, and at least one post-save poll after a 6 s sleep (**read**: `jobs.py` `check_unit`, `save`,
`SAVE_LIST_INTERVAL_S`). Three listings at about 16 s each plus the 6 s sleep comes to about 54 s,
which fits the 62-72 s seen.

Job pickup adds 0-5 s while the bridge is in its fast window, and 0-15 s after 5 idle minutes
(**read**: `agent.py` `POLL_FAST_S`, `POLL_SLOW_S`, `FAST_WINDOW_S`).

## The big lever: the user-only directory READ

qc-mcp documents this at the commit we vendor (`docs/DIRECTORY.md`, "Reading only the user's
content", commit `bfc1da0`, an ancestor of `f634ecd`). Cortex Control's own directory READ on connect
is `File{READ, omit_factory_content: true}`. Upstream reports (**unverified on the owner's unit**):

- The unit streams about 28 folders instead of about 590: user setlists, Downloads, the plugins'
  User folders, user captures and IRs. No factory library.
- The stream **opens with a bare `File{user_content_estimate: n}`** (request_id 0): "the number of
  named items that will follow". So the end of the listing is known, and no quiet window is needed.
- Measured on a Mini with about 2100 user items: **1.7 s**, against 6-12 s for the full READ.

Both fields exist in our 4.1 descriptors (`FileMessage` fields 11 and 12; upstream's PROTOCOL.md
lists them under the 4.1 schema changes as "backup scoping").

Why this is also a safety improvement: today a listing counts as complete once it has been quiet for
3 s. That is a guess. With a count, a listing that ends short of `n` can be refused outright, which
is a stronger version of the current "an incomplete listing is refused" rule.

What each job needs:

| Listing | Needs factory content? | Could use the user-only READ? |
|---|---|---|
| `list_slots` | no (My Presets only) | yes |
| Push: slot check before writing | only if the plan places a **factory** capture | yes, unless it does |
| Push: check before saving | slot check: no. Capture re-resolution: user part fresh, factory part from the first listing (see below) | yes |
| Push: post-save polls | no (My Presets name at one position) | yes, and the 6 s interval can shrink |
| `sync_device_library` | yes (it lists every capture on the unit) | no: keep the full READ |

Re-resolving captures before the save exists to catch a file that changed mid-push. Factory folders
are read-only, and they change only with a firmware update. A firmware update needs a reboot, which
the bridge already treats as "unit gone" and exits on. So reusing the factory part of the first
listing for the pre-save check keeps that guarantee (**reasoning; the read-only claim about factory
folders is unverified**). A user capture added or removed mid-push still shows up in the fresh
user-only read.

A user capture that resolves needs only user folders (`captures.resolve_one` filters on
`is_factory`). Without factory content, one error message would change: "the unit only has it as a
factory capture" becomes "not on the unit". A push of user captures only could skip the full READ
entirely if that wording loss is acceptable.

### What has to be verified on the unit first

1. **What `n` counts.** "Named items" could mean files with a non-empty name (empty My Presets
   slots have none), all files, or folders. Compare `n` with the folders and files that actually
   arrive in a user-only listing. If it can't be pinned down, keep the quiet window as a fallback
   and use `n` only as a lower bound ("not complete before `n`").
2. **That My Presets still arrives as the full 256-slot table**, with the same keys and the
   position = slot rule (`directory_snapshot` refuses anything else, so a wrong shape fails closed).
3. **That user captures look the same**: folder flags, `ProductData.version`, and dependency hashes
   in My Presets.
4. **The time saved on this unit.** Upstream's 1.7 s is from a Mini.

### Code changes it needs (not made yet)

- **Wire guard**: allow `File` READ with exactly `{action, request_id, omit_factory_content: true}`
  in addition to the bare READ (`WireGuard._check`; `READABLE` admits only `{action, request_id}`
  today). This is a guard change, so `/security-review` applies.
- `list_directory_raw(user_only=True)`: send the flag, read the estimate (request_id 0, no folder),
  and stop at `n` items. Refuse when the deadline passes first. Keep the quiet-window path for the
  full READ.
- `DeviceBackend.read_directory(user_only=...)`, and the fake device and fake HID to match.
- Push: one full READ in `check_unit` when the plan has a factory capture; user-only reads
  elsewhere; pre-save re-resolution against fresh user content plus the first read's factory entries.

Rough effect if upstream's numbers hold here (**unverified**): `list_slots` would drop from about
17 s to about 3-4 s. A push without factory captures would drop by about 40 s, and a push with them
by about 25 s.

## Smaller levers

- **The full READ's 3 s quiet tail.** Upstream's own default is 1.5 s. Don't shorten it before the
  new "longest gap" number shows the real gaps between folder messages on this unit. If the full
  READ also opens with `user_content_estimate` (the new log line shows it), use the count there too.
- **Post-save polling.** It sleeps 6 s and then lists. With a 2-3 s listing, polling every 2 s over
  the same window of about 2 minutes would confirm the save soon after it lands. G3 saw the slot
  still blank 3 s after the save, so keep the window long.
- **Job pickup.** `POLL_FAST_S` 5 s could become 2 s, and the 5 minute fast window could be longer.
  Contract section 14 (gate G5) found a 3-5 s interval while running is not cost-constrained. 2 s is
  outside what it checked, so redo its arithmetic before changing it (**unverified**).
- **Per-write round trips.** Before every write the push reads `SetlistPosition`. Our `request()`
  override waits in `_collect(0.1)` slices, so each read costs at least 0.1 s even when the reply is
  faster, on top of `PACE_S` 0.15 s. A 0.02 s slice in `_make_request` (our code, not the vendored
  file) would cut that. Keep `PACE_S` as G3 measured it. **Unverified** on the unit.
- **Connecting once per job** costs about 1.3 s. Keeping the session open across jobs conflicts with
  plan section 5 (exclusive access only while a job runs, so Cortex Control works in between). Not
  worth it.

## The MCP `wait_seconds` ceiling

`maxDeviceWait` is 120 s (`internal/mcpserver/devicetools.go`), but the client was **seen** giving up
at 60 s. Cap it at about 45 s, or say in the tool descriptions that longer waits fail on some
clients and that the caller should poll `get_device_job`. Even with the user-only READ, a push with a
factory capture may exceed 45 s, so polling stays the documented path.

## Measure first (added with this note)

The bridge log now has:

- `job <id> <kind>: started, queued N s before pickup`: the bucket wait. It compares the server's
  `created_at` with this PC's clock, so it is approximate.
- `connected in N s (open, handshake, firmware read)`.
- One line per listing, numbers only, no device text: `listing complete in T s: first folder after
  A s, stream S s (longest gap G s), quiet tail Q s; F folders, N files, K KB;
  user_content_estimate E|none`.

Next step on the unit: run `list_slots` and one push, then read those lines. That settles the split
above and whether the full READ also sends an estimate. Then implement the user-only READ behind
the hardware checks in "What has to be verified on the unit first".
