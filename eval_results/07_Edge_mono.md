Greetings. I am **QC-2**, your Quad Cortex Systems Engineer. 

To recreate the legendary "Edge Chime" from a 1964 Vox AC30, we must perfectly balance input headroom, harmonic clipping, and precise spatial effects. The Edge’s signature sound relies on the Brilliant/Top Boost circuit pushed just to the edge of breakup, heavily compressed by the EL84 power tubes, with a sharply filtered dotted-eighth (1/8d) delay mimicking the Korg SDD-3000 or TC 2290.

Since you are running direct to a **QSC CP12 Active PA Speaker**, we must aggressively manage the high frequencies (Low Pass Filtering) in the Cab and Delay blocks. An un-filtered AC30 model through a PA speaker will result in digital "ice-pick" harshness rather than musical chime.

Here is the exact protocol to adapt this circuit for both your **Gibson ES-339 (Humbuckers)** and your **Fender Telecaster (Single Coils)**.

---

### 3. Gain Staging & Pickup Compensation Strategy

**Target 1: Fender Telecaster (Single Coil - Vintage Output)**
*   **Physics Goal:** Retain pick attack but add low-mid "weight" so the AC30 doesn't sound thin. The amp needs more drive to hit the edge of breakup.
*   **Global Input Setting:** Gain: `0.0dB`, Impedance: `1M Ohm`.

**Target 2: Gibson ES-339 (Humbuckers - Medium/High Output)**
*   **Physics Goal:** Prevent the hotter humbuckers from fully distorting the AC30 model, which ruins the percussive delay effect. We must increase headroom.
*   **Global Input Setting:** Gain: `-4.5dB` (Crucial pad to lower the pickup output to single-coil levels), Impedance: `1M Ohm`.

---

### 4. Organization Standard (Split-Bank Matrix)
*Assign these via Right-Click > Assign to Scenes.*

*   **Row 1 (Scenes A-D): Telecaster Single Coil Profile**
    *   **Scene A:** Rhythm Chime (Edge-of-breakup, 1/8d Delay active)
    *   **Scene B:** Lead / Solo (+1.5dB Output, Booster engaged, Mix up)
    *   **Scene C:** Dry / Comping (Delays bypassed, pure AC30)
    *   **Scene D:** "Where The Streets..." Ambient (High Mix Delay + Shimmer)
*   **Row 2 (Scenes E-H): ES-339 Humbucker Profile**
    *   **Scene E:** Rhythm Chime (Amp Volume lowered, EQ HPF engaged)
    *   **Scene F:** Lead / Solo 
    *   **Scene G:** Dry / Comping
    *   **Scene H:** Ambient

---

### 5. Table A: Main Signal Chain (Edge Chime Protocol)

*Note: Parameters with multiple values indicate (Telecaster Sc A/B) vs (ES-339 Sc E/F).*

| Block Category | Model Name | Rhythm Settings (Sc A / E) | Lead Settings (Sc B / F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: `-60dB` (Tele) / `-55dB` (339) | Thresh: `-60dB` / `-55dB` | Hotter humbuckers need a slightly higher threshold to ensure crisp silences between staccato delay repeats. |
| **Pre-FX (Comp)** | Optical Comp | Sustain: `35%`, Level: `0.0dB` | Sustain: `45%`, Level: `+1.0dB` | Evens out the pick attack for a "percussive" transient that hits the delay line perfectly. |
| **Pre-FX (EQ)** | Parametric-8 | Band 1 (Peak): `+2.0dB @ 200Hz` (Tele) / `0.0dB` (339) | Same as Rhythm | Chameleon Strategy: Gives the Telecaster humbucker-like low-mid weight; leaves the 339 natural. |
| **Pre-FX (Drive)** | Booster | State: `Bypassed` | State: `Active`, Drive: `2.0`, Level: `6.0` | Simulates the Boss FA-1 FET preamp used for classic U2 solos. Pushes the amp without compressing. |
| **Amp** | UK C30 Top Boost | Vol: `6.0` (Tele) / `4.5` (339)<br>Bass: `4.5`, Treble: `7.0`, Cut: `3.5` | Vol: `6.0` / `4.5`<br>*(Boosted by Pre-FX)* | Vox AC30 Top Boost. Lower Volume for the ES-339 prevents "farting out" the EL84 simulation. Cut rolls off high-end hash. |
| **Cab** | 212 UK C30 Fawn Blue | Mic A: `Dyn 57` (Pos 0.5, Dist 1.0")<br>Mic B: `Ribbon 121` (Pos 1.5, Dist 2.5") | Same as Rhythm | Alnico Blue speakers. 57 provides the "chime" and bite. 121 smooths the top end. **High Cut @ 5.5kHz** to optimize for QSC CP12 PA horn. |
| **Post-FX (Delay)** | Digital Delay | Sync: `1/8d`, Fdbk: `30%`, Mix: `28%`<br>Mod Depth: `15%`, Low Cut: `300Hz`, High Cut: `2.5kHz` | Mix: `35%`, Fdbk: `40%` | Dotted-eighth percussive delay. High/Low cuts ensure the repeats sit *behind* the dry signal and don't clash with the chime. Mod simulates vintage SDD-3000 analog warmth. |
| **Post-FX (Verb)** | Plate Reverb | Mix: `10%`, Decay: `1.2s` | Mix: `15%`, Decay: `1.8s` | Very subtle spatial depth so the mono AC30 doesn't feel entirely dead in the PA. |
| **Output** | Lane 1 Output | Level: `0.0dB` | Level: `+1.5dB` | Pure Master Volume increase for Solo headroom. |

---

### 6. Troubleshooting & Refinement Tree

If you test the ES-339 and report: **"The dotted-eighth delay sounds muddy/cluttered."**
1. **Input Check:** Confirm your Input Block Gain is at `-4.5dB` or lower. If the AC30 is distorting, the delay repeats will compress and mush together.
2. **Delay EQ:** In the Digital Delay block, raise the **Low Cut to 400Hz** and lower the **High Cut to 1.8kHz**. This filters the delay repeats so only the mid-range "tick" is audible.
3. **Amp Tone Cut:** Increase the `Tone Cut` parameter on the UK C30 Top Boost block from `3.5` to `5.0`. Remember, on Vox amps, increasing the Cut *removes* treble.

If you test the Telecaster and report: **"It's too piercing through the QSC CP12."**
1. **Cab LPF:** Drop the High Cut in the Cab block down to `4.5kHz`.
2. **Amp Treble:** Lower the `Treble` parameter on the Amp block in 0.5 increments. *Never* use a post-EQ block to cut highs here; fix it at the amp/cab stage for accurate tube physics.

---

### 8. Session Library (Active Presets)

**1. Preset Name: "Edge Chime 1/8d - Tele/339"**
*   **Target:** The Edge (U2) / 1964 Vox AC30 Top Boost + SDD-3000.
*   **Guitars:** Fender Telecaster (Sc A-D) / Gibson ES-339 (Sc E-H).
*   **Physics Goal:** Retain transient chime, dotted-eighth percussive echo, strict edge-of-breakup EL84 power amp response.
*   **Full Configuration:**
    *   **Block 1 (Global Gate):** Thresh [-60dB (Tele) / -55dB (339)]. 
    *   **Block 2 (Optical Comp):** Sustain [35% (Rhy) / 45% (Lead)], Level [0.0dB / +1.0dB].
    *   **Block 3 (EQ-8):** Band 1 Peak [200Hz, +2.0dB (Tele) / 0.0dB (339)].
    *   **Block 4 (Booster):** Bypassed (Rhy), Active (Lead): Drive 2.0, Level 6.0.
    *   **Block 5 (Amp - UK C30 Top Boost):** Vol [6.0 (Tele) / 4.5 (339)], Bass [4.5], Treble [7.0], Tone Cut [3.5].
    *   **Block 6 (Cab - 212 UK C30 Fawn Blue):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 2.5"), Mix [0dB / -2dB], High Cut [5.5kHz].
    *   **Block 7 (Digital Delay):** Sync [1/8d], Fdbk [30% / 40%], Mix [28% / 35%], Mod Depth [15%], Low Cut [300Hz], High Cut [2500Hz].
    *   **Block 8 (Plate Reverb):** Mix [10% / 15%], Decay [1.2s / 1.8s].
    *   **Lane 1 Output:** Level [0.0dB (Rhy) / +1.5dB (Lead)].