package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrivateEvidenceSpanPublicOccurrences(t *testing.T) {
	const public = "The harbor beacon shines blue."
	for _, revealed := range []bool{false, true} {
		name := "separate_public_occurrence"
		private := public + " I privately hid the key under its base."
		item := map[string]any{"owner": "Mira", "summary": "Hidden key", "evidence_excerpt": private}
		if revealed {
			name = "explicit_public_disclosure"
			private = "Mira announces to everyone: " + public
			item["evidence_excerpt"] = private
			item["disclosure_policy"] = "public"
			item["knowledge_scope"] = map[string]any{"publicly_revealed": true}
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			ex := map[string]any{"turn_summary": public, "evidence_excerpts": []any{public}, "protected_secrets": []any{item}}
			before := mustCompactJSON(ex)
			st := &revalidationStore{turnRecordingStore: &turnRecordingStore{}}
			cfg := config.Default()
			srv := NewServer(cfg)
			srv.Store, srv.StoreOpenError = st, nil
			result := srv.saveCriticExtractionArtifacts(context.Background(), "public-occurrence", 1, ex, public+" "+private, completeTurnEmbeddingConfig{}, time.Unix(300, 0))
			if result.Errors != 0 || len(st.returnEvidence) == 0 || len(st.savedMemories) == 0 {
				t.Fatalf("save did not exercise memory and evidence: %+v", result)
			}
			for _, e := range st.returnEvidence {
				if e.EvidenceText == public && e.EvidenceKind != "turn_excerpt" {
					t.Error("independent public occurrence became private")
				}
			}
			m := *st.savedMemories[0]
			projection := buildPublicMemoryProjection(parseJSONMap(m.SummaryJSON), m.Evidence)
			if !projection.Eligible || !strings.Contains(projection.SearchText.Text, public) {
				t.Fatalf("evidence-only public memory disappeared on read: %+v", projection)
			}
			if mustCompactJSON(ex) != before {
				t.Error("canonical extraction mutated")
			}
		})
	}
}

func TestPrivateEvidenceSpanNormalizedEventBody(t *testing.T) {
	const private = "Behind the screen, I admitted that my former name was Neris Vale."
	const partial = "my former name was Neris Vale."
	const public = "The harbor beacon shines blue."
	ex := normalizeCriticExtraction(map[string]any{
		"subjective_entity_memories": []any{map[string]any{"owner_entity_name": "Ilan", "memory_text": "A private former identity", "owner_visibility": "owner_private", "evidence_excerpt": private}},
		"narrative_events":           []any{map[string]any{"event": partial, "evidence_excerpt": partial}, map[string]any{"event": public, "evidence_excerpt": public}},
	})
	before := mustCompactJSON(ex)
	projection := buildPublicMemoryProjection(ex, "")
	if strings.Contains(projection.SearchText.Text, partial) || strings.Contains(mustCompactJSON(projection.Extraction), partial) {
		t.Error("protected event body regenerated a public canonical summary")
	}
	if !strings.Contains(projection.SearchText.Text, public) || mustCompactJSON(ex) != before {
		t.Error("public event or canonical input changed")
	}
}

func TestPrivateEvidenceSpanSourceBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, private, crossing, public string }{
		{"suffix", "Private amber. Public blue.", "Private amber.", "amber. Public", "Public blue."},
		{"prefix", "Public blue. Private amber.", "Private amber.", "blue. Private", "Public blue."},
		{"unicode_no_minimum", "甲乙丙丁", "乙丙", "丙丁", "甲"},
		{"normalized_offsets", "PUBLIC\tblue.\nPrivate\u00a0“amber”.", "private \"amber\".", "BLUE. private", "public blue."},
		{"repeated_public", "Public blue. Public blue. Private amber.", "Public blue. Private amber.", "blue. Private", "Public blue."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ex := map[string]any{"protected_secrets": []any{map[string]any{"evidence_excerpt": tc.private}}, "evidence_excerpts": []any{tc.crossing, tc.public}}
			scope, _ := memoryAdmissionPerspectiveEvidenceScope(ex, tc.source)
			stored := memoryAdmissionEvidenceJSON(ex, tc.source)
			for _, view := range []memoryPerspectiveEvidenceScope{scope, memoryPerspectiveEvidenceScopeFromStored(ex, stored)} {
				if !memoryAdmissionPerspectiveEvidenceContains(view, tc.crossing) {
					t.Error("source boundary overlap lost its scope")
				}
				if memoryAdmissionPerspectiveEvidenceContains(view, tc.public) {
					t.Error("independent public occurrence hidden")
				}
			}
			memory := store.Memory{TurnIndex: 1, SummaryJSON: mustCompactJSON(ex), Evidence: stored}
			rows := []store.DirectEvidence{{ID: 1, SourceTurnStart: 1, SourceTurnEnd: 1, EvidenceKind: "turn_excerpt", EvidenceText: tc.crossing}, {ID: 2, SourceTurnStart: 1, SourceTurnEnd: 1, EvidenceKind: "turn_excerpt", EvidenceText: tc.public}}
			safe, blocked := filterPrepareTurnPerspectiveScopedEvidence(rows, []store.Memory{memory})
			if len(safe) != 1 || safe[0].ID != 2 || !blocked[1] {
				t.Errorf("stored occurrence scope not used by recall: %v %v", safe, blocked)
			}
		})
	}
}

func TestPrivateEvidenceSpanEventCitationDoesNotChangeSourceScope(t *testing.T) {
	const private = "Behind the screen, I admitted that my former name was Neris Vale."
	const public = "The harbor beacon shines blue."
	const tail = "my former name was Neris Vale."
	for _, body := range []string{tail, tail + " " + public} {
		for _, citation := range []string{"", public, tail} {
			ex := map[string]any{
				"protected_secrets": []any{map[string]any{"evidence_excerpt": private}},
				"narrative_events":  []any{map[string]any{"event": body, "evidence_excerpt": citation}, map[string]any{"event": public, "evidence_excerpt": public}},
			}
			stored := memoryAdmissionEvidenceJSON(ex, private+" "+public)
			projection := buildPublicMemoryProjection(ex, stored)
			if strings.Contains(projection.SearchText.Text, "Neris Vale") || !strings.Contains(projection.SearchText.Text, public) {
				t.Errorf("event source scope changed with citation %q: %s", citation, projection.SearchText.Text)
			}
		}
	}
}

func TestPrivateEvidenceSpanUnlocatedQuoteRetainsScope(t *testing.T) {
	const private = "Behind the screen, I admitted that my former name was Neris Vale."
	const excerpt = "my former name was Neris Vale."
	ex := map[string]any{"protected_secrets": []any{map[string]any{"evidence_excerpt": private}}, "evidence_excerpts": []any{excerpt}}
	scope, _ := memoryAdmissionPerspectiveEvidenceScope(ex, excerpt)
	if !memoryAdmissionPerspectiveEvidenceContains(scope, excerpt) {
		t.Fatal("missing full source coordinates weakened an existing private quote")
	}
}

func TestPrivateEvidenceSpanSanitizedCrossingQuote(t *testing.T) {
	const private = "Behind the screen, I admitted that my former name was Neris Vale."
	const tail = "my former name was Neris Vale."
	public := strings.Repeat("The harbor beacon shines blue. ", 100)
	content, crossing := private+" "+public, tail+" "+public
	storedQuote := sanitizeEvidenceExcerptForTurn(crossing, content)
	if storedQuote == "" || strings.TrimSpace(crossing) == storedQuote {
		t.Fatal("fixture must exercise the existing evidence shortening owner")
	}
	ex := map[string]any{"protected_secrets": []any{map[string]any{"evidence_excerpt": private}}, "evidence_excerpts": []any{crossing}}
	result := artifactSaveResult{}
	rows := buildMemoryAdmissionEvidence("shortened-span", 1, ex, content, nil, nil, time.Unix(0, 0), &result)
	if len(rows) != 1 || rows[0].EvidenceText != storedQuote || rows[0].EvidenceKind != "perspective_scoped_turn_excerpt" {
		t.Fatalf("shortening lost source boundary scope: %+v", rows)
	}
	scope := memoryPerspectiveEvidenceScopeFromStored(ex, memoryAdmissionEvidenceJSON(ex, content))
	if !memoryAdmissionPerspectiveEvidenceContains(scope, storedQuote) {
		t.Error("shortened source scope lost on read")
	}
}

func TestPrivateEvidenceSpanRetainsTypedPublicValuesWithinPrivateCitation(t *testing.T) {
	const publicValue = "her entire body was immobilized"
	const citation = "Mira observed that her entire body was immobilized."
	const private = citation + " She privately worried about her hidden former identity."
	ex := map[string]any{
		"subjective_entity_memories":     []any{map[string]any{"owner_entity_name": "Mira", "owner_visibility": "owner_private", "memory_text": "A private worry", "evidence_excerpt": private}},
		"state_claims":                   []any{map[string]any{"subject": "Mira", "state_slot": "body_state", "value": publicValue, "evidence_excerpt": citation}},
		"character_profile_observations": []any{map[string]any{"subject_entity": "Mira", "supported_expression": publicValue, "visibility": "public", "evidence_excerpt": citation}},
	}
	before := mustCompactJSON(ex)
	projection := buildPublicMemoryProjection(ex, "")
	state := mapFromAny(sliceFromAny(projection.Extraction["state_claims"])[0])
	profile := mapFromAny(sliceFromAny(projection.Extraction["character_profile_observations"])[0])
	if stringFromMap(state, "value") != publicValue || stringFromMap(profile, "supported_expression") != publicValue {
		t.Fatal("typed public meaning was removed with its private citation")
	}
	if strings.Contains(mustCompactJSON(projection.Extraction), citation) || mustCompactJSON(ex) != before {
		t.Fatal("private citation or canonical-input boundary changed")
	}
}

// Synthetic forms of the audit's profile/state quotations. The private quote
// and its shorter reuse are deliberately different lengths, while an unrelated
// public event from the same source must survive admission and final delivery.
func TestPrivateEvidenceSpanAdmissionAndDelivery(t *testing.T) {
	const sid = "private-span-regression"
	const privateQuote = "Behind the screen, I admitted that my former name was Neris Vale."
	const partialQuote = "my former name was Neris Vale."
	const publicQuote = "The harbor beacon shines blue."
	for _, accepted := range []bool{false, true} {
		for _, lane := range []string{"character_profile_observations", "state_claims", "evidence_excerpts", "observation", "change"} {
			name := lane + "/legacy"
			if accepted {
				name = lane + "/admission"
			}
			t.Run(name, func(t *testing.T) {
				t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
				ex := map[string]any{
					"turn_summary":      publicQuote,
					"evidence_excerpts": []any{publicQuote},
					"narrative_events":  []any{map[string]any{"event": publicQuote, "evidence_excerpt": publicQuote}},
					"subjective_entity_memories": []any{map[string]any{
						"owner_entity_name": "Ilan", "owner_entity_key": "ilan", "owner_entity_role": "npc",
						"memory_text": "Ilan recalled a former identity.", "owner_visibility": "owner_private",
						"evidence_excerpt": privateQuote,
					}},
				}
				switch lane {
				case "observation", "change":
					ex["state_claims"] = []any{map[string]any{"subject": "Ilan", "state_slot": "identity", lane: partialQuote, "evidence_excerpt": partialQuote}}
					ex["evidence_excerpts"] = []any{publicQuote, partialQuote}
				case "character_profile_observations":
					ex[lane] = []any{map[string]any{"subject_entity": "Ilan", "profile_section": "values", "trait_domain": "loyalty", "supported_expression": "Values companionship", "visibility": "public", "evidence_excerpt": partialQuote}}
				case "state_claims":
					ex[lane] = []any{map[string]any{"subject": "Harbor watch", "subject_type": "session", "state_slot": "goal_status", "value": "Promise reaffirmed", "transition": "reaffirm", "evidence_excerpt": partialQuote}}
				default:
					ex[lane] = []any{publicQuote, partialQuote}
				}
				st := &revalidationAdmissionStore{revalidationSourceStore: &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{}}}}
				cfg := config.Default()
				cfg.StoreMode = config.StoreModeDualShadow
				srv := NewServer(cfg)
				srv.Store, srv.StoreOpenError = st, nil
				ctx := context.Background()
				if accepted {
					ctx = context.WithValue(ctx, entityIdentitySourceContextKey{}, entityIdentitySourceContext{ContractVersion: completeTurnSourceAcceptanceContract, Revision: "offline-rev-1"})
				} else {
					srv.Store = st.revalidationStore
					// Legacy extraction already contains its profile evidence; the
					// accepted-source appender owns this step for current admission.
					if lane == "character_profile_observations" {
						ex["evidence_excerpts"] = []any{publicQuote, partialQuote}
					}
				}
				ex = normalizeCriticExtraction(ex)
				result := srv.saveCriticExtractionArtifacts(ctx, sid, 1, ex, publicQuote+" "+privateQuote, completeTurnEmbeddingConfig{}, time.Unix(300, 0))
				if result.Errors != 0 {
					t.Fatalf("save: %+v", result)
				}
				if !accepted {
					for _, m := range st.savedMemories {
						copy := *m
						copy.ID = 1
						st.returnMemories = append(st.returnMemories, copy)
					}
				}
				found := false
				for _, e := range st.returnEvidence {
					if e.EvidenceText == partialQuote {
						found = true
						if e.EvidenceKind != "perspective_scoped_turn_excerpt" {
							t.Errorf("private partial quote stored as %s", e.EvidenceKind)
						}
					}
					if e.EvidenceText == publicQuote && e.EvidenceKind != "turn_excerpt" {
						t.Error("public evidence was hidden")
					}
				}
				if !found {
					t.Error("private evidence lost instead of retained with its scope")
				}
				response := revalidationHTTP(t, srv, map[string]any{
					"chat_session_id": sid, "turn_index": 2, "raw_user_input": "Ilan harbor beacon Neris Vale",
					"client_meta": map[string]any{"perspective_context": map[string]any{"current_pov": "Visitor"}},
					"settings":    map[string]any{"injection_enabled": true, "max_injection_chars": 30000, "input_context_enabled": false, "top_k": 5},
				})
				plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
				main := stringFromMap(plan, "main_memory_text")
				payload := stringFromMap(mapFromAny(response["payload_application_plan"]), "auxiliary_text")
				for label, text := range map[string]string{"main": main, "payload": payload} {
					if strings.Contains(text, partialQuote) {
						t.Errorf("private partial quote reached %s", label)
					}
					if !strings.Contains(text, publicQuote) {
						t.Errorf("public positive control missing from %s", label)
					}
				}
			})
		}
	}
}

func TestPrivateEvidenceSpanLegacyReadProjection(t *testing.T) {
	const protected = "The sealed message says the northern safe opens with amber."
	for _, excerpt := range []string{protected, "the northern safe opens with amber.", "The messenger said: " + protected} {
		extraction := map[string]any{"protected_secrets": []any{map[string]any{"owner": "Mira", "summary": "Safe combination", "evidence_excerpt": protected}}, "evidence_excerpts": []any{excerpt, "Public bells rang."}}
		projection := buildPublicMemoryProjection(extraction, mustCompactJSON(map[string]any{"evidence_excerpts": []string{excerpt}}))
		if strings.Contains(projection.SearchText.Text, "northern safe") {
			t.Errorf("private span in public search: %s", projection.SearchText.Text)
		}
		if !strings.Contains(projection.SearchText.Text, "Public bells rang") {
			t.Error("public evidence lost")
		}
		memory := store.Memory{ChatSessionID: "span", TurnIndex: 3, SummaryJSON: mustCompactJSON(extraction)}
		evidence := []store.DirectEvidence{{ID: 1, SourceTurnStart: 3, SourceTurnEnd: 3, EvidenceKind: "turn_excerpt", EvidenceText: excerpt}, {ID: 2, SourceTurnStart: 4, SourceTurnEnd: 4, EvidenceKind: "turn_excerpt", EvidenceText: excerpt}, {ID: 3, SourceTurnStart: 3, SourceTurnEnd: 3, EvidenceKind: "turn_excerpt", EvidenceText: "Public bells rang."}}
		safe, blocked := filterPrepareTurnPerspectiveScopedEvidence(evidence, []store.Memory{memory})
		if len(safe) != 2 || !blocked[1] || blocked[2] || blocked[3] {
			t.Errorf("source-bound legacy read scope: safe=%+v blocked=%v", safe, blocked)
		}
	}
}

func TestPrivateEvidenceSpanRetainsPublicMeaningWithoutPrivateCitation(t *testing.T) {
	const private = "The bridge reopened; Mira privately recited the amber vault password."
	const citation = "Mira privately recited the amber vault password."
	const public = "The bridge reopened"
	ex := map[string]any{
		"protected_secrets": []any{map[string]any{"owner": "Mira", "summary": "Vault password", "evidence_excerpt": private}},
		"state_claims":      []any{map[string]any{"subject": "bridge", "subject_type": "world", "state_slot": "status", "value": public, "evidence_excerpt": citation}},
		"evidence_excerpts": []any{citation},
	}
	before := mustCompactJSON(ex)
	projection := buildPublicMemoryProjection(ex, "")
	if strings.Contains(mustCompactJSON(projection.Extraction), citation) {
		t.Error("private citation remains in public projection")
	}
	if !strings.Contains(projection.SearchText.Text, public) {
		t.Error("public semantic fact disappeared with private citation")
	}
	if mustCompactJSON(ex) != before {
		t.Error("public read modified canonical extraction")
	}
	result := artifactSaveResult{}
	evidence := buildMemoryAdmissionEvidence("private-span", 1, ex, private, nil, nil, time.Unix(0, 0), &result)
	rows := make([]store.DirectEvidence, 0, len(evidence))
	for _, e := range evidence {
		rows = append(rows, *e)
	}
	srv := &Server{}
	units := srv.buildPreciseMemoryUnitsFromExtraction(acceptedPreciseMemoryContext("private-span-rev"), "private-span", 1, ex, private, rows, nil, time.Unix(0, 0), &result)
	if len(units) == 0 {
		t.Fatalf("scoped source unit lost: %+v", result)
	}
	for _, u := range units {
		if strings.Contains(u.EvidenceExcerpt, citation) && store.PreciseMemoryGeneralVectorEligible(u) {
			t.Error("private quotation remains eligible for general precise vector recall")
		}
	}
}

func TestPrivateEvidenceSpanCrossingAdmissionAndDelivery(t *testing.T) {
	const sid = "private-span-regression"
	const privateQuote = "Behind the screen, I admitted that my former name was Neris Vale."
	const partialQuote = "my former name was Neris Vale. The harbor beacon shines blue."
	const publicQuote = "The harbor beacon shines blue."
	for _, accepted := range []bool{false, true} {
		for _, lane := range []string{"character_profile_observations", "state_claims", "evidence_excerpts"} {
			name := lane + "/legacy"
			if accepted {
				name = lane + "/admission"
			}
			t.Run(name, func(t *testing.T) {
				t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
				ex := map[string]any{
					"turn_summary":      publicQuote,
					"evidence_excerpts": []any{publicQuote},
					"narrative_events":  []any{map[string]any{"event": publicQuote, "evidence_excerpt": publicQuote}},
					"subjective_entity_memories": []any{map[string]any{
						"owner_entity_name": "Ilan", "owner_entity_key": "ilan", "owner_entity_role": "npc",
						"memory_text": "Ilan recalled a former identity.", "owner_visibility": "owner_private",
						"evidence_excerpt": privateQuote,
					}},
				}
				switch lane {
				case "character_profile_observations":
					ex[lane] = []any{map[string]any{"subject_entity": "Ilan", "profile_section": "values", "trait_domain": "loyalty", "supported_expression": "Values companionship", "visibility": "public", "evidence_excerpt": partialQuote}}
				case "state_claims":
					ex[lane] = []any{map[string]any{"subject": "Harbor watch", "subject_type": "session", "state_slot": "goal_status", "value": "Promise reaffirmed", "transition": "reaffirm", "evidence_excerpt": partialQuote}}
				default:
					ex[lane] = []any{publicQuote, partialQuote}
				}
				st := &revalidationAdmissionStore{revalidationSourceStore: &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{}}}}
				cfg := config.Default()
				cfg.StoreMode = config.StoreModeDualShadow
				srv := NewServer(cfg)
				srv.Store, srv.StoreOpenError = st, nil
				ctx := context.Background()
				if accepted {
					ctx = context.WithValue(ctx, entityIdentitySourceContextKey{}, entityIdentitySourceContext{ContractVersion: completeTurnSourceAcceptanceContract, Revision: "offline-rev-1"})
				} else {
					srv.Store = st.revalidationStore
					// Legacy extraction already contains its profile evidence; the
					// accepted-source appender owns this step for current admission.
					if lane == "character_profile_observations" {
						ex["evidence_excerpts"] = []any{publicQuote, partialQuote}
					}
				}
				ex = normalizeCriticExtraction(ex)
				result := srv.saveCriticExtractionArtifacts(ctx, sid, 1, ex, privateQuote+" "+publicQuote, completeTurnEmbeddingConfig{}, time.Unix(300, 0))
				if result.Errors != 0 {
					t.Fatalf("save: %+v", result)
				}
				if !accepted {
					for _, m := range st.savedMemories {
						copy := *m
						copy.ID = 1
						st.returnMemories = append(st.returnMemories, copy)
					}
				}
				found := false
				for _, e := range st.returnEvidence {
					if e.EvidenceText == partialQuote {
						found = true
						if e.EvidenceKind != "perspective_scoped_turn_excerpt" {
							t.Errorf("private partial quote stored as %s", e.EvidenceKind)
						}
					}
					if e.EvidenceText == publicQuote && e.EvidenceKind != "turn_excerpt" {
						t.Error("public evidence was hidden")
					}
				}
				if !found {
					t.Error("private evidence lost instead of retained with its scope")
				}
				response := revalidationHTTP(t, srv, map[string]any{
					"chat_session_id": sid, "turn_index": 2, "raw_user_input": "Ilan harbor beacon Neris Vale",
					"client_meta": map[string]any{"perspective_context": map[string]any{"current_pov": "Visitor"}},
					"settings":    map[string]any{"injection_enabled": true, "max_injection_chars": 30000, "input_context_enabled": false, "top_k": 5},
				})
				plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
				main := stringFromMap(plan, "main_memory_text")
				payload := stringFromMap(mapFromAny(response["payload_application_plan"]), "auxiliary_text")
				for label, text := range map[string]string{"main": main, "payload": payload} {
					if strings.Contains(text, partialQuote) {
						t.Errorf("private partial quote reached %s", label)
					}
					if !strings.Contains(text, publicQuote) {
						t.Errorf("public positive control missing from %s", label)
					}
				}
			})
		}
	}
}

func TestPrivateEvidenceSpanEventBodyAdmissionAndDelivery(t *testing.T) {
	const sid = "private-span-regression"
	const privateQuote = "Behind the screen, I admitted that my former name was Neris Vale."
	const partialQuote = "my former name was Neris Vale."
	const publicQuote = "The harbor beacon shines blue."
	for _, accepted := range []bool{false, true} {
		for _, lane := range []string{"character_profile_observations", "state_claims", "evidence_excerpts"} {
			name := lane + "/legacy"
			if accepted {
				name = lane + "/admission"
			}
			t.Run(name, func(t *testing.T) {
				t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
				ex := map[string]any{
					"turn_summary":      publicQuote,
					"evidence_excerpts": []any{publicQuote},
					"narrative_events":  []any{map[string]any{"event": publicQuote, "evidence_excerpt": publicQuote}, map[string]any{"event": partialQuote, "evidence_excerpt": partialQuote}},
					"subjective_entity_memories": []any{map[string]any{
						"owner_entity_name": "Ilan", "owner_entity_key": "ilan", "owner_entity_role": "npc",
						"memory_text": "Ilan recalled a former identity.", "owner_visibility": "owner_private",
						"evidence_excerpt": privateQuote,
					}},
				}
				switch lane {
				case "character_profile_observations":
					ex[lane] = []any{map[string]any{"subject_entity": "Ilan", "profile_section": "values", "trait_domain": "loyalty", "supported_expression": "Values companionship", "visibility": "public", "evidence_excerpt": partialQuote}}
				case "state_claims":
					ex[lane] = []any{map[string]any{"subject": "Harbor watch", "subject_type": "session", "state_slot": "goal_status", "value": "Promise reaffirmed", "transition": "reaffirm", "evidence_excerpt": partialQuote}}
				default:
					ex[lane] = []any{publicQuote, partialQuote}
				}
				st := &revalidationAdmissionStore{revalidationSourceStore: &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{}}}}
				cfg := config.Default()
				cfg.StoreMode = config.StoreModeDualShadow
				srv := NewServer(cfg)
				srv.Store, srv.StoreOpenError = st, nil
				ctx := context.Background()
				if accepted {
					ctx = context.WithValue(ctx, entityIdentitySourceContextKey{}, entityIdentitySourceContext{ContractVersion: completeTurnSourceAcceptanceContract, Revision: "offline-rev-1"})
				} else {
					srv.Store = st.revalidationStore
					// Legacy extraction already contains its profile evidence; the
					// accepted-source appender owns this step for current admission.
					if lane == "character_profile_observations" {
						ex["evidence_excerpts"] = []any{publicQuote, partialQuote}
					}
				}
				ex = normalizeCriticExtraction(ex)
				result := srv.saveCriticExtractionArtifacts(ctx, sid, 1, ex, publicQuote+" "+privateQuote, completeTurnEmbeddingConfig{}, time.Unix(300, 0))
				if result.Errors != 0 {
					t.Fatalf("save: %+v", result)
				}
				if !accepted {
					for _, m := range st.savedMemories {
						copy := *m
						copy.ID = 1
						st.returnMemories = append(st.returnMemories, copy)
					}
				}
				found := false
				for _, e := range st.returnEvidence {
					if e.EvidenceText == partialQuote {
						found = true
						if e.EvidenceKind != "perspective_scoped_turn_excerpt" {
							t.Errorf("private partial quote stored as %s", e.EvidenceKind)
						}
					}
					if e.EvidenceText == publicQuote && e.EvidenceKind != "turn_excerpt" {
						t.Error("public evidence was hidden")
					}
				}
				if !found {
					t.Error("private evidence lost instead of retained with its scope")
				}
				response := revalidationHTTP(t, srv, map[string]any{
					"chat_session_id": sid, "turn_index": 2, "raw_user_input": "Ilan harbor beacon Neris Vale",
					"client_meta": map[string]any{"perspective_context": map[string]any{"current_pov": "Visitor"}},
					"settings":    map[string]any{"injection_enabled": true, "max_injection_chars": 30000, "input_context_enabled": false, "top_k": 5},
				})
				plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
				main := stringFromMap(plan, "main_memory_text")
				payload := stringFromMap(mapFromAny(response["payload_application_plan"]), "auxiliary_text")
				for label, text := range map[string]string{"main": main, "payload": payload} {
					if strings.Contains(text, partialQuote) {
						t.Errorf("private partial quote reached %s", label)
					}
					if !strings.Contains(text, publicQuote) {
						t.Errorf("public positive control missing from %s", label)
					}
				}
			})
		}
	}
}
