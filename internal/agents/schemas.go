package agents

import "google.golang.org/genai"

// This file wires up Tier 0 of the pipeline quality proposal: real Gemini structured
// output (so JSON compliance is enforced by the API instead of prompt prose + a brittle
// json.Unmarshal), per-agent temperature (low for numeric/physics agents, higher for the
// research/prose agents), and real Google Search grounding for the two agents (Tone
// Historian, Community Scraper) whose prompts already claim to have search capability.

// agentTemperature returns the per-role sampling temperature. Numeric/physics agents run
// cold (low variance on values that should be consistent given the same inputs); the
// research and final-assembly agents, which synthesize prose alongside data, run warmer.
func agentTemperature(key string) *float32 {
	temps := map[string]float32{
		"1_tone_historian":    0.3,
		"2_sonic_profiler":    0.2,
		"3_community_scraper": 0.4,
		"4_coros_librarian":   0.3,
		"5_cloud_navigator":   0.1, // strict verbatim matching against an injected list
		"6_acoustician":       0.15,
		"7_transducer_tech":   0.2,
		"8_foh_optimizer":     0.15,
		"9_mix_engineer":      0.15,
		"10_control_mapper":   0.2,
		"11_dsp_dispatcher":   0.15,
		"12_architect":        0.6, // bumped from 0.3: baseline's judge-preferred prose specificity (EP-3 preamp tricks, Varitone physics) may have been a temperature effect, not a grounding effect -- testing that hypothesis directly
	}
	t, ok := temps[key]
	if !ok {
		return nil // let the API default apply for anything unrecognized (e.g. RefineChat's "12_architect" key is covered above)
	}
	return &t
}

// agentSearchTool returns the Google Search grounding tool for the two agents whose
// prompts are written as if they already have live search access (Tone Historian,
// Community Scraper). Every other agent reasons over already-gathered context and gets
// no tool.
func agentSearchTool(key string) []*genai.Tool {
	switch key {
	case "1_tone_historian", "3_community_scraper":
		return []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}
	default:
		return nil
	}
}

func strSchema(desc string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, Description: desc}
}

func numSchema(desc string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeNumber, Description: desc}
}

func strArraySchema(desc string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: desc}
}

func objSchema(props map[string]*genai.Schema, required []string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeObject, Properties: props, Required: required}
}

// agentResponseSchema returns the typed OpenAPI-subset schema for agents whose output has
// fixed, known field names. It does not cover Agent 12 (Architect), whose payload has
// guitar-name-keyed dynamic objects that this typed Schema type cannot express — see
// agentResponseJSONSchema below for that one.
//
// Agents 2, 4, and 6 have a v1 and a v2 prompt variant with different field sets (see
// internal/agents/prompts/*_v2.md). Rather than threading the active prompt version
// through the call path, each of those three schemas is a union of both variants' fields,
// with only the fields common to both marked required. The active prompt's own
// instructions drive which fields actually get filled; the schema's job is to bound the
// vocabulary and types of what's allowed, not to pick the version.
func agentResponseSchema(key string) *genai.Schema {
	switch key {
	case "1_tone_historian":
		return objSchema(map[string]*genai.Schema{
			"amp_type":     strSchema("The primary amplifier used to achieve this tone."),
			"pedal_names":  strArraySchema("Pedals/stompboxes used in the original signal chain."),
			"speaker_type": strSchema("The cabinet/speaker used."),
			"mic_type":     strSchema("The microphone used to capture the amp, if known."),
		}, []string{"amp_type", "pedal_names", "speaker_type", "mic_type"})

	case "2_sonic_profiler":
		return objSchema(map[string]*genai.Schema{
			"peak_freq_hz":          numSchema("v1 field: the single dominant peak frequency in Hz."),
			"eq_profile":            {Type: genai.TypeString, Enum: []string{"scooped", "mid_pushed", "flat", "bright", "dark"}, Description: "v2 field: overall EQ shape."},
			"suggested_low_cut_hz":  numSchema("v2 field: high-pass point, typically 80-120Hz, never above 200Hz."),
			"suggested_high_cut_hz": numSchema("v2 field: low-pass point, never below 5000Hz."),
			"saturation_style":      strSchema("Style of overdrive/saturation, e.g. asymmetric clipping, tape saturation, hard clipping."),
			"reverb_type":           strSchema("Characteristic reverb space for this tone."),
			"noise_gate_target_db":  numSchema("Global input gate threshold in dB (-65 for single coils, -55 to -60 for humbuckers/high gain)."),
		}, []string{"saturation_style", "reverb_type", "noise_gate_target_db"})

	case "3_community_scraper":
		return objSchema(map[string]*genai.Schema{
			"suggested_qc_blocks": strArraySchema("At least two Quad Cortex blocks/parameters the community favors for this tone."),
			"community_warnings":  strArraySchema("Known pitfalls or gotchas reported by users dialing in this tone."),
		}, []string{"suggested_qc_blocks", "community_warnings"})

	case "4_coros_librarian":
		return objSchema(map[string]*genai.Schema{
			"verified_blocks":  strArraySchema("v1 field: gear resolved to verified native CorOS block names."),
			"mandatory_blocks": strArraySchema("v2 field: Amplifiers/Cabs the Architect MUST use, verbatim from the Dictionary/Menu/Capture Library."),
			"suggested_blocks": strArraySchema("v2 field: Effects the Architect should use but may swap for a better-fitting native block."),
			"tactical_hints":   strArraySchema("v2 field: positional/usage guidance for downstream agents (no numeric parameters)."),
			"dropped_gear":     strArraySchema("Gear that could not be verified against any of the three allowed sources and was dropped."),
		}, []string{"dropped_gear"})

	case "5_cloud_navigator":
		return objSchema(map[string]*genai.Schema{
			"matches": {
				Type: genai.TypeArray,
				Items: objSchema(map[string]*genai.Schema{
					"dropped_gear_item": strSchema("The dropped_gear entry this match corresponds to."),
					"matched":           {Type: genai.TypeBoolean, Description: "True only if a verbatim match was found in the injected User Capture Library."},
					"capture_name":      strSchema("The exact capture name from the User Capture Library. Omit when matched is false."),
					"source":            strSchema("The capture's source/creator. Omit when matched is false."),
				}, []string{"dropped_gear_item", "matched"}),
			},
		}, []string{"matches"})

	case "6_acoustician":
		return objSchema(map[string]*genai.Schema{
			// v1 (flat) fields:
			"input_gain_db": numSchema("v1 field: input gain compensation in dB (+3.0 for single coil, -3.0 to -6.0 for humbucker)."),
			"amp_vol":       numSchema("v1 field: absolute amp volume/gain knob value."),
			"eq_params": objSchema(map[string]*genai.Schema{
				"bass": numSchema(""), "mid": numSchema(""), "treble": numSchema(""),
			}, nil),
			"chameleon_pre_eq": objSchema(map[string]*genai.Schema{
				"band_1_body": strSchema("Low shelf/peak target around 200Hz, e.g. '200Hz Shelf +2dB'."),
				"band_8_lpf":  strSchema("Low-pass target around 3-5kHz, e.g. 'LPF 4kHz'."),
			}, nil),
			// v2 (per-pickup-type) fields:
			"humbucker": objSchema(map[string]*genai.Schema{
				"input_gain_db":  numSchema(""),
				"amp_vol_offset": numSchema(""),
				"eq_params": objSchema(map[string]*genai.Schema{
					"bass": numSchema(""), "mid": numSchema(""), "treble": numSchema(""),
				}, nil),
				"rationale": strSchema("Why gain/EQ were adjusted this way for humbuckers."),
			}, nil),
			"single_coil": objSchema(map[string]*genai.Schema{
				"input_gain_db":  numSchema(""),
				"amp_vol_offset": numSchema(""),
				"eq_params": objSchema(map[string]*genai.Schema{
					"bass": numSchema(""), "mid": numSchema(""), "treble": numSchema(""),
				}, nil),
				"rationale": strSchema("Why gain/EQ were adjusted this way for single coils."),
			}, nil),
		}, nil) // v1 and v2 share no common field name, so nothing can be marked required here; the active prompt's instructions drive which shape is actually used.

	case "7_transducer_tech":
		return objSchema(map[string]*genai.Schema{
			"mic_1_pos":   strSchema("Primary virtual mic X/Y placement against the speaker cone."),
			"mic_2_pos":   strSchema("Secondary virtual mic X/Y placement against the speaker cone."),
			"blend_ratio": numSchema("Blend ratio between mic 1 and mic 2, 0-100."),
		}, []string{"mic_1_pos", "mic_2_pos", "blend_ratio"})

	case "8_foh_optimizer":
		return objSchema(map[string]*genai.Schema{
			"global_eq_cuts": objSchema(map[string]*genai.Schema{
				"hz_start": numSchema(""), "hz_end": numSchema(""),
			}, []string{"hz_start", "hz_end"}),
			"lane_output_db": numSchema("Lane output level compensation in dB."),
		}, []string{"global_eq_cuts", "lane_output_db"})

	case "9_mix_engineer":
		return objSchema(map[string]*genai.Schema{
			"delay_ms":        numSchema("Delay time in milliseconds."),
			"reverb_decay_s":  numSchema("Reverb decay time in seconds."),
			"mix_percent":     numSchema("Wet/dry mix percentage, 0-100."),
		}, []string{"delay_ms", "reverb_decay_s", "mix_percent"})

	case "10_control_mapper":
		return objSchema(map[string]*genai.Schema{
			"scene_a_params": strArraySchema("Rhythm scene parameter mappings."),
			"scene_b_params": strArraySchema("Lead scene parameter mappings."),
		}, []string{"scene_a_params", "scene_b_params"})

	case "11_dsp_dispatcher":
		return objSchema(map[string]*genai.Schema{
			"grid_layout_map": strArraySchema("Blocks routed per physical Grid Lane."),
		}, []string{"grid_layout_map"})

	default:
		return nil
	}
}

// effectBlockJSONSchema is the raw-JSON-Schema fragment for one EffectBlock, matching
// storage.EffectBlock/BlockParameter exactly (id/type/model/parameters, each parameter
// carrying name/type/value). Used inside both Architect response shapes below.
func effectBlockJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":    map[string]any{"type": "string"},
			"type":  map[string]any{"type": "string"},
			"model": map[string]any{"type": "string", "description": "Must be the exact string verified by the Librarian/Navigator. Never invented."},
			"parameters": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":    map[string]any{"type": "string"},
						"type":    map[string]any{"type": "string", "enum": []string{"slider", "toggle", "dropdown"}},
						"value":   map[string]any{"type": "string", "description": "Exactly one decisive value for Scene A (Rhythm), never a range."},
						"value_b": map[string]any{"type": "string", "description": "Scene B (Lead) value, ONLY if it genuinely differs from Scene A's value. Omit entirely when the two scenes share the same setting -- do not repeat the same value here."},
					},
					"required": []string{"name", "type", "value"},
				},
			},
			"rationale": map[string]any{"type": "string", "description": "One or two sentences on why this specific block/model was chosen. This is the only place this reasoning is captured -- there is no separate prose table."},
		},
		"required": []string{"id", "type", "model", "parameters", "rationale"},
	}
}

// guitarsMapJSONSchema is the raw-JSON-Schema fragment for structured_payload.guitars: a
// map keyed by the (request-specific, unknown-ahead-of-time) guitar name to an array of
// EffectBlocks. additionalProperties is what makes the dynamic keying possible; the typed
// genai.Schema (OpenAPI subset) used for the other 11 agents has no equivalent field, which
// is why the Architect uses ResponseJsonSchema instead of ResponseSchema.
func guitarsMapJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"guitars": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "array", "items": effectBlockJSONSchema()},
			},
		},
		"required": []string{"guitars"},
	}
}

// architectGenerationJSONSchema is Agent 12's schema in normal (RunPipeline) generation
// mode. The model no longer produces final_html_payload directly -- asking it to generate
// both a free-text HTML table AND a structured array in the same response left room for
// the two to silently disagree (confirmed empirically: the model's own HTML repeatedly
// listed blocks its structured_payload dropped). final_html_payload is now rendered in Go
// from the validated structured_payload after the fact, so by construction it can't drift.
func architectGenerationJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"builder_statement":  map[string]any{"type": "string"},
			"structured_payload": guitarsMapJSONSchema(),
			"agent_impact":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Exactly 11 entries, one per preceding agent."},
		},
		"required": []string{"builder_statement", "structured_payload", "agent_impact"},
	}
}

// architectRefineJSONSchema is Agent 12's schema in RefineChat mode. Same reasoning as
// architectGenerationJSONSchema -- final_html_payload is rendered in Go, not generated.
// structured_payload is intentionally not required: the prompt instructs the Architect to
// leave it empty when dsp_matrix_updated is false (a pure Q&A turn with no preset change).
func architectRefineJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"conversational_response": map[string]any{"type": "string"},
			"builder_statement":       map[string]any{"type": "string"},
			"dsp_matrix_updated":      map[string]any{"type": "boolean"},
			"structured_payload": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"guitars": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "array", "items": effectBlockJSONSchema()},
					},
				},
			},
			"agent_impact": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"conversational_response", "dsp_matrix_updated"},
	}
}
