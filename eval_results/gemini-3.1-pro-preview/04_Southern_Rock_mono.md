Target Identified: Southern Rock Slide Style (Allman Brothers / Derek Trucks inspiration). 
Physics Goal: Infinite "singing" sustain, thick midrange to support a glass/brass slide, and edge-of-breakup dynamics controlled by the guitar's volume knob. We are utilizing the Neural DSP "US Tweed Basslad Jumped" (closest available model to a vintage Fender Tweed Bassman 5F6-A) pushing into a transparent overdrive to generate sustain without the low-end "farting out" common in Tweed circuits.

Since you are running into a QSC CP12 FRFR active speaker, managing low-end resonance and taming high-end fizz is critical. 

Here is your Quad Cortex configuration. Per system protocol, I am providing distinct routing and gain staging for both your Gibson ES-339 and a Fender Telecaster.

***

### 🎸 MULTI-GUITAR GAIN STAGING & CONFIGURATION

To maximize the physics of each pickup type while maintaining the same amplifier core, you must set your Global Input differently for each guitar. 

#### Row 1 (Scenes A-D): Single Coil Profile (Fender Telecaster)
*Slide on a Telecaster can become piercingly bright, lacking the natural compression of humbuckers.*
*   **Input Block Gain:** `+2.0dB` (Pushes the amp closer to humbucker territory).
*   **EQ Compensation (Parametric-8):** Enable Band 1 (Low Shelf) at `200Hz` with `+3.0dB` gain to add physical weight/body to the slide notes. Enable Band 8 (LPF) at `4500Hz` to tame the "glassy" transient of the slide hitting the strings.
*   **Amp Block Adjustments:** Increase `Vol Norm` to `6.0` to thicken the low-mids.
*   **Scene Matrix:** [A] Rhythm (Tight), [B] Lead (+1.5dB / Mid-Boost), [C] Dry Comping, [D] Ambient Slide Swells.

#### Row 2 (Scenes E-H): Humbucker Profile (Gibson ES-339)
*Vintage Tweed circuits lack a tight power amp; feeding high-output humbuckers straight in will cause extreme tube sag and muddiness.*
*   **Input Block Gain:** `-3.0dB` to `-4.5dB` (Crucial to prevent digital clipping and preserve the amp's headroom).
*   **EQ Compensation (Parametric-8):** Disable the 200Hz boost. Ensure a High-Pass Filter (HPF) is active at `90Hz` to prevent the neck humbucker from blowing out the Tweed's power section.
*   **Amp Block Adjustments:** Drop `Bass` to `2.5` to tighten the sag. Let the humbuckers do the heavy lifting for the low-end.
*   **Scene Matrix:** [E] Rhythm (Tight), [F] Lead (+1.5dB / Mid-Boost), [G] Dry Comping, [H] Ambient Slide Swells.

***

### TABLE A: MAIN SIGNAL CHAIN (The "Statesboro" Build)
*(Note: Parameters marked "Assign" should be assigned to Scenes B/F via Right-Click in Cortex Control for your Lead tone).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: `40%` | Noise Red: `15%` (Assign) | Lower gate % on lead preserves the dying sustain of slide vibrato. |
| **Pre-FX (Drive)** | Myth Drive | Gain: `1.5`, Tone: `6.0`, Level: `7.5` | Gain: `4.0` (Assign), Tone: `6.5` | Emulates a Klon. Low gain hits the amp hard for natural tube sustain. Lead scene adds clipping. |
| **Amp** | US Tweed Basslad Jumped | Vol Norm: `3.5`, Vol Bright: `5.0`, Bass: `3.0`, Mid: `6.5`, Treb: `6.0`, Pres: `5.5` | Vol Norm: `4.5` (Assign), Vol Bright: `6.5` (Assign) | **No Master Volume on this model.** Normal channel adds girth; Bright channel adds the slide "bite". Midrange is pushed for sustain. |
| **Cab** | 410 Basslad PR10 | Mic A: Dyn 57 (Pos 0.5, Dist 1.0"), Mic B: Rib 121 (Pos 0.8, Dist 4.0") | Mix: `A: 0dB`, `B: -3dB` | 57 captures the string scratch; 121 ribbon adds the warm "wood" of the guitar body. |
| **Post-FX (Delay)** | Analog Delay | Mix: `12%`, Time: `350ms`, Fdbk: `20%` | Mix: `25%` (Assign), Time: `350ms`, Fdbk: `35%` | Fills the space between slide notes without washing out the core tone. |
| **Post-FX (Reverb)** | Plate | Mix: `15%`, Decay: `1.2s`, Pre-delay: `15ms` | Mix: `20%` (Assign), Decay: `1.8s` | Replicates the live Fillmore East hall acoustics. |
| **Output** | Lane Output Level | Level: `0.0dB` | Level: `+1.5dB` (Assign) | Sole control for actual FOH/QSC SPL loudness. |

***

### 🔧 TROUBLESHOOTING & REFINEMENT TREE
If playing through your QSC CP12 reveals issues:
1. **The Amp sounds "broken", "farty", or "woofy" on the low strings:**
   * *Fix:* The Bassman circuit's power tubes are sagging too hard under the ES-339 humbuckers. Drop the Amp Block's `Bass` parameter by 1.0 increments. If it persists, lower your Global Input Gain to `-6.0dB`.
2. **The slide attack is harsh, icy, or fatiguing:**
   * *Fix:* Go to the Cab Block and increase the Distance of Mic A (Dyn 57) from `1.0"` to `2.5"`. Alternatively, lower the `Tone` knob on the Myth Drive block.
3. **Not enough volume for the gig:**
   * *Fix:* **Do not touch the Amp Volume knobs** (this will just add more distortion and fuzz). Go to your Lane Output Level at the end of the grid and increase the output by `+2.0dB` to `+4.0dB`.

***

### 💾 PRESET REGISTRY PROTOCOL (Session Library)
*Saving configuration to session memory...*

**Session Library (Active Presets)**
1. Preset Name: "Spoonful - ES339" *(Saved in previous memory)*
2. Preset Name: "Statesboro Slide"
Target: Duane Allman / Derek Trucks (Southern Rock).
Guitar 1: ES-339 (Humbuckers) / Guitar 2: Telecaster (Single Coils). Slide playing.
Physics Goal: Singing sustain via mid-pushed preamp without excessive fuzz, utilizing a transparent drive into a jumped Tweed circuit.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%].
Block 2 (EQ-8): ES-339 [HPF 90Hz]. Telecaster [HPF 90Hz, Band 1: 200Hz +3.0dB, LPF 4500Hz].
Block 3 (Myth Drive): Gain [Rhy: 1.5 / Lead: 4.0], Tone [6.0 / 6.5], Level [7.5].
Block 4 (Amp - US Tweed Basslad Jumped): Vol Norm [3.5 / 4.5], Vol Bright [5.0 / 6.5], Bass [3.0], Mid [6.5], Treble [6.0], Presence [5.5].
Block 5 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Rib 121, Pos 0.8, Dist 4.0"), Mix [A: 0dB, B: -3dB].
Block 6 (Analog Delay): Mix [12% / 25%], Time [350ms], Fdbk [20% / 35%].
Block 7 (Plate Reverb): Mix [15% / 20%], Decay [1.2s / 1.8s].
Lane Output: Rhy [0.0dB] / Lead [+1.5dB].