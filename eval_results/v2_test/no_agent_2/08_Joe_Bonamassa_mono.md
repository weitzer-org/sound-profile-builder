**QC-2 Systems Engineer Terminal. Session Initialized.**

Target Confirmed: Joe Bonamassa — "Sloe Gin" (Circa 2007).
Constraint: ES-335 (Humbuckers) + Multi-Amp Blend Synthesis.
Output Matrix: QSC CP12 FRFR. 

### Deep Research: Analog to QC Mapping
Bonamassa’s tone during the *Sloe Gin* era relied on a massive, complex multi-amp rig. He used a **Marshall Silver Jubilee** for aggressive low-mid crunch and bite, blended simultaneously with a Dumble-style amp (like a **Two-Rock Custom Reverb** or **Carol-Ann**) for smooth, singing midrange, infinite sustain, and articulate attack. 

To prevent your ES-335’s humbuckers from turning this dual-amp synthesis into "mud" through your QSC CP12, we must apply strict High-Pass filtering and leverage the **Headroom Rule**. We will run a parallel split on the grid (Lane 1 into Lane 2) to blend these amplifiers seamlessly.

---

### Row 2: Humbucker Matrix (Scenes E-H)
*Since you are running an ES-335, we will utilize the Humbucker Scene Bank. Use Scene E for the rolling, moody intro, and Scene F for the soaring solo.*

*   **Scene E (Intro/Rhythm):** Clean/Edge-of-breakup. Amps gain backed down, Overdrive bypassed. (Roll your guitar volume down to ~4 for maximum authenticity).
*   **Scene F (Epic Lead):** Full Amp Gain, Overdrive engaged, Delay pushed. 
*   **Scene G (Dry Lead):** Scene F minus Spatial FX.
*   **Scene H (Ambient Swells):** Max Reverb/Delay mix for the atmospheric volume swells.

### Table A: Main Signal Chain (Dual Amp Synthesis)
*(Note: Right-Click parameters in Cortex Control Mac App to assign to Scenes E/F)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55dB | Thresh: -45dB | Higher threshold on Lead to clamp hum from the dual-amp gain stacking. |
| **Pre-FX (Drive)** | Green 808 | *Bypassed* | Drive: 1.5, Level: 8.0, Tone: 6.5 | Trims 335 sub-bass; pushes upper-mids into the amp inputs for soaring sustain. |
| **Amp 1 (Top Lane)** | Brit Silver 2555 | Input Gain: 3.5, Rhythm Clip: On, Bass: 3.0 | Input Gain: 6.5, Rhythm Clip: Off, Bass: 3.5 | Marshall bite. Low bass keeps the 335 from "farting out" the low end. |
| **Cab 1 (Top Lane)** | 412 Brit V30 | Mic: Dyn 57, Pos: 1.5, HPF: 100Hz | Mic: Dyn 57, Pos: 1.5, HPF: 100Hz | Aggressive upper-mid cut to balance the dark amp. |
| **Amp 2 (Bot Lane)** | D-Cell ODS | OD Drive: 3.0, OD Level: 4.5, Presence: 5.0 | OD Drive: 7.0, OD Level: 6.5, Presence: 5.5 | Dumble-style smooth clipping. Provides the thick, vocal sustain Bonamassa is known for. |
| **Cab 2 (Bot Lane)** | 212 D-Cell | Mic: Ribbon 121, Pos: 2.0, LPF: 5000Hz | Mic: Ribbon 121, Pos: 2.0, LPF: 5000Hz | Ribbon mic adds lower-mid weight; LPF ensures the D-style amp doesn't get fizzy. |
| **Summing Mixer** | Lane Output Level | Amp 1: 0dB, Amp 2: 0dB | Amp 1: -1.0dB, Amp 2: +1.5dB | In Lead mode, the D-Cell is pushed slightly louder for vocal soloing prominence. |
| **Post-EQ** | Parametric-8 | Band 1 (200Hz): -2.0dB | Band 1 (200Hz): -1.5dB | *Chameleon Strategy*: Tames the ES-335 neck-pickup woof before hitting spatial FX. |
| **Post-FX (Delay)**| Analog Delay | Mix: 12%, Time: 420ms | Mix: 22%, Time: 420ms, Fdbk: 35% | Analog repeats sit *behind* the dry signal to widen the dual amps without clutter. |
| **Post-FX (Verb)** | Plate Reverb | Mix: 15%, Decay: 1.5s | Mix: 25%, Decay: 2.2s | Emulates the expansive studio space on the studio recording. |

---

### Pickup Compensation & Troubleshooting Tree
If your ES-335 has modern/high-output humbuckers, the amps may sound "Too Compressed" or "Fuzzy" through the QSC CP12. Follow this strict order of operations:
1. **Input Pad:** Go to the global Input block and lower the Gain to **-3.0dB** or **-6.0dB**. This immediately restores headroom to the dual-amp front ends.
2. **Amp Sag/Bass:** If the neck pickup sounds "broken" on the low-E string, lower the **Bass** parameter on the *D-Cell ODS* from 5.0 to 3.0. 
3. **Volume Loss Compensation:** If padding the input makes the CP12 too quiet, **DO NOT raise Amp Gain**. Raise the final **Lane Output Level** on the QC grid to push more clean voltage to the QSC.

---

### Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
*(Retained from previous memory)*

**2. Preset Name: "Sloe Gin Blend - ES335"**
Target: Joe Bonamassa (2007).
Guitar: ES-335 (Humbuckers) w/ Pick.
Physics Goal: Synthesize a Marshall/Dumble split rig without phase cancellation or low-end blowout on an active PA speaker.
Full Configuration:
*   Block 1 (Global Input): Gate Thresh [Rhy: -55dB / Lead: -45dB].
*   Block 2 (Green 808): Bypass [Rhy: ON / Lead: OFF], Drive [1.5], Level [8.0], Tone [6.5].
*   Block 3 (Amp 1 - Brit Silver 2555): Input Gain [3.5 / 6.5], Bass [3.0 / 3.5], Mid [6.0], Treb [5.5], Pres [4.0], Vol [6.0].
*   Block 4 (Cab 1 - 412 Brit V30): Mic 1 (Dyn 57, Pos 1.5, HPF 100Hz, LPF 6000Hz).
*   Block 5 (Amp 2 - D-Cell ODS): OD Drive [3.0 / 7.0], OD Level [4.5 / 6.5], Bass [4.5], Mid [6.5], Treb [6.0].
*   Block 6 (Cab 2 - 212 D-Cell): Mic 1 (Ribbon 121, Pos 2.0, HPF 90Hz, LPF 5000Hz).
*   Block 7 (EQ-8): Band 1 Peak [200Hz, -2.0dB / -1.5dB], Band 8 LPF [5500Hz]. 
*   Block 8 (Analog Delay): Mix [12% / 22%], Time [420ms], Feedback [25% / 35%], Tone [4.0].
*   Block 9 (Plate Reverb): Mix [15% / 25%], Decay [1.5s / 2.2s], Pre-Delay [20ms], Low Cut [150Hz].