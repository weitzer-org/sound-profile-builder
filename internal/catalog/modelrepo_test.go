package catalog

import (
	"fmt"
	"strings"
	"testing"
)

// A synthetic ModelRepo.xml in the device's shape, covering every filtering rule.
const fixtureXML = `<?xml version="1.0" ?><Models>
<Category id="1" name="Guitar Amplifier">
  <Model blob="x" id="1001" name="Test Amp" style="" tm="Based on Some Amp">
    <Padding cpu="1.0"/>
    <Parameter defaultValue="5" displayPos="0" max="10" min="0" name="GAIN" type="float" units=""/>
    <Parameter defaultValue="0" max="1" min="0" name="bypass" steps="2" type="switch" units="" hidden="true"/>
    <Parameter defaultValue="0.0" displayPos="6" max="12" min="-60" min_string="OFF" name="OUTPUT" type="float" units="dB"/>
    <Parameter defaultValue="0" max="1" min="0" name="MODE" stepNames="LP,HP" steps="2" type="switch" units="" hidden="atma"/>
  </Model>
  <Model id="1002" name="Hidden Amp" tm="" hidden="true"/>
  <Model id="1003" name="Plugin Amp" tm="" plugin_id="3" sku="13"/>
</Category>
<Category id="12" name="Cabsim Guitar (M)">
  <Model id="12001" name="212 Test Cab (M)" tm="Based on Some Cab">
    <Parameter defaultValue="0.0" max="MAX_CABSIM_DB" min="MIN_CABSIM_DB" name="LEVEL" type="float" units="dB"/>
    <Parameter defaultValue="4.0" max="10" min="0" name="POSITION" type="float" units=""/>
    <Parameter defaultValue="4.0" max="10" min="0" name="POSITION" type="float" units=""/>
  </Model>
</Category>
<Category id="32" name="Cabsim Guitar (ST)">
  <Model id="32001" name="212 Test Cab (ST)" tm="Based on Some Cab"/>
</Category>
<Category id="8" name="Reverb">
  <Model id="8015" name="Default Reverb" tm="" hidden="true">
    <Parameter defaultValue="50" max="100" min="0" name="DECAY" type="float" units="%"/>
    <Parameter defaultValue="1" max="1" min="0" name="TAIL" type="switch" steps="2" units=""/>
  </Model>
  <Model clones=" 8015 " id="8016" name="Cloned Reverb" tm="">
    <Parameter defaultValue="70" max="100" min="0" name="DECAY" replaces="0" type="float" units="%"/>
    <Parameter defaultValue="500" max="20000" min="20" name="LOW CUT" replaces=" " type="float" units="Hz"/>
  </Model>
  <Model id="8020" name="Renamed Param Reverb" tm="">
    <Parameter defaultValue="5" max="10" min="0" name="DEPTH" replaces="INTENSITY" type="float" units=""/>
  </Model>
</Category>
<Category id="22" name="Internal Routing">
  <Model id="22000" name="Router" tm=""/>
</Category>
<Category id="25" name="Tempo control">
  <Model id="25000" name="Tempo" tm="" internal="true"/>
</Category>
</Models>`

func parseFixture(t *testing.T) *Catalog {
	t.Helper()
	models, err := ParseModelRepo(strings.NewReader(fixtureXML))
	if err != nil {
		t.Fatal(err)
	}
	return New(models, Source{CorOSVersion: "test"}, "")
}

func TestParseModelRepoFiltering(t *testing.T) {
	c := parseFixture(t)
	var names []string
	for _, m := range c.Models {
		if m.Category == "Reverb" {
			names = append(names, fmt.Sprintf("%d:%s", m.ID, m.Name))
			continue
		}
		names = append(names, m.Name)
	}
	got := strings.Join(names, "|")
	want := "Test Amp|Plugin Amp|8016:Cloned Reverb|8020:Renamed Param Reverb|212 Test Cab (M)|212 Test Cab (ST)"
	if got != want {
		t.Fatalf("models = %q, want %q (hidden, internal and plumbing categories dropped; sorted by id)", got, want)
	}
	if c.Source.ModelCount != 6 {
		t.Errorf("ModelCount = %d, want 6", c.Source.ModelCount)
	}
}

func TestParseModelRepoParams(t *testing.T) {
	c := parseFixture(t)
	amp, ok := c.Lookup("Test Amp")
	if !ok {
		t.Fatal("Test Amp missing")
	}
	if amp.BasedOn != "Based on Some Amp" || amp.Category != "Guitar Amplifier" || amp.ID != 1001 {
		t.Errorf("amp fields wrong: %+v", amp)
	}
	if len(amp.Params) != 3 {
		t.Fatalf("got %d params, want 3 (hidden=true dropped, hidden=atma kept): %+v", len(amp.Params), amp.Params)
	}
	out := amp.Params[1]
	if out.Name != "OUTPUT" || out.Index != 2 {
		t.Errorf("OUTPUT should keep its device index 2 despite the hidden param before it, got %+v", out)
	}
	if !out.RangeKnown() || *out.Min != -60 || *out.Max != 12 || out.Units != "dB" || out.MinLabel != "OFF" {
		t.Errorf("OUTPUT range/labels wrong: %+v", out)
	}
	if out.DisplayPos == nil || *out.DisplayPos != 6 || out.Default == nil || *out.Default != 0 {
		t.Errorf("OUTPUT display/default wrong: %+v", out)
	}
	mode := amp.Params[2]
	if mode.Steps != 2 || strings.Join(mode.StepNames, ",") != "LP,HP" {
		t.Errorf("MODE steps wrong: %+v", mode)
	}

	if p, _ := c.Lookup("Plugin Amp"); p.PluginID != "3" {
		t.Errorf("plugin id not kept: %+v", p)
	}
}

func TestSymbolicBoundsAreUnknown(t *testing.T) {
	c := parseFixture(t)
	cab, _ := c.Lookup("212 Test Cab (M)")
	lvl := cab.Params[0]
	if lvl.RangeKnown() || lvl.Min != nil || lvl.Max != nil {
		t.Errorf("symbolic bounds must not become numbers: %+v", lvl)
	}
	if lvl.MinSymbol != "MIN_CABSIM_DB" || lvl.MaxSymbol != "MAX_CABSIM_DB" {
		t.Errorf("symbols not kept: %+v", lvl)
	}
	if cab.Params[1].Index == cab.Params[2].Index {
		t.Error("repeated parameter names must keep distinct indexes")
	}
}

func TestFamilies(t *testing.T) {
	c := parseFixture(t)
	if got := len(c.Family("212 Test Cab")); got != 2 {
		t.Errorf("family size = %d, want 2 (M and ST)", got)
	}
	for _, name := range []string{"212 Test Cab", "212 Test Cab (M)", "212 Test Cab (ST)", "Test Amp"} {
		if !c.IsKnownName(name) {
			t.Errorf("IsKnownName(%q) = false", name)
		}
	}
	for _, name := range []string{"Hidden Amp", "Router", "212 Test", "test amp"} {
		if c.IsKnownName(name) {
			t.Errorf("IsKnownName(%q) = true", name)
		}
	}
	if FamilyName("Dream Chorus (ST)") != "Dream Chorus" || FamilyName("Brit 2203") != "Brit 2203" {
		t.Error("FamilyName wrong")
	}
}

func TestClonedModelInheritsBaseParams(t *testing.T) {
	c := parseFixture(t)
	m, ok := c.Lookup("Cloned Reverb")
	if !ok {
		t.Fatal("Cloned Reverb missing")
	}
	var got []string
	for _, p := range m.Params {
		got = append(got, fmt.Sprintf("%d:%s=%v", p.Index, p.Name, *p.Default))
	}
	// DECAY overridden in slot 0, TAIL inherited in slot 1, LOW CUT appended after the base.
	// The fixture's clones attribute carries stray spaces, and LOW CUT has a whitespace-only
	// replaces (= no replaces); neither may break parsing.
	if want := "0:DECAY=70|1:TAIL=1|2:LOW CUT=500"; strings.Join(got, "|") != want {
		t.Errorf("params = %q, want %q", strings.Join(got, "|"), want)
	}

	// On a non-cloned model, replaces is a rename note, not an index.
	r, _ := c.Lookup("Renamed Param Reverb")
	if len(r.Params) != 1 || r.Params[0].Name != "DEPTH" || r.Params[0].Index != 0 {
		t.Errorf("rename-note param mishandled: %+v", r.Params)
	}
}

func TestParseModelRepoRejectsBadClones(t *testing.T) {
	for name, xml := range map[string]string{
		"duplicated template": `<Models><Category name="A"><Model id="1" name="X" hidden="true"><Parameter name="P"/></Model><Model id="1" name="X2" hidden="true"><Parameter name="Q"/></Model><Model id="2" name="Y" clones="1"/></Category></Models>`,
		"no models":           `<Error><Message>not a catalog</Message></Error>`,
		"unknown base":        `<Models><Category name="A"><Model id="2" name="Y" clones="9"/></Category></Models>`,
		"replaces past":       `<Models><Category name="A"><Model id="1" name="X"><Parameter name="P"/></Model><Model id="2" name="Y" clones="1"><Parameter name="Q" replaces="5"/></Model></Category></Models>`,
	} {
		if _, err := ParseModelRepo(strings.NewReader(xml)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestCloneIDsCompareNumerically(t *testing.T) {
	xml := `<Models><Category name="A"><Model id="001" name="Base" hidden="true"><Parameter name="P" max="1" min="0"/></Model><Model id="2" name="Clone" clones="1"/></Category></Models>`
	models, err := ParseModelRepo(strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || len(models[0].Params) != 1 || models[0].Params[0].Name != "P" {
		t.Errorf("clone of id 001 via clones=\"1\" should inherit P, got %+v", models)
	}
}

func TestParseModelRepoRejectsDuplicateIDs(t *testing.T) {
	xml := `<Models><Category name="A"><Model id="1" name="X"/><Model id="1" name="Y"/></Category></Models>`
	if _, err := ParseModelRepo(strings.NewReader(xml)); err == nil {
		t.Fatal("expected a duplicate-id error")
	}
}

func TestParseSkew(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float64 // 0 = nil
		err  bool
	}{
		{"", 0, false}, {"  ", 0, false},
		{"LIN_SKEW", 1.0, false}, {"LOG_SKEW", 0.3, false},
		{"3.8018", 3.8018, false}, {" 0.4", 0.4, false}, {"1", 1, false},
		{"MID_SKEW", 0, true}, {"lin_skew", 0, true}, {"0", 0, true}, {"-1", 0, true},
		{"NaN", 0, true}, {"Inf", 0, true}, {"0..7", 0, true},
	} {
		got, err := parseSkew(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("parseSkew(%q) err = %v, want error=%v", tc.in, err, tc.err)
			continue
		}
		if err == nil && ((got == nil) != (tc.want == 0) || (got != nil && *got != tc.want)) {
			t.Errorf("parseSkew(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseModelRepoSkewAndParamCount(t *testing.T) {
	c := parseFixtureWith(t, `<Category id="1" name="Guitar Amplifier">
  <Model id="1" name="A" tm="">
    <Parameter max="12" min="-60" name="OUT" skew="3.8018" type="float" units="dB"/>
    <Parameter max="10" min="0" name="LIN" skew="LIN_SKEW" type="float" units=""/>
    <Parameter max="500" min="20" name="LOG" skew="LOG_SKEW" type="float" units="Hz"/>
    <Parameter max="10" min="0" name="NONE" type="float" units=""/>
    <Parameter max="1" min="0" name="gone" type="switch" steps="2" hidden="true" units=""/>
  </Model></Category>`)
	m := c.Models[0]
	if m.ParamCount != 5 || len(m.Params) != 4 {
		t.Errorf("ParamCount %d, listed %d; want 5 (hidden counted) and 4", m.ParamCount, len(m.Params))
	}
	want := []*float64{f(3.8018), f(1.0), f(0.3), nil}
	for i, p := range m.Params {
		if (p.Skew == nil) != (want[i] == nil) || (p.Skew != nil && *p.Skew != *want[i]) {
			t.Errorf("%s skew = %v, want %v", p.Name, p.Skew, want[i])
		}
	}
	wantSym := []string{"", "LIN_SKEW", "LOG_SKEW", ""}
	for i, p := range m.Params {
		if p.SkewSymbol != wantSym[i] {
			t.Errorf("%s skew_symbol = %q, want %q", p.Name, p.SkewSymbol, wantSym[i])
		}
	}
	// A cloned model's count is the base list plus appended params, and it records its base.
	cl := parseFixture(t)
	r, _ := cl.Lookup("Cloned Reverb")
	if r.ParamCount != 3 {
		t.Errorf("Cloned Reverb ParamCount = %d, want 3 (base 2 + 1 appended)", r.ParamCount)
	}
	if r.Clones != 8015 {
		t.Errorf("Cloned Reverb Clones = %d, want 8015", r.Clones)
	}
	if a, _ := cl.Lookup("Test Amp"); a.Clones != 0 {
		t.Errorf("Test Amp Clones = %d, want 0", a.Clones)
	}
}

func TestParseModelRepoRejectsUnknownSkew(t *testing.T) {
	doc := `<?xml version="1.0" ?><Models><Category id="1" name="Guitar Amplifier">
  <Model id="1" name="A" tm=""><Parameter max="10" min="0" name="X" skew="SQUARE_SKEW" type="float"/></Model>
</Category></Models>`
	_, err := ParseModelRepo(strings.NewReader(doc))
	if err == nil || !strings.Contains(err.Error(), "SQUARE_SKEW") {
		t.Fatalf("err = %v, want an import error naming the skew", err)
	}
}

func parseFixtureWith(t *testing.T, categories string) *Catalog {
	t.Helper()
	models, err := ParseModelRepo(strings.NewReader(`<?xml version="1.0" ?><Models>` + categories + `</Models>`))
	if err != nil {
		t.Fatal(err)
	}
	return New(models, Source{CorOSVersion: "test"}, "")
}

// TestStringDefaultStaysString: a string parameter's default is kept as a string even when
// it looks numeric, and a float's default is never a string.
func TestStringDefaultStaysString(t *testing.T) {
	c := parseFixtureWith(t, `<Category id="1" name="Cabsim Guitar (M)">
  <Model id="1" name="A" tm="">
    <Parameter defaultValue="57" name="ir selector" type="string" max="999" min="0"/>
    <Parameter defaultValue="ME_A_Dynamic 57" name="ir selector" type="string" max="999" min="0"/>
    <Parameter defaultValue="5" max="10" min="0" name="GAIN" type="float"/>
  </Model></Category>`)
	ps := c.Models[0].Params
	if ps[0].DefaultString != "57" || ps[0].Default != nil {
		t.Errorf("numeric-looking string default: %+v", ps[0])
	}
	if ps[1].DefaultString != "ME_A_Dynamic 57" || ps[1].Default != nil {
		t.Errorf("string default: %+v", ps[1])
	}
	if ps[2].DefaultString != "" || ps[2].Default == nil || *ps[2].Default != 5 {
		t.Errorf("float default: %+v", ps[2])
	}
}
