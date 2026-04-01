System Initialization: QC-2 Quad Cortex Systems Engineer
Target Acquired: John Mayer Trio Lead ("Try!" Era)
Analog Pedigree: Two-Rock Custom Reverb / Dumble Steel String Singer / Fender Super Reverb, Klon Centaur, Ibanez TS10.
Physics Strategy: High-headroom, mid-scooped clean platform (Fender Twin architecture mimics the Two-Rock clean channel) pushed by cascading soft-clipping drives (Klon -> TS808) to achieve a smooth, blooming lead tone without harsh preamp distortion.

As requested, I have developed two distinct hardware profiles utilizing the **Split-Bank Matrix** to adapt this Stratocaster-centric tone for both an **ES-339 (Humbuckers)** and a **Telecaster (Single Coils)**.

---

### Phase 1: Main Signal Chain & Base Architecture
*Apply this chain to your QC grid. Parameters marked with `(Right-Click > Assign)` should be assigned to Scenes.*

**Table A: Main Signal Chain**
| Block Category | Model Name | Rhythm (Sc A/E) | Lead (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -55dB | Thresh: -45dB | Keep gate loose on Rhythm for dynamic range. Tighten for Lead to cut TS808 noise. |
| **Pre-FX 1** | Opto Comp | Comp: 3.0, Gain: 5.0 | Comp: 3.0, Gain: 5.0 | Subtle optical leveling. Prevents hard pick attacks from instantly clipping the Dumble-style drives. |
| **Pre-FX 2** | Myth Drive (Klon) | Gain: 2.0, Out: 6.5 | Gain: 2.0, Out: 6.5 | Always-on base color. Adds harmonic excitement and a slight low-mid push. |
| **Pre-FX 3** | Green 808 (TS) | Bypass | OD: 4.5, Bal: 7.0 | Engaged for Lead. Pushes the scooped amp with a massive 720Hz midrange hump. |
| **Amp** | US TWN Vibrato | Vol: 4.5, Treb: 6.0 | Vol: 5.0, Treb: 5.5 | Extremely high headroom. Bass: 4.0, Mid: 3.5. Mimics the scoop of a Two-Rock. |
| **Cab** | 212 US TWN | Mic A: Dyn 57 (Center) | Mic B: Rib 121 (Edge) | Dyn 57 for Rhythm cut; Ribbon 121 mixed higher on Lead to smooth high-end fizz. |
| **Post-FX 1** | Analog Delay | Bypass | Mix: 18%, Time: 320ms | Thickens single notes. High-cut at 1.5kHz so repeats stay out of the way. |
| **Post-FX 2** | Spring Reverb | Mix: 25%, Decay: 2.5s | Mix: 25%, Decay: 2.5s | Classic Mayer drip. LP filter at 4kHz to prevent splashy artifacts. |

---

### Phase 2: Multi-Guitar Target Output & Compensation
John Mayer’s tone relies heavily on the scooped, glassy nature of Stratocaster neck/middle pickups. We must aggressively EQ and gain-stage your two instruments to force them into this sonic footprint.

#### Target 1: Fender Telecaster (Scenes A - D)
**Physics Goal:** A Telecaster neck pickup is typically a bit more muffled and less "hollow" than a Stratocaster, while the bridge is too sharp. We need to add low-end bloom and tame the treble bite.
*   **Input Block:** Set Gain to **+1.5dB**. Tele neck pickups can be slightly under-wound compared to "Texas" style Strat pickups.
*   **EQ-8 Block (Place between Cab and Delay):**
    *   **Band 2 (150Hz - Body):** +2.5dB (Adds the "Strat neck" low-end resonance).
    *   **Band 3 (400Hz - Wood):** -1.5dB (Clears out honk).
    *   **Band 8 (LPF - Twang Control):** Set to **4.5kHz**. This is critical. It chops off the Telecaster ice-pick frequencies when the Green 808 is engaged, making the overdrive sound "smooth" rather than "gritty."

#### Target 2: Gibson ES-339 Humbuckers (Scenes E - H)
**Physics Goal:** Humbuckers will immediately overload the Myth Drive and Amp block, destroying the Two-Rock headroom and turning the tone into muddy classic rock. We must simulate a single-coil frequency response and drop the output voltage.
*   **Input Block:** Set Gain to **-4.5dB**. This prevents the humbuckers from clipping the front of the Opto Comp and keeps the amp strictly in the clean zone during Rhythm playing.
*   **EQ-8 Block (Place between Cab and Delay):**
    *   **Band 2 (200Hz - Mud):** -3.0dB (Removes the humbucker low-end buildup).
    *   **Band 4 (800Hz - Honk):** -4.0dB (Massively scoops the midrange to fake the "hollow" Stratocaster sound).
    *   **Band 6 (3000Hz - Glass):** +2.5dB (Simulates single-coil presence).
    *   **Band 8 (LPF):** Set to **5.5kHz**. Tames the aggressive upper-harmonics generated when humbuckers push the Green 808.

---

### Phase 3: Troubleshooting & Refinement Tree
If the tone isn't reacting properly to your picking hand, follow this strict diagnostic path:

1.  **Too Distorted on Rhythm (ES-339):** Your pickups are very hot. Lower the Input Block Gain to -6.0dB. Do *not* lower the Amp volume, as this will change the tube taper.
2.  **Lead Tone Sounds "Fuzzy" instead of "Smooth":** The Green 808 is clipping too much low-end. Go to the Amp block and reduce the **Bass** parameter from 4.0 to 3.0. In Fender/Two-Rock circuits, bass frequencies distort first. Rolling off bass clears up overdrive.
3.  **Volume Drop on Lead:** If the Green 808 doesn't feel loud enough when engaged, **do not** increase the Drive pedal's level (this just compresses the amp more). Instead, assign the **Lane Output Level** (at the far right of the grid) to boost by +2.0dB on Scenes B and F.

---

### Phase 4: Session Registry Protocol
*Appending to Active Session Memory...*

**Session Library (Active Presets)**

2. Preset Name: "Mayer Trio D-Style"
Target: John Mayer Trio (2005) - "Who Did You Think I Was".
Guitar: Fender Telecaster (Sc A-D) / Gibson ES-339 (Sc E-H).
Physics Goal: High-headroom scooped clean platform cascading into soft-clipping drives for sustain without harsh clipping.
Full Configuration:
Block 1 (Global Gate): Thresh [Rhy: -55dB / Lead: -45dB].
Block 2 (Opto Comp): Comp [3.0], Ratio [4:1], Gain [5.0].
Block 3 (Myth Drive): Gain [2.0], Treb [5.5], Vol [6.5].
Block 4 (Green 808): OD [Rhy: Bypass / Lead: 4.5], Tone [6.0], Balance [7.0].
Block 5 (Amp - US TWN Vibrato): Vol [4.5 / 5.0], Bass [4.0], Mid [3.5], Treble [6.0 / 5.5], Lane Level [Rhy: 0.0dB / Lead: +2.0dB].
Block 6 (Cab - 212 US TWN): Mic A (Dyn 57, Pos 1.0), Mic B (Ribbon 121, Pos 2.5), Mix [Rhy: 70% A / Lead: 50% A].
Block 7 (EQ-8 - Tele): Gain Pad [+1.5dB]. 150Hz [+2.5dB], 400Hz [-1.5dB], LPF [4.5kHz].
Block 8 (EQ-8 - ES339): Gain Pad [-4.5dB]. 200Hz [-3.0dB], 800Hz [-4.0dB], 3000Hz [+2.5dB], LPF [5.5kHz].
Block 9 (Analog Delay): Mix [18%], Time [320ms], Fdbk [25%], HP [250Hz], LP [1500Hz].
Block 10 (Spring Reverb): Mix [25%], Decay [2.5s], HP [150Hz], LP [4000Hz].