This is **QC-2**, Systems Engineer. We are configuring a "High Headroom Texas Funk" preset optimized for **Cortex Control** and monitored through your **QSC CP12**.

To achieve the SRV-style "Glassy Boldness" while maintaining funk percussiveness, we will utilize the **US Super Reverb** model. This circuit provides the 10" speaker "snap" required for funk and the high-voltage headroom for blues leads.

**Critical Calibration Step:** Before we begin, are your pickups **Vintage Output** (lower gain) or **High Output** (modern/hot)? This significantly dictates our Input Block Gain.

### Table A: Main Signal Chain (SRV Funk-Blues)

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -65dB | Thresh: -60dB | High headroom requires a "relaxed" gate to preserve note decay. |
| **Pre-FX (Drive)** | Green 808 | **OFF** (Bypassed) | **ON** | TS808 acts as a mid-hump filter. Sc B/F uses it to "cut." |
| **Amp** | US Super Reverb | Vol: 3.8 / Bright: ON | Vol: 4.5 / Bright: ON | Non-Master circuit. We push the "Vol" for tube saturation, not clipping. |
| **Cab** | 410 US Super PR10 | Mic A: Dyn 57 (Pos 0.4) | Mic B: Ribbon 121 (Pos 0.6) | 4x10 physics: Fast transient response for funk "quack." |
| **EQ (Chameleon)** | Parametric-8 | *See Guitar Config Below* | *See Guitar Config Below* | Compensates for the ES-339 vs. Telecaster physical differences. |
| **Post-FX (Spatial)** | Room Reverb | Mix: 15% / Decay: 1.2s | Mix: 22% / Decay: 1.5s | Simulates a reflective studio floor for "Lenny" style depth. |

---

### Guitar-Specific Output Configurations

#### 1. Target: Fender Telecaster (Single Coils)
*Signal Goal: Add "Weight" and "Body" to thin bridge pickups.*
*   **Input Block Gain:** +2.5dB (to hit the amp front-end harder).
*   **Green 808 (Sc B/F):** Overdrive: 1.5 | Tone: 4.5 | Level: 7.0 (The high level pushes the amp into compression).
*   **Parametric-8 EQ:**
    *   **Band 1 (Body):** Peak at 220Hz (+3.0dB). Adds "thump" to the funk strums.
    *   **Band 8 (Twang):** LPF at 4.8kHz. Tames the ice-pick bridge snap.
*   **Amp EQ:** Bass: 6.5 | Mid: 5.0 | Treble: 7.5.

#### 2. Target: Gibson ES-339 (Humbuckers)
*Signal Goal: Clarity and "Air." Prevent the low-mids from becoming "muddy" or "fuzzed."*
*   **Input Block Gain:** -4.5dB (Essential to keep the humbuckers from clipping the input converter).
*   **Green 808 (Sc B/F):** Overdrive: 0.8 | Tone: 6.5 | Level: 6.0 (More "filter," less "dirt").
*   **Parametric-8 EQ:**
    *   **Band 1 (Body):** High Pass Filter (HPF) at 100Hz. Removes sub-bass "woof."
    *   **Band 4 (Clarity):** Peak at 1.2kHz (+2.0dB). Brings the "funk" click back to the humbucker.
*   **Amp EQ:** Bass: 3.5 | Mid: 4.0 | Treble: 8.5.

---

### Row 1 & 2: Split-Bank Matrix

| Scene | Name | Green 808 | Amp Vol | Reverb Mix | Physics Goal |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **A (Tele)** | Clean Funk | OFF | 3.8 | 12% | Max headroom, percussive "clink." |
| **B (Tele)** | Texas Lead | **ON** | 4.2 | 20% | Mid-forward, singing sustain. |
| **E (339)** | Jazzy Funk | OFF | 3.5 | 15% | Warm but articulate. |
| **F (339)** | Thick Blues | **ON** | 4.0 | 25% | Creamy saturation without mud. |

---

### Troubleshooting & Refinement Tree (QC-2 Protocol)
1.  **Too Distorted on ES-339?** Go to the **Input Block** and toggle the **"Inst Pad"** to ON. This drops the signal by 12dB and is the fastest way to restore high headroom for humbuckers.
2.  **Too Bright on CP12?** The QSC CP12 is an active PA speaker. If the high end is harsh, do not touch the Amp EQ. Instead, go to the **Cab Block** and move the **Dyn 57** microphone further from the center (increase "Position" to 0.6).
3.  **Lacking "Squish"?** Do not add a Compressor block yet. Instead, increase the **Amp Block "Sag"** parameter (if available in the model settings) or increase the **Green 808 "Level"** while decreasing its "Overdrive."

### Session Library (Active Presets)
1. **Preset Name: "Texas Funk High-HR"**
   * **Target:** SRV Clean/Drive Hybrid.
   * **Physics Goal:** High voltage headroom with percussive transient response.
   * **Configuration:** US Super Reverb + 410 US Super PR10, Green 808, Parametric-8 EQ (Compensated).