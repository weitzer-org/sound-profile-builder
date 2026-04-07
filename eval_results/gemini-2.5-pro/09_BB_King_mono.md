Acknowledged. Commencing preset build for a BB King "Lucille" tone on a high-headroom clean platform. The target is the articulate, percussive clean sound with a strong midrange focus, characteristic of his playing on a Gibson ES-355 through a Fender Twin Reverb.

I will construct two distinct versions of this preset to match the required `Split-Bank Matrix` standard.

*   **Row 1 (Scenes A-D):** Profiled for a **Fender Telecaster (Single Coils)**.
*   **Row 2 (Scenes E-H):** Profiled for a **Gibson ES-339 (Humbuckers)**, the primary target.

Before finalizing gain stages, please confirm your primary guitar's pickup type: **Are your pickups Vintage Output, Medium, or High Output?** The following configurations assume medium-output pickups for both profiles.

***

### **Preset: BK Twin Lucille**

This preset utilizes a Parametric EQ to emulate the effect of the "Varitone" switch found on BB King's Lucille, creating a focused midrange "honk" for the lead scene.

### **Profile 1: Gibson ES-339 (Humbucker)**
This is the core "Lucille" configuration. The primary goal is to manage the humbuckers' higher output to prevent unwanted amp breakup while retaining body and clarity.

**Table A: Main Signal Chain (Scenes E/F)**
*Marked changes are Scene-specific (Right-Click > Assign)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65.0dB | Thresh: -65.0dB | Humbuckers have a lower noise floor, allowing for a lower threshold. This keeps note trails natural. |
| **Pre-FX** | Parametric-8 | Band 4 (Mid Peak): **0.0dB**, Freq: 950Hz, Q: 1.0 | Band 4 (Mid Peak): **+3.5dB**, Freq: 950Hz, Q: 1.0 | The lead scene's mid-boost mimics the Varitone's tonal shaping, pushing the guitar forward in the mix without adding gain. A High Pass at 80Hz and Low Pass at 6.0kHz are active globally to clean up mud and fizz. |
| **Amp** | US TWN Rvb (Bright: **OFF**) | Vol: **3.8**, Bass: 3.5, Mid: 8.0, Treble: 6.5, Rev: 0 | Vol: **3.8**, Bass: 3.5, Mid: 8.0, Treble: 6.5, Rev: 0 | **Closest Available Model:** Fender '65 Twin Reverb. The `Bright` switch is OFF to tame the harshness of humbuckers. Amp Volume is kept constant; loudness comes from the Post-FX block and overall Lane Output. |
| **Cab** | 212 US TWN | Mic A: Dyn 57 (On Axis, 1.0"). Mic B: Ribbon 121 (Off Axis, 3.0") | Mic A: Dyn 57 (On Axis, 1.0"). Mic B: Ribbon 121 (Off Axis, 3.0") | The Dyn 57 provides the classic percussive attack. The Ribbon 121 adds body and warmth, essential for a full semi-hollowbody sound. Mix is 70% Dyn 57 / 30% Rbn 121. |
| **Post-FX** | Spring Reverb | Mix: **20%**, Dwell: 6.0, Tone: 6.0, Decay: 1.8s | Mix: **28%**, Dwell: 6.0, Tone: 6.0, Decay: 1.8s | A classic spring reverb provides ambience. The mix is increased for the lead scene to give solos more space and depth. |
| **Output** | Lane Output Level | Level: **-1.5dB** | Level: **0.0dB** | The lead scene gets a +1.5dB boost in pure volume (SPL) relative to the rhythm scene, ensuring it cuts through the mix cleanly. This is the correct method for loudness control, not raising amp gain. |

***

### **Profile 2: Fender Telecaster (Single Coil)**
This profile adapts the core tone for a lower-output, brighter guitar. The "Chameleon" strategy is employed to add body and tame harsh highs, compensating for the physical differences in the instrument.

**Table A: Main Signal Chain (Scenes A/B)**
*Marked changes are Scene-specific (Right-Click > Assign)*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input Block** | Input 1 Gain | Gain: **+3.0dB** | Gain: **+3.0dB** | Compensates for the lower output of single coils, ensuring the amp receives a similar signal level as the humbucker profile for consistent response. |
| **Input/Gate** | Global Input Gate | Thresh: **-55.0dB** | Thresh: **-55.0dB** | Single coils are noisier. The threshold is raised by 10dB to effectively combat 60-cycle hum without cutting off sustained notes. |
| **Pre-FX** | Parametric-8 | Band 1 (Low Shelf): **+2.0dB**, Freq: 250Hz. Band 4 (Mid Peak): **0.0dB** | Band 1 (Low Shelf): **+2.0dB**, Freq: 250Hz. Band 4 (Mid Peak): **+3.5dB** | The Low Shelf adds weight and "body" back into the thinner Telecaster signal. The lead scene uses the same mid-boost as the humbucker profile for consistency. |
| **Amp** | US TWN Rvb (Bright: **ON**) | Vol: **4.5**, Bass: 4.0, Mid: 7.5, Treble: 6.0, Rev: 0 | Vol: **4.5**, Bass: 4.0, Mid: 7.5, Treble: 6.0, Rev: 0 | The `Bright` switch is ON to leverage the Telecaster's natural clarity. Amp Volume is slightly higher to push the preamp tubes a bit more, creating a richer clean tone to compensate for the pickup type. Mids are slightly reduced to prevent "honkiness." |
| **Cab** | 212 US TWN | Mic A: Dyn 57 (Off Axis, 1.5"). Mic B: Ribbon 121 (Off Axis, 2.0") | Mic A: Dyn 57 (Off Axis, 1.5"). Mic B: Ribbon 121 (Off Axis, 2.0") | The Dyn 57 is moved slightly off-axis to reduce the "ice pick" highs common with Telecaster bridge pickups. The blend remains the same. |
| **Post-FX** | Spring Reverb | Mix: **22%**, Dwell: 6.0, Tone: 6.0, Decay: 1.8s | Mix: **30%**, Dwell: 6.0, Tone: 6.0, Decay: 1.8s | Reverb mix is slightly increased across the board to complement the brighter, more "airy" nature of the Telecaster. |
| **Output** | Lane Output Level | Level: **-1.5dB** | Level: **0.0dB** | Same logic as the humbucker profile: a pure SPL boost for the lead scene. |

***
### **Session Library (Active Presets)**

*Appended to registry for session recall.*

**1. Preset Name: "Spoonful - ES339"**
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

**2. Preset Name: "BK Twin Lucille"**
*   **Target:** B.B. King Live Tone (c. 1980s).
*   **Guitar:** ES-339 (Humbuckers).
*   **Physics Goal:** High-headroom pedal platform clean with a switchable mid-boost ("Varitone") for solos.
*   **Full Configuration:**
    *   Block 1 (Global Gate): Thresh [-65.0dB].
    *   Block 2 (EQ-8): HPF [80Hz], Band 4 [Rhy: 0.0dB / Lead: +3.5dB @ 950Hz Q1.0], LPF [6.0kHz].
    *   Block 3 (Amp - US TWN Rvb): Bright [OFF], Vol [3.8], Bass [3.5], Mid [8.0], Treble [6.5].
    *   Block 4 (Cab - 212 US TWN): Mic A (Dyn 57, On Axis, 1.0"), Mic B (Ribbon 121, Off Axis, 3.0"), Mix [A: 70%, B: 30%].
    *   Block 5 (Spring Reverb): Mix [Rhy: 20% / Lead: 28%], Dwell [6.0], Tone [6.0], Decay [1.8s].
    *   Block 6 (Output): Level [Rhy: -1.5dB / Lead: 0.0dB].