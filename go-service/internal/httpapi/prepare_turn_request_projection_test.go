package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"weak"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test44RequestProjectionParity(t *testing.T) {
	for _, large := range []bool{false, true} {
		input := requestProjectionFixture(large)
		base := buildPrepareTurnInjectionAssemblyWithBudget(input)
		before, _ := json.Marshal(base)
		for q := 0; q < 5; q++ {
			local := requestProjectionQuery(input, q)
			projected := base.supplementProjection(local.VectorTrace, local.Perspective.Selection)
			if projected.preparation != base.preparation {
				t.Fatal("question rebuilt request sources")
			}
			fresh := buildPrepareTurnAssembly(local, false)
			got, gotS := multiAgentCandidatePool(&projected)
			want, wantS := multiAgentCandidatePool(&fresh)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotS, wantS) {
				t.Fatalf("large=%v question=%d source/score/order mismatch", large, q)
			}
		}
		local := requestProjectionQuery(input, 4)
		counts := func() []int {
			p := base.preparation
			return []int{len(p.memorySeeds), len(p.summaries), len(p.facts), len(p.seedTemplates), len(p.payloads)}
		}
		preparedCounts := counts()
		base.supplementProjection(local.VectorTrace, local.Perspective.Selection)
		if !reflect.DeepEqual(counts(), preparedCounts) {
			t.Fatal("repeated discovery repeated source preparation")
		}
		after, _ := json.Marshal(base)
		if string(before) != string(after) {
			t.Fatal("supplement changed initial source/delivery snapshot")
		}
	}
}

func Test44FinalDeliveryUsesPreparedPool(t *testing.T) {
	input := requestProjectionFixture(false)
	for _, ai := range []bool{false, true} {
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		if ai {
			facts, summaries := multiAgentCandidatePool(&out)
			m := &multiAgentSelection{Candidates: facts, Summaries: summaries}
			m.captureBaseline(out.MemoryDeliveryPlan)
			for _, lane := range multiAgentRoles {
				r := multiAgentRoleResult{Role: lane, Source: "ai"}
				for i := len(facts) - 1; i >= 0; i-- {
					if facts[i].Lane == lane {
						r.Selection.SelectedIDs = append(r.Selection.SelectedIDs, facts[i].CanonicalFactID)
					}
				}
				m.Roles = append(m.Roles, r)
			}
			out.Preprocessing = m
		}
		got := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, input.MaxChars, 5, input.BudgetMode, input.Budgets, input.Perspective.Selection)
		fresh := out
		want := buildPrepareTurnPriorityMemoryDeliveryPlan(&fresh, input.MaxChars, 5, input.BudgetMode, input.Budgets, input.Perspective.Selection)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("final delivery changed source metadata, AI order or Go fallback")
		}
		if !ai {
			out.ActualMemoryText += "\n- Mira records a newly discovered silver archive door."
			retained := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, input.MaxChars, 5, input.BudgetMode, input.Budgets, input.Perspective.Selection)
			rebuilt := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, input.MaxChars, 5, input.BudgetMode, input.Budgets, input.Perspective.Selection)
			if !reflect.DeepEqual(retained, got) || reflect.DeepEqual(rebuilt, got) {
				t.Fatal("negative assertion did not distinguish reuse from rebuilding")
			}
		}
	}
}

func Test44ProjectionRediscoversOldWorldSource(t *testing.T) {
	input := requestProjectionFixture(false)
	input.CanonicalLayers = []store.CanonicalStateLayer{{ID: 92, ChatSessionID: "candidate-pool", LayerType: "world_state", SourceTurn: 20, TurnIndex: 20, Content: `{"background":"Mira archive","rules":[{"scope":"root","key":"archive","value":{"door":"sealed"}}]}`}}
	input.Common = prepareTurnCommonAssemblySources(input)
	base := buildPrepareTurnInjectionAssemblyWithBudget(input)
	if strings.Contains(base.WorldRulesText, "blue") {
		t.Fatal("negative control already selected distant world source")
	}
	for _, found := range []bool{true, false, true} {
		query := requestProjectionQuery(input, 3)
		if found {
			query.VectorTrace = map[string]any{"search_result": "ok", "search_results": []map[string]any{{"source_table": "world_rules", "source_row_id": "62", "tier": "world_rule", "similarity": 0.9, "similarity_source": "cosine"}}}
		}
		projected := base.supplementProjection(query.VectorTrace, query.Perspective.Selection)
		fresh := buildPrepareTurnAssembly(query, false)
		got, gs := multiAgentCandidatePool(&projected)
		want, ws := multiAgentCandidatePool(&fresh)
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gs, ws) || projected.CanonWorldText != fresh.CanonWorldText {
			t.Fatal("source rediscovery changed canonical content, fact provenance or order")
		}
		if strings.Contains(projected.WorldRulesText, "blue") != found {
			t.Fatal("new query evidence was lost or leaked into the next query")
		}
	}
}

func Test44DeliveryCountsEqualActualUnicodeSections(t *testing.T) {
	for _, cap := range []int{1, 80, 200, 450, 1500, 18000} {
		out := autoBudgetEvidenceFixture()
		out.PersonaText = "  개인적인 기억 안내 🌏\n\t\n- 미라는 기록 보관소에서 친구와 만났던 약속을 기억한다.\u2003"
		out.DirectEvidenceText = strings.Repeat("  - 화자 미라: 안녕 👋\u2003\n", 20)
		plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira 미라 archive", CurrentTurn: 170})
		sum, sections := 0, 0
		for _, raw := range prepareTurnMemoryLineageSlice(plan["classes"]) {
			c := mapFromAny(raw)
			text := extractionStringFromAny(c["text"])
			if intFromAny(c["used_chars"], 0) != len([]rune(text)) {
				t.Fatalf("cap=%d lane=%v count differs from rendering", cap, c["key"])
			}
			if text != "" {
				sum += len([]rune(text))
				sections++
			}
		}
		if sections > 0 {
			sum += 2 * (sections - 1)
		}
		if sum != len([]rune(extractionStringFromAny(plan["final_text"]))) || sum > cap {
			t.Fatal("whole delivery counter/budget mismatch")
		}
	}
}

func Benchmark44RequestProjection(b *testing.B) {
	for _, count := range []int{0, 1, 2, 4, 5} {
		b.Run(fmt.Sprintf("questions-%d", count), func(b *testing.B) {
			input := requestProjectionFixture(true)
			input.Common = nil
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				local := input
				local.Common = prepareTurnCommonAssemblySources(input)
				base := buildPrepareTurnInjectionAssemblyWithBudget(local)
				for q := 0; q < count; q++ {
					query := requestProjectionQuery(local, q)
					out := base.supplementProjection(query.VectorTrace, query.Perspective.Selection)
					facts, _ := multiAgentCandidatePool(&out)
					if len(facts) < len(input.Perspective.CharacterSeeds) {
						b.Fatal("memory breadth lost")
					}
				}
			}
		})
	}
}

func Test44RequestPreparationReleased(t *testing.T) {
	// Keep final delivered text alive, but release the internal request assembly.
	// A session/global cache retaining this workspace would fail this assertion.
	run := func() (weak.Pointer[prepareTurnRequestPreparation], string) {
		input := requestProjectionFixture(true)
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		for q := 0; q < 5; q++ {
			query := requestProjectionQuery(input, q)
			out.supplementProjection(query.VectorTrace, query.Perspective.Selection)
		}
		return weak.Make(out.preparation), extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	}
	var readings []uint64
	for n := 0; n < 10; n++ {
		pointer, delivered := run()
		for i := 0; i < 3; i++ {
			runtime.GC()
		}
		if pointer.Value() != nil {
			t.Fatal("request preparation retained after its owner ended")
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		readings = append(readings, m.HeapAlloc)
		runtime.KeepAlive(delivered)
	}
	t.Logf("post-GC live heap across ten requests: %v", readings)
}

// An opt-in artifact probe exercises the production source owner. The same test
// is run before and after the refactor; no expected scores are manufactured here.
func Test44RequestProjectionArtifacts(t *testing.T) {
	dir := os.Getenv("AC_PROJECTION_ARTIFACTS")
	if dir == "" {
		t.Skip("offline before/after artifact export")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, large := range []bool{false, true} {
		input := requestProjectionFixture(large)
		for _, count := range []int{0, 1, 2, 4, 5} {
			base := buildPrepareTurnInjectionAssemblyWithBudget(input)
			facts, summaries := multiAgentCandidatePool(&base)
			rows := []any{map[string]any{"assembly": base, "facts": facts, "summaries": summaries}}
			for q := 0; q < count; q++ {
				local := requestProjectionQuery(input, q)
				out := base.supplementProjection(local.VectorTrace, local.Perspective.Selection)
				f, s := multiAgentCandidatePool(&out)
				rows = append(rows, map[string]any{"facts": f, "summaries": s})
			}
			data, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("large-%v-questions-%d.json", large, count)), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func requestProjectionFixture(large bool) prepareTurnAssemblyInput {
	p := priorityMemoryTestContext(5)
	p["_priority_memory_query"] = "Mira checks the archive door."
	p[prepareTurnPriorityQuerySetContextKey] = []string{"Mira checks the archive door.", "Mira kept the brass key."}
	p["_priority_memory_current_turn"] = 31
	i := priorityCandidatePoolTestInput(p)
	if large {
		i = preprocessingEfficiencyInput()
	}
	i.PendingThreads = []store.PendingThread{
		{ID: 81, Description: "Rowan must deliver the hidden mineral parcel.", Status: "open", SourceTurn: 14, Priority: 7},
		{ID: 82, Description: "Mira promised to open the archive.", Status: "open", SourceTurn: 12, Pinned: true},
	}
	i.WorldRules = []store.WorldRule{
		{ID: 61, ChatSessionID: "candidate-pool", Scope: "root", Key: "archive", ValueJSON: `{"door":"sealed"}`, SourceTurn: 20},
		{ID: 62, ChatSessionID: "candidate-pool", Scope: "location", ScopeName: "distant mine", Key: "mineral", ValueJSON: `{"colour":"blue"}`, SourceTurn: 4},
	}
	i.Common = prepareTurnCommonAssemblySources(i)
	return i
}

func requestProjectionQuery(i prepareTurnAssemblyInput, q int) prepareTurnAssemblyInput {
	questions := []string{"Mira archive", "Rowan parcel", "기록단서가", "hidden mineral", "old promise"}
	p := *i.Perspective
	p.Selection.QuerySet = append(append([]string{}, p.Selection.QuerySet...), questions[q])
	p.Selection.SemanticFacts = append(append([]prepareTurnPrioritySemanticFact{}, p.Selection.SemanticFacts...), prepareTurnPrioritySemanticFact{
		UnitID: fmt.Sprintf("supplement-%d", q%2), SourceTurn: 3 + q%2, Lane: "world_state", Visibility: "public",
		Similarity: .8, SimilaritySource: "cosine", SupplementalQueryMatched: true,
		Fact: prepareTurnPriorityMemoryFact{Text: fmt.Sprintf("The hidden mineral has marking %d.", q%2)},
	})
	i.Perspective = &p
	return i
}
