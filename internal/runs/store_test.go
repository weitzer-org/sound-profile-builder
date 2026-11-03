package runs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"gen-test-1", true},
		{"chat-0a1B2c", true},
		{"a", true},
		{strings.Repeat("a", 80), true},
		{strings.Repeat("a", 81), false},
		{"", false},
		{"-leading-dash", false},
		{"has/slash", false},
		{"../escape", false},
		{"dot.ted", false},
		{"has space", false},
		{"under_score", false},
	}
	for _, tt := range tests {
		if got := ValidID(tt.id); got != tt.want {
			t.Errorf("ValidID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestStoreRejectsInvalidID(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	bad := "../etc/passwd"

	if err := store.Put(ctx, &RunRecord{ID: bad}); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Put: got %v, want ErrInvalidID", err)
	}
	if _, err := store.Get(ctx, bad); !errors.Is(err, ErrInvalidID) {
		t.Errorf("Get: got %v, want ErrInvalidID", err)
	}
	if err := store.WriteLog(ctx, bad, "x.txt", []byte("x")); !errors.Is(err, ErrInvalidID) {
		t.Errorf("WriteLog: got %v, want ErrInvalidID", err)
	}
}

func TestStoreGetMissingIsNotFound(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.Get(context.Background(), "gen-missing-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: got %v, want ErrNotFound", err)
	}
}

func TestStoreWriteLogKey(t *testing.T) {
	store, mc := newTestStore(t)
	if err := store.WriteLog(context.Background(), "gen-test-1", "1_tone_historian.input.txt", []byte("hi")); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	keys, _ := mc.ListFiles(context.Background(), testBucket, "logs/gen-test-1/")
	if len(keys) != 1 || keys[0] != "logs/gen-test-1/1_tone_historian.input.txt" {
		t.Errorf("log keys: %v", keys)
	}
}

func TestNewStoreAbandonAfterEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"unset uses default", "", DefaultAbandonAfter},
		{"valid duration overrides", "15m", 15 * time.Minute},
		{"unparseable falls back to default", "bogus", DefaultAbandonAfter},
		{"non-positive falls back to default", "-1m", DefaultAbandonAfter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RUN_ABANDON_AFTER", tt.env)
			store := NewStore(newMemClient(), testBucket)
			if store.AbandonAfter != tt.want {
				t.Errorf("AbandonAfter = %v, want %v", store.AbandonAfter, tt.want)
			}
		})
	}
}

func TestStoreZeroAbandonAfterUsesDefault(t *testing.T) {
	store, _ := newTestStore(t)
	store.AbandonAfter = 0
	if got := store.abandonAfter(); got != DefaultAbandonAfter {
		t.Errorf("abandonAfter() = %v, want %v", got, DefaultAbandonAfter)
	}
}
