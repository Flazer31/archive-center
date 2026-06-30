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
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type statusSchemaHTTPStore struct {
	store.Store
	proposals     []store.StatusSchemaProposal
	definitions   []store.StatusSchemaDefinition
	currentValues []store.StatusCurrentValue
	events        []store.StatusChangeEvent
	effects       []store.StatusEffect
	saved         store.StatusSchemaProposal
	updated       struct {
		id            int64
		proposalState string
		reviewNote    string
		reviewer      string
	}
}

func (f *statusSchemaHTTPStore) ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]store.StatusSchemaProposal, error) {
	out := make([]store.StatusSchemaProposal, 0, len(f.proposals))
	for _, proposal := range f.proposals {
		if proposal.ChatSessionID != chatSessionID {
			continue
		}
		if proposalState != "" && proposal.ProposalState != proposalState {
			continue
		}
		out = append(out, proposal)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *statusSchemaHTTPStore) GetStatusSchemaProposal(ctx context.Context, id int64) (store.StatusSchemaProposal, error) {
	for _, proposal := range f.proposals {
		if proposal.ID == id {
			return proposal, nil
		}
	}
	return store.StatusSchemaProposal{}, store.ErrNotFound
}

func (f *statusSchemaHTTPStore) SaveStatusSchemaProposal(ctx context.Context, proposal store.StatusSchemaProposal) (store.StatusSchemaProposal, error) {
	proposal.ID = 24
	proposal.CreatedAt = time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC)
	proposal.UpdatedAt = proposal.CreatedAt
	f.saved = proposal
	f.proposals = append(f.proposals, proposal)
	return proposal, nil
}

func (f *statusSchemaHTTPStore) UpdateStatusSchemaProposalReview(ctx context.Context, id int64, proposalState, reviewNote, reviewer string) error {
	f.updated.id = id
	f.updated.proposalState = proposalState
	f.updated.reviewNote = reviewNote
	f.updated.reviewer = reviewer
	for idx := range f.proposals {
		if f.proposals[idx].ID == id {
			f.proposals[idx].ProposalState = proposalState
			f.proposals[idx].ReviewNote = reviewNote
			f.proposals[idx].Reviewer = reviewer
			f.proposals[idx].ReviewedAt = time.Date(2026, 6, 28, 11, 30, 0, 0, time.UTC)
			f.proposals[idx].UpdatedAt = f.proposals[idx].ReviewedAt
		}
	}
	return nil
}

func (f *statusSchemaHTTPStore) ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]store.StatusSchemaDefinition, error) {
	out := make([]store.StatusSchemaDefinition, 0, len(f.definitions))
	for _, definition := range f.definitions {
		if definition.ChatSessionID != chatSessionID {
			continue
		}
		if registryState != "" && definition.RegistryState != registryState {
			continue
		}
		out = append(out, definition)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *statusSchemaHTTPStore) SaveStatusSchemaDefinitions(ctx context.Context, definitions []store.StatusSchemaDefinition) ([]store.StatusSchemaDefinition, error) {
	out := make([]store.StatusSchemaDefinition, 0, len(definitions))
	for idx, definition := range definitions {
		definition.ID = int64(100 + idx)
		definition.CreatedAt = time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
		definition.UpdatedAt = definition.CreatedAt
		if definition.RegistryState == "" {
			definition.RegistryState = "active"
		}
		f.definitions = append(f.definitions, definition)
		out = append(out, definition)
	}
	return out, nil
}

func (f *statusSchemaHTTPStore) GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (store.StatusSchemaDefinition, error) {
	for _, definition := range f.definitions {
		if definition.ChatSessionID == chatSessionID && definition.StatusKey == statusKey && definition.OwnerScope == ownerScope && definition.RegistryState == "active" {
			return definition, nil
		}
	}
	return store.StatusSchemaDefinition{}, store.ErrNotFound
}

func (f *statusSchemaHTTPStore) ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]store.StatusCurrentValue, error) {
	out := make([]store.StatusCurrentValue, 0, len(f.currentValues))
	for _, value := range f.currentValues {
		if value.ChatSessionID != chatSessionID || value.WriteState != "current" {
			continue
		}
		if ownerScope != "" && value.OwnerScope != ownerScope {
			continue
		}
		if ownerID != "" && value.OwnerID != ownerID {
			continue
		}
		if statusKey != "" && value.StatusKey != statusKey {
			continue
		}
		out = append(out, value)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *statusSchemaHTTPStore) SaveStatusCurrentValue(ctx context.Context, value store.StatusCurrentValue) (store.StatusCurrentValue, error) {
	value.ID = int64(200 + len(f.currentValues))
	value.CreatedAt = time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC)
	value.UpdatedAt = value.CreatedAt
	if value.WriteState == "" {
		value.WriteState = "current"
	}
	f.currentValues = append(f.currentValues, value)
	return value, nil
}

func (f *statusSchemaHTTPStore) ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]store.StatusChangeEvent, error) {
	out := make([]store.StatusChangeEvent, 0, len(f.events))
	for _, event := range f.events {
		if event.ChatSessionID != chatSessionID {
			continue
		}
		if ownerScope != "" && event.OwnerScope != ownerScope {
			continue
		}
		if ownerID != "" && event.OwnerID != ownerID {
			continue
		}
		if statusKey != "" && event.StatusKey != statusKey {
			continue
		}
		out = append(out, event)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *statusSchemaHTTPStore) SaveStatusChangeEvent(ctx context.Context, event store.StatusChangeEvent) (store.StatusChangeEvent, error) {
	event.ID = int64(300 + len(f.events))
	event.CreatedAt = time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)
	if event.EventState == "" {
		event.EventState = "recorded"
	}
	f.events = append(f.events, event)
	return event, nil
}

func (f *statusSchemaHTTPStore) ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]store.StatusEffect, error) {
	out := make([]store.StatusEffect, 0, len(f.effects))
	for _, effect := range f.effects {
		if effect.ChatSessionID != chatSessionID {
			continue
		}
		if ownerScope != "" && effect.OwnerScope != ownerScope {
			continue
		}
		if ownerID != "" && effect.OwnerID != ownerID {
			continue
		}
		if effectState != "" && effect.EffectState != effectState {
			continue
		}
		out = append(out, effect)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *statusSchemaHTTPStore) SaveStatusEffect(ctx context.Context, effect store.StatusEffect) (store.StatusEffect, error) {
	effect.ID = int64(400 + len(f.effects))
	effect.CreatedAt = time.Date(2026, 6, 28, 14, 10, 0, 0, time.UTC)
	effect.UpdatedAt = effect.CreatedAt
	if effect.EffectState == "" {
		effect.EffectState = "active"
	}
	f.effects = append(f.effects, effect)
	return effect, nil
}

func (f *statusSchemaHTTPStore) UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error {
	for idx := range f.effects {
		if f.effects[idx].ID == id {
			f.effects[idx].EffectState = effectState
			f.effects[idx].ClearedEvidenceJSON = clearedEvidenceJSON
			f.effects[idx].ClearedTurn = clearedTurn
			return nil
		}
	}
	return store.ErrNotFound
}

type statusSchemaVectorStore struct {
	vector.VectorStore
	upsertSessionID string
	upsertDocs      []vector.VectorDocument
	upsertErr       error
}

func (v *statusSchemaVectorStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	v.upsertSessionID = sessionID
	v.upsertDocs = append([]vector.VectorDocument(nil), docs...)
	return v.upsertErr
}

func TestStatusSchemaListProposalOnlyBoundary(t *testing.T) {
	now := time.Date(2026, 6, 28, 11, 10, 0, 0, time.UTC)
	srv := NewServer(config.Default())
	srv.Store = &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		proposals: []store.StatusSchemaProposal{
			{ID: 1, ChatSessionID: "sess-status", InputChannel: "bootstrap", ProposalState: "pending_review", SchemaName: "core", SchemaJSON: `{"stats":[]}`, CreatedAt: now, UpdatedAt: now},
			{ID: 2, ChatSessionID: "sess-status", InputChannel: "direct_json", ProposalState: "approved", SchemaName: "user", SchemaJSON: `{"stats":[]}`, CreatedAt: now, UpdatedAt: now},
			{ID: 3, ChatSessionID: "other", InputChannel: "portable_import", ProposalState: "rejected", SchemaName: "other", SchemaJSON: `{"stats":[]}`, CreatedAt: now, UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status-schema/proposals/sess-status", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusSchemaListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != statusSchemaContractVersion || len(resp.Proposals) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Counts.PendingReview != 1 || resp.Counts.Approved != 1 || resp.Counts.Total != 2 {
		t.Fatalf("unexpected counts: %+v", resp.Counts)
	}
	if !resp.TruthBoundary.ProposalOnly || resp.TruthBoundary.CanonicalStatusWriter || resp.TruthBoundary.ApprovalRegistersSchema || resp.TruthBoundary.CurrentValueWritesAllowed {
		t.Fatalf("unexpected truth boundary: %+v", resp.TruthBoundary)
	}
}

func TestStatusSchemaCreateForcesPendingReviewAndImportProvenance(t *testing.T) {
	fake := &statusSchemaHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-status",
		"input_channel":"portable_import",
		"schema_name":"portable-core",
		"ruleset_label":"Imported",
		"schema_json":{"stats":[{"key":"custom_metric","kind":"number"}]},
		"provenance_json":{"source":"user_import","format":"archive-center-status-schema"}
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/proposals", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.saved.InputChannel != "portable_import" || fake.saved.ProposalState != "pending_review" {
		t.Fatalf("proposal should remain pending review: %+v", fake.saved)
	}
	if !strings.Contains(fake.saved.SchemaJSON, `"stats"`) || !strings.Contains(fake.saved.ProvenanceJSON, `"user_import"`) {
		t.Fatalf("schema/provenance not compacted into proposal: %+v", fake.saved)
	}
}

func TestStatusSchemaCreateIndexesChromaSupportDocument(t *testing.T) {
	fake := &statusSchemaHTTPStore{Store: store.NewNoopStore()}
	vec := &statusSchemaVectorStore{}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://chromadb.test"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-status",
		"input_channel":"bootstrap",
		"schema_name":"combat-core",
		"ruleset_label":"Combat",
		"schema_json":{"stats":[{"key":"hp","kind":"resource","bounds":{"min":0,"max":100}}]},
		"vector_embedding":[0.1,0.2,0.3]
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/proposals", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if vec.upsertSessionID != "sess-status" || len(vec.upsertDocs) != 1 {
		t.Fatalf("vector upsert not called for status schema proposal: session=%q docs=%+v", vec.upsertSessionID, vec.upsertDocs)
	}
	doc := vec.upsertDocs[0]
	if doc.Tier != "status_schema_proposal" || doc.SourceTable != "status_schema_proposals" || doc.SourceRowID != "24" {
		t.Fatalf("status schema vector doc must hydrate to proposal row: %+v", doc)
	}
	if !strings.Contains(doc.DocumentText, `"key":"hp"`) || !strings.Contains(doc.DocumentText, "proposal_state: pending_review") {
		t.Fatalf("vector document text lost schema/review context: %q", doc.DocumentText)
	}
	var resp statusSchemaCreateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.VectorIndex.Status != "ok" || resp.VectorIndex.Tier != "status_schema_proposal" || resp.VectorIndex.SourceTable != "status_schema_proposals" {
		t.Fatalf("unexpected vector index trace: %+v", resp.VectorIndex)
	}
	if !resp.VectorPolicy.HydrateRequired || resp.VectorPolicy.TruthWriter {
		t.Fatalf("vector policy must stay support-only with hydration required: %+v", resp.VectorPolicy)
	}
}

func TestStatusSchemaCreateRejectsInvalidProposalInput(t *testing.T) {
	fake := &statusSchemaHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/proposals", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"input_channel":"portable_import",
		"schema_name":"portable-core",
		"schema_json":{"stats":[]}
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing provenance status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/proposals", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"input_channel":"direct_json",
		"schema_name":"bad-core",
		"schema_json":[]
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-object schema status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.saved.ID != 0 {
		t.Fatalf("invalid proposal should be rejected before store save: %+v", fake.saved)
	}
}

func TestStatusSchemaReviewUpdatesReviewStateOnly(t *testing.T) {
	fake := &statusSchemaHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/status-schema/proposals/24/review", strings.NewReader(`{"proposal_state":"pending_review"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("pending review status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/status-schema/proposals/24/review", strings.NewReader(`{"proposal_state":"approved","review_note":"ready for registry import","reviewer":"user"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.updated.id != 24 || fake.updated.proposalState != "approved" || fake.updated.reviewNote == "" || fake.updated.reviewer != "user" {
		t.Fatalf("unexpected review update: %+v", fake.updated)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	boundary, ok := resp["truth_boundary"].(map[string]any)
	if !ok || boundary["approval_registers_schema"] != false || boundary["current_value_writes_allowed"] != false {
		t.Fatalf("review response must remain proposal-only: %+v", resp)
	}
}

func TestStatusSchemaReviewReindexesChromaSupportDocument(t *testing.T) {
	now := time.Date(2026, 6, 28, 11, 10, 0, 0, time.UTC)
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		proposals: []store.StatusSchemaProposal{
			{
				ID:             24,
				ChatSessionID:  "sess-status",
				InputChannel:   "direct_json",
				ProposalState:  "pending_review",
				SchemaName:     "combat-core",
				SchemaJSON:     `{"stats":[{"key":"hp","kind":"resource"}]}`,
				ProvenanceJSON: `{"source":"settings"}`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	vec := &statusSchemaVectorStore{}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://chromadb.test"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/status-schema/proposals/24/review", strings.NewReader(`{
		"proposal_state":"approved",
		"review_note":"ready for registry import",
		"reviewer":"user",
		"vector_embedding":[0.4,0.5,0.6]
	}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(vec.upsertDocs) != 1 {
		t.Fatalf("review should reindex one status schema support doc: %+v", vec.upsertDocs)
	}
	doc := vec.upsertDocs[0]
	if doc.ID != "status_schema_proposal:sess-status:24" || doc.SourceTable != "status_schema_proposals" || doc.SourceRowID != "24" {
		t.Fatalf("review vector doc lost hydration pointer: %+v", doc)
	}
	if !strings.Contains(doc.DocumentText, "proposal_state: approved") || !strings.Contains(doc.DocumentText, "ready for registry import") {
		t.Fatalf("review vector doc did not reflect latest MariaDB proposal row: %q", doc.DocumentText)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	vectorIndex, ok := resp["vector_index"].(map[string]any)
	if !ok || vectorIndex["status"] != "ok" || vectorIndex["source_table"] != "status_schema_proposals" {
		t.Fatalf("unexpected review vector trace: %+v", resp)
	}
}

func TestStatusSchemaRegistryImportRequiresApprovedProposalAndRegistersDefinitions(t *testing.T) {
	now := time.Date(2026, 6, 28, 11, 10, 0, 0, time.UTC)
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		proposals: []store.StatusSchemaProposal{
			{
				ID:            24,
				ChatSessionID: "sess-status",
				InputChannel:  "bootstrap",
				ProposalState: "approved",
				SchemaName:    "combat-core",
				RulesetLabel:  "Combat",
				SchemaJSON: `{"stats":[
					{"status_key":"hp","label":"Health","owner_scope":"character","value_kind":"resource","bounds":{"min":0,"max":100},"default_value":100},
					{"status_key":"poisoned","label":"Poisoned","owner_scope":"character","value_kind":"boolean","default_value":false}
				]}`,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/registry/from-proposal/24", strings.NewReader(`{}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.definitions) != 2 {
		t.Fatalf("definitions saved=%d want 2: %+v", len(fake.definitions), fake.definitions)
	}
	if fake.definitions[0].StatusKey != "hp" || fake.definitions[0].OwnerScope != "character" || fake.definitions[0].ValueKind != "resource" {
		t.Fatalf("definition did not preserve schema structure: %+v", fake.definitions[0])
	}
	if fake.definitions[0].BoundsJSON == "" || fake.definitions[0].DefaultValueJSON != "100" {
		t.Fatalf("definition lost bounds/default: %+v", fake.definitions[0])
	}
	var resp statusSchemaRegistryImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != statusSchemaRegistryContractVersion || len(resp.Definitions) != 2 {
		t.Fatalf("unexpected registry import response: %+v", resp)
	}
	if !resp.RegistryPolicy.CanonicalSchemaRegistry || resp.RegistryPolicy.CurrentValueWritesAllowed || resp.RegistryPolicy.HardcodedStatusNamesAllowed {
		t.Fatalf("registry policy must be canonical schema only: %+v", resp.RegistryPolicy)
	}
}

func TestStatusSchemaRegistryImportRejectsUnapprovedOrExecutableSchema(t *testing.T) {
	now := time.Date(2026, 6, 28, 11, 10, 0, 0, time.UTC)
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		proposals: []store.StatusSchemaProposal{
			{ID: 24, ChatSessionID: "sess-status", ProposalState: "pending_review", SchemaName: "pending", SchemaJSON: `{"stats":[{"status_key":"hp","owner_scope":"character","value_kind":"resource"}]}`, CreatedAt: now, UpdatedAt: now},
			{ID: 25, ChatSessionID: "sess-status", ProposalState: "approved", SchemaName: "bad", SchemaJSON: `{"stats":[{"status_key":"hp","owner_scope":"character","value_kind":"resource","formula":"hp+1"}]}`, CreatedAt: now, UpdatedAt: now},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/registry/from-proposal/24", strings.NewReader(`{}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("pending proposal status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/registry/from-proposal/25", strings.NewReader(`{}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("executable schema status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.definitions) != 0 {
		t.Fatalf("invalid proposal should not save definitions: %+v", fake.definitions)
	}
}

func TestStatusSchemaRegistryImportIndexesDefinitionsAsChromaSupportDocs(t *testing.T) {
	now := time.Date(2026, 6, 28, 11, 10, 0, 0, time.UTC)
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		proposals: []store.StatusSchemaProposal{
			{
				ID:            24,
				ChatSessionID: "sess-status",
				ProposalState: "approved",
				SchemaName:    "combat-core",
				SchemaJSON:    `{"stats":[{"status_key":"hp","label":"Health","owner_scope":"character","value_kind":"resource","bounds":{"min":0,"max":100}}]}`,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}
	vec := &statusSchemaVectorStore{}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://chromadb.test"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/registry/from-proposal/24", strings.NewReader(`{"vector_embedding":[0.7,0.8,0.9]}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(vec.upsertDocs) != 1 {
		t.Fatalf("registry import should index one definition doc: %+v", vec.upsertDocs)
	}
	doc := vec.upsertDocs[0]
	if doc.Tier != "status_schema_definition" || doc.SourceTable != "status_schema_registry" || doc.SourceRowID != "100" {
		t.Fatalf("definition vector doc must hydrate to registry row: %+v", doc)
	}
	if !strings.Contains(doc.DocumentText, "status_key: hp") || !strings.Contains(doc.DocumentText, "bounds_json:") {
		t.Fatalf("definition vector doc lost full schema structure: %q", doc.DocumentText)
	}
	var resp statusSchemaRegistryImportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.VectorIndexes) != 1 || resp.VectorIndexes[0].Status != "ok" || resp.VectorIndexes[0].SourceTable != "status_schema_registry" {
		t.Fatalf("unexpected vector index trace: %+v", resp.VectorIndexes)
	}
}

func TestStatusSchemaRegistryListReturnsSessionDefinitions(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 1, ChatSessionID: "sess-status", StatusKey: "hp", Label: "Health", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
			{ID: 2, ChatSessionID: "other", StatusKey: "san", Label: "Sanity", OwnerScope: "character", ValueKind: "scalar", RegistryState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status-schema/registry/sess-status", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusSchemaRegistryListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Definitions) != 1 || resp.Definitions[0].StatusKey != "hp" || resp.Counts["total"] != 1 {
		t.Fatalf("unexpected registry list response: %+v", resp)
	}
}

func TestStatusCurrentValueWriteRequiresActiveRegistryAndEvidence(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "Health", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/values", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"mana",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"value_json":75,
		"evidence_json":{"turn":2}
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unregistered status write status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/values", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"value_json":75
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "evidence_json") {
		t.Fatalf("missing evidence status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatusCurrentValueWritePersistsEvidenceBoundCurrentValue(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "Health", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/values", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"owner_label":"이시우",
		"value_json":75,
		"evidence_json":{"source":"direct_evidence","turn":2},
		"source_turn":2
	}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusCurrentValueWriteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != statusSchemaValueContractVersion || !resp.Policy.CanonicalCurrentValueWriter || resp.Policy.VectorTruthWriter {
		t.Fatalf("unexpected value policy: %+v", resp)
	}
	if resp.Value.RegistryID != 100 || resp.Value.StatusKey != "hp" || resp.Value.OwnerID != "siwoo" || resp.Value.EvidenceJSON == "" {
		t.Fatalf("unexpected current value: %+v", resp.Value)
	}
}

func TestStatusCurrentValueListFiltersCurrentValues(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		currentValues: []store.StatusCurrentValue{
			{ID: 200, ChatSessionID: "sess-status", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "resource", ValueJSON: "75", EvidenceJSON: `{"turn":2}`, WriteState: "current"},
			{ID: 201, ChatSessionID: "sess-status", RegistryID: 101, StatusKey: "poisoned", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "boolean", ValueJSON: "false", EvidenceJSON: `{"turn":2}`, WriteState: "current"},
			{ID: 202, ChatSessionID: "other", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "resource", ValueJSON: "10", EvidenceJSON: `{"turn":1}`, WriteState: "current"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status-schema/values/sess-status?owner_scope=character&owner_id=siwoo&status_key=hp", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusCurrentValueListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Values) != 1 || resp.Values[0].StatusKey != "hp" || resp.Counts["total"] != 1 {
		t.Fatalf("unexpected current value list: %+v", resp)
	}
}

func TestStatusChangeEventWriteRecordsLedgerWithoutMutatingCurrentValue(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "Health", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
		currentValues: []store.StatusCurrentValue{
			{ID: 200, ChatSessionID: "sess-status", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "resource", ValueJSON: "75", EvidenceJSON: `{"turn":2}`, WriteState: "current"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/events", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_value_id":200,
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"event_kind":"decrease",
		"previous_value_json":75,
		"new_value_json":63,
		"evidence_json":{"source":"combat","turn":3},
		"source_turn":3,
		"story_clock_json":{"day":2,"phase":"night","precision_label":"scene"}
	}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusChangeEventWriteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != statusSchemaLifecycleContractVersion || resp.Policy.CurrentValueMutationAllowed {
		t.Fatalf("unexpected lifecycle policy: %+v", resp.Policy)
	}
	if resp.Event.RegistryID != 100 || resp.Event.EventKind != "decrease" || resp.Event.NewValueJSON != "63" {
		t.Fatalf("unexpected event: %+v", resp.Event)
	}
	if len(fake.currentValues) != 1 || fake.currentValues[0].ValueJSON != "75" {
		t.Fatalf("event ledger must not mutate current values: %+v", fake.currentValues)
	}
}

func TestStatusEffectWriteRequiresStoryClockDurationBoundary(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "Health", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/effects", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"effect_kind":"debuff",
		"evidence_json":{"turn":3},
		"duration_json":{"turns":2}
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "start_clock_json") {
		t.Fatalf("missing clock status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/effects", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"effect_kind":"debuff",
		"effect_label":"bleeding",
		"evidence_json":{"turn":3},
		"start_clock_json":{"day":2,"phase":"night","precision_label":"scene"}
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "duration_json") {
		t.Fatalf("missing duration status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStatusEffectLifecycleWriteListAndClear(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "Health", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/effects", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"effect_kind":"cooldown",
		"effect_label":"dash cooldown",
		"effect_payload_json":{"skill":"dash"},
		"evidence_json":{"turn":4},
		"source_turn":4,
		"start_clock_json":{"day":2,"phase":"night","precision_label":"scene"},
		"duration_json":{"turns":2},
		"effect_state":"active"
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var writeResp statusEffectWriteResponse
	if err := json.NewDecoder(rec.Body).Decode(&writeResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if writeResp.Effect.ID != 400 || writeResp.Effect.EffectKind != "cooldown" || writeResp.Effect.DurationJSON == "" {
		t.Fatalf("unexpected effect: %+v", writeResp.Effect)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/status-schema/effects/400/state", strings.NewReader(`{
		"effect_state":"cleared",
		"cleared_evidence_json":{"turn":5,"reason":"cooldown completed"},
		"cleared_turn":5
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/status-schema/effects/sess-status?effect_state=cleared", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var listResp statusEffectListResponse
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Effects) != 1 || listResp.Effects[0].EffectState != "cleared" || listResp.Counts["cleared"] != 1 {
		t.Fatalf("unexpected effect list: %+v", listResp)
	}
}

func TestStatusQueryUsesCanonicalCurrentValueBeforeMemoryFallback(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "체력", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
		currentValues: []store.StatusCurrentValue{
			{ID: 200, ChatSessionID: "sess-status", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "resource", ValueJSON: "75", EvidenceJSON: `{"turn":4}`, WriteState: "current"},
		},
		effects: []store.StatusEffect{
			{ID: 400, ChatSessionID: "sess-status", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", EffectKind: "injury", EffectLabel: "bleeding", EvidenceJSON: `{"turn":4}`, EffectState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"projection_density":"full"
	}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusQueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ResultState != "answered" || !resp.Policy.CanonFirstQuery || resp.Policy.SemanticMemoryFallbackAsTruth {
		t.Fatalf("unexpected query policy/state: %+v", resp)
	}
	if len(resp.Projection) != 1 {
		t.Fatalf("projection len=%d want 1: %+v", len(resp.Projection), resp.Projection)
	}
	item := resp.Projection[0]
	if item.ValueSource != "archive_current" || item.AuthorityMode != "archive_canonical" || item.Value == nil || item.Value.ValueJSON != "75" {
		t.Fatalf("canonical current value not projected: %+v", item)
	}
	if len(item.Effects) != 1 || item.Effects[0].EffectLabel != "bleeding" {
		t.Fatalf("active effect not projected with full density: %+v", item.Effects)
	}
}

func TestStatusQueryExternalRuntimeAuthorityDoesNotUseArchiveValue(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "체력", OwnerScope: "character", ValueKind: "resource", OptionsJSON: `{"runtime_authority":"lua"}`, RegistryState: "active"},
		},
		currentValues: []store.StatusCurrentValue{
			{ID: 200, ChatSessionID: "sess-status", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "resource", ValueJSON: "75", EvidenceJSON: `{"turn":4}`, WriteState: "current"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo",
		"external_values":[{"status_key":"hp","owner_scope":"character","owner_id":"siwoo","value_json":61,"runtime_name":"risu_lua"}]
	}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusQueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ResultState != "answered" || !resp.Policy.ExternalRuntimeAuthoritySupported || !resp.Policy.ExternalRuntimeOverridesArchive {
		t.Fatalf("unexpected external runtime policy/state: %+v", resp)
	}
	if len(resp.Projection) != 1 {
		t.Fatalf("projection len=%d want 1: %+v", len(resp.Projection), resp.Projection)
	}
	item := resp.Projection[0]
	if item.AuthorityMode != "external_runtime" || item.ValueSource != "external_runtime" || item.Value != nil {
		t.Fatalf("archive value should not be used for external runtime authority: %+v", item)
	}
	if item.ExternalRuntime == nil || item.ExternalRuntime.ValueJSON != "61" || item.ExternalRuntime.RuntimeName != "risu_lua" {
		t.Fatalf("external runtime value not projected: %+v", item.ExternalRuntime)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"hp",
		"owner_scope":"character",
		"owner_id":"siwoo"
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("missing external status=%d body=%s", rec.Code, rec.Body.String())
	}
	resp = statusQueryResponse{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode missing external: %v", err)
	}
	if resp.ResultState != "external_value_required" || len(resp.Projection) != 1 || resp.Projection[0].ValueSource != "external_value_required" {
		t.Fatalf("missing external value must not fall back to archive current: %+v", resp)
	}
}

func TestStatusQueryUnknownStatusReturnsProposalGateOnly(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "체력", OwnerScope: "character", ValueKind: "resource", RegistryState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-status",
		"status_key":"mana",
		"owner_scope":"character",
		"owner_id":"siwoo"
	}`))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusQueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ResultState != "proposal_required" || !resp.ProposalGate.Required || !resp.ProposalGate.ProposalOnly || resp.ProposalGate.AutoCanonWriteAllowed {
		t.Fatalf("unknown status must be proposal-only: %+v", resp)
	}
	if len(resp.Definitions) != 0 || len(resp.Projection) != 0 || !resp.Policy.UnknownStatusProposalOnly || resp.Policy.UnknownStatusCreatesCanon {
		t.Fatalf("unknown status should not create canonical projection: %+v", resp)
	}
}

func TestStatusProjectionAutoDensityKeepsLowPriorityLight(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-status", StatusKey: "hp", Label: "체력", OwnerScope: "character", ValueKind: "resource", OptionsJSON: `{"critical":true}`, RegistryState: "active"},
			{ID: 101, ChatSessionID: "sess-status", StatusKey: "mood", Label: "기분", OwnerScope: "character", ValueKind: "enum", RegistryState: "active"},
		},
		currentValues: []store.StatusCurrentValue{
			{ID: 200, ChatSessionID: "sess-status", RegistryID: 100, StatusKey: "hp", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "resource", ValueJSON: "75", EvidenceJSON: `{"turn":4}`, WriteState: "current"},
			{ID: 201, ChatSessionID: "sess-status", RegistryID: 101, StatusKey: "mood", OwnerScope: "character", OwnerID: "siwoo", ValueKind: "enum", ValueJSON: `"calm"`, EvidenceJSON: `{"turn":4}`, WriteState: "current"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status-schema/projection/sess-status?owner_scope=character&owner_id=siwoo&density=auto", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusProjectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Projection) != 2 || resp.Counts["density:full"] != 1 || resp.Counts["density:light"] != 1 {
		t.Fatalf("unexpected projection density counts: %+v", resp)
	}
	byKey := map[string]statusProjectionItem{}
	for _, item := range resp.Projection {
		byKey[item.Definition.StatusKey] = item
	}
	if byKey["hp"].Density != "full" || byKey["hp"].Value == nil || byKey["hp"].Value.ValueJSON != "75" {
		t.Fatalf("critical status should use full projection: %+v", byKey["hp"])
	}
	if byKey["mood"].Density != "light" || byKey["mood"].Value != nil || !strings.Contains(byKey["mood"].ProjectionText, "available") {
		t.Fatalf("low-priority carryover should stay light: %+v", byKey["mood"])
	}
}

func TestStep24ValidationGateCustomSchemaNoHardcodedStatusNames(t *testing.T) {
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
		definitions: []store.StatusSchemaDefinition{
			{ID: 100, ChatSessionID: "sess-step24", StatusKey: "custom_focus", Label: "Custom Focus", OwnerScope: "character", ValueKind: "enum", OptionsJSON: `{"critical":true}`, RegistryState: "active"},
			{ID: 101, ChatSessionID: "sess-step24", StatusKey: "custom_pressure", Label: "Custom Pressure", OwnerScope: "character", ValueKind: "note", RegistryState: "active"},
			{ID: 102, ChatSessionID: "sess-step24", StatusKey: "lua_heat", Label: "Lua Heat", OwnerScope: "character", ValueKind: "scalar", OptionsJSON: `{"runtime_authority":"lua"}`, RegistryState: "active"},
		},
		currentValues: []store.StatusCurrentValue{
			{ID: 200, ChatSessionID: "sess-step24", RegistryID: 100, StatusKey: "custom_focus", OwnerScope: "character", OwnerID: "actor-a", ValueKind: "enum", ValueJSON: `"locked_on_gate"`, EvidenceJSON: `{"turn":7}`, WriteState: "current"},
			{ID: 201, ChatSessionID: "sess-step24", RegistryID: 101, StatusKey: "custom_pressure", OwnerScope: "character", OwnerID: "actor-a", ValueKind: "note", ValueJSON: `"low carryover only"`, EvidenceJSON: `{"turn":7}`, WriteState: "current"},
			{ID: 202, ChatSessionID: "sess-step24", RegistryID: 102, StatusKey: "lua_heat", OwnerScope: "character", OwnerID: "actor-a", ValueKind: "scalar", ValueJSON: "42", EvidenceJSON: `{"turn":7}`, WriteState: "current"},
		},
		effects: []store.StatusEffect{
			{ID: 300, ChatSessionID: "sess-step24", RegistryID: 100, StatusKey: "custom_focus", OwnerScope: "character", OwnerID: "actor-a", EffectKind: "buff", EffectLabel: "focus lock", EvidenceJSON: `{"turn":7}`, StartClockJSON: `{"turn":7}`, DurationJSON: `{"turns":1}`, EffectState: "active"},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-step24",
		"candidate_status_keys":["custom_focus"],
		"owner_scope":"character",
		"owner_id":"actor-a",
		"projection_density":"auto"
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("custom schema query status=%d body=%s", rec.Code, rec.Body.String())
	}
	var queryResp statusQueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&queryResp); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if queryResp.ResultState != "answered" || len(queryResp.Projection) != 1 {
		t.Fatalf("custom status should be answered from registry/current value: %+v", queryResp)
	}
	focus := queryResp.Projection[0]
	if focus.Definition.StatusKey != "custom_focus" || focus.Value == nil || focus.Value.ValueJSON != `"locked_on_gate"` || focus.Density != "full" || len(focus.Effects) != 1 {
		t.Fatalf("custom status projection lost canonical value/effects: %+v", focus)
	}
	if queryResp.Policy.SemanticMemoryFallbackAsTruth || queryResp.Policy.VectorTruthWriter || queryResp.Policy.UnknownStatusCreatesCanon {
		t.Fatalf("query policy must keep memory/vector support-only: %+v", queryResp.Policy)
	}

	beforeValues := len(fake.currentValues)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-step24",
		"status_key":"unregistered_custom_status",
		"owner_scope":"character",
		"owner_id":"actor-a"
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unknown status query status=%d body=%s", rec.Code, rec.Body.String())
	}
	queryResp = statusQueryResponse{}
	if err := json.NewDecoder(rec.Body).Decode(&queryResp); err != nil {
		t.Fatalf("decode unknown query: %v", err)
	}
	if queryResp.ResultState != "proposal_required" || !queryResp.ProposalGate.ProposalOnly || queryResp.ProposalGate.AutoCanonWriteAllowed || len(fake.currentValues) != beforeValues {
		t.Fatalf("unknown status must stay proposal-only without canon writes: %+v values=%d", queryResp, len(fake.currentValues))
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-step24",
		"status_key":"lua_heat",
		"owner_scope":"character",
		"owner_id":"actor-a"
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("external missing status=%d body=%s", rec.Code, rec.Body.String())
	}
	queryResp = statusQueryResponse{}
	if err := json.NewDecoder(rec.Body).Decode(&queryResp); err != nil {
		t.Fatalf("decode missing external: %v", err)
	}
	if queryResp.ResultState != "external_value_required" || len(queryResp.Projection) != 1 || queryResp.Projection[0].Value != nil || queryResp.Projection[0].ValueSource != "external_value_required" {
		t.Fatalf("lua-authoritative status must not use archive fallback: %+v", queryResp)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/status-schema/query", strings.NewReader(`{
		"chat_session_id":"sess-step24",
		"status_key":"lua_heat",
		"owner_scope":"character",
		"owner_id":"actor-a",
		"external_values":[{"status_key":"lua_heat","owner_scope":"character","owner_id":"actor-a","value_json":108,"runtime_name":"risu_lua"}]
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("external supplied status=%d body=%s", rec.Code, rec.Body.String())
	}
	queryResp = statusQueryResponse{}
	if err := json.NewDecoder(rec.Body).Decode(&queryResp); err != nil {
		t.Fatalf("decode supplied external: %v", err)
	}
	if queryResp.ResultState != "answered" || len(queryResp.Projection) != 1 || queryResp.Projection[0].ValueSource != "external_runtime" || queryResp.Projection[0].ExternalRuntime == nil || queryResp.Projection[0].ExternalRuntime.ValueJSON != "108" {
		t.Fatalf("lua-authoritative supplied value should project from external runtime: %+v", queryResp)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/status-schema/projection/sess-step24?owner_scope=character&owner_id=actor-a&density=auto", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("projection status=%d body=%s", rec.Code, rec.Body.String())
	}
	var projectionResp statusProjectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&projectionResp); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	if projectionResp.Counts["density:full"] != 1 || projectionResp.Counts["density:light"] != 1 {
		t.Fatalf("auto density should keep critical full and carryover light: %+v", projectionResp)
	}
	if projectionResp.Policy.SemanticMemoryFallbackAsTruth || projectionResp.Policy.VectorTruthWriter {
		t.Fatalf("projection policy must not let vector/memory become truth: %+v", projectionResp.Policy)
	}
}

func TestStep24ValidationGateChromaSupportPolicyIsVisible(t *testing.T) {
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	fake := &statusSchemaHTTPStore{
		Store: store.NewNoopStore(),
	}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/status-schema/proposals", strings.NewReader(`{
		"chat_session_id":"sess-step24-vector",
		"input_channel":"bootstrap",
		"schema_name":"vector schema",
		"schema_json":{"stats":[{"status_key":"custom_focus","label":"Focus","owner_scope":"character","value_kind":"enum"}]},
		"provenance_json":{"source":"test"},
		"vector_embedding":[0.1,0.2,0.3]
	}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("proposal create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp statusSchemaCreateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if resp.VectorIndex.Status != "ok" || resp.VectorIndex.SourceTable != "status_schema_proposals" || resp.VectorIndex.SourceRowID != "24" {
		t.Fatalf("proposal vector index should expose Chroma row pointer: %+v", resp.VectorIndex)
	}
	if !resp.VectorPolicy.ChromaLinked || !resp.VectorPolicy.HydrateRequired || resp.VectorPolicy.TruthWriter {
		t.Fatalf("proposal vector policy must be visible and support-only: %+v", resp.VectorPolicy)
	}
}
