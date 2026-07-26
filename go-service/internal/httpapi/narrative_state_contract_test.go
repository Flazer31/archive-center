package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestNormalizeNarrativeStateClaimsSeparatesFactAndBelief(t *testing.T) {
	extraction := map[string]any{
		"state_claims": []any{map[string]any{
			"subject": "A", "subject_type": "character", "state_slot": "life_status", "value": "alive", "evidence_excerpt": "A opened their eyes.",
		}},
		"belief_updates": []any{map[string]any{
			"perspective_owner": "B", "subject": "A", "state_slot": "life_status", "value": "dead", "evidence_excerpt": "B still believed A was dead.",
		}},
	}
	claims := normalizeNarrativeStateClaims(extraction)
	if len(claims) != 2 {
		t.Fatalf("claims=%d, want 2", len(claims))
	}
	if claims[0].ClaimScope != "objective" || claims[0].PerspectiveOwner != "" {
		t.Fatalf("objective claim normalized incorrectly: %#v", claims[0])
	}
	if claims[1].ClaimScope != "belief" || claims[1].PerspectiveOwner != "B" {
		t.Fatalf("belief claim normalized incorrectly: %#v", claims[1])
	}
}

func TestSaveNarrativeStateKeepsOneCurrentValueAndLinksEvidence(t *testing.T) {
	ctx := context.Background()
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	now := time.Now().UTC()
	evidence := []store.DirectEvidence{{ID: 77, ChatSessionID: "sess", TurnAnchor: 1, EvidenceText: "A was believed dead."}}
	first := map[string]any{"state_claims": []any{map[string]any{
		"subject": "A", "subject_type": "character", "state_slot": "life_status", "value": "dead", "transition": "set", "evidence_excerpt": "A was believed dead.",
	}}}
	result := artifactSaveResult{}
	srv.saveNarrativeStateFromExtraction(ctx, "sess", 1, first, "The witnesses lowered their heads. A was believed dead. The room fell silent.", evidence, now, &result)
	if len(st.returnStatusCurrent) != 1 || len(st.savedStatusEvents) != 1 {
		t.Fatalf("first write current=%d events=%d", len(st.returnStatusCurrent), len(st.savedStatusEvents))
	}
	var evidencePayload map[string]any
	if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].EvidenceJSON), &evidencePayload); err != nil {
		t.Fatal(err)
	}
	ids, _ := evidencePayload["direct_evidence_ids"].([]any)
	if len(ids) != 1 || int(ids[0].(float64)) != 77 {
		t.Fatalf("evidence ids=%v, want [77]", evidencePayload["direct_evidence_ids"])
	}

	evidence = append(evidence, store.DirectEvidence{ID: 88, ChatSessionID: "sess", TurnAnchor: 2, EvidenceText: "A returned alive."})
	second := map[string]any{"state_claims": []any{map[string]any{
		"subject": "A", "subject_type": "character", "state_slot": "life_status", "value": "alive", "transition": "reversal", "evidence_excerpt": "A returned alive.",
	}}}
	srv.saveNarrativeStateFromExtraction(ctx, "sess", 2, second, "The door opened without warning. A returned alive. B dropped the cup.", evidence, now.Add(time.Minute), &result)
	if len(st.returnStatusCurrent) != 1 {
		t.Fatalf("current rows=%d, want one replaced row", len(st.returnStatusCurrent))
	}
	if len(st.savedStatusEvents) != 2 {
		t.Fatalf("events=%d, want 2", len(st.savedStatusEvents))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].ValueJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["value"] != "alive" || payload["previous_value"] != "dead" {
		t.Fatalf("current payload=%v", payload)
	}

	srv.saveNarrativeStateFromExtraction(ctx, "sess", 3, second, "Everyone saw it clearly. A returned alive. No one could deny it.", evidence, now.Add(2*time.Minute), &result)
	if len(st.savedStatusCurrent) != 2 || len(st.savedStatusEvents) != 2 {
		t.Fatalf("exact reaffirm should skip writes: current writes=%d events=%d", len(st.savedStatusCurrent), len(st.savedStatusEvents))
	}
}

func TestSaveNarrativeStateRejectsNonFinalReplacementAndRequiresExplicitReactivation(t *testing.T) {
	ctx := context.Background()
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	now := time.Now().UTC()
	evidence := []store.DirectEvidence{}
	save := func(turn int, value, transition string, confidence float64) {
		t.Helper()
		excerpt := "Project Meridian state evidence " + value
		evidence = append(evidence, store.DirectEvidence{
			ID:            int64(turn),
			ChatSessionID: "sess-transition-fence",
			TurnAnchor:    turn,
			EvidenceText:  excerpt,
		})
		extraction := map[string]any{"state_claims": []any{map[string]any{
			"subject":          "Project Meridian",
			"subject_type":     "entity",
			"state_slot":       "goal_status",
			"value":            value,
			"claim_scope":      "objective",
			"transition":       transition,
			"confidence":       confidence,
			"evidence_excerpt": excerpt,
		}}}
		result := artifactSaveResult{}
		srv.saveNarrativeStateFromExtraction(ctx, "sess-transition-fence", turn, extraction, "Before the update. "+excerpt+". After the update.", evidence, now.Add(time.Duration(turn)*time.Minute), &result)
	}
	currentPayload := func() map[string]any {
		t.Helper()
		if len(st.returnStatusCurrent) != 1 {
			t.Fatalf("current rows=%d, want 1", len(st.returnStatusCurrent))
		}
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].ValueJSON), &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}

	save(1, "active assignment", "set", 0.9)
	for index, attempt := range []struct {
		value      string
		transition string
		confidence float64
	}{
		{value: "possibly deferred", transition: "uncertain", confidence: 0.9},
		{value: "different reaffirmation", transition: "reaffirm", confidence: 0.9},
		{value: "implicit replacement", transition: "set", confidence: 0.9},
		{value: "low confidence completion", transition: "complete", confidence: 0.2},
	} {
		save(index+2, attempt.value, attempt.transition, attempt.confidence)
		if got := extractionStringFromAny(currentPayload()["value"]); got != "active assignment" {
			t.Fatalf("%s replaced the current value with %q", attempt.transition, got)
		}
	}

	save(6, "active assignment", "defer", 0.9)
	if payload := currentPayload(); payload["value"] != "active assignment" || payload["transition"] != "defer" {
		t.Fatalf("same-value terminal transition was not recorded: %v", payload)
	}
	save(7, "abandoned assignment", "abandon", 0.9)
	save(8, "completed assignment", "complete", 0.9)
	save(9, "completed assignment", "supersede", 0.9)
	if payload := currentPayload(); payload["value"] != "completed assignment" || payload["transition"] != "supersede" {
		t.Fatalf("explicit terminal-to-terminal transitions were frozen: %v", payload)
	}
	save(10, "active without reopen", "change", 0.9)
	if got := extractionStringFromAny(currentPayload()["value"]); got != "completed assignment" {
		t.Fatalf("closed state reopened without explicit reopen/resume/correction: %q", got)
	}
	save(11, "active after explicit reopen", "reopen", 0.9)
	if got := extractionStringFromAny(currentPayload()["value"]); got != "active after explicit reopen" {
		t.Fatalf("explicit reopen did not replace closed state: %q", got)
	}
	save(12, "completed again", "complete", 0.9)
	save(13, "reversed by direct evidence", "reversal", 0.9)
	if got := extractionStringFromAny(currentPayload()["value"]); got != "reversed by direct evidence" {
		t.Fatalf("explicit reversal did not replace closed state: %q", got)
	}
	if len(st.savedStatusCurrent) != 8 || len(st.savedStatusEvents) != 8 {
		t.Fatalf("writes current=%d events=%d, want 8 explicit state writes", len(st.savedStatusCurrent), len(st.savedStatusEvents))
	}
}

func TestSaveNarrativeStateHighConfidenceTransitionReplacesLowConfidenceCurrent(t *testing.T) {
	claim := narrativeStateClaim{
		Subject: "Project Meridian", SubjectType: "entity", StateSlot: "goal_status",
		Value: "uncertain state", ClaimScope: "objective", Transition: "set", Confidence: 0.2,
	}
	st := &turnRecordingStore{returnStatusCurrent: []store.StatusCurrentValue{{
		ID: 1, ChatSessionID: "sess-low-current", RegistryID: 1, StatusKey: narrativeStateStatusKey,
		OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), OwnerLabel: narrativeStateOwnerLabel(claim),
		ValueKind: "note", ValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", 1)),
		SourceTurn: 1, WriteState: "current",
	}}}
	srv := &Server{Store: st}
	result := artifactSaveResult{}
	srv.saveNarrativeStateFromExtraction(
		context.Background(), "sess-low-current", 2,
		map[string]any{"state_claims": []any{map[string]any{
			"subject": "Project Meridian", "subject_type": "entity", "state_slot": "goal_status",
			"value": "completed", "claim_scope": "objective", "transition": "complete",
			"confidence": 0.9, "evidence_excerpt": "Project Meridian was completed.",
		}}},
		"At noon, Project Meridian was completed. The team signed the report.",
		nil, time.Now().UTC(), &result,
	)
	if len(st.savedStatusCurrent) != 1 || len(st.savedStatusEvents) != 1 {
		t.Fatalf("high-confidence transition did not replace low-confidence current: current=%d events=%d", len(st.savedStatusCurrent), len(st.savedStatusEvents))
	}
}

func TestSaveNarrativeStateNonGoalClaimsKeepLegacyReplacementBehavior(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	save := func(turn int, objectiveValue, beliefValue string, confidence float64) {
		t.Helper()
		objectiveEvidence := "A objective evidence " + objectiveValue
		beliefEvidence := "B belief evidence " + beliefValue
		extraction := map[string]any{
			"state_claims": []any{map[string]any{
				"subject": "A", "subject_type": "character", "state_slot": "life_status",
				"value": objectiveValue, "transition": "set", "confidence": confidence,
				"evidence_excerpt": objectiveEvidence,
			}},
			"belief_updates": []any{map[string]any{
				"subject": "A", "subject_type": "character", "state_slot": "life_status",
				"value": beliefValue, "perspective_owner": "B", "transition": "set", "confidence": confidence,
				"evidence_excerpt": beliefEvidence,
			}},
		}
		result := artifactSaveResult{}
		srv.saveNarrativeStateFromExtraction(
			context.Background(), "sess-non-goal", turn, extraction,
			objectiveEvidence+". "+beliefEvidence+".", nil, time.Now().UTC(), &result,
		)
	}
	save(1, "alive", "missing", 0.9)
	save(2, "recovering", "alive", 0.2)
	if len(st.savedStatusCurrent) != 4 || len(st.savedStatusEvents) != 4 {
		t.Fatalf("goal-only lifecycle fence changed non-goal fact/belief writes: current=%d events=%d", len(st.savedStatusCurrent), len(st.savedStatusEvents))
	}
}

func TestContinuityCorrectionNewLifecycleCarryIsGoalOnly(t *testing.T) {
	nonGoal := narrativeTestGroundedCurrentValue("character", "A", "life_status", "stable", "", "objective", "", "complete", 0.9, 30)
	belief := narrativeTestGroundedCurrentValue("character", "A", "life_status", "safe", "", "belief", "B", "resume", 0.9, 30)
	for name, value := range map[string]store.StatusCurrentValue{"non_goal": nonGoal, "belief": belief} {
		view := narrativeCurrentStateViews([]store.StatusCurrentValue{value})
		if len(view) != 1 {
			t.Fatalf("%s view missing", name)
		}
		if narrativeCorrectionTransitionNeedsCarry(view[0].Payload) {
			t.Fatalf("%s gained goal lifecycle carry authority: %#v", name, view[0].Payload)
		}
	}
	nonGoalAssembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "A and B continue talking.", "default", nil, nil, nil,
		prepareTurnPerspectiveWithNarrativeState(map[string]any{}, []store.StatusCurrentValue{nonGoal, belief}, nil),
	)
	if nonGoalAssembly.ContinuityCorrectionText != "" {
		t.Fatalf("non-goal terminal enum changed continuity carry behavior: %q", nonGoalAssembly.ContinuityCorrectionText)
	}

	goal := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "", "objective", "", "complete", 0.9, 30)
	goalAssembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "Atlas Restoration is discussed.", "default", nil, nil, nil,
		prepareTurnPerspectiveWithNarrativeState(map[string]any{}, []store.StatusCurrentValue{goal}, nil),
	)
	if !strings.Contains(goalAssembly.ContinuityCorrectionText, "Atlas Restoration / goal_status: completed") {
		t.Fatalf("structured goal terminal carry missing: %q", goalAssembly.ContinuityCorrectionText)
	}
}

func TestNarrativeStateInjectionDropsOffSceneBelief(t *testing.T) {
	values := []store.StatusCurrentValue{
		narrativeTestCurrentValue("A", "life_status", "alive", "objective", "", 10),
		narrativeTestCurrentValue("A", "life_status", "dead", "belief", "B", 9),
		narrativeTestCurrentValue("A", "life_status", "dangerous", "belief", "C", 8),
	}
	chatLogs := []store.ChatLog{{TurnIndex: 10, Role: "assistant", Content: "A met B at the gate."}}
	facts, perceptions, dropped := filterNarrativeCurrentStateViews(values, "B asks whether A survived.", chatLogs, nil)
	if len(facts) != 1 || len(perceptions) != 1 || dropped != 1 {
		t.Fatalf("facts=%d perceptions=%d dropped=%d", len(facts), len(perceptions), dropped)
	}
	if perceptions[0].Perspective != "B" {
		t.Fatalf("perspective=%q, want B", perceptions[0].Perspective)
	}
}

func TestPrepareTurnCurrentStateTransitionSuppressesOlderActiveArtifacts(t *testing.T) {
	const (
		subject  = "Atlas Restoration"
		previous = "active"
	)
	current := narrativeTestGroundedCurrentValue("entity", subject, "goal_status", "completed", previous, "objective", "", "resolve", 0.9, 30)
	activeStates := []store.ActiveState{{
		ID:        1,
		StateType: "state_deltas",
		Content:   `{"scene_state":{"unresolved_threads":{"opened":[{"subject":"Atlas Restoration","state_slot":"goal_status"}]}}}`,
		TurnIndex: 10,
	}}
	pendingThreads := []store.PendingThread{{
		ID:               1,
		ThreadKey:        "atlas_assignment",
		Description:      subject,
		Status:           "open",
		CreatedTurn:      10,
		LastSeenTurn:     40,
		HookMetadataJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`,
	}}
	canonicalLayers := []store.CanonicalStateLayer{
		{
			ID:               1,
			LayerType:        "scene_state",
			Content:          `{"unresolved_threads":{"opened":[{"subject":"Atlas Restoration","state_slot":"goal_status"}]}}`,
			TurnIndex:        10,
			LastVerifiedTurn: 40,
			Confidence:       0.9,
		},
		{
			ID:         2,
			LayerType:  "world_state",
			Content:    `{"rules":[{"category":"workflow","value":"Atlas Restoration status active"}]}`,
			TurnIndex:  10,
			SourceTurn: 10,
			Confidence: 0.9,
		},
	}
	_, pendingThreads, activeStates, canonicalLayers, _ = filterPrepareTurnSupersededOpenGoals(
		[]store.StatusCurrentValue{current}, nil, pendingThreads, activeStates, canonicalLayers,
	)
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, []store.StatusCurrentValue{current}, activeStates)
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{
			ID:          1,
			TurnIndex:   10,
			SummaryJSON: `{"turn_summary":"Atlas Restoration status active"}`,
			Importance:  0.9,
		}},
		nil,
		nil,
		nil,
		nil,
		[]store.WorldRule{{
			ID:         1,
			Scope:      "system",
			Category:   "workflow",
			Key:        "atlas_assignment",
			ValueJSON:  `{"value":"Atlas Restoration status active"}`,
			SourceTurn: 10,
		}},
		nil,
		pendingThreads,
		canonicalLayers,
		nil, nil, nil, nil,
		5, 9000, "Elena asks Marco why their trust changed.", "default", nil, nil, nil, perspective,
	)

	for name, text := range map[string]string{
		"actual_memory":   assembly.ActualMemoryText,
		"world_rules":     assembly.WorldRulesText,
		"pending_threads": assembly.PendingThreadText,
		"canonical_world": assembly.CanonWorldText,
		"canonical_all":   assembly.CanonText,
		"final_delivery":  extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"]),
	} {
		if strings.Contains(text, subject) || strings.Contains(text, previous) {
			t.Errorf("%s reactivated an older superseded assignment: %q", name, text)
		}
	}
}

func TestPrepareTurnCurrentStatePreservesMultiEventAndProtectedContext(t *testing.T) {
	current := narrativeTestGroundedCurrentValue(
		"entity", "Atlas Restoration", "goal_status", "completed", "active",
		"objective", "", "complete", 0.9, 30,
	)
	perspective := prepareTurnPerspectiveWithNarrativeState(
		map[string]any{"current_pov": "Mina"},
		[]store.StatusCurrentValue{current},
		nil,
	)
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{
			{
				ID:        1,
				TurnIndex: 10,
				SummaryJSON: mustCompactJSON(map[string]any{
					"turn_summary": "Atlas Restoration remained active. Mina met Rowan at the archive.",
					"characters":   []string{"Mina", "Rowan"},
					"narrative_events": []any{
						map[string]any{"summary": "Atlas Restoration remained active."},
						map[string]any{"summary": "Mina met Rowan at the archive."},
					},
				}),
				Importance: 0.9,
			},
			{
				ID:        2,
				TurnIndex: 11,
				SummaryJSON: mustCompactJSON(map[string]any{
					"turn_summary": "Mina reviewed a hidden plan.",
					"characters":   []string{"Mina"},
					"protected_secrets": []any{map[string]any{
						"owner":             "Mina",
						"secret_kind":       "hidden_plan",
						"secret_summary":    "RAW_HIDDEN_PLAN",
						"disclosure_policy": "owner_private_until_revealed",
						"knowledge_scope":   map[string]any{"known_by": []string{"Mina"}},
					}},
				}),
				Importance: 0.8,
			},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "Mina reflects on the archive meeting.", "default", nil, nil, nil, perspective,
	)
	if !boolFromAny(assembly.Counts["protected_perspective_recognized"]) {
		t.Fatalf("early current-state filtering discarded multi-event protected context: counts=%#v", assembly.Counts)
	}
	if strings.Contains(assembly.Text, "RAW_HIDDEN_PLAN") {
		t.Fatalf("preserved protected context leaked raw secret content: %q", assembly.Text)
	}
	if !strings.Contains(assembly.ActualMemoryText, "Mina met Rowan at the archive") ||
		!strings.Contains(extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"]), "Mina met Rowan at the archive") {
		t.Fatalf("non-conflicting durable event was removed from delivery: actual=%q final=%q", assembly.ActualMemoryText, assembly.MemoryDeliveryPlan["final_text"])
	}
}

func TestNarrativeStateOpenIdentityMatchIsExact(t *testing.T) {
	current := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "active", "objective", "", "complete", 0.9, 30)
	if !narrativeCurrentStateSupersedesOpenArtifact(
		[]store.StatusCurrentValue{current}, 10,
		`{"subject":"Atlas Restoration","state_slot":"goal_status"}`,
	) {
		t.Fatal("exact structured open-goal identity was not superseded")
	}
	if narrativeCurrentStateSupersedesOpenArtifact(
		[]store.StatusCurrentValue{current}, 10,
		`{"subject":"Atlas Restoration","state_slot":"assignment_status"}`,
	) {
		t.Fatal("the same subject with a different state slot was suppressed")
	}
	if narrativeCurrentStateSupersedesOpenArtifact(
		[]store.StatusCurrentValue{current}, 10,
		`{"subject":"Atlas Restoration Planning","state_slot":"goal_status"}`,
	) {
		t.Fatal("a different longer goal was collapsed by substring matching")
	}
	if narrativeCurrentStateSupersedesOpenArtifact([]store.StatusCurrentValue{current}, 10, "Atlas Restoration") {
		t.Fatal("subject-only text gained open-goal suppression authority")
	}
	firstTerminal := narrativeTestGroundedCurrentValue("entity", "First Terminal Goal", "goal_status", "completed", "", "objective", "", "complete", 0.9, 30)
	if !narrativeCurrentStateSupersedesOpenArtifact(
		[]store.StatusCurrentValue{firstTerminal}, 10,
		`{"subject":"First Terminal Goal","state_slot":"goal_status"}`,
	) {
		t.Fatal("first grounded terminal claim did not suppress its exact older open artifact")
	}
}

func TestPrepareTurnSupersededOpenGoalFilterUsesStructuralExactIdentity(t *testing.T) {
	current := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "", "objective", "", "complete", 0.9, 30)
	storylines := []store.Storyline{
		{ID: 1, Name: "Atlas Restoration", Status: "active", CurrentContext: "STALE_STORYLINE", OngoingTensionsJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`, LastEvidenceTurn: 10, LastTurn: 40},
		{ID: 2, Name: "Atlas Restoration Survey", Status: "active", CurrentContext: "SUBGOAL_STORYLINE", OngoingTensionsJSON: `{"subject":"Atlas Restoration Survey","state_slot":"goal_status"}`, LastEvidenceTurn: 10},
		{ID: 3, Name: "Atlas Restoration", Status: "active", CurrentContext: "PINNED_STORYLINE", OngoingTensionsJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`, LastEvidenceTurn: 10, Pinned: true},
		{ID: 4, Name: "Atlas Restoration", Status: "active", CurrentContext: "LEGACY_STORYLINE", OngoingTensionsJSON: `{"title":"Atlas Restoration"}`, LastEvidenceTurn: 10},
	}
	pendingThreads := []store.PendingThread{
		{ID: 1, Title: "Atlas Restoration", Status: "open", CreatedTurn: 10, LastSeenTurn: 40, HookMetadataJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`},
		{ID: 2, Title: "Atlas Restoration Survey", Status: "open", CreatedTurn: 10, HookMetadataJSON: `{"subject":"Atlas Restoration Survey","state_slot":"goal_status"}`},
		{ID: 3, Title: "Atlas Restoration", Status: "open", CreatedTurn: 10, UserCorrected: true, HookMetadataJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`},
		{ID: 4, Title: "Atlas Restoration", Status: "open", CreatedTurn: 10, HookMetadataJSON: `{"title":"Atlas Restoration"}`},
	}
	activeStates := []store.ActiveState{
		{ID: 1, StateType: "unresolved_threads", Content: `{"title":"Atlas Restoration","state_slot":"goal_status","status":"open"}`, TurnIndex: 10},
		{ID: 2, StateType: "state_deltas", Content: `{"unresolved_threads":{"opened":[{"title":"Atlas Restoration","state_slot":"goal_status","status":"open"},{"title":"Atlas Restoration Survey","state_slot":"goal_status","status":"open"}]}}`, TurnIndex: 10},
		{ID: 3, StateType: "unrelated", Content: "", TurnIndex: 10},
		{ID: 4, StateType: "unresolved_threads", Content: `{"title":"Atlas Restoration","status":"open"}`, TurnIndex: 10},
	}
	canonicalLayers := []store.CanonicalStateLayer{
		{ID: 1, LayerType: "unresolved_threads", Content: `{"title":"Atlas Restoration","state_slot":"goal_status","status":"open"}`, TurnIndex: 10, LastVerifiedTurn: 40},
		{ID: 2, LayerType: "scene_state", Content: `{"unresolved_threads":{"opened":[{"title":"Atlas Restoration","state_slot":"goal_status","status":"open"},{"title":"Atlas Restoration Survey","state_slot":"goal_status","status":"open"}]}}`, TurnIndex: 10},
		{ID: 3, LayerType: "unrelated", Content: "", TurnIndex: 10},
		{ID: 4, LayerType: "unresolved_threads", Content: `{"title":"Atlas Restoration","status":"open"}`, TurnIndex: 10},
	}

	storylines, pendingThreads, activeStates, canonicalLayers, trace := filterPrepareTurnSupersededOpenGoals(
		[]store.StatusCurrentValue{current}, storylines, pendingThreads, activeStates, canonicalLayers,
	)
	if len(storylines) != 3 || storylines[0].Name != "Atlas Restoration Survey" || !storylines[1].Pinned ||
		storylines[2].CurrentContext != "LEGACY_STORYLINE" {
		t.Fatalf("storyline exact/user-owned filtering mismatch: %#v", storylines)
	}
	if len(pendingThreads) != 3 || pendingThreads[0].Title != "Atlas Restoration Survey" || !pendingThreads[1].UserCorrected ||
		pendingThreads[2].ID != 4 {
		t.Fatalf("pending exact/user-owned filtering mismatch: %#v", pendingThreads)
	}
	if len(activeStates) != 3 || activeStates[1].StateType != "unrelated" || activeStates[2].ID != 4 ||
		strings.Contains(activeStates[0].Content, `"title":"Atlas Restoration"`) ||
		!strings.Contains(activeStates[0].Content, "Atlas Restoration Survey") {
		t.Fatalf("active-state structured pruning mismatch: %#v", activeStates)
	}
	if len(canonicalLayers) != 3 || canonicalLayers[1].LayerType != "unrelated" || canonicalLayers[2].ID != 4 ||
		strings.Contains(canonicalLayers[0].Content, `"title":"Atlas Restoration"`) ||
		!strings.Contains(canonicalLayers[0].Content, "Atlas Restoration Survey") {
		t.Fatalf("canonical structured pruning mismatch: %#v", canonicalLayers)
	}
	if intFromAny(trace["storylines_dropped"], 0) != 1 ||
		intFromAny(trace["pending_threads_dropped"], 0) != 1 ||
		intFromAny(trace["active_states_dropped"], 0) != 1 ||
		intFromAny(trace["canonical_layers_dropped"], 0) != 1 ||
		intFromAny(trace["user_owned_items_preserved"], 0) != 2 {
		t.Fatalf("filter trace mismatch: %#v", trace)
	}
}

func TestPendingThreadProducerMirrorsGoalIdentityIntoPrepareFilter(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	result := artifactSaveResult{}
	cost := canonicalStateWriteCostMeasurement{}
	srv.saveCharacterAndStateArtifacts(
		context.Background(), "sess-producer-goal", 10,
		map[string]any{"pending_threads": []any{map[string]any{
			"title": "Production Goal", "subject": "Production Goal", "state_slot": "goal_status",
			"thread_type": "open_question", "confidence": 0.9,
		}}},
		completeTurnEmbeddingConfig{}, time.Now().UTC(), &result, nil, &cost,
	)
	if len(st.savedPendingThreads) != 1 || len(st.savedActiveStates) != 1 ||
		len(st.savedCanonicalLayers) != 1 || len(st.savedStorylines) != 1 {
		t.Fatalf(
			"producer rows pending=%d active=%d canonical=%d storylines=%d",
			len(st.savedPendingThreads), len(st.savedActiveStates), len(st.savedCanonicalLayers), len(st.savedStorylines),
		)
	}
	for name, content := range map[string]string{
		"active":    st.savedActiveStates[0].Content,
		"canonical": st.savedCanonicalLayers[0].Content,
	} {
		payload := parseJSONMap(content)
		if payload["subject"] != "Production Goal" || payload["state_slot"] != "goal_status" {
			t.Fatalf("%s producer did not mirror exact goal identity: %#v", name, payload)
		}
	}

	storylines := []store.Storyline{*st.savedStorylines[0]}
	pendingThreads := []store.PendingThread{*st.savedPendingThreads[0]}
	activeStates := []store.ActiveState{*st.savedActiveStates[0]}
	canonicalLayers := []store.CanonicalStateLayer{*st.savedCanonicalLayers[0]}
	current := narrativeTestGroundedCurrentValue("entity", "Production Goal", "goal_status", "completed", "", "objective", "", "complete", 0.9, 30)
	storylines, pendingThreads, activeStates, canonicalLayers, trace := filterPrepareTurnSupersededOpenGoals(
		[]store.StatusCurrentValue{current}, storylines, pendingThreads, activeStates, canonicalLayers,
	)
	if len(storylines)+len(pendingThreads)+len(activeStates)+len(canonicalLayers) != 0 {
		t.Fatalf(
			"producer artifacts survived exact newer terminal current: storylines=%#v pending=%#v active=%#v canonical=%#v",
			storylines, pendingThreads, activeStates, canonicalLayers,
		)
	}
	if intFromAny(trace["storylines_dropped"], 0) != 1 ||
		intFromAny(trace["pending_threads_dropped"], 0) != 1 ||
		intFromAny(trace["active_states_dropped"], 0) != 1 ||
		intFromAny(trace["canonical_layers_dropped"], 0) != 1 {
		t.Fatalf("producer-to-prepare filter trace mismatch: %#v", trace)
	}
}

func TestPendingThreadProducerIdentityMismatchFailsOpen(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Store: st}
	result := artifactSaveResult{}
	cost := canonicalStateWriteCostMeasurement{}
	srv.saveCharacterAndStateArtifacts(
		context.Background(), "sess-producer-mismatch", 10,
		map[string]any{"pending_threads": []any{map[string]any{
			"title": "Production Goal", "subject": "Different Goal", "state_slot": "goal_status",
			"thread_type": "open_question", "confidence": 0.9,
		}}},
		completeTurnEmbeddingConfig{}, time.Now().UTC(), &result, nil, &cost,
	)
	if len(st.savedPendingThreads) != 1 || len(st.savedActiveStates) != 1 ||
		len(st.savedCanonicalLayers) != 1 || len(st.savedStorylines) != 1 {
		t.Fatalf("mismatch producer rows were not preserved")
	}
	for name, content := range map[string]string{
		"active":    st.savedActiveStates[0].Content,
		"canonical": st.savedCanonicalLayers[0].Content,
	} {
		payload := parseJSONMap(content)
		if payload["subject"] != nil || payload["state_slot"] != nil {
			t.Fatalf("%s producer mirrored mismatched goal identity: %#v", name, payload)
		}
	}
	current := narrativeTestGroundedCurrentValue("entity", "Production Goal", "goal_status", "completed", "", "objective", "", "complete", 0.9, 30)
	storylines, pendingThreads, activeStates, canonicalLayers, _ := filterPrepareTurnSupersededOpenGoals(
		[]store.StatusCurrentValue{current},
		[]store.Storyline{*st.savedStorylines[0]},
		[]store.PendingThread{*st.savedPendingThreads[0]},
		[]store.ActiveState{*st.savedActiveStates[0]},
		[]store.CanonicalStateLayer{*st.savedCanonicalLayers[0]},
	)
	if len(storylines) != 1 || len(pendingThreads) != 1 || len(activeStates) != 1 || len(canonicalLayers) != 1 {
		t.Fatalf(
			"mismatched producer identity did not fail open: storylines=%d pending=%d active=%d canonical=%d",
			len(storylines), len(pendingThreads), len(activeStates), len(canonicalLayers),
		)
	}
}

func TestPrepareTurnFiltersSupersededOpenGoalsBeforeAllConsumers(t *testing.T) {
	current := narrativeTestGroundedCurrentValue("entity", "Atlas Restoration", "goal_status", "completed", "", "objective", "", "complete", 0.9, 30)
	current.ChatSessionID = "sess-lineage"
	st := &turnRecordingStore{
		returnStatusCurrent: []store.StatusCurrentValue{current},
		returnStorylines: []store.Storyline{
			{ID: 1, Name: "Atlas Restoration", Status: "active", CurrentContext: "STALE_STORYLINE_LINEAGE", OngoingTensionsJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`, LastEvidenceTurn: 10},
			{ID: 2, Name: "Atlas Restoration Survey", Status: "active", CurrentContext: "KEEP_STORYLINE_LINEAGE", OngoingTensionsJSON: `{"subject":"Atlas Restoration Survey","state_slot":"goal_status"}`, LastEvidenceTurn: 10},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, Title: "Atlas Restoration", Description: "STALE_PENDING_LINEAGE", Status: "open", SourceTurn: 10, HookMetadataJSON: `{"subject":"Atlas Restoration","state_slot":"goal_status"}`},
			{ID: 2, Title: "Atlas Restoration Survey", Description: "KEEP_PENDING_LINEAGE", Status: "open", SourceTurn: 10, HookMetadataJSON: `{"subject":"Atlas Restoration Survey","state_slot":"goal_status"}`},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, StateType: "unresolved_threads", Content: `{"title":"Atlas Restoration","state_slot":"goal_status","status":"open","note":"STALE_ACTIVE_LINEAGE"}`, TurnIndex: 10},
			{ID: 2, StateType: "unresolved_threads", Content: `{"title":"Atlas Restoration Survey","state_slot":"goal_status","status":"open","note":"KEEP_ACTIVE_LINEAGE"}`, TurnIndex: 10},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, LayerType: "unresolved_threads", Content: `{"title":"Atlas Restoration","state_slot":"goal_status","status":"open","note":"STALE_CANONICAL_LINEAGE"}`, SourceTurn: 10},
			{ID: 2, LayerType: "unresolved_threads", Content: `{"title":"Atlas Restoration Survey","state_slot":"goal_status","status":"open","note":"KEEP_CANONICAL_LINEAGE"}`, SourceTurn: 10},
		},
	}
	srv := setupTestServer()
	srv.Store = st
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(
		http.MethodPost,
		"/prepare-turn",
		strings.NewReader(`{"chat_session_id":"sess-lineage","turn_index":31,"raw_user_input":"Review Atlas Restoration Survey.","settings":{"top_k":5,"max_injection_chars":9000,"injection_enabled":true,"input_context_enabled":false}}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, stale := range []string{
		"STALE_STORYLINE_LINEAGE",
		"STALE_PENDING_LINEAGE",
		"STALE_ACTIVE_LINEAGE",
		"STALE_CANONICAL_LINEAGE",
	} {
		if strings.Contains(rec.Body.String(), stale) {
			t.Fatalf("upstream filter left stale artifact for a downstream consumer %q: %s", stale, rec.Body.String())
		}
	}
	response := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	tracePreview := mapFromAny(response["trace_preview"])
	materialization := mapFromAny(tracePreview["materialization"])
	filterTrace := mapFromAny(materialization["superseded_open_goals"])
	for key, want := range map[string]int{
		"storylines_dropped":       1,
		"pending_threads_dropped":  1,
		"active_states_dropped":    1,
		"canonical_layers_dropped": 1,
	} {
		if got := intFromAny(filterTrace[key], -1); got != want {
			t.Fatalf("%s=%d want %d; trace=%#v", key, got, want, filterTrace)
		}
	}
	traceSummary := mapFromAny(mapFromAny(response["generation_packet"])["trace_summary"])
	for key, want := range map[string]int{
		"storyline_count":       1,
		"pending_thread_count":  1,
		"active_state_count":    1,
		"canonical_layer_count": 1,
	} {
		if got := intFromAny(traceSummary[key], -1); got != want {
			t.Fatalf("%s=%d want %d; trace=%#v", key, got, want, traceSummary)
		}
	}
}

func TestOpenNarrativeThreadsKeepsPausedGoalInactiveUntilExplicitReopen(t *testing.T) {
	items := []store.PendingThread{
		{ID: 1, Title: "Restore the observatory clock", Status: "paused"},
		{ID: 2, Title: "Repair the library window", Status: "open"},
	}
	active := openNarrativeThreads(items)
	if len(active) != 1 || active[0].ID != 2 {
		t.Fatalf("paused goal entered active delivery: %#v", active)
	}
	if items[0].Status != "paused" {
		t.Fatalf("paused goal was mutated instead of remaining stored: %#v", items[0])
	}

	items[0].Status = "open"
	active = openNarrativeThreads(items)
	if len(active) != 2 {
		t.Fatalf("explicitly reopened goal did not return to active delivery: %#v", active)
	}
}

func TestPrepareTurnStateTransitionDoesNotSuppressUncertainOrReactivatedOpenArtifacts(t *testing.T) {
	tests := []struct {
		name       string
		subject    string
		current    store.StatusCurrentValue
		ruleKey    string
		activeText string
	}{
		{
			name:       "uncertain research change does not close active work",
			subject:    "Orchid Research",
			current:    narrativeTestGroundedCurrentValue("entity", "Orchid Research", "goal_status", "Orchid Research may change", "Orchid Research remains active", "objective", "", "uncertain", 0.9, 30),
			ruleKey:    "orchid_research",
			activeText: "Orchid Research remains active",
		},
		{
			name:       "explicit reversal reopens completed appointment",
			subject:    "Riverside Appointment",
			current:    narrativeTestGroundedCurrentValue("entity", "Riverside Appointment", "goal_status", "Riverside Appointment is active again", "Riverside Appointment was completed", "objective", "", "reversal", 0.9, 30),
			ruleKey:    "riverside_appointment",
			activeText: "Riverside Appointment is active again",
		},
		{
			name:       "explicit reopen preserves reopened quest",
			subject:    "North Gate Quest",
			current:    narrativeTestGroundedCurrentValue("entity", "North Gate Quest", "goal_status", "North Gate Quest reopened", "North Gate Quest completed", "objective", "", "reopen", 0.9, 30),
			ruleKey:    "north_gate_quest",
			activeText: "North Gate Quest reopened",
		},
		{
			name:       "explicit resume preserves resumed study",
			subject:    "Harbor Study",
			current:    narrativeTestGroundedCurrentValue("entity", "Harbor Study", "goal_status", "Harbor Study resumed", "Harbor Study deferred", "objective", "", "resume", 0.9, 30),
			ruleKey:    "harbor_study",
			activeText: "Harbor Study resumed",
		},
		{
			name:       "low confidence terminal state cannot suppress open work",
			subject:    "Signal Survey",
			current:    narrativeTestGroundedCurrentValue("entity", "Signal Survey", "goal_status", "Signal Survey completed", "Signal Survey remains active", "objective", "", "complete", 0.2, 30),
			ruleKey:    "signal_survey",
			activeText: "Signal Survey remains active",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			activeStates := []store.ActiveState{{
				ID:        1,
				StateType: "state_deltas",
				Content: mustCompactJSON(map[string]any{"scene_state": map[string]any{"unresolved_threads": map[string]any{"opened": []any{
					map[string]any{"subject": test.subject, "state_slot": "goal_status"},
				}}}}),
				TurnIndex: 10,
			}}
			pendingThreads := []store.PendingThread{{
				ID: 1, ThreadKey: test.ruleKey, Title: test.subject, Description: test.subject,
				Status: "open", CreatedTurn: 10, SourceTurn: 10,
				HookMetadataJSON: mustCompactJSON(map[string]any{"subject": test.subject, "state_slot": "goal_status"}),
			}}
			canonicalLayers := []store.CanonicalStateLayer{{
				ID: 1, LayerType: "scene_state",
				Content: mustCompactJSON(map[string]any{"unresolved_threads": map[string]any{"opened": []any{
					map[string]any{"subject": test.subject, "state_slot": "goal_status"},
				}}}),
				TurnIndex:  10,
				SourceTurn: 10,
				Confidence: 0.9,
			}}
			_, pendingThreads, activeStates, canonicalLayers, _ = filterPrepareTurnSupersededOpenGoals(
				[]store.StatusCurrentValue{test.current}, nil, pendingThreads, activeStates, canonicalLayers,
			)
			perspective := prepareTurnPerspectiveWithNarrativeState(
				map[string]any{},
				[]store.StatusCurrentValue{test.current},
				activeStates,
			)
			assembly := buildPrepareTurnInjectionAssembly(
				nil, nil, nil, nil, nil,
				[]store.WorldRule{{
					ID:         1,
					Scope:      "session",
					Category:   "workflow",
					Key:        test.ruleKey,
					ValueJSON:  mustCompactJSON(map[string]any{"value": test.activeText}),
					SourceTurn: 10,
				}},
				nil,
				pendingThreads,
				canonicalLayers,
				nil, nil, nil, nil,
				5, 9000, "Elena reviews "+test.subject+" with Marco.", "default", nil, nil, nil, perspective,
			)
			if !strings.Contains(assembly.WorldRulesText, test.activeText) {
				t.Fatalf("non-final or explicitly reopened state was suppressed: %q", assembly.WorldRulesText)
			}
			if !strings.Contains(assembly.PendingThreadText, test.subject) {
				t.Fatalf("open goal was suppressed by %s: pending=%q", test.current.ValueJSON, assembly.PendingThreadText)
			}
			if strings.Contains(assembly.CanonWorldText, test.subject) {
				t.Fatalf("open goal leaked into canonical world-state delivery: %q", assembly.CanonWorldText)
			}
		})
	}
}

func TestRestoreNarrativeCurrentStateUsesLatestRemainingLedgerValue(t *testing.T) {
	st := &turnRecordingStore{}
	claim := narrativeStateClaim{Subject: "A", SubjectType: "character", StateSlot: "life_status", ClaimScope: "objective"}
	ownerID := narrativeStateOwnerID(claim)
	dead := claim
	dead.Value = "dead"
	alive := claim
	alive.Value = "alive"
	st.savedStatusEvents = []store.StatusChangeEvent{
		{ID: 1, ChatSessionID: "sess", RegistryID: 9, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: ownerID, EventKind: "set", NewValueJSON: mustCompactJSON(narrativeStateValuePayload(dead, "", 4)), EvidenceJSON: `{}`, SourceTurn: 4},
		{ID: 2, ChatSessionID: "sess", RegistryID: 9, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: ownerID, EventKind: "change", NewValueJSON: mustCompactJSON(narrativeStateValuePayload(alive, "dead", 8)), EvidenceJSON: `{}`, SourceTurn: 8},
	}
	restored, err := restoreNarrativeCurrentStatesAfterRollback(context.Background(), st, "sess")
	if err != nil {
		t.Fatal(err)
	}
	if restored != 1 || len(st.returnStatusCurrent) != 1 {
		t.Fatalf("restored=%d current=%d", restored, len(st.returnStatusCurrent))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(st.returnStatusCurrent[0].ValueJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["value"] != "alive" || st.returnStatusCurrent[0].SourceTurn != 8 {
		t.Fatalf("restored payload=%v turn=%d", payload, st.returnStatusCurrent[0].SourceTurn)
	}
}

func TestPrepareTurnAssemblyAppendsOnlyNeededContinuityCorrection(t *testing.T) {
	values := []store.StatusCurrentValue{
		narrativeTestCurrentValueWithPrevious("Alice", "life_status", "alive", "dead", "objective", "", "reversal", 20),
		narrativeTestCurrentValue("Alice", "life_status", "dead", "belief", "Cara", 18),
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, values, nil)
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 2, TurnIndex: 19, SummaryJSON: `{"turn_summary":"Alice is dead at the archive gate"}`, Importance: 0.8}},
		nil, nil, []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "Alice spoke with Bryn."}}, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "Bryn asks Alice about the archive gate.", "default", nil, nil, nil, perspective,
	)
	correctionIndex := strings.Index(assembly.Text, "[Continuity Correction]")
	if correctionIndex < 0 || !strings.HasSuffix(assembly.Text, assembly.ContinuityCorrectionText) {
		t.Fatalf("continuity correction must follow the 2.5 memory assembly:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.ActualMemoryText, "Alice is dead at the archive gate") {
		t.Fatalf("historical event memory was removed instead of being distinguished by current correction:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.ContinuityCorrectionText, "Alice / life_status: alive") {
		t.Fatalf("current correction missing:\n%s", assembly.ContinuityCorrectionText)
	}
	if strings.Contains(assembly.Text, "Cara believes") {
		t.Fatalf("off-scene perspective leaked into injection:\n%s", assembly.Text)
	}
}

func TestContinuityCorrectionDoesNotUsePreviousAssistantRawAsSearchEvidence(t *testing.T) {
	values := []store.StatusCurrentValue{
		narrativeTestCurrentValueWithPrevious("A", "life_status", "alive", "dead", "objective", "", "reversal", 20),
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, values, nil)
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 2, TurnIndex: 10, SummaryJSON: `{"turn_summary":"A is dead"}`, Importance: 0.8}},
		nil, nil, []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "A is alive and standing at the gate."}}, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "B watches A.", "default", nil, nil, nil, perspective,
	)
	if !strings.Contains(assembly.ContinuityCorrectionText, "A / life_status: alive") {
		t.Fatalf("stored current state correction was suppressed by previous assistant raw:\n%s", assembly.ContinuityCorrectionText)
	}
	trace := mapFromAny(assembly.Counts["continuity_correction"])
	if intFromAny(trace["already_present_dropped"], 0) != 0 {
		t.Fatalf("trace=%v", trace)
	}
}

func TestContinuityCorrectionDoesNotInjectUnchangedRelevantStateWithoutConflict(t *testing.T) {
	values := []store.StatusCurrentValue{narrativeTestCurrentValue("A", "location", "market", "objective", "", 20)}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, values, nil)
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "A looks around."}}, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "A takes a breath.", "default", nil, nil, nil, perspective,
	)
	if assembly.ContinuityCorrectionText != "" {
		t.Fatalf("unchanged state without a conflicting recall must not become a standing prompt:\n%s", assembly.ContinuityCorrectionText)
	}
}

func TestCurrentQueryDoesNotPromoteOldUnrelatedMemoryByImportanceAlone(t *testing.T) {
	selection := selectPrepareTurnMemoryLanes([]store.Memory{
		{ID: 1, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Ashley guarded the old tower"}`, Importance: 1.0},
		{ID: 2, TurnIndex: 40, SummaryJSON: `{"turn_summary":"The group reached the market"}`, Importance: 0.2},
	}, "Nive and Ingrid whisper nearby", 1)
	if len(selection.Deep) != 0 {
		t.Fatalf("query-present selection must not promote old memory by importance: %#v", selection.Deep)
	}
	if len(selection.Recent) != 0 {
		t.Fatalf("current query must leave unrelated recent memory out: %#v", selection.Recent)
	}
}

func narrativeTestCurrentValue(subject, slot, value, scope, perspective string, turn int) store.StatusCurrentValue {
	claim := narrativeStateClaim{Subject: subject, SubjectType: "character", StateSlot: slot, Value: value, ClaimScope: scope, PerspectiveOwner: perspective, Transition: "set", Confidence: 0.9}
	return store.StatusCurrentValue{ID: int64(turn), ChatSessionID: "sess", RegistryID: 1, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: narrativeStateOwnerID(claim), OwnerLabel: narrativeStateOwnerLabel(claim), ValueKind: "note", ValueJSON: mustCompactJSON(narrativeStateValuePayload(claim, "", turn)), EvidenceJSON: `{}`, SourceTurn: turn, WriteState: "current"}
}

func narrativeTestCurrentValueWithPrevious(subject, slot, value, previous, scope, perspective, transition string, turn int) store.StatusCurrentValue {
	return narrativeTestGroundedCurrentValue("character", subject, slot, value, previous, scope, perspective, transition, 0.9, turn)
}

func narrativeTestGroundedCurrentValue(subjectType, subject, slot, value, previous, scope, perspective, transition string, confidence float64, turn int) store.StatusCurrentValue {
	claim := narrativeStateClaim{Subject: subject, SubjectType: subjectType, StateSlot: slot, Value: value, ClaimScope: scope, PerspectiveOwner: perspective, Transition: transition, Confidence: confidence}
	return store.StatusCurrentValue{
		ID:            int64(turn),
		ChatSessionID: "sess",
		RegistryID:    1,
		StatusKey:     narrativeStateStatusKey,
		OwnerScope:    "entity",
		OwnerID:       narrativeStateOwnerID(claim),
		OwnerLabel:    narrativeStateOwnerLabel(claim),
		ValueKind:     "note",
		ValueJSON:     mustCompactJSON(narrativeStateValuePayload(claim, previous, turn)),
		EvidenceJSON: mustCompactJSON(map[string]any{
			"contract_version": narrativeStateContractVersion,
			"source_turn":      turn,
			"evidence_excerpt": value,
		}),
		SourceTurn: turn,
		WriteState: "current",
	}
}
