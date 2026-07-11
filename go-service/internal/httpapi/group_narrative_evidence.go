package httpapi

import (
	"context"
	"errors"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// narrativeEvidence aggregates store-backed read evidence for a single session.
type narrativeEvidence struct {
	ChatLogs             []store.ChatLog
	Memories             []store.Memory
	Evidence             []store.DirectEvidence
	KGTriples            []store.KGTriple
	Storylines           []store.Storyline
	WorldRules           []store.WorldRule
	CharacterStates      []store.CharacterState
	PendingThreads       []store.PendingThread
	ActiveStates         []store.ActiveState
	CanonicalStateLayers []store.CanonicalStateLayer
	EpisodeSummaries     []store.EpisodeSummary
	ResumePack           *store.ResumePack
	AuditLogs            []store.AuditLog
	CriticFeedback       []store.CriticFeedback
	Disabled             bool
}

func (s *Server) collectNarrativeEvidence(ctx context.Context, chatSessionID string) narrativeEvidence {
	var ev narrativeEvidence
	if s.Store == nil {
		ev.Disabled = true
		return ev
	}
	disabled := false

	mark := func(err error) {
		if err != nil && errors.Is(err, store.ErrNotEnabled) {
			disabled = true
		}
	}

	if v, err := s.Store.ListChatLogs(ctx, chatSessionID, 0, 0); err != nil {
		mark(err)
	} else {
		ev.ChatLogs = v
	}
	if v, err := s.Store.ListMemories(ctx, chatSessionID, 0, 0); err != nil {
		mark(err)
	} else {
		ev.Memories = v
	}
	if v, err := s.Store.ListEvidence(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.Evidence = v
	}
	if v, err := s.Store.ListKGTriples(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.KGTriples = v
	}
	if v, err := s.Store.ListStorylines(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.Storylines = v
	}
	if v, err := s.Store.ListWorldRules(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.WorldRules = v
	}
	if v, err := s.Store.ListCharacterStates(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.CharacterStates = v
	}
	if v, err := s.Store.ListPendingThreads(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.PendingThreads = v
	}
	if v, err := s.Store.ListActiveStates(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.ActiveStates = v
	}
	if v, err := s.Store.ListCanonicalStateLayers(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.CanonicalStateLayers = v
	}
	if v, err := s.Store.ListEpisodeSummaries(ctx, chatSessionID, 0, 0, 0); err != nil {
		mark(err)
	} else {
		ev.EpisodeSummaries = v
	}
	if v, err := s.Store.GetResumePack(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.ResumePack = v
	}
	if v, err := s.Store.ListAuditLogs(ctx, chatSessionID, "", 1000); err != nil {
		mark(err)
	} else {
		ev.AuditLogs = v
	}
	if v, err := s.Store.ListCriticFeedback(ctx, chatSessionID, "", 0); err != nil {
		mark(err)
	} else {
		ev.CriticFeedback = v
	}
	ev.Disabled = disabled
	return ev
}

func sourceFromEvidence(ev narrativeEvidence) string {
	if ev.Disabled {
		return "shadow-degraded"
	}
	return "shadow"
}

func storeStatusFromEvidence(ev narrativeEvidence) string {
	if ev.Disabled {
		return "disabled"
	}
	return "active"
}

func latestTurnIndex(chatLogs []store.ChatLog) any {
	if len(chatLogs) == 0 {
		return nil
	}
	maxTurn := 0
	for _, item := range chatLogs {
		if item.TurnIndex > maxTurn {
			maxTurn = item.TurnIndex
		}
	}
	return maxTurn
}

func countAuditEvents(items []store.AuditLog, eventType string) int {
	count := 0
	for _, item := range items {
		if item.EventType == eventType {
			count++
		}
	}
	return count
}

func regressionCorpusManifest() map[string]any {
	return map[string]any{
		"policy_version":     "lc1r.v1",
		"definition_state":   "defined",
		"execution_state":    "pending_restart_replay",
		"release_gate_ready": false,
		"corpus": []any{
			map[string]any{"step": "14", "lane": "character_and_guidance", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "15", "lane": "inspection_and_context", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "16", "lane": "retrieval_temporal_foundation", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "16.5", "lane": "adaptive_governor", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "16.8", "lane": "replay_gate_and_stale_arc_suppression", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
		},
	}
}

func step17BundleClosure() map[string]any {
	return map[string]any{
		"policy_version":      "lc1s.v1",
		"bundle_label":        "Archive Center Release 1.0.0",
		"runtime_version":     "1.0.0",
		"step_context":        "21st step",
		"closure_status":      "closed",
		"release_gate_closed": true,
		"closure_scope":       "bundle_release_artifact_sync",
		"closure_mode":        "bundle_release_closed_session_cutover_separate",
		"closure_record": map[string]any{
			"source":      "historical_step17_release_gate_record",
			"recorded_at": "2026-05-07",
			"meaning":     "Step 17 closure record remains preserved inside the Step 21 Release 1.0.0 source artifact.",
		},
		"checklist": []any{
			map[string]any{"item": "historical_release_gate_record", "passed": true, "detail": "2026-05-07 current-candidate closure record"},
			map[string]any{"item": "bundle_release_artifact_sync", "passed": true, "detail": "README, BUNDLE_NOTES, plugin version markers, and runtime metadata align to the Step 21 Release 1.0.0 target while preserving Step 17 closure carry-in"},
			map[string]any{"item": "runtime_gate_surface_present", "passed": true, "detail": "Inspection, visibility, and adoption/release read-only panels are present in the bundle runtime"},
			map[string]any{"item": "fresh_bundle_embedding_baseline_ready", "passed": true, "detail": "default embedding model aligns to text-embedding-3-small so startup preflight is not falsely blocked"},
		},
		"warnings": []string{
			"Session-local adoption/release gates may remain hold or pending until live shadow signal and operator evidence are supplied.",
			"This snapshot does not approve live limited cutover or default runtime change.",
		},
	}
}
