**Quad Cortex Systems Engineer (QC-2) - Initialization Complete.**

Target Acquired: **Van Halen "Brown Sound" (Early Era)**
Analog Specs: 1968 Marshall Super Lead 100W (Plexi), Variac set to ~90V (Sag/Brown-out), Celestion Greenbacks, MXR Phase 90, EMT 140 Plate Reverb.
Cortex Output Target: QSC CP12 (Flat-response active PA speaker). 

### Physics & Hardware Protocol
Because the QSC CP12 has a dedicated high-frequency tweeter, a dimed Plexi model will sound inherently brighter and harsher than a traditional guitar cabinet in the room. The "Variac" effect—which lowers the operating voltage of the amplifier, causing earlier distortion, severe power-tube sag, and a "chewier" midrange—must be simulated in the QC. Since the CorOS Plexi models do not have a dedicated "Voltage" knob, we will simulate the Variac by diming the amp's Input Volumes, heavily reducing the Amp Block Bass (to prevent farty low-end sag), and applying a post-amp EQ to focus the midrange and roll off the fizz.

As requested, below are two completely independent signal chains and gain staging strategies tailored for your specific guitars.

---

### CONFIGURATION 1: Fender Telecaster (Single Coils)
**Bank Mapping:** Row 1 (Scenes A-D)
*Scene A: Rhythm | Scene B: Lead | Scene C: Dry/Comping | Scene D: Ambient/Phase*

**Input Gain Logic:** Telecaster single coils lack the high-output push required to easily overdrive a Plexi. 
**Action:** Go to the Global Input Block (Circle 1) and set **In 1 Gain to +3.0dB**. 

#### Table A.1: Telecaster Signal Chain (Scenes A-D)
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (In 1) | Thresh: -55.0dB *(Right-Click > Assign)* | Thresh: -50.0dB *(Right-Click > Assign)* | Tighter gate on Lead to kill single-coil 60-cycle hum when not playing. |
| **Pre-FX 1** | Phaser 95 | Bypass: ON *(Right-Click > Assign)* | Bypass: OFF *(Right-Click > Assign)* | Script-style phaser. Rate set to 25% for the classic "Eruption" slow sweep. |
| **Pre-FX 2** | Parametric-8 | Band 1 (Low Shelf): +3.5dB @ 200Hz | Band 1: +3.5dB @ 200Hz | **Chameleon Strategy:** Adds massive low-mid body to thin single-coils to mimic a humbucker pushing the amp. |
| **Amp** | Brit Plexi 100 Jump | Vol Norm: 8.5, Vol High: 8.0 | Vol Norm: 9.5, Vol High: 8.5 *(Right-Click > Assign)* | **No Master Volume.** Pushing the inputs simulates the Variac overdrive. Bass set to 3.0 to prevent sagging/farting. Mid: 7.5, Treb: 6.0, Pres: 6.5. |
| **Cab** | 412 Brit Green | Mic: Dyn 57, Pos: 1.5, Dist: 1.0" | Mic: Dyn 57, Pos: 1.5, Dist: 1.0" | Off-center positioning tames the aggressive top-end bite of the Telecaster bridge pickup through the QSC CP12. |
| **Post-EQ** | Parametric-8 | Band 8 (LPF): 4.5kHz, Band 4: +2dB @ 800Hz | Band 8 (LPF): 4.5kHz, Band 4: +4dB @ 800Hz | **Variac Simulation:** The LPF mimics the rounded-off highs of the low voltage, while the 800Hz boost provides the "chewy" midrange. |
| **Post-FX** | Plate Reverb | Mix: 12%, Decay: 1.8s | Mix: 22%, Decay: 2.5s *(Right-Click > Assign)* | Emulates the Sunset Sound EMT 140 plate reverb. Pre-delay set to 20ms to preserve pick attack. |

*To increase overall loudness for this preset without adding distortion, use the Lane Output Level slider at the far right of the grid.*

---

### CONFIGURATION 2: Gibson ES-339 (Humbuckers)
**Bank Mapping:** Row 2 (Scenes E-H)
*Scene E: Rhythm | Scene F: Lead | Scene G: Dry/Comping | Scene H: Ambient/Phase*

**Input Gain Logic:** The PAF-style humbuckers in the ES-339 hit the front of the Quad Cortex significantly harder, which can cause digital artifacts before the amp block.
**Action:** Go to the Global Input Block (Circle 1) and set **In 1 Gain to -1.5dB**. 

#### Table A.2: ES-339 Signal Chain (Scenes E-H)
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (In 1) | Thresh: -62.0dB *(Right-Click > Assign)* | Thresh: -58.0dB *(Right-Click > Assign)* | Humbuckers are quieter at idle; lower threshold allows more sustain/bloom to bleed through the gate. |
| **Pre-FX 1** | Phaser 95 | Bypass: ON *(Right-Click > Assign)* | Bypass: OFF *(Right-Click > Assign)* | Used for the "Ain't Talkin' 'Bout Love" swirling lead tone. Rate 25%. |
| **Pre-FX 2** | Parametric-8 | Band 1: 0.0dB (Flat) | Band 1: 0.0dB (Flat) | ES-339 inherently provides the necessary low-mid push; no body boost required here. |
| **Amp** | Brit Plexi 100 Jump | Vol Norm: 6.5, Vol High: 7.0 | Vol Norm: 7.5, Vol High: 8.5 *(Right-Click > Assign)* | **No Master Volume.** Volumes are backed off compared to the Telecaster because the humbuckers will compress and distort the Plexi model much earlier. |
| **Cab** | 412 Brit Green | Mic: Dyn 57, Pos: 1.0, Dist: 1.5" | Mic: Dyn 57, Pos: 1.0, Dist: 1.5" | Positioned closer to the center cap (Pos 1.0) than the Telecaster, allowing the humbucker's natural top-end to cut through the mix. |
| **Post-EQ** | Parametric-8 | Band 8 (LPF): 6.0kHz, Band 4: +1.5dB @ 800Hz | Band 8 (LPF): 6.0kHz, Band 4: +3.0dB @ 800Hz | **Variac Simulation:** LPF is set higher (6.0kHz) because humbuckers are naturally darker than single coils. |
| **Post-FX** | Plate Reverb | Mix: 10%, Decay: 1.8s | Mix: 20%, Decay: 2.5s *(Right-Click > Assign)* | High-pass filter inside the Reverb block set to 200Hz to prevent the 339's low-end from making the reverb muddy. |

---

### Troubleshooting & Refinement Tree
If the tone sounds incorrect through your QSC CP12, follow this strict sequence:
1. **Too Distorted/Fuzzy (Clipping)?**
   * Lower the Input Block Gain by an additional -3.0dB. This mimics rolling off the volume knob on your guitar, cleaning up the front-end of the Plexi.
2. **Too "Farty" or "Broken" in the low-end?**
   * *Do not lower the Amp Volume.* Reduce the Amp Block "Bass" parameter in 0.5 increments. Plexi circuits are notoriously flubby when cranked; reducing bass tightens the power tube sag. 
3. **Too Harsh/Piercing through the PA speaker?**
   * Increase the "Position" parameter on the Cab block (move it from 1.0 closer to 2.0 or 3.0). This moves the virtual mic further to the edge of the speaker cone, organically rolling off high frequencies.
4. **Volume Dropped Too Much?**
   * Raise the Lane Output Level on the far right of the grid. **Never** use a compressor block to fix a volume issue; it will destroy the Variac dynamics.

---

### Session Library (Active Presets)

**2. Preset Name: "Brown Sound - Multi-Strat"**
*   **Target:** Van Halen I Era (1978).
*   **Guitars:** Fender Telecaster (SC) / Gibson ES-339 (HB).
*   **Physics Goal:** Simulate Variac voltage drop (sag/mid-push) into a jumped Plexi out to a flat-response PA speaker.
*   **Full Configuration:**
    *   **Block 1 (Global Gate):** Thresh [Tele: -55dB Rhy / -50dB Lead] | [339: -62dB Rhy / -58dB Lead].
    *   **Block 2 (Phaser 95):** Rate [25%], Sync [Off]. Bypass assigned to Scenes.
    *   **Block 3 (Parametric-8 Pre):** Band 1 Low Shelf [Tele: +3.5dB @ 200Hz | 339: 0.0dB @ 200Hz]. Band 2-8 [Flat].
    *   **Block 4 (Amp - Brit Plexi 100 Jump):** Vol Norm [Tele: 8.5 Rhy, 9.5 Lead | 339: 6.5 Rhy, 7.5 Lead], Vol High [Tele: 8.0 Rhy, 8.5 Lead | 339: 7.0 Rhy, 8.5 Lead], Bass [3.0], Mid [7.5], Treble [6.0], Presence [6.5].
    *   **Block 5 (Cab - 412 Brit Green):** Mic A (Dyn 57), Pos [Tele: 1.5 | 339: 1.0], Dist [Tele: 1.0" | 339: 1.5"], Center [0].
    *   **Block 6 (Parametric-8 Post):** LPF [Tele: 4500Hz | 339: 6000Hz], Band 4 Peak [+2.0dB Rhy, +4.0dB Lead @ 800Hz, Q: 1.5].
    *   **Block 7 (Plate Reverb):** Mix [10-12% Rhy / 20-22% Lead], Decay [1.8s Rhy / 2.5s Lead], Pre-delay [20ms], HPF [200Hz], LPF [4000Hz].