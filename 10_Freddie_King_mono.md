### QC-2 Systems Engineering Report: "Hideaway" - Freddie King (1960)
**Target Tone:** Freddie King, early 1960s instrumental blues. Sharp, biting, plastic-thumb-pick attack with a hollowed-out midrange.
**User Hardware:** Gibson ES-345 (Humbuckers with Varitone) into QSC CP12 (Active PA).

#### Physics & Gain Staging Protocol
The ES-345 humbuckers will inherently push a vintage Tweed/Brownface circuit into premature distortion, losing the "acoustic snap" crucial to Freddie's picking style. The QSC CP12 has plenty of headroom, so we will rely on **Lane Output Level** for volume, keeping the **Amp Volume low** to retain transient punch. 

The "Acoustic Snap" is simulated via a combination of a precise midrange notch (simulating Varitone Position 2 or 3) and an ultra-fast, zero-feedback Tape Delay (simulating the mechanical *thwack* of the string hitting the frets and resonating the semi-hollow body).

**Pickup Verification:** Are your ES-345 pickups Vintage Output (PAFs) or modern High Output? *This build assumes Vintage/Medium PAFs. If they are modern/hot, reduce Global Input Gain to -3.0dB.*

---

#### Organization Standard (Split-Bank Matrix)
*   **Row 1 (Scenes A-D) - Single Coil Base (Tele/Strat):** Bypass Varitone EQ, increase Amp Volume by +1.5 to compensate for lower pickup output.
*   **Row 2 (Scenes E-H) - ES-345 Humbucker Base (Primary Focus):** 
    *   **Scene E (Rhythm/Theme):** Varitone Notch active, 75ms Snap Delay active.
    *   **Scene F (Lead/Solo):** Varitone Notch flattened (more body), Amp Bright Volume increased.
    *   **Scene G (Dry/Comping):** Snap Delay and Reverb Bypassed.
    *   **Scene H (Ambient/FX):** Spring Reverb mix increased.

---

#### Table A: Main Signal Chain
*(Right-Click > Assign parameters for Scene E and Scene F below)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -65dB, Red: 30% | Thresh: -65dB, Red: 20% | Humbuckers need less reduction. Fast decay allows the "snap" transient to survive. |
| **Pre-FX (Varitone)** | Parametric-8 EQ | Band 4: 700Hz, -7.0dB, Q: 2.5<br>Out Level: +2.0dB | Band 4: 700Hz, -2.0dB, Q: 2.0<br>Out Level: 0.0dB | Simulates Varitone Pos 2/3 (hollow mid-scoop). Output level compensates for passive circuit volume loss. |
| **Amp** | US Tweed Basslad Bright | Vol Bright: 3.5<br>Bass: 2.5<br>Treble: 6.5 | Vol Bright: 5.0<br>Bass: 3.0<br>Treble: 6.0 | Tweed Bassman pushed to edge-of-breakup. Bass kept low to prevent humbucker "farting" (tube sag limits). |
| **Cab** | 410 US Basslad PR10 | Mic A (Dyn 57): Pos 0.3, Dist 1.0"<br>Mic B (Rib 121): Pos 0.8, Dist 3.0" | *(Same)* | 57 adds the cutting attack (thumb pick), 121 catches the semi-hollow body resonance. |
| **Post-FX (Snap)** | Tape Delay | Time: 75ms<br>Fdbk: 0%<br>Mix: 18% | Time: 75ms<br>Fdbk: 0%<br>Mix: 12% | 0% feedback = exactly one repeat. 75ms creates a "slap" that tricks the ear into hearing a percussive acoustic body snap. |
| **Post-FX (Verb)** | Spring Reverb | Mix: 15%<br>Tone: 6.0 | Mix: 18%<br>Tone: 5.5 | Period-accurate splash. Kept low to avoid washing out the fast 16th-note turnaround licks. |

---

#### Troubleshooting & Refinement Tree (QSC CP12 Output)
1. **If the snap sounds "metallic" or harsh through the QSC tweeter:** Go to the Parametric-8 EQ block and engage **Band 8 (LPF)**. Set it to **5000Hz**. This rolls off the digital top-end and mimics the natural roll-off of a vintage 10" speaker.
2. **If the amp sounds "broken/farty" on the low strings:** The ES-345 neck pickup is overloading the Tweed power amp simulation. Lower the **Amp Bass** parameter to 1.5 or 2.0. (Vintage Tweeds drop low-end efficiently; do not use the EQ block for this, fix it at the amp tone stack).
3. **If it's not loud enough in the room:** **DO NOT** turn up the Amp Volume (this will ruin the clean snap and add fuzz). Instead, swipe down from the top of the QC screen and raise the **Lane 1 Output Level** by +4.0dB to push the QSC CP12 harder.

---

#### Session Library (Active Presets)
*Appending to memory...*

**2. Preset Name: "Hideaway Varitone - ES-345"**
*   **Target:** Freddie King (1960). 
*   **Guitar:** Gibson ES-345 (Humbuckers) w/ Varitone.
*   **Physics Goal:** Edge-of-breakup transient punch, simulate mechanical string "snap" via ultra-short delay, emulate passive RC notch filter of Varitone circuit.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Rhy: 30% / Lead: 20%], Thresh [-65dB], Decay [50ms].
    *   **Block 2 (EQ-8 - Varitone Sim):** HPF [80Hz], Band 4 (Bell) [Freq: 700Hz, Gain: Rhy -7.0dB / Lead -2.0dB, Q: Rhy 2.5 / Lead 2.0], Band 8 (LPF) [5500Hz], Master Out [Rhy +2.0dB / Lead 0.0dB].
    *   **Block 3 (Amp - US Tweed Basslad Bright):** Vol Bright [Rhy: 3.5 / Lead: 5.0], Bass [2.5], Mid [5.5], Treble [Rhy: 6.5 / Lead: 6.0], Presence [6.0].
    *   **Block 4 (Cab - 410 US Basslad PR10):** Mic A (Dyn 57, Pos 0.3, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 3.0"), Mix [A: 0dB, B: -3dB].
    *   **Block 5 (Tape Delay - Acoustic Snap):** Mix [18% / 12%], Time [75ms], Sync [Off], Fdbk [0%], Drive [10%], HP [200Hz], LP [3000Hz].
    *   **Block 6 (Spring Reverb):** Mix [15% / 18%], Decay [1.2s], Tone [6.0], Boing [4.0].