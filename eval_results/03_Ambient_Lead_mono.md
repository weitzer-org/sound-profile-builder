Here is your Quad Cortex build for a massive, spacey David Gilmour-style lead tone. 

To achieve this specific physics profile, we must rely on the **Headroom Rule**. Gilmour’s tone is famously built on ultra-clean, high-headroom amplifiers (Hiwatt) pushed by massive fuzz and compression, then smoothed out by tape delays and studio plates. Because you are playing through a QSC CP12 (an active PA speaker, which has a pristine, unforgiving top-end), we will use a Ribbon mic in the Cab block and specific LPF (Low Pass Filter) settings on the delay/reverb blocks to prevent the fuzz from sounding like a swarm of bees.

### 4. Organization Standard (Split-Bank Matrix)
*   **Row 1 (Scenes A-D): Fender Telecaster (Single Coil)** - Focuses on adding low-mid body and taming the bridge pickup ice-pick.
*   **Row 2 (Scenes E-H): Gibson ES-339 (Humbuckers)** - Focuses on clarity, padding the input to prevent muddying the Fuzz block, and rolling off the massive low-end.

***

### Table A: Main Signal Chain
*Note: Parameters marked with different Scene values must be assigned to Scenes (Right-Click > Assign on your Cortex Control Mac app).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 30% | Noise Red: 65% | Fuzz dramatically raises the noise floor; high reduction needed for Lead to prevent feedback. |
| **Pre-FX 1** | Optical Comp | Comp: 4.0, Gain: +2.0dB | Comp: 6.5, Gain: +4.0dB | Gilmour uses heavy compression *before* his drive for infinite sustain. |
| **Pre-FX 2** | Fuzz Pi | Bypass: ON (Disabled) | Bypass: OFF (Tone: 4.0, Sus: 7.0) | Big Muff style fuzz. Tone kept low (4.0) so the FRFR speaker doesn't sound harsh. |
| **Pre-FX 3** | Electric Flanger | Bypass: ON | Mix: 25%, Rate: 1.5Hz | (Electric Mistress) Placed before the amp to create liquid, swooshing movement. |
| **Amp** | Custom 100 | Norm Vol: 6, Master: 8.5 | Norm Vol: 6, Master: 8.5 | Hiwatt DR103. Massive headroom. Does not distort on its own; acts as a clean pedal platform. |
| **Cab** | 412 UK High WT | Mic 1: Dyn 57 (Center, 1") | Mic 2: Rib 121 (Edge, 4") | 57 provides the cut; 121 ribbon smooths the high-end fizz of the Fuzz Pi. Mix: 50/50. |
| **EQ** | Parametric-8 | *See Guitar Targets Below* | *See Guitar Targets Below* | The "Chameleon" block. Tailors the pickup output to the Fuzz/Hiwatt physics. |
| **Post-FX 1** | Tape Delay | Mix: 15%, Mod Dep: 15% | Mix: 35%, Mod Dep: 30% | Modulated tape echo. Time: ~430ms. LPF set to 2.5kHz so echoes don't clash with the lead. |
| **Post-FX 2** | Plate | Decay: 2.2s, Mix: 20% | Decay: 4.5s, Mix: 45% | Massive studio plate. HPF at 200Hz ensures the giant reverb tail doesn't muddy the QSC CP12. |

***

### Multi-Guitar Target Output & Gain Staging Instructions

You must configure the Global Input Block and the EQ-8 Block differently depending on which guitar you are playing to accurately hit the target physics.

#### Guitar 1: Fender Telecaster (Single Coil)
*Single coils have lower output and less midrange compared to the Strat neck pickups Gilmour often uses. The bridge pickup is notoriously sharp.*
*   **Global Input Block (Circle 1):** Leave Level at **0.0dB**.
*   **Amp Block:** Increase `Norm Vol` to **7.0** to compensate for lower pickup output.
*   **Parametric-8 EQ Block Settings:**
    *   **Band 1 (Body Boost):** Low Shelf, Freq: 250Hz, Gain: **+3.0dB** (Thickens the single-coil to match a Strat's neck position).
    *   **Band 8 (Ice-Pick Tame):** LPF (Low Pass Filter), Freq: **4500Hz** (Rounds off the sharp transient of the Tele bridge pickup so the Fuzz Pi doesn't get brittle).

#### Guitar 2: Gibson ES-339 (Humbuckers)
*Humbuckers will hit the Optical Comp and Fuzz Pi too hard, resulting in compressed, "farty" square-wave distortion rather than soaring violin-like sustain.*
*   **Global Input Block (Circle 1):** Pad the Input Level to **-4.5dB**. This perfectly simulates rolling your guitar's volume knob down and prevents the digital input stage from clipping before hitting the amp.
*   **Amp Block:** Decrease `Bass` from 5.0 to **3.5**.
*   **Parametric-8 EQ Block Settings:**
    *   **Band 1 (Mud Cut):** Low Shelf, Freq: 300Hz, Gain: **-2.5dB** (Removes the low-mid "woof" inherent to semi-hollow humbuckers).
    *   **Band 5 (Upper Mid Push):** Peak, Freq: 2200Hz, Gain: **+1.5dB** (Adds back the bite and articulation that humbuckers lose under heavy fuzz).
    *   **Band 8 (Air):** LPF (Low Pass Filter), Freq: **6500Hz** (Allows slightly more high-end through to keep the dark humbuckers clear).

***

### 6. Troubleshooting & Refinement Protocol
*   **If the ES-339 sounds "Too Distorted" or "Sputtery":** Lower the Input Block Gain to -6.0dB. Do not lower the Fuzz Pi sustain, as that kills the Gilmour vibe. Lower the input signal instead.
*   **If the Telecaster lacks volume on Rhythm scenes:** Do not increase the Amp Volume (this will induce unwanted amp overdrive). Instead, raise the **Lane Output Level** block on the far right of the grid by +2.0dB to +4.0dB.

***

### Session Library (Active Presets)

2. Preset Name: "Gilmour Space Plate - Tele/339"
Target: David Gilmour (Mid-70s / Pulse era hybrid).
Guitar: Fender Telecaster / Gibson ES-339.
Physics Goal: Infinite fuzz sustain into ultra-clean, high-headroom amplifier with heavy, modulated spatial trails optimized for FRFR.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 30% / Lead: 65%].
Block 2 (Optical Comp): Comp [Rhy: 4.0 / Lead: 6.5], Gain [Rhy: +2dB / Lead: +4dB].
Block 3 (Fuzz Pi): Bypass [Rhy: ON / Lead: OFF], Tone [4.0], Sustain [7.0].
Block 4 (Electric Flanger): Bypass [Rhy: ON / Lead: OFF], Mix [25%], Rate [1.5Hz].
Block 5 (Amp - Custom 100): Norm Vol [6.0], Brill Vol [4.0], Bass [4.5], Mid [6.0], Treble [5.0], Presence [4.0], Master [8.5].
Block 6 (Cab - 412 UK High WT): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Rib 121, Pos 0.8, Dist 4.0"), Mix [50/50].
Block 7 (EQ-8 - Telecaster): L-Shelf 250Hz [+3.0dB], LPF [4500Hz]. 
Block 7 (EQ-8 - ES-339): Input Pad [-4.5dB], L-Shelf 300Hz [-2.5dB], Peak 2200Hz [+1.5dB], LPF [6500Hz].
Block 8 (Tape Delay): Mix [Rhy: 15% / Lead: 35%], Mod Depth [Rhy: 15% / Lead: 30%], Time [430ms], LPF [2500Hz].
Block 9 (Plate Reverb): Mix [Rhy: 20% / Lead: 45%], Decay [Rhy: 2.2s / Lead: 4.5s], HPF [200Hz].