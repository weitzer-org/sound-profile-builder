**System Identity:** QC-2 (Quad Cortex Systems Engineer)
**Target Acknowledged:** Vintage Cream-era Eric Clapton ("Woman Tone"). 
**Hardware Output:** QSC CP12 (Active 12" PA Speaker) via Cortex Control Desktop.

To achieve Clapton’s legendary 1960s tone, we must respect the physical constraints of the analog gear he used: a fully cranked non-master volume Marshall Super Lead pushing Celestion Greenbacks to the point of structural failure, paired with a humbucker guitar whose tone knob is rolled back to 0-2. 

Because you are alternating between a **Gibson ES-339 (Humbuckers)** and a **Fender Telecaster (Single Coils)**, we cannot treat them equally. The Telecaster physically lacks the correct inductance and capacitor characteristics to achieve a true "Woman Tone" by simply rolling down its tone knob. Therefore, I have deployed the **"Chameleon" Strategy** using an EQ-8 block to artificially replicate the resonant peak of a rolled-off PAF humbucker for the Telecaster.

---

### 1. The Split-Bank Matrix Organization
Assign your footswitches using Cortex Control's Scene mode.
*   **Row 1 (Scenes A-D) - FENDER TELECASTER:** Uses dedicated EQ filtering to simulate a neck humbucker with a .022µF capacitor rolled to 0. 
*   **Row 2 (Scenes E-H) - GIBSON ES-339:** True bypass of the extreme EQ. You will physically roll your guitar's neck tone knob down to 1 or 2, and max the volume.

---

### 2. Main Signal Chain & Multi-Guitar Settings

*Note: All parameters marked with [Scene Assigned] must be assigned by right-clicking the parameter in Cortex Control and setting values per scene.*

**Table A: Main Signal Chain**
| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate (Circle 1) | **Tele:** Gain +3.0dB, Thresh -55dB<br>**339:** Gain 0.0dB, Thresh -60dB | **Tele:** Gain +3.0dB, Thresh -55dB<br>**339:** Gain 0.0dB, Thresh -60dB | Tele needs +3dB to hit amp tubes as hard as the 339. Humbuckers need a slightly lower gate. |
| **Pre-FX (EQ)** | Parametric-8 | **Tele (Sc A):** LPF @ 1500Hz, Band 5: +5dB @ 600Hz.<br>**339 (Sc E):** Bypass block. | **Tele (Sc B):** LPF @ 1800Hz, Band 5: +6dB @ 800Hz.<br>**339 (Sc F):** Bypass block. | **Chameleon Strategy:** The strict LPF and mid-hump digitally recreate the physics of a rolled-off tone pot for the Tele. |
| **Amp** | Brit Plexi 100 Jumped | **Vol Normal:** 5.0<br>**Vol High:** 6.5<br>**Bass:** 2.0 *(Crucial)*<br>**Mid:** 8.5<br>**Treble:** 4.5<br>**Lane Out:** +3.5dB | **Vol Normal:** 7.0<br>**Vol High:** 9.0<br>**Bass:** 1.5<br>**Mid:** 9.0<br>**Treble:** 5.0<br>**Lane Out:** +2.0dB | Vintage Plexi has no Master. High Volume controls clipping. Bass *must* drop as Volume rises to prevent speaker/tube flub. |
| **Cab** | 412 Brit Green | **Mic A:** Ribbon 121, Pos 2.0, Dist 1.0"<br>**Mic B:** Dyn 57, Pos 1.5, Dist 1.0"<br>**Mix:** A: 0dB, B: -3dB | Same as Rhythm | Greenbacks compress smoothly. The Ribbon 121 off-center provides the "flute-like" darkness; the 57 retains pick definition. |
| **Post-FX** | Room Reverb | **Mix:** 12%<br>**Decay:** 1.2s<br>**HP:** 150Hz / **LP:** 4000Hz | **Mix:** 15%<br>**Decay:** 1.4s<br>**HP:** 150Hz / **LP:** 4000Hz | Simulates the ambient bleed of a loud amp bleeding into the drum mics at live Cream shows. |

---

### 3. Multi-Guitar Gain Staging Instructions

**For the Gibson ES-339 (Scenes E-H):**
1. Switch to your Neck Pickup. 
2. Roll the ES-339's Neck Tone knob down to **1 or 2**. 
3. Roll Guitar Volume to 10.
4. *Physics Check:* The natural output of the humbuckers will push the Brit Plexi 100 into heavy saturation. Use your guitar volume knob to clean up for rhythm sections (roll back to 6 or 7).

**For the Fender Telecaster (Scenes A-D):**
1. Switch to your Neck Pickup.
2. Keep your Telecaster Tone and Volume at **10** (Wide Open).
3. *Physics Check:* The EQ-8 block is active here. By hard-chopping everything above 1500Hz-1800Hz, we are preventing the Telecaster's high-end sizzle from turning the Plexi into a fuzz box, while the 600Hz-800Hz boost simulates the humbucker body. 

---

### 4. Troubleshooting & Refinement Tree

If monitoring through your QSC CP12 yields unsatisfactory results, execute the following protocol strictly in this order:

*   **Symptom: Tone is "Farty", "Loose", or sounds like a broken speaker.**
    *   *Action:* Lower the Amp Block's **Bass** parameter by -1.0 increments. A fully cranked Plexi cannot handle low-end frequencies before the clipping stage.
*   **Symptom: Too much fuzz/buzz, losing the "flute" character (ES-339).**
    *   *Action:* Your humbuckers may be higher output than vintage PAFs. Go to the Global Input (Circle 1) and drop the input gain from `0.0dB` to `-3.0dB` or `-6.0dB` to simulate lower-output vintage pickups. Compensate for the volume drop by raising the overall Lane Output Level.
*   **Symptom: QSC CP12 sounds too "boomy" or muddy in the room.**
    *   *Action:* Engage the High Pass Filter (HPF) in the Cab Block and set it to `90Hz`. PA speakers inherently produce more sub-bass than actual guitar cabs.

---

### 5. Preset Registry Protocol (Session Memory)

**Session Library (Active Presets)**

1. Preset Name: "Spoonful - ES339" (Previously loaded in memory - Howlin' Wolf Target)
2. Preset Name: "Sunshine Woman - Multi-Gtr"
Target: Eric Clapton (Cream, 1967).
Guitar: Gibson ES-339 (Humbuckers) / Fender Telecaster (Single Coil).
Physics Goal: Emulate cranked Plexi Sag + Rolled-off PAF tone capacitor physics.
Full Configuration:
Block 1 (Global Input): Gain [Tele: +3.0dB / 339: 0.0dB], Gate Thresh [Tele: -55dB / 339: -60dB].
Block 2 (Parametric-8 EQ): Tele Active / 339 Bypass. LPF [Rhy: 1500Hz / Lead: 1800Hz], Band 5 [Rhy: +5.0dB @ 600Hz / Lead: +6.0dB @ 800Hz]. 
Block 3 (Amp - Brit Plexi 100 Jumped): Vol Normal [5.0 / 7.0], Vol High [6.5 / 9.0], Bass [2.0 / 1.5], Mid [8.5 / 9.0], Treble [4.5 / 5.0], Presence [4.0], Lane Output Level [+3.5dB / +2.0dB].
Block 4 (Cab - 412 Brit Green): Mic A (Ribbon 121, Pos 2.0, Dist 1.0"), Mic B (Dyn 57, Pos 1.5, Dist 1.0"), Mix [A: 0dB, B: -3dB].
Block 5 (Room Reverb): Mix [12% / 15%], Decay [1.2s / 1.4s], HP [150Hz], LP [4000Hz].