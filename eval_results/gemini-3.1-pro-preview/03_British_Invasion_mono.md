**QC-2 Systems Diagnostics & Build Protocol Initialized.**

Target Confirmed: **Early British Invasion (Beatles, The Kinks, Yardbirds)**.
Physics Goal: Class-A Chime, touch-sensitive edge-of-breakup, pronounced upper-midrange sparkle without piercing high-end harshness through a QSC CP12 active monitor. 

To achieve the AC30/JTM45 hybrid chime, we will utilize the **UK C30 Top Boost** model. *Note per protocol: Vintage Vox circuits lack a Master Volume. We will strictly use the Amp Block's "Volume" to control overdrive/chime, and the Block's "Output Level" to normalize your overall PA loudness.*

---

### **Multi-Guitar Gain Staging & Routing Instructions**

*Verification Request:* Are the humbuckers in your ES-339 vintage-spec (PAF style) or high-output modern? I am assuming Vintage/Medium output for these calculations. 

To host both your Fender Telecaster and Gibson ES-339 in a single gig-ready preset without digital clipping, we utilize the **Split-Bank Matrix** strategy. We will assign parameter changes to the Scenes (Right-Click > Assign) to physically alter the AC30's circuit response based on the guitar you are holding.

**Guitar 1: Fender Telecaster (Single Coil) — Scenes A-D (Row 1)**
*   **Physics Challenge:** Single coils lack low-mid weight and can sound "ice-picky" through an active PA speaker.
*   **QC Compensation:** We engage a parametric EQ to boost 200Hz (Body) and aggressively use the AC30's "Tone Cut" (which operates backwards: higher numbers = darker tone) to smooth the pick attack. Amp volume is pushed higher to hit the edge-of-breakup sweet spot.

**Guitar 2: Gibson ES-339 (Humbuckers) — Scenes E-H (Row 2)**
*   **Physics Challenge:** Humbuckers will immediately overload the AC30 model, turning "chime" into muddy fuzz.
*   **QC Compensation:** We reduce the Pre-FX EQ-8 output by -3.0dB to simulate rolling off the guitar volume. We drastically cut the Amp Block's Bass parameter and lower the Amp Volume to maintain the "glassy" clean headroom.

---

### **Table A: Main Signal Chain (British Invasion)**

*Marked Scene-Specific changes clearly with "(Right-Click > Assign)".*

| Block Category | Model Name | Rhythm Settings (Sc A / Sc E) | Lead Settings (Sc B / Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Global Input Gate | Thresh: -65dB | Thresh: -65dB | Tames 60-cycle hum without killing Telecaster sustain. |
| **Pre-FX** | EQ-8 (Pickup Adapter) | **Tele (A):** 200Hz [+2dB], LPF [5kHz]<br>**339 (E):** Output [-3.0dB], HPF [100Hz] *(Assign)* | **Tele (B):** 200Hz [+2dB], LPF [5kHz]<br>**339 (F):** Output [-3.0dB], HPF [100Hz] *(Assign)* | Tailors the input frequency and signal strength to prevent clipping the AC30 model. |
| **Pre-FX 2** | Opto Comp | Comp: 3.5, Ratio: 3:1, Mix: 40% | Comp: 3.5, Ratio: 3:1, Mix: 40% | Replicates the Abbey Road studio optical compression. Glues chords together. |
| **Amp** | UK C30 Top Boost | **Tele (A):** Vol [4.5], Bass [5.0], Tone Cut [6.0]<br>**339 (E):** Vol [3.0], Bass [3.0], Tone Cut [4.0] *(Assign)* | **Tele (B):** Vol [6.5], Tone Cut [5.0]<br>**339 (F):** Vol [5.0], Tone Cut [3.5] *(Assign)* | **No Master Vol.** Pushing "Vol" increases EL84 tube saturation. Tone Cut removes high-end fizz. |
| **Amp Output** | Block Level | **Tele (A):** 0.0dB<br>**339 (E):** +1.5dB *(Assign)* | **Tele (B):** -1.5dB<br>**339 (F):** 0.0dB *(Assign)* | **Headroom Rule:** As we increase Amp Vol for the Lead scenes, we lower Block Level to keep SPL constant for FOH. |
| **Cab** | 212 UK C30 | Mic A: Dyn 57 (Pos: 1.0, Dist: 1.0")<br>Mic B: Rib 121 (Pos: 2.0, Dist: 3.0") | Mix: Mic A [-2dB], Mic B [0dB]<br>*(Same for all scenes)* | 57 provides upper-mid punch. The Ribbon 121 provides the low-end "thump" needed for the QSC CP12 PA speaker. |
| **Post-FX** | Spring Reverb | Mix: 15%, Decay: 1.2s, Tone: 4.0 | Mix: 20%, Decay: 1.5s, Tone: 5.0 *(Assign)* | Simulates the splashy tank reverbs of the 1960s. Increased mix/tone for lead sustain. |

---

### **Troubleshooting & Refinement Tree**
*   **If the ES-339 sounds "Too Fuzzy/Distorted" on Rhythm (Scene E):** Your humbuckers are likely high-output. Do not touch the Amp Volume yet. Go to the **Input Block** and lower the Global Input Gain from 0.0dB to -4.0dB. 
*   **If the Telecaster sounds "Thin/Harsh" through the QSC CP12:** Increase the Amp Block's "Tone Cut" by 1.0 increments. If it still lacks punch, go to the Cab block and move Mic B (Ribbon 121) closer to the center of the cone (Pos: 1.0).

---

### **Session Library (Active Presets)**

**2. Preset Name:** "British Chime Matrix"
**Target:** The Beatles / Early Kinks (1964)
**Guitars:** Fender Telecaster (Sc A-D) / Gibson ES-339 (Sc E-H).
**Physics Goal:** Dual-guitar AC30 chime utilizing scene-assigned input padding, EQ compensations, and EL84 tube tapers.
**Full Configuration:**
*   **Block 1 (Global Gate):** Threshold [-65dB].
*   **Block 2 (EQ-8):** Output [Tele: 0.0dB / 339: -3.0dB], HPF [Tele: 80Hz / 339: 100Hz], Band 2 [Tele: 200Hz +2.0dB / 339: 0.0dB], LPF [Tele: 5000Hz / 339: 6500Hz]. 
*   **Block 3 (Opto Comp):** Comp [3.5], Ratio [3:1], Attack [Fast], Rel [Med], Mix [40%].
*   **Block 4 (Amp - UK C30 Top Boost):** Vol [Tele Rhy: 4.5 / Tele Ld: 6.5 | 339 Rhy: 3.0 / 339 Ld: 5.0], Bass [Tele: 5.0 / 339: 3.0], Treble [6.0], Tone Cut [Tele Rhy: 6.0 / Tele Ld: 5.0 | 339 Rhy: 4.0 / 339 Ld: 3.5], Output Level [Tele Rhy: 0.0dB / Tele Ld: -1.5dB | 339 Rhy: +1.5dB / 339 Ld: 0.0dB].
*   **Block 5 (Cab - 212 UK C30):** Mic A (Dyn 57, Pos 1.0, Dist 1.0"), Mic B (Ribbon 121, Pos 2.0, Dist 3.0"), Mix [A: -2dB, B: 0dB].
*   **Block 6 (Spring Reverb):** Mix [Rhy: 15% / Ld: 20%], Decay [Rhy: 1.2s / Ld: 1.5s], Tone [Rhy: 4.0 / Ld: 5.0].