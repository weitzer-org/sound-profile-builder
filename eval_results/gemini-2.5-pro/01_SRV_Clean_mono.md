Acknowledged. QC-2 online. Analyzing request for a high-headroom, SRV-style clean funk blues tone, designed to be pushed by a Tube Screamer.

I will construct two distinct preset profiles based on the Split-Bank Matrix standard. The first profile is optimized for a Fender Telecaster (Single Coils), serving as our baseline SRV tone. The second is a "Chameleon" adaptation for a Gibson ES-339 (Humbuckers), compensating for the increased output and different EQ curve.

**Critical First Question:** To ensure proper gain staging, please confirm your pickup output. Are your pickups Vintage Output (like a standard Tele/Strat), Medium, or High Output (active, modern metal)? I will proceed assuming vintage/medium output for both guitars.

---

### **Preset Profile 1: Fender Telecaster (Single Coils)**
**Bank Row:** Scenes A-D
**Physics Goal:** Achieve the "edge-of-breakup" chime of a cranked black-panel amp. The core tone will be clean with hard picking, allowing the overdrive pedal to provide all primary distortion for leads. The Global Input Gate is set higher to manage single-coil noise.

**Table A: Main Signal Chain (Telecaster)**
*(Note: Changes for Scenes C/D are specified in the rationale.)*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input 1 | Threshold: -52.0dB | Threshold: -52.0dB | A higher threshold is necessary to tame 60-cycle hum from single coils without cutting off sustained notes. |
| **Pre-FX 1** | Opto Comp | Sust: 4.5, Lvl: 6.0, **On** | Sust: 4.5, Lvl: 6.0, **Off** | (Right-Click > Assign) Evens out dynamics for funky comping (Rhythm). Disabled for Lead to allow full picking dynamics to interact with the overdrive. |
| **Pre-FX 2** | Chief OD | Drive: 6.5, Tone: 5.5, Lvl: 7.0, **Off** | Drive: 6.5, Tone: 5.5, Lvl: 7.0, **On** | **Closest Available Model: Ibanez TS808**. (Right-Click > Assign) The mid-hump and level boost will drive the amp's preamp section into saturation for a classic lead tone. |
| **Amp** | US SPR RVRB | Vol: 4.2, Bright: **On**, Bass: 5.0, Mid: 6.5, Treb: 7.0 | Vol: 4.2, Bright: **On**, Bass: 5.0, Mid: 6.5, Treb: 7.0 | **Closest Available Model: Fender Super Reverb**. Volume is set just before audible breakup. This is a Non-Master Volume amp; use Lane Output for SPL. |
| **Cab** | 410 US SPR RVRB CTS10 | Mic A: Dyn 57, Dist: 1.5" <br> Mic B: Rbn 121, Dist: 3.0" | Mic A: Dyn 57, Dist: 1.5" <br> Mic B: Rbn 121, Dist: 3.0" | The Dynamic 57 captures the bright paper-cone attack. The Ribbon 121 adds body and warmth. This combination provides a full-range, realistic sound. |
| **Post-FX 1** | Spring | Mix: 25%, Tone: 6.0, Dwell: 5.5 | Mix: 30%, Tone: 6.0, Dwell: 5.5 | Emulates the amp's onboard reverb tank. Slightly more mix on the lead scene for space. Scene C (Dry) will have this block Bypassed. |
| **Lane Output** | Level | **-1.5dB** | **+1.5dB** | (Right-Click > Assign) Creates the necessary volume boost for solos, ensuring the lead cuts through the mix without altering the core amp tone. |

**Scene Configuration (Telecaster):**
*   **Scene A (Rhythm):** Comp ON, Chief OD OFF.
*   **Scene B (Lead):** Comp OFF, Chief OD ON, Lane Output +1.5dB.
*   **Scene C (Dry Comping):** Comp ON, Chief OD OFF, Spring Reverb Bypassed.
*   **Scene D (FX/Ambient):** Add a "Rotary" block (Mix: 40%, Speed: Slow) for a classic Vibratone effect.

---

### **Preset Profile 2: Gibson ES-339 (Humbuckers)**
**Bank Row:** Scenes E-H
**Physics Goal:** Tame the higher output and darker tone of humbuckers to match the Telecaster's high-headroom profile. This requires reducing input gain and using EQ to restore clarity and prevent "mud."

**Table A: Main Signal Chain (ES-339)**
*(Note: Scene mapping E/F/G/H corresponds to A/B/C/D functionality.)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input 1 | Threshold: -65.0dB, **Gain: -4.5dB** | Threshold: -65.0dB, **Gain: -4.5dB** | **Headroom Rule Applied.** The -4.5dB pad prevents the hot humbuckers from clipping the A/D converter and pushing the amp model too hard, preserving clean headroom. Gate threshold is lower as humbuckers are quieter. |
| **Pre-FX 1** | EQ: Parametric-8 | HPF: 120Hz, Band 2 (Low Mid): -1.5dB @ 350Hz, Q: 0.7 | HPF: 120Hz, Band 2 (Low Mid): -1.5dB @ 350Hz, Q: 0.7 | **Chameleon Strategy.** HPF cuts low-end boom common with semi-hollows. The slight mid-cut reduces muddiness and enhances clarity, simulating a single-coil's response. |
| **Pre-FX 2** | Chief OD | Drive: 5.0, Tone: 6.0, Lvl: 6.5, **Off** | Drive: 5.0, Tone: 6.0, Lvl: 6.5, **On** | (Right-Click > Assign) Drive is reduced because the humbuckers are already providing a stronger signal. Level is also reduced to avoid over-saturating the amp. |
| **Amp** | US SPR RVRB | Vol: 2.8, Bright: **On**, Bass: 4.5, Mid: 6.5, Treb: 7.5 | Vol: 2.8, Bright: **On**, Bass: 4.5, Mid: 6.5, Treb: 7.5 | **Pickup Compensation.** Amp Volume is reduced by ~33% to match the breakup point of the single-coil profile. Bass is slightly lowered to further prevent mud. |
| **Cab** | 410 US SPR RVRB CTS10 | Mic A: Dyn 57, Dist: 1.5" <br> Mic B: Rbn 121, Dist: 3.0" | Mic A: Dyn 57, Dist: 1.5" <br> Mic B: Rbn 121, Dist: 3.0" | The cab and mic setup remains consistent to maintain the core tonal goal. The pre-amp EQ adjustments handle the guitar's tonal differences. |
| **Post-FX 1** | Spring | Mix: 25%, Tone: 6.0, Dwell: 5.5 | Mix: 30%, Tone: 6.0, Dwell: 5.5 | Identical settings to the Telecaster profile, providing consistent spatial effects across both guitar types. Scene G will have this Bypassed. |
| **Lane Output** | Level | **-1.5dB** | **+1.5dB** | (Right-Click > Assign) This standardized volume jump ensures consistent live performance levels when switching from rhythm to lead. |

### **Troubleshooting & Refinement**
If the tone for either guitar sounds "too fuzzy" or distorts prematurely:
1.  **Input Pad:** For the ES-339, increase the Input Block's negative gain from -4.5dB to -6.0dB.
2.  **Amp Gain:** Reduce the `US SPR RVRB` Volume knob in increments of 0.5.
3.  **Output Compensation:** Increase the Lane Output Level to restore any lost volume. Do not use the compressor to fix this.

This dual-profile approach provides a gig-ready preset that can be instantly adapted to two very different guitars by simply changing rows, ensuring consistent tone and behavior.

---
### **Session Library (Active Presets)**

1.  **Preset Name: "Spoonful - ES339"**
    *   Target: Howlin' Wolf / Hubert Sumlin (1960).
    *   Guitar: ES-339 (Humbuckers) w/ Pick.
    *   Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
    *   Full Configuration:
        *   Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
        *   Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
        *   Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
        *   Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
        *   Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
        *   Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

2.  **Preset Name: "Texas Flood Clean"**
    *   Target: Stevie Ray Vaughan "Clean Funk Blues" Platform.
    *   Guitar: Fender Telecaster / Gibson ES-339.
    *   Physics Goal: High-headroom pedal platform with separate gain-staging for Single Coils and Humbuckers.
    *   **Full Configuration (Telecaster - Row 1):**
        *   Block 1 (Input): Global Gate Threshold [-52.0dB].
        *   Block 2 (Opto Comp): Sust [4.5], Lvl [6.0]. (SCENE: A/C=On, B=Off).
        *   Block 3 (Chief OD): Drive [6.5], Tone [5.5], Lvl [7.0]. (SCENE: B=On).
        *   Block 4 (Amp - US SPR RVRB): Vol [4.2], Bright [On], Bass [5.0], Mid [6.5], Treb [7.0].
        *   Block 5 (Cab - 410 US SPR RVRB): Mic A (Dyn 57, 1.5"), Mic B (Rbn 121, 3.0").
        *   Block 6 (Spring Reverb): Mix [25%/30%], Tone [6.0], Dwell [5.5]. (SCENE: C=Bypassed).
        *   Lane Output: [-1.5dB / +1.5dB].
    *   **Full Configuration (ES-339 - Row 2):**
        *   Block 1 (Input): Global Gate Threshold [-65.0dB], Gain [-4.5dB].
        *   Block 2 (Parametric-8): HPF [120Hz], Band 2 [-1.5dB @ 350Hz].
        *   Block 3 (Chief OD): Drive [5.0], Tone [6.0], Lvl [6.5]. (SCENE: F=On).
        *   Block 4 (Amp - US SPR RVRB): Vol [2.8], Bright [On], Bass [4.5], Mid [6.5], Treb [7.5].
        *   Block 5 (Cab): Identical to Telecaster.
        *   Block 6 (Spring Reverb): Identical to Telecaster. (SCENE: G=Bypassed).
        *   Lane Output: [-1.5dB / +1.5dB].