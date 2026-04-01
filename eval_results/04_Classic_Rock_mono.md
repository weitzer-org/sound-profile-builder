Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To capture that definitive 70s classic rock crunch (think AC/DC, Led Zeppelin, or early Aerosmith), we must use the **"Brit Plexi 100 Jumped"** model. The authentic vintage Plexi circuit does not feature a Master Volume; overdrive is generated purely by pushing the phase inverter and power tubes via the channel volumes. To manage your loudness through the QSC CP12 PA speaker without altering the overdrive characteristics, we will rely strictly on the **Lane Output Level**.

Per protocol, I have engineered a dual-profile preset to accommodate both the requested Humbuckers and a Single Coil alternative. 

*System Trigger Check:* Before locking in these parameters, **are your ES-339 humbuckers Vintage Output (PAF style), Medium, or High Output?** The settings below assume Vintage/Medium PAFs. If they are High Output, please reduce the Input Block Gain to -3.0dB to prevent digital clipping.

---

### Row 2 (Scenes E-H): Gibson ES-339 (Humbucker Profile)
**Focus:** Controlling low-mid "mud" inherent to humbuckers while maintaining bite and string definition.

**Scene Layout:**
*   **Scene E (Rhythm):** Pure jumped amp crunch. Tightened low-end.
*   **Scene F (Lead):** Myth Drive engaged for a mid-hump push; +1.5dB output bump; Delay engaged.
*   **Scene G (Dry):** Rhythm tone, no spatial FX.
*   **Scene H (Ambient):** Volume rolled back (via Input Gain -4dB simulate volume pot roll-off), heavier reverb.

#### Table A: Main Signal Chain (ES-339 Humbuckers)
*Note: Right-Click > Assign to parameters showing dual values.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -62dB | Thresh: -58dB | Humbuckers have higher signal-to-noise. Slightly tighter gate on lead to suppress overdrive floor noise. |
| **Pre-FX** | Myth Drive | Bypass | Active / Gain: 2.0, Tone: 6.5, Vol: 8.0 | Pushes the amp's front end with upper-mids to cut through the mix. Tightens bass. |
| **Amp** | Brit Plexi 100 Jumped | Vol Norm: 3.5, Vol Bright: 6.0, Bass: 3.0, Mid: 6.0, Treb: 5.5, Pres: 6.0 | Vol Norm: 4.0, Vol Bright: 6.5, Bass: 3.0, Mid: 6.5, Treb: 5.5, Pres: 6.0 | Blending Normal (warmth) and Bright (punch). Lower Bass prevents "farty" power amp sag. No Master Vol. |
| **EQ** | Parametric-8 | HPF: 95Hz, Band 2: -1.5dB @ 300Hz, LPF: 6500Hz | HPF: 95Hz, Band 2: -1.5dB @ 300Hz, LPF: 6500Hz | HPF keeps the QSC CP12 from getting boomy. The 300Hz cut removes humbucker boxiness. |
| **Cab** | 412 Brit 35A (Greenbacks) | Mic 1 (Dyn 57): Pos 1.2, Dist 1.0". Mic 2 (Rib 121): Pos 2.0, Dist 2.5". Mix: 0dB / -3dB | [Same] | 57 adds aggressive crunch; 121 Ribbon restores the warm analog "woodiness". |
| **Post-FX** | Tape Delay | Bypass | Active / Mix: 18%, Time: 350ms, Fdbk: 25% | Classic tape saturation simulating a vintage Echoplex for lead trails. |
| **Post-FX** | Room Reverb | Active / Mix: 15%, Decay: 1.2s | Active / Mix: 18%, Decay: 1.4s | Simulates a 1970s wood-paneled tracking room. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB | Pure volume lift for solos without altering tube saturation. |

---

### Row 1 (Scenes A-D): Fender Telecaster (Single Coil Profile)
**Focus:** "The Chameleon Strategy". Adding physical weight and body to the single coils while taming the ice-pick transients. 

**Scene Layout:**
*   **Scene A (Rhythm):** Amp volume pushed harder to compensate for lower pickup output.
*   **Scene B (Lead):** Drive engaged; EQ tweaked for warmth; +1.5dB output bump.
*   **Scene C (Dry):** Rhythm tone, no spatial FX.
*   **Scene D (Ambient):** Cleaned up volume roll-off, wet FX.

#### Table B: Main Signal Chain (Telecaster Single Coils)
*Note: Right-Click > Assign to parameters showing dual values.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -68dB, Gain: +2.5dB | Thresh: -65dB, Gain: +2.5dB | +2.5dB gain added physically compensates for lower pickup output hitting the amp block. Lower gate threshold for weaker signal. |
| **Pre-FX** | Myth Drive | Bypass | Active / Gain: 3.0, Tone: 4.5, Vol: 7.5 | Tone rolled back to prevent Telecaster bridge pickup from sounding harsh when boosted. |
| **Amp** | Brit Plexi 100 Jumped | Vol Norm: 5.5, Vol Bright: 5.5, Bass: 4.5, Mid: 5.0, Treb: 4.0, Pres: 4.5 | Vol Norm: 6.0, Vol Bright: 6.0, Bass: 4.5, Mid: 5.5, Treb: 4.0, Pres: 4.5 | Increased Normal Volume for thickness. Treble/Presence reduced to combat single-coil "ice-pick". |
| **EQ** | Parametric-8 | HPF: 80Hz, Band 1: +3.0dB @ 200Hz, LPF: 5000Hz | HPF: 80Hz, Band 1: +3.0dB @ 200Hz, LPF: 5000Hz | **Chameleon Trigger:** Band 1 at 200Hz adds critical "body" to singles. LPF at 5kHz drastically tames pick attack. |
| **Cab** | 412 Brit 35A (Greenbacks) | Mic 1 (Dyn 57): Pos 2.0, Dist 1.0". Mic 2 (Rib 121): Pos 1.5, Dist 1.5". Mix: -2dB / 0dB | [Same] | Shifted the mix to favor the Ribbon mic (121) to thicken the Telecaster. Moved 57 further off-center. |
| **Post-FX** | Tape Delay | Bypass | Active / Mix: 20%, Time: 350ms, Fdbk: 25% | Tape age/LPF inside delay naturally darkens the repeats. |
| **Post-FX** | Room Reverb | Active / Mix: 15%, Decay: 1.2s | Active / Mix: 18%, Decay: 1.4s | 70s room ambiance. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB | Pure volume lift for solos. |

---

### Troubleshooting & Refinement Tree (For QSC CP12)
*   **If the low-end "farts out" on palm mutes:** The QSC CP12 pushes a lot of low frequencies. Lower the `Bass` on the Amp block to 2.0, or raise the `HPF` on the EQ block to 110Hz. Vintage Plexi circuits inherently sag when bass frequencies are pushed too hard.
*   **If it's too distorted (The Headroom Rule):** Reduce `Vol Bright` and `Vol Normal` on the amp by 1.0 increment each, and raise the `Lane Output Level` by +2.0dB to restore the volume. 

***

### Session Library (Active Presets)

**2. Preset Name: "70s Plexi Punch"**
*   **Target:** 1970s Classic Rock (AC/DC, Led Zeppelin).
*   **Guitars:** Dual configuration (ES-339 Humbuckers / Telecaster Single Coils).
*   **Physics Goal:** Vintage jumped-channel overdrive utilizing preamp tube saturation and FRFR frequency compensation.
*   **Full Configuration:**
    *   **Block 1 (Myth Drive):** Bypass [Sc A/E], Active [Sc B/F]. Gain [HB: 2.0 / SC: 3.0], Tone [HB: 6.5 / SC: 4.5], Vol [HB: 8.0 / SC: 7.5].
    *   **Block 2 (Amp - Brit Plexi 100 Jumped):** Vol Norm [HB Rhy: 3.5, Ld: 4.0 / SC Rhy: 5.5, Ld: 6.0], Vol Bright [HB Rhy: 6.0, Ld: 6.5 / SC Rhy: 5.5, Ld: 6.0], Bass [HB: 3.0 / SC: 4.5], Mid [HB: 6.0 / SC: 5.0], Treb [HB: 5.5 / SC: 4.0], Pres [HB: 6.0 / SC: 4.5].
    *   **Block 3 (EQ-8):** HPF [HB: 95Hz / SC: 80Hz], Band 1 Low-Shelf [SC: +3.0dB @ 200Hz], Band 2 [HB: -1.5dB @ 300Hz], LPF [HB: 6500Hz / SC: 5000Hz].
    *   **Block 4 (Cab - 412 Brit 35A):** Mic 1 (Dyn 57), Mic 2 (Ribbon 121). Position/Mix shifted per guitar output (see Tables A & B).
    *   **Block 5 (Tape Delay):** Mix [18-20%], Time [350ms], Fdbk [25%]. Bypass on Rhythm.
    *   **Block 6 (Room Reverb):** Mix [15%], Decay [1.2s].
    *   **Lane Output:** Level [Rhythm: 0.0dB, Lead: +1.5dB].