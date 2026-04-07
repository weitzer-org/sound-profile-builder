This is **QC-2**, Quad Cortex Systems Engineer. To achieve the Joe Bonamassa "Dumble" lead tone—specifically that high-headroom, cascading gain "violin" sustain—we will utilize the **Overdrive Special Lead** model. 

For the **QSC CP12**, we must manage the high-mid resonance to avoid the "boxy" characteristic of active PAs. We will use the **Split-Bank Matrix** to optimize for your ES-339 and Telecaster.

### **The "Bonamassa ODS" Protocol**
*   **Physics Logic:** JB’s tone relies on the "cascading gain" of the ODS circuit. The `Drive` controls the distortion, while the `Ratio` controls how much of that gain is pushed into the power section. 
*   **Monitoring Note:** The QSC CP12 has a DSP "Contour" switch. Ensure it is set to **"Default" or "Dance" (Flat)**. If you use the "Vocal/Speech" setting, this preset will sound overly bright.

---

### **Table A: Main Signal Chain**

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | **Adaptive Gate** | Red: 45% / Thresh: -55dB | Red: 20% / Thresh: -62dB | Lower reduction on Lead for natural sustain/feedback. |
| **Pre-FX** | **Green 808** | Gain: 0 / Level: 6.0 | Gain: 1.5 / Level: 7.5 | Mid-hump to tighten the ODS bottom end. |
| **Amp** | **Overdrive Special Lead** | Drive: 4.5 / Ratio: 5.5 | Drive: 6.8 / Ratio: 7.5 | Cascading gain logic; higher Ratio increases "bloom." |
| **Cab** | **412 ODS (Factory)** | Mic: Dyn 57 / Ribbon 121 | Mic: Dyn 57 / Ribbon 121 | 57 for bite, 121 for woodiness. |
| **Post-EQ** | **Parametric-8** | (See Guitar Specifics) | (See Guitar Specifics) | Compensating for Pickup inductance. |
| **Spatial 1** | **Vintage Tape** | Mix: 15% / 450ms | Mix: 25% / 500ms | 3-4 repeats. "Echoplex" style wash. |
| **Spatial 2** | **Plate Lush** | Mix: 10% / Decay: 2.2s | Mix: 18% / Decay: 2.8s | Large hall feel without washing out the dry signal. |

---

### **Multi-Guitar Configuration & Gain Staging**

#### **Target 1: Gibson ES-339 (Humbuckers)**
*Scenes E, F, G, H*
*   **Input Block Gain:** **-4.0dB**. (Prevents the humbuckers from hitting the Green 808 too hard, which causes "fuzz" instead of "drive").
*   **Amp EQ:** Bass: 3.8 / Mid: 6.5 / Treble: 5.2.
*   **Parametric-8 (Clarity Focus):** 
    *   **Band 2 (Mud):** -3.0dB at 250Hz (Q: 1.5).
    *   **Band 7 (Air):** +2.0dB at 4kHz High Shelf (adds "pick click" definition).
*   **Physics Goal:** Maintaining note separation during fast pentatonic runs.

#### **Target 2: Fender Telecaster (Single Coils)**
*Scenes A, B, C, D*
*   **Input Block Gain:** **+1.5dB**. (Compensates for lower output; ensures the ODS circuit "blooms" correctly).
*   **Amp EQ:** Bass: 5.2 / Mid: 7.5 / Treble: 4.5.
*   **Parametric-8 (Body Focus):** 
    *   **Band 1 (Weight):** +3.5dB at 180Hz (Low Shelf). Adds the "thump" usually missing from Teles.
    *   **Band 8 (Smoothing):** LPF (Low Pass Filter) at **5.2kHz**. Essential to remove the "ice-pick" when using the bridge pickup with high ODS gain.
*   **Physics Goal:** Giving a 25.5" scale guitar the "mass" of a shorter scale mahogany guitar.

---

### **Execution Strategy (Cortex Control)**
1.  **Gain Staging:** If the ES-339 sounds "farty" on low notes, go to the **Overdrive Special Lead** block and reduce the `Bass` parameter before touching the gain. Dumble circuits are notorious for low-end flub.
2.  **Scene Assignment:** 
    *   Right-click `Amp Drive` and `Amp Ratio`. 
    *   Assign to **Scene**. 
    *   Set higher values for Scenes B (Tele Lead) and F (ES-339 Lead).
3.  **Output Volume:** Use the **Lane Output Level** (far right of the grid) to balance the ES-339 and Tele banks so you don't have to reach for your QSC volume knob when switching guitars.

---

### **Session Library (Active Presets)**
1. **Preset Name: "JB ODS - Dual Path"**
   * **Target:** Joe Bonamassa / Dumble ODS.
   * **Physics Goal:** Smooth cascading gain for QSC CP12.
   * **Key Components:** Overdrive Special Lead, Green 808, Parametric-8 (Pickup Match).
   * **Input Staging:** ES-339 (-4.0dB) / Tele (+1.5dB).
   * **EQ Strategy:** 180Hz Boost for Tele / 250Hz Cut for ES-339.