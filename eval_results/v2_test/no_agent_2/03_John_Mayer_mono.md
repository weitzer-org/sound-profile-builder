Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To capture the John Mayer "Gravity" tone, we are targeting a high-headroom, mid-scooped analog clean tone that serves as a pristine pedal platform, driven into smooth, singing saturation for the solo. 

Per the **Pseudonym Rule**, the Quad Cortex does not use the names "Dumble" or "Klon." We will be utilizing verified CorOS models:
*   **Amp:** `D-Cell Custom Clean` (Modeled after the Dumble Overdrive Special - Clean Channel)
*   **Overdrive:** `Myth Drive` (Modeled after the Klon Centaur)
*   **Cab:** `212 D-Cell Custom` (Capturing the classic G12-65 speaker response Mayer favors).

Per System Protocols, I must adapt your low-output single-coil request into our **Multi-Guitar Matrix**. Below are strictly separated, physics-accurate configurations for **Fender Telecaster (Single Coils)** and **Gibson ES-339 (Humbuckers)** monitoring through your **QSC CP12 Active PA Speaker**.

---

### GUITAR 1: Fender Telecaster (Low-Output Single Coils)
**Target Bank:** Row 1 (Scenes A-D)
**Physics Goal:** Compensate for low-output pickups by pushing the Input Gain and Amp Volume. We use the Parametric-8 EQ to add "Strat-like" neck pickup body to the Telecaster and roll off the ice-pick bridge frequencies for the solo.

#### Table A: Main Signal Chain (Fender Telecaster)
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh/Red: 35% | Thresh/Red: 50% | Single coils hum. Higher reduction needed when Myth Drive is engaged. |
| **Pre-FX (Drive)** | Myth Drive | *Bypassed* | Gain: 5.0, Treb: 6.0, Out: 7.5 | Klon engaged for solo. Pushes the D-Cell preamp into smooth compression. |
| **Pre-FX (EQ)** | Parametric-8 | B1 (Peak): +3.0dB @ 200Hz | B1: +3.0dB, B8 (LPF): 4.5kHz | B1 adds neck-pickup "tubiness". B8 tames pick-attack when driven. |
| **Amp** | D-Cell Custom Clean | Vol: 6.5, Bass: 6.0, Mid: 3.5, Treb: 6.5 | Vol: 6.5 (Same) | Mayer's tone is notoriously mid-scooped. High volume compensates for low-output pickups. |
| **Cab** | 212 D-Cell Custom | Mic A: Dyn 57 (Pos 1.0, Dist 1.0") | Mic B: Rib 121 (Pos 1.5, Dist 3.0") | 57 adds glass. Ribbon 121 adds low-end girth. Mix A at 0dB, B at -2dB. |
| **Post-FX (Delay)** | Tape Delay | *Bypassed* | Mix: 18%, Time: 350ms, Fdbk: 20% | Adds sustain and depth to the solo without muddying the mix. |
| **Post-FX (Rev)** | Plate Reverb | Mix: 25%, Decay: 2.2s | Mix: 35%, Decay: 2.8s | Replicates the lush studio plate used on the Continuum album. |

---

### GUITAR 2: Gibson ES-339 (Medium/High-Output Humbuckers)
**Target Bank:** Row 2 (Scenes E-H)
**Physics Goal:** Prevent the ES-339's higher output from collapsing the D-Cell's clean headroom. We must heavily scoop the lower-mids (mud zone) and pad the input stage to simulate the clarity of single-coil pickups.

#### Table B: Main Signal Chain (Gibson ES-339)
*Mark Scene-Specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Input Pad + Gate | Gain: -4.5dB, Thresh/Red: 15% | Gain: -4.5dB, Thresh/Red: 25% | **Crucial:** Padding input restores clean headroom. Humbuckers need less noise reduction. |
| **Pre-FX (Drive)** | Myth Drive | *Bypassed* | Gain: 3.0, Treb: 7.5, Out: 6.0 | Lower gain/output than Tele to prevent clipping. Higher treble for bite. |
| **Pre-FX (EQ)** | Parametric-8 | B2: -4.0dB @ 300Hz, B6: +3.0dB @ 3kHz | *Same as Rhythm* | Clears humbucker mud (300Hz) and mimics single-coil glass (3kHz). |
| **Amp** | D-Cell Custom Clean | Vol: 4.5, Bass: 4.5, Mid: 3.5, Treb: 7.0 | Vol: 4.5 (Same) | Amp Vol and Bass reduced by 2.0 to prevent early tube saturation and low-end flub. |
| **Cab** | 212 D-Cell Custom | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") | Mic B: Rib 121 (Pos 2.0, Dist 4.0") | 57 moved closer to center (0.5) to maximize high-end articulation for humbuckers. |
| **Post-FX (Delay)** | Tape Delay | *Bypassed* | Mix: 15%, Time: 350ms, Fdbk: 15% | Lower mix for humbuckers to prevent spatial clutter. |
| **Post-FX (Rev)** | Plate Reverb | Mix: 20%, Decay: 2.2s | Mix: 30%, Decay: 2.8s | Slightly lower mix than single coils to maintain note definition. |

---

### Troubleshooting & Refinement Tree
If monitoring through your QSC CP12 and the tone feels "Too Distorted" or "Too Fuzzy" during the Lead Scenes (B/F):
1. **Input Pad:** Ensure the ES-339 input is strictly padded to -4.5dB or even -6.0dB. 
2. **Amp Gain:** Drop the `Myth Drive` output by 1.5 increments. Do not touch the Amp Volume first, as it dictates your base clean tone.
3. **Tube Sag/Flub:** If the neck pickup on the ES-339 sounds "farty" when the Klon is engaged, lower the `D-Cell Custom Clean` Bass parameter to 3.5. 
4. **Output Compensation:** If adjustments cause a volume drop, increase the **Lane Output Level** (far right output block on the grid) by +2.0dB.

---

### Session Library (Active Presets)
*Logging configuration to session memory for strict recall.*

**2. Preset Name: "Gravity - Dual Profile"**
*   **Target:** John Mayer (2006, Continuum era).
*   **Guitar 1:** Fender Telecaster (Single Coils).
*   **Guitar 2:** Gibson ES-339 (Humbuckers).
*   **Physics Goal:** Massive clean headroom, mid-scooped preamp, transparent overdrive boost.
*   **Full Configuration (Telecaster / ES-339):**
    *   **Block 1 (Input/Gate):** Input Gain [0.0dB / -4.5dB], Noise Red [Rhy: 35%, Lead: 50% / Rhy: 15%, Lead: 25%].
    *   **Block 2 (Myth Drive):** Gain [5.0 / 3.0], Treble [6.0 / 7.5], Output [7.5 / 6.0]. Bypassed on Rhy.
    *   **Block 3 (EQ-8):** B1 [200Hz: +3dB / 0dB], B2 [300Hz: 0dB / -4.0dB], B6 [3kHz: 0dB / +3.0dB], LPF [Lead: 4.5kHz / Off].
    *   **Block 4 (Amp - D-Cell Custom Clean):** Vol [6.5 / 4.5], Bass [6.0 / 4.5], Mid [3.5 / 3.5], Treble [6.5 / 7.0], Presence [5.0], Master [7.0].
    *   **Block 5 (Cab - 212 D-Cell Custom):** Mic A (Dyn 57, Pos [1.0 / 0.5], Dist 1.0"), Mic B (Ribbon 121, Pos [1.5 / 2.0], Dist [3.0" / 4.0"]), Mix [A: 0dB, B: -2dB].
    *   **Block 6 (Tape Delay):** Mix [18% / 15%], Time [350ms], Fdbk [20% / 15%]. Bypassed on Rhy.
    *   **Block 7 (Plate Reverb):** Mix [Rhy: 25%, Lead: 35% / Rhy: 20%, Lead: 30%], Decay [2.2s / 2.8s].