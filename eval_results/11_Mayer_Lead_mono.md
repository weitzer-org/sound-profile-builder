**QC-2 Systems Engineer Terminal**
**Target Acquired:** John Mayer - "Slow Dancing in a Burning Room"
**Amp Lineage:** Dumble ODS Clean / Two-Rock Custom Reverb Signature
**Speaker Profile:** QSC CP12 (FRFR/Active PA) — *Note: Relying heavily on Cab Block for frequency roll-off to prevent tweeter harshness.*

### 1. Physics First: The "Slow Dancing" Protocol
John Mayer's signature tone on this track is notoriously defined by an underwound Fender Stratocaster in Position 4 (Neck/Middle). This produces a massive frequency scoop in the 400Hz-600Hz range, a hollow, bouncy low-end, and a glassy, uncompressed high-end. 

To recreate this pristine clean through a QSC CP12 without sounding sterile, we are utilizing the **Custom Clean** (Dumble ODS Clean channel) model. It provides immense headroom and a very fast transient response. 

### 2. Multi-Guitar Target Output & Pickup Compensation
You requested this build for two entirely different physics profiles. We will use Cortex Control's Scene functionality to dynamically alter the Input Gain, Amp EQ, and Parametric EQ blocks depending on which guitar you are holding.

*   **Row 1 (Scenes A-D) - Fender Telecaster (Single Coil):** 
    *   *Physics:* The Telecaster in middle position (parallel) is close to the Strat Pos 4, but inherently has more midrange punch and a spikier transient attack. We will apply a moderate mid-scoop to hollow it out and slightly compress the front end.
*   **Row 2 (Scenes E-H) - Gibson ES-339 (Humbuckers):**
    *   *Physics:* Humbuckers will immediately clip the pristine Dumble preamp model and introduce a 250Hz-400Hz "mud" that destroys the glassy Mayer vibe. 
    *   *Solution:* We must pad the Input Block Gain by **-4.5dB** (Scene assigned to Row 2) to simulate rolling back a volume pot, restoring headroom. We will use the Parametric-8 EQ to drastically cut the low-mids and artificially boost the 4kHz range to simulate the "quack" of single-coil pickups.

---

### 3. Split-Bank Matrix & Scene Assignments
*   **A:** Telecaster Rhythm (Pristine Clean, Hollow)
*   **B:** Telecaster Lead (Myth Drive engaged for the solo, +1.5dB)
*   **C:** Telecaster Dry (Comping/Practice - Reverb lowered)
*   **D:** Telecaster Ambient (Reverb Decay Maxed)
*   **E:** ES-339 Rhythm (Padded Input, Heavy Mid-Scoop)
*   **F:** ES-339 Lead (Myth Drive engaged, +1.5dB)
*   **G:** ES-339 Dry
*   **H:** ES-339 Ambient

---

### 4. Table A: Main Signal Chain
*Note: Parameters marked with **[Scene X]** require you to Right-Click > Assign to Scene in Cortex Control.*

| Block Category | Model Name | Rhythm Settings (Sc A / E) | Lead Settings (Sc B / F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Input 1 (Global Gate) | **Gain:** [Sc A: 0dB] / [Sc E: -4.5dB]<br>**Thresh:** -65dB | **Gain:** [Sc B: 0dB] / [Sc F: -4.5dB]<br>**Thresh:** -65dB | ES-339 requires heavy input padding to prevent the humbuckers from clipping the high-headroom clean amp. |
| **Pre-FX 1** | Jewel Comp | **Comp:** 2.0<br>**Vol:** 5.0<br>**Tone:** 6.0 | **Comp:** 2.0<br>**Vol:** 5.0<br>**Tone:** 6.0 | Optical compression. Smooths out pick attack without squashing dynamics. Keeps the low-end bouncy. |
| **Pre-FX 2** | Myth Drive | *[Bypassed]* | **Drive:** 2.5<br>**Tone:** 7.0<br>**Level:** 6.5 | Klon circuit. Introduces the mild, transparent clipping used for the solo. Pushes upper mids slightly. |
| **Amp** | Custom Clean | **Vol:** [Sc A: 4.5 / Sc E: 4.0]<br>**Mid:** [Sc A: 3.5 / Sc E: 2.5]<br>**Treble:** 6.5 | **Vol:** [Sc B: 4.5 / Sc F: 4.0]<br>**Mid:** [Sc B: 3.5 / Sc F: 2.5]<br>**Treble:** 6.5 | Dumble ODS Clean. ES-339 requires an even deeper mid-scoop and lower preamp volume to fake the underwound Strat pickup response. |
| **Cab** | 212 Custom Clean | **Mic A:** Dyn 57 (Pos 1.5, Dist 1.0")<br>**Mic B:** Rib 121 (Pos 0.5, Dist 3.0") | **Mic A:** Dyn 57 (Pos 1.5, Dist 1.0")<br>**Mic B:** Rib 121 (Pos 0.5, Dist 3.0") | Dyn 57 provides the glassy attack; Ribbon 121 mixed at -3dB provides the huge low-end warmth without harsh PA tweeter frequencies. |
| **Post-FX 1** | Parametric-8 EQ | **Band 2 (250Hz):** [Sc A: 0dB / Sc E: -4dB]<br>**Band 3 (500Hz):** [Sc A: -2dB / Sc E: -5dB]<br>**Band 7 (4kHz):** [Sc A: +1dB / Sc E: +4dB] | **Band 2 (250Hz):** [Sc B: 0dB / Sc F: -4dB]<br>**Band 3 (500Hz):** [Sc B: -2dB / Sc F: -5dB]<br>**Band 7 (4kHz):** [Sc B: +1dB / Sc F: +4dB] | *The Chameleon Block.* This strictly manipulates the ES-339 to sound hollow and glassy like a Stratocaster Position 4. |
| **Post-FX 2** | Analog Delay | **Mix:** 15%<br>**Time:** 135ms<br>**Fdbk:** 10% | **Mix:** 15%<br>**Time:** 135ms<br>**Fdbk:** 10% | Emulates the Korg SDD-3000 / Aqua Puss "thickening" slapback Mayer uses to widen the clean tone. |
| **Post-FX 3** | Spring Reverb | **Mix:** 35%<br>**Decay:** 2.5s<br>**High Pass:** 150Hz | **Mix:** 35%<br>**Decay:** 2.5s<br>**High Pass:** 150Hz | Captures the outboard Fender Reverb Unit splash. High Pass filter is critical to stop the low-E string from making the reverb muddy on the QSC CP12. |

---

### 5. Troubleshooting & Refinement Tree
If the tone isn't interacting with your QSC CP12 FRFR speaker correctly, follow this strict protocol:

1.  **Too Distorted / Loss of Pristine Clean (ES-339):**
    *   *Action:* Lower the Input Block Gain to **-6.0dB**. Humbuckers vary wildly in output; if yours are modern output, they will overwhelm the Dumble model. 
2.  **Too Thin / Piercing Highs (Telecaster):**
    *   *Action:* Lower the Amp Treble parameter by 1.0 increment. If it's still harsh out of the QSC tweeter, go to the EQ-8 Block and engage the High Pass Filter (LPF) at **6500Hz**.
3.  **Low-End is "Farty" or Flubby:**
    *   *Action:* This happens when the Dumble preamp sags too much on the low-E string. Reduce Amp Bass to **3.5** and raise the Ribbon 121 mic distance to **4.0"** in the Cab block.

---

### 6. Session Library (Active Presets)

**2. Preset Name: "Slow Burning - Dual"**
Target: John Mayer (2006).
Guitar: Dual Support (Gibson ES-339 / Fender Telecaster).
Physics Goal: Dumble headroom, extremely high signal-to-noise ratio, massive scooped single-coil emulation for Humbuckers.
Full Configuration:
*   **Block 1 (Input/Global Gate):** Gain [Sc A-D: 0.0dB / Sc E-H: -4.5dB], Thresh [-65dB].
*   **Block 2 (Jewel Comp):** Comp [2.0], Vol [5.0], Tone [6.0], Mix [100%].
*   **Block 3 (Myth Drive):** Bypass [Sc A/C/D/E/G/H], Active [Sc B/F], Drive [2.5], Tone [7.0], Level [6.5].
*   **Block 4 (Amp - Custom Clean):** Vol [Sc A: 4.5 / Sc E: 4.0], Bass [4.5], Mid [Sc A: 3.5 / Sc E: 2.5], Treble [6.5], Presence [5.5], Master [8.0], Output Level [0.0dB].
*   **Block 5 (Cab - 212 Custom Clean):** Mic A (Dyn 57, Pos 1.5, Dist 1.0", Pan C), Mic B (Rib 121, Pos 0.5, Dist 3.0", Pan C), Mix [A: 0dB, B: -3.0dB], High Cut [8500Hz].
*   **Block 6 (EQ-8):** Band 2 [Sc A: 0dB / Sc E: -4dB @ 250Hz Q1.0], Band 3 [Sc A: -2dB / Sc E: -5dB @ 500Hz Q1.4], Band 7 [Sc A: +1dB / Sc E: +4dB @ 4000Hz Q1.0].
*   **Block 7 (Analog Delay):** Mix [15%], Time [135ms], Fdbk [10%], Drive [0%], Sync [Off].
*   **Block 8 (Spring Reverb):** Mix [35%], Decay [2.5s], High Pass [150Hz], Low Pass [4500Hz], Pre-Delay [15ms].
*   **Lane 1 Output:** Level [Rhythm Scenes: 0.0dB / Lead Scenes: +1.5dB].