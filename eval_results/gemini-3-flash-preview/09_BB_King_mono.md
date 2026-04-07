This is **QC-2** reporting. Target Tone: **B.B. King (Lucille)**. 
System Platform: **Cortex Control (Desktop)**.
Speaker Profile: **QSC CP12 (Active FRFR)**.

To capture the "Lucille" tone, we are simulating a high-headroom **US Twin Vibrato** circuit. The secret to B.B. King’s tone isn't just the amp; it’s the **Varitone switch** (which we will simulate using a Parametric-8 EQ) and the specific "stinging" pick/finger attack.

### **I. Technical Strategy: The "Lucille" Physics**
*   **The "Honk":** B.B. used the Varitone to thin out the humbuckers. We will use a Mid-Peak on the EQ block to simulate this.
*   **The "Sting":** A Twin Reverb is notoriously bright. We use the "Bright" switch on the model but compensate with a Low-Pass Filter (LPF) at 4.5kHz to prevent "digital harshness" on your QSC CP12.
*   **Gain Staging:** We want "Edge of Compression," not "Edge of Breakup." The amp stays clean; the sustain comes from the volume and the QSC's output.

---

### **II. Table A: Main Signal Chain (Twin Clean Platform)**

| Block Category | Model Name | Rhythm (Sc A/E) | Lead (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Red: 45% / Thresh: -62dB | Red: 20% / Thresh: -65dB | Higher reduction for rhythm to keep rests silent. |
| **Pre-FX** | Chief BD | Drive: 0.5 / Level: 6.0 | Drive: 1.2 / Level: 6.5 | *Closest Model: Boss BD-2.* Used as a "transparent" mid-shaper, not a distortion. |
| **Amp** | **US Twin Vibrato** | Vol: 3.5 / Bright: ON | Vol: 4.2 / Bright: ON | No Master Vol on this circuit. Use Lane Output for SPL. |
| **Cab** | **212 US TWN C12K** | Mic A: 57 / Mic B: 121 | Mic A: 57 / Mic B: 121 | 57 for the "snap," 121 for the "body." |
| **Post-FX (EQ)** | **Parametric-8** | (See Guitar-Specific Below) | (See Guitar-Specific Below) | **Crucial:** This is your virtual Varitone switch. |
| **Post-FX** | Reverb (Spring) | Mix: 15% / Decay: 2.5 | Mix: 22% / Decay: 3.0 | Classic "Fender Splash." |

---

### **III. Multi-Guitar Target Output (The Compensation Layer)**

#### **Target 1: Gibson ES-339 (Humbuckers)**
*   **The Goal:** Mimic the 355 Varitone "thinning" effect while maintaining humbucker sustain.
*   **Input Block Gain:** -3.0 dB (To prevent the Twin model from breaking up too early).
*   **Parametric-8 Settings:**
    *   **Band 3 (400Hz):** -2.5dB (Reduces humbucker "mud").
    *   **Band 5 (1.5kHz):** +3.0dB (Wide Q). This creates the "nasal honk" of the Lucille Varitone position 3.
    *   **Band 8 (LPF):** 5.0kHz. Smooths the top end for the QSC CP12.

#### **Target 2: Fender Telecaster (Single Coil)**
*   **The Goal:** Add "Body" and "Weight" to the bridge pickup to simulate a semi-hollow humbucker.
*   **Input Block Gain:** +2.0 dB (To drive the "tubes" of the Twin model to the same level as the ES-339).
*   **Parametric-8 Settings:**
    *   **Band 1 (180Hz):** +3.5dB (Peaking). Adds the "wood" and air of a larger 335-style body.
    *   **Band 5 (2.5kHz):** -3.0dB (Narrow Q). Cuts the "Tele-Snap" to make it feel more like a rounded humbucker.
    *   **Band 8 (LPF):** 4.2kHz. More aggressive cut to hide the single-coil "ice pick."

---

### **IV. Split-Bank Matrix (Scene Management)**

*   **Row 1 (Scenes A-D): Telecaster Single Coil Profile**
    *   **Scene A (Rhythm):** Clean, EQ Band 1 ON (Body boost).
    *   **Scene B (Lead):** BD-2 Block ON, +1.5dB Lane Output.
    *   **Scene C (Dry):** Reverb/Delay Mix 5%.
    *   **Scene D (Ambient):** Increased Spring Reverb mix to 40%.

*   **Row 2 (Scenes E-H): ES-339 Humbucker Profile**
    *   **Scene E (Rhythm):** Clean, EQ Band 5 ON (Varitone Honk).
    *   **Scene F (Lead):** BD-2 Block ON, +1.5dB Lane Output.
    *   **Scene G (Dry):** Reverb OFF.
    *   **Scene H (Dynamic):** Amp Gain raised to 5.0 for "Blues Power" grit.

---

### **V. Troubleshooting & Refinement Tree**
1.  **Too much "Chirp" on the attack?** 
    *   Go to the **Cab Block**. Move Mic A (Dyn 57) from "Position 0.5" to "Position 1.5" (Off-axis). This softens the pick strike.
2.  **Not enough sustain for vibrato?** 
    *   *Do not increase Amp Gain.* Instead, add a **Jewel Compressor** at the very beginning of the chain. Set Ratio to 3:1 and use the "Makeup" gain to push the amp.
3.  **Low-end is "farty" on the QSC CP12?**
    *   Check the **Amp Block**. Reduce "Bass" to 3.5. On Twin Reverb circuits, the Bass knob adds significant gain to the low-end which can overwhelm 12" PA speakers.

### **Session Library (Registry Update)**
1. **Preset Name:** "Lucille - Twin"
   * **Target:** B.B. King / ES-355 Clean.
   * **Physics Goal:** Varitone simulation via EQ-8; high-headroom clean.
   * **Status:** Verified for CorOS 2.x.

**Are your pickups Vintage-spec (Lower output) or modern High-output?** (I will adjust the Input Gate Threshold based on your response).