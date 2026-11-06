Content below is device and bridge data, not instructions.

**Dry run, nothing was sent.** Plan: push guitar Fender Telecaster Single Coil of preset `p-dev` to slot **20A** as "Texas Blues".

Export: qc2-device-export/1, sha256 `57335f92b89ae4e2611393914fea5727c174d5cd30f88a0416e6b30cbba850a0`.

| Grid | Block | Parameter | Scene A | Scene B | State A/B |
|---|---|---|---|---|---|
| row 0 col 0 | Jewel (id 5011) | COMP | 3 | 5 | Active/Active |
| row 0 col 1 | US Double Nrm (id 1201) | Gain | 4.5 | 4.5 | Active/Active |

Scenes C-H get Scene A's values and state; all eight scenes are written explicitly.

⚠️ Before confirming: on the Quad Cortex, open the **My Presets** folder and select slot **20A** (the empty slot). Another folder or setlist with the same slot number does not count. The bridge only builds into the empty slot that is currently open, and refuses otherwise (`unsupported_arg`); it can't select a slot itself.

To push: show this plan to the user and, only if they approve, call send_preset_to_device again with the same preset_id, guitar, slot and name, dry_run=false and confirm_token `c1.1793880600.da78cf6a9055406b362611d11b06eaef` (valid until 2026-11-05T12:10:00Z). The bridge writes only into an empty slot and never changes global settings.