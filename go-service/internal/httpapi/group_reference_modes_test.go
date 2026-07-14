package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestReferenceBindingModeKeepsLegacyCompatibilityAndBlocksInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		expect string
	}{
		{name: "legacy empty", value: "", expect: referenceModeSupplement},
		{name: "supplement", value: "supplement", expect: referenceModeSupplement},
		{name: "primary", value: "primary", expect: referenceModePrimary},
		{name: "invalid", value: "automatic_guess", expect: referenceModeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := referenceBindingMode(store.SessionReferenceBinding{ReferenceMode: tt.value})
			if got != tt.expect {
				t.Fatalf("referenceBindingMode(%q) = %q, want %q", tt.value, got, tt.expect)
			}
		})
	}
}

func TestReferenceRecallModeAwareLimitOnlyExpandsPrimaryByFour(t *testing.T) {
	supplement := []store.SessionReferenceBinding{{ReferenceMode: referenceModeSupplement}}
	primary := []store.SessionReferenceBinding{{ReferenceMode: referenceModePrimary}}

	if got := referenceRecallModeAwareLimit(supplement, 8); got != 8 {
		t.Fatalf("supplement limit = %d, want 8", got)
	}
	if got := referenceRecallModeAwareLimit(primary, 8); got != 12 {
		t.Fatalf("primary limit = %d, want 12", got)
	}
	if got := referenceRecallModeAwareLimit(primary, 20); got != 24 {
		t.Fatalf("primary explicit limit = %d, want 24", got)
	}
	if got := referenceRecallModeAwareLimit(primary, 29); got != 30 {
		t.Fatalf("primary capped limit = %d, want 30", got)
	}
}
