System Initialization: QC-2 (Quad Cortex Systems Engineer)
Target Identified: Guns N' Roses / Slash (Modded JCM800, Creamy Neck Pickup Lead, Standard Delay).
Output Hardware Assumed: QSC CP12 (Active 12" PA Speaker). High-frequency roll-off required to prevent FRFR tweeter fizz and emulate actual Marshall 4x12 cabinet physics.

### 1. Multi-Guitar Gain Staging & Pickup Compensation Strategy
To achieve Slash’s signature sustaining, Alnico II Les Paul neck tone, we must physically alter the signal path depending on your guitar. 

Gibson ES-339 Humbuckers (Scenes E-H)
*   **Physics Goal:** Your ES-339 likely has PAF-style humbuckers ('57 Classics). These are slightly hotter and tighter than Slash’s Alnico II Pro pickups. 
*   **Compensation:** Set Global Input Gain (Circle 1) to **-1.5dB**. This prevents the pre-amp from compressing too early, retaining the spongy, dynamic attack characteristic of Alnico II magnets. Roll your guitar's tone knob down to about 6 or 7.

Fender Telecaster Single Coils (Scenes A-D)
*   **Physics Goal:** Single coils lack the inductance, output, and low-mid resonant peak of a Les Paul neck humbucker. 
*   **Compensation:** Set Global Input Gain (Circle 1) to **+3.5dB**. We will utilize a dedicated Parametric-8 EQ block (active *only* on Scenes A-D) to artificially induce a humbucker resonant peak. Switch your Telecaster to the **Neck Pickup**.

---

### 2. Organization Standard (Split-Bank Matrix)
*   **Row 1 (Telecaster):** Scene A (Rhythm), Scene B (Slash Lead), Scene C (Dry Lead), Scene D (Ambient).
*   **Row 2 (ES-339):** Scene E (Rhythm), Scene F (Slash Lead), Scene G (Dry Lead), Scene H (Ambient).

---

### 3. Main Signal Chain (Table A)
*Note: Parameters marked with "(Assign)" require you to Right-Click > Assign to Scene in the Cortex Control Mac Desktop App.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global In 1 | Gate Thresh: -60dB | Gate Thresh: -55dB | Tighter gate on lead to control the hot JCM800 hiss when resting. |
| **Pre-FX (Tele Only)** | Parametric-8 | *Bypassed for ES-339.* | Band 2: +4.0dB @ 250Hz. LPF: 4500Hz. | Artificially thickens the single coil to mimic Les Paul neck mahogany body/humbucker output. |
| **Drive** | Green 808 | Bypassed (Off) | Active (On). Drive: 0, Tone: 6.5, Level: 8.5 | Simulates the famous SIR #36/#39 JCM800 mod by slamming the preamp tubes for liquid sustain. |
| **Amp** | Brit 800 | Preamp: 6.0<br>Vol (Master): 4.5 | Preamp: 8.0 (Assign)<br>Vol (Master): 4.5 | Slash’s tone is heavily mid-focused. Bass: 4.5, Mid: 7.5, Treble: 5.0, Presence: 4.5. |
| **Cab** | 412 Brit V30 | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") | Mic B: Ribbon 121 (Pos 1.5, Dist 1.5") | Vintage 30s are essential for GNR. The Ribbon mic smooths out the high-mids for creamy neck sweeps. Mix: A: 0dB, B: -2dB. |
| **Post-FX** | Digital Delay | Bypassed (Off) | Mix: 22% (Assign). Time: 420ms, Fdbk: 25% | Standard DD-3 style delay. High Cut (LPF) set to 2.5kHz so repeats don't clash with the solo. |
| **Post-FX** | Plate | Mix: 10%, Decay: 1.2s | Mix: 15% (Assign), Decay: 1.8s (Assign) | Replicates the studio plate reverb used on *Appetite for Destruction*. |
| **Output** | Lane 1 Out | Level: 0.0dB | Level: +1.5dB (Assign) | Lifts the solo physically above the band mix (headroom rule). |

---

### 4. Troubleshooting & Refinement Tree
If the lead tone sounds **"Too Distorted" or "Too Fuzzy"** through your QSC CP12:
1.  **Input Pad:** Lower Global Input Gain by another -3.0dB. Hot humbuckers into a boosted 800 can cause digital clipping.
2.  **Amp Gain:** Reduce the Amp Block *Preamp* parameter from 8.0 down to 6.0. 
3.  **FRFR Fizz Check:** If the CP12 sounds harsh on the high strings, go to the Cab Block and ensure the High Cut (LPF) is set strictly to 6500Hz. PA speakers reproduce frequencies up to 20kHz, whereas a real Marshall 1960B cabinet physically cannot produce sound above ~6kHz.

---

### 5. Session Registry Protocol
**Appending to Active Session Library...**

**2. Preset Name:** "Appetite Neck Lead - Dual"
*   **Target:** Slash / Guns N' Roses (1987).
*   **Guitar(s):** Gibson ES-339 (Humbuckers) & Fender Telecaster (Single Coil).
*   **Physics Goal:** High-gain, mid-focused creamy neck pickup sustain with analog-style delay trails, optimized for 12" FRFR active speakers.
*   **Full Configuration:**
    *   **Block 1 (Pre-EQ Tele Compensator):** EQ-8. Active Scenes A-D only. HPF [90Hz], Band 2 [+4.0dB @ 250Hz / Q 1.0], LPF [4500Hz]. 
    *   **Block 2 (Drive - Green 808):** Active Lead Scenes. Drive [0.0], Tone [6.5], Level [8.5].
    *   **Block 3 (Amp - Brit 800):** Preamp [Rhy: 6.0 / Lead: 8.0], Master [4.5], Bass [4.5], Mid [7.5], Treble [5.0], Presence [4.5]. Lane Output Level [Rhy: 0.0dB / Lead: +1.5dB].
    *   **Block 4 (Cab - 412 Brit V30):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 1.5"). Mix [A: 0dB, B: -2dB]. HPF [80Hz], LPF [6500Hz].
    *   **Block 5 (Delay - Digital):** Bypassed Rhythm / Active Lead. Mix [22%], Time [420ms], Feedback [25%], Low Cut [200Hz], High Cut [2500Hz].
    *   **Block 6 (Reverb - Plate):** Mix [Rhy: 10% / Lead: 15%], Decay [Rhy: 1.2s / Lead: 1.8s], Pre-Delay [20ms].