Acknowledged. QC-2 Systems Engineer online. Standby for preset construction.

The target is the "Woman Tone" from the Cream era: a Les Paul with the tone knob rolled down, driving a cranked Marshall stack. This sound is characterized by a significant high-frequency cut *before* the amplifier, which smooths the distortion character and creates a vocal, flute-like quality. We will replicate this using a pre-amp EQ block.

First, to ensure proper gain staging, please confirm your primary guitar's pickup output.
**Are your pickups Vintage Output, Medium, or High Output?**

I will provide two distinct configurations optimized for different physics profiles:
1.  **Gibson ES-339 (Humbuckers):** Assumes medium-output humbuckers. This is the primary target tone.
2.  **Fender Telecaster (Single Coil):** Compensated for lower output and brighter frequency response.

---

### **Preset Build: "Brit Woman Tone"**

This preset utilizes Scenes A/B for the Telecaster (Row 1) and Scenes E/F for the ES-339 (Row 2), allowing you to switch guitars without changing presets.

*   **Scene A/E:** Rhythm (Base "Woman Tone")
*   **Scene B/F:** Lead (More saturated, boosted, enhanced vocal quality)

---

### **1. Gibson ES-339 (Humbucker) Configuration**

This profile assumes medium-output humbuckers and is designed for **Scenes E (Rhythm) & F (Lead)**.

**Table A: Main Signal Chain (ES-339)**
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | Thresh: **-55.0dB** | Thresh: **-58.0dB** | Humbuckers have a good signal-to-noise ratio. The threshold is lowered on the Lead scene to allow for more sustain and dynamic nuance before the gate engages. |
| **Pre-FX** | **Parametric-8** | HPF: 80Hz<br>LPF: **2.2kHz (Assign)**<br>Band 4 (Peak): 800Hz, Q: 1.0, Gain: **+1.0dB (Assign)** | HPF: 80Hz<br>LPF: **1.8kHz (Assign)**<br>Band 4 (Peak): 800Hz, Q: 1.0, Gain: **+2.5dB (Assign)** | **This is the core of the "Woman Tone."** The aggressive Low Pass Filter (LPF) simulates the guitar's tone knob being rolled off *before* hitting the amp. The lead scene's LPF is even lower for that "flute-like" sound, and the mid-boost helps it cut through. |
| **Amp** | **Brit 45** | Vol I: **7.5 (Assign)**<br>Vol II: 5.0<br>Treble: 4.5<br>Mid: 8.0<br>Bass: 6.0<br>Presence: 5.5 | Vol I: **8.8 (Assign)**<br>Vol II: 5.0<br>Treble: 4.5<br>Mid: 8.0<br>Bass: 6.0<br>Presence: 5.5 | Closest Available Model: Marshall JTM45. This non-master volume amp is jumped (both volumes used). Vol I (High Treble) is the primary gain control. Pushing it harder for the Lead scene increases saturation and harmonic complexity, true to the original circuit's physics. |
| **Cab** | **412 Brit G12M** | Mic A: Dyn 57 (Pos 0.8, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 1.5, Dist 3.0")<br>Mix: A: 0dB, B: -3.0dB | Mic A: Dyn 57 (Pos 0.8, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 1.5, Dist 3.0")<br>Mix: A: 0dB, B: -3.0dB | This cab is loaded with Greenbacks, the period-correct speaker. Blending the bite of the Dynamic 57 with the warmth and body of the Ribbon 121 provides a balanced, authentic sound ready for a PA system. |
| **Post-FX** | **Tape Delay** | Mix: **10% (Assign)**<br>Time: 280ms<br>Fdbk: 20% | Mix: **25% (Assign)**<br>Time: 280ms<br>Fdbk: 25% | Adds vintage-correct slapback and space. The mix is increased significantly on the Lead scene to give solos more depth and trail. |
| **Lane Output** | **Level** | **-1.5dB (Assign)** | **+1.5dB (Assign)** | **Critical for loudness control.** This provides a 3dB volume jump for the lead scene, ensuring it cuts through the mix without altering the amp's core tone or gain structure. |

---

### **2. Fender Telecaster (Single Coil) Configuration**

This profile is a "Chameleon" adaptation for **Scenes A (Rhythm) & B (Lead)**. It compensates for the Telecaster's lower output and brighter tone.

**Table A: Main Signal Chain (Telecaster)**
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | Thresh: **-48.0dB** | Thresh: **-50.0dB** | Single coils are inherently noisier. A higher (less sensitive) threshold is required to manage 60-cycle hum, especially with high amp gain. |
| **Input Block** | **Gain** | Gain: **+4.5dB** | Gain: **+4.5dB** | **Pickup Compensation.** This boosts the lower-output single coil signal *before* it hits any other blocks, allowing it to drive the amp model in a similar way to the humbucker. |
| **Pre-FX** | **Parametric-8** | HPF: 90Hz<br>LPF: **2.8kHz (Assign)**<br>Band 1 (LShelf): 250Hz, Q: 0.7, Gain: **+3.0dB** | HPF: 90Hz<br>LPF: **2.4kHz (Assign)**<br>Band 1 (LShelf): 250Hz, Q: 0.7, Gain: **+3.0dB** | **Chameleon Strategy.** We add a Low Shelf (LShelf) to give the Telecaster the body and low-mid weight it lacks compared to a Les Paul. The LPF is set slightly higher to retain some of the Tele's character without being shrill. |
| **Amp** | **Brit 45** | Vol I: **8.0 (Assign)**<br>Vol II: 5.5<br>Treble: 4.0<br>Mid: 8.0<br>Bass: 6.5<br>Presence: 5.0 | Vol I: **9.5 (Assign)**<br>Vol II: 5.5<br>Treble: 4.0<br>Mid: 8.0<br>Bass: 6.5<br>Presence: 5.0 | Amp settings are slightly more aggressive to compensate for the different pickup dynamics. Treble and Presence are reduced slightly to prevent ice-pick frequencies common with Telecasters under high gain. |
| **Cab** | **412 Brit G12M** | Mic A: Dyn 57 (Pos 0.8, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 1.5, Dist 3.0")<br>Mix: A: 0dB, B: -3.0dB | Mic A: Dyn 57 (Pos 0.8, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 1.5, Dist 3.0")<br>Mix: A: 0dB, B: -3.0dB | The cabinet and mic configuration remains consistent, as it represents the constant factor in the recording chain. |
| **Post-FX** | **Tape Delay** | Mix: **10% (Assign)**<br>Time: 280ms<br>Fdbk: 20% | Mix: **25% (Assign)**<br>Time: 280ms<br>Fdbk: 25% | FX settings remain consistent for stylistic cohesion. |
| **Lane Output** | **Level** | **-1.5dB (Assign)** | **+1.5dB (Assign)** | Provides the same controlled volume boost for lead work. |

---

### **Troubleshooting & Refinement Tree**

If the tone sounds incorrect with your specific guitar, follow this protocol:

1.  **Too Distorted / "Fuzzy":** The signal is clipping the amp model too hard.
    *   **Action:** In the **Input Block**, lower the **Gain** parameter. For high-output humbuckers, try setting it to -3.0dB or -6.0dB. This simulates rolling back your guitar's volume knob.
    *   **Compensate:** Increase the **Lane Output Level** to restore overall volume.
2.  **Too Thin (Especially Telecaster):** Lacks body.
    *   **Action:** In the **Parametric-8** block, increase the **Gain** of **Band 1 (LShelf)** to +4.0dB or higher.
3.  **Too Dark / Muffled:** The LPF is too aggressive for your pickups.
    *   **Action:** In the **Parametric-8** block, increase the **LPF Frequency**. Try 2.5kHz for Rhythm and 2.0kHz for Lead as a starting point.

System ready for further instructions.

---
### **Session Library (Active Presets)**

1.  **Preset Name:** "Spoonful - ES339"
    *   **Target:** Howlin' Wolf / Hubert Sumlin (1960).
    *   **Guitar:** ES-339 (Humbuckers) w/ Pick.
    *   **Physics Goal:** Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
    *   **Full Configuration:**
        *   Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
        *   Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
        *   Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
        *   Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
        *   Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
        *   Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

2.  **Preset Name:** "Brit Woman Tone - ES339"
    *   **Target:** Cream-era Eric Clapton "Woman Tone".
    *   **Guitar:** Gibson ES-339 (Humbuckers).
    *   **Physics Goal:** Replicate the effect of a rolled-off guitar tone knob into a cranked non-master volume amp using a pre-amp LPF.
    *   **Full Configuration:**
        *   Block 1 (Global Gate): Thresh [Rhy: -55dB / Lead: -58dB].
        *   Block 2 (Parametric-8): HPF [80Hz], LPF [Rhy: 2.2kHz / Lead: 1.8kHz], Band 4 (800Hz Peak) [Gain: +1.0dB / +2.5dB].
        *   Block 3 (Amp - Brit 45): Vol I [7.5 / 8.8], Vol II [5.0], Treble [4.5], Mid [8.0], Bass [6.0], Presence [5.5].
        *   Block 4 (Cab - 412 Brit G12M): Mic A (Dyn 57), Mic B (Ribbon 121), Mix [A: 0dB, B: -3.0dB].
        *   Block 5 (Tape Delay): Mix [10% / 25%], Time [280ms], Fdbk [20% / 25%].
        *   Block 6 (Lane Output): Level [-1.5dB / +1.5dB].