package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test46CharacterDeltaKeepsEachFieldSourceAndExplicitTime(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	save := func(turn int, delta map[string]any) store.CharacterState {
		t.Helper()
		ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{Revision: fmt.Sprintf("field-source-%d", turn)})
		result := artifactSaveResult{}
		srv.saveCharacterAndStateArtifacts(ctx, "field-session", turn, map[string]any{"character_deltas": []any{delta}}, "The scene is recalled. Mina was bound in chains. Mina was released at dawn. The report ends.", completeTurnEmbeddingConfig{}, time.Unix(int64(turn), 0), &result, nil, &canonicalStateWriteCostMeasurement{})
		if result.Errors != 0 || len(st.savedCharacterStates) == 0 {
			t.Fatalf("character delta production save failed: %#v", result)
		}
		next := *st.savedCharacterStates[len(st.savedCharacterStates)-1]
		st.returnCharStates = []store.CharacterState{next}
		return next
	}
	first := save(1, map[string]any{
		"name": "Mina", "appearance": map[string]any{"binding": "bound in chains", "hair": "black"},
		"status":           map[string]any{"movement": "restrained", "profession": "cartographer"},
		"evidence_excerpt": "Mina was bound in chains.",
		"field_provenance": map[string]any{"fields": map[string]any{"/appearance/binding": map[string]any{"occurrence_time": "previous winter", "validity": map[string]any{"valid_from": "previous winter"}}}},
	})
	firstFields := store.DecodeCharacterFieldProvenance(first.FieldProvenanceJSON)
	if intFromAny(firstFields["/appearance/binding"]["source_turn"], 0) != 1 || firstFields["/appearance/binding"]["occurrence_time"] != "previous winter" {
		t.Fatalf("first field provenance lost: %#v", firstFields)
	}
	next := save(5, map[string]any{
		"name": "Mina", "status": map[string]any{"movement": "released"}, "evidence_excerpt": "Mina was released at dawn.",
		"field_provenance": map[string]any{"fields": map[string]any{"/status/movement": map[string]any{"occurrence_time": "dawn", "validity": map[string]any{"valid_from": "dawn"}}}},
	})
	fields := store.DecodeCharacterFieldProvenance(next.FieldProvenanceJSON)
	for _, path := range []string{"/appearance/binding", "/appearance/hair", "/status/profession"} {
		if intFromAny(fields[path]["source_turn"], 0) != 1 || fields[path]["source_revision"] != "field-source-1" {
			t.Fatalf("untouched field inherited row freshness at %s: %#v", path, fields[path])
		}
	}
	if fields["/status/movement"]["source_revision"] != "field-source-5" || fields["/status/movement"]["occurrence_time"] != "dawn" || fields["/status/movement"]["evidence_excerpt"] != "Mina was released at dawn." {
		t.Fatalf("changed field observation/occurrence was conflated: %#v", fields["/status/movement"])
	}
	if got := stringFromMap(parseJSONMap(next.AppearanceJSON), "binding"); got != "bound in chains" {
		t.Fatalf("history was overwritten rather than source-linked: %q", got)
	}
	repeated := save(7, map[string]any{"name": "Mina", "status": map[string]any{"movement": "released"}})
	if repeated.TurnIndex != 7 || intFromAny(store.DecodeCharacterFieldProvenance(repeated.FieldProvenanceJSON)["/status/movement"]["source_turn"], 0) != 5 {
		t.Fatal("unchanged field gained evidence freshness from later snapshot storage")
	}
}

func Test46NarrativeSourceFieldLinksKeepHistoricalFieldAndCurrentMeaning(t *testing.T) {
	st := &turnRecordingStore{returnCharStates: []store.CharacterState{{ChatSessionID: "field-session", CharacterName: "Mina", AppearanceJSON: `{"binding":"bound in chains","hair":"black"}`, TurnIndex: 1}}}
	srv := &Server{Store: st}
	result := artifactSaveResult{}
	srv.saveNarrativeStateFromExtraction(context.Background(), "field-session", 5, map[string]any{"state_claims": []any{map[string]any{
		"subject": "Mina", "subject_type": "character", "state_slot": "restraint", "value": "released", "transition": "change", "evidence_excerpt": "Mina was released at dawn.",
		"source_fields": []any{"/appearance/binding", "/status/movement"}, "occurrence_time": "dawn", "validity": map[string]any{"valid_from": "dawn"},
	}}}, "The report arrived. Mina was released at dawn. The guard nodded.", nil, time.Unix(5, 0), &result)
	if result.Errors != 0 || len(st.returnStatusCurrent) != 1 || len(st.savedStatusEvents) != 1 {
		t.Fatalf("explicit cross-field state did not reach existing owner: %#v", result)
	}
	payload := parseJSONMap(st.returnStatusCurrent[0].ValueJSON)
	if got := stringsFromAny(payload["source_fields"]); len(got) != 2 || got[0] != "/appearance/binding" || payload["value"] != "released" {
		t.Fatalf("semantic association lost: %#v", payload)
	}
	if payload["occurrence_time"] != "dawn" || stringFromMap(mapFromAny(payload["observed_at"]), "story_time") != "unknown" || stringFromMap(mapFromAny(payload["validity"]), "valid_from") != "dawn" {
		t.Fatalf("occurrence/observation/current validity conflated: %#v", payload)
	}
	if st.returnCharStates[0].AppearanceJSON != `{"binding":"bound in chains","hair":"black"}` {
		t.Fatal("semantic link mutated unrelated historical character fields")
	}
}

func Test46ReversibleSourceFieldsKeepTransferBalanceInventoryAndCapability(t *testing.T) {
	st := newIdentityRecordingStore()
	for _, label := range []string{"Mina", "Lio", "Bronze Compass"} {
		st.reviewCanonicalLabel(label)
	}
	characters := []any{map[string]any{"name": "Mina"}, map[string]any{"name": "Lio"}}
	items := []any{map[string]any{"name": "Bronze Compass", "entity_type": "item"}}
	proposal := func(domain, transition, owner, slot, excerpt, value, path string) map[string]any {
		p := reversibleStateProposal(domain, transition, owner, slot, excerpt, value)
		p["source_fields"] = []any{path}
		return p
	}
	initial := []any{
		proposal("possession", "set", "Mina", "bronze_key", "Mina held the bronze key.", "held the bronze key", "/status/held_items"),
		proposal("possession", "set", "Mina", "balance", "Mina owned forty crowns.", "owned forty crowns", "/status/balance"),
		proposal("possession", "set", "Mina", "equipment", "Mina carried rope and chalk.", "carried rope and chalk", "/status/equipment"),
		proposal("entity_condition", "set", "Bronze Compass", "navigation", "Bronze Compass pointed north reliably.", "pointed north reliably", "/status/navigation"),
	}
	saveReversibleTurn(t, st, 1, "field-initial", "Mina held the bronze key. Mina owned forty crowns. Mina carried rope and chalk. Bronze Compass pointed north reliably.", initial, characters, items)
	clear := reversibleStateProposal("possession", "clear", "Mina", "bronze_key", "Mina gave the bronze key to Lio.", "")
	balance := proposal("possession", "change", "Mina", "balance", "Mina retained ten crowns after paying.", "retained ten crowns after paying", "/status/balance")
	balance["occurrence_time"] = "market morning"
	balance["validity"] = map[string]any{"valid_from": "2030-05-01"}
	saveReversibleTurn(t, st, 2, "field-transfer", "Mina gave the bronze key to Lio. Lio held the bronze key. Mina retained ten crowns after paying. Bronze Compass lost its northfinding ability.", []any{
		clear,
		proposal("possession", "set", "Lio", "bronze_key", "Lio held the bronze key.", "held the bronze key", "/status/held_items"),
		balance,
		proposal("entity_condition", "change", "Bronze Compass", "navigation", "Bronze Compass lost its northfinding ability.", "lost its northfinding ability", "/status/navigation"),
	}, characters, items)
	owners := map[string]map[string]any{}
	for _, current := range st.returnStatusCurrent {
		if current.StatusKey == reversiblePossessionStatusKey || current.StatusKey == reversibleEntityStatusKey {
			projection := parseJSONMap(current.ValueJSON)
			owners[stringFromMap(projection, "subject_label")] = mapFromAny(projection["slots"])
		}
	}
	if owners["Mina"]["bronze_key"] != nil || owners["Lio"]["bronze_key"] == nil {
		t.Fatalf("transferred item has stale ownership: %#v", owners)
	}
	equipment := mapFromAny(owners["Mina"]["equipment"])
	if intFromAny(mapFromAny(equipment["source"])["source_turn"], 0) != 1 || stringFromMap(mapFromAny(equipment["source"]), "evidence_excerpt") != "Mina carried rope and chalk." {
		t.Fatalf("unchanged inventory acquired later balance quote/time: %#v", equipment)
	}
	currentBalance := mapFromAny(owners["Mina"]["balance"])
	if stringFromMap(mapFromAny(currentBalance["value"]), "text") != "retained ten crowns after paying" || currentBalance["occurrence_time"] != "market morning" || stringFromMap(mapFromAny(currentBalance["validity"]), "valid_from") != "2030-05-01" {
		t.Fatalf("balance value and effective-time evidence lost: %#v", currentBalance)
	}
	if got := stringFromMap(mapFromAny(mapFromAny(owners["Bronze Compass"]["navigation"])["value"]), "text"); got != "lost its northfinding ability" {
		t.Fatalf("item capability remained stale: %q", got)
	}
	foundClear := false
	for _, event := range st.savedStatusEvents {
		if event.EventKind != "clear" {
			continue
		}
		observation := mapFromAny(parseJSONMap(event.EvidenceJSON)["history_observation"])
		fields := stringsFromAny(observation["source_fields"])
		if len(fields) == 1 && fields[0] == "/status/held_items" && stringFromMap(observation, "evidence_excerpt") == "Mina gave the bronze key to Lio." {
			foundClear = true
		}
	}
	if !foundClear {
		t.Fatal("clear removed the slot's historical field association")
	}
}

func Test46ReversibleHistoricalAndPrivateKnowledgeDoNotBecomeCurrent(t *testing.T) {
	st := newIdentityRecordingStore()
	st.reviewCanonicalLabel("Mina")
	characters := []any{map[string]any{"name": "Mina"}}
	initial := reversibleStateProposal("possession", "set", "Mina", "balance", "Mina owned ten crowns.", "owned ten crowns")
	initial["source_fields"] = []any{"/status/balance"}
	saveReversibleTurn(t, st, 2, "time-current", "Mina owned ten crowns.", []any{initial}, characters, nil)
	flashback := reversibleStateProposal("possession", "change", "Mina", "balance", "Mina owned fifty crowns last winter.", "owned fifty crowns last winter")
	flashback["scene_scope"], flashback["occurrence_time"] = "flashback", "last winter"
	saveReversibleTurn(t, st, 6, "time-flashback", "Mina owned fifty crowns last winter.", []any{flashback}, characters, nil)
	belief := reversibleStateProposal("possession", "change", "Mina", "balance", "Mina believed she owned a hundred crowns.", "owned a hundred crowns")
	belief["visibility"], belief["authority"] = "private", "needs_review"
	saveReversibleTurn(t, st, 7, "time-belief", "Mina believed she owned a hundred crowns.", []any{belief}, characters, nil)
	current := currentReversibleProjection(t, st, reversiblePossessionStatusKey)
	slot := mapFromAny(mapFromAny(current["slots"])["balance"])
	if stringFromMap(mapFromAny(slot["value"]), "text") != "owned ten crowns" || intFromAny(mapFromAny(slot["source"])["source_turn"], 0) != 2 {
		t.Fatalf("later observation/knowledge changed objective current balance: %#v", slot)
	}
	if len(st.savedStatusEvents) != 3 || st.savedStatusEvents[1].EventState != "history_only" || st.savedStatusEvents[2].EventState != "history_only" {
		t.Fatalf("historical/knowledge observations were lost: %#v", st.savedStatusEvents)
	}
	packet, text := buildReversibleStatePacket(st.returnStatusCurrent, map[string]any{}, 10000, prepareTurnRequestEntityScope{Direct: []string{"Mina"}})
	if !strings.Contains(text, "ten crowns") || strings.Contains(text, "hundred crowns") || strings.Contains(text, "fifty crowns") || intFromAny(packet["active_count"], 0) != 1 {
		t.Fatalf("current packet confused observation with objective validity: %s %#v", text, packet)
	}
}

func Test46RepairProjectionSeparatesRecordedOrderFromSourceEvidence(t *testing.T) {
	for _, repaired := range []bool{false, true} {
		t.Run(fmt.Sprint(repaired), func(t *testing.T) {
			st := &turnRecordingStore{}
			metadata := map[string]any{"title": "Bridge repair", "lifecycle_key": "bridge-repair", "confidence": .9, "status": "open"}
			expectedOrder := 2
			if repaired {
				metadata["repair_recorded_turn"] = 9
				expectedOrder = 9
			}
			thread := store.PendingThread{ChatSessionID: "repair-projection", ThreadKey: narrativeLifecycleStorageKey("bridge-repair"), Title: "Bridge repair", SourceTurn: 2, CreatedTurn: 1, Status: "open", Confidence: .9, HookMetadataJSON: mustCompactJSON(metadata)}
			result := artifactSaveResult{}
			(&Server{Store: st}).projectNarrativePendingArtifacts(context.Background(), "repair-projection", 9, thread, nil, time.Unix(9, 0), &result, nil, &canonicalStateWriteCostMeasurement{})
			if result.Errors != 0 || len(st.savedActiveStates) != 1 || len(st.savedCanonicalLayers) != 1 || len(st.savedStorylines) != 1 || len(st.savedPendingThreads) != 0 {
				t.Fatalf("projection owner wrote incorrect surfaces: %#v", result)
			}
			active, canonical, storyline := st.savedActiveStates[0], st.savedCanonicalLayers[0], st.savedStorylines[0]
			if active.TurnIndex != expectedOrder || canonical.TurnIndex != expectedOrder || storyline.LastTurn != expectedOrder {
				t.Fatalf("repair recording did not order current projection: active=%#v canonical=%#v storyline=%#v", active, canonical, storyline)
			}
			if intFromAny(parseJSONMap(active.Content)["source_turn"], 0) != 2 || canonical.SourceTurn != 2 || storyline.LastEvidenceTurn != 2 || thread.SourceTurn != 2 {
				t.Fatal("repair recording time became original evidence time")
			}
		})
	}
}

func Test46TypedProfileKeepsEvidenceTimeSeparateFromSnapshotTime(t *testing.T) {
	initial := store.CharacterState{ChatSessionID: "typed-fields", CharacterName: "Mira", AppearanceJSON: `{"hair":"black"}`, StatusJSON: `{"movement":"released"}`, TurnIndex: 10,
		FieldProvenanceJSON: `{"fields":{"/appearance/hair":{"source_turn":1,"source_revision":"hair-origin"},"/status/movement":{"source_turn":8,"source_revision":"release-origin"}}}`}
	st := newCharacterProjectionRecordingStore([]store.CharacterState{initial})
	profile := characterProfileTestUnit("typed-fields", "profile-source-3", 3, "profile", 3, "entity-mira", "Mira", "stable", "values", "honesty", "values honesty", "explicit_statement", "", "", "", "", "", "Mira values honesty.", "owner_private")
	voice := voiceProjectionTestUnit("typed-fields", "voice-source-7", 7, "voice", 7, "entity-mira", "Mira", "directness", "brief", "utterance", `"Enough."`, "", "", "", "", "", "", "", `Mira says "Enough."`, "owner_private")
	result := artifactSaveResult{}
	(&Server{Store: st}).saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "typed-fields", []*store.PreciseMemoryUnit{profile, voice}, time.Unix(10, 0), &result)
	if result.Errors != 0 || len(st.saved) != 1 {
		t.Fatalf("typed projection failed: %#v", result)
	}
	next := st.saved[0]
	fields := store.DecodeCharacterFieldProvenance(next.FieldProvenanceJSON)
	if next.TurnIndex != 10 || intFromAny(fields["/appearance/hair"]["source_turn"], 0) != 1 || fields["/status/movement"]["source_revision"] != "release-origin" {
		t.Fatalf("typed update restamped unrelated field evidence: %#v", fields)
	}
	for path, metadata := range fields {
		expectedSource := 0
		if strings.HasPrefix(path, "/personality/") {
			expectedSource = 3
		} else if strings.HasPrefix(path, "/speech_style/") {
			expectedSource = 7
		}
		if expectedSource > 0 && (intFromAny(metadata["source_turn"], 0) != expectedSource || intFromAny(metadata["recorded_turn"], 0) != 10) {
			t.Fatalf("typed source became latest snapshot time at %s: %#v", path, metadata)
		}
	}
}

func Test46RepairRollbackTombstoneAndExactPendingRemainAuthoritative(t *testing.T) {
	claim := narrativeStateClaim{Subject: "Archive repair", SubjectType: "entity", StateSlot: "goal_status", ClaimScope: "objective", Value: "complete", LifecycleKey: "archive-repair"}
	prior := store.StatusChangeEvent{ID: 1, ChatSessionID: "restore-46", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), EventKind: "complete", NewValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", 5)), EvidenceJSON: `{"current_projection":true}`, SourceTurn: 5}
	exact := store.PendingThread{ID: 7, ChatSessionID: "restore-46", ThreadKey: narrativeLifecycleStorageKey("archive-repair"), Title: "Archive repair", Status: "open", SourceTurn: 1, CreatedTurn: 1, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0)}
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			repair := prior
			repair.ID, repair.SourceTurn = 2, 0
			evidence := map[string]any{"current_projection": true, "source_contract": store.StateRepairContract, "repair_recorded_turn": 9}
			payload := narrativeStateValuePayload(claim, "", 0)
			payload["value"], payload["pending_thread"], payload["repair_pending_snapshot"] = "restored prior value", exact, true
			if remove {
				evidence["projection_action"] = "remove"
				payload = map[string]any{"pending_thread": exact, "repair_pending_snapshot": true}
			}
			repair.NewValueJSON, repair.EvidenceJSON = mustCompactJSON(payload), mustCompactJSON(evidence)
			st := &lifecycle46RestoreStore{turnRecordingStore: &turnRecordingStore{savedStatusEvents: []store.StatusChangeEvent{prior, repair}, returnPendingThreads: []store.PendingThread{exact}}, activeEvents: []store.StatusChangeEvent{repair}}
			count, err := restoreNarrativeCurrentStatesAfterRollback(context.Background(), st, "restore-46", 6)
			expectedCount := 1
			if remove {
				expectedCount = 0
			}
			if err != nil || count != expectedCount || len(st.returnStatusCurrent) != expectedCount {
				t.Fatalf("latest repair/tombstone lost to older cause: count=%d current=%#v err=%v", count, st.returnStatusCurrent, err)
			}
			if !remove && stringFromMap(parseJSONMap(st.returnStatusCurrent[0].ValueJSON), "value") != "restored prior value" {
				t.Fatal("repair recording order was replaced by historical source order")
			}
			if len(st.savedPendingThreads) != 0 || st.returnPendingThreads[0].UpdatedAt != exact.UpdatedAt {
				t.Fatal("generic legacy pending saver modified the exact repair snapshot")
			}
		})
	}
}

func Test46ConfirmedLegacyRepairSupersedesOpenProjectionWithoutInventedConfidence(t *testing.T) {
	claim := narrativeStateClaim{Subject: "Archive repair", SubjectType: "entity", StateSlot: "goal_status", ClaimScope: "objective", Value: "Repair completed", Transition: "complete", LifecycleKey: "archive-repair", Confidence: 0}
	for _, sourceTurn := range []int{0, 2} {
		current := store.StatusCurrentValue{StatusKey: narrativeStateStatusKey, OwnerScope: narrativeStateOwnerScope(claim), OwnerID: narrativeStateOwnerID(claim), WriteState: "current", SourceTurn: sourceTurn,
			ValueJSON:    mustCompactJSON(narrativeStateValuePayload(claim, "open", sourceTurn)),
			EvidenceJSON: mustCompactJSON(map[string]any{"source": "admin.state_repair", "source_contract": store.StateRepairContract, "source_turn": sourceTurn, "repair_recorded_turn": 9, "evidence_excerpt": "The ledger confirms completion."})}
		artifact := `{"lifecycle_key":"archive-repair","title":"Archive repair","status":"open"}`
		for _, artifactTurn := range []int{0, 7} {
			if !narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{current}, artifactTurn, artifact) {
				t.Fatalf("confirmed repair left stale open projection with source=%d candidate=%d confidence=0", sourceTurn, artifactTurn)
			}
		}
		if extractionFloatFromAny(parseJSONMap(current.ValueJSON)["confidence"], -1) != 0 {
			t.Fatal("repair invented historical confidence")
		}
		current.EvidenceJSON = mustCompactJSON(map[string]any{"source": "critic.resolved_threads", "source_turn": sourceTurn})
		if narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{current}, 0, artifact) {
			t.Fatal("repair support changed normal unknown-source legacy projection semantics")
		}
	}
}
