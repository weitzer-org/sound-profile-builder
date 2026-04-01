Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To achieve a David Gilmour-inspired ambient, spacey lead tone, we must focus on high-headroom analog preamp emulation, heavily compressed fuzz for infinite sustain, and bucket-brigade style modulated delay cascading into a vast Plate reverb. 

**Deep Research Target:** David Gilmour (circa *The Wall* / *Pulse* eras).
*   **Analog Specs:** Hiwatt DR103 (100W, ultra-linear power section), WEM 4x12 with Fane Crescendo speakers, MXR Dyna Comp/Boss CS-2, Electro-Harmonix Ram’s Head Big Muff, Binson Echorec/TC2290 Delays, Lexicon Plate Reverb.
*   **QC Translation:** "UK C100 Bright" Amp, "412 UK C Fane" Cab, "Chief CS3" Comp, "Pi Fuzz", "Analog Delay" (for inherent BBD modulation), and "Plate" Reverb.

Before we finalize, **Trigger Verification: Are your pickups Vintage Output, Medium, or High Output?** 
Assuming standard output for both your Fender Telecaster (Single Coils) and Gibson ES-339 (Humbuckers), I have engineered a Split-Bank preset to mathematically compensate for the physics of both instruments through a QSC CP12 active PA speaker.

---

### Multi-Guitar Output Strategy & Split-Bank Matrix

Because single coils and humbuckers react drastically different to a Big Muff-style circuit, we are utilizing the **Split-Bank Matrix**:
*   **Row 1 (Scenes A-D): Fender Telecaster (Single Coil Profile)**. Focuses on noise management, boosting 200Hz for body, and pushing the input gain (+1.5dB) to hit the fuzz optimally.
*   **Row 2 (Scenes E-H): Gibson ES-339 (Humbucker Profile)**. Focuses on low-mid clarity. Input gain is padded (-4.0dB) to prevent the humbuckers from choking the Pi Fuzz into a muddy, square-wave distortion. 

**Scene Functions:**
*   **A/E:** Rhythm (Clean, Comp + Amp only, -1.5dB output).
*   **B/F:** Lead (Gilmour Main: Fuzz + Mod Delay + Plate, +1.5dB output).
*   **C/G:** Dry Lead (Fuzz active, Reverb/Delay bypassed for upfront solos).
*   **D/H:** Ambient Space (Flanger added, Mix levels pushed to max for cinematic swells).

---

### Table A: Main Signal Chain
*Note: Parameters marked with **[Rhy / Lead]** require you to Right-Click > Assign to Scenes.*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input Gate | Tele: Thresh -55dB<br>339: Thresh -65dB | Tele: Thresh -55dB<br>339: Thresh -65dB | Tele needs higher threshold due to 60-cycle hum reacting with the Pi Fuzz. |
| **Input Pad** | Input Block Gain | Tele: +1.5dB<br>339: -4.0dB | Tele: +1.5dB<br>339: -4.0dB | **Crucial:** Humbuckers will clip the Fuzz block too early. Pad the ES-339 to retain articulation. |
| **Pre-FX 1** | Chief CS3 | Sustain: 45%<br>Level: 5.5 | Sustain: 65%<br>Level: 6.0 | CS-3 style compression grabs the pick attack and feeds an even signal into the fuzz for infinite sustain. |
| **Pre-FX 2** | Pi Fuzz | *Bypassed* | Sustain: 7.5<br>Tone: 4.0<br>Vol: 6.5 | Ram's Head emulation. Tone is kept below 5.0 to prevent ice-pick highs through the QSC tweeter. |
| **Amp** | UK C100 Bright | Vol: 3.5<br>Master: 8.0 | Vol: 4.5<br>Master: 8.0 | Hiwatt logic: High Master, Low Preamp Volume. Yields massive, uncompressed clean headroom. |
| **Cab** | 412 UK C Fane | Mic A: Dyn 57 (Pos 0.2)<br>Mic B: Rib 121 (Pos 0.8) | Mic A: Mix +0dB<br>Mic B: Mix -3dB | Fane speakers have extended highs. Ribbon mic placed off-center tames the fizz for the QSC CP12. |
| **Post-FX 1** | Parametric-8 | *See Chameleon Protocol below* | *See Chameleon Protocol below* | Matches the guitar's physical output to the amp's sweet spot. |
| **Post-FX 2** | Flanger | *Bypassed* | *Bypassed* *(Active in Sc D/H)* | Electric Mistress-style sweep for extreme "Any Colour You Like" ambient modulation. |
| **Post-FX 3** | Analog Delay | Mix: 15%<br>Time: 380ms | Mix: 35%<br>Time: 480ms<br>Mod Depth: 45% | BBD delay darkens repeats. High Mod Depth creates the "spacey" chorusing effect on the trails. |
| **Post-FX 4** | Plate Reverb | Mix: 15%<br>Decay: 1.2s | Mix: 45%<br>Decay: 3.8s | Replicates massive studio plate. Long decay creates the ambient wash behind the solo. |

---

### Execution Protocol: The "Chameleon" EQ Strategy
To ensure the Pi Fuzz sounds identical on both guitars, map the **Parametric-8 EQ** block (placed *after* the Cab block) to Scenes A-D and E-H:
*   **Scenes A-D (Telecaster Single Coils):** Band 1 (Low Shelf): **+3.0dB at 200Hz**. Band 8 (LPF): **4500Hz**. Single coils need low-end mass so the fuzz sounds massive, and the LPF rolls off the brittle bridge pickup attack.
*   **Scenes E-H (ES-339 Humbuckers):** Band 1 (Low Shelf): **-2.5dB at 250Hz**. Band 8 (LPF): **6000Hz**. Humbuckers create muddy low-mids when fuzzed; scooping 250Hz clears the mud while opening the LPF allows upper harmonics to pierce the reverb.

---

### Troubleshooting & Refinement Tree
If the tone through your QSC CP12 is **"Too Fuzzy" or "Farting Out"**:
1.  **Input Pad:** Lower the Input Block Gain another -3.0dB (simulates rolling down the guitar's volume knob to clean up the Pi Fuzz).
2.  **Amp Gain:** Reduce the UK C100 "Vol" by 1.5 increments. Do *not* lower the Master. 
3.  **Output Compensation:** If the tone loses loudness, raise the Lane Output Level, *not* the Amp Volume. 

---

### Session Library (Active Presets)

**2. Preset Name: "Comfortably Spaced - Multi-Guitar"**
*   **Target:** David Gilmour (Ambient Lead).
*   **Guitar:** Dual Routing (Fender Telecaster / Gibson ES-339).
*   **Physics Goal:** Massive headroom, heavily compressed fuzz, highly modulated analog delay, long-tail plate reverb, balanced for QSC CP12.
*   **Full Configuration:**
    *   **Block 1 (Chief CS3):** Sustain [Rhy: 45% / Lead: 65%], Attack [Fast], Level [5.5 / 6.0].
    *   **Block 2 (Pi Fuzz):** Sustain [7.5], Tone [4.0], Vol [6.5].
    *   **Block 3 (Amp - UK C100 Bright):** Vol [3.5 / 4.5], Master [8.0], Bass [4.5], Mid [6.0], Treb [5.5], Pres [4.0].
    *   **Block 4 (Cab - 412 UK C Fane):** Mic A (Dyn 57, Pos 0.2, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 3.0"), Mix [A: 0dB, B: -3dB], Low Cut [80Hz], High Cut [8500Hz].
    *   **Block 5 (EQ-8):** Tele (Sc A-D) -> LS [+3.0dB @ 200Hz], LPF [4500Hz]. ES-339 (Sc E-H) -> LS [-2.5dB @ 250Hz], LPF [6000Hz].
    *   **Block 6 (Flanger):** Rate [0.35Hz], Depth [75%], Mix [50%] *(Only active Scenes D/H)*.
    *   **Block 7 (Analog Delay):** Mix [15% / 35%], Time [380ms / 480ms], Fdbk [40%], Mod Depth [45%], Mod Rate [1.5Hz], Sync [Off].
    *   **Block 8 (Plate Reverb):** Mix [15% / 45%], Decay [1.2s / 3.8s], Pre-Delay [25ms], Low Color [Bypassed], High Color [Cut 3.5kHz].