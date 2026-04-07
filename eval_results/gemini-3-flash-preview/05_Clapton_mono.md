This is QC-2. We are targeting the 1967-1968 "Disraeli Gears" era tone. The "Woman Tone" is achieved through a specific interaction between humbuckers with rolled-off tone pots and a Marshall JTM45 or Super Lead circuit pushed into power-tube saturation.

To translate this to the Quad Cortex and your QSC CP12, we must prioritize **Lower-Mid Saturation** and **High-End Roll-off (LPF)** to simulate the lack of treble from the guitar's wiring.

### Physics Protocol: The Woman Tone Logic
*   **For the ES-339:** We will use the **British 45 (JTM45)**. Its tube rectifier provides the "sag" needed for that vocal-like sustain.
*   **For the Telecaster:** We will use the **Parametric-8 EQ** before the amp to "re-voice" the single coils into humbucker territory by boosting the 400Hz resonance and cutting the 3kHz-5kHz "snap."

---

### Table A: Main Signal Chain (Row 1/2 Matrix)
*Note: Right-click parameters in Cortex Control to assign to Scenes.*

| Block Category | Model Name | Rhythm (Sc A/E) | Lead (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Circle 1 (Global) | Thresh: -65dB | Thresh: -70dB | Loose gate to prevent cutting off sustained "vocal" notes. |
| **Pre-FX (EQ)** | **Parametric-8** | (See Tele Settings) | (See Tele Settings) | Essential for "re-voicing" the Telecaster's inductance. |
| **Amp** | **British 45** | Normal: 6.5 / Br: 4.0 | Normal: 8.5 / Br: 3.0 | "Jumped" inputs. High Normal vol provides the "cream." |
| **Cab** | **Brit 412 Straight** | Mic: 121 Ribbon | Mic: 121 Ribbon | Ribbon mics at 1.5" distance capture the warm "wood" of the cab. |
| **Post-FX** | **Room Reverb** | Mix: 8% | Mix: 15% | Simulates the Atlantic Studios room sound. |
| **Output** | Lane Output | -2.0 dB | +1.0 dB | Final SPL compensation for the QSC CP12. |

---

### Target 1: Gibson ES-339 (Humbucker Profile)
*Focus: Maximize sustain and power-amp sag. Since you have humbuckers, your guitar's physical tone knob is your primary controller.*

**Block 3: British 45 (Amp Settings)**
*   **Gain/Normal Vol:** 8.2 (This is where the "fluid" feel lives).
*   **Bright Vol:** 3.5 (Keep this low to avoid "fizzy" digital artifacts).
*   **Bass:** 4.0 (Keep moderate to prevent "farting" out on the neck pickup).
*   **Mids:** 8.5 (The "Woman Tone" lives in the 500Hz-800Hz range).
*   **Treble:** 2.5 (Dark but defined).
*   **Presence:** 1.0 (Roll this off to smooth out the QSC CP12's tweeter).
*   **Master:** 7.5 (Pushes the virtual power tubes into compression).

**Block 4: Cab (Speaker Physics)**
*   **Mic A:** 121 Ribbon | Position: 0.35 | Distance: 1.5" (Softens the pick attack).
*   **Mic B:** 57 Dynamic | Position: 0.50 | Distance: 2.0" | Level: -8.0dB (Adds just a hint of "cut").

---

### Target 2: Fender Telecaster (Single Coil Profile)
*Focus: Removing the "Twang" and adding "Body" to simulate a Gibson SG/335.*

**Block 2: Parametric-8 EQ (The "Chameleon" Block)**
*   **Band 1 (HPF):** 100Hz (Cleans up the mud for the bridge pickup).
*   **Band 2 (Body):** 400Hz | Gain: +4.5dB | Q: 1.5 (Adds the "Humbucker thickness").
*   **Band 7 (High Cut):** 3.2kHz | Gain: -6.0dB (Crucial: This simulates the Tele's tone knob being rolled to '0').
*   **Band 8 (LPF):** 4.5kHz (Removes the single-coil "clack" from the QSC CP12).

**Amp Adjustments for Single Coil:**
*   **Input Gain (Circle 1):** +3.0dB (Compensates for lower output pickups).
*   **Amp Gain/Normal Vol:** 9.5 (You need more preamp drive to get the same sustain as the 339).

---

### Physics & Tone-Setting Instructions
1.  **The Physical Tone Knob:** To achieve the *exact* Clapton sound on the **ES-339**, switch to your **Neck Pickup** and roll your guitar's physical tone knob to **0**. If it's too muddy, roll it to **2**.
2.  **The QSC CP12 Factor:** Ensure your CP12 is set to the **"Default" or "Dance"** setting (if it has the contour switch). Do **not** use the "Speech" or "Floor Monitor" modes, as they cut the low-mids essential for this preset.
3.  **Troubleshooting "Fuzziness":** If the low end feels "broken" or "fizzy," go to the Amp Block and reduce **Bass** to 3.0. JTM45 circuits are notorious for "Bass Flub" when the gain is cranked.

---

### Session Library (Active Presets)
1. **Preset Name: "Spoonful - ES339"** (Howlin' Wolf / Sumlin build).
2. **Preset Name: "Creamy Woman - QC2"**
   * **Target:** Clapton (1967).
   * **Physics Goal:** High-mid resonance with zero high-end harshness.
   * **Models:** British 45, Brit 412 Cab, Parametric-8 EQ.
   * **Scene Map:** A/E (Rhythm), B/F (Woman Tone Lead), C/G (Dry), D/H (Ambient Room).