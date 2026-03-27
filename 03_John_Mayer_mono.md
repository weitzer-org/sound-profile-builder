**System Initialization: QC-2 (Quad Cortex Systems Engineer)**
**Target Confirmed:** John Mayer - "Gravity" (Studio/Live Hybrid)
**Pickup Profile:** Low-Output Stratocaster (Vintage Single-Coils)
**Hardware Environment:** QSC CP12 (Active 12" PA Speaker)

### 1. Physics & Gain Staging Protocol
John Mayer’s "Gravity" tone is defined by a massive, high-headroom clean platform (Dumble Steel String Singer/Two-Rock) that pushes significant low-end while maintaining a glassy, compressed treble response. 

**The Pickup Output Compensation Strategy:**
Since you are using low-output Strat pickups, the signal hitting the Dumble preamp will be too weak, resulting in a thin, brittle tone lacking the famous Mayer "bloom." 
*   **Action:** We will increase the **Global Input Gain to +3.0dB**. This effectively "tricks" the D-Cell Amp block into reacting as if you have slightly hotter, perfectly balanced pickups, providing the correct tube compression without early clipping.
*   **Speaker Constraint (QSC CP12):** Mayer's tone has huge bass. Pushing a full-range 12" PA speaker with vintage amp bass frequencies will cause woofy phase issues and eat up amplifier headroom. We will utilize a strict High-Pass Filter (HPF) at 85Hz in the EQ block to keep the CP12 punching clearly in the low-mids without "farting out."

---

### 2. The Split-Bank Matrix (Single-Coil Profile)
*   **Scene A (Rhythm):** Pristine Clean. Position 4 (Neck/Middle) or pure Neck pickup. Fingerpicked or light pick attack. 
*   **Scene B (Lead):** Myth Drive (Klon) Engaged. Pushes the low-mids for that thick, vocal sustaining solo tone. +1.5dB output jump.
*   **Scene C (Dry):** Reverb/Delay bypassed for tight comping.
*   **Scene D (Ambient):** Massive Plate Reverb decay for the atmospheric outro.

---

### 3. Signal Chain Execution (Table A)

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB <br>Gain: +3.0dB | Thresh: -65dB <br>Gain: +3.0dB | Low-output Strat needs a +3.0dB boost to hit the D-Cell tube grid for optimal harmonic bloom. |
| **Pre-FX 1** | Parametric-8 EQ | HPF: 85Hz <br>Band 1: +2.0dB (200Hz) | HPF: 85Hz <br>Band 1: +2.0dB (200Hz) | **Chameleon Strategy:** HPF protects the QSC CP12. Band 1 shelf adds "Mayer" weight to thin single coils. |
| **Pre-FX 2** | Myth Drive <br>*(Klon Centaur)* | **BYPASSED** | **ENGAGED** *(Assign)*<br>Gain: 2.0<br>Treble: 5.5<br>Level: 7.0 | Klon logic: Low gain, high output. Acts as a clean mid-push to drive the D-Cell preamp into smooth saturation. |
| **Amp** | D-Cell ODS 100 Clean | Vol: 6.0<br>Bass: 6.5<br>Mid: 4.5<br>Treble: 5.5<br>Level: 0.0dB | Vol: 6.0<br>Bass: 6.5<br>Mid: 4.5<br>Treble: 5.5<br>Level: +1.5dB *(Assign)* | Mids are slightly scooped (4.5) to mimic the "glassy" Fender-style Dumble clean. Lane Output boosted for solo. |
| **Cab** | 412 D-Cell | Mic A (Dyn 57): Pos 0.5<br>Mic B (Rib 121): Pos 1.2 | Mic A: 0.0dB<br>Mic B: -2.0dB | 57 captures the snappy pick attack; 121 Ribbon positioned off-center warms up the highs to prevent "ice-pick" tones. |
| **Post-FX 1** | Analog Delay | Mix: 15% <br>Time: ~320ms <br>Fdbk: 15% | Mix: 15% <br>Time: ~320ms <br>Fdbk: 15% | Very subtle slap/thickener. Keeps the solo from sounding too dry in a live setting without muddying the mix. |
| **Post-FX 2** | Plate Reverb | Mix: 25% <br>Decay: 2.2s | Mix: 25% <br>Decay: 2.2s | Mimics the EMT Plate reverbs used on the *Continuum* studio sessions. Lush and wide. |

---

### 4. Troubleshooting & Refinement Tree
If you load this preset and the tone feels incorrect, follow this strict protocol:
1.  **Too Spiky/Harsh on the High E String?** 
    *   *Fix:* Go to the **Parametric-8 EQ** and activate Band 8 as a Low Pass Filter (LPF). Set it to `4800Hz`. This will roll off the very top end without killing the "glassy" presence.
2.  **Solo Tone is Too Distorted (Fuzzy)?**
    *   *Fix:* Lower the **Level** on the Myth Drive from `7.0` to `5.5`. The Klon is hitting the D-Cell preamp too hard. 
3.  **Low E String Sounds "Flubby" or Farty?**
    *   *Fix:* Increase the HPF on the Parametric-8 EQ from `85Hz` to `100Hz`, and reduce the Amp Block Bass from `6.5` to `5.0`. 

---

### 5. Session Registry Protocol
*Appending build configuration to the local session memory for future recall.*

**Session Library (Active Presets)**
**2. Preset Name:** "Gravity - Strat"
**Target:** John Mayer (2006).
**Guitar:** Stratocaster (Low-Output Single Coils) w/ Pick & Fingers.
**Physics Goal:** Massive high-headroom clean platform + Klon mid-push. HPF implemented for QSC CP12 active wedge protection.
**Full Configuration:**
*   **Block 1 (Input/Gate):** Global Gate, Thresh [-65dB], Input Gain [+3.0dB].
*   **Block 2 (EQ-8):** HPF [85Hz], Band 1 Low Shelf [+2.0dB @ 200Hz], Band 2-7 [Flat], LPF [Off].
*   **Block 3 (Myth Drive):** Bypass [Sc A: Off / Sc B: On], Gain [2.0], Treble [5.5], Output [7.0].
*   **Block 4 (Amp - D-Cell ODS 100 Clean):** Vol [6.0], Bass [6.5], Mid [4.5], Treble [5.5], Master [7.0], Presence [5.0], Level [Sc A: 0.0dB / Sc B: +1.5dB].
*   **Block 5 (Cab - 412 D-Cell):** Mic A (Dyn 57, Pos 0.5, Dist 1.0", Lvl 0dB), Mic B (Rib 121, Pos 1.2, Dist 2.5", Lvl -2dB).
*   **Block 6 (Analog Delay):** Mix [15%], Time [320ms], Fdbk [15%].
*   **Block 7 (Plate Reverb):** Mix [25%], Decay [2.2s], Pre-Delay [15ms], HP [150Hz], LP [4000Hz].