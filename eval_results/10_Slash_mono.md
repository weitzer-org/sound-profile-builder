System Engineer "QC-2" initialized. 

Target acquired: Guns N' Roses / Slash Lead Tone (JCM800 + Delay, Neck Pickup focused). 
Physics Goal: Emulate a modified, hot-rodded Marshall JCM800 2203 (SIR #34/#39 era) driving Celestion Vintage 30s. To achieve Slash’s iconic neck pickup sustain (e.g., "Sweet Child O' Mine" or "Knockin' on Heaven's Door" solos), we rely heavily on midrange emphasis, power amp headroom, and analog delay trails—without fizz.

**Crucial Analog Requirement:** For true Slash neck-pickup tones, you must roll your guitar’s physical Neck Tone knob down to **0–3**. The amp will provide the gain; the guitar's rolled-off capacitor provides the famous "flutey" resonance.

Here is your Split-Bank execution protocol, engineered for a QSC CP12 FRFR speaker.

---

### 🎸 MULTI-GUITAR TARGET PROTOCOL
To ensure physics-accurate signal-to-noise ratios and frequency response, you must adapt the Input and Pre-EQ stages based on your guitar. 

**Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers)**
*   **Physics:** Medium/High-output PAF-style pickups push the Quad Cortex ADC naturally. The neck pickup is prone to low-end mud when fed into a cranked Marshall.
*   **Input Block:** In Gain: **-1.5dB**. Input Gate: Threshold **-55dB**.
*   **Pre-FX EQ (EQ-8):** HPF at 90Hz. Cut Band 2 (250Hz) by -2.0dB to remove neck pickup boominess. 

**Row 1 (Scenes A-D): Fender Telecaster (Single Coils)**
*   **Physics:** Low-output single coils will sound thin and spiky through a JCM800. We must artificially inject "mahogany/humbucker" resonance and tame the 5kHz ice-pick attack.
*   **Input Block:** In Gain: **+3.5dB**. Input Gate: Threshold **-62dB**.
*   **Pre-FX EQ (EQ-8):** Low Shelf +4.0dB @ 200Hz (Adds Les Paul body). Peak +3.0dB @ 600Hz (Adds humbucker honk). LPF at 5.5kHz (Simulates rolled-off humbucker tone).

---

### TABLE A: MAIN SIGNAL CHAIN (SLASH JCM800)
*(Note: Right-Click > "Assign to Scene" on any parameter with dual values to map them to Rhythm/Lead scenes).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55dB (Humbucker) | Thresh: -60dB (Lead) | Tighter gate for rhythm chugs; lower threshold for infinite lead sustain. |
| **Pre-FX** | Parametric-8 | *See Multi-Guitar Protocol above* | *See Multi-Guitar Protocol above* | "Chameleon Strategy" to compensate for pickup resonance prior to distortion. |
| **Amp** | Brit 800 | Preamp: 6.5, Master: 6.0, Bass: 4.5, Mid: 6.5, Treb: 5.5, Pres: 5.0 | Preamp: 8.5, Master: 6.0, Bass: 5.0, Mid: 8.0, Treb: 5.0, Pres: 4.5 | Master at 6.0 engages virtual EL34 power amp sag. Lead scene pushes mids to cut through stadium mixes. |
| **Cab** | 412 Brit V30 | Mic A (Dyn 57): Pos 0.5, Dist 1.0" | Mic B (Rib 121): Pos 1.5, Dist 2.5" | V30s provide the upper-mid spike. 57 provides the bite; 121 provides the massive low-end weight. Mix: A 0dB, B -3dB. |
| **Post-FX 1** | Analog Delay | Mix: 8%, Time: 350ms, Fdbk: 15% | Mix: 28%, Time: 450ms, Fdbk: 35% | Analog model keeps repeats dark so they don't step on the neck pickup phrasing. |
| **Post-FX 2** | Hall | Mix: 10%, Decay: 1.8s | Mix: 18%, Decay: 2.8s | Big stadium air. High-Pass the reverb at 150Hz to prevent low-end wash. |

---

### 🔧 TROUBLESHOOTING & REFINEMENT TREE
If the tone is **Too Distorted/Fuzzy** or feeding back uncontrollably:
1.  **Input Pad:** Drop the Input Block Gain to -4.0dB. This mimics rolling off your guitar's volume knob to clean up the amp naturally.
2.  **Amp Gain:** Lower the `Brit 800` Preamp knob to 6.0 on the Lead scene. 
3.  **Speaker Target:** If the QSC CP12 sounds too harsh, move the Dyn 57 microphone position from 0.5 (Center) to 1.5 (Cap Edge) in the Cab block to shave off 4kHz frequencies.

---

### 🗄️ SESSION LIBRARY (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

**2. Preset Name: "Appetite Neck Lead"**
Target: Guns N' Roses / Slash (1987).
Guitar: Dual Support (ES-339 / Telecaster).
Physics Goal: Hot-rodded Marshall EL34 saturation, pronounced midrange for neck pickup definition, stadium delay trails.
Full Configuration:
Block 1 (Global Input): Gain [ES: -1.5dB / Tele: +3.5dB], Gate Thresh [ES: -55dB / Tele: -62dB].
Block 2 (EQ-8): Tele Mode: Low Shelf [+4dB @ 200Hz], Peak [+3dB @ 600Hz], LPF [5.5kHz]. ES Mode: Cut [-2dB @ 250Hz].
Block 3 (Amp - Brit 800): Preamp [Rhy: 6.5 / Lead: 8.5], Master [6.0], Bass [4.5 / 5.0], Mid [6.5 / 8.0], Treble [5.5 / 5.0], Pres [5.0 / 4.5].
Block 4 (Cab - 412 Brit V30): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 2.5"), Mix [A: 0dB, B: -3dB].
Block 5 (Analog Delay): Mix [Rhy: 8% / Lead: 28%], Time [350ms / 450ms], Fdbk [15% / 35%].
Block 6 (Hall Reverb): Mix [Rhy: 10% / Lead: 18%], Decay [1.8s / 2.8s], HP [150Hz].