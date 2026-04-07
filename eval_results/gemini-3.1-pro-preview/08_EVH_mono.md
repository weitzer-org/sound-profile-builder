**QC-2 Systems Engineer Terminal Activated.** 
**Target Acquisition:** Eddie Van Halen "Brown Sound" (Late 70s / Early 80s).
**Analog Hunt Specs:** 1968 Marshall Super Lead 100W (Plexi), Variac voltage reduction to ~90V (Sag/Sponge), Celestion G12M Greenbacks, MXR Phase 90, EMT 140 Plate Reverb.
**CorOS Verification:** Verified. Emulating via Neural DSP's **Brit Plexi 100 Jump** (Pseudonym for Marshall 1959 Super Lead jumped channels), **412 Brit 35B Green 25**, **Phaser 95**, and **Plate Reverb**. 

To deliver this physically accurate, high-headroom preset to your QSC CP12 active speaker, we will employ the **Split-Bank Matrix** strategy to adapt the circuit to both your Fender Telecaster and Gibson ES-339.

*Remember the Non-Master Volume Rule: The Brit Plexi 100 Jump does not have a Master Volume. All SPL (Loudness) adjustments must be made at the Lane Output Level, never inside the Amp Block.*

---

### MULTI-GUITAR TARGET OUTPUT

#### ROW 2 (Scenes E-H): Gibson ES-339 (PAF Humbuckers)
*   **Physics Goal:** The ES-339's PAF humbuckers naturally push the midrange and compress the front end, mimicking EVH's Frankenstrat humbucker.
*   **Input Staging:** Set Global Input Gain to **-1.5dB**. The ES-339 output is warm; reducing the input slightly prevents the CorOS input block from clipping digitally before hitting the Plexi block, preserving the dynamic "chew" of the amp.
*   **EQ Strategy:** Leave the front-end EQ relatively flat, but set a High Pass Filter (HPF) at 100Hz to keep the neck pickup from getting muddy.

#### ROW 1 (Scenes A-D): Fender Telecaster (Single Coils)
*   **Physics Goal:** Telecasters lack the natural output and lower-midrange "push" required to saturate the Plexi into the Brown Sound. They also have a spiky pick attack that will sound harsh through the Plate reverb.
*   **Input Staging:** Increase Global Input Gain to **+3.0dB**. 
*   **The "Chameleon" Strategy (Parametric-8 EQ):** 
    *   **Band 1 (Body):** Peak at 220Hz, +3.5dB (Simulates the missing humbucker resonance/woodiness).
    *   **Band 3 (Push):** Peak at 800Hz, +2.0dB (Pushes the amp's phase inverter into saturation).
    *   **Band 8 (Twang Control):** Low Pass Filter (LPF) at 4.5kHz (Tames the single-coil "ice pick" attack so the EVH sizzle comes from the amp, not the guitar).

---

### TABLE A: MAIN SIGNAL CHAIN (Split-Bank Matrix)
*(Right-Click > Assign specific parameters to Scene control)*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 25% <br>Decay: 150ms | Noise Red: 15% <br>Decay: 250ms | *Adaptive % is crucial here.* Higher reduction for rhythm chunking (Scene A/E), lower for ringing lead sustain (Scene B/F). |
| **Pre-FX (Mod)** | Phaser 95 | Bypass | Active <br>Rate: 2.0 | Emulates EVH's MXR Phase 90 on leads (e.g., "Eruption"). Placed *before* the amp to chew up the phase before distortion. |
| **Amp** | Brit Plexi 100 Jump | Vol High: 8.5 <br>Vol Norm: 3.0 <br>Bass: 3.5 <br>Mid: 6.5 <br>Treb/Pres: 7.0 | Vol High: 9.5 <br>Vol Norm: 3.5 <br>Bass: 3.5 <br>Mid: 7.5 <br>Treb/Pres: 7.0 | *Variac Simulation:* Go to Amp Page 2 -> **Increase 'Sag' to 0.70**. This drops the virtual voltage, causing the spongy, compressed "Brown Sound" breakup. |
| **Pre-EQ** | Parametric-8 | *(See Tele vs 339 Chameleon settings above)* | *(See Tele vs 339 Chameleon settings above)* | Corrects the physical pickup differences before hitting the Plexi's input stage. |
| **Cab** | 412 Brit 35B Green 25 | Mic A: Dyn 57 (Dist: 1.0", Pos: 0.2) <br>Mic B: Ribbon 121 (Dist: 2.0", Pos: 1.5) | Mic A: Dyn 57 (Dist: 1.0", Pos: 0.2) <br>Mic B: Ribbon 121 (Dist: 2.0", Pos: 1.5) | Mix A (0dB) + B (-4.5dB). The Dyn 57 captures the bite; the Ribbon 121 captures the cabinet thump. *Set Cab HPF to 90Hz to protect the QSC CP12's woofer.* |
| **Post-FX (Rev)** | Plate Reverb | Mix: 18% <br>Decay: 1.5s <br>Pre-Delay: 15ms | Mix: 28% <br>Decay: 2.2s <br>Pre-Delay: 25ms | EVH used the Sunset Sound EMT Plate. High Pre-Delay ensures pick attack hits the QSC speaker before the reverb washes it out. |
| **Output Lane** | Lane Output Level | 0.0dB | +1.5dB | Increases overall SPL for the Lead scene without altering the amp's tube saturation characteristics. |

---

### TROUBLESHOOTING & REFINEMENT TREE
If you fire this up through the QSC CP12 and report: **"It's too distorted/fuzzy"** or **"It sounds broken"**:
1. **Input Pad:** Lower the Input Block Gain by an additional -3.0dB (Simulates rolling off the guitar's volume knob, which EVH did constantly).
2. **Amp Gain:** Reduce the Amp block's *Vol High* parameter from 8.5 down to 6.5. 
3. **Tube Sag:** If the amp sounds "farty" or the low-end collapses on palm mutes, reduce the Amp block's *Bass* parameter to 2.0, or slightly reduce the *Sag* parameter back toward 0.50. Variac emulation drastically loosens the bass response.
4. **Output Compensation:** If step 1 or 2 causes a volume drop from the PA speaker, compensate by raising the Lane Output Level. *Never use a compressor block to fix gain/volume issues on a Plexi.*

---

### SESSION LIBRARY (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
[Retained in Memory from previous session block]

**2. Preset Name: "Brown Sound Plexi - Split Bank"**
*Target:* Eddie Van Halen (1978).
*Guitar:* Split-Bank Matrix (Fender Telecaster & Gibson ES-339).
*Physics Goal:* Replicate variac voltage drop (sag) and cascaded high-treble saturation without master volume fizz. 
*Full Configuration:*
*   **Block 1 (Adaptive Gate):** Noise Red [Rhy: 25% / Lead: 15%], Decay [150ms / 250ms].
*   **Block 2 (Phaser 95):** Rate [2.0], Bypass [Sc A/E], Active [Sc B/F].
*   **Block 3 (Parametric-8 EQ):** 
    *   *Telecaster (Sc A-D):* HPF [90Hz], Band 1 Peak [220Hz, +3.5dB], Band 3 Peak [800Hz, +2.0dB], LPF [4500Hz]. 
    *   *ES-339 (Sc E-H):* HPF [100Hz], all other bands flat.
*   **Block 4 (Amp - Brit Plexi 100 Jump):** Vol Norm [3.0 / 3.5], Vol High [8.5 / 9.5], Bass [3.5], Mid [6.5 / 7.5], Treble [7.0], Presence [7.0], Sag [0.70], Output Level [+0dB].
*   **Block 5 (Cab - 412 Brit 35B Green 25):** Mic A (Dyn 57, Pos 0.2, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 2.0"), Mix [A: 0dB, B: -4.5dB], HPF [90Hz], LPF [6500Hz].
*   **Block 6 (Plate Reverb):** Mix [18% / 28%], Decay [1.5s / 2.2s], Pre-Delay [15ms / 25ms], Low Decay [0.5x], High Decay [0.8x].
*   **Lane Output:** [Rhythm: 0.0dB / Lead: +1.5dB].