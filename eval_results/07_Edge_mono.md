### QC-2 Systems Engineer: Preset Build Report
**Target:** The Edge (U2) - "Where the Streets Have No Name"
**Hardware:** Cortex Control (Mac) -> QSC CP12 (Active FRFR)
**Physics Goal:** Achieve the iconic, percussive "chime" and extreme rhythmic clarity of a Vox AC30TB pushed to the edge of breakup. The delay block must feature heavy modulation and act almost as a secondary instrument, blending seamlessly without muddying the fundamental pick attack. 

*Pre-flight Check:* The tempo for this track is exactly **108 BPM**. Set your global Quad Cortex tempo to 108 BPM and sync the delay block to "3/16" (Dotted 8th note) to guarantee the correct rhythmic cascade.

---

### Phase 1: Multi-Guitar Compensation Strategy

To make this percussive, bright tone work on both a Telecaster and a dark, hot ES-339, we must deploy the "Chameleon" EQ and manipulate the Input Gain. 

**Guitar 1: Fender Telecaster (Single Coil - Bridge Pickup)**
*   **Input Block:** 0.0dB. 
*   **EQ Strategy:** The Telecaster has the requisite bite but lacks the low-mid girth of The Edge's Stratocaster (often used in position 2 or bridge). We will use the Parametric-8 to add a slight body bump.
*   **Scene Assignment:** Bank Row 1 (Scenes A - D)

**Guitar 2: Gibson ES-339 (Humbuckers - Middle Position)**
*   **Input Block:** **-4.5dB**. *Crucial step.* Humbuckers will immediately overload the AC30 model's Brilliant channel, causing a fuzzy, unmusical clipping instead of chime.
*   **EQ Strategy:** Aggressive low-frequency reduction to prevent the delay repeats from turning into a muddy wash. Treble emphasis to simulate single-coil scrape.
*   **Scene Assignment:** Bank Row 2 (Scenes E - H)

---

### Phase 2: Main Signal Chain (Split-Bank Matrix)

*Assign Scene changes by Right-Clicking the parameter in Cortex Control.*

| Block Category | Model Name | Single Coil Settings (Scenes A/B) | Humbucker Settings (Scenes E/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input Gate** | *Global* | Thresh: -65dB | Thresh: -58dB | Humbuckers need higher threshold due to increased baseline output; Tele needs lower to let the chime ring. |
| **Pre-FX (Comp)** | `Jewel Comp` | Comp: 3.5, Vol: 5.0 | Comp: 2.0, Vol: 4.0 | Optical compression evens out the percussive attack, ensuring the delay triggers uniformly. |
| **Pre-FX (EQ)** | `Parametric-8` | Band 2 (200Hz): +1.5dB<br>Band 6 (4kHz): +1.0dB | HPF: 120Hz<br>Band 3 (400Hz): -3.0dB<br>Band 6 (5kHz): +3.5dB | *Chameleon Block.* Thinning out the ES-339 humbucker to simulate single-coil transient response. |
| **Amp** | `UK C30 Top Boost` | Vol: 4.5<br>Bass: 3.5<br>Treble: 7.5<br>Tone Cut: 3.0 | Vol: 4.0<br>Bass: 2.5<br>Treble: 8.5<br>Tone Cut: 2.0 | Vintage Vox has no Mid or Master Vol. Treble handles saturation. Lowering bass for ES-339 prevents AC30 sag/farting. |
| **Cab** | `2x12 UK C30` | Mic A: Dyn 57 (Pos 1.5, Dist 1")<br>Mic B: Rib 121 (Pos 3, Dist 2") | Mic A: Dyn 57 (Pos 1.0, Dist 1")<br>Mic B: Cond 414 (Pos 2, Dist 1.5") | Alnico Blues. 57 provides the bite. Swapped ribbon for a condenser on the ES-339 to retain upper harmonics. |
| **Post-FX (Delay)** | `Digital Delay` | Sync: 3/16 (Dotted 8th)<br>Mix: 45%<br>Mod Depth: 40% | Sync: 3/16 (Dotted 8th)<br>Mix: 40%<br>Mod Depth: 40% | Simulates the Korg SDD-3000. Dotted 8th is non-negotiable. Mod Depth mimics tape/analog chorusing. |
| **Post-FX (Verb)** | `Hall Reverb` | Mix: 12%<br>Decay: 2.5s | Mix: 10%<br>Decay: 2.5s | Simulates stadium ambiance. High pass at 200Hz prevents low-end buildup in the QSC CP12. |

---

### Phase 3: Scene Organization Standard

**Row 1: Fender Telecaster**
*   **Scene A (Rhythm):** Standard Dotted 8th. Delay Mix 45%. Output Level 0.0dB.
*   **Scene B (Lead/Fills):** Amp Vol increased to 5.5. Delay Mix to 50%. Output Level +1.5dB.
*   **Scene C (Dry):** Delay block bypassed. Used for tuning or tight, dry rhythm. 
*   **Scene D (Ambient):** Reverb mix up to 35%, Delay Feedback increased to 60%.

**Row 2: Gibson ES-339**
*   **Scene E (Rhythm):** Standard Dotted 8th. Delay Mix 40%. Output Level 0.0dB.
*   **Scene F (Lead/Fills):** Amp Vol increased to 5.0. Output Level +1.5dB. 
*   **Scene G (Dry):** Delay bypassed.
*   **Scene H (Ambient):** Reverb mix up to 30%, Delay Feedback increased to 55%.

---

### Troubleshooting & Refinement Tree
*   **If the repeats sound too messy/clashing:** Check your tap tempo. If the track is 108 BPM and you are tapped at 112 BPM, the dotted 8th algorithm will phase out of rhythm with the band.
*   **If the ES-339 sounds "farty" or "broken":** Lower the Amp Block `Bass` parameter by another 1.0. The Top Boost circuit has immense low-end sag when hit with humbuckers.
*   **If the tone lacks "scrape":** The Edge uses a Herdim pick held backwards (hitting the strings with the dimpled grip). If using a standard pick, increase the EQ-8 Band 7 (6kHz) by +2dB to simulate this plastic scraping effect.

---

### Session Registry Protocol

**Session Library (Active Presets)**

1. **Preset Name:** "Streets Dt8th - Tele/339"
   **Target:** The Edge / U2 (1987)
   **Guitar:** Dual Target (ES-339 Humbuckers / Telecaster Single Coils)
   **Physics Goal:** Extreme percussive clarity, heavily modulated Dotted-8th delay, AC30 edge-of-breakup saturation.
   **Full Configuration:**
   *   **Block 1 (Input/Gate):** Input Pad [Tele: 0.0dB / ES339: -4.5dB], Global Thresh [-65dB / -58dB].
   *   **Block 2 (Jewel Comp):** Comp [3.5 / 2.0], Attack [Fast], Mix [100%], Vol [5.0 / 4.0].
   *   **Block 3 (Parametric-8):** [Tele: Band 2 +1.5dB @ 200Hz, Band 6 +1.0dB @ 4kHz] | [ES339: HPF 120Hz, Band 3 -3.0dB @ 400Hz, Band 6 +3.5dB @ 5kHz].
   *   **Block 4 (Amp - UK C30 Top Boost):** Vol [4.5 / 4.0], Bass [3.5 / 2.5], Treble [7.5 / 8.5], Tone Cut [3.0 / 2.0]. Output Level adjusted for parity.
   *   **Block 5 (Cab - 2x12 UK C30):** [Tele: 57 (Pos 1.5) + 121 (Pos 3)] | [ES339: 57 (Pos 1.0) + 414 (Pos 2)]. Mix 50/50.
   *   **Block 6 (Digital Delay):** Sync [3/16], Tempo [108 BPM], Mix [45% / 40%], Fdbk [35%], Mod Rate [1.5Hz], Mod Depth [40%], Low Cut [200Hz], High Cut [3000Hz].
   *   **Block 7 (Hall Reverb):** Mix [12% / 10%], Decay [2.5s], Pre-Delay [20ms], Low Cut [200Hz].