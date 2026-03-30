**Systems Engineer QC-2 Initializing...**

Target Acquired: **Eric Clapton - "Crossroads" (Cream / Live at Winterland 1968)**.
Hardware Target: Gibson SG ("The Fool" with PAF humbuckers) into a dimed 100W Marshall Super Lead (Plexi), pushing 4x12 Celestion Greenbacks.
Physics Profile: The quintessential "Woman Tone" relies on a massive, non-master-volume tube amplifier pushed to the brink of explosion, combined with rolling the guitar's neck pickup Tone knob completely to zero. The amp's natural brightness compensates for the rolled-off pickup, creating a vocal, sustaining midrange without piercing highs.

As requested by Protocol 9, I have structured this matrix to adapt this vintage analog circuit to two completely different sets of pickup physics: **Gibson ES-339 (Humbuckers)** and **Fender Telecaster (Single Coils)**.

---

### Pickup Compensation Strategy & Physics First Protocol
*QSC CP12 Tuning:* Since the CP12 is an FRFR PA speaker, it reproduces frequencies up to 20kHz. A real 4x12 Greenback cabinet dies off steeply around 5kHz. We will strictly enforce High-Pass and Low-Pass filters in the Cab Block to prevent modern digital harshness and faithfully recreate the 1968 microphone limitations.

**Guitar 1: Gibson ES-339 (Humbuckers) [Scenes E-H]**
*   **Physics:** Medium/Vintage-output humbuckers perfectly emulate Clapton's SG. 
*   **Woman Tone Execution:** You must physically switch to your Neck Pickup and roll your guitar's Tone Knob down to `0` or `1`. The amp block is dialed exceptionally bright and aggressive to catch the remaining midrange frequencies and force them into harmonic feedback. 
*   **Input Gain:** Set Global Input Gain to `-1.5dB`.

**Guitar 2: Fender Telecaster (Single Coil) [Scenes A-D]**
*   **Physics:** Single coils lack the inductance, output, and low-mid thrust to push a Plexi into Cream-era saturation. Furthermore, rolling a Tele neck tone to `0` often sounds muddy, not "vocal."
*   **Woman Tone Execution:** Keep your Telecaster Tone knob at `10`. Scene B will engage the Parametric-8 EQ block, which uses a steep Low-Pass Filter (LPF) at 1.8kHz to artificially simulate the humbucker "Woman Tone" roll-off while boosting output by +4dB to hammer the amp's front end. 
*   **Input Gain:** Set Global Input Gain to `+2.5dB`.

---

### The Split-Bank Matrix Standard
*Row 1 (Telecaster)*: **A** Rhythm (Crunch) | **B** Lead (Woman Tone EQ) | **C** Dry/Comping | **D** Ambient/FX
*Row 2 (ES-339)*: **E** Rhythm (Crunch) | **F** Lead (Vocal Sustain) | **G** Dry/Comping | **H** Ambient/FX

#### Table A: Main Signal Chain
*(Right-Click parameters in Cortex Control to Assign to Scenes)*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 35%<br>Decay: 150ms | Noise Red: 20%<br>Decay: 300ms | Adaptive block used. Lower reduction on leads preserves natural tube sustain and feedback. |
| **Pre-FX** | Parametric-8 EQ | *(Tele Sc A Only)*<br>Gain: +2dB<br>Band 1: 250Hz (+2dB) | *(Tele Sc B Only)*<br>Gain: +4dB<br>LPF: 1.8kHz (12dB/oct)<br>Band 4: 600Hz (+3dB) | Simulates the PAF humbucker push for the Tele. Tele Scene B cuts highs to fake the Woman Tone. *(Bypassed for ES-339).* |
| **Amp** | Brit Plexi 100 Jumped | Vol I: 6.5<br>Vol II: 5.5<br>Output Level: -1.5dB | Vol I: 8.0<br>Vol II: 7.0<br>Output Level: 0.0dB | Vol I controls bite, Vol II adds girth. No Master Volume exists on a Plexi. Use Lane Output for SPL. |
| **Amp (Tone Stack)**| Brit Plexi 100 Jumped | Bass: 2.5<br>Mid: 7.0<br>Treb: 6.5<br>Pres: 5.0 | Bass: 2.0<br>Mid: 8.5<br>Treb: 7.0<br>Pres: 6.0 | *Bass must be kept exceptionally low.* Cranked Plexis sag and "fart out" with high bass. Mids pushed for vocal quality. |
| **Cab** | 412 Brit Green | Mic 1: Dyn 57 (Pos 1.2)<br>Mic 2: Cond 67 (Pos 3.0) | Mic 1: Dyn 57 (Pos 1.2)<br>Mic 2: Cond 67 (Pos 3.0) | Essential 1968 Winterland mic pairing. 57 for aggression, 67 back slightly for resonance. |
| **Cab (Filters)** | Cab EQ / Mix | Mix: 50/50<br>HPF: 95Hz<br>LPF: 5200Hz | Mix: 50/50<br>HPF: 95Hz<br>LPF: 5200Hz | Crucial for the QSC CP12. The steep 5.2kHz LPF kills digital fizz and PA tweeter harshness. |
| **Post-FX** | Room Reverb | Mix: 12%<br>Decay: 1.2s<br>Predelay: 15ms | Mix: 18%<br>Decay: 1.5s<br>Predelay: 15ms | Replicates the natural venue ambiance of Winterland. Lead scene adds slight wash for soloing. |

---

### Troubleshooting & Refinement Tree
If the tone is **"Too Fuzzy" or "Broken"**:
1.  **Input Pad:** Lower Global Input Gain by another `-3.0dB`. Hot humbuckers into a dimed Plexi model will cause digital input clipping before it even hits the virtual tubes.
2.  **Tube Sag:** Lower the Amp Block `Bass` parameter to `1.5` or `1.0`. Vintage Marshall power sections cannot handle low frequencies when Volumes are past 6. 
3.  **Volume Compensation:** If cleaning up the gain lowers your overall volume in the room, **do not** touch the Amp Volumes. Raise the Lane Output Level or the Grid Output Level.

---

### Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
Target: Howlin' Wolf / Hubert Sumlin (1960). 
*(Previously Logged in System)*

**2. Preset Name: "Crossroads - Woman"**
Target: Eric Clapton (Cream, 1968).
Guitars: Gibson ES-339 (Humbuckers) / Fender Telecaster (Single Coil).
Physics Goal: Simulate non-master volume 100W Marshall saturation. Vocal midrange via physics-based tone roll-off and steep LPF filtering. Adapting PAF impedance curves for single coils via Pre-EQ.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 35% / Lead: 20%], Thresh [-60dB], Decay [150ms / 300ms].
Block 2 (EQ-8): *ES-339 = Bypassed.* Tele Rhy (Sc A) = Gain [+2dB], Band 1 Peak [250Hz, +2dB]. Tele Lead (Sc B) = Gain [+4dB], LPF [1800Hz, 12dB/oct], Band 4 Peak [600Hz, +3dB].
Block 3 (Amp - Brit Plexi 100 Jumped): Vol I [6.5 / 8.0], Vol II [5.5 / 7.0], Bass [2.5 / 2.0], Mid [7.0 / 8.5], Treble [6.5 / 7.0], Presence [5.0 / 6.0], Output Level [-1.5dB / 0.0dB].
Block 4 (Cab - 412 Brit Green): Mic A (Dyn 57, Pos 1.2, Dist 1.0"), Mic B (Cond 67, Pos 3.0, Dist 2.0"), Mix [50/50], HPF [95Hz], LPF [5200Hz].
Block 5 (Room Reverb): Mix [12% / 18%], Decay [1.2s / 1.5s], Predelay [15ms], HPF [150Hz], LPF [4000Hz].