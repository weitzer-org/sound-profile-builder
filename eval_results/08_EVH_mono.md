**QC-2 Systems Engineer Initialization...**
**Target Acquired:** Van Halen "Brown Sound" (1968 Marshall Super Lead Plexi, Variac Sag, Plate Reverb).
**Output Medium:** QSC CP12 (Active PA 12").

Before we initialize the final grid, I must verify our input stage parameters:
***Trigger: Are your pickups Vintage Output, Medium, or High Output?*** *(Note: I have pre-calculated the physics for medium-output PAF humbuckers and vintage single coils below, but this dictates our physical input headroom).*

### The Physics of the "Brown Sound" on the QC
Eddie Van Halen used a variac to drop the voltage of his 100W 1968 Marshall Super Lead down to ~90V. This induced extreme tube sag, lowering the headroom, increasing harmonic compression, and "browning out" the harsh upper frequencies. 

Because the Quad Cortex does not have a dedicated "Variac" block, we simulate this physical phenomenon through **three strict steps**:
1. **Gain Staging:** Pushing the Amp Block volume to maximum tube saturation, but compensating by dropping the Lane Output Level to prevent digital clipping in the QSC CP12.
2. **Frequency Roll-off:** EVH's tone was surprisingly dark and midrange-heavy. We will aggressively High-Pass (HPF) and Low-Pass (LPF) the signal to remove modern PA fizz.
3. **Phase-Inversion:** Running an MXR-style Phaser *before* the amp block, so the sweep physically distorts the preamp tubes, exactly as EVH did on *VH I*.

---

### Organization Standard (Split-Bank Matrix)
We are utilizing the Split-Bank matrix to accommodate both guitars within a single preset file.
*   **Row 1 (Scenes A-D):** Fender Telecaster (Single Coil)
*   **Row 2 (Scenes E-H):** Gibson ES-339 (Humbucker)
*   **Functions:** A/E = Rhythm (Dry/Tight), B/F = Lead (Phaser + Volume Boost), C/G = Comping (Volume rolled off), D/H = Ambient (Heavy Plate).

---

### CONFIGURATION 1: Gibson ES-339 (Humbuckers)
**Physics Goal:** The ES-339's medium-output PAF-style humbuckers have a strong low-midrange focus. If we run these into a jumped Plexi, the 12" woofer of the QSC CP12 will become muddy. We must reduce the 'Vol Normal' (Bass channel) on the amp and tighten the Input Gate to preserve percussive attack.

**Gain Staging Instruction:** Set Global Input Level to **0.0dB**. 

**Table A: Main Signal Chain (Gibson ES-339 - Scenes E-H)**
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55dB | Thresh: -60dB | Humbuckers are quieter. Tighter gate on Rhythm for staccato palm mutes. *(Right-Click > Assign)* |
| **Pre-FX** | Phaser | Bypass | Engaged (Mix: 40%, Rate: 2.0) | Script-style phaser hitting the preamp tubes for the "Eruption" sweep. |
| **Amp** | Brit Plexi 100 Jump | Vol Norm: 3.5<br>Vol Bright: 8.5 | Vol Norm: 3.5<br>Vol Bright: 9.0 | Driving the Bright channel hard. Normal channel kept low to prevent ES-339 low-mid mud. |
| **EQ** | Parametric-8 | Band 1: 150Hz (-2dB)<br>Band 8: LPF 5500Hz | Band 4: 800Hz (+1.5dB)<br>Level: +1.5dB | Tames ES-339 boominess. LPF mimics the "brown" variac treble roll-off. Mid-boost for Lead. *(Right-Click > Assign)* |
| **Cab** | 412 Brit 25G (Greenbacks) | Mic A: Dyn 57 (Pos 0.4)<br>Mic B: Ribbon 121 (Pos 1.0) | Mic A: Dyn 57 (Pos 0.4)<br>Mic B: Ribbon 121 (Pos 1.0) | Greenback 25W speakers are essential for the EVH paper-cone breakup. Ribbon mic adds QSC PA warmth. |
| **Post-FX** | Plate Reverb | Mix: 15%, Decay: 2.5s | Mix: 22%, Decay: 3.0s | Simulates Sunset Sound's echo chambers. Wider decay on Lead. |

---

### CONFIGURATION 2: Fender Telecaster (Single Coils)
**Physics Goal:** Telecaster single coils lack the physical output to drive the Plexi into EVH saturation, and their inherent 3kHz-5kHz spike will sound like an ice-pick through the QSC CP12 tweeter. We must use the Chameleon Strategy: boosting the input block gain, injecting an artificial low-end "body" boost via EQ, and darkening the top end.

**Gain Staging Instruction:** Set Global Input Level to **+3.0dB** (Simulates humbucker output voltage hitting the amp).

**Table A: Main Signal Chain (Fender Telecaster - Scenes A-D)**
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -45dB | Thresh: -50dB | Single coils have 60-cycle hum. Higher threshold required to keep silence between staccato riffs. |
| **Pre-FX** | Phaser | Bypass | Engaged (Mix: 40%, Rate: 2.0) | Placed pre-amp to squash the high-end transients of the Telecaster pick attack. |
| **EQ (Pre)** | Parametric-8 | Band 1 (Low Shelf): 200Hz (+3.5dB)<br>Band 8 (LPF): 4000Hz | Band 4: 800Hz (+2.0dB)<br>Level: +1.5dB | **Chameleon Protocol:** +3.5dB at 200Hz gives the Telecaster the physical weight of a mahogany body. Aggressive LPF kills ice-pick highs. |
| **Amp** | Brit Plexi 100 Jump | Vol Norm: 6.0<br>Vol Bright: 8.5 | Vol Norm: 6.5<br>Vol Bright: 9.5 | Normal channel increased significantly to add natural tube bass response to the single coils. |
| **Cab** | 412 Brit 25G (Greenbacks) | Mic A: Dyn 57 (Pos 0.6)<br>Mic B: Ribbon 121 (Pos 1.0) | Mic A: Dyn 57 (Pos 0.6)<br>Mic B: Ribbon 121 (Pos 1.0) | Mic A moved further to edge of cone (0.6) to darken the single-coil presence. |
| **Post-FX** | Plate Reverb | Mix: 15%, Decay: 2.5s | Mix: 22%, Decay: 3.0s | Provides the necessary 3D spatial widening to single-coil notes. |

---

### Troubleshooting & Refinement Tree (The "Fuzz" Protocol)
If the QSC CP12 sounds too "fuzzy" or harsh when playing chords:
1. **Input Pad:** Lower the Input Block Gain to `-3.0dB`. (Simulates rolling off the guitar's volume knob).
2. **Tube Sag Correction:** The Brit Plexi model can get "farty" in the low-end when maxed. Reduce the Amp Block *Bass* parameter by `1.5` increments. 
3. **PA Correction:** Never use a compressor to fix EVH gain. Instead, lower the Amp Block *Vol Bright* to `7.0` and increase the overall *Lane Output Level* by `+2.0dB` to regain the SPL loss.

---

### Session Library (Active Presets)

**2. Preset Name: "Brown Sound Matrix - 1968"**
*   **Target:** Eddie Van Halen (VH1, 1978).
*   **Guitars:** ES-339 (Humbuckers) / Telecaster (Single Coil).
*   **Physics Goal:** Maximize tube saturation while simulating voltage-drop sag via EQ and LPF, preventing digital harshness in an FRFR/PA speaker.
*   **Full Configuration (ES-339):**
    *   Block 1 (Global Gate): Thresh [-55dB / -60dB]. Input Gain [0.0dB].
    *   Block 2 (Phaser): Mix [0% / 40%], Rate [2.0], Depth [50%].
    *   Block 3 (Amp - Brit Plexi 100 Jump): Vol Norm [3.5], Vol Bright [8.5 / 9.0], Bass [3.5], Mid [6.5], Treble [6.0], Presence [5.5], Output Level [-4.0dB to compensate for high gain].
    *   Block 4 (EQ-8): Band 1 [150Hz, -2dB], Band 8 LPF [5500Hz]. Lead Level [+1.5dB].
    *   Block 5 (Cab - 412 Brit 25G): Mic A (Dyn 57, Pos 0.4), Mic B (Ribbon 121, Pos 1.0), Mix [A: 0dB, B: -2dB], HPF [80Hz].
    *   Block 6 (Plate Reverb): Mix [15% / 22%], Pre-Delay [20ms], Decay [2.5s / 3.0s].
*   **Full Configuration (Telecaster):**
    *   Block 1 (Global Gate): Thresh [-45dB / -50dB]. Input Gain [+3.0dB].
    *   Block 2 (EQ-8 - Pre-Amp): Low Shelf [200Hz, +3.5dB], LPF [4000Hz]. Lead Level [+1.5dB].
    *   Block 3 (Phaser): Mix [0% / 40%], Rate [2.0], Depth [50%].
    *   Block 4 (Amp - Brit Plexi 100 Jump): Vol Norm [6.0], Vol Bright [8.5 / 9.5], Bass [4.5], Mid [7.0], Treble [5.0], Presence [4.0], Output Level [-4.0dB].
    *   Block 5 (Cab - 412 Brit 25G): Mic A (Dyn 57, Pos 0.6), Mic B (Ribbon 121, Pos 1.0), Mix [A: 0dB, B: 0dB].
    *   Block 6 (Plate Reverb): Mix [15% / 22%], Pre-Delay [20ms], Decay [2.5s / 3.0s].