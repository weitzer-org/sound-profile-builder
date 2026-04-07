**QC-2 Systems Engineer Online.** 

Acknowledged. Target acquired: **Hubert Sumlin / Howlin' Wolf (Chess Records Era)**. 
Guitar Profile: **Les Paul w/ P90s** (Medium/High output, heavy 60-cycle hum, pronounced 1.5k-2.5k high-mid spike).
Output Profile: **QSC CP12 PA** (FRFR - requires aggressive high-frequency management to simulate a vintage cab).

### Physics & Pickup Compensation Strategy
P90s in a Les Paul push a lot of midrange weight and output. Sumlin achieved his dark, percussive tone largely by playing without a pick (bare thumb and fingers) into small, pushed Tweed amps. 
1. **Noise Management:** P90s will introduce heavy 60-cycle hum when the Tweed amp compresses. We are deploying the **Adaptive Gate** block (Grid) rather than the Global Input Gate. The Adaptive Gate eliminates hum via percentage without prematurely chopping the decay of your notes.
2. **High-Mid Mapping:** The CP12 tweeter will ruthlessly expose the high-mid "honk" of a P90 if not filtered. We will use the **Parametric-8 EQ** *before* the amp to simulate a thumb-plucked attack and flatten the P90's aggressive 2.5kHz spike.

Here is your Split-Bank Matrix configuration. We will map this to **Row 2 (Scenes E-H)** to align with the Gibson Les Paul body profile.

### Table A: Main Signal Chain
*Note: Any parameter with two values requires you to Right-Click > Assign to Scenes.*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 60%<br>Decay: 150ms | Noise Red: 35%<br>Decay: 300ms | P90s hum heavily. Higher reduction on Rhythm for dead-silence between chord stabs. Lower on Lead to preserve sustain. |
| **Pre-FX (EQ)** | Parametric-8 | HPF: 90Hz<br>Band 6 (2500Hz): -3.5dB<br>LPF: 4200Hz | HPF: 90Hz<br>Band 6 (2500Hz): -2.0dB<br>LPF: 4800Hz | **High-Mid Comp:** Band 6 tames P90 harshness. The aggressive LPF physically simulates the lack of pick attack (bare thumb). |
| **Amp** | US DLX 58 | Vol Inst: 3.5<br>Tone: 5.5<br>*No Master Vol* | Vol Inst: 6.5<br>Tone: 6.0<br>*No Master Vol* | Vintage Tweed lacks a Master Vol. Pushing Vol Inst to 6.5 induces natural tube sag and compression. *Compensate output via Lane Output Level.* |
| **Cab** | 112 US DLX | Mic A: Dyn 57 (Pos 0.5, Dist 1.0")<br>Mic B: Rib 121 (Pos 0.5, Dist 3.0") | Mic A: Mix 0dB<br>Mic B: Mix -2dB | Jensen P12Q simulation. The Ribbon 121 adds the necessary dark, low-mid weight to counterbalance the P90's treble. |
| **Post-FX** | Tape Delay | Mix: 15%<br>Time: 115ms<br>Fdbk: 10% | Mix: 22%<br>Time: 115ms<br>Fdbk: 15% | Emulates the physical tape echo used heavily at Chess Records (Slapback). Adds depth to the dry Tweed sound. |
| **Post-FX** | Room Reverb | Mix: 12%<br>Decay: 0.8s<br>LPF: 3000Hz | Mix: 12%<br>Decay: 0.8s<br>LPF: 3000Hz | Simulates the acoustic signature of the Chess Studio room. Low LPF keeps the reverb tail dark and out of the way. |
| **Output** | Lane 1 Output | Level: +3.0dB | Level: 0.0dB | **Headroom Rule:** Because the amp is cleaner/quieter in Scene E, we boost the Lane Output to match the SPL of Scene F. |

### Troubleshooting & Refinement Tree
If the Les Paul P90s hit the "US DLX 58" too hard and the tone sounds "Too Distorted" or "Farty/Flubby":
1. **Input Pad (QC Screen Swipe-Down):** Lower Input 1 Gain to **-3.0dB**. P90s vary wildly in output; this simulates rolling your guitar volume knob down slightly to clean up the front end.
2. **Tube Sag Fix:** Tweed amps are notorious for "farting out" in the low end when pushed. If Scene F (Lead) is too muddy, return to the Parametric-8 block and raise the HPF from 90Hz to **120Hz**. Cutting bass *before* the amp prevents the power amp simulation from choking.

***

### Session Library (Active Presets)

**1. Preset Name: "Spoonful - ES339"**
*Target: Howlin' Wolf / Hubert Sumlin (1960).*
*Guitar: ES-339 (Humbuckers) w/ Pick.*
*(Stored in memory from previous query).*

**2. Preset Name: "Smokestack P90 - LP"**
*Target: Howlin' Wolf / Hubert Sumlin.*
*Guitar: Les Paul (P90 Single Coils) w/ Thumb/Fingerstyle simulation.*
*Physics Goal: High-mid noise filtration + dynamic Tweed sag without digital clipping.*
*Full Configuration:*
*   **Block 1 (Adaptive Gate):** Noise Red [Rhy: 60% / Lead: 35%], Thresh [-55dB], Decay [150ms / 300ms].
*   **Block 2 (EQ-8):** HPF [90Hz], Band 6 [2500Hz, Q 1.5, Rhy: -3.5dB / Lead: -2.0dB], LPF [Rhy: 4200Hz / Lead: 4800Hz].
*   **Block 3 (Amp - US DLX 58):** Vol Inst [3.5 / 6.5], Vol Mic [0.0], Tone [5.5 / 6.0].
*   **Block 4 (Cab - 112 US DLX):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.5, Dist 3.0"), Mix [A: 0dB, B: -2dB].
*   **Block 5 (Tape Delay):** Mix [15% / 22%], Time [115ms], Fdbk [10% / 15%], Drive [25%], HP [150Hz], LP [2500Hz].
*   **Block 6 (Room Reverb):** Mix [12%], Decay [0.8s], HP [120Hz], LP [3000Hz].
*   **Lane 1 Output:** Level [+3.0dB / 0.0dB].