This is **QC-2**, Systems Engineer. We are targeting the "Gravity/Try" era Trio tone. The physics of this sound rely on a high-headroom, high-voltage preamp (Two-Rock) with a specific mid-frequency dip, coupled with a "mid-hump" overdrive to fill that gap during leads.

**Speaker Profile:** Optimized for QSC CP12 (Flat/Default Mode).
**Closest Available Model:** **US Two-Stone Custom** (Based on Two-Rock Custom Reverb Signature).

### **1. Signal Chain Layout**
This layout utilizes a Split-Bank Matrix. Row 1 is optimized for the **Fender Telecaster** (Low Output); Row 2 is optimized for the **Gibson ES-339** (Medium-High Output).

---

### **Table A: Main Signal Chain (Row 1: Telecaster / Row 2: ES-339)**

| Block Category | Model Name | Telecaster (Sc A/B) | ES-339 (Sc E/F) | Physics / Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input / Gate** | Global Input | **Gain: +2.5dB** / Thresh: -65dB | **Gain: -3.0dB** / Thresh: -60dB | Prevents 339 from choking the preamp; boosts Tele "body." |
| **Pre-FX 1** | Minotaur (Klon) | Gain: 1.5, Treble: 6.0, Vol: 7.5 | Gain: 1.0, Treble: 5.0, Vol: 6.5 | "Always On" clean boost. Lower treble for 339 to avoid mud. |
| **Pre-FX 2** | Green 808 (TS) | **(Assign to Lead)** Gain: 4.5, Tone: 5.5 | **(Assign to Lead)** Gain: 3.0, Tone: 4.0 | TS-style mid-push for soloing. 339 needs less gain to stay "smooth." |
| **Amp** | **US Two-Stone** | Gain: 4.5, Bass: 5.5, **Mid: 3.2**, Treb: 7.2 | Gain: 3.5, Bass: 4.0, **Mid: 3.5**, Treb: 6.5 | The "Scooped" base. 339 requires lower bass to prevent "farting" out. |
| **EQ Block** | Parametric-8 | **Peak 200Hz (+2dB)** | **Shelf 3.5kHz (-2dB)** | Tele needs weight; 339 needs "glass" management. |
| **Cab** | 212 Two-Stone | Mic 121 (Pos 0.40), Mic 57 (Pos 0.55) | Mic 121 (Pos 0.35), Mic 57 (Pos 0.60) | 121 Ribbon provides the "smoothness"; 57 provides the "cut." |
| **Post-FX** | Plate Lush | Mix: 18%, Decay: 2.8s | Mix: 15%, Decay: 2.5s | Mayer uses high-end studio reverb for "depth." |

---

### **2. Setup Instructions & Scene Logic**

#### **For Fender Telecaster (Row 1: Scenes A-D)**
*   **Physics Goal:** Compensate for thin single coils while maintaining "cluck."
*   **Input Stage:** Use the +2.5dB Input Gain to hit the **Minotaur** block harder. This simulates a "fatter" pickup.
*   **Scene A (Rhythm):** Clean, Minotaur ON (Low gain). Output: -1.5dB.
*   **Scene B (Lead):** Minotaur ON + Green 808 ON. This stacks the drives for that "bold as love" sustain. Output: +1.5dB.

#### **For Gibson ES-339 (Row 2: Scenes E-H)**
*   **Physics Goal:** Prevent the humbucker from over-saturating the preamp, which kills the "D-style" Bloom.
*   **The Headroom Rule:** The Input Gain is set to **-3.0dB**. If the tone feels "fuzzy" or "congested" in the low-mids, lower this further to -4.5dB.
*   **Scene E (Rhythm):** Clean. The US Two-Stone Gain is lowered to 3.5 to keep the ES-339 articulate.
*   **Scene F (Lead):** Green 808 ON, but with Tone rolled back. Humbuckers produce more natural mids, so the TS-style pedal is used primarily for sustain, not EQ.

---

### **3. Troubleshooting the "Trio" Tone**

1.  **Too "Ice-Pick" (Telecaster):** Go to the **Parametric-8 EQ** and lower the High Frequency (Band 8) to 4500Hz using a Low Pass Filter (LPF). This replicates the high-end roll-off of a long coil cable.
2.  **Too "Boomy" (ES-339):** The US Two-Stone is a bass-heavy circuit. If the QSC CP12 is vibrating excessively, reduce the **Bass** parameter on the Amp block to 3.5. Do not use a compressor; use the Amp's "Master" to control the feel of the sag.
3.  **Gain Staging Check:** With your loudest strum on the ES-339, ensure the Input Circle on the QC is **Green/Yellow**, never Red. If it hits Red, engage the **Input Pad** in the I/O Settings.

---

### **Session Library (Active Presets)**
1. **Preset Name: "JM Trio - Two-Stone"**
   *   **Target:** John Mayer Trio / Continuum.
   *   **Guitars:** Telecaster (S/S) & ES-339 (H/H).
   *   **Physics Goal:** Mid-scooped high-headroom clean platform with stacked drive physics.
   *   **Core Model:** US Two-Stone Custom.
   *   **Status:** Gig-Ready (Optimized for QSC CP12).