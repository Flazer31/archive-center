package httpapi

import (
	"testing"
	"unicode/utf8"
)

func TestCompactPrepareTurnLineHonorsRuneLimitAndWordBoundary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{
			name:  "compacts whitespace before limiting",
			input: "  alpha \n beta\t gamma  ",
			limit: 12,
			want:  "alpha beta",
		},
		{
			name:  "preserves korean rune boundary",
			input: "\uac00\ub098\ub2e4\ub77c\ub9c8\ubc14\uc0ac",
			limit: 4,
			want:  "\uac00\ub098\ub2e4\ub77c",
		},
		{
			name:  "preserves emoji rune boundary",
			input: "\U0001F9ED\U0001F4DA\U0001F9ED\U0001F4DA\U0001F9ED",
			limit: 3,
			want:  "\U0001F9ED\U0001F4DA\U0001F9ED",
		},
		{
			name:  "nonpositive limit remains unbounded",
			input: " alpha   beta ",
			limit: 0,
			want:  "alpha beta",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compactPrepareTurnLine(tc.input, tc.limit)
			if got != tc.want {
				t.Fatalf("compactPrepareTurnLine(%q, %d) = %q, want %q", tc.input, tc.limit, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("compactPrepareTurnLine returned invalid UTF-8: %q", got)
			}
			if tc.limit > 0 && len([]rune(got)) > tc.limit {
				t.Fatalf("compactPrepareTurnLine returned %d runes, limit %d", len([]rune(got)), tc.limit)
			}
		})
	}
}

func TestSupervisorProposalCoverageKeepsStrengthProposalOnly(t *testing.T) {
	tests := []struct {
		strength string
		scope    string
		call     string
	}{
		{strength: "none", scope: "none", call: "none"},
		{strength: "weak", scope: "response_focus_and_must_account", call: "source_backed_optional"},
		{strength: "medium", scope: "may_advance_or_hold_allowed", call: "source_backed_optional"},
		{strength: "strong", scope: "arc_anchor_and_preferred_frontier", call: "source_backed_optional"},
	}
	for _, tc := range tests {
		t.Run(tc.strength, func(t *testing.T) {
			coverage := supervisorProposalCoverage(tc.strength)
			if coverage["guidance_scope"] != tc.scope || coverage["supervisor_call"] != tc.call {
				t.Fatalf("%s coverage semantics = %#v", tc.strength, coverage)
			}
			for _, key := range []string{
				"truth_authority",
				"canonical_write",
				"force_progress",
				"proactive_complication_opt_in",
				"strong_implies_proactive",
				"strong_implies_forced_progress",
			} {
				if coverage[key] != false {
					t.Fatalf("%s coverage %s = %#v, want false", tc.strength, key, coverage[key])
				}
			}
			for _, key := range []string{
				"blocked_user_action",
				"blocked_new_truth",
				"blocked_relationship_change",
				"blocked_unresolved_event_closure",
				"proactive_complication_requires_opt_in",
			} {
				if coverage[key] != true {
					t.Fatalf("%s coverage %s = %#v, want true", tc.strength, key, coverage[key])
				}
			}
		})
	}
}

func TestTurnWorkflowHUDNarrativeGuidanceFactDoesNotClaimEmptyDelivery(t *testing.T) {
	tests := []struct {
		status      string
		disposition string
		severity    string
	}{
		{status: "applied", disposition: "delivered", severity: turnWorkflowHUDSeverityNormal},
		{status: "valid_empty", disposition: "selected", severity: turnWorkflowHUDSeverityNormal},
		{status: "unsupported_rejected", disposition: "dropped", severity: turnWorkflowHUDSeverityNotice},
		{status: "malformed_failed_open", disposition: "dropped", severity: turnWorkflowHUDSeverityWarning},
		{status: "disabled", disposition: "dropped", severity: turnWorkflowHUDSeverityNormal},
		{status: "deferred_no_guide_support", disposition: "deferred", severity: turnWorkflowHUDSeverityNotice},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			fact := buildTurnWorkflowHUDNarrativeGuidanceFact(tc.status, "")
			if fact.Status != tc.status || fact.Disposition != tc.disposition || fact.Severity != tc.severity {
				t.Fatalf("guidance fact = %#v, want status=%q disposition=%q severity=%q", fact, tc.status, tc.disposition, tc.severity)
			}
		})
	}
	schemaInvalid := buildTurnWorkflowHUDNarrativeGuidanceFact("malformed_failed_open", "supervisor_schema_invalid")
	if schemaInvalid.ReasonCode != "supervisor_schema_invalid" {
		t.Fatalf("schema-invalid reason was hidden: %#v", schemaInvalid)
	}
}
