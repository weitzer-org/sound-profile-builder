**System: Quad Cortex Systems Engineer (QC-2)**
**Target Initialization:** John Mayer - "Gravity"
**Hardware Constraint Profile:** QSC CP12 (Active FRFR) | Input: Low-Output Stratocaster (Single Coils).

Welcome to the Cortex Control matrix. Designing a "Gravity" tone requires replicating John Mayer's massive headroom, glass-like top end, and thick low-mids, famously achieved by running a Stratocaster into a Dumble Overdrive Special (Clean Channel) and boosting it with a Klon Centaur. 

### Physics First: Pickup Compensation Strategy
Because your Stratocaster has **low-output pickups**, we must prevent the Dumble amp model from sounding thin or "pingy." 
1. **Global Input Block:** I recommend setting your Global Input Gain to **+2.5dB**. This hits the Dumble's virtual preamp tubes harder, recovering the missing low-mid frequency weight without adding distortion.
2. **The Klon Mid-Hump:** The Klon is highly dependent on incoming signal voltage. We will use the QC's **Myth Drive** as a clean-to-edge-of-breakup line driver to push the midrange during your solos, perfectly counteracting the natural "scoop" of single coils.

Here is your exact build via Cortex Control.

---

### Table A: Main Signal Chain
*Note: To assign Scene variations, Right-Click the parameter in Cortex Control and select "Assign to Scene".*

| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 40%<br>Thresh: -60dB | Noise Red: 55%<br>Thresh: -55dB | Using % reduction directly on the grid tames 60-cycle hum from your Strat without choking the delicate sustain. Higher reduction on Lead to quiet the Klon. |
| **Pre-FX** | Myth Drive | *Bypassed* | Engaged<br>Gain: 2.5<br>Tone: 4.5<br>Level: 7.5 | Klon Centaur topology. High level/low gain pushes the amp's front end. Tone at 4.5 rolls off ice-pick frequencies before they hit the amp. |
| **Amp** | D-Cell Custom Clean | Vol: 5.5<br>Bass: 5.0<br>Mid: 4.5<br>Treble: 6.0<br>Master: 8.0 | Vol: 5.5 *(Keep same)*<br>*(Right-Click Lane Output Level +1.5dB)* | Dumble ODS Clean. Pushing the Master to 8.0 maximizes power-amp headroom. We boost Lane Output Level (not Amp Vol) for the lead to keep it clean but louder. |
| **EQ** | Parametric-8 | Band 1 (Peak): 150Hz (+2.0dB)<br>Band 8 (LPF): 6000Hz | Band 1: 150Hz (+2.0dB)<br>Band 8: 6000Hz | Band 1 adds "Body" to low-output single coils. The LPF at 6kHz removes digital fizz, which is crucial when playing through an FRFR like the QSC CP12. |
| **Cab** | 212 D-Cell | Mic A: Ribbon 121 (Center, 1.0")<br>Mic B: Dyn 57 (Edge, 2.0") | Mic A: 0.0dB<br>Mic B: -3.0dB | Emulates Celestion G12-65s. The Ribbon 121 handles the "glassy" highs smoothly, while the off-axis 57 provides upper-mid attack. |
| **Post-FX 1** | Analog Delay | *Bypassed* | Engaged<br>Mix: 12%<br>Time: 340ms | Subtle bucket-brigade slap/trail to add 3D depth to the solo, typical of Mayer's lead tone. |
| **Post-FX 2** | Plate Reverb | Mix: 25%<br>Decay: 1.8s<br>Pre-Delay: 15ms | Mix: 30%<br>Decay: 2.2s<br>Pre-Delay: 15ms | Emulates the EMT-140 plate reverbs at Capitol Studios. Massive, lush spatial spread without muddying the fundamental note. |

---

### Row/Scene Matrix (Standard Organization)
*   **Row 1 (Single Coils - Active):**
    *   **Scene A (Rhythm):** Massive clean. Use Neck or Neck/Middle pickup (Position 4 or 5).
    *   **Scene B (Lead):** Myth Drive engaged, Delay on, +1.5dB volume jump. Use Neck pickup.
    *   **Scene C (Dry):** Reverb down to 5%, Delay bypassed. For dry chord comping.
    *   **Scene D (Ambient):** Plate Reverb Mix to 45%, Decay to 4.0s for swells.
*   **Row 2 (Humbuckers - Dormant):** *(If you plug in a humbucker guitar, copy Row 1 to Row 2, but drop Input Gain to -3.0dB and lower the Parametric EQ Band 1 body boost to 0.0dB).*

---

### Troubleshooting & Refinement Tree
If the QSC CP12 produces a tone that feels...
1. **"Too Distorted" or "Harsh" on Lead:** 
   * *Action:* Your low-output strat might have spikes in the upper midrange. Lower the **Tone** knob on the Myth Drive to 3.5. Do *not* lower the amp's Master volume (you will lose headroom).
2. **"Too Thin" or "Plinky" on Rhythm:**
   * *Action:* Your pickups lack voltage. Go to the Global Input Block (Circle 1) and raise the input gain to **+4.0dB**. If it's still thin, raise the Amp Block's **Bass** parameter to 6.0.
3. **"Too Boomy" through the PA:**
   * *Action:* Active PA speakers placed on the floor generate bass coupling. Engage the **HPF (High Pass Filter)** in the Cab Block and set it to **85Hz** to clean up the sub-frequencies.

---

### Session Library (Active Presets)

**2. Preset Name: "Gravity - Strat/Dumble"**
*   **Target:** John Mayer (Live/Studio Hybrid).
*   **Guitar:** Stratocaster (Low-Output Single Coils).
*   **Physics Goal:** Massive headroom clean, glassy top-end without FRFR harshness, mid-pushed lead.
*   **Full Configuration:**
    *   **Block 1 (Adaptive Gate):** Noise Red [Rhy: 40% / Lead: 55%], Thresh [-60dB / -55dB].
    *   **Block 2 (Myth Drive):** Bypass [Sc A], On [Sc B], Gain [2.5], Treble [4.5], Output [7.5].
    *   **Block 3 (Amp - D-Cell Custom Clean):** Vol [5.5], Bass [5.0], Mid [4.5], Treble [6.0], Presence [5.0], Master [8.0], Output Level [Rhy: 0.0dB / Lead: +1.5dB].
    *   **Block 4 (EQ-8):** Peak Band 1 [150Hz, +2.0dB], LPF Band 8 [6000Hz].
    *   **Block 5 (Cab - 212 D-Cell):** Mic A (Ribbon 121, Pos 0.0, Dist 1.0"), Mic B (Dyn 57, Pos 0.8, Dist 2.0"), Mix [A: 0dB, B: -3dB], HPF [85Hz].
    *   **Block 6 (Analog Delay):** Mix [12%], Time [340ms], Fdbk [20%], Trails [On].
    *   **Block 7 (Plate Reverb):** Mix [25% / 30%], Decay [1.8s / 2.2s], Pre-Delay [15ms], HP [120Hz], LP [4000Hz].