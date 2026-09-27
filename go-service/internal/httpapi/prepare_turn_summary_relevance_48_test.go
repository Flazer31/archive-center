package httpapi

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func summary48CrowdedInput(query, target string, budget int) prepareTurnAssemblyInput {
	const sid = "summary-current-query"
	p := testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(8))
	p.Selection.Query = query
	p.Selection.QuerySet = []string{query, "세린은 항구에서 주민들과 물품을 정리하고 보수 일정을 논의한다."}
	p.Selection.CurrentTurn = 400
	in := prepareTurnAssemblyInput{TopK: 8, MaxChars: budget, UserInput: query, Profile: "default", BudgetMode: "auto", Perspective: p}
	add := func(id int64, turn int, text string, extra map[string]any) {
		event := map[string]any{"actor": "세린", "event": text, "visibility": "public", "evidence_excerpt": text}
		for k, v := range extra {
			event[k] = v
		}
		in.Memories = append(in.Memories, store.Memory{ID: id, ChatSessionID: sid, TurnIndex: turn, Importance: .8, SummaryJSON: mustCompactJSON(map[string]any{"narrative_events": []any{event}})})
	}
	add(701, 3, target, nil)
	for i := 0; i < 160; i++ {
		note := fmt.Sprintf("세린은 항구의 제%d 물품 창고에서 주민들과 오늘의 보수 일정을 확인했다. 담당자는 나무 상자와 도구의 수량, 배의 접안 순서, 작업 구역을 기록했다. 작업 뒤에는 여관에서 저녁을 먹고 다음날 운반할 물품을 준비했다. 이 기록은 해당 창고의 오늘 작업에 관한 것이다.", i+1)
		add(int64(1000+i), 100+i, note, map[string]any{"location": fmt.Sprintf("항구 창고 %d", i+1), "result": "오늘 보수 물품 점검을 마쳤다."})
	}
	hits := []map[string]any{}
	for _, m := range in.Memories {
		// Equal similarities do not preselect the desired historical record.
		hits = append(hits, map[string]any{"id": fmt.Sprintf("memory:%s:%d", sid, m.ID), "tier": "memory", "source_table": "memories", "source_row_id": fmt.Sprint(m.ID), "chat_session_id": sid, "similarity": .8})
	}
	in.VectorTrace = map[string]any{"memory_search_result": "ok", "search_result": "ok", "memory_search_results": hits, "search_results": hits}
	return in
}

func Test48SummaryCurrentQueryPreservesIndirectRecall(t *testing.T) {
	for _, tc := range []struct{ name, query, target string }{
		{"group", "선원들의 깃발에는 바늘이 반달을 꿰뚫는 무늬가 그려져 있었다. 피란민을 태워주던 이들이었다.", "회색돛 조합은 바늘에 꿰인 초승달 문장을 쓴다. 난민에게 무료로 바다를 건너게 해주는 선원 단체다. 세린과 항해 협약을 맺었다."},
		{"rule", "해가 저물자 일행은 들키지 않고 거짓 정보를 전하기 위해 종이에 적는 방법을 생각한다.", "은무 지역에서는 해가 진 뒤 말한 거짓말이 푸른 불꽃으로 드러난다. 글로 적은 문장에는 이 제약이 적용되지 않는다. 세린은 이를 배웠다."},
	} {
		for _, budget := range []int{18000, 32000} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, budget), func(t *testing.T) {
				in := summary48CrowdedInput(tc.query, tc.target, budget)
				before := mustCompactJSON(in)
				out := buildPrepareTurnInjectionAssemblyWithBudget(in)
				if !strings.Contains(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), tc.target) {
					t.Fatal("current-scene historical context lost behind old-context summaries")
				}
				assert45Budget(t, out.MemoryDeliveryPlan, budget)
				if mustCompactJSON(in) != before {
					t.Fatal("assembly changed source observations")
				}
				_, summaries := multiAgentCandidatePool(&out)
				for _, s := range summaries {
					if s.SourceVectorSimilarityObserved && s.SourceVectorSimilarity != .8 {
						t.Fatal("effective ranking overwrote the observed vector similarity")
					}
				}
			})
		}
	}
}

func Test48SummaryCurrentQueryKeepsUndifferentiatedRecall(t *testing.T) {
	in := summary48CrowdedInput("Continue.", "Mira keeps her promise to Rowan.", 18000)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, summaries := multiAgentCandidatePool(&out)
	if !reflect.DeepEqual(summaries, prepareTurnBuildPriorityTurnSummaries(facts)) {
		t.Fatal("a continuation without distinguishing words changed summary ranking")
	}
}

func Test48NarrativePayloadReadingMatchesRelationFrame(t *testing.T) {
	row := store.StatusCurrentValue{ID: 1, StatusKey: narrativeStateStatusKey, WriteState: "current", ValueJSON: `{"subject":"Mira","state_slot":"inventory","value":"The compass belongs to Rowan.","direct_evidence_ids":[9223372036854775807],"unknown_metadata":{"nested":true}}`}
	views := narrativeCurrentStateViews([]store.StatusCurrentValue{row})
	full := readMemoryRelations(memoryRelationInput{CurrentStates: []store.StatusCurrentValue{row}})
	if len(views) != 1 || !reflect.DeepEqual(views[0].Payload, full.Records[0].Frames[0].Fields) {
		t.Fatal("payload-only view lost original fields or exact evidence numbers")
	}
}
