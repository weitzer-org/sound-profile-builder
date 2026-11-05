package devicejobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// Queue limits (best effort: storage.Client has no conditional write).
const (
	MaxPendingJobs = 5
	DefaultPollGap = 2 * time.Second
)

// UserError is an error whose message is written for the user or the model.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

func userErr(format string, args ...any) error { return &UserError{Msg: fmt.Sprintf(format, args...)} }

// Bridge is the QC-2 side of the device bridge: it plans and queues jobs and
// reports their state. It never talks to the device.
type Bridge struct {
	Store    *Store
	Exporter Exporter
	Keys     *Keyring
	// Now and PollGap are injectable for tests.
	Now     func() time.Time
	PollGap time.Duration
}

// NewBridge wires a Bridge over a storage client.
func NewBridge(client storage.Client, cfg *Config, exp Exporter) *Bridge {
	if exp == nil {
		exp = UnavailableExporter{}
	}
	return &Bridge{Store: NewStore(client, cfg), Exporter: exp, Keys: cfg.Keys, Now: time.Now, PollGap: DefaultPollGap}
}

func (b *Bridge) now() time.Time {
	if b.Now == nil {
		return time.Now().UTC()
	}
	return b.Now().UTC()
}

// Status reads the heartbeat and counts pending jobs.
func (b *Bridge) Status(ctx context.Context) (HeartbeatView, int, error) {
	hv, err := b.Store.Heartbeat(ctx, b.now())
	if err != nil {
		return hv, 0, err
	}
	pend, err := b.Store.PendingJobs(ctx, b.now())
	if err != nil {
		return hv, 0, err
	}
	return hv, len(pend), nil
}

// PushRequest is send_preset_to_device's arguments, validated by the caller.
type PushRequest struct {
	PresetID string
	Guitar   string
	Slot     string // normalized
	Name     string
}

// PushPlan is a dry run's answer.
type PushPlan struct {
	Export     *Export
	PlanSHA256 string
	// Token is empty when the plan has unresolved items.
	Token     string
	TokenExp  time.Time
	Bridge    HeartbeatView
	SlotNote  string // non-empty when the bridge says the slot is outside its allowed range
	BlockRows int
}

func (b *Bridge) export(ctx context.Context, sp storage.StructuredPreset, guitar string) (*Export, string, error) {
	e, err := b.Exporter.Export(ctx, sp, guitar)
	if errors.Is(err, ErrExportUnavailable) {
		return nil, "", userErr("Pushing presets to the device is not available yet: the device export (plan step P2) has not been built. Nothing was queued.")
	}
	if err != nil {
		return nil, "", err
	}
	if err := e.Validate(); err != nil {
		return nil, "", fmt.Errorf("exporter produced an invalid export: %w", err)
	}
	sum, err := e.PlanSHA256()
	if err != nil {
		return nil, "", err
	}
	return e, sum, nil
}

func (r PushRequest) claims(sum string) ConfirmClaims {
	return ConfirmClaims{PresetID: r.PresetID, Guitar: r.Guitar, Slot: r.Slot, Name: r.Name, PlanSHA256: sum}
}

// PlanPush is the dry run: it builds the export and, when nothing is
// unresolved, a confirm token. It writes nothing.
func (b *Bridge) PlanPush(ctx context.Context, sp storage.StructuredPreset, r PushRequest) (*PushPlan, error) {
	e, sum, err := b.export(ctx, sp, r.Guitar)
	if err != nil {
		return nil, err
	}
	hv, err := b.Store.Heartbeat(ctx, b.now())
	if err != nil {
		return nil, err
	}
	p := &PushPlan{Export: e, PlanSHA256: sum, Bridge: hv}
	if hv.Heartbeat != nil && !SlotAllowed(r.Slot, hv.Heartbeat.AllowedSlots) {
		p.SlotNote = fmt.Sprintf("slot %s is outside the bridge's allowed slots; the bridge will refuse it", r.Slot)
	}
	if len(e.Unresolved) == 0 {
		p.Token, p.TokenExp = b.Keys.IssueConfirm(r.claims(sum), b.now())
	}
	return p, nil
}

// ConfirmPush checks the token against the arguments and a freshly rebuilt
// export, then queues the push. A repeated confirm returns the existing job
// (existing=true) without writing again.
func (b *Bridge) ConfirmPush(ctx context.Context, sp storage.StructuredPreset, r PushRequest, token string) (id string, existing bool, err error) {
	now := b.now()
	// The claims include the plan hash, so the export is rebuilt from the
	// preset as it is now before the token can be checked.
	e, sum, err := b.export(ctx, sp, r.Guitar)
	if err != nil {
		return "", false, err
	}
	switch err := b.Keys.CheckConfirm(token, r.claims(sum), now); {
	case errors.Is(err, ErrConfirmExpired):
		return "", false, userErr("The confirm_token has expired (they last 10 minutes). Run a new dry run, show the plan to the user again, and confirm with the new token.")
	case errors.Is(err, ErrConfirmMismatch):
		return "", false, userErr("The confirm_token does not match: the preset_id, guitar, slot or name differ from the dry run, or the preset changed since then. Run a new dry run and show the user the new plan.")
	case err != nil:
		return "", false, userErr("confirm_token is not valid. Run send_preset_to_device without confirm_token (a dry run) to get one.")
	}
	if len(e.Unresolved) > 0 {
		return "", false, userErr("The plan has unresolved items, so it cannot be pushed. Run a dry run to see them.")
	}
	id = JobIDFromToken(token)
	if v, err := b.Store.JobState(ctx, id, nil, now); err != nil {
		return "", false, err
	} else if v.State != StateNotFound {
		return id, true, nil
	}
	hv, err := b.Store.Heartbeat(ctx, now)
	if err != nil {
		return "", false, err
	}
	if !hv.Online {
		return "", false, userErr("Not queued: %s.", hv.Reason)
	}
	if !SlotAllowed(r.Slot, hv.Heartbeat.AllowedSlots) {
		return "", false, userErr("Not queued: slot %s is outside the slots the bridge may write. Pick a slot in its allowed range (see device_status).", r.Slot)
	}
	if err := b.checkQueue(ctx, r.Slot); err != nil {
		return "", false, err
	}
	p, err := e.compactPayload()
	if err != nil {
		return "", false, err
	}
	args := PushArgs{
		Slot: r.Slot, Name: r.Name,
		Source: PushSource{PresetID: r.PresetID, Guitar: r.Guitar},
		Export: ExportRecord{Format: e.Format, Payload: p},
	}
	j, err := b.Store.NewJob(id, KindPushPreset, args, now)
	if err != nil {
		return "", false, err
	}
	if err := b.Store.Enqueue(ctx, j); err != nil {
		if errors.Is(err, ErrJobExists) {
			return id, true, nil
		}
		return "", false, err
	}
	return id, false, nil
}

func (b *Bridge) checkQueue(ctx context.Context, pushSlot string) error {
	pend, err := b.Store.PendingJobs(ctx, b.now())
	if err != nil {
		return err
	}
	if len(pend) >= MaxPendingJobs {
		return userErr("Not queued: %d device jobs are already waiting for the bridge. Wait for them to finish (get_device_job), then retry.", len(pend))
	}
	if pushSlot == "" {
		return nil
	}
	for _, j := range pend {
		if j.Kind != KindPushPreset {
			continue
		}
		var a PushArgs
		if json.Unmarshal(j.Args, &a) == nil && a.Slot == pushSlot {
			return userErr("Not queued: a push to slot %s is already waiting (job %s). Wait for it with get_device_job.", pushSlot, j.JobID)
		}
	}
	return nil
}

// QueueRead queues a list_slots or read_preset job and returns its id. It
// refuses when the bridge is offline.
func (b *Bridge) QueueRead(ctx context.Context, kind string) (string, error) {
	var args any
	switch kind {
	case KindListSlots:
		args = struct{}{}
	case KindReadPreset:
		args = ReadArgs{Slot: nil}
	default:
		return "", fmt.Errorf("not a read kind: %s", kind)
	}
	now := b.now()
	hv, err := b.Store.Heartbeat(ctx, now)
	if err != nil {
		return "", err
	}
	if !hv.Online {
		return "", userErr("Not queued: %s.", hv.Reason)
	}
	if err := b.checkQueue(ctx, ""); err != nil {
		return "", err
	}
	j, err := b.Store.NewJob(uuid.NewString(), kind, args, now)
	if err != nil {
		return "", err
	}
	if err := b.Store.Enqueue(ctx, j); err != nil {
		return "", err
	}
	return j.JobID, nil
}

// Job reports a job's state once.
func (b *Bridge) Job(ctx context.Context, id string) (*JobView, error) {
	now := b.now()
	hv, err := b.Store.Heartbeat(ctx, now)
	if err != nil {
		return nil, err
	}
	var hb *Heartbeat
	if hv.Online {
		hb = hv.Heartbeat
	}
	return b.Store.JobState(ctx, id, hb, now)
}

// Wait polls a job until it reaches a final state, max elapses, or ctx ends,
// and returns the last state seen. A failed poll (a storage hiccup) is retried
// until the deadline; it is returned only if no poll succeeded by then.
func (b *Bridge) Wait(ctx context.Context, id string, max time.Duration) (*JobView, error) {
	deadline := time.Now().Add(max)
	gap := b.PollGap
	if gap <= 0 {
		gap = DefaultPollGap
	}
	var last *JobView
	for {
		v, err := b.Job(ctx, id)
		if err == nil {
			last = v
		}
		done := !time.Now().Add(gap).Before(deadline)
		switch {
		case err == nil && Terminal(v.State):
			return v, nil
		case done && last != nil:
			return last, nil
		case done:
			return nil, err
		}
		select {
		case <-ctx.Done():
			if last == nil {
				return nil, ctx.Err()
			}
			return last, nil
		case <-time.After(gap):
		}
	}
}

// slotIndex maps "20A" to 0-based (bank-1)*8+letter.
func slotIndex(s string) (int, bool) {
	s, ok := NormalizeSlot(s)
	if !ok {
		return 0, false
	}
	bank := 0
	for _, c := range s[:len(s)-1] {
		bank = bank*10 + int(c-'0')
	}
	return (bank-1)*8 + int(s[len(s)-1]-'A'), true
}

// SlotAllowed reports whether slot falls in one of the heartbeat's ranges
// ("20A-25H" or a single "20A"). No ranges, or none parseable, means no
// early check: the agent is the enforcer either way.
func SlotAllowed(slot string, ranges []string) bool {
	si, ok := slotIndex(slot)
	if !ok {
		return false
	}
	parsed := 0
	for _, r := range ranges {
		lo, hi, found := strings.Cut(r, "-")
		if !found {
			hi = lo
		}
		a, ok1 := slotIndex(lo)
		z, ok2 := slotIndex(hi)
		if !ok1 || !ok2 || a > z {
			continue
		}
		parsed++
		if si >= a && si <= z {
			return true
		}
	}
	return parsed == 0
}
