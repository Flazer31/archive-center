package compare

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ValueResult extends Result with deep payload comparison.
type ValueResult struct {
	Endpoint          string      `json:"endpoint"`
	Allowed           bool        `json:"allowed"`
	StatusMatch       bool        `json:"status_match"`
	KeysMatch         bool        `json:"keys_match"`
	ExactJSONMatch    bool        `json:"exact_json_match"`
	BehaviorMatch     bool        `json:"behavior_match"`
	VolatileOnlyDiffs bool        `json:"volatile_only_diffs,omitempty"`
	TypeMatch         bool        `json:"type_match"`
	PythonStatus      int         `json:"python_status"`
	GoStatus          int         `json:"go_status"`
	PythonKeys        []string    `json:"python_keys,omitempty"`
	GoKeys            []string    `json:"go_keys,omitempty"`
	MissingKeys       []string    `json:"missing_keys,omitempty"`
	ExtraKeys         []string    `json:"extra_keys,omitempty"`
	DurationMS        int64       `json:"duration_ms"`
	Error             string      `json:"error,omitempty"`
	Diffs             []ValueDiff `json:"diffs,omitempty"`
	DiffTruncated     bool        `json:"diff_truncated"`
}

// ValueReport holds aggregate results of a value parity run.
type ValueReport struct {
	Timestamp   time.Time
	PythonBase  string
	GoBase      string
	MaxDiffs    int
	Results     []ValueResult
	SkipReasons map[string]string
}

// ReportSummary provides aggregated counts for a ValueReport.
type ReportSummary struct {
	Total                 int `json:"total"`
	Compared              int `json:"compared"`
	Skipped               int `json:"skipped"`
	Allowed               int `json:"allowed"`
	Blocked               int `json:"blocked"`
	ExactMatches          int `json:"exact_matches"`
	BehaviorMatches       int `json:"behavior_matches"`
	MismatchCount         int `json:"mismatch_count"`
	BehaviorMismatchCount int `json:"behavior_mismatch_count"`
	TransportErrorCount   int `json:"transport_error_count"`
	StatusMismatchCount   int `json:"status_mismatch_count"`
	KeysMismatchCount     int `json:"keys_mismatch_count"`
	TypeMismatchCount     int `json:"type_mismatch_count"`
	ValueMismatchCount    int `json:"value_mismatch_count"`
	VolatileOnlyCount     int `json:"volatile_only_count"`
	TruncatedDiffCount    int `json:"truncated_diff_count"`
}

// Summarize computes aggregate counts from the report results.
func (r *ValueReport) Summarize() ReportSummary {
	var s ReportSummary
	s.Total = len(r.Results)

	for _, res := range r.Results {
		if _, ok := r.SkipReasons[res.Endpoint]; ok {
			s.Skipped++
			if res.Allowed {
				s.Allowed++
			} else {
				s.Blocked++
			}
			continue
		}
		s.Compared++
		if res.Allowed {
			s.Allowed++
		} else {
			s.Blocked++
		}
		if res.ExactJSONMatch {
			s.ExactMatches++
		}
		if res.BehaviorMatch {
			s.BehaviorMatches++
		}
		if res.VolatileOnlyDiffs {
			s.VolatileOnlyCount++
		}
		if strings.Contains(res.Error, "backend:") {
			s.TransportErrorCount++
		}
		if !res.StatusMatch {
			s.StatusMismatchCount++
		}
		if !res.KeysMatch {
			s.KeysMismatchCount++
		}
		if !res.TypeMatch {
			s.TypeMismatchCount++
		}
		if res.DiffTruncated {
			s.TruncatedDiffCount++
		}
		hasValueMismatch := false
		for _, d := range res.Diffs {
			if d.Type == "scalar" || d.Type == "list_length" {
				hasValueMismatch = true
				break
			}
		}
		if hasValueMismatch {
			s.ValueMismatchCount++
		}
		if res.Error != "" || !res.StatusMatch || !res.KeysMatch || !res.ExactJSONMatch {
			s.MismatchCount++
		}
		if res.Error != "" || !res.BehaviorMatch {
			s.BehaviorMismatchCount++
		}
	}

	return s
}

// WriteMarkdown emits a markdown value parity report to w.
func (r *ValueReport) WriteMarkdown(w io.Writer) error {
	var b strings.Builder

	b.WriteString("# Shadow Value Parity Report (R1 Report-Only)\n\n")
	b.WriteString("> **Disclaimer**: This is R1 report-only evidence. It measures and documents payload/value differences between Python and Go shadow responses. It is **not** a green/cutover-ready claim.\n\n")
	b.WriteString(fmt.Sprintf("- **Generated**: %s\n", r.Timestamp.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Python Base**: %s\n", r.PythonBase))
	b.WriteString(fmt.Sprintf("- **Go Base**: %s\n", r.GoBase))
	b.WriteString(fmt.Sprintf("- **Diff Limit**: %d per endpoint\n\n", r.MaxDiffs))

	b.WriteString("## Volatile Field Policy\n\n")
	b.WriteString("The following fields are treated as volatile and may legitimately differ between backends:\n")
	b.WriteString("- `timestamp`, `exported_at`, `generated_at`, `runtime_updated_at`, `updated_at`\n\n")
	b.WriteString("Differences in these fields are reported but marked as volatile and do not count toward type, value, or behavior mismatch totals.\n\n")

	// Summary table
	b.WriteString("## Endpoint Summary\n\n")
	b.WriteString("| Endpoint | Allowed | Status Match | Keys Match | Exact JSON | Behavior Match | Type Match | Diffs | Truncated | Error |\n")
	b.WriteString("|----------|---------|--------------|------------|------------|----------------|------------|-------|-----------|-------|\n")

	compared := 0
	skipped := 0
	exactMatches := 0
	behaviorMatches := 0
	typeMatches := 0
	valueMismatchEndpoints := 0

	for _, res := range r.Results {
		if reason, ok := r.SkipReasons[res.Endpoint]; ok {
			skipped++
			b.WriteString(fmt.Sprintf("| %s | %v | none | none | none | none | none | none | SKIP: %s |\n", res.Endpoint, res.Allowed, reason))
			continue
		}
		if res.Allowed {
			compared++
			if res.ExactJSONMatch {
				exactMatches++
			}
			if res.BehaviorMatch {
				behaviorMatches++
			}
			if res.TypeMatch {
				typeMatches++
			}
			hasValueMismatch := false
			for _, d := range res.Diffs {
				if d.Type == "scalar" || d.Type == "list_length" {
					hasValueMismatch = true
					break
				}
			}
			if hasValueMismatch {
				valueMismatchEndpoints++
			}
		}
		diffs := fmt.Sprintf("%d", len(res.Diffs))
		truncated := fmt.Sprintf("%v", res.DiffTruncated)
		errStr := res.Error
		if errStr == "" {
			errStr = "none"
		}
		b.WriteString(fmt.Sprintf("| %s | %v | %v | %v | %v | %v | %v | %s | %s | %s |\n",
			res.Endpoint, res.Allowed, res.StatusMatch, res.KeysMatch, res.ExactJSONMatch, res.BehaviorMatch, res.TypeMatch, diffs, truncated, errStr))
	}

	b.WriteString("\n")

	// Detailed diffs
	b.WriteString("## Detailed Diffs\n\n")
	for _, res := range r.Results {
		if _, ok := r.SkipReasons[res.Endpoint]; ok {
			continue
		}
		if len(res.Diffs) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("### %s\n\n", res.Endpoint))
		for _, d := range res.Diffs {
			b.WriteString(fmt.Sprintf("- **Path**: `%s` | **Type**: `%s` | **Python**: `%s` | **Go**: `%s`", d.Path, d.Type, d.PythonVal, d.GoVal))
			if d.Note != "" {
				b.WriteString(fmt.Sprintf(" | **Note**: %s", d.Note))
			}
			b.WriteString("\n")
		}
		if res.DiffTruncated {
			b.WriteString("- *(diffs truncated due to limit)*\n")
		}
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- **Total Probes**: %d\n", len(r.Results)))
	b.WriteString(fmt.Sprintf("- **Compared**: %d\n", compared))
	b.WriteString(fmt.Sprintf("- **Skipped**: %d\n", skipped))
	b.WriteString(fmt.Sprintf("- **Exact JSON Matches**: %d / %d\n", exactMatches, compared))
	b.WriteString(fmt.Sprintf("- **Behavior Matches**: %d / %d\n", behaviorMatches, compared))
	b.WriteString(fmt.Sprintf("- **Type Matches**: %d / %d\n", typeMatches, compared))
	b.WriteString(fmt.Sprintf("- **Endpoints with Value/List-Length Mismatches**: %d / %d\n", valueMismatchEndpoints, compared))

	_, err := w.Write([]byte(b.String()))
	return err
}

// WriteJSON emits the ValueReport as JSON to w, including a computed summary.
func (r *ValueReport) WriteJSON(w io.Writer) error {
	type envelope struct {
		Timestamp   time.Time         `json:"timestamp"`
		PythonBase  string            `json:"python_base"`
		GoBase      string            `json:"go_base"`
		MaxDiffs    int               `json:"max_diffs"`
		Summary     ReportSummary     `json:"summary"`
		Results     []ValueResult     `json:"results"`
		SkipReasons map[string]string `json:"skip_reasons"`
	}

	e := envelope{
		Timestamp:   r.Timestamp,
		PythonBase:  r.PythonBase,
		GoBase:      r.GoBase,
		MaxDiffs:    r.MaxDiffs,
		Summary:     r.Summarize(),
		Results:     r.Results,
		SkipReasons: r.SkipReasons,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}
