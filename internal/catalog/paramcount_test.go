package catalog

import "testing"

// The device's param count N = ParamCount (the XML list with clones= resolved, hidden
// params included) + 1 trailing slot. Read back on the owner's unit (CorOS 4.1.0, plan
// 7.2, preset "bb king test 2") for these five blocks. ONLY ONE clone family is covered
// (the cab cloning 12000); the other cloned models are not verified by this test.
func TestParamCountMatchesDeviceReadback(t *testing.T) {
	c := mustEmbedded(t)
	for _, tc := range []struct {
		id          int
		name        string
		effective   int // EFF
		deviceCount int // N read from the device
	}{
		{5011, "Jewel", 5, 6},
		{5, "Green 808", 3, 4},
		{1120, "US Prince", 4, 5},
		{12045, "110 US PRN C10R (M)", 21, 22}, // clones 12000: the only clone family checked
		{8006, "Spring (M)", 5, 6},
	} {
		var m *Model
		for i := range c.Models {
			if c.Models[i].ID == tc.id {
				m = &c.Models[i]
			}
		}
		if m == nil || m.Name != tc.name {
			t.Fatalf("model %d: got %+v, want %q", tc.id, m, tc.name)
		}
		if m.ParamCount != tc.effective || m.DeviceParamCount() != tc.deviceCount {
			t.Errorf("%s: ParamCount %d, DeviceParamCount %d; want %d and %d",
				tc.name, m.ParamCount, m.DeviceParamCount(), tc.effective, tc.deviceCount)
		}
	}
}

// The cab layout the device showed (plan 7.2): per-mic IR-name strings at 1 and 9, HPF 16,
// LPF 17, hidden IR NAME strings at 19 and 20 (not listed in Params, but still indexes),
// and an unexplained extra slot at 21.
func TestCabLayoutAndTrailingSlot(t *testing.T) {
	c := mustEmbedded(t)
	m, _ := c.Lookup("110 US PRN C10R (M)")
	at := map[int]Param{}
	for _, p := range m.Params {
		at[p.Index] = p
	}
	for _, i := range []int{1, 9} {
		if at[i].Name != "ir selector" || at[i].Type != "string" {
			t.Errorf("index %d = %+v, want an IR-name string", i, at[i])
		}
	}
	if at[16].Name != "HPF" || at[17].Name != "LPF" {
		t.Errorf("16/17 = %q/%q, want HPF/LPF", at[16].Name, at[17].Name)
	}
	for _, i := range []int{19, 20} {
		if _, listed := at[i]; listed {
			t.Errorf("index %d is hidden and must not be listed", i)
		}
		if !m.ValidParamIndex(i) {
			t.Errorf("hidden index %d keeps its index and must be valid", i)
		}
	}
	for _, tc := range []struct {
		i  int
		ok bool
	}{{-1, false}, {0, true}, {20, true}, {21, false}, {22, false}, {1 << 30, false}} {
		if got := m.ValidParamIndex(tc.i); got != tc.ok {
			t.Errorf("ValidParamIndex(%d) = %v, want %v", tc.i, got, tc.ok)
		}
	}
}

// Whole-catalog invariants the helper relies on. These do not check any index against a
// device; they check the catalog is internally consistent.
func TestParamCountInvariants(t *testing.T) {
	c := mustEmbedded(t)
	for _, m := range c.Models {
		if m.ParamCount < len(m.Params) {
			t.Errorf("%s (%d): ParamCount %d < %d listed params", m.Name, m.ID, m.ParamCount, len(m.Params))
		}
		prev := -1
		for _, p := range m.Params {
			if p.Index <= prev || !m.ValidParamIndex(p.Index) {
				t.Errorf("%s (%d): param %q index %d (prev %d, count %d)", m.Name, m.ID, p.Name, p.Index, prev, m.ParamCount)
			}
			prev = p.Index
		}
	}
}
