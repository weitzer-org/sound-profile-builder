### QC-2 Systems Engineering Report: "Chess Records 5E3 Platform"

**Target Acoustic Profile:** Late 1950s Chicago Blues (Chess Records). 
**Analog Circuit Target:** Fender 5E3 Tweed Deluxe (Small 1x12 Combo, cathode-biased).
**Output Translation:** QSC CP12 (Active FRFR). *Note: Because the CP12 reproduces full frequencies (unlike a vintage paper-voice-coil 12" speaker), we must aggressively use High-Pass (HPF) and Low-Pass (LPF) filtering to prevent modern "hi-fi" artifacts from ruining the vintage illusion.*

***

#### ⚠️ Pre-Flight Verification: Pickup Output Strategy
To accurately simulate a non-master volume 5E3 Tweed amp, your guitar's output heavily dictates the overdrive. 
*Trigger Verification:* Are the pickups in your ES-339 and Telecaster Vintage (PAF/Broadcaster), Medium, or High Output? 
*Assumption for this build:* Vintage-to-Medium output. We will pad the ES-339's input to prevent catastrophic low-end "flub" (tube sag collapse) typical of humbuckers pushing a Tweed circuit.

***

### 🎸 PROFILE 1: Fender Telecaster (Single Coils)
**Deployment:** Row 1 (Scenes A - D)
**Physics Goal:** Add body/weight to the bridge pickup while taming the 60-cycle hum. Force the amp into "edge-of-breakup" via input volume manipulation.

**Table A: Telecaster Signal Chain**
*(Note: Right-Click > Assign to parameters with dual values for Scenes A & B)*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -60dB, Red: 40% | Thresh: -60dB, Red: 40% | Targets 60-cycle hum. Red at 40% preserves sustain. |
| **Pre-FX** | Parametric-8 | Band 1: Low Shelf +2.5dB @ 200Hz | Band 2: Peak +1.5dB @ 800Hz | Adds "wood/body" to single coils. Mid-push for Lead cut. |
| **Amp** | US DLX 58 | Inst Vol: 4.0, Tone: 4.5 | Inst Vol: 6.5, Tone: 5.5 | **No Master Vol.** Tube saturation achieved via Inst Vol. Tone increased to counter sag. |
| **Cab** | 112 US DLX | Mic A: Dyn 57 (0.5 Pos, 1.0" Dist) | Mic A + B (Mix: +0dB A / -3dB B) | Mic B: Ribbon 121 (1.0 Pos, 4.0" Dist). Ribbon darkens the aggressive Tele attack. |
| **Post-FX 1** | Tape Delay | Mix: 12%, Time: 110ms | Mix: 18%, Time: 120ms | Slapback echo. Essential for 50s Chess Records acoustic depth. |
| **Post-FX 2** | Room Reverb | Mix: 15%, Decay: 0.8s | Mix: 18%, Decay: 1.0s | Simulates physical studio tracking room. LPF @ 3kHz to sink it behind the amp. |
| **Global** | Lane Output | Level: +3.0dB | Level: +1.5dB | *The Headroom Rule:* As Amp Vol goes up (Lead), Lane Output must drop to match overall SPL. |

**Telecaster Scene Mapping (Row 1):**
*   **A (Rhythm):** Edge of breakup. Dynamic to pick attack.
*   **B (Lead):** Gritty, mid-pushed 5E3 overdrive.
*   **C (Dry/Comping):** Bypasses Delay and Reverb blocks for dry, in-your-face rhythm.
*   **D (Ambient/FX):** Increases Delay Mix to 25% for exaggerated "Sun Studios/Chess" slapback.

***

### 🎸 PROFILE 2: Gibson ES-339 (Humbuckers)
**Deployment:** Row 2 (Scenes E - H)
**Physics Goal:** Prevent the 5E3 circuit from "choking" or getting overly fuzzy/farty in the low-end, a common physical reaction when pushing cathode-biased amps with humbuckers. 

**Table B: ES-339 Signal Chain**
*(Note: Right-Click > Assign to parameters with dual values for Scenes E & F)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB, In Pad: -3.0dB | Thresh: -65dB, In Pad: -3.0dB | **Crucial:** -3dB Input Pad prevents humbuckers from clipping the front-end DSP block. |
| **Pre-FX** | Parametric-8 | HPF: 120Hz, Band 8: LPF @ 4.5kHz | HPF: 120Hz, Band 8: LPF @ 4.5kHz | Aggressive HPF prevents bass frequencies from collapsing the Tweed's power section. |
| **Amp** | US DLX 58 | Inst Vol: 2.5, Tone: 6.5 | Inst Vol: 4.5, Tone: 7.5 | Humbuckers drive the amp much faster. Vol 4.5 here equals Vol 6.5 on the Telecaster. |
| **Cab** | 112 US DLX | Mic A: Dyn 57 (0.0 Pos, 1.0" Dist) | Mic A + B (Mix: +0dB A / -6dB B) | Center position on the 57 extracts more bite/clarity from the darker humbuckers. |
| **Post-FX 1** | Tape Delay | Mix: 10%, Time: 110ms | Mix: 15%, Time: 120ms | Humbuckers have a wider frequency spread; lowering mix prevents masking the fundamental. |
| **Post-FX 2** | Room Reverb | Mix: 12%, Decay: 0.8s | Mix: 15%, Decay: 1.0s | Slight reduction in mix vs. Tele to maintain definition. |
| **Global** | Lane Output | Level: +5.0dB | Level: +3.0dB | Compensating for the heavily reduced Amp Inst Vol to match the Telecaster's SPL on the QSC. |

**ES-339 Scene Mapping (Row 2):**
*   **E (Rhythm):** Clean, warm jazz/blues rhythm. Highly articulate.
*   **F (Lead):** Thick, saturated overdrive.
*   **G (Dry/Comping):** Reverb/Delay Bypassed.
*   **H (Ambient/FX):** Delay Mix to 20%.

***

### 🔧 Troubleshooting & Refinement Tree
If the CP12 speaker is projecting a tone that is "Too Distorted" or "Too Fuzzy" (especially on the ES-339):
1.  **Input Pad:** Lower Global Input Level to -6.0dB (simulating rolling off your guitar's volume knob).
2.  **Amp Gain:** Lower the `US DLX 58` Inst Vol by 1.0 increments. 
3.  **Tube Sag Control:** If the amp sounds "broken" or farty on the low E string, raise the HPF frequency on the Parametric-8 block from 120Hz to 150Hz. 
4.  **Output Compensation:** Never use a compressor to fix this volume loss. Raise the Lane Output Level to recover SPL.

***

### 🗄️ Session Library (Active Presets)

**2. Preset Name: "Chess 5E3 Combo - Dual Rig"**
*   **Target:** 1950s Chicago Blues / Chess Records Overdrive.
*   **Guitar:** Fender Telecaster (Single) & Gibson ES-339 (Humbucker).
*   **Physics Goal:** Manage tube sag and low-end distortion on a non-master volume circuit while translating smoothly to a modern FRFR PA speaker. 
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Tele: 40% / 339: Bypassed], Thresh [-60dB]. (Global Input Pad [-3.0dB] for 339).
    *   **Block 2 (EQ-8):** [Tele: Low Shelf +2.5dB @ 200Hz] / [339: HPF 120Hz, LPF 4.5kHz]. 
    *   **Block 3 (Amp - US DLX 58):** Inst Vol [Tele Rhy: 4.0, Lead: 6.5] / [339 Rhy: 2.5, Lead: 4.5], Mic Vol [0.0], Tone [Tele: 4.5 / 339: 6.5].
    *   **Block 4 (Cab - 112 US DLX):** Mic A (Dyn 57, Pos [Tele: 0.5 / 339: 0.0], Dist 1.0"), Mic B (Ribbon 121, Pos 1.0, Dist 4.0"), Mix [A: 0dB, B: -3dB(Tele)/-6dB(339)].
    *   **Block 5 (Tape Delay):** Mix [12% Rhy / 18% Lead], Time [110ms], Fdbk [15%], Drive [25%], HP [150Hz], LP [2500Hz].
    *   **Block 6 (Room Reverb):** Mix [15%], Decay [0.8s], Pre-Delay [15ms], HP [120Hz], LP [3000Hz].
    *   **Lane Output:** [Tele Rhy: +3.0dB / Lead: +1.5dB] | [339 Rhy: +5.0dB / Lead: +3.0dB].