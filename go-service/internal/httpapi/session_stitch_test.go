package httpapi

import (
	"context"
	"fmt"
	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"testing"
)

func TestSessionStitchCurrentChatUsesOffsetAfterRestart(t *testing.T) {
	s := &Server{Store: &durableRoutingBaselineStore{Store: store.NewNoopStore(), baseline: &store.SessionRoutingBaseline{MigrationID: 1, SourceSessionID: "current-before-stitch", TargetSessionID: "stitched", Mode: store.SessionMigrationModeStitch, ImportedThroughTurn: 12}}}
	for _, client := range []*routingTurnBaseline{nil, {BackendTurnAtRoute: 15, LocalPairsAtRoute: 3, Reason: "timeline_attach"}} {
		baseline := s.resolveDurableSessionRoutingBaseline(context.Background(), "stitched", client)
		for _, tc := range []struct {
			name        string
			local, want int
		}{
			{"same_request_retry", 3, 15}, {"same_host_row_reroll", 3, 15}, {"edited_row_after_assistant_delete", 3, 15}, {"new_row_identical_text", 4, 16}, {"ordinary_new_turn", 5, 17},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "pair", ObservedPairOrdinal: tc.local, Baseline: baseline})
				if got.TurnIndex != tc.want || got.ProtectedBeforeTurn != 12 || got.Resolution == "skip_pre_route_visible_pair" {
					t.Fatalf("current row frozen or misrouted: %+v", got)
				}
			})
		}
		visible := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "visible_completed", VisibleCompletedTurns: 1, Baseline: baseline})
		if visible.CompletedTurns != 13 || visible.ProtectedBeforeTurn != 12 {
			t.Fatalf("delete leaked into historical sources: %+v", visible)
		}
	}
}

func TestSessionStitchAcceptanceKeepsCurrentHostIdentity(t *testing.T) {
	ctx := context.Background()
	const oldSID, sid, pastTurns = "current-before-stitch", "stitched", 12
	storage := &durableSessionIdentityBindingStore{Store: &memoryFakeStore{}, sources: map[string][]store.MemorySourceRevision{}, baseline: &store.SessionRoutingBaseline{SourceSessionID: "original-before-an-earlier-stitch", SourceSessionIDs: []string{oldSID}, TargetSessionID: sid, Mode: store.SessionMigrationModeStitch, ImportedThroughTurn: pastTurns}}
	makeServer := func() *Server {
		return &Server{Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage, SourceAcceptances: newCompleteTurnSourceAcceptanceLedger()}
	}
	first := groupAcceptanceRequest([]string{"user-A", "user-last"}, []string{"same input", "last part"}, 1, 1000, "original answer")
	first.ChatSessionID = oldSID
	first.ClientMeta["source_acceptance_observation"].(map[string]any)["session_id"] = oldSID
	original := makeServer().beginCompleteTurnSourceAcceptance(ctx, first)
	if !original.Accepted {
		t.Fatalf("seed: %+v", original)
	}
	storage.baseline.InputGroupAliases = map[string][]string{original.LogicalTurnID: completeTurnInputGroupLogicalIDs(oldSID, original.Observation)}
	globalTurn := pastTurns + original.BoundTurn
	storage.sources[sid] = []store.MemorySourceRevision{{ChatSessionID: sid, TurnIndex: globalTurn, SourceRevision: original.Revision, LogicalTurnID: original.LogicalTurnID, HostObservedAtMS: 1000, LifecycleState: "active"}}
	for i, tc := range []struct {
		name, userID, input string
		replace             bool
	}{
		{"reroll", "user-A", "same input", true},
		{"edit_after_assistant_removal", "user-A", "edited input", true},
		{"new_row_same_text", "user-B", "same input", false},
		{"ordinary_new_turn", "user-C", "ordinary input", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := makeServer()
			req := groupAcceptanceRequest([]string{tc.userID}, []string{tc.input}, globalTurn+1, int64(2000+i*1000), fmt.Sprint("answer-", i))
			req.ChatSessionID = sid
			req.ClientMeta["source_acceptance_observation"].(map[string]any)["session_id"] = sid
			got := server.beginCompleteTurnSourceAcceptance(ctx, req)
			wantTurn := globalTurn + 1
			if tc.replace {
				wantTurn = globalTurn
			}
			if !got.Accepted || got.ReplaceExisting != tc.replace || got.BoundTurn != wantTurn {
				t.Fatalf("wrong replacement/append: %+v", got)
			}
			retry := server.beginCompleteTurnSourceAcceptance(ctx, req)
			if !retry.Accepted || retry.Revision != got.Revision || retry.BoundTurn != got.BoundTurn {
				t.Fatalf("provider retry: %+v", retry)
			}
		})
	}
}

func TestSessionStitchDeletionOnlyObservesCurrentChat(t *testing.T) {
	const sid, oldSID, past = "stitched", "current-before-stitch", 7
	storage := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), sources: map[string][]store.MemorySourceRevision{}}
	var observations []rollbackAssistantObservation
	var users []completeTurnSourceObservation
	for i := 0; i < past+12; i++ {
		user := completeTurnSourceObservation{HostChatID: "host", HostChatIDState: "observed", UserMessageChatID: fmt.Sprint("user-", i), UserMessageChatIDState: "observed"}
		source := store.MemorySourceRevision{ChatSessionID: sid, TurnIndex: i + 1, LogicalTurnID: completeTurnLogicalTurnID(oldSID, user), SourceMessageID: fmt.Sprint("answer-", i), AssistantContent: fmt.Sprint("output-", i), LifecycleState: "active"}
		storage.sources[sid] = append(storage.sources[sid], source)
		if i >= past {
			users = append(users, user)
			observations = append(observations, rollbackAssistantObservation{MessageID: source.SourceMessageID, MessageIndex: (i-past)*2 + 1})
		}
	}
	baseline := &routingTurnBaseline{BackendTurnAtRoute: past, Reason: "timeline_stitch", durableSourceID: oldSID}
	negative, _ := verifyRollbackAssistantDeletionEvidence(context.Background(), storage, sid, observations, nil)
	if negative.Verified {
		t.Fatal("fixture fails to detect old whole-session comparison")
	}
	for _, remaining := range []int{12, 11, 2, 0} {
		got, err := verifyRollbackAssistantDeletionEvidence(context.Background(), storage, sid, observations[:remaining], nil, baseline)
		if err != nil || !got.Verified || got.RemovedCount != len(observations)-remaining {
			t.Fatalf("remaining=%d: %+v %v", remaining, got, err)
		}
		if got.RemovedCount > 0 && got.FirstRemovedTurn != past+remaining+1 {
			t.Fatalf("deleted imported history: %+v", got)
		}
	}
	for _, edited := range []bool{false, true} {
		pending := users[len(users)-1]
		if edited {
			pending.UserObservedContentHash = "edited"
		}
		got, err := verifyRollbackAssistantDeletionEvidence(context.Background(), storage, sid, observations[:len(observations)-1], &pending, baseline)
		if err != nil || !got.Verified || got.RemovedCount != 0 {
			t.Fatalf("reroll/edit became deletion: %+v %v", got, err)
		}
	}
}

func TestSessionStitchBodySettingsPreserveSourceAndRetry(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	original := bodyTrackingSettingsFile{ContractVersion: "body_tracking_settings.v1", Sessions: map[string]bodyTrackingConfig{
		"old":     {CycleTrackingEnabled: true, SimulationSeed: "old-seed", Characters: []bodyCharacterConfig{{EntityID: "old-ID", CharacterName: "Mina", GestationDays: 300}}},
		"current": {AutomaticPregnancyEnabled: true, SimulationSeed: "current-seed", Characters: []bodyCharacterConfig{{EntityID: "new-ID", CharacterName: "Rin", GestationDays: 400}}},
	}}
	if err := writeBodyTrackingSettings(original); err != nil {
		t.Fatal(err)
	}
	result := &store.SessionStitchResult{TargetSessionID: "combined", Segments: []store.SessionStitchSegment{{SessionID: "old"}, {SessionID: "current"}}, EntityIDMap: map[string]string{"old-ID": "target-old-ID", "new-ID": "target-new-ID"}}
	server := &Server{}
	if err := server.stitchBodyTrackingConfig(result); err != nil {
		t.Fatal(err)
	}
	got, err := readBodyTrackingSettings()
	if err != nil {
		t.Fatal(err)
	}
	combined := got.Sessions["combined"]
	if combined.CycleTrackingEnabled || !combined.AutomaticPregnancyEnabled || len(combined.Characters) != 2 || combined.Characters[0].EntityID != "target-old-ID" || combined.Characters[0].OriginEntityID != "old-ID" || combined.Characters[0].GestationDays != 300 || combined.Characters[1].GestationDays != 400 {
		t.Fatalf("settings lost: %+v", combined)
	}
	if got.Sessions["old"].Characters[0].EntityID != "old-ID" || got.Sessions["current"].Characters[0].EntityID != "new-ID" {
		t.Fatal("original settings overwritten")
	}
	combined.Characters[0].GestationDays = 500
	got.Sessions["combined"] = combined
	if err := writeBodyTrackingSettings(got); err != nil {
		t.Fatal(err)
	}
	if err := server.stitchBodyTrackingConfig(result); err != nil {
		t.Fatal(err)
	}
	again, err := readBodyTrackingSettings()
	if err != nil || again.Sessions["combined"].Characters[0].GestationDays != 500 {
		t.Fatal("retry reset edited config", err)
	}
}
