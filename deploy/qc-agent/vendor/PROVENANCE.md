# Vendored: qc-mcp transport

| | |
|---|---|
| Upstream | https://github.com/lexasoft123/qc-mcp |
| Commit | `f634ecd3dcf426090b90bc4cc176e4b500494b9e` (committed 2026-10-01T15:14:41+03:00) |
| License | MIT, upstream's `LICENSE` copied as `qc_mcp/LICENSE` |
| Reviewed in | `docs/research/qc-usb-tools-audit.md` (the same commit) |
| Used by | `deploy/qc-agent/qc_agent/qcmcp_backend.py`, as a library (never qc-mcp's MCP server) |
| Vendored | 2026-11-05, for plan P4 (`docs/design/qc-device-bridge-plan.md` section 5) |

## Files

Copied byte for byte with `cp` from a clone checked out at the commit above. Nothing is modified.
`.gitattributes` here sets `* -text` so a Windows checkout with `core.autocrlf` doesn't rewrite the bytes.

| Vendored path | Upstream path | Git blob at f634ecd | SHA-256 |
|---|---|---|---|
| `qc_mcp/LICENSE` | `LICENSE` | `bf29392bd4cc44f0a03d909757859ac8faa192e7` | `5a8c27f4317d555055d8eb3868c23dc0e0d11ff511fd0905b617c7c254a6ebc1` |
| `qc_mcp/__init__.py` | `src/qc_mcp/__init__.py` | `8b137891791fe96927ad78e64b0aad7bded08bdc` | `01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b` |
| `qc_mcp/transport.py` | `src/qc_mcp/transport.py` | `11c95c795f7a5409d5f50eed5aef1bbd6d76b0c2` | `9c5229946e5656edc7650f9b8c7b195d4083286137f5cd54ecc1898a3202ad3a` |
| `qc_mcp/protocol.py` | `src/qc_mcp/protocol.py` | `98b6a9c97de7646b7714ae47ea17322adf6faf87` | `197a447bcaba1601df659586ed28bd62b9dc640854101bc76c1c1f8fbd9cf911` |
| `qc_mcp/backend.py` | `src/qc_mcp/backend.py` | `4c7033ad3f3e8173d2d2c81e3c971828ba5e1962` | `bc13d6c6921faa4bd585b1a5da7cb695f413ccb3968cfa7c9fa1bf31b52afa55` |
| `qc_mcp/winhid.py` | `src/qc_mcp/winhid.py` | `d5b6b5317e9c9d771a649b061666ba2b6f452c37` | `5359005ecb5163455fcfaad09ed50b827e1765faf0b83df986d2bfbfe32235ab` |
| `qc_mcp/directory.py` | `src/qc_mcp/directory.py` | `c5aea505d479111edd83b07a7a0560d6a510daaa` | `ddacedf0f58521b57d643887509ec1dca1f18b434213944f57e43b9d83593205` |
| `qc_mcp/descriptors/qc_descriptors-4.0.pb` | `src/qc_mcp/descriptors/qc_descriptors-4.0.pb` | `2f2bcf60c48fe378a36d33f723916c504db29d18` | `fe672cdd85886bac0ccae31920c7dd8a7650aaaaeaedb3301e55663e0fffdfb3` |
| `qc_mcp/descriptors/qc_descriptors-4.1.pb` | `src/qc_mcp/descriptors/qc_descriptors-4.1.pb` | `294caf46af4b8e203dd4ce58f8a3ec350faf4837` | `eed223846169c1beb7a62b76c288d04d541fd1d5fd1853d0a2d8f68660f530c7` |

`__init__.py` (a single newline upstream) is vendored too: the five modules use relative imports and need the package.

Check them yourself on any clone of upstream:

```bash
git -C qc-mcp rev-parse f634ecd:src/qc_mcp/transport.py      # the blob id above
git -C qc-mcp show f634ecd:src/qc_mcp/transport.py | sha256sum
```

## How the pins are enforced

- `tests/test_vendor_pins.py` fails if any file differs, is missing, or an unpinned file appears under `qc_mcp/`.
- At runtime `qcmcp_backend.load_vendored()` recomputes the same hashes and refuses to import the transport on any
  mismatch, then checks the imported modules came from this folder (not a qc-mcp installed in the venv).
- The pins live in `qc_agent/vendor_pins.py`; this table must list the same values (the test checks).

## Not vendored, and why

- `server.py` and the other 50+ MCP tools: the agent calls the transport directly (plan section 5).
- `catalog.py` and `ModelRepo.xml`: the agent validates against this repo's `internal/catalog/qc_catalog.json`,
  whose clone-expanded parameter indexes matched the device in Phase 0 gate G1; qc-mcp's unexpanded reading didn't.
- `bridge.py`, `winbridge.py`, `iohid.py`: bridge mode and macOS. `backend.py` imports them lazily, only on those
  paths, which the agent never takes (exclusive direct mode on Windows only).
- `lockfile.py`, `daemon.py`, leveling and audio modules.

## Behaviour the agent relies on, and works around

From the audit (section 1.5) and Phase 0 (plan section 7):
- `save_preset` (RecallPreset SAVE) can hang the unit on an unsaved slot. The agent saves only with
  `write_preset_file` (File CREATE), and its wire guard refuses any RecallPreset UPDATE.
- `send()` counts non-benign write codes in `write_errors` instead of raising; G2(c) found that and a dead reader
  thread are the earliest signs the unit went away. The agent reads both (private attributes, read-only).
- `float_value` travels as a 32-bit float: 0.2 reads back as 0.20000000298. The read-back diff uses a 1e-4 tolerance.
- `protocol._recovered_fdps` would import `tools/extract_protos` if the descriptor sets were missing; both are
  vendored, so that path never runs.

## Re-vendoring

Only through a PR that changes all three together: the files, `qc_agent/vendor_pins.py` and this table, after a
fresh review of the upstream diff (security and device safety, as in the audit).
