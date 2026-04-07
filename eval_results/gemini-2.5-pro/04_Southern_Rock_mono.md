QC-2 Systems Engineer online. Initiating preset build protocol.

Target parameters identified: Southern Rock slide, dual humbuckers, cranked American Tweed circuit, singing sustain. The primary physics goal is to achieve sustain through a combination of compression and authentic amp saturation, not excessive gain, to maintain note clarity and dynamics. The US Tweed Basslad model is the optimal choice for its headroom and breakup characteristics, mirroring the amps favored by artists like Duane Allman.

To calibrate gain staging accurately, I will proceed assuming a **Medium/High Output** humbucker (e.g., Gibson '57 Classic, Burstbucker) and a **Vintage Output** single-coil (standard Telecaster).

**Preset Architecture: "Tweed Bottle Slide"**

This preset uses a compressor for sustain and an EQ for shaping the lead tone, assigned to separate scenes for maximum control.

### Table A: Main Signal Chain
*(Note: Rhythm scenes are tighter and cleaner; Lead scenes introduce more compression, gain, and a vocal mid-range boost for slide.)*

| Block | Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | Input | Global Input Gate | Threshold: -65.0dB | Threshold: -70.0dB | **Sustain Priority:** A lower threshold in the Lead scene prevents the gate from closing prematurely on decaying slide notes. The Rhythm scene is tighter to manage hum during rests. |
| 2 | Pre-FX | **US FET 76** | (Bypassed) | **Input: 4.5**, Output: 5.0, Attack: 4.0, Release: 6.0, Ratio: 4:1 | **Singing Sustain:** *(Right-Click > Assign to Scene F)* Engaged only for leads. This studio-style FET compressor adds sustain and evens out dynamics without the "squash" of a pedal compressor. Essential for the "singing" quality. |
| 3 | Amp | **Closest Available Model: US Tweed Basslad Jumped** | Vol Norm: **2.8**<br>Vol Bright: **3.5**<br>Bass: 4.0<br>Mid: 6.5<br>Treble: 7.0<br>Presence: 6.0 | Vol Norm: **3.2**<br>Vol Bright: **4.0**<br>Bass: 4.0<br>Mid: 7.5<br>Treble: 6.5<br>Presence: 6.0 | **Non-Master Volume Amp:** The Volume knobs control gain, not overall loudness. Pushing them for the Lead scene *(Assign)* simulates driving the preamp tubes harder. Use **Lane Output Level** for stage volume. |
| 4 | Cab | **410 Basslad PR10** | Mic A: Dyn 57 (Pos 0.5)<br>Mic B: Ribbon 121 (Pos 0.8)<br>Mix A/B: -3.0dB<br>HPF: 90Hz, LPF: 8.0kHz | Mic A: Dyn 57 (Pos 0.5)<br>Mic B: Ribbon 121 (Pos 0.8)<br>Mix A/B: -3.0dB<br>HPF: 90Hz, LPF: 8.0kHz | **Speaker Physics:** The 4x10 Jensen-style speakers provide the classic Tweed breakup. The Dyn 57 captures the mid-range bite, while the Ribbon 121 adds warmth and body. The mix provides a balanced, studio-ready tone. |
| 5 | Post-FX | **Parametric-8** | (Bypassed) | Band 5 Type: Peak<br>Freq: **1.2kHz**<br>Q: **1.00**<br>Gain: **+4.0dB** | **Vocal Mid-Range:** *(Assign to Scene F)* This narrow EQ boost mimics the "vocal" quality of a slide guitar cutting through a mix, emphasizing the harmonics that make notes sing and sustain. |
| 6 | Post-FX | **Spring63 Reverb** | Mix: 20%<br>Dwell: 4.5<br>Tone: 6.0 | Mix: **30%**<br>Dwell: 5.0<br>Tone: 6.0 | **Spatial Goal:** A classic spring reverb adds depth and dimension. The mix is increased for the Lead scene *(Assign)* to give slide notes a larger sense of space without becoming washed out. |

---

### Gain Staging & EQ Calibration: Humbucker vs. Single Coil

This is a critical step. The following adjustments adapt the core preset for the two primary guitar types.

#### **1. Gibson ES-339 / Les Paul (Humbuckers - Medium/High Output)**

*   **Grid Placement:** Use **Row 2 (Scenes E-H)** for this profile.
*   **Input Block Gain:** Set to **-3.0dB**.
    *   **Rationale:** Humbuckers have a higher output that can prematurely clip the input converter and cause unwanted digital harshness. This pad simulates rolling back the guitar's volume knob slightly, preserving headroom and allowing the amp model to break up more naturally.
*   **Amp Settings:** Use the values in the table above as the baseline. They are calibrated for this pickup type.
*   **EQ Adjustment:** If the neck pickup sounds muddy, use a free band on the Parametric-8 EQ block to apply a subtle cut of -2.0dB at 250Hz with a Q of 0.7.

#### **2. Fender Telecaster (Single Coil - Vintage Output)**

*   **Grid Placement:** Use **Row 1 (Scenes A-D)** for this profile.
*   **Input Block Gain:** Set to **+3.0dB**.
    *   **Rationale:** Vintage single coils have a lower output. This clean boost raises the signal level *before* it hits the amp, ensuring the "US Tweed Basslad" model receives enough signal to achieve the same level of saturation and sustain as the humbucker preset.
*   **Amp Settings:** You may need to increase the Amp's **Vol Norm** and **Vol Bright** knobs by an additional 0.5 to 1.0 to achieve singing sustain, as single coils have less natural compression.
*   **"Chameleon" EQ Adjustment (Parametric-8 Block):**
    *   **Band 1 (Body):** Set as **Low Shelf**, Freq **200Hz**, Gain **+2.5dB**. This adds the low-midrange weight and body that single coils lack compared to humbuckers.
    *   **Band 8 (Tame Ice Pick):** Set as **LPF (Low Pass Filter)**, Freq **4.5kHz**. This gently rolls off the extreme high-end "ice pick" frequencies common with Telecaster bridge pickups, which can be harsh for slide playing.

### Scene Configuration Summary

*   **Scene E (Humbucker Rhythm):** Base Tweed tone. No Compressor or EQ.
*   **Scene F (Humbucker Lead):** Compressor, Amp Gain boost, Mid-EQ boost, and more Reverb engaged for slide.
*   **Scene A (Single Coil Rhythm):** Base tone calibrated for Telecaster.
*   **Scene B (Single Coil Lead):** Same as Scene F, but with the specific gain/EQ compensations for the Telecaster applied.
*   **Scenes C/G:** Dry comping tone (Reverb bypassed).
*   **Scenes D/H:** Ambient tone (Reverb/Delay mix increased significantly).

Build complete. Ready for deployment and field testing.

---
### Session Library (Active Presets)

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

2.  **Preset Name:** "Tweed Bottle Slide"
    *   **Target:** Southern Rock slide (Duane Allman / Derek Trucks style).
    *   **Guitar:** Humbucker Primary / Single Coil Secondary.
    *   **Physics Goal:** Singing sustain via compression and amp saturation, with vocal mid-range for slide.
    *   **Full Configuration:**
        *   Block 1 (Global Gate): Threshold [Rhy: -65dB / Lead: -70dB].
        *   Block 2 (Comp - US FET 76): [Bypassed / Lead: Input 4.5, Ratio 4:1].
        *   Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [Rhy: 2.8 / Lead: 3.2], Vol Bright [Rhy: 3.5 / Lead: 4.0], Bass [4.0], Mid [Rhy: 6.5 / Lead: 7.5], Treble [Rhy: 7.0 / Lead: 6.5], Presence [6.0].
        *   Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57), Mic B (Ribbon 121), HPF [90Hz], LPF [8.0kHz].
        *   Block 5 (EQ - Parametric-8): [Bypassed / Lead: +4.0dB @ 1.2kHz, Q 1.0].
        *   Block 6 (Reverb - Spring63): Mix [Rhy: 20% / Lead: 30%].
        *   **Gain Staging Note:** Input Block set to -3.0dB for Humbuckers, +3.0dB for Single Coils.