### QC-2 Preset Build: "Muddy Waters - Edge-of-Fuzz"

**Target Verification:** Muddy Waters (Chicago Blues). Telecaster in middle position. Pushed 5E3 (Tweed Deluxe) edge-of-fuzz.
**FRFR Compensation (QSC CP12):** PA speakers expose the harsh ultra-highs of clipping vintage amp models. We will aggressively filter above 5.5kHz to ensure the speaker behaves like an analog 12" Jensen.
**Physics First (The "Flub" Problem):** The 5E3 circuit lacks negative feedback and has a notoriously loose low-end. Pushing the volume for "fuzz" usually causes low-end intermodulation distortion (flub). **Solution:** We are deploying a Pre-Amp EQ-8 block to high-pass (HPF) the guitar signal *before* it hits the amp. This starves the tube grid of sub-bass, keeping the fuzz tight, while allowing the mid-range to bark.

#### **Pickup Compensation Strategy (Telecaster Middle Position)**
Your Telecaster middle position is bright, slightly scooped, and vintage output. We will run the Input Gate at default, push the Amp Volume high for natural tube compression (fuzz), and use the Pre-EQ to add low-mid "weight" (250Hz) while cutting out the sub-bass that causes the 5E3 to fart out.

***

### **Preset Matrix: Split-Bank Standard**
*   **Row 1 (Scenes A-D): Telecaster Profile** (Higher Amp Gain, Mid-Body Boost)
*   **Row 2 (Scenes E-H): Humbucker Profile** (-3.0dB Input Pad, Lower Amp Gain to prevent modern pickups from destroying the vintage headroom).

**Table A: Main Signal Chain (Row 1 - Telecaster Focus)**
*Note: Mark Scene-Specific changes clearly with `(Right-Click > Assign)`.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -65dB | Vintage singles have hum, but we don't want to choke the soft blues dynamics. |
| **Pre-FX** | Parametric-8 EQ | HPF: 120Hz (12dB/oct)<br>Band 2 (250Hz): +1.5dB | HPF: 120Hz<br>Band 2 (250Hz): +2.5dB | **Crucial Anti-Flub:** Cuts sub-bass *before* the amp so the 5E3 doesn't choke. Boosts Tele body. |
| **Amp** | US DLX Tweed Jump | Inst Vol: 6.5<br>Mic Vol: 4.0<br>Tone: 6.0 | Inst Vol: 8.5<br>Mic Vol: 4.5<br>Tone: 6.5 | **Non-Master Vol:** Pushing Inst Vol past 7 creates the iconic Tweed edge-of-fuzz. Mic Vol adds low-mid thickness. |
| **Cab** | 112 US DLX Tweed | Mic A: Dyn 57 (Pos 0.5, Dist 1")<br>Mic B: Rib 121 (Pos 1.0, Dist 3") | Mix A: 0dB<br>Mix B: -3dB<br>LPF: 5500Hz | Ribbon 121 smooths the Telecaster pick attack. LPF at 5.5kHz stops the QSC CP12 from sounding "digital/fizzy". |
| **Post-FX** | Spring Reverb | Mix: 12%<br>Decay: 1.2s | Mix: 15%<br>Decay: 1.5s | Emulates the natural splash of a small Chicago blues club or Chess Records room. |

***

### **Hardware & Gain Staging Protocol**

1.  **Output Volume Compensation:** Because a 5E3 has no Master Volume, Scene B (Lead) with `Inst Vol: 8.5` will be physically louder than Scene A (Rhythm) with `Inst Vol: 6.5`. Use the **Lane Output Level** (Right-click the output circle) to pull Scene B down by about `-2.5dB` so it matches Scene A's SPL, allowing the fuzz to increase without blowing out the QSC speaker.
2.  **Too Fuzzy/Muddy on Sc A?** Do *not* touch the tone knob. Lower the `Inst Vol` to 5.5.
3.  **Humbucker Adaptation (Scenes E-H):** If you plug in a Gibson, go to the Input Block and drop the Input Gain to `-4.5dB`. The Tweed circuit collapses under high-output humbuckers; rolling off the input gain simulates rolling back the guitar's volume knob, restoring the edge-of-breakup clarity.

***

### **Session Library (Active Presets)**

1. Preset Name: "Spoonful - ES339"
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

2. Preset Name: "Muddy Waters Edge-Fuzz"
Target: Muddy Waters (Chicago Blues).
Guitar: Telecaster (Middle Position, Vintage Single Coils).
Physics Goal: Pushed Tweed edge-of-fuzz, completely mitigating 5E3 low-end flub for FRFR.
Full Configuration:
Block 1 (Global Input): Gate Thresh [-65dB], Input Gain [0.0dB for Tele / -4.5dB for Humbucker].
Block 2 (EQ-8 - Anti-Flub): HPF [120Hz at 12dB/oct], Band 2 [250Hz, +1.5dB Rhy / +2.5dB Lead], Band 8 LPF [Off].
Block 3 (Amp - US DLX Tweed Jump): Inst Vol [6.5 Rhy / 8.5 Lead], Mic Vol [4.0 / 4.5], Tone [6.0 / 6.5].
Block 4 (Cab - 112 US DLX Tweed): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Rib 121, Pos 1.0, Dist 3.0"), Mix [A: 0dB, B: -3dB], LPF [5500Hz].
Block 5 (Spring Reverb): Mix [12% Rhy / 15% Lead], Decay [1.2s / 1.5s], Tone [Standard].
Lane Output Compensation: Scene B (Lead) reduced by -2.5dB relative to Scene A to balance SPL increase from Amp Volume push.