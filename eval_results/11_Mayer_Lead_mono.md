**Trigger Verification:** Before we finalize the rig in your Cortex Control app, **are your ES-339 and Telecaster pickups Vintage Output, Medium, or High Output?** (Defaulting to Vintage for the Tele and Medium for the ES-339 for this build). 

To nail the John Mayer Trio era (e.g., *Try!* live album) tone, we need a massive, high-headroom, mid-scooped clean platform that doesn't compress until it's hit hard by a mid-hump overdrive. We will use the **Captain 50** (Neural DSP’s capture of the Morgan SW50, which is heavily based on the Dumble Steel String Singer / Two-Rock architectures Mayer is famous for). 

Per your constraints, this preset uses the **Split-Bank Matrix** to house entirely separate gain-staging, EQ, and gating physics for your Fender Telecaster (Scenes A-D) and Gibson ES-339 (Scenes E-H).

### 4. Organization Standard (Split-Bank Matrix)
*   **Row 1 (Telecaster Single Coil):**
    *   **Scene A (Rhythm):** Huge, scooped clean. Thickened via EQ to emulate Stratocaster body.
    *   **Scene B (Lead):** Green 808 engaged. Piercing highs rolled off.
    *   **Scene C (Dry/Comping):** Reverb/Delay bypassed for tight funk (e.g., *Vultures*).
    *   **Scene D (Ambient/FX):** Myth Drive + Analog Delay for dynamic swells.
*   **Row 2 (Gibson ES-339 Humbucker):**
    *   **Scene E (Rhythm):** HPF engaged to cut mahogany mud. Amp gain reduced.
    *   **Scene F (Lead):** Green 808 engaged, but Tone backed off to compensate for humbucker upper-mid push.
    *   **Scene G (Dry/Comping):** Completely dry, tight noise gate.
    *   **Scene H (Ambient/FX):** Deep plate reverb + subtle delay.

---

### Execution Protocol: Main Signal Chain
*Note: Parameters marked with **[Rhy / Lead]** or **[Tele / ES-339]** require Scene Assignment (Right-Click > Assign to Scene in Cortex Control).*

| Block Category | Model Name | Rhythm Settings (Sc A / E) | Lead Settings (Sc B / F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | **Global Input Gate** | Tele (A): Thresh: -55dB<br>ES339 (E): Thresh: -45dB | Tele (B): Thresh: -50dB<br>ES339 (F): Thresh: -40dB | Humbuckers (ES-339) generate more line noise under gain. Gate is tightened. |
| **Pre-FX (Drive)** | **Green 808** | *Bypassed* | Overdrive: 3.5<br>Tone: [Tele: 5.5 / ES339: 4.5]<br>Level: 8.0 | Simulates Mayer’s TS10. Low drive/high level pushes the Dumble preamp tubes into natural, saggy saturation. ES-339 needs less Tone control. |
| **Amp** | **Captain 50** | Vol: [Tele: 4.5 / ES339: 3.0]<br>Bass: 4.0, Mid: 3.5, Treb: 5.5<br>Bright: OFF, Bass Cut: OFF | Vol: [Tele: 4.5 / ES339: 3.0]<br>Bass: 4.0, Mid: 3.5, Treb: 5.5<br>Bright: OFF, Bass Cut: OFF | Mid-scooped Dumble/Two-Rock platform. Humbucker volume is dropped 1.5 notches to maintain the exact same clean headroom as the Telecaster. |
| **EQ (Adaptation)**| **Parametric-8** | **Tele:** Band 1 (+2.5dB @ 200Hz), Band 8 LPF (4.5kHz)<br>**ES339:** HPF (110Hz), Band 3 (-2dB @ 400Hz) | **Tele:** Band 1 (+2.5dB), LPF (4.8kHz)<br>**ES339:** HPF (110Hz), Band 3 (-2dB) | **The Chameleon Strategy:** Adds necessary Strat-like "wood" to the Telecaster while taming the ice-pick. Cleans up low-mid mud on the ES-339 humbuckers. |
| **Cab** | **212 Captain 50** | Mic A: Dyn 57 (Pos 0.5, Dist 1.0")<br>Mic B: Rib 121 (Pos 1.5, Dist 3.0")<br>Mix: [A: 0dB, B: -2dB] | Mic A: Dyn 57 (Pos 0.5, Dist 1.0")<br>Mic B: Rib 121 (Pos 1.5, Dist 3.0")<br>Mix: [A: 0dB, B: -2dB] | Dynamic 57 captures the hard pick attack (Mayer’s slap technique); Ribbon 121 provides massive low-end warmth without fizz. |
| **Post-FX (Delay)**| **Analog Delay** | *Bypassed* | Mix: 18%<br>Time: 315ms<br>Fdbk: 15% (1-2 repeats) | Simulates the Aqua Puss analog delay used purely to thicken the solo tail. Does not interfere with fast playing. |
| **Post-FX (Verb)** | **Spring** | Mix: 25%<br>Decay: 1.5s<br>Tone: 4.0 | Mix: 30%<br>Decay: 1.8s<br>Tone: 4.5 | Two-Rock amps are famous for lush, dripping spring reverb tanks. Keeps the clean tone glassy and 3-dimensional. |

---

### Troubleshooting & Refinement Tree
If the tone feels **"Too Compressed" or "Fuzzy"** when using the Gibson ES-339:
1.  **Input Pad:** Lower the Global Input Gain (or add a Gain Block at the very front) to **-4.5dB**. Medium/High output humbuckers will cause the *Captain 50* model to clip prematurely, destroying the Dumble headroom effect.
2.  **Tube Sag:** Lower the Amp Bass control to **3.0**. The Morgan/Dumble circuits have notoriously loose low-end; hitting them with a humbucker can cause the simulated power amp to "fart out."
3.  **Output Compensation:** If step 1 and 2 drop your overall loudness, increase the **Lane Output Level** by +2.0dB to push the QSC CP12 PA speaker, rather than pushing the virtual amp.

---

### Session Library (Active Presets)

**2. Preset Name: "Mayer Trio Dumble Rig"**
*   **Target:** John Mayer Trio / Two-Rock / Dumble SSS
*   **Guitars:** Fender Telecaster (Single) & Gibson ES-339 (Humbucker)
*   **Physics Goal:** Massive scooped clean headroom pushed into singing, mid-heavy compression via analog overdrive, compensated for two drastically different guitar impedances.
*   **Full Configuration:**
    *   **Block 1 (Global Input/Gate):** Thresh [Tele: -55dB / ES339: -45dB], Decay [150ms].
    *   **Block 2 (Green 808):** Overdrive [3.5], Tone [Tele: 5.5 / ES339: 4.5], Level [8.0].
    *   **Block 3 (Amp - Captain 50):** Vol [Tele: 4.5 / ES339: 3.0], Bass [4.0], Mid [3.5], Treble [5.5], Bass Cut [Off], Bright [Off].
    *   **Block 4 (EQ-8):** **Tele Scenes:** Band 1 [+2.5dB @ 200Hz Peak], Band 8 LPF [4.5kHz]. **ES339 Scenes:** HPF [110Hz], Band 3 [-2.0dB @ 400Hz], Band 8 LPF [Off].
    *   **Block 5 (Cab - 212 Captain 50):** Mic A (Dyn 57, Pos 0.5, Dist 1.0"), Mic B (Ribbon 121, Pos 1.5, Dist 3.0"), Pan [Center], Mix [A: 0dB, B: -2dB].
    *   **Block 6 (Analog Delay):** Mix [18%], Time [315ms], Fdbk [15%], Tone [5.0].
    *   **Block 7 (Spring Reverb):** Mix [Tele: 25% / ES339: 22%], Decay [1.5s], Tone [4.0].
    *   **Output Routing:** Multi-Out / Output 1/2 to QSC CP12. Level [+0.0dB].