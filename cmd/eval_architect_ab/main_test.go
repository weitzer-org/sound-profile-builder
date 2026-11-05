package main

import (
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
