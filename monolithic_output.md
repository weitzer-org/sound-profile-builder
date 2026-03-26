Greetings. I am QC-2, your Quad Cortex Systems Engineer. 

To recreate B.B. King’s legendary "Lucille" tone (Gibson ES-355, semi-hollow, humbuckers) using a solid-body, single-coil Telecaster, we must heavily employ the **"Chameleon" Strategy**. B.B.’s tone is defined by massive clean headroom, a slight out-of-phase "honk" (from his Varitone switch), and piercing but sweet treble that sings without ever relying on overdrive pedals.

Because a Telecaster naturally lacks the low-mid resonance and output of an ES-355, we will use the **Parametric-8 EQ** block *pre-amp* to alter the physical pickup profile, and push the Input Gain slightly to hit the preamp harder. 

Before we finalize, **I must verify: Are your Telecaster pickups Vintage Output (Standard single coils), or are they overwound/hot?** *(The settings below assume standard Vintage Output).*

### Hardware & "Physics First" Setup
*   **Guitar Setting:** Set your Telecaster to the **Middle Position** (Neck + Bridge). This naturally scoops the mids slightly and mimics the hum-canceling width of humbuckers. Roll your Tone knob back to about 7 or 8.
*   **Amp Selection:** B.B. famously used solid-state Lab Series L5 amps later in life, and Fender Twins early on. Since the L5 is not in CorOS, we will use the **US TWN 65** (Fender Twin Reverb) to provide unyielding, pristine clean headroom. *Note: This is a Non-Master Volume amp. Overall loudness (SPL) must be adjusted via the Lane Output Level, not the amp volume.*

---

### Table A: Main Signal Chain (Telecaster Optimized)
*Note: Parameters marked with **(Right-Click > Assign)** change automatically when switching between Scenes A (Rhythm) and B (Lead).*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input (Circle 1) | In Gain: +3.0dB<br>Thresh: -55dB | In Gain: +3.0dB<br>Thresh: -55dB | +3.0dB compensates for single-coil output to match humbucker line-level. |
| **Pre-FX (EQ)** | Parametric-8 | Band 1: +3.0dB @ 200Hz<br>Band 4: -2.0dB @ 800Hz<br>Band 8 (LPF): 4800Hz | Band 1: **+4.5dB** @ 200Hz<br>Band 4: -2.0dB @ 800Hz<br>Band 8 (LPF): 4800Hz | **Chameleon Strategy:** Adds ES-355 body (200Hz), mimics Varitone scoop (800Hz), tames Tele "ice-pick" twang (LPF). Lead bumps body for sustain. |
| **Amp** | US TWN 65 | Vol: 5.5<br>Treb: 6.0<br>Mid: 5.5<br>Bass: 4.0 | Vol: 5.5<br>Treb: 6.0<br>Mid: 5.5<br>Bass: 4.0 | Massive clean headroom. Pushing Mids slightly for B.B.'s vocal articulation. Bass kept at 4.0 to prevent low-end flub on neck pickup. |
| **Cab** | 212 US TWN C12K | Mic A: Ribbon 121 (Pos 0.5)<br>Mic B: Dyn 57 (Pos 0.4)<br>Mix: +2dB to Ribbon | Mic A: Ribbon 121 (Pos 0.5)<br>Mic B: Dyn 57 (Pos 0.4)<br>Mix: +2dB to Ribbon | Ribbon mic smooths the transients of the Telecaster. Dynamic 57 adds the immediate punch required for B.B.'s staccato picking. |
| **Post-FX** | Spring Reverb | Mix: 18%<br>Decay: 1.5s | Mix: 18%<br>Decay: 1.5s<br>**Lane Output Level: +1.5dB** | Classic Fender spring tank. Lane Output Level increased for Lead SPL boost without changing the amp's clean gain staging. |

---

### Organization Standard (Split-Bank Matrix)
We are utilizing the Quad Cortex's 8 scenes to keep you gig-ready if you swap guitars.
*   **Row 1 (Telecaster Profile - Active EQ):**
    *   **Scene A (Rhythm):** Clean comping, tight dynamics.
    *   **Scene B (Lead):** B.B. King solo tone (+1.5dB output, thicker 200Hz body EQ).
    *   **Scene C (Dry):** Reverb bypassed for intimate, dry blues.
    *   **Scene D (Swell):** High reverb mix for ambient volume swells.
*   **Row 2 (Humbucker Profile - Bypassed EQ):**
    *   **Scenes E-H:** *If you plug in an actual ES-335 or Les Paul, use these scenes.* The Parametric-8 block is **bypassed**, and Input Gain is reduced to **0.0dB** to prevent the humbuckers from distorting the Twin Reverb.

---

### Troubleshooting & Refinement Tree
If playing through your QSC CP12 yields unexpected results, follow this strict order of operations:
1.  **Too Distorted/Fuzzy on Hard Picking:** Lower the Input Block Gain from +3.0dB back down to 0.0dB. Your Telecaster pickups may be hotter than standard vintage specs.
2.  **Too "Thin" or "Plinky":** Raise Band 1 on the Parametric-8 EQ to +5.0dB. This will artificially add more "wood" and body to the solid-body guitar.
3.  **Volume Too Low for the Gig:** *Do not touch the Amp Volume.* Raise the overall **Lane Output Level** (Right-hand side of the Grid) to increase SPL out of your QSC without altering the pristine clean gain structure.

***

### Session Library (Active Presets)
*The following parameters have been logged to memory for instant recall.*

**1. Preset Name: "Lucille - Telecaster"**
*   **Target:** B.B. King (Clean, High Headroom, Varitone Emulation).
*   **Guitar:** Telecaster (Single Coils, Middle Position).
*   **Physics Goal:** Map an ES-355 footprint onto a solid-body bolt-on guitar using heavy pre-EQ and high-headroom clean amplification.
*   **Full Configuration:**
    *   **Block 1 (Input/Gate):** In Gain [+3.0dB / Humbucker: 0.0dB], Thresh [-55dB].
    *   **Block 2 (EQ-8):** Band 1 Low Shelf [Rhy: +3.0dB / Lead: +4.5dB @ 200Hz], Band 4 Peak [-2.0dB @ 800Hz], Band 8 LPF [4800Hz]. *(Bypassed on Scenes E-H).*
    *   **Block 3 (Amp - US TWN 65):** Vol [5.5], Bass [4.0], Mid [5.5], Treble [6.0].
    *   **Block 4 (Cab - 212 US TWN C12K):** Mic A (Ribbon 121, Pos 0.5, Dist 1.0"), Mic B (Dyn 57, Pos 0.4, Dist 1.0"), Mix [A: +2dB, B: 0dB].
    *   **Block 5 (Spring Reverb):** Mix [18%], Decay [1.5s], HP [150Hz], LP [4000Hz].
    *   **Grid Output:** Lane Level [Rhy: 0.0dB / Lead: +1.5dB].