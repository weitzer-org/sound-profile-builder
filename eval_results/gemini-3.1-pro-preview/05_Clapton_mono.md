### Target Acquisition: Cream-Era Eric Clapton ("Woman Tone")
To capture Clapton’s late-1960s Cream tone (e.g., *Sunshine of Your Love*, *I Feel Free*), we must rely on "Physics First" principles. The physical recipe is a Gibson-style humbucker (usually the neck pickup) with the guitar's tone knob rolled down to 0–2, pushing the front end of a cranked, non-master volume Marshall 100W Super Lead. 

Because a rolled-off humbucker introduces a massive amount of low-mid frequencies, **the amplifier must be set incredibly bright with low bass** to prevent the speaker from producing "farty," unintelligible mud. 

Below are the dual-configured builds for your Gibson ES-339 and Fender Telecaster, strictly mapped to Cortex Control.

***

### 🎸 PROFILE 1: GIBSON ES-339 (HUMBUCKERS)
**Row 2 Allocation: Scenes E (Rhythm) / F (Lead)**
*Physical Instruction:* For Scene F (Lead), physically switch your ES-339 to the **Neck Pickup**, roll the **Tone knob to 1**, and keep the **Volume at 10**. 

#### Table A: Main Signal Chain (ES-339)
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input / Adaptive Gate | In: -3.0dB<br>Red: 35% | In: -3.0dB<br>Red: 20% | *Pad humbuckers by -3dB to prevent digital input clipping. Gate relaxed for sustain.* |
| **Pre-FX** | Parametric-8 EQ | *(Bypassed)* | *(Bypassed)* | *Not needed; relying on your physical guitar tone knob.* |
| **Amp** | Brit Plexi 100 Jumped | Vol I: 4.5<br>Vol II: 3.0 | Vol I: 8.0<br>Vol II: 5.5 | *Vol I (Bright) pushes cut. Bass at 2.0, Mids at 8.0. Non-master amp: Loudness controlled via Lane Output!* |
| **Cab** | 412 Brit GB | Mic A: Dyn 57 (Center)<br>Mix: +0dB | Mic A: Dyn 57 (Center)<br>Mix: +0dB | *Greenback speakers. Ribbon 121 (Mic B, Edge, -3dB) adds the woolly, lower-mid resonance.* |
| **Post-FX** | Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 22%<br>Decay: 1.8s | *Simulates the large, reflective tracking rooms of 1960s London studios (IBC/Advision).* |
| **Lane Output** | Output 1/2 | Level: +2.0dB | Level: +3.5dB | *Controls final SPL for the QSC CP12 without altering tube saturation.* |

***

### 🎸 PROFILE 2: FENDER TELECASTER (SINGLE COIL)
**Row 1 Allocation: Scenes A (Rhythm) / B (Lead)**
*Physical Instruction:* Telecaster neck pickups retain too much treble "chime" even with the tone knob at zero, and lack the output to push the Plexi into natural compression. We will use the **Chameleon Protocol** (EQ + Input Gain) to artificially synthesize the "Woman Tone." Keep your guitar volume and tone at 10.

#### Table B: Main Signal Chain (Telecaster)
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input / Adaptive Gate | In: +2.5dB<br>Red: 45% | In: +4.0dB<br>Red: 30% | *Boosts single-coil voltage to hit the Plexi preamp tubes as hard as a PAF humbucker.* |
| **Pre-FX** | Parametric-8 EQ | *(Right-Click > Assign)*<br>Bypassed | *(Active)*<br>Band 2: +4dB (350Hz)<br>LPF: 950Hz | *Synthesizes a rolled-off humbucker. Massive Low-Pass Filter mimics a tone capacitor; low-mids boosted for body.* |
| **Amp** | Brit Plexi 100 Jumped | Vol I: 5.5<br>Vol II: 4.0 | Vol I: 9.0<br>Vol II: 6.0 | *Amp driven slightly harder than the ES-339 profile to compensate for single-coil dynamic drop.* |
| **Cab** | 412 Brit GB | Mic A: Dyn 57<br>Mix: -2dB | Mic A: Dyn 57<br>Mix: -2dB | *Favor Mic B (Ribbon 121, +0dB) here to thicken the Telecaster and round off the attack.* |
| **Post-FX** | Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 22%<br>Decay: 1.8s | *Adds spatial depth. High-Pass filter set to 150Hz to keep reverb tails clear of EQ-boosted mud.* |
| **Lane Output** | Output 1/2 | Level: +1.5dB | Level: +3.0dB | *Master volume compensation for the QSC CP12 PA speaker.* |

***

### ⚠️ Troubleshooting & Refinement Tree
If testing through your QSC CP12 yields unsatisfactory results, execute these steps in order:

1. **"It sounds like a blanket is over the amp / Too Muddy"**
   * *Fix:* The classic "Woman Tone" mistake is leaving the Amp's Bass knob too high. Lower the **Amp Bass to 1.5** and raise the **Presence to 7.0**. The warmth comes from the rolled-off guitar, not the amp.
2. **"The tone is sputtering / farty fuzz on low notes"**
   * *Fix:* The Plexi model is suffering from low-frequency tube sag. Go to the Cab Block and raise the **High-Pass Filter (HPF) to 110Hz** to clean up the sub-frequencies hitting the QSC CP12's woofer.
3. **"It's too distorted/harsh when I play chords"**
   * *Fix:* Lower the Input Block Gain by -2.0dB. Do not touch the Amp Volume knobs yet, as we need them cranked for the "Woman Tone" sustain. 

***

### 💾 Session Library (Active Presets)
**2. Preset Name: "Crossroads Plexi - Split"**
*   **Target:** Eric Clapton (Cream, 1967-1968). "Woman Tone".
*   **Guitars:** Gibson ES-339 (Row 2) & Fender Telecaster (Row 1).
*   **Physics Goal:** Vintage non-master volume overdrive reliant on guitar tone knob manipulation (ES-339) and artificial EQ roll-off (Telecaster).
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Tele: 45%/30% | 339: 35%/20%], Thresh [-65dB]. Input Gain [Tele: +2.5/+4.0dB | 339: -3.0dB].
    *   **Block 2 (EQ-8):** [Tele Lead Only]: HPF [90Hz], Band 2 [350Hz, +4.0dB, Q 1.5], Band 8 LPF [950Hz, 12dB/oct]. [339: Bypassed].
    *   **Block 3 (Amp - Brit Plexi 100 Jumped):** Vol I Bright [Tele R/L: 5.5/9.0 | 339 R/L: 4.5/8.0], Vol II Normal [Tele R/L: 4.0/6.0 | 339 R/L: 3.0/5.5], Bass [2.0], Mid [8.0], Treble [7.5], Presence [6.0].
    *   **Block 4 (Cab - 412 Brit GB):** Mic A (Dyn 57, Pos 0.2, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 3.0"). Mix [Tele: A -2dB, B 0dB | 339: A 0dB, B -3dB]. HPF [90Hz], LPF [6500Hz].
    *   **Block 5 (Plate Reverb):** Mix [15% / 22%], Decay [1.2s / 1.8s], Pre-Delay [15ms], HP [150Hz], LP [4000Hz].
    *   **Lane Output:** Level [Tele R/L: +1.5/+3.0dB | 339 R/L: +2.0/+3.5dB].