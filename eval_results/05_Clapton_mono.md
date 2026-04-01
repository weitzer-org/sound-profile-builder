**QC-2 Systems Engineer.**
**Protocol Initialized: Project "Woman Tone" (Cream-Era Clapton).**

Target Acquired: 1968 Eric Clapton (Crossroads / Sunshine of Your Love). 
Analog Signal Path: Late 50s Les Paul / '64 SG (Neck Pickup, Tone at 0, Volume at 10) -> Marshall 1959 Super Lead 100W (Cranked) -> 4x12 w/ Celestion Greenbacks. 
Physics Challenge: Pushing a completely darkened, high-output neck humbucker into a fully saturated non-master-volume tube amp without causing low-frequency phase cancellation ("flubbing" or "farting out"). 

Below is the engineered split-bank matrix to achieve this on the Quad Cortex, utilizing a QSC CP12 active FRFR monitor.

---

### Phase 1: Split-Bank Matrix Configuration
To satisfy the multi-guitar requirement, we will use the Cortex Control Scene Matrix to handle the massive physical differences between your Gibson ES-339 and Fender Telecaster. 

**Row 1 (Scenes A-D): Fender Telecaster (Single Coil)**
*Physics Goal:* A Telecaster physically cannot produce the "Woman Tone" merely by rolling off the tone knob; it lacks the resonant peak and output of a PAF humbucker. We will engage a Parametric-8 EQ block exclusively on these scenes to artificially enforce a dark humbucker EQ curve and push +4.0dB of gain into the amp. 

**Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers)**
*Physics Goal:* True analog replication. You will physically roll your ES-339's Neck Pickup Tone knob to 0 (or 1), and Volume to 10. The amp is tuned to handle this massive influx of low-mid frequencies without muddying the QSC CP12 speaker.

---

### Phase 2: Main Signal Chain (Table A)
*Note: Parameters marked with an asterisk (*) change per Scene. Right-click the parameter in Cortex Control and assign it to Scenes.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input (Global)** | Input 1 | Gain: +4.0dB* (Tele) / 0.0dB* (ES-339) | Gain: +4.0dB* (Tele) / 0.0dB* (ES-339) | Tele needs +4dB to hit the Plexi preamp tubes with humbucker-equivalent voltage. |
| **Global Gate** | Gate (Circle 1) | Thresh: -55dB / Decay: 200ms | Thresh: -55dB / Decay: 200ms | Tames single-coil hum and cranked Plexi hiss. |
| **Pre-FX (EQ)** | Parametric-8 | *Bypass on Sc E/F (ES-339)* | *Active on Sc A/B (Tele)* | *See Telecaster Custom Filter settings below.* |
| **Amp** | Brit Plexi 100 Jumped | Vol Norm: 6.0*<br>Vol Bright: 5.5*<br>Bass: 1.5<br>Mid: 8.5<br>Out Level: +3.0dB* | Vol Norm: 8.5*<br>Vol Bright: 7.5*<br>Bass: 1.0*<br>Mid: 9.0*<br>Out Level: +4.5dB* | **No Master Vol.** High Mids are crucial for this tone. Bass kept extremely low to prevent speaker flubbing from the dark guitar signal. |
| **Cab** | 412 Brit Green | Mic 1: Ribbon 121 (Pos 1.5, Dist 2.0") | Mic 2: Dyn 57 (Pos 0.0, Dist 1.0") | Greenbacks (G12M) are historically accurate. Ribbon catches the thick body; Dyn 57 catches pick attack. Mix: 50/50. |
| **Post-FX (Rev)**| Room Reverb | Mix: 15% / Decay 0.9s | Mix: 15% / Decay 1.2s* | Simulates the live/studio room sound of Cream's era. Cut Lows below 150Hz. |

---

### Phase 3: Guitar-Specific Execution Instructions

#### Target 1: Gibson ES-339 (Humbuckers) - Use Scenes E (Rhythm) & F (Lead)
1. **Guitar Settings:** Switch to your Neck Pickup. Roll the Volume up to 10. Roll the Tone knob completely down to 0 (or up to 1 if it becomes entirely indistinct).
2. **QC Bypass Logic:** Ensure the Parametric-8 EQ block is **Bypassed** on Scenes E and F. The true humbuckers are doing the physical work. 
3. **Pick Attack:** Pick near the neck for a flute-like, vocal sustain. The low Bass setting on the Plexi amp block prevents the ES-339 from overloading the QSC CP12's woofer.

#### Target 2: Fender Telecaster (Single Coil) - Use Scenes A (Rhythm) & B (Lead)
1. **Guitar Settings:** Switch to your Neck Pickup. Keep Volume at 10 and Tone at 10 (Let the QC do the EQ mapping). 
2. **QC Bypass Logic:** Ensure the Parametric-8 EQ block is **Active** on Scenes A and B. 
3. **Parametric-8 "Virtual Woman Tone" Configuration:**
   * **Band 3 (Peak):** +4.5dB @ 350Hz (Adds humbucker wood/body).
   * **Band 4 (Peak):** +3.0dB @ 600Hz (Pushes the vocal/horn-like midrange).
   * **LPF (Low Pass Filter):** Set to **1200Hz** at **24dB/Octave**. (This is the critical step: it violently cuts all treble, simulating a physical capacitor tone roll-off, while leaving the synthesized mid-hump intact).

---

### Phase 4: Troubleshooting & Refinement Tree
If the tone through your QSC CP12 sounds incorrect, execute these checks strictly in order:
1. **Tone is "Farty" or "Blowing out the speaker":** Your guitar is putting out too much sub-bass. Lower the Amp Block **Bass** parameter to 0.5. Alternatively, raise the Cab Block HPF to 100Hz.
2. **Tone is too Fuzzy/Harsh:** The ES-339 pickups might be too hot. Lower the Global Input Gain from 0.0dB to -3.0dB. 
3. **Not Loud Enough:** Do **not** raise the Vol Norm/Bright on the Amp block (this adds distortion, not SPL headroom). Raise the Amp Block **Output Level** or the Lane Output Level.

---

### Phase 5: Session Library (Active Presets)

**2. Preset Name: "Cream Woman Tone - Matrix"**
*   **Target:** Eric Clapton / Cream (1968).
*   **Guitar(s):** Fender Telecaster (Sc A-D) / Gibson ES-339 (Sc E-H).
*   **Physics Goal:** Vintage jumped Plexi distortion; simulated tone roll-off for single coils; low-end phase management for dark humbuckers.
*   **Full Configuration:**
    *   **Block 1 (Global Gate):** Input 1, Thresh [-55dB], Decay [200ms], Input Gain [Tele: +4.0dB / ES-339: 0.0dB].
    *   **Block 2 (EQ-8):** *[Bypassed on Sc E-H]* Band 3 [+4.5dB @ 350Hz], Band 4 [+3.0dB @ 600Hz], LPF [1200Hz, 24dB/oct].
    *   **Block 3 (Amp - Brit Plexi 100 Jumped):** Vol Norm [Rhy: 6.0 / Lead: 8.5], Vol Bright [Rhy: 5.5 / Lead: 7.5], Bass [Rhy: 1.5 / Lead: 1.0], Mid [Rhy: 8.5 / Lead: 9.0], Treble [6.0], Presence [5.0], Master [N/A], Output Level [+3.0dB / +4.5dB].
    *   **Block 4 (Cab - 412 Brit Green):** Mic A (Ribbon 121, Pos 1.5, Dist 2.0"), Mic B (Dyn 57, Pos 0.0, Dist 1.0"), Mix [A: 0dB, B: 0dB], HPF [85Hz], LPF [8000Hz].
    *   **Block 5 (Room Reverb):** Mix [15%], Decay [0.9s / 1.2s], Pre-delay [15ms], HP [150Hz], LP [4000Hz].