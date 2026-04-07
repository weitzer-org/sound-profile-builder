System Initialized: **QC-2 Quad Cortex Systems Engineer**  
Target Acknowledged: Early British Invasion Tone (Vox AC30 / Marshall JTM45 hybrid).  
Target SPL Environment: QSC CP12 (Active PA/FRFR).  

To achieve the "chime" of the AC30 alongside the "punchy mids" of a JTM45, we will deploy a **Dual-Amp / Parallel Routing** strategy. Vintage circuits do not feature Master Volumes; therefore, we will push the preamp volumes to the "edge of breakup" and strictly use the Lane Output Level to control stage volume.

Because you are alternating between a **Fender Telecaster (Single Coil)** and a **Gibson ES-339 (Humbucker)**, the physics of your guitar outputs dictate that we must utilize the **Split-Bank Matrix** and **Chameleon EQ Strategy** to prevent the ES-339 from muddying the AC30, and to keep the Telecaster from sounding brittle through the JTM45.

---

### Phase 1: Input & Pickup Compensation Strategy
**Crucial:** Do not skip this step. Vintage amp blocks react violently to modern or humbucker output levels, resulting in unmusical digital clipping.

*   **Fender Telecaster (Row 1 - Scenes A-D):** Vintage/Medium Output. The global Input Block Gain remains at **0.0dB**. 
*   **Gibson ES-339 (Row 2 - Scenes E-H):** Higher Output. We must simulate rolling off the guitar volume to hit the same edge-of-breakup sweet spot. You must set the Input Block Gain to **-3.5dB** (Right-Click > Assign to Scenes E-H).

---

### Phase 2: Main Signal Chain (Table A)
*Note: Parameters marked with `(Sc)` require `Right-Click > Assign to Scene`.*

| Block Category | Model Name | Rhythm Settings (Clean/Edge) | Lead Settings (Pushed Mids) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | Thresh: -55dB | Thresh: -60dB | Vintage single coils need a slightly higher threshold than shielded humbuckers. |
| **Pre-FX (EQ)** | Parametric-8 | *Tele:* B1 LowShelf +3dB (200Hz)<br>*339:* HPF 100Hz | *Tele:* LPF 4.5kHz<br>*339:* B6 Peak +2dB (2.5kHz) | **Chameleon Strategy:** Adds physical body to Tele. Cuts muddy neck-humbucker frequencies on ES-339. |
| **Pre-FX (Drive)** | Range Booster | Bypass | Active (Boost: 6.5, Treble: 5.0) | Replicates Dallas Rangemaster treble booster used by Clapton/May to push UK amps. |
| **Amp 1 (Top)** | UK C30 Top Boost | Vol: 4.5 (Sc), Bass: 4.0, Treb: 6.0 | Vol: 5.5 (Sc) | No Master Volume. Provides upper-harmonic chime and glass. |
| **Amp 2 (Bottom)**| Brit 45 Jumped | Vol Nrm: 5.0 (Sc), Vol Brt: 4.5, Mid: 7.0 | Vol Nrm: 6.5 (Sc) | No Master Volume. JTM45 provides the physical low-mid punch that the AC30 lacks. |
| **Cab 1 (Top)** | 212 UK C30 | Mic: Ribbon 121<br>Pos: 0.6, Dist: 2.0" | Mix: 0.0dB | Silver Bell speakers. Ribbon mic tames the AC30 "ice-pick" harshness through your QSC CP12. |
| **Cab 2 (Bottom)**| 412 Brit 35B | Mic: Dyn 57<br>Pos: 0.2, Dist: 1.0" | Mix: -2.0dB | Greenbacks. Close-miked dynamic captures the aggressive midrange punch of the JTM. |
| **Post-FX (Dly)** | Slapback Delay | Mix: 12%, Time: 90ms | Mix: 18%, Time: 110ms | Fills out the sonic footprint without washing out the staccato invasion riffs. |
| **Post-FX (Rev)** | Spring Reverb | Mix: 15%, Decay: 1.2s | Mix: 20% | Authentically recreates 60s mechanical tank reflections. |
| **Output** | Lane 1/2 Output | Level: 0.0dB | Level: +1.5dB (Sc) | **Headroom Rule:** Lifts total SPL for solos without altering amp gain saturation. |

---

### Phase 3: Split-Bank Matrix Assignment
Use Cortex Control to assign the following functions across your footswitches.

#### Row 1: Fender Telecaster Profile
*   **Scene A (Rhythm):** Edge of breakup. Both amps engaged. Range Booster OFF. Parametric EQ engaged (adds 200Hz body).
*   **Scene B (Lead):** Range Booster ON. Amp volumes pushed. Output Level +1.5dB. EQ LPF lowered to 4.2kHz to tame the bridge pickup pick-attack.
*   **Scene C (Dry/Comping):** Scene A settings, but Spring Reverb and Delay bypassed.
*   **Scene D (Ambient/FX):** Scene A settings, Reverb Mix increased to 35%, Delay Feedback increased to 40%.

#### Row 2: Gibson ES-339 Profile
*   **Scene E (Rhythm):** Input Block padded to -3.5dB. Parametric EQ shifts to HPF at 100Hz to remove humbucker mud.
*   **Scene F (Lead):** Range Booster ON. Input Pad remains -3.5dB. Output Level +1.5dB. 
*   **Scene G (Dry/Comping):** Scene E settings, Reverb/Delay bypassed.
*   **Scene H (Ambient/FX):** Scene E settings, spatial effects maximized.

---

### Phase 4: Troubleshooting & Refinement Tree
If the tone sounds "Too Distorted" or exhibits digital artifacts through your QSC CP12:
1.  **Input Pad:** Verify the Input Block Gain is correctly padded (-3.5dB or even -5.0dB) for the ES-339.
2.  **Tube Sag/Bass:** The `Brit 45 Jumped` block features intense low-end sag. If the JTM45 side sounds "farty" or broken on your ES-339 neck pickup, reduce the Amp Block Bass parameter by 2.0 increments. 
3.  **Speaker High-Frequency Resonance:** FRFR speakers like the CP12 reproduce frequencies up to 20kHz, unlike real guitar cabs which drop off at 5kHz. If the AC30 chime is piercing, push the Cab 1 Mic distance from `2.0"` to `3.5"`.

---

### Session Registry Protocol (Active Presets)
*Appending current build to Session Library...*

**Session Library (Active Presets)**
1.  *Preset Name:* "Spoonful - ES339" (Retained from prior session memory).
2.  *Preset Name:* "Invasion Dual - Tele/339"
    *   **Target:** Beatles / Kinks / Yardbirds (1964-1966).
    *   **Guitars:** Gibson ES-339 (Humbucker) & Fender Telecaster (Single Coil).
    *   **Physics Goal:** Dual amp blending AC30 glass with JTM45 punch; edge-of-breakup via preamp volume; strictly controlled via Input padded gain staging.
    *   **Full Configuration:**
        *   Block 1 (Global Gate): Thresh [-55dB / -60dB]. Input Gain [Tele: 0.0dB / ES-339: -3.5dB].
        *   Block 2 (EQ-8): Tele [Band 1 Peak +3dB @ 200Hz, LPF @ 4.5kHz]. ES-339 [HPF @ 100Hz, Band 6 Peak +2dB @ 2.5kHz].
        *   Block 3 (Range Booster): Bypass [Scenes A, C, D, E, G, H]. Active [Scenes B, F]. Boost [6.5].
        *   Block 4 (Amp Top - UK C30 Top Boost): Vol [4.5 / 5.5], Bass [4.0], Treb [6.0], Cut [3.0].
        *   Block 5 (Amp Bot - Brit 45 Jumped): Vol Nrm [5.0 / 6.5], Vol Brt [4.5], Bass [3.0], Mid [7.0], Treb [5.5], Pres [4.0].
        *   Block 6 (Cab Top - 212 UK C30): Mic (Ribbon 121, Pos 0.6, Dist 2.0"). Mix [0dB].
        *   Block 7 (Cab Bot - 412 Brit 35B): Mic (Dyn 57, Pos 0.2, Dist 1.0"). Mix [-2.0dB].
        *   Block 8 (Slapback Delay): Mix [12% / 18%], Time [90ms / 110ms].
        *   Block 9 (Spring Reverb): Mix [15% / 20%], Decay [1.2s].
        *   Output Lane Level: [Rhythm: 0.0dB / Lead: +1.5dB].