package store

import "testing"

func TestCanonFactIdentityDeduplicatesExactTextButPreservesScopeAndMeaningDifferences(t *testing.T) {
	_, base := canonFactIdentity("edition-1", "continuity-1", "fact", "Person guards the archive.")
	_, normalized := canonFactIdentity("edition-1", "continuity-1", " FACT ", "  person   guards the archive. ")
	_, punctuationChanged := canonFactIdentity("edition-1", "continuity-1", "fact", "Person guards the archive!")
	_, otherEdition := canonFactIdentity("edition-2", "continuity-1", "fact", "Person guards the archive.")
	_, otherContinuity := canonFactIdentity("edition-1", "continuity-2", "fact", "Person guards the archive.")
	if base != normalized {
		t.Fatal("case and whitespace normalization did not deduplicate an exact fact")
	}
	if base == punctuationChanged || base == otherEdition || base == otherContinuity {
		t.Fatal("near text or scope-distinct facts were merged")
	}
}
