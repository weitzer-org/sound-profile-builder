Greetings. I am **QC-2**, your Quad Cortex Systems Engineer. 

To achieve the Joe Bonamassa modern blues lead tone—characterized by vocal, cascading midrange, smooth top-end compression, and immense sustain—we will utilize the **Custom ODS OD** (Neural DSP's exact emulation of the Dumble Overdrive Special). Because we are monitoring through a **QSC CP12 (Active PA Speaker)**, we will retain full Cabinet simulation and ensure the low-frequency energy (below 100Hz) is strictly managed to prevent the speaker's DSP limiters from engaging and ruining our transient response.

Before we finalize, my core diagnostic trigger: *Are your specific ES-339 and Telecaster pickups Vintage Output, Medium, or High Output?* (Assuming Medium-High for the ES-339 and Vintage for the Telecaster for this build).

Here is your Split-Bank Matrix protocol, followed by the exact block-by-block configurations for both instruments.

### **Organization Standard (Split-Bank Matrix)**
*   **Row 1 (Scenes A-D) - Fender Telecaster (Single Coil Base)**
    *   **Scene A:** Rhythm (Tight, -1.5dB output, Mid-Gain).
    *   **Scene B:** Lead (Mid-boosted, +1.5dB output, Delay ON).
    *   **Scene C:** Dry/Comping (No Reverb/Delay).
    *   **Scene D:** Ambient/FX (High Mix Reverb/Delay).
*   **Row 2 (Scenes E-H) - Gibson ES-339 (Humbucker Base)**
    *   **Scene E:** Rhythm (Tight, -1.5dB output, Pad engaged).
    *   **Scene F:** Lead (Mid-boosted, +1.5dB output, Delay ON).
    *   **Scene G:** Dry/Comping (No Reverb/Delay).
    *   **Scene H:** Ambient/FX (High Mix Reverb/Delay).

---

### **Multi-Guitar Target Output: Distinct Configurations**

Due to the extreme physics differences between an ES-339 (massive low-mid resonance, high output humbuckers) and a Telecaster (spiky transients, low output, scooped mids), driving a Dumble-style amp requires fundamentally different gain staging and EQ. 

#### **1. Gibson ES-339 (Humbuckers) Configuration**
**Goal:** Prevent low-mid mud, avoid digital clipping at the input, and keep the overdrive smooth without fuzzing out. 
*   **Input Block:** Set Gain to **-3.0dB** (Crucial for high-output humbuckers to preserve headroom).

**Table A: Main Signal Chain (ES-339)**
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 30% | Noise Red: 15% | Lower reduction on lead preserves sustain tail. Threshold set to dynamic floor. |
| **Pre-FX** | Myth Drive | Drive: 1.0, Tone: 6.0, Level: 7.5 | Drive: 2.5, Tone: 6.5, Level: 8.5 | Acts as a Klon-style mid-push. Tightens bass before it hits the Dumble preamp. |
| **Amp** | Custom ODS OD | Drive: 4.5, OD: 5.0, Bass: 3.5, Mid: 5.5, Treb: 5.0, Master: 5.0 | Drive: 6.0, OD: 6.5, Bass: 3.0, Mid: 7.0, Treb: 5.5, Master: 5.5 | Cascading gain structure. Lowered bass prevents tube sag/farting out on humbuckers. |
| **Cab** | 412 UK V30 | Mic A: Dyn 57 (Pos 1.5, Dist 1")<br>Mic B: Rib 121 (Pos 3.0, Dist 2") | Mic A: Dyn 57 (Pos 1.0, Dist 1")<br>Mic B: Rib 121 (Pos 3.0, Dist 2") | Dyn 57 moved closer to center on Lead for aggressive cut. Ribbon adds 339 body. Mix A: 0dB, B: -4dB. |
| **Post-FX 1** | Analog Delay | Bypass: ON | Mix: 15%, Time: 380ms, Fdbk: 25% | Classic Bonamassa thickener. Dark repeats prevent clashing with the dry lead note. |
| **Post-FX 2** | Plate Reverb | Mix: 12%, HPF: 150Hz, LPF: 4000Hz | Mix: 18%, HPF: 150Hz, LPF: 4000Hz | Simulates studio plate/rack reverb. HPF keeps the QSC CP12 woofer tight. |

---

#### **2. Fender Telecaster (Single Coils) Configuration**
**Goal:** Synthesize the missing low-mid body of a humbucker, tame the ice-pick bridge pickup attack, and push the amp harder to compensate for lower pickup output.
*   **Input Block:** Set Gain to **+1.5dB** (Recovers signal lost from vintage single coils). 
*   **EQ Block Addition:** Insert a **Parametric-8 EQ** block *immediately after the Adaptive Gate* but *before the Myth Drive*.

**Table B: Main Signal Chain Modifications (Telecaster)**
*Apply these overrides to the Table A baseline using Scenes A & B.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Gate/EQ** | Parametric-8 | Band 2: +3.0dB @ 250Hz (Peak)<br>Band 8: LPF @ 5500Hz | Band 2: +4.0dB @ 250Hz (Peak)<br>Band 8: LPF @ 4800Hz | **Chameleon Strategy:** Band 2 mimics the humbucker body. LPF aggressively shaves Telecaster ice-pick transients. |
| **Pre-FX** | Myth Drive | Drive: 3.0, Tone: 4.5, Level: 8.0 | Drive: 4.5, Tone: 5.0, Level: 9.0 | Telecaster needs more pedal drive and a darker Tone setting compared to the ES-339. |
| **Amp** | Custom ODS OD | Drive: 6.0, OD: 6.0, Bass: 5.5, Mid: 6.0, Treb: 4.5 | Drive: 7.5, OD: 7.5, Bass: 5.0, Mid: 7.5, Treb: 4.0 | Increased bass to fill out the single coils. Lowered treble. Cascading drive pushed harder for sustain. |

---

### **Troubleshooting & Refinement Tree**
If testing through your QSC CP12 yields unsatisfactory results, apply this strict order of operations:
1.  **If the tone is "Too Distorted" or "Too Fuzzy" (Usually ES-339 issue):**
    *   *Input Pad:* Lower Input Block Gain further to -6.0dB (Simulates rolling off guitar volume, directly clearing up the Dumble preamp).
    *   *Amp Gain:* Reduce the `OD` parameter on the Custom ODS by 2.0 increments. Do *not* touch the `Drive` first.
2.  **If the amp sounds "broken/farty" in the low-end:**
    *   *Tube Sag:* Reduce `Bass` on the Custom ODS to 2.0. Dumble circuits are notorious for low-end flub when the OD section is pushed hard.
3.  **If the volume drops too much after reducing gain:**
    *   *Output Compensation:* Raise the Lane Output Level (far right circle on the grid). Never use a compressor to fix this volume discrepancy.

---

### **Session Library (Active Presets)**

**2. Preset Name: "JB Modern Blues - ES339/Tele"**
*   **Target:** Joe Bonamassa (Modern Dumble Lead Tone).
*   **Guitar:** ES-339 (Humbuckers) / Telecaster (Single Coils).
*   **Physics Goal:** Cascading vocal midrange, zero harsh fizz, high sustain, FRFR optimized.
*   **Full Configuration:**
    *   *Block 1 (Adaptive Gate):* Noise Red [ES339 Rhy: 30% / Lead: 15%] [Tele Rhy: 45% / Lead: 25%].
    *   *Block 2 (EQ-8 - Telecaster Only):* HPF [100Hz], Band 2 [+3dB/+4dB @ 250Hz], LPF [5500Hz/4800Hz].
    *   *Block 3 (Myth Drive):* Drive [ES: 1.0/2.5] [Tele: 3.0/4.5], Tone [ES: 6.0/6.5] [Tele: 4.5/5.0], Level [7.5 to 9.0 depending on Scene].
    *   *Block 4 (Amp - Custom ODS OD):* Drive [ES: 4.5/6.0] [Tele: 6.0/7.5], OD [ES: 5.0/6.5] [Tele: 6.0/7.5], Bass [ES: 3.5/3.0] [Tele: 5.5/5.0], Mid [ES: 5.5/7.0] [Tele: 6.0/7.5], Treble [ES: 5.0/5.5] [Tele: 4.5/4.0], Master [5.0/5.5].
    *   *Block 5 (Cab - 412 UK V30):* Mic A (Dyn 57, Pos 1.5/1.0, Dist 1.0"), Mic B (Ribbon 121, Pos 3.0, Dist 2.0"), Mix [A: 0dB, B: -4dB].
    *   *Block 6 (Analog Delay):* Mix [15%], Time [380ms], Fdbk [25%]. Active on Lead Scenes.
    *   *Block 7 (Plate Reverb):* Mix [12% / 18%], HP [150Hz], LP [4000Hz]. Decay [1.8s]. Lane Output Level managed to +1.5dB on Lead Scenes.