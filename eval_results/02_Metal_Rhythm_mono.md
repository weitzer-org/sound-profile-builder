Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To achieve a modern, high-gain Peavey 5150 tone with a tight low-end and scooped mids, we must strictly manage signal-to-noise ratio and low-frequency physics. High-gain amplifier models easily succumb to "flub" or "mud" if fed too much bass frequencies from the guitar's pickups prior to the distortion stages. 

To combat this, we will employ a classic modern-metal physics trick: placing a **Green 808** (Ibanez Tube Screamer) *before* the **PV-505 Lead** (Peavey 5150) amp block. The Green 808 acts as an active high-pass filter and clean boost, tightening the low-end perfectly for palm-muted chugs. 

Per your parameters, I have engineered a Split-Bank preset. **Row 1 (Scenes A-D)** is meticulously calibrated for a **Fender Telecaster (Single Coils)**, while **Row 2 (Scenes E-H)** is optimized for a **Gibson ES-339 (Humbuckers)**.

***

### 🎸 PROFILE 1: Fender Telecaster (Single Coils)
**Target:** Row 1 (Scenes A-D)
**Gain Staging Protocol:** Telecasters inherently lack the low-end output to push a high-gain amp into modern metal saturation. We must increase the Input Block Gain to **+2.5dB**. We will also use the EQ-8 block to boost the "body" at 200Hz, and a harsh Low Pass Filter (LPF) to tame the 60-cycle single-coil hum and ice-pick high-end fizz.

**Table A: Main Signal Chain (Telecaster)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input Gate | Thresh: -45.0dB | Thresh: -50.0dB | Aggressive threshold needed to combat 60-cycle hum at high gain. |
| **Gate** | Adaptive Gate | Noise Red: 65% | Noise Red: 40% | Grid block. High reduction for tight staccato chugging. Lowered for lead sustain. |
| **Pre-FX 1** | Green 808 | Drive: 2.0, Tone: 6.0, Level: 9.0 | Drive: 3.5, Tone: 6.0, Level: 9.0 | Slight drive added to compensate for lower-output single coils. |
| **Pre-FX 2** | Parametric-8 EQ | Band 1: +3dB (200Hz), Band 4: -3dB (500Hz), LPF: 5500Hz | Band 1: +3dB (200Hz), Band 4: +2dB (800Hz), LPF: 6000Hz | Chameleon strategy: Adds humbucker-like low-end weight. Rhythm scooped, Lead mid-boosted. |
| **Amp** | PV-505 Lead | Pre Gain: 6.0, Low: 6.5, Mid: 3.5, High: 4.5, Presence: 4.0 | Pre Gain: 6.5, Low: 6.0, Mid: 5.5, High: 4.5, Presence: 4.5 | High Pre Gain for single-coil saturation. Highs/Presence rolled off to prevent fizz. |
| **Cab** | 412 US Recto OS V30 | Mic A: Dyn 57 (Pos 0.0) <br>Mic B: Ribbon 121 (Pos 1.5) | Mic A: Dyn 57 (Pos 0.0) <br>Mic B: Ribbon 121 (Pos 1.5) | Mesa Oversized Cab. Ribbon mic adds missing low-end resonance to Telecaster. |
| **Post-FX 1** | Digital Delay | Mix: 0% | Mix: 25%, Time: 350ms, Fdbk: 25% | Adds spatial width and tail specifically for soaring lead lines. |
| **Post-FX 2** | Plate Reverb | Mix: 8%, Decay: 1.2s | Mix: 18%, Decay: 1.5s | Short decay to prevent metal riffs from getting washed out in muddy reflections. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | Overall loudness bump for solos without altering amp saturation. |

***

### 🎸 PROFILE 2: Gibson ES-339 (Humbuckers)
**Target:** Row 2 (Scenes E-H)
**Gain Staging Protocol:** The ES-339 has medium/high-output humbuckers with a dominant low-mid resonance. Pushing this directly into a PV-505 will cause immediate, muddy clipping. We must lower the Input Block Gain to **-2.0dB**. The Green 808 will be run as a pure clean boost (0 Drive) to carve out the mud before it hits the preamp tubes.

**Table B: Main Signal Chain (ES-339)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input Gate | Thresh: -60.0dB | Thresh: -65.0dB | Humbuckers are quieter at baseline; we can relax the threshold for better sustain. |
| **Gate** | Adaptive Gate | Noise Red: 50% | Noise Red: 20% | Grid block. Tight cutoff for rhythmic syncopation, minimal reduction for leads. |
| **Pre-FX 1** | Green 808 | Drive: 0.0, Tone: 7.5, Level: 10.0 | Drive: 0.0, Tone: 7.0, Level: 10.0 | Pure active high-pass filter. Zero drive prevents humbucker fuzz. Level slams amp preamp. |
| **Pre-FX 2** | Parametric-8 EQ | HPF: 80Hz, Band 2: -2dB (250Hz), Band 4: -4dB (500Hz) | HPF: 80Hz, Band 2: -2dB (250Hz), Band 4: +2.5dB (800Hz) | HPF and Band 2 cut removes inherent ES-339 semi-hollow "mud". Band 4 dictates scoop vs. lead. |
| **Amp** | PV-505 Lead | Pre Gain: 4.5, Low: 4.5, Mid: 3.0, High: 5.5, Presence: 5.5 | Pre Gain: 5.0, Low: 4.0, Mid: 5.5, High: 5.5, Presence: 5.5 | Pre Gain/Low kept moderate; humbuckers provide the saturation naturally. |
| **Cab** | 412 US Recto OS V30 | Mic A: Dyn 57 (Pos 0.5) <br>Mic B: Cond 87 (Pos 1.5) | Mic A: Dyn 57 (Pos 0.5) <br>Mic B: Cond 87 (Pos 1.5) | Condenser mic used instead of Ribbon to retain clarity in humbucker low-mids. |
| **Post-FX 1** | Digital Delay | Mix: 0% | Mix: 25%, Time: 350ms, Fdbk: 25% | Retained from profile 1. |
| **Post-FX 2** | Plate Reverb | Mix: 6%, Decay: 1.2s | Mix: 15%, Decay: 1.5s | Retained from profile 1. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | Solo volume boost. |

***

### 🔧 Troubleshooting & Refinement Tree
If the tone sounds **"Too Fuzzy"** or the palm mutes aren't **"Tight Enough"**:
1. **Input Pad:** Lower the Global Input Block Gain by an additional -3.0dB (simulates rolling off your guitar volume knob to regain clarity).
2. **Amp Gain:** Reduce the PV-505 Pre Gain by 1.0. Do *not* reduce the Green 808 Level; it is providing your tightness.
3. **Tube Sag/Flub:** If the palm mutes sound "farty" or overwhelming, increase the HPF in the Parametric-8 EQ to `100Hz`, and reduce the Amp's `Resonance` parameter.
4. **Output Compensation:** If step 1 and 2 drop your overall volume too much, compensate by raising the Lane Output Level.

***

### 🗄️ Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"** *(From Previous Memory)*
Target: Howlin' Wolf / Hubert Sumlin (1960).

**2. Preset Name: "505 Modern Metal - Tele/ES339 Dual"**
**Target:** Modern High-Gain Metal Rhythm / Peavey 5150.
**Guitar:** Split-Bank (Telecaster Row 1 / ES-339 Row 2).
**Physics Goal:** Extreme saturation with zero low-end flub; surgically scooped mids for rhythm via EQ blocks rather than amp controls to maintain tube modeling stability.
**Full Configuration (Telecaster SCENES A-D):**
* Block 1 (Adaptive Gate): Noise Red [Rhy: 65% / Lead: 40%].
* Block 2 (Green 808): Drive [2.0 / 3.5], Tone [6.0], Level [9.0].
* Block 3 (EQ-8): Band 1 [+3.0dB @ 200Hz], Band 4 [-3.0dB @ 500Hz Rhy / +2.0dB @ 800Hz Lead], LPF [5500Hz / 6000Hz].
* Block 4 (Amp - PV-505 Lead): Pre Gain [6.0 / 6.5], Low [6.5], Mid [3.5 / 5.5], High [4.5], Presence [4.0], Resonance [6.0].
* Block 5 (Cab - 412 US Recto OS V30): Mic A (Dyn 57, Pos 0.0), Mic B (Ribbon 121, Pos 1.5), Mix [50/50].
* Block 6 (Digital Delay): Mix [0% / 25%], Time [350ms], Fdbk [25%].
* Block 7 (Plate Reverb): Mix [8% / 18%], Decay [1.2s / 1.5s].
* Output Lane 1: [+0.0dB / +1.5dB].

**Full Configuration (ES-339 SCENES E-H):**
* Block 1 (Adaptive Gate): Noise Red [Rhy: 50% / Lead: 20%].
* Block 2 (Green 808): Drive [0.0], Tone [7.5 / 7.0], Level [10.0].
* Block 3 (EQ-8): HPF [80Hz], Band 2 [-2.0dB @ 250Hz], Band 4 [-4.0dB @ 500Hz Rhy / +2.5dB @ 800Hz Lead].
* Block 4 (Amp - PV-505 Lead): Pre Gain [4.5 / 5.0], Low [4.5 / 4.0], Mid [3.0 / 5.5], High [5.5], Presence [5.5], Resonance [4.0].
* Block 5 (Cab - 412 US Recto OS V30): Mic A (Dyn 57, Pos 0.5), Mic B (Cond 87, Pos 1.5), Mix [50/50].
* Block 6 & 7 (Delay/Reverb): Same as Telecaster parameters.
* Output Lane 2: [+0.0dB / +1.5dB].