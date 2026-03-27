### Quad Cortex Systems Engineer (QC-2) 
**Target Verification:** Joe Bonamassa Signature High-Gain Lead. 
**Physics Goal:** To achieve Joe’s "flute-like" neck pickup sustain and biting bridge attack, we must utilize a **Dual-Amp Routing Architecture**. By running a transient-rich, high-headroom Tweed circuit in parallel with a highly compressed, liquid Dumble circuit, we simulate his legendary live rig blend. 

---

### Part 1: Gain Staging & Pickup Compensation

**Guitar 1: Gibson ES-339 (Humbuckers)**
*   **Physics Problem:** Humbuckers into a jumped Tweed and a cascading-gain Dumble will easily hit digital clipping and sound "woofy" or "farty" in the low-mids (250Hz-400Hz).
*   **QC Input Strategy:** Set Global Input Gain to **-2.5dB**. This simulates rolling back the guitar's volume slightly to let the amps breathe.
*   **Tone Strategy:** Utilize the neck pickup with the tone knob rolled to 7 for the classic Bonamassa "violin" sustain. 

**Guitar 2: Fender Telecaster (Single Coils)**
*   **Physics Problem:** Single coils lack the preamp-driving voltage of humbuckers and possess a resonant peak around 3kHz-5kHz that will sound harsh ("ice-pick") through the Dumble's overdrive stage.
*   **QC Input Strategy:** Set Global Input Gain to **+3.0dB** to hit the amp's V1 tubes with humbucker-like voltage. 
*   **Tone Strategy:** Use the EQ-8 block (The "Chameleon") to artificially inject lower-midrange "wood" into the Telecaster body, and aggressively low-pass the high frequencies.

---

### Part 2: Split-Bank Scene Matrix

*   **Row 1 (Scenes A-D) - Fender Telecaster Profile:**
    *   **Scene A:** Tweed/Dumble Rhythm (Edge of Breakup, tight).
    *   **Scene B:** Bonamassa Lead (+1.5dB Output, Green 808 engaged, Mid-bump).
    *   **Scene C:** Dry Lead (No Reverb/Delay for fast alternate picking).
    *   **Scene D:** Ambient Volume Swells.
*   **Row 2 (Scenes E-H) - Gibson ES-339 Profile:**
    *   **Scene E:** Tweed/Dumble Rhythm (Cleaner, reduced amp drive to compensate for humbuckers).
    *   **Scene F:** Bonamassa Lead (+1.5dB Output, endless sustain).
    *   **Scene G:** Dry Lead (No spatial FX).
    *   **Scene H:** Ambient Volume Swells.

---

### Part 3: Main Signal Chain Build
*(Note: Build this using a parallel split path in Cortex Control. Path 1: Tweed. Path 2: Dumble. Merge them at the Cab Block).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Gate** | Adaptive Gate | Red: 35% / Thresh: -55dB | Red: 15% / Thresh: -65dB | Lower threshold on Lead allows sustained notes to decay naturally into feedback. |
| **Pre-FX (Drive)** | Green 808 | Bypass *(Assign)* | Active: Gain 1.5, Level 8.5, Tone 6.0 | Pushes the midrange (800Hz) into both amps, tightening the low-end for leads. |
| **Amp A (Parallel 1)** | US Tweed HW Jumped | Vol Norm: 4.0, Vol Brt: 4.5, Bass: 3.0, Treble: 6.0, Pres: 5.5 | Vol Norm: 6.0, Vol Brt: 6.5, Bass: 2.5, Treble: 6.5, Pres: 6.0 | *No Master Vol.* Provides the massive, uncompressed transient pick attack and low-end girth. |
| **Amp B (Parallel 2)** | ODS 100 Lead | OD: 4.5, Level: 5.0, Bass: 4.0, Mid: 6.0, Treble: 5.0 | OD: 7.5, Level: 5.5, Bass: 3.5, Mid: 7.5, Treble: 5.5 | Provides the liquid, compressed "Dumble" sustain. Midrange bump pushes it forward in the mix. |
| **Cab (Dual)** | 212 US Tweed HW + 412 Zila Custom | Tweed: Mic 121 (Pos 0.5, Dist 2") Zila: Mic 57 (Pos 1.0, Dist 1") | *(Same)* | Ribbon mic on the Tweed captures body; 57 on the Zila EV-style speakers captures the aggressive cut. |
| **Post-FX (EQ)** | EQ-8 (Parametric) | *See Guitar Breakdown Below* | *See Guitar Breakdown Below* | "Chameleon" strategy applied here to adapt to Tele vs. ES-339. |
| **Post-FX (Delay)** | Analog Delay | Mix: 15%, Time: 350ms, Fdbk: 20%, Tone: 4.0 | Mix: 25%, Time: 420ms, Fdbk: 35%, Tone: 5.0 | Dark analog delay thickens the lead tone without stepping on the dry signal transients. |
| **Post-FX (Reverb)**| Plate Reverb | Mix: 12%, Decay: 1.2s, Pre-Delay: 15ms | Mix: 18%, Decay: 1.8s, Pre-Delay: 25ms | Simulates Joe's use of studio plates or outboard tanks for a subtle 3D halo. |

---

### Part 4: The EQ-8 "Chameleon" Guitar Breakdown
Assign the following Parameters to Scenes via Right-Click.

**For Fender Telecaster (Scenes A & B):**
*   **Band 1 (Body Boost):** +3.5dB @ 200Hz (Adds humbucker-like weight).
*   **Band 3 (Mud Cut):** -1.5dB @ 400Hz (Prevents the 200Hz boost from sounding boxy).
*   **Band 6 (Bite):** +1.0dB @ 2500Hz (Accentuates pick attack).
*   **High-Pass (HPF):** 90Hz.
*   **Low-Pass (LPF):** 4500Hz (Crucial: Cuts the harsh single-coil fizz and mimics the rolled-off tone knob effect).

**For Gibson ES-339 (Scenes E & F):**
*   **Band 1 (Body Trim):** -1.0dB @ 200Hz (Tames the natural low-end bloom of the semi-hollow).
*   **Band 3 (Clarity):** -2.5dB @ 350Hz (Clears up the "woof" from the neck humbucker).
*   **Band 6 (Presence):** +2.5dB @ 3000Hz (Adds articulation back to the humbucker).
*   **High-Pass (HPF):** 110Hz (Tighter low-end for high-gain).
*   **Low-Pass (LPF):** 6500Hz (Allows the natural harmonics of the humbucker to breathe).

---

### Part 5: Troubleshooting & Refinement Tree
If the tone isn't reacting perfectly to your specific guitar, follow this strict protocol:

1.  **"It sounds too fuzzy/farty on the low strings."**
    *   *Fix:* Go to the **US Tweed HW Jumped** block and lower the Bass parameter by 1.0. Tweed circuits do not have negative feedback, meaning low frequencies distort unpleasantly if pushed too hard.
2.  **"I'm not getting that endless sustain."**
    *   *Fix:* First, increase your ES-339 or Tele's physical volume knob to 10. Second, increase the **Green 808 Level** to 10. Do *not* add a compressor; you will raise the noise floor and ruin the dynamics.
3.  **"The amps are clipping the Quad Cortex outputs (Red LED)."**
    *   *Fix:* Lower the overall **Lane Output Level** block on the far right of the grid by -3.0dB. Do not lower the Amp Block Volumes, as this will change the tube saturation physics.

---

### Session Library (Active Presets)

**2. Preset Name: "JB Dumble-Tweed Matrix"**
*   **Target:** Joe Bonamassa live lead tone (High Power Twin + ODS 100).
*   **Guitar:** Gibson ES-339 / Fender Telecaster.
*   **Physics Goal:** Blend transient Tweed headroom with heavily saturated, mid-focused Dumble overdrive.
*   **Full Configuration:**
    *   *Block 1 (Adaptive Gate):* Noise Red [35% / 15%], Thresh [-55dB / -65dB].
    *   *Block 2 (Green 808):* Bypass/Active, Gain [1.5], Level [8.5], Tone [6.0].
    *   *Block 3 (Amp A - US Tweed HW Jumped):* Vol Norm [4.0 / 6.0], Vol Bright [4.5 / 6.5], Bass [3.0 / 2.5], Treble [6.0 / 6.5], Pres [5.5 / 6.0].
    *   *Block 4 (Amp B - ODS 100 Lead):* OD [4.5 / 7.5], Level [5.0 / 5.5], Bass [4.0 / 3.5], Mid [6.0 / 7.5], Treble [5.0 / 5.5].
    *   *Block 5 (Cab Split):* Cab A (212 US Tweed HW, Mic 121, Pos 0.5, Dist 2"). Cab B (412 Zila Custom, Mic 57, Pos 1.0, Dist 1").
    *   *Block 6 (EQ-8):* Tele [LPF 4500Hz, B1 +3.5dB(200Hz)]. ES-339 [LPF 6500Hz, B1 -1.0dB(200Hz), B3 -2.5dB(350Hz)].
    *   *Block 7 (Analog Delay):* Mix [15% / 25%], Time [350ms / 420ms], Fdbk [20% / 35%].
    *   *Block 8 (Plate Reverb):* Mix [12% / 18%], Decay [1.2s / 1.8s], Pre-Delay [15ms / 25ms].