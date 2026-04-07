**QC-2 Systems Engineer Active.** 
**Target Acknowledged:** B.B. King "Lucille" profile. 
**Analog Target Specs:** Gibson ES-345/355 (Humbuckers, Varitone circuit, out-of-phase mid-honk) -> Fender Twin Reverb (High-headroom, pushed mids, tamed highs) -> PA/FRFR.

Before we finalize gain stages, I must verify: **Are the humbuckers in your ES-339 and the single coils in your Telecaster Vintage Output, Medium, or High Output?** (This preset is optimized for Vintage/Medium output. If you use high-output modern pickups, you will need to lower the Input Block).

To achieve B.B. King's biting but pristine clean tone, we are using the **US TWN Vibrato** model. The authentic Twin Reverb circuit does *not* feature a Master Volume. To achieve the massive headroom required for "Lucille" without driving the preamp tubes into breakup, we will rely strictly on the Amp Volume knob (kept low) and push the overall output SPL via the Lane Output Level to properly feed your active QSC CP12 PA speaker. 

Here is your engineered Split-Bank Preset.

---

### 🎸 MULTI-GUITAR TARGET OUTPUT & SPLIT-BANK MATRIX

Because a Telecaster and an ES-339 interact with a Twin Reverb's preamp in fundamentally different ways (physics of inductance and output voltage), we are deploying the **"Chameleon" EQ Strategy** via Scene Controllers.

*   **Row 1 (Scenes A-D): Fender Telecaster (Single Coil Profile)**
    *   **Physics Goal:** Single coils lack the lower-midrange push of a Gibson and hit the preamp with less voltage, which can cause a Twin to sound sterile and "ice-picky."
    *   **Compensation:** We assign the Input Block Gain to **+2.5dB**. The Parametric-8 EQ engages Band 1 (Low Shelf) at +3.0dB (200Hz) to simulate the physical body/wood of an ES-339, and a Low-Pass Filter at 4.5kHz to shave off the harsh single-coil transient pick attack. 
*   **Row 2 (Scenes E-H): Gibson ES-339 (Humbucker Profile)**
    *   **Physics Goal:** Humbuckers will naturally push a Twin Reverb closer to the edge-of-breakup and can sound boomy on the neck pickup.
    *   **Compensation:** We drop the Input Block Gain to **-3.0dB** to pad the input and guarantee pristine headroom. The Parametric-8 EQ flattens the bass response and pushes Band 5 (800Hz) by +2.0dB to simulate B.B. King's signature Varitone midrange "honk."

#### Scene Assignments
*   **Scene A / E (Rhythm):** High headroom clean, perfect for comping and fingerpicking. 
*   **Scene B / F (Lead):** B.B. King solo tone. Volume boosted via Lane Output, mids pushed, light optical compression added for sustain mimicking high-SPL tube sag.
*   **Scene C / G (Dry):** Studio dry (Reverb bypassed).
*   **Scene D / H (Ambient):** Extended reverb tail for atmospheric blues phrasing.

---

### TABLE A: MAIN SIGNAL CHAIN

*(Note: Parameters marked "Assign" use Scene Controllers to pivot between Rhythm and Lead dynamics.)*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -65dB | Thresh: -65dB | High-headroom cleans require a low threshold to preserve delicate fingerpicked dynamics. |
| **Pre-FX** | Parametric-8 | *Tele:* +3dB @ 200Hz<br>*339:* +2dB @ 800Hz | *Tele:* +3.5dB @ 200Hz<br>*339:* +3.5dB @ 800Hz | **The Chameleon Block:** Transforms Tele into a semi-hollow (body boost), gives 339 the Varitone mid-honk. |
| **Pre-FX** | Opto Comp | Mix: 0% (Bypassed) | Mix: 45%, Comp: 35% | Simulates the natural tube compression of a Twin Reverb cranked to stage volume, adding sustain to B.B.'s vibrato. |
| **Amp** | US TWN Vibrato | Vol: 3.5, Mid: 6.5 | Vol: 4.5, Mid: 7.5 *(Assign)* | **No Master Vol.** Pushing amp volume slightly for lead adds harmonic richness. Mids are pushed to cut through the mix. |
| **Cab** | 212 US TWN PR-75 | Mic A: Ribbon 121 (Pos: 1.0)<br>Mic B: Dyn 57 (Pos: 0.0) | Mic A: Ribbon 121 (Pos: 1.0)<br>Mic B: Dyn 57 (Pos: 0.0) | Ribbon mic smooths out the Twin's harsh top-end. Dynamic mic strictly captures the punch of the speaker dust cap. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 2.0s | Mix: 20%, Decay: 2.5s *(Assign)* | Authentically replicates the Fender analog spring tank. Sits behind the dry signal to avoid washing out staccato phrasing. |
| **Lane Output**| Output Level | +4.0dB | +6.5dB *(Assign)* | Controls overall SPL to the QSC CP12 without altering the preamp tube gain staging. |

---

### 🔧 TROUBLESHOOTING & REFINEMENT TREE

If feeding the QSC CP12 FRFR speaker feels "Too Bright" or "Fuzzy" at gig volumes, follow this exact order of operations:
1.  **Input Pad:** If the amp is breaking up (distorting) on the ES-339, your pickups are hotter than vintage spec. Lower the Global Input Gain from -3.0dB down to **-6.0dB**.
2.  **Amp EQ:** If the tone is "ice-picky" on the Telecaster, do not lower the Amp Treble below 4.0, or the amp model loses its physical character. Instead, go to the EQ-8 block and lower the High-Pass Filter (LPF Band 8) from 4.5kHz down to **3.5kHz**. 
3.  **Tube Sag:** The Twin Reverb model has a robust low-end. If palm-muted notes sound "farty" or overwhelming through the 12" PA speaker, reduce the Amp Bass parameter from 4.0 down to 2.5. 

---

### 💾 SESSION LIBRARY (ACTIVE PRESETS)

**2. Preset Name: "Lucille Twin - Multi-Gen"**
*   **Target:** B.B. King (ES-345 into Fender Twin Reverb). High headroom clean platform.
*   **Target Output:** QSC CP12 Active PA Speaker.
*   **Full Configuration:**
    *   **Block 1 (Input/Gate - Global):** Gate Thresh [-65dB], Input Gain [Tele: +2.5dB / 339: -3.0dB].
    *   **Block 2 (Opto Comp):** Comp [35%], Ratio [4.0], Attack [15ms], Release [150ms], Mix [Rhy: 0% / Lead: 45%].
    *   **Block 3 (Parametric-8):** HPF [80Hz], Band 1 Low Shelf [Tele: +3.0dB @ 200Hz / 339: 0.0dB], Band 5 Peak [Tele: +1.0dB @ 800Hz / 339: +2.0dB @ 800Hz], LPF [Tele: 4500Hz / 339: 6000Hz]. 
    *   **Block 4 (Amp - US TWN Vibrato):** Vol [Rhy: 3.5 / Lead: 4.5], Treble [5.0], Middle [Rhy: 6.5 / Lead: 7.5], Bass [4.0]. *(Note: No Master Volume on circuit).*
    *   **Block 5 (Cab - 212 US TWN PR-75):** Mic A (Ribbon 121, Pos 1.0, Dist 2.0"), Mic B (Dyn 57, Pos 0.0, Dist 1.0"), Mix [A: 0dB, B: -3dB].
    *   **Block 6 (Spring Reverb):** Mix [Rhy: 15% / Lead: 20%], Decay [2.0s / 2.5s], High Damp [3500Hz].
    *   **Lane 1 Output:** Pan [Center], Level [Rhy: +4.0dB / Lead: +6.5dB].