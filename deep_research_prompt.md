# Deep Research Prompt for CorOS Librarian (`coros_map.json`)

**Copy and paste the following prompt directly into Gemini Deep Research to generate your foundational DSP mapping file:**

***

You are an expert audio engineer and archivist specializing in Neural DSP's Quad Cortex (CorOS) ecosystem. My application requires a strict JSON dictionary mapping popular real-world analog guitar pedals and amplifiers to their exact or closest equivalent DSP block names inside the Quad Cortex. 

Please break this research down into two strict phases:

**Phase 1: Cataloging the Quad Cortex API (The "Ground Truth")**
First, perform a deep web search traversing official Neural DSP documentation, Cortex Cloud forums, and community gear wikis to build a comprehensive list of *every single exact DSP block name* currently available in the Quad Cortex ecosystem (e.g., "Green 808", "Chief CE2W", "UK C30 Burst", etc.). Hold this exact list in your context as the definitive dictionary. 

**Phase 2: Mapping Real-World Gear to CorOS**
Using your compiled ground-truth dictionary from Phase 1, trace *every single* Amplifier, Cabinet, and Pedal/Effect block available in CorOS and map it back to its specific real-world analog inspiration (e.g., mapping "Green 808" to "Ibanez Tube Screamer TS9", or "UK C30 Burst" to "Vox AC30"). You must map the entirety of the CorOS ecosystem. 

**JSON Requirements:**
1. Cross-reference these analog names firmly against the official Neural DSP Quad Cortex block list you found in Phase 1. 
2. Output the final result *strictly* as a valid JSON object map named `coros_map.json`. 

**JSON Schema Structure:**
```json
{
  "analog_gear_name": {
    "coros_equivalent": "CorOS Block Name",
    "type": "drive | amp | mod | delay | reverb | eq",
    "confidence_score": 0.0 - 1.0
  }
}
```

**Example Entry:**
```json
{
  "Ibanez Tube Screamer TS9": {
    "coros_equivalent": "Green 808",
    "type": "drive",
    "confidence_score": 1.0
  }
}
```

Ensure the JSON is perfectly valid and ready to be dropped securely into a Google Cloud Storage bucket for an AI pipeline to consume.
