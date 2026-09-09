package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Exercise both production JS rescan passes against the real Go routing API.
// Only Host reads and Store I/O are fixtures; no LLM, user's DB or live backend.
func TestColdStartProductionMergeReproduction(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"plain", "translated", "assistant_only", "all_inherited"} {
		t.Run(variant, func(t *testing.T) {
			parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "root-u"}, {MessageIndex: 1, Role: "char", MessageChatID: "root-a"}}
			source := activeSourceRevisionForUserAnchor("A", "root", "root-u", "root-a", len(parents)/2)
			st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00root": "A", "stable\x00branch": "B"}, sources: map[string][]store.MemorySourceRevision{"A": {source}}}
			server := &Server{Store: st}
			lineage := server.resolveRisuWorldlineObservation(context.Background(), originBranchRequest("root", "branch", parents, false), "B")
			if lineage.State != "confirmed" {
				t.Fatalf("setup lineage: %+v", lineage)
			}
			mux := http.NewServeMux()
			server.RegisterRoutes(mux)
			local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/session-routing/turn-resolution" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected route", 400)
					return
				}
				mux.ServeHTTP(w, r)
			}))
			defer local.Close()
			cmd := exec.Command(node, filepath.Join(root, "tests", "fixtures", "worldline-coldstart-probe.cjs"), filepath.Join(root, "Archive Center.js"), local.URL, variant)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("production probe: %v\n%s", err, output)
			}
			var report struct {
				Original string `json:"original"`
				Entries  []struct {
					Turn      int    `json:"turn_index"`
					Assistant string `json:"assistant_content"`
				} `json:"entries"`
				Calls []struct {
					Result []struct {
						Resolution string `json:"resolution"`
						Turn       int    `json:"turn_index"`
					} `json:"result"`
				} `json:"routeCalls"`
			}
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("decode probe: %v\n%s", err, output)
			}
			if dir := os.Getenv("ARCHIVE_CENTER_REPRO_REPORT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, "cold-start-"+variant+".json"), output, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if len(report.Calls) != 2 || len(report.Calls[1].Result) == 0 {
				t.Fatal("second routing pass missing")
			}
			if report.Calls[1].Result[0].Resolution != "skip_pre_route_visible_pair" {
				t.Fatalf("Go did not identify inherited prefix: %+v", report.Calls[1])
			}
			wantCount := 1
			if variant == "all_inherited" {
				wantCount = 0
			}
			if len(report.Entries) != wantCount {
				t.Errorf("repair entry count=%d want=%d; inheritedThrough=%d entries=%+v", len(report.Entries), wantCount, lineage.InheritedThroughTurn, report.Entries)
			}
			for _, entry := range report.Entries {
				if entry.Turn <= lineage.InheritedThroughTurn {
					t.Errorf("inherited turn re-entered repair: %+v", entry)
				}
				if entry.Turn > lineage.InheritedThroughTurn && entry.Assistant != report.Original {
					t.Errorf("canonical assistant overwritten: got=%q want=%q", entry.Assistant, report.Original)
				}
			}
		})
	}
}

// The same pending-source fixture now verifies that lineage and existing memory
// scope are available before the parent's normal delayed persistence runs.
func TestWorldlinePendingSourceRecoveryReproduction(t *testing.T) {
	for _, reissue := range []bool{false, true} {
		name := "preserved_ids"
		if reissue {
			name = "reissued_ids"
		}
		t.Run(name, func(t *testing.T) {
			parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "u1"}, {MessageIndex: 1, Role: "char", MessageChatID: "a1"}}
			firstSource := activeSourceRevisionForUserAnchor("A", "root", "u1", "a1", len(parents)/2)
			st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00root": "A", "stable\x00branch": "B"}, sources: map[string][]store.MemorySourceRevision{"A": {firstSource}}, history: map[string][]store.MemorySourceRevision{"A": {firstSource}}}
			server := &Server{Store: st}
			parentReq := originBranchRequest("root", "branch", parents, reissue)
			if vm := server.resolveRisuWorldlineObservation(context.Background(), parentReq, "B"); vm.State != "confirmed" {
				t.Fatalf("parent: %+v", vm)
			}
			branchMessages := append([]risuWorldlineMessageObservation(nil), parentReq.WorldlineObservation.MessageOrigins.ChildMessages...)
			branchMessages = append(branchMessages, risuWorldlineMessageObservation{MessageIndex: 2, Role: "char", Disabled: true}, risuWorldlineMessageObservation{MessageIndex: 3, Role: "user", MessageChatID: "u2"}, risuWorldlineMessageObservation{MessageIndex: 4, Role: "char", MessageChatID: "a2"})
			req := originBranchRequest("branch", "child", branchMessages, reissue)
			missing := server.resolveRisuWorldlineObservation(context.Background(), req, "C")
			before := resolvePrepareTurnHistoryScope(context.Background(), st, "C", 0)
			if missing.State != "confirmed" || missing.Reason != "official_branch_marker_observed_prefix_validated" || missing.ParentSessionID != "B" || missing.ForkTurn != firstSource.TurnIndex+1 {
				t.Fatalf("missing: %+v", missing)
			}
			if len(before.Segments) != 3 || before.Segments[0].SessionID != "A" || before.Segments[1].SessionID != "B" {
				t.Fatalf("before: %+v", before)
			}
			if len(st.sources["B"]) != 0 {
				t.Fatal("lineage resolution persisted the parent's pending response")
			}
			// Store boundary transition: the pending parent's source becomes available.
			// Actual next-input hook/DB persistence is covered separately, not simulated here.
			lastSource := activeSourceRevisionForUserAnchor("B", "branch", "u2", "a2", firstSource.TurnIndex+1)
			st.sources["B"] = []store.MemorySourceRevision{lastSource}
			st.history["B"] = []store.MemorySourceRevision{lastSource}
			recovered := server.resolveRisuWorldlineObservation(context.Background(), req, "C")
			after := resolvePrepareTurnHistoryScope(context.Background(), st, "C", 0)
			if recovered.State != "confirmed" || recovered.ParentSessionID != "B" || recovered.ForkTurn != lastSource.TurnIndex {
				t.Fatalf("recovered: %+v", recovered)
			}
			if len(after.Segments) != 3 || after.Segments[0].SessionID != "A" || after.Segments[1].SessionID != "B" {
				t.Fatalf("after: %+v", after)
			}
			t.Logf("before=%s scope=%+v; source stored; after=%s scope=%+v", missing.Reason, before.Segments, recovered.State, after.Segments)
		})
	}
}

// Pending endpoints use their observed place after a stored turn, including
// offsets after migration/deletion and complete-turn sources without char IDs.
func TestWorldlinePendingObservedPrefixCoordinates(t *testing.T) {
	for _, reissue := range []bool{false, true} {
		for _, endpointRole := range []string{"user", "char"} {
			for _, saved := range []bool{false, true} {
				t.Run(fmt.Sprintf("reissue=%t/%s/saved=%t", reissue, endpointRole, saved), func(t *testing.T) {
					parents := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "char", MessageChatID: "greeting"}}
					st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{"stable\x00root": "A"}, sources: map[string][]store.MemorySourceRevision{}}
					base := 0
					if saved {
						// The displayed window no longer starts at canonical turn one.
						source := activeSourceRevisionForUserAnchor("A", "root", "saved-u", "", 12)
						base = source.TurnIndex
						st.sources["A"] = []store.MemorySourceRevision{source}
						parents = append(parents, risuWorldlineMessageObservation{MessageIndex: len(parents), Role: "user", MessageChatID: "saved-u"}, risuWorldlineMessageObservation{MessageIndex: len(parents) + 1, Role: "char", MessageChatID: "saved-a"})
					}
					parents = append(parents, risuWorldlineMessageObservation{MessageIndex: len(parents), Role: "user", MessageChatID: "pending-u"})
					if endpointRole == "char" {
						parents = append(parents, risuWorldlineMessageObservation{MessageIndex: len(parents), Role: "char", MessageChatID: "pending-a"})
					}
					req := originBranchRequest("root", "branch", parents, reissue)
					server := &Server{Store: st}
					vm := server.resolveRisuWorldlineObservation(context.Background(), req, "B")
					want := base + 1 // exactly one new user row after the known coordinate
					through := want
					if endpointRole == "user" {
						through--
					}
					if vm.State != "confirmed" || vm.ForkTurn != want || vm.InheritedThroughTurn != through {
						t.Fatalf("pending endpoint: %+v; want turn=%d through=%d", vm, want, through)
					}
					// A later parent source cannot extend an already copied prefix.
					st.sources["A"] = append(st.sources["A"], activeSourceRevisionForUserAnchor("A", "root", "future-u", "future-a", want+1))
					got := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), req, "B")
					if got.ForkTurn != want || got.InheritedThroughTurn != through {
						t.Fatalf("parent future changed cut: %+v", got)
					}
				})
			}
		}
	}
}

func TestWorldlinePendingDeepRebranchBeforeAnySourceSave(t *testing.T) {
	for _, reissue := range []bool{false, true} {
		t.Run(fmt.Sprintf("reissue=%t", reissue), func(t *testing.T) {
			messages := []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "pending-u"}, {MessageIndex: 1, Role: "char", MessageChatID: "pending-a"}}
			st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{}, sources: map[string][]store.MemorySourceRevision{}}
			parentID, parentHost := "A", "root"
			want := len(messages) / 2
			for _, child := range []string{"B", "C", "D", "E"} {
				st.bindings["stable\x00"+parentHost] = parentID
				req := originBranchRequest(parentHost, child, messages, reissue)
				vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), req, child)
				if vm.State != "confirmed" || vm.ParentSessionID != parentID || vm.ForkTurn != want {
					t.Fatalf("child=%s: %+v", child, vm)
				}
				messages = append([]risuWorldlineMessageObservation(nil), req.WorldlineObservation.MessageOrigins.ChildMessages...)
				messages = append(messages,
					risuWorldlineMessageObservation{MessageIndex: len(messages), Role: "char", Disabled: true},
					risuWorldlineMessageObservation{MessageIndex: len(messages) + 1, Role: "user", MessageChatID: child + "-own-u"},
					risuWorldlineMessageObservation{MessageIndex: len(messages) + 2, Role: "char", MessageChatID: child + "-own-a"})
				parentID, parentHost = child, child
				want++
			}
			scope := resolvePrepareTurnHistoryScope(context.Background(), st, parentID, 0)
			if len(scope.Segments) != 5 {
				t.Fatalf("ancestor scope missing: %+v", scope)
			}
			if len(st.sources) != 0 {
				t.Fatal("branch observation admitted pending responses")
			}
		})
	}
}
