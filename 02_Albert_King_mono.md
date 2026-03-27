**QC-2 Systems Engineer Online.**
**Target Acquired:** Albert King - "Born Under A Bad Sign" (1967, Stax Records).
**Hardware Constraints:** Cortex Control (Mac). QSC CP12. Medium Humbuckers.
**Physics/Analog Logic:** Albert King played a left-handed Gibson Flying V (PAF humbuckers) right-handed, pulling the strings downwards. His signature amp was a solid-state **Acoustic Control Corporation 260**. These amps had zero tube sag, extreme headroom, an immediate transient attack, massive low-mid presence (unlike modern scooped solid-state amps), and a notoriously raspy built-in fuzz circuit. 

To recreate this in CorOS using your medium humbuckers, we will use the **Jazz CH120** (Roland JC-120) to enforce the unyielding solid-state transient response, paired with a radical Parametric EQ to introduce the missing "Acoustic Corp Honk" and tame your pick attack to simulate his bare-thumb technique.

### Pickup Output Compensation Strategy (Medium Humbucker -> Vintage PAF)
*   **Input Block Gain:** Set to **-1.5dB**. This slightly cools off your medium humbuckers to match the dynamics of late 50s PAFs, ensuring the solid-state amp model doesn't hit unmusical digital clipping.
*   **Bank Matrix:** Deploying **Row 2 (Scenes E-H)** as requested by the Humbucker Standard Protocol.

---

### Table A: Main Signal Chain
*Note: Mark Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input | Thresh: -60dB, Gain: -1.5dB | Thresh: -65dB, Gain: -1.5dB | Drops medium HB output to vintage spec. Lower gate on lead catches fuzz noise. |
| **Pre-FX 1** | Parametric-8 EQ | HPF: 100Hz, Band 4 (800Hz): +4.0dB, LPF: 4200Hz | *(Same as Rhythm)* | **The "Thumb & Acoustic" EQ.** Forces the massive midrange honk of the Acoustic 260 while the LPF rolls off plastic pick attack to mimic flesh/thumb transients. |
| **Pre-FX 2** | Facial Fuzz | **BYPASSED** | Fuzz: 3.5, Vol: 6.0 | Simulates the notoriously raspy built-in silicon fuzz of the Acoustic 260. Dialed low for edge/bite rather than doom. |
| **Amp** | Jazz CH120 | Vol: 6.5, Bass: 4.5, Mid: 8.5, Treble: 3.5, Bright: OFF | Vol: 7.0, Bass: 4.5, Mid: 9.0, Treble: 4.0, Bright: OFF | Forces a scooped Roland solid-state amp to act like a mid-heavy Acoustic Corp. Gives the instant, uncompressed transient attack Albert is famous for. |
| **Cab** | 115 US Tweed + 212 Jazz C120 | Cab A: Dyn 57 (Pos 0.2, Dist 1.0"), Cab B: Ribbon 121 (Pos 0.5, Dist 3.0"). Mix: A +2dB, B -2dB | *(Same as Rhythm)* | Albert used large enclosures (15" JBLs/Cerwin Vegas). The 1x15 provides the massive low-end weight; the 2x12 provides the upper-mid cut. |
| **Post-FX 1** | Spring Reverb | Mix: 12%, Decay: 1.8s, Tone: 4.0 | Mix: 18%, Decay: 2.2s, Tone: 4.5 | Stax Records studio had natural room decay, but Albert used slight spring verb to offset the extreme dryness of the solid-state amp. |
| **Output** | Lane 1 Output | Level: 0.0dB | Level: +1.5dB | Overall Scene volume control (Headroom Rule). |

---

### Organization Standard (Row 2 - Humbucker Profile)
*   **Scene E (Rhythm/Stax Comping):** Dry, unyielding solid-state punch. Heavily mid-focused.
*   **Scene F (Lead/The Sting):** Facial Fuzz engaged. Raspy, immediate attack. Use this for the signature intro riff and solos.
*   **Scene G (Dry):** Reverb bypassed completely for ultra-dry studio isolation tracking.
*   **Scene H (Studio LA-2A):** Rhythm tone with an **Opto Comp** (added to end of chain, Peak Red: 40%, Gain: 5.0) simulating the Stax analog console bus compression.

---

### Troubleshooting & Refinement Tree
If the tone is **Too Thin / Missing the "Honk"**:
1. Increase Band 4 (800Hz) and Band 5 (1.5kHz) on the Parametric-8 block by +1.5dB increments. The Acoustic amp sound lives entirely in the low-mids.
2. Ensure your guitar's tone knob is rolled back slightly (around 7 or 8) to assist the LPF in masking the humbucker high-end.

If the Fuzz in Scene F is **Too Distorted/Fuzzy**:
1. Lower the "Fuzz" parameter on the Facial Fuzz to 2.0.
2. Drop the Global Input Gain to -3.0dB (simulates rolling down your guitar's volume pot, which the Facial Fuzz model cleans up beautifully).

---

### Session Registry Protocol (Active Presets)

**Session Library**
1. Preset Name: "Spoonful - ES339" *(From Previous Memory)*
2. Preset Name: "Bad Sign - Acoustic SS"
   **Target:** Albert King / Stax Records (1967).
   **Guitar:** Medium Humbuckers.
   **Physics Goal:** Unyielding solid-state transient response, heavy 800Hz midrange honk, built-in silicon fuzz simulation, thumb-attack EQ curve.
   **Full Configuration:**
   *   Block 1 (Global Input): Gate Thresh [-60dB / -65dB], Gain [-1.5dB].
   *   Block 2 (EQ-8): HPF [100Hz], Band 4 800Hz Peak [+4.0dB], Band 5 1.5kHz Peak [+1.5dB], LPF [4200Hz].
   *   Block 3 (Facial Fuzz): Fuzz [3.5], Vol [6.0]. Bypassed in Rhythm, Engaged in Lead.
   *   Block 4 (Amp - Jazz CH120): Vol [6.5 / 7.0], Bass [4.5], Mid [8.5 / 9.0], Treble [3.5 / 4.0], Bright [OFF], Chorus/Vib [OFF].
   *   Block 5 (Dual Cab): 115 US Tweed (Dyn 57, Pos 0.2, Dist 1.0", Lvl +2dB) + 212 Jazz C120 (Rib 121, Pos 0.5, Dist 3.0", Lvl -2dB).
   *   Block 6 (Spring Reverb): Mix [12% / 18%], Decay [1.8s / 2.2s], Tone [4.0 / 4.5].
   *   Lane Output: Level [0.0dB / +1.5dB].