Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To capture the legendary Southern Rock slide tone (reminiscent of Duane Allman or Derek Trucks) using a cranked American Tweed circuit, we must manage the extreme dynamics of a glass or brass slide. When a Tweed amp is pushed, the low-end "sag" can quickly turn into unusable fuzz, and slide friction can create piercing high-frequency artifacts. 

**Physics First Protocol:** We will use the **US Tweed Basslad Jumped** (QC pseudonym for a 4x10 Fender Bassman with jumpered channels). Because vintage Tweed models **do not have a Master Volume**, we will rely strictly on preamp volume for drive and use the *Lane Output Level* to govern stage SPL through your QSC CP12. 

*Note per System Directives (Rule 9): Although you specified dual humbuckers, my protocol requires me to deliver perfectly gain-staged matrices for BOTH an ES-339 (Humbuckers) and a Telecaster (Single Coils). Ensure you are on the correct Scene Row for your instrument.*

***

### TARGET 1: Gibson ES-339 (Humbuckers) 
**Organization:** Row 2 (Scenes E - H)
**Physics Goal:** Humbuckers push a lot of low-midrange energy. A cranked Tweed Basslad will "fart out" or sound broken if we do not drastically cut the Bass knob on the amp and drop the Global Input Gain. We use a Myth Drive to push the midrange for singing sustain without adding bass.

**Global Input Level:** Pad to **-3.0dB** (Prevents humbuckers from clipping the input converters and muddying the Tweed preamp).

#### Table A: Main Signal Chain (ES-339 Humbuckers)
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)" in Cortex Control.*

| Block Category | Model Name | Rhythm Settings (Scene E) | Lead Slide Settings (Scene F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -60dB, Red: 25% | Thresh: -65dB, Red: 15% | Lower noise reduction on lead allows the note to sustain into feedback naturally. |
| **Pre-FX (Drive)** | Myth Drive | Bypass | Drive: 2.5, Tone: 6.5, Vol: 8.0 | Klon-style circuit boosts upper-mids (around 1kHz) to force the slide to cut through the mix. |
| **Amp** | US Tweed Basslad Jumped | Vol Norm: 3.5, Vol Bright: 4.5<br>Bass: 2.0, Mid: 6.5, Treb: 6.0 | Vol Norm: 5.5, Vol Bright: 7.0<br>Bass: 2.0, Mid: 7.5, Treb: 6.0 | *No Master Volume.* Bass is kept at 2.0 to prevent tube sag/farting. Bright channel pushed for lead sustain. |
| **Cab** | 410 Basslad PR10 | Mic A: Dyn 57 (Pos 0.5, Dist 1")<br>Mic B: Rib 121 (Pos 1.5, Dist 4") | Mix A: 0dB<br>Mix B: -2dB | Ribbon mic adds lower-mid warmth; 57 captures the attack of the slide. |
| **Post-FX (EQ)** | Parametric-8 | HPF: 100Hz, LPF: 6000Hz | HPF: 100Hz, LPF: 5000Hz | LPF (Low Pass Filter) is lowered to 5kHz on lead to physically chop off fret/slide scraping noises. |
| **Post-FX (Delay)** | Analog Delay | Mix: 15%, Time: 120ms | Mix: 25%, Time: 350ms | Short slapback for rhythm; longer, darker analog trails for thick, stadium-style lead sustain. |
| **Output Level** | Lane Output | Level: 0.0dB | Level: +1.5dB | Replaces Master Volume to increase SPL for the FOH/PA speaker. |

***

### TARGET 2: Fender Telecaster (Single Coils)
**Organization:** Row 1 (Scenes A - D)
**Physics Goal:** Single coils lack the natural compression and low-end girth of humbuckers. To achieve thick slide sustain, we must increase the Input Gain to hit the Tweed's preamp tubes harder, boost the 200Hz body frequencies, and aggressively clamp down on 60-cycle hum and ice-pick highs.

**Global Input Level:** Boost to **+2.5dB** (Compensates for lower magnetic pickup output, forcing the amp into earlier saturation).

#### Table B: Main Signal Chain (Telecaster Single Coils)
*Mark Scene-Specific changes clearly with "(Right-Click > Assign)" in Cortex Control.*

| Block Category | Model Name | Rhythm Settings (Scene A) | Lead Slide Settings (Scene B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Thresh: -50dB, Red: 50% | Thresh: -55dB, Red: 40% | Higher reduction (%) required to combat 60-cycle single-coil hum under high gain. |
| **Pre-FX (Comp)** | Optical Comp | Comp: 35%, Gain: 4.5 | Comp: 60%, Gain: 6.0 | LA-2A style optical compression physically grabs the slide note and holds it, mimicking humbucker sustain. |
| **Amp** | US Tweed Basslad Jumped | Vol Norm: 5.0, Vol Bright: 5.5<br>Bass: 3.5, Mid: 6.0, Treb: 5.0 | Vol Norm: 7.5, Vol Bright: 8.5<br>Bass: 3.0, Mid: 7.0, Treb: 4.5 | Tele bridge pickups are bright; Treble is rolled back. Amp volume must be pushed harder for single coils. |
| **Cab** | 410 Basslad PR10 | Mic A: Dyn 57 (Pos 1.5, Dist 2")<br>Mic B: Rib 121 (Pos 0.5, Dist 2") | Mix A: -3dB<br>Mix B: 0dB | Swapped mic dominance: Ribbon 121 takes priority to smooth out single-coil "spank". |
| **Post-FX (EQ)** | Parametric-8 | Band 1: +3.0dB @ 200Hz | Band 1: +4.5dB @ 200Hz<br>LPF: 4000Hz | 200Hz adds physical "weight" to the Tele. LPF aggressively cuts at 4kHz to kill ice-pick frequencies. |
| **Post-FX (Delay)** | Tape Delay | Mix: 12%, Time: 100ms | Mix: 20%, Time: 300ms | Tape delay adds harmonic saturation to the echoes, warming up the single-coil decay. |
| **Output Level** | Lane Output | Level: 0.0dB | Level: +1.5dB | Overall volume jump for the solo. |

***

### 6. Troubleshooting & Refinement Tree
If you plug in your dual humbucker guitar and the tone feels "Too Distorted" or "Broken/Farty" through your QSC CP12:
1. **Input Pad:** Lower Global Input Gain from -3.0dB to -6.0dB (Simulates rolling off your guitar's volume knob slightly).
2. **Tube Sag Correction:** The Tweed Basslad model accurately models tube power-amp sag. If low notes "collapse" or sound fuzzy, reduce the Amp Block **Bass** parameter down to 1.0 or 0.5. (Tweed circuits generate their own low-end when cranked).
3. **Output Compensation:** If cleaning up the amp block drops your volume too much, DO NOT add a compressor. Raise the **Lane Output Level** by +2.0dB.

***

### Session Library (Active Presets)
**2. Preset Name: "Southern Slide - Dual Profile"**
*   **Target:** Duane Allman / Derek Trucks (Southern Rock Slide).
*   **Guitars:** ES-339 Humbuckers (Row 2) & Telecaster Single Coils (Row 1).
*   **Physics Goal:** Manage Tweed low-end sag, eliminate high-frequency slide friction, and generate singing sustain without relying heavily on high-gain distortion blocks.
*   **Full Configuration (ES-339):**
    *   *Block 1 (Adaptive Gate):* Thresh [-60dB / -65dB], Red [25% / 15%].
    *   *Block 2 (Myth Drive):* [Bypass / Active], Drive [2.5], Tone [6.5], Vol [8.0].
    *   *Block 3 (US Tweed Basslad Jumped):* Vol Norm [3.5 / 5.5], Vol Bright [4.5 / 7.0], Bass [2.0], Mid [6.5 / 7.5], Treble [6.0]. 
    *   *Block 4 (410 Basslad PR10):* Mic A (Dyn 57, Pos 0.5, Dist 1"), Mic B (Rib 121, Pos 1.5, Dist 4"), Mix A [0dB], Mix B [-2dB].
    *   *Block 5 (Parametric-8):* HPF [100Hz], LPF [6000Hz / 5000Hz].
    *   *Block 6 (Analog Delay):* Mix [15% / 25%], Time [120ms / 350ms].
*   **Full Configuration (Telecaster):**
    *   *Block 1 (Adaptive Gate):* Thresh [-50dB / -55dB], Red [50% / 40%].
    *   *Block 2 (Optical Comp):* Comp [35% / 60%], Gain [4.5 / 6.0].
    *   *Block 3 (US Tweed Basslad Jumped):* Vol Norm [5.0 / 7.5], Vol Bright [5.5 / 8.5], Bass [3.5 / 3.0], Mid [6.0 / 7.0], Treble [5.0 / 4.5].
    *   *Block 4 (410 Basslad PR10):* Mix A [-3dB], Mix B [0dB].
    *   *Block 5 (Parametric-8):* Band 1 (+3.0dB / +4.5dB @ 200Hz), LPF [Bypass / 4000Hz].
    *   *Block 6 (Tape Delay):* Mix [12% / 20%], Time [100ms / 300ms].