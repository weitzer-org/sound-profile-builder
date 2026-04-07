QC-2 Systems Engineer online. Acknowledged: Target tone is Chicago Blues, Chess Records style. Small combo amp, warm, slightly gritty clean platform. This requires a touch-sensitive, edge-of-breakup tone with sag and dynamic response. We will use a non-master volume amp model and control the gain primarily with pick attack and guitar volume.

The physics goal is to replicate the sound of a low-wattage tweed amplifier being pushed by the instrument, captured by vintage microphones in a treated room. The QSC CP12's flat response is ideal for this, as the cabinet simulation will provide the full character.

**Target Amp Selection:**
Closest Available Model: `US Tweed Basslad Jumped`
This model simulates a 1959 Fender 5F6A Bassman with the Normal and Bright channels internally linked. This provides the warmth and complex overdrive characteristic of Chess-era recordings and allows us to blend the two channels for tonal shaping directly at the amp block.

Before proceeding, I must verify your pickup configuration to ensure proper gain staging.
**Query: Are your pickups Vintage Output, Medium, or High Output?** The following configurations assume vintage/medium output pickups. If you use high-output active pickups, we will need to engage the Input Block Pad.

I will build this preset using the Split-Bank Matrix standard. Row 1 (Scenes A-D) will be calibrated for a Fender Telecaster. Row 2 (Scenes E-H) will be calibrated for a Gibson ES-339.

---

### **Configuration 1: Gibson ES-339 (Humbuckers)**

Humbuckers have higher output and a stronger midrange. Our primary goal is to prevent muddiness and preserve note clarity while achieving warm overdrive. We will achieve this by lowering the input gain and using the amp's Bright channel to add articulation.

**Table A: Main Signal Chain (ES-339)**
*Changes for the Lead Scene (F) are marked with `(Assign)`.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| Input | **Input 1** | Gain: **-3.0dB** | Gain: **-3.0dB** | **Pickup Compensation:** Reduces the hot humbucker signal to prevent digital clipping and avoid oversaturating the amp model's input stage, mimicking a lower-output pickup. |
| Gate | **Global Input Gate** | Threshold: **-65.0dB** | Threshold: **-65.0dB** | Set low to allow notes to decay naturally. The goal is noise reduction, not a hard gate effect. |
| Pre-FX | **Parametric-8** | **Band 1 (LPF):** F: 5.5kHz<br>**Band 8 (HPF):** F: 90Hz | **Band 1 (LPF):** F: 6.0kHz<br>**Band 8 (HPF):** F: 90Hz | **Chameleon Strategy:** The LPF tames excessive high-end fizz from the humbuckers. The HPF cuts sub-bass mud, which is critical for clarity with a Tweed circuit. |
| Amp | **US Tweed Basslad Jumped** | **Vol Norm:** 2.8<br>**Vol Bright:** 3.5<br>**Bass:** 3.0<br>**Mid:** 6.5<br>**Treble:** 7.0<br>**Presence:** 6.0 | **Vol Norm:** 3.0<br>**Vol Bright:** 4.2 `(Assign)`<br>**Bass:** 3.0<br>**Mid:** 7.0 `(Assign)`<br>**Treble:** 7.0<br>**Presence:** 6.0 | **Tube Taper Logic:** This is a non-master volume amp. `Vol Norm` adds warmth/body. `Vol Bright` adds gain/grit. Pushing the Bright volume for the lead scene increases drive without adding bass mush. Bass is kept low to prevent "farty" low-end collapse. |
| Cab | **410 Basslad PR10** | **Mic A:** Dyn 57 (Pos 0.5)<br>**Mic B:** Ribbon 121 (Pos 0.8)<br>**Mix:** A: 0dB, B: -4.5dB | **Mic A:** Dyn 57 (Pos 0.5)<br>**Mic B:** Ribbon 121 (Pos 0.8)<br>**Mix:** A: 0dB, B: -4.5dB | **Speaker Physics:** The Dyn 57 provides the classic mid-range punch. The Ribbon 121, mixed lower, adds warmth and smooths the aggressive top end, simulating a classic studio mic blend. |
| Post-FX | **Tape Delay** | Mix: 15%<br>Time: 120ms<br>Fdbk: 10% | Mix: 20% `(Assign)`<br>Time: 120ms<br>Fdbk: 10% | **Spatial Goal:** A subtle, dark slapback echo common on Chess recordings. The low feedback keeps it from cluttering the signal. |
| Post-FX | **Room Reverb** | Mix: 18%<br>Decay: 1.2s<br>HPF: 150Hz | Mix: 18%<br>Decay: 1.2s<br>HPF: 150Hz | Simulates the sound of the recording studio room, adding depth without washing out the core tone. |

---

### **Configuration 2: Fender Telecaster (Single Coils)**

Single coils are lower output and brighter. The goal here is to add body, tame any "ice pick" harshness, and drive the amp model harder to achieve the same level of breakup as the humbucker version.

**Table A: Main Signal Chain (Telecaster)**
*Changes for the Lead Scene (B) are marked with `(Assign)`.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| Input | **Input 1** | Gain: **0.0dB** | Gain: **0.0dB** | **Pickup Compensation:** The standard input level is appropriate for vintage-style single coils. |
| Gate | **Global Input Gate** | Threshold: **-70.0dB** | Threshold: **-70.0dB** | Lower threshold than the humbucker version to account for the lower noise floor and to preserve delicate pick dynamics. |
| Pre-FX | **Parametric-8** | **Band 1 (Peak):** F: 220Hz, Q: 0.7, G: +1.5dB<br>**Band 8 (LPF):** F: 4.8kHz | **Band 1 (Peak):** F: 220Hz, Q: 0.7, G: +1.5dB<br>**Band 8 (LPF):** F: 5.2kHz | **Chameleon Strategy:** The 220Hz peak adds "body" and weight to the thinner single-coil sound. The LPF is crucial for taming the Telecaster's bridge pickup "ice pick" frequencies. |
| Amp | **US Tweed Basslad Jumped** | **Vol Norm:** 4.0<br>**Vol Bright:** 4.5<br>**Bass:** 3.5<br>**Mid:** 6.0<br>**Treble:** 6.5<br>**Presence:** 5.5 | **Vol Norm:** 4.2<br>**Vol Bright:** 5.5 `(Assign)`<br>**Bass:** 3.5<br>**Mid:** 6.5 `(Assign)`<br>**Treble:** 6.5<br>**Presence:** 5.5 | **Tube Taper Logic:** The amp volumes are set significantly higher to compensate for the lower output pickups, driving the virtual power tubes into saturation. Treble and Presence are slightly lower to complement the naturally bright guitar. |
| Cab | **410 Basslad PR10** | **Mic A:** Dyn 57 (Pos 0.5)<br>**Mic B:** Ribbon 121 (Pos 0.8)<br>**Mix:** A: 0dB, B: -3.0dB | **Mic A:** Dyn 57 (Pos 0.5)<br>**Mic B:** Ribbon 121 (Pos 0.8)<br>**Mix:** A: 0dB, B: -3.0dB | **Speaker Physics:** The Ribbon 121 is mixed slightly higher here to add more warmth and counter the inherent brightness of the single coils. |
| Post-FX | **Tape Delay** | Mix: 15%<br>Time: 120ms<br>Fdbk: 10% | Mix: 20% `(Assign)`<br>Time: 120ms<br>Fdbk: 10% | Identical settings provide a consistent slapback effect for this style. |
| Post-FX | **Room Reverb** | Mix: 18%<br>Decay: 1.2s<br>HPF: 150Hz | Mix: 18%<br>Decay: 1.2s<br>HPF: 150Hz | Consistent room sound is maintained across both guitar profiles. |

### Gain Staging & Scene Breakdown

*   **Lane Output Levels:** Set the Lane Output Level for your Rhythm scenes (A/E) to **-1.5dB** and your Lead scenes (B/F) to **0.0dB**. This creates the necessary volume boost for solos without altering the core amp tone.
*   **Scene C/G (Dry):** Create a copy of your Rhythm scene and disable the Delay and Reverb blocks for a tighter, more direct comping tone.
*   **Scene D/H (FX):** Create a copy of your Lead scene and increase the Mix/Feedback on the Delay for more experimental or ambient textures.
*   **Touch Sensitivity:** The primary source of gain is your picking hand and the guitar's volume knob. Roll the guitar volume back to 7-8 for a cleaner sound; push it to 10 for maximum grit.

### Troubleshooting & Refinement Tree
*   **If the tone is "Too Distorted/Fuzzy":**
    1.  First, lower the **Amp `Vol Bright`** knob in 0.5 increments. This is your primary gain control.
    2.  If still too fuzzy (especially with the ES-339), lower the **Input Block Gain** to -6.0dB.
    3.  Compensate for any volume loss by increasing the **Lane Output Level**.
*   **If the tone is "Too Thin/Bright" (Telecaster):**
    1.  In the Parametric-8 block, increase the Gain (G) on **Band 1** to +2.0dB or +2.5dB.
    2.  In the Cab block, increase the level of **Mic B (Ribbon 121)** to -2.0dB to add more warmth.
*   **If the tone is "Too Dark/Muddy" (ES-339):**
    1.  In the Parametric-8 block, lower the frequency of the **HPF (Band 8)** to 100Hz or 110Hz.
    2.  In the Amp block, slightly decrease the **`Vol Norm`** and slightly increase the **`Vol Bright`** to shift the overdrive character towards the upper-mids.

---
### **Preset Registry Protocol**

*Appended to Session Library.*

**2. Preset Name: "Chess King - ES339"**
*   **Target:** Warm Chicago Blues (Chess Records).
*   **Guitar:** Gibson ES-339 (Humbuckers).
*   **Physics Goal:** Achieve touch-sensitive edge-of-breakup by managing humbucker output into a non-master volume amp model.
*   **Full Configuration:**
    *   **Block 1 (Input):** Gain [-3.0dB].
    *   **Block 2 (Global Gate):** Threshold [-65.0dB].
    *   **Block 3 (EQ-8):** HPF [90Hz], LPF [Rhy: 5.5kHz / Lead: 6.0kHz].
    *   **Block 4 (Amp - US Tweed Basslad Jumped):** Vol Norm [2.8 / 3.0], Vol Bright [3.5 / 4.2], Bass [3.0], Mid [6.5 / 7.0], Treble [7.0], Presence [6.0].
    *   **Block 5 (Cab - 410 Basslad PR10):** Mic A (Dyn 57), Mic B (Ribbon 121), Mix [B: -4.5dB].
    *   **Block 6 (Tape Delay):** Mix [15% / 20%], Time [120ms], Fdbk [10%].
    *   **Block 7 (Room Reverb):** Mix [18%], Decay [1.2s], HPF [150Hz].
    *   **Lane Output:** Rhythm [-1.5dB], Lead [0.0dB].

**3. Preset Name: "Chess King - Telecaster"**
*   **Target:** Warm Chicago Blues (Chess Records).
*   **Guitar:** Fender Telecaster (Single Coils).
*   **Physics Goal:** Add body and tame harshness while driving the amp model harder to achieve breakup.
*   **Full Configuration:**
    *   **Block 1 (Input):** Gain [0.0dB].
    *   **Block 2 (Global Gate):** Threshold [-70.0dB].
    *   **Block 3 (EQ-8):** Band 1 Peak [F:220Hz, Q:0.7, G:+1.5dB], LPF [Rhy: 4.8kHz / Lead: 5.2kHz].
    *   **Block 4 (Amp - US Tweed Basslad Jumped):** Vol Norm [4.0 / 4.2], Vol Bright [4.5 / 5.5], Bass [3.5], Mid [6.0 / 6.5], Treble [6.5], Presence [5.5].
    *   **Block 5 (Cab - 410 Basslad PR10):** Mic A (Dyn 57), Mic B (Ribbon 121), Mix [B: -3.0dB].
    *   **Block 6 (Tape Delay):** Mix [15% / 20%], Time [120ms], Fdbk [10%].
    *   **Block 7 (Room Reverb):** Mix [18%], Decay [1.2s], HPF [150Hz].
    *   **Lane Output:** Rhythm [-1.5dB], Lead [0.0dB].