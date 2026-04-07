**QC-2 Systems Engineer: Initialization Complete.**

To achieve that authentic, warm 1950s/1960s Chess Records Chicago Blues tone (think Muddy Waters, Buddy Guy, early Howlin' Wolf), we are targeting a low-wattage Fender 5E3 circuit. Since they were recorded live in a room, we will rely on power tube saturation and an ambient room reverb rather than heavy overdrive pedals. 

**Trigger Protocol Verification:** *Are your pickups Vintage Output, Medium, or High Output?* (I have assumed Vintage-to-Medium for this baseline build. If they are modern High-Output, you must lower the Global Input Gate Threshold by an additional -3.0dB).

Here is your Split-Bank Matrix configuration to accommodate both the Fender Telecaster and the Gibson ES-339 through your active QSC CP12 PA speaker.

### **Organization Standard (Split-Bank Matrix)**
*   **Row 1 (Scenes A-D): Fender Telecaster (Single Coil)** - Focus on adding low-end weight and taming the bridge pickup "ice-pick" attack for FRFR/PA translation.
*   **Row 2 (Scenes E-H): Gibson ES-339 (Humbucker)** - Focus on high-end clarity and preventing the dreaded 5E3 "tube sag fart-out" caused by heavy humbucker low-end.

---

### **Table A: Main Signal Chain (Fender Telecaster - Single Coils)**
*Use Scenes A (Rhythm), B (Lead), C (Dry), D (Ambient)*
*(Right-Click parameters to Assign to Scenes)*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 30%<br>Thresh: -65dB | Noise Red: 15%<br>Thresh: -70dB | Single coils need moderate reduction. Lowered in Lead for max sustain. |
| **Pre-FX** | Parametric-8 | Band 1 (Low Shelf): +2.0dB @ 200Hz<br>Band 8 (LPF): 4.5kHz | Band 1: +2.5dB @ 200Hz<br>Band 8: 5.0kHz | **Chameleon Strategy:** Adds body to thin single coils. LPF smooths FRFR harshness. |
| **Amp** | US DLX Tweed Jump | Vol Norm: 3.5<br>Vol Bright: 4.5<br>Tone: 4.5 | Vol Norm: 5.5<br>Vol Bright: 6.5<br>Tone: 5.0 | Pushing volume drives the power amp simulation. *Note: Amp has NO Master Volume.* |
| **Cab** | 112 US DLX Tweed | Mic A: Dyn 57 (Center, 1.0")<br>Mic B: Ribbon 121 (Edge, 4.0")<br>Mix: +2dB Ribbon | Mic A: Dyn 57 (Center, 1.0")<br>Mic B: Ribbon 121 (Edge, 4.0")<br>Mix: 0dB (Equal blend) | Ribbon 121 provides Chess Records warmth. Adding 57 in Lead cuts through mix. |
| **Post-FX 1** | Tape Delay | Mix: 15%<br>Time: 110ms<br>Fdbk: 0% | Mix: 22%<br>Time: 110ms<br>Fdbk: 15% | Analog slapback simulates early tape echo used in Chicago studios. |
| **Post-FX 2** | Room Reverb | Mix: 18%<br>Decay: 1.2s | Mix: 24%<br>Decay: 1.5s | Places the CP12 speaker back into a physical studio room space. |

*Loudness Compensation (Telecaster):* Raise **Lane Output Level** to `+3.0dB` for Scene A and `+4.5dB` for Scene B to achieve performance SPL without altering the physical tube gain stage.

---

### **Table B: Main Signal Chain (Gibson ES-339 - Humbuckers)**
*Use Scenes E (Rhythm), F (Lead), G (Dry), H (Ambient)*
*(Right-Click parameters to Assign to Scenes)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 10%<br>Thresh: -70dB | Noise Red: 5%<br>Thresh: -75dB | Humbuckers are quieter; minimal gate needed to retain dynamics. |
| **Pre-FX** | Parametric-8 | Band 1 (HPF): 110Hz<br>Band 4 (Peak): +1.5dB @ 1.2kHz | Band 1 (HPF): 130Hz<br>Band 4 (Peak): +2.5dB @ 1.5kHz | **Chameleon Strategy:** HPF prevents 339 low-end from muddying the Tweed preamp. Mid-boost adds bite. |
| **Amp** | US DLX Tweed Jump | Vol Norm: 2.0<br>Vol Bright: 2.5<br>Tone: 6.5 | Vol Norm: 3.5<br>Vol Bright: 4.5<br>Tone: 7.0 | Humbuckers hit the preamp much harder. Lower volumes prevent ugly "blocking distortion". |
| **Cab** | 112 US DLX Tweed | Mic A: Dyn 57 (Center, 0.5")<br>Mic B: Ribbon 121 (Edge, 6.0")<br>Mix: +2dB Dyn 57 | Mic A: Dyn 57 (Center, 0.5")<br>Mic B: Ribbon 121 (Edge, 6.0")<br>Mix: +4dB Dyn 57 | Humbuckers are naturally dark. Prioritize the Dyn 57 for treble cut. |
| **Post-FX 1** | Tape Delay | Mix: 12%<br>Time: 110ms<br>Fdbk: 0% | Mix: 18%<br>Time: 110ms<br>Fdbk: 10% | Same slapback physics, slightly lower mix to preserve humbucker clarity. |
| **Post-FX 2** | Room Reverb | Mix: 15%<br>Decay: 1.0s | Mix: 20%<br>Decay: 1.3s | Emulates the natural Chess Records room sound. |

*Loudness Compensation (ES-339):* Because we drastically lowered the Amp Volumes to keep the humbuckers clean, raise **Lane Output Level** heavily to `+7.0dB` for Scene E and `+8.5dB` for Scene F.

---

### **Troubleshooting & Refinement Tree**
If playing the ES-339 and the amp sounds "broken," "farty," or like the speaker is tearing (a common physics reality of plugging humbuckers into a 5E3 Tweed):
1. **Input Pad:** Go to the global Input block (Circle 1) and lower the Input Gain to `-3.0dB` or `-6.0dB`. This simulates rolling off the guitar's volume pot and clears up input digital clipping.
2. **Pre-FX HPF:** Raise the High-Pass Filter in the EQ-8 up to 150Hz. You must starve a Tweed amp of low-end frequencies *before* it hits the tubes to maintain a clean platform.

---

### **Session Library (Active Presets)**
*Appending to memory...*

2. Preset Name: "Chess Club Duo - Tele/339"
Target: Muddy Waters / Buddy Guy (1950s Chicago).
Guitar: Multi-Target (Fender Telecaster SC / Gibson ES-339 HB).
Physics Goal: Authentic power-tube edge-of-breakup via vintage 5E3 circuit. Heavy reliance on Pre-EQ to manage pickup discrepancies and FRFR translation.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Tele: 30% / 339: 10%], Thresh [-65dB / -70dB], Decay [150ms].
Block 2 (EQ-8): 
- Tele: Low Shelf [+2.0dB @ 200Hz], LPF [4.5kHz]. 
- 339: HPF [110Hz], Peak [+1.5dB @ 1.2kHz].
Block 3 (Amp - US DLX Tweed Jump): 
- Tele: Vol Norm [3.5/5.5], Vol Bright [4.5/6.5], Tone [4.5/5.0].
- 339: Vol Norm [2.0/3.5], Vol Bright [2.5/4.5], Tone [6.5/7.0].
Block 4 (Cab - 112 US DLX Tweed): Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 0.8, Dist 4.0"). Mix varied by pickup type.
Block 5 (Tape Delay): Mix [12-22%], Time [110ms], Fdbk [0-15%], Drive [10%].
Block 6 (Room Reverb): Mix [15-24%], Decay [1.0s-1.5s], HP [150Hz], LP [4000Hz].
Lane Output Compensation: Tele (+3.0dB / +4.5dB), 339 (+7.0dB / +8.5dB).