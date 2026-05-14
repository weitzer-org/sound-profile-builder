**System Initialization: QC-2 Quad Cortex Systems Engineer**
**Target Acknowledged:** U2 / The Edge - 1964 Vox AC30TB Edge-of-Breakup with Dotted-Eighth Rhythmic Delay.
**Hardware Environment:** Cortex Control (Mac Desktop) routed to QSC CP12 Active PA Speaker.

To achieve the iconic "Edge" chime, we must replicate the physics of his rig: an SDD-3000 preamp pushing the front end of a 1964 Vox AC30 Top Boost, captured by a Shure SM57, with a highly pronounced modulated digital delay. Because the Vox circuit lacks a Master Volume and responds drastically to input impedance, we will use the **"Chameleon" Protocol** via the Parametric-8 EQ block to simulate the SDD-3000 preamp curve while simultaneously adapting the rig for your two distinct guitars. 

Below are the strictly separated configurations for your Fender Telecaster (Scenes A-D) and Gibson ES-339 (Scenes E-H).

---

### Part 1: Fender Telecaster (Single Coil) Configuration
**Split-Bank Matrix: Row 1 (Scenes A-D)**
*Physics Goal:* Telecasters naturally lack the low-mid "thump" of Edge’s preferred Explorer/Strat bridge pickups. We will use the EQ-8 to push the preamp harder (+3.0dB) to hit the exact edge-of-breakup threshold, while boosting the 200Hz body band to prevent the AC30 Top Boost circuit from sounding shrill through your QSC CP12.

**Table A: Main Signal Chain (Telecaster)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -65dB | Low output single coils require a lower gate threshold to preserve delicate Herdim pick scrapes. |
| **Pre-FX (Preamp)** | Parametric-8 | Output: +3.0dB<br>Band 2 (Peak): +2.5dB @ 250Hz<br>Band 6 (Peak): +1.5dB @ 3000Hz | Output: +4.5dB<br>Band 2: +2.5dB<br>Band 6: +2.5dB | Simulates the Korg SDD-3000 preamp push. Adds missing body weight to the Telecaster bridge pickup. |
| **Amp** | UK C30 Top Boost | Vol: 6.5<br>Bass: 4.5<br>Treble: 6.5<br>Cut: 3.5 | Vol: 6.5<br>Bass: 4.5<br>Treble: 6.5<br>Cut: 3.5 | Classic TB settings. High treble, moderate cut (Cut acts as a reverse presence/LPF). No Master Volume; rely on Lane Level for SPL. |
| **Cab** | 212 UK C30 (Alnico Blue) | Mic A: Dyn 57 (Pos 0.4, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 0.8, Dist 2.5")<br>Mix: A at 0dB, B at -3dB | [Same] | Edge's exact mic pairing. SM57 provides the aggressive 3kHz bite; Ribbon 121 adds warmth. |
| **Post-FX (Delay)** | Digital Delay | Sync: On (1/8d)<br>Mix: 42%<br>Fdbk: 28%<br>Mod Depth: 15% | Sync: On (1/8d)<br>Mix: 45%<br>Fdbk: 35%<br>Mod Depth: 18% | 1/8d (dotted eighth) is the signature rhythm. High mix percentage ensures repeats are almost as loud as the attack. Mod adds SDD-3000 wow/flutter. |
| **Post-FX (Verb)** | Plate Reverb | Mix: 12%<br>Decay: 1.2s<br>HPF: 150Hz / LPF: 4000Hz | Mix: 16%<br>Decay: 1.5s<br>HPF: 150Hz / LPF: 4000Hz | Subtle stadium air. LPF set low to ensure the reverb doesn't mask the crisp delay repeats. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB | Volume jump for lead lines without altering the amp's tube saturation physics. |

---

### Part 2: Gibson ES-339 (Humbucker) Configuration
**Split-Bank Matrix: Row 2 (Scenes E-H)**
*Physics Goal:* The ES-339’s PAF-style humbuckers output significantly more voltage and low-mid frequencies (200-400Hz). Hitting a Vox AC30TB with an unpadded humbucker instantly bypasses "chime" and results in muddy, unfocused overdrive. We must aggressively pad the input gain and scoop the low-mids to force the ES-339 to behave like a single-coil.

**Table B: Main Signal Chain (ES-339)**
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55dB<br>Input Pad: -4.5dB | Thresh: -55dB<br>Input Pad: -4.5dB | The -4.5dB pad prevents the humbuckers from clipping the amp block's virtual 12AX7 tubes too early. Higher gate threshold due to humbucker output. |
| **Pre-FX (Preamp)** | Parametric-8 | Output: 0.0dB<br>Band 2 (Peak): -3.0dB @ 300Hz<br>HPF: 110Hz | Output: +1.5dB<br>Band 2: -3.0dB @ 300Hz<br>HPF: 110Hz | Hollowbody humbuckers carry mud at 300Hz. Scooping this band reveals the Vox chime. HPF prevents PA boominess. |
| **Amp** | UK C30 Top Boost | Vol: 4.0<br>Bass: 3.0<br>Treble: 7.0<br>Cut: 4.5 | Vol: 4.5<br>Bass: 3.0<br>Treble: 7.0<br>Cut: 4.5 | Volume heavily reduced (from 6.5 on Tele) to maintain edge-of-breakup. Cut increased to tame harsh upper-harmonics from humbuckers. |
| **Cab** | 212 UK C30 (Alnico Blue) | Mic A: Dyn 57 (Pos 0.6, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 0.8, Dist 3.0")<br>Mix: A at 0dB, B at -6dB | [Same] | Moved the SM57 slightly off-center (Pos 0.6) to reduce humbucker 'ice-pick'. Reduced Ribbon mix to prevent low-end build-up. |
| **Post-FX (Delay)** | Digital Delay | Sync: On (1/8d)<br>Mix: 38%<br>Fdbk: 25%<br>Mod Depth: 15% | Sync: On (1/8d)<br>Mix: 42%<br>Fdbk: 32%<br>Mod Depth: 18% | Slightly lower mix than the Telecaster, as humbucker transients are thicker and can clutter the rhythmic grid if delays are too loud. |
| **Post-FX (Verb)** | Plate Reverb | Mix: 10%<br>Decay: 1.2s<br>HPF: 180Hz / LPF: 3500Hz | Mix: 14%<br>Decay: 1.5s<br>HPF: 180Hz / LPF: 3500Hz | HPF raised to 180Hz to keep the ES-339's lower fundamentals completely dry. |
| **Output** | Lane Output | Level: +2.0dB | Level: +3.5dB | Because we lowered the Amp Volume to clean up the humbuckers, we must raise Lane Output Level to match SPL with the Telecaster scenes. |

---

### Troubleshooting & Refinement Tree (Vox Circuits)
If the tone is **"Too Distorted"** or sounding fuzzy through your QSC CP12:
1. **Input Pad:** Ensure the ES-339 has the Global Input Level pulled back to at least -4.5dB. 
2. **Amp Gain:** Lower the `Vol` parameter on the UK C30 Top Boost block in 0.5 increments. 
3. **Tube Sag/Farting:** The AC30 model can sag aggressively if pushed with too much low end. Lower the `Bass` parameter on the Amp block to 2.0.
4. **Output Compensation:** If the tone is finally clean but now too quiet, *do not touch the Amp Volume*. Raise the Lane Output Level slider on the right side of the Cortex Control UI.

---

### Session Registry Protocol (Active Presets)
**Session Library Update:**
2. **Preset Name:** "Streets Chime Matrix"
*Target:* U2 / The Edge (1964 Vox AC30TB w/ Dotted 8th).
*Guitar:* Split-Matrix (Sc A-D: Telecaster | Sc E-H: ES-339).
*Physics Goal:* Replicate SDD-3000 preamp push into AC30TB edge-of-breakup. Compensate for extreme input impedance variance between vintage single-coils and PAF humbuckers.
*Full Configuration:*
- **Input/Gate:** Thresh [-65dB Tele / -55dB 339]. Input Gain [0.0dB Tele / -4.5dB 339].
- **EQ-8 (Preamp):** Tele [Out: +3.0dB, B2: +2.5dB@250Hz, B6: +1.5dB@3kHz]. 339 [Out: 0.0dB, HPF: 110Hz, B2: -3.0dB@300Hz]. 
- **Amp (UK C30 Top Boost):** Tele [Vol 6.5, Bass 4.5, Treb 6.5, Cut 3.5]. 339 [Vol 4.0, Bass 3.0, Treb 7.0, Cut 4.5].
- **Cab (212 UK C30):** Mic A (Dyn 57), Mic B (Rib 121). Tele Mix [0dB / -3dB]. 339 Mix [0dB / -6dB]. 
- **Delay (Digital Delay):** Sync [On - 1/8d], Mod Depth [15%], Tele Mix [42%], 339 Mix [38%].
- **Reverb (Plate):** Mix [12% Tele / 10% 339], HPF [150Hz Tele / 180Hz 339]. LPF [4000Hz].
- **Lane Level:** Tele [0.0dB Rhy / +1.5dB Ld]. 339 [+2.0dB Rhy / +3.5dB Ld].