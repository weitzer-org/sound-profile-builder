package presetsvc

import (
	"reflect"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/config"
)

func bp(b bool) *bool { return &b }

// TestBuildConstraints_NewOptions covers guitars, single_amp_mode and
// allow_cloud_captures: nil falls back to the default (config for the two
// booleans), and an explicit request value always wins, in both directions.
func TestBuildConstraints_NewOptions(t *testing.T) {
	for _, cfgVal := range []bool{true, false} {
		for _, tc := range []struct {
			name string
			req  *bool
			want func(cfgVal bool) bool
		}{
			{"nil uses config", nil, func(c bool) bool { return c }},
			{"true wins", bp(true), func(bool) bool { return true }},
			{"false wins", bp(false), func(bool) bool { return false }},
		} {
			cfg := &config.AppConfig{SingleAmpMode: cfgVal, AllowCloudCaptures: cfgVal}
			got := buildConstraints(cfg, GenerateOptions{SingleAmpMode: tc.req, AllowCloudCaptures: tc.req})
			if g := got["single_amp_mode"]; g != tc.want(cfgVal) {
				t.Errorf("single_amp_mode cfg=%v %s: got %v want %v", cfgVal, tc.name, g, tc.want(cfgVal))
			}
			if g := got["allow_cloud_captures"]; g != tc.want(cfgVal) {
				t.Errorf("allow_cloud_captures cfg=%v %s: got %v want %v", cfgVal, tc.name, g, tc.want(cfgVal))
			}
		}
	}
}

// The two flags are independent of each other.
func TestBuildConstraints_NewBoolsIndependent(t *testing.T) {
	got := buildConstraints(&config.AppConfig{SingleAmpMode: true}, GenerateOptions{AllowCloudCaptures: bp(true)})
	if got["single_amp_mode"] != true || got["allow_cloud_captures"] != true {
		t.Errorf("got single_amp_mode=%v allow_cloud_captures=%v", got["single_amp_mode"], got["allow_cloud_captures"])
	}
}

func TestBuildConstraints_Guitars(t *testing.T) {
	cfg := &config.AppConfig{}
	def := []string{"Gibson ES-339 Humbuckers", "Fender Telecaster Single Coil"}

	for name, in := range map[string][]string{"nil": nil, "empty": {}} {
		if got := buildConstraints(cfg, GenerateOptions{Guitars: in})["guitars"]; !reflect.DeepEqual(got, def) {
			t.Errorf("%s guitars: got %v want default %v", name, got, def)
		}
	}

	custom := []string{"Gibson ES-339"}
	got := buildConstraints(cfg, GenerateOptions{Guitars: custom})["guitars"]
	if !reflect.DeepEqual(got, custom) {
		t.Errorf("custom guitars: got %v want %v", got, custom)
	}
	// The constraint is a copy; mutating the caller's slice later must not leak in.
	custom[0] = "changed"
	if got.([]string)[0] != "Gibson ES-339" {
		t.Errorf("constraints alias the caller's slice: %v", got)
	}
	// The default is never shared between runs either.
	a := buildConstraints(cfg, GenerateOptions{})["guitars"].([]string)
	a[0] = "mutated"
	if b := buildConstraints(cfg, GenerateOptions{})["guitars"]; !reflect.DeepEqual(b, def) {
		t.Errorf("default guitars were mutated across calls: %v", b)
	}
}

// Existing options keep their semantics: allow_* is capped by config,
// favor_* is request-over-config.
func TestBuildConstraints_ExistingSemanticsUnchanged(t *testing.T) {
	cfg := &config.AppConfig{AllowFactoryCaptures: true, AllowUserCaptures: false, AllowPaidPlugins: true, FavorCaptures: true}
	got := buildConstraints(cfg, GenerateOptions{AllowUserCaptures: bp(true), AllowPaidPlugins: bp(false), FavorCaptures: nil, FavorCloudCaptures: bp(true)})
	want := map[string]bool{
		"allow_factory_captures": true,
		"allow_user_captures":    false, // config ceiling beats the request
		"allow_paid_plugins":     false,
		"favor_captures":         true, // nil -> config
		"favor_cloud_captures":   true,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v want %v", k, got[k], v)
		}
	}
}
