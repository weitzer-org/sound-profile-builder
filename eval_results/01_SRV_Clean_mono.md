**Quad Cortex Systems Engineer (QC-2) - Initialization Complete.**

To achieve a Stevie Ray Vaughan (SRV) clean funk blues tone with massive headroom, we are utilizing the **US TWN Vibrato** (based on a '65 Fender Twin Reverb) paired with a **Green 808** (Ibanez TS808). The physics goal here is to establish a pristine, high-wattage clean foundation that does not compress under heavy right-hand strumming, allowing the Green 808 to provide the characteristic asymmetrical clipping and 720Hz mid-hump when engaged for leads. 

Because you are running into a **QSC CP12 Active PA Speaker**, we must aggressively manage the low-end. The Twin Reverb circuit is notoriously bass-heavy, which will cause the QSC's DSP limiters to pump if not properly high-passed. 

Per protocol, I have engineered a Split-Bank configuration accommodating both the **Fender Telecaster (Single Coils)** and the **Gibson ES-339 (Humbuckers)** to account for the massive variance in output impedance and frequency response.

### 🎸 Gain Staging & Pickup Compensation Strategy
**For Fender Telecaster (Single Coil):**
*   **Physics Goal:** Retain the "glassy" top end but add lower-mid weight to simulate SRV's heavy string gauge and overwound Stratocaster pickups. The Green 808 will run with higher Level and lower Drive to hit the amp's front-end hard.
*   **Input Block Gain:** `0.0dB`

**For Gibson ES-339 (Humbuckers):**
*   **Physics Goal:** Humbuckers will inherently push the Twin into early breakup, destroying the "funk clean" headroom. We must artificially lower the guitar's output and scoop the 400Hz range to prevent the Green 808 from turning into a muddy fuzz. 
*   **Input Block Gain:** `-4.5dB` (Crucial for maintaining clean headroom).

---

### 🎛 Split-Bank Matrix (Assign via Scene Right-Click)
*   **Row 1 (Telecaster Single Coils):** Scene A (Rhythm - Clean), Scene B (Lead - Pushed 808).
*   **Row 2 (Gibson ES-339 Humbuckers):** Scene E (Rhythm - Clean), Scene F (Lead - Pushed 808).

### Table A: Main Signal Chain Configuration
*Note: Parameters marked with brackets `[A/B | E/F]` indicate Scene-specific assignments. Right-click these parameters in Cortex Control to map them.*

| Block Category | Model Name | Telecaster (Sc A / B) | ES-339 (Sc E / F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -65dB | Thresh: -60dB | ES-339 requires tighter gating due to higher output noise floor. |
| **Pre-FX (EQ)** | Parametric-8 | B1 (Peak): +2dB @ 250Hz<br>B8 (LPF): 6000Hz | B1 (Peak): -3dB @ 400Hz<br>B8 (LPF): 4500Hz | **Chameleon Strategy:** Thickens the Telecaster; scoops the "honk" from the ES-339 humbuckers. |
| **Pre-FX (Drive)** | Green 808 | Drive: 2.0 / 4.0<br>Tone: 6.5<br>Level: 5.0 / 8.5 | Drive: 1.0 / 2.5<br>Tone: 7.5<br>Level: 5.0 / 7.0 | SRV Trick: High level, low drive. Humbuckers require less drive and more tone (treble) to cut through. |
| **Amp** | US TWN Vibrato | Vol: 4.5<br>Bass: 4.0<br>Mid: 5.5<br>Treble: 6.5 | Vol: 3.5<br>Bass: 3.0<br>Mid: 4.0<br>Treble: 7.0 | '65 Twin has no Master Volume. Lower volume and bass on the ES-339 ensures the amp stays in the high-headroom clean zone. |
| **Cab** | 212 US TWN | Mic A: Dyn 57 (Pos 1.0)<br>Mic B: Rib 121 (Pos 2.5)<br>HPF: 85Hz | Mic A: Dyn 57 (Pos 1.5)<br>Mic B: Cond 414 (Pos 3.0)<br>HPF: 100Hz | 57 provides the bite. Tele gets the Ribbon 121 for warmth. ES-339 gets the Condenser 414 for clarity. HPF protects the QSC CP12. |
| **Post-FX** | Spring Reverb | Mix: 15% / 15%<br>Decay: 2.5s | Mix: 12% / 12%<br>Decay: 2.0s | Classic Fender spatial depth. ES-339 gets a slightly shorter decay to avoid low-end wash. |
| **Output** | Lane Output Level | Level: 0.0dB / +1.5dB | Level: +2.0dB / +3.5dB | ES-339 lane output is raised to compensate for the -4.5dB Input Gain reduction used to save headroom. |

---

### 🔧 Troubleshooting & Refinement Tree
If you load this preset and the ES-339 still sounds **"Too Distorted"** or lacks headroom:
1.  **Input Pad:** Verify your Input Block Gain is at `-4.5dB` or lower. If your 339 has modern/hot PAFs, you may need to drop this to `-6.0dB`.
2.  **Amp Gain:** Reduce the US TWN Vol from `3.5` down to `2.5`.
3.  **Output Compensation:** Never use a compressor to fix gain issues. Go to the final Lane Output Level block on the right side of the grid and raise the output by `+2.0dB` to restore overall loudness to the QSC speaker.

---

### 💾 Session Library (Active Presets)
**2. Preset Name:** "Texas Flood - Dual Rig"
**Target:** Stevie Ray Vaughan (Clean Funk / High Headroom 808).
**Guitars:** Fender Telecaster (Single Coil) & Gibson ES-339 (Humbucker).
**Physics Goal:** Pristine Twin Reverb headroom with asymmetric TS-style clipping. Preventing QSC low-end buildup.
**Full Configuration:**
*   **Block 1 (Global Input):** Level [Tele: 0.0dB / ES339: -4.5dB], Gate Thresh [Tele: -65dB / ES339: -60dB].
*   **Block 2 (EQ-8):** [Tele] Peak B1 +2.0dB @ 250Hz, LPF 6000Hz. [ES339] Peak B1 -3.0dB @ 400Hz, LPF 4500Hz. 
*   **Block 3 (Green 808):** Drive [Tele Rhy: 2.0, Lead: 4.0 / ES339 Rhy: 1.0, Lead: 2.5], Tone [Tele: 6.5 / ES339: 7.5], Level [Tele Rhy: 5.0, Lead: 8.5 / ES339 Rhy: 5.0, Lead: 7.0].
*   **Block 4 (Amp - US TWN Vibrato):** Vol [Tele: 4.5 / ES339: 3.5], Bass [Tele: 4.0 / ES339: 3.0], Mid [Tele: 5.5 / ES339: 4.0], Treble [Tele: 6.5 / ES339: 7.0].
*   **Block 5 (Cab - 212 US TWN):** Mic A (Dyn 57), Mic B [Tele: Rib 121 / ES339: Cond 414]. HPF [Tele: 85Hz / ES339: 100Hz]. Mix [A: 0dB, B: -3dB].
*   **Block 6 (Spring Reverb):** Mix [Tele: 15% / ES339: 12%], Decay [Tele: 2.5s / ES339: 2.0s]. 
*   **Lane Output:** Level [Tele Rhy: 0.0dB, Lead: +1.5dB / ES339 Rhy: +2.0dB, Lead: +3.5dB].