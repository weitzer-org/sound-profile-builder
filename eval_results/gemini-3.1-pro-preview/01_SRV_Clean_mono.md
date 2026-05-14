As QC-2, I have configured your requested Stevie Ray Vaughan (SRV) style high-headroom clean funk blues tone. 

To achieve this accurately on the Quad Cortex through your QSC CP12, we must employ the **Headroom Rule**: SRV achieved his aggressive bite not through preamp distortion, but by slamming the front end of a massive, clean power section with a Tube Screamer. We will use the **US Twin 65 Vibrato** model (which lacks a Master Volume; overall SPL will be controlled via Lane Output Level) and the **Green 808** overdrive.

Below are the strictly separated configurations for your **Fender Telecaster** (Row 1) and **Gibson ES-339** (Row 2). 

---

### Part 1: Fender Telecaster Single Coil Profile (Scenes A–D)
**Goal:** Add "Strat-like" neck/middle position body (200Hz) to the Telecaster, tame the extreme ice-pick highs of the bridge pickup, and provide a clean +3.0dB boost to hit the amp's sweet spot.

**Global Input Stage (Telecaster):** Input 1 Gain set to **+2.0dB**.

**Table A: Main Signal Chain (Telecaster Single Coil)**
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -55dB | Thresh: -60dB | Single coils hum; slightly tighter gate for staccato funk comping. |
| **Pre-FX 1** | Green 808 | Bypass | Active (Drive: 2.0, Tone: 5.0, Level: 9.0) | High Level/Low Drive physically slams the amp's virtual V1 tube, creating SRV compression. |
| **Pre-FX 2** | Parametric-8 | Active | Active | *Chameleon Strategy:* Band 2 (Peak) +2.5dB at 250Hz for body. Band 8 (LPF) at 5.5kHz to tame Tele pick attack. |
| **Amp** | US Twin 65 Vibrato | Vol: 4.5, Treb: 6.0, Mid: 6.5, Bass: 4.0, Bright: OFF | *(Right-Click > Assign)* Vol: 5.0, Treb: 6.0, Mid: 7.0, Bass: 4.0 | No Master Volume. Pushing mids to simulate SRV's Strat neck-pickup bark. |
| **Cab** | 212 US C12N | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") | SM57 near center cap provides the aggressive, glassy bite needed to cut through a mix. |
| **Cab Mix** | 212 US C12N | Mic B: Ribbon 121 (Pos 1.2, Dist 4.0") Mix: A: 0dB, B: -3dB | Mic B: Ribbon 121 (Pos 1.2, Dist 4.0") Mix: A: 0dB, B: -3dB | Ribbon mic adds the low-end thump of a 12" speaker pushing air. |
| **Post-FX 1** | Spring Reverb | Mix: 15%, Decay: 1.5s | Mix: 18%, Decay: 1.5s | Essential Fender-style spring "drip" for vintage blues authenticity. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | Overall SPL control. Boosted for lead without changing amp compression. |

*Scene Functions (Row 1):*
*   **Scene A (Rhythm):** High-headroom clean, EQ'd for Strat-like body.
*   **Scene B (Lead):** Green 808 engaged, Amp Mid boosted, Output +1.5dB.
*   **Scene C (Dry/Comping):** Spring Reverb bypassed for ultra-tight funk.
*   **Scene D (Ambient/FX):** Spring Reverb Mix increased to 35% for slow blues.

---

### Part 2: Gibson ES-339 Humbucker Profile (Scenes E–H)
**Goal:** Prevent the high-output humbuckers from prematurely distorting the Twin Reverb. Scoop the low-mids to remove "mud" and activate the Bright switch to simulate single-coil glassiness.

**Global Input Stage (ES-339):** Input 1 Gain set to **-4.0dB**. *(Crucial to simulate rolling off the guitar volume and preventing digital clipping).*

**Table B: Main Signal Chain (ES-339 Humbuckers)**
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -65dB | Thresh: -65dB | Humbuckers are quieter regarding 60-cycle hum; lower threshold preserves sustain and dynamics. |
| **Pre-FX 1** | Green 808 | Bypass | Active (Drive: 1.5, Tone: 7.0, Level: 8.0) | Drive reduced further, Tone increased to cut through the naturally dark humbucker EQ. |
| **Pre-FX 2** | Parametric-8 | Active | Active | *Chameleon Strategy:* Band 3 (Peak) -3.0dB at 350Hz (removes humbucker mud). Band 8 (LPF) bypassed. |
| **Amp** | US Twin 65 Vibrato | Vol: 4.0, Treb: 6.5, Mid: 5.0, Bass: 3.0, Bright: ON | *(Right-Click > Assign)* Vol: 4.5, Treb: 6.5, Mid: 5.5, Bass: 3.0 | Bright switch ON is mandatory here. Bass lowered to prevent tube sag/farting from hot pickups. |
| **Cab** | 212 US C12N | Mic A: Dyn 57 (Pos 0.8, Dist 1.0") | Mic A: Dyn 57 (Pos 0.8, Dist 1.0") | Mic pushed slightly further from center cap to avoid humbucker harshness on the top end. |
| **Cab Mix** | 212 US C12N | Mic B: Ribbon 121 (Pos 1.5, Dist 4.0") Mix: A: 0dB, B: -6dB | Mic B: Ribbon 121 (Pos 1.5, Dist 4.0") Mix: A: 0dB, B: -6dB | Ribbon mic mixed lower (-6dB) because the ES-339 already provides massive low-end body. |
| **Post-FX 1** | Spring Reverb | Mix: 12%, Decay: 1.5s | Mix: 15%, Decay: 1.5s | Slightly lower mix than Tele to retain clarity and note definition with humbuckers. |
| **Output** | Lane 1 Output | Level: +2.0dB | Level: +3.5dB | Compensating for the -4.0dB cut at the Input Stage to match SPL across guitars. |

*Scene Functions (Row 2):*
*   **Scene E (Rhythm):** Glassy, scooped clean.
*   **Scene F (Lead):** Green 808 engaged, biting, output boosted.
*   **Scene G (Dry/Comping):** Spring Reverb bypassed.
*   **Scene H (Ambient/FX):** Spring Reverb Mix increased to 30%.

---

### Troubleshooting & Refinement Tree
If you test this through your QSC CP12 and report that the ES-339 tone is "Too Distorted/Fuzzy" when you want a pure clean:
1.  **Input Pad:** Verify your Input Block Gain is strictly at **-4.0dB** or even **-6.0dB**. Hot humbuckers will completely destroy the headroom of this amp model if fed at 0.0dB.
2.  **Amp Gain:** Reduce the US Twin 65 Volume knob to 3.0. 
3.  **Tube Sag:** Ensure Bass is at or below 3.0. Tweed/Blackface circuits become "farty" when the bass frequencies overload the virtual power tubes.
4.  **Output Compensation:** Make up the volume loss by raising the Lane Output Level. Do not touch the Amp Volume to increase loudness.

---

### Session Library (Active Presets)
**2. Preset Name: "Texas Flood Twin - Multi"**
*   **Target:** Stevie Ray Vaughan / High Headroom Blues Funk.
*   **Guitar:** Fender Telecaster (Scenes A-D) / Gibson ES-339 (Scenes E-H).
*   **Physics Goal:** Maximize clean headroom, prevent humbucker clipping, utilize TS808 as a clean level boost to compress the V1 tube.
*   **Full Configuration:**
    *   **Block 1 (EQ-8):** [Tele] B2 (+2.5dB @ 250Hz), B8 LPF (5.5kHz). [ES-339] B3 (-3.0dB @ 350Hz), LPF OFF.
    *   **Block 2 (Green 808):** Drive [Tele: 2.0 / ES: 1.5], Tone [Tele: 5.0 / ES: 7.0], Level [Tele: 9.0 / ES: 8.0].
    *   **Block 3 (US Twin 65 Vibrato):** Vol [Tele: 4.5/5.0 / ES: 4.0/4.5], Treble [Tele: 6.0 / ES: 6.5], Mid [Tele: 6.5/7.0 / ES: 5.0/5.5], Bass [Tele: 4.0 / ES: 3.0], Bright Switch [Tele: OFF / ES: ON].
    *   **Block 4 (212 US C12N):** Mic A (Dyn 57), Mic B (Ribbon 121). Mix [Tele: A 0dB, B -3dB / ES: A 0dB, B -6dB].
    *   **Block 5 (Spring Reverb):** Mix [Tele: 15% / ES: 12%], Decay [1.5s].
    *   **Output Strategy:** Master Loudness managed via Lane Output Level (+1.5dB boost on lead scenes). Compensatory -4.0dB input pad mapped for ES-339 to prevent clipping.