Welcome to the Quad Cortex Systems Engineering interface. I am QC-2. 

### Deep Research: "Still Got The Blues" Target Analysis
**Target Tone:** Gary Moore (1990). Massive, endlessly sustaining, fluid lead tone with biting articulation.
**Analog Specs:** 1959 Gibson Les Paul "Greeny" (Vintage PAF pickups, neck pickup reversed but played mostly on the neck/bridge independently for this track) -> Marshall Guv'nor (Mk1) -> Marshall JTM45 Reissue -> Marshall 1960B 4x12 (G12T-75 or Greenbacks) -> SSL Console (adding massive Hall reverb).

### Physics First: The High-Output Humbucker Problem
Gary Moore used vintage-output PAF humbuckers. Pushing a Marshall Guv'nor into a non-master volume JTM45 with **High Output Humbuckers** will instantly overload the amp block's virtual tube grid, creating excessive low-end "farting" (tube sag) and digital clipping. 

**Our Compensation Strategy:** 
1. We must apply an **Input Pad (-6.0dB)** at the Global Input to simulate the output of a vintage PAF.
2. The JTM45 has no Master Volume. We must keep the Amp Block's **Bass parameter very low (2.0 - 2.5)** to prevent muddy blocking-distortion. We will restore the perceived warmth using the Cab block and EQ.

---

### Organization Standard (Split-Bank Matrix)
*Since you are running High Output Humbuckers, your primary bank is Row 2 (Scenes E-H). Row 1 is pre-configured for Single Coils in case you switch guitars.*

*   **Scene E (Rhythm):** Tighter gate, slightly backed-off drive, standard reverb.
*   **Scene F (Lead):** The "Gary Moore" Scene. Increased Guv'nor gain, boosted Amp Bright Volume, delayed activated, +1.5dB output boost.
*   **Scene G (Dry):** No spatial effects for tight tracking.
*   **Scene H (Ambient/FX):** Infinite sustain focus. Long decay on the Hall reverb.

---

### Table A: Main Signal Chain
*(Apply these Scene-Specific changes by Right-Clicking the parameter in Cortex Control and selecting "Assign to Scene")*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate (Grid) | Thresh/Red: 40% | Thresh/Red: 55% | High gain + High Output PUs = Noise. Higher reduction on Lead to quiet the resting hum. **Crucial: Set Input Block Gain to -6.0dB.** |
| **Pre-FX (Drive)**| Governor | Gain: 4.5, Vol: 6.5, Bass: 4.0, Mid: 7.0, Treb: 5.5 | Gain: 6.5, Vol: 7.5, Bass: 4.0, Mid: 7.5, Treb: 6.0 | Mid-push forces the JTM into fluid saturation. High volume hits the amp's V1 tube harder. |
| **Amp** | Brit 45 Jumped | Vol Bright: 6.0, Vol Norm: 3.5, Bass: 2.5, Mid: 6.0, Treb: 5.5, Pres: 5.0 | Vol Bright: 7.5, Vol Norm: 4.5, Bass: 2.0, Mid: 6.5, Treb: 6.0, Pres: 5.5 | JTM taper logic: Keep Bass low to prevent low-frequency tube sag when pushed by the pedal. Use Lane Output Level for overall loudness. |
| **Cab** | 412 Brit 35B | Mic A: Dyn 57 (Pos 1.5, Dist 1.0"), Mix 0dB | Mic B: Ribbon 121 (Pos 2.5, Dist 2.5"), Mix -3dB | 57 gives the bite; 121 ribbon mic adds back the warmth we cut from the Amp's bass knob. Optimizes translation to your QSC CP12 PA. |
| **Post-FX (EQ)**| Parametric-8 | Band 3: -2dB @ 300Hz, Band 8: LPF @ 6000Hz | Band 3: -2dB @ 300Hz, Band 8: LPF @ 6500Hz | *Chameleon Strategy*: Tames the harsh buzz of high-output pickups while clearing up the low-mid mud (300Hz). |
| **Post-FX (Dly)** | Analog Delay | Mix: 10% | Mix: 22%, Time: 380ms, Fdbk: 30% | Thickens the lead tone. Analog style rolls off high frequencies so repeats don't clash with your soloing. |
| **Post-FX (Rev)** | Hall | Mix: 18%, Decay: 1.8s | Mix: 28%, Decay: 2.8s | Replicates the massive stadium SSL console reverb used on the original 1990 track. |

---

### Troubleshooting & Refinement Tree
If the tone isn't interacting with your specific guitar correctly:
1. **Too Compressed/Fuzzy?** Your pickups are still hitting the blocks too hard. Lower the Global Input Gain to -8.0dB. Do *not* lower the Governor volume, as that changes the tone profile.
2. **Too Much Bass/Booming through the QSC CP12?** Do not touch the Amp Bass knob (it is already at the minimum safe limit for this circuit). Instead, go to the Cab Block and enable the High Pass Filter (HPF) at 90Hz to keep the active PA speaker from struggling.
3. **Not Enough Loudness?** Do not turn up the Amp Volume (that just adds drive). Slide the **Lane Output Level** on the right side of the Cortex Control grid up by +3dB to +6dB.

---

### Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

**2. Preset Name: "Still Got The Blues - GM"**
Target: Gary Moore (1990).
Guitar: High Output Humbuckers.
Physics Goal: Compensate for hot pickups to prevent JTM45 blocking distortion while replicating a driven 1959 PAF tone.
Full Configuration:
Global Input Gain: -6.0dB (Critical).
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 55%].
Block 2 (Governor): Gain [4.5 / 6.5], Level [6.5 / 7.5], Bass [4.0], Mid [7.0 / 7.5], Treb [5.5 / 6.0].
Block 3 (Amp - Brit 45 Jumped): Vol Bright [6.0 / 7.5], Vol Norm [3.5 / 4.5], Bass [2.5 / 2.0], Mid [6.0 / 6.5], Treble [5.5 / 6.0], Presence [5.0 / 5.5]. Lane Output [+1.5dB Lead scene].
Block 4 (Cab - 412 Brit 35B): Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 2.5, Dist 2.5"), Mix [A: 0dB, B: -3dB].
Block 5 (Parametric-8): Band 3 [-2.0dB @ 300Hz], LPF [6000Hz / 6500Hz].
Block 6 (Analog Delay): Mix [10% / 22%], Time [380ms], Fdbk [30%].
Block 7 (Hall Reverb): Mix [18% / 28%], Decay [1.8s / 2.8s].