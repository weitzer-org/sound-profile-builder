**System Engineer QC-2 Initialization Complete.** 

### Hardware & "Physics First" Protocol 
**Target Output:** QSC CP12 (Active 12" PA Speaker). 
**Target Tone:** Joe Bonamassa Modern Blues Lead (Thick, vocal-like midrange, smooth high-end roll-off, infinite sustain without fizzy clipping).

**CorOS Hardware Verification & Pseudonym Protocol:** 
*Notice:* The Quad Cortex factory device list currently does not feature a dedicated Dumble Overdrive Special amplifier block (these are typically acquired via Neural Captures). To physically recreate Bonamassa’s signature smooth, Dumble-like midrange compression using *factory blocks*, I am deploying his other iconic amplifier staple: the **UK Silver 2555** (Marshall Silver Jubilee). When pushed with a **Myth Drive** (Klon Centaur), it geometrically mimics the cascading gain stages and localized low-mid thickness of an Overdrive Special.

---

### GUITAR 1: Gibson ES-339 Humbuckers
**Profile Goal:** Manage humbucker output to prevent digital clipping (fuzz), retain string separation, and roll off low-end mud. 
*Input Block (Global): Set Input 1 Gain to **-3.0dB** (Headroom Rule: Humbucker Output Compensation).*

#### Table A: ES-339 Signal Chain (Row 2: Scenes E-H)
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | Thresh: -60dB | Thresh: -65dB | Lower threshold on Lead to maintain sustain trails; catches humbucker hum. |
| **Pre-FX** | Myth Drive | Gain: 2.0<br>Tone: 4.5<br>Level: 6.5 | Gain: 4.5<br>Tone: 5.5<br>Level: 7.5 | Klon-style circuit pushes 1kHz into the amp. Lead scene increases symmetric clipping. |
| **Amp** | UK Silver 2555 | Input Gain: 4.5<br>Lead Master: 5.0<br>Treble: 4.5<br>Mid: 6.5<br>Bass: 4.0 | Input Gain: 6.5<br>Lead Master: 5.0<br>Treble: 4.5<br>Mid: 7.5<br>Bass: 4.5 | High mids create the "Dumble/Vocal" quality. Treble kept below 5 to ensure smooth, non-piercing drive. |
| **Cab** | 412 UK Silver 2555 | Mic A: Ribbon 121 (Pos: 1.5, Dist: 2.0")<br>Mic B: Dyn 57 (Pos: 0.5, Dist: 1.0") | *Same as Rhythm* | Ribbon 121 tames high-end fizz. Dyn 57 blended at -4dB for pick articulation. |
| **Post-EQ** | Parametric-8 | HPF: 110Hz<br>Band 3 (400Hz): -1.5dB<br>LPF: 6000Hz | HPF: 110Hz<br>Band 3 (400Hz): 0dB<br>LPF: 6500Hz | Chameleon Strategy: Clears humbucker low-mid mud (400Hz) on rhythm, adds thickness back for Lead. |
| **Post-FX** | Analog Delay | Mix: 12%<br>Time: 350ms<br>Fdbk: 25% | Mix: 22%<br>Time: 400ms<br>Fdbk: 35% | Analog BBD delay darkens the repeats, staying out of the way of the lead lines. |
| **Post-FX** | Room Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 20%<br>Decay: 1.5s | Simulates the natural studio room reflection Bonamassa favors over spring reverb. |

---

### GUITAR 2: Fender Telecaster Single Coils
**Profile Goal:** Thicken the inherently thin single-coil output to emulate a humbucker hitting a Dumble, tame the ice-pick bridge pickup, and push the amp harder. 
*Input Block (Global): Set Input 1 Gain to **+2.5dB** (Headroom Rule: Single Coil Output Compensation).*

#### Table B: Telecaster Signal Chain (Row 1: Scenes A-D)
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate (Grid) | Noise Red: 45% | Noise Red: 30% | Pivot to Adaptive Gate (%) to actively track and suppress 60-cycle single-coil hum. |
| **Pre-FX** | Myth Drive | Gain: 4.0<br>Tone: 3.5<br>Level: 7.0 | Gain: 6.0<br>Tone: 4.0<br>Level: 8.5 | Drive increased to compensate for lower pickup output. Tone rolled back to fight Tele twang. |
| **Amp** | UK Silver 2555 | Input Gain: 6.0<br>Lead Master: 5.0<br>Treble: 3.5<br>Mid: 7.0<br>Bass: 5.5 | Input Gain: 8.0<br>Lead Master: 5.5<br>Treble: 3.5<br>Mid: 8.0<br>Bass: 6.0 | Reduced treble and boosted bass. We rely heavily on preamp gain to compress the single coils into a "violin" texture. |
| **Cab** | 412 UK Silver 2555 | Mic A: Ribbon 121 (Pos: 2.0, Dist: 3.0")<br>Mic B: Dyn 57 (Pos: 1.5, Dist: 2.0") | *Same as Rhythm* | Mics moved further out from the dust cap (Pos 1.5/2.0) to physically roll off upper-frequency harshness. |
| **Post-EQ** | Parametric-8 | Band 1 (200Hz): +3.0dB<br>Band 6 (2.5kHz): -2.0dB<br>LPF: 4500Hz | Band 1 (200Hz): +4.5dB<br>Band 6 (2.5kHz): -1.5dB<br>LPF: 5000Hz | Chameleon Strategy: Massive Body boost (200Hz) simulates humbucker weight. Aggressive LPF kills ice-pick pick attack. |
| **Post-FX** | Analog Delay | Mix: 15%<br>Time: 350ms<br>Fdbk: 25% | Mix: 25%<br>Time: 400ms<br>Fdbk: 35% | Fills the space behind the thinner single coil. |
| **Post-FX** | Room Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 22%<br>Decay: 1.5s | Adds width and spatial dimension for the QSC CP12 PA speaker. |

---

### Troubleshooting & Refinement Tree (Tone Adjustment)
*If the tone is "Too Distorted" or "Too Fuzzy" through your QSC CP12:*
1. **Input Pad:** Lower the Input Block Gain by an additional -3.0dB (simulates rolling off the guitar's volume knob, which Bonamassa does constantly). 
2. **Amp Gain:** Reduce the `Input Gain` on the UK Silver 2555 in increments of 1.0. 
3. **Tube Sag/Mud:** If the low-end "farts out" on the ES-339 neck pickup, raise the HPF on the Parametric-8 to 140Hz. 
4. **Volume Compensation:** Do not raise amp gain to get louder. Use the **Lane Output Level** (far right circle on the grid) to increase SPL out of the QSC speaker.

---

### Session Library (Active Presets)

**2. Preset Name: "JB Modern ODS - Multi-Rig"**
*   **Target:** Joe Bonamassa / Dumble ODS style smooth lead.
*   **Guitars Administered:** ES-339 (Humbuckers) / Telecaster (Single Coil).
*   **Physics Goal:** Cascade a Klon into a dark British preamp to emulate Dumble mid-compression, utilizing heavy EQ shaping to normalize extreme pickup differences.
*   **Full Configuration (ES-339 baseline):**
    *   **Block 1 (Gate):** Global Gate, Thresh [-60dB / -65dB]. Input Gain [-3.0dB].
    *   **Block 2 (Myth Drive):** Gain [2.0 / 4.5], Tone [4.5 / 5.5], Level [6.5 / 7.5].
    *   **Block 3 (Amp - UK Silver 2555):** Input Gain [4.5 / 6.5], Lead Master [5.0], Treble [4.5], Mid [6.5 / 7.5], Bass [4.0 / 4.5], Presence [4.0]. 
    *   **Block 4 (Cab - 412 UK Silver):** Mic A (Ribbon 121, Pos 1.5, Dist 2.0"), Mic B (Dyn 57, Pos 0.5, Dist 1.0"), Mix [A: 0dB, B: -4dB].
    *   **Block 5 (EQ-8):** HPF [110Hz], Band 3 / 400Hz [-1.5dB / 0dB], LPF [6000Hz / 6500Hz]. 
    *   **Block 6 (Analog Delay):** Mix [12% / 22%], Time [350ms / 400ms], Fdbk [25% / 35%].
    *   **Block 7 (Room Reverb):** Mix [15% / 20%], Decay [1.2s / 1.5s]. 
*   **Telecaster Modifiers:** Adaptive Gate (Grid) added [45% Noise Red]. Input Gain [+2.5dB]. EQ-8 Band 1 (200Hz) [+3.0dB / +4.5dB]. LPF dropped to [4500Hz]. Cab mics moved to Pos [2.0] to physically tame treble.