As "QC-2," Quad Cortex Systems Engineer, I have initialized the workspace for a high-headroom, Stevie Ray Vaughan-style funk/blues profile. 

### Target Physics & Acoustic Rationale
To achieve the SRV tone, we must rely on a **"Cold Amp / Hot Pedal"** gain-staging philosophy. The core amplifier will be the **US SPR 65** (Fender Super Reverb '65), utilizing its massive headroom and scooped midrange. To execute the SRV lead tone, we will bypass the amp's natural breakup and instead violently push the V1 preamp tube using the **Green 808** (Ibanez TS808 pseudonym), configured with maximum level and minimum drive. 

Because you are alternating between Vintage Single Coils and Medium-Output Humbuckers, the preset is mapped across a Split-Bank Matrix. 

***

### GUITAR 1: Fender Telecaster Single Coil (Row 1: Scenes A–D)
**Pickup Compensation Strategy:** Vintage single coils lack the low-mid "thump" of SRV's heavy-gauge Stratocaster strings. We will use a +1.0dB input gain, a Parametric-8 EQ to synthesize physical body (200Hz), and a High Cut to tame the ice-pick transients of the Telecaster bridge pickup.

**Table A: Main Signal Chain (Telecaster - Row 1)**
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -65dB / Red: 25% | Thresh: -60dB / Red: 40% | Tele single-coil 60-cycle hum management. |
| **Pre-FX** | Green 808 | Bypass: OFF | Bypass: ON (Drive: 2.0, Level: 9.0, Tone: 6.5) | Hits the amp's headroom ceiling violently for asymmetrical clipping. |
| **EQ** | Parametric-8 | Band 1: +2.5dB (200Hz Shelf), High Pass: 80Hz | Band 1: +2.5dB (200Hz Shelf), High Pass: 80Hz | Chameleon Strategy: Simulates thick SRV strings (0.13s) and adds weight. |
| **Amp** | US SPR 65 | Vol: 4.5, Treb: 6.5, Mid: 4.0, Bass: 3.5, Bright: ON | Vol: 4.5, Treb: 6.5, Mid: 4.0, Bass: 3.5, Bright: ON | Non-Master Vol amp. Scooped mids leave room for the TS808 mid-hump. |
| **Cab** | 410 US SPR | Mic A: Dyn 57 (Pos 0.2, Dist 1.0"), Mix: 0dB | Mic A: Dyn 57 (Pos 0.2, Dist 1.0"), Mix: 0dB | 10" speakers provide fast transient response for funk strumming. |
| **Post-FX** | Spring Reverb | Mix: 20%, Decay: 1.5s, Low Cut: 150Hz | Mix: 25%, Decay: 1.5s, Low Cut: 150Hz | Essential Fender tank physics. Low cut prevents QSC CP12 woofer mud. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB | Overall SPL push for soloing without altering the gain structure. |

*(Note: Scene C = Dry/Comping, Scene D = Ambient/Vibrato for "Cold Shot" vibes).*

***

### GUITAR 2: Gibson ES-339 Humbuckers (Row 2: Scenes E–H)
**Pickup Compensation Strategy:** Humbuckers will immediately collapse the headroom of the US SPR 65, turning a pristine clean tone into a fuzzy, congested overdrive. We must apply a strict **-4.0dB Pad** at the Input Block and utilize the EQ block to carve out the muddy low-mids (300Hz), forcing the ES-339 to mimic a bell-like single coil.

**Table B: Main Signal Chain (ES-339 - Row 2)**
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -70dB / Red: 15% | Thresh: -65dB / Red: 20% | Humbuckers are quieter at baseline; lower Noise Reduction (%) needed. |
| **Pre-FX** | Green 808 | Bypass: OFF | Bypass: ON (Drive: 1.0, Level: 7.5, Tone: 7.5) | Lower level/drive compared to Tele to prevent humbucker low-end blowout. Tone increased for bite. |
| **EQ** | Parametric-8 | HPF: 110Hz, Band 2: -3.0dB (300Hz Q:1.5) | HPF: 110Hz, Band 2: -3.0dB (300Hz Q:1.5) | Chameleon Strategy: Carves out Gibson low-mid thickness to restore "spank" and clarity. |
| **Amp** | US SPR 65 | Vol: 3.8, Treb: 7.0, Mid: 4.5, Bass: 2.0, Bright: OFF | Vol: 3.8, Treb: 7.0, Mid: 4.5, Bass: 2.0, Bright: OFF | Lower Amp Vol preserves headroom. Bright switch OFF to prevent harsh humbucker transients. |
| **Cab** | 410 US SPR | Mic A: Dyn 57, Mic B: Cond 414 (Mix: -3dB) | Mic A: Dyn 57, Mic B: Cond 414 (Mix: -3dB) | Condenser mic blended in to add "air" and upper-treble sparkle back to the ES-339. |
| **Post-FX** | Spring Reverb | Mix: 18%, Decay: 1.5s, Low Cut: 200Hz | Mix: 22%, Decay: 1.5s, Low Cut: 200Hz | Slightly reduced mix compared to Tele to maintain rhythmic articulation. |
| **Output** | Lane Output | Level: +2.0dB | Level: +3.5dB | Compensates for the -4.0dB Input Pad to match the QSC CP12 output levels of the Telecaster. |

***

### Troubleshooting & Refinement Tree (FRFR / CP12 Logic)
If you audition this preset through your QSC CP12 and encounter issues, follow this exact protocol:
1. **Tone is "Too Compressed / Farty" on the ES-339 Lead Scene:** The humbuckers are pushing too much low-frequency energy into the TS808. **Fix:** Lower the Input Block Gain to -6.0dB. Do not touch the Amp Bass knob yet. 
2. **Telecaster sounds "Piercing" or "Ice-Pick" on the Lead Scene:** The QSC CP12 horn tweeter is accurately reproducing the 5kHz peak of the Telecaster. **Fix:** Enter the Parametric-8 EQ block and engage the Low Pass Filter (LPF) at 4500Hz. 
3. **Not Enough Volume (SPL) on Clean Rhythm:** Because the US SPR 65 lacks a Master Volume, turning up the Amp 'Vol' knob will add grit. **Fix:** Strictly increase the Lane Output Level to raise the physical loudness in the room.

***

### Session Library (Active Presets)
**2. Preset Name:** "Texas Flood - Split Matrix"
**Target:** Stevie Ray Vaughan (1983).
**Guitar:** Multi-Target (Fender Telecaster Row 1 / Gibson ES-339 Row 2).
**Physics Goal:** High-headroom 4x10 cleans violently pushed into asymmetrical clipping via an 808 circuit, mimicking cold-amp/hot-pedal physics. 
**Full Configuration:**
*   **Block 1 (Adaptive Gate):** Noise Red [Tele: 25% / 339: 15%], Thresh [-65dB], Decay [150ms].
*   **Block 2 (Green 808):** Drive [Tele Lead: 2.0 / 339 Lead: 1.0], Level [Tele: 9.0 / 339: 7.5], Tone [Tele: 6.5 / 339: 7.5].
*   **Block 3 (EQ-8):** Tele [HPF 80Hz, Band 1 +2.5dB @ 200Hz] / 339 [HPF 110Hz, Band 2 -3.0dB @ 300Hz]. 
*   **Block 4 (Amp - US SPR 65):** Vol [Tele: 4.5 / 339: 3.8], Bass [Tele: 3.5 / 339: 2.0], Mid [4.0 / 4.5], Treble [6.5 / 7.0], Bright [Tele: ON / 339: OFF]. 
*   **Block 5 (Cab - 410 US SPR):** Mic A (Dyn 57, Pos 0.2, Dist 1.0"). Mic B (339 Only: Cond 414, Pos 0.5, Dist 2.0", Mix -3dB).
*   **Block 6 (Spring Reverb):** Mix [18-25% Scene Dependent], Decay [1.5s], Low Cut [150Hz].
*   **Global/Lane Out:** Output Level [+1.5dB Lead Compensation]. Input Gain [Tele: +1.0dB / 339: -4.0dB].