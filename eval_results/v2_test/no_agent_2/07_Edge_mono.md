Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To achieve "The Edge" style chime—a 1964 Vox AC30 Top Boost on the edge-of-breakup combined with rhythmic, modulated dotted-eighth delays—we must prioritize signal clarity and transient attack. Running this through a QSC CP12 (an active, full-range PA speaker) requires strict management of high-frequency "ice-pick" harmonics (inherent to Vox circuits) and precise gain staging to ensure the delay repeats do not clip the digital headroom.

Per my operating parameters, I have mapped this build to the verified Neural DSP model: **UK C30 Top Boost**. Since the original AC30 lacks a Master Volume, we will rely exclusively on the Amp Block **Volume** for drive/chime, and the **Lane Output Level** for overall loudness.

Below are the distinct configurations and gain staging instructions separated for your two target instruments: the Fender Telecaster and the Gibson ES-339.

***

### 🎸 PROFILE 1: FENDER TELECASTER (SINGLE COILS)
**Matrix Allocation:** Row 1 (Scenes A–D)
**Physics Strategy:** Single coils naturally excel at The Edge's tone, providing the necessary percussive attack (simulating his famous Herdim pick scrape). We will use an EQ to boost the body slightly (200Hz) so the bridge pickup doesn't sound thin through the QSC CP12, and push the AC30's input volume higher to hit that sweet "edge-of-breakup" compression.

**Table A: Main Signal Chain (Telecaster)**
*Note: Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | Thresh: -65dB | Thresh: -65dB | High threshold to instantly clamp single-coil 60-cycle hum between rhythmic rests. |
| **Pre-FX (Comp)** | Jewel Comp | Comp: 3.0, Mix: 60% | Comp: 4.5, Mix: 70% | Evens out the pick attack for consistent delay triggering without squashing dynamics. |
| **Pre-FX (EQ)** | Parametric-8 | Band 1 (Peak): +2.0dB @ 200Hz | Band 1 (Peak): +2.5dB @ 200Hz | Adds essential physical "body/weight" to the Tele bridge pickup. |
| **Amp** | UK C30 Top Boost | Vol: 5.5, Tone Cut: 3.5 | Vol: 6.5, Tone Cut: 3.0 | Tube Taper Logic: High volume pushes the EL84 virtual tubes into harmonic saturation. Lower Tone Cut = brighter. |
| **Cab** | 212 UK C30 | Mic A: Dyn 57 (Dist 1.0"), Mix 0dB | Mic B: Rib 121 (Dist 2.5"), Mix -3dB | Speaker Physics: Alnico Blues. 57 captures transient attack; 121 warms up the harsh PA top-end. |
| **Post-FX (Delay)** | Vintage Digital Delay | Note: 1/8D, Fdbk: 35%, Mix: 40% | Note: 1/8D, Fdbk: 45%, Mix: 45% | Spatial Goal: Replicates the classic SDD-3000. Dotted-eighth (1/8D) creates the rhythmic counter-melody. |
| **Output** | Lane 1 Output | Level: -1.5dB | Level: +1.5dB | Headroom Rule: Uses output fader for SPL boost rather than amp gain to maintain clarity. |

* **Scene C (Dry/Comping):** Bypass Delay block. Reduce Amp Vol to 4.5.
* **Scene D (Ambient):** Enable a `Lush Reverb` block at 35% mix, Delay Feedback increased to 60%.

***

### 🎸 PROFILE 2: GIBSON ES-339 (HUMBUCKERS)
**Matrix Allocation:** Row 2 (Scenes E–H)
**Physics Strategy:** Humbuckers will inherently output +3dB to +6dB more than your Telecaster, immediately pushing the AC30 into unwanted fuzz/mud and destroying the pristine delay repeats. **We must aggressively pad the input** and utilize the EQ block to carve out low-mid mud while simulating single-coil clarity. 

**Table B: Main Signal Chain (Gibson ES-339)**
*Note: Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | Thresh: -70dB, Gain: -4.5dB | Thresh: -70dB, Gain: -3.0dB | **Crucial:** Pad Input Gain by -4.5dB to prevent humbuckers from clipping the amp block. Lower gate threshold (humbuckers are quieter). |
| **Pre-FX (Comp)** | Jewel Comp | Comp: 2.0, Mix: 40% | Comp: 3.5, Mix: 50% | Less compression needed than the Tele, as humbuckers naturally compress the signal. |
| **Pre-FX (EQ)** | Parametric-8 | Band 2 (Shelf): -3.0dB @ 300Hz | Band 6 (Peak): +2.0dB @ 3.5kHz | Chameleon Strategy: Cuts mud (300Hz) and boosts high-mid "chime" (3.5kHz) to fake a single-coil attack. |
| **Amp** | UK C30 Top Boost | Vol: 3.0, Tone Cut: 2.0 | Vol: 4.2, Tone Cut: 1.5 | Tube Taper Logic: Drastically lower volume to keep the humbuckers clean. Tone Cut is lowered to open up high frequencies. |
| **Cab** | 212 UK C30 | Mic A: Dyn 57 (Dist 1.5"), Mix 0dB | Mic B: Rib 121 (Dist 2.0"), Mix -5dB | Speaker Physics: Pulling the 57 back slightly (1.5") reduces proximity effect low-end boom from the ES-339. |
| **Post-FX (Delay)** | Vintage Digital Delay | Note: 1/8D, Fdbk: 30%, Mix: 38% | Note: 1/8D, Fdbk: 40%, Mix: 42% | Spatial Goal: Slightly lower mix than Tele to prevent overlapping frequencies from blurring the dotted-eighth rhythm. |
| **Output** | Lane Output Level | Level: +1.5dB | Level: +3.0dB | Headroom Rule: Compensates for the -4.5dB drop at the input block so SPL matches the Telecaster. |

* **Scene G (Dry/Comping):** Bypass Delay block. Amp Tone Cut to 2.5 (darker).
* **Scene H (Ambient):** Enable `Lush Reverb` at 30% mix, Delay Feedback 55%.

***

### ⚙️ Troubleshooting & Refinement Tree
If through your QSC CP12 the tone sounds **"Too Piercing/Harsh"**:
1. **EQ Adjustment:** Go to the EQ-8 Block and engage Band 8 as a **Low Pass Filter (LPF)** set to `5500Hz`. This acts as a global "cut" to stop high-frequency PA tweeter fizz.
2. **Cab Mic:** Increase the distance of Mic A (Dyn 57) to `2.0"`.

If the tone sounds **"Too Distorted/Fuzzy"** (Especially on the ES-339):
1. **Input Pad:** Verify your Input Block Gain is at `-4.5dB` or lower. 
2. **Amp Gain:** Reduce the `UK C30 Top Boost` Volume knob by 1.0 increments.
3. **Tube Sag:** Decrease the Amp's Bass parameter to `3.0` to tighten the low end.
4. **Output Compensation:** Raise Lane Output Level to recover any lost volume. *Never use a compressor to fix gain staging.*

***

### 💾 Session Library (Active Presets)
**2. Preset Name: "Streets Chime - Dual Profile"**
*   **Target:** The Edge (U2) / 1964 Vox AC30 Top Boost.
*   **Guitars:** Fender Telecaster (Scenes A-D) / Gibson ES-339 (Scenes E-H).
*   **Physics Goal:** Fast transient response, pristine edge-of-breakup EL84 saturation, distinct dotted-eighth separation.
*   **Full Configuration:**
    *   *Block 1 (Global Input Gate):* Thresh [Tele: -65dB / ES-339: -70dB], Input Gain [Tele: 0.0dB / ES-339: -4.5dB].
    *   *Block 2 (Jewel Comp):* Comp [Tele: 3.0-4.5 / ES-339: 2.0-3.5], Level [Unity], Mix [Tele: 60% / ES-339: 40%].
    *   *Block 3 (EQ-8):* HPF [100Hz], Band 1 [Tele: +2dB@200Hz / ES-339: 0dB], Band 2 [Tele: 0dB / ES-339: -3dB@300Hz], Band 6 [Tele: 0dB / ES-339: +2dB@3.5kHz], LPF [6500Hz].
    *   *Block 4 (Amp - UK C30 Top Boost):* Vol [Tele: 5.5-6.5 / ES-339: 3.0-4.2], Bass [4.5], Treble [6.0], Tone Cut [Tele: 3.5 / ES-339: 2.0].
    *   *Block 5 (Cab - 212 UK C30):* Mic A (Dyn 57, Pos 0.5, Dist 1.0"-1.5"), Mic B (Ribbon 121, Pos 1.0, Dist 2.0"-2.5"), Mix [A: 0dB, B: -3dB to -5dB].
    *   *Block 6 (Vintage Digital Delay):* Note [1/8D], Fdbk [30-45%], Mix [38-45%], Mod Depth [15%], Mod Rate [1.5Hz].
    *   *Lane Output:* Level [Tele: -1.5dB to +1.5dB / ES-339: +1.5dB to +3.0dB].