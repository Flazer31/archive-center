package httpapi

import (
	"net/http"
	"regexp"
)

var htmlImgTagPattern = regexp.MustCompile(`<img=[^>]*>`)

// registerNarrativeRoutes mounts session, narrative, character, world-rule,
// metric, feedback, and import endpoints.
func (s *Server) registerNarrativeRoutes(mux *http.ServeMux) {
	// Session read: R1
	mux.HandleFunc("GET /sessions", s.handleSessionsList)
	mux.HandleFunc("GET /sessions/{chat_session_id}/export", s.handleSessionExport)
	mux.HandleFunc("GET /sessions/{chat_session_id}/guidance-snapshot", s.handleSessionGuidanceSnapshot)
	mux.HandleFunc("GET /sessions/{chat_session_id}/step7-health", s.handleSessionStep7Health)
	mux.HandleFunc("GET /sessions/{chat_session_id}/resume-pack", s.handleSessionResumePack)
	mux.HandleFunc("GET /sessions/compare", s.handleSessionsCompare)
	mux.HandleFunc("GET /sessions/{sid}", s.handleSessionsGet404)
	mux.HandleFunc("DELETE /sessions/{chat_session_id}", s.handleSessionDelete)
	mux.HandleFunc("GET /active-states/{chat_session_id}", s.handleActiveStates)
	mux.HandleFunc("GET /canonical-state-layer/{chat_session_id}", s.handleCanonicalStateLayer)
	mux.HandleFunc("GET /session-state/{chat_session_id}", s.handleSessionState)
	mux.HandleFunc("GET /continuity-pack/{chat_session_id}", s.handleContinuityPack)
	mux.HandleFunc("GET /pending-threads/{chat_session_id}", s.handlePendingThreads)
	mux.HandleFunc("GET /continuity-hooks/{chat_session_id}", s.handleContinuityHooks)
	mux.HandleFunc("GET /narrative-recall/packet/preview", s.handleNarrativeRecallPacketPreview)
	mux.HandleFunc("GET /session/{chat_session_id}/active-scope", s.handleActiveScopeGet)
	mux.HandleFunc("GET /session/{sid}", s.handleSessionGet404)
	mux.HandleFunc("GET /momentum-packet/{chat_session_id}", s.handleMomentumPacket)
	mux.HandleFunc("GET /narrative-control/{chat_session_id}", s.handleNarrativeControlGet)

	// Session write: R2
	mux.HandleFunc("PATCH /session/{chat_session_id}/active-scope", s.handleActiveScopePatch)
	mux.HandleFunc("PATCH /narrative-control/{chat_session_id}/director-patch", s.handleDirectorPatch)

	// Storyline: R1 read, R2 write
	mux.HandleFunc("GET /storylines/{chat_session_id}", s.handleStorylinesGet)
	mux.HandleFunc("PATCH /storylines/{storyline_id}", s.handleStorylinePatch)
	mux.HandleFunc("PATCH /storylines/{storyline_id}/trust", s.handleStorylineTrust)
	mux.HandleFunc("DELETE /storylines/{storyline_id}", s.handleStorylineDelete)
	mux.HandleFunc("POST /storylines/sync", s.handleStorylinesSync)

	// Character: R1 read, R2 write
	mux.HandleFunc("GET /characters/{chat_session_id}", s.handleCharactersGet)
	mux.HandleFunc("GET /characters/{chat_session_id}/{character_name}", s.handleCharacterDetail)
	mux.HandleFunc("GET /characters/{chat_session_id}/{character_name}/events", s.handleCharacterEvents)
	mux.HandleFunc("GET /characters/{chat_session_id}/{character_name}/state-history", s.handleCharacterStateHistory)
	mux.HandleFunc("PATCH /characters/{chat_session_id}/{character_name}", s.handleCharacterPatch)
	mux.HandleFunc("PATCH /characters/{chat_session_id}/{character_name}/speech", s.handleCharacterSpeech)
	mux.HandleFunc("DELETE /characters/{chat_session_id}/{character_name}", s.handleCharacterDelete)

	// World rules: R1 read, R2 write
	mux.HandleFunc("GET /world-rules/{chat_session_id}", s.handleWorldRulesGet)
	mux.HandleFunc("GET /world-rules/{chat_session_id}/inherited", s.handleWorldRulesInherited)
	mux.HandleFunc("POST /world-rules/sync", s.handleWorldRulesSync)
	mux.HandleFunc("PATCH /world-rules/{rule_id}", s.handleWorldRulePatch)
	mux.HandleFunc("PATCH /world-rules/{rule_id}/trust", s.handleWorldRuleTrust)
	mux.HandleFunc("DELETE /world-rules/{rule_id}", s.handleWorldRuleDelete)

	// Episodes: R1 read/search, R2 generate/write
	mux.HandleFunc("GET /episodes/{chat_session_id}", s.handleEpisodesGet)
	mux.HandleFunc("GET /episodes/detail/{episode_id}", s.handleEpisodeDetail)
	mux.HandleFunc("POST /episodes/generate", s.handleEpisodeGenerate)
	mux.HandleFunc("POST /chapters/generate", s.handleChapterGenerate)
	mux.HandleFunc("POST /arcs/generate", s.handleArcGenerate)
	mux.HandleFunc("POST /sagas/generate", s.handleSagaGenerate)
	mux.HandleFunc("POST /chapters/dry-run", s.handleChapterDryRun)
	mux.HandleFunc("POST /chapters/search", s.handleChapterSearch)
	mux.HandleFunc("POST /episodes/search", s.handleEpisodeSearch)
	mux.HandleFunc("PATCH /episodes/{episode_id}", s.handleEpisodePatch)
	mux.HandleFunc("DELETE /episodes/{episode_id}", s.handleEpisodeDelete)
	mux.HandleFunc("POST /episodes/regenerate", s.handleEpisodeRegenerate)
	mux.HandleFunc("POST /episodes/merge", s.handleEpisodeMerge)

	// Pending threads: R1 read, R2 write
	mux.HandleFunc("PATCH /pending-threads/{hook_id}", s.handlePendingThreadPatch)
	mux.HandleFunc("PATCH /continuity-hooks/{hook_id}", s.handlePendingThreadPatch)
	mux.HandleFunc("PATCH /pending-threads/{hook_id}/trust", s.handlePendingThreadTrust)
	mux.HandleFunc("DELETE /pending-threads/{hook_id}", s.handlePendingThreadDelete)

	// Metrics: R1 read
	mux.HandleFunc("GET /metrics/lc1c/{chat_session_id}", s.handleMetricsLC1C)
	mux.HandleFunc("GET /metrics/lc1d/{chat_session_id}", s.handleMetricsLC1D)
	mux.HandleFunc("GET /metrics/lc1e/{chat_session_id}", s.handleMetricsLC1E)
	mux.HandleFunc("GET /metrics/lc1f/{chat_session_id}", s.handleMetricsLC1F)
	mux.HandleFunc("GET /metrics/lc1g/{chat_session_id}", s.handleMetricsLC1G)
	mux.HandleFunc("GET /metrics/lc1h/{chat_session_id}", s.handleMetricsLC1H)
	mux.HandleFunc("GET /metrics/lc1i/{chat_session_id}", s.handleMetricsLC1I)
	mux.HandleFunc("GET /metrics/lc1j/{chat_session_id}", s.handleMetricsLC1J)
	mux.HandleFunc("GET /metrics/lc1k/{chat_session_id}", s.handleMetricsLC1K)
	mux.HandleFunc("GET /metrics/lc1l/{chat_session_id}", s.handleMetricsLC1L)
	mux.HandleFunc("GET /metrics/lc1m/{chat_session_id}", s.handleMetricsLC1M)
	mux.HandleFunc("GET /metrics/lc1n/{chat_session_id}", s.handleMetricsLC1N)
	mux.HandleFunc("GET /metrics/lc1o/{chat_session_id}", s.handleMetricsLC1O)
	mux.HandleFunc("GET /metrics/lc1p/{chat_session_id}", s.handleMetricsLC1P)
	mux.HandleFunc("GET /metrics/lc1q/{chat_session_id}", s.handleMetricsLC1Q)
	mux.HandleFunc("GET /metrics/lc1r/regression-corpus", s.handleMetricsLC1R)
	mux.HandleFunc("GET /metrics/lc1s/step17-bundle-closure", s.handleMetricsLC1S)
	mux.HandleFunc("GET /metrics/tm1d/{chat_session_id}", s.handleMetricsTM1D)

	// Audit / feedback / import
	mux.HandleFunc("GET /audit", s.handleAuditGet)
	mux.HandleFunc("POST /feedback", s.handleFeedbackPost)
	mux.HandleFunc("GET /feedback/latest", s.handleFeedbackLatest)
	mux.HandleFunc("POST /import/hypamemory", s.handleImportHypamemory)
}
