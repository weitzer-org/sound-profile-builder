package evalscore

import (
	"fmt"
	"sort"
	"strings"
)

// Phase1Roles are the agents whose cost is excluded from "cost excluding Phase 1",
// because a paired run reuses frozen Phase 1 output in one condition and not the other.
var Phase1Roles = map[string]bool{"Tone Historian": true, "Sonic Profiler": true, "Community Scraper": true}

// AgentTokens is one agent's token use in a pipeline run.
type AgentTokens struct {
	Model    string  `json:"model"`
	In       int32   `json:"in"`
	Cached   int32   `json:"cached"`
	Out      int32   `json:"out"`
	Thinking int32   `json:"thinking"`
	CostUSD  float64 `json:"cost_usd"`
}

// PipelineRun is one full-pipeline run in a paired eval (a row of manifest.jsonl).
type PipelineRun struct {
	Condition     string                 `json:"condition"`
	Query         string                 `json:"query"`
	Sample        int                    `json:"sample"`
	OK            bool                   `json:"ok"`
	Err           string                 `json:"err,omitempty"`
	LatencySec    float64                `json:"latency_sec"`
	Phase1Reused  bool                   `json:"phase1_reused"`
	HTMLPath      string                 `json:"html_path,omitempty"`
	LibrarianPath string                 `json:"librarian_path,omitempty"`
	In            int32                  `json:"in"`
	Cached        int32                  `json:"cached"`
	Out           int32                  `json:"out"`
	Thinking      int32                  `json:"thinking"`
	CostUSD       float64                `json:"cost_usd"`
	CostExP1USD   float64                `json:"cost_ex_phase1_usd"`
	PerAgent      map[string]AgentTokens `json:"per_agent,omitempty"`
	ModelsUsed    map[string]int         `json:"models_used,omitempty"`
	MechParseErr  string                 `json:"mech_parse_error,omitempty"`
	MechDefects   int                    `json:"mech_defects"`
	LibScore      *Result                `json:"librarian_score,omitempty"`
	PresetScore   *Result                `json:"preset_score,omitempty"`
}

// Metric is one comparable number extracted from a run.
type Metric struct {
	Name           string
	HigherIsBetter bool
	Quality        bool // gates the non-inferiority verdict
	Get            func(PipelineRun) (float64, bool)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func frac(p, t int) float64 {
	if t == 0 {
		return 1
	}
	return float64(p) / float64(t)
}

// PipelineMetrics are the quality / cost metrics compared between conditions.
func PipelineMetrics() []Metric {
	ok := func(r PipelineRun) bool { return r.OK }
	return []Metric{
		{"pipeline succeeded", true, true, func(r PipelineRun) (float64, bool) { return b2f(r.OK), true }},
		{"preset: all hard rules + single amp", true, true, func(r PipelineRun) (float64, bool) {
			return b2f(r.PresetScore != nil && r.PresetScore.AllHardPass), ok(r) && r.PresetScore != nil
		}},
		{"preset: hard-rule pass fraction", true, true, func(r PipelineRun) (float64, bool) {
			if r.PresetScore == nil {
				return 0, false
			}
			return frac(r.PresetScore.HardPass, r.PresetScore.HardTotal), ok(r)
		}},
		{"preset: soft-rule pass fraction", true, false, func(r PipelineRun) (float64, bool) {
			if r.PresetScore == nil {
				return 0, false
			}
			return frac(r.PresetScore.SoftPass, r.PresetScore.SoftTotal), ok(r)
		}},
		{"librarian: all hard rules + single amp", true, true, func(r PipelineRun) (float64, bool) {
			return b2f(r.LibScore != nil && r.LibScore.AllHardPass), ok(r) && r.LibScore != nil
		}},
		{"librarian: soft-rule pass fraction", true, false, func(r PipelineRun) (float64, bool) {
			if r.LibScore == nil {
				return 0, false
			}
			return frac(r.LibScore.SoftPass, r.LibScore.SoftTotal), ok(r)
		}},
		{"preset: Range Rverb uses", false, false, func(r PipelineRun) (float64, bool) {
			if r.PresetScore == nil {
				return 0, false
			}
			return float64(r.PresetScore.RangeRverb), ok(r)
		}},
		{"mechanical defects (Architect JSON)", false, true, func(r PipelineRun) (float64, bool) {
			return float64(r.MechDefects), ok(r) && r.MechParseErr == ""
		}},
		{"cached input share", true, false, func(r PipelineRun) (float64, bool) {
			if r.In == 0 {
				return 0, false
			}
			return float64(r.Cached) / float64(r.In), ok(r)
		}},
		{"cost excl. Phase 1 (USD)", false, false, func(r PipelineRun) (float64, bool) { return r.CostExP1USD, ok(r) }},
		{"latency (s)", false, false, func(r PipelineRun) (float64, bool) { return r.LatencySec, ok(r) }},
	}
}

// MetricRow is the paired comparison of one metric.
type MetricRow struct {
	Name             string
	N                int
	NewMean, OldMean float64
	Diff, Lo, Hi     float64
	Verdict          Verdict // only meaningful for Quality metrics
	Quality          bool
	SignPos, SignNeg int
	SignP            float64
	HigherIsBetter   bool
}

// ComparePipelines pairs new and old runs on (query, sample) and compares every metric.
// Pairs where either side lacks the metric (e.g. a failed run) are dropped for that
// metric; the 'pipeline succeeded' metric counts failures explicitly. margin applies to
// quality metrics (a score in 0..1 may drop by at most margin; for defect counts it is
// the allowed increase in absolute defects per run).
func ComparePipelines(newRuns, oldRuns []PipelineRun, margin float64) []MetricRow {
	type k struct {
		q string
		s int
	}
	oldBy := map[k]PipelineRun{}
	for _, r := range oldRuns {
		oldBy[k{r.Query, r.Sample}] = r
	}
	var rows []MetricRow
	for mi, m := range PipelineMetrics() {
		var diffs, nv, ov []float64
		row := MetricRow{Name: m.Name, Quality: m.Quality, HigherIsBetter: m.HigherIsBetter}
		for _, nr := range newRuns {
			or, ok := oldBy[k{nr.Query, nr.Sample}]
			if !ok {
				continue
			}
			a, aok := m.Get(nr)
			b, bok := m.Get(or)
			if !aok || !bok {
				continue
			}
			nv, ov = append(nv, a), append(ov, b)
			d := a - b
			if !m.HigherIsBetter {
				d = -d // normalise so a positive diff always means "new is better"
			}
			diffs = append(diffs, d)
			switch {
			case d > 0:
				row.SignPos++
			case d < 0:
				row.SignNeg++
			}
		}
		row.N = len(diffs)
		row.NewMean, row.OldMean = Mean(nv), Mean(ov)
		if row.N > 0 {
			if m.Quality {
				row.Verdict, row.Diff, row.Lo, row.Hi = NonInferiority(diffs, margin, int64(mi+1))
			} else {
				row.Diff = Mean(diffs)
				row.Lo, row.Hi = BootstrapMeanCI(diffs, 10000, 0.95, int64(mi+1))
			}
		}
		row.SignP = SignTestP(row.SignPos, row.SignNeg)
		rows = append(rows, row)
	}
	return rows
}

// PipelineMarkdown renders the comparison. Differences are signed so that positive
// always means "new is better", including for lower-is-better metrics.
func PipelineMarkdown(rows []MetricRow, margin float64, nNew, nOld int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Full-pipeline paired comparison: new order vs old order\n\nruns: new=%d old=%d. Differences are (new - old), signed so **positive = new is better** for every metric. 95%% bootstrap CIs over paired (query, sample) units. Quality metrics carry a non-inferiority verdict at a %.0f-point margin; others are descriptive.\n\n", nNew, nOld, 100*margin)
	b.WriteString("| metric | n pairs | new mean | old mean | diff (new better +) | 95% CI | verdict | sign test (new/old better, p) |\n|---|---|---|---|---|---|---|---|\n")
	sorted := append([]MetricRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Quality && !sorted[j].Quality })
	for _, r := range sorted {
		v := "-"
		if r.Quality && r.N > 0 {
			v = "**" + string(r.Verdict) + "**"
		}
		fmt.Fprintf(&b, "| %s | %d | %.3f | %.3f | %+.3f | [%+.3f, %+.3f] | %s | %d/%d, p=%.3f |\n", r.Name, r.N, r.NewMean, r.OldMean, r.Diff, r.Lo, r.Hi, v, r.SignPos, r.SignNeg, r.SignP)
	}
	return b.String()
}
