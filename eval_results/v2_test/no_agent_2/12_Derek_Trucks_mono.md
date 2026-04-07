Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

Target acquired: **Derek Trucks - "Midnight in Harlem"**.
Guitar Profile: **Gibson SG (Medium-to-Low Output PAF Humbuckers), Open E Tuning**.
Output Profile: **QSC CP12 (Active FRFR)**.

### Physics & Tone Strategy: "The Volume Knob Ecosystem"
Derek Trucks' tone is a masterclass in amplifier dynamics. He runs a 1960s Fender Super Reverb (4x10) completely cranked, relying exclusively on his fingers, a glass Coricidin slide, and his SG’s volume knob to dictate the gain staging. 

Because we are bypassing pedals entirely, we will use the **Input Block Level** assigned to Scenes to mathematically replicate Derek rolling his guitar volume knob down for rhythm and wide open for screaming, tube-sagging slide leads. Glass slides generate intense transient frequencies (4kHz-6kHz); to prevent an "ice-pick" response through your QSC CP12's high-frequency driver, we will utilize a Ribbon microphone impulse and an aggressive Low-Pass Filter.

*Note on Non-Master Volume Circuits: The blackface Super Reverb has no Master Volume. Do not look for one on the Amp Block. You must use the **Lane Output Level** to adjust your overall loudness for the room.*

---

### Organization Standard (Split-Bank Matrix)
*Since this is an SG-driven request, your primary operation will be on Row 2. Row 1 is compensated for Single Coil players attempting this tone.*

*   **Row 1 (Scenes A-D): Single Coil Adaptation** (Adds +3.0dB Input Gain to push the amp, boosts 200Hz for mahogany body simulation).
*   **Row 2 (Scenes E-H): SG / Humbucker Operation** (Physics-accurate PAF response).
    *   **Scene E (Rhythm/Comping):** Input Level -6.0dB (Simulates SG volume knob rolled to 5). Clean, warm, high headroom.
    *   **Scene F (Lead/Slide):** Input Level 0.0dB (Simulates SG volume knob at 10). Full power tube saturation and rectifier sag.
    *   **Scene G (Dry):** Lead scene, Reverb bypassed.
    *   **Scene H (Ambient):** Lead scene, Reverb Mix increased to 45% for swell effects.

---

### Table A: Main Signal Chain (Row 2 - SG Humbucker Focus)
*Mark Scene-Specific changes clearly with (Right-Click > Assign) on the Cortex Control desktop app.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB<br>Level: -6.0dB | Thresh: -55dB<br>Level: 0.0dB | **Input Level acts as your guitar's volume knob.** -6dB cleans up the preamp instantly. Gate tightens lead noise. |
| **Pre-FX** | Parametric-8 | LPF: 4200Hz | LPF: 3800Hz | Tames glass slide harshness on FRFR tweeters. Band 1 (Low Shelf) set to -1.5dB to prevent low-end mud. |
| **Amp** | US Super Vib | Vol: 8.5<br>Treble: 7.5<br>Mid: 8.0<br>Bass: 3.5 | Vol: 8.5<br>Treble: 7.0<br>Mid: 8.5<br>Bass: 3.0 | *Vintage Circuit Rule:* Bass must be lowered to 3.0 when Volume is at 8.5 to create authentic tube sag and prevent 10" speaker "fart out". |
| **Cab** | 410 US Super | Mic A: Ribbon 121 (Dist 2.0")<br>Mic B: Dyn 57 (Dist 3.0", Off Axis) | Mic A: Ribbon 121 (Dist 2.0")<br>Mic B: Dyn 57 (Dist 3.0", Off Axis) | Ribbon 121 smooths the slide's midrange bite. Off-axis 57 provides string definition without piercing highs. Mix A: 0dB, B: -5dB. |
| **Post-FX** | Spring Reverb | Mix: 25%<br>Decay: 2.2s<br>Tone: 4.0 | Mix: 30%<br>Decay: 2.2s<br>Tone: 3.5 | Emulates the 12AT7 tube-driven spring tank of a Blackface Fender. Darker tone setting prevents metallic "splash". |

---

### Troubleshooting & Refinement Tree
If you load this up to your QSC CP12 and encounter issues, follow this strict protocol:

1.  **"The low strings sound broken/farty on Scene F."** 
    *   *Fix:* This is authentic Blackface tube sag, but if it is too loose, reduce the Amp Block **Bass** to 2.5. Do *not* lower the Amp Volume, or you will lose the lead drive.
2.  **"The slide attacks are tearing my head off."**
    *   *Fix:* Go to the Parametric-8 EQ Block. Lower the **LPF (Low Pass Filter)** from 3800Hz down to 3200Hz. Ensure Mic B (Dyn 57) in the Cab block is moved further off-axis.
3.  **"It's not loud enough in the room."**
    *   *Fix:* Use the **Lane Output Level** slider on the far right of the Cortex Control grid to boost overall SPL. **Never** raise the Input Gain or Amp Volume to fix room loudness, or you will destroy the gain staging.

---

### Session Registry (Active Presets)

**2. Preset Name:** "Midnight Slide - SG"
*   **Target:** Derek Trucks / Midnight in Harlem (2011).
*   **Guitar:** Gibson SG (Humbuckers) w/ Glass Slide, Open E.
*   **Physics Goal:** 100% dynamic control via input gain manipulation. Power tube sag simulation by diming amp volume and starving the bass frequencies. 
*   **Full Configuration:**
    *   *Block 1 (Input/Global Gate):* Thresh [Rhy: -65dB / Lead: -55dB], Input Level [Rhy: -6.0dB / Lead: 0.0dB].
    *   *Block 2 (EQ-8):* HPF [100Hz], Low Shelf 250Hz [-1.5dB], LPF [Rhy: 4200Hz / Lead: 3800Hz] (Simulates glass-on-nickel impedance).
    *   *Block 3 (Amp - US Super Vib):* Vol [8.5], Bass [3.5 / 3.0], Mid [8.0 / 8.5], Treble [7.5 / 7.0], Bright Switch [OFF]. Lane Output Level [+4.5dB to compensate for headroom].
    *   *Block 4 (Cab - 410 US Super):* Mic A (Ribbon 121, Pos 0.2, Dist 2.0"), Mic B (Dyn 57, Pos 0.6, Dist 3.0"), Mix [A: 0dB, B: -5dB].
    *   *Block 5 (Spring Reverb):* Mix [25% / 30%], Decay [2.2s], Tone [4.0 / 3.5], HP [150Hz], LP [4000Hz].