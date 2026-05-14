### QUAD CORTEX SYSTEMS ENGINEER (QC-2)
**Target Acknowledged:** John Mayer Trio Lead
**Hardware Profile:** Cortex Control (Mac Desktop) -> QSC CP12 (Active FRFR)
**Analog Counterpart:** Dumble Overdrive Special Clean Channel + TS10/Klon + Two-Rock 2x12 (Celestion G12-65).
**Closest Available QC Models:** *Custom ODS Clean* (Amp), *212 Custom G65* (Cab), *Green 808* (Drive).

To achieve this iconic "glassy but pushed" tone on an active PA speaker like the QSC CP12, we must strictly manage the upper-midrange frequencies to prevent digital harshness, while utilizing the **Chameleon Strategy** to adapt the exact same preset for both your ES-339 and your Telecaster. 

---

### MULTI-GUITAR GAIN STAGING STRATEGY
Because the Fender Telecaster and Gibson ES-339 have vastly different magnetic profiles and output voltages, we are utilizing a **Split-Bank Matrix**:
*   **Row 1 (Scenes A-D) - Fender Telecaster (Single Coil):** The goal here is adding fundamental weight. We rely on the EQ-8 to boost the 200Hz body and slightly push the amp volume to hit edge-of-breakup.
*   **Row 2 (Scenes E-H) - Gibson ES-339 (Humbucker):** The goal here is clarity and headroom. The ES-339's higher output will naturally clip the *Custom ODS Clean* too early. We will attenuate the Input Gain by -4.0dB and use the EQ-8 to clear out 400Hz low-mid mud, allowing the natural humbucker compression to sing.

---

### TABLE A: MAIN SIGNAL CHAIN
*Note: All parameters marked with `(Assign)` must be Right-Clicked > "Assign to Scene" in Cortex Control. Values are displayed as **[Telecaster] / [ES-339]**.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -62dB `[A]` / -55dB `[E]`<br>Gain: 0.0dB `[A]` / -4.0dB `[E]` | Thresh: -62dB `[B]` / -55dB `[F]`<br>Gain: 0.0dB `[B]` / -4.0dB `[F]` | ES-339 requires a -4.0dB pad to prevent premature preamp clipping. Humbuckers also need a higher gate threshold for sympathetic noise. |
| **Pre-FX (Chameleon)** | Parametric-8 | Band 1 (Low Shelf): +3.0dB `[A]` / 0.0dB `[E]`<br>Band 3 (Peak): 0.0dB `[A]` / -2.5dB @ 400Hz `[E]` | Band 8 (LPF): 5000Hz `[B]` / Off `[F]` | Adds 200Hz weight to the Telecaster. Scoops 400Hz mud from the ES-339. Telecaster LPF tamed on Lead to prevent pick-attack harshness. |
| **Pre-FX (Drive)** | Green 808 | Bypass State: OFF (Assigned) | Bypass State: ON (Assigned)<br>Drive: 3.5<br>Tone: 6.0<br>Level: 7.5 | Simulates Mayer's TS10. By keeping Drive low and Level high, we push the tubes rather than relying on op-amp clipping. |
| **Amp** | Custom ODS Clean | Vol (Assign): 5.5 `[A]` / 4.5 `[E]`<br>Bass: 4.0<br>Mid: 3.5<br>Treble: 6.0 | Vol (Assign): 5.5 `[B]` / 4.5 `[F]`<br>Deep Sw: ON<br>Bright Sw: OFF | Dumble-style preamp is naturally bottom-heavy. Mid is scooped to 3.5 for the "glassy" Mayer rhythm platform. ES-339 Vol is lowered to retain headroom. |
| **Cab** | 212 Custom G65 | Mic A (Dyn 57): Pos 1.5, Dist 1.0"<br>Mic B (Rib 121): Pos 1.0, Dist 2.5"<br>Mix: A at 0dB, B at -3dB | Same as Rhythm | G12-65 speakers naturally roll off harsh highs. The Ribbon 121 adds 3D depth, counteracting the directional tweeter of the QSC CP12. |
| **Post-FX (Delay)** | Analog Delay | Mix (Assign): 5%<br>Time: 350ms<br>Fdbk: 20% | Mix (Assign): 15%<br>Time: 350ms<br>Fdbk: 30% | Mayer uses subtle analog slap/trail to thicken leads without muddying the rhythm chops. |
| **Post-FX (Reverb)** | Spring Reverb | Mix (Assign): 25%<br>Dwell: 4.0<br>Tone: 5.0 | Mix (Assign): 30%<br>Dwell: 5.5<br>Tone: 5.5 | Replicates the Fender-style spring tank built into Two-Rock/Dumble platforms. Increased Dwell/Mix on Lead for sustain. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | Output Level raised for Lead SPL increase without adding further preamp saturation (The Headroom Rule). |

---

### TROUBLESHOOTING & REFINEMENT TREE
If playing through the **QSC CP12** and the tone feels:
1.  **Too "Fuzzy" on the ES-339 Lead:** Lower the Global Input Gain from -4.0dB to -6.0dB. Do *not* turn down the Amp Volume first.
2.  **Too "Ice-Pick" on the Telecaster:** Do not lower the Treble on the amp. Instead, go to the EQ-8 Block and lower the Band 8 LPF from 5000Hz down to 4200Hz. This specifically tames the QSC CP12's high-frequency driver without ruining the amplifier's fundamental presence.
3.  **Lacking Sustain on Lead:** Increase the `Level` parameter on the Green 808, *not* the Drive parameter.

---

### SESSION LIBRARY (ACTIVE PRESETS)
*(Appending to Session Memory for future recall)*

**2. Preset Name:** "Gravity Trio - Tele/339 Split"
**Target:** John Mayer Trio (2005).
**Guitar:** Matrix Split - Fender Telecaster (SC) / Gibson ES-339 (HB).
**Physics Goal:** Deep-scooped high-headroom clean platform with TS-style mid-hump lead push, optimized for FRFR playback.
**Full Configuration:**
*   **Input (Global Gate):** Thresh [-62dB ScA-D / -55dB ScE-H], Gain [0.0dB ScA-D / -4.0dB ScE-H].
*   **Block 1 (EQ-8):** HPF [80Hz], Band 1 Low Shelf [200Hz, +3dB ScA-D / 0dB ScE-H], Band 3 Peak [400Hz, 0dB ScA-D / -2.5dB ScE-H], Band 8 LPF [5000Hz ScA-D / Off ScE-H].
*   **Block 2 (Green 808):** Bypass [On Scenes B/F, Off Scenes A/E], Drive [3.5], Tone [6.0], Level [7.5].
*   **Block 3 (Amp - Custom ODS Clean):** Vol [5.5 ScA-D / 4.5 ScE-H], Bass [4.0], Mid [3.5], Treble [6.0], Deep [On], Bright [Off].
*   **Block 4 (Cab - 212 Custom G65):** Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.0, Dist 2.5"), Mix [A: 0dB, B: -3dB].
*   **Block 5 (Analog Delay):** Mix [5% Rhy / 15% Lead], Time [350ms], Fdbk [20% Rhy / 30% Lead].
*   **Block 6 (Spring Reverb):** Mix [25% Rhy / 30% Lead], Dwell [4.0 Rhy / 5.5 Lead], Tone [5.0 Rhy / 5.5 Lead].
*   **Lane 1 Output:** Level [0.0dB Rhy / +1.5dB Lead].