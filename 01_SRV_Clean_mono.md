Greetings. I am QC-2, Quad Cortex Systems Engineer. 

To recreate Stevie Ray Vaughan’s "Texas Flood" tone, we are executing a **Physics First** approach based on high-headroom Fender-style circuits driven into physical compression by a high-output/low-drive Tube Screamer. 

Since you are running Vintage Single Coils, our primary physics challenge is **Signal-to-Noise Ratio (SNR)** and **Low-End Body**. SRV used famously heavy strings (0.13s) tuned down a half-step to generate massive low-end induction. To compensate for standard string gauges, we will utilize the **EQ-8 Block** to push the 200Hz range, simulating that heavier physical mass before it hits the amp.

### Hardware Validation
*   **Amp Model:** `US Vibra 40` (Simulates his legendary Fender Vibroverb 1x15). This is a non-master volume circuit. **Do not look for a Master Volume knob**; use the Lane Output Level to adjust the SPL for your QSC CP12.
*   **Cab Model:** `410 US Spr 65` (Simulates the 4x10 Super Reverb punch utilized heavily in the studio sessions). 
*   **Overdrive:** `Green 808` (Simulates the Ibanez TS808).

Here is your Split-Bank Matrix configuration. *(Note: Row 1 / Scenes A-D are optimized for your Single Coils).*

### Table A: Main Signal Chain ('Texas Flood' Stratocaster)
*Right-Click parameters in Cortex Control to assign Scene-specific values.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 35%<br>Thresh: -65dB | Noise Red: 55%<br>Thresh: -60dB | *Grid-based Adaptive Gate.* Tightens the 60Hz hum when the 808's output is maximized in Lead mode. |
| **Pre-FX (Drive)** | Green 808 | Drive: 1.0<br>Tone: 5.5<br>Level: 6.5 | Drive: 2.5<br>Tone: 6.5<br>Level: 9.5 | The SRV Method: Low drive, maximum level. Slams the V1 preamp tube to create natural, uncompressed amp overdrive. |
| **Pre-FX (EQ)** | Parametric-8 | Band 2: +2.5dB @ 200Hz | Band 2: +2.5dB @ 200Hz | Simulates the magnetic pull of heavy-gauge strings. Adds thickness to vintage single coils. |
| **Amp** | US Vibra 40 | Volume: 5.5<br>Treble: 6.5<br>Bass: 4.0 | Volume: 6.5<br>Treble: 7.0<br>Bass: 3.5 | *Non-Master Amp.* We reduce bass slightly in Lead mode to prevent tube "sag" and farty low-end when the amp is pushed hard. |
| **Cab** | 410 US Spr 65 | Mic A: Dyn 57 (Center, 1.0")<br>Mic B: Rib 121 (Edge, 4.0") | Mic A: 0.0dB<br>Mic B: -3.0dB | Dyn 57 provides the pick-attack "bite". The Ribbon 121 provides the low-end "thump" required for Texas Blues. |
| **Post-FX (Verb)**| Spring Reverb | Mix: 22%<br>Decay: 2.5s | Mix: 28%<br>Decay: 2.5s | Emulates the analog spring tank. Sits behind the dry signal but blooms beautifully on sustained bends. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +2.5dB | Overall SPL control. Boosts volume for the solo without altering the tube saturation profile. |

***

### Extended Scene Matrix (Row 1: Single Coils)
*   **Scene A (Rhythm):** Edge of breakup. Lightly compressed. Dig in with the right hand to make it bark.
*   **Scene B (Lead):** Full "Texas Flood" solo tone. High mid-hump from the Green 808 pushing the amp into saturation.
*   **Scene C (Lenny/Clean):** Turn `Green 808` OFF. Drop Amp Volume to `4.0`. Increase Spring Reverb Mix to `35%`.
*   **Scene D (Cold Shot):** Turn `Green 808` OFF. Add a `Rotary` block post-amp (Speed: Fast, Mix: 45%) to emulate the Fender Vibratone rotating speaker.

***

### Troubleshooting & Refinement Tree
If the preset reacts unexpectedly to your specific vintage pickups via the QSC CP12, follow this strict order of operations:
1.  **Too "Ice-Pick" Bright?** Do not touch the Amp's Treble yet. Go to the `Parametric-8` EQ block and enable Band 8 as a Low-Pass Filter (LPF) at **5500Hz**. This slices off the digital fizz from the Green 808 without losing the midrange bite.
2.  **Too Fuzzy / Distorted?** Lower the Input Block Gain to **-3.0dB**. Vintage pickups with weakened magnets can sometimes output unpredictable transient spikes; padding the input smooths the waveform before it hits the Green 808.
3.  **Lacking "Girth"?** Increase the Amp Block's `Volume` by 1.0, but reduce the `Green 808` Level by 1.0. This shifts the gain staging away from the pedal and puts more workload onto the amp's virtual power section.

***

### Session Library (Active Presets)

**1. Preset Name: "Texas Flood - Vintage Strat"**
*   **Target:** Stevie Ray Vaughan (1983).
*   **Guitar:** Stratocaster (Vintage Single Coils).
*   **Physics Goal:** Non-master volume amp pushed into edge-of-breakup via high-headroom, high-level Tube Screamer.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Rhy: 35% / Lead: 55%], Thresh [-65dB / -60dB], Decay [150ms].
    *   **Block 2 (Green 808):** Drive [1.0 / 2.5], Tone [5.5 / 6.5], Level [6.5 / 9.5].
    *   **Block 3 (EQ-8):** HPF [80Hz], Band 2 Peak [+2.5dB @ 200Hz / Q: 1.0], LPF [6500Hz].
    *   **Block 4 (Amp - US Vibra 40):** Volume [5.5 / 6.5], Treble [6.5 / 7.0], Bass [4.0 / 3.5].
    *   **Block 5 (Cab - 410 US Spr 65):** Mic A (Dyn 57, Pos 0.0, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 4.0"), Mix [A: 0dB, B: -3.0dB].
    *   **Block 6 (Spring Reverb):** Mix [22% / 28%], Decay [2.5s], Tone [6.0].
    *   **Output Lane:** Level [0.0dB / +2.5dB].