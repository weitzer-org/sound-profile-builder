**QC-2 Systems Engineer: Initialization Complete.**

To capture the iconic Early British Invasion tone (The Beatles, The Kinks, early Rolling Stones), we must emulate the physics of a Vox AC30 Top Boost pushed to the edge of breakup, captured in a large studio room with tape slapback. 

**Pre-Flight Verification:** *Are your pickups Vintage Output, Medium, or High Output?* The ES-339 humbuckers will inherently drive the virtual tubes into hard clipping much faster than the Telecaster, ruining the "chime." To solve this, we will rely heavily on our **Pickup Output Compensation Strategy** and **Split-Bank Matrix**.

Here are your distinct, physics-accurate configurations.

---

### **PART 1: Fender Telecaster (Single Coil) Configuration**
**Matrix Placement:** Row 1 (Scenes A - D)
*   **Scene A:** Rhythm (Tight, Jangle, -1.5dB Output)
*   **Scene B:** Lead (Pushed Brilliant Volume, +1.5dB Output)
*   **Scene C:** Dry/Comping (No Reverb/Delay)
*   **Scene D:** Ambient/FX (Abbey Road Studio Slapback + Plate)
*   **Physics Goal:** Add body to the bridge pickup while taming the ice-pick attack. Push the amp hard enough to compress naturally.

**Table A: Main Signal Chain (Telecaster)**
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)"*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -65dB / Gain: +1.0dB | Thresh: -65dB / Gain: +1.0dB | Single coils need a slight push to hit the sweet spot of the AC30 phase inverter. |
| **Pre-FX** | Opto Comp | Comp: 45% / Level: +2.0dB | Comp: 35% / Level: +2.0dB | Emulates studio optical compression; evens out jangly chord work. |
| **EQ** | Parametric-8 | Band 1: +2.5dB @ 200Hz (Peak) | Band 1: +2.5dB @ 200Hz (Peak) | **Chameleon Rule:** Adds physical weight and body to the Telecaster single coil. |
| **Amp** | UK C30 Top Boost | Brilliant: 4.5 / Bass: 4.0 | Brilliant: 6.0 / Bass: 3.5 | *Non-Master Vol Amp.* Tele needs more Brilliant Vol to reach edge-of-breakup. Bass lowered slightly on Lead to prevent tube sag. |
| **Cab** | 2x12 UK C30 | Mic 1 (Dyn 57): Pos 0.5, Dist 1.0" | Mic 1 (Dyn 57): Pos 0.5, Dist 1.0" | Celestion Alnico Blues. Dyn 57 provides punch; Mic 2 (below) provides the sparkle. |
| **Cab (Mic 2)** | *Dual Cab Block* | Mic 2 (Rib 121): Pos 1.5, Dist 2.5" | Mic 2 (Rib 121): Pos 1.5, Dist 2.5" | Ribbon mic placed further back to capture the physical "chime" without harshness. Mix A/B equally. |
| **Post-FX** | Tape Delay | Mix: 15% / Time: 90ms / Fdbk: 0% | Mix: 22% / Time: 110ms / Fdbk: 15% | Emulates classic EMI Studios STEED tape slapback. |
| **Post-FX** | Plate Reverb | Mix: 18% / Decay: 1.2s | Mix: 25% / Decay: 1.5s | Replicates vintage studio EMT-140 plate reverb. LPF set to 4000Hz. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB | The Headroom Rule: Increase SPL here, not via Amp controls, to preserve tone. |

---

### **PART 2: Gibson ES-339 (Humbuckers) Configuration**
**Matrix Placement:** Row 2 (Scenes E - H)
*   **Scene E:** Rhythm (Cleaned up, -1.5dB Output)
*   **Scene F:** Lead (Slightly pushed, +1.5dB Output)
*   **Scene G:** Dry/Comping (No Reverb/Delay)
*   **Scene H:** Ambient/FX (Abbey Road Studio Slapback + Plate)
*   **Physics Goal:** Prevent the hotter humbuckers from turning the AC30 into a fuzzy, saturated mess. Radically cut input gain and low-mids to preserve the crystalline chime.

**Table B: Main Signal Chain (ES-339)**
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)"*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -55dB / Gain: -4.5dB | Thresh: -55dB / Gain: -4.5dB | **Crucial Headroom Rule:** -4.5dB padding prevents the humbuckers from digitally clipping or instantly fuzzing the amp. |
| **Pre-FX** | Opto Comp | Comp: 25% / Level: 0.0dB | Comp: 20% / Level: 0.0dB | Less compression needed; humbuckers naturally compress the amp's front end. |
| **EQ** | Parametric-8 | HPF: 100Hz / Band 2: -3.0dB @ 250Hz | HPF: 100Hz / Band 2: -3.0dB @ 250Hz | **Chameleon Rule:** Clears out humbucker "mud" that conflicts with the AC30's low-end profile. |
| **Amp** | UK C30 Top Boost | Brilliant: 2.8 / Bass: 2.5 | Brilliant: 4.0 / Bass: 2.0 | *Non-Master Vol Amp.* Lower Brilliant volume required to maintain "chime" with hot pickups. |
| **Cab** | 2x12 UK C30 | Mic 1 (Dyn 57): Pos 0.8, Dist 1.5" | Mic 1 (Dyn 57): Pos 0.8, Dist 1.5" | Mic moved slightly further from cap edge to soften the aggressive humbucker midrange. |
| **Cab (Mic 2)** | *Dual Cab Block* | Mic 2 (Rib 121): Pos 2.0, Dist 3.5" | Mic 2 (Rib 121): Pos 2.0, Dist 3.5" | Pulled back further than the Tele config to drastically reduce proximity effect (bass buildup). |
| **Post-FX** | Tape Delay | Mix: 12% / Time: 90ms / Fdbk: 0% | Mix: 18% / Time: 110ms / Fdbk: 15% | Kept slightly lower in mix for humbuckers to maintain chord clarity. |
| **Post-FX** | Plate Reverb | Mix: 15% / Decay: 1.2s | Mix: 20% / Decay: 1.5s | LPF set to 3500Hz to keep the reverb tail out of the humbucker's upper-mid punch. |
| **Output** | Lane Output | Level: +1.5dB | Level: +3.0dB | Compensating for the -4.5dB Input drop so the QSC CP12 receives optimal signal level. |

---

### **Troubleshooting & Refinement Tree**
If playing through the QSC CP12 reveals issues at gig volume:
1.  **If the Tone is "Too Distorted" or "Fuzzy" (especially on the 339):**
    *   *Input Pad:* Lower Global Input Gain to -6.0dB.
    *   *Amp Gain:* Reduce the Brilliant knob by 0.5 increments. Do NOT touch the compressor. 
    *   *Output Comp:* Raise Lane Output Level by +2.0dB to recover lost volume.
2.  **If the Amp sounds "Broken/Farty" on low notes:**
    *   *Tube Sag:* The AC30 model replicates power amp sag. Reduce the Amp block's *Bass* parameter to 1.5 or 2.0.
3.  **If the Telecaster sounds too thin/shrill:**
    *   Reduce the Amp block's *Tone Cut* parameter (Note: Vox Tone Cut works backwards—turning it *up* cuts treble). Set Cut to 5.0 or 6.0.

---

### **Session Library (Active Presets)**

**1. Preset Name:** "Spoonful - ES339" (Previously Saved)

**2. Preset Name:** "British Invasion - Tele/339 Matrix"
*   **Target:** Beatles / Kinks / Rolling Stones (1964-1966).
*   **Guitar:** Row 1 (Telecaster / Single Coils), Row 2 (ES-339 / Humbuckers).
*   **Physics Goal:** Vintage AC30 top-boost chime, maintaining edge-of-breakup clarity across radically different pickup outputs.
*   **Full Configuration (Telecaster - Row 1):**
    *   Input: Thresh [-65dB], Gain [+1.0dB].
    *   Pre-FX (Opto Comp): Comp [Rhy: 45% / Lead: 35%], Level [+2.0dB].
    *   EQ-8: Band 1 Peak [+2.5dB @ 200Hz].
    *   Amp (UK C30 Top Boost): Brilliant [Rhy: 4.5 / Lead: 6.0], Normal [2.5], Bass [Rhy: 4.0 / Lead: 3.5], Treble [6.5], Cut [4.0].
    *   Cab (2x12 UK C30): Mic 1 (Dyn 57, Pos 0.5, Dist 1.0"), Mic 2 (Ribbon 121, Pos 1.5, Dist 2.5").
    *   Post-FX 1 (Tape Delay): Mix [15% / 22%], Time [90ms / 110ms].
    *   Post-FX 2 (Plate Reverb): Mix [18% / 25%], Decay [1.2s / 1.5s], LPF [4000Hz].
    *   Lane Output: [0.0dB / +1.5dB].
*   **Full Configuration (ES-339 - Row 2):**
    *   Input: Thresh [-55dB], Gain [-4.5dB] *(Crucial humbucker pad)*.
    *   Pre-FX (Opto Comp): Comp [Rhy: 25% / Lead: 20%], Level [0.0dB].
    *   EQ-8: HPF [100Hz], Band 2 [-3.0dB @ 250Hz].
    *   Amp (UK C30 Top Boost): Brilliant [Rhy: 2.8 / Lead: 4.0], Normal [1.5], Bass [Rhy: 2.5 / Lead: 2.0], Treble [6.0], Cut [4.5].
    *   Cab (2x12 UK C30): Mic 1 (Dyn 57, Pos 0.8, Dist 1.5"), Mic 2 (Ribbon 121, Pos 2.0, Dist 3.5").
    *   Post-FX 1 (Tape Delay): Mix [12% / 18%], Time [90ms / 110ms].
    *   Post-FX 2 (Plate Reverb): Mix [15% / 20%], Decay [1.2s / 1.5s], LPF [3500Hz].
    *   Lane Output: [+1.5dB / +3.0dB].