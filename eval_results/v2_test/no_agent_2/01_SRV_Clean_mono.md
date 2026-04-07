As QC-2, your Quad Cortex Systems Engineer, I have mapped out your "SRV Clean Funk Blues" preset. 

To achieve Stevie Ray Vaughan’s massive, high-headroom clean tone while retaining the ability to slam the front end with a TS808, we must respect the physics of analog gain staging. Since SRV famously used Fender Super Reverbs and Twin Reverbs, we will use the **US TWN 65 Vibrato** model paired with a **410 US Super** cabinet. This amp lacks a Master Volume; headroom is determined entirely by the relationship between the Input Gain, the Amp Volume, and the Lane Output Level.

Per your instructions, I have engineered **two distinct, isolated signal chains** to perfectly match the pickup physics of a Fender Telecaster and a Gibson ES-339. 

***

### GUITAR 1: FENDER TELECASTER (SINGLE COIL)
**Bank Row 1 (Scenes A-D) | Target: Thicker body, tamed ice-pick attack.**
*Telecasters naturally lack the low-mid "girth" of SRV's heavy-gauge Stratocaster setup. We will use the Parametric EQ to add simulated string mass and keep the Amp Volume slightly higher to achieve edge-of-breakup compression.*

**Input Gain (Grid Circle 1):** `0.0dB`

#### Table 1: Telecaster Signal Chain
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate | Thresh: -62dB | Thresh: -58dB | Single coils have higher noise floors, especially when the 808 is engaged. |
| **Pre-FX 1** | Parametric-8 | *Always On:* Band 2 (Peak): +2.5dB @ 200Hz, LPF: 5.5kHz | *Always On* | Adds "thick string" body. LPF removes PA-speaker digital harshness. |
| **Pre-FX 2** | Green 808 | Bypass | Drive: 2.5, Tone: 6.5, Level: 8.5 | SRV famously ran the 808 with low Drive and high Level to slam the V1 tube. |
| **Amp** | US TWN 65 Vibrato | Bright: OFF, Vol: 5.5, Treb: 5.5, Mid: 6.0, Bass: 4.5 | Bright: OFF, Vol: 5.5, Treb: 5.5, Mid: 6.0, Bass: 4.5 | Tele bridge is bright; keep Bright OFF. Mids bumped slightly for funk body. No Master Vol. |
| **Cab** | 410 US Super | Mic A: Dyn 57 (Pos 0.5, Dist 1.0"), Mix: 0dB | Mic B: Ribbon 160 (Pos 1.5, Dist 2.5"), Mix: -3dB | 4x10 setup gives that punchy, fast transient response for funk strumming. |
| **Post-FX** | Spring Reverb | Mix: 18%, Decay: 2.5s | Mix: 22%, Decay: 2.5s | Emulates the analog tank in the Twin Reverb. Longer tail for leads. |
| **Output** | Lane 1 Output | Level: +2.0dB | Level: +3.5dB | Overall SPL control to keep the QSC CP12 punching without clipping the QC outputs. |

***

### GUITAR 2: GIBSON ES-339 (HUMBUCKERS)
**Bank Row 2 (Scenes E-H) | Target: Maximum clarity, preventing premature tube clipping.**
*Humbuckers output roughly +3.0dB to +6.0dB more voltage than vintage single coils. If plugged directly into the Telecaster preset, the amp will compress, lose its "funk/snap", and the TS808 will turn into mush. We must aggressively pad the input and scoop the lower-mids to maintain SRV high-headroom clarity.*

**Input Gain (Grid Circle 1):** `-4.5dB` *(Crucial Step: Do not skip this!)*

#### Table 2: ES-339 Signal Chain
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate | Thresh: -68dB | Thresh: -65dB | Humbuckers are inherently quieter. Gate threshold lowered to let chords ring out. |
| **Pre-FX 1** | Parametric-8 | *Always On:* HPF: 110Hz, Band 3 (Peak): -3.0dB @ 300Hz | *Always On* | Strips out humbucker "mud" that causes the Twin model to fart out on funk chords. |
| **Pre-FX 2** | Green 808 | Bypass | Drive: 1.0, Tone: 7.0, Level: 7.0 | Lower drive needed because humbuckers push the pedal's op-amp into clipping faster. |
| **Amp** | US TWN 65 Vibrato | Bright: ON, Vol: 4.0, Treb: 6.5, Mid: 4.0, Bass: 2.5 | Bright: ON, Vol: 4.0, Treb: 6.5, Mid: 4.0, Bass: 2.5 | Bright switch ON restores top-end chime. Bass dropped to 2.5 to tighten the 339's mahogany body resonance. |
| **Cab** | 410 US Super | Mic A: Dyn 57 (Pos 0.5, Dist 1.0"), Mix: 0dB | Mic B: Cond 414 (Pos 1.0, Dist 2.0"), Mix: -2dB | Swapped the Ribbon for a Condenser on Mic B to retain high-end sparkle for humbuckers. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 2.0s | Mix: 18%, Decay: 2.0s | Slightly shorter decay and lower mix to prevent humbucker low-end buildup in the reverb tail. |
| **Output** | Lane 2 Output | Level: +5.0dB | Level: +6.5dB | Because we lowered Amp Vol and Input Gain to stay clean, we use transparent Output Gain to match the Telecaster's overall loudness. |

***

### Troubleshooting & Refinement Tree
If monitoring through the QSC CP12 at gig volume and the tone feels incorrect, follow this protocol:
1. **Too Distorted/Fuzzy (ES-339):** Lower the Input Block Gain further to `-6.0dB`. The ES-339 might have hotter-than-vintage pickups. Compensate for volume loss by raising Lane Output Level.
2. **Too "Farty" on the Low E String (Telecaster):** The 4x10 speaker simulation is taking on too much bass from the TS808. Lower the Amp Bass knob from `4.5` to `3.0`.
3. **Ice-Pick Highs through the QSC:** If the horn on the CP12 is glaring, lower the LPF in the Parametric-8 block from `5.5kHz` down to `4.2kHz`. Do *not* lower the Amp Treble knob, as that will ruin the core SRV tube voicings.

***

### Session Library (Active Presets)
**2. Preset Name: "Texas Twister - Dual Rig"**
*   **Target:** Stevie Ray Vaughan Clean Funk / Lead.
*   **Guitars:** Telecaster (Single Coil) / ES-339 (Humbucker).
*   **Physics Goal:** Maximize Twin Reverb headroom. Manage pickup output discrepancies via Input padding and pre-EQ to keep TS808 dynamics identical across guitars.
*   **Full Configuration:**
    *   **Row 1 (Telecaster):** Input [0.0dB]. Gate [-62dB/-58dB]. EQ-8 [Band 2 +2.5dB @ 200Hz, LPF 5.5kHz]. Green 808 [Drive 2.5, Tone 6.5, Lev 8.5]. Amp US TWN 65 [Bright OFF, Vol 5.5, Treb 5.5, Mid 6.0, Bass 4.5]. Cab 410 US Super [57/160 Mix -3dB]. Spring Reverb [Mix 18/22%, Dec 2.5s]. Out [+2.0dB/+3.5dB].
    *   **Row 2 (ES-339):** Input [-4.5dB]. Gate [-68dB/-65dB]. EQ-8 [HPF 110Hz, Band 3 -3.0dB @ 300Hz]. Green 808 [Drive 1.0, Tone 7.0, Lev 7.0]. Amp US TWN 65 [Bright ON, Vol 4.0, Treb 6.5, Mid 4.0, Bass 2.5]. Cab 410 US Super [57/414 Mix -2dB]. Spring Reverb [Mix 15/18%, Dec 2.0s]. Out [+5.0dB/+6.5dB].