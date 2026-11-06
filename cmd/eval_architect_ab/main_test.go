package main

import (
	"reflect"
	"strings"
	"testing"
)

// Every report row has one cell per header column: Arm, OK/Runs, then reportColumns averages.
func TestReportColumnsMatchHeader(t *testing.T) {
	var l runLine
	l.Arm = "v5"
	l.Report.Catalog.UnrecognizedBlocks = 3
	out := renderReport([]runLine{l}, []string{"v5"})
	rows := strings.Split(strings.TrimSpace(out), "\n")
	var header, sep, row string
	for i, r := range rows {
		if strings.HasPrefix(r, "| Arm |") {
			header, sep, row = r, rows[i+1], rows[i+2]
		}
	}
	cells := func(s string) int { return strings.Count(s, "|") - 1 }
	if cells(header) != reportColumns+2 || cells(sep) != cells(header) || cells(row) != cells(header) {
		t.Fatalf("header %d, separator %d, row %d cells; want %d\n%s", cells(header), cells(sep), cells(row), reportColumns+2, out)
	}
	if !strings.Contains(header, "DevUnrecognized") || !strings.Contains(row, " 3.00 |") {
		t.Fatalf("UnrecognizedBlocks column missing:\n%s", out)
	}
}

func TestPanStats(t *testing.T) {
	raw := `{"structured_payload":{"guitars":{"G":[{"type":"Cabinet","model":"212 UK C30 65",
	  "parameters":[
	    {"name":"Mic 1 Pan","value":"0"},
	    {"name":"Mic 2 Pan","value":"5"},
	    {"name":"Mic 1 Position","value":"3"}],
	  "kept_defaults":["MIC 1 BALANCE","MIC 2 BALANCE"]},
	 {"type":"Cabinet","model":"212 UK C30 65",
	  "parameters":[
	    {"name":"Mic 1 Pan","value":"Center"},
	    {"name":"Mic 2 Pan","value":"30"},
	    {"name":"Mic 1 Balance","value":"0.5"}]}]}}}`
	got := panStats(raw)
	want := PanStats{Default: 1, Kept: 2, AtMin: 1, Other: 1, OutOfRange: 1, Text: 1}
	got.Written = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("panStats = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(panStats("not json"), PanStats{}) {
		t.Error("unparseable output should give zero stats")
	}
}

// A pan the model repeats, in parameters or in kept_defaults, is still counted once.
func TestPanStatsCountsEachPanOnce(t *testing.T) {
	raw := `{"structured_payload":{"guitars":{"G":[{"type":"Cabinet","model":"212 UK C30 65",
	  "parameters":[
	    {"name":"Mic 1 Pan","value":"0"},
	    {"name":"MIC 1 PAN","value":"0"}],
	  "kept_defaults":["MIC 2 PAN","Mic 2 Pan","MIC 2 PAN"]}]}}}`
	got := panStats(raw)
	got.Written = nil
	want := PanStats{Kept: 1, AtMin: 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("panStats = %+v, want %+v", got, want)
	}
}
