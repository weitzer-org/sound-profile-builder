**QC-2 Systems Engineer Active.** 
Initializing "Physics First" protocol for Cortex Control (Mac Desktop) mapped to QSC CP12 active PA speaker. 

Target acquired: Modern high-gain metal rhythm (Tight low end, scooped mids). 
Closest Available Model to Peavey 5150: **PV-505 Lead**.

To achieve a true modern metal tone without muddying the QSC CP12's DSP, we must utilize the "Pre-Drive Low-Cut" physics trick. By placing a tube screamer circuit (Green 808) with zero drive and max level in front of the amp, we aggressively cut the sub-bass frequencies *before* they hit the high-gain preamp tubes, preventing the low end from becoming "flubby" or "farty." 

Per protocol, here is the Split-Bank Matrix configuration to adapt this aggressive tone for both a Fender Telecaster (Single Coils) and a Gibson ES-339 (Semi-hollow Humbuckers).

---

### Pickup Output Compensation Strategy

**Gibson ES-339 (Medium/High Output Humbuckers + Semi-Hollow)**
*   **Physics Challenge:** The ES-339's resonant chamber will instantly create microphonic howling and uncontrolled low-mid feedback under PV-505 gain stages. 
*   **Action:** Set Global Input Gain to **-3.0dB**. This acts as an input pad to tame the semi-hollow bloom. Adaptive Gate is set aggressively.

**Fender Telecaster (Low/Medium Output Single Coils)**
*   **Physics Challenge:** Lacks the output to push the PV-505 into heavy saturation; bridge pickup has an ice-pick "twang" that sounds brittle under high gain. 
*   **Action:** Set Global Input Gain to **+3.0dB**. Post-EQ will boost the ~150Hz "body" frequency and apply a Low-Pass Filter (LPF) to chop off the harsh high-frequency pick attack.

---

### Table A: Main Signal Chain & Split-Bank Matrix
*Assign Scene changes via Cortex Control: Right-Click > Assign to Scene.*

*   **Row 1 (Scenes A-D):** Fender Telecaster (Single Coil)
*   **Row 2 (Scenes E-H):** Gibson ES-339 (Humbucker)

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Grid Gate** | Adaptive Gate | **Tele (A):** Red: 60%, Thresh: -50dB<br>**339 (E):** Red: 75%, Thresh: -45dB | **Tele (B):** Red: 50%, Thresh: -55dB<br>**339 (F):** Red: 60%, Thresh: -50dB | Using % Adaptive Gate for fast "chug" cutoff. 339 requires stricter gating due to semi-hollow feedback. |
| **Pre-FX** | Green 808 | Drive: 0.0, Tone: 7.5, Level: 10.0 | Drive: 0.0, Tone: 6.0, Level: 10.0 | "The Tightener". Cuts bass before the amp block to prevent low-end tube flub. Tone rolled back slightly for leads to thicken high notes. |
| **Amp** | PV-505 Lead | **Tele (A):** Pre Gain: 5.5<br>**339 (E):** Pre Gain: 4.0<br>*Both:* Low: 6.0, Mid: 3.5, High: 5.5, Post Gain: 4.0 | **Tele (B):** Pre Gain: 6.5<br>**339 (F):** Pre Gain: 5.0<br>*Both:* Low: 5.5, Mid: 6.5, High: 5.0, Post Gain: 4.5 | Tele requires more Pre-Gain. Rhythm features scooped mids (3.5). Lead pushes mids (6.5) and Post Gain (Master Vol) for +1.5dB output jump to cut the mix. |
| **Cab** | 412 US Mega V30 | Mic A: Dyn 57 (Pos 0.4, Dist 1.0")<br>Mic B: Ribbon 121 (Pos 0.8, Dist 3.0") | *Same as Rhythm* | Oversized Mesa-style 4x12. Dyn 57 provides the aggressive bite; Ribbon 121 adds warmth and tames the digital fizz for the QSC CP12. |
| **Post-EQ** | Parametric-8 | **Tele (A):** Band 2 (150Hz) +4dB, Band 4 (500Hz) -3dB, LPF 4.5kHz.<br>**339 (E):** Band 2 (150Hz) -3dB, Band 4 (500Hz) -4dB. | **Tele (B):** LPF 5.5kHz. Band 5 (800Hz) +3dB.<br>**339 (F):** Band 5 (800Hz) +4dB. | **The Chameleon Block:** Tele gets +4dB at 150Hz to simulate humbucker weight. 339 gets -3dB at 150Hz to stop body boominess. Rhythm gets 500Hz scoop; Lead gets 800Hz mid-hump. |
| **Post-FX** | Delay / Reverb | **Delay:** Bypass<br>**Room Rev:** Mix 8%, Decay 0.8s | **Dual Delay:** Mix 20%, Time 1/8d<br>**Room Rev:** Mix 20%, Decay 1.5s | Dry and tight for rhythm chugs. Washy and sustained for solos to mask fret noise. |

*(Note: Scenes C/G bypass Delay/Reverb entirely for ultimate dry tracking. Scenes D/H push Reverb Mix to 45% for Ambient swells).*

---

### Troubleshooting & Refinement Tree
If the QSC CP12 sounds **"Too Distorted / Harsh / Fuzzy"**:
1.  **Input Pad:** Verify your Global Input Gain. Drop it by another -3.0dB. (This simulates rolling your guitar volume knob down 10%).
2.  **Amp Gain:** Lower the `Pre Gain` on the PV-505 by 1.0 increments. Do *not* lower the Green 808 level (it needs to hit the amp hard to keep the bass tight).
3.  **Speaker Fizz:** If the QSC tweeter is too harsh, go to the Cab Block -> Mic A (Dyn 57) and move the `Position` from 0.4 further toward the edge (e.g., 0.7). 
4.  **Output Compensation:** If the tone cleans up but is now too quiet, raise the `Lane Output Level` (the circle at the far right of the grid) by +2.0dB. Do not touch amp gain.

---

### Session Library (Active Presets)

**2. Preset Name: "PV-505 Chug - Multi-Scale"**
*   **Target:** Modern High-Gain Metal (Peavey 5150 characteristic).
*   **Guitar(s):** Fender Telecaster (Single) / Gibson ES-339 (Semi-Hollow Humbucker).
*   **Physics Goal:** Razor-tight low end using pre-amp overdrive low-cut; taming ES-339 feedback via input padding; artificially thickening Telecaster via Parametric EQ body boost.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Tele Rhy: 60% / Lead: 50% | 339 Rhy: 75% / Lead: 60%], Thresh [Tele: -50dB / 339: -45dB].
    *   **Block 2 (Green 808):** Drive [0.0], Tone [7.5 / Lead: 6.0], Level [10.0].
    *   **Block 3 (Amp - PV-505 Lead):** Pre Gain [Tele Rhy: 5.5 / Lead: 6.5 | 339 Rhy: 4.0 / Lead: 5.0], Low [Rhy: 6.0 / Lead: 5.5], Mid [Rhy: 3.5 / Lead: 6.5], High [Rhy: 5.5 / Lead: 5.0], Post Gain [Rhy: 4.0 / Lead: 4.5].
    *   **Block 4 (Cab - 412 US Mega V30):** Mic A (Dyn 57, Pos 0.4, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 3.0"), Mix [A: 0dB, B: -3dB].
    *   **Block 5 (Parametric-8 EQ):** 
        *   *Tele:* HPF [80Hz], Band 2 [150Hz, +4.0dB], Band 4 [500Hz, Rhy: -3.0dB / Lead: 0.0dB], Band 5 [800Hz, Rhy: 0.0dB / Lead: +3.0dB], LPF [Rhy: 4500Hz / Lead: 5500Hz].
        *   *ES-339:* HPF [100Hz], Band 2 [150Hz, -3.0dB], Band 4 [500Hz, Rhy: -4.0dB / Lead: 0.0dB], Band 5 [800Hz, Rhy: 0.0dB / Lead: +4.0dB], LPF [8000Hz].
    *   **Block 6 (Dual Delay):** Mix [Rhy: Bypass / Lead: 20%], Time [1/8 dotted], Fdbk [25%].
    *   **Block 7 (Room Reverb):** Mix [Rhy: 8% / Lead: 20%], Decay [Rhy: 0.8s / Lead: 1.5s], HP [120Hz], LP [4000Hz].