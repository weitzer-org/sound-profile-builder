**QC-2 Systems Engineer Online.**
Target acquired: Joe Bonamassa Modern Blues Lead. 
Core Physics: Smooth, high-headroom tube drive, mid-forward articulation, Dumble-style cascade gain staging.

Before we finalize the grid, I must run the **Trigger Protocol**:
*Are your pickups Vintage Output, Medium, or High Output?* (Assuming Medium output for both the ES-339 and Telecaster for this baseline configuration).

To achieve this tone through your QSC CP12 (Active FRFR PA Speaker), we must severely manage the high-frequency crossover. FRFRs reproduce high-end fizz that a traditional guitar speaker naturally rolls off. We will aggressively use High/Low Pass Filters (HPF/LPF) to simulate physical speaker impedance limits.

Here is your Split-Bank Matrix. We will use the CorOS **"Overdrive Rev OD"** (verified Dumble Overdrive Special pseudonym) pushed by a **"Green 808"** (Tube Screamer).

### 🎸 MULTI-GUITAR TARGET SEPARATION & GAIN STAGING

**Row 1 (Scenes A-D): Fender Telecaster (Single Coil)**
*   **Physics Goal:** Thicken the fundamental frequencies so the single coils don't sound "wiry" through the Dumble circuit. 
*   **Input Staging:** Set Global Input Gain to **+2.5dB**. 
*   **EQ Compensation:** Active Parametric-8 block inline. Band 2 (Low Shelf) +3.0dB at 200Hz (adds body). Band 8 (LPF) at 4500Hz (tames the bridge pickup ice-pick).

**Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers)**
*   **Physics Goal:** Tighten the low-mids. Dumble circuits are notorious for "farting out" in the bass frequencies when hit with neck humbuckers.
*   **Input Staging:** Set Global Input Gain to **-3.0dB** to prevent premature clipping of the Green 808 block.
*   **EQ Compensation:** Active Parametric-8 block inline. HPF set to 100Hz. Band 3 (Peak) -2.5dB at 350Hz (clears out the mud).

---

### 🎛️ TABLE A: MAIN SIGNAL CHAIN
*(Right-Click > Assign) specific parameters to Scenes A/B (Telecaster) and Scenes E/F (ES-339).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -62dB [Tele] / -58dB [339] | Thresh: -65dB [Tele] / -60dB [339] | 339 humbuckers require a tighter gate for palm mutes. Lead scenes relaxed for sustain. |
| **Pre-FX (EQ)** | Parametric-8 | *See specific EQ notes above* | *See specific EQ notes above* | The "Chameleon strategy." Re-voices the guitar to hit the amp with the optimal frequency band. |
| **Pre-FX (OD)** | Green 808 | Bypass: OFF | Bypass: ON <br>Drive: 2.0 <br>Tone: 5.5 <br>Level: 8.5 | Asymmetrical clipping. Low drive, high level pushes the amp's preamp tubes into natural, smooth compression. |
| **Amp** | Overdrive Rev OD | Drive: 3.5 [Tele] / 2.5 [339]<br>Bass: 3.5<br>Mid: 5.0<br>Treble: 4.5<br>Level: 6.0 | Drive: 5.5 [Tele] / 4.0 [339]<br>Bass: 3.5<br>Mid: 6.5 [Tele] / 6.0 [339]<br>Treble: 4.5<br>Level: +1.5dB relative | Dumble cascade logic. Pushing Mids on Lead scenes creates the Bonamassa "honk" that cuts through a mix. Lower drive for 339 to prevent fuzz. |
| **Cab** | 212 CA Duo Ch | Mic A: Ribbon 121 (Pos 0.5, Dist 1.0")<br>Mic B: Dyn 57 (Pos 1.5, Dist 2.0")<br>Mix: A at 0dB, B at -3dB | *Same as Rhythm* | Ribbon 121 provides a massive, smooth low-mid. Dyn 57 adds necessary bite. HPF 85Hz / LPF 5000Hz tames the QSC CP12 tweeter. |
| **Post-FX 1** | Analog Delay | Mix: 10%<br>Time: 350ms<br>Fdbk: 15% | Mix: 22%<br>Time: 420ms<br>Fdbk: 25% | Warm BBD repeats do not clash with the primary lead notes. Lengthened time for lead creates a wider stereo field. |
| **Post-FX 2** | Plate Reverb | Mix: 12%<br>Decay: 1.2s | Mix: 18%<br>Decay: 1.8s | Plate simulates studio reflections. Smooth tail without muddying the fast pentatonic runs. |

---

### 🔧 TROUBLESHOOTING & REFINEMENT TREE
If the lead tone feels "Too Distorted", "Fuzzy", or "Flubby" through your QSC CP12:
1. **Input Pad:** Lower the Input Block Gain to -6.0dB (This simulates rolling off your guitar volume and instantly cleans up the amp headroom).
2. **Amp Gain:** Reduce the Amp block *Drive* by 2.0 increments. Do *not* lower the Level (Master).
3. **Tube Sag/Flub:** If the neck pickup on the ES-339 sounds "farty" or broken during fast runs, reduce the Amp block *Bass* to 2.0. The Dumble circuit naturally amplifies low-end exponentially; cut bass before the gain stages.
4. **Output Compensation:** Compensate for any volume loss by raising the Lane Output Level. Never use a compressor to fix gain issues.

---

### 💾 SESSION LIBRARY (ACTIVE PRESETS)

**1. Preset Name: "Spoonful - ES339"**
Target: Howlin' Wolf / Hubert Sumlin (1960).
Guitar: ES-339 (Humbuckers) w/ Pick.
Physics Goal: Clean/Edge-of-breakup rhythm + Fuzz/Sag lead without using pedals.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 15%], Thresh [-60dB / -65dB], Decay [100ms / 250ms].
Block 2 (EQ-8): HPF [90Hz], Band 6 [0.0dB], LPF [Rhy: 4200Hz / Lead: 4500Hz] (Simulates thumb attack).
Block 3 (Amp - US Tweed Basslad Jumped): Vol Norm [2.0 / 2.2], Vol Bright [2.5 / 3.2], Bass [2.5], Mid [6.0 / 7.0], Treble [7.0 / 6.5], Presence [6.0], Output Level [+7.0dB / +8.5dB].
Block 4 (Cab - 410 Basslad PR10): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 5.0"), Mix [A: 0dB, B: -4dB].
Block 5 (Tape Delay): Mix [15% / 22%], Time [110ms], Fdbk [15%], Drive [35%], HP [150Hz], LP [2500Hz].
Block 6 (Room Reverb): Mix [12%], Decay [0.8s], HP [120Hz], LP [3500Hz].

**2. Preset Name: "JB Modern Blues - Dual Rig"** *(NEW)*
Target: Joe Bonamassa.
Guitar: Gibson ES-339 / Fender Telecaster.
Physics Goal: Smooth Dumble ODS cascading gain, mid-pushed articulate lead via FRFR speaker limiters.
Full Configuration:
Global Input: Gate [-62dB Tele / -58dB 339], Gain [+2.5dB Tele / -3.0dB 339].
Block 1 (EQ-8): Tele [Band 2: +3.0dB @ 200Hz, LPF @ 4500Hz] / 339 [HPF @ 100Hz, Band 3: -2.5dB @ 350Hz].
Block 2 (Green 808): Bypass [Rhy: Off / Lead: On], Drive [2.0], Tone [5.5], Level [8.5].
Block 3 (Amp - Overdrive Rev OD): Drive [Rhy: 3.5(T)/2.5(339) / Lead: 5.5(T)/4.0(339)], Bass [3.5], Mid [Rhy: 5.0 / Lead: 6.5(T)/6.0(339)], Treble [4.5], Level [Rhy: 6.0 / Lead: +1.5dB].
Block 4 (Cab - 212 CA Duo Ch): Mic A (Ribbon 121, Pos 0.5, Dist 1.0"), Mic B (Dyn 57, Pos 1.5, Dist 2.0"), Mix [A: 0dB, B: -3dB], HPF [85Hz], LPF [5000Hz].
Block 5 (Analog Delay): Mix [Rhy: 10% / Lead: 22%], Time [Rhy: 350ms / Lead: 420ms], Fdbk [Rhy: 15% / Lead: 25%].
Block 6 (Plate Reverb): Mix [Rhy: 12% / Lead: 18%], Decay [Rhy: 1.2s / Lead: 1.8s].