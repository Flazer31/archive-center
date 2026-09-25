package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func TestTranslationOriginal47HostBodyAcceptanceCriticAndStorage(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal("Node is required for the production translation-source replay")
		}
	}
	out, err := exec.Command(node, "../../../ops/translation-original-smoke.cjs").CombinedOutput()
	if err != nil {
		t.Fatalf("production JS replay: %v\n%s", err, out)
	}
	var replay struct {
		Status string `json:"status"`
		Cases  []struct {
			Kind     string                    `json:"kind"`
			Language string                    `json:"language"`
			Original string                    `json:"original"`
			Previous dto.M4CompleteTurnRequest `json:"previous"`
			Current  dto.M4CompleteTurnRequest `json:"current"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(out, &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Status != "passed" || len(replay.Cases) != 8 {
		t.Fatalf("incomplete replay: %s", out)
	}
	for _, tc := range replay.Cases {
		for mode, req := range map[string]dto.M4CompleteTurnRequest{"previous": tc.Previous, "current": tc.Current} {
			t.Run(tc.Kind+"/"+tc.Language+"/"+mode, func(t *testing.T) {
				decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
				if !decision.Accepted {
					t.Fatalf("original source rejected: %+v", decision)
				}
				fake := &turnRecordingStore{}
				cfg := config.Default()
				cfg.CriticLedgerEnabled = false
				srv := NewServer(cfg)
				srv.Store = fake
				wire := criticWireJSONForTest(map[string]any{
					"turn_summary": tc.Original, "importance_score": 6, "evidence_excerpts": []any{tc.Original},
				})
				calls := 0
				old := proxyHTTPClient
				proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					messages := sliceFromAny(body["messages"])
					prompt := stringFromMap(mapFromAny(messages[1]), "content")
					if !strings.Contains(prompt, tc.Original) || !strings.Contains(prompt, `"summary_language":"`+tc.Language+`"`) {
						t.Error("original text/language missing from actual Critic request")
					}
					if strings.Contains(prompt, "그녀는 약속을 지키고 조용히 마을로 돌아왔다.") {
						t.Error("translated display reached Critic")
					}
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"choices":[{"message":{"content":%s}}]}`, strconv.Quote(wire))))}, nil
				})}
				t.Cleanup(func() { proxyHTTPClient = old })
				extraction, _, err := srv.runCompleteTurnCriticWithInputPolicy(context.Background(), req.ChatSessionID, req.TurnIndex, *req.UserInput, *req.AssistantContent, nil, nil,
					completeTurnLLMConfig{Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "fixture-only", Model: "translation-fixture", TimeoutMs: 1000},
					true, completeTurnCriticInputPolicy{}, completeTurnCriticInputReplay{}, mapFromAny(req.ClientMeta["language_context"]))
				if err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("Critic calls=%d", calls)
				}
				result := srv.saveCriticExtractionArtifacts(context.Background(), req.ChatSessionID, req.TurnIndex, extraction, *req.UserInput+"\n"+*req.AssistantContent, completeTurnEmbeddingConfig{}, time.Now())
				if result.Errors != 0 || len(fake.savedMemories) != 1 {
					t.Fatalf("artifact save: %+v", result)
				}
				stored := parseJSONMap(fake.savedMemories[0].SummaryJSON)
				if stringFromMap(mapFromAny(stored["memory_write_contract"]), "summary_language") != tc.Language {
					t.Fatal("stored language changed")
				}
				if !strings.Contains(fake.savedMemories[0].Evidence, tc.Original) {
					t.Fatal("original evidence missing")
				}
			})
		}
	}
}
