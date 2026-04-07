Acknowledged. QC-2 online. Constructing a gig-ready David Gilmour-style preset targeting the *Animals* and *The Wall* era. The foundation will be a high-headroom pedal platform amp with switchable fuzz, EQ, and time-based effects.

Executing protocol. I will build two distinct gain-staged versions for immediate deployment with two common guitar types.

---

### **Preset: `Gilmour UKC100 Lead`**

#### **Target Guitar 1: Gibson ES-339 (Medium/High-Output Humbuckers)**

**Gain Staging Strategy:** Humbuckers will naturally drive the amp and pedals harder. The primary goal is to maintain clarity and prevent the low-end from becoming saturated or "farty" when the Muff Fuzz is engaged. The Global Input Gain will be lowered to -3.0dB to simulate a slightly rolled-back guitar volume, creating headroom and improving picking dynamics.

**Table A: Main Signal Chain (Gibson ES-339)**
*Scene-specific changes are marked with `(Right-Click > Assign)`.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65.0dB | Thresh: -60.0dB | Humbuckers are quieter; a lower threshold is fine for cleans. Raised for Lead to gate the high-gain noise from the Muff Fuzz. |
| **Pre-FX** | Muff Fuzz | **[BYPASSED]** | **Type:** Rams Head, **Sustain:** 6.5, **Tone:** 5.0, **Level:** 7.0 | Engaged for lead tone only. Sustain is lowered for humbuckers to prevent oversaturation. Level is set for unity gain with the amp. |
| **Pre-FX** | Parametric-8 | **[BYPASSED]** | **Band 4 Freq:** 950Hz, **Gain:** +4.0dB, **Q:** 1.0 | The Ram's Head is notoriously mid-scooped. This EQ is assigned *only* to the Lead scene to push the mids back in, allowing the guitar to cut through a dense band mix. |
| **Amp** | **Closest Available: UK C-100** | **Vol Norm:** 5.0, **Vol Brill:** 6.5, **Bass:** 4.0, **Mid:** 7.0, **Treb:** 6.5, **Pres:** 5.5, **Level:** 0.0dB | **Vol Norm:** 5.0, **Vol Brill:** 6.5, **Bass:** 4.0, **Mid:** 7.0, **Treb:** 6.5, **Pres:** 5.5, **Level:** 0.0dB | The Hiwatt circuit is extremely clean with massive headroom. The "Jumped" input (blending Normal and Brill channels) is key. All gain comes from the Muff Fuzz, not the amp. Amp Level remains constant. |
| **Cab** | 412 UK C-Star F75 | **Mic A:** Dyn 57 (Dist 1.0"), **Mic B:** Ribbon 121 (Dist 3.0"), **Pan:** A: L15, B: R15 | **Mic A:** Dyn 57 (Dist 1.0"), **Mic B:** Ribbon 121 (Dist 3.0"), **Pan:** A: L15, B: R15 | Simulates a WEM 4x12 with Fane speakers. The Dyn 57 captures the aggressive midrange bite, while the Ribbon 121 adds body and smooths the top-end fizz. Panning creates a wide stereo image. |
| **Post-FX** | Digital Delay | **Mix:** 15%, **Time:** 380ms, **Fdbk:** 25%, **LPF:** 3.5kHz | **Mix:** 35%, **Time:** 440ms, **Fdbk:** 30%, **LPF:** 3.0kHz | Subtle quarter-note repeat for rhythm. A more prominent, darker (lower LPF) dotted-eighth feel for solos to create space without interfering with the dry signal. |
| **Post-FX** | Plate Reverb | **Mix:** 20%, **Decay:** 2.5s, **PreDly:** 10ms | **Mix:** 30%, **Decay:** 4.5s, **PreDly:** 25ms | The "massive" plate requested. Kept shorter for rhythm to avoid mud. The Lead scene gets a longer decay and more pre-delay to separate the initial attack from the reverb wash, enhancing clarity. |
| **Output** | Lane 1 Output | **Level:** -1.5dB | **Level:** 0.0dB | The Lead scene is boosted by +1.5dB relative to the Rhythm scene for appropriate live volume dynamics. |

---

#### **Target Guitar 2: Fender Telecaster (Vintage/Low-Output Single Coils)**

**Gain Staging Strategy:** The lower output of Telecaster pickups requires compensation. The signal needs to be boosted to properly drive the Muff Fuzz and achieve saturation without sounding thin or weak. The Global Input Gate threshold will be set higher to combat single-coil noise (60-cycle hum), and the Global Input Gain will be increased.

**Table A: Main Signal Chain (Fender Telecaster)**
*Scene-specific changes are marked with `(Right-Click > Assign)`.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -58.0dB | Thresh: -52.0dB | Single coils are noisy. The threshold is set higher to aggressively gate hum, especially when the high-gain Muff Fuzz is active. |
| **Pre-FX** | Muff Fuzz | **[BYPASSED]** | **Type:** Rams Head, **Sustain:** 8.0, **Tone:** 5.5, **Level:** 7.0 | Sustain is increased significantly to compensate for the lower pickup output and achieve the desired saturation. Tone is nudged up to add bite. |
| **Pre-FX** | Parametric-8 | **[BYPASSED]** | **Band 4 Freq:** 950Hz, **Gain:** +4.5dB, **Q:** 1.0, **Band 1 (LShelf) Freq:** 200Hz, **Gain:** +2.0dB | Same mid-boost as the humbucker preset, but slightly more gain. A Low Shelf (Band 1) is added *only for the lead scene* to give the thinner single-coil bridge pickup more body and weight. |
| **Amp** | **Closest Available: UK C-100** | **Vol Norm:** 5.5, **Vol Brill:** 7.0, **Bass:** 4.5, **Mid:** 7.0, **Treb:** 6.0, **Pres:** 5.5, **Level:** +1.0dB | **Vol Norm:** 5.5, **Vol Brill:** 7.0, **Bass:** 4.5, **Mid:** 7.0, **Treb:** 6.0, **Pres:** 5.5, **Level:** +1.0dB | Amp input volumes and overall Level are pushed slightly higher to work better with the lower output pickups. Treble is reduced to tame potential "ice pick" frequencies from the bridge pickup. |
| **Cab** | 412 UK C-Star F75 | **Mic A:** Dyn 57 (Dist 1.0"), **Mic B:** Ribbon 121 (Dist 3.0"), **Pan:** A: L15, B: R15 | **Mic A:** Dyn 57 (Dist 1.0"), **Mic B:** Ribbon 121 (Dist 3.0"), **Pan:** A: L15, B: R15 | The cabinet and mic selection remains identical, as its function is to accurately reproduce the speaker's physical response, which does not change. |
| **Post-FX** | Digital Delay | **Mix:** 15%, **Time:** 380ms, **Fdbk:** 25%, **LPF:** 3.8kHz | **Mix:** 35%, **Time:** 440ms, **Fdbk:** 30%, **LPF:** 3.2kHz | Settings are functionally identical. The LPF frequency is slightly adjusted to complement the Telecaster's brighter transient attack. |
| **Post-FX** | Plate Reverb | **Mix:** 20%, **Decay:** 2.5s, **PreDly:** 10ms | **Mix:** 30%, **Decay:** 4.5s, **PreDly:** 25ms | Reverb settings remain constant. The goal is to create a consistent spatial environment for the instrument, regardless of pickup type. |
| **Output** | Lane 1 Output | **Level:** -1.5dB | **Level:** 0.0dB | Lead/Rhythm volume boost structure is maintained for consistent live performance. |

---
**Troubleshooting Note:** If the lead tone with the Muff Fuzz feels too harsh or "fizzy," the first adjustment should be to lower the `LPF` (Low-Pass Filter) parameter on the **Cab Block** to around 4.5kHz - 5.5kHz. This simulates the natural high-frequency roll-off of a physical speaker and is more effective than reducing the Treble or Presence on the amp.

### **Session Library (Active Presets)**
1. Preset Name: "Spoonful - ES339"
   - *(...as previously defined...)*
2. Preset Name: "Gilmour - UKC100 Lead"
   - **Target:** David Gilmour (The Wall era).
   - **Guitar:** Gibson ES-339 (Humbuckers).
   - **Physics Goal:** High-headroom clean platform with a high-gain, mid-boosted fuzz lead tone and expansive time-based effects.
   - **Full Configuration:**
     - **Block 1 (Input Gate):** Thresh [Rhy: -65dB / Lead: -60dB].
     - **Block 2 (Muff Fuzz):** [Bypassed / Active], Type [Rams Head], Sustain [6.5], Tone [5.0], Level [7.0].
     - **Block 3 (EQ-8):** [Bypassed / Active], Band 4 Freq [950Hz], Gain [+4.0dB], Q [1.0].
     - **Block 4 (Amp - UK C-100):** Vol Norm [5.0], Vol Brill [6.5], Bass [4.0], Mid [7.0], Treb [6.5], Pres [5.5], Level [0.0dB].
     - **Block 5 (Cab - 412 UK C-Star F75):** Mic A (Dyn 57), Mic B (Ribbon 121), Pan [L15/R15].
     - **Block 6 (Digital Delay):** Mix [15% / 35%], Time [380ms / 440ms], Fdbk [25% / 30%], LPF [3.5kHz / 3.0kHz].
     - **Block 7 (Plate Reverb):** Mix [20% / 30%], Decay [2.5s / 4.5s], PreDly [10ms / 25ms].
     - **Block 8 (Output):** Level [-1.5dB / 0.0dB].