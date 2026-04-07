Hello. I am **QC-2**, your Quad Cortex Systems Engineer. 

To achieve the "Pink Floyd" stadium sound through your **QSC CP12**, we must account for the high headroom of the Hiwatt circuit and the massive harmonic saturation of the Big Muff. Because the CP12 is a flat-response (FRFR) monitor, we will use the **Lane Output Level** to manage volume without clipping the virtual power tubes of the Hiwatt model.

Below is your technical build for the **David Gilmour "Lunar Plate"** preset.

### **Pickup Compensation Protocol**
*   **Gibson ES-339 (Humbuckers):** High output. We will reduce the **Input Gain** to **-4.5dB** and lower the **Piggy (Muff)** sustain to prevent "mud" in the low-mids.
*   **Fender Telecaster (Single Coils):** Lower output. We will keep **Input Gain** at **0.0dB** and use the **EQ-8** to add "Body" at 200Hz to simulate the thickness of Gilmour’s Black Strat.

---

### **Table A: Main Signal Chain (Dark Side / Wall Specs)**

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Red: 45% / Thresh: -62dB | Red: 15% / Thresh: -68dB | Keeps Fuzz noise silent during rests; Lead has lower reduction for sustain. |
| **Pre-FX (Comp)** | **Grey Comp** | Level: 6.5 / Sens: 3.0 | Level: 7.0 / Sens: 4.5 | Essential "Ross" style squeeze for the "Wall" lead sustain. |
| **Pre-FX (Drive)** | **Piggy** (Muff) | **BYPASS** | Gain: [Tele: 6.0 / 339: 4.5] Tone: 5.5 | The Ram's Head "Piggy" provides the violin-like sustain. |
| **Amp** | **UK C100** | Master: 8.5 / Gain: 2.5 | Master: 9.0 / Gain: 3.0 | Hiwatt DR103 Logic: Run the Master high for headroom; Gain low for "pedal platform" clarity. |
| **Cab** | **412 UK C100** | Mic 1: Dyn 57 (Pos 0.25) | Mic 2: Cond 87 (Pos 0.45) | Fane-speaker simulation. Ribbon/Condenser mix provides the "WEM" warmth. |
| **EQ** | **Parametric-8** | HPF: 100Hz / LPF: 4.8kHz | +2.0dB at 1.5kHz (Mid-Hump) | Adapts the ES-339 clarity and the Telecaster body. |
| **Delay** | **Tape** | Time: 480ms / Mix: 15% | Mix: 28% / Fdbk: 35% | Simulates the Binson Echorec used for rhythmic repeats. |
| **Post-FX (Verb)**| **Plate Lush** | Mix: 20% / Decay: 4.5s | Mix: 35% / Decay: 6.0s | The "Massive" Plate requirement. Set Pre-delay to 40ms for clarity. |

---

### **Multi-Guitar Configuration Strategy**

#### **Target 1: Fender Telecaster (Single Coil)**
*   **Input Block:** Gain: **+0.5dB**.
*   **EQ-8 (Band 2 - Body):** Set to 220Hz, Gain **+2.5dB**, Q 0.7. This compensates for the thinness of the bridge pickup when hitting the Big Muff.
*   **Piggy (Muff) Sustain:** Increase to **7.0**. Single coils require more gain to achieve the "infinite" sustain heard on *Comfortably Numb*.

#### **Target 2: Gibson ES-339 (Humbuckers)**
*   **Input Block:** Gain: **-5.0dB**. This prevents the humbuckers from hitting the "Grey Comp" too hard, which would squash your dynamics.
*   **EQ-8 (Band 7 - Air):** High Shelf at 3.5kHz, Gain **+1.5dB**. This keeps the 339 from sounding "wooly" through the Hiwatt.
*   **Piggy (Muff) Sustain:** Decrease to **4.5**. The humbuckers provide natural sustain; excess pedal gain will cause digital artifacts/fuzz "farting."

---

### **Troubleshooting & Refinement**
*   **Issue: "Too much hiss on Lead."** -> Go to the **Adaptive Gate** and increase the **Noise Reduction %** to 30%. Do not touch the Threshold.
*   **Issue: "The low end is flubby."** -> On the **UK C100 Amp Block**, reduce the **Bass** parameter to 3.5. Hiwatts have massive low-end transformers that can overwhelm a 12" PA speaker like the CP12.
*   **Output Logic:** Use the **Lane Output Level** (final circle on the grid) to set your stage volume. For the CP12, keep the QC Physical Volume knob at 100% and adjust the digital Lane Output to roughly **-6.0dB** for safe headroom.

---

### **Session Library (Active Presets)**
1. **Preset Name: "Spoonful - ES339"** (Howlin' Wolf Style - See previous entry).
2. **Preset Name: "Lunar Plate - Gilmour"**
   - **Target:** David Gilmour (1973-1979).
   - **Signal Path:** Grey Comp > Piggy > UK C100 > 412 UK C100 > EQ-8 > Tape Delay > Plate Lush.
   - **Physics Goal:** Clean platform with massive "Big Muff" saturation and high-headroom Hiwatt power section.
   - **Control:** Lane Output used for SPL management on QSC CP12.