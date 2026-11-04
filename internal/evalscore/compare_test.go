package evalscore

import (
	"strings"
	"testing"
)

func run(q string, p int, order string, rep int, allHard bool, names ...string) LibrarianRun {
	hp := 1
	if !allHard {
		hp = 0
	}
	return LibrarianRun{Query: q, Phase1: p, Order: order, Rep: rep, ParseOK: true, Names: names, In: 1000, Cached: 500,
		Score: &Result{HardTotal: 1, HardPass: hp, SoftTotal: 1, SoftPass: 1, OneAmp: true, AllHardPass: allHard}}
}

func TestCompareLibrarian_IdenticalIsNotWorse(t *testing.T) {
	var runs []LibrarianRun
	for _, q := range []string{"a", "b", "c", "d", "e", "f"} {
		for p := 0; p < 2; p++ {
			for rep := 0; rep < 4; rep++ {
				for _, o := range []string{"new", "old"} {
					runs = append(runs, run(q, p, o, rep, true, "Brit 2203"))
				}
			}
		}
	}
	c := CompareLibrarian(runs, 0.05)
	if c.Units != 12 || c.AllHardVerdict != NotWorse || c.HardFracVerdict != NotWorse {
		t.Errorf("identical conditions: %+v", c)
	}
	if c.SignPos != 0 || c.SignNeg != 0 || c.SignTies != 12 {
		t.Errorf("sign counts: %+v", c)
	}
	if !strings.Contains(c.Markdown(), "NOT_WORSE") {
		t.Error("report should state the verdict")
	}
}

func TestCompareLibrarian_ClearRegressionIsWorse(t *testing.T) {
	var runs []LibrarianRun
	for _, q := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		for rep := 0; rep < 4; rep++ {
			runs = append(runs, run(q, 0, "new", rep, false, "x"), run(q, 0, "old", rep, true, "y"))
		}
	}
	c := CompareLibrarian(runs, 0.05)
	if c.AllHardVerdict != Worse || c.SignNeg != 8 || c.SignPos != 0 {
		t.Errorf("want WORSE with 8 old-better units, got %+v", c)
	}
}

func TestCompareLibrarian_ErrorsAndUnparseableAreExcludedNotCountedAsPasses(t *testing.T) {
	runs := []LibrarianRun{
		run("a", 0, "new", 0, true, "x"),
		{Query: "a", Phase1: 0, Order: "new", Rep: 1, Err: "boom"},
		{Query: "a", Phase1: 0, Order: "new", Rep: 2, ParseOK: false},
		run("a", 0, "old", 0, true, "x"),
	}
	c := CompareLibrarian(runs, 0.05)
	if c.New.Errors != 1 || c.New.ParseFail != 1 || c.New.Runs != 3 {
		t.Errorf("accounting wrong: %+v", c.New)
	}
	if c.Units != 1 {
		t.Errorf("units = %d", c.Units)
	}
}

func TestCompareLibrarian_UnitNeedsBothOrders(t *testing.T) {
	c := CompareLibrarian([]LibrarianRun{run("a", 0, "new", 0, true, "x")}, 0.05)
	if c.Units != 0 {
		t.Errorf("a unit with only one order must not count: %d", c.Units)
	}
}
