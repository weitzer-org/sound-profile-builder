This is **QC-2**, Quad Cortex Systems Engineer. Target identified: **Slash "AFD/Illusion" Lead Tone**.

To achieve this on a Quad Cortex using Cortex Control, we must address the "neck pickup bloom." Slash’s tone relies on a pushed Marshall power section and a specific mid-range hump that prevents the neck humbucker from becoming "woolly" or "farty."

### Physics & Hardware Constraints
*   **Speaker Profile:** QSC CP12 (Set to "Default" or "Dance" mode for flat response; do not use "Floor Monitor" mode as it cuts the low-end we need for the 4x12 resonance).
*   **The Headroom Rule:** To maintain the "Hot 800" feel without digital fizz, we will run the **Brit 800** Master Volume high (simulating power tube saturation) and use the **Lane Output** to trim the final dB for the CP12.

---

### 1. Main Signal Chain (Split-Bank Matrix)

| Block Category | Model Name | Rhythm (Sc A/E) | Lead (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Input Circle 1 | Thresh: -65dB | Thresh: -60dB | Lower threshold for Lead to allow sustain/feedback. |
| **Pre-FX (Drive)** | Green 808 | Drive: 0 / Level: 6.5 | Drive: 1.0 / Level: 8.0 | Acting as a mid-hump filter, not a distortion source. |
| **Amp** | **Brit 800** | Gain: 6.5 / Mast: 7.5 | Gain: 7.2 / Mast: 8.0 | High Master mimics the EL34 "sag" crucial for Slash. |
| **Cab** | **412 Brit V30** | Mic 1: 57 / Mic 2: 121 | (Same) | V30 speakers provide the iconic 3kHz "bark." |
| **Post-FX (Delay)** | Digital Delay | Mix: OFF | Mix: 22% / 420ms | Standard 80s/90s rack delay voicing. |
| **Post-FX (Verb)** | Hall Reverb | Mix: 8% | Mix: 15% | Simulates a large studio room/arena. |

---

### 2. The Chameleon Strategy: Pickup Compensation
Because you are switching between an **ES-339 (Humbuckers)** and a **Telecaster (Single Coils)**, the **Parametric-8 Block** is mandatory at the end of the chain (Post-Cab) to equalize the physical displacement of the pickups.

**Table B: Parametric-8 EQ Settings**
| Band | Frequency | ES-339 (Scenes E-H) | Telecaster (Scenes A-D) | Logic |
| :--- | :--- | :--- | :--- | :--- |
| **1 (HPF)** | 80Hz | ON | ON | Remove sub-low "mud." |
| **2 (Body)** | 220Hz | -2.5dB (Wide Q) | +3.5dB (Wide Q) | Adds "wood" to Tele; thins out 339 neck. |
| **5 (Mids)** | 750Hz | 0.0dB | +2.0dB | Pushes the Tele into the "Gibson" mid-range. |
| **8 (LPF)** | 4.5kHz | OFF | ON (5.2kHz) | Tames Tele "ice-pick" for high-gain leads. |

---

### 3. Guitar-Specific Gain Staging

#### **Target 1: Gibson ES-339 (Humbuckers)**
*   **Input Gain:** Set to **-3.0dB**. The 339 has significant resonance; we need to prevent the Brit 800's input stage from "choking."
*   **Scene E (Rhythm):** Amp Gain at 6.0. Focus on bridge pickup.
*   **Scene F (Lead):** This is your Slash moment. **Switch to Neck Pickup.** The Green 808 block will tighten the low-end, while the Brit 800 provides the "singing" sustain.
*   **Constraint:** If the tone is "farty" on low notes, reduce the **Amp Bass** to 3.5.

#### **Target 2: Fender Telecaster (Single Coil)**
*   **Input Gain:** Set to **+3.5dB**. We need to "fake" the output of a PAF humbucker to drive the Brit 800 front end.
*   **Scene A (Rhythm):** Amp Gain at 7.5. (Single coils need more gain to reach the same saturation level).
*   **Scene B (Lead):** Use the **Bridge Pickup** with the Tone knob rolled back to 7. The Parametric EQ (Table B) will handle the rest.
*   **Constraint:** If the Tele is too noisy, increase the **Adaptive Gate Noise Reduction** to 45%.

---

### 4. Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"** (Previous Session)
**2. Preset Name: "Paradise Lead - Multi"**
*   **Target:** Slash / GNR Lead.
*   **Guitars:** ES-339 & Telecaster.
*   **Physics Goal:** EL34 Power Tube Saturation via high Master Volume + Mid-range compensation EQ.
*   **Full Configuration:**
    *   **Block 1 (Input):** Gate [-60dB], Input Gain [Var: -3.0 / +3.5].
    *   **Block 2 (Green 808):** Tone [5.5], Level [7.0], Drive [1.0].
    *   **Block 3 (Brit 800):** Bass [4.2], Mid [6.5], Treb [5.8], Mast [8.0], Gain [Var: 6.5 / 7.5], Pres [5.0].
    *   **Block 4 (412 Brit V30):** Mic A: Dyn 57 (Pos 0.35), Mic B: Ribbon 121 (Pos 0.55). Mix -3dB.
    *   **Block 5 (EQ-8):** (See Table B for Chameleon Strategy).
    *   **Block 6 (Digital Delay):** 420ms, Feedback [25%], Mix [22% Lead].
    *   **Lane Output:** -4.5dB (to protect CP12 headroom).

**Engineer Note:** For the Slash "Neck Pickup" lead, ensure your ES-339 Tone knob is around 6-8. This mimics the "woman tone" characteristic of his AFD recordings. **Do not use the neck pickup on the Telecaster for this preset; stay on the Bridge or Middle position for clarity.**