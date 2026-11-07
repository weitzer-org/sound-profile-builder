package devicejobs_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/weitzer-org/sound-builder/internal/devicejobs"
)

// ConfirmEdit's refusals and its happy path, in isolation from the MCP layer (preset-editing E4).
func TestConfirmEdit(t *testing.T) {
	ctx := context.Background()
	text := editPlanText(t)
	sum := devicejobs.EditPlanSHA256(devicejobs.EditFormat, text)
	claims := devicejobs.EditClaims{PresetID: "p-1", Guitar: "From the unit", Mode: devicejobs.EditModeLive, Slot: "4A",
		DeviceName: "Texas Blues", Snapshot: "3f1c2e1a-3b7d-4c55-9a0e-2f4b8d9c1e02", PlanSHA256: sum}
	request := func(c devicejobs.EditClaims) devicejobs.EditRequest {
		return devicejobs.EditRequest{Claims: c, PlanText: text, Source: devicejobs.PushSource{PresetID: "p-1", Guitar: "From the unit"}}
	}

	t.Run("queues once and a repeat names the same job", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(claims)
		id, existing, err := r.bridge.ConfirmEdit(ctx, request(claims), tok)
		if err != nil || existing || id != devicejobs.EditJobIDFromToken(tok) {
			t.Fatalf("%q %v %v", id, existing, err)
		}
		again, existing, err := r.bridge.ConfirmEdit(ctx, request(claims), tok)
		if err != nil || !existing || again != id {
			t.Fatalf("repeat: %q %v %v", again, existing, err)
		}
		if n, _ := r.agent.RunOnce(); n != 1 {
			t.Fatalf("the agent ran %d jobs", n)
		}
	})

	refusals := []struct {
		name   string
		mutate func(r *rig, c *devicejobs.EditClaims, tok *string)
		want   string
	}{
		{"expired token", func(r *rig, c *devicejobs.EditClaims, tok *string) {
			*r.clock = r.clock.Add(devicejobs.ConfirmTTL + time.Second)
		}, "expired"},
		{"claims differ from the token's", func(r *rig, c *devicejobs.EditClaims, tok *string) { c.Guitar = "Other" }, "does not match"},
		{"a garbled token", func(r *rig, c *devicejobs.EditClaims, tok *string) { *tok = "e1.1.zz" }, "not valid"},
		{"a push token", func(r *rig, c *devicejobs.EditClaims, tok *string) {
			*tok, _ = r.bridge.Keys.IssueConfirm(devicejobs.ConfirmClaims{PresetID: "p-1"}, *r.clock)
		}, "not valid"},
		{"overwrite is not available", func(r *rig, c *devicejobs.EditClaims, tok *string) { c.Mode = devicejobs.EditModeOverwrite }, "overwrite is not available"},
		{"a copy without a target", func(r *rig, c *devicejobs.EditClaims, tok *string) { c.Mode = devicejobs.EditModeCopy; c.Name = "Copy" }, "needs target_slot"},
		{"a copy onto the open slot", func(r *rig, c *devicejobs.EditClaims, tok *string) {
			c.Mode, c.TargetSlot, c.Name = devicejobs.EditModeCopy, "4A", "Copy"
		}, "needs target_slot"},
		{"a copy with a name the agent would refuse", func(r *rig, c *devicejobs.EditClaims, tok *string) {
			c.Mode, c.TargetSlot, c.Name = devicejobs.EditModeCopy, "21A", "a/b"
		}, "needs a name"},
		{"bridge offline", func(r *rig, c *devicejobs.EditClaims, tok *string) { *r.clock = r.clock.Add(time.Hour) }, ""},
		{"slot outside the allowed range", func(r *rig, c *devicejobs.EditClaims, tok *string) {}, "outside the slots"},
	}
	for _, c := range refusals {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.agent.AllowedSlots = []string{"1A-32H"}
			if c.name == "slot outside the allowed range" {
				r.agent.AllowedSlots = []string{"20A-25H"}
			}
			r.online(t)
			cl := claims
			tok, _ := r.bridge.IssueEditToken(cl)
			c.mutate(r, &cl, &tok)
			_, _, err := r.bridge.ConfirmEdit(ctx, request(cl), tok)
			if err == nil || !strings.Contains(userMsg(t, err), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
			if n, _ := r.agent.RunOnce(); n != 0 {
				t.Fatalf("a refused confirm queued %d job(s)", n)
			}
		})
	}

	copyClaims := claims
	copyClaims.Mode, copyClaims.TargetSlot, copyClaims.Name = devicejobs.EditModeCopy, "21A", "Texas Blues Edit"

	t.Run("a copy queues with its target slot and name", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(copyClaims)
		if _, _, err := r.bridge.ConfirmEdit(ctx, request(copyClaims), tok); err != nil {
			t.Fatal(err)
		}
		if n, _ := r.agent.RunOnce(); n != 1 {
			t.Fatalf("the agent ran %d jobs", n)
		}
		var found bool
		for _, j := range r.agent.ExecutedJobs() {
			if j.Kind == devicejobs.KindEditPreset {
				found = strings.Contains(string(j.Args), `"mode":"copy"`) && strings.Contains(string(j.Args), `"target_slot":"21A"`) &&
					strings.Contains(string(j.Args), `"name":"Texas Blues Edit"`)
			}
		}
		if !found {
			t.Fatal("the executed edit job does not carry the copy's mode, target_slot and name")
		}
	})

	t.Run("a copy token binds the target and the name", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(copyClaims)
		for name, mut := range map[string]func(c *devicejobs.EditClaims){
			"another target": func(c *devicejobs.EditClaims) { c.TargetSlot = "22A" },
			"another name":   func(c *devicejobs.EditClaims) { c.Name = "Other" },
		} {
			c := copyClaims
			mut(&c)
			if _, _, err := r.bridge.ConfirmEdit(ctx, request(c), tok); err == nil || !strings.Contains(userMsg(t, err), "does not match") {
				t.Errorf("%s: err = %v", name, err)
			}
		}
		if n, _ := r.agent.RunOnce(); n != 0 {
			t.Fatalf("a refused confirm queued %d job(s)", n)
		}
	})

	t.Run("a copy target outside the allowed slots", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-8H", "4A"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(copyClaims)
		_, _, err := r.bridge.ConfirmEdit(ctx, request(copyClaims), tok)
		if err == nil || !strings.Contains(userMsg(t, err), "outside the slots") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("two copies into one target do not queue together", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(copyClaims)
		if _, _, err := r.bridge.ConfirmEdit(ctx, request(copyClaims), tok); err != nil {
			t.Fatal(err)
		}
		other := copyClaims
		other.Slot = "5A"
		tok2, _ := r.bridge.IssueEditToken(other)
		if _, _, err := r.bridge.ConfirmEdit(ctx, request(other), tok2); err == nil || !strings.Contains(userMsg(t, err), "already waiting") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a plan that does not match its hash", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(claims)
		bad := request(claims)
		bad.PlanText = append(append([]byte(nil), text...), ' ')
		_, _, err := r.bridge.ConfirmEdit(ctx, bad, tok)
		if err == nil || !strings.Contains(err.Error(), "does not match its hash") {
			t.Fatalf("a plan that doesn't hash to the claims: err = %v", err)
		}
	})

	t.Run("a second edit of the same slot waits", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(claims)
		if _, _, err := r.bridge.ConfirmEdit(ctx, request(claims), tok); err != nil {
			t.Fatal(err)
		}
		other := claims
		other.PresetID = "p-2"
		tok2, _ := r.bridge.IssueEditToken(other)
		_, _, err := r.bridge.ConfirmEdit(ctx, request(other), tok2)
		if err == nil || !strings.Contains(userMsg(t, err), "already waiting") {
			t.Fatalf("err = %v", err)
		}
	})
}
