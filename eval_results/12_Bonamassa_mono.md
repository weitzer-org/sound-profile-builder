**SYSTEM INITIALIZATION: QC-2 (QUAD CORTEX SYSTEMS ENGINEER)**
**TARGET PROTOCOL:** Joe Bonamassa Modern Blues Lead (Dumble-Style)
**SPEAKER PROFILE:** QSC CP12 (Active 12" PA Monitor) — *Note: The QSC CP12 has a pronounced low-mid projection. High-Pass Filtering (HPF) is strictly required to prevent muddy "boom" when running heavy midrange D-style circuits.*

### Physics & Tone Rationale
Joe Bonamassa’s signature "Dumble" lead tone relies on cascading tube gain stages, massive headroom, and pronounced midrange emphasis, typically pushed by a mid-hump overdrive. 
* **Closest Available Model:** While Neural DSP features factory Captures of Dumble ODS amps, the closest algorithmic CorOS Amp Model is the **Capt Hook 34 Ch2** (Hook Captain 34). This boutique circuit captures the exact thick, chewy, cascading midrange of a D-style amp. 
* **Speaker Physics:** We are matching this with a **412 Brit 65A** (Marshall 4x12 loaded with Celestion G12-65s), which are the quintessential speakers used to smooth out D-style amplifiers.

We will map this preset using the **Split-Bank Matrix**:
* **Row 1 (Scenes A-D):** Fender Telecaster (Single Coil)
* **Row 2 (Scenes E-H):** Gibson ES-339 (Humbuckers)

---

### PART 1: FENDER TELECASTER (SINGLE COIL) PROFILE
**Pickup Output Compensation:** Single coils lack the low-end push and output of humbuckers. We will increase the Input Block Gain by **+2.0dB** to hit the virtual tubes harder. We also rely on the Parametric-8 EQ to add "body" (Band 1) and roll off the ice-pick attack (LPF).

**Table A.1: Main Signal Chain (Fender Telecaster)**
*Assign parameters to Scenes A & B via Right-Click > Assign to Scene.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 45% | Noise Red: 25% | Single coils hum more. Higher reduction on Rhythm. Drop for Lead sustain. |
| **Pre-FX (Drive)** | Green 808 | Gain: 1.5, Tone: 4.5 | Gain: 3.5, Tone: 5.0 | Tightens low-end and provides the signature mid-hump into the amp. |
| **Amp** | Capt Hook 34 Ch2 | Gain: 6.0, Bass: 4.5, Mid: 6.5, Treb: 4.0 | Gain: 7.5, Bass: 4.5, Mid: 7.0, Treb: 4.0 | Tele bridge pickup needs treble rolled off. Push mids for "chewy" D-style tone. |
| **EQ** | Parametric-8 | Band 1 (200Hz): +2.5dB | Band 1 (200Hz): +2.5dB | Adds physical weight/girth to the single coils. LPF at 4500Hz kills pick noise. |
| **Cab** | 412 Brit 65A | Mic 1 (121): Pos 0.5<br>Mic 2 (57): Pos 1.2 | Mic 1 (121): Pos 0.5<br>Mic 2 (57): Pos 1.2 | Ribbon 121 yields dark smoothness. Dyn 57 adds bite. HPF set to **95Hz** for QSC CP12. |
| **Post-FX (Dly)** | Analog Delay | Mix: 10%, Time: 350ms | Mix: 22%, Time: 350ms | Analog delay repeats are dark, staying out of the way of the lead notes. |
| **Post-FX (Rev)** | Plate Reverb | Mix: 15%, Decay: 1.5s | Mix: 25%, Decay: 2.2s | Simulates a large studio room or live hall. +1.5dB overall Lane Output boost in Sc B. |

---

### PART 2: GIBSON ES-339 (HUMBUCKERS) PROFILE
**Pickup Output Compensation:** High-output humbuckers can cause digital clipping and low-end mud in D-style circuits. We will reduce the Input Block Gain to **-3.0dB** to retain pick dynamics and clarity. The amp's Bass parameter is significantly reduced to account for the Gibson's inherent warmth.

**Table A.2: Main Signal Chain (Gibson ES-339)**
*Assign parameters to Scenes E & F via Right-Click > Assign to Scene.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 25% | Noise Red: 15% | Humbuckers are quieter. Lower reduction preserves dynamic ghost notes. |
| **Pre-FX (Drive)** | Green 808 | Gain: 1.0, Tone: 6.5 | Gain: 2.0, Tone: 7.0 | Used strictly as a clean boost. Tone increased to cut through ES-339 dark mahogany body. |
| **Amp** | Capt Hook 34 Ch2 | Gain: 4.0, Bass: 3.0, Mid: 6.0, Treb: 5.5 | Gain: 5.5, Bass: 3.0, Mid: 6.5, Treb: 6.0 | Dropped Bass to 3.0 to prevent QSC CP12 from booming. Treble up for clarity. |
| **EQ** | Parametric-8 | HPF: 110Hz | HPF: 110Hz | No body boost needed. HPF raised to 110Hz to strictly control humbucker low-end resonance. |
| **Cab** | 412 Brit 65A | Mic 1 (121): Pos 0.5<br>Mic 2 (57): Pos 1.2 | Mic 1 (121): Pos 0.5<br>Mic 2 (57): Pos 1.2 | Same physical speaker setup, HPF enforced at **100Hz**. |
| **Post-FX (Dly)** | Analog Delay | Mix: 8%, Time: 350ms | Mix: 18%, Time: 350ms | Mix slightly lowered compared to Tele to prevent muddying the thicker humbucker signal. |
| **Post-FX (Rev)** | Plate Reverb | Mix: 12%, Decay: 1.5s | Mix: 20%, Decay: 2.2s | +1.5dB overall Lane Output boost in Sc F. |

---

### Troubleshooting & Refinement Tree (The "Headroom Rule")
If testing through your QSC CP12 yields a tone that is **"Too Distorted," "Fuzzy," or "Boomy"**:
1. **Input Pad (Fuzz Fix):** Lower your Global Input Gain block by another -3.0dB (Simulates rolling off the guitar's volume knob).
2. **Amp Gain:** Reduce the `Capt Hook 34` Gain knob by 2.0 increments. 
3. **Tube Sag/Mud:** If the low notes "fart out" or sound loose, lower the Amp block's Bass parameter to 2.0. The QSC CP12 projects extreme bass naturally; the amp must be leaner to compensate.
4. **Volume Recovery:** *Never use a compressor to fix volume drops from lowering gain.* Go to the far-right output circle and raise the **Lane Output Level** by +2.0dB to regain your SPL loudness.

---

### Session Registry Protocol
**Session Library (Active Presets)**
2. **Preset Name:** "Bonamassa D-Style Split"
**Target:** Joe Bonamassa Modern Blues Lead (Mid-forward Dumble ODS tone).
**Guitars:** Telecaster (Sc A-D) / ES-339 (Sc E-H).
**Physics Goal:** Smooth cascading tube drive, elimination of QSC CP12 low-end mud, and exact volume matching between mismatched pickup types.
**Full Configuration:**
* **Block 1 (Adaptive Gate):** Noise Red [Tele: 45/25% | ES339: 25/15%].
* **Block 2 (Green 808):** Drive [Tele: 1.5/3.5 | ES339: 1.0/2.0], Tone [Tele: 4.5/5.0 | ES339: 6.5/7.0], Level [8.0].
* **Block 3 (Amp - Capt Hook 34 Ch2):** Gain [Tele: 6.0/7.5 | ES339: 4.0/5.5], Bass [Tele: 4.5 | ES339: 3.0], Mid [Tele: 6.5/7.0 | ES339: 6.0/6.5], Treble [Tele: 4.0 | ES339: 5.5/6.0], Master [5.0].
* **Block 4 (EQ-8):** HPF [Tele: 80Hz | ES339: 110Hz], Band 1 Peak [Tele: +2.5dB @ 200Hz | ES339: 0dB], LPF [Tele: 4500Hz | ES339: 6500Hz].
* **Block 5 (Cab - 412 Brit 65A):** Mic 1 (121 Ribbon, Pos 0.5, Dist 1.0"), Mic 2 (57 Dyn, Pos 1.2, Dist 1.5"), Mix [50/50].
* **Block 6 (Analog Delay):** Mix [Tele: 10/22% | ES339: 8/18%], Time [350ms], Fdbk [25%], Sync [Off].
* **Block 7 (Plate Reverb):** Mix [Tele: 15/25% | ES339: 12/20%], Decay [1.5s/2.2s].
* **Output Node:** Level [Rhythm: 0.0dB | Lead: +1.5dB].