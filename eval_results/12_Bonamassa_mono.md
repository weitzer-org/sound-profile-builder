System Initialization: QC-2 Quad Cortex Systems Engineer
Target Confirmed: Joe Bonamassa Modern Blues Lead (Dumble-Style)
Hardware Target: QSC CP12 Active PA Speaker
Verification: "Dumble Overdrive Special" is represented in CorOS as **Overdrive RM**. The Klon Centaur is represented as **Myth Drive**. 

*Trigger Verification:* Before finalizing your lane output volume on the gig, I must ask: **Are your pickups Vintage Output, Medium, or High Output?** The configurations below assume standard modern PAFs for the ES-339 and standard 50s-wound single coils for the Telecaster. Adjust the Input Block Gain +/- 2.0dB based on your specific pickup winding.

Per your mandate, here is the Split-Bank Matrix configuration to achieve this exact analog target across two distinct guitar architectures. 

***

### 🎸 PROFILE 1: Gibson ES-339 (Humbuckers)
**Bank Assignment:** Row 2 (Scenes E-H)
**Gain Staging Physics:** Humbuckers produce a heavily compressed, bass-rich signal that easily "muds out" the cascading gain stages of a Dumble circuit. We must pad the Global Input to prevent digital input clipping and utilize an HPF (High-Pass Filter) to keep the low-end tight through the 12" QSC CP12.

**Scene Allocation:**
*   **Scene E (Rhythm):** Smooth, touch-sensitive edge-of-breakup. Drive pedal bypassed.
*   **Scene F (Lead):** Mid-focused, singing sustain. Myth Drive engaged. +1.5dB output boost.
*   **Scene G (Dry/Comping):** Lead tone with Delays/Reverbs bypassed.
*   **Scene H (Ambient):** High-mix delay/reverb for volume swells.

#### Table A: Main Signal Chain (ES-339)
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 35% | Noise Red: 50% | Humbuckers are quieter, but high-gain cascades raise the noise floor. Higher reduction on Lead. **Input Gain: -3.0dB** to preserve amp headroom. |
| **Pre-FX** | Myth Drive | *Bypassed* | Gain: 1.5, Tone: 6.0, Vol: 8.5 | Acts as a clean mid-boost. Hits the V1 tube of the amp hard without compressing the bass frequencies. |
| **Amp** | Overdrive RM | Vol: 4.5, OD: 4.0 | Vol: 5.5, OD: 6.5 | Dumble topology. Lower bass (3.5) prevents tube sag/farting. We rely on the Master Volume (6.5) for tube compression, not just preamp gain. |
| **EQ** | Parametric-8 | *Bypassed* | HPF: 110Hz, LPF: 5500Hz | Tames the resonant low-mid frequencies of the semi-hollow ES-339. LPF rolls off high-fuzz. |
| **Cab** | 412 Brit 30 | Mic 1 (121 Ribbon), Mix 0dB | Mic 1 (121), Pos: 1.5, Dist: 2.0" | V30 speakers match Bonamassa's live rig. Ribbon mic smooths out the upper midrange spikes from the bridge humbucker. |
| **Post-FX 1** | Analog Delay | Mix: 12%, Time: 350ms | Mix: 22%, Time: 350ms | Simulates a Way Huge Aqua Puss. Thickens the lead line without cluttering the rhythm pocket. |
| **Post-FX 2** | Plate Reverb | Mix: 15% | Mix: 25% | Hall/Plate verbs provide the "amp in a room" width. High Pass the reverb at 200Hz to avoid mud. |

***

### 🎸 PROFILE 2: Fender Telecaster (Single Coils)
**Bank Assignment:** Row 1 (Scenes A-D)
**Gain Staging Physics:** Single coils lack the midrange output required to push a Dumble circuit into its signature smooth compression. If you just crank the amp gain, the tone becomes brittle. We will use the *Chameleon Strategy*: boosting the Input block, warming the low-mids with EQ, and heavily rolling off the "ice-pick" treble.

**Scene Allocation:**
*   **Scene A (Rhythm):** Fat, SRV-style clean/grit.
*   **Scene B (Lead):** Thick, saturated modern blues lead.
*   **Scene C (Dry/Comping):** Lead tone, time-based FX off.
*   **Scene D (Ambient/FX):** Heavy tape delay/verb.

#### Table B: Main Signal Chain (Telecaster)
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 55% | Noise Red: 70% | Single coils hum heavily. High reduction required. **Input Gain: +2.5dB** to compensate for low output pickups. |
| **Pre-FX** | Myth Drive | *Bypassed* | Gain: 3.5, Tone: 4.0, Vol: 8.0 | Higher gain than the humbucker profile to force compression. Tone rolled back to tame bridge pickup harshness. |
| **EQ** | Parametric-8 | Band 2: +2.5dB (200Hz) | Band 2: +2.5dB, LPF: 4200Hz | Adds "weight" (Body) to the single coil. LPF physically limits the pick attack, simulating a humbucker's frequency curve. |
| **Amp** | Overdrive RM | Vol: 5.5, OD: 5.0 | Vol: 7.0, OD: 7.5 | Amp must work harder. Bass increased to 5.0 (Teles don't fart out like humbuckers do). Treble pulled back to 4.0. |
| **Cab** | 412 Brit 30 | Mic 1 (57 Dyn), Mix 0dB | Mic 1 (57), Pos: 2.0, Dist: 1.0" | Switched to an SM57 equivalent for punch. Moving the mic off-center (Pos 2.0) naturally warms the bright Telecaster tone. |
| **Post-FX 1** | Analog Delay | Mix: 15%, Time: 350ms | Mix: 25%, Time: 350ms | Smears the transients of the single coil, making fast runs sound more connected and liquid. |
| **Post-FX 2** | Plate Reverb | Mix: 15% | Mix: 25% | Decay set to 1.8s. Fills the gaps between staccato blues phrasing. |

***

### 🔧 Troubleshooting & Refinement Tree
If the tone translates poorly through your QSC CP12 at gig volume:
1.  **If the Tone is "Too Distorted" or "Too Fuzzy" (especially on the ES-339):**
    *   *Input Pad:* Lower Input Block Gain from -3.0dB down to -6.0dB (Simulates rolling off your guitar's volume knob).
    *   *Amp Gain:* Reduce Overdrive RM `OD` parameter by 2.0 increments.
2.  **If the Amp sounds "Broken/Farty" on the low strings:**
    *   *Tube Sag:* Reduce Amp Block `Bass` parameter down to 2.5 or 3.0. Dumble circuits cascade the bass frequencies; less is more.
3.  **To Restore Volume (Headroom Rule):** 
    *   If you lowered the gain, compensate for the lost volume by raising the Lane Output Level (the circle at the far right of the grid) by +2.0dB. Never use a compressor to fix lost output volume.

***

### 💾 Session Registry Protocol
Appending to Session Library...

**Session Library (Active Presets)**
1.  Preset Name: "Spoonful - ES339" *(Pre-existing)*
2.  Preset Name: "Smokin' Joe Dumble - Dual Rig"
    *   **Target:** Joe Bonamassa Modern Blues Lead (Mid-2010s live tone).
    *   **Guitar A:** ES-339 (Humbuckers) / **Guitar B:** Telecaster (Single Coil).
    *   **Physics Goal:** Cascading tube overdrive with Klon mid-push. Manage headroom limits to prevent Dumble circuit "sag/farting" while playing fast legato runs.
    *   **Full Configuration (ES-339 Base):**
        *   Block 1 (Input/Gate): Input [-3.0dB], Adaptive Gate Block: Noise Red [Rhy: 35% / Lead: 50%], Thresh [-55dB], Decay [150ms].
        *   Block 2 (Myth Drive): Gain [1.5], Treble [6.0], Output [8.5]. Bypass assigned to Scene E.
        *   Block 3 (Amp - Overdrive RM): Vol [4.5 / 5.5], OD [4.0 / 6.5], Bass [3.5], Mid [6.0], Treble [5.0], Master [6.5], Presence [4.0]. Output Level [Rhy: +0dB / Lead: +1.5dB].
        *   Block 4 (EQ-8): HPF [110Hz], Band 3 [-1.5dB at 300Hz], LPF [5500Hz]. 
        *   Block 5 (Cab - 412 Brit 30): Mic A (Ribbon 121, Pos 1.5, Dist 2.0"), High Cut [6.5kHz].
        *   Block 6 (Analog Delay): Mix [12% / 22%], Time [350ms], Fdbk [25%].
        *   Block 7 (Plate Reverb): Mix [15% / 25%], Decay [1.8s], HPF [200Hz].
    *   **Full Configuration (Telecaster Base):**
        *   Block 1 (Input/Gate): Input [+2.5dB], Adaptive Gate Block: Noise Red [Rhy: 55% / Lead: 70%].
        *   Block 2 (Myth Drive): Gain [3.5], Treble [4.0], Output [8.0].
        *   Block 3 (Amp - Overdrive RM): Vol [5.5 / 7.0], OD [5.0 / 7.5], Bass [5.0], Mid [6.5], Treble [4.0]. Output Level [Rhy: +0dB / Lead: +1.5dB].
        *   Block 4 (EQ-8): HPF [90Hz], Band 2 [+2.5dB at 200Hz], LPF [4200Hz] (Simulates humbucker rounded top-end).
        *   Block 5 (Cab - 412 Brit 30): Mic A (Dyn 57, Pos 2.0, Dist 1.0").