**Target Identified:** Classic 70s Rock Crunch (Warm & Punchy)
**QC Hardware Verified:** CorOS Native Models selected. 
**Speaker Profile Target:** QSC CP12 (Active FRFR). High-frequency roll-offs applied to prevent digital harshness through PA tweeters.

To accurately capture the non-master volume behavior of a vintage Marshall Plexi, we will use the **"Brit Plexi 50 Jumped"** model. Because this model lacks a master volume, **do not look for one**. We will drive the *Volume Normal* and *Volume High Treble* parameters for tone/saturation, and strictly use the Amp Block *Output Level* to control your actual room loudness (SPL) through your QSC CP12.

As requested, here are the completely distinct gain-staging protocols and configurations for both a Telecaster and an ES-339.

---

### GUITAR 1: Fender Telecaster Single Coil (Row 1 / Scenes A-D)
**Pickup Compensation Strategy:** Vintage single coils lack the low-mid push to naturally overdrive a Plexi's input stage. We must increase the Amp Block's *Volume Normal* to add girth to the bridge pickup, and utilize the Parametric-8 EQ to add physical weight (200Hz) while taming the aggressive pick attack (5kHz LPF) typical of a Telecaster.

* **Scene A:** Rhythm (Crunch, -1.5dB output relative to Lead)
* **Scene B:** Lead (Chief OD engaged, +1.5dB output, Mid-boosted)
* **Scene C:** Dry/Comping (Delay/Reverb Bypassed)
* **Scene D:** Ambient/FX (Higher Delay/Reverb Mix)

**Table A: Telecaster Signal Chain**
*(Right-Click > Assign to Scenes A-D)*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -55dB | Thresh: -55dB | Standard single-coil noise floor suppression. |
| **Pre-EQ** | Parametric-8 | Band 1 (Low Shelf): +3.5dB @ 200Hz | Band 1: +3.5dB @ 200Hz. Band 4: +2.0dB @ 800Hz | Simulates humbucker body (200Hz) and boosts midrange for lead projection. LPF at 5000Hz tames Tele ice-pick. |
| **Pre-FX** | Chief OD | [Bypassed] | Drive: 2.0, Tone: 5.5, Level: 7.5 | Pushes the tube grid for lead saturation without altering the Plexi voicing. |
| **Amp** | Brit Plexi 50 Jumped | Vol Norm: 5.0, Vol High Treb: 5.5, Bass: 3.5, Mid: 5.5, Treb: 5.0, Output Level: 0.0dB | Vol Norm: 5.0, Vol High Treb: 5.5, Bass: 3.5, Mid: 5.5, Treb: 5.0, Output Level: +1.5dB | Normal Vol is pushed to 5.0 to compensate for single-coil thinness. Output Level drives the SPL jump. |
| **Cab** | 412 Brit 1960TV | Mic A (Dyn 57): Pos 1.0, Dist 1.0". Mic B (Rib 121): Pos 1.5, Dist 2.0" | *Same as Rhythm* | Greenback speakers. Ribbon mic blended at -3dB to warm up the Tele bridge pickup through the QSC CP12 tweeter. |
| **Post-FX 1** | Analog Delay | Mix: 12%, Time: 330ms, Fdbk: 25% | Mix: 20%, Time: 330ms, Fdbk: 30% | Bucket-brigade style delay darkens repeats, keeping the mix uncluttered. |
| **Post-FX 2** | Plate Reverb | Mix: 15%, Decay: 1.2s | Mix: 18%, Decay: 1.5s | Emulates classic 70s studio spatial depth. |

---

### GUITAR 2: Gibson ES-339 Humbuckers (Row 2 / Scenes E-H)
**Pickup Compensation Strategy:** Medium/High output humbuckers naturally push the Plexi into compression. We must lower the *Volume Normal* parameter significantly to avoid low-end mud (tube sag) and rely heavily on the *Volume High Treble* to maintain clarity. 

* **Scene E:** Rhythm (Tight Crunch, -1.5dB output relative to Lead)
* **Scene F:** Lead (Chief OD engaged, +1.5dB output)
* **Scene G:** Dry/Comping (Delay/Reverb Bypassed)
* **Scene H:** Ambient/FX (Higher Delay/Reverb Mix)

**Table B: ES-339 Signal Chain**
*(Right-Click > Assign to Scenes E-H)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -65dB | Thresh: -65dB | Humbuckers are quieter; lowering threshold preserves sustain and dynamics. |
| **Pre-EQ** | Parametric-8 | HPF: 100Hz. Band 1: 0.0dB. LPF: 6500Hz | HPF: 100Hz. Band 4: +1.5dB @ 1kHz. LPF: 6500Hz | HPF removes resonant humbucker mud. Allows the Plexi's natural EQ to shine. |
| **Pre-FX** | Chief OD | [Bypassed] | Drive: 1.5, Tone: 6.0, Level: 7.0 | Lower drive than the Tele scene, as the ES-339 already hits the amp harder. |
| **Amp** | Brit Plexi 50 Jumped | Vol Norm: 2.5, Vol High Treb: 6.5, Bass: 3.0, Mid: 6.0, Treb: 5.5, Output Level: 0.0dB | Vol Norm: 2.5, Vol High Treb: 6.5, Bass: 3.0, Mid: 6.0, Treb: 5.5, Output Level: +1.5dB | Normal Vol dropped to 2.5 to prevent humbucker "farting/sag". High Treble raised for bite. |
| **Cab** | 412 Brit 1960TV | Mic A (Dyn 57): Pos 1.5, Dist 1.0". Mic B (Rib 121): Pos 2.0, Dist 2.5" | *Same as Rhythm* | Dyn 57 moved to Pos 1.5 (closer to cone edge) to tame humbucker harshness when distorted. |
| **Post-FX 1** | Analog Delay | Mix: 10%, Time: 330ms, Fdbk: 20% | Mix: 18%, Time: 330ms, Fdbk: 25% | Slightly lower mix than Tele to preserve humbucker note definition. |
| **Post-FX 2** | Plate Reverb | Mix: 12%, Decay: 1.2s | Mix: 15%, Decay: 1.5s | Emulates classic 70s studio spatial depth. |

---

### Troubleshooting & Refinement Tree
If the tone sounds **"Too Distorted"**, **"Farty"**, or **"Too Fuzzy"** with your specific guitar through the QSC CP12, execute these steps in strict order:

1. **Input Pad Verification:** The ES-339 may clip the digital converters depending on pickup height. Lower the Global Input Block Gain (Circle 1) to **-3.0dB** or **-6.0dB**. This effectively simulates rolling your guitar volume back.
2. **Amp Gain Reduction:** Reduce the Amp's *Volume High Treble* by 1.0 increment.
3. **Tube Sag Fix:** If the low-end sounds broken/farty on power chords (especially with the ES-339), reduce the Amp Block *Bass* parameter to 2.0. Vintage Plexi circuits phase-invert heavily when fed too much bass signal. 
4. **Output Compensation:** If steps 1-3 cause a drop in room volume, *do not add compression or turn the amp gain back up*. Raise the Lane Output Level on the right side of the Grid to recover the lost SPL.

---

### Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz].
Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

**2. Preset Name: "70s Rock Crunch Matrix"**
Target: Classic 70s Rock / Marshall Plexi.
Guitar: Multi-Matrix (Telecaster Row 1 / ES-339 Row 2).
Physics Goal: Non-master volume tube saturation, retaining low-end clarity for humbuckers and adding body to single coils.
Full Configuration:
Global Input (Circle 1): Thresh [-55dB Tele / -65dB ES339]. Gain [0.0dB].
Block 1 (EQ-8): Tele [Band 1: +3.5dB @ 200Hz, LPF: 5kHz] / ES339 [HPF: 100Hz, LPF: 6.5kHz].
Block 2 (Pre-FX - Chief OD): Drive [Tele Lead: 2.0 / ES339 Lead: 1.5], Tone [5.5 / 6.0], Level [7.5 / 7.0].
Block 3 (Amp - Brit Plexi 50 Jumped): Tele [Vol Norm 5.0, Vol Treb 5.5, Bass 3.5, Mid 5.5, Treb 5.0] / ES339 [Vol Norm 2.5, Vol Treb 6.5, Bass 3.0, Mid 6.0, Treb 5.5]. Block Output Level [Rhythm: 0.0dB / Lead: +1.5dB].
Block 4 (Cab - 412 Brit 1960TV): Mic A (Dyn 57), Mic B (Rib 121). Mix [0dB / -3dB].
Block 5 (Analog Delay): Mix [Tele: 12% / ES339: 10%], Time [330ms].
Block 6 (Plate Reverb): Mix [12-18% depending on Scene], Decay [1.2s - 1.5s].