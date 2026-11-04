# How to present a QC-2 preset in chat

The QC-2 web app shows a preset as one page: name, the Architect's builder statement,
a tab per guitar, and per tab one row per block (type and model, rationale in small
italics underneath) beside Scene A (Rhythm) and Scene B (Lead) settings. Chat cannot
carry that HTML, so keep the same shape in Markdown. get_preset and generate_preset
results already arrive in it; keep it.

## Rules
1. Lead with the preset ID and whether it is saved or an unsaved draft (a draft is
   named "Draft Preset" until save_preset is called).
2. Then the builder statement, as the pipeline wrote it.
3. One table per guitar under a `###` heading with its name, the same columns for every
   guitar: `# | Block | Model | Scene A (Rhythm) | Scene B (Lead)`. Never describe a
   second guitar as a prose diff of the first, even when they are close.
4. A and B stay in separate columns, not "A→B" in one cell. A block whose B settings
   equal A shows "same as A" in the B column.
5. Keep the `*` after an estimated value and the `\* = estimated value` legend.
6. Always show the Why rationales (one line per block) and every pipeline warning:
   a ⚠️ in a model name ("Unverified") and ⚠️ notes inside a rationale (capture
   formatting, incomplete cabinet, leftover value range). Never drop or soften them.
7. Keep pipeline data and your own words visibly apart. Put anything you add (tips,
   doubts, playing advice) under a heading labeled "My take, not from the pipeline".
   Do not edit the pipeline's text in place.
8. End by offering next steps: refine (refine_preset; ask what to change) and save
   (save_preset; ask for a name). Do not start either on your own.

## Skeleton
**Preset `<preset_id>`** · unsaved draft (or: saved as "<name>")
<builder statement>
### <Guitar 1>
(table: `# | Block | Model | Scene A (Rhythm) | Scene B (Lead)`, one row per block)
**Why:** one line per block, with any ⚠️ note
### <Guitar 2>
(same columns, same layout)
\* = estimated value
**My take, not from the pipeline:** optional
Next: refine anything, or give me a name and I will save it.
