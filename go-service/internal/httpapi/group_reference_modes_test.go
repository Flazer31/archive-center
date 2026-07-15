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
