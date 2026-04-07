Welcome to QC-2 engineering. Target acquired: **Jimi Hendrix - "Voodoo Child (Slight Return)"**.

### Physics & Hardware Protocol
To achieve this tone on a QSC CP12 PA speaker, we must respect the volatile physics of the **Dallas Arbiter Fuzz Face** running into a **Marshall 1959 Super Lead Plexi**. 

*   **The Fuzz Face Dilemma:** A real Fuzz Face has very low input impedance. Jimi got his famous "glassy" intro tone not by turning off the fuzz, but by rolling his Stratocaster's volume knob down to ~4. To replicate this dynamically in Cortex Control without forcing you to touch your guitar, we will assign the **Input Block Gain** to Scene controllers. 
*   **The Headroom Rule:** The "Brit Plexi 100" models do *not* have a Master Volume. To increase overall loudness (SPL) without adding more distortion, you must use the **Lane Output Level**.

---

### Multi-Guitar Target Output & Gain Staging Strategy

*Trigger Verification:* Assuming your ES-339 has Medium/High Output Humbuckers and your Telecaster has Vintage/Medium Single Coils. 

#### 1. Fender Telecaster (Row 1: Scenes A–D)
*   **Physics Goal:** Tame the extreme bridge pickup "ice-pick" highs while maintaining the aggressive Strat-like single-coil bite. 
*   **EQ Strategy:** Parametric-8 EQ block will use a Low Pass Filter (LPF) at 5.5kHz to shave off the harsh resonant peak of the Telecaster bridge pickup.
*   **Input Staging:** Global Input Gain set to **0.0dB** (Baseline). Scene A will drop to **-12.0dB** to simulate the guitar volume roll-off for the intro.

#### 2. Gibson ES-339 Humbuckers (Row 2: Scenes E–H)
*   **Physics Goal:** Prevent the humbuckers from instantly choking the Fuzz block. High-output pickups push too much low-mid frequency into a Fuzz Face, resulting in a "farty," broken-velcro sound that ruins the Plexi's tube sag.
*   **EQ Strategy:** Parametric-8 EQ block will use a High Pass Filter (HPF) at 120Hz to aggressively cut low-end mud *before* the amp, plus a slight dip at 400Hz to remove humbucker boxiness. 
*   **Input Staging:** Global Input Gain must be padded to **-6.0dB** globally. Scene E (Intro) will drop further to **-16.0dB** to ensure the humbuckers clean up to a glassy sparkle.

---

### Table A: Main Signal Chain (Split-Bank Matrix)
*Note: Parameters marked with **(SC)** require you to Right-Click > Assign to Scene.*

| Block Category | Model Name | Rhythm / Intro (Sc A / E) | Lead / Riff (Sc B / F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | **Input 1** | Gain: -12.0dB (A) / -16.0dB (E) *(SC)* | Gain: 0.0dB (B) / -6.0dB (F) *(SC)* | Simulates rolling guitar volume knob down for glassy fuzz clean-up. |
| **Gate** | **Adaptive Gate** | Noise Red: 50% / Thresh: -55dB | Noise Red: 70% / Thresh: -45dB *(SC)* | High % needed for Fuzz. Scene B/F requires stricter gating for high-gain noise. |
| **Wah** | **Crying Wah** | Position: EXP 1 (or 85%) | Position: EXP 1 (or Auto-Engage) | Essential for the sweeping, throaty intro attack. |
| **Pre-FX** | **Facial Fuzz** | Fuzz: 10.0 / Level: 6.5 | Fuzz: 10.0 / Level: 8.5 *(SC)* | Keep Fuzz maxed; tone clean-up is handled entirely by the Input Block volume drop. |
| **EQ** | **Parametric-8** | *Tele (A):* LPF @ 5.5kHz | *ES-339 (F):* HPF @ 120Hz, Band 3: -3dB @ 400Hz | The "Chameleon" block. Matches pickup physics to the target vintage single-coil tone. |
| **Amp** | **Brit Plexi 100 Jumped** | Vol Norm: 6.0 / Vol High: 7.5 | Vol Norm: 6.0 / Vol High: 7.5 | Vintage Marshall taper. Bass kept low (3.0) to prevent tube sag mud. (No Master Vol). |
| **Cab** | **412 Brit Green 25** | Mic A: Dyn 57 (Pos 1.5) | Mic B: Ribbon 121 (Pos 3.0) | Mix A: 0dB / B: -3dB. 57 provides the aggressive bite; 121 provides the low-mid thump. |
| **Post-FX** | **Analog Delay** | Mix: 0% *(SC)* | Mix: 20%, Time: 320ms, Fdbk: 30% *(SC)* | Adds the psychedelic, slap/trail needed for the solo sections. |
| **Post-FX** | **Room Reverb** | Mix: 15%, Decay: 1.2s | Mix: 15%, Decay: 1.2s | Simulates the live room sound of Electric Lady Studios. |
| **Output** | **Lane 1 Output** | Level: +4.0dB *(SC)* | Level: 0.0dB *(SC)* | Output Compensation: Boosts lane volume in Rhythm scenes to make up for the Input Block drop. |

---

### Troubleshooting & Refinement Tree
If the ES-339 fuzz tone still sounds "Too Farty/Muddy" or the Telecaster sounds "Too Piercing", follow this strict order of operations:

1.  **Input Pad (The Fix):** Lower the Input Block Gain by another -2.0dB. Fuzz Faces are hyper-sensitive to input voltage. 
2.  **Amp Gain:** Reduce the Plexi's `Vol High` parameter in 1.0 increments. 
3.  **Tube Sag:** Lower the Plexi's `Bass` parameter to 1.5. Vintage Marshalls generate extreme low-end phase issues when pushed with Fuzz. 
4.  **Output Compensation:** If the tone is finally clear but now too quiet through your QSC CP12, **increase the Lane 1 Output Level**. *Never increase the Amp Volume to fix SPL loudness.*

---

### Session Library (Active Presets)

**2. Preset Name: "Voodoo Child - Dual Profile"**
*   **Target:** Jimi Hendrix (1968).
*   **Guitars:** Telecaster (SC) / Gibson ES-339 (HB).
*   **Physics Goal:** Replicate analog guitar-volume roll-off clean-up via Scene input staging, tame humbucker low-end mud before hitting an Arbiter Fuzz Face and Plexi.
*   **Full Configuration:**
    *   **Block 1 (Input):** Gain [Sc A: -12dB, Sc B: 0dB, Sc E: -16dB, Sc F: -6dB].
    *   **Block 2 (Adaptive Gate):** Noise Red [Rhy: 50% / Lead: 70%], Thresh [-55dB / -45dB].
    *   **Block 3 (Crying Wah):** Position [EXP 1], Mix [100%].
    *   **Block 4 (Facial Fuzz):** Fuzz [10.0], Level [Sc A/E: 6.5, Sc B/F: 8.5].
    *   **Block 5 (EQ-8):** *Tele Mode (Rows 1):* LPF [5500Hz]. *ES339 Mode (Rows 2):* HPF [120Hz], Band 3 [-3.0dB, Freq 400Hz].
    *   **Block 6 (Amp - Brit Plexi 100 Jumped):** Vol Norm [6.0], Vol High [7.5], Bass [3.0], Mid [7.0], Treble [6.5], Presence [7.0]. (No Master Vol).
    *   **Block 7 (Cab - 412 Brit Green 25):** Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 3.0, Dist 2.0"), Mix [A: 0dB, B: -3dB].
    *   **Block 8 (Analog Delay):** Mix [Sc A/E: 0% / Sc B/F: 20%], Time [320ms], Fdbk [30%].
    *   **Block 9 (Room Reverb):** Mix [15%], Decay [1.2s], HP [150Hz], LP [4000Hz].
    *   **Lane 1 Output:** Level [Sc A/E: +4.0dB / Sc B/F: 0.0dB].