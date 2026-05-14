System Initialization: QC-2 (Quad Cortex Systems Engineer).
Target Acquired: Eddie Van Halen "Brown Sound" (1968 Marshall Super Lead, Variac Sag, Plate Reverb).
Speaker Profile: QSC CP12 (Active 12" PA - Flat Response configured).

To accurately reproduce the "Brown Sound" on the Quad Cortex, we must rely heavily on the **Physics First Protocol**. The secret to EVH’s tone was dropping the wall voltage of his 100W 1968 Marshall Super Lead via a Variac to roughly 90V. This starved the tubes, creating massive **power amp compression (Sag)** and a spongy attack, allowing him to crank the volume to 10 without immediate harsh clipping. 

Because the `Brit Plexi 100 Jumped` model has **No Master Volume**, we will drive the front end hard and strictly use the Amp Block Output Level or Lane Output for your QSC CP12 loudness to prevent PA blowout.

Here is your dual-guitar split-bank configuration.

---

### 1. Pickup Compensation & "Chameleon" Routing Strategy
To make a Gibson ES-339 (PAF-style Humbuckers) and a Fender Telecaster (Single Coils) both sound like EVH's custom-wound Frankenstein humbucker, we must manipulate the Input Stage and the Parametric-8 EQ Block.

**Guitar 1: Gibson ES-339 (Humbuckers)**
*   **Input Block Gain:** Set to **-3.0dB**. Humbuckers will naturally push the Plexi model into muddy clipping if not padded slightly. 
*   **Adaptive Gate Block:** Noise Reduction: 50% | Threshold: -55dB.
*   **EQ-8 Block (Clarity):** HPF: 80Hz. Band 3 (Low-Mid): -1.5dB at 250Hz (removes semi-hollow body mud). Band 6 (High-Mid): +1.0dB at 2.5kHz (adds pick attack).

**Guitar 2: Fender Telecaster (Single Coil)**
*   **Input Block Gain:** Set to **+2.5dB**. We must artificially push the amp harder to simulate the output of a hot humbucker.
*   **Adaptive Gate Block:** Noise Reduction: 65% | Threshold: -60dB (Single coils will introduce 60-cycle hum under Plexi gain; requires aggressive % reduction).
*   **EQ-8 Block (Body Boost & Tame):** HPF: 90Hz. Band 2 (Low Shelf): +3.5dB at 200Hz (simulates humbucker low-end weight). Band 8 (LPF): 4.5kHz (Crucial: rolls off the "ice-pick" Telecaster treble to smooth out the distortion).

---

### 2. Organization Standard (Split-Bank Matrix)
*   **Row 1 (Scenes A-D):** Fender Telecaster Profile.
*   **Row 2 (Scenes E-H):** Gibson ES-339 Profile.
*   *Scene Functions (per row):* Rhythm (A/E) | Lead (B/F) | Dry/Comping (C/G) | Ambient/Phaser (D/H)

---

### 3. Main Signal Chain & Configuration
*Note: Parameters marked with "(Right-Click > Assign)" shift between Scenes A/E (Rhythm) and B/F (Lead).*

**Table A: Main Signal Chain**
| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -55dB / Red: 50% | Thresh: -55dB / Red: 30% | Lower % reduction on Lead to let sustain trail off naturally without cutting out. |
| **Pre-FX** | Phaser 90 | Bypass: ON (unless Sc D/H) | Bypass: OFF, Rate: 25% | Classic MXR Script Phase 90 placed *before* the amp for the "Eruption" sweeping texture. |
| **Amp** | Brit Plexi 100 Jumped | Vol Normal: 6.5, Vol High: 7.0 | Vol Normal: 6.5, Vol High: 8.5 *(Assign)* | **Variac Physics:** Set Amp **Sag to 8.0** (spongy feel). **No Master Vol:** Drive the High channel for bite. |
| **Cab** | 412 Brit Green 25 | Mic A (57): Pos 0.5, Dist 1" | Mic A (57): Pos 0.5, Dist 1" | EVH used 20W/25W Greenbacks. Mic B: MD421 (Pos 1.5, Dist 2") mixed at -4.0dB for cabinet resonance. |
| **Post-FX 1** | Analog Delay | Bypass: ON | Mix: 15%, Time: 350ms, Fdbk: 20% | Slight analog slap/echo to thicken solos and add stereo width behind the amp. |
| **Post-FX 2** | Plate | Mix: 22%, Decay: 2.2s | Mix: 25%, Decay: 2.5s *(Assign)* | EMT 140 Plate simulation. HPF set to 250Hz so the reverb doesn't muddy the low-end palm mutes. |

---

### 4. Troubleshooting & Refinement Tree
If the tone translates through your QSC CP12 as "Too Harsh," "Fuzzy," or "Flubby," execute these steps immediately:
1.  **Input Pad:** Lower the Global Input Block Gain by an additional -2.0dB. This mimics rolling off the volume pot on your guitar.
2.  **Tube Sag Control:** If the bass notes sound "farty" or "flubby" on palm mutes, the Variac simulation (Sag at 8.0) is hitting the virtual power tubes too hard. Reduce **Amp Bass** to 3.0, and dial **Sag** down to 6.5. 
3.  **Output Compensation:** Remember, the `Brit Plexi 100 Jumped` has no Master Volume. If you need more stage volume, *do not touch the Amp Volumes*. Increase the Amp Block **Output Level** slider, or raise the final **Lane Output Level**.

---

### 5. Session Library (Active Presets)

**Appending current build to session registry...**

1. Preset Name: "Spoonful - ES339" (Previously saved session data)
**2. Preset Name: "VH Brown Sound - Dual Rig"**
*   **Target:** Eddie Van Halen (1978, 1968 Marshall Super Lead, Plate Reverb).
*   **Guitar:** Gibson ES-339 / Fender Telecaster.
*   **Physics Goal:** Simulate Variac voltage drop (sag) on a non-master volume circuit; compensate single-coil vs humbucker output via EQ geometry.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Tele: 65% / ES339: 50%], Thresh [-60dB / -55dB].
    *   **Block 2 (EQ-8 - Tele):** HPF [90Hz], Band 2 [+3.5dB @ 200Hz], Band 8 LPF [4500Hz]. 
    *   **Block 2 (EQ-8 - ES339):** HPF [80Hz], Band 3 [-1.5dB @ 250Hz], Band 6 [+1.0dB @ 2500Hz].
    *   **Block 3 (Phaser 90):** Rate [25%], Sync [Off], Mix [100% - placed pre-amp].
    *   **Block 4 (Amp - Brit Plexi 100 Jumped):** Vol Normal [6.5], Vol High [Rhy: 7.0 / Lead: 8.5], Bass [3.5], Mid [6.5], Treble [7.0], Presence [6.5], **Sag [8.0]**, Output Level [Rhy: 0.0dB / Lead: +1.5dB].
    *   **Block 5 (Cab - 412 Brit Green 25):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Dyn 421, Pos 1.5, Dist 2.0"), Mix [A: 0dB, B: -4.0dB], Phase [Aligned].
    *   **Block 6 (Analog Delay):** Mix [15%], Time [350ms], Fdbk [20%], Tone [5.0].
    *   **Block 7 (Plate):** Mix [Rhy: 22% / Lead: 25%], Decay [Rhy: 2.2s / Lead: 2.5s], Pre-Delay [15ms], HPF [250Hz].