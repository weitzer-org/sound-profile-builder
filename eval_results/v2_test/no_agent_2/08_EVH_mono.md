**QUAD CORTEX SYSTEMS ENGINEER (QC-2) // SESSION INITIATED**
**TARGET PROFILE:** "Van Halen Brown Sound" (1968 Marshall Super Lead, Hot-Rodded w/ Variac Sag & Plate Reverb).
**HARDWARE ENVIRONMENT:** Cortex Control (Mac Desktop) -> Quad Cortex -> QSC CP12 (FRFR Active PA Speaker).

To faithfully recreate the "Brown Sound" on the Quad Cortex, we must adhere strictly to the physics of Eddie Van Halen’s rig. EVH used a 100-watt Marshall Super Lead (Model 1959) run into a dummy load or variable voltage transformer (Variac) dropping the wall voltage from 120V to around 90V. This voltage starvation creates massive power amp compression, earlier breakup, and a spongy, "sagging" feel. 

Because vintage Plexis lack a Master Volume, increasing preamp drive means massive SPL output. We will bypass this physical constraint using the QC's **Lane Output Level** to control stage volume via your QSC CP12, allowing us to crank the Amp Block volumes for pure tube saturation.

Below are the distinct, physically separated configurations for your Humbucker and Single Coil instruments.

---

### PART 1: GIBSON ES-339 (HUMBUCKERS) CONFIGURATION
**Target Output Row:** Row 2 (Scenes E-H)
**Pickup Output Compensation Strategy:** Standard PAF-style output. No global input boost required. Gate threshold lowered to preserve note decay and harmonics.

**Table A: Main Signal Chain (ES-339)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign) in Cortex Control.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -60dB<br>Red: 40% | Thresh: -60dB<br>Red: 35% | Humbuckers reject 60-cycle hum naturally. Lower reduction allows for ringing "Eruption" style harmonics. |
| **Pre-FX** | Phaser 95 | *Bypassed* | *Active on Sc H (FX)*<br>Rate: 2.0 | Emulates the MXR Phase 90 placed *before* the amp for chewy, swept mid-range (e.g., "Ain't Talkin' 'Bout Love"). |
| **Amp** | Brit Super Lead Jumped | Vol Bright: 6.5<br>Vol Normal: 3.5 | Vol Bright: 8.5<br>Vol Normal: 4.0 | **No Master Volume.** Jumped channels blend bright aggression with low-end chunk. (Assign to Scenes). |
| **Amp Deep Tweak** | *Amp Block Settings* | Sag: 7.5<br>Bias: -1.0 | Sag: 8.5<br>Bias: -1.0 | **Variac Simulation:** Dropping bias and increasing tube Sag replicates the voltage-starved compression of the Brown Sound. |
| **Amp EQ** | *Amp Block Settings* | Bass: 2.5<br>Mid: 7.0<br>Tre: 6.5 | Bass: 2.5<br>Mid: 7.5<br>Tre: 6.0 | Extreme Sag creates flubby lows. We must drastically cut Bass to keep the QSC CP12 speaker tight during palm mutes. |
| **Cab** | 412 Brit Green | Mic A: Dyn 57 (Pos 0.4)<br>Mic B: Rib 121 (Pos 1.2) | Mic A: Dyn 57 (Pos 0.4)<br>Mic B: Rib 121 (Pos 1.2) | Celestion Greenbacks. Rib 121 adds body. **Crucial:** Set Cab LPF to 6.5kHz to tame digital fizz in your FRFR speaker. |
| **Post-FX** | Tape Delay | Mix: 10%<br>Time: 350ms | Mix: 22%<br>Time: 350ms | EP-3 Echoplex emulation. Pushed into the background for Rhythm, brought forward for Eruption/Solos. |
| **Post-FX** | Lush Plate | Mix: 15%<br>Decay: 1.8s | Mix: 25%<br>Decay: 2.5s | Plate reverb simulates the Sunset Sound studio echo chambers. High-Pass the reverb tail at 200Hz to prevent low-end mud. |

---

### PART 2: FENDER TELECASTER (SINGLE COIL) CONFIGURATION
**Target Output Row:** Row 1 (Scenes A-D)
**Pickup Output Compensation Strategy:** Vintage single coils lack the midrange output and bass response to drive the Plexi block into proper Brown Sound saturation. We will compensate by utilizing a Global Input Gain boost (+3.0dB) and an active EQ block to simulate a humbucker's physical footprint.

**Table B: Main Signal Chain (Telecaster)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign) in Cortex Control.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global In / Adaptive Gate | Global In: +3.0dB<br>Thresh: -50dB / Red: 60% | Global In: +3.0dB<br>Thresh: -50dB / Red: 55% | +3.0dB hits the Plexi pre-tubes harder. Higher Noise Reduction (%) combats 60-cycle hum from single coils under high gain. |
| **Pre-FX (EQ)** | Parametric-8 | *Active*<br>Band 2: +3dB @ 250Hz | *Active*<br>Band 2: +3dB @ 250Hz | **Chameleon Strategy:** Boosts the 250Hz "body" of the Telecaster to simulate a humbucker's magnetic field. |
| **Pre-FX (EQ)** | Parametric-8 | Band 8: LPF @ 4.5kHz | Band 8: LPF @ 5.0kHz | Tames the sharp transient pick attack inherent to Telecaster bridge pickups, preventing "ice-pick" tones. |
| **Pre-FX** | Phaser 95 | *Bypassed* | *Active on Sc D (FX)*<br>Rate: 2.0 | Classic EVH pre-gain phasing. |
| **Amp** | Brit Super Lead Jumped | Vol Bright: 7.5<br>Vol Normal: 4.5 | Vol Bright: 9.5<br>Vol Normal: 5.0 | Notice volumes are +1.0 higher than the ES-339. This mechanically compensates for the weaker single-coil output. |
| **Amp Deep Tweak** | *Amp Block Settings* | Sag: 8.0<br>Bias: -1.0 | Sag: 9.0<br>Bias: -1.0 | Higher Sag required here to induce artificial tube compression, smoothing out the Telecaster's snappy dynamics. |
| **Cab** | 412 Brit Green | Mic A: Dyn 57 (Pos 0.6)<br>Mic B: Rib 121 (Pos 1.0) | Mic A: Dyn 57 (Pos 0.6)<br>Mic B: Rib 121 (Pos 1.0) | Mics pushed slightly further off-center (Pos 0.6) compared to the Gibson to roll off the Telecaster's high-frequency bite. |
| **Post-FX** | Tape Delay | Mix: 10%<br>Time: 350ms | Mix: 22%<br>Time: 350ms | Constant Echoplex slap. |
| **Post-FX** | Lush Plate | Mix: 15%<br>Decay: 1.8s | Mix: 25%<br>Decay: 2.5s | Classic Sunset Sound spatial emulation. |

---

### Troubleshooting & Refinement Tree (FRFR Specific)
If you are playing through the **QSC CP12** and the tone feels "Too Distorted," "Harsh," or "Fuzzy," execute the following physical corrections in CorOS:
1. **Input Pad:** Lower the Input Block Gain to -3.0dB (This simulates rolling your guitar's physical volume knob down to 8, which Eddie frequently did).
2. **High-Frequency Fizz:** If the QSC tweeter sounds harsh, go to the **Cab Block** and lower the Low Pass Filter (LPF) from 6.5kHz down to 5.0kHz.
3. **Amp Flub/Farting:** The Variac (Sag) setting introduces low-end instability. If Palm Mutes sound broken, reduce the Amp Block Bass parameter by 1.0 increment. *Never use a compressor block to fix Plexi gain staging.*
4. **Volume Loss Correction:** If cleaning up the amp lowers your stage volume too much, DO NOT touch the Amp Volume knobs. Raise the **Lane Output Level** by +2.0dB.

---

### Session Library (Active Presets)
**2. Preset Name:** "Brown Sound - Multi-Instrument"
*Target:* Eddie Van Halen (1978).
*Guitar:* Gibson ES-339 (Humbuckers) / Fender Telecaster (Single Coil).
*Physics Goal:* Emulate a variac-starved 1968 Marshall 1959 Super Lead via deep-tweak Sag adjustments. Keep SPL managed for active FRFR PA speakers.
*Full Configuration:*
**Block 1 (Adaptive Gate):** Noise Red [Tele: 60% / 339: 40%], Thresh [-50dB / -60dB], Decay [150ms].
**Block 2 (EQ-8 - Telecaster Only):** Peak Band 2 [+3.0dB @ 250Hz], LPF [4500Hz].
**Block 3 (Phaser 95):** Rate [2.0], Mix [100% - placed Pre-Amp].
**Block 4 (Amp - Brit Super Lead Jumped):** Vol Bright [Rhy: 6.5 / Lead: 8.5 (Add +1.0 for Tele)], Vol Normal [Rhy: 3.5 / Lead: 4.0], Bass [2.5], Mid [7.0], Treble [6.5], Presence [7.0]. *Deep Tweak:* Sag [7.5 to 9.0], Bias [-1.0].
**Block 5 (Cab - 412 Brit Green):** Mic A (Dyn 57, Pos 0.4, Dist 1.0"), Mic B (Ribbon 121, Pos 1.2, Dist 1.5"), Mix [A: 0dB, B: -2dB], LPF [6500Hz].
**Block 6 (Tape Delay):** Mix [10% / 22%], Time [350ms], Fdbk [25%], Drive [10%].
**Block 7 (Lush Plate):** Mix [15% / 25%], Decay [1.8s / 2.5s], HP [200Hz], LP [4000Hz].