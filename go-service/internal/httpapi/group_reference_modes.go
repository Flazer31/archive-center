package httpapi

import (
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	referenceModeSupplement = "supplement"
	referenceModePrimary    = "primary"
	referenceModeUnknown    = "unknown"
)

func referenceBindingMode(binding store.SessionReferenceBinding) string {
	switch strings.ToLower(strings.TrimSpace(binding.ReferenceMode)) {
	case "", referenceModeSupplement:
		return referenceModeSupplement
	case referenceModePrimary:
		return referenceModePrimary
	default:
		return referenceModeUnknown
	}
}

func referenceBindingModeCounts(bindings []store.SessionReferenceBinding) map[string]int {
	counts := map[string]int{
		referenceModeSupplement: 0,
		referenceModePrimary:    0,
		referenceModeUnknown:    0,
	}
	for _, binding := range bindings {
		counts[referenceBindingMode(binding)]++
	}
	return counts
}

func referencePrimaryCandidateApplicable(item referenceRecallItem) bool {
	if !item.Eligible || !referenceCoverageStatusInjectable(item.CoverageStatus) {
		return false
	}
	for _, reason := range item.NeededBy {
		if reason == "primary_chroma_relevance" {
			return true
		}
	}
	return false
}

func referenceRecallModeInstruction(items []referenceInjectionItem) string {
	hasSupplement := false
	hasPrimary := false
	for _, item := range items {
		switch item.ReferenceMode {
		case referenceModeSupplement:
			hasSupplement = true
		case referenceModePrimary:
			hasPrimary = true
		}
	}
	switch {
	case hasPrimary && hasSupplement:
		return "Primary sources establish missing canon foundations; supplement sources fill only uncovered details."
	case hasPrimary:
		return "This reference library is the user-selected primary canon source for missing world and character context."
	default:
		return "Supplement only: fill uncovered canon details without repeating or replacing supplied lore."
	}
}

func referenceRecallPrecedenceInstruction(items []referenceInjectionItem) string {
	for _, item := range items {
		if item.ReferenceMode == referenceModePrimary {
			return "Current user input and explicit user-authored divergence override this reference. Approved primary canon overrides unsupported model-invented or session-derived claims. Preserve session-original additions only when they do not conflict with approved canon."
		}
	}
	return "Current user input and session-established facts override this reference."
}
