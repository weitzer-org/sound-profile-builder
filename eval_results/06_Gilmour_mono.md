Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To recreate David Gilmour’s legendary "Comfortably Numb" tone—specifically the massive, soaring outro solo—we must carefully manage the physics of the **Big Muff (Fuzz Pi)** into a **Hiwatt DR103 (UK C100)**. 

A Big Muff applies massive gain and severely scoops the midrange, which can sound buzzy and hollow through a flat-response PA speaker like your QSC CP12. To combat this, we will use a high-headroom clean amp setting with heavily boosted mids, followed by a strict Low Pass Filter (LPF) to remove digital fizz. 

Per my operational protocols, I have split this configuration into a **Multi-Guitar Target Output**, explicitly separating the parameter math for your **Fender Telecaster** (Single Coils) and **Gibson ES-339** (Humbuckers).

***

### The "Chameleon" Strategy & Gain Staging

**1. Fender Telecaster (Vintage/Low Output Single Coils)**
*   **Physics Challenge:** Single coils lack the low-mid girth to push a Big Muff into smooth, violin-like sustain, often resulting in a spiky, thin fuzz tone. 
*   **Global Input Setting:** `0.0 dB` (Unity). 
*   **EQ-8 Strategy:** We will add a +3.0dB Low Shelf at 200Hz to simulate humbucker body, allowing the Fuzz Pi to compress smoothly. 
*   **Bank Logic:** **Row 1 (Scenes A-D)**

**2. Gibson ES-339 (Medium/High Output Humbuckers)**
*   **Physics Challenge:** Humbuckers push too much voltage into a Big Muff, causing the clipping diodes to "choke" and sound muddy or farty on the low strings.
*   **Global Input Setting:** `-4.5 dB` Pad. This is critical. It cleans up the input signal so the fuzz acts like a smooth distortion rather than a broken square wave.
*   **EQ-8 Strategy:** High Pass Filter (HPF) raised to 110Hz to cut muddy neck-pickup rumble, with a slight dip at 350Hz.
*   **Bank Logic:** **Row 2 (Scenes E-H)**

***

### TABLE A: Main Signal Chain ("Comfortably Numb" Build)
*Note: Parameters marked with **(Assign)** require you to Right-Click and assign them to Scenes. "Rhythm" replicates the smoother, delay-light first solo. "Lead" replicates the massive outro solo.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Red: 35% / Thresh: -55dB | Red: 45% / Thresh: -55dB **(Assign)** | Tames single-coil 60-cycle hum and Muff noise floor without killing sustain. |
| **Pre-FX 1** | Opto Comp | Peak Red: 40, Gain: 5.0 | Peak Red: 55, Gain: 6.0 **(Assign)** | Feeds the Fuzz an already-compressed, sustained signal (Gilmour secret). |
| **Pre-FX 2** | Fuzz Pi | Sust: 4.5, Tone: 4.5, Vol: 5.0 | Sust: 7.5, Tone: 5.5, Vol: 5.5 **(Assign)** | Triangle/Ram's Head style fuzz. Keep Tone at or below 5.5 to prevent FRFR fizz. |
| **Pre-FX 3** | Flanger | Mix: 0% (Bypassed) | Mix: 15%, Rate: 0.5Hz, Depth: 40% **(Assign)** | Simulates the Electric Mistress. Kept very subtle to widen the fuzz. |
| **Amp** | UK C100 Normal | Vol: 4.0, Master: 8.5, Bass: 4.0, Mid: 7.5, Treb: 5.0, Pres: 4.5 | Vol: 4.5, Master: 8.5, Bass: 4.0, Mid: 7.5, Treb: 5.0, Pres: 4.5 **(Assign)** | Hiwatts require high Master / low Vol for high-headroom, pedal-taking cleans. Mids pushed to counteract Fuzz scoop. |
| **Cab** | 412 UK C100 | Mic A: Dyn 57 (Pos 0.5) <br> Mic B: Rib 121 (Pos 1.5) | Mic A: Dyn 57 (Mix 0dB) <br> Mic B: Rib 121 (Mix -2dB) | Fane speakers. Ribbon mic softens the high-end spikes of the Fuzz Pi. |
| **Post-EQ** | Parametric-8 | LPF: 5500Hz, HPF: 90Hz | LPF: 5500Hz, HPF: 90Hz | **Crucial for QSC CP12.** Removes the 6kHz+ frequencies where digital fuzz "wasps" live. |
| **Post-FX 1** | Tape Delay | Mix: 15%, Time: 480ms, Fdbk: 25% | Mix: 28%, Time: 480ms, Fdbk: 45% **(Assign)** | Simulates his Binson Echorec / MXR. Sits warmly behind the notes. |
| **Post-FX 2** | Hall Reverb | Mix: 15%, Decay: 2.5s | Mix: 25%, Decay: 3.5s **(Assign)** | Simulates large stadium decay. High Cut at 3000Hz keeps repeats from clashing. |

***

### Troubleshooting & Refinement Tree

If monitoring through your QSC CP12 and the tone feels incorrect, execute these steps strictly in order:

1.  **"It sounds too fizzy, fuzzy, or synthetic."** 
    *   *Action:* Lower the **Parametric-8 LPF** from 5500Hz down to 4500Hz. Fuzz into a PA speaker inherently reveals high-frequency harshness that a real guitar cabinet would naturally filter out.
2.  **"The ES-339 sounds muddy/muffled when I play chords."**
    *   *Action:* You are overloading the Fuzz Pi. Go to the Global Input block and lower the input gain to `-6.0dB`. 
3.  **"The Telecaster dies out too quickly; I can't hold the long bends."**
    *   *Action:* Increase the **Opto Comp** Peak Reduction to `65` and increase the **Fuzz Pi** Sustain to `8.5`. Do *not* add Amp Gain, as this will introduce non-musical preamp fizz.

***

### Session Library (Active Presets)

**2. Preset Name: "Numb Solo - Tele / ES-339"**
*   **Target:** David Gilmour, "Comfortably Numb" (1979).
*   **Guitar:** Multi-Target (Fender Telecaster / Gibson ES-339).
*   **Physics Goal:** Extreme sustain via Fuzz->Comp staging, tamed by LPF for QSC CP12 Active PA Speaker.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Red [Rhy: 35% / Lead: 45%], Thresh [-55dB].
    *   **Block 2 (Opto Comp):** Peak Red [Rhy: 40 / Lead: 55], Gain [5.0 / 6.0].
    *   **Block 3 (Fuzz Pi):** Sust [Rhy: 4.5 / Lead: 7.5], Tone [4.5 / 5.5], Vol [5.0 / 5.5]. ES-339 Input Pad requires -4.5dB offset.
    *   **Block 4 (Flanger):** Mix [Rhy: 0% / Lead: 15%], Rate [0.5Hz], Depth [40%].
    *   **Block 5 (Amp - UK C100 Normal):** Vol [4.0 / 4.5], Bass [4.0], Mid [7.5], Treble [5.0], Presence [4.5], Master [8.5].
    *   **Block 6 (Cab - 412 UK C100):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Rib 121, Pos 1.5, Dist 2.0"), Mix [A: 0dB, B: -2dB].
    *   **Block 7 (EQ-8):** HPF [90Hz], LPF [5500Hz]. Telecaster: Band 1 Low Shelf [+3.0dB @ 200Hz]. ES-339: Band 2 Peak [-2.0dB @ 350Hz].
    *   **Block 8 (Tape Delay):** Mix [15% / 28%], Time [480ms], Fdbk [25% / 45%], Drive [10%], HP [150Hz], LP [2500Hz].
    *   **Block 9 (Hall Reverb):** Mix [15% / 25%], Decay [2.5s / 3.5s], Pre-Delay [20ms], High Cut [3000Hz].