Target Locked: **The Edge (U2) - 1964 Vox AC30TB & Dotted-Eighth Delay.**
Analog Hunt Specs: Korg SDD-3000 Preamp (always-on for signal coloration), 1964 Vox AC30 Top Boost (Alnico Blue speakers), Herdim pick (dimpled side for "chiff" attack), TC Electronic 2290 / Korg SDD-3000 Delays. 

As **QC-2**, I have engineered this preset to handle the strict physics of an AC30 Top Boost (a non-master volume circuit that easily falls into "fuzz" territory if gain-staged improperly). To simulate the legendary always-on SDD-3000 preamp and the Herdim pick attack, we are utilizing the **Parametric-8 EQ** block before the amp.

Per your requirement, here is the Split-Bank Matrix detailing distinct configurations for your **Fender Telecaster (Row 1)** and **Gibson ES-339 (Row 2)**.

***

### 🎸 ROW 1: FENDER TELECASTER (Scenes A–D)
**Pickup Output Compensation:** Single-coils lack the low-mid push to drive an AC30 into that specific thick, chiming compression. We will use the Pre-EQ to boost the 200Hz body and push the Brilliant Volume higher. 
*   **Scene A:** Rhythm (Dry, tight edge-of-breakup)
*   **Scene B:** Lead / Classic Edge (Dotted-8th Delay engaged)
*   **Scene C:** Dry / Comping (No spatial FX)
*   **Scene D:** Ambient (Shimmer added for "With or Without You" swells)

#### Table A: Telecaster Signal Chain
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -50dB / Red: 40% | Thresh: -55dB / Red: 30% | 60-cycle hum management. Lead scene opens up to preserve delay trails. |
| **Pre-FX (Preamp)** | Parametric-8 | Peak 1: +2.5dB @ 200Hz | Peak 1: +2.5dB @ 200Hz | **Chameleon EQ:** Simulates SDD-3000 preamp thickness. Band 7 (+3dB @ 4.5kHz) simulates Herdim pick attack. |
| **Amp** | UK C30 Top Boost | Bril Vol: 5.5 / Norm: 3.0 | Bril Vol: 6.5 / Norm: 3.0 | Tube Taper Logic: Pushing Brilliant Vol creates the chime. Cut set to 4.0. *Non-Master Vol amp.* |
| **Cab** | 212 UK C30 Fawn | Dyn 57 (Pos 1.5, Dist 1") | Dyn 57 (Pos 1.5, Dist 1") | Speaker Physics: Celestion Alnico Blues. Rib 121 added at -4dB for low-end warmth. |
| **Post-FX 1** | Digital Delay | *Bypassed* | Mix: 45% / Sync: 3/16 | The "Streets" engine. High mix ensures repeats are almost identical in volume to the dry pick attack. Mod Rate: 1.5Hz. |
| **Post-FX 2** | Hall Reverb | Mix: 12% / Decay: 1.5s | Mix: 18% / Decay: 2.0s | Spatial Goal: Creates the arena-sized stadium wash without muddying the dotted-8th repeats. |

***

### 🎸 ROW 2: GIBSON ES-339 (Scenes E–H)
**Pickup Output Compensation:** Humbuckers will completely collapse the headroom of a vintage AC30 model, resulting in a dark, muddy fuzz rather than crystalline chime. We must apply a severe Input Pad, aggressively cut the low-mids, and raise the Tone Cut frequency to restore the "glass" of the AC30.
*   **Scene E:** Rhythm (Input Padded, tight)
*   **Scene F:** Lead / Classic Edge (Dotted-8th Delay engaged)
*   **Scene G:** Dry / Comping 
*   **Scene H:** Ambient (Shimmer Wash)

#### Table B: ES-339 Signal Chain
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Gain: -4.5dB / Red: 20% | Gain: -4.5dB / Red: 15% | **Critical:** -4.5dB Input Gain pad prevents the humbuckers from clipping the amp block into muddy fuzz. |
| **Pre-FX (Preamp)** | Parametric-8 | Peak 1: -3.5dB @ 250Hz | Peak 1: -3.5dB @ 250Hz | **Chameleon EQ:** Carves out humbucker mud. HPF raised to 120Hz. Band 7 (+4dB @ 4.5kHz) forces the "chiff" attack through the dark pickups. |
| **Amp** | UK C30 Top Boost | Bril Vol: 3.5 / Norm: 1.5 | Bril Vol: 4.5 / Norm: 1.5 | Tube Taper Logic: Lower volume to maintain edge-of-breakup. Treble raised to 7.5. Tone Cut reduced to 2.0 to allow more high-end glass. |
| **Cab** | 212 UK C30 Fawn | Dyn 57 (Pos 0.5, Dist 1") | Dyn 57 (Pos 0.5, Dist 1") | Speaker Physics: Mic moved closer to the dust cap (Pos 0.5) to artificially extract more top-end bite from the ES-339. |
| **Post-FX 1** | Digital Delay | *Bypassed* | Mix: 40% / Sync: 3/16 | Feedback: 40% (~4 repeats). High Pass set to 200Hz to prevent low-end humbucker frequencies from cascading into mud. |
| **Post-FX 2** | Hall Reverb | Mix: 10% / Decay: 1.5s | Mix: 15% / Decay: 2.0s | Spatial Goal: Slightly lower mix than the Telecaster to prevent the naturally thicker 339 tone from overwhelming the DSP space. |

***

### ⚙️ Troubleshooting & Refinement Tree
If the preset sounds inaccurate through your QSC CP12 PA Speaker, follow this strict operational order:
1. **Delay Timing Sounds Messy/Clashing:** The Digital Delay block is set to `Sync: 3/16`. You **must** tap the tempo of the song on the QC footswitch for the rhythm to work. If you are playing "Where the Streets Have No Name," tap ~125 BPM. 
2. **ES-339 Sounds "Broken" or "Farty" on Power Chords:** The vintage Vox circuit sags heavily under humbucker load. Lower the `Bass` parameter on the Amp block to `2.0` and verify the Global Input Gain is padded to `-4.5dB`.
3. **Not Enough Volume (SPL):** Because the `UK C30 Top Boost` lacks a Master Volume, do *not* touch the Brilliant/Normal volume knobs to get louder in the room (this will only add distortion). Instead, swipe down on the Cortex Desktop App and raise the **Lane Output Level** from `0.0dB` to `+3.0dB` or `+5.0dB`.

***

### 🗄️ Session Library (Active Presets)
**2. Preset Name: "Chime & Streets - Tele/339"**
*   **Target:** The Edge (U2) - 1964 Vox AC30TB w/ SDD-3000.
*   **Guitars:** Fender Telecaster (Row 1) & Gibson ES-339 (Row 2).
*   **Physics Goal:** Edge-of-breakup chime with high-mix, bucket-brigade modulated repeats. Humbuckers padded to avoid AC30 sag collapse.
*   **Full Configuration (Telecaster Sc A/B):**
    *   Block 1 (Adaptive Gate): Thresh [-50dB / -55dB], Noise Red [40% / 30%].
    *   Block 2 (EQ-8): HPF [90Hz], Band 1 [+2.5dB @ 200Hz], Band 7 [+3.0dB @ 4500Hz], LPF [10kHz].
    *   Block 3 (Amp - UK C30 Top Boost): Bril Vol [5.5 / 6.5], Norm Vol [3.0], Bass [4.0], Treble [6.5], Tone Cut [4.0]. Lane Out [+2.0dB].
    *   Block 4 (Cab - 212 UK C30 Fawn): Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.0, Dist 2.0"), Mix [A: 0dB, B: -4dB].
    *   Block 5 (Digital Delay): Bypassed/Active, Mix [45%], Time [Sync 3/16], Fdbk [38%], Mod Rate [1.5Hz], Mod Depth [20%], HPF [150Hz].
    *   Block 6 (Hall Reverb): Mix [12% / 18%], Decay [1.5s / 2.0s], Pre-Delay [20ms].
*   **Full Configuration (ES-339 Sc E/F):**
    *   *Input Pad: -4.5dB.*
    *   Block 1 (Adaptive Gate): Thresh [-60dB], Noise Red [20% / 15%].
    *   Block 2 (EQ-8): HPF [120Hz], Band 1 [-3.5dB @ 250Hz], Band 6 [+2.0dB @ 3000Hz], Band 7 [+4.0dB @ 4500Hz], LPF [10kHz].
    *   Block 3 (Amp - UK C30 Top Boost): Bril Vol [3.5 / 4.5], Norm Vol [1.5], Bass [2.0], Treble [7.5], Tone Cut [2.0]. Lane Out [+4.5dB].
    *   Block 4 (Cab - 212 UK C30 Fawn): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 2.0, Dist 3.0"), Mix [A: 0dB, B: -8dB].
    *   Block 5 (Digital Delay): Bypassed/Active, Mix [40%], Time [Sync 3/16], Fdbk [40%], Mod Rate [1.5Hz], Mod Depth [20%], HPF [200Hz].
    *   Block 6 (Hall Reverb): Mix [10% / 15%], Decay [1.5s / 2.0s].