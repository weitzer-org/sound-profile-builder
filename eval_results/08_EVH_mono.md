Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To accurately capture Eddie Van Halen's "Brown Sound" (circa 1978), we must recreate a specific physics environment: a voltage-starved, fully-cranked Marshall Super Lead (Plexi) driving Celestion Greenbacks. Because a vintage Plexi has **no master volume**, all preamp gain comes from the Normal/Bright volume knobs. We will control your actual stage loudness (SPL) exclusively via the Lane Output Level. 

Additionally, EVH famously ran his MXR Phase 90 *in front* of his distorted amp, which compresses the modulation and creates that signature thick, chewing sweep (e.g., "Eruption").

Below is the execution protocol, distinctly split for your Gibson ES-339 and Fender Telecaster.

---

### 4. Organization Standard (Split-Bank Matrix)
We will utilize an 8-Scene layout. Right-click the parameters marked below to assign them to Scenes.
*   **Row 1 (Telecaster - Single Coil):** Scenes A-D
*   **Row 2 (ES-339 - Humbucker):** Scenes E-H

*Scene Breakdown:*
*   **A / E:** Rhythm (Plexi cranked, Plate Reverb).
*   **B / F:** Lead (Adds Phaser, Tape Delay, +1.5dB Output).
*   **C / G:** Flanger Rhythm ("Unchained" / "Hear About It Later" style).
*   **D / H:** Dry/Comping (Modulation/Time FX bypassed).

---

### 5. Execution Protocol: Main Signal Chain

**Table A: Main Signal Chain (Global Target Build)**
*Apply these blocks from left to right on the grid.*

| Block Category | Model Name | Rhythm/Flange Settings (Sc A,C,E,G) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 40% (Right-Click) | Noise Red: 20% (Right-Click) | EVH tone is noisy, but clamping too hard kills harmonics. Looser gate for leads. |
| **Pre-FX (Mod)** | Phaser 95 | Bypass (On for Sc C/G) | Rate: 2.5, Mix: 45% (Active) | Placed *before* the amp. Compresses the sweep into the tube distortion. |
| **Pre-FX (Mod)** | Flanger | Bypass (Active on Sc C/G) | Bypass | Pre-amp flanger creates intense jet-engine sweep ("Unchained" physics). |
| **Amp** | Brit Plexi 100 Jump | Vol Bright: 8.5, Vol Norm: 4.0 | Vol Bright: 8.5, Vol Norm: 4.0 | No Master Vol. Jumped channels blend bright attack with low-end body. Bass set to 3.0 to prevent sag/farting. |
| **EQ** | Parametric-8 | *(See Multi-Guitar Specs Below)* | *(See Multi-Guitar Specs Below)* | Formant matching between Telecaster and ES-339 pickups. |
| **Cab** | 412 Brit M25 | Mic A: Dyn 57 (Pos 0.5) | Mic A: Dyn 57 (Pos 0.5) | Simulates G12M 25W Greenbacks. 57 handles the bite, Ribbon adds physical cabinet weight. |
| **Post-FX (Echo)**| Tape Delay | Bypass | Mix: 20%, Time: 300ms (Active) | Slap/thickening echo simulating EVH's Echoplex bombast. |
| **Post-FX (Rev)** | Plate Reverb | Mix: 15%, Decay: 1.5s | Mix: 18%, Decay: 1.5s | Emulates the Sunset Sound Plate reverb chamber used on VH1. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB (Right-Click) | Headroom Rule: Gain comes from Amp Vol; SPL boosts come from the Lane. |

---

### 9. Multi-Guitar Target Output

Due to the extreme difference in resonant frequency and output voltage between a Gibson ES-339 (PAF-style Humbuckers) and a Fender Telecaster (Single Coils), you **must** apply the following compensations to prevent the humbuckers from turning to mud, and the single coils from sounding thin/piercing.

#### 🎸 Guitar 1: Gibson ES-339 (Humbuckers)
*Targeting Row 2 (Scenes E-H)*
EVH's original "Frankenstrat" utilized a PAF humbucker ripped from an ES-335. Your ES-339 is naturally close to the input voltage and midrange focus required.
1.  **Global Input Pad:** Leave at **0.0dB**.
2.  **Adaptive Gate:** Threshold set to **-60dB**. Humbuckers are inherently quieter regarding 60-cycle hum.
3.  **EQ-8 Block Configuration:** 
    *   **HPF (High Pass):** 80Hz (Tightens the low end, stopping the semi-hollow body resonance from flubbing the Plexi).
    *   **Band 6 (Upper Mid):** +1.5dB at 2.5kHz (Adds pick attack clarity to combat humbucker warmth).
    *   **LPF (Low Pass):** 6000Hz (Tames any digital fizz from the FRFR speaker).

#### 🎸 Guitar 2: Fender Telecaster (Single Coils)
*Targeting Row 1 (Scenes A-D)*
Single coils lack the inductance to properly push the V1 preamp tube of a Plexi into "Brown Sound" territory. We must electronically simulate a humbucker's physical characteristics.
1.  **Global Input Pad:** Increase to **+3.0dB**. This physically hits the amp block harder, creating the required saturation without changing the amp's tone stack.
2.  **Adaptive Gate:** Threshold raised to **-52dB**. Single coils under this much gain will induce 60-cycle noise; the gate must work harder.
3.  **EQ-8 Block Configuration:**
    *   **Band 1 (Body Boost):** +3.5dB Low Shelf at 200Hz. (Critically important: adds the physical "thump" of a humbucker).
    *   **Band 3 (Mid Push):** +2.0dB Peak at 800Hz. (Fills in the scooped midrange of a Telecaster).
    *   **LPF (Low Pass):** 4500Hz. (Aggressively trims the "ice-pick" high frequencies typical of Tele bridge pickups when driving a Plexi).

---

### 6. Troubleshooting & Refinement Tree
If testing through your QSC CP12 yields unsatisfactory results, execute the following in order:
*   **"The low-end sounds like a broken speaker (Flubby/Farty)":** You are experiencing tube sag physics. Reduce the `Vol Normal` on the Amp block to 2.0, and drop the Amp `Bass` to 1.5. 
*   **"The Phaser sounds too subtle":** Ensure the Phaser 95 block is physically placed *to the left* of the Amp block on the grid. EVH ran his phaser into the front of the amp; placing it post-amp will sound like an 80s studio sheen, not vintage VH.
*   **"The tone is too fuzzy/harsh":** Lower the Global Input Gain by -3.0dB (simulating rolling off your guitar's volume pot). 

---

### 8. Preset Registry Protocol (Session Memory)

**Session Library (Active Presets)**
1. Preset Name: "Spoonful - ES339" [PREVIOUSLY LOGGED]
2. Preset Name: "Brown Sound EVH 78"
*   Target: Eddie Van Halen (1978).
*   Guitars: Gibson ES-339 (Humbuckers) & Fender Telecaster (Single Coils).
*   Physics Goal: Voltage-starved Plexi saturation, pre-amp modulation compression, non-master volume headroom control.
*   Full Configuration:
    *   Block 1 (Adaptive Gate): Noise Red [Rhy: 40% / Lead: 20%], Thresh [-60dB (ES339) / -52dB (Tele)], Decay [150ms].
    *   Block 2 (Phaser 95): Bypass [Scenes A,C,D,E,G,H], Active [Scenes B/F], Rate [2.5], Mix [45%].
    *   Block 3 (Flanger): Active [Scenes C/G], Rate [4.0], Width [70%], Regen [65%], Mix [50%].
    *   Block 4 (Amp - Brit Plexi 100 Jump): Vol Norm [4.0], Vol Bright [8.5], Bass [3.0], Mid [6.5], Treble [7.0], Presence [7.5].
    *   Block 5 (EQ-8 - Telecaster Profile): Input [+3.0dB], Low Shelf [+3.5dB @ 200Hz], Peak [+2.0dB @ 800Hz], LPF [4500Hz]. 
    *   Block 5.1 (EQ-8 - ES-339 Profile): Input [0.0dB], HPF [80Hz], Peak [+1.5dB @ 2.5kHz], LPF [6000Hz].
    *   Block 6 (Cab - 412 Brit M25): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 2.5"), Mix [A: 0dB, B: -2dB].
    *   Block 7 (Tape Delay): Bypass [Rhy], Active [Lead], Mix [20%], Time [300ms], Fdbk [25%].
    *   Block 8 (Plate Reverb): Mix [15% / Lead: 18%], Decay [1.5s], Pre-Delay [20ms], HP [150Hz], LP [4000Hz].
    *   Output: Lane 1 Output [Rhy: 0.0dB / Lead: +1.5dB].