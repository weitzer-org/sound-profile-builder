**QC-2 Systems Engineer Online.** 

Target Confirmed: Early British Invasion (1964-1966). 
Analog Topology: Vox AC30 Top Boost chime with Abbey Road studio-style plate reverb and tape slapback. 
Target PA: QSC CP12 (Active FRFR). 

*Pre-Flight Trigger:* Are your Telecaster and ES-339 pickups Vintage Output, Medium, or High Output? (Assuming Vintage/Medium for this build). 

To accommodate the massive dynamic and frequency differences between a Fender Telecaster and a Gibson ES-339 without compromising the delicate "edge-of-breakup" physics of an AC30 circuit, we are utilizing the **Split-Bank Matrix** and **Pickup Output Compensation Strategy**. 

The AC30 model ("UK C30 Top Boost") is a non-master volume circuit. Drive is achieved strictly by pushing the 'Vol' parameter, and overall SPL is balanced using the Lane Output Level.

---

### **GUITAR 1: FENDER TELECASTER (SINGLE COIL)**
**Routing:** Row 1 (Scenes A, B, C, D)
**Gain Staging Protocol:** Single coils require a stronger physical push to hit the Vox's breakup threshold. Set your Global Input Block Gain to **+2.0dB**. 
**Physics Goal:** Boost the 200Hz body to prevent the Telecaster from sounding thin, while clamping down on the 4.5kHz "ice-pick" frequencies that the QSC CP12 tweeter will otherwise expose.

**Table A1: Telecaster Signal Chain (Scenes A-D)**
| Block Category | Model Name | Rhythm Settings (Scene A) | Lead Settings (Scene B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -55dB | Thresh: -55dB | Telecasters inherently hum. Moderate threshold to clamp 60Hz cycle noise. |
| **Pre-FX** | Parametric-8 | Band 2 (200Hz): +3.0dB<br>LPF: 4.5kHz | Band 2 (200Hz): +3.0dB<br>LPF: 5.0kHz | *Chameleon Strategy*: Adds low-mid body to single coils. LPF tames pick attack harshness. |
| **Amp** | UK C30 Top Boost | Vol: 3.5<br>Treble: 6.0<br>Tone Cut: 5.5 | Vol: 5.5 *(Assigned)*<br>Treble: 6.5 *(Assigned)*<br>Tone Cut: 4.5 *(Assigned)* | Tone Cut works backwards (higher = darker). Lead drops the cut for more JTM/Vox chime and pushes Vol for saturation. |
| **Cab** | 212 UK C30 | Mic A: Dyn 57 (Pos 0.5)<br>Mic B: Rib 121 (Pos 2.0) | Mic A: Dyn 57 (Pos 0.5)<br>Mic B: Rib 121 (Pos 2.0) | 57 gives the British punch; 121 ribbon adds warmth and speaker cabinet resonance. |
| **Post-FX 1** | Tape Delay | Time: 90ms<br>Mix: 15% | Time: 110ms *(Assigned)*<br>Mix: 25% *(Assigned)* | Simulates studio tape slapback used heavily by The Beatles/Kinks. |
| **Post-FX 2** | Plate Reverb | Decay: 1.2s<br>Mix: 18% | Decay: 1.5s *(Assigned)*<br>Mix: 22% *(Assigned)* | Abbey Road plate simulation. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB *(Assigned)* | Output compensation for lead boost without clipping the PA. |

---

### **GUITAR 2: GIBSON ES-339 (HUMBUCKERS)**
**Routing:** Row 2 (Scenes E, F, G, H)
**Gain Staging Protocol:** Humbuckers will immediately overload the AC30 model, causing modern distortion rather than vintage breakup. Set your Global Input Block Gain to **-4.0dB**.
**Physics Goal:** De-mud the low-mids (300Hz-400Hz) associated with a semi-hollow body pushing a Vox, and open the high-pass filters to let the humbuckers sparkle naturally.

**Table A2: Gibson ES-339 Signal Chain (Scenes E-H)**
| Block Category | Model Name | Rhythm Settings (Scene E) | Lead Settings (Scene F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Gate (Circle 1) | Thresh: -62dB | Thresh: -62dB | Humbuckers are quieter at rest. Lower threshold preserves playing dynamics. |
| **Pre-FX** | Parametric-8 | HPF: 100Hz<br>Band 3 (400Hz): -2.5dB<br>LPF: 6.5kHz | HPF: 100Hz<br>Band 3 (400Hz): -2.5dB<br>LPF: 6.5kHz | *Chameleon Strategy*: HPF and Band 3 cut remove the "boomy" mud from the semi-hollow body. Higher LPF allows humbucker presence through. |
| **Amp** | UK C30 Top Boost | Vol: 3.0<br>Treble: 6.5<br>Tone Cut: 6.0 | Vol: 4.8 *(Assigned)*<br>Treble: 7.0 *(Assigned)*<br>Tone Cut: 5.0 *(Assigned)* | Slightly lower Vol than the Tele to maintain absolute clarity on chords. Tone cut slightly higher to manage the midrange spike. |
| **Cab** | 212 UK C30 | Mic A: Dyn 57 (Pos 1.0)<br>Mic B: Rib 121 (Pos 2.5) | Mic A: Dyn 57 (Pos 1.0)<br>Mic B: Rib 121 (Pos 2.5) | Moved the 57 slightly further from the cap (Pos 1.0) to smooth out the humbucker's upper-mid bite. |
| **Post-FX 1** | Tape Delay | Time: 90ms<br>Mix: 12% | Time: 110ms *(Assigned)*<br>Mix: 20% *(Assigned)* | Tape saturation kept lower for the ES-339 to prevent low-end build-up in the repeats. |
| **Post-FX 2** | Plate Reverb | Decay: 1.2s<br>Mix: 15% | Decay: 1.5s *(Assigned)*<br>Mix: 20% *(Assigned)* | Slightly lower mix than Tele to retain definition of the mahogany neck/body. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB *(Assigned)* | Output compensation for gig-ready soloing. |

---

### **Troubleshooting & Refinement Tree**
If the QSC CP12 sounds "Too Fuzzy" or "Farty" on the low strings (especially with the ES-339):
1. **Input Pad:** Ensure the ES-339 Input Gain is strictly set to -4.0dB (or even -6.0dB if your specific pickups are hot).
2. **Tube Sag Correction:** Lower the `Bass` parameter on the UK C30 Top Boost block from its default to `3.5`. Vintage Vox models suffer from phase-inverter blocking distortion if fed too much bass. 
3. **Compensate:** If you lose perceived loudness from dropping the bass, raise the Lane Output Level by +1.0dB.

---

### **Session Library (Active Presets)**

2. Preset Name: "British Invasion Split"
Target: Beatles / Kinks / Early Stones (1964-1966).
Guitar: Fender Telecaster (Scenes A-D) & Gibson ES-339 (Scenes E-H).
Physics Goal: AC30 chime, edge-of-breakup dynamic response, managed speaker bite for QSC CP12.
Full Configuration:
Input Block: Tele Gain [+2.0dB], ES-339 Gain [-4.0dB]. Gate Thresh [Tele: -55dB / 339: -62dB].
Block 1 (EQ-8): Tele: Band 2 [+3.0dB, 200Hz], LPF [4.5kHz]. 339: HPF [100Hz], Band 3 [-2.5dB, 400Hz], LPF [6.5kHz].
Block 2 (Amp - UK C30 Top Boost): Vol [Tele Rhy: 3.5 / Lead: 5.5 | 339 Rhy: 3.0 / Lead: 4.8], Bass [4.5], Treble [Tele: 6.0/6.5 | 339: 6.5/7.0], Tone Cut [Tele: 5.5/4.5 | 339: 6.0/5.0].
Block 3 (Cab - 212 UK C30): Mic A (Dyn 57, Tele Pos 0.5, 339 Pos 1.0, Dist 1.0"), Mic B (Ribbon 121, Pos 2.0/2.5, Dist 3.0"), Mix [A: 0dB, B: -3dB].
Block 4 (Tape Delay): Time [Rhy: 90ms / Lead: 110ms], Mix [Tele: 15/25% | 339: 12/20%], Fdbk [10%].
Block 5 (Plate Reverb): Decay [Rhy: 1.2s / Lead: 1.5s], Mix [Tele: 18/22% | 339: 15/20%], HP [150Hz], LP [4000Hz].
Block 6 (Lane Output): Level [Rhy: 0.0dB / Lead: +1.5dB].