**QC-2 Systems Engineer**
**Target Acquired:** Slash - "Sweet Child O' Mine" (Intro)
**Analog DNA:** S.I.R. Stock #39 Modified Marshall JCM800 2203, Marshall 1960B 4x12 (Celestion Vintage 30s), Seymour Duncan Alnico II Pro (Neck pickup, tone knob rolled back).
**Hardware Profile:** QSC CP12 (Active FRFR PA Speaker). High-gain Marshall profiles can output severe high-frequency fizz through PA tweeters; steep LPF (Low Pass Filtering) is required for this build. 

Below is your Split-Bank execution. As requested, we are targeting TWO distinct pickup configurations. 

---

### **Part 1: Gibson ES-339 Humbucker Configuration (Scenes E-H)**
*Targeting Alnico II / PAF-style pickups. The ES-339 has inherent midrange bark. We will tame the high-end to mimic Slash rolling his neck pickup tone knob down to ~4.*

**Organization Matrix:**
*   **Scene E (Rhythm):** Bridge Pickup (Full Tone) - Standard GNR Crunch. Output Level: Base.
*   **Scene F (Lead):** Neck Pickup (SCOM Intro) - Singing Sustain, mid-pushed. Output Level: +1.5dB.
*   **Scene G (Dry):** Same as F, but all spatial FX bypassed.
*   **Scene H (Solo):** Bridge Pickup - High gain, longer delay trail for the main solo.

**Table A: Main Signal Chain (Gibson ES-339)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 35%<br>Thresh: -55dB | Noise Red: 20%<br>Thresh: -62dB | % used. Lower reduction/threshold on Lead to preserve the singing, infinite sustain of the neck pickup without sputtering. |
| **Pre-FX (Mod)** | Parametric-8 | Gain: +2.0dB<br>Band 6: 0dB | Gain: +4.0dB<br>Band 6: +3dB (800Hz) | *S.I.R. Mod Simulation:* Pushes the amp's front end exactly like an extra tube gain stage. Boosts mids for the vocal lead tone. |
| **Amp** | Brit 800 | Preamp: 6.5<br>Master: 5.5<br>Treble: 6.0<br>Pres: 5.0 | Preamp: 8.5<br>Master: 5.5<br>Treble: 4.0<br>Pres: 3.5 | SCOM intro relies on Preamp saturation and lower Treble/Presence to achieve the "flute-like" neck pickup sound. |
| **Cab** | 412 Brit V30 | Mic A: Dyn 57 (Pos 1.5, Dist 1")<br>Mic B: Ribbon 121 (Pos 0.5, Dist 3") | Mic A: Dyn 57 (Pos 1.5, Dist 1")<br>Mic B: Ribbon 121 (Pos 0.5, Dist 3") | Vintage 30s cut through mixes. Ribbon mic adds bottom-end warmth. LPF set globally at 6.0kHz to tame the QSC CP12 tweeter. |
| **Post-FX** | Analog Delay | Mix: 10%<br>Time: 350ms<br>Fdbk: 15% | Mix: 20%<br>Time: 420ms<br>Fdbk: 25% | Studio thickness. Increased mix and time on the Lead scene to add spatial depth to single notes. |
| **Post-FX** | Plate Reverb | Mix: 12%<br>Decay: 1.5s | Mix: 15%<br>Decay: 1.8s | Replicates the 1980s Sound City studio ambient space. |

---

### **Part 2: Fender Telecaster Single Coil Configuration (Scenes A-D)**
*Targeting vintage/medium output single coils. To achieve Slash's tone on a Telecaster, we must artificially generate humbucker body (200Hz), completely strip away single-coil ice-pick frequencies (3kHz-5kHz), and aggressively push the input gain to compensate for the lower mV output of the pickups.*

**Organization Matrix:**
*   **Scene A (Rhythm):** Bridge Pickup - Adjusted for single coil crunch. Output Level: Base.
*   **Scene B (Lead):** Neck Pickup - SCOM Intro adapted for Tele. Output Level: +1.5dB.
*   **Scene C (Dry):** Same as B, but all spatial FX bypassed.
*   **Scene D (Solo):** Bridge Pickup - High gain, longer delay.

**Table B: Main Signal Chain (Fender Telecaster)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 50%<br>Thresh: -45dB | Noise Red: 35%<br>Thresh: -50dB | Single coils inherently capture more 60-cycle hum. Aggressive % reduction required, especially when hitting a JCM800. |
| **Pre-FX (EQ)** | Parametric-8 | Input Level: +3.0dB<br>Band 1: +2dB (200Hz) | Input Level: +6.0dB<br>Band 1: +4dB (250Hz)<br>LPF: 4000Hz | *The Chameleon Strategy:* Simulates a humbucker. Massive input gain push. Band 1 adds "wood/body". Steep LPF kills the Tele "twang/spank" entirely. |
| **Amp** | Brit 800 | Preamp: 7.5<br>Master: 5.5<br>Treble: 5.0<br>Pres: 4.0 | Preamp: 10.0<br>Master: 5.5<br>Treble: 3.0<br>Pres: 2.0 | Tele neck pickups need maximum Preamp gain to simulate humbucker sustain. Treble and Presence aggressively choked. |
| **Cab** | 412 Brit V30 | Mic A: Dyn 57 (Pos 2.0, Dist 1")<br>Mic B: Ribbon 121 (Pos 0.0, Dist 2") | Mic A: Dyn 57 (Pos 2.0, Dist 1")<br>Mic B: Ribbon 121 (Pos 0.0, Dist 2") | Mic A pushed further off-center (Pos 2.0) to physically darken the speaker capture for the brighter Telecaster. |
| **Post-FX** | Analog Delay | Mix: 12%<br>Time: 350ms<br>Fdbk: 15% | Mix: 22%<br>Time: 420ms<br>Fdbk: 25% | Same spatial logic as the humbucker build. |
| **Post-FX** | Plate Reverb | Mix: 12%<br>Decay: 1.5s | Mix: 15%<br>Decay: 1.8s | Replicates the 1980s studio ambient space. |

---

### **Troubleshooting & Refinement Tree (QSC CP12 specifics)**
*   **If the intro tone sounds "thin" or "frizzy" on the ES-339:** Roll your physical guitar tone knob down to 3 or 4. If it's still harsh, lower the Cab Block LPF down to `5500Hz`. The QSC CP12's horn is unforgiving with Marshall presence. 
*   **If the Telecaster lacks sustain on the bends:** Do *not* add a compressor block. Increase the Pre-FX Parametric-8 Input Level by another `+2.0dB` to slam the Brit 800's preamp harder.
*   **If the amp sounds "broken/farty" on palm mutes:** Reduce the `Bass` parameter on the Brit 800 block by 1.5 increments. High-gain Marshalls require low-end to be shaped by the 4x12 Cab block, not the amp EQ.

---

### **Session Library (Active Presets)**

**2. Preset Name: "Sweet Child S.I.R - ES339/Tele"**
*   **Target:** Slash / Guns N' Roses (1987).
*   **Guitar:** ES-339 (Humbuckers) / Telecaster (Single Coils).
*   **Physics Goal:** Replicate the S.I.R. #39 JCM800 mod via pre-EQ pushing, manage QSC CP12 tweeter response via aggressive LPF, adapt tone per pickup geometry.
*   **Full Configuration (ES-339 Scene F):**
    *   **Block 1 (Adaptive Gate):** Noise Red [20%], Thresh [-62dB].
    *   **Block 2 (EQ-8):** Gain [+4.0dB], Band 6 [+3.0dB @ 800Hz], High Shelf [-3.0dB @ 4000Hz].
    *   **Block 3 (Amp - Brit 800):** Preamp [8.5], Master [5.5], Bass [4.5], Mid [7.0], Treble [4.0], Presence [3.5], Output [+1.5dB].
    *   **Block 4 (Cab - 412 Brit V30):** Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.5, Dist 3.0"), HPF [85Hz], LPF [6000Hz]. Mix [A: 0dB, B: -2dB].
    *   **Block 5 (Analog Delay):** Mix [20%], Time [420ms], Fdbk [25%], Sync [Off].
    *   **Block 6 (Plate Reverb):** Mix [15%], Decay [1.8s], Pre-Delay [15ms].
*   **Full Configuration (Telecaster Scene B - Delta parameters):**
    *   **Block 1:** Noise Red [35%], Thresh [-50dB].
    *   **Block 2 (EQ-8):** Input [+6.0dB], Band 1 [+4.0dB @ 250Hz], LPF [4000Hz].
    *   **Block 3 (Amp - Brit 800):** Preamp [10.0], Treble [3.0], Presence [2.0].
    *   **Block 4 (Cab):** Mic A (Pos 2.0).