**System Engineer QC-2 Initializing...**
**Protocol:** Physics First | Cortex Control Desktop Verification
**Speaker Profile Target:** QSC CP12 (Active FRFR)

To achieve the Stevie Ray Vaughan "Texas Flood" tone, we must replicate the physics of his rig: a vintage Fender Super Reverb pushed to the edge of breakup, slammed into overdrive not by a pedal's clipping diodes, but by the sheer output volume of a Tube Screamer. 

*Target Verification:* SRV used heavy-gauge strings on a vintage Stratocaster. Per System Rule 9, I have formatted this build to explicitly accommodate both **Fender Telecaster Single Coils** (giving it heavy-string Strat thickness) and **Gibson ES-339 Humbuckers** (stripping out the mud to achieve Texas glass).

***

### 1. Multi-Guitar Gain Staging & Pickup Compensation

**Fender Telecaster (Single Coil) - Row 1 (Scenes A-D)**
*   **Input Block Gain:** `0.0dB`
*   **Physics Goal:** Your vintage single coils lack the mass of SRV's heavy strings. We will use the EQ-8 block to add a +3.0dB Low-Shelf at 200Hz to add "Strat body" to the Telecaster, while rolling off the 5kHz "ice pick" so the neck pickup sounds hollow and bell-like.

**Gibson ES-339 (Humbuckers) - Row 2 (Scenes E-H)**
*   **Input Block Gain:** `-4.5dB` (CRITICAL)
*   **Physics Goal:** Vintage PAF-style humbuckers output roughly 30-50% more voltage than vintage single coils. If we don't pad the input, the ES-339 will clip the amp model into muddy hard-rock distortion. The EQ-8 block will cut -4.0dB at 250Hz to remove humbucker mud and boost +2.0dB at 3.5kHz for single-coil "snap."

***

### 2. Main Signal Chain (Table A)
*Note: Parameters marked with (Right-Click > Assign) are Scene-dependent.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -65dB | High threshold to keep the single coils dead quiet without choking sustain. |
| **Pre-FX** | Green 808 | Drive: 0.5<br>Tone: 6.5<br>Level: 8.5 *(Right-Click > Assign)* | Drive: 2.0<br>Tone: 7.0<br>Level: 10.0 *(Right-Click > Assign)* | SRV used the TS808 as a clean boost. High level smashes the amp's V1 preamp tube; low drive avoids pedal fizz. |
| **Amp** | US SR65 (Normal) | Vol: 5.5 *(Right-Click > Assign)*<br>Bass: 3.5<br>Mid: 5.0<br>Treble: 6.5 | Vol: 6.5 *(Right-Click > Assign)*<br>Bass: 3.0<br>Mid: 5.5<br>Treble: 6.5 | Emulates a cranked Fender Super Reverb. Bass must drop as Volume increases to prevent "farty" tube sag on the low E string. |
| **Cab** | 410 US SR65 | Mic A: Dyn 57 (Pos 0.5, Dist 1")<br>Mic B: Rib 121 (Pos 1.0, Dist 3") | Mic A: Mix +0dB<br>Mic B: Mix -3dB | 4x10 Jensens move fast. The 57 captures the pick attack; the 121 catches the low-end resonance. |
| **Post-FX 1** | Parametric-8 EQ | Tele (A/B): Band 1 +3.0dB (200Hz), LPF 5.5kHz<br>ES339 (E/F): Band 2 -4.0dB (250Hz) | *Same as Rhythm* | **The Chameleon Block.** Alters guitar physics. HPF set to 90Hz globally to stop the QSC CP12 from rumbling the stage. |
| **Post-FX 2** | Spring Reverb | Mix: 20%<br>Decay: 1.8s<br>Tone: 6.0 | Mix: 25%<br>Decay: 1.8s<br>Tone: 6.5 | Emulates the analog spring tank. Placed after the cab to prevent muddying the overdrive transients. |

***

### 3. Troubleshooting & Refinement Tree
If the tone reacts poorly through your QSC CP12, follow this strict sequence:

1.  **Too Distorted / Fuzzy (Especially on the ES-339):**
    *   *Action:* Lower Input Block Gain to `-6.0dB`.
    *   *Why:* Your humbuckers are hitting the digital input too hard, causing unmusical clipping before the signal even reaches the TS808 block.
2.  **Too "Farty" or Flubby on the Low E String:**
    *   *Action:* Lower the Amp Block `Bass` parameter from 3.5 to `2.0`.
    *   *Why:* Fender Super Reverb circuits have inherently loose low-end. When the power tubes are pushed by a Tube Screamer, bass frequencies cause the phase inverter to collapse.
3.  **Harsh/Piercing Highs through the FRFR Speaker:**
    *   *Action:* Lower the Parametric-8 EQ `LPF` (Low Pass Filter) to `4.5kHz`.
    *   *Why:* Guitar cabs naturally roll off around 5kHz. Active PA speakers like the CP12 reproduce up to 20kHz, which exposes digital fizz.

***

### Session Library (Active Presets)

**2. Preset Name:** "Texas Flood - Dual Rig"
**Target:** Stevie Ray Vaughan (1983).
**Guitar:** Gibson ES-339 (Humbuckers) / Fender Telecaster (Single Coils).
**Physics Goal:** Clean-boosted Super Reverb, utilizing EQ to match string-gauge and pickup discrepancy.
**Full Configuration:**
*   **Block 1 (Global Gate):** Threshold [-65dB], Range [-80dB].
*   **Block 2 (Green 808):** Drive [Rhy: 0.5 / Lead: 2.0], Tone [6.5 / 7.0], Level [Rhy: 8.5 / Lead: 10.0].
*   **Block 3 (Amp - US SR65):** Vol [5.5 / 6.5], Bass [3.5 / 3.0], Mid [5.0 / 5.5], Treble [6.5], Output Level [+2.0dB].
*   **Block 4 (Cab - 410 US SR65):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.0, Dist 3.0"), Mix [A: 0dB, B: -3dB].
*   **Block 5 (EQ-8):** HPF [90Hz]. Tele Scenes (A-D): LowShelf +3.0dB @ 200Hz, LPF 5500Hz. ES-339 Scenes (E-H): Peak -4.0dB @ 250Hz, Peak +2.0dB @ 3500Hz, LPF 6500Hz.
*   **Block 6 (Spring Reverb):** Mix [20% / 25%], Decay [1.8s], Tone [6.0].