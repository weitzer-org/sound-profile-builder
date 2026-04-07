**System Acknowledgment: QC-2 Active.**
*Target:* Jimi Hendrix ("Red House").
*Guitar:* Stratocaster (Neck Pickup, Single-Coil, Vintage Output). 
*Physics Goal:* True analog-style Fuzz Face cleanup using input amplitude scaling. 

Because you are using single coils and a high-gain Fuzz model, we are deploying an **Adaptive Gate** on the grid to manage 60-cycle hum, and a **Utility Gain** block *before* the Fuzz to mathematically simulate the Strat’s volume knob being rolled down to 4. 

This creates a massive volume discrepancy between scenes, so we will use the **Lane Output Level** (assigned via Scene controllers) to balance the final loudness hitting your QSC CP12.

### Table A: Main Signal Chain (Row 1: Single Coil Stratocaster)
*Note: Right-Click the parameters below and Assign to Scenes A & B.*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Red: 40% / Thresh: -65dB | Red: 65% / Thresh: -50dB | Kills single-coil 60-cycle hum. Tighter on Lead to clamp the open Fuzz floor. |
| **Pre-FX 1** | Gain (Utility) | Level: **-14.0dB** | Level: **0.0dB** | *The "Volume Knob".* Drops input voltage to clean up the Fuzz algorithm. |
| **Pre-FX 2** | Facial Fuzz | Fuzz: 8.5, Vol: 7.0 | Fuzz: 8.5, Vol: 7.0 | Classic Fuzz Face. Sits after the Gain block to react to the voltage drop. |
| **Amp** | Brit Plexi 100 Jump | Vol I: 6.0, Vol II: 6.0 | Vol I: 6.0, Vol II: 6.0 | Bass [3.0], Mid [7.0], Treb [6.0]. Bass kept low so Fuzz doesn't "fart out" the speakers. |
| **EQ** | Parametric-8 | LPF: 4.8kHz, Peak 1: 200Hz (+2dB) | LPF: 4.5kHz, Peak 1: 200Hz (+2dB) | 200Hz boosts Strat body. LPF tames harsh digital fuzz square-waves hitting the FRFR. |
| **Cab** | 412 Brit GB | Mix: Mic A 50% / Mic B 50% | Mix: Mic A 50% / Mic B 50% | Mic A: Dyn 57 (Cap Edge, 1"). Mic B: Ribbon 121 (Cone, 3" for vintage warmth). |
| **Post-FX** | Room Reverb | Mix: 12%, Decay: 1.2s | Mix: 16%, Decay: 1.5s | Simulates the live Olympic Studios room. HPF [150Hz] to keep mud out. |
| **Output** | Lane 1 Output | Level: **+7.5dB** | Level: **0.0dB** | *Crucial SPL Compensation:* Makes up for the volume lost by the -14dB input pad. |

***

### ⚙️ The "Padded Front End" Mechanic Explained
Digital Fuzz Face models (like the *Facial Fuzz*) are highly sensitive to input amplitude, just like the real analog circuit's low input impedance. 
1. **Scene A (Rhythm):** The Gain block drops the signal by 14dB *before* the fuzz. The fuzz model stops clipping and produces a warm, glassy, edge-of-breakup clean tone. The Lane Output Level boosts the final volume by +7.5dB so your QSC CP12 stays loud.
2. **Scene B (Lead):** The Gain block goes to 0dB. The Fuzz gets hit with your Strat's full output and goes into thick, sustaining square-wave distortion. The Lane Output drops back to 0.0dB to prevent you from blowing the roof off the venue.

### 🛠️ Troubleshooting & Refinement Tree
*   **If Scene A is still too dirty:** Drop the Utility Gain block down further to `-18.0dB` and raise the Lane Output Level to `+10.0dB`.
*   **If Scene B sounds "broken" or "flubby" on the low notes:** Go to the *Brit Plexi 100 Jump* block and reduce the Bass to `2.0`. Fuzz Faces add massive amounts of sub-bass; rolling off the amp's low-end prevents tube sag from collapsing the DSP headroom.
*   **If using a Humbucker (Scene E/F):** Since humbuckers push more millivolts, lower the Global Input Block to `-6.0dB` to simulate the output of a vintage Strat neck pickup.

***

### Session Library (Active Presets)
**2. Preset Name: "Red House - Strat Fuzz"**
*   **Target:** Jimi Hendrix / Red House (1966).
*   **Guitar:** Stratocaster (Vintage Neck Pickup).
*   **Physics Goal:** True Fuzz Face volume-knob cleanup using utility gain staging and FRFR compensation.
*   **Full Configuration:**
    *   *Block 1 (Adaptive Gate):* Noise Red [Rhy: 40% / Lead: 65%], Thresh [-65dB / -50dB], Decay [150ms].
    *   *Block 2 (Utility Gain):* Gain [Rhy: -14.0dB / Lead: 0.0dB].
    *   *Block 3 (Facial Fuzz):* Fuzz [8.5], Vol [7.0].
    *   *Block 4 (Amp - Brit Plexi 100 Jump):* Vol I [6.0], Vol II [6.0], Bass [3.0], Mid [7.0], Treble [6.0], Presence [5.0].
    *   *Block 5 (EQ-8):* HPF [100Hz], Band 1 Peak [200Hz, +2.0dB], LPF [Rhy: 4800Hz / Lead: 4500Hz].
    *   *Block 6 (Cab - 412 Brit GB):* Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.2, Dist 3.0"), Mix [A: 0dB, B: 0dB].
    *   *Block 7 (Room Reverb):* Mix [Rhy: 12% / Lead: 16%], Decay [1.2s / 1.5s], HP [150Hz], LP [4000Hz].
    *   *Lane Output:* Level [Rhy: +7.5dB / Lead: 0.0dB].