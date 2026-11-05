package catalog

import "testing"

// Spot checks against the committed snapshot. If a CorOS update changes one of these on
// purpose, update the expectation alongside the regenerated qc_catalog.json.
func TestEmbeddedCatalog(t *testing.T) {
	c, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Models) == 0 || c.Source.ModelCount != len(c.Models) {
		t.Fatalf("model_count %d does not match %d models", c.Source.ModelCount, len(c.Models))
	}
	if c.Source.CorOSVersion == "" {
		t.Error("source.coros_version must be recorded")
	}

	amp, ok := c.Lookup("Brit 2203")
	if !ok {
		t.Fatal("Brit 2203 missing")
	}
	if amp.BasedOn != "Based on Marshall® JCM800®" {
		t.Errorf("Brit 2203 based_on = %q", amp.BasedOn)
	}
	gain := findParam(amp, "GAIN")
	if gain == nil || !gain.RangeKnown() || *gain.Min != 0 || *gain.Max != 10 {
		t.Errorf("Brit 2203 GAIN = %+v, want 0..10", gain)
	}

	// Dual-mic cab: each mic has its own POSITION/DISTANCE, told apart by index.
	cab, ok := c.Lookup("412 Brit 60B V30 95 (M)")
	if !ok {
		t.Fatal("412 Brit 60B V30 95 (M) missing")
	}
	var positions []int
	for _, p := range cab.Params {
		if p.Name == "POSITION" {
			positions = append(positions, p.Index)
		}
	}
	if len(positions) != 2 || positions[0] == positions[1] {
		t.Errorf("cab POSITION indexes = %v, want two distinct", positions)
	}
}

func findParam(m *Model, name string) *Param {
	for i := range m.Params {
		if m.Params[i].Name == name {
			return &m.Params[i]
		}
	}
	return nil
}
