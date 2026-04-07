As **QC-2**, your Quad Cortex Systems Engineer, I have engineered a dual-profile preset specifically tailored to capture the quintessential late 50s/early 60s Chess Records Chicago Blues tone. 

Our target is a "warm, slightly gritty but clean platform" characteristic of vintage Fender Tweed combos recorded in an open room. To accommodate the physics of both your Gibson ES-339 (Humbuckers) and Fender Telecaster (Single Coils) through a QSC CP12 FRFR speaker, we must utilize strict input gain staging and EQ compensation. Vintage Tweed circuits (like the 5E3 Deluxe) lack a Master Volume and sag heavily under humbucker transients. 

Here is your Split-Bank Configuration. **Row 1 (Scenes A-D)** is calibrated for the Telecaster. **Row 2 (Scenes E-H)** is calibrated for the ES-339.

### Guitar 1: Fender Telecaster (Single Coils)
**Target:** Row 1 (Scenes A-D). 
**Physics Goal:** Vintage Telecasters are bright with a sharp attack and lower output. We will increase the Input Gain to hit the Tweed circuit harder, achieving that natural gritty breakup. We will also use the EQ to add low-mid body and roll off the ice-pick highs to mimic vintage tape/console darkening.

**Pre-Requisite Global Input Setting (In 1):** Set Input Gain to **+2.0dB**. 

#### Table A.1: Telecaster Signal Chain (Row 1)
*Note: Parameters marked with "(Right-Click > Assign)" change between Scene A (Rhythm) and Scene B (Lead).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -55.0dB | Thresh: -60.0dB | Single coils have 60-cycle hum; higher threshold on Rhythm keeps comping tight. |
| **Pre-FX (EQ)** | Parametric-8 | Band 1: +3.0dB (200Hz)<br>High Pass: 80Hz | Band 1: +3.0dB (200Hz)<br>High Pass: 80Hz | Adds essential physical "body" to the single coils. HPF prevents sub-rumble in the QSC. |
| **Pre-FX (Drive)**| Green 808 | Bypass | On (Gain: 0, Tone: 4.5, Level: 7) | Classic mid-hump push for leads without adding artificial fuzz. |
| **Amp** | US Tweed DLX Normal | Inst Vol: 4.5<br>Tone: 6.0 | Inst Vol: 6.0<br>Tone: 6.5 | 5E3 circuit. No Master Vol. Inst Vol creates natural tube overdrive and sag. |
| **Cab** | 112 US Tweed DLX | Mic A: Dyn 57 (Pos 1.0)<br>Mic B: Ribbon 160 (Pos 0.5) | Mic A: Dyn 57 (Pos 1.0)<br>Mic B: Ribbon 160 (Pos 0.5) | Ribbon mic adds warmth. SM57 off-center prevents harsh pick attack through FRFR. |
| **Post-FX (Dly)** | Slapback Delay | Mix: 15% / Time: 120ms | Mix: 20% / Time: 120ms | Simulates Chess Records tape slap / room echo. |
| **Post-FX (Rev)** | Room Reverb | Mix: 18% / Decay: 1.2s | Mix: 22% / Decay: 1.5s | Short decay places the amp in a physical room, avoiding ambient wash. |
| **Output** | Lane Output Level | 0.0dB | +1.5dB | Compensates for the lack of Amp Master Volume to jump the SPL for solos. |

***

### Guitar 2: Gibson ES-339 (Humbuckers)
**Target:** Row 2 (Scenes E-H).
**Physics Goal:** The ES-339 produces higher output and heavier low-midrange frequencies. If we run this directly into a Tweed circuit at unity gain, the virtual power tubes will digitally clip and "fart out" (tube sag collapse). We must pad the input and lean out the EQ to maintain clarity and that "clean platform" grit.

**Pre-Requisite Global Input Setting (In 1):** Set Input Gain to **-4.5dB**. *(Crucial for Headroom Rule).*

#### Table A.2: Gibson ES-339 Signal Chain (Row 2)
*Note: Parameters marked with "(Right-Click > Assign)" change between Scene E (Rhythm) and Scene F (Lead).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65.0dB | Thresh: -65.0dB | Humbuckers are quieter regarding EMI. Lower threshold allows max sustain. |
| **Pre-FX (EQ)** | Parametric-8 | Band 2: -2.5dB (300Hz)<br>High Pass: 100Hz | Band 2: -2.5dB (300Hz)<br>High Pass: 100Hz | Tames humbucker low-mid mud. HPF raised to 100Hz to prevent 5E3 sag collapse. |
| **Pre-FX (Drive)**| Green 808 | Bypass | On (Gain: 1, Tone: 6.0, Level: 6) | Tone is brighter here to cut through the ES-339's natural dark voicing. |
| **Amp** | US Tweed DLX Normal | Inst Vol: 3.5<br>Tone: 7.5 | Inst Vol: 4.5<br>Tone: 8.0 | Lower Inst Vol compared to Tele to retain clean platform. Brighter Tone knob setting. |
| **Cab** | 112 US Tweed DLX | Mic A: Dyn 57 (Pos 0.5)<br>Mic B: Ribbon 160 (Pos 1.5) | Mic A: Dyn 57 (Pos 0.5)<br>Mic B: Ribbon 160 (Pos 1.5) | SM57 moved closer to cone center (0.5) to add bite to the humbuckers. |
| **Post-FX (Dly)** | Slapback Delay | Mix: 12% / Time: 120ms | Mix: 16% / Time: 120ms | Slightly lower mix for humbuckers to prevent rhythmic muddying. |
| **Post-FX (Rev)** | Room Reverb | Mix: 15% / Decay: 1.2s | Mix: 18% / Decay: 1.5s | Studio room simulation. |
| **Output** | Lane Output Level | +2.0dB | +3.5dB | Overall level raised to compensate for the -4.5dB input pad on the humbuckers. |

***

### ⚠️ Troubleshooting & Refinement Tree
If you test this through your QSC CP12 and find the tone is **"Too Distorted" or "Too Fuzzy"** (especially on the ES-339):
1. **Input Pad:** Verify your Input 1 block is set to -4.5dB (or drop it to -6.0dB). This mimics rolling off your guitar volume. 
2. **Amp Gain:** Reduce the Amp block `Inst Vol` by 0.5 increments. 
3. **Tube Sag:** If the low notes sound "broken" or loose, raise the EQ block's High Pass Filter to `120Hz` to starve the Tweed amp of low-frequency energy.
4. **Output Compensation:** If the tone cleans up but is now too quiet, *only* increase the Lane Output Level.

***

### 💾 Session Library (Active Presets)
**2. Preset Name:** "Chess Records Chicago - Split"
**Target:** Muddy Waters / Buddy Guy early studio tones.
**Guitars:** Gibson ES-339 & Fender Telecaster.
**Physics Goal:** Authentic non-master-volume 5E3 Tweed edge-of-breakup with split-gain staging to prevent humbucker clipping and single-coil thinness. 
**Full Configuration (Archived for Recall):**
*Row 1 (Telecaster): Input +2.0dB. Gate Thresh [-55dB / -60dB]. EQ-8: HPF 80Hz, Band 1 +3.0dB (200Hz). TS808: Bypass/On, Gain 0, Level 7. Amp (US Tweed DLX Norm): Inst Vol [4.5 / 6.0], Tone [6.0 / 6.5]. Cab: 112 US Tweed (57 Pos 1.0, 160 Pos 0.5). Output Level [0dB / +1.5dB].*
*Row 2 (ES-339): Input -4.5dB. Gate Thresh -65dB. EQ-8: HPF 100Hz, Band 2 -2.5dB (300Hz). TS808: Bypass/On, Gain 1, Level 6, Tone 6. Amp (US Tweed DLX Norm): Inst Vol [3.5 / 4.5], Tone [7.5 / 8.0]. Cab: 112 US Tweed (57 Pos 0.5, 160 Pos 1.5). Output Level [+2.0dB / +3.5dB].*