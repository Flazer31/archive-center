package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type psychologyHTTPStore struct {
	store.Store
	branches []store.PsychologyBranch
	saved    store.PsychologyBranch
	updated  struct {
		id         int64
		status     string
		quietTurns int
	}
}

func (f *psychologyHTTPStore) ListPsychologyBranches(ctx context.Context, chatSessionID string, limit int) ([]store.PsychologyBranch, error) {
	out := make([]store.PsychologyBranch, 0, len(f.branches))
	for _, branch := range f.branches {
		if branch.ChatSessionID == chatSessionID {
			out = append(out, branch)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *psychologyHTTPStore) SavePsychologyBranch(ctx context.Context, branch store.PsychologyBranch) (store.PsychologyBranch, error) {
	branch.ID = 55
	branch.CreatedAt = time.Date(2026, 6, 23, 2, 0, 0, 0, time.UTC)
	branch.UpdatedAt = branch.CreatedAt
	f.saved = branch
	f.branches = append(f.branches, branch)
	return branch, nil
}

func (f *psychologyHTTPStore) UpdatePsychologyBranchStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	f.updated.id = id
	f.updated.status = status
	f.updated.quietTurns = quietTurns
	return nil
}

func TestStep23PsychologyListReturnsSupportOnlyBranches(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &psychologyHTTPStore{
		Store: store.NewNoopStore(),
		branches: []store.PsychologyBranch{
			{
				ID:                     1,
				ChatSessionID:          "sess-psy",
				CharacterName:          "Mina",
				BranchType:             "fear",
				AxisName:               "fear",
				Summary:                "Mina fears that Rowan will abandon the plan.",
				Status:                 "active",
				Confidence:             0.8,
				ConfidenceLabel:        "high",
				SourceTurnStart:        3,
				SourceTurnEnd:          4,
				SourceHash:             "hash-34",
				EvidenceJSON:           `{"turns":[3,4]}`,
				DormantAfterQuietTurns: 15,
			},
			{ID: 2, ChatSessionID: "sess-psy", Status: "dormant"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/step23/psychology-branches/sess-psy?limit=10", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23PsychologyListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != step23PsychologyContractVersion {
		t.Fatalf("contract_version=%q", resp.ContractVersion)
	}
	if len(resp.Branches) != 2 || resp.Counts.Total != 2 || resp.Counts.Active != 1 || resp.Counts.Dormant != 1 {
		t.Fatalf("unexpected branches/counts: %+v", resp)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalTruthWriter || !resp.TruthBoundary.RequiresEvidence {
		t.Fatalf("truth boundary should stay support-only: %+v", resp.TruthBoundary)
	}
	if resp.TruthBoundary.MayDecideUserPersonaAction || resp.TruthBoundary.MotiveShadowHintAutoPersistAllowed {
		t.Fatalf("psychology branch must not decide persona or auto-persist motive shadows: %+v", resp.TruthBoundary)
	}
}

func TestStep23PsychologyCreatePersistsEvidenceBoundBranch(t *testing.T) {
	fake := &psychologyHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-psy",
		"character_name":"Mina",
		"branch_type":"fear",
		"summary":"Mina fears that Rowan will abandon the plan.",
		"confidence":2.0,
		"source_kind":"critic_extraction",
		"source_turn_start":3,
		"source_turn_end":4,
		"source_hash":"hash-34",
		"evidence_json":"{\"turns\":[3,4]}",
		"quiet_turns":15,
		"dormant_after_quiet_turns":15
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/psychology-branches", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23PsychologyCreateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Branch.ID != 55 || fake.saved.ChatSessionID != "sess-psy" || fake.saved.BranchType != "fear" {
		t.Fatalf("branch was not saved as expected: resp=%+v saved=%+v", resp.Branch, fake.saved)
	}
	if resp.Branch.Confidence != 1 || resp.Branch.ConfidenceLabel != "high" {
		t.Fatalf("confidence should be clamped/labeled, got %+v", resp.Branch)
	}
	if resp.Branch.Status != "dormant" {
		t.Fatalf("quiet branch should become dormant, got %q", resp.Branch.Status)
	}
}

func TestStep23PsychologyCreateRejectsMissingEvidenceAndMotiveShadowSource(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &psychologyHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/psychology-branches", strings.NewReader(`{"chat_session_id":"sess-psy","character_name":"Mina","branch_type":"fear","summary":"Mina is afraid.","source_turn_start":1,"source_turn_end":1}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing evidence status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/step23/psychology-branches", strings.NewReader(`{"chat_session_id":"sess-psy","character_name":"Mina","branch_type":"fear","summary":"Mina is afraid.","source_kind":"motive_shadow_hint","source_turn_start":1,"source_turn_end":1,"source_hash":"hash-1"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("motive shadow source status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStep23PsychologyStatusUpdatesLifecycleOnly(t *testing.T) {
	fake := &psychologyHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/step23/psychology-branches/55/status", strings.NewReader(`{"status":"dormant","quiet_turns":16}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.updated.id != 55 || fake.updated.status != "dormant" || fake.updated.quietTurns != 16 {
		t.Fatalf("unexpected update: %+v", fake.updated)
	}
}
