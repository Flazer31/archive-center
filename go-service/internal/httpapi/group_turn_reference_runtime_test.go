package httpapi

import (
	"context"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestRollbackClearsOnlyReferenceRuntimeCandidateFromDeletedTurn(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.bindings = []store.SessionReferenceBinding{
		{BindingID: "old", ChatSessionID: "session-1", Enabled: true},
		{BindingID: "deleted", ChatSessionID: "session-1", Enabled: true},
	}
	fake.runtimes["old"] = store.SessionReferenceRuntime{BindingID: "old", CandidateNodeID: "node-old", CandidateSourceTurn: 4, Revision: 1}
	fake.runtimes["deleted"] = store.SessionReferenceRuntime{BindingID: "deleted", CandidateNodeID: "node-new", CandidateSourceTurn: 7, Revision: 2}

	cleared, err := clearReferenceRuntimeCandidatesAfterRollback(context.Background(), fake, "session-1", 7)
	if err != nil || cleared != 1 {
		t.Fatalf("cleared=%d err=%v", cleared, err)
	}
	if fake.runtimes["old"].CandidateNodeID != "node-old" {
		t.Fatalf("older runtime changed: %#v", fake.runtimes["old"])
	}
	if got := fake.runtimes["deleted"]; got.CandidateNodeID != "" || got.CandidateSourceTurn != 0 || got.CandidateConfirmed {
		t.Fatalf("deleted-turn runtime was not cleared: %#v", got)
	}
	if len(fake.bindings) != 2 {
		t.Fatalf("turn rollback removed bindings: %#v", fake.bindings)
	}
}
