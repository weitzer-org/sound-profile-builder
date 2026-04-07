Acknowledged. QC-2 Systems Engineer online. Target tone is "Early British Invasion"—a profile characterized by the chime of EL84 power tubes (AC30) and the foundational warmth of KT66 tubes (JTM45), operating at the edge of breakup. The objective is a dynamic preset that responds to pick attack, providing sparkle when played softly and harmonic compression when pushed.

This build will use the **UK C30 Top Boost** as the primary amplifier model due to its characteristic high-end detail and mid-range complexity, which is central to the requested tone.

**Initial Verification:** Before finalizing gain stages, confirm your pickup configuration. **Are your pickups Vintage Output (e.g., standard Tele/Strat), Medium (e.g., PAF), or High Output (e.g., Modern/Active)?** The following settings are calibrated for a medium-output baseline and must be adjusted per my instructions.

This preset is designed using the Split-Bank Matrix. Row 1 (Scenes A-D) is optimized for a **Fender Telecaster**. Row 2 (Scenes E-H) is adapted for a **Gibson ES-339**.

### Preset: BritChime 64

The core of this tone is managing the amp's "Cut" control and using a treble booster for lead scenes, which pushes the specific upper-mid frequencies that cause the amp to compress musically without adding excessive low-end saturation.

---

### Table A: Main Signal Chain

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input Gate | Threshold: -65.0dB | Threshold: -65.0dB | Set just above the guitar's noise floor. Vintage single coils may require a higher threshold (-55dB). |
| **Pre-FX** | Chief Treble | **(Bypassed)** | Level: 6.5, Range: Full<br/>**(Active)** | (Right-Click > Assign) Pushes the amp's preamp tubes for gain and harmonic saturation without adding mud. Critical for the lead tone. |
| **Amp** | UK C30 Top Boost | Vol Norm: 4.5<br/>Vol TB: 6.0<br/>Treble: 7.0<br/>Bass: 4.5<br/>Cut: 6.5 | Vol Norm: 4.5<br/>Vol TB: 6.0<br/>Treble: 7.0<br/>Bass: 4.5<br/>Cut: 6.5 | The Top Boost volume provides the gain; Bass is kept low to maintain clarity. The **Cut** knob is a post-PI master tone control; higher values *reduce* treble. This setting tames harshness. |
| **Cab** | 212 UK C-Blue | Mic A: Dyn 57 (Dist 1.0")<br/>Mic B: Ribbon 121 (Dist 3.0")<br/>Mix: A:0dB, B:-3.5dB | Mic A: Dyn 57 (Dist 1.0")<br/>Mic B: Ribbon 121 (Dist 3.0")<br/>Mix: A:0dB, B:-3.5dB | Simulates the definitive Celestion Alnico Blue speaker. The Dyn 57 provides mid-range bite, while the Ribbon 121 captures warmth and body, creating a full-range sound for the PA. |
| **Post-FX (EQ)** | Parametric-8 | *See Guitar-Specific EQ Below* | *See Guitar-Specific EQ Below* | The "Chameleon" EQ shapes the core amp tone to match the physics of different pickup types. |
| **Post-FX (Delay)** | Tape Delay | Mix: 18%<br/>Time: 1/8<br/>Fdbk: 20% | Mix: 25%<br/>Time: 1/8.<br/>Fdbk: 25% | A subtle slapback/echo effect typical of studio recordings from the era. Dotted eighth note provides rhythmic texture for lead lines. |
| **Post-FX (Reverb)**| Plate Reverb | Mix: 22%<br/>Decay: 1.8s<br/>HPF: 150Hz | Mix: 22%<br/>Decay: 1.8s<br/>HPF: 150Hz | Simulates a studio plate reverb, adding space without washing out the core tone. The HPF prevents low-end mud. |
| **Lane Output** | Output Level | Level: -1.5dB | Level: 0.0dB | Balances scene volumes. The Lead scene is the reference 0.0dB, with Rhythm slightly lower to create dynamic lift. |

---

### Guitar-Specific EQ Configurations (Chameleon Strategy)

#### Row 1 (Scenes A-D): Fender Telecaster Profile
The goal here is to add body and tame the aggressive pick attack inherent to single-coil pickups.

| Band | Type | Freq (Hz) | Q | Gain (dB) | Rationale |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **1** | Low Shelf | 200 | 0.7 | +1.5 | Adds weight and "body" to compensate for the thinner single-coil sound. |
| **8** | Low Pass | 5500 | 0.7 | 0.0 | Tames the "ice pick" frequencies common in Telecaster bridge pickups, ensuring a smooth top end through the PA. |

#### Row 2 (Scenes E-H): Gibson ES-339 Profile
The goal is to increase clarity and prevent the low-mid "mud" that humbuckers can produce with an AC30 circuit.

| Band | Type | Freq (Hz) | Q | Gain (dB) | Rationale |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **1** | High Pass | 100 | 0.7 | 0.0 | Tightens the low-end response, removing non-musical sub-frequencies and preventing speaker "flub." |
| **4** | Peak | 450 | 1.4 | -2.0 | A surgical cut to reduce the "boxy" or "muddy" low-mid buildup from humbuckers, enhancing note definition. |

---

### Scene Configuration & Gain Staging

**Split-Bank Matrix:**
*   **Scenes A-D (Row 1):** For Telecaster.
*   **Scenes E-H (Row 2):** For ES-339.

**Scene Functions:**
*   **A / E (Rhythm):** Core tone. Treble booster is OFF.
*   **B / F (Lead):** Treble booster is ON. Lane output is +1.5dB louder than rhythm.
*   **C / G (Dry/Comping):** Same as Rhythm, but with Delay and Reverb blocks bypassed.
*   **D / H (Ambient):** Based on the Rhythm scene. Increase Delay Mix to 35% and Reverb Mix to 40% for swells or atmospheric parts.

**Pickup Output Compensation (The Headroom Rule):**
*   **If your pickups are High Output:** The amp will distort prematurely.
    1.  Go to the **Input Block** on the Grid.
    2.  Set the **Level to -3.0dB**.
    3.  Reduce the **Amp Block -> Vol TB to 5.0**. This maintains the "edge of breakup" feel.
*   **If your pickups are Vintage/Low Output:** The sound may be too clean.
    1.  Go to the **Input Block** on the Grid.
    2.  Set the **Level to +3.0dB**.
    3.  Alternatively, leave the Input at 0dB and increase the **Amp Block -> Vol TB to 7.0**.

**To adjust loudness without changing tone, use the Lane Output Level. To adjust drive/breakup, use the Amp Block's Vol TB knob or the Pre-FX Level.**

---
### PRESET REGISTRY PROTOCOL (SESSION MEMORY)

*This configuration has been added to the active session library.*

**1. Preset Name:** "BritChime 64"
    *   **Target:** Early British Invasion (Vox AC30 Top Boost).
    *   **Guitar:** Fender Telecaster (Vintage Output).
    *   **Physics Goal:** Dynamic edge-of-breakup with chime. Lead tone achieved via preamp push, not a distortion pedal.
    *   **Full Configuration:**
        *   **Block 1 (Global Gate):** Threshold [-65.0dB].
        *   **Block 2 (Chief Treble):** Bypassed on Scene A, Active on Scene B [Level 6.5, Range Full].
        *   **Block 3 (Amp - UK C30 Top Boost):** Vol Norm [4.5], Vol TB [6.0], Treble [7.0], Bass [4.5], Cut [6.5].
        *   **Block 4 (Cab - 212 UK C-Blue):** Mic A (Dyn 57, Dist 1.0"), Mic B (Ribbon 121, Dist 3.0"), Mix [A: 0dB, B: -3.5dB].
        *   **Block 5 (EQ-8 for Tele):** Low Shelf [200Hz, +1.5dB], LPF [5500Hz].
        *   **Block 6 (Tape Delay):** Scene A [Mix 18%, 1/8, Fdbk 20%], Scene B [Mix 25%, 1/8., Fdbk 25%].
        *   **Block 7 (Plate Reverb):** Mix [22%], Decay [1.8s], HPF [150Hz].
        *   **Block 8 (Lane Output):** Scene A [-1.5dB], Scene B [0.0dB].