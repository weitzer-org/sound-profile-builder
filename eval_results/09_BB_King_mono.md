Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

Target acquired: **B.B. King "Lucille" Tone**.
Physics Goal: High-headroom, crystalline clean platform with pronounced vocal midrange, snappy attack, and tight low-end. No clipping; dynamics must be controlled entirely by pickup output, picking hand pressure, and speaker compression. 

To ensure optimal translation through your QSC CP12 active PA speaker (which reproduces high and low frequencies with brutal honesty compared to a traditional guitar cab), we will utilize a strict EQ-compensation strategy. B.B. King played a hollowbody ES-355 with a Varitone switch, which physically rolled off lower-mids to prevent feedback and increase "sting." We will simulate this using the EQ-8 block.

*Trigger Verification:* Before you load this, **are your pickups Vintage Output, Medium, or High Output?** This preset assumes Medium Output. Adjust the Input Block Gain accordingly if they are excessively hot.

Here is your dual-instrument split-bank configuration. 

---

### MULTI-GUITAR TARGET OUTPUT & SPLIT-BANK MATRIX

We are utilizing the **Split-Bank Matrix** to house both your Telecaster and ES-339 within a single preset. The EQ and Input Gain blocks are the "Chameleons" here, altering the physics of the QC to match the instrument.

*   **Row 1 (Scenes A-D): Fender Telecaster Single Coil Profile**
    *   *Physics Protocol:* Telecasters lack the low-mid mass of Lucille and have a sharper transient. We must boost the Input Block to hit the amp's sweet spot and add "Body" via EQ.
    *   *Input Block Gain:* **+3.0dB**
    *   *Parametric-8 EQ (Tele Mode):* Band 1 (Low Shelf) +2.5dB at 200Hz. Band 8 (LPF) set to 4500Hz to tame the ice-pick pick attack.
*   **Row 2 (Scenes E-H): Gibson ES-339 Humbucker Profile**
    *   *Physics Protocol:* Humbuckers will overdrive the Twin Reverb model if not padded. To mimic the Varitone switch (Position 2 or 3) and prevent mud, we pad the input and carve out lower mids.
    *   *Input Block Gain:* **-3.0dB**
    *   *Parametric-8 EQ (339 Mode):* Band 3 (Bell) -3.0dB at 350Hz. Band 4 (Bell) +1.5dB at 1.2kHz for the vocal "sting".

**Scene Assignments:**
*   **A / E:** Rhythm (Dynamic, -1.5dB Lane Output).
*   **B / F:** Lead (B.B.'s Stinging Lead, +1.5dB Lane Output).
*   **C / G:** Dry / Comping (Reverb bypassed for tight studio rhythm).
*   **D / H:** Vibrato (Classic B.B. slow amplitude modulation, Chorus/Vibrato block engaged).

---

### Table A: Main Signal Chain (B.B. King / US Twin)
*Note: Mark Scene-Specific changes clearly with (Right-Click > Assign) in Cortex Control.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -65dB | High-headroom cleans expose floor noise. Keep threshold low to preserve sustain. |
| **Pre-FX** | Parametric-8 | *Tele:* Body +2.5dB <br>*339:* Mid -3.0dB | *Tele:* Body +2.5dB <br>*339:* Mid -3.0dB | Adapts pickup physics to mimic an ES-355 Varitone circuit. |
| **Amp** | US TWN Vibrato | Vol: 4.5 <br>Mid: 5.5 | Vol: 6.0 <br>Mid: 6.5 | Emulates non-master volume preamp push. Mids are boosted for Lucille's vocal honk. |
| **Cab** | 212 US TWN C12K | Mic A: Dyn 57 (Pos:0.4) <br>Mic B: Rib 121 (Pos:0.6) | Mic A: Dyn 57 (Pos:0.4) <br>Mic B: Rib 121 (Pos:0.6) | Dyn 57 captures the "sting" edge-of-cap. Rib 121 provides warmth to counteract QSC CP12 tweeter harshness. |
| **Post-FX** | Chief CE2W Chorus | Bypassed | Bypassed (Active in Sc D/H) | Used in Vibrato mode (Rate 3.0Hz, Depth 60%) to mimic B.B.'s finger vibrato or amp tremolo. |
| **Post-FX** | Spring | Mix: 15% <br>Dwell: 3.5 | Mix: 18% <br>Dwell: 4.0 | Fender tube spring tank emulation. Slightly higher mix on lead for trail sustain. |
| **Lane Output** | Output Level | Level: 0.0dB | Level: +1.5dB | Overall SPL management. Keeps preamp clean while increasing stage volume. |

---

### Troubleshooting & Refinement Tree
If you load this preset and it sounds **Too Distorted / Broken** (especially on the ES-339):
1.  **Input Pad:** Lower the Global Input Block Gain from -3.0dB to -6.0dB. Hot humbuckers will cause digital fuzz if they clip the front of the Amp block.
2.  **Amp Gain:** Lower the `Vol` parameter on the US TWN Vibrato block by 1.5 increments. 
3.  **Tube Sag:** Ensure the `Bass` parameter on the US TWN Vibrato is no higher than 3.5. Vintage Twin circuits "fart out" and sag terribly if too much low-end hits the power amp section.
4.  **Output Compensation:** If lowering the Volume drops your stage level through the QSC CP12, do *not* add a compressor. Raise the `Lane Output Level` or the master volume on the CP12 itself.

---

### Session Registry Protocol
*Appending build to Active Session Library...*

**Session Library (Active Presets)**

1. Preset Name: "Spoonful - ES339" *(Previously logged)*
2. Preset Name: "Lucille's Twin - Dual"
*   **Target:** B.B. King (1970s Live Tone).
*   **Guitar:** ES-339 (Humbuckers) & Telecaster (Single Coils).
*   **Physics Goal:** Maximize clean headroom, mimic ES-355 Varitone switch, control dynamics via Lane Output rather than compression.
*   **Full Configuration:**
    *   **Block 1 (EQ-8 - Scene Controlled):** Tele Mode (Sc A-D) [HPF 90Hz, Band 1 Low Shelf +2.5dB @ 200Hz, LPF 4500Hz]. 339 Mode (Sc E-H) [HPF 110Hz, Band 3 -3.0dB @ 350Hz, Band 4 +1.5dB @ 1200Hz, LPF 6000Hz].
    *   **Block 2 (Amp - US TWN Vibrato):** Bright Switch [Off], Vol [Rhy: 4.5 / Lead: 6.0], Treble [6.0], Mid [Rhy: 5.5 / Lead: 6.5], Bass [3.5], Presence [5.0]. 
    *   **Block 3 (Cab - 212 US TWN C12K):** Mic A (Dyn 57, Pos 0.4, Dist 1.0"), Mic B (Ribbon 121, Pos 0.6, Dist 2.5"), Mix [A: 0dB, B: -3dB], Low Cut [100Hz], High Cut [6500Hz].
    *   **Block 4 (Mod - Chief CE2W Chorus):** Mode [Vibrato], Rate [3.0Hz], Depth [60%]. (Active only on Scenes D/H).
    *   **Block 5 (Reverb - Spring):** Mix [15% / 18%], Dwell [3.5 / 4.0], Low Color [5.0], High Color [4.0].
    *   **Output Node:** Level [Rhy: 0.0dB / Lead: +1.5dB]. Input Gain Block assigned to -3.0dB for ES-339 and +3.0dB for Telecaster.