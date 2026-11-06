package presetsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

// ImportDraft stores a preset read back from a Quad Cortex (plan P5) as an unsaved draft:
// Name is DraftName, so list_presets shows it as a draft and save_preset can name it, and
// Origin records that it came from the unit. The draft is written through the same
// storage.Client every other preset uses.
//
// id is chosen by the caller (the device tool derives it from the read job, so asking twice
// for the same read doesn't create twice). If a preset with that id already exists it is
// returned untouched with created=false (a storage error is returned, never treated as absent): an import never overwrites, so a draft the user has
// since renamed or refined stays as it is.
func (s *Service) ImportDraft(ctx context.Context, id string, sp storage.StructuredPreset, origin storage.PresetOrigin, statement string) (p *storage.Preset, created bool, err error) {
	if id == "" || len(sp.Guitars) == 0 {
		return nil, false, errors.New("an imported draft needs an id and at least one guitar")
	}
	// Only a clean "not there" may lead to a write: a storage error must not let a draft the
	// user has since renamed or refined be replaced by the original import.
	exists, err := s.d.Store.Exists(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("checking for an earlier import: %w", err)
	}
	if exists {
		existing, gerr := s.d.Store.Get(ctx, id)
		if gerr != nil {
			return nil, false, fmt.Errorf("reading the earlier import: %w", gerr)
		}
		return existing, false, nil
	}
	// Same envelope shape the generator writes ({"structured":…,"legacy_html":…}); an import
	// has no HTML view, so legacy_html is empty and the UI renders the structured chain.
	payload, err := json.Marshal(map[string]interface{}{
		"structured":  sp,
		"legacy_html": map[string]string{},
	})
	if err != nil {
		return nil, false, fmt.Errorf("encoding imported preset: %w", err)
	}
	o := origin
	p = &storage.Preset{ID: id, Name: DraftName, Payload: string(payload), BuilderStatement: statement, Origin: &o}
	if err := s.d.Store.Save(ctx, p); err != nil {
		return nil, false, fmt.Errorf("saving imported draft: %w", err)
	}
	return p, true, nil
}
