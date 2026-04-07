**Systems Engineer QC-2 Initialization...**
**Target Acquired:** John Mayer Trio Lead Tone (Era: *Try!* Live Album / *Where The Light Is*).
**Physics Profile:** High-headroom Dumble/Two-Rock style clean platform, mid-scooped EQ to leave space for the vocal frequency band, pushed into a smooth, liquid midrange compression using cascaded Klon (Myth Drive) and TS10 (Green 808) topologies. 
**Hardware Output:** QSC CP12 Active PA Speaker (FRFR). Requires aggressive High-Pass and Low-Pass filtering in the Cab block to mimic the mechanical high-frequency roll-off of a traditional 2x12 guitar cabinet and prevent the CP12's internal DSP limiter from clipping on low-end transients.

Before we finalize output levels for the gig, **I must verify: Are your specific guitar pickups Vintage Output, Medium, or High Output?** (The settings below assume Vintage output for the Telecaster and Medium output for the ES-339).

Here is your Split-Bank Matrix build. I have strictly separated the gain-staging and physics constraints for your two requested guitars.

---

### **Configuration 1: Fender Telecaster (Single Coil)**
**Row 1 Target:** Scenes A-D.
**Physics Goal:** Vintage single coils lack the midrange punch needed to drive the Dumble circuit into smooth compression. We must increase the Input Block Gain and use an EQ-8 to add physical "weight" (200Hz) before hitting the drive pedals, while using a heavier Adaptive Gate to suppress 60-cycle hum.

*Input Block Routing:* Gain set to **+2.5dB**.

#### **Table A: Main Signal Chain (Telecaster)**
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 35% *(Right-Click > Assign)* | Noise Red: 20% *(Right-Click > Assign)* | Suppresses 60-cycle hum. Lowered on Lead to allow for Klon/TS sustain to decay naturally. |
| **Pre-FX 1** | Parametric-8 | Band 1 (Low Shelf): +2.5dB @ 200Hz | Band 1 (Low Shelf): +2.5dB @ 200Hz | **Telecaster Body Boost**. Simulates the low-mid magnetic pull of a larger pickup. |
| **Pre-FX 2** | Myth Drive | Bypass: ON *(Right-Click > Assign)* | Bypass: OFF, Gain: 3.0, Treble: 5.5, Out: 6.5 | Transparent edge-of-breakup push. Acts as the first cascading gain stage. |
| **Pre-FX 3** | Green 808 | Bypass: OFF, O.Drive: 2.0, Tone: 4.5 | Bypass: OFF, O.Drive: 6.5, Tone: 5.5 | TS10 Substitute. Rhythm adds subtle mid-hump. Lead increases compression/sustain. |
| **Amp** | D-Cell ODS Clean | Vol: 4.5, Bass: 4.0, Mid: 3.0, Treble: 6.0 | Vol: 5.5 *(Right-Click > Assign)*, Mid: 3.0 | *Two-Rock/SSS Substitute*. Scooped mids. Lead volume increased for +1.5dB output bump. |
| **Cab** | 212 D-Cell | Mic 1: 57 (Pos 0.5, Dist 1.0) | Mic 1: 57 (Pos 0.5) / Mic 2: 121 (Pos 1.5) | G12-65 speakers. LPF at 5.5kHz is critical for FRFR use to remove digital fizz. |
| **Post-FX 1** | Analog Delay | Mix: 12%, Time: 320ms, Fdbk: 20% | Mix: 22% *(Right-Click > Assign)* | BBD-style delay thickens the single-coil lead without stepping on the dry transient. |
| **Post-FX 2** | Spring Reverb | Mix: 18%, Decay: 1.5s | Mix: 18%, Decay: 1.5s | Emulates the 3-spring tank of a Two-Rock Custom Reverb. |

---

### **Configuration 2: Gibson ES-339 (Humbuckers)**
**Row 2 Target:** Scenes E-H.
**Physics Goal:** Humbuckers will immediately overload the input stage of the D-Cell model, resulting in "farty" low-end sag and loss of headroom. We must pad the Input Block, aggressively cut low-end frequencies before the amp, and reduce the Green 808 gain to maintain note clarity in complex chords.

*Input Block Routing:* Gain set to **-4.0dB**.

#### **Table B: Main Signal Chain (ES-339)**
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 15% *(Right-Click > Assign)* | Noise Red: 10% *(Right-Click > Assign)* | Humbuckers are inherently quieter. Minimal gating required to preserve dynamics. |
| **Pre-FX 1** | Parametric-8 | HPF: 120Hz, Band 2: -3.0dB @ 250Hz | HPF: 120Hz, Band 2: -3.0dB @ 250Hz | **Humbucker Mud Cut**. Clears room for the bass guitar and prevents Dumble tube sag. |
| **Pre-FX 2** | Myth Drive | Bypass: ON *(Right-Click > Assign)* | Bypass: OFF, Gain: 2.0, Treble: 6.5, Out: 6.5 | Lower gain than Tele. Treble increased to add pick articulation to the humbuckers. |
| **Pre-FX 3** | Green 808 | Bypass: OFF, O.Drive: 1.0, Tone: 5.5 | Bypass: OFF, O.Drive: 4.5, Tone: 6.0 | Overdrive reduced by 30% compared to Tele to prevent compressed "mush". |
| **Amp** | D-Cell ODS Clean | Vol: 3.5, Bass: 2.5, Mid: 3.5, Treble: 6.5 | Vol: 4.5 *(Right-Click > Assign)*, Bass: 2.5 | Bass dropped significantly to accommodate the natural low-end of the ES-339 semi-hollow. |
| **Cab** | 212 D-Cell | Mic 1: 57 (Pos 0.5, Dist 1.0) | Mic 1: 57 (Pos 0.5) / Mic 2: 121 (Pos 1.5) | Same FRFR routing, but HPF moved up to 110Hz to tighten the 339's resonant frequency. |
| **Post-FX 1** | Analog Delay | Mix: 10%, Time: 320ms, Fdbk: 15% | Mix: 18% *(Right-Click > Assign)* | Lower mix percentage than Telecaster to prevent muddying the humbucker's thicker signal. |
| **Post-FX 2** | Spring Reverb | Mix: 15%, Decay: 1.2s | Mix: 15%, Decay: 1.2s | Reverb mix slightly lowered for humbuckers to retain rhythmic definition. |

---

### **Troubleshooting & Refinement Tree**
If playing at stage volume through the QSC CP12 and the tone feels "Too Distorted/Fuzzy" on the ES-339:
1. **Input Pad:** Verify Input Gain is physically set to -4.0dB or lower.
2. **Amp Gain:** Reduce the D-Cell Volume from 3.5 down to 2.5.
3. **Tube Sag Correction:** If the low strings sound broken or "farty," reduce the Amp Bass parameter by another 1.0 increment.
4. **Output Compensation:** To reclaim lost loudness (SPL) from these cuts, increase the Lane Output Level (far right circle on the grid) by +3.0dB. Do not use the amp volume to regain loudness.

---

### **Session Library (Active Presets)**

**2. Preset Name: "Mayer Trio - Tele/339 Matrix"**
*Target:* John Mayer Trio / Two-Rock SSS Clean w/ Cascading Drives.
*Guitar:* Fender Telecaster (Vintage SC) / Gibson ES-339 (Humbuckers).
*Physics Goal:* High-headroom D-style clean with FRFR optimization for QSC CP12. Mid-scooped base tone pushed by TS/Klon topology.
*Full Configuration:*
*   **Block 1 (Adaptive Gate):** Noise Red [Tele Rhy: 35% / Tele Lead: 20%] / [339 Rhy: 15% / 339 Lead: 10%].
*   **Block 2 (EQ-8):** 
    *   *Tele Settings:* HPF [90Hz], Band 1 [+2.5dB @ 200Hz, Q: 1.0], LPF [6500Hz].
    *   *339 Settings:* HPF [120Hz], Band 2 [-3.0dB @ 250Hz, Q: 1.4], LPF [6500Hz].
*   **Block 3 (Myth Drive):** Gain [Tele: 3.0 / 339: 2.0], Treble [Tele: 5.5 / 339: 6.5], Output [6.5].
*   **Block 4 (Green 808):** Overdrive [Tele Rhy: 2.0, Lead: 6.5 / 339 Rhy: 1.0, Lead: 4.5], Tone [Tele: 4.5 / 339: 6.0], Level [6.5].
*   **Block 5 (Amp - D-Cell ODS Clean):** Vol [Tele Rhy: 4.5, Lead: 5.5 / 339 Rhy: 3.5, Lead: 4.5], Bass [Tele: 4.0 / 339: 2.5], Mid [Tele: 3.0 / 339: 3.5], Treble [Tele: 6.0 / 339: 6.5], Presence [5.0], Master [7.0].
*   **Block 6 (Cab - 212 D-Cell):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 2.0"), Mix [A: 0dB, B: -3dB], HPF [Tele: 90Hz / 339: 110Hz], LPF [5500Hz].
*   **Block 7 (Analog Delay):** Mix [Tele Lead: 22% / 339 Lead: 18%], Time [320ms], Fdbk [Tele: 20% / 339: 15%], Tone [5.0].
*   **Block 8 (Spring Reverb):** Mix [Tele: 18% / 339: 15%], Decay [Tele: 1.5s / 339: 1.2s], HP [150Hz], LP [4000Hz].