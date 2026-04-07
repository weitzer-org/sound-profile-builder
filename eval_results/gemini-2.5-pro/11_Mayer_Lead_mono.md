Acknowledged. Commencing deep research for John Mayer Trio lead tone.

*   **Target Analysis:** John Mayer Trio era. Core tone is derived from high-headroom, D-style (Dumble/Two-Rock) amplifiers. The goal is a pristine clean foundation ("mid-scooped") with a lead tone achieved by pushing the front end of the amp with a transparent overdrive, not a distortion pedal.
*   **Analog Mapping:**
    *   **Amplifier:** Two-Rock Custom Reverb Signature / Dumble ODS. 6L6 power section. High-headroom clean channel is the platform.
    *   **Cabinet:** 2x12 closed-back with Celestion G12-65 speakers.
    *   **Drive Pedal:** Klon Centaur (or clone). Used as a clean/treble boost with gain set low to saturate the amp's preamp tubes.
    *   **Miking:** Combination of dynamic (SM57) and ribbon (Royer R121) mics is common for capturing both bite and warmth.
*   **Quad Cortex Model Translation:**
    *   **Amplifier:** ODS-100 Clean. This is the verified model for the Dumble Overdrive Special clean channel.
    *   **Cabinet:** 212 US DLX (emulating a 2x12 Fender cabinet). A good, balanced platform.
    *   **Drive Pedal:** Minotaur. This is the verified model for the Klon Centaur.
    *   **Miking:** Dyn 57 and Rbn 121 are available.

Executing preset build based on these findings. I will provide distinct configurations for two guitar types as per protocol.

***

### **Preset Build: JM Trio Lead**

This preset uses a transparent overdrive to push the D-style amp model into saturation for the lead tone, while a Parametric EQ handles the core frequency shaping between rhythm and lead scenes.

#### **Target 1: Gibson ES-339 (Humbuckers)**

Humbuckers have a higher output and a stronger midrange. The following settings compensate by reducing input gain and scooping the midrange to maintain clarity and the characteristic "hi-fi" clean tone.

**Table A: Main Signal Chain (Humbucker Profile)**
*Marked with (Assign)* for scene-specific changes.

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input Gate | Threshold: -55.0dB | Threshold: -60.0dB | Manages humbucker noise. Lowered threshold for lead prevents cutting off sustained notes. |
| **Pre-FX 1** | Minotaur | **Bypassed** | Gain: 2.5, Treble: 6.5, Output: 8.0 *(Assign On)* | Functions as a transparent boost to push the amp's input stage into natural saturation, avoiding pedal-based distortion. This is the core of the lead tone. |
| **Pre-FX 2** | Parametric-8 EQ | Band 4 (Mid): -3.0dB @ 800Hz, Q: 0.7 | Band 5 (Hi-Mid): **+2.5dB @ 1.5kHz**, Q: 1.0 *(Assign)* | **Rhythm:** Creates the essential mid-scoop for the hi-fi clean tone. **Lead:** Adds a focused mid-boost to cut through the mix without adding mud. |
| **Amp** | ODS-100 Clean | Gain: 3.5, Bass: 4.5, Mid: 5.0, Treb: 6.5, Master: 6.0 | Gain: 3.5, Bass: 4.5, Mid: 5.0, Treb: 6.5, Master: 6.0 | The ODS circuit provides the high-headroom clean platform. Amp gain is kept moderate; the drive comes from the Minotaur pedal hitting the preamp, simulating real-world gain staging. |
| **Cab** | 212 US DLX | Mic A: Dyn 57, Dist: 1.0"<br>Mic B: Rbn 121, Dist: 3.0" | Mic A: Dyn 57, Dist: 1.0"<br>Mic B: Rbn 121, Dist: 3.0" | The 57/121 blend captures both the articulate pick attack and the warm speaker body, mimicking a studio miking technique for a polished sound. |
| **Post-FX 1** | Digital Delay | Mix: 18%, Time: 380ms, Fdbk: 25% | Mix: 25%, Time: 380ms, Fdbk: 25% *(Assign)* | A clean, articulate delay for space. The mix is increased slightly for the lead scene to enhance presence. |
| **Post-FX 2** | Plate Reverb | Mix: 20%, Decay: 2.5s | Mix: 20%, Decay: 2.5s | Provides a smooth, studio-quality reverb tail that doesn't wash out the core tone. |
| **Output** | Lane Output | -1.5dB | **+1.5dB** *(Assign)* | Creates the necessary volume jump for solos, ensuring the lead tone is perceived as louder and more prominent. |

***

#### **Target 2: Fender Telecaster (Single Coil)**

Single coils have a lower output and are inherently brighter and noisier. These settings compensate by increasing input gain, adjusting the gate, and taming high frequencies to prevent ice-pick harshness.

**Table A: Main Signal Chain (Single Coil Profile)**
*Marked with (Assign)* for scene-specific changes.

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input Gate | Threshold: -50.0dB | Threshold: -55.0dB | Higher threshold manages single-coil noise more aggressively. Lowered for lead scene to protect sustain. |
| **Pre-FX 1** | Minotaur | **Bypassed** | Gain: 3.0, Treble: 5.5, Output: 8.0 *(Assign On)* | The gain is slightly higher to compensate for lower pickup output. Treble is reduced to prevent the bright single coils from becoming harsh when boosted. |
| **Pre-FX 2**| Parametric-8 EQ | Band 4 (Mid): -2.5dB @ 800Hz, Q: 0.7<br>**LPF:** 6.0kHz | Band 5 (Hi-Mid): **+2.5dB @ 1.5kHz**, Q: 1.0<br>**LPF:** 6.0kHz *(Assign)* | The mid-scoop is slightly less aggressive. A Low Pass Filter (LPF) is added to tame the characteristic "ice pick" frequencies of the bridge pickup. |
| **Amp** | ODS-100 Clean | Gain: **4.8**, Bass: 5.0, Mid: 5.0, Treb: 6.0, Master: 6.0 | Gain: **4.8**, Bass: 5.0, Mid: 5.0, Treb: 6.0, Master: 6.0 | Amp Gain is increased significantly to bring the lower-output single coils to the same "edge of breakup" feel as the humbuckers. Treble is backed off slightly. |
| **Cab** | 212 US DLX | Mic A: Dyn 57, Dist: 1.0"<br>Mic B: Rbn 121, Dist: 3.0" | Mic A: Dyn 57, Dist: 1.0"<br>Mic B: Rbn 121, Dist: 3.0" | Cab and mic settings remain consistent as the EQ and Gain adjustments have already compensated for the guitar's frequency profile. |
| **Post-FX 1** | Digital Delay | Mix: 18%, Time: 380ms, Fdbk: 25% | Mix: 25%, Time: 380ms, Fdbk: 25% *(Assign)* | Identical settings provide consistent spatial effects across both guitar profiles. |
| **Post-FX 2** | Plate Reverb | Mix: 20%, Decay: 2.5s | Mix: 20%, Decay: 2.5s | Identical settings provide consistent spatial effects across both guitar profiles. |
| **Output** | Lane Output | -1.5dB | **+1.5dB** *(Assign)* | Maintains the crucial volume boost for lead parts. |

***

### **Session Library Update**

**Preset Registry Protocol engaged. Appending preset to session memory.**

2.  **Preset Name:** "JM Trio Lead"
    *   **Target:** John Mayer Trio (Live/Studio).
    *   **Guitar:** Dual-profile for ES-339 (Humbuckers) and Telecaster (Single Coils).
    *   **Physics Goal:** Achieve D-style amp tone with a transparent boost for lead saturation. Scene-based EQ for Rhythm (scooped) and Lead (boosted) tones.
    *   **Full Configuration (Humbucker Profile - Sc E/F):**
        *   **Block 1 (Input Gate):** Thresh [-55.0dB / -60.0dB].
        *   **Block 2 (Minotaur Drive):** [Bypassed / Active], Gain [2.5], Treble [6.5], Output [8.0].
        *   **Block 3 (EQ-8):** Band 4 [-3.0dB @ 800Hz / Inactive], Band 5 [Inactive / +2.5dB @ 1.5kHz].
        *   **Block 4 (Amp - ODS-100 Clean):** Gain [3.5], Bass [4.5], Mid [5.0], Treble [6.5], Master [6.0].
        *   **Block 5 (Cab - 212 US DLX):** Mic A (Dyn 57, Dist 1.0"), Mic B (Rbn 121, Dist 3.0").
        *   **Block 6 (Digital Delay):** Mix [18% / 25%], Time [380ms], Fdbk [25%].
        *   **Block 7 (Plate Reverb):** Mix [20%], Decay [2.5s].
        *   **Lane Output:** [-1.5dB / +1.5dB].
    *   **Full Configuration (Single Coil Profile - Sc A/B):**
        *   **Block 1 (Input Gate):** Thresh [-50.0dB / -55.0dB].
        *   **Block 2 (Minotaur Drive):** [Bypassed / Active], Gain [3.0], Treble [5.5], Output [8.0].
        *   **Block 3 (EQ-8):** Band 4 [-2.5dB @ 800Hz / Inactive], Band 5 [Inactive / +2.5dB @ 1.5kHz], LPF active @ 6.0kHz for both.
        *   **Block 4 (Amp - ODS-100 Clean):** Gain [4.8], Bass [5.0], Mid [5.0], Treble [6.0], Master [6.0].
        *   **Block 5 (Cab - 212 US DLX):** Mic A (Dyn 57, Dist 1.0"), Mic B (Rbn 121, Dist 3.0").
        *   **Block 6 (Digital Delay):** Mix [18% / 25%], Time [380ms], Fdbk [25%].
        *   **Block 7 (Plate Reverb):** Mix [20%], Decay [2.5s].
        *   **Lane Output:** [-1.5dB / +1.5dB].