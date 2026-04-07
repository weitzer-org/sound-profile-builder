### System Initialization: Quad Cortex Engineer "QC-2"
**Target Engaged:** John Mayer Trio Lead (Dumble SSS / Two-Rock Platform).
**Audio Physics Goal:** High headroom, mid-scooped clean with a thick lower-mid floor. Smooth, non-clipping drive push via Tube Screamer analog topology. 
**Hardware Consideration:** The QSC CP12 is an active PA speaker with a relatively flat response, but it can push harsh treble at high SPL. We must utilize a Low Pass Filter (LPF) to emulate the natural high-frequency roll-off of Mayer's G12-65/EVM12L guitar speakers to prevent "ice-pick" highs.

**Verified Amp Model:** `Grammatico GSG100` (Neural DSP's official circuit-level capture of the Grammatico GSG, a legendary Dumble Steel String Singer clone).

---

### Phase 1: Input Leveling & Pickup Compensation
Before setting up the blocks, we must configure the **Global Input Block** to prevent the ES-339 humbuckers from clipping the highly-sensitive GSG preamp phase inverter. 

*   **Fender Telecaster (Vintage/Medium Single Coil):** Set Input 1 Gain to **+1.5dB**. 
*   **Gibson ES-339 (Medium/High Humbucker):** Set Input 1 Gain to **-3.5dB**. *(This is critical: John's Dumble tone relies on the amp doing the lifting, not hot pickups slamming the front end).*

---

### Phase 2: Split-Bank Scene Architecture
We will utilize the standard QC-2 Split-Bank Matrix. Ensure you right-click the parameters below in Cortex Control to Assign them to Scenes.

*   **Row 1 (Single Coils - Telecaster):** Scenes A (Rhythm), B (Lead), C (Dry), D (Ambient).
*   **Row 2 (Humbuckers - ES-339):** Scenes E (Rhythm), F (Lead), G (Dry), H (Ambient).

#### Table A: Main Signal Chain
*(Note: Right-Click > Assign any parameter marked with differing Scene values)*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Gate (Grid)** | `Adaptive Gate` | Noise Red: 35%<br>Thresh: -60dB | Noise Red: 15%<br>Thresh: -65dB | Higher reduction on Rhythm for dead silence; lower on Lead to preserve sustain/decay. |
| **Pre-FX (Drive)**| `Green 808` | Bypass: ON | Bypass: OFF<br>Gain: 2.5<br>Tone: 5.5<br>Level: 8.0 | Simulates TS10 pushing the GSG100. High level/low gain hits the amp tubes harder without compressing the signal. |
| **Pre-FX (EQ)** | `Parametric-8` | *(See Guitar Configs)* | *(See Guitar Configs)* | Used to physically morph the humbuckers to single-coils and vice-versa. |
| **Amp** | `Grammatico GSG100`| Vol: 4.5<br>Treb: 7.0<br>Mid: 3.5<br>Bass: 5.5<br>Master: 7.5 | Vol: 4.5 *(Assigned)*<br>Treb: 6.5 *(Assigned)*<br>Mid: 4.5 *(Assigned)* | Mid-scooped rhythm. On Lead scenes, Treble drops slightly and Mids raise to cut through the mix without piercing. |
| **Cab** | `212 Grammatico GSG`| Mic A: Dyn 57 (Pos 1.2)<br>Mic B: Ribbon 121 (Pos 2.0) | Mic Mix: A: 0dB<br>Mic B: -2dB | 57 captures the pick attack; 121 placed further out captures the "sag" and low-end depth. |
| **Post-FX (Delay)**| `Analog Delay` | Bypass: ON | Bypass: OFF<br>Mix: 18%<br>Time: 320ms | Aqua Puss style slap/trail. Fills the empty space behind a trio guitar solo. |
| **Post-FX (Verb)**| `Plate Reverb` | Mix: 15%<br>Decay: 1.2s | Mix: 18%<br>Decay: 1.5s | Mayer utilizes outboard studio plates and springs. HPF set to 150Hz to keep low-mids un-muddied. |

---

### Phase 3: Multi-Guitar EQ Targeting (`Parametric-8` Block)
To make this preset gig-ready for both guitars seamlessly, insert the `Parametric-8` EQ block immediately after the `Adaptive Gate` and program these Scene-specific assignments.

#### Configuration 1: Fender Telecaster Single Coil (Scenes A-D)
*Single coils naturally lack the massive low-end of Mayer's Stratocaster "Big Dipper" pickups. We must synthesize that body.*
*   **Band 1 (Low Shelf):** Frequency 150Hz | Gain +2.5dB (Adds weight/chest thump through the QSC CP12).
*   **Band 3 (Bell):** Frequency 400Hz | Gain -1.5dB (Hollows out the boxy frequency to achieve the scooped "Trio" tone).
*   **Band 8 (High Pass/LPF):** Frequency 4500Hz. (Crucial: Cuts the Telecaster's "ice-pick" bridge pickup frequencies so it sounds smooth like a neck position under drive).

#### Configuration 2: Gibson ES-339 Humbuckers (Scenes E-H)
*Humbuckers will naturally sound congested and muddy through a Dumble circuit due to overlapping 250Hz-500Hz frequencies. We must synthesize single-coil clarity.*
*   **Band 1 (Low Shelf):** Frequency 120Hz | Gain -3.0dB (Removes humbucker mud and prevents the GSG amp from sagging too hard).
*   **Band 4 (Bell):** Frequency 800Hz | Gain +2.0dB (Adds the "quack" and pick attack usually lost in humbuckers).
*   **Band 8 (High Pass/LPF):** Frequency 6000Hz. (Allows more treble through than the Telecaster to maintain string definition).

---

### Phase 4: Troubleshooting & Refinement Tree
*   **If the Telecaster Lead is "Too Biting":** Lower the `Tone` knob on the `Green 808` from 5.5 to 4.0. 
*   **If the ES-339 sounds "Farty" on the Low E String:** The amp block is receiving too much bass. Lower the `Grammatico GSG100` Bass knob to 4.0.
*   **If overall SPL is too quiet on the QSC CP12:** **DO NOT** turn up the `Vol` or `Master` on the Amp block, as this alters the Dumble clipping physics. Instead, swipe down on the QC screen and increase the **Lane Output Level** by +3.0dB to +5.0dB.

---

### Phase 5: Preset Registry Protocol (Session Memory)

```text
Session Library (Active Presets)

1. Preset Name: "Spoonful - ES339"
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration: [Stored in previous output]

2. Preset Name: "Mayer Trio - Dual Rig"
Target: John Mayer Trio (Dumble SSS Tone).
Guitar: Fender Telecaster & Gibson ES-339.
Physics Goal: Extremely high headroom, mid-scooped clean with non-compressing analog drive push.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 35% / Lead: 15%], Thresh [-60dB / -65dB], Decay [150ms].
Block 2 (EQ-8 - Telecaster Sc A-D): Low Shelf [150Hz, +2.5dB], Band 3 [400Hz, -1.5dB], LPF [4500Hz].
Block 3 (EQ-8 - ES339 Sc E-H): Low Shelf [120Hz, -3.0dB], Band 4 [800Hz, +2.0dB], LPF [6000Hz].
Block 4 (Green 808 - Lead Only): Gain [2.5], Tone [5.5], Level [8.0].
Block 5 (Amp - Grammatico GSG100): Vol [4.5], Treb [Rhy: 7.0 / Lead: 6.5], Mid [Rhy: 3.5 / Lead: 4.5], Bass [5.5], Master [7.5], Presence [5.0].
Block 6 (Cab - 212 Grammatico GSG): Mic A (Dyn 57, Pos 1.2, Dist 1.0"), Mic B (Ribbon 121, Pos 2.0, Dist 3.0"), Mix [A: 0dB, B: -2dB].
Block 7 (Analog Delay - Lead Only): Mix [18%], Time [320ms], Fdbk [20%], HP [150Hz].
Block 8 (Plate Reverb): Mix [Rhy: 15% / Lead: 18%], Decay [Rhy: 1.2s / Lead: 1.5s], HP [150Hz], LP [4000Hz].
```