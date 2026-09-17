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

func TestPublisherStrengthProfileKeepsSixLevelsProposalOnly(t *testing.T) {
	tests := []struct {
		strength     string
		explicitness string
		call         string
	}{
		{strength: "none", explicitness: "disabled", call: "none"},
		{strength: "weak", explicitness: "gentle", call: "single_source_backed"},
		{strength: "medium", explicitness: "balanced", call: "single_source_backed"},
		{strength: "strong", explicitness: "direct", call: "single_source_backed"},
		{strength: "extreme", explicitness: "ordered", call: "single_source_backed"},
		{strength: "maximum", explicitness: "execution_brief", call: "single_source_backed"},
	}
	for _, tc := range tests {
		t.Run(tc.strength, func(t *testing.T) {
			if got := normalizeNarrativeGuideStrength(tc.strength); got != tc.strength {
				t.Fatalf("normalizeNarrativeGuideStrength(%q) = %q", tc.strength, got)
			}
			profile := publisherStrengthProfile(tc.strength)
			if profile["contract_version"] != "publisher_strength_profile.v1" ||
				profile["guidance_explicitness"] != tc.explicitness || profile["publisher_call"] != tc.call {
				t.Fatalf("%s strength semantics = %#v", tc.strength, profile)
			}
			for _, key := range []string{
				"truth_authority",
				"canonical_write",
				"force_progress",
				"force_user_action",
				"invent_new_facts",
				"confirm_relationship_change",
				"close_unresolved_event",
				"persistent_carry",
			} {
				if profile[key] != false {
					t.Fatalf("%s profile %s = %#v, want false", tc.strength, key, profile[key])
				}
			}
			if profile["pressure_independent"] != true || profile["item_policy"] != "supported_items_only_no_filler" {
				t.Fatalf("%s profile introduced pressure coupling or filler: %#v", tc.strength, profile)
			}
			roles := stringSliceFromAny(profile["roles"])
			if tc.strength == "none" && len(roles) != 0 {
				t.Fatalf("none strength has Publisher roles: %#v", profile)
			}
			if tc.strength != "none" && len(roles) != 2 {
				t.Fatalf("%s strength roles = %#v", tc.strength, roles)
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
		{status: "applied_partial", disposition: "delivered", severity: turnWorkflowHUDSeverityNotice},
		{status: "valid_empty", disposition: "selected", severity: turnWorkflowHUDSeverityNormal},
		{status: "publisher_plan_no_valid_items", disposition: "dropped", severity: turnWorkflowHUDSeverityWarning},
		{status: "publisher_json_malformed", disposition: "dropped", severity: turnWorkflowHUDSeverityWarning},
		{status: "publisher_json_truncated", disposition: "dropped", severity: turnWorkflowHUDSeverityWarning},
		{status: "publisher_schema_invalid", disposition: "dropped", severity: turnWorkflowHUDSeverityWarning},
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
	schemaInvalid := buildTurnWorkflowHUDNarrativeGuidanceFact("publisher_schema_invalid", "publisher_schema_invalid")
	if schemaInvalid.ReasonCode != "publisher_schema_invalid" {
		t.Fatalf("schema-invalid reason was hidden: %#v", schemaInvalid)
	}
}
