Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To capture B.B. King’s "Lucille" tone using a high-headroom American Twin Reverb platform, we must adhere strictly to the **Headroom Rule**. B.B. King’s tone relies on an absolutely clean, pristine signal with immense dynamic range. All sustain comes from volume (SPL), picking dynamics, and subtle compression—*not* preamp saturation. 

Because the "US TWN Vibrato" (Neural DSP's exact model for the Fender Twin Reverb) is a non-master volume circuit, pushing the Amp Volume will introduce overdrive. Therefore, we will keep the Amp Volume low (below the breakup threshold) and use the **Lane Output Level** to achieve your target room volume through your QSC CP12.

Per your requirement, I have structured this preset using the **Split-Bank Matrix** to provide perfectly gain-staged profiles for both your Gibson ES-339 and Fender Telecaster within a single preset.

---

### **Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers) Profile**
**Target:** The authentic "Lucille" configuration. High output humbuckers naturally push amps into overdrive, so we must heavily pad the input gain to preserve the Twin Reverb's high-headroom clean platform. We will utilize a Parametric-8 EQ to simulate Gibson's "Varitone" circuit (Position 2/3), which scoops out low-mid mud to emphasize a vocal, honking midrange.

**Guitar Instructions:** Select both pickups (Middle Position) or Bridge pickup. Roll your guitar's Tone knob back to 6 or 7.

#### **Table A: ES-339 Signal Chain (Scenes E-H)**
*Note: Right-Click > Assign parameters for Rhythm/Lead scene changes.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -62dB, Gain: -4.0dB | Thresh: -62dB, Gain: -4.0dB | **Pad applied (-4dB)**. Prevents humbucker transients from clipping the Twin's input stage. |
| **Pre-FX** | Optical Comp | Comp: 40%, Gain: 2.0dB | Comp: 55%, Gain: 3.5dB | Transparent LA-2A style. Adds the sustain B.B. King gets from sheer stage volume. |
| **Amp** | US TWN Vibrato | Vol: 3.5, Bass: 3.0, Mid: 6.5, Treb: 6.0 | Vol: 3.5, Bass: 3.0, Mid: 7.5, Treb: 6.5 | Non-Master Vol circuit. Bass is kept low to prevent humbucker "farting/sag". Mids pushed for cut. |
| **Cab** | 212 US TWN C25 | Mic A: Dyn 57 (Dist 1.0"), Mic B: Rib 121 (Dist 2.5") | Mix: A -2dB, B 0dB (Unchanged) | Dynamic 57 captures pick attack; Ribbon 121 restores smooth warmth to the high-end. |
| **Post-FX** | Parametric-8 | Cut 400Hz (-3dB), Boost 1.5kHz (+2dB) | Boost 1.5kHz (+3.5dB), Level: +1.5dB | Simulates Varitone circuit. Removes low-mid mud, pushes vocal "honk". Lead scene boosts Lane Level. |
| **Post-FX** | Spring | Mix: 15%, Decay: 2.0s | Mix: 20%, Decay: 2.5s | Classic Fender spring tank. Kept subtle to maintain note clarity. |

---

### **Row 1 (Scenes A-D): Fender Telecaster (Single Coils) Profile**
**Target:** Forcing a Telecaster to mimic Lucille requires the **Chameleon Strategy**. Telecasters lack the low-end weight of a semi-hollow ES-339 and have a sharp, treble-heavy bridge pickup ("ice-pick"). We will increase the Input Block gain to match the ES-339's output, boost the 200Hz range to simulate a semi-hollow body, and aggressively low-pass the high frequencies.

**Guitar Instructions:** Select the Neck pickup, or Middle position. Tone knob at 8.

#### **Table B: Telecaster Signal Chain (Scenes A-D)**
*Note: Right-Click > Assign parameters for Rhythm/Lead scene changes.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -50dB, Gain: +1.5dB | Thresh: -50dB, Gain: +1.5dB | **Boost applied (+1.5dB)** to compensate for weak single coils. Higher gate threshold for 60-cycle hum. |
| **Pre-FX** | Optical Comp | Comp: 45%, Gain: 3.0dB | Comp: 60%, Gain: 4.5dB | Single coils decay faster; higher compression required for B.B. style vibrato sustain. |
| **Amp** | US TWN Vibrato | Vol: 4.5, Bass: 4.5, Mid: 7.0, Treb: 5.0 | Vol: 4.5, Bass: 4.5, Mid: 8.0, Treb: 5.0 | Amp Volume raised slightly. Treble rolled back to combat single-coil brightness. |
| **Cab** | 212 US TWN C25 | Mic A: Dyn 57 (Dist 1.5"), Mic B: Rib 121 (Dist 1.0") | Mix: A -4dB, B 0dB (Unchanged) | 57 pushed back slightly. Ribbon mic emphasized heavily to thicken the Telecaster tone. |
| **Post-FX** | Parametric-8 | Low Shelf 200Hz (+4dB), LPF 4.5kHz | Mid 1.5kHz (+2.5dB), Level: +1.5dB | **Chameleon EQ**: Adds semi-hollow "weight" (200Hz) and physically chops off the Tele "ice-pick" (LPF). |
| **Post-FX** | Spring | Mix: 18%, Decay: 2.0s | Mix: 22%, Decay: 2.5s | Reverb mix slightly higher to add perceived depth to the solid-body guitar. |

---

### **Troubleshooting & Refinement Tree**
If testing through your QSC CP12 yields unsatisfactory results, execute the following in order:
1. **Tone is Breaking Up / "Too Fuzzy" on chords:** Your specific pickups may be exceptionally hot. Lower the Input Block Gain by an additional -2.0dB. *Do not touch the Amp Volume.*
2. **Tone is too quiet compared to other presets:** High Headroom presets often feel quiet because they lack the RMS boost of compression/distortion. **Fix:** Raise the overall **Lane Output Level** on the far right of the QC grid by +3.0dB to +5.0dB until it matches your other presets.
3. **Too "Thin" / Lacking Bass (Especially on Telecaster):** Do not raise the Amp's Bass knob (this will cause tube sag). Instead, go to the Parametric-8 EQ and increase the Band 1 Low Shelf (+200Hz) by another +2.0dB.

---

### **Session Library (Active Presets)**

**2. Preset Name:** "Lucille's Twin - Dual Rig"
**Target:** B.B. King / High Headroom Clean.
**Guitar:** Gibson ES-339 (Humbuckers) & Fender Telecaster (Single Coils).
**Physics Goal:** Absolute clean platform, vocal midrange push, volume-dependent sustain, Varitone simulation via EQ.
**Full Configuration:**
*   **Block 1 (Global Input):** ES-339 [Thresh: -62dB, Gain: -4.0dB] / Telecaster [Thresh: -50dB, Gain: +1.5dB].
*   **Block 2 (Optical Comp):** Peak Red [ES: 40/55%, Tele: 45/60%], Gain [ES: 2.0/3.5dB, Tele: 3.0/4.5dB].
*   **Block 3 (Amp - US TWN Vibrato):** Vol [ES: 3.5, Tele: 4.5], Bass [ES: 3.0, Tele: 4.5], Mid [ES: 6.5/7.5, Tele: 7.0/8.0], Treb [ES: 6.0/6.5, Tele: 5.0]. Bright Switch: OFF. Master Volume: N/A.
*   **Block 4 (Cab - 212 US TWN C25):** Mic A (Dyn 57, Pos 1.0, Dist [ES: 1.0", Tele: 1.5"]), Mic B (Ribbon 121, Pos 1.5, Dist [ES: 2.5", Tele: 1.0"]).
*   **Block 5 (EQ-8):** ES-339 [Cut 400Hz -3dB, Peak 1500Hz +2/+3.5dB]. Telecaster [Low Shelf 200Hz +4dB, LPF 4500Hz, Peak 1500Hz +2.5dB (Lead)]. Output Level [Lead Scene +1.5dB].
*   **Block 6 (Spring Reverb):** Mix [ES: 15/20%, Tele: 18/22%], Decay [2.0s / 2.5s], Tone [5.0].