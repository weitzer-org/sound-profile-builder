Here is your "Cream-Era Clapton" System Profile. 

To recreate the legendary "Woman Tone" (heard on "Sunshine of Your Love" and "White Room"), we are relying heavily on vintage physics. Clapton achieved this by plugging into the Bright channel of a Marshall 100W Super Lead, cranking the amp's volume, but rolling the **Tone knob on his neck pickup completely down to 0-2**. 

Because Plexi circuits lack a Master Volume, cranking the amp's Volume pushes immense amounts of low-end into the transformer, which can cause terrible "farty" sag if not controlled. To counteract this and keep the tone articulate through your QSC CP12 speaker, **the Amp's Bass parameter is strictly lowered**.

### 🎸 MULTI-GUITAR TARGET STRATEGY
Below are the dedicated signal chains. **Row 1 (Scenes A-D)** is configured to make your Fender Telecaster sound like a thick, rolled-off humbucker. **Row 2 (Scenes E-H)** is engineered for your Gibson ES-339 to act as the authentic vintage source.

---

### ROW 2: Gibson ES-339 Humbuckers (Scenes E-H)
*Hardware Instruction:* Switch to your **Neck Pickup**. Roll your guitar's **Tone knob to 0 or 1**. Keep guitar Volume at 10.
*Input Block Strategy:* 0.0dB Gain. Let the natural humbucker output drive the front of the Plexi.

**Table A: Main Signal Chain (ES-339)**
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -60dB | Thresh: -65dB | Lower threshold on lead to let the thick, sustained notes ring out without chopping. |
| **Pre-FX** | EQ-8 (Parametric) | *Bypassed* | *Bypassed* | Not needed. The ES-339 provides the necessary natural low-mid frequency push. |
| **Amp** | Brit Plexi 100 Bright | Vol: 5.5 *(Assign)*<br>Bass: 2.5<br>Mid: 7.5<br>Treble: 6.5<br>Presence: 5.0 | Vol: 8.5 *(Assign)*<br>Bass: 1.5 *(Assign)*<br>Mid: 8.5 *(Assign)*<br>Treble: 7.5<br>Presence: 6.0 | **Tube Taper Logic:** Volume acts as drive. Bass MUST drop to 1.5 on Lead to prevent transformer blowout/farting when Vol hits 8.5. |
| **Cab** | 412 Brit Green | Mic A: Dyn 57 (Pos: 0.4, Dist: 1.0")<br>Mic B: Ribbon 160 (Pos: 1.5, Dist: 4.0") | Mix A: -2.0dB<br>Mix B: 0.0dB | **Speaker Physics:** Greenbacks provide mid-range chewiness. Ribbon mic backs off harsh PA speaker highs. |
| **Post-FX** | Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 20%<br>Decay: 1.5s | **Spatial Goal:** Simulates the analog plate reverb used at Olympic Studios during Disraeli Gears sessions. |
| **Output** | Lane Output Level | Level: +2.0dB *(Assign)* | Level: -3.5dB *(Assign)* | **Headroom Rule:** Compensates for the massive volume jump when Amp Vol goes from 5.5 to 8.5. SPL stays matched for the QSC. |

---

### ROW 1: Fender Telecaster Single Coils (Scenes A-D)
*Hardware Instruction:* Switch to your **Neck Pickup**. Keep your guitar's **Tone knob at 5** (Do not roll to 0, or the single coil will disappear into the mix). We will use the Quad Cortex to simulate the rest of the humbucker's physical response.
*Input Block Strategy:* +3.0dB Gain. We must artificially push the amp harder to simulate the higher output of PAF humbuckers.

**Table B: Main Signal Chain (Telecaster)**
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55dB<br>Gain: +3.0dB | Thresh: -58dB<br>Gain: +3.0dB | Boosts the single coil signal to hit the amp's virtual V1 tube as hard as a humbucker would. |
| **Pre-FX** | EQ-8 (Parametric) | *Active*<br>Band 2: +3.5dB (250Hz)<br>LPF: 4500Hz | *Active*<br>Band 2: +4.0dB (250Hz)<br>LPF: 2200Hz *(Assign)* | **The Chameleon:** Adds humbucker "body" (250Hz). The aggressive Low Pass Filter (2200Hz) mathematically simulates a rolled-off humbucker tone pot! |
| **Amp** | Brit Plexi 100 Bright | Vol: 6.5 *(Assign)*<br>Bass: 3.5<br>Mid: 7.0<br>Treble: 6.5<br>Presence: 5.0 | Vol: 9.5 *(Assign)*<br>Bass: 2.5 *(Assign)*<br>Mid: 8.0 *(Assign)*<br>Treble: 7.5<br>Presence: 6.0 | **Tube Taper Logic:** Single coils are naturally thinner, so we can afford slightly more Bass (2.5) on the lead scene than the ES-339. |
| **Cab** | 412 Brit Green | Mic A: Dyn 57 (Pos: 0.4, Dist: 1.0")<br>Mic B: Ribbon 160 (Pos: 1.5, Dist: 4.0") | Mix A: -2.0dB<br>Mix B: 0.0dB | Exact same cabinet geometry as the 339 to maintain tonal cohesion. |
| **Post-FX** | Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 20%<br>Decay: 1.5s | Replicates studio plate. High Pass set to 150Hz in Reverb block to prevent muddy trails. |
| **Output** | Lane Output Level | Level: +1.5dB *(Assign)* | Level: -4.0dB *(Assign)* | Matches QSC CP12 volume output between Rhythm and Lead scenes. |

---

### 🔧 Troubleshooting & Refinement Tree
If testing through your QSC CP12 yields unsatisfactory results, follow this strict protocol:
1. **Tone is too "Farty" or the low-end collapses on Lead:** The Plexi is sagging too hard. Go to the Amp Block and lower the `Bass` parameter by 0.5 increments. 
2. **Tone is too fuzzy/distorted on Rhythm:** Lower the Global Input Block Gain to `-3.0dB` (simulating rolling off your guitar's volume knob to clean up the amp).
3. **Telecaster Lead is too shrill:** Go to the Pre-FX EQ-8 block and lower the LPF frequency from `2200Hz` down to `1500Hz`. This darkens the simulation further.

---

### 📁 Session Library (Active Presets)
*QC-2 Memory Log for instant recall/modification:*

**2. Preset Name: "White Room - Woman Tone"**
*Target:* Eric Cream-Era (1967) / "Woman Tone"
*Guitar:* ES-339 Humbuckers (Scenes E-H) & Telecaster Single Coils (Scenes A-D).
*Physics Goal:* Non-Master Volume Plexi pushed to saturation with heavy treble roll-off, avoiding low-end transformer sag.
*Full Configuration:*
*   **Input Block:** Gate Thresh [-60dB], Gain [ES-339: 0dB / Tele: +3.0dB].
*   **Block 1 (EQ-8 - Telecaster Only):** HPF [90Hz], Band 2 [250Hz, +3.5dB / +4.0dB], LPF [Rhy: 4500Hz / Lead: 2200Hz].
*   **Block 2 (Amp - Brit Plexi 100 Bright):** Vol [ES: 5.5/8.5 | Tele: 6.5/9.5], Bass [ES: 2.5/1.5 | Tele: 3.5/2.5], Mid [7.0/8.5], Treble [6.5/7.5], Presence [5.0/6.0].
*   **Block 3 (Cab - 412 Brit Green):** Mic A (Dyn 57, Pos 0.4, Dist 1.0"), Mic B (Ribbon 160, Pos 1.5, Dist 4.0"), Mix [A: -2dB, B: 0dB], High Cut [8.5kHz], Low Cut [90Hz].
*   **Block 4 (Plate Reverb):** Mix [15% / 20%], Decay [1.2s / 1.5s], HP [150Hz], LP [4000Hz].
*   **Lane Output:** Level [ES Rhy: +2.0dB / ES Lead: -3.5dB | Tele Rhy: +1.5dB / Tele Lead: -4.0dB].