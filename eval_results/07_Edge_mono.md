As "QC-2", your Quad Cortex Systems Engineer, I have mapped the analog topology of The Edge’s 1964 Vox AC30 Top Boost, SDD-3000 Delay, and SD-1 Overdrive into verified CorOS components. 

To achieve this iconic "chime" through your QSC CP12, we must strictly manage headroom. The AC30 is a non-master volume circuit; it relies entirely on preamp gain interaction. Furthermore, a dotted-eighth delay requires extreme transient clarity so the repeats do not mask your dry signal.

Here is your dual-guitar deployment strategy following the Split-Bank Matrix protocol.

### Multi-Guitar Target Output & Split-Bank Matrix
**Row 1 (Scenes A-D): Fender Telecaster (Single Coil)**
*   **A: Tele Rhythm** (Edge-of-breakup, core dotted-eighth delay)
*   **B: Tele Lead** (+1.5dB, Yellow Overdrive engaged, delay mix increased)
*   **C: Tele Dry** (Delay bypassed, pure AC30 chime)
*   **D: Tele Ambient** (Delay feedback increased, Shimmer Reverb engaged)

**Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers)**
*   **E: 339 Rhythm** (Input padded -4.0dB, EQ sculpted for single-coil clarity)
*   **F: 339 Lead** (+1.5dB output, Yellow Overdrive engaged)
*   **G: 339 Dry** (Pure AC30 chunk)
*   **H: 339 Ambient** (Washes and pads)

---

### Table A: Main Signal Chain
*Note: Scene-Specific changes are marked with `(Right-Click > Assign)`. Amp & Output blocks handle SPL vs. Drive physics.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Red: 35% <br>Thresh: -65dB | Red: 35% <br>Thresh: -65dB | Adaptive block avoids chopping off the delicate delay trails compared to Global Gate. |
| **Pre-FX 1** | Parametric-8 EQ | *(See Multi-Guitar Specifics below)* | *(See Multi-Guitar Specifics below)* | Compensates for the physical differences between Humbucker and Single Coil magnetic fields. |
| **Pre-FX 2** | Yellow Overdrive | **Bypassed** | **Engaged** <br>Drive: 2.5 <br>Level: 7.0 | Simulates his Boss SD-1. Low drive, high level slams the AC30 input for harmonic saturation. |
| **Amp** | UK C30 Top Boost | **Vol:** *(See below)* <br>Bass: 3.5 <br>Treble: 7.0 <br>Cut: 4.0 | **Vol:** *(See below)* <br>Bass: 3.5 <br>Treble: 7.5 <br>Cut: 3.5 | *Non-Master Amp Rule:* Volume controls drive, not SPL. High Treble / Low Cut simulates the Herdim pick scrape. |
| **Cab** | 212 UK C30 Blue | Mic 1: Dyn 57 (Pos 0.5) <br>Mic 2: Rib 121 (Pos 2.0) | Mic 1: Dyn 57 (Pos 0.5) <br>Mic 2: Rib 121 (Pos 2.0) | Celestion Alnico Blues are mandatory for the bell-like upper-mid frequency response. |
| **Post-FX 1** | Digital Delay | **Mix: 42%** <br>Time: Sync 1/8d <br>Fdbk: 30% <br>Mod Depth: 25% | **Mix: 48%** <br>Time: Sync 1/8d <br>Fdbk: 40% <br>Mod Depth: 25% | 1/8d Sync maps to SDD-3000. Modulation adds the characteristic chorusing to the repeats. HPF set to 200Hz to keep lows clear. |
| **Post-FX 2** | Plate Reverb | Mix: 15% <br>Decay: 1.5s <br>Pre-Delay: 20ms | Mix: 20% <br>Decay: 1.8s <br>Pre-Delay: 20ms | Slight plate reverb adds 3D space for the QSC CP12 without washing out the dotted-eighth transients. |

---

### Multi-Guitar Target Execution Instructions

**Target 1: Fender Telecaster (Scenes A-D)**
*   **Physics Goal:** Thicken the vintage single-coil output while retaining the aggressive high-end snap.
*   **Input Block Logic:** Gain set to **0.0dB**.
*   **Pre-FX EQ-8 Settings:**
    *   HPF: 80Hz.
    *   Band 2 (Body Boost): +2.5dB at 250Hz (adds weight to the Telecaster bridge pickup).
    *   Band 8 (LPF): 6500Hz (tames the ice-pick frequencies inherent to Tele bridge pickups hitting an AC30).
*   **Amp Gain Staging:** Normal Vol: 3.0 | Brilliant Vol: 5.5.
*   **SPL Output:** Lane Output Level at **0.0dB** (Rhythm) / **+1.5dB** (Lead).

**Target 2: Gibson ES-339 (Scenes E-H)**
*   **Physics Goal:** Prevent the hotter humbuckers from compressing the AC30 front-end and losing the dotted-eighth chime.
*   **Input Block Logic:** Gain set to **-4.0dB**. *(Crucial: This simulates rolling off the volume knob, keeping the amp clean/edge-of-breakup).*
*   **Pre-FX EQ-8 Settings:**
    *   HPF: 120Hz (Cuts humbucker mud before it hits the amp).
    *   Band 3 (Mud Cut): -3.0dB at 400Hz.
    *   Band 6 (Chime Boost): +2.0dB at 3500Hz (simulates single-coil articulation).
*   **Amp Gain Staging:** Normal Vol: 2.0 | Brilliant Vol: 4.0. *(Lower volume needed due to humbucker output).*
*   **SPL Output:** Lane Output Level raised to **+3.5dB** (Rhythm) / **+5.0dB** (Lead) to compensate for the Input Block reduction.

---

### Troubleshooting & Refinement Tree
*   **If the repeats sound too muddy/clashing:** Go to the Digital Delay block. Raise the internal High-Pass Filter (HPF) to 300Hz and lower the Low-Pass Filter (LPF) to 3000Hz. This mimics analog tape degradation and separates the repeats from your dry picking.
*   **If the ES-339 sounds "Too Fuzzy":** Do not lower the Treble. Lower the Input Block Gain to -6.0dB, then raise your Lane Output Level to compensate. AC30 Top Boost models fuzz out quickly if hit with too much input signal.
*   **If the tone lacks "punch" through the QSC CP12:** The CP12 is an active PA speaker, meaning it's highly linear. If it sounds sterile, increase the Ribbon 121 mic mix on the Cab block by +2.0dB to add analog warmth.

---

### Session Library (Active Presets)

**2. Preset Name: "Streets Chime - AC30"**
*   **Target:** The Edge (U2) / 1964 Vox AC30 Top Boost w/ SDD-3000 Delay.
*   **Guitars:** Telecaster (Single Coil) / ES-339 (Humbucker).
*   **Physics Goal:** Extreme transient clarity for dotted-eighth rhythmic playing, edge-of-breakup harmonic saturation without collapsing into fuzz.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [35%], Thresh [-65dB], Decay [150ms].
    *   **Block 2 (EQ-8 - Tele):** HPF [80Hz], Band 2 [+2.5dB @ 250Hz], LPF [6500Hz].
    *   **Block 2 (EQ-8 - ES339):** HPF [120Hz], Band 3 [-3.0dB @ 400Hz], Band 6 [+2.0dB @ 3500Hz].
    *   **Block 3 (Yellow Overdrive):** Drive [2.5], Tone [6.0], Level [7.0] *(Lead Scenes Only)*.
    *   **Block 4 (Amp - UK C30 Top Boost):** Normal Vol [Tele: 3.0 / 339: 2.0], Brilliant Vol [Tele: 5.5 / 339: 4.0], Bass [3.5], Treble [7.0 / 7.5], Cut [4.0 / 3.5].
    *   **Block 5 (Cab - 212 UK C30 Blue):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 2.0, Dist 2.0"), Mix [A: 0dB, B: -2dB].
    *   **Block 6 (Digital Delay):** Mix [Rhy: 42% / Lead: 48%], Time [Sync 1/8d], Fdbk [30% / 40%], Mod Depth [25%], Mod Rate [1.5Hz], HPF [200Hz], LPF [4000Hz].
    *   **Block 7 (Plate Reverb):** Mix [15% / 20%], Decay [1.5s / 1.8s], Pre-Delay [20ms], HP [150Hz], LP [4000Hz].
    *   **Output Levels (Tele):** Input [0.0dB] -> Lane Out Rhy [0.0dB] / Lane Out Lead [+1.5dB].
    *   **Output Levels (ES339):** Input [-4.0dB] -> Lane Out Rhy [+3.5dB] / Lane Out Lead [+5.0dB].