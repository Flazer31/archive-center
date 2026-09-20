package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// This is a deterministic provider boundary, not a substitute for the HTTP
// acceptance, extraction, source admission, status reducer or SQL owners.
type storyTime46Provider struct {
	mu         sync.Mutex
	extraction map[string]any
	calls      int
}

func (p *storyTime46Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if strings.HasSuffix(r.URL.Path, "/embeddings") {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{.1, .2, .3}, "index": 0}}})
		return
	}
	p.calls++
	raw, _ := json.Marshal(p.extraction)
	_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(raw)}}}})
}

func storyTime46Server(t *testing.T, st archiveStore.Store) (http.Handler, *storyTime46Provider, string) {
	t.Helper()
	provider := &storyTime46Provider{}
	upstream := httptest.NewServer(provider)
	t.Cleanup(upstream.Close)
	server := httpapi.NewServer(config.Config{})
	server.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	server.Cfg.ChromaEndpoint = upstream.URL + "/fake-vector-in-process"
	server.Store, server.StoreOpenError = st, nil
	server.Vector = vector.NewFakeVectorStore()
	server.RuntimeConfig = httpapi.RuntimeConfig{Synced: true, CriticTimeoutSec: 10, LLMRetryCount: 1}
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	return routes, provider, upstream.URL + "/v1"
}

func storyTime46JSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode JSON %q: %v", raw, err)
	}
	return out
}

func storyTime46Map(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func storyTime46Request(t *testing.T, routes http.Handler, method, path string, payload any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	routes.ServeHTTP(response, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	if response.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
	}
	return storyTime46JSON(t, response.Body.String())
}

func storyTime46Hash(value string) string {
	var hash int64 = 5381
	for _, unit := range utf16.Encode([]rune(value)) {
		hash = ((hash << 5) + hash + int64(unit)) & 0x7fffffff
	}
	return "or1c_" + strconv.FormatInt(hash, 36)
}

func storyTime46Complete(t *testing.T, routes http.Handler, provider *storyTime46Provider, endpoint, sid string, turn, generation int, assistant string, extraction map[string]any) map[string]any {
	t.Helper()
	provider.mu.Lock()
	provider.extraction = extraction
	provider.mu.Unlock()
	user := fmt.Sprintf("Continue the scene at step %d.", turn)
	observed := int64(turn*10000 + generation*100)
	observation := map[string]any{
		"contract_version": "source_acceptance_observation.v1", "observed_at_ms": observed, "session_id": sid,
		"host_chat_id": sid + "-chat", "host_chat_id_state": "observed", "chat_streaming_state": "not_streaming",
		"active_message_count": turn * 2, "message_index": turn*2 - 1, "message_role": "char",
		"message_chat_id": fmt.Sprintf("%s-assistant-%d", sid, turn), "message_chat_id_state": "observed",
		"generation_id": fmt.Sprintf("%s-generation-%d-%d", sid, turn, generation), "generation_id_state": "observed",
		"branch_id": "", "branch_id_state": "not_exposed_by_risuai", "message_swipe_id": -1, "message_swipe_id_state": "not_present",
		"message_time_ms": observed - 10, "message_time_state": "observed",
		"observed_content_hash": storyTime46Hash(assistant), "persistence_content_hash": storyTime46Hash(assistant),
		"hash_algorithm": "or1c_utf16_djb2.v1", "position_observation": "current_active_chat_tail", "revision_state": "not_exposed_by_risuai",
		"user_message_index": turn*2 - 2, "user_observed_pair_ordinal": turn,
		"user_message_time_ms": int64(turn * 1000), "user_message_time_state": "observed",
		"user_observed_content_hash": storyTime46Hash(user), "user_persistence_content_hash": storyTime46Hash(user),
	}
	response := storyTime46Request(t, routes, http.MethodPost, "/complete-turn", map[string]any{
		"chat_session_id": sid, "turn_index": turn, "user_input": user, "assistant_content": assistant,
		"client_meta": map[string]any{"source_acceptance_required": true, "source_acceptance_observation": observation,
			"critic":    map[string]any{"provider": "openai", "api_key": "disposable-local-test", "endpoint": endpoint, "model": "story-time-fixture", "timeout_ms": 10000},
			"embedding": map[string]any{"provider": "openai", "api_key": "disposable-local-test", "endpoint": endpoint + "/embeddings", "model": "local-test-embedding", "timeout_ms": 10000}},
	})
	if response["status"] != "ok" || (response["critic_triggered"] != true && storyTime46Map(response["trace_handoff"])["idempotent_replay"] != true) {
		t.Fatalf("complete-turn did not run production extraction: %#v", response)
	}
	if count, ok := response["derived_artifacts_errors"].(float64); ok && count != 0 {
		t.Fatalf("complete-turn SQL reducer errors: %#v", response)
	}
	return response
}

func storyTime46Extraction(kind, precision, excerpt, state string, detail map[string]any) map[string]any {
	clock := map[string]any{"version": "story_clock.v1", "observation_kind": kind, "precision": precision, "scene_scope": "current", "transition": "advance", "evidence_excerpt": excerpt}
	clock[kind] = detail
	return map[string]any{
		"turn_summary": excerpt + " Mina records the delivery state.", "importance_score": 7,
		"evidence_excerpts": []any{excerpt}, "story_clock": clock,
		"state_claims":     []any{map[string]any{"subject": "Caravan delivery", "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": "time-delivery", "value": state, "transition": "partial", "confidence": .9, "evidence_excerpt": excerpt}},
		"character_deltas": []any{map[string]any{"name": "Mina", "status": map[string]any{"delivery": state}, "evidence_excerpt": excerpt}},
	}
}

func storyTime46Memory(t *testing.T, st archiveStore.Store, sid string, turn int) archiveStore.Memory {
	t.Helper()
	memories, err := st.ListMemories(context.Background(), sid, turn, turn)
	if err != nil || len(memories) != 1 {
		t.Fatalf("memory at %s/%d: count=%d err=%v", sid, turn, len(memories), err)
	}
	return memories[0]
}

func storyTime46Current(t *testing.T, st archiveStore.Store, sid, key string) archiveStore.StatusCurrentValue {
	t.Helper()
	values, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(context.Background(), sid, "", "", key, -1)
	if err != nil || len(values) != 1 {
		t.Fatalf("current %s/%s: %#v err=%v", sid, key, values, err)
	}
	return values[0]
}

func storyTime46Observation(t *testing.T, memory archiveStore.Memory) map[string]any {
	t.Helper()
	return storyTime46Map(storyTime46Map(storyTime46JSON(t, memory.SummaryJSON)["temporal_context"])["observed_at"])
}

func storyTime46Date(observation map[string]any) any {
	return storyTime46Map(storyTime46Map(observation["story_clock"])["absolute"])["date"]
}

func storyTime46AssertSameTurn(t *testing.T, st archiveStore.Store, sid string, turn int, date string) {
	t.Helper()
	observation := storyTime46Observation(t, storyTime46Memory(t, st, sid, turn))
	if storyTime46Date(observation) != date {
		t.Fatalf("source observation date: %#v want=%s", observation, date)
	}
	clock := storyTime46Current(t, st, sid, "story_clock")
	if storyTime46Map(storyTime46JSON(t, clock.ValueJSON)["absolute"])["date"] != date || clock.SourceTurn != turn {
		t.Fatalf("current clock: %+v want turn=%d date=%s", clock, turn, date)
	}
	narrative := storyTime46Current(t, st, sid, "narrative_state")
	payload := storyTime46JSON(t, narrative.ValueJSON)
	if !reflect.DeepEqual(payload["observed_at"], observation) {
		t.Fatalf("same-turn narrative observed_at differs: narrative=%#v source=%#v", payload["observed_at"], observation)
	}
	events, err := st.(archiveStore.StatusLifecycleStore).ListStatusChangeEvents(context.Background(), sid, "", "", "narrative_state", -1)
	if err != nil || len(events) == 0 {
		t.Fatalf("same-turn narrative history missing: events=%+v err=%v", events, err)
	}
	if storyTime46JSON(t, events[0].NewValueJSON)["observed_at"] == nil {
		t.Fatal("narrative history lost source observation clock")
	}
	if events[0].StoryClockJSON == "" || storyTime46Map(storyTime46JSON(t, events[0].StoryClockJSON)["absolute"])["date"] != date {
		t.Fatalf("same-turn narrative clock used previous turn: events=%+v err=%v", events, err)
	}
	character, err := st.GetCharacterState(context.Background(), sid, "Mina")
	if err != nil {
		t.Fatal(err)
	}
	field := characterFieldMetadata46(t, character, "/status/delivery")
	if !reflect.DeepEqual(field["observed_at"], observation) {
		t.Fatalf("changed character field clock differs: field=%#v source=%#v", field, observation)
	}
}

func TestStoryTime46HTTPMariaDBSourceAnchorsAdvanceReplaceRollbackAndBranch(t *testing.T) {
	db, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid, branch = "story-time-main", "story-time-branch"
	firstText := "The scene begins on 1423-04-12. Mina has three crates left to deliver."
	first := storyTime46Extraction("absolute", "exact", "The scene begins on 1423-04-12.", "three crates remain", map[string]any{"date": "1423-04-12"})
	storyTime46Complete(t, routes, provider, endpoint, sid, 1, 1, firstText, first)
	storyTime46AssertSameTurn(t, st, sid, 1, "1423-04-12")
	firstMemory := storyTime46Memory(t, st, sid, 1)
	firstClock := storyTime46Current(t, st, sid, "story_clock")
	secondText := "Two days later, Mina has two crates left to deliver."
	second := storyTime46Extraction("relative", "exact", "Two days later", "two crates remain", map[string]any{"offset": 2, "unit": "day", "anchor": "story_clock.current"})
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 1, secondText, second)
	storyTime46AssertSameTurn(t, st, sid, 2, "1423-04-14")
	secondMemory := storyTime46Memory(t, st, sid, 2)
	secondClock := storyTime46Current(t, st, sid, "story_clock")
	if got := storyTime46Memory(t, st, sid, 1); got.SummaryJSON != firstMemory.SummaryJSON {
		t.Fatal("relative advance rewrote immutable older memory source time")
	}
	// Repeating the accepted source must not apply the relative offset twice.
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 1, secondText, second)
	if got := storyTime46Memory(t, st, sid, 2); got.SummaryJSON != secondMemory.SummaryJSON {
		t.Fatal("accepted source replay re-anchored the memory")
	}
	if got := storyTime46Current(t, st, sid, "story_clock"); got.ValueJSON != secondClock.ValueJSON {
		t.Fatal("accepted source replay advanced the relative clock twice")
	}
	copyResult := storyTime46Request(t, routes, http.MethodPost, "/sessions/migrate-complete", map[string]any{"source_session_id": sid, "target_session_id": branch, "mode": archiveStore.SessionMigrationModeCopyKeepSource})
	if copyResult["blocked"] == true || copyResult["write_attempted"] != true {
		t.Fatalf("branch HTTP copy failed: %#v", copyResult)
	}
	if got := storyTime46Memory(t, st, branch, 1); got.SummaryJSON != firstMemory.SummaryJSON {
		t.Fatal("branch copy changed historical source anchor")
	}
	branchClock := storyTime46Current(t, st, branch, "story_clock")
	if branchClock.ValueJSON != secondClock.ValueJSON {
		t.Fatal("branch current clock differs from copied current")
	}
	branchSource := storyTime46JSON(t, branchClock.EvidenceJSON)["source_revision"]
	if branchSource == storyTime46JSON(t, secondClock.EvidenceJSON)["source_revision"] {
		t.Fatal("branch operational clock retained parent source binding")
	}
	var active int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_source_revisions WHERE chat_session_id=? AND source_revision=? AND lifecycle_state='active'`, branch, branchSource).Scan(&active); err != nil || active != 1 {
		t.Fatalf("branch clock source is not active in branch: %v active=%d", err, active)
	}
	// A reroll on the existing logical tail first restores its predecessor, then
	// resolves the new relative expression against that restored clock.
	replacementText := "One day later, Mina has one crate left to deliver."
	replacement := storyTime46Extraction("relative", "exact", "One day later", "one crate remains", map[string]any{"offset": 1, "unit": "day", "anchor": "story_clock.current"})
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 2, replacementText, replacement)
	storyTime46AssertSameTurn(t, st, sid, 2, "1423-04-13")
	if got := storyTime46Current(t, st, branch, "story_clock"); got.ValueJSON != branchClock.ValueJSON {
		t.Fatal("parent replacement changed sibling clock")
	}
	storyTime46Request(t, routes, http.MethodDelete, "/rollback/2?chat_session_id="+sid+"&req_source=timeline_manual_delete", nil)
	if got := storyTime46Current(t, st, sid, "story_clock"); got.SourceTurn != 1 || got.ValueJSON != firstClock.ValueJSON {
		t.Fatalf("rollback did not restore exact prior clock: %+v", got)
	}
	if got := storyTime46Memory(t, st, sid, 1); got.SummaryJSON != firstMemory.SummaryJSON {
		t.Fatal("rollback rewrote surviving source anchor")
	}
	if got := storyTime46Memory(t, st, branch, 2); got.SummaryJSON != secondMemory.SummaryJSON {
		t.Fatal("parent rollback changed sibling source time")
	}
}

func TestStoryTime46HTTPMariaDBUnknownPartialAndFlashbackKeepTheirMeaning(t *testing.T) {
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid = "story-time-uncertain"
	unknown := map[string]any{"turn_summary": "Mina entered the hall at an unspecified time.", "importance_score": 5, "evidence_excerpts": []any{"Mina entered the hall"}}
	storyTime46Complete(t, routes, provider, endpoint, sid, 1, 1, "Mina entered the hall and looked around.", unknown)
	firstMemory := storyTime46Memory(t, st, sid, 1)
	if got := storyTime46Observation(t, firstMemory); got["story_time"] != "unknown" || got["story_clock"] != nil {
		t.Fatalf("unknown source time acquired a turn/server date: %#v", got)
	}
	partial := storyTime46Extraction("partial", "partial", "Dusk falls over the Ember calendar's festival.", "festival delivery", map[string]any{"daypart": "dusk"})
	storyTime46Map(partial["story_clock"])["calendar"] = map[string]any{"id": "ember", "label": "Ember festival day"}
	storyTime46Complete(t, routes, provider, endpoint, sid, 2, 1, "Dusk falls over the Ember calendar's festival. Mina waits by the door.", partial)
	partialMemory := storyTime46Memory(t, st, sid, 2)
	partialObservation := storyTime46Observation(t, partialMemory)
	partialClock := storyTime46Map(partialObservation["story_clock"])
	if partialClock["precision"] != "partial" || partialClock["absolute"] != nil || storyTime46Map(partialClock["calendar"])["id"] != "ember" {
		t.Fatalf("partial/custom calendar was fabricated or lost: %#v", partialClock)
	}
	relative := storyTime46Extraction("relative", "exact", "Two days later", "still waiting", map[string]any{"offset": 2, "unit": "day", "anchor": "story_clock.current"})
	storyTime46Complete(t, routes, provider, endpoint, sid, 3, 1, "Two days later, Mina is still waiting.", relative)
	unresolved := storyTime46Map(storyTime46Observation(t, storyTime46Memory(t, st, sid, 3))["story_clock"])
	if unresolved["precision"] != "unknown" || unresolved["absolute"] != nil || storyTime46Map(unresolved["relative"])["offset"] != float64(2) {
		t.Fatalf("relative expression without a comparable anchor was lost or fabricated: %#v", unresolved)
	}
	abs := storyTime46Extraction("absolute", "exact", "The current scene is dated 1423-05-01.", "one crate remains", map[string]any{"date": "1423-05-01"})
	storyTime46Complete(t, routes, provider, endpoint, sid, 4, 1, "The current scene is dated 1423-05-01. Mina checks the remaining crate.", abs)
	confirmed := storyTime46Current(t, st, sid, "story_clock")
	flashback := map[string]any{
		"turn_summary": "Mina remembers the old scene from 1401-01-01.", "importance_score": 5,
		"evidence_excerpts": []any{"The flashback took place on 1401-01-01."},
		"story_clock":       map[string]any{"version": "story_clock.v1", "observation_kind": "absolute", "precision": "exact", "scene_scope": "flashback", "transition": "set", "absolute": map[string]any{"date": "1401-01-01"}, "evidence_excerpt": "The flashback took place on 1401-01-01."},
	}
	storyTime46Complete(t, routes, provider, endpoint, sid, 5, 1, "The flashback took place on 1401-01-01. Mina remembers the old hall.", flashback)
	if got := storyTime46Current(t, st, sid, "story_clock"); got.ValueJSON != confirmed.ValueJSON || got.SourceTurn != 4 {
		t.Fatalf("historical occurrence replaced current clock: %+v", got)
	}
	flashbackMemory := storyTime46Memory(t, st, sid, 5)
	if got := storyTime46Observation(t, flashbackMemory); storyTime46Date(got) != "1423-05-01" || got["resolution_source"] != "last_confirmed_clock" {
		t.Fatalf("historical occurrence was confused with observation: %#v", got)
	}
	if proposal := storyTime46Map(storyTime46JSON(t, flashbackMemory.SummaryJSON)["story_clock"]); storyTime46Map(proposal["absolute"])["date"] != "1401-01-01" || proposal["scene_scope"] != "flashback" {
		t.Fatalf("original flashback source expression changed: %#v", proposal)
	}
	history, err := st.(archiveStore.StatusLifecycleStore).ListStatusChangeEvents(context.Background(), sid, "", "", "story_clock", -1)
	if err != nil || len(history) != 4 || history[0].OwnerID == "current" || storyTime46JSON(t, history[0].EvidenceJSON)["current_projection"] != false {
		t.Fatalf("noncurrent clock lost historical-only ownership: %+v err=%v", history, err)
	}
	for _, memory := range []archiveStore.Memory{firstMemory, partialMemory} {
		if current := storyTime46Memory(t, st, sid, memory.TurnIndex); current.SummaryJSON != memory.SummaryJSON {
			t.Fatalf("later exact clock retroactively dated earlier turn %d", memory.TurnIndex)
		}
	}
}
