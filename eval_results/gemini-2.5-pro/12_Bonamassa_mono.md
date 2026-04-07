Acknowledged. QC-2 online. Analyzing request for a modern Joe Bonamassa lead tone featuring a Dumble-style amplifier. This profile requires a dynamic, touch-sensitive gain structure that cleans up with the guitar's volume knob and provides a smooth, vocal-like sustain for lead work.

The core of this tone is the cascading gain stage of the ODS-style amplifier. We will model this using the **ODS-100 Drive** model, using a pre-boost for saturation and carefully selected cabinet simulation to achieve the required clarity and warmth.

Executing Deep Research Trigger...
*   **Target Artist:** Joe Bonamassa
*   **Core Amplifier:** Dumble Overdrive Special (ODS)
*   **Core Speaker:** Electro-Voice EVM12L or Celestion-loaded custom cabinets.
*   **Common Pedals:** Tube Screamer (for mid-focus and tightening low-end), Analog Delay (Way Huge Aqua-Puss type).
*   **QC Model Mapping:**
    *   **Amp:** ODS-100 Drive
    *   **Cab:** 212 ODS-70s (Closest available Dumble-style cabinet)
    *   **Boost:** Green 808
    *   **Delay:** Analog Mono Delay

To ensure preset integrity, please verify your pickup output level.

**Are your pickups Vintage Output, Medium, or High Output?**

The following presets are built on the Split-Bank Matrix standard. The first table is optimized for the higher output and warmer frequency response of **Gibson ES-339 Humbuckers**. The second table is re-calibrated for the lower output and brighter transient attack of a **Fender Telecaster with Single Coils**, incorporating the "Chameleon" EQ strategy.

***

### **Preset Build 1: Gibson ES-339 (Humbuckers)**

**Row 2 (Scenes E-H):** Optimized for medium-to-high output humbuckers. Focus is on clarity and preventing low-end muddiness.

**Table A: Main Signal Chain (Humbucker Profile)**
*Marked changes are Scene-specific (Right-Click > Assign)*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | **Global Input Gate** | Threshold: **-60.0dB** | Threshold: **-62.0dB** | Humbuckers are inherently quieter. A lower threshold maintains sustain while cutting noise. Lead scene is slightly lower to allow notes to trail off more naturally. |
| **Pre-FX** | **Green 808** | **Bypassed** | **Active**<br>Drive: 2.2<br>Tone: 6.5<br>Level: 8.5 | Pushes the amp's preamp section into saturation. Low drive/high level tightens bass and adds mid-range focus without adding pedal "fizz." |
| **Amp** | **Closest Available Model: ODS-100 Drive** | Gain: **4.5**<br>Ratio: 3.0<br>Master: 7.0<br>Bass/Mid/Treble: 4.5/7.0/6.0<br>Bright/Deep: On/Off | Gain: **6.8**<br>Ratio: 5.5<br>Master: **7.5**<br>Bass/Mid/Treble: 4.5/7.0/6.0<br>Bright/Deep: On/Off | The Ratio knob is critical; it blends the clean and OD channels. The lead scene uses higher Gain/Ratio for saturation and a Master boost for SPL. Bright switch ensures articulation against the humbucker's natural warmth. |
| **Cab** | **212 ODS-70s** | Mic A: Dyn 57 (Pos 0.7)<br>Mic B: Ribbon 121 (Pos 0.5)<br>Level A/B: 0dB/-3.5dB<br>LPF: 8.0kHz | Mic A: Dyn 57 (Pos 0.7)<br>Mic B: Ribbon 121 (Pos 0.5)<br>Level A/B: 0dB/-3.5dB<br>LPF: 8.0kHz | The Dyn 57 provides the aggressive midrange bite. The Ribbon 121, blended in at a lower level, adds body and smooths the top-end. The LPF cuts non-musical "fizz" for the direct-to-PA profile. |
| **Post-FX 1** | **Analog Mono Delay** | **Bypassed** | **Active**<br>Mix: 30%<br>Time: 380ms<br>Feedback: 25% | Classic Bonamassa-style delay. The time is set to a moderate length that adds space without cluttering fast passages. |
| **Post-FX 2** | **Hall Reverb** | Mix: 18%<br>Decay: 2.5s<br>Pre-Delay: 20ms<br>LPF: 4.5kHz | Mix: **25%**<br>Decay: 2.5s<br>Pre-Delay: 20ms<br>LPF: 4.5kHz | Creates the sense of a large room or stage. The LPF on the reverb prevents it from sounding harsh. The lead scene has a slightly wetter mix for a more epic feel. |

***

### **Preset Build 2: Fender Telecaster (Single Coils)**

**Row 1 (Scenes A-D):** Re-calibrated for lower output single coils. Focus is on noise management, boosting body, and taming harsh high-end frequencies.

**Table A: Main Signal Chain (Single Coil Profile)**
*Marked changes are Scene-specific (Right-Click > Assign)*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | **Global Input Gate** | Threshold: **-52.0dB** | Threshold: **-54.0dB** | Single coils have a higher noise floor. A higher (less sensitive) threshold is required to effectively gate the 60-cycle hum without cutting off notes prematurely. |
| **Pre-FX 1** | **Parametric-8** | **Active**<br>Band 1 (Peak): 180Hz, Q 0.7, **+3.0dB**<br>Band 8 (LPF): **4.8kHz** | **Active**<br>Band 1 (Peak): 180Hz, Q 0.7, **+3.0dB**<br>Band 8 (LPF): **5.2kHz** | **Chameleon EQ:** Crucial for this guitar. Band 1 adds the low-mid "body" single coils lack. The LPF tames the "ice pick" treble attack characteristic of a Telecaster bridge pickup. Slightly higher LPF on lead for more cut. |
| **Pre-FX 2** | **Green 808** | **Bypassed** | **Active**<br>Drive: 2.5<br>Tone: 6.0<br>Level: 8.5 | Settings are similar to the humbucker profile, but its role here is even more critical for shaping the Telecaster's thinner tone before it hits the amp. |
| **Amp** | **Closest Available Model: ODS-100 Drive** | Gain: **6.0**<br>Ratio: 4.0<br>Master: 7.0<br>Bass/Mid/Treble: 5.0/7.0/5.5<br>Bright/Deep: **Off**/Off | Gain: **8.2**<br>Ratio: 6.5<br>Master: **7.5**<br>Bass/Mid/Treble: 5.0/7.0/5.5<br>Bright/Deep: **Off**/Off | **Gain Compensation:** Amp Gain is increased significantly (~30%) to compensate for the lower output of single coils. The Bright switch is turned OFF as the Telecaster and Parametric EQ provide sufficient top-end. |
| **Cab** | **212 ODS-70s** | Mic A: Dyn 57 (Pos 0.7)<br>Mic B: Ribbon 121 (Pos 0.5)<br>Level A/B: 0dB/-3.5dB<br>LPF: 8.0kHz | Mic A: Dyn 57 (Pos 0.7)<br>Mic B: Ribbon 121 (Pos 0.5)<br>Level A/B: 0dB/-3.5dB<br>LPF: 8.0kHz | The cabinet and mic settings remain consistent, providing a stable foundation. The tonal changes are made in the pre-amp stage, which is more effective. |
| **Post-FX 1** | **Analog Mono Delay** | **Bypassed** | **Active**<br>Mix: 30%<br>Time: 380ms<br>Feedback: 25% | Identical settings. This effect is not dependent on pickup type. |
| **Post-FX 2** | **Hall Reverb** | Mix: 18%<br>Decay: 2.5s<br>Pre-Delay: 20ms<br>LPF: 4.5kHz | Mix: **25%**<br>Decay: 2.5s<br>Pre-Delay: 20ms<br>LPF: 4.5kHz | Identical settings. |

### **Troubleshooting & Refinement**
*   **If the tone is "Too Distorted/Fuzzy" (especially with high-output pickups):**
    1.  Engage the **Input Pad**. Set the Input Block Gain to **-3.0dB**.
    2.  Reduce the **Amp Block's Gain** knob in increments of 0.5.
    3.  Compensate for any volume loss by increasing the **Lane Output Level**, not the Amp Master.
*   **If the tone is "Too Thin" (especially with the Telecaster):**
    1.  Increase the **Gain** of **Band 1** on the **Parametric-8** block to +4.0dB.
    2.  Slightly increase the **Bass** knob on the **ODS-100 Drive** amp block to 5.5.

Preset build complete. Awaiting feedback for further calibration.

***

**Preset Registry Protocol: Appending to Session Library...**

**2. Preset Name: "JB ODS Lead"**
*   **Target:** Joe Bonamassa modern Dumble-style lead.
*   **Guitar A:** Gibson ES-339 (Humbuckers).
*   **Guitar B:** Fender Telecaster (Single Coils).
*   **Physics Goal:** Achieve dynamic, touch-sensitive overdrive with singing sustain suitable for blues-rock lead, optimized for two distinct pickup types.
*   **Full Configuration (Humbucker - Scene F Lead):**
    *   Block 1 (Input Gate): Thresh [-62.0dB].
    *   Block 2 (Green 808): Drive [2.2], Tone [6.5], Level [8.5].
    *   Block 3 (Amp - ODS-100 Drive): Gain [6.8], Ratio [5.5], Master [7.5], Bass [4.5], Mid [7.0], Treble [6.0], Bright [On].
    *   Block 4 (Cab - 212 ODS-70s): Mic A (Dyn 57), Mic B (Ribbon 121), Mix [A: 0dB, B: -3.5dB], LPF [8.0kHz].
    *   Block 5 (Analog Mono Delay): Mix [30%], Time [380ms], Fdbk [25%].
    *   Block 6 (Hall Reverb): Mix [25%], Decay [2.5s], LPF [4.5kHz].
*   **Full Configuration (Single Coil - Scene B Lead):**
    *   Block 1 (Input Gate): Thresh [-54.0dB].
    *   Block 2 (Parametric-8): Band 1 [Peak, 180Hz, +3.0dB], Band 8 [LPF, 5.2kHz].
    *   Block 3 (Green 808): Drive [2.5], Tone [6.0], Level [8.5].
    *   Block 4 (Amp - ODS-100 Drive): Gain [8.2], Ratio [6.5], Master [7.5], Bass [5.0], Mid [7.0], Treble [5.5], Bright [Off].
    *   Block 5 (Cab - 212 ODS-70s): Mic A (Dyn 57), Mic B (Ribbon 121), Mix [A: 0dB, B: -3.5dB], LPF [8.0kHz].
    *   Block 6 (Analog Mono Delay): Mix [30%], Time [380ms], Fdbk [25%].
    *   Block 7 (Hall Reverb): Mix [25%], Decay [2.5s], LPF [4.5kHz].