### QC-2 Systems Engineer: David Gilmour Tone Adaptation
**Target Analogs:** Hiwatt Custom 100, Ram's Head Big Muff, WEM 4x12, Plate Reverb.
**CorOS Verified Models:** `Hiw103` (Amp), `Chief Fuzz` (Pre-FX), `412 Hiw Fane` (Cab - Simulates Fane Crescendo speakers), `Plate Reverb` (Post-FX). 
**Output Profile:** Full-range optimization for QSC CP12 Active PA.

**Physics & Gain Staging Protocol:**
The David Gilmour signal path relies on slamming a massive-headroom, ultra-clean amplifier (`Hiw103`) with an aggressively scooped, high-gain fuzz (`Chief Fuzz`). If the amp breaks up, the fuzz turns to muddy static. We must keep the Amp's Preamp Volumes low and Master Volume high. Furthermore, because the Big Muff circuit scoops the midrange, a Single Coil (Telecaster) will sound thin and piercing without EQ compensation, whereas a Humbucker (ES-339) will overload the low-frequencies of the fuzz, causing "farting" or sagging.

Below are the precisely separated, dedicated configurations for your Fender Telecaster (Row 1) and Gibson ES-339 (Row 2). 

---

### Row 1: Single Coil Profile (Fender Telecaster)
**Goal:** Tame the ice-pick treble of the Tele bridge pickup, add low-mid "body" to survive the Big Muff mid-scoop, and maintain endless sustain.

**Table A: Main Signal Chain (Telecaster)**
*Use Scenes A (Rhythm) and B (Lead). Assign parameters via Right-Click > Assign to Scene.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global In (Circle 1) | Thresh: -60dB, Gain: 0.0dB | Thresh: -65dB, Gain: +1.5dB | Tele output needs a slight push for lead sustain; looser gate prevents choking the fuzz tail. |
| **Pre-FX 1** | Parametric-8 EQ | Band 2: +3.0dB @ 250Hz | Band 2: +3.0dB @ 250Hz | Adds "body" and weight to the single coil before it hits the fuzz to simulate Stratocaster neck-pickup thickness. |
| **Pre-FX 2** | Chief Fuzz | Bypass: ON (Clean) | Bypass: OFF (Sustain: 7.5, Tone: 4.5, Vol: 6.0) | High sustain, Tone rolled back below noon to tame the Telecaster's high-end pick attack. |
| **Amp** | Hiw103 | Norm Vol: 3.0, Brill: 2.0 | Norm Vol: 3.0, Brill: 2.0 | Master: 8.5. Tremendous headroom. Amp stays physically clean to allow Fuzz to act as the sole clipping stage. |
| **Cab** | 412 Hiw Fane | Mic A: Dyn 57 (Center) | Mic A: Dyn 57 (Center) | Fane speakers are inherently bright. Dyn 57 captures the bite. |
| **Post-FX 1** | Tape Delay | Bypass: ON (Or Mix 10%) | Bypass: OFF (Mix: 25%, Time: 440ms, Fdbk: 30%) | Emulates Binson Echorec. 440ms is Gilmour's golden delay time. |
| **Post-FX 2** | Plate Reverb | Mix: 15%, Decay: 1.5s | Mix: 35%, Decay: 4.2s | High decay and mix on Lead creates the massive "Comfortably Numb" spatial depth. |

---

### Row 2: Humbucker Profile (Gibson ES-339)
**Goal:** Prevent the high-output humbuckers from choking the Fuzz input stage, tight high-pass filtering to prevent bass-wobble, and increasing presence for clarity.

**Table B: Main Signal Chain (ES-339)**
*Use Scenes E (Rhythm) and F (Lead). Assign parameters via Right-Click > Assign to Scene.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global In (Circle 1) | Thresh: -50dB, Gain: -4.0dB | Thresh: -50dB, Gain: -4.0dB | **Crucial:** -4.0dB pad prevents high-output humbuckers from instantly clipping the digital input and turning the Fuzz into square-wave mud. |
| **Pre-FX 1** | Parametric-8 EQ | HPF: 120Hz | HPF: 120Hz | Trims humbucker bass resonance before the fuzz. Prevents the Ram's Head circuit from sagging or "farting out". |
| **Pre-FX 2** | Chief Fuzz | Bypass: ON (Clean) | Bypass: OFF (Sustain: 5.5, Tone: 6.5, Vol: 5.5) | Lower sustain required since humbuckers already push hard. Tone increased to 6.5 to add cut and articulation. |
| **Amp** | Hiw103 | Norm Vol: 2.5, Brill: 3.5 | Norm Vol: 2.5, Brill: 3.5 | Swapped volume dominance vs Tele: Brilliant channel pushed higher to add chime to dark humbuckers. Master: 8.5. |
| **Cab** | 412 Hiw Fane | Mic A: Dyn 57, B: Rib 121 | Mic A: Dyn 57, B: Rib 121 | Blending Ribbon 121 (Mix -3dB) to add smooth low-mids, balancing the brighter Amp settings. |
| **Post-FX 1** | Tape Delay | Bypass: ON | Bypass: OFF (Mix: 20%, Time: 440ms) | Mix lowered slightly vs Single Coils to preserve humbucker note definition. |
| **Post-FX 2** | Plate Reverb | Mix: 12%, Decay: 1.5s | Mix: 30%, Decay: 4.2s | High Pass filter in Reverb set to 200Hz to prevent low-end wash with the ES-339. |

---

### Troubleshooting & Refinement Tree
If the tone played through your QSC CP12 is **"Too Fuzzy" or "Farty/Broken"**:
1. **Input Pad:** Ensure your ES-339 Input block is at -4.0dB or even -6.0dB. Humbuckers into Big Muffs naturally create chaotic low-end harmonics.
2. **Fuzz Sustain:** Lower the `Chief Fuzz` Sustain parameter by 1.0 increments. Let the sheer volume of the Hiwatt model do the heavy lifting.
3. **Amp Headroom:** Verify `Hiw103` Master Volume is at 8.0+ and Normal/Brilliant volumes are below 4.0.
4. **Speaker EQ (PA Tuning):** To tame fizz on the QSC CP12 PA speaker, go to your Lane Output (Right side of grid) and set the High Cut (LPF) to 5.5kHz to slice off digital artifacts.

---

### Session Library (Active Presets)
**1. Preset Name: "Spoonful - ES339"**
[Previous Session Data Retained in Memory - See System Prompt History]

**2. Preset Name: "Wall Lead - Tele / 339 Matrix"**
*Target:* David Gilmour (1970s Era).
*Guitars:* Fender Telecaster (Row 1) / Gibson ES-339 (Row 2).
*Physics Goal:* High headroom pedal-platform + scooped sustaining fuzz.
*Full Configuration:*
*   **Block 1 (EQ-8 - Tele adaptation):** HPF [80Hz], Band 2 [250Hz, +3.0dB, Q 1.0], Band 8 LPF [Bypass]. (Bypass this block for ES-339).
*   **Block 2 (EQ-8 - 339 adaptation):** HPF [120Hz], Band 4 [-2dB @ 400Hz], Band 8 LPF [Bypass]. (Bypass this block for Telecaster).
*   **Block 3 (Chief Fuzz):** Sustain [Tele: 7.5 / 339: 5.5], Tone [Tele: 4.5 / 339: 6.5], Vol [6.0].
*   **Block 4 (Amp - Hiw103):** Norm Vol [Tele: 3.0 / 339: 2.5], Brill Vol [Tele: 2.0 / 339: 3.5], Bass [4.5], Mid [5.5], Treble [6.0], Presence [5.0], Master [8.5].
*   **Block 5 (Cab - 412 Hiw Fane):** Mic A (Dyn 57, Pos 0.0, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 3.0"). Mix [A: 0dB, B: -3dB].
*   **Block 6 (Tape Delay):** Mix [Tele: 25% / 339: 20%], Time [440ms], Fdbk [30%], HP [150Hz], LP [3000Hz].
*   **Block 7 (Plate Reverb):** Mix [Rhythm: 15% / Lead: 35%], Decay [Rhythm: 1.5s / Lead: 4.2s], Pre-Delay [20ms], HP [200Hz], LP [4000Hz]. 
*   **Lane Output:** Level [+2.0dB], Pan [Center].