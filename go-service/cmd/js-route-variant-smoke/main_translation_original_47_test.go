package main

import (
	"strings"
	"testing"
)

// Shared production dependencies for fixtures that read source prose. Do not
// replace translation decoding with identity stubs in these fixtures.
func archiveTranslationOriginalReadJS(t *testing.T, src string) string {
	t.Helper()
	names := []string{
		"buildYumiV1ArchiveReadContext", "extractActiveChatOriginalMessages", "readTranslationOriginalsForArchive",
		"extractActiveChatMessageList", "getRisuActiveMessageWindowStart", "extractActiveChatComparableMessages",
		"extractComparableMessageRoleAndContent", "normalizeRollbackMessageRole", "extractMessageContentCandidate",
		"normalizeAssistantPersistenceCandidate", "canonicalizeAssistantOutputForPersistence",
		"canonicalizeAssistantTranslationDisplayForPersistence", "extractPostprocessorCanonicalAssistantText",
		"extractGigaTransCanonicalAssistantText", "extractAssistantTaggedBlocks", "removeAssistantTaggedBlocks",
		"attachTranslationDisplayCanonicalizationTrace", "attachPostprocessorCanonicalizationTrace",
		"sanitizeNarrativeOutputForDisplay", "stripHiddenReasoningEnvelopes",
		"normalizeReasoningEnvelopeName", "isReasoningEnvelopeName",
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		if strings.Contains(src, "  async function "+name+"(") {
			parts = append(parts, extractArchiveCenterJSAsyncFunction(t, src, name))
		} else {
			parts = append(parts, extractArchiveCenterJSFunction(t, src, name))
		}
	}
	return strings.Join(parts, "\n") + "\n"
}
