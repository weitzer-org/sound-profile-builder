To capture B.B. King’s "Lucille" tone, we must adhere strictly to the **Physics First Protocol**. B.B. King relied on massive, high-headroom clean amplifiers (Fender Twin Reverbs or Gibson LAB Series) and pushed them with heavy pick attack and a signature vibrato, not preamp distortion. His ES-355 (Lucille) utilized humbuckers and a Varitone switch, often resulting in a fat, compressed, yet articulate tone with rolled-off treble to avoid "ice-pick" harshness.

Since the Fender Twin Reverb (`CA Twin Ch2`) is a non-master volume circuit, we will strictly control the Amp Block `Volume` to prevent tube saturation, and use the **Lane Output Level** for all Scene-based loudness increases (Lead boosts). 

Here is your dual-guitar split-bank matrix configured for the QSC CP12 flat-response speaker.

---

### **Organization Standard (Split-Bank Matrix)**
*Note: Ensure your QC Input Block is configured before loading these scenes.*
* **Row 1 (Scenes A-D): Fender Telecaster (Single Coil)** | Input Block Gain: **+2.5dB** (Compensates for low output).
* **Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers)** | Input Block Gain: **-3.0dB** (Prevents high-output humbuckers from clipping the digital A/D converters or driving the Twin into early breakup).

**Scene Functions:**
* **A/E (Rhythm):** Pristine clean comping. Lane Output: 0.0dB.
* **B/F (Lead):** B.B. King Sting. Lane Output: +2.0dB. Reverb mix increased slightly.
* **C/G (Dry):** Studio dry. No Reverb.
* **D/H (Varitone):** Out-of-phase "honk" simulation (Aggressive mid-scoop via EQ).

---

### **Table A: Main Signal Chain**
*Right-Click parameters to Assign them to Scenes A/E (Rhythm) and B/F (Lead).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -65dB | Thresh: -65dB | High headroom clean tones require very little gating. Set low to preserve B.B.'s long vibrato sustain. |
| **Pre-FX (EQ)** | Parametric-8 | *See Multi-Guitar Specs Below* | *See Multi-Guitar Specs Below* | The "Chameleon" block. Used to mimic humbucker weight on the Tele, or Varitone clarity on the 339. |
| **Amp** | CA Twin Ch2 | Vol: 4.0 <br> Treble: 5.0 <br> Middle: 6.5 <br> Bass: 4.0 <br> Bright: OFF | Vol: 4.0 <br> Treble: 5.5 <br> Middle: 7.0 <br> Bass: 4.0 <br> Bright: OFF | Non-master volume circuit. Vol stays at 4.0 to ensure zero preamp tube clipping. Middle is boosted for vocal-like lead articulation. |
| **Cab** | 212 CA Twin | Mic A: Dyn 57 (Pos: 0.4, Dist: 1.0") <br> Mic B: Rib 121 (Pos: 0.8, Dist: 3.0") | Mic A: Dyn 57 (Pos: 0.4, Dist: 1.0") <br> Mic B: Rib 121 (Pos: 0.8, Dist: 3.0") | 57 captures the hard pick attack; 121 captures the woody resonance of the hollowbody. Mix A: 0dB, Mix B: -3dB. |
| **Post-FX** | Spring Reverb | Mix: 15% <br> Decay: 2.0s <br> Tone: 4.0 | Mix: 22% <br> Decay: 2.5s <br> Tone: 5.0 | Emulates the analog Twin spring tank. Tone is kept low/dark to prevent splashing over the fundamental note. |

---

### **Multi-Guitar Target Output & The Chameleon Strategy**

To achieve the identical Lucille physics on two drastically different instruments, utilize the `Parametric-8` block immediately before the Amp block.

#### **Target 1: Gibson ES-339 (Humbuckers) - The Natural Fit**
* **Pickup Selection:** Middle Position (Both pickups) or Neck, depending on era. Roll guitar tone knob down to 7.
* **Input Block Gain:** -3.0dB (Crucial headroom rule).
* **EQ-8 Settings:**
  * HPF: 80Hz (Cleans up low-end boominess from the semi-hollow body).
  * Band 3 (Peak): -2.5dB at 400Hz, Q: 1.5 (Simulates a mild Varitone notch to add clarity to humbuckers).
  * LPF: 6500Hz (Removes modern humbucker fizz, keeps it vintage).
* **Gain Staging:** The ES-339 pushes more voltage. Let the Twin model breathe. If you hit the strings hard and hear crackling, drop Input Gain to -5.0dB.

#### **Target 2: Fender Telecaster (Single Coil) - The Chameleon Adaptation**
* **Pickup Selection:** Neck Pickup or Middle Position. Keep guitar tone and volume at 10.
* **Input Block Gain:** +2.5dB.
* **EQ-8 Settings:**
  * HPF: 60Hz.
  * Band 1 (Low Shelf): +3.5dB at 250Hz (Adds the missing physical "wood/body" weight of a Gibson 339).
  * Band 6 (Peak): -1.5dB at 2.5kHz, Q: 1.0 (Tames the inherent "spank" and upper-mid spike of Fender pickups).
  * LPF (Twang Control): 4500Hz (Strictly cuts off the Telecaster ice-pick treble attack).
* **Gain Staging:** The added low-shelf EQ might cause bass-flub on the Twin model. If the amp sounds "farty" on the low E string, reduce the Amp Block Bass parameter from 4.0 to 3.0. 

---

### **Troubleshooting & Refinement Tree**
If the tone is "Too Distorted" or "Too Piercing":
1. **Input Pad:** Lower Input Block Gain by an additional -3.0dB (simulates rolling off guitar volume).
2. **Amp Volume:** Ensure the `CA Twin Ch2` Volume is at 4.0 or lower. Never push it past 5.0 for this genre. 
3. **Pick Attack:** If the transient attack is too harsh through the QSC CP12 horn, lower the Parametric-8 LPF frequency by 500Hz until it smooths out.
4. **Output Compensation:** To make the Lead Scene louder, strictly use the **Lane Output Level** (drag the circle on the far right of the grid) from 0.0dB to +2.0dB. Do not use an overdrive pedal or compressor block for volume.

---

### **Session Library (Active Presets)**

**2. Preset Name: "Lucille's Twin - Dual Rig"**
* **Target:** B.B. King / "The Thrill Is Gone" (Live 1970s).
* **Guitars:** Gibson ES-339 (Humbuckers) / Fender Telecaster (Single Coils).
* **Physics Goal:** High headroom, pristine clean, high-transient attack with rolled-off treble, tube spring spatial depth.
* **Full Configuration:**
  * **Block 1 (Global Gate):** Thresh [-65dB], Decay [200ms].
  * **Block 2 (EQ-8 - ES339):** HPF [80Hz], Band 3 [-2.5dB @ 400Hz], LPF [6500Hz]. 
  * **Block 2 (EQ-8 - Tele):** HPF [60Hz], Low Shelf [+3.5dB @ 250Hz], Band 6 [-1.5dB @ 2.5kHz], LPF [4500Hz].
  * **Block 3 (Amp - CA Twin Ch2):** Vol [4.0], Treble [5.0 / 5.5], Middle [6.5 / 7.0], Bass [4.0], Bright [OFF], Lane Output Level [Rhy: 0.0dB / Lead: +2.0dB].
  * **Block 4 (Cab - 212 CA Twin):** Mic A (Dyn 57, Pos 0.4, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 3.0"), Mix [A: 0dB, B: -3dB].
  * **Block 5 (Spring Reverb):** Mix [15% / 22%], Decay [2.0s / 2.5s], Tone [4.0 / 5.0].