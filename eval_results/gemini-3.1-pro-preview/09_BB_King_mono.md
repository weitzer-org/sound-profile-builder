### Quad Cortex Systems Engineer (QC-2)
**Target Identification:** B.B. King "Lucille" Tone 
**Analog Target Specs:** Gibson ES-355 (Varitone, no F-holes, out-of-phase humbuckers) -> Fender Twin Reverb (High-headroom clean) -> 2x12 Jensen C12K Speakers -> Spring Reverb. High SPL, hard pick attack, intense vibrato, zero preamp distortion. 

---

### 1. Hardware & "Physics First" Protocol
*   **Target Output:** QSC CP12 (Active PA). We are relying heavily on the Cab Block to simulate the 2x12 projection, keeping the QSC DSP flat (no contour).
*   **Amp Physics (US TWN 65):** This is a **Non-Master Volume Amp**. There is no "Master" knob. To increase your stage volume (SPL) without introducing tube clipping, you must leave the Amp `Volume` fixed and use the **Lane Output Level** slider on the QC. 
*   **The "Lucille" Secret:** B.B.'s sting comes from the Varitone circuit (a passive notch filter) scooping the lower-mids to create a "honky" out-of-phase sound, while his amp runs crystal clean. We will simulate this using the **Parametric-8** block.

---

### 2. Multi-Guitar Gain Staging & Compensation
To achieve this ultra-clean, high-headroom target, we must execute two distinct pickup compensation strategies. 

**Guitar 1: Gibson ES-339 (Humbuckers) - Assigned to Row 2 (Scenes E-H)**
*   **Physics:** Humbuckers will naturally push the Twin Reverb into edge-of-breakup if we aren't careful.
*   **Input Compensation:** Set Global Input Gain to **-3.0dB**. This acts as a pad to prevent transient clipping from your pick attack. 
*   **Tone Shaping:** We must engage the Bright Switch on the amp and use an EQ block to simulate the Varitone scoop (-3dB at 450Hz).

**Guitar 2: Fender Telecaster (Single Coil) - Assigned to Row 1 (Scenes A-D)**
*   **Physics:** The Telecaster will lack the lower-mid body of a semi-hollow and possess an ice-pick attack that B.B. specifically avoided. 
*   **Input Compensation:** Set Global Input Gain to **+1.5dB** to feed the amp the required signal voltage for sustain.
*   **Tone Shaping:** Bright Switch OFF. Use the EQ block to boost body (+3dB at 200Hz) and apply a Low-Pass Filter (LPF) to kill the twang.

---

### 3. Split-Bank Matrix Execution

#### **Table A1: Gibson ES-339 Signal Chain (Row 2: Scenes E-H)**
*Use Scene E for Rhythm, Scene F for Lead.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB, Gain: -3.0dB | Thresh: -65dB, Gain: -3.0dB | Padded input to guarantee maximum clean headroom. Low threshold preserves vibrato sustain. |
| **Pre-FX** | Parametric-8 | Band 3 (450Hz): -3.0dB<br>Band 6 (2.5kHz): 0.0dB | Band 3 (450Hz): -3.0dB<br>Band 6 (2.5kHz): +2.5dB | Simulates the ES-355 Varitone "scoop". Scene F pushes upper mids to cut through the mix without using overdrive. |
| **Amp** | US TWN 65 | Vol: 4.0, Bright: ON<br>Bass: 3.5, Mid: 6.0, Treb: 6.5<br>Out Level: 0.0dB | Vol: 4.0, Bright: ON<br>Bass: 3.5, Mid: 6.0, Treb: 6.5<br>Out Level: +1.5dB | Non-Master Volume rules apply. Keep Volume at 4.0 for pure clean. Boost Output Level for lead loudness. |
| **Cab** | 212 US TWN C12K | Mic A: Dyn 57, Pos 1.0<br>Mic B: Rib 121, Pos 1.5 | Mic A: Dyn 57, Pos 1.0<br>Mic B: Rib 121, Pos 1.5 | 57 captures the hard pick attack; 121 Ribbon tames the highs and adds cabinet warmth. Mix at 50/50. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 1.5s<br>Tone: 4.0 | Mix: 20%, Decay: 1.8s<br>Tone: 4.5 | Essential Twin spatial recreation. Slightly longer/wetter for leads. |

<br>

#### **Table A2: Fender Telecaster Signal Chain (Row 1: Scenes A-D)**
*Use Scene A for Rhythm, Scene B for Lead. Select the Middle position (Bridge + Neck) on your guitar.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55dB, Gain: +1.5dB | Thresh: -55dB, Gain: +1.5dB | Single coils have a lower signal-to-noise ratio. Higher threshold kills 60-cycle hum; boosted gain compensates output. |
| **Pre-FX** | Parametric-8 | Band 2 (200Hz): +3.0dB<br>Band 8 (LPF): 4500Hz | Band 2 (200Hz): +3.0dB<br>Band 8 (LPF): 5000Hz | "Chameleon Strategy": 200Hz boost adds hollow-body weight. LPF tames the harsh Telecaster bridge transient. |
| **Amp** | US TWN 65 | Vol: 4.5, Bright: OFF<br>Bass: 4.5, Mid: 5.5, Treb: 5.0<br>Out Level: 0.0dB | Vol: 4.5, Bright: OFF<br>Bass: 4.5, Mid: 5.5, Treb: 5.0<br>Out Level: +1.5dB | Bright switch must be OFF to avoid ice-pick. Bass raised slightly to compensate for single coils. |
| **Cab** | 212 US TWN C12K | Mic A: Dyn 57, Pos 1.5<br>Mic B: Rib 121, Pos 1.0 | Mic A: Dyn 57, Pos 1.5<br>Mic B: Rib 121, Pos 1.0 | Pushing the 57 further out (Pos 1.5) rolls off the top-end fizz of the Telecaster. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 1.5s<br>Tone: 3.5 | Mix: 20%, Decay: 1.8s<br>Tone: 4.0 | Darker reverb tone settings to avoid highlighting single-coil fret noise. |

---

### 4. Troubleshooting & Refinement Tree
If the tone sounds incorrect through your QSC CP12, follow this strict sequence:
1.  **Too Distorted/Clipping on Attack:** Lower the Global Input Gain by another -2.0dB. Ensure you are not running an external compressor pedal before the QC.
2.  **Too Thin/Piercing (Especially the Tele):** In the Cab Block, lower the Level of Mic A (Dyn 57) by -3dB and increase Mic B (Rib 121) by +2dB. 
3.  **Not Sustaining Enough on Lead:** *Do not add an overdrive pedal.* Increase the Lane Output Level (far right circle on the QC grid) by +2.0dB to push the QSC CP12 harder, or pick harder with a thicker plectrum. B.B.'s sustain came from high SPL moving the guitar strings.

---

### 5. Session Library (Active Presets)

**2. Preset Name: "Lucille's Twin - Dual"**
*   **Target:** B.B. King / Clean American High-Headroom (1970s).
*   **Guitar:** ES-339 (Humbuckers) & Telecaster (Single Coils).
*   **Physics Goal:** Massive clean headroom, Varitone scoop simulation, high SPL sustain without preamp distortion.
*   **Full Configuration:**
    *   *Input Block:* ES-339 [-65dB Thresh, -3.0dB Gain] / Telecaster [-55dB Thresh, +1.5dB Gain].
    *   *Block 1 (EQ-8 ES-339):* HPF [80Hz], Band 3 [450Hz, -3.0dB], Band 6 [2500Hz, Rhy: 0dB / Lead: +2.5dB].
    *   *Block 2 (EQ-8 Telecaster):* HPF [90Hz], Band 2 [200Hz, +3.0dB], LPF [Rhy: 4500Hz / Lead: 5000Hz].
    *   *Block 3 (Amp - US TWN 65):* ES-339 [Vol 4.0, Bright ON, Bass 3.5, Mid 6.0, Treb 6.5, Out +1.5dB (Lead)]. Telecaster [Vol 4.5, Bright OFF, Bass 4.5, Mid 5.5, Treb 5.0, Out +1.5dB (Lead)].
    *   *Block 4 (Cab - 212 US TWN C12K):* Mic A (Dyn 57, Pos 1.0 (1.5 for Tele), Dist 1.0"), Mic B (Ribbon 121, Pos 1.5 (1.0 for Tele), Dist 1.0"), Mix [50/50].
    *   *Block 5 (Spring Reverb):* Mix [15% / 20%], Decay [1.5s / 1.8s], Tone [4.0 / 4.5].