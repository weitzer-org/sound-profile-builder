Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To capture Slash’s iconic "Sweet Child / November Rain" lead tone (Alnico II neck humbucker into a modified, pushed Marshall JCM800), we must rely on physics-accurate mid-range staging and delay settings that do not muddy the fast legato runs. Since you are running through a full-range QSC CP12 PA speaker, we will rely heavily on the Cab Block’s microphone selection and LPF (Low Pass Filter) to replicate the physical dampening of a 4x12 Marshall cabinet and remove PA-induced high-end fizz.

Per system protocols, I have engineered a dual-profile routing matrix to perfectly accommodate both a **Gibson ES-339 (Humbuckers)** and a **Fender Telecaster (Single Coils)** within the exact same preset.

***

### 🎸 Guitar-Specific Hardware Prep
*To accurately hit the Slash target, you must adjust your physical guitar controls before striking a chord.*

**1. Gibson ES-339 (Target: Les Paul Simulation)**
*   **Pickup:** Neck.
*   **Guitar Controls:** Volume 10. **Tone rolled back to 4 or 5.** (This is the physical secret to Slash’s "Woman Tone" sustain).
*   **QC Input Gain:** 0.0dB. 

**2. Fender Telecaster (Target: Mahogany/Humbucker Simulation)**
*   **Pickup:** Neck.
*   **Guitar Controls:** Volume 10. **Tone rolled back to 5.**
*   **QC Input Gain:** +4.0dB (Compensates for lower single-coil output to hit the Marshall’s front-end with equal force).
*   *Note: We are using a dedicated "Pre-EQ" block (Parametric-8) exclusively on your Telecaster scenes (Scenes A-D) to physically simulate a humbucker's frequency response.*

***

### 🎛️ The Split-Bank Matrix (Preset Architecture)
*   **Row 1 (Scenes A-D) - Telecaster Mode:** Input Gain +4.0dB, Pre-EQ [ON].
*   **Row 2 (Scenes E-H) - ES-339 Mode:** Input Gain 0.0dB, Pre-EQ [Bypassed].

#### Table A: Main Signal Chain
*Note: Parameters marked with (Right-Click > Assign) change automatically when switching between Rhythm (A/E) and Lead (B/F) scenes.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input (Circle 1) | Thresh: -55dB | Thresh: -60dB (Right-Click > Assign) | Lowered threshold on lead lets sustained notes ring out into feedback gracefully. |
| **Pre-FX (Tele Only)** | Parametric-8 | Bypassed (For 339) | Engaged (For Tele) | **Band 2:** +4dB @ 250Hz (Adds Mahogany body weight). **Band 8 (LPF):** 4.5kHz (Tames single-coil pick attack). |
| **Overdrive** | Green 808 | Gain: 0, Level: 8.5 | Gain: 0, Level: 10 (Right-Click > Assign) | Simulates Slash's legendary rented S.I.R. #39/#34 Marshall by pushing the JCM800 preamp. |
| **Amp** | Brit 800 | Gain: 6.5, Mid: 6.0, Master: 5.5 | Gain: 7.5, Mid: 7.5, Master: 6.5 (Right-Click > Assign) | Boosting the mids and master volume pushes the virtual power tubes for natural compression. |
| **Cab** | 412 Brit V30 | Mic A: Dyn 57 (Pos 0.5) <br> Mic B: Rib 121 (Pos 1.5) | Mix: A -2dB, B 0dB <br> HPF: 80Hz, LPF: 6.5kHz | 121 Ribbon mic tames the V30 upper-mid spike. 6.5kHz LPF prevents the active QSC CP12 from sounding harsh. |
| **Post-FX 1** | Digital Delay | Mix: 15%, Time: 420ms | Mix: 28%, Time: 420ms (Right-Click > Assign) | Standard Boss DD-3 style digital delay. 420ms allows triplets without stepping on the dry signal. |
| **Post-FX 2** | Hall Reverb | Mix: 12%, Decay 2.0s | Mix: 22%, Decay 2.5s (Right-Click > Assign) | Simulates the large arena/stadium reflections essential to the *Use Your Illusion* tones. |
| **Output** | Lane Output | Level: -1.5dB | Level: +1.5dB | Guarantees your solo jumps over the live band mix without changing the amp's distortion character. |

***

### 🔧 Troubleshooting & Refinement Tree
If the tone is "Too Distorted", "Too Fuzzy", or squealing excessively, execute these steps in strict order:
1. **Input Pad:** Lower the Input Block Gain to -3.0dB (ES-339) or 0.0dB (Telecaster).
2. **Pedal Check:** Reduce the Level on the *Green 808* block from 10 down to 7. 
3. **Amp Gain:** Reduce the Amp Block *Gain* parameter by 1.5 increments. 
4. **Compensation:** If the volume drops, raise the *Lane Output Level* at the far right of the grid. Never use a compressor to fix gain-staging issues.

***

### 💾 Session Library (Active Presets)

**2. Preset Name: "Appetite Lead - Dual Profile"**
*   **Target:** Guns N' Roses / Slash (Neck Pickup Lead Tone).
*   **Guitars:** Gibson ES-339 (Scenes E-H) / Fender Telecaster (Scenes A-D).
*   **Physics Goal:** Simulate pushed Marshall JCM800 preamps + Alnico II humbucker sustain via mid-range EQ and analog-style routing. QSC CP12 optimized.
*   **Full Configuration:**
    *   **Block 1 (Input/Gate):** Inp Gain [Tele: +4.0dB / ES-339: 0.0dB], Gate Thresh [Rhy: -55dB / Lead: -60dB].
    *   **Block 2 (Pre-EQ - Parametric-8):** [ON for Scenes A-D, OFF for E-H], HPF [90Hz], Band 2 [Peak, +4dB @ 250Hz, Q 1.0], LPF [4500Hz].
    *   **Block 3 (Green 808):** Drive [0], Level [Rhy: 8.5 / Lead: 10.0], Tone [5.0].
    *   **Block 4 (Amp - Brit 800):** Pre-amp [Rhy: 6.5 / Lead: 7.5], Master [Rhy: 5.5 / Lead: 6.5], Bass [4.5], Mid [Rhy: 6.0 / Lead: 7.5], Treble [5.0], Presence [4.5].
    *   **Block 5 (Cab - 412 Brit V30):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 1.5"), Mix [A: -2dB, B: 0dB], HPF [80Hz], LPF [6500Hz]. 
    *   **Block 6 (Digital Delay):** Mix [Rhy: 15% / Lead: 28%], Time [420ms], Feedback [30%], HPF [200Hz], LPF [3000Hz].
    *   **Block 7 (Hall Reverb):** Mix [Rhy: 12% / Lead: 22%], Decay [Rhy: 2.0s / Lead: 2.5s], Pre-Delay [20ms].
    *   **Block 8 (Lane Output):** Level [Rhy: -1.5dB / Lead: +1.5dB].