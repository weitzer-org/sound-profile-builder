**QC-2 Systems Engineer: Initialization Complete.**

To recreate B.B. King's iconic studio tone on "The Thrill Is Gone" (1969), we must understand the physics of his gear during that session. He used a Gibson ES-355 running into a cranked **Fender Twin Reverb** (`US TWN Reverb`). 

The signature "hollow, biting" out-of-phase tone was achieved either via the ES-355's Varitone switch (Position 2) or rolling the guitar's volume knobs against each other in the middle position. Since you are using a QSC CP12 active PA speaker (which has a wide, hi-fi frequency response), we must aggressively manipulate the EQ to simulate that out-of-phase nasal honk and tame the PA's low-end boominess. 

To satisfy the **Multi-Guitar Target Output** protocol, I have separated the configurations into two distinct profiles. 

*Trigger Verification:* Before saving, please confirm if your ES-339 and Telecaster feature Vintage, Medium, or High-Output pickups. The math below assumes standard/vintage output.

---

### PART 1: GIBSON ES-339 (HUMBUCKERS) PROFILE
**Goal:** Tame humbucker low-end, lower the headroom ceiling to simulate B.B. rolling down his guitar volume, and force an out-of-phase "honk" using pre-EQ. 
**Gain Staging Instruction:** Set your Global Input (Circle 1) Gain to **-5.0dB**. This is critical. It prevents the high-output humbuckers from clipping the Twin Reverb model, ensuring the compressed, stinging clean tone remains bell-like.

**Table A: ES-339 Main Signal Chain (Row 2: Scenes E-H)**
*Mark Scene-specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -65dB | Thresh: -65dB | High sustain requirement. Only gate extreme floor noise. |
| **Pre-EQ** | Parametric-8 | Mix: 100% | Mix: 100% | **Simulates ES-355 Out-Of-Phase/Varitone.** |
| *(EQ Details)* | *Varitone Sim* | HPF: 350Hz, B3: -6dB (600Hz), B5: +5dB (1.8kHz) | HPF: 350Hz, B3: -6dB (600Hz), B5: +6.5dB (1.8kHz) | HPF removes all humbucker mud. B5 spikes the upper-mid "stinging" frequencies. |
| **Amp** | US TWN Reverb | Vol: 4.5, Treb: 6.5, Mid: 5.0, Bass: 3.5 | Vol: 6.5, Treb: 7.5, Mid: 6.0, Bass: 3.5 | Non-Master amp. Raising volume compresses the tube stage for the lead tone. |
| **Cab** | 212 US TWN C12N | Mic A (Ribbon 121): Pos 0.6, Dist 2.0" | Mic A (Ribbon 121): Pos 0.6, Dist 2.0" | Ribbon mic tames ice-pick transients. |
| *(Cab Mix)* | *Dual Mic* | Mic B (Dyn 57): Pos 0.2, Dist 1.0" (Mix: -4dB) | Mic B (Dyn 57): Pos 0.2, Dist 1.0" (Mix: -2dB) | Dynamic 57 adds the string attack back into the QSC speaker. |
| **Post-FX** | Vintage Plate | Mix: 18%, Decay: 1.8s | Mix: 26%, Decay: 2.2s | The studio recording features a lush chamber/plate, not just spring verb. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | Compensates for the ES-339's lowered input gain. |

---

### PART 2: FENDER TELECASTER (SINGLE COIL) PROFILE
**Goal:** The Telecaster already has cut and bite, but inherently lacks the nasal, hollow midrange of an out-of-phase humbucker. We must push the amp slightly harder, thicken the body, but still carve out the specific 600Hz dip.
**Gain Staging Instruction:** Set your Global Input (Circle 1) Gain to **+1.5dB**. Single coils have lower voltage output; this slight boost ensures the amp hits its "edge of compression" sweet spot. Play on the middle pickup position.

**Table B: Telecaster Main Signal Chain (Row 1: Scenes A-D)**
*Mark Scene-specific changes clearly with (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -60dB | Thresh: -60dB | Telecasters have 60-cycle hum; threshold raised slightly. |
| **Pre-EQ** | Parametric-8 | Mix: 100% | Mix: 100% | **Simulates ES-355 on a Telecaster.** |
| *(EQ Details)* | *Varitone Sim* | HPF: 150Hz, B2: +3dB (250Hz), B3: -5dB (600Hz) | HPF: 150Hz, B2: +3dB (250Hz), B3: -5dB (600Hz) | HPF is lower here to preserve Tele body. B2 adds "humbucker weight." |
| **Amp** | US TWN Reverb | Vol: 5.0, Treb: 5.5, Mid: 6.0, Bass: 4.5 | Vol: 7.0, Treb: 6.0, Mid: 7.0, Bass: 4.5 | Tele bridge/middle is naturally bright; Treble is dialed back compared to the Gibson. |
| **Cab** | 212 US TWN C12N | Mic A (Ribbon 121): Pos 0.4, Dist 1.0" | Mic A (Ribbon 121): Pos 0.4, Dist 1.0" | Pushed closer to the cone for proximity effect (bass boost). |
| *(Cab Mix)* | *Dual Mic* | Mic B (Dyn 57): Pos 0.8, Dist 3.0" (Mix: -6dB) | Mic B (Dyn 57): Pos 0.8, Dist 3.0" (Mix: -6dB) | Dyn 57 pushed further back to prevent Tele ice-pick through the PA. |
| **Post-FX** | Vintage Plate | Mix: 15%, Decay: 1.8s | Mix: 22%, Decay: 2.2s | Same lush studio tail. |
| **Output** | Lane 1 Output | Level: -1.0dB | Level: +1.0dB | Overall volume balancing against the Humbucker bank. |

---

### Troubleshooting & Refinement Tree
If you find the tone is acting incorrectly through your QSC CP12:
1. **If the tone is "Too Piercing/Harsh" (Common with PA speakers):** Do not touch the amp EQ first. Go to the Cab Block and pull the **Dynamic 57** microphone distance back from `1.0"` to `3.0"`. This simulates moving the mic away from the speaker cone in a studio space.
2. **If the amp is distorting/fuzzing out:** Follow the strict protocol. Lower the Input Pad to `-6.0dB`. The US TWN Reverb is a non-master volume circuit; it *will* overdrive if your pickups hit it too hard. 
3. **If it lacks sustain for the vibrato:** Instead of adding gain, add an `Opto Comp` in the Pre-FX slot (Settings: Sustain 4.0, Comp 3.0, Level +2.0dB) to transparently hold the note without adding distortion.

---

### Session Library (Active Presets)
**2. Preset Name:** "Thrill Is Gone - Multi-Target"
**Target:** B.B. King / The Thrill Is Gone (1969 Studio Tone).
**Guitars:** ES-339 (Humbuckers) / Telecaster (Single Coils).
**Physics Goal:** Simulate Varitone out-of-phase wiring via parametric EQ, recreate high-headroom tube compression, and format for FRFR PA projection.
**Full Configuration:**
**Bank 1 (Humbucker/ES-339):** Input Gain [-5.0dB]. Gate Thresh [-65dB]. Pre-EQ (Parametric-8) [HPF 350Hz, B3 -6dB/600Hz, B5 +6.5dB/1.8kHz]. Amp (US TWN Reverb) [Vol 4.5/6.5, Treb 6.5/7.5, Mid 5.0/6.0, Bass 3.5]. Cab (212 US TWN C12N) [Mic 121 Pos 0.6 Dist 2.0", Mic 57 Pos 0.2 Dist 1.0" Mix -2dB]. Plate Reverb [Mix 18/26%, Dec 1.8/2.2s].
**Bank 2 (Single Coil/Tele):** Input Gain [+1.5dB]. Gate Thresh [-60dB]. Pre-EQ (Parametric-8) [HPF 150Hz, B2 +3dB/250Hz, B3 -5dB/600Hz]. Amp (US TWN Reverb) [Vol 5.0/7.0, Treb 5.5/6.0, Mid 6.0/7.0, Bass 4.5]. Cab (212 US TWN C12N) [Mic 121 Pos 0.4 Dist 1.0", Mic 57 Pos 0.8 Dist 3.0" Mix -6dB]. Plate Reverb [Mix 15/22%, Dec 1.8/2.2s].