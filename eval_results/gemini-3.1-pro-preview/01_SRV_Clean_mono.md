**Quad Cortex Systems Engineer (QC-2) Initialization...**
**Target Protocol:** Clean Funk Blues / "SRV" Style (High Headroom + TS808 Push)
**Speaker Profile Target:** QSC CP12 (Active PA/FRFR)

To achieve the classic SRV high-headroom, glassy clean tone that takes an Ibanez Tube Screamer (TS808) without collapsing into fuzz, we must rely on the **"US Vibra"** (Fender Vibroverb) amplifier model. The fundamental physics of this tone rely on slamming the preamp tubes with signal volume *from the pedal*, not distortion *from the amp*. 

Because an ES-339 (Humbuckers) and a Telecaster (Single Coils) interact with a TS-style mid-hump entirely differently, we will utilize the **Split-Bank Matrix** to build two distinct rigs within the same preset.

---

### Part 1: Fender Telecaster Single Coil Configuration (Row 1 / Scenes A-D)
**Physics Goal:** Single coils naturally provide the glassy snap required for this tone, but they lack the low-end authority SRV got from thick strings and aggressive picking. We will use the Parametric-8 EQ to add "body" and tame the highest ice-pick frequencies before they hit the TS808.

**Gain Staging:** Input Block Gain set to **0.0dB**. 

#### Table A: Main Signal Chain (Telecaster Single Coils)
*(Right-Click > Assign to Scenes A/B for Rhythm/Lead variations)*

| Block Category | Model Name | Rhythm Settings (Sc A - Clean Funk) | Lead Settings (Sc B - Pushed SRV) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -60dB | Single coils have 60-cycle hum. Tighter gate on Lead prevents TS808 hiss. |
| **Pre-FX (EQ)** | Parametric-8 | Band 1 (Peak): +2.5dB @ 200Hz | Band 1 (Peak): +2.5dB @ 200Hz | Adds "Texas" body to thinner single coils. LPF @ 6kHz to tame snap. |
| **Pre-FX (Drive)**| Green 808 | *Bypassed* | Drive: 1.5, Level: 9.0, Tone: 5.5 | The SRV secret: Drive low, Level maxed. Slams the amp's headroom into pure tube compression. |
| **Amp** | US Vibra | Vol: 4.5, Treb: 6.5, Bass: 4.0 | Vol: 4.5, Treb: 6.5, Bass: 4.0 | Classic 40W Fender circuit. Bright switch ON. Master Volume is N/A for this circuit. |
| **Cab** | 1x15 US Vibra | Mic A (Dyn 57): Pos 0.5, Dist 1.0" | Mic B (Ribbon 121): Pos 1.5, Dist 3.0" | 15" speaker provides the massive, uncompressed low-end essential to SRV tones. Mix A: 0dB, Mix B: -3dB. |
| **Post-FX** | Spring Reverb | Mix: 20%, Decay: 1.5s | Mix: 25%, Decay: 1.5s | SRV's signature spatial footprint. Essential for the "drip" on muted funk strums. |

---

### Part 2: Gibson ES-339 Humbucker Configuration (Row 2 / Scenes E-H)
**Physics Goal:** Humbuckers inherently push 30-50% more voltage than vintage single coils. If we plug the ES-339 straight into the Telecaster rig, the Amp block will compress too early, losing the "glassy funk" headroom, and the Green 808's mid-hump will make the humbuckers sound honky and muddy. 

**Gain Staging:** Input Block Gain reduced to **-5.0dB**. This effectively simulates rolling your guitar's volume knob down, forcing the humbucker to behave like a single coil regarding headroom. Compensate by raising Lane Output Level by +3.0dB.

#### Table B: Main Signal Chain (ES-339 Humbuckers)
*(Right-Click > Assign to Scenes E/F for Rhythm/Lead variations)*

| Block Category | Model Name | Rhythm Settings (Sc E - Clean Funk) | Lead Settings (Sc F - Pushed SRV) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -70dB | Thresh: -65dB | Humbuckers are quieter natively, allowing a looser gate for more sustain. |
| **Pre-FX (EQ)** | Parametric-8 | Band 3: -3.0dB @ 400Hz | Band 3: -3.0dB @ 400Hz | Humbuckers + TS808 = Mud. Scooping 400Hz cleans up the low-mids for funk rhythms. |
| **Pre-FX (Drive)**| Green 808 | *Bypassed* | Drive: 1.0, Level: 8.0, Tone: 7.0 | Tone knob raised higher than Tele config to cut through the ES-339's darker mahogany wood resonance. |
| **Amp** | US Vibra | Vol: 3.5, Treb: 7.0, Bass: 3.5 | Vol: 3.5, Treb: 7.0, Bass: 3.5 | Amp volume lowered to preserve headroom against the hotter pickup transient. Treble increased for "glass". |
| **Cab** | 1x15 US Vibra | Mic A (Dyn 57): Pos 1.0, Dist 1.5" | Mic B (Ribbon 121): Pos 2.0, Dist 4.0" | Pushing the Mic A position slightly further outward prevents humbucker proximity-effect booming. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 1.2s | Mix: 20%, Decay: 1.2s | Reverb mix slightly lowered compared to single coils to maintain note definition on thicker humbucker chords. |

---

### Troubleshooting & Refinement Tree
If the tone is still "Too Distorted" or lacking headroom on the ES-339:
1. **Input Pad:** Lower Input Block Gain further to -7.0dB. 
2. **Amp Gain:** Reduce Amp Volume to 3.0. 
3. **Tube Sag:** Lower the Bass parameter on the Amp block to 2.5. Low frequencies distort faster than high frequencies in Fender circuits. 
4. **Output Compensation:** If the tone is now too quiet through your QSC CP12, raise the Lane Output Level, *not* the Amp Volume.

---

### Session Registry Protocol
Appending to active session memory for recall.

**Session Library (Active Presets)**

1. Preset Name: "Spoonful - ES339"
*Target: Howlin' Wolf / Hubert Sumlin (1960).*
*(Archived - See previous session).*

2. Preset Name: "Texas Glass Matrix - Tele & 339"
*Target: Stevie Ray Vaughan (Clean Funk to Pushed Lead).*
*Guitars: Telecaster (Row 1), ES-339 (Row 2).*
*Physics Goal: Absolute headroom preservation with asymmetric pedal gain staging.*
*Full Configuration (Telecaster):*
Block 1 (Global Gate): Thresh [-65dB / -60dB]. Input Gain: [0.0dB].
Block 2 (EQ-8): Peak 200Hz [+2.5dB], LPF [6000Hz].
Block 3 (Green 808): Drive [1.5], Level [9.0], Tone [5.5]. (Bypass on Scene A, Active Scene B).
Block 4 (Amp - US Vibra): Vol [4.5], Treble [6.5], Bass [4.0], Bright [ON].
Block 5 (Cab - 1x15 US Vibra): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 3.0"), Mix [A: 0dB, B: -3dB].
Block 6 (Spring Reverb): Mix [20% / 25%], Decay [1.5s].

*Full Configuration (ES-339):*
Block 1 (Global Gate): Thresh [-70dB / -65dB]. Input Gain: [-5.0dB].
Block 2 (EQ-8): Bell 400Hz [-3.0dB].
Block 3 (Green 808): Drive [1.0], Level [8.0], Tone [7.0]. (Bypass on Scene E, Active Scene F).
Block 4 (Amp - US Vibra): Vol [3.5], Treble [7.0], Bass [3.5], Bright [ON].
Block 5 (Cab - 1x15 US Vibra): Mic A (Dyn 57, Pos 1.0, Dist 1.5"), Mic B (Ribbon 121, Pos 2.0, Dist 4.0"), Mix [A: 0dB, B: -3dB]. Lane Output Level [+3.0dB].
Block 6 (Spring Reverb): Mix [15% / 20%], Decay [1.2s].