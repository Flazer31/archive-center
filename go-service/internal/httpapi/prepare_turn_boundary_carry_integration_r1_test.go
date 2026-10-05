package httpapi

import (
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"strings"
	"testing"
)

func TestBoundaryCarryIntegrationR1ArrayRecordDelivery(t *testing.T) {
	for _, kind := range []string{"storyline_key_points", "storyline_tensions", "episode_loops", "episode_events", "episode_relationships", "direct_lineage"} {
		t.Run(kind, func(t *testing.T) {
			in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
			linked := `[{"text":"The amber task continues","source_refs":[{"fact_ref":"F41"}]}]`
			table := "storylines"
			switch kind {
			case "storyline_key_points", "storyline_tensions":
				sl := store.Storyline{ID: 30, ChatSessionID: "carry-synthetic", CurrentContext: "Mira continues amber contingency"}
				if kind == "storyline_key_points" {
					sl.KeyPointsJSON = linked
					sl.OngoingTensionsJSON = `["public progress"]`
				} else {
					sl.KeyPointsJSON = `["public progress"]`
					sl.OngoingTensionsJSON = linked
				}
				in.Storylines = []store.Storyline{sl}
			case "episode_loops", "episode_events", "episode_relationships":
				table = "episode_summaries"
				es := store.EpisodeSummary{ID: 31, ChatSessionID: "carry-synthetic", SummaryText: "Mira continues amber contingency", FromTurn: 3, ToTurn: 5}
				switch kind {
				case "episode_loops":
					es.OpenLoopsJSON = linked
				case "episode_events":
					es.KeyEvents = linked
				case "episode_relationships":
					es.RelationshipChangesJSON = linked
				}
				in.EpisodeSummaries = []store.EpisodeSummary{es}
			case "direct_lineage":
				table = "direct_evidence_records"
				in.Evidence = []store.DirectEvidence{{ID: 32, ChatSessionID: "carry-synthetic", EvidenceText: "Mira mentions amber contingency", TurnAnchor: 5, LineageJSON: `{"citations":[{"fact_ref":"F41"}]}`}}
				in.VectorTrace = map[string]any{"search_result": "ok", "search_results": []any{map[string]any{"id": "evidence:32", "metadata": map[string]any{"source_table": table, "source_row_id": 32, "tier": "evidence"}, "similarity": .9}}}
			}
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			if table == "direct_evidence_records" {
				if !strings.Contains(out.DirectEvidenceText, "unknown_to") || !strings.Contains(out.DirectEvidenceText, "amber contingency") {
					t.Fatal("nested direct lineage lost attributed scope")
				}
				return
			}
			facts, _ := multiAgentCandidatePool(&out)
			found := false
			for _, c := range facts {
				if c.SourceTable == table {
					found = true
					row := prepareTurnMemoryModelCandidate(c, nil)
					reading := prepareTurnMemoryReadingText(c)
					// Episode child parts say "The amber task continues"; the old
					// "amber contingency" check passed via a copied protected body.
					if !strings.Contains(mustCompactJSON(row["knowledge_boundaries"]), "unknown_to") || !strings.Contains(reading, c.CompleteText) || !strings.Contains(reading, "memories:11/protected_secrets/0") {
						t.Fatalf("array-shaped record lost its own body or attributed scope: %s", reading)
					}
					if strings.Contains(reading, "Mira keeps an amber contingency") {
						t.Fatal("array-shaped child copied the protected parent's body")
					}
				}
			}
			if !found {
				t.Fatal("production downstream candidate missing")
			}
		})
	}
	// Ordinary string arrays contain no reference metadata, even if their prose
	// happens to equal a current identifier.
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	in.Storylines = []store.Storyline{{ID: 30, ChatSessionID: "carry-synthetic", CurrentContext: "Mira continues amber contingency", KeyPointsJSON: `["F41","Mira","lifecycle_key:F41"]`}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	for _, c := range facts {
		if c.SourceTable == "storylines" && len(c.KnowledgeBoundaries) > 0 {
			t.Fatal("plain array entries became inferred links")
		}
	}
}

func TestBoundaryCarryIntegrationR1CharacterFieldLinksStayLocal(t *testing.T) {
	for _, kind := range []string{"stored_field", "provenance"} {
		t.Run(kind, func(t *testing.T) {
			state := store.CharacterState{ID: 31, ChatSessionID: "integration-r1", CharacterName: "Mira", TurnIndex: 9, StatusJSON: `{"mobility":{"value":"Mira reserves her left hand"},"posture":{"value":"Mira stands by the public cart"}}`}
			if kind == "stored_field" {
				state.StatusJSON = `{"mobility":{"value":"Mira reserves her left hand","fact_ref":"F41"},"posture":{"value":"Mira stands by the public cart"}}`
			} else {
				state.FieldProvenanceJSON = `{"contract_version":"character_field_provenance.v1","fields":{"/status/mobility":{"source_turn":2,"evidence_refs":["F41"]}}}`
			}
			facts := prepareTurnPrioritySplitFact("Mira: state=" + state.StatusJSON)
			out := prepareTurnInjectionAssembly{}
			for _, f := range facts {
				seed := carryIntegrationR1Seed("character_states", 31, nil)
				seed.Fact = f
				out.PriorityFactSeeds = append(out.PriorityFactSeeds, seed)
			}
			prepareTurnAttachCharacterFieldContext(&out, 0, state, nil)
			prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{carryIntegrationR1Authority("protected_secrets", nil)}, CharacterStates: []store.CharacterState{state}})
			found := false
			for _, seed := range out.PriorityFactSeeds {
				if strings.HasSuffix(seed.Fact.SourceFieldPath, "/mobility/value") {
					found = true
					if len(seed.Fact.KnowledgeBoundaries) != 1 {
						t.Fatal("exact stored field link was lost")
					}
				}
				if strings.Contains(seed.Fact.SourceFieldPath, "/posture/") && len(seed.Fact.KnowledgeBoundaries) != 0 {
					t.Fatal("sibling field inherited a boundary without a link")
				}
			}
			if !found {
				t.Fatal("actual split field not present")
			}
		})
	}
}

func TestBoundaryCarryIntegrationR1FactAttributionInMainAndNotes(t *testing.T) {
	first := carryIntegrationR1Authority("protected_secrets", nil)
	second := carryIntegrationR1Authority("protected_secrets", map[string]any{"summary": "Tess keeps the blue route", "owner": "Tess", "knowledge_scope": map[string]any{"known_by": []string{"Tess"}, "unknown_to": []string{"Mira"}}})
	second.ID = 14
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed("pending_threads", 23, nil)}}
	prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{first, second}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "integration-r1", HookMetadataJSON: `{"fact_ref":"F41"}`}}})
	facts, _, _, _ := prepareTurnResolvePrioritySourcePool(&out, "Mira public progress", []string{"Mira public progress"}, 10, nil)
	if len(facts) != 1 {
		t.Fatal("downstream-only fixture missing")
	}
	id := facts[0].CanonicalFactID
	selection := &multiAgentSelection{Candidates: facts, Roles: []multiAgentRoleResult{{Role: facts[0].Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{id}, Reasons: map[string]string{id: "Continue the progress"}}}}}
	out.Preprocessing = selection
	plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&out, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira public progress"}, nil, nil, nil, nil)
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
	for _, text := range []string{stringFromMap(plan, "main_memory_text"), stringFromMap(notes, "final_text")} {
		// A linked boundary identifies each source fact without delivering its
		// protected body through the ordinary/preprocessing allocation.
		for _, ref := range []string{"memories:11/protected_secrets/0", "memories:14/protected_secrets/0", "unknown_to"} {
			if !strings.Contains(text, ref) {
				t.Fatalf("explicit fact attribution or named scope absent: %s", text)
			}
		}
		for _, body := range []string{"Mira keeps the amber contingency", "Tess keeps the blue route"} {
			if strings.Contains(text, body) {
				t.Fatalf("protected body bypassed its independent delivery owner: %s", text)
			}
		}
	}
	for _, b := range facts[0].KnowledgeBoundaries {
		if len(stringsFromAny(mapFromAny(b["knowledge_scope"])["known_by"])) != 1 {
			t.Fatal("fact knowers unioned")
		}
	}
	if intFromAny(mapFromAny(plan["protected_secret_budget"])["candidate_count"], 0) != 0 {
		t.Fatal("carry manufactured a separate protected card")
	}
}

func TestBoundaryCarryIntegrationR1PerspectiveProductionPayload(t *testing.T) {
	raw := map[string]any{"owner_entity_name": "Mira", "memory_text": "Mira recalls an amber instruction", "evidence_excerpt": "Mira heard an amber instruction.", "fact_ref": "F41"}
	written := subjectivePerspectiveMemoryCandidates(map[string]any{"subjective_entity_memories": []any{raw}})
	if len(written) != 1 {
		t.Fatal("production perspective writer rejected fixture")
	}
	payload := written[0].payload
	payload["knowledge_holder_entity_id"] = "mira-id"
	unit := store.PreciseMemoryUnit{UnitID: "amber-perspective", ChatSessionID: "carry-synthetic", Kind: "observation", Subtype: "subjective_memory", LifecycleState: "active", EpistemicMode: "known", KnowledgeHolderEntityID: "mira-id", SourceTurnStart: 5, SourceTurnEnd: 5, PayloadJSON: mustCompactJSON(payload)}
	packet, _ := buildCharacterPerspectivePacket([]store.PreciseMemoryUnit{unit}, map[string]any{"current_pov_entity_id": "mira-id", "identity_state": "resolved"}, 4000)
	seeds, ok := packet["_character_perspective_fact_seeds"].([]prepareTurnPriorityFactSeed)
	if !ok || len(seeds) != 1 {
		t.Fatal("actual perspective packet did not provide a recollection seed")
	}
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	in.Perspective.CharacterSeeds = seeds
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, summaries := multiAgentCandidatePool(&out)
	found := false
	for _, c := range facts {
		if c.SourceTable == "precise_memory_units" {
			found = true
			if len(c.KnowledgeBoundaries) != 1 {
				t.Fatal("actual writer/packet candidate lost linked boundary")
			}
			row := prepareTurnMemoryModelCandidate(c, nil)
			if len(prepareTurnMemoryLineageSlice(row["knowledge_boundaries"])) != 1 {
				t.Fatal("original model owner lost boundary")
			}
		}
	}
	if !found {
		t.Fatal("production perspective candidate absent")
	}
	input := multiAgentInput("subjective_relationship", facts, summaries, dto.PrepareTurnRequest{}, multiAgentSettings{}, 36000, 5, nil)
	if !strings.Contains(multiAgentModelInput(input, 1), "unknown_to") {
		t.Fatal("perspective model packet lost boundary")
	}
}

func carryIntegrationR1Authority(field string, extra map[string]any) store.Memory {
	item := map[string]any{"summary": "Mira keeps the amber contingency", "owner": "Mira", "visibility": "owner_private", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Oren"}}, "fact_ref": "F41"}
	for k, v := range extra {
		item[k] = v
	}
	return store.Memory{ID: 11, ChatSessionID: "integration-r1", TurnIndex: 2, SummaryJSON: mustCompactJSON(map[string]any{field: []any{item}})}
}
func carryIntegrationR1Seed(table string, id int64, refs []string) prepareTurnPriorityFactSeed {
	return prepareTurnPriorityFactSeed{Lane: "event_recent", SourceTable: table, SourceRowID: id, SourceRef: prepareTurnPrioritySourceRef(table, id, ""), ParentLineKey: "Mira public progress", Visibility: "general", Fact: prepareTurnPriorityMemoryFact{Text: "Mira public progress", ExplicitRefs: refs}}
}
func TestBoundaryCarryIntegrationR1ExplicitFactPositive(t *testing.T) {
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed("pending_threads", 23, nil)}}
	input := prepareTurnAssemblyInput{Memories: []store.Memory{carryIntegrationR1Authority("protected_secrets", nil)}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "integration-r1", HookMetadataJSON: `{"fact_ref":"F41"}`}}}
	prepareTurnCarryKnowledgeBoundaries(&out, input)
	if len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) != 1 {
		t.Fatal("positive exact fact link not carried")
	}
}
func TestBoundaryCarryIntegrationR1ProtectedNormalizerLinks(t *testing.T) {
	raw := map[string]any{"summary": "Mira keeps an amber contingency", "owner": "Mira", "secret_kind": "plan", "lifecycle_key": "amber-plan", "fact_ref": "F41", "source_refs": []string{"pending_threads:23"}, "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Oren"}}}
	normalized := normalizeProtectedSecrets([]any{raw})
	if len(normalized) != 1 {
		t.Fatal("normalized authority control empty")
	}
	if len(prepareTurnKnowledgeRefs(mapFromAny(normalized[0]))) == 0 {
		t.Error("existing protected-secret normalizer discards provided lifecycle/fact/source references before stored carry input")
	}
}
func TestBoundaryCarryIntegrationR1SourceMemoryReference(t *testing.T) {
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed("pending_threads", 23, nil)}}
	input := prepareTurnAssemblyInput{Memories: []store.Memory{carryIntegrationR1Authority("protected_secrets", nil)}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "integration-r1", HookMetadataJSON: `{"source_memory_id":11}`}}}
	prepareTurnCarryKnowledgeBoundaries(&out, input)
	if len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) == 0 {
		t.Error("explicit source_memory_id=11 lost the scoped source fact; no whole-turn/name link was supplied")
	}
}
func TestBoundaryCarryIntegrationR1ScopedSourceTypes(t *testing.T) {
	for _, field := range []string{"subjective_entity_memories", "belief_updates", "character_profile_observations", "character_deltas", "pending_threads", "world_rules", "reversible_states", "physical_conditions", "entity_conditions", "narrative_events", "state_claims"} {
		t.Run(field, func(t *testing.T) {
			out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed("pending_threads", 23, nil)}}
			input := prepareTurnAssemblyInput{Memories: []store.Memory{carryIntegrationR1Authority(field, nil)}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "integration-r1", HookMetadataJSON: `{"fact_ref":"F41"}`}}}
			prepareTurnCarryKnowledgeBoundaries(&out, input)
			if len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) == 0 {
				t.Error("guarded source type with exact fact ref did not carry")
			}
		})
	}
}
func TestBoundaryCarryIntegrationR1CharacterFieldShape(t *testing.T) {
	state := store.CharacterState{ID: 31, ChatSessionID: "integration-r1", CharacterName: "Mira", TurnIndex: 9, StatusJSON: `{"mobility":{"value":"Mira reserves her left hand","fact_ref":"F41"}}`, FieldProvenanceJSON: `{"contract_version":"character_field_provenance.v1","fields":{"/status/mobility":{"source_turn":2,"evidence_refs":["F41"]}}}`}
	facts := prepareTurnPrioritySplitFact(`Mira: state={"mobility":{"value":"Mira reserves her left hand","fact_ref":"F41"}}`)
	out := prepareTurnInjectionAssembly{}
	for _, fact := range facts {
		seed := carryIntegrationR1Seed("character_states", 31, nil)
		seed.Fact = fact
		out.PriorityFactSeeds = append(out.PriorityFactSeeds, seed)
	}
	prepareTurnAttachCharacterFieldContext(&out, 0, state, nil)
	prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{carryIntegrationR1Authority("protected_secrets", nil)}, CharacterStates: []store.CharacterState{state}})
	found := false
	for _, seed := range out.PriorityFactSeeds {
		if strings.HasSuffix(seed.Fact.SourcePath, "/value") {
			found = true
			if len(seed.Fact.KnowledgeBoundaries) == 0 {
				t.Errorf("real split field path=%s field=%s misses stored status fact_ref and provenance evidence_refs", seed.Fact.SourcePath, seed.Fact.SourceFieldPath)
			}
		}
	}
	if !found {
		t.Fatal("field value control missing")
	}
}
func TestBoundaryCarryIntegrationR1PerspectiveWriterShape(t *testing.T) {
	raw := map[string]any{"owner_entity_name": "Mira", "memory_text": "Mira recalls the amber instruction", "evidence_excerpt": "Mira heard the amber instruction.", "fact_ref": "F41", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Oren"}}}
	candidates := subjectivePerspectiveMemoryCandidates(map[string]any{"subjective_entity_memories": []any{raw}})
	if len(candidates) != 1 {
		t.Fatal("perspective writer fixture not accepted")
	}
	if len(prepareTurnKnowledgeRefs(candidates[0].payload)) == 0 || len(prepareTurnOwnKnowledgeBoundary(candidates[0].payload)) == 0 {
		t.Error("actual perspective_memory.v1 writer drops supplied explicit fact ref and knowledge_scope before carry reader")
	}
}
func TestBoundaryCarryIntegrationR1SummaryUncertaintyScope(t *testing.T) {
	b := []map[string]any{{"protected_fact_ref": "memories:11/protected_secrets/0", "knowledge_scope": map[string]any{"unknown_to": []string{"Oren"}}}}
	sel := &multiAgentSelection{Roles: []multiAgentRoleResult{{Role: "event_recent", SelectionRound: 1, Calls: []multiAgentCall{{Round: 1, Input: map[string]any{"turn_summaries": []map[string]any{{"knowledge_boundaries": b}}}}}, Selection: multiAgentRecommendation{Unresolved: []string{"Is this progress still pending?"}}}}}
	notes := buildPrepareTurnPreprocessingNotes(sel, nil, nil)
	if !strings.Contains(mustCompactJSON(notes["source_catalog"]), "unknown_to") {
		t.Error("summary-only accepted analysis uncertainty has no summary scope in final P catalog")
	}
}
func TestBoundaryCarryIntegrationR1DownstreamRecordSurfaces(t *testing.T) {
	for _, table := range []string{"pending_threads", "storylines", "episode_summaries", "direct_evidence_records", "world_rules", "canonical_state_layers"} {
		t.Run(table, func(t *testing.T) {
			authority := carryIntegrationR1Authority("protected_secrets", map[string]any{"source_ref": table + ":23"})
			input := prepareTurnAssemblyInput{Memories: []store.Memory{authority}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "integration-r1"}}, Storylines: []store.Storyline{{ID: 23, ChatSessionID: "integration-r1"}}, EpisodeSummaries: []store.EpisodeSummary{{ID: 23, ChatSessionID: "integration-r1"}}, Evidence: []store.DirectEvidence{{ID: 23, ChatSessionID: "integration-r1"}}, WorldRules: []store.WorldRule{{ID: 23, ChatSessionID: "integration-r1"}}, CanonicalLayers: []store.CanonicalStateLayer{{ID: 23, ChatSessionID: "integration-r1"}}}
			out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed(table, 23, nil)}}
			prepareTurnCarryKnowledgeBoundaries(&out, input)
			if len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) == 0 {
				t.Error("explicitly referenced downstream record lacks session lookup and carried boundary")
			}
		})
	}
}

func TestBoundaryCarryIntegrationR1HierarchyAndPrivateRecordRefs(t *testing.T) {
	for _, table := range []string{"chapter_summaries", "arc_summaries", "saga_digests", "protagonist_entity_memories"} {
		t.Run(table, func(t *testing.T) {
			authority := carryIntegrationR1Authority("protected_secrets", map[string]any{"source_ref": table + ":23"})
			input := prepareTurnAssemblyInput{Memories: []store.Memory{authority}, ResumePack: &store.ResumePack{Chapter: &store.ChapterSummary{ID: 23, ChatSessionID: "integration-r1"}, Arc: &store.ArcSummary{ID: 23, ChatSessionID: "integration-r1"}, Saga: &store.SagaDigest{ID: 23, ChatSessionID: "integration-r1"}}, CharacterPrivateMemories: []store.ProtagonistEntityMemory{{ID: 23, SourceChatSessionID: "integration-r1"}}}
			out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed(table, 23, nil)}}
			prepareTurnCarryKnowledgeBoundaries(&out, input)
			if len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) != 1 {
				t.Fatal("explicit downstream record link lost its source session")
			}
		})
	}
}
func TestBoundaryCarryIntegrationR1SummaryNotesMetadata(t *testing.T) {
	seed := carryIntegrationR1Seed("memories", 12, []string{"F41"})
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{seed}}
	input := prepareTurnAssemblyInput{Memories: []store.Memory{carryIntegrationR1Authority("protected_secrets", nil), {ID: 12, ChatSessionID: "integration-r1", SummaryJSON: `{"turn_summary":"Mira public progress"}`}}}
	prepareTurnCarryKnowledgeBoundaries(&out, input)
	facts, summaries, _, _ := prepareTurnResolvePrioritySourcePool(&out, "Mira progress", []string{"Mira progress"}, 10, nil)
	if len(facts) != 1 || len(summaries) != 1 {
		t.Fatalf("fixture pool facts=%d summaries=%d", len(facts), len(summaries))
	}
	if !strings.Contains(summaries[0].Minimum.Text, "unknown_to") {
		t.Fatal("summary reading positive control lost boundary")
	}
	summaries[0].SelectionStatus = "selected"
	id := summaries[0].SummaryID
	sel := &multiAgentSelection{Roles: []multiAgentRoleResult{{Role: "event_recent", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedSummaryIDs: []string{id}, Reasons: map[string]string{id: "Continue the public progress"}}}}}
	notes := buildPrepareTurnPreprocessingNotes(sel, map[string]any{"turn_summary_items": []map[string]any{prepareTurnPrioritySummaryMap(summaries[0])}}, nil)
	if !strings.Contains(mustCompactJSON(notes["source_catalog"]), "unknown_to") {
		t.Errorf("summary has guarded reading but final notes/P scope lost metadata: %s", mustCompactJSON(notes["source_catalog"]))
	}
}
func TestBoundaryCarryIntegrationR1UnlinkedOwnScope(t *testing.T) {
	item := map[string]any{"summary": "Mira recalls her public walk", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}}}
	facts := prepareTurnPriorityStructuredMemoryItemFacts("events", item)
	if len(facts) == 0 {
		t.Fatal("typed fact fixture empty")
	}
	before := mustCompactJSON(facts[0].Reading)
	seed := carryIntegrationR1Seed("memories", 12, nil)
	seed.Fact = facts[0]
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{seed}}
	input := prepareTurnAssemblyInput{Memories: []store.Memory{{ID: 12, ChatSessionID: "integration-r1", SummaryJSON: `{"events":[{"summary":"Mira recalls her public walk","knowledge_scope":{"known_by":["Mira"]}}]}`}}}
	prepareTurnCarryKnowledgeBoundaries(&out, input)
	after := mustCompactJSON(out.PriorityFactSeeds[0].Fact.Reading)
	if before != after {
		t.Errorf("no protected authority or explicit link: own scope gained reading/guard bytes; before=%s after=%s", before, after)
	}
}

func TestBoundaryCarryIntegrationR1UnlinkedPublicNamedScopeBytes(t *testing.T) {
	for _, scope := range []map[string]any{{"known_by": []string{"Mira"}}, {"known_by": []string{"Mira"}, "unknown_to": []string{"Oren"}, "suspected_by": []string{"Tess"}, "misinformed_by": []string{"Lio"}, "revealed_to": []string{"Paz"}}} {
		item := map[string]any{"summary": "Mira recalls a public walk", "visibility": "public", "knowledge_scope": scope}
		facts := prepareTurnPriorityStructuredMemoryItemFacts("events", item)
		if len(facts) == 0 {
			t.Fatal("public typed fixture missing")
		}
		before := mustCompactJSON(facts[0].Reading)
		seed := carryIntegrationR1Seed("memories", 12, nil)
		seed.Fact = facts[0]
		out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{seed}}
		prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{{ID: 12, ChatSessionID: "integration-r1", SummaryJSON: mustCompactJSON(map[string]any{"events": []any{item}})}}})
		if before != mustCompactJSON(out.PriorityFactSeeds[0].Fact.Reading) || len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) != 0 {
			t.Fatal("unlinked public named scope gained a second guard or notes metadata")
		}
	}
}
func TestBoundaryCarryIntegrationR1DistinctBoundaryPositive(t *testing.T) {
	first := carryIntegrationR1Authority("protected_secrets", nil)
	second := carryIntegrationR1Authority("protected_secrets", map[string]any{"summary": "Tess keeps the blue route", "owner": "Tess", "knowledge_scope": map[string]any{"known_by": []string{"Tess"}, "unknown_to": []string{"Mira"}}})
	second.ID = 14
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed("pending_threads", 23, nil)}}
	input := prepareTurnAssemblyInput{Memories: []store.Memory{first, second}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "integration-r1", HookMetadataJSON: `{"fact_ref":"F41"}`}}}
	prepareTurnCarryKnowledgeBoundaries(&out, input)
	bounds := out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries
	if len(bounds) != 2 {
		t.Fatalf("distinct attribution count=%d", len(bounds))
	}
	for _, b := range bounds {
		if len(stringsFromAny(mapFromAny(b["knowledge_scope"])["known_by"])) != 1 || stringFromMap(b, "protected_fact_ref") == "" {
			t.Fatal("scope union or missing fact attribution")
		}
	}
	if len(out.PriorityFactSeeds[0].AllowedViewers) != 0 {
		t.Fatal("linked scopes changed representation viewers")
	}
}
