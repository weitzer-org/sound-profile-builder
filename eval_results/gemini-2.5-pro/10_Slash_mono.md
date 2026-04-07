Acknowledged. As QC-2, I will engineer a gig-ready preset targeting the Slash GNR lead tone. This configuration will be optimized for a QSC CP12 speaker profile, prioritizing physics-accurate gain staging for both high-output humbuckers and vintage-output single coils.

**Target Analysis:**
*   **Artist:** Slash (Guns N' Roses era)
*   **Amp:** Hot-rodded Marshall JCM800. The key is high preamp gain combined with a pushed power section for saturation and response.
*   **Pickup:** Les Paul Neck. This is critical; it's warmer and has more low-mid content than the bridge, requiring careful EQ to avoid mud.
*   **FX:** Simple, clean digital or analog delay, sitting behind the primary tone.

**Quad Cortex Model Mapping:**
*   **Amp:** Brit 800. This is the quintessential JCM800 model in the Quad Cortex.
*   **Cab:** 412 Brit V30. This maps directly to the Celestion Vintage 30-loaded Marshall 1960 cabs Slash uses.
*   **Microphone:** Dyn 57 (Shure SM57). The industry standard for this tone.
*   **EQ:** Parametric-8. Essential for sculpting the neck pickup's character and achieving the "vocal" quality of Slash's solos.
*   **Delay:** Digital Delay. Clean and effective.

I will structure this preset using the Split-Bank Matrix for two guitar profiles. First, for a Gibson ES-339 (Humbuckers), which is tonally similar to a Les Paul. Second, for a Fender Telecaster (Single Coil), which will require significant compensation.

Before finalizing gain, please verify your pickup output: **Are your pickups Vintage Output, Medium, or High Output?** The following settings assume a standard, medium-to-high output humbucker and a vintage-output single coil. Adjust Input/Amp gain accordingly.

***

### **Preset Build: "LA Sunset Lead"**

#### **1. Gibson ES-339 (Humbucker) Configuration - Row 2**

This profile is designed for the higher output and warmer voice of humbuckers, focusing on clarity and aggression.

**Table A: Main Signal Chain (Scenes E & F)**
*Marked with (Assign) for Scene-specific changes.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Threshold: -52.0dB | Threshold: -52.0dB | A moderate threshold to cut high-gain hiss from humbuckers without killing sustain. |
| **Pre-FX** | Parametric-8 | Type: Band 5 Peak<br>Freq: 1.1kHz<br>Gain: **-1.0dB** (Assign)<br>Q: 1.2 | Type: Band 5 Peak<br>Freq: 1.1kHz<br>Gain: **+2.5dB** (Assign)<br>Q: 1.2 | The core of the lead tone. This mid-boost makes the neck pickup "sing" and cut through the mix for solos. Rhythm scene slightly scoops it for clarity. |
| **Amp** | **Brit 800** | Preamp: 6.8<br>Bass: 5.5<br>Mid: 7.0<br>Treble: 6.0<br>Master: 7.5<br>Output Level: -1.5dB (Assign) | Preamp: **7.5** (Assign)<br>Bass: 5.5<br>Mid: 7.0<br>Treble: 6.0<br>Master: 7.5<br>Output Level: **+1.5dB** (Assign) | Preamp is set high for saturation. Master is high to emulate a pushed power section. The small Preamp boost in the Lead scene adds harmonic complexity and sustain, not just volume. The Output Level provides the clean 3dB SPL jump for solos. |
| **Cab** | 412 Brit V30 | Mic: Dyn 57<br>Pos: 0.7 (Off-Axis)<br>Dist: 1.0"<br>Low Cut: 90Hz<br>High Cut: 7.5kHz | Mic: Dyn 57<br>Pos: 0.7 (Off-Axis)<br>Dist: 1.0"<br>Low Cut: 90Hz<br>High Cut: 7.5kHz | Off-axis placement tames harsh fizz. The cuts frame the guitar in a dense mix, removing sub-bass mud and inaudible high frequencies, crucial for an FRFR speaker like the CP12. |
| **Post-FX** | Digital Delay | Mix: **18%** (Assign)<br>Time: 380ms (Quarter Note)<br>Feedback: 25%<br>HPF: 150Hz / LPF: 3200Hz | Mix: **28%** (Assign)<br>Time: 380ms<br>Feedback: 25%<br>HPF: 150Hz / LPF: 3200Hz | The delay sits behind the dry signal. Filtering the repeats prevents them from clashing with the main tone. The mix increases significantly for the Lead scene to create a sense of space. |

---

#### **2. Fender Telecaster (Single Coil) Configuration - Row 1**

This profile uses the "Chameleon" strategy to compensate for the lower output and brighter tone of single coils, adding weight and taming harshness.

**Table A: Main Signal Chain (Scenes A & B)**
*Marked with (Assign) for Scene-specific changes.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Input Block Gain & Gate | **Gain: +3.0dB**<br>Threshold: -60.0dB | **Gain: +3.0dB**<br>Threshold: -60.0dB | The +3.0dB gain boost compensates for the lower single-coil output, ensuring the amp is driven correctly. The gate threshold is lower as there's less inherent noise. |
| **Pre-FX** | Parametric-8 | Band 1 Peak (220Hz): +2.0dB<br>Band 5 Peak (1.1kHz): **0.0dB** (Assign)<br>Band 8 LPF (4.5kHz): ON | Band 1 Peak (220Hz): +2.0dB<br>Band 5 Peak (1.1kHz): **+3.5dB** (Assign)<br>Band 8 LPF (4.5kHz): ON | **Chameleon EQ:** Band 1 adds the "body" of a humbucker. Band 8 (LPF) tames the "ice pick" treble. Band 5 provides a stronger mid-boost for the Lead scene to help the thinner pickup cut through. |
| **Amp** | **Brit 800** | Preamp: 7.5<br>Bass: 5.5<br>Mid: 7.0<br>Treble: 5.5<br>Master: 7.5<br>Output Level: -1.5dB (Assign) | Preamp: **8.5** (Assign)<br>Bass: 5.5<br>Mid: 7.0<br>Treble: 5.5<br>Master: 7.5<br>Output Level: **+1.5dB** (Assign) | Preamp gain is higher across both scenes to achieve the same level of saturation as the humbucker version. Treble is slightly reduced on the amp itself to work with the brighter pickup. |
| **Cab** | 412 Brit V30 | Mic: Dyn 57<br>Pos: 0.7<br>Dist: 1.0"<br>Low Cut: 90Hz<br>High Cut: 7.5kHz | Mic: Dyn 57<br>Pos: 0.7<br>Dist: 1.0"<br>Low Cut: 90Hz<br>High Cut: 7.5kHz | The cab settings remain identical as their function is to shape the speaker's response, which is constant regardless of the guitar used. |
| **Post-FX** | Digital Delay | Mix: **18%** (Assign)<br>Time: 380ms<br>Feedback: 25%<br>HPF: 150Hz / LPF: 3200Hz | Mix: **28%** (Assign)<br>Time: 380ms<br>Feedback: 25%<br>HPF: 150Hz / LPF: 3200Hz | The delay settings are identical, serving the same spatial purpose for both guitar profiles. |

### **Refinement & Troubleshooting**

If the tone is "Too Distorted" or "Too Fuzzy," especially with very high-output pickups:
1.  **Input Pad:** In the Input Block, reduce the Gain from 0.0dB to -3.0dB or -6.0dB. This is the most effective first step.
2.  **Amp Gain:** Reduce the Amp block's `Preamp` knob in increments of 0.5.
3.  **Output Compensation:** After reducing gain, the preset will be quieter. Compensate by raising the `Output Level` on the Amp block or the overall Lane Output Level. Do not use a compressor to fix gain staging.

This preset is now ready for deployment.

***
**Session Library (Active Presets)**
*I am appending this preset's primary configuration to the registry for this session.*

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

2.  **Preset Name:** "LA Sunset Lead"
    *   **Target:** Slash / Guns N' Roses Lead.
    *   **Guitar:** Gibson Humbucker (ES-339/Les Paul).
    *   **Physics Goal:** Hot-rodded JCM800 tone with vocal midrange for neck pickup solos.
    *   **Full Configuration (Humbucker Profile, Lead Scene F):**
        *   Block 1 (Global Gate): Threshold [-52.0dB].
        *   Block 2 (Parametric-8): Band 5 (Peak, 1.1kHz, Q 1.2) Gain [+2.5dB].
        *   Block 3 (Amp - Brit 800): Preamp [7.5], Bass [5.5], Mid [7.0], Treble [6.0], Master [7.5], Output Level [+1.5dB].
        *   Block 4 (Cab - 412 Brit V30): Mic (Dyn 57, Pos 0.7, Dist 1.0"), Low Cut [90Hz], High Cut [7.5kHz].
        *   Block 5 (Digital Delay): Mix [28%], Time [380ms], Feedback [25%], HPF [150Hz], LPF [3200Hz].