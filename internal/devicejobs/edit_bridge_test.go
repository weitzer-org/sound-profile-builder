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
		{"copy is not available", func(r *rig, c *devicejobs.EditClaims, tok *string) { c.Mode = devicejobs.EditModeCopy }, "Only live"},
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

	t.Run("a plan that does not match its hash", func(t *testing.T) {
		r := newRig(t)
		r.agent.AllowedSlots = []string{"1A-32H"}
		r.online(t)
		tok, _ := r.bridge.IssueEditToken(claims)
		bad := request(claims)
		bad.PlanText = append(append([]byte(nil), text...), ' ')
		if _, _, err := r.bridge.ConfirmEdit(ctx, bad, tok); err == nil {
			t.Fatal("a plan that doesn't hash to the claims was queued")
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
