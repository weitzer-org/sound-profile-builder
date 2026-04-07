This is **QC-2**, Systems Engineer. I have mapped the "Early British Invasion" profile specifically for the **UK C30 Top Boost** (Vox AC30) and the **UK JTM45** (Marshall). Given your request for "Chime and Sparkle," the signal chain below focuses on the **UK C30 Top Boost**, as it better replicates the Class A compression and "jangle" characteristic of 1963–1966 recordings.

### Physics-First Protocol: QSC CP12 Calibration
To ensure this preset translates to your **QSC CP12**, set the speaker to **"Default" or "Studio" mode**. Do not use "Dance" or "Speech" modes, as they will induce a non-linear "smiley-face" EQ that clashes with the Quad Cortex’s IR/Cab block.

---

### Table A: Main Signal Chain (CorOS Verified)

| Block Category | Model Name | Rhythm (Sc A/E) | Lead (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Circle 1 (Global) | **Thresh:** -65dB | **Thresh:** -62dB | Low threshold to maintain the natural "ring" of chimey chords. |
| **Pre-FX** | **Brit Blues** | **Gain:** 1.5 / **Vol:** 6.0 | **Gain:** 3.5 / **Vol:** 6.5 | Based on a Bluesbreaker. Adds mid-push without over-saturating the preamp tubes. |
| **Amp** | **UK C30 Top Boost** | **Drive:** 3.5 | **Drive:** 5.5 | **Edge-of-breakup:** Uses input sensitivity rather than master volume to create "bloom." |
| **Cab** | **212 UK C30** | **Mic A:** Dyn 57 | **Mic B:** Ribbon 121 | **Physics:** The 57 captures the 3kHz "bite," the 121 captures the cabinet's woodiness. |
| **EQ** | **Parametric-8** | (See Specifics Below) | (See Specifics Below) | Used for Pickup Compensation (Humbucker vs. Tele). |
| **Post-FX** | **Plate Lush** | **Mix:** 15% | **Mix:** 22% | 1960s Studio Plate reverb mimicry. High-pass filter at 150Hz. |

---

### Multi-Guitar Target Output (The "Chameleon" Logic)

To maintain the same "Invasion" feel across two wildly different guitars, we will utilize a Split-Bank Matrix. Row 1 is optimized for the low-output Telecaster; Row 2 is padded for the ES-339 humbuckers.

#### 1. Fender Telecaster (Single Coil) | Scenes A-D
*Focus: Adding "Weight" and Taming "Ice-Pick" Highs.*

*   **Input Block:** Gain set to **+1.5dB** to hit the amp's front end harder.
*   **EQ-8 Block (Position: Post-Amp):** 
    *   **Band 1 (Body):** +2.0dB Boost at 180Hz (Peak). This adds the "humbucker-like" girth the Tele lacks.
    *   **Band 8 (Twang):** Low Pass Filter (LPF) at **4.5kHz**. This prevents the AC30 "Top Boost" from becoming harsh on the bridge pickup.
*   **Amp Drive:** 4.2 (Higher to compensate for lower pickup output).

#### 2. Gibson ES-339 (Humbuckers) | Scenes E-H
*Focus: Clarity and Definition. Preventing the "Mud" in the low-mids.*

*   **Input Block:** Gain set to **-4.0dB**. This prevents the ES-339 from pushing the UK C30 into a "fuzz-like" distortion, maintaining the "Invasion" chime.
*   **EQ-8 Block (Position: Post-Amp):**
    *   **Band 1 (Clarity):** High Pass Filter (HPF) at **110Hz**. This removes the "woof" common in semi-hollow bodies.
    *   **Band 4 (Definition):** +1.5dB at 2.4kHz. Helps the humbucker "cut" through the mix like a single coil.
*   **Amp Drive:** 2.8 (Lower to maintain the "Edge of Breakup" without full-on saturation).

---

### Table B: Scene Parameters (Cortex Control Right-Click Assignments)

| Scene | Function | Amp Drive | Brit Blues (Pre) | Lane Output Level |
| :--- | :--- | :--- | :--- | :--- |
| **Sc A (Tele Rhy)** | Clean/Chime | 3.5 | OFF | 0.0dB |
| **Sc B (Tele Lead)**| Mid-Boost | 5.5 | ON | +2.0dB |
| **Sc E (339 Rhy)** | Clean/Chime | 2.5 | OFF | -1.5dB |
| **Sc F (339 Lead)** | Punchy Drive | 4.2 | ON | +1.0dB |

---

### Troubleshooting & Refinement
*   **If the ES-339 sounds "farty" or "broken" on the low E string:** Go to the **UK C30 Amp Block** and reduce the **Bass** parameter to 3.0. Vintage AC30 circuits have very little low-end headroom.
*   **If the Telecaster sounds too "thin":** Increase the **Tube Sag** parameter in the Amp block to 6.5. This will make the notes feel "spongier" and more physically substantial.

---

### Session Library (Active Presets)
1.  **Preset Name: "Invasion 64"**
    *   **Target:** Beatles/Kinks/Early Stones.
    *   **Guitars:** Telecaster (Row 1) / ES-339 (Row 2).
    *   **Model:** UK C30 Top Boost.
    *   **I/O:** Input Gain [Tele: +1.5dB / 339: -4.0dB].
    *   **EQ-8:** Tele [LPF 4.5kHz] / 339 [HPF 110Hz].
    *   **Gate:** Adaptive Noise Red 25%.