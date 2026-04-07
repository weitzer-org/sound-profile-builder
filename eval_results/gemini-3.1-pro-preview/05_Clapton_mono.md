**QUAD CORTEX SYSTEMS ENGINEER "QC-2"**
**TARGET IDENTIFICATION:** Eric Clapton (Cream Era, 1966-1968). 
**ANALOG SPECS:** Marshall JTM45/100 or 1959 Super Lead 100W, cranked. Gibson Les Paul/SG/335 with PAF Humbuckers. 
**SIGNATURE TONE:** "Woman Tone" — created by running the neck or bridge humbucker with the guitar's physical tone knob rolled down to 0–2, pushing the mid-frequencies of a heavily saturated, non-master volume Marshall tube amp. 

To recreate this accurately through a **QSC CP12 Active PA Speaker**, we must manage the extreme volume jump that occurs when simulating a cranked Plexi circuit. Since vintage Marshalls do not have a Master Volume, we will use the Quad Cortex's **Lane Output Level** to control your stage volume (SPL) without altering the tube saturation.

*Trigger Verification:* Are your pickups Vintage Output, Medium, or High Output? (The settings below assume Vintage/Medium PAF specs).

---

### **1. ORGANIZATION STANDARD (SPLIT-BANK MATRIX)**
*We will use a unified grid, but bypass specific EQ blocks depending on the guitar being used.*
*   **Row 1 (Scenes A-D): Fender Telecaster (Single Coil)** - Uses Pre-EQ to simulate humbucker body and roll off high-end to emulate the "Woman Tone" curve.
*   **Row 2 (Scenes E-H): Gibson ES-339 (Humbucker)** - Pure, direct amp interaction. Relies on the physical tone knobs on your Gibson.
    *   **A / E:** Rhythm (Amp volumes backed off slightly, tighter feel).
    *   **B / F:** Lead ("Woman Tone" - Amp fully cranked, output leveled).
    *   **C / G:** Dry (Reverb bypassed for in-your-face blues comping).
    *   **D / H:** Psychedelic (Adds a NuVibe to simulate "Badge" or Cream's psychedelic era).

---

### **2. MULTI-GUITAR TARGET CONFIGURATION**

#### **GUITAR 1: GIBSON ES-339 (Humbuckers)**
*   **Input Block Logic:** Set Global Input Gain to **0.0dB**. 
*   **The "Woman Tone" Action:** You *must* use your physical guitar controls. Select the Neck pickup (or bridge) and roll your physical Tone knob down to **1 or 2**. Leave your Volume knob at 10 for Leads, and roll it down to 6 for Rhythm.
*   **Routing:** Bypasses the Parametric-8 EQ block entirely. Let the mahogany and humbucker do the physical work.

#### **GUITAR 2: FENDER TELECASTER (Single Coil)**
*   **Input Block Logic:** Set Global Input Gain to **+3.0dB** (Telecasters have lower output; we must compensate so the virtual tubes saturate properly).
*   **The "Chameleon" Action:** Do *not* roll off your Telecaster tone knob (it will just get muddy due to single-coil inductance). Instead, we will engage the **Parametric-8** block.
*   **EQ-8 Physics:** 
    *   *Band 1 (Low Shelf):* +4.0dB at 250Hz (Simulates humbucker/mahogany body).
    *   *Band 8 (High Cut / LPF):* 12dB/Oct curve at **1.8kHz**. (This precisely mimics the resonant peak shift of a rolled-off Gibson tone pot).

---

### **3. SIGNAL CHAIN & EXECUTION PROTOCOL**

**Table A: Main Signal Chain**
*(Right-Click > Assign specific parameters to Scenes A/E and B/F as mapped below)*

| Block Category | Model Name | Rhythm Settings (Sc A/E) | Lead Settings (Sc B/F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -60dB | Plexis are noisy when cranked. Higher threshold on Lead masks 60-cycle hum. |
| **Pre-FX (Tele Only)** | Parametric-8 | Active (Tele) / Bypass (339) | Active (Tele) / Bypass (339) | Chameleon Strategy: Simulates the "Woman Tone" LPF curve for single coils. |
| **Pre-FX (Mod)** | NuVibe | Bypass | Bypass | Activate on Scenes D/H for psychedelic rotary swirl ("Badge" tones). |
| **Amp** | Brit Plexi 100 Jumped | Vol Norm: 5.5<br>Vol High: 6.0 | Vol Norm: 8.5<br>Vol High: 8.5 | *No Master Volume.* Jumped channels blend bright attack with dark, sagging bass. |
| **Amp EQ** | Brit Plexi 100 Jumped | Bass: 3.5, Mid: 7.0, Treb: 6.0 | Bass: 3.0, Mid: 8.5, Treb: 5.5 | Lowering bass prevents "farting out" as tube sag increases in Lead scene. |
| **Amp Output** | Brit Plexi 100 Jumped | Output Level: +2.0dB | Output Level: -3.5dB | *Crucial:* Cranking virtual preamp pots causes a massive SPL jump. This balances the volume to the PA. |
| **Cab** | 412 Brit Green | Mic A (57): Pos 1.5, Dist 1.0" | Mic A (57): Pos 1.5, Dist 1.0" | 25W Greenbacks compress perfectly under load. |
| **Cab Mic B** | Ribbon 121 | Pos: 3.0, Dist: 2.0" (-3dB Mix) | Pos: 3.0, Dist: 2.0" (-3dB Mix) | Adds low-mid girth to the QSC CP12 without piercing top-end. |
| **Cab EQ** | High/Low Cut | LPF: 6.5kHz, HPF: 95Hz | LPF: 6.5kHz, HPF: 95Hz | Tames digital high-end fizz hitting the active QSC tweeter. |
| **Post-FX** | Room Reverb | Mix: 15%, Decay: 1.2s | Mix: 20%, Decay: 1.5s | Simulates a loud amp in an acoustic space (Royal Albert Hall). |

---

### **4. TROUBLESHOOTING & REFINEMENT TREE**
If your tone through the QSC CP12 is **"Too Distorted"** or **"Too Fuzzy"** (which can happen if your PAFs are wound hotter than vintage spec):
1.  **Input Pad:** Lower the Input Block Gain to **-3.0dB** or **-6.0dB**. This effectively mimics lowering your guitar's volume knob without losing treble fidelity.
2.  **Amp Gain:** Reduce *Vol High* and *Vol Normal* by 1.5 increments in the Amp Block.
3.  **Tube Sag Control:** If the amp sounds broken, flubby, or "farty" on the low notes, drop the Amp Block **Bass** parameter to 2.0. Vintage Marshall circuits do not handle excessive bass well when fully saturated.
4.  **Output Compensation:** Never use a compressor to fix this. Raise the Lane Output Level to recover lost loudness.

---

### **SESSION LIBRARY (ACTIVE PRESETS)**

**1. Preset Name:** "Sunshine Woman - Dual Routing"
**Target:** Eric Clapton (Cream, 1967) / "Sunshine of Your Love" / "SWLABR".
**Guitar:** Gibson ES-339 / Fender Telecaster.
**Physics Goal:** Non-master volume pure tube saturation. Woman tone replication via physical tone pot (Gibson) or extreme LPF EQ (Telecaster).
**Full Configuration:**
*   **Block 1 (Input Gate):** Global Gate, Thresh [Rhy: -65dB / Lead: -60dB]. Input Gain [ES-339: 0.0dB / Tele: +3.0dB].
*   **Block 2 (Parametric-8):** [Scene A-D Active / Scene E-H Bypassed]. Band 1 Peak [250Hz, +4.0dB, Q: 1.0], Band 8 LPF [1800Hz, 12dB/Oct].
*   **Block 3 (NuVibe):** Mix [45%], Chorus Mode, Rate [4.5Hz], Depth [70%]. (Active Sc D/H only).
*   **Block 4 (Amp - Brit Plexi 100 Jumped):** Vol Norm [Rhy: 5.5 / Lead: 8.5], Vol High [Rhy: 6.0 / Lead: 8.5], Bass [Rhy: 3.5 / Lead: 3.0], Mid [Rhy: 7.0 / Lead: 8.5], Treble [Rhy: 6.0 / Lead: 5.5], Presence [5.0], Amp Level [Rhy: +2.0dB / Lead: -3.5dB].
*   **Block 5 (Cab - 412 Brit Green):** Mic A (Dyn 57, Pos 1.5, Dist 1.0"), Mic B (Ribbon 121, Pos 3.0, Dist 2.0"), Mix [A: 0dB, B: -3dB], HPF [95Hz], LPF [6500Hz]. 
*   **Block 6 (Room Reverb):** Mix [Rhy: 15% / Lead: 20%], Decay [Rhy: 1.2s / Lead: 1.5s], Pre-Delay [15ms], Low Cut [150Hz], High Cut [3000Hz].
*   **Routing:** Out 1/2 assigned to Lane 1. Lane Output Level used for global gig SPL adjustments.