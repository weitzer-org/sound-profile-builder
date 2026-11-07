# Editing a preset that lives on the Quad Cortex (device draft)

This preset was read back from the user's physical Quad Cortex. It is not a fresh design: the unit already holds it, and your answer will be applied to the unit as a small patch of parameter and bypass changes, never as a rebuild. For this refinement these rules win over any general refinement guidance above.

1. Keep every block exactly as listed: the same "id", the same "model", the same order. Return the whole chain, including the blocks you do not change.
2. Prefer changing parameter values ("value" for Scene A, "value_b" for Scene B) and the Bypass parameter. Almost every tone request can be met that way.
3. Do not swap, add, remove or reorder blocks unless the request cannot be met without it. If a block is truly needed, give it a new unique "id" and say in its "rationale" why the request needs it. A request to sound like another amp or pedal is not a reason to swap a model: move the existing controls toward it, and say in "conversational_response" what a swap would add.
4. To switch an effect off, set its Bypass parameter to On in the scenes concerned. Do not delete the block.
5. Change only controls a block already lists. A control that is not listed on a block is locked on the unit: never add it, never mention it, and do not try to make up for it elsewhere.
6. Leave every block and control the request does not concern exactly as it is: the same value, value_b and basis.
7. Say in "conversational_response" what you changed and what you left alone, in the user's terms. If the request cannot be met without a structural change, say so and offer the nearest parameter-only change instead of making the structural one.
