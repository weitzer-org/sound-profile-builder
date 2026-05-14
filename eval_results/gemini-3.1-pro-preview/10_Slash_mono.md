**Target Identified:** Slash (Guns N' Roses) 
**Analog Specs Hunt:** Marshall JCM800 2203 (often modded for extra gain) or Silver Jubilee, Seymour Duncan Alnico II Pro pickups (Medium Output PAFs, Neck Position for signature solos), Marshall 1960AV Cabinets (Celestion Vintage 30s), Boss DD-3 Digital Delay. 
**Quad Cortex Mapping:** Brit 2203, Green 808 (acting as the "hot mod" tube gain stage), 412 Brit V30, Digital Delay.

To achieve that signature "flutey," infinitely sustaining neck pickup tone through a QSC CP12 PA Speaker, we have to fight the physics of the neck humbucker: it naturally outputs excessive bass frequencies that will "fart out" and cause intermodulation distortion in a high-gain JCM800 model. We will use the Green 808 not for distortion, but as a severe bass-cut/mid-boost before the amp to keep the neck pickup tight and vocal.

Here is your gig-ready configuration, split specifically for Humbucker and Single Coil architectures.

### **Multi-Guitar Target Output**

#### **Guitar 1: Gibson Les Paul / ES-339 (Humbuckers) - Row 2 (Scenes E-H)**
*   **Pickup Output:** Medium-to-High.
*   **Gain Staging Protocol:** Input Block Gain set to **0.0dB**. 
*   **Chameleon Strategy (Scene F - Lead):** Humbuckers in the neck position require mud management. We will aggressively high-pass the signal before the amp and push the 800Hz–1.5kHz range to get that vocal Slash character.

#### **Guitar 2: Fender Telecaster (Single Coils) - Row 1 (Scenes A-D)**
*   **Pickup Output:** Vintage/Low.
*   **Gain Staging Protocol:** Input Block Gain set to **+3.5dB** to hit the amp's preamp tubes with the same voltage as a humbucker.
*   **Chameleon Strategy (Scene B - Lead):** Telecaster neck pickups are inherently hollow compared to an Alnico II Pro. We will use the Parametric-8 block to add +4.0dB at 250Hz (Body) and apply a Low Pass Filter at 4.5kHz to tame the single-coil pick attack and simulate the humbucker's rolled-off top end.

---

### **Table A: Main Signal Chain**
*Note: Parameters marked with different values for Rhythm/Lead indicate a Scene-Specific change (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Input | Gain: +3.5dB (Tele) / 0.0dB (LP) | Gain: +3.5dB (Tele) / 0.0dB (LP) | Normalizing pickup voltage hitting the digital grid to prevent fuzz. |
| **Gate** | Adaptive Gate | Red: 50%, Thresh: -55dB | Red: 30%, Thresh: -65dB | Higher reduction for choppy rhythm; opened up for lead to allow infinite sustain. |
| **Pre-FX (EQ)** | Parametric-8 | *Bypassed* | Tele: +4dB @ 250Hz, LPF 4.5kHz <br> LP: -2dB @ 300Hz | **Chameleon EQ:** Fattens the Telecaster neck pickup; clears low-mid mud from the Les Paul neck pickup. |
| **Pre-FX (Drive)**| Green 808 | Bypassed (Rhythm uses raw amp) | Gain: 1.0, Level: 8.5, Tone: 6.5 | Acts as the "hot mod." Tightens sub-bass before the amp and hits preamp tubes hard for sustain. |
| **Amp** | Brit 2203 | Master: 6.0, Preamp: 5.5 <br> Bass: 4.0, Mid: 6.0, Treb: 6.0 | Master: 6.0, Preamp: 7.5 <br> Bass: 4.5, Mid: 7.5, Treb: 5.5 | Boosting Mids on the lead scene pushes the "vocal" frequencies of the neck pickup forward. |
| **Cab** | 412 Brit V30 | Mic A: Dyn 57 (Pos 0.5, Dist 1.0") <br> Mic B: Rib 121 (Pos 1.5, Dist 3") | Mix: Mic A (-2dB), Mic B (0dB) <br> HPF: 80Hz, LPF: 6.0kHz | Blending the bite of the 57 with the dark warmth of the 121. LPF at 6kHz is vital for the QSC CP12 to avoid digital fizz. |
| **Post-FX (Dly)** | Digital Delay | Mix: 5% (Barely audible) | Mix: 22%, Time: 420ms, Fdbk: 25% | 420ms is the standard Slash stadium delay time. High Cut set to 2.5kHz inside block to keep repeats behind the lead note. |
| **Post-FX (Rev)** | Hall Reverb | Mix: 10%, Decay: 1.2s | Mix: 15%, Decay: 1.8s | Simulates the massive arena sound of the *Use Your Illusion* tour. |

---

### **Troubleshooting & Refinement Tree**
If you load this up through your QSC CP12 and switch to the Neck Pickup on your Les Paul, and it sounds **"Too muddy/farty on the low notes"**:
1.  **Tube Sag/Bass Physics:** Go to the Brit 2203 Amp block and reduce the *Bass* parameter from 4.5 down to 3.0. JCM800s create low-end distortion when the preamp is pushed. 
2.  **Drive Block:** Ensure the Green 808 is engaged. Tube Screamers inherently cut frequencies below 100Hz, acting as a physical cleanup for hot humbuckers.
3.  **Input Pad:** If the front of your Les Paul's notes sound "splatty," your pickups are hotter than standard PAFs. Lower the Input Block Gain to -3.0dB.

---

### **Session Registry Protocol**
*Appended to Active Session Memory for future recall.*

**Session Library (Active Presets)**

2. Preset Name: "SChild Neck Lead - Multi-Route"
Target: Slash / Guns N' Roses (1987-1991).
Guitar: Dual Profile (Telecaster SC / Gibson LP HB).
Physics Goal: Hot-rodded JCM800 with tight low-end for neck pickup clarity, sustained via preamp saturation and mid-boosting rather than pure fuzz.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Rhy: 50% / Lead: 30%], Thresh [-55dB / -65dB], Decay [150ms / 300ms].
Block 2 (EQ-8 - Chameleon): [Tele Lead: Band 2 +4.0dB @ 250Hz, LPF 4500Hz] / [LP Lead: Band 2 -2.0dB @ 300Hz, HPF 100Hz].
Block 3 (Green 808): Bypass [Scene A/E], Active [Scene B/F], Gain [1.0], Level [8.5], Tone [6.5].
Block 4 (Amp - Brit 2203): Preamp [Rhy: 5.5 / Lead: 7.5], Master [6.0], Bass [Rhy: 4.0 / Lead: 4.5], Mid [Rhy: 6.0 / Lead: 7.5], Treble [Rhy: 6.0 / Lead: 5.5], Presence [5.0], Output Level [0.0dB].
Block 5 (Cab - 412 Brit V30): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 3.0"), Mix [A: -2dB, B: 0dB], HPF [80Hz], LPF [6000Hz].
Block 6 (Digital Delay): Mix [Rhy: 5% / Lead: 22%], Time [420ms], Fdbk [25%], Low Cut [150Hz], High Cut [2500Hz].
Block 7 (Hall Reverb): Mix [Rhy: 10% / Lead: 15%], Decay [Rhy: 1.2s / Lead: 1.8s], Pre-Delay [20ms].