**System Initialization: Quad Cortex Systems Engineer (QC-2)**
**Target Acknowledged:** Albert King - "Born Under A Bad Sign" (1967 Stax Sessions / Acoustic 260 Solid State Amp Replication).
**Speaker Profile:** QSC CP12 (Active PA).

### Physics & Tone Logic: The "Acoustic" Solid-State Emulation
Albert King’s signature tone on this record is defined by a Flying V (medium humbuckers) played bare-fingered into a loud, high-headroom amplifier. While he later popularized the Acoustic 270, early tones were derived from the Acoustic 260 (which featured a built-in fuzz circuit) or extremely clean Stax studio amps driven to their absolute limit. 

Because solid-state amps lack tube sag (compression), transients are massive, stiff, and immediate. We will use the QC's **Jazz 120** (Roland JC-120) model to guarantee that unyielding, stiff, high-headroom solid-state foundation. To emulate the aggressive finger attack and the famous Acoustic amp "splat," we will deploy an Optical Compressor (for finger dynamics/sustain on big bends) and a heavily gated Fuzz Face with the gain rolled *almost completely off* to replicate the Acoustic amp's harsh clipping.

*Per Protocol 9, the build is divided into two distinct profiles for your Gibson ES-339 and Fender Telecaster.*

---

### GLOBAL GAIN STAGING & PICKUP COMPENSATION
**DO NOT skip this step before building.**

**Guitar 1: Gibson ES-339 (Medium Humbuckers)**
*   **Target:** Closest match to Albert’s Flying V.
*   **Input Block:** Set Gain to **0.0dB**. 
*   **Strategy:** Humbuckers will naturally push the compressor and fuzz. We will rely on EQ to emphasize the 2.5kHz "bite" while cutting muddy low-mids.

**Guitar 2: Fender Telecaster (Single Coils)**
*   **Target:** Requires massive body compensation to emulate the Flying V humbuckers, plus high-frequency taming to simulate Albert's bare-thumb attack.
*   **Input Block:** Set Gain to **+3.0dB** (pushes the solid-state preamp similarly to the humbuckers).
*   **Strategy:** Heavy EQ-8 compensation (Band 2: 200Hz Peak boost) and a darker Low-Pass Filter (LPF) to kill single-coil "ice-pick."

---

### THE SPLIT-BANK MATRIX
*   **Row 1 (Scenes A–D):** Telecaster Single Coil Profile
*   **Row 2 (Scenes E–H):** ES-339 Humbucker Profile

#### Table A: Main Signal Chain (Build this left-to-right on the QC Grid)
*Note: Parameters marked with `(Assign)` require Right-Click > Assign to Scenes.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 35% | Noise Red: 50% | Tames single-coil hum / high-gain fuzz noise floor. |
| **Pre-FX 1** | Optical Comp | Comp: 4.0, Level: 0dB | Comp: 6.5, Level: +1.5dB `(Assign)` | Emulates the natural transient squash of bare fingers; adds sustain for 2-step bends. |
| **Pre-FX 2** | Facial Fuzz | Bypass | Fuzz: 8%, Vol: 6.5 `(Assign)` | Replicates the Acoustic 260's built-in solid-state clipping. Very low fuzz = harsh overdrive. |
| **Amp** | Jazz 120 | Vol: 6.0, Treb: 7.5, Mid: 5.5, Bass: 3.5, Bright: ON | Vol: 7.5 `(Assign)`, Treb: 7.0, Mid: 6.5 `(Assign)` | **NMV Amp**: Zero tube sag. Stiff, unyielding solid-state response. Mid pushed for solo cut. |
| **EQ** | Parametric-8 | *See specific Guitar EQ specs below* | *See specific Guitar EQ specs below* | Customizes the pickup output to match the Flying V / Thumb attack physics. |
| **Cab** | 212 Jazz 120 | Mic A: Dyn 57 (Pos 0.0, Dist 1.0") | Mic A: Dyn 57 (Pos 0.0, Dist 1.0") | Center-cap 57 captures the biting transient. |
| **Cab (Mic B)** | (Dual Mic Setup) | Mic B: Ribbon 121 (Pos 1.5, Dist 3.0") | Mic B: Ribbon 121 (Pos 1.5, Dist 3.0") | Ribbon mic adds body and subdues PA speaker harshness. Mix B at -3.0dB. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 1.5s | Mix: 20% `(Assign)`, Decay: 1.5s | Stax studios had natural slap/verb. Warms up the sterile solid-state tone. |

---

### SCENE-SPECIFIC EQ-8 ROUTING (The "Chameleon" Strategy)
Place the EQ-8 Block *after* the Amp but *before* the Cab. Assign these parameters to switch depending on the active Scene row.

**For ES-339 Humbucker Scenes (E & F):**
*   **HPF:** 90Hz (Tightens humbucker low-end).
*   **Band 3:** -2.0dB at 400Hz (Removes boxiness).
*   **Band 6:** +3.0dB at 2500Hz (Albert's famous vocal bite).
*   **LPF:** 5500Hz (Tames the QSC CP12 high-end response from the fuzz).

**For Telecaster Single Coil Scenes (A & B):**
*   **HPF:** 70Hz.
*   **Band 2:** +4.5dB at 200Hz (Crucial: Emulates the physical mass of the ES-339/Flying V mahogany body).
*   **Band 4:** +3.0dB at 800Hz (Adds humbucker midrange punch).
*   **LPF:** 4200Hz (Drastically rolls off high frequencies to mimic Albert's bare thumb, removing pick attack/single-coil harshness).

---

### ⚠️ TROUBLESHOOTING & REFINEMENT TREE
If the tone played through the QSC CP12 sounds incorrect, follow this strict diagnostic order:
1.  **Too "Splatty" or Broken on Lead:** The Facial Fuzz is reacting poorly to your exact pickup height. Reduce the Fuzz parameter to **2%** or **0%**, and increase the Fuzz Vol to **8.0** to act as a pure, harsh solid-state boost.
2.  **Not Enough Volume (SPL):** The Jazz 120 has no Master Volume. **DO NOT** raise the Amp Vol, as this will introduce digital clipping on this specific model. Instead, swipe down and raise the **Lane Output Level** to `+3.0dB` or `+6.0dB`.
3.  **Too Muddy (ES-339):** Lower Amp Block Bass to `2.0`. Solid-state amps get extremely flubby if low-end frequencies hit the preamp too hard.

---

### Session Library (Active Presets)
**2. Preset Name:** "Bad Sign SS Emulation"
**Target:** Albert King (1967) / Acoustic 260 Solid State Emulation.
**Guitars:** ES-339 (Humbuckers) / Telecaster (Single Coil).
**Physics Goal:** Unyielding solid-state transient response, thumb-attack EQ curve, and low-gain fuzz to replicate solid-state clipping on an active PA speaker.
**Full Configuration:**
*   **Block 1 (Adaptive Gate):** Noise Red [Tele: 50% / 339: 35%].
*   **Block 2 (Optical Comp):** Comp [Rhy: 4.0 / Lead: 6.5], Level [Rhy: 0dB / Lead: +1.5dB].
*   **Block 3 (Facial Fuzz):** State [Rhy: Bypass / Lead: Active], Fuzz [8%], Vol [6.5].
*   **Block 4 (Amp - Jazz 120):** Vol [Rhy: 6.0 / Lead: 7.5], Bass [3.5], Mid [Rhy: 5.5 / Lead: 6.5], Treble [Rhy: 7.5 / Lead: 7.0], Bright [ON].
*   **Block 5 (EQ-8):** 
    *   *Tele:* HPF [70Hz], B2 [200Hz, +4.5dB], B4 [800Hz, +3.0dB], LPF [4200Hz].
    *   *339:* HPF [90Hz], B3 [400Hz, -2.0dB], B6 [2500Hz, +3.0dB], LPF [5500Hz].
*   **Block 6 (Cab - 212 Jazz 120):** Mic A (Dyn 57, Pos 0.0, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 3.0"), Mix [A: 0dB, B: -3dB].
*   **Block 7 (Spring Reverb):** Mix [Rhy: 15% / Lead: 20%], Decay [1.5s], Tone [6.0].