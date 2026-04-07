**QC-2 Systems Engineer: Initialization Complete.**

Analyzing Target Tone: **David Gilmour (Pink Floyd).**
Analog Physics Profile: Massive-headroom, ultra-linear clean amplifier driven into saturation by a mid-scooped, silicon-transistor fuzz circuit. Spatial environment requires dark tape-style repeats cascading into a high-decay plate reverb. 
Speaker Profile: Output is optimized for a **QSC CP12 Active PA Speaker**. To prevent digital harshness and low-end mud through a flat-response driver, strict High-Pass (HPF) and Low-Pass (LPF) filtering will be applied.

### Factory Model Verification & Pseudonym Protocol
*   **Target Amp:** Hiwatt Custom 100 $\rightarrow$ **QC Model:** `Custom 100` (Based on Hiwatt® DR103).
*   **Target Fuzz:** EHX Ram's Head Big Muff $\rightarrow$ **QC Model:** `Fuzz Pi` (Closest available official model to the EHX Big Muff circuit).
*   **Target Cab:** WEM 4x12 (Fane Speakers) $\rightarrow$ **QC Model:** `UK 412 Green 25W` (Closest available high-headroom 4x12. We will emulate the darker Fane driver physics using dual-mic phase blending and EQ).
*   **Target Reverb:** Massive Plate $\rightarrow$ **QC Model:** `Plate` + `Tape Delay` (Binson Echorec analog emulation).

---

### MULTI-GUITAR TARGET OUTPUT

The classic Gilmour tone was built on low-output Stratocaster single coils. A Ram's Head Muff is highly sensitive to input impedance and pickup output. Plugging Gibson ES-339 humbuckers into a preset built for a Fender Telecaster will result in massive low-end mud, choked square-wave clipping, and loss of dynamics. 

We will use the **Split-Bank Matrix** and the **Chameleon Pre-EQ Strategy** to optimize the exact same signal chain for both guitars. 

*   **Row 1 (Scenes A-D):** Fender Telecaster (Single Coil Profile)
*   **Row 2 (Scenes E-H):** Gibson ES-339 (Humbucker Profile)

---

#### PART 1: FENDER TELECASTER CONFIGURATION (SCENES A-D)
**Physics Goal:** Single coils lack the low-end weight to push a Big Muff smoothly. We will boost the global input, enhance the low-mids in the Pre-EQ, and tame the "ice-pick" bridge pickup frequencies before they hit the fuzz.
*   **Global Input Gain:** +1.5dB (Pushes the Fuzz Pi into saturation).
*   **Scene Organization:** Scene A (Clean Rhythm), Scene B (Epic Lead), Scene C (Dry Clean), Scene D (Ambient Swells).

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Gate (Grid)** | Adaptive Gate | Noise Red: 40% <br>Thresh: -55dB | Noise Red: 40% <br>Thresh: -55dB | Single coils require higher Noise Reduction (%) to gate 60-cycle hum before the high-gain Fuzz stage. |
| **Pre-FX (EQ)** | Parametric-8 | Band 1 (Low Shelf): +3.0dB @ 150Hz | Band 8 (LPF): 6000Hz | Adds "Strat-like" body/weight to the Telecaster bridge pickup. LPF prevents high-end harshness hitting the Muff. |
| **Pre-FX (Drive)**| Fuzz Pi | *Bypass* | Fuzz: 70% <br>Tone: 4.5 <br>Level: 6.5 | Engaged for Lead. Telecaster needs higher Fuzz % to sustain. Tone rolled back below 5.0 to tame treble bite. |
| **Amp** | Custom 100 | Normal Vol: 4.5 <br>Brill Vol: 3.5 | Normal Vol: 4.5 <br>Brill Vol: 3.5 | Hiwatt circuit runs hyper-clean. Master Volume: 8.0. Bass: 4.5, Mid: 6.5, Treb: 5.5, Presence: 4.5. |
| **Cab** | UK 412 Green | Mic A: Dyn 57 (Center) <br>Mic B: Ribbon 121 | Mic A: 0dB Mix<br>Mic B: -2dB Mix | Ribbon mic adds the darker, smoother low-mid response characteristic of vintage Fane speakers. |
| **Post-FX (Dly)** | Tape Delay | Mix: 12% <br>Time: 380ms | Mix: 25% <br>Time: 430ms | 430ms is a classic Gilmour solo tempo. Tape emulation rolls off the highs on the repeats. |
| **Post-FX (Rev)** | Plate | Decay: 2.5s <br>Mix: 15% | Decay: 4.0s <br>Mix: 35% | Massive spatial decay for solos. EQ: High Pass @ 120Hz to prevent low-end mud in the QSC CP12. |

---

#### PART 2: GIBSON ES-339 CONFIGURATION (SCENES E-H)
**Physics Goal:** PAF-style humbuckers output significantly more voltage and low-midrange frequencies. If fed directly into a Big Muff, the circuit instantly compresses and "farts out". We must drop the input gain and completely invert the Pre-EQ block to carve out the low-end mud while boosting presence to pierce through the Muff's inherent mid-scoop.
*   **Global Input Gain:** -4.5dB (Essential: simulates rolling off the guitar volume to restore headroom before the Fuzz).
*   **Scene Organization:** Scene E (Clean Rhythm), Scene F (Epic Lead), Scene G (Dry Clean), Scene H (Ambient Swells).

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Gate (Grid)** | Adaptive Gate | Noise Red: 15% <br>Thresh: -65dB | Noise Red: 15% <br>Thresh: -65dB | Humbuckers are inherently quieter. Lower Noise Reduction (%) prevents cutting off natural sustain. |
| **Pre-FX (EQ)** | Parametric-8 | HPF: 110Hz | Band 4 (Peak): +2.5dB @ 800Hz | HPF removes humbucker boominess. The 800Hz mid-boost allows the ES-339 to cut through the Fuzz Pi scoop. |
| **Pre-FX (Drive)**| Fuzz Pi | *Bypass* | Fuzz: 45% <br>Tone: 6.5 <br>Level: 6.0 | Fuzz dropped to 45% (Humbuckers provide natural front-end drive). Tone increased to 6.5 for clarity. |
| **Amp** | Custom 100 | Normal Vol: 4.5 <br>Brill Vol: 4.5 | Normal Vol: 4.5 <br>Brill Vol: 4.5 | Brilliant channel volume increased to 4.5 to add top-end sparkle to the darker mahogany/humbucker guitar. |
| **Cab** | UK 412 Green | Mic A: Dyn 57 (Center) <br>Mic B: Ribbon 121 | Mic A: 0dB Mix<br>Mic B: -5dB Mix | Ribbon mic mix is lowered (-5dB) because the ES-339 naturally produces enough low-mid energy. |
| **Post-FX (Dly)** | Tape Delay | Mix: 12% <br>Time: 380ms | Mix: 25% <br>Time: 430ms | Unchanged from Telecaster. Spatial time-based effects are relatively pickup-agnostic. |
| **Post-FX (Rev)** | Plate | Decay: 2.5s <br>Mix: 15% | Decay: 4.0s <br>Mix: 35% | Unchanged. *Lane Output Level raised +2.0dB to compensate for the -4.5dB Global Input drop.* |

---

### Troubleshooting & Refinement Tree
If routed through your QSC CP12 at gig volume and the Lead Tone (Scene B or F) sounds **Too Fuzzy/Harsh**:
1.  **Input Pad:** Lower the Guitar Input Gain by another -3.0dB. 
2.  **Fuzz Tone:** Reduce the `Fuzz Pi` Tone knob by 1.0. Do not reduce the Amp Treble (the clean tone needs it).
3.  **Speaker Physics:** Check your Global EQ. Ensure a Master Low Pass Filter (LPF) is set around 8kHz-10kHz to cut out the "digital fizz" that the PA speaker horn will unnaturally reproduce.

---

### Session Registry Protocol (Active Presets)
**Appending to Session Library...**

2. **Preset Name:** "Wall of Gilmour - Tele/339"
*Target:* David Gilmour (1973-1979 Era)
*Guitar:* Split-Bank (Fender Telecaster / Gibson ES-339).
*Physics Goal:* High-headroom clean platform driven by silicon fuzz. Compensated for both single coils and humbuckers to prevent clipping and mud.
*Full Configuration:*
*   **Block 1 (Adaptive Gate):** [Tele: Red 40%, Thr -55dB] / [339: Red 15%, Thr -65dB].
*   **Block 2 (EQ-8):** [Tele: Low Shelf +3dB @ 150Hz, LPF 6kHz] / [339: HPF 110Hz, Peak +2.5dB @ 800Hz].
*   **Block 3 (Fuzz Pi):** [Tele: Fuzz 70%, Tone 4.5, Lvl 6.5] / [339: Fuzz 45%, Tone 6.5, Lvl 6.0].
*   **Block 4 (Custom 100):** Vol Norm 4.5, Vol Bright [Tele: 3.5 / 339: 4.5], Bass 4.5, Mid 6.5, Treb 5.5, Pres 4.5, Master 8.0.
*   **Block 5 (UK 412 Green):** Mic A (Dyn 57, Pos 0.0), Mic B (Ribbon 121, Pos 1.5), Mix [Tele: A 0dB, B -2dB] / [339: A 0dB, B -5dB].
*   **Block 6 (Tape Delay):** Mix [Rhy: 12% / Lead: 25%], Time [Rhy: 380ms / Lead: 430ms], Fdbk 30%, HP 150Hz, LP 3000Hz.
*   **Block 7 (Plate):** Mix [Rhy: 15% / Lead: 35%], Decay [Rhy: 2.5s / Lead: 4.0s], Pre-Delay 15ms.