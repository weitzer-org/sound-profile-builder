Acknowledged. QC-2 online. Analyzing request for Van Halen "Brown Sound" with specific hardware targets: Hot-rodded '68 Marshall Plexi, Variac sag, and plate reverb. The core of this tone is a non-master volume amplifier pushed to its absolute saturation point, with voltage starvation creating compression and sag.

I will construct a dual-profile preset optimized for two distinct guitar types as per protocol:
1.  **Gibson ES-339 (Humbuckers):** High-output pickups. Focus will be on taming input gain to avoid digital clipping while achieving maximum power tube saturation.
2.  **Fender Telecaster (Single Coils):** Lower-output pickups. Focus will be on boosting the signal cleanly before the amp and managing noise.

**Target Tone Analysis:**
*   **Amp:** The Marshall 1959 Super Lead is the target. The "Jumped" configuration links the bright and normal channels for maximum gain and tonal flexibility. The closest available model is the **Brit Plexi Jumped**.
*   **Variac Sag:** The physical effect of a Variac is to lower the wall voltage supplied to the amplifier, causing the power tubes to compress and "sag" more dramatically at a lower overall SPL. This will be simulated using the **Power Sag** parameter within the Amp Block.
*   **Cabinet:** EVH used Celestion G12M Greenbacks. The **412 Brit G12M** is the correct IR profile.
*   **Effects:** The core effects are an MXR Phase 90, an Echoplex EP-3 for slapback delay, and studio plate reverb. These will be mapped to scenes.

Executing build for the Split-Bank Matrix.

---

### **Preset Build: VH Brown Sound**

This preset is designed with two distinct profiles. Row 1 (Scenes A-D) is calibrated for a **Fender Telecaster**. Row 2 (Scenes E-H) is calibrated for a **Gibson ES-339**.

**Guitar Pickup Verification:**
*   **Gibson ES-339:** High Output. Input block gain will be *reduced* to -3.0dB.
*   **Fender Telecaster:** Vintage/Medium Output. Input block will be *boosted* to +3.0dB and supplemented with a Booster block.

---

### **Table A: Main Signal Chain (Telecaster & ES-339 Profiles)**

| Block Category | Model Name | **Telecaster Settings (Scenes A-D)** | **ES-339 Settings (Scenes E-H)** | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Input 1 | Gain: **+3.0dB** | Gain: **-3.0dB** | **Headroom Rule:** Compensating for pickup output differences. The Tele needs a boost to drive the amp; the Humbucker needs a cut to prevent digital "fizz" before the amp block. |
| **Global Gate** | Circle 1 | Threshold: **-55.0dB** | Threshold: **-65.0dB** | **Noise Management:** Single coils require a higher (less sensitive) threshold to combat 60-cycle hum inherent in their design. |
| **Pre-FX** | Horus Drive | **ON**. Gain: 2.0, Tone: 6.5, Level: 7.0 | **OFF (Bypassed)** | **Chameleon Strategy:** The Horus (Klon model) provides a clean, mid-focused boost to fatten the Telecaster signal, simulating the girth of a humbucker hitting the amp's input. |
| **Pre-FX** | Chief CE25 Phaser | *(Assign to Scene C/G)* Rate: 0.9Hz, Depth: 100%, Mix: 50% | *(Assign to Scene C/G)* Rate: 0.9Hz, Depth: 100%, Mix: 50% | Emulates the MXR Phase 90 "script" logo pedal used on early recordings. Slow rate is key. |
| **Pre-FX** | Flangeristo | *(Assign to Scene D/H)* Rate: 0.4Hz, Depth: 70%, Fdbk: 45%, Mix: 30% | *(Assign to Scene D/H)* Rate: 0.4Hz, Depth: 70%, Fdbk: 45%, Mix: 30% | Emulates the MXR Flanger for tones like "Unchained." Placed before the amp for authentic interaction. |
| **Amp** | **Closest: Brit Plexi Jumped** | Vol I: **10.0**, Vol II: **10.0**, Bass: 4.0, Mid: 6.5, Treb: 6.0, Pres: 5.5, **Pwr Sag: 7.5** | Vol I: **10.0**, Vol II: **10.0**, Bass: 3.5, Mid: 6.5, Treb: 6.0, Pres: 5.5, **Pwr Sag: 8.0** | **Variac Simulation:** Maxing both volumes is essential. The high Power Sag setting emulates voltage starvation. Bass is reduced to prevent "flubbing" and maintain tight low-end tracking, which is critical for this sound. |
| **Cab** | Dual Cab | **412 Brit G12M** | **412 Brit G12M** | **Speaker Physics:** This IR matches the Celestion G12M Greenbacks used in the original Marshall cabs. |
| | *Mic A* | Dyn 57, Pos 0.7, Dist 1.0" | Dyn 57, Pos 0.7, Dist 1.0" | The classic dynamic mic choice for this cab. Captures the aggressive upper-midrange bite. |
| | *Mic B* | Ribbon 121, Pos 1.5, Dist 4.0" | Ribbon 121, Pos 1.5, Dist 4.0" | Adds low-mid warmth and body. Pulled back to blend in room character. |
| | *Mix* | A: 0dB, B: -4.5dB | A: 0dB, B: -4.5dB | Primary tone is the SM57; the ribbon mic fills it out. |
| **Post-FX** | Tape Delay | *(Assign to Scene B/F)* Mix: 20%, Time: 120ms, Fdbk: 10%, Drive: 40% | *(Assign to Scene B/F)* Mix: 20%, Time: 120ms, Fdbk: 10%, Drive: 40% | **Spatial Goal:** Emulates an Echoplex EP-3 for a dark, single slapback. The low feedback and high drive simulate tape saturation and decay. |
| **Post-FX** | Plate Reverb | Mix: 15%, Decay: 1.2s, PreDly: 20ms | Mix: 15%, Decay: 1.2s, PreDly: 20ms | The target reverb type. Provides studio-like space without washing out the sound. Not active on Dry scenes (C/G). |
| **Output** | Lane 1 Output | A/C/D: -1.5dB, **B (Lead): 0.0dB** | E/G/H: -1.5dB, **F (Lead): 0.0dB** | **Gain Staging:** Lead scenes (B/F) receive a +1.5dB boost for solos. The base level is slightly padded to ensure ample headroom at the main outputs. |

### **Scene Configuration**

*   **Scene A (Tele - Rhythm):** Base tone. Horus Drive ON.
*   **Scene B (Tele - Lead):** Tape Delay ON, Lane Output +1.5dB.
*   **Scene C (Tele - Phaser):** Chief CE25 Phaser ON. Tape Delay OFF.
*   **Scene D (Tele - Flanger):** Flangeristo ON. Tape Delay OFF.

*   **Scene E (ES-339 - Rhythm):** Base tone. Horus Drive OFF.
*   **Scene F (ES-339 - Lead):** Tape Delay ON, Lane Output +1.5dB.
*   **Scene G (ES-339 - Phaser):** Chief CE25 Phaser ON. Tape Delay OFF.
*   **Scene H (ES-339 - Flanger):** Flangeristo ON. Tape Delay OFF.

---

### **Troubleshooting & Refinement Notes**

*   **If the tone is "Too Fizzy" (especially with the ES-339):** The first step is to lower the **Input Block Gain** to -6.0dB. Do not touch the amp's volume knobs, as they are critical to the core tone. Compensate for any volume loss by raising the **Lane Output Level**.
*   **If the tone is "Too Thin" (especially with the Telecaster):** Increase the **Level** on the **Horus Drive** block before increasing amp gain. This maintains the amp's saturation character while pushing it harder with a clean signal.
*   **To adjust the "Sag":** The **Power Sag** parameter in the amp block is your primary control. Higher values = more compression and a "spongier" feel. Lower values = tighter and more aggressive response.

---
### **Session Library (Active Presets)**

1.  **Preset Name:** "Spoonful - ES339"
    *   *Configuration stored in system memory.*
2.  **Preset Name:** "VH Brown Sound"
    *   **Target:** Van Halen (1978). Variac'd Marshall Super Lead.
    *   **Guitars:** Fender Telecaster (Profile 1), Gibson ES-339 (Profile 2).
    *   **Physics Goal:** Achieve authentic power tube saturation and voltage sag while compensating for different pickup outputs.
    *   **Full Configuration:**
        *   **Input Block:** Gain [+3.0dB / -3.0dB].
        *   **Global Gate:** Threshold [-55dB / -65dB].
        *   **Block 1 (Booster - Horus Drive):** ON for Tele only. Gain [2.0], Level [7.0].
        *   **Block 2 (Mod - Chief CE25 Phaser):** Scene C/G. Rate [0.9Hz].
        *   **Block 3 (Mod - Flangeristo):** Scene D/H. Rate [0.4Hz].
        *   **Block 4 (Amp - Brit Plexi Jumped):** Vol I/II [10.0], Bass [4.0/3.5], Mid [6.5], Treble [6.0], Presence [5.5], Pwr Sag [7.5/8.0].
        *   **Block 5 (Cab - 412 Brit G12M):** Mic A (Dyn 57), Mic B (Ribbon 121), Mix [A: 0dB, B: -4.5dB].
        *   **Block 6 (Delay - Tape):** Scene B/F. Mix [20%], Time [120ms], Fdbk [10%].
        *   **Block 7 (Reverb - Plate):** Mix [15%], Decay [1.2s].
        *   **Output Level:** Rhythm Scenes [-1.5dB], Lead Scenes [0.0dB].