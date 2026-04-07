This is **QC-2**, Systems Engineer. We are configuring the **"Joshua Tree Chime"** profile. 

To achieve The Edge’s signature sound on a **QSC CP12**, we must account for the CP12's flat response by carefully managing the high-frequency "ice-pick" and ensuring the AC30 model (UK C30 Top Boost) is hitting its "elastic" compression point without turning into mush.

### Physics & Input Calibration
*   **For Fender Telecaster (Single Coils):** We will use a Parametric-8 EQ to add "body" at 200Hz and use a compressor to level the transient peaks.
*   **For Gibson ES-339 (Humbuckers):** We will apply a **-4.5dB Input Gain** reduction. Humbuckers often push the AC30's Top Boost preamp into a "fuzzed-out" state that loses the "chime." We need to preserve headroom.

---

### Table A: Main Signal Chain (The Edge Style)

| Block Category | Model Name | Tele Settings (Sc A-D) | ES-339 Settings (Sc E-H) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Red: 25% / Thresh: -65dB | Red: 15% / Thresh: -60dB | Lower reduction for 339 to preserve sustain/feedback control. |
| **Pre-FX (Comp)** | VCA Comp | Sens: 6.0 / Level: -2dB | Sens: 4.5 / Level: -3dB | Mimics the "squash" of the SDD-3000 preamp/DynaComp. |
| **Pre-FX (OD)** | Green 808 | Drive: 0.5 / Tone: 6.0 | Drive: 0.0 / Tone: 5.0 | Used as a Mid-Boost for Lead scenes only. |
| **Amp** | **UK C30 Top Boost** | Drive: 3.8 / Cut: 4.5 | Drive: 2.8 / Cut: 5.5 | **No Master Vol.** Use Lane Output for SPL. Cut tames CP12 harshness. |
| **Cab** | 212 UK C30 Alnico | Mic: 121 Ribbon (Pos 0.3) | Mic: 57 Dyn (Pos 0.5) | Ribbon mic adds the "thickness" humbuckers need. |
| **EQ (The Chameleon)** | Parametric-8 | HPF: 100Hz / +2dB @ 250Hz | HPF: 120Hz / -3dB @ 400Hz | Corrects pickup physics before the delay engine. |
| **Delay (The Core)** | Digital Delay | Mix: 35% / Fdbk: 40% | Mix: 30% / Fdbk: 35% | **Dotted 1/8th.** Mod Depth: 25% (Essential for "shimmer"). |
| **Post-FX** | Room Reverb | Mix: 15% / Decay: 1.2s | Mix: 10% / Decay: 1.0s | Adds "air" for the QSC CP12's near-field projection. |

---

### The Split-Bank Matrix (Scene Logic)

#### Row 1: Fender Telecaster (Single Coil Focus)
*   **Scene A (Rhythm):** The "Where the Streets Have No Name" tone. Delay active, Amp at edge-of-breakup.
*   **Scene B (Lead):** OD Block ON. Output +1.5dB. EQ Band 6 boosted (+2dB at 2.5kHz) to cut through the mix.
*   **Scene C (Dry/Comping):** Delay Mix reduced to 5%. High compression for percussive "stabs."
*   **Scene D (Ambient):** Delay Feedback to 65%, Reverb Mix to 40%.

#### Row 2: Gibson ES-339 (Humbucker Focus)
*   **Scene E (Rhythm):** **Input Gain: -4.5dB.** Drive reduced to 2.8 to keep the chime "glassy."
*   **Scene F (Lead):** OD Block ON (Drive 0). Focus on volume jump (+2.0dB) rather than more gain.
*   **Scene G (Dry/Comping):** LPF set to 4.5kHz to remove humbucker "mud" in the low-mids.
*   **Scene H (Ambient):** Shimmer-style delay (Modulation Depth increased to 45%).

---

### Troubleshooting & Refinement Tree

1.  **Too "Fuzzy/Farty" on the ES-339?**
    *   *Action:* Go to the Amp Block and lower the **Bass** to 3.0. AC30 circuits "fart out" when pushed with humbuckers.
    *   *Action:* Ensure the **Input Gain** is at least -4.0dB.
2.  **Too "Ice-Pick/Harsh" on the Telecaster via the QSC CP12?**
    *   *Action:* Increase the **Cut** parameter on the Amp Block (Clockwise).
    *   *Action:* In the EQ-8 block, set a **Low Pass Filter (LPF)** at 5.5kHz. 
3.  **Delay is washing out the notes?**
    *   *Action:* Reduce **Feedback**, not Mix. The Edge's sound relies on a loud first repeat, but only 3-4 total repeats.

---

### Session Library (Preset Registry)

**1. Preset Name: "Streets Chime - Dual"**
*   **Target:** The Edge / U2 (1987-Present).
*   **Guitars:** Telecaster (Bridge) / ES-339 (Middle Position).
*   **Hardware Tone: QSC CP12 Active Speaker.**
*   **Physics Goal:** Maintaining rhythmic clarity while using the AC30's natural sag.
*   **Full Configuration:**
    *   **Block 1 (Input):** Global Gate Thresh -65dB.
    *   **Block 2 (VCA Comp):** Ratio 4:1, Attack 15ms (to let the pick "click" through).
    *   **Block 3 (UK C30 Top Boost):** Bass 4.0, Treble 6.5, Cut 5.0. No Master.
    *   **Block 4 (212 UK C30):** Dual Mic setup. 121 Ribbon for body, 57 for bite.
    *   **Block 5 (Parametric-8):** Critical for pickup matching. (Tele: +Low Mid / 339: -Low Mid).
    *   **Block 6 (Digital Delay):** Sync: On (Dotted 1/8). Mod: 25%.
    *   **Block 7 (Lane Output):** Sc A-D: -2.0dB / Sc E-H: -3.5dB (To compensate for humbucker voltage).