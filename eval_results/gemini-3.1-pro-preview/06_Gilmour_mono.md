**System Designation:** QC-2 (Quad Cortex Systems Engineer)
**Target Profile:** David Gilmour (Mid-70s / *Animals* to *The Wall* era)
**Analog Hardware Target:** Hiwatt DR103 (100W), EHX Ram's Head Big Muff Pi, WEM 4x12 (Fane Crescendo speakers), EMT 140 Plate Reverb.
**CorOS Translation Protocol:** 
*   *Hiwatt Custom 100* -> **Custom 100 Jumped**
*   *Ram's Head Big Muff* -> Closest Available Model: **Big Fuzz** (Triangle/Standard Muff architecture)
*   *WEM 4x12 Fane* -> Closest Available Model: **412 UK Hiw Fane**
*   *Massive Plate* -> **Plate**

To achieve Gilmour’s signature infinite sustain without devolving into noise, we must rely on the **Headroom Rule**. Gilmour ran his Hiwatt preamps clean and extremely loud, letting the Big Muff do the heavy lifting for saturation, while the Fane speakers provided a flat, high-headroom frequency response. 

Below are two strictly separated configurations to adapt this massive, pedal-platform physics for both Single Coils (Fender Telecaster) and Humbuckers (Gibson ES-339), utilizing the Quad Cortex Split-Bank Matrix.

---

### Guitar 1: Fender Telecaster Single Coil Configuration
**Target Bank:** Row 1 (Scenes A - D)
**Pickup Compensation Strategy:** Vintage single coils inherently lack the low-midrange mass needed to make a Big Muff sound "creamy." We will use the Global Input Gain to push the signal (+2.0dB), and the Parametric-8 EQ block to simulate the magnetic pull and warmth of a fatter pickup before it hits the fuzz. 

**Table A: Main Signal Chain (Fender Telecaster)**
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 40% <br>Thresh: -65dB | Noise Red: 20% <br>Thresh: -70dB | Single coils + Fuzz = 60-cycle hum. Grid-based Adaptive Gate tracks guitar transients better than the Global circle gate. |
| **Pre-FX 1** | Parametric-8 | Band 1 (LS): +3.0dB @ 200Hz<br>Band 8 (LPF): 4.5kHz | Band 1 (LS): +3.0dB @ 200Hz<br>Band 8 (LPF): 5.0kHz | **Chameleon Strategy:** Adds necessary "body" weight to the Telecaster and aggressively rolls off the ice-pick pick attack before hitting the Fuzz. |
| **Pre-FX 2** | Big Fuzz | Vol: 5.0, Tone: 4.0<br>Sustain: 4.5 | Vol: 5.5, Tone: 4.5<br>Sustain: 7.5 | Lower tone knob tames the triangle/NYC muff circuit to emulate the smoother, creamier Ram's Head era. |
| **Amp** | Custom 100 Jumped | Vol Norm: 3.5<br>Vol Bright: 3.0 | Vol Norm: 3.5<br>Vol Bright: 3.0 | **Tube Taper Logic:** Hiwatts are high-headroom pedal platforms. Keep Preamp volumes low. Master Vol is set to 8.0. Mid: 6.5 to counteract the Muff's mid-scoop. |
| **Cab** | 412 UK Hiw Fane | Mic A: Dyn 57 (Center, 1.0")<br>Mic B: Rib 121 (Edge, 2.0") | Mic A: Mix +0dB<br>Mic B: Mix -3dB | **Speaker Physics:** Fane speakers have massive headroom. The Ribbon 121 provides the low-end thump, the 57 provides the Gilmour bite. |
| **Post-FX 1** | Analog Delay | Mix: 15% <br>Time: 440ms | Mix: 25% <br>Time: 440ms | Emulates Gilmour's Binson Echorec/MXR analog delays. Placed *before* reverb to diffuse the repeats. |
| **Post-FX 2** | Plate | Decay: 2.8s<br>Mix: 25% | Decay: 4.5s<br>Mix: 40% | **Spatial Goal:** EMT 140 emulation. High Pass set to 150Hz to prevent the massive tails from creating low-end mud with the fuzz. |

---

### Guitar 2: Gibson ES-339 Humbuckers Configuration
**Target Bank:** Row 2 (Scenes E - H)
**Pickup Compensation Strategy:** Humbuckers will inherently hit the Big Fuzz too hard, causing the internal gain stages of the fuzz block to clip in an unmusical, "farty" way. We must apply a massive pad to the Input Block (-4.0dB) to simulate rolling off the guitar volume, preserving the fuzz's clarity.

**Table B: Main Signal Chain (Gibson ES-339)**
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 30% <br>Thresh: -70dB | Noise Red: 15% <br>Thresh: -75dB | Humbuckers are quieter regarding 60-cycle hum, allowing for a gentler threshold to preserve sustain and dynamics. |
| **Pre-FX 1** | Parametric-8 | Band 2 (Peak): -2.5dB @ 250Hz<br>Band 8 (LPF): 6.0kHz | Band 2 (Peak): -2.5dB @ 250Hz<br>Band 8 (LPF): 6.5kHz | **Chameleon Strategy:** Cuts the muddy low-mid frequencies of the 339 to prevent the Fuzz from "farting out." LPF is raised to let humbucker clarity through. |
| **Pre-FX 2** | Big Fuzz | Vol: 5.0, Tone: 5.5<br>Sustain: 3.5 | Vol: 6.0, Tone: 6.0<br>Sustain: 5.5 | Humbuckers naturally compress. Sustain must be drastically lowered compared to the Telecaster to maintain note definition on chords. |
| **Amp** | Custom 100 Jumped | Vol Norm: 3.0<br>Vol Bright: 3.5 | Vol Norm: 3.0<br>Vol Bright: 3.5 | Because humbuckers are darker, we bias the amp toward the "Bright" channel to add "glass" back to the tone. Bass reduced to 4.0. |
| **Cab** | 412 UK Hiw Fane | Mic A: Dyn 57 (Center, 0.5")<br>Mic B: Rib 121 (Edge, 3.0") | Mic A: Mix +0dB<br>Mic B: Mix -5dB | Pushing the 57 closer to the cap adds high-end presence, while lowering the Ribbon mic mix prevents the 339 from sounding too thick. |
| **Post-FX 1** | Analog Delay | Mix: 12% <br>Time: 440ms | Mix: 22% <br>Time: 440ms | Identical timing, but slightly lower mix to prevent the thicker humbucker transients from washing out the rhythmic bed. |
| **Post-FX 2** | Plate | Decay: 2.8s<br>Mix: 20% | Decay: 4.5s<br>Mix: 35% | Humbuckers excite reverb tanks more aggressively due to their output; mix is reduced by 5% to maintain spatial depth without drowning the dry signal. |

---

### Troubleshooting & Refinement Tree
If either guitar sounds **"Too Distorted", "Fuzzy", or "Broken"**:
1. **Input Pad (Crucial for ES-339):** Lower Global Input Block Gain (Circle 1) to -6.0dB. 
2. **Amp Gain:** Reduce *Vol Norm* and *Vol Bright* on the Custom 100 Jumped by 1.0 increments. The Amp should be completely clean. 
3. **Tube Sag/Farting:** Lower the Bass parameter on the Amp block to 3.0. The Big Fuzz adds massive low-end; the amp must stay tight to handle it.
4. **Volume Compensation:** *Never* increase the Fuzz Volume to compensate for overall quietness. Raise the **Lane Output Level** (far right output circle) to achieve desired PA/Gig loudness.

---

### Session Library (Active Presets)
**2. Preset Name: "Comfortably Fuzz - Tele / 339"**
*   **Target:** David Gilmour (1977-1979)
*   **Physics Goal:** Massive, infinite-sustain pedal platform utilizing high-headroom Fane speakers and aggressive EQ-chameleon strategies to make Fuzz work for both single coils and humbuckers.
*   **Full Configuration:**
    *   **Global Input Gain:** [Tele: +2.0dB / ES-339: -4.0dB]
    *   **Block 1 (Adaptive Gate):** Noise Red [Tele Rhy: 40% / 339 Rhy: 30%], Thresh [-65dB / -70dB], Decay [150ms].
    *   **Block 2 (EQ-8):** HPF [80Hz]. *Tele:* Band 1 [+3.0dB @ 200Hz], LPF [4.5kHz]. *339:* Band 2 [-2.5dB @ 250Hz], LPF [6.0kHz].
    *   **Block 3 (Big Fuzz):** *Tele:* Tone [4.0 / 4.5], Sustain [4.5 / 7.5]. *339:* Tone [5.5 / 6.0], Sustain [3.5 / 5.5].
    *   **Block 4 (Custom 100 Jumped):** Vol Norm [3.5 / 3.0], Vol Bright [3.0 / 3.5], Bass [5.0 / 4.0], Mid [6.5], Treble [6.0], Presence [5.5], Master [8.0].
    *   **Block 5 (412 UK Hiw Fane):** Mic A (Dyn 57, Pos 1.0, Dist 1.0"), Mic B (Rib 121, Pos 2.0, Dist 2.0"), Mix [A: 0dB, B: -3dB / -5dB].
    *   **Block 6 (Analog Delay):** Mix [15% / 25%], Time [440ms], Fdbk [35%], Tone [5.0].
    *   **Block 7 (Plate):** Mix [25% / 40%], Decay [2.8s / 4.5s], Pre-Delay [15ms], HP [150Hz], LP [4500Hz].