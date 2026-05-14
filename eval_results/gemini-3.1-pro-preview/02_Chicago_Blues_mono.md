Hello. I am QC-2, your Quad Cortex Systems Engineer. 

To nail the legendary Chess Records tone (think Muddy Waters, Little Walter, and early Hubert Sumlin), we are bypassing modern pedals. This preset relies entirely on vintage tube sag, transformer saturation, and physical room acoustics. 

Our analog target is a pushed 1950s 5E3 circuit. In the Quad Cortex, the **Closest Available Model** is the **US DLX 58**. Because vintage 5E3 circuits do not have a Master Volume, the physical overdrive is generated exclusively by pushing the "Inst Vol" (Instrument Volume) knob. Overall loudness going into your QSC CP12 must be controlled via the **Lane Output Level**.

Below are the exact, clearly separated configurations for both your **Fender Telecaster (Row 1)** and **Gibson ES-339 (Row 2)** to compensate for pickup output and frequency differences.

---

### Pickup Output & Global Gain Compensation Strategy
Before building the blocks, apply these Global/Lane adjustments to ensure proper signal-to-noise ratio and headroom:

*   **Fender Telecaster (Vintage/Single Coil):** 
    *   Input Block Gain: `0.0dB`
    *   Lane Output Level: `+2.0dB` (Compensates for lower natural pickup output).
*   **Gibson ES-339 (Medium Humbucker):** 
    *   Input Block Gain: `-3.0dB` (Prevents immediate digital clipping and unwanted early tube flub).
    *   Lane Output Level: `0.0dB`

---

### Table A: Main Signal Chain (Split-Bank Matrix)
*Row 1 (Scenes A-D) = Telecaster / Row 2 (Scenes E-H) = ES-339.*
*(Right-Click > Assign) on values with multiple Scene brackets.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | **Tele (Sc A):** Red: 35%, Thresh: -65dB<br>**339 (Sc E):** Red: 15%, Thresh: -70dB | **Tele (Sc B):** Red: 45%, Thresh: -60dB<br>**339 (Sc F):** Red: 20%, Thresh: -65dB | Single coils require higher reduction %. Humbuckers need less, allowing more natural decay. |
| **Pre-FX (EQ)** | Parametric-8 | **Tele (Sc A):** Low Shelf +3dB @ 200Hz, LPF @ 4.5kHz<br>**339 (Sc E):** HPF @ 100Hz, LPF @ 6.0kHz | **Tele (Sc B):** Mid Peak +2dB @ 1kHz<br>**339 (Sc F):** Mid Peak +1.5dB @ 1kHz | *Chameleon Strategy*: Tele needs body boost. 339 needs HPF to prevent the Tweed amp from farting out on low notes. |
| **Amp** | US DLX 58 | **Tele (Sc A):** Inst Vol: 4.5, Tone: 6.5<br>**339 (Sc E):** Inst Vol: 3.0, Tone: 7.0 | **Tele (Sc B):** Inst Vol: 6.5, Tone: 6.0<br>**339 (Sc F):** Inst Vol: 5.0, Tone: 6.5 | 339 hits the preamp harder; requires lower Inst Vol to stay at the same edge-of-breakup clean platform as the Tele. |
| **Cab** | 112 US DLX 58 | **Both:** Mic A (Ribbon 121), Pos 0.5, Dist 2.0" (Mix: 0dB)<br>Mic B (Dyn 57), Pos 0.0, Dist 1.0" (Mix: -6dB) | *(Cab parameters remain static)* | Ribbon mic provides the warm Chess Records vocal quality. Dynamic 57 adds slight pick attack bite for the QSC CP12 PA speaker. |
| **Post-FX 1** | Analog Delay | **Both:** Mix 8%, Time 90ms, Fdbk 5% | **Both:** Mix 12%, Time 90ms, Fdbk 5% | Emulates the fast tape/analog slapback echo often used in 50s Chicago blues studios. |
| **Post-FX 2** | Room Reverb | **Both:** Mix 15%, Decay 0.9s, HPF 150Hz, LPF 3000Hz | **Both:** Mix 15%, Decay 0.9s, HPF 150Hz, LPF 3000Hz | Replicates the physical acoustic space of 2120 S. Michigan Ave (Chess Studios). Filtered to avoid mud. |

*Scene Function Reminder:* 
*   **Scenes C / G (Dry/Comping):** Same as Rhythm (A/E) but Bypass the Delay and Reverb blocks. 
*   **Scenes D / H (Ambient):** Increase Delay Mix to 20% and Reverb Mix to 25%.

---

### Troubleshooting & Refinement Tree
Tweed circuits are notoriously sensitive to picking dynamics. If you find the tone is **"Too Distorted"** or **"Too Fuzzy"** through your QSC CP12, execute this strict order of operations:
1. **Input Pad:** Lower the Input Block Gain by an additional `-3.0dB`. (This physically simulates rolling off your guitar's volume knob without losing treble).
2. **Amp Gain:** Reduce the Amp Block `Inst Vol` by 1.0 to 2.0 increments.
3. **Tube Sag/Flub:** If the low-end sounds "broken" or "farty" (especially on the ES-339 neck pickup), return to the Pre-FX Parametric-8 EQ and raise the HPF from 100Hz up to 130Hz. 
4. **Output Compensation:** If step 1 or 2 causes a drop in volume, **do not add a compressor**. Compensate by raising the global Lane Output Level.

---

### Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration:
*   Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
*   Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
*   Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
*   Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
*   Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
*   Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

**2. Preset Name: "Chess 2120 - Split Bank"**
Target: Chicago Blues / Muddy Waters / Early Buddy Guy (1950s).
Guitar: Dual Matrix (Telecaster Single Coils & ES-339 Humbuckers).
Physics Goal: Vintage 5E3 Sag, Console Slapback, strictly edge-of-breakup via Inst Vol manipulation.
Full Configuration:
*   Input Block: Gain [Tele: 0.0dB / 339: -3.0dB].
*   Block 1 (Adaptive Gate): Noise Red [Tele: 35-45% / 339: 15-20%], Thresh [-65dB to -60dB].
*   Block 2 (EQ-8): Tele [Low Shelf +3dB @ 200Hz, LPF 4.5kHz]. 339 [HPF 100Hz, LPF 6.0kHz]. Lead Scene Mid Peak [+1.5dB to +2.0dB @ 1kHz].
*   Block 3 (Amp - US DLX 58): Inst Vol [Tele Rhy: 4.5, Lead: 6.5 / 339 Rhy: 3.0, Lead: 5.0], Tone [Tele: 6.5 / 339: 7.0], Mic Vol [0.0]. 
*   Block 4 (Cab - 112 US DLX 58): Mic A (Ribbon 121, Pos 0.5, Dist 2.0"), Mic B (Dyn 57, Pos 0.0, Dist 1.0"), Mix [A: 0dB, B: -6dB].
*   Block 5 (Analog Delay): Mix [8% / Lead: 12%], Time [90ms], Fdbk [5%].
*   Block 6 (Room Reverb): Mix [15%], Decay [0.9s], HP [150Hz], LP [3000Hz].
*   Lane Output Level: [Tele: +2.0dB / 339: 0.0dB].