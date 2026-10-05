package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestReindexCommittedScopeHTTPMariaDB(t *testing.T) {
	chromaEndpoint := os.Getenv("AC_FEEDBACK_TEST_CHROMA")
	if chromaEndpoint == "" {
		t.Skip("requires disposable Chroma collection")
	}
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	db, st := feedback43Database(t)
	provider := &storyTime46Provider{}
	upstream := httptest.NewServer(provider)
	t.Cleanup(upstream.Close)
	endpoint := upstream.URL + "/v1"
	server := httpapi.NewServer(config.Default())
	server.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	server.Cfg.ChromaEndpoint = chromaEndpoint
	live, err := vector.NewChromaStore(chromaEndpoint, fmt.Sprintf("reindex_scope_%d", time.Now().UnixNano()), "/api/v2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := live.(vector.CollectionResetter).ResetAll(context.Background()); err != nil {
			t.Error(err)
		}
	})
	server.Store, server.Vector = st, live
	server.RuntimeConfig = httpapi.RuntimeConfig{Synced: true, CriticTimeoutSec: 10, LLMRetryCount: 1, EmbeddingProvider: "openai", EmbeddingAPIKey: "disposable-local-test", EmbeddingEndpoint: endpoint + "/embeddings", EmbeddingModel: "local-test-embedding", EmbeddingTimeoutSec: 30}
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	const sid = "reindex-committed-scope"
	const sentence = "Zara tells Ari that the hidden password is silver."
	extraction := map[string]any{
		"turn_summary": sentence, "importance_score": 7,
		"evidence_excerpts": []any{sentence},
		"protected_secrets": []any{map[string]any{
			"secret_kind": "plan", "owner": "Zara", "subject": "Zara",
			"summary": "The hidden password is silver.", "evidence_excerpt": sentence,
			"disclosure_policy": "owner_private_until_revealed",
			"knowledge_scope":   map[string]any{"known_by": []any{"Zara", "Ari"}, "unknown_to": []any{"Noel"}},
		}},
	}
	storyTime46Complete(t, routes, provider, endpoint, sid, 1, 1, sentence, extraction)
	var revision, original, originalHash, user, assistant string
	if err := db.QueryRow(`SELECT source_revision,derived_result_json,derived_result_hash,raw_user_content,raw_assistant_content FROM memory_source_revisions WHERE chat_session_id=? AND lifecycle_state='active'`, sid).Scan(&revision, &original, &originalHash, &user, &assistant); err != nil {
		t.Fatal(err)
	}
	counts := func() []int {
		values := []int{}
		for _, table := range []string{"memories", "direct_evidence_records", "precise_memory_units", "storylines", "pending_threads"} {
			var n int
			if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE chat_session_id=?", sid).Scan(&n); err != nil {
				t.Fatal(err)
			}
			values = append(values, n)
		}
		return values
	}
	before := fmt.Sprint(counts())
	workerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server.StartMemoryWorkers(workerCtx)
	provider.mu.Lock()
	callsBefore := provider.calls
	provider.mu.Unlock()
	for replay := 0; replay < 2; replay++ {
		// Wait for the real background vector delivery before refreshing the
		// same outbox rows again; an active delivery lease is not a hash fault.
		deadline := time.Now().Add(10 * time.Second)
		for {
			var pending int
			if err := db.QueryRow(`SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id=? AND status NOT IN ('completed','stale_rejected')`, sid).Scan(&pending); err != nil {
				t.Fatal(err)
			}
			if pending == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("vector delivery did not settle before replay")
			}
			time.Sleep(10 * time.Millisecond)
		}
		result := storyTime46Request(t, routes, http.MethodPost, "/admin/reindex", map[string]any{"chat_session_id": sid, "force": true, "client_meta": map[string]any{"embedding": map[string]any{"provider": "openai", "api_key": "disposable-local-test", "endpoint": endpoint + "/embeddings", "model": "local-test-embedding"}}})
		if result["canonical_replays_completed"] != float64(1) || len(result["errors"].([]any)) != 0 {
			var activeLeases int
			db.QueryRow(`SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id=? AND status='leased' AND lease_until>UTC_TIMESTAMP(6)`, sid).Scan(&activeLeases)
			t.Fatalf("committed reindex failed: completed=%v errors=%v actual_active_leases=%d", result["canonical_replays_completed"], result["errors"], activeLeases)
		}
		current, err := st.(archiveStore.SourceRevisionStore).GetSourceRevision(context.Background(), sid, revision)
		if err != nil {
			t.Fatal(err)
		}
		if current.DerivedResultJSON != original || current.DerivedResultHash != originalHash || current.UserContent != user || current.AssistantContent != assistant {
			t.Fatal("reindex changed committed extraction, scope, hash or original chat")
		}
		if fmt.Sprint(counts()) != before {
			t.Fatal("reindex duplicated or removed saved rows")
		}
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.calls != callsBefore {
		t.Fatal("reindex called Critic instead of replaying the snapshot")
	}
}

func TestManualTrustBrowserHTTPMariaDBLifecycle(t *testing.T) {
	script := os.Getenv("AC_BROWSER_E2E_SCRIPT")
	if script == "" {
		t.Skip("AC_BROWSER_E2E_SCRIPT not configured")
	}
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	ctx := context.Background()
	const sid, key = "manual-trust-browser", "shield-handover-browser"
	rule := map[string]any{"scope": "location", "scope_name": "Library", "category": "access", "key": "cellar_needs_key", "value": "The cellar needs a brass key."}
	initial := map[string]any{"turn_summary": "The artisan promises a shield; the library cellar needs a brass key.", "importance_score": 7, "world_rules": []any{rule}, "pending_threads": []any{map[string]any{"title": "Collect the shield", "description": "Collect the engraved shield from the workshop.", "lifecycle_key": key, "status": "open", "confidence": .9}}}
	storyTime46Complete(t, routes, provider, endpoint, sid, 1, 1, "At the library, the artisan promises to finish and deliver the engraved shield. The cellar needs a brass key.", initial)
	if err := st.(characterStateWriter46).SaveCharacterState(ctx, &archiveStore.CharacterState{ChatSessionID: sid, CharacterName: "Mira", TurnIndex: 1, AppearanceJSON: `{"hair":"brown"}`}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveKGTriple(ctx, &archiveStore.KGTriple{ChatSessionID: sid, Subject: "Mira", Predicate: "owns", Object: "Brass key", SourceTurn: 1, ValidFrom: 1, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/__test/ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><style>body{background:#101319;color:#eee;font:15px sans-serif;padding:24px}.mo-trust-row,.mo-ent-card{padding:12px;border:1px solid #45475b;margin:12px 0}.mo-trust-badge{margin-left:12px;color:#c4caff}button,input,textarea{margin:4px;padding:8px}.mo-ent-subtab-active,.mo-trust-btn-on{background:#6774cf;color:white}</style><main id="panel"></main>`)
	})
	for _, step := range []string{"next", "reroll", "replay"} {
		mux.HandleFunc("/__test/"+step, func(w http.ResponseWriter, r *http.Request) {
			if step == "next" {
				text := "The shield engraving is finished but the handle remains unfinished."
				x := map[string]any{"turn_summary": text, "importance_score": 7, "world_rules": []any{rule}, "state_claims": []any{map[string]any{"subject": "Shield collection", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": key, "value": text, "transition": "partial", "confidence": .9, "evidence_excerpt": text}}}
				storyTime46Complete(t, routes, provider, endpoint, sid, 2, 1, text+" The cellar still needs a brass key.", x)
			} else {
				storyTime46Complete(t, routes, provider, endpoint, sid, 2, 2, "Instead the traveler rests at the inn without working on the shield.", map[string]any{"turn_summary": "The traveler rests.", "importance_score": 5})
			}
			fmt.Fprint(w, `{"status":"ok"}`)
		})
	}
	mux.Handle("/", routes)
	server := httptest.NewServer(mux)
	defer server.Close()
	timeout, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		node = "node"
	}
	cmd := exec.CommandContext(timeout, node, script, server.URL, sid, os.Getenv("AC_BROWSER_E2E_OUTPUT"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("browser to SQL lifecycle: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}

// Uses real complete/prepare HTTP, MariaDB and lifecycle storage. Provider
// responses and vectors are controlled external boundaries, not real-model QA.
func TestCriticPreservation48HTTPMariaDBCompletionAndReroll(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	ctx := context.Background()
	const sid, key = "critic-preservation48", "ferry-repair"
	complete := func(turn, generation int, phase, value string) {
		line := "Rowan reports: " + value + "."
		reaction := "Mira feels relieved after Rowan completed the ferry repairs."
		x := map[string]any{
			"turn_summary": value, "importance_score": 8, "evidence_excerpts": []any{line, reaction},
			"entities":                   map[string]any{"characters": []any{map[string]any{"name": "Rowan"}, map[string]any{"name": "Mira"}}},
			"state_claims":               []any{map[string]any{"subject": "Rowan", "state_slot": "repair_status", "lifecycle_key": key, "value": value, "transition": phase, "evidence_excerpt": line}},
			"subjective_entity_memories": []any{map[string]any{"owner_entity_name": "Mira", "owner_visibility": "owner_private", "memory_text": reaction, "evidence_excerpt": reaction}},
		}
		storyTime46Complete(t, routes, provider, endpoint, sid, turn, generation, "At the ferry, the crew gathers. "+line+" "+reaction+" The crew departs.", x)
	}
	assertState := func(phase, value string, count int) {
		t.Helper()
		rows, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
		if err != nil || len(rows) != 1 {
			t.Fatalf("current rows=%d err=%v", len(rows), err)
		}
		p := storyTime46JSON(t, rows[0].ValueJSON)
		if p["transition"] != phase || p["value"] != value {
			t.Fatalf("wrong lifecycle: %#v", p)
		}
		memories, err := st.ListMemories(ctx, sid, -1, -1)
		if err != nil || len(memories) != count {
			t.Fatalf("retry/replacement count=%d want=%d err=%v", len(memories), count, err)
		}
		query := "Mira asks about Rowan's ferry repair progress."
		response := storyTime46Request(t, routes, http.MethodPost, "/prepare-turn", map[string]any{"chat_session_id": sid, "turn_index": 4, "raw_user_input": query, "messages": []any{map[string]any{"role": "user", "content": query}}, "settings": map[string]any{"apply_mode": "live", "guide_mode": "off", "max_injection_chars": 32000, "memory_delivery_budget_mode": "auto"}})
		text, _ := storyTime46Map(storyTime46Map(response["injection_pack"])["memory_delivery_plan"])["final_text"].(string)
		if !strings.Contains(text, value) {
			t.Fatalf("stored current state did not reach final delivery: %s", text)
		}
	}
	complete(1, 1, "partial", "Deck repaired; railing still needs work")
	assertState("partial", "Deck repaired; railing still needs work", 1)
	complete(2, 1, "complete", "All ferry repairs finished")
	assertState("complete", "All ferry repairs finished", 2)
	complete(2, 1, "complete", "All ferry repairs finished")
	assertState("complete", "All ferry repairs finished", 2)
	complete(2, 2, "partial", "Railing remains unfinished after inspection")
	assertState("partial", "Railing remains unfinished after inspection", 2)
	complete(3, 1, "complete", "Railing repaired and ferry reopened")
	assertState("complete", "Railing repaired and ferry reopened", 3)
}

func TestStorylineFullHistoryMariaDBProjectionAndTrust(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	ctx := context.Background()
	for _, mode := range []string{"reroll_without_goal_mention", "manual_trust_survives_progress", "manual_trust_survives_reroll"} {
		t.Run(mode, func(t *testing.T) {
			sid := "storyline-audit-" + mode
			key := "shield-handover"
			initial := map[string]any{"turn_summary": "The shield handover was promised.", "importance_score": 7,
				"pending_threads": []any{map[string]any{"title": "Collect the shield", "description": "Collect the engraved shield from the workshop.", "lifecycle_key": key, "status": "open", "confidence": .9}}}
			storyTime46Complete(t, routes, provider, endpoint, sid, 1, 1, "The artisan promises to finish and hand over the engraved shield at the workshop.", initial)
			rows, err := st.ListStorylines(ctx, sid)
			if err != nil || len(rows) != 1 || rows[0].Status != "active" {
				t.Fatalf("initial projection: %+v, %v", rows, err)
			}
			phase, text := "complete", "The engraved shield is handed over."
			if strings.HasPrefix(mode, "manual_trust") {
				_, err = st.(interface {
					PatchStorylineTrust(context.Context, int64, map[string]any) ([]string, error)
				}).PatchStorylineTrust(ctx, rows[0].ID, map[string]any{"pinned": true, "suppressed": true, "user_corrected": true})
				if err != nil {
					t.Fatal(err)
				}
				storyTime46Request(t, routes, http.MethodPatch, fmt.Sprintf("/storylines/%d", rows[0].ID), map[string]any{"current_context": "User checked the workshop promise."})
				pending, err := st.ListPendingThreads(ctx, sid, "all")
				if err != nil || len(pending) != 1 {
					t.Fatalf("pending before edit: %+v %v", pending, err)
				}
				storyTime46Request(t, routes, http.MethodPatch, fmt.Sprintf("/pending-threads/%d", pending[0].ID), map[string]any{"title": "Collect the engraved shield"})
				phase, text = "partial", "The shield engraving is finished, but the handle is unfinished."
			}
			update := map[string]any{"turn_summary": text, "importance_score": 7,
				"state_claims": []any{map[string]any{"subject": "Shield collection", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": key, "value": text, "transition": phase, "confidence": .9, "evidence_excerpt": text}}}
			storyTime46Complete(t, routes, provider, endpoint, sid, 2, 1, "At the workshop, the artisan checks the task. "+text, update)
			if strings.Contains(mode, "reroll") {
				storyTime46Complete(t, routes, provider, endpoint, sid, 2, 2, "Instead, the traveler spends the afternoon at the inn and makes no progress on the workshop task.", map[string]any{"turn_summary": "The traveler rests at the inn.", "importance_score": 5})
			}
			rows, err = st.ListStorylines(ctx, sid)
			if err != nil || len(rows) != 1 || rows[0].Status != "active" {
				t.Fatalf("Storyline must reflect surviving open goal: %+v, %v", rows, err)
			}
			if strings.HasPrefix(mode, "manual_trust") && (!rows[0].Pinned || !rows[0].Suppressed || !rows[0].UserCorrected) {
				t.Fatalf("Critic erased manual trust settings: %+v", rows[0])
			}
			pending, err := st.ListPendingThreads(ctx, sid, "all")
			if err != nil || len(pending) != 1 || pending[0].Status != "open" {
				t.Fatalf("pending current state disagrees: %+v, %v", pending, err)
			}
			if strings.HasPrefix(mode, "manual_trust") && !pending[0].UserCorrected {
				t.Fatalf("pending edit mark lost: %+v", pending[0])
			}
			response := storyTime46Request(t, routes, http.MethodGet, "/storylines/"+sid, nil)
			got := response["storylines"].([]any)[0].(map[string]any)
			if got["user_corrected"] != strings.HasPrefix(mode, "manual_trust") {
				t.Fatalf("incorrect persisted marker: %+v", got)
			}

		})
	}
}

func TestWorldRuleManualTrustSurvivesCriticAndReroll(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	ctx := context.Background()
	sid := "audit48-world-rule-trust"
	sentence := "The cellar can only be opened with the brass key."
	extraction := map[string]any{"turn_summary": sentence, "importance_score": 7,
		"world_rules":       []any{map[string]any{"scope": "location", "scope_name": "old library", "category": "access", "key": "cellar_needs_key", "value": sentence}},
		"evidence_excerpts": []any{sentence}}
	storyTime46Complete(t, routes, provider, endpoint, sid, 1, 1, "The librarian explains the standing access rule. "+sentence, extraction)
	rows, err := st.ListWorldRules(ctx, sid)
	if err != nil || len(rows) != 1 {
		t.Fatalf("initial rules=%+v err=%v", rows, err)
	}
	if rows[0].UserCorrected {
		t.Fatal("automatic row incorrectly marked")
	}
	storyTime46Request(t, routes, http.MethodPatch, fmt.Sprintf("/world-rules/%d", rows[0].ID), map[string]any{"category": "user-confirmed access"})
	_, err = st.(interface {
		PatchWorldRuleTrust(context.Context, int64, map[string]any) ([]string, error)
	}).PatchWorldRuleTrust(ctx, rows[0].ID, map[string]any{"pinned": true, "suppressed": true, "user_corrected": true})
	if err != nil {
		t.Fatal(err)
	}
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 1, "Later the librarian repeats the same standing access rule. "+sentence, extraction)
	rows, err = st.ListWorldRules(ctx, sid)
	if err != nil || len(rows) != 1 {
		t.Fatalf("updated rules=%+v err=%v", rows, err)
	}
	if !rows[0].Pinned || !rows[0].Suppressed || !rows[0].UserCorrected {
		t.Fatalf("Critic reset manual world-rule trust: %+v", rows[0])
	}

	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 2, "Instead the librarian leaves the room and the traveler rests.", map[string]any{"turn_summary": "The traveler rests.", "importance_score": 5})
	rows, err = st.ListWorldRules(ctx, sid)
	if err != nil || len(rows) != 1 || !rows[0].Pinned || !rows[0].Suppressed || !rows[0].UserCorrected {
		t.Fatalf("reroll lost world rule trust: %+v %v", rows, err)
	}
	// Trust edited on the newer version must also survive, including explicit
	// removal of a flag that used to be enabled on the older version.
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 3, "The librarian repeats the rule once more. "+sentence, extraction)
	rows, err = st.ListWorldRules(ctx, sid)
	if err != nil || len(rows) != 1 {
		t.Fatalf("newer world rule: %+v %v", rows, err)
	}
	storyTime46Request(t, routes, http.MethodPatch, fmt.Sprintf("/world-rules/%d/trust", rows[0].ID), map[string]any{"user_corrected": false, "pinned": false})
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 4, "The traveler instead rests outside the library.", map[string]any{"turn_summary": "A rest outside.", "importance_score": 5})
	rows, err = st.ListWorldRules(ctx, sid)
	if err != nil || len(rows) != 1 || rows[0].UserCorrected || rows[0].Pinned || !rows[0].Suppressed {
		t.Fatalf("latest trust edit lost on rollback: %+v %v", rows, err)
	}
}
