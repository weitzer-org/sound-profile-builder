### Quad Cortex Systems Engineer (QC-2) Log: Southern Rock Slide Profile
**Target:** Southern Rock Slide (Capricorn Studios era – Duane Allman / Dickey Betts vibe). 
**Amp Architecture:** Cranked American Tweed. Singing sustain achieved through preamp tube compression and speaker drive, augmented by mid-pushed overdrive, rather than excessive modern gain. 
**Output Profile:** QSC CP12 (Active FRFR). 

To achieve "singing sustain" with a slide, we must carefully manage the signal-to-noise ratio. Excessive gain will cause harsh, unmusical overtones with a brass or glass slide. Instead, we use a classic Tweed circuit (Fender Bassman 5F6-A) pushed to edge-of-breakup, relying on a **Green 808** (TS-Style) pedal to compress and push the midrange for the lead scenes.

Per protocol, I have engineered this preset to accommodate both your dual-humbucker requirement and a single-coil alternative via the **Split-Bank Matrix**.

---

### 1. Pickup Compensation & Gain Staging Strategy
*Crucial Physics Note: Cranked Tweed amps have massive, loose low-end. Humbuckers will cause "blocking distortion" (farty/sputtering bass) if not managed at the input stage.*

**Target 1: Gibson ES-339 (Humbuckers) - Row 2 (Scenes E-H)**
*   **Input Block Gain:** Set to **-4.0dB**. This prevents the high-output humbuckers from overwhelming the Tweed’s phase inverter, maintaining clarity for slide articulation.
*   **EQ-8 Block:** Bypassed for Rhythm. For Lead, use a High-Pass Filter (HPF) at 120Hz to keep the low-mids tight.
*   **Gate:** Threshold set very low (-68dB) to prevent cutting off the natural decay of the slide.

**Target 2: Fender Telecaster (Single Coil) - Row 1 (Scenes A-D)**
*   **Input Block Gain:** Set to **+2.5dB** to hit the amp's front end hard enough to induce natural sag and compression.
*   **EQ-8 Block:** Active. Low Shelf at 200Hz (+3.5dB) to simulate the thicker body of a humbucker. Low Pass Filter (LPF) at 4.5kHz to tame the harsh pick attack of the bridge pickup against the slide. 
*   **Gate:** Threshold slightly higher (-62dB) as single coils inherit 60-cycle hum.

---

### 2. Main Signal Chain (Split-Bank Matrix)

#### Table A: Main Signal Chain
*(Right-Click > Assign to Scene for any parameter with multiple values)*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Gain: [Tele: +2.5dB / 339: -4.0dB]<br>Thresh: -68dB | Gain: [Tele: +2.5dB / 339: -4.0dB]<br>Thresh: -68dB | extremely low threshold to preserve slide sustain trails. Input gain offsets pickup physics. |
| **Pre-FX (EQ)** | Parametric-8 | *Tele: Active (See Strategy)*<br>*339: Bypassed* | *Tele: Active*<br>*339: HPF 120Hz Active* | The "Chameleon" block. Tailors guitar output to match the Tweed's optimal frequency response. |
| **Pre-FX (Drive)**| Green 808 | **Bypassed** | **Active**<br>Drive: 1.5<br>Tone: 6.5<br>Level: 8.0 | Tightens loose Tweed bass. Compresses the signal and pushes 800Hz for singing slide sustain. |
| **Amp** | US Tweed Basslad Jumped | Vol Norm: 3.5<br>Vol Bright: 4.0<br>Bass: 2.0<br>Mid: 6.5<br>Treble: 6.0<br>Presence: 5.5 | Vol Norm: 4.5<br>Vol Bright: **6.0**<br>Bass: 2.0<br>Mid: **7.5**<br>Treble: 6.0<br>Presence: 5.5 | *No Master Volume on this model.* Bass kept at 2.0 to prevent low-end collapse. Mids pushed for vocal quality. |
| **Cab** | 410 Basslad PR10 | Mic A: Dyn 57 (Pos 0.2, Dist 1")<br>Mic B: Ribbon 160 (Pos 0.5, Dist 4")<br>Mix: A(-2dB), B(0dB) | *(Same)* | Dyn 57 provides string articulation. Ribbon 160 (Beyer M160) tames the harsh "glass" resonant peaks of the slide. |
| **Post-FX (Dly)**| Analog Delay | Mix: 10%<br>Time: 300ms<br>Fdbk: 15% | Mix: **20%**<br>Time: 350ms<br>Fdbk: 25% | Bucket Brigade decay sits *behind* the dry signal, thickening the slide without muddying the fundamental note. |
| **Post-FX (Rev)**| Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 15%<br>Decay: 1.2s | Simulates the physical plate reverbs used at Capricorn Studios in the 1970s. |

*Note: Overall loudness control must be handled via the **Lane Output Level** (Right side of the grid), as adjusting the Amp Volume will alter your gain staging and saturation.*

---

### 3. Troubleshooting & Refinement Tree
If the tone isn't behaving properly with your specific slide technique:

1.  **If the tone sounds "Farty" or "Sputtering" on low strings:**
    *   *Action:* Your humbuckers are overloading the virtual tube grid. Lower the **Input Block Gain** by another -2.0dB. Ensure the **US Tweed Basslad** Bass knob is at 2.0 or lower. 
2.  **If the slide sounds too Harsh/Piercing (Ice-pick):**
    *   *Action:* Do not lower the amp's treble (you will lose sustain). Instead, go to the Cab block and move **Mic A (Dyn 57)** further from the center (Position 0.6 or 0.8) or reduce its mix level. 
3.  **If you lack Sustain:**
    *   *Action:* Do not add more amp gain; it will turn to mush. Instead, increase the **Level** parameter on the **Green 808** block to hit the preamp harder, or raise the global Input Block Gain by +1.5dB increments.

---

### 4. Session Registry (Active Presets)

**2. Preset Name: "Macon Slide - Dual Rig"**
*   **Target:** Duane Allman / Southern Rock Slide (1971).
*   **Guitar:** Gibson ES-339 (Humbuckers) & Fender Telecaster (Single Coil).
*   **Physics Goal:** Deep, saggy Tweed response tailored for slide without low-end collapse. Mid-range pushed via Green 808 for vocal sustain. 
*   **Full Configuration:**
    *   **Block 1 (Global Input):** Gain [Tele: +2.5dB / 339: -4.0dB], Gate Thresh [-68dB].
    *   **Block 2 (EQ-8):** [Tele: Active - Low Shelf 200Hz +3.5dB, LPF 4500Hz] / [339: Active Lead Only - HPF 120Hz].
    *   **Block 3 (Green 808):** [Rhy: Bypassed] / [Lead: Active - Drive 1.5, Tone 6.5, Level 8.0].
    *   **Block 4 (Amp - US Tweed Basslad Jumped):** Vol Norm [3.5 / 4.5], Vol Bright [4.0 / 6.0], Bass [2.0], Mid [6.5 / 7.5], Treble [6.0], Presence [5.5].
    *   **Block 5 (Cab - 410 Basslad PR10):** Mic A (Dyn 57, Pos 0.2, Dist 1.0"), Mic B (Ribbon 160, Pos 0.5, Dist 4.0"), Mix [A: -2dB, B: 0dB].
    *   **Block 6 (Analog Delay):** Sync [Off], Time [300ms / 350ms], Fdbk [15% / 25%], Mix [10% / 20%].
    *   **Block 7 (Plate Reverb):** Decay [1.2s], Pre-Delay [15ms], HPF [150Hz], LPF [4000Hz], Mix [15%].
    *   **Lane Output Level:** Rhythm [-1.5dB] / Lead [+1.5dB].