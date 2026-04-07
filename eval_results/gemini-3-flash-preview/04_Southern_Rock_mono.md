This is QC-2. We are targeting the "Fillmore East" era of Southern Rock—think Duane Allman’s singing sustain and Derek Trucks’ vocal-like slide articulation. To achieve this on the Quad Cortex for a QSC CP12 (which is very linear), we must manage the low-end "flub" inherent in cranked Tweed circuits while maintaining the harmonic bloom required for slide.

### **The Physics of the Build**
*   **The Amp:** We are using the **US Tweed Basslad Jumped**. By jumping the channels, we blend the "Normal" (body) and "Bright" (bite) circuits.
*   **The Challenge:** Slide guitar requires massive sustain but high clarity. Humbuckers will easily "choke" a Tweed amp's input. We will use the **Input Block Gain** to regulate the "push" and the **Parametric-8 EQ** to sculpt the "honk" around 400Hz–600Hz.

---

### **Table A: Main Signal Chain (Southern Slide)**

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | **Global Circle 1** | Thresh: -55dB | Thresh: -62dB | Looser gate on Lead to prevent cutting off slide vibrato tails. |
| **Pre-FX** | **Green 808** | Gain: 1.0 / Level: 6.0 | Gain: 2.5 / Level: 7.0 | Used as a mid-hump filter. Tightens the Tweed's loose low end. |
| **Amp** | **US Tweed Basslad Jumped** | Vol Norm: 4.5 / Bright: 5.5 | Vol Norm: 5.0 / Bright: 7.5 | No Master Vol on this model. High Bright Vol provides the "singing" grit. |
| **EQ (Post)** | **Parametric-8** | HPF: 100Hz / LPF: 4.5kHz | 450Hz: +2.5dB (Q: 1.2) | Boosting 450Hz provides the "vocal" quality essential for slide. |
| **Cab** | **410 Basslad PR10** | Mic A: 57 (Pos: 1.5) | Mic B: 121 (Pos: 2.5) | Ribbon 121 (Mic B) adds the "creamy" high-end roll-off for the QSC CP12. |
| **Post-FX** | **Tape Delay** | Mix: 12% / Feed: 15% | Mix: 22% / Feed: 28% | "Slap-ish" settings (180ms) to create depth without washing out the dry signal. |

---

### **Multi-Guitar Target Output**

#### **1. Gibson ES-339 (Humbuckers) Configuration**
*   **Input Block Gain:** **-4.5dB**. Humbuckers will hit the US Tweed Basslad too hard, causing the low strings to "fart out." This reduction preserves headroom.
*   **Amp Sag (Advanced Params):** Set to **4.0**. Increasing Sag simulates the tube rectifier "giving" under the weight of humbucker chords.
*   **EQ-8 Adjustment:** Reduce **200Hz by -3.0dB**. This prevents the ES-339's semi-hollow body from feeding back at gig volumes.
*   **Scene Strategy (Row 2: E-H):** Focus on "Clarity." Use Scene F (Lead) with the Green 808 engaged but Gain kept low to avoid turning the tone into "Fuzz."

#### **2. Fender Telecaster (Single Coil) Configuration**
*   **Input Block Gain:** **+2.0dB**. This compensates for the lower magnetic pull of the single coils and ensures the "Jumped" channels still break up.
*   **Amp Settings:** Reduce **Treble to 4.5** and **Presence to 4.0**. Telecasters can be "ice-picky" through Tweed circuits; we want the mids to do the work, not the highs.
*   **EQ-8 Adjustment:** Boost **Low Shelf (150Hz) by +2.0dB**. This adds "weight" to the bridge pickup so the slide notes sound thick like a humbucker.
*   **Scene Strategy (Row 1: A-D):** Focus on "Body." Scene B (Lead) should have the **Green 808 Tone set to 4.0** to keep the top end sweet.

---

### **Session Library (Active Presets)**

1.  **Preset Name: "Spoonful - ES339"** (Howlin' Wolf / Sumlin Style)
2.  **Preset Name: "Skydog Slide"**
    *   **Target:** Duane Allman / Derek Trucks.
    *   **Guitar:** Optimized for ES-339 (HB) and Telecaster (SC).
    *   **Physics Goal:** Mid-forward "vocal" sustain for slide; physics-aligned for QSC CP12.
    *   **Full Configuration:**
        *   **Block 1 (Input):** Gate Thresh [-58dB], Input Gain [SC: +2dB / HB: -4.5dB].
        *   **Block 2 (Green 808):** Gain [2.0], Tone [5.0], Level [7.0].
        *   **Block 3 (US Tweed Basslad Jumped):** Vol Norm [5.0], Vol Bright [7.0], Bass [3.5], Mid [7.5], Treble [5.5], Presence [5.0].
        *   **Block 4 (EQ-8):** HPF [100Hz], 450Hz [+3dB], 2.5kHz [+1.5dB], LPF [5kHz].
        *   **Block 5 (410 Basslad Cab):** Mic A (57) 40% Mix, Mic B (121) 60% Mix. Dist: 1.0".
        *   **Block 6 (Plate Reverb):** Decay [2.2s], Mix [15%], Pre-delay [20ms].

**QC-2 Engineering Note:** If the slide vibrato sounds "choppy," immediately go to the **Input Gate (Circle 1)** and lower the Threshold further. Tweed circuits have a high noise floor when cranked; the Adaptive Gate (%) is better for high-gain metal, but for this Southern Rock build, the Global Input Gate provides a more natural decay for slide.