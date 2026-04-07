This is **QC-2**, Systems Engineer. We are configuring for the **"Brown Sound"**—a specific intersection of power-tube saturation, voltage starvation (Variac), and the "wet-dry-wet" studio processing used on *Van Halen I*.

### 1. Physics Analysis: The "Brown" Architecture
*   **The Variac Sag:** By dropping voltage to ~90V, EVH induced a specific compression where the low end doesn't "fart out," and the highs become "silky" rather than "glassy." On the Quad Cortex, we simulate this using the **Brit Plexi 100 Bright** model, pushing the **Master Volume** high to trigger power-amp distortion, then reigning in the **Level** to protect the QSC CP12’s headroom.
*   **The Speaker Physics:** 1960B Cabs with Celestion G12M "Greenbacks." We will use a dual-mic setup (SM57 and Royer 121) to capture the "bark" and "body."
*   **The Signal Chain:** Guitar > Phase 90 (Pre-Amp) > Plexi > Plate Reverb (Post-Amp).

---

### 2. Multi-Guitar Configuration Matrix
*Are your pickups Vintage Output (PAF style) or High Output?*
**Standard Assumption:** ES-339 (Medium-High Humbucker) / Telecaster (Vintage Single Coil).

#### **Table A: Main Signal Chain (CorOS 2.x/3.x)**

| Block Category | Model Name | ES-339 Settings (Sc E-H) | Telecaster Settings (Sc A-D) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | **Red: 25%** / Thresh: -62dB | **Red: 45%** / Thresh: -58dB | Higher reduction for Tele to kill single-coil hum. |
| **Pre-FX (Phaser)** | Chief Phase | Rate: 0.8Hz / Depth: 50% | Rate: 1.1Hz / Depth: 65% | "Eruption" style sweep. Tele needs more depth to "feel" the phase. |
| **Amp** | **Brit Plexi 100 Bright** | **Gain: 6.5 / Master: 8.5** | **Gain: 8.5 / Master: 9.0** | High Master = "Variac" Sag. Tele requires more Gain to hit the "brown" threshold. |
| **EQ (Parametric-8)** | Parametric-8 | Band 2: -2dB @ 300Hz | Band 2: +3dB @ 250Hz | **"The Body Rule"**: Adding weight to the Tele; removing mud from the ES-339. |
| **Cab (Dual)** | **Brit 412 Straight** | Mic A: 57 / Mic B: 121 | Mic A: 57 / Mic B: 160 | Ribbon mics (121/160) are essential for the "Brown" smoothness. |
| **Post-FX (Plate)** | Plate Lush | Mix: 18% / Decay: 2.4s | Mix: 22% / Decay: 2.8s | Simulates the Sunset Sound plate reverb chamber. |

---

### 3. Split-Bank Scene Allocation (The Chameleon Strategy)

**Row 1 (Telecaster Profile):**
*   **Scene A (Rhythm):** Gain at 7.0. Tight, percussive. LPF at 4.5kHz (to tame bridge-pickup ice-pick).
*   **Scene B (Lead):** Gain at 8.5. Level +2.0dB. Delay added (1/4 note, low mix).
*   **Scene C (Dry):** Phase/Reverb OFF. Pure "Variac" Plexi bark.
*   **Scene D (Unchained):** Flanger (Gray Flanger) added.

**Row 2 (ES-339 Humbucker Profile):**
*   **Scene E (Rhythm):** Gain at 5.5. Input Gain -3.0dB (to preserve ES-339 clarity).
*   **Scene F (Lead):** Gain at 7.5. Level +1.5dB. **The "Brown" EQ:** +2dB boost at 1.5kHz for solo cutting.
*   **Scene G (Dry):** Reverb/Delay OFF. Focus on "Mean Street" style percussive muting.
*   **Scene H (Ambient):** Plate Mix at 40% for "Cathedral" vibes.

---

### 4. Gain Staging & Optimization Protocols

**The "Variac" Leveling:**
Because the **Master Volume** on the Brit Plexi 100 model is set to **8.5-9.0**, the signal will be extremely hot. 
*   **Action:** Reduce the **Amp Block "Level"** to -10dB or lower. 
*   **Action:** Use the **Lane Output Level** (Right side of the grid) to bring the signal up to -6dB on the output meters. This ensures your QSC CP12 isn't being hit with a digital "over" signal, which causes harshness.

**Telecaster "Single Coil" Optimization:**
*   To prevent the Telecaster from sounding thin compared to the ES-339, I have increased the **Amp Master** to 9.0 for Scenes A-D. This increases "Tube Sag," making the single coils feel "chewy" and less "plinky."

**Troubleshooting "Fuzziness":**
If the ES-339 sounds "farty" or the low-end is "broken":
1.  Go to the **Amp Block**.
2.  Reduce **Bass** to 3.0 (EVH famously ran Bass very low).
3.  Increase **Presence** to 7.5 to regain "sizzle."

---

### 5. Session Library (Preset Registry)
**Preset Name:** "1978 Brown Sound"
*   **Target:** Van Halen I Era.
*   **Guitar A:** ES-339 (Humbuckers).
*   **Guitar B:** Telecaster (Single Coil).
*   **Amp Model:** Brit Plexi 100 Bright.
*   **Cab:** Brit 412 Straight (G12M).
*   **Key Physics:** Master Volume at 8.5+ to simulate Variac voltage drop. Lane Output used for SPL control.

**Engineer Note:** "When switching from the ES-339 to the Tele, stay in Rows E-H for the Humbuckers and A-D for the Telecaster. The EQ-8 block is hard-coded to compensate for the Tele's lack of mid-range 'meat' automatically."