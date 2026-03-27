System initializing... **QC-2 Quad Cortex Systems Engineer** online. 

Target acquired: **Stevie Ray Vaughan "Texas Flood"**
Hardware environment: **Mac Desktop (Cortex Control) -> QSC CP12 (Active FRFR)**
Pickup profile: **Vintage Single Coil Stratocaster** (Low Output)

To capture SRV’s massive tone with standard vintage single coils (without using his heavy .013 gauge strings), we must rely on "Physics First" gain staging. We will use a classic '65 Super Reverb circuit pushed by an 808-style drive. SRV used his Tube Screamer primarily as a *clean boost*—hitting the front end of the preamp tubes hard with volume rather than pedal clipping. 

Since you are running into a QSC CP12, we must strictly manage the high-frequency tweeter using a Parametric-8 EQ block to prevent the Tube Screamer/Fender combination from becoming "ice-picky."

### Standard Split-Bank Matrix Protocol
*   **Row 1 (Scenes A-D):** Vintage Single Coil Configuration (Your primary target).
*   **Row 2 (Scenes E-H):** Humbucker Configuration (Pre-compensated gain reduction for higher output pickups).

***

### Table A: Main Signal Chain
*Note: Right-Click parameters in Cortex Control to assign them to Scenes. The Amp block has no Master Volume; overall loudness is controlled via the Lane Output Level at the end of the grid.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | Thresh: -65dB | Thresh: -60dB | Vintage coils hum. Lower threshold for Lead prevents cutoff as the TS noise floor rises. |
| **Pre-FX (Drive)** | Green 808 | **Bypassed** | **Active** <br>Overdrive: 1.5<br>Tone: 4.5<br>Level: 8.5 | SRV used the TS808 to pummel the amp's input stage. Low drive, high level. |
| **Amp** | US SPR 65 | Vol: 4.5<br>Treb: 6.0<br>Mid: 7.0<br>Bass: 3.5 | Vol: 4.5<br>Treb: 6.0<br>Mid: 7.0<br>Bass: 3.5 | Midrange pushed to simulate heavy gauge strings. Bass kept low to prevent "farty" tube sag on neck pickup. |
| **Cab** | 410 US SPR | Mic A: Dyn 57 (Pos 1.5)<br>Mic B: Ribbon 121 (Pos 3.0)<br>Mix: 50/50 | Mic A: Dyn 57 (Pos 1.5)<br>Mic B: Ribbon 121 (Pos 3.0)<br>Mix: 50/50 | The 121 Ribbon mic thickens the high-E string pick attack. 57 provides the "Texas Flood" bite. |
| **Post-FX (EQ)** | Parametric-8 | Band 1 (200Hz): +2dB<br>Band 8 (LPF): 5kHz | Band 1 (200Hz): +2dB<br>Band 8 (LPF): 4.8kHz | LPF tames the QSC CP12 tweeter. Band 1 adds Strat "body." Lower LPF on Lead smooths 808 clipping. |
| **Post-FX (Verb)**| Spring | Mix: 18%<br>Decay: 1.5s<br>Tone: 4.0 | Mix: 18%<br>Decay: 1.5s<br>Tone: 4.0 | Simulates the Super Reverb’s onboard tube spring tank. |
| **Lane Output** | Output Level | +0.0dB | +1.5dB | Physical loudness bump for solos without altering tube saturation characteristics. |

*(Scenes C/G disable Reverb for completely dry comping. Scenes D/H engage a "Vibrato" block pre-amp for "Cold Shot" rotary simulations).*

***

### Troubleshooting & Refinement Tree
If the tone responds poorly through your QSC CP12, follow this strict operational order:

1. **If it sounds "Too Fuzzy/Distorted" on Rhythm:** Your guitar's pickups might be hotter than true vintage spec. Lower the **Global Input Block Gain** from `0.0dB` to `-3.0dB`. 
2. **If the Neck Pickup sounds "Muddy/Farty" on Lead:** The Super Reverb circuit sags under heavy low-end. Reduce the **US SPR 65 Bass** parameter to `2.5`. 
3. **If it sounds "Piercing" (Ice-pick highs):** Lower the **Parametric-8 Band 8 (LPF)** to `4.2kHz` to block more high frequencies from the CP12's tweeter. 
4. **To Increase Loudness:** Never increase the Amp Volume, as this will change the physical gain staging. Raise the **Lane Output Level**.

***

### Session Library (Active Presets)
```text
1. Preset Name: "Texas Flood - Strat SC"
Target: Stevie Ray Vaughan (1983).
Guitar: Vintage Single Coil Stratocaster.
Speaker: QSC CP12.
Physics Goal: Edge-of-breakup Super Reverb pushed by high-level/low-gain TS808.

Full Configuration:
Block 1 (Global Input Gate): Thresh [Rhy: -65dB / Lead: -60dB], Input Gain [0.0dB (SC) / -4.5dB (HB)].
Block 2 (Green 808): Bypass State [Rhy: Off / Lead: On], Overdrive [1.5], Tone [4.5], Level [8.5].
Block 3 (Amp - US SPR 65): Vol [4.5], Treble [6.0], Middle [7.0], Bass [3.5].
Block 4 (Cab - 410 US SPR): Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 3.0, Dist 2.0"), Mix [50/50], Low Cut [90Hz].
Block 5 (Parametric-8): HPF [Bypassed - handled by Cab], Band 1 Low Shelf [200Hz, +2.0dB], Band 6 Peak [2500Hz, +1.0dB], Band 8 LPF [Rhy: 5000Hz / Lead: 4800Hz].
Block 6 (Spring Reverb): Mix [18%], Decay [1.5s], Tone [4.0].
Grid Output (Lane Output): Level [Rhy: +0.0dB / Lead: +1.5dB].
```