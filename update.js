const fs = require('fs');

const userMatrixStr = `{"Fender Telecaster Single Coil":"\\u003ctable class='grid-matrix' style='width: 100%; border-collapse: collapse;'\\u003e \\u003cthead\\u003e\\u003ctr\\u003e\\u003cth style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'\\u003eEffect Type \\u0026 Name\\u003c/th\\u003e\\u003cth style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'\\u003eScene A (Rhythm)\\u003c/th\\u003e\\u003cth style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'\\u003eScene B (Lead)\\u003c/th\\u003e\\u003c/tr\\u003e\\u003c/thead\\u003e \\u003ctbody\\u003e \\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eInput: Global Pre-EQ\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eGain: -3.0dB\\u003cbr/\\u003ePre-EQ: 200Hz Shelf -2.5dB\\u003cbr/\\u003eLPF: 4.5kHz\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eGain: -3.0dB\\u003cbr/\\u003ePre-EQ: 200Hz Shelf -2.5dB\\u003cbr/\\u003eLPF: 4.5kHz\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eGate: Noise Gate\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eThreshold: -65.0dB\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eThreshold: -65.0dB\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eAmp: US DLX 58\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eDrive: 5.5\\u003cbr/\\u003eVolume: 4.9\\u003cbr/\\u003eBass: 3.1\\u003cbr/\\u003eMid: 6.8\\u003cbr/\\u003eTreble: 5.2\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eDrive: 8.5\\u003cbr/\\u003eVolume: 6.5\\u003cbr/\\u003eBass: 3.1\\u003cbr/\\u003eMid: 6.8\\u003cbr/\\u003eTreble: 5.2\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eCab: 1x12 US DLX\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMic 1: Cap Edge, On-Axis, 1.0in\\u003cbr/\\u003eMic 2: Mid-Cone, 45-deg Off-Axis, 2.5in\\u003cbr/\\u003eBlend: 65%\\u003cbr/\\u003eHPF: 80Hz\\u003cbr/\\u003eLPF: 8000Hz\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMic 1: Cap Edge, On-Axis, 1.0in\\u003cbr/\\u003eMic 2: Mid-Cone, 45-deg Off-Axis, 2.5in\\u003cbr/\\u003eBlend: 65%\\u003cbr/\\u003eHPF: 80Hz\\u003cbr/\\u003eLPF: 8000Hz\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eDelay: Standard Delay\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eTime: 125ms\\u003cbr/\\u003eMix: 15%\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eTime: 125ms\\u003cbr/\\u003eMix: 15%\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eReverb: Studio Plate 70\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMix: 15%\\u003cbr/\\u003eDecay: 1.5s\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMix: 20%\\u003cbr/\\u003eDecay: 2.0s\\u003c/td\\u003e\\u003c/tr\\u003e \\u003c/tbody\\u003e \\u003c/table\\u003e","Gibson ES-339 Humbuckers":"\\u003ctable class='grid-matrix' style='width: 100%; border-collapse: collapse;'\\u003e \\u003cthead\\u003e\\u003ctr\\u003e\\u003cth style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'\\u003eEffect Type \\u0026 Name\\u003c/th\\u003e\\u003cth style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'\\u003eScene A (Rhythm)\\u003c/th\\u003e\\u003cth style='border-bottom: 2px solid #52525b; padding: 12px; text-align: left;'\\u003eScene B (Lead)\\u003c/th\\u003e\\u003c/tr\\u003e\\u003c/thead\\u003e \\u003ctbody\\u003e \\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eInput: Global Pre-EQ\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eGain: -3.0dB\\u003cbr/\\u003ePre-EQ: 200Hz Shelf -2.5dB\\u003cbr/\\u003eLPF: 4.5kHz\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eGain: -3.0dB\\u003cbr/\\u003ePre-EQ: 200Hz Shelf -2.5dB\\u003cbr/\\u003eLPF: 4.5kHz\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eGate: Noise Gate\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eThreshold: -65.0dB\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eThreshold: -65.0dB\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eAmp: US DLX 58\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eDrive: 5.5\\u003cbr/\\u003eVolume: 4.9\\u003cbr/\\u003eBass: 3.1\\u003cbr/\\u003eMid: 6.8\\u003cbr/\\u003eTreble: 5.2\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eDrive: 8.5\\u003cbr/\\u003eVolume: 6.5\\u003cbr/\\u003eBass: 3.1\\u003cbr/\\u003eMid: 6.8\\u003cbr/\\u003eTreble: 5.2\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eCab: 1x12 US DLX\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMic 1: Cap Edge, On-Axis, 1.0in\\u003cbr/\\u003eMic 2: Mid-Cone, 45-deg Off-Axis, 2.5in\\u003cbr/\\u003eBlend: 65%\\u003cbr/\\u003eHPF: 80Hz\\u003cbr/\\u003eLPF: 8000Hz\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMic 1: Cap Edge, On-Axis, 1.0in\\u003cbr/\\u003eMic 2: Mid-Cone, 45-deg Off-Axis, 2.5in\\u003cbr/\\u003eBlend: 65%\\u003cbr/\\u003eHPF: 80Hz\\u003cbr/\\u003eLPF: 8000Hz\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eDelay: Standard Delay\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eTime: 125ms\\u003cbr/\\u003eMix: 15%\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eTime: 125ms\\u003cbr/\\u003eMix: 15%\\u003c/td\\u003e\\u003c/tr\\u003e\\u003ctr\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eReverb: Studio Plate 70\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMix: 15%\\u003cbr/\\u003eDecay: 1.5s\\u003c/td\\u003e\\u003ctd style='padding: 12px; border-bottom: 1px solid #3f3f46;'\\u003eMix: 20%\\u003cbr/\\u003eDecay: 2.0s\\u003c/td\\u003e\\u003c/tr\\u003e \\u003c/tbody\\u003e \\u003c/table\\u003e"}`;
const finalPayloadMap = JSON.parse(userMatrixStr);

const architectGen = {
  final_html_payload: finalPayloadMap,
  agent_impact: [
    "<strong>Agent 1 (Tone Historian):</strong> Identified US DLX 58 as the target amp.",
    "<strong>Agent 2 (Sonic Profiler):</strong> Selected 1x12 Cab with dual mics natively adjusted.",
    "<strong>Agent 3 (Amplifier AI):</strong> Added multi-guitar compatibility tuning map for target axes."
  ]
};

const genData = {
  candidates: [{
    content: {
      parts: [{ text: JSON.stringify(architectGen) }],
      role: "model"
    }
  }],
  usageMetadata: { promptTokenCount: 220, candidatesTokenCount: 620, totalTokenCount: 840 }
};

const architectRef = {
  conversational_response: "I've rolled off the upper frequencies using a shelf EQ per your request for both guitars.",
  dsp_matrix_updated: true,
  final_html_payload: finalPayloadMap,
  agent_impact: [
    "<strong>Evaluator:</strong> Adjusted EQ parameters consistently inside hardware tabs."
  ]
};

const refData = {
  candidates: [{
    content: {
      parts: [{ text: JSON.stringify(architectRef) }],
      role: "model"
    }
  }],
  usageMetadata: { promptTokenCount: 1040, candidatesTokenCount: 710, totalTokenCount: 1750 }
};

fs.writeFileSync('testdata/e2e_mocks/architect_generate.json', JSON.stringify(genData, null, 2));
fs.writeFileSync('testdata/e2e_mocks/architect_refine.json', JSON.stringify(refData, null, 2));

console.log("Mock data updated with map[string]string payloads.");
