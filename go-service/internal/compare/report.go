package compare

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Report holds the aggregate results of a parity run.
type Report struct {
	Timestamp   time.Time
	PythonBase  string
	GoBase      string
	Results     []Result
	SkipReasons map[string]string
}

// WriteMarkdown emits a markdown parity report to w.
func (r *Report) WriteMarkdown(w io.Writer) error {
	var b strings.Builder

	b.WriteString("# Shadow Parity Report\n\n")
	b.WriteString(fmt.Sprintf("- **Generated**: %s\n", r.Timestamp.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Python Base**: %s\n", r.PythonBase))
	b.WriteString(fmt.Sprintf("- **Go Base**: %s\n\n", r.GoBase))

	// Route table
	b.WriteString("## Route Table\n\n")
	b.WriteString("| Endpoint | Allowed | Status Match | Py Status | Go Status | Keys Match | Missing Keys | Extra Keys | Duration (ms) | Error |\n")
	b.WriteString("|----------|---------|--------------|-----------|-----------|------------|--------------|------------|---------------|-------|\n")

	allowedCount := 0
	statusMatchCount := 0
	keysMatchCount := 0
	skipCount := 0
	errorCount := 0

	for _, res := range r.Results {
		if reason, ok := r.SkipReasons[res.Endpoint]; ok {
			skipCount++
			b.WriteString(fmt.Sprintf("| %s | %v | none | none | none | none | none | none | none | SKIP: %s |\n", res.Endpoint, res.Allowed, reason))
			continue
		}
		if res.Error != "" {
			errorCount++
		}
		if res.Allowed {
			allowedCount++
			if res.StatusMatch {
				statusMatchCount++
			}
			if res.KeysMatch {
				keysMatchCount++
			}
		}
		missing := strings.Join(res.MissingKeys, ", ")
		extra := strings.Join(res.ExtraKeys, ", ")
		if missing == "" {
			missing = "none"
		}
		if extra == "" {
			extra = "none"
		}
		errStr := res.Error
		if errStr == "" {
			errStr = "none"
		}
		b.WriteString(fmt.Sprintf("| %s | %v | %v | %d | %d | %v | %s | %s | %d | %s |\n",
			res.Endpoint, res.Allowed, res.StatusMatch, res.PythonStatus, res.GoStatus, res.KeysMatch, missing, extra, res.DurationMS, errStr))
	}

	b.WriteString("\n")

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- **Total Probes**: %d\n", len(r.Results)))
	b.WriteString(fmt.Sprintf("- **Allowed**: %d\n", allowedCount))
	b.WriteString(fmt.Sprintf("- **Skipped**: %d\n", skipCount))
	b.WriteString(fmt.Sprintf("- **Errors**: %d\n", errorCount))
	b.WriteString(fmt.Sprintf("- **Status Match**: %d / %d\n", statusMatchCount, allowedCount))
	b.WriteString(fmt.Sprintf("- **Keys Match**: %d / %d\n", keysMatchCount, allowedCount))

	if len(r.SkipReasons) > 0 {
		b.WriteString("\n## Skip Reasons\n\n")
		for endpoint, reason := range r.SkipReasons {
			b.WriteString(fmt.Sprintf("- `%s`: %s\n", endpoint, reason))
		}
	}

	_, err := w.Write([]byte(b.String()))
	return err
}
