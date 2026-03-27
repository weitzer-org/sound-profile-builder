**QUAD CORTEX SYSTEMS ENGINEER (QC-2)**
**INITIALIZATION...**
**TARGET IDENTIFIED:** Stevie Ray Vaughan — "Texas Flood" Tone. 
**ANALOG RESEARCH:** 
*   **Original Hardware:** Fender Super Reverb (4x10) / Vibroverb (1x15) driven by an Ibanez TS808 Tube Screamer. No master volume. 
*   **Physics:** SRV used massive string gauges (0.13s) tuned to Eb, delivering unusually high mechanical output and bass response to low-output vintage single-coils (~5.8k ohms).
*   **QC Mapping:** We will utilize the **US TWN 65 Vibrato** for high-headroom blackface cleans, pushed into compression by the **Green 808**. The **410 US Basslad PR10** cab perfectly replicates the punchy, rapid transient response of a Super Reverb.

To satisfy the **Multi-Guitar Target Output** protocol, this preset is engineered with a split-bank routing matrix. We will use a pre-drive **Parametric-8 EQ** block to digitally simulate switching between vintage low-output single coils and higher-output humbuckers without needing to completely rebuild the amplifier block.

---

### 4. ORGANIZATION STANDARD (SPLIT-BANK MATRIX)
Assign these parameters to your QC Scenes (A-H) to seamlessly switch between your single coils and humbuckers while maintaining the identical core amp tone.

*   **Row 1 (Single Coil Strat/Tele Profile)**
    *   **Scene A (Rhythm):** Edge of breakup. Tube Screamer ON, but set clean.
    *   **Scene B (Lead):** TS808 Level cranked, Amp pushed into saturation. +1.5dB output.
    *   **Scene C (Dry):** Rhythm with Spring Reverb bypassed.
    *   **Scene D (Vibe):** Adds NuVibe (Chorus/Vibrato) for "Cold Shot" textures.
*   **Row 2 (Gibson ES-339 Humbucker Profile)**
    *   **Scene E (Rhythm):** Humbucker-padded clean rhythm.
    *   **Scene F (Lead):** Humbucker-padded lead tone. +1.5dB output.
    *   **Scene G (Dry):** Same as E, Reverb off.
    *   **Scene H (Vibe):** Same as F, NuVibe engaged.

---

### 9. MULTI-GUITAR GAIN STAGING & PICKUP COMPENSATION
Before hitting the Amp block, place a **Parametric-8 EQ** in Grid Block 1. Assign its parameters to Scenes A-D vs. Scenes E-H.

**Guitar 1: Fender Strat/Tele Single Coil (Scenes A, B, C, D)**
*   **EQ Block Output Level:** +2.5dB (Simulates the output push of SRV's heavy 0.13 strings).
*   **Band 1 (Low Shelf):** 180Hz at +2.5dB (Adds the physical "thump" of a heavy low-E string).
*   **Band 8 (High Pass):** OFF (Retain maximum top-end glass).

**Guitar 2: Gibson ES-339 Humbuckers (Scenes E, F, G, H)**
*   **EQ Block Output Level:** -5.0dB (CRITICAL: Pads the high-output humbuckers so they don't instantly fuzz out the Blackface preamp).
*   **Band 1 (Low Shelf):** 200Hz at -3.0dB (Removes humbucker low-mid mud).
*   **Band 6 (Peak):** 3500Hz at +3.0dB (Injects single-coil-style "bite" and pick attack).
*   **Band 8 (Low Pass):** 6000Hz (Tames any harsh humbucker fizz when hitting the Tube Screamer).

---

### 5. EXECUTION PROTOCOL: MAIN SIGNAL CHAIN (Table A)
*Note: Parameters marked with *(Right-Click > Assign)* must be assigned to Scenes.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global In 1 | Thresh: -65dB | Thresh: -65dB | SRV rigs are notoriously noisy. Keep threshold low to allow natural decay without chopping the trailing spring reverb. |
| **Pre-FX** | Green 808 | Drive: 1.0 <br>Tone: 6.5 <br>Level: 7.5 *(Assign)* | Drive: 3.5 <br>Tone: 7.0 <br>Level: 10.0 *(Assign)* | **The "Clean Edge":** Maxing the TS level while keeping drive low physically slams the amp's V1 preamp tube, creating dynamic, uncompressed overdrive. |
| **Amp** | US TWN 65 Vibrato | Vol: 5.5 *(Assign)* <br>Bass: 3.5 <br>Mid: 6.0 <br>Treble: 6.5 | Vol: 7.5 *(Assign)* <br>Bass: 3.0 <br>Mid: 6.5 <br>Treble: 6.5 | **Tube Taper Logic:** Fender circuits have NO Master Volume. Adjust Vol to push tubes; use Lane Output Level for actual room loudness. Bass is reduced on Lead to prevent "farting out". |
| **Cab** | 410 US Basslad PR10 | Mic A: Dyn 57 (Pos 0.5) <br>Mic B: Rib 121 (Pos 0.8) | Mic A: Dyn 57 (Pos 0.5) <br>Mic B: Rib 121 (Pos 0.8) | **Speaker Physics:** 4x10s have a tight, fast transient response. The SM57 captures the bite; the Ribbon 121 pushed slightly back captures the air/body. |
| **Post-FX** | Spring | Mix: 15% *(Assign)* <br>Decay: 2.0s | Mix: 22% *(Assign)* <br>Decay: 2.5s | **Spatial Goal:** Authentic vintage Fender outboard spring splash. Increased mix on leads for sustained vocal quality. |

---

### 6. TROUBLESHOOTING & REFINEMENT TREE
If playing through your **QSC CP12 PA Speaker** and the tone feels inaccurate:
1.  **"It sounds too distorted/fuzzy with my Strat"**: Lower the Output Level on your Grid 1 EQ block by another -2.0dB. Your specific single coils may be wound hotter than vintage spec. 
2.  **"The low-end is farting out/flubby on the ES-339"**: Reduce the Bass parameter on the **US TWN 65 Vibrato** block down to 2.5 or 2.0. Vintage Fender circuits famously cannot handle heavy bass frequencies when the volume knob passes 5. 
3.  **"It's not loud enough"**: DO NOT touch the Amp Volume or Green 808 Drive. Increase the overall **Lane Output Level** on the right side of the Cortex Control app.

---

### 8. PRESET REGISTRY PROTOCOL (SESSION MEMORY)

**Session Library (Active Presets)**
**2. Preset Name:** "Texas Flood - Dual Rig"
**Target:** Stevie Ray Vaughan (1983).
**Guitars:** Fender Single Coil (Scenes A-D) / Gibson ES-339 (Scenes E-H).
**Physics Goal:** Massive headroom clean-edge driven entirely by preamp tube slamming via a clean-boosted Tube Screamer. 
**Full Configuration:**
*   **Block 1 (EQ-8):** Out Level [A-D: +2.5dB / E-H: -5.0dB]. Band 1 Low Shelf [A-D: 180Hz +2.5dB / E-H: 200Hz -3.0dB]. Band 6 Peak [E-H: 3500Hz +3.0dB]. LPF [E-H: 6000Hz]. 
*   **Block 2 (Green 808):** Drive [Rhy: 1.0 / Lead: 3.5], Tone [Rhy: 6.5 / Lead: 7.0], Level [Rhy: 7.5 / Lead: 10.0].
*   **Block 3 (NuVibe - Optional on Scene D/H):** Mix [45%], Intensity [60%], Speed [4.5Hz], Chorus Mode.
*   **Block 4 (Amp - US TWN 65 Vibrato):** Vol [Rhy: 5.5 / Lead: 7.5], Bright [OFF], Treble [6.5], Mid [Rhy: 6.0 / Lead: 6.5], Bass [Rhy: 3.5 / Lead: 3.0]. Output Level [+2.0dB]. *Note: No Master Volume parameter exists for this hardware.*
*   **Block 5 (Cab - 410 US Basslad PR10):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 3.0"), Mix [A: 0dB, B: -3dB].
*   **Block 6 (Spring Reverb):** Mix [Rhy: 15% / Lead: 22%], Decay [Rhy: 2.0s / Lead: 2.5s], Low-Cut [150Hz], High-Cut [4000Hz].