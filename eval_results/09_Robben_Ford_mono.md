**System Initialization: QC-2 Active.**
**Target Acquisition:** Robben Ford, "Talk To Your Daughter" (1988).
**Guitar Profile:** Fender Esprit Ultra (Medium-High Output Humbuckers).
**Amp Topology:** Dumble Overdrive Special (100W, 6L6) into a 2x12 (Celestion G12-65s).
**Monitor Constraint:** QSC CP12 (Active FRFR).

### 1. Physics & Gain Staging Protocol
The Dumble Overdrive Special (QC Model: **Gladstone OD**) relies on cascaded gain stages. The Clean channel volume pushes the front of the Overdrive channel. Since you are using Fender Esprit humbuckers, their higher output will naturally compress the front end. 

To achieve Robben’s signature "clean-to-overdrive morphing" without touching a pedal, we will use **Scene Controllers** to manipulate the amp's internal cascading gain logic. Furthermore, to adapt this to the QSC CP12, we must apply aggressive low-pass filtering to tame the 10kHz+ harshness that FRFR speakers reproduce (which analog guitar cabinets physically cannot).

### 2. Output Compensation Strategy
*   **Humbucker Pad (Fender Esprit):** You must drop the Global Input (Circle 1) Gain to **-3.0dB**. The Esprit humbuckers will cause digital clipping and unwanted harshness in the Gladstone model if fed at 0.0dB. 
*   **Dynamic Morphing:** We will assign the `OD Drive` and `OD Level` parameters to Scenes. You will achieve the final 10% of the morph using your guitar's volume knob (roll back to 6 for glassy cleans, push to 10 for thick vocal lead).

---

### 3. Split-Bank Matrix (Cortex Control Configuration)
*   **Row 1 (Scenes A-D):** Single Coil Compensation (Telecaster / Stratocaster)
*   **Row 2 (Scenes E-H):** Humbucker Optimization (Fender Esprit) *(Primary Target)*

#### Table A: Main Signal Chain
*Note: Parameters marked with an asterisk must be mapped via (Right-Click > Assign) to Scenes.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -62dB, Gain: -3.0dB | Thresh: -62dB, Gain: -3.0dB | -3dB pad accommodates the Esprit humbuckers, preserving Dumble headroom. |
| **Pre-FX (EQ)** | Parametric-8 | *HB (Row 2):* Flat<br>*SC (Row 1):* Peak 200Hz +2.5dB | *HB (Row 2):* Flat<br>*SC (Row 1):* Peak 200Hz +2.5dB | Chameleon Block: Simulates Esprit mahogany body resonance when a single coil is plugged in. |
| **Amp** | Gladstone OD | Vol: 6.5<br>OD Drive*: 1.5<br>OD Level*: 7.0<br>Mid: 7.5 | Vol: 6.5<br>OD Drive*: 6.5<br>OD Level*: 5.5<br>Mid: 7.5 | Cascaded morphing. OD Drive increases saturation while OD Level compensates for volume jumps. Heavy mid-push matches Ford's "Rock" switch. |
| **Cab** | 212 Gladstone | Mic 1: Dyn 57 (Pos 1.5, Dist 1")<br>Mic 2: Rbn 121 (Pos 3.0, Dist 2") | *(Same as Rhythm)* | Dyn 57 captures pick attack; Ribbon 121 (off-axis) adds low-mid warmth. **Crucial:** HPF 90Hz / LPF 6000Hz to flatter the QSC CP12. |
| **Post-FX 1** | Chorus (Analog) | Mix: 15%, Depth: 35%, Rate: 1.2Hz | Mix: 20%, Depth: 40%, Rate: 1.2Hz | Simulates the spatial widening Robben got from his TC Electronic 2290. |
| **Post-FX 2** | Vintage Digital Delay | Mix: 12%, Time: 380ms, Fdbk: 20% | Mix: 22%, Time: 380ms, Fdbk: 25% | Thickens the vocal midrange of the lead scene without stepping on transients. |
| **Post-FX 3** | Hall Reverb | Mix: 15%, Decay: 1.8s, Pre-D: 15ms | Mix: 18%, Decay: 2.2s, Pre-D: 15ms | Replicates standard late-80s studio rack reflections. |

*(Scene C/G = Bypasses Delay/Reverb/Chorus for Dry Comping. Scene D/H = Max Mix on Delay/Reverb for Ambient Volume Swells).*

---

### 4. Troubleshooting & Refinement Tree
If the tone translates poorly through your QSC CP12:
1.  **Too Fuzzy/Spatty on the Overdrive:** The Esprit is hitting the block too hard. Lower Input Block Gain to **-6.0dB**. 
2.  **Lacking Low-End "Thump":** Do *not* turn up the Bass knob on the Gladstone. Dumble circuits get "farty" and sag too much when bass is pushed pre-gain. Instead, adjust the **Parametric-8 Block**, adding a +2.0dB Low Shelf at 120Hz.
3.  **Too Piercing/Harsh:** In the Cab Block, lower the LPF from 6000Hz to **5200Hz**. This physical frequency cut is necessary when standing close to an FRFR monitor.

---

### Session Library (Active Presets)

**2. Preset Name: "Talk To Your Daughter - Esprit"**
*   **Target:** Robben Ford (1988).
*   **Guitar:** Fender Esprit (Medium-High Output Humbuckers).
*   **Physics Goal:** Cascaded gain manipulation allowing single-amp morphing from Clean to Lead via Scene Controllers, optimized for active FRFR wedge.
*   **Full Configuration:**
    *   **Block 1 (Global Input):** Noise Gate Thresh [-62dB], Input Gain [-3.0dB].
    *   **Block 2 (Parametric-8):** HPF [Off], Band 2 [200Hz, +2.5dB on Sc A-D only], LPF [Off].
    *   **Block 3 (Amp - Gladstone OD):** Clean Vol [6.5], OD Drive [Rhy: 1.5 / Lead: 6.5], OD Level [Rhy: 7.0 / Lead: 5.5], Bass [4.0], Mid [7.5], Treble [6.0], Presence [5.0], Master [5.0].
    *   **Block 4 (Cab - 212 Gladstone):** Mic 1 (Dyn 57, Pos 1.5, Dist 1.0"), Mic 2 (Ribbon 121, Pos 3.0, Dist 2.0"), Mix [50/50], Low Cut [90Hz], High Cut [6000Hz].
    *   **Block 5 (Chorus):** Mix [15% / 20%], Depth [35% / 40%], Rate [1.2Hz].
    *   **Block 6 (Vintage Digital Delay):** Mix [12% / 22%], Time [380ms], Feedback [20% / 25%], LP Cut [3000Hz].
    *   **Block 7 (Hall Reverb):** Mix [15% / 18%], Decay [1.8s / 2.2s], High Cut [4500Hz].