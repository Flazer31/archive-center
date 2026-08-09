package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func interactionProjectionUnit(
	id, kind, contract, actorID, targetID, actor, target, visibility string,
	turn int,
	extra map[string]any,
) store.PreciseMemoryUnit {
	payload := map[string]any{
		"contract_version": contract,
		"visibility":       visibility,
	}
	if kind == "observation" {
		payload["source_entity"] = actor
		payload["target_entity"] = target
	} else {
		payload["actor"] = actor
		payload["counterpart"] = target
	}
	for key, value := range extra {
		payload[key] = value
	}
	encoded, _ := json.Marshal(payload)
	return store.PreciseMemoryUnit{
		UnitID: id, ChatSessionID: "session", SourceTurnStart: turn,
		SourceTurnEnd: turn, SourceRevision: "revision-" + id,
		Kind: kind, PayloadJSON: string(encoded), ActorEntityID: actorID,
		AffectedEntityID: targetID, AdmissionState: "committed",
		ReviewState: "source_observed", Visibility: visibility,
		LifecycleState: "active",
	}
}

func TestActiveInteractionProjectionLaterWithdrawalSuppressesOldAllow(t *testing.T) {
	allow := interactionProjectionUnit(
		"allow", "boundary", interactionBoundaryContract,
		"alice-id", "bob-id", "Alice", "Bob", "owner_private", 1,
		map[string]any{
			"action_scope": "touch", "decision": "allow", "effective_scope": "event",
			"effective_time": map[string]any{"scene": "garden"},
		},
	)
	withdrawn := interactionProjectionUnit(
		"withdrawn", "boundary", interactionBoundaryContract,
		"alice-id", "bob-id", "Alice", "Bob", "owner_private", 2,
		map[string]any{
			"action_scope": "touch", "decision": "withdrawn", "effective_scope": "event",
			"effective_time": map[string]any{"scene": "hall"},
		},
	)
	packet, publicText, guardedText := buildPrepareTurnActiveInteractionProjection(
		[]store.PreciseMemoryUnit{allow, withdrawn},
		map[string]any{"identity_state": "resolved", "current_pov_entity_id": "alice-id"},
		"Bob waits.", []string{"Alice", "Bob"}, 3, true,
	)
	if publicText != "" || !strings.Contains(guardedText, "decision=withdrawn") || strings.Contains(guardedText, "decision=allow") {
		t.Fatalf("public=%q guarded=%q", publicText, guardedText)
	}
	if intFromAny(packet["candidate_count"], 0) != 1 ||
		intFromAny(mapFromAny(packet["dropped_counts"])["superseded_by_later_source_turn"], 0) != 1 {
		t.Fatalf("packet=%#v", packet)
	}
}

func TestActiveInteractionProjectionWithholdsPrivateBoundaryWithoutResolvedActorPOV(t *testing.T) {
	private := interactionProjectionUnit(
		"private", "boundary", interactionBoundaryContract,
		"alice-id", "bob-id", "Alice", "Bob", "owner_private", 2,
		map[string]any{"action_scope": "touch", "decision": "refuse", "effective_scope": "event"},
	)
	for _, perspective := range []map[string]any{
		{"identity_state": "needs_review"},
		{"identity_state": "resolved", "current_pov_entity_id": "carol-id"},
	} {
		packet, publicText, guardedText := buildPrepareTurnActiveInteractionProjection(
			[]store.PreciseMemoryUnit{private}, perspective, "Alice refuses.", []string{"Alice"}, 3, true,
		)
		if publicText != "" || guardedText != "" || intFromAny(packet["candidate_count"], 0) != 0 {
			t.Fatalf("perspective=%#v packet=%#v public=%q guarded=%q", perspective, packet, publicText, guardedText)
		}
		if intFromAny(mapFromAny(packet["dropped_counts"])["resolved_actor_pov_required"], 0) != 1 {
			t.Fatalf("missing POV drop: %#v", packet)
		}
	}
}

func TestActiveInteractionProjectionDeliversRelevantPublicDirectionalRelation(t *testing.T) {
	relation := interactionProjectionUnit(
		"relation", "observation", relationshipObservationContract,
		"alice-id", "bob-id", "Alice", "Bob", "public", 2,
		map[string]any{
			"domain": "trust", "observation": "Alice explicitly trusts Bob.",
			"support_kind": "explicit_statement",
		},
	)
	packet, publicText, guardedText := buildPrepareTurnActiveInteractionProjection(
		[]store.PreciseMemoryUnit{relation}, nil, "Alice asks Bob for help.", nil, 3, true,
	)
	if guardedText != "" || !strings.Contains(publicText, "Alice -> Bob") ||
		!strings.Contains(publicText, "domain=trust") || strings.Contains(publicText, "Bob -> Alice") ||
		strings.Contains(publicText, "source_ref=") || strings.Contains(publicText, "precise_memory:") {
		t.Fatalf("packet=%#v public=%q guarded=%q", packet, publicText, guardedText)
	}
	if !strings.Contains(publicText, "source_turn=2") {
		t.Fatalf("semantic source turn was removed with opaque source identity: %q", publicText)
	}
	if intFromAny(packet["candidate_count"], 0) != 1 {
		t.Fatalf("packet=%#v", packet)
	}
}

func TestActiveInteractionProjectionDoesNotReusePastEventAllow(t *testing.T) {
	allow := interactionProjectionUnit(
		"allow", "boundary", interactionBoundaryContract,
		"alice-id", "bob-id", "Alice", "Bob", "public", 2,
		map[string]any{"action_scope": "touch", "decision": "allow", "effective_scope": "event"},
	)
	packet, publicText, guardedText := buildPrepareTurnActiveInteractionProjection(
		[]store.PreciseMemoryUnit{allow}, nil, "Alice meets Bob again.", nil, 3, true,
	)
	if publicText != "" || guardedText != "" ||
		intFromAny(mapFromAny(packet["dropped_counts"])["event_scoped_allow_not_current"], 0) != 1 {
		t.Fatalf("packet=%#v public=%q guarded=%q", packet, publicText, guardedText)
	}
}

func TestActiveInteractionProjectionUsesExistingMemoryDeliveryBudget(t *testing.T) {
	relation := interactionProjectionUnit(
		"relation", "observation", relationshipObservationContract,
		"alice-id", "bob-id", "Alice", "Bob", "public", 2,
		map[string]any{
			"domain": "respect", "observation": "Alice explicitly respects Bob.",
			"support_kind": "explicit_statement",
		},
	)
	packet, publicCandidate, guardedCandidate := buildPrepareTurnActiveInteractionProjection(
		[]store.PreciseMemoryUnit{relation}, nil, "Alice greets Bob.", nil, 3, true,
	)
	perspective := map[string]any{
		"_active_interaction_public_text":     publicCandidate,
		"_active_interaction_guarded_text":    guardedCandidate,
		"_active_interaction_candidate_count": intFromAny(packet["candidate_count"], 0),
	}
	assembly := buildPrepareTurnInjectionAssemblyWithBudget(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		1, 3000, "Alice greets Bob.", "default", nil, nil, nil, "auto", nil, perspective,
	)
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	packet, publicText, guardedText := finalizePrepareTurnActiveInteractionProjection(
		packet, publicCandidate, guardedCandidate, finalText,
	)
	if guardedText != "" || !strings.Contains(publicText, "domain=respect") ||
		intFromAny(packet["selected_count"], 0) != 1 ||
		intFromAny(assembly.MemoryDeliveryPlan["used_chars"], 0) > intFromAny(assembly.MemoryDeliveryPlan["delivery_cap_chars"], 0) {
		t.Fatalf("packet=%#v plan=%#v public=%q guarded=%q", packet, assembly.MemoryDeliveryPlan, publicText, guardedText)
	}

	tinyAssembly := buildPrepareTurnInjectionAssemblyWithBudget(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		1, 1, "Alice greets Bob.", "default", nil, nil, nil, "auto", nil, perspective,
	)
	tinyPacket, tinyPublic, tinyGuarded := finalizePrepareTurnActiveInteractionProjection(
		packet, publicCandidate, guardedCandidate,
		extractionStringFromAny(tinyAssembly.MemoryDeliveryPlan["final_text"]),
	)
	if tinyPublic != "" || tinyGuarded != "" ||
		extractionStringFromAny(tinyPacket["status"]) != "deferred_by_memory_delivery_plan" ||
		intFromAny(tinyPacket["selected_count"], -1) != 0 {
		t.Fatalf("tiny packet=%#v public=%q guarded=%q plan=%#v", tinyPacket, tinyPublic, tinyGuarded, tinyAssembly.MemoryDeliveryPlan)
	}
}

func TestPrepareTurnRouteReadsAndDeliversActiveInteractionProjection(t *testing.T) {
	relation := interactionProjectionUnit(
		"route-relation", "observation", relationshipObservationContract,
		"alice-id", "bob-id", "Alice", "Bob", "public", 2,
		map[string]any{
			"domain": "respect", "observation": "Alice explicitly respects Bob.",
			"support_kind": "explicit_statement",
		},
	)
	relation.ChatSessionID = "session-route-interaction"
	fake := &turnRecordingStore{returnActiveInteractions: []store.PreciseMemoryUnit{relation}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"session-route-interaction","turn_index":3,"raw_user_input":"Alice asks Bob for help.","settings":{"max_injection_chars":3000,"max_input_context_chars":0,"injection_enabled":true,"input_context_enabled":false,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	response := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(extractionStringFromAny(response["injection_text"]), "Alice -> Bob") {
		t.Fatalf("active interaction missing from injection: %#v", response)
	}
	packet := mapFromAny(mapFromAny(response["injection_pack"])["active_interaction_packet"])
	if extractionStringFromAny(packet["status"]) != "ready" ||
		intFromAny(packet["selected_count"], 0) != 1 {
		t.Fatalf("active interaction packet=%#v", packet)
	}
}
