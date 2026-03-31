Greetings. I am **QC-2**, your Quad Cortex Systems Engineer. 

To achieve a "warm and punchy" 70s rock crunch, we must rely heavily on physics and proper gain staging. The quintessential 70s Marshall is a 100-watt Plexi Super Lead. In the CorOS, we will use the **"Brit Plexi 100 Jumped"**. 

**CRITICAL PLEXI PROTOCOL:** The original Plexi circuit—and therefore our QC model—**does not have a Master Volume**. The "Vol Normal" and "Vol Bright" act as your drive/crunch controls. If you raise these to get more distortion, the amp will get exponentially louder. **You must use the Lane Output Level (far right circle on the grid) to control your actual room/speaker volume.** 

Per System Protocol 9, I have constructed two completely distinct gain-staging profiles to optimize this tone for both your Humbucker and Single Coil instruments.

---

### PART 1: GIBSON ES-339 (HUMBUCKERS) PROFILE
**Target:** Scenes E (Rhythm), F (Lead), G (Dry), H (Ambient)
**Physics Goal:** Humbuckers naturally push the front end of a Plexi hard, which can cause low-end "farting" or tube sag. We will roll off the Bass on the amp, push the Bright channel for punch, and use a slight mid-cut to prevent mud.

#### Table 1: ES-339 Main Signal Chain
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global In (Circle 1) | Thresh: -55dB | Thresh: -55dB | Humbuckers are quieter; standard threshold prevents gating sustains. |
| **Pre-FX** | Chief OD | Bypass | Active (Drive: 2, Vol: 7) | Bluesbreaker-style transparent push for Lead sustain without coloring the Plexi. |
| **Amp** | Brit Plexi 100 Jumped | Normal: 3.5, Bright: 5.5 | Normal: 4.0, Bright: 6.5 | *(Right-Click Assign)*. Low bass (3.0), High Mids (6.5). Bright channel provides the "punch" for humbuckers. |
| **Cab** | 412 Brit 35B | Dyn 57 (Mix: +2dB) | Dyn 57 (Mix: +2dB) | Greenback speakers. Dyn 57 on Cap Edge (Pos 1.5) provides bite; Ribbon 121 (Cone, Pos 3.5) adds warmth. |
| **EQ** | Parametric-8 | Band 2: 400Hz (-1.5dB) | Band 2: 400Hz (-1.5dB) | Clears up low-mid mud inherent to Gibson semi-hollow bodies. LPF @ 6kHz removes digital fizz. |
| **Post-FX** | Analog Delay | Mix: 0% (Bypass) | Mix: 15% (350ms) | Thickens the lead tone. Keep feedback low (20%) to prevent washing out the crunch. |
| **Post-FX** | Room Reverb | Mix: 12% | Mix: 18% | Simulates a wood-paneled 70s tracking room. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | *(Right-Click Assign)*. Sole control for SPL/Volume boost during solos. |

---

### PART 2: FENDER TELECASTER (SINGLE COIL) PROFILE
**Target:** Scenes A (Rhythm), B (Lead), C (Dry), D (Ambient)
**Physics Goal:** Single coils lack the output to naturally distort a Plexi into 70s rock territory and can sound thin or "ice-picky." We will increase the Global Input Gain, push the "Normal" (warmth) channel of the Plexi, and use a 200Hz EQ shelf to simulate humbucker body.

#### Table 2: Telecaster Main Signal Chain
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global In (Circle 1) | Gain: +3.0dB, Thresh: -45dB | Gain: +3.0dB, Thresh: -45dB | Boosted input level hits the tubes harder to compensate for vintage single coils. Tighter gate for 60-cycle hum. |
| **Pre-FX** | Chief OD | Bypass | Active (Drive: 4, Vol: 6) | Higher drive required to push single coils into singing 70s lead territory. |
| **Amp** | Brit Plexi 100 Jumped | Normal: 6.0, Bright: 4.0 | Normal: 7.5, Bright: 5.0 | *(Right-Click Assign)*. Bass at 5.0. Driving the Normal channel hard provides the missing low-mid thickness. |
| **Cab** | 412 Brit 35B | Rib 121 (Mix: +3dB) | Rib 121 (Mix: +3dB) | Ribbon 121 dominant. Rolls off the harsh Tele bridge pickup treble naturally via mic physics. |
| **EQ** | Parametric-8 | Band 1: 200Hz (+3.0dB) | Band 1: 200Hz (+3.0dB) | *The Chameleon Strategy:* Low Shelf at 200Hz adds physical "weight" to the Tele. LPF @ 4.5kHz to tame pick attack. |
| **Post-FX** | Analog Delay | Mix: 0% (Bypass) | Mix: 18% (350ms) | Analog BBD decay rolls off highs, keeping the delays warm and out of the Tele's frequency path. |
| **Post-FX** | Room Reverb | Mix: 15% | Mix: 20% | Slightly higher mix to add perceived dimension to the single coils. |
| **Output** | Lane 1 Output | Level: -2.0dB | Level: -0.5dB | *(Right-Click Assign)*. Lowered globally because we pushed the Amp Vol knobs so high to achieve distortion. |

---

### 🛠 Troubleshooting & Refinement Tree
If the tone is **"Too Farty/Loose"** (common with Plexi models):
1. **Tube Sag Correction:** Lower the Amp Bass parameter down to 2.0 or 1.5. Plexis derive their thickness from Mids, not Bass.
2. **Pre-Amp Roll-off:** Insert a Parametric EQ *before* the Amp block and set a High-Pass Filter (HPF) to 100Hz. This stops low-end frequencies from ever hitting the distortion stage.

If the tone is **"Too Distorted/Fuzzy"** (especially on the ES-339):
1. **Input Pad:** Lower the Input Block Gain to -3.0dB or -6.0dB to simulate rolling off your guitar's volume knob. 
2. **Amp Volume:** Reduce the Vol Bright and Vol Normal by 1.5 increments. Compensate for the volume loss by raising the Lane Output Level.

---

### 📁 Session Registry Protocol
*Session data logged for memory recall.*

**Session Library (Active Presets)**
1. Preset Name: "Spoonful - ES339" [Previously Stored]
2. Preset Name: "70s Plexi Crunch - Dual Mod"
Target: 1970s Classic Rock (Marshall 100w).
Guitar: ES-339 (Humbuckers) / Telecaster (Single Coils).
Physics Goal: Plexi jumped blending without Master Volume. Compensating for single-coil thinness and humbucker mud.
Full Configuration (ES-339): Input [Gain 0dB, Thresh -55dB]. Chief OD [Bypassed / Drv 2, Vol 7]. Amp: Brit Plexi 100 Jumped [Norm 3.5/4.0, Bright 5.5/6.5, Bass 3.0, Mid 6.5, Treb 5.5, Pres 5.0]. Cab: 412 Brit 35B [Mic A 57 CapEdge 1.5, Mic B 121 Cone 3.5, Mix A+2/B-2]. EQ-8 [Band 2 400Hz -1.5dB, LPF 6kHz]. Delay: Analog [Mix 0/15%, 350ms]. Reverb: Room [Mix 12/18%]. Lane Out [0.0 / +1.5dB].
Full Configuration (Tele): Input [Gain +3.0dB, Thresh -45dB]. Chief OD [Bypassed / Drv 4, Vol 6]. Amp: Brit Plexi 100 Jumped [Norm 6.0/7.5, Bright 4.0/5.0, Bass 5.0, Mid 6.0, Treb 4.5, Pres 4.0]. Cab: 412 Brit 35B [Mix A-2/B+3]. EQ-8 [Band 1 200Hz Shelf +3.0dB, LPF 4.5kHz]. Delay: Analog [Mix 0/18%, 350ms]. Reverb: Room [Mix 15/20%]. Lane Out [-2.0 / -0.5dB].