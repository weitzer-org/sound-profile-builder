package main

import "testing"

func TestResolveDates_SingleDate(t *testing.T) {
	dates, err := resolveDates("2026-07-29", "", "")
	if err != nil {
		t.Fatalf("resolveDates: %v", err)
	}
	if len(dates) != 1 || dates[0] != "2026-07-29" {
		t.Fatalf("dates = %v, want [2026-07-29]", dates)
	}
}

func TestResolveDates_Range(t *testing.T) {
	dates, err := resolveDates("", "2026-07-27", "2026-07-29")
	if err != nil {
		t.Fatalf("resolveDates: %v", err)
	}
	want := []string{"2026-07-27", "2026-07-28", "2026-07-29"}
	if len(dates) != len(want) {
		t.Fatalf("dates = %v, want %v", dates, want)
	}
	for i := range want {
		if dates[i] != want[i] {
			t.Errorf("dates[%d] = %q, want %q", i, dates[i], want[i])
		}
	}
}

func TestResolveDates_Defaults_ToToday(t *testing.T) {
	dates, err := resolveDates("", "", "")
	if err != nil {
		t.Fatalf("resolveDates: %v", err)
	}
	if len(dates) != 1 {
		t.Fatalf("expected exactly one (today's) date, got %v", dates)
	}
}

func TestResolveDates_DateWithRange_IsRejected(t *testing.T) {
	if _, err := resolveDates("2026-07-29", "2026-07-27", ""); err == nil {
		t.Fatal("expected an error when -date and -from/-to are both set")
	}
}

func TestResolveDates_OnlyFrom_IsRejected(t *testing.T) {
	if _, err := resolveDates("", "2026-07-27", ""); err == nil {
		t.Fatal("expected an error when only -from is set")
	}
}

func TestResolveDates_ToBeforeFrom_IsRejected(t *testing.T) {
	if _, err := resolveDates("", "2026-07-29", "2026-07-27"); err == nil {
		t.Fatal("expected an error when -to is before -from")
	}
}

func TestResolveDates_InvalidDate_IsRejected(t *testing.T) {
	if _, err := resolveDates("not-a-date", "", ""); err == nil {
		t.Fatal("expected an error for a malformed -date")
	}
}
