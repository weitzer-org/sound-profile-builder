**SYSTEM IDENTIFICATION:** QC-2 (Quad Cortex Systems Engineer)
**TARGET VERIFIED:** "Texas Flood" - Stevie Ray Vaughan (1983).
**ANALOG SPECS:** High-headroom 6L6 Fender Blackface circuits (Vibroverb/Super Reverb), 4x10/1x15 Jensen speakers, heavy string gauge (.013s), Ibanez TS808 Tube Screamer used strictly as a clean, mid-boosting line driver.
**FRFR PROFILE:** QSC CP12 (Active 12"). High-Pass Filter (HPF) required at ~90Hz to prevent low-end mud from 4x10 cabinet resonance in a PA speaker.

Per **Rule 9** and **Rule 3**, the Quad Cortex reacts drastically differently to Single Coils vs. Humbuckers. SRV’s tone relies on extreme input voltage hitting a high-headroom preamp. To achieve this correctly, we must split the physical build into two distinct profiles.

---

### GAIN STAGING & PICKUP COMPENSATION
Before touching the grid, configure your Input Block (Circle 1):

*   **Guitar 1: Fender Single Coil (Strat/Tele) - ROW 1**
    *   **Input Gain:** +0.0dB.
    *   **Global Gate Threshold:** -62.0dB (Strat single coils need a relaxed gate to allow for dynamic, uncompressed picking).
*   **Guitar 2: Gibson ES-339 (Humbuckers) - ROW 2**
    *   **Input Gain:** -4.5dB. (Crucial. Humbuckers will inherently clip the *Green 808* diode stage. Dropping input gain simulates the output of vintage underwound Strat pickups).
    *   **Global Gate Threshold:** -55.0dB.

---

### ORGANIZATION STANDARD: SPLIT-BANK MATRIX
*   **Row 1 (Scenes A-D): Fender Single Coil Profile.** EQ focused on adding body (250Hz) to mimic .013 gauge strings.
*   **Row 2 (Scenes E-H): Gibson ES-339 Profile.** EQ focused on removing low-mid mud (300Hz) and tightening the attack to simulate single-coil articulation.

---

### TABLE A: MAIN SIGNAL CHAIN (Texas Flood)
*Mark Scene-Specific changes clearly by Right-Clicking the parameter and choosing "Assign".*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -62dB (SC) / -55dB (HB) | Thresh: -62dB (SC) / -55dB (HB) | Preserves SRV's extreme picking dynamic range. |
| **Pre-FX 1** | Green 808 | Drive: 0.0, Tone: 6.0, Level: 6.5 | Drive: 1.5, Tone: 7.0, Level: 10.0 | **The SRV Secret:** Maxing pedal Level with minimal Drive pushes the Amp's 12AX7 preamp block into organic compression. |
| **Pre-FX 2** | Parametric-8 EQ | Band 2: +2.5dB at 250Hz (SC) | Band 2: -3.0dB at 300Hz (HB) | *Chameleon Strategy:* SC gets string "girth". HB gets "de-mudded" to mimic hollowed-out Strat neck pickup. |
| **Amp** | US Twin 65 Normal | Vol: 6.0, Bass: 4.0, Mid: 6.5, Treb: 5.5 | Vol: 6.0, Bass: 4.0, Mid: 6.5, Treb: 5.5 | Bright Switch OFF. High headroom 6L6 platform. Mids boosted higher than standard "scooped" Fender settings. |
| **Cab** | 410 US Basslad PR10 | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") | Captures the punchy 4x10 Super Reverb physics. HPF set to 90Hz internally to tame QSC CP12 boominess. |
| **Post-FX 1** | Spring Reverb | Mix: 12%, Decay: 2.0s, Tone: 5.5 | Mix: 15%, Decay: 2.2s, Tone: 6.0 | Analog spring tank replication. Extended decay on lead scenes for Texas blues trailing notes. |

---

### TROUBLESHOOTING & REFINEMENT TREE
If the tone sounds "Too Distorted", "Fuzzy", or "Harsh" through your QSC CP12:
1.  **Input Pad:** Lower the Input Block Gain an additional -3.0dB (especially if your ES-339 has high-output PAFs).
2.  **Amp Gain:** The `US Twin 65` should remain clean. If it's crunching too hard on Rhythm, lower Amp Vol from 6.0 to 4.5.
3.  **Harsh Pick Attack:** If the TS808 sounds like an ice-pick on the Strat bridge pickup, go to the EQ-8 Block and lower **Band 8 (LPF)** to `4500Hz`.
4.  **Output Compensation:** If adjustments cause a drop in gigging SPL, *do not touch the amp volume*. Raise the Lane Output Level (far right circle) by +2.0dB to push the QSC CP12 harder.

---

### SESSION REGISTRY PROTOCOL
Updating Active Presets Session Library.

**Session Library (Active Presets)**
1. **Preset Name:** "Spoonful - ES339" | Target: Howlin' Wolf (1960) | Configuration: [Stored in previous memory state].
2. **Preset Name:** "Texas Flood - Split Matrix"
   Target: Stevie Ray Vaughan (1983).
   Guitars: Fender Stratocaster (Row 1) & Gibson ES-339 (Row 2) w/ Heavy Pick.
   Physics Goal: Extreme clean headroom pushed entirely by line-level voltage from a diode-clipping overdrive pedal.
   Full Configuration:
   *Block 1 (EQ-8):* HPF [85Hz], SC Profile Band 2 [+2.5dB @ 250Hz], HB Profile Band 2 [-3.0dB @ 300Hz], LPF [6500Hz].
   *Block 2 (Green 808):* Drive [Rhy: 0.0 / Lead: 1.5], Tone [Rhy: 6.0 / Lead: 7.0], Level [Rhy: 6.5 / Lead: 10.0].
   *Block 3 (Amp - US Twin 65 Normal):* Bright [OFF], Vol [6.0], Bass [4.0], Mid [6.5], Treble [5.5], Output Level [+2.0dB].
   *Block 4 (Cab - 410 US Basslad PR10):* Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Dyn 421, Pos 0.8, Dist 2.5"), Mix [A: 0dB, B: -3dB], HPF [90Hz], LPF [8000Hz].
   *Block 5 (Spring Reverb):* Mix [Rhy: 12% / Lead: 15%], Decay [Rhy: 2.0s / Lead: 2.2s], Tone [5.5].