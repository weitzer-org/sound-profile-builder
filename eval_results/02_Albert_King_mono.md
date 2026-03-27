Here is your gig-ready Cortex Control build. 

### Analog Research & Physics Translation: Albert King 
Albert King famously played a Gibson Flying V (humbuckers) plugged into **solid-state Acoustic Control Corp amplifiers** (like the Acoustic 260/270). These amps featured massive headroom, a built-in aggressive fuzz/clipping circuit, and 15-inch speakers with high-frequency horns. Furthermore, Albert picked with his bare thumb and index finger, creating a tone that was simultaneously round (flesh attack) and searingly bright (solid-state amp + intense string bending). 

To recreate this on the Quad Cortex for a QSC CP12 without sounding "digital," we will use the highest-headroom solid-state amp in the QC (**Jazz Rivet 120**) combined with a hard-clipping solid-state preamp pedal (**OD250**) to mimic the Acoustic's built-in drive. We will also utilize an EQ block to emulate his bare-finger attack.

Since you are using **Medium Humbuckers**, we will build this on **Row 2 (Scenes E-H)** of the standard split-bank matrix.

---

### Table A: Main Signal Chain (Row 2 - Humbucker Profile)
*Note: Parameters marked with "(Sc E/F)" require Scene Assignment (Right-Click > Assign).*

| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input** | Global Gate | Thresh: -62dB | Thresh: -62dB | Low threshold to capture the extreme dynamic range of bare-finger/hybrid picking. |
| **Pre-FX (Drive)** | OD250 | Gain: 1.5 <br>Level: 6.0 | Gain: 4.5 <br>Level: 7.5 | Replicates the hard-clipping solid-state preamp/fuzz circuit of the Acoustic 260. |
| **Amp** | Jazz Rivet 120 | Vol: 6.5 <br>Mid: 8.5 <br>Treb: 6.0 <br>Bass: 4.5 | Vol: 6.5 <br>Mid: 8.5 <br>Treb: 6.0 <br>Bass: 4.5 | JC-120 is the best solid-state QC model. Bright switch OFF. Mids cranked for Albert's honky, vocal-like sustain. |
| **Cab** | Dual Cab: <br>1x 212 Jazz Rivet <br>1x 115 US Tweed | **Mic A (212):** Dyn 57 (Dist 1.0") <br>**Mic B (115):** Ribbon 121 (Dist 3.0") | Mix: 0dB (A) <br>Mix: -2dB (B) | The 15" speaker replicates the Acoustic 260's massive low-mid footprint. The 2x12 gives the harsh solid-state bite. |
| **Post-EQ** | Parametric-8 | Band 5: +2.5dB @ 800Hz <br>Band 8 (LPF): 4.8kHz | Band 5: +3.5dB @ 800Hz <br>Band 8 (LPF): 4.5kHz | **Chameleon Strategy:** LPF simulates the duller attack of a bare thumb. 800Hz bump mimics the Acoustic 260's high-mid horn. |
| **Post-FX** | Spring Reverb | Mix: 8% <br>Decay: 1.2s | Mix: 8% <br>Decay: 1.2s | Kept extremely dry to replicate the dead, tight "Stax Records" studio room sound. |
| **Output** | Lane Output | Level: 0.0dB | Level: +1.5dB | Overall loudness boost for solos without adding more saturation to the amp block. |

---

### Troubleshooting & Refinement Tree
If the preset isn't interacting perfectly with your specific humbuckers and the QSC CP12:

1. **If the tone is too "Fuzzy" or "Harsh" on Lead:** 
   * *Physics:* Solid-state clipping (OD250) can easily overload if your humbuckers have a high resonant peak.
   * *Fix:* Go to the **Input Block** and lower the Gain to `-3.0dB`. This acts like rolling back your guitar's volume pot just enough to sweeten the hard-clipping diode simulation.
2. **If the tone lacks Albert's "Bite" (Sounds too muffled):**
   * *Physics:* Your humbuckers might be darker than average. 
   * *Fix:* Go to the **EQ-8 Block** and raise the LPF (Band 8) from `4.5kHz` up to `6.0kHz`. Do *not* turn on the Amp's Bright switch, as solid-state bright switches introduce a "shatter" artifact through PA speakers like the CP12.
3. **If the low strings "Fart out" during bends:**
   * *Fix:* Lower the Amp Bass parameter to `3.0` and increase the EQ-8 HPF (Band 1) to `110Hz`. 

---

### Session Library (Active Presets)

**2. Preset Name: "Bad Sign - Albert SolidState"**
*   **Target:** Albert King / Stax Records (Solid State Acoustic 260 replication).
*   **Guitar:** Medium Humbuckers (Simulating Bare Thumb/Finger picking).
*   **Physics Goal:** Massive solid-state headroom + aggressive hard-clipping overdrive, mimicking a 15-inch speaker with a high-frequency horn.
*   **Full Configuration:**
    *   *Block 1 (Global Input Gate):* Threshold [-62dB].
    *   *Block 2 (OD250):* Gain [Rhy: 1.5 / Lead: 4.5], Level [Rhy: 6.0 / Lead: 7.5].
    *   *Block 3 (Amp - Jazz Rivet 120):* Bright [OFF], Vol [6.5], Bass [4.5], Mid [8.5], Treble [6.0].
    *   *Block 4 (Dual Cab):* Cab A [212 Jazz Rivet, Mic: Dyn 57, Pos 0.5, Dist 1.0", Level 0dB], Cab B [115 US Tweed, Mic: Ribbon 121, Pos 0.0, Dist 3.0", Level -2.0dB].
    *   *Block 5 (EQ-8):* HPF [90Hz], Band 5 Peak [+2.5dB Rhy / +3.5dB Lead @ 800Hz, Q: 1.5], LPF [4800Hz Rhy / 4500Hz Lead]. 
    *   *Block 6 (Spring Reverb):* Mix [8%], Decay [1.2s], Tone [4.0].
    *   *Lane Output:* Level [Rhy: 0.0dB / Lead: +1.5dB].