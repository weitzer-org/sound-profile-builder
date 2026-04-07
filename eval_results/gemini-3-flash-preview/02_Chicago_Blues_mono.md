Hello. I am **QC-2**, your Quad Cortex Systems Engineer. 

To achieve that mid-to-late 50s **Chess Records** sound (think Hubert Sumlin or Muddy Waters), we must focus on the physics of a small-to-medium vintage combo amp being pushed into its sweet spot—where the "clean" still has a "halo" of grit. Because you are using a **QSC CP12** (a flat-response PA speaker), our goal is to simulate the cabinet’s physical roll-off to avoid the clinical high-end of a powered monitor.

### Physics-First Implementation
*   **The Tube Taper:** On vintage Tweed circuits (US DLX 58 / US Tweed Basslad), the "Volume" is both your gain and your EQ. As you increase it, the bass becomes "farty" or "loose." We will use the Parametric EQ-8 to tighten the low end before it hits the amp.
*   **The Microphone Profile:** Chess Records used early ribbon mics and dynamics. I will spec a **Ribbon 121** and a **Dynamic 57** combo to provide that "warm" texture.

---

### **Configuration 1: Gibson ES-339 (Humbuckers)**
*Target: Focus on clarity and preventing "mud" in the low-mids.*

| Block Category | Model Name | Rhythm (Sc E) | Lead (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -65dB | Thresh: -62dB | Low threshold to preserve long, bluesy sustains. |
| **Pre-FX** | EQ-Parametric 8 | HPF: 110Hz / LPF: 4.5kHz | Same | Cuts the "humbucker mud" before the amp compresses it. |
| **Amp** | **US Tweed Basslad** | Vol Norm: 3.5 / Vol Br: 4.0 | Vol Norm: 4.5 / Vol Br: 5.5 | Bassman circuit. "Jumped" inputs for maximum body. |
| **Cab** | 410 Basslad PR10 | Mic A (121): Pos 0.4 / Dist 2" | Same | Ribbon mic provides the "warmth" of old studio tape. |
| **Post-FX** | Room Reverb | Mix: 15% | Mix: 20% | Simulates the 2120 S. Michigan Ave studio floor. |
| **Output** | Lane 1 Output | **-2.5dB** | **+1.5dB** | Use this for overall loudness (No Master Vol on this amp). |

---

### **Configuration 2: Fender Telecaster (Single Coils)**
*Target: Adding "weight" and "body" to the bridge pickup while taming "ice-pick" highs.*

| Block Category | Model Name | Rhythm (Sc A) | Lead (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | **Gain: +2.0dB** | **Gain: +3.0dB** | Compels the single coil to hit the amp stage like a humbucker. |
| **Pre-FX** | Myth Drive | Drive: 0.5 / Level: 6.0 | Drive: 2.5 / Level: 6.5 | "Klon" style mid-bump to thicken the thin Tele bridge. |
| **Amp** | **US DLX 58** | Inst Vol: 5.0 / Tone: 6.0 | Inst Vol: 7.0 / Tone: 5.5 | The Tweed Deluxe. Higher gain than the Bassman, more sag. |
| **Cab** | 112 DLX 58 | Mic A (57): Pos 0.2 / Dist 1" | Mic A (57): Pos 0.3 / Dist 2" | Dynamic mic provides the "bite" for Chicago slide work. |
| **Post-FX** | Tape Delay | Mix: 8% / Time: 125ms | Mix: 12% | Vintage slapback to thicken the single-coil note. |
| **Output** | Lane 1 Output | **-1.0dB** | **+2.0dB** | Compensates for the lower RMS of the single-coil signal. |

---

### **System Engineering Notes & Troubleshooting**

1.  **Pickup Compensation:** If your **ES-339** feels "fuzzier" than intended, go to the **Input Block** and set the Gain to **-3.0dB**. This mimics a lower-output vintage PAF and restores the "clean platform" feel.
2.  **The "Tweed Sizzle":** Small Tweed amps can sound "fizzy" through a QSC CP12 because the speaker reproduces frequencies a real guitar speaker cannot. 
    *   *Fix:* In the **Cab Block**, ensure the **High Cut** is set between **4.5kHz and 5.2kHz**. 
3.  **No Master Volume:** Note that the *US DLX 58* and *US Tweed Basslad* do **not** have a Master Volume parameter on the QC (matching the physical hardware). Do not attempt to find one. Adjust your volume via the **Lane Output Level** (last circle on the right of the grid).
4.  **Scene Logic:** 
    *   **Scenes C/G (Dry):** Bypass the Reverb and Delay blocks for a "raw" 1951 Chess recording sound.

---

### **Session Library (Active Presets)**
1.  **Preset Name: "Chess Era Blues"**
    *   **Target:** Howlin' Wolf / Hubert Sumlin / Muddy Waters.
    *   **Hardware:** ES-339 (HB) & Telecaster (SC).
    *   **Physics Goal:** Vintage sag, ribbon mic warmth, edge-of-breakup "halo."
    *   **Signal Path (HB):** In > EQ-8 (HPF/LPF) > US Tweed Basslad > 410 Basslad (121/57) > Room Reverb > Out.
    *   **Signal Path (SC):** In (+2dB) > Myth Drive > US DLX 58 > 112 DLX 58 (57) > Tape Slap > Out.

**Ready for synchronization. Would you like to refine the "sag" response or adjust the Slapback timing for a specific song?**