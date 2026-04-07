**QC-2 Systems Engineer Online.**
**Target Verified:** Southern Rock Slide Style (Allman / Skynyrd dynamics). 
**Physics Goal:** Extreme natural compression and singing sustain via a cranked vintage American Tweed circuit. 

To achieve "singing sustain" without harsh slide clatter, we must manage the resonant frequencies of the glass/brass slide against the physical limitations of a Tweed circuit. Tweeds lack negative feedback and have loose low-end; when you push them with humbuckers, they "fart out" or completely collapse in the bass frequencies. 

To satisfy the **Multi-Guitar Protocol (Rule 9)** and the **Split-Bank Matrix (Rule 4)**, this preset is engineered to house both your Gibson ES-339 and a Fender Telecaster in a single session. We will use a dedicated Grid Gain Block and a Parametric EQ to instantly adapt the physics for either guitar via Scene selection.

***

### 🎸 MULTI-GUITAR GAIN STAGING & COMPENSATION

**Profile 1: Gibson ES-339 (Humbuckers) — Assigned to Row 2 (Scenes E-H)**
*   **Physics Challenge:** Humbuckers hitting a cranked Tweed create excessive low-mid mud, killing sustain and replacing it with fuzz.
*   **QC Compensation:** The Grid Gain block is dropped to -4.0dB. The EQ-8 block cuts bass heavily at 150Hz. This tightens the humbuckers to allow the Green 808 to push the preamp tubes purely in the midrange, resulting in infinite, singing slide sustain.

**Profile 2: Fender Telecaster (Single Coils) — Assigned to Row 1 (Scenes A-D)**
*   **Physics Challenge:** Telecasters lack the output to push the Tweed into heavy sustain, and the bridge pickup will make the slide sound piercing (ice-pick effect).
*   **QC Compensation:** The Grid Gain block is boosted to +2.0dB. The EQ-8 block adds a +3.0dB shelf at 200Hz for body, and applies an aggressive Low Pass Filter (LPF) at 4.2kHz to physically roll off the harsh glass/metal scrape of the slide.

***

### 🎛️ SIGNAL CHAIN & SCENE MATRIX (FRFR/QSC CP12 Optimized)

*Note: The vintage '57 Twin does not have a Master Volume. To increase your overall room loudness (SPL) without altering the drive tone, you MUST use the Lane Output Level at the end of the grid.*

#### Table A: Main Signal Chain
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input 1 | Thresh: -60dB | Thresh: -65dB | Slide playing is noisy. Relaxing the gate on Lead prevents choking the singing tail of the sustain. |
| **Pre-FX (Gain)** | Utility Gain | **Sc A (Tele):** +2.0dB<br>**Sc E (339):** -4.0dB | **Sc B (Tele):** +2.0dB<br>**Sc F (339):** -4.0dB | Levels the pickup output to hit the overdrive and amp with identical voltage pressure. |
| **Pre-FX (Drive)** | Green 808 | Overdrive: 2.0<br>Tone: 6.0<br>Level: 6.5 | Overdrive: 6.5<br>Tone: 5.5<br>Level: 8.0 | *TS-style circuit.* Cuts bass natively. High level/drive on Lead heavily compresses the signal for infinite slide sustain. |
| **Amp** | US TWN 57 Jumped | Vol Bright: 6.0<br>Vol Norm: 5.0<br>Bass: 2.5<br>Treble: 6.5<br>Presence: 5.0 | Vol Bright: 8.5<br>Vol Norm: 5.0<br>Bass: 2.0<br>Treble: 6.0<br>Presence: 4.5 | *Vintage Tweed Twin.* Bass must be remarkably low (2.0) to prevent tube sag collapse. Jumped channels add low-mid thickness. |
| **Cab** | 212 US TWN 57 | Mic A: Dyn 57 (Pos 0.5, Dist 1")<br>Mic B: Rib 121 (Pos 1.5, Dist 3") | Mix: Mic A (0dB), Mic B (-2dB) | 57 captures the bite of the slide. Ribbon 121 (Off-center) captures the cabinet resonance and smooths the harsh treble. |
| **Post-FX (EQ)** | Parametric-8 | **Sc A:** LPF 4.5kHz, +3dB @ 200Hz<br>**Sc E:** LPF 6kHz, -2dB @ 150Hz | **Sc B:** LPF 4.2kHz, +3dB @ 200Hz<br>**Sc F:** LPF 5.5kHz, -2dB @ 150Hz | *The Chameleon Strategy.* Physically morphs the amp response to suit Single Coils (Row 1) or Humbuckers (Row 2). |
| **Post-FX (Delay)**| Analog Delay | Mix: 0% *(Bypassed)* | Mix: 25%<br>Time: 380ms<br>Fdbk: 30% | Analog bucket-brigade repeats sit *behind* the dry signal, thickening the slide tone without washing it out. |
| **Post-FX (Verb)** | Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 22%<br>Decay: 1.8s | Replicates the acoustic reflections of vintage Southern Rock live rooms (e.g., Fillmore East). |

***

### 🔧 TROUBLESHOOTING & REFINEMENT TREE

If the sustain feels "Too Distorted," "Too Fuzzy," or "Farty" through your QSC CP12, follow this exact order of operations:
1.  **Input Pad:** Lower the Utility Gain block by another -2.0dB. The amp is receiving too much voltage.
2.  **Amp Sag Control:** Go to the US TWN 57 Jumped amp block and lower the **Bass** to 1.0. Cranked Tweeds generate low-end fuzz naturally; cutting bass removes the "fart."
3.  **Slide Attack Harshness:** If the physical slide hitting the strings is deafening, go to the Parametric-8 block and lower the High Cut (LPF) from 4.2kHz down to 3.5kHz. 
4.  **Output Compensation:** If step 1 or 2 causes a drop in volume, **DO NOT** turn the Amp Volumes back up. Go to the far right of the grid and raise the Lane Output Level.

***

### 💾 SESSION REGISTRY PROTOCOL
*Appending to Active Memory...*

**Session Library (Active Presets)**
2. Preset Name: "Southern Slide - Dual Profile"
Target: Duane Allman / Gary Rossington (1971).
Guitar: Gibson ES-339 (Humbuckers) & Fender Telecaster (Single Coils) w/ Heavy Slide.
Physics Goal: Singing compression, aggressive bass cut to prevent Tweed sag collapse, high-headroom projection.
Full Configuration:
Block 1 (Utility Gain): Sc A/B [+2.0dB], Sc E/F [-4.0dB].
Block 2 (Green 808): Overdrive [Rhy: 2.0 / Lead: 6.5], Tone [Rhy: 6.0 / Lead: 5.5], Level [Rhy: 6.5 / Lead: 8.0].
Block 3 (Amp - US TWN 57 Jumped): Vol Bright [Rhy: 6.0 / Lead: 8.5], Vol Normal [5.0], Bass [Rhy: 2.5 / Lead: 2.0], Treble [Rhy: 6.5 / Lead: 6.0], Presence [Rhy: 5.0 / Lead: 4.5].
Block 4 (Cab - 212 US TWN 57): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 3.0"), Mix [A: 0dB, B: -2dB].
Block 5 (EQ-8): Telecaster (Sc A/B) [Band 1: +3.0dB @ 200Hz, LPF: 4.2kHz]. ES-339 (Sc E/F) [Band 1: -2.0dB @ 150Hz, LPF: 5.5kHz].
Block 6 (Analog Delay): Mix [Rhy: 0% / Lead: 25%], Time [380ms], Fdbk [30%].
Block 7 (Plate Reverb): Mix [Rhy: 15% / Lead: 22%], Decay [Rhy: 1.2s / Lead: 1.8s], HP [150Hz], LP [4000Hz].