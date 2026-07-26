// Package evalfixtures holds the eval golden-set query data shared across every cmd/eval_*
// tool. It used to be copy-pasted verbatim into cmd/eval_compare, cmd/eval_full_pipeline, and
// cmd/eval_subagent -- three separate main packages can't import each other, so the query map
// and its ordering drifted (cmd/eval_compare was missing the 13th query, "13_Hard_Rock_Blues",
// entirely). Consolidating here so every eval tool runs the same golden set by construction,
// not by convention.
package evalfixtures

// GoldenQueryOrder returns the canonical, stable ordering of the golden-set query names, used
// wherever a tool needs to iterate deterministically (report tables, sequential live runs).
// Returns a fresh slice on every call -- a package-level var here would let one caller's
// in-place mutation (e.g. sorting, appending) silently corrupt every other eval tool importing
// this package for the rest of the process's lifetime (GSR finding on PR #84).
func GoldenQueryOrder() []string {
	return []string{
		"01_SRV_Clean", "02_Chicago_Blues", "03_British_Invasion", "04_Southern_Rock",
		"05_Clapton", "06_Gilmour", "07_Edge", "08_EVH",
		"09_BB_King", "10_Slash", "11_Mayer_Lead", "12_Bonamassa",
		"13_Hard_Rock_Blues",
	}
}

// GoldenQueries returns a fresh map from each golden-set query name to its user-facing prompt
// text, for the same reason GoldenQueryOrder returns a fresh slice.
func GoldenQueries() map[string]string {
	return map[string]string{
		"01_SRV_Clean":        "Clean funk blues tone. Stevie Ray Vaughan style with high headroom. Wants to push it with a TS808.",
		"02_Chicago_Blues":    "Chicago Blues style. Warm Chess Records style overdrive into a small combo amp. Slightly gritty but clean platform.",
		"03_British_Invasion": "Early British Invasion tone. Vox AC30/JTM45 chime and edge of breakup. Punchy mids, sparkle.",
		"04_Southern_Rock":    "Southern Rock slide style. Dual lead humbuckers into a cranked American Tweed amp. Singing sustain.",
		"05_Clapton":          "Vintage Cream-era Clapton tone. Rolled-off Les Paul tone knobs into a cranked Marshall.",
		"06_Gilmour":          "David Gilmour preset using a Hiwatt Custom 100, Ram's Head Big Muff, WEM 4x12, and a massive Plate Reverb.",
		"07_Edge":             "The Edge style chime. 1964 Vox AC30 edge-of-breakup with rhythmic dotted-eighth delays.",
		"08_EVH":              "Van Halen Brown Sound. Hot-rodded 1968 Marshall Plexi, variac sag, plate reverb.",
		"09_BB_King":          "BB King Lucille tone. High-headroom American Twin Reverb clean platform.",
		"10_Slash":            "Guns N' Roses Slash lead. Les Paul neck pickup into a hot JCM800 with standard delay.",
		"11_Mayer_Lead":       "John Mayer Trio Lead. Smooth Two-Rock/Dumble platform, mid-scooped clean with a subtle drive push.",
		"12_Bonamassa":        "Joe Bonamassa modern blues lead features, smooth tube drive into a Dumble style amplifier.",
		"13_Hard_Rock_Blues":  "hard rock blues from the 1960s - 1980s",
	}
}

// DefaultConstraints is the constraints payload every eval tool has passed to RunPipeline
// (single amp mode, factory captures allowed/favored, cloud captures off, the two standard
// eval guitars) -- also previously duplicated verbatim in every full-pipeline eval tool.
func DefaultConstraints() map[string]interface{} {
	return map[string]interface{}{
		"single_amp_mode":        true,
		"allow_cloud_captures":   false,
		"allow_factory_captures": true,
		"favor_captures":         true,
		"guitars":                []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"},
	}
}
