To nail Slash’s iconic soaring, sustaining neck-pickup lead tone (like the "Sweet Child O' Mine" or "November Rain" solos) through a QSC CP12 PA speaker, we have to respect the physics of the Les Paul neck pickup. It pushes a massive amount of low-midrange frequencies, which easily turns a high-gain JCM800 into a muddy, undefined mess if not gain-staged correctly.

Per your system requirements, I have deployed the **Split-Bank Matrix** to provide strictly separated, physics-accurate configurations for both a **Humbucker (Les Paul/ES-339)** and a **Single Coil (Telecaster)**.

***

### GUITAR 1: Humbucker Profile (Les Paul / ES-339)
**Target:** Slash Neck-Pickup Lead.
**Input Block Strategy:** Set Global Input Gain to **-3.0dB**. Humbuckers hit the digital converters hard; dropping the input prevents digital clipping and keeps the neck pickup articulate before it hits the gain stage.
**Bank Routing:** Use **Row 2 (Scenes E-H)**.

#### Table A: Main Signal Chain (Humbucker / Les Paul)
| Block Category | Model Name | Rhythm Settings (Sc E) | Lead Settings (Sc F) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 50%, Thresh: -50dB | Noise Red: 35%, Thresh: -60dB | Less reduction on Lead preserves neck-pickup sustain. |
| **Pre-FX (EQ)** | Parametric-8 | HPF: 120Hz, Band 3 (300Hz): -3dB | HPF: 120Hz, Band 6 (2kHz): +2dB | Cuts Les Paul neck "mud" (300Hz). Lead adds a 2kHz mid-push for pick articulation. |
| **Amp** | Brit 800 | Preamp: 6.5, Master: 6.0, Mid: 6.5 | Preamp: 8.5, Master: 6.0, Mid: 8.0 | *JCM800 2203 Circuit.* Pushing Mids to 8.0 forces the neck pickup to cut through the mix. Master at 6.0 simulates power-tube saturation. |
| **Cab** | 412 Brit V30 | Mic A (Dyn 57): Pos 0.5, Dist 1.0" | Mic A (Dyn 57): Pos 0.0, Dist 1.0" | Moving Mic A to Cap Center (0.0) on Lead maximizes treble bite to balance the dark neck pickup. Mic B (Ribbon 121) at Pos 1.5, Mix -6dB for warmth. |
| **Post-FX** | Analog Delay | Mix: 10%, Time: 450ms, Fdbk: 15% | Mix: 25%, Time: 450ms, Fdbk: 30% | 450ms is standard stadium rock delay. Analog model decays darkly so it won't clash with the solo. |
| **Post-FX** | Hall Reverb | Mix: 15%, Decay: 1.5s | Mix: 20%, Decay: 2.2s | Simulates the massive arena/studio spaces typical of 80s/90s GNR records. |

*(Note for Guitar 1: Roll your Les Paul neck Tone knob back to about 7 or 8 to smooth out the highest, fizzy frequencies while letting the Amp block provide the upper-midrange cut).*

***

### GUITAR 2: Single Coil Profile (Fender Telecaster)
**Target:** Adapting a high-gain Slash tone for bright, lower-output pickups.
**Input Block Strategy:** Set Global Input Gain to **+3.0dB**. Telecasters lack the voltage to push a JCM800 into heavy saturation organically.
**Bank Routing:** Use **Row 1 (Scenes A-D)**.

#### Table A: Main Signal Chain (Single Coil / Telecaster)
| Block Category | Model Name | Rhythm Settings (Sc A) | Lead Settings (Sc B) | Physics/Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Input/Gate** | Adaptive Gate | Noise Red: 60%, Thresh: -45dB | Noise Red: 45%, Thresh: -55dB | Single coils have 60-cycle hum. Higher Noise Reduction (%) is required to keep the JCM800 quiet. |
| **Pre-FX (EQ)** | Parametric-8 | Band 2 (200Hz): +3.5dB, LPF: 5kHz | Band 2 (200Hz): +4.0dB, LPF: 4.5kHz | **Crucial:** Simulates humbucker body. Boosts low-mid weight (200Hz) and violently cuts ice-pick highs (LPF) before hitting the amp. |
| **Amp** | Brit 800 | Preamp: 8.0, Master: 6.5, Treb: 4.5 | Preamp: 10.0, Master: 6.5, Treb: 4.0 | Tele bridge/neck needs max Preamp gain to sustain, but Treble/Presence must be dropped to prevent harshness. |
| **Cab** | 412 Brit V30 | Mic A (Dyn 57): Pos 1.5, Dist 1.0" | Mic A (Dyn 57): Pos 2.0, Dist 1.5" | Moving the Dyn 57 toward the cone edge (Pos 2.0) naturally warms up the bright Telecaster without using aggressive post-EQ. |
| **Post-FX** | Analog Delay | Mix: 10%, Time: 450ms, Fdbk: 15% | Mix: 25%, Time: 450ms, Fdbk: 30% | Identical spatial mapping to Guitar 1. |
| **Post-FX** | Hall Reverb | Mix: 15%, Decay: 1.5s | Mix: 20%, Decay: 2.2s | Identical spatial mapping to Guitar 1. |

***

### Troubleshooting & Refinement Tree
If the tone isn't translating correctly through your QSC CP12, follow this strict order of operations:
1. **Too Distorted/Fuzzy (Humbucker):** Lower the Global Input Block Gain to **-6.0dB**. High-output modern pickups will push the Brit 800 model into harsh, unmusical clipping. 
2. **Too Much "Boom/Mud" on the Neck Pickup:** Do *not* lower the Amp Bass knob past 3.0 (it thins the tone). Instead, increase the HPF on the Parametric-8 block from 120Hz to **150Hz**. 
3. **Loss of Volume:** If you lower the Amp Gain to clean up the tone, you *must* compensate by raising the Lane Output Level by +2.0dB.
4. **Harsh Top End (QSC CP12 Specific):** PA speakers have tweeters that project high frequencies aggressively. If it sounds brittle, move the Cab Block's Mic A from Position `0.0` to `1.5`.

***

### Session Registry Protocol
**Session Library (Active Presets)**

2. Preset Name: "Appetite Neck Lead - Multi-Scale"
Target: Slash / Guns N' Roses (JCM800 Lead).
Guitar 1: Les Paul/ES-339 (Humbuckers) w/ Pick.
Guitar 2: Telecaster (Single Coils) w/ Pick.
Physics Goal: Articulate neck-pickup saturation (LP) vs. thickened, tamed high-gain (Tele) through FRFR.
Full Configuration:
Block 1 (Adaptive Gate): Noise Red [Sc A: 60% / Sc B: 45% / Sc E: 50% / Sc F: 35%], Thresh [-45dB / -55dB / -50dB / -60dB], Decay [150ms].
Block 2 (EQ-8): Sc A/B (Tele) -> HPF [80Hz], Band 2 [+4.0dB @ 200Hz], LPF [4.5kHz]. Sc E/F (LP) -> HPF [120Hz], Band 3 [-3.0dB @ 300Hz], Band 6 [+2.0dB @ 2kHz].
Block 3 (Amp - Brit 800): Preamp [Sc A: 8.0 / Sc B: 10.0 / Sc E: 6.5 / Sc F: 8.5], Master [6.0/6.5], Bass [4.0], Mid [Sc A/B: 5.5 / Sc E/F: 8.0], Treble [Sc A/B: 4.0 / Sc E/F: 7.0], Presence [5.0].
Block 4 (Cab - 412 Brit V30): Mic A (Dyn 57) Pos [Sc A/B: 2.0 / Sc E/F: 0.0], Dist 1.0". Mic B (Ribbon 121) Pos 1.5, Dist 3.0". Mix [A: 0dB, B: -6dB].
Block 5 (Analog Delay): Mix [Rhy: 10% / Lead: 25%], Time [450ms], Fdbk [Rhy: 15% / Lead: 30%].
Block 6 (Hall Reverb): Mix [Rhy: 15% / Lead: 20%], Decay [Rhy: 1.5s / Lead: 2.2s], HP [150Hz], LP [4000Hz].