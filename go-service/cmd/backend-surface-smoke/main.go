package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

type surfaceCheck struct {
	Phase                  string   `json:"phase"`
	SourceFunctionsSummary string   `json:"source_functions_summary"`
	GoSurface              string   `json:"go_surface"`
	Route                  string   `json:"route"`
	RequestBody            string   `json:"-"`
	ContentType            string   `json:"-"`
	Status                 string   `json:"status"`
	HTTPStatus             int      `json:"http_status"`
	JSONValid              bool     `json:"json_valid"`
	TopLevelKeys           []string `json:"top_level_keys"`
	AllowedHTTPStatuses    []int    `json:"allowed_http_statuses,omitempty"`
	MissingNotes           string   `json:"missing_notes,omitempty"`
	OpenGaps               string   `json:"open_gaps,omitempty"`
}

type report struct {
	CheckedAt string         `json:"checked_at"`
	Status    string         `json:"status"`
	Note      string         `json:"note"`
	Checks    []surfaceCheck `json:"checks"`
}

func main() {
	sessionID := flag.String("session-id", os.Getenv("AC_SMOKE_SESSION_ID"), "session id used for backend surface smoke routes")
	jsonOut := flag.String("json-out", "", "optional path to write the JSON report")
	flag.Parse()

	if *sessionID == "" {
		*sessionID = fmt.Sprintf("backend-surface-smoke-%d", time.Now().UTC().UnixNano())
	}

	rep := runSurfaceSmoke(*sessionID)

	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if *jsonOut != "" {
		if err := os.WriteFile(*jsonOut, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(1)
		}
	} else {
		os.Stdout.Write(data)
	}

	if rep.Status != "ok" {
		os.Exit(1)
	}
}

func runSurfaceSmoke(sessionID string) report {
	if sessionID == "" {
		sessionID = fmt.Sprintf("backend-surface-smoke-%d", time.Now().UTC().UnixNano())
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeNoop

	server := httpapi.NewServer(cfg)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	now := time.Now().UTC().Format(time.RFC3339)
	rep := report{
		CheckedAt: now,
		Status:    "ok",
		Note:      "2.0-4m R1 backend decomposition surface smoke. No DB/vector/LLM calls. Shadow/noop only.",
		Checks:    []surfaceCheck{},
	}

	checks := []surfaceCheck{
		// Phase 1-A: story guidance surfaces
		{
			Phase:                  "1-A",
			SourceFunctionsSummary: "_sg14_build_handoff_edge, _sg15_build_execution_contract, _sg15_build_fail_mode, _build_story_guidance_surface_sg14, _normalize_story_ledger_label_lw1a, _contains_payoff_hint_lw1a, _derive_anchor_relations_lw1b, _derive_anchor_world_lw1b, _build_story_ledger_anchor_lw1b, _classify_world_pressure_bucket_lw1d, _build_world_pressure_lw1d, _build_supporting_precedence_guard_lw1e, _build_compatibility_contract_lw1f, _build_lifecycle_model_lw1g, _lifecycle_profile_for_state_lw1g, _build_do_not_resolve_guard_lw1h, _is_long_horizon_candidate_lw1h, _attach_do_not_resolve_fields_lw1h, _build_story_ledger_lw1a, _build_progression_ledger_n2a, _build_autonomy_plan_n2b, _build_micro_beat_proposal_n3a, _build_scene_step_proposal_n3b, _build_combined_planner_n3c, _build_generation_packet_o1b",
			GoSurface:              "group_narrative.go: handleSessionGuidanceSnapshot",
			Route:                  fmt.Sprintf("GET /sessions/%s/guidance-snapshot", sessionID),
		},
		{
			Phase:                  "1-A",
			SourceFunctionsSummary: "Story guidance surface + director snapshot + progression ledger",
			GoSurface:              "group_narrative.go: handleNarrativeControlGet",
			Route:                  fmt.Sprintf("GET /narrative-control/%s", sessionID),
		},
		// Phase 1-B: temporal state surfaces
		{
			Phase:                  "1-B",
			SourceFunctionsSummary: "_build_temporal_state_surface_sc19, _build_temporal_support_packet_sc19, _normalize_sc19_relative_lookup_key, _normalize_relative_label_fields_sc19",
			GoSurface:              "group_narrative.go: handleActiveStates",
			Route:                  fmt.Sprintf("GET /active-states/%s", sessionID),
		},
		{
			Phase:                  "1-B",
			SourceFunctionsSummary: "Temporal state read + session canonical state layer",
			GoSurface:              "group_narrative.go: handleSessionState",
			Route:                  fmt.Sprintf("GET /session-state/%s", sessionID),
		},
		{
			Phase:                  "1-B",
			SourceFunctionsSummary: "Continuity pack assembly from temporal + narrative sources",
			GoSurface:              "group_narrative.go: handleContinuityPack",
			Route:                  fmt.Sprintf("GET /continuity-pack/%s", sessionID),
		},
		// Phase 1-C: resume pack surfaces
		{
			Phase:                  "1-C",
			SourceFunctionsSummary: "_bundle_resume_pack* (resume pack assembly)",
			GoSurface:              "group_narrative.go: handleSessionResumePack",
			Route:                  fmt.Sprintf("GET /sessions/%s/resume-pack?continuity_trigger_mode=resume", sessionID),
			AllowedHTTPStatuses:    []int{http.StatusNotFound},
		},
		{
			Phase:                  "1-C",
			SourceFunctionsSummary: "Resume pack idle reentry variant",
			GoSurface:              "group_narrative.go: handleSessionResumePack",
			Route:                  fmt.Sprintf("GET /sessions/%s/resume-pack?continuity_trigger_mode=idle", sessionID),
			AllowedHTTPStatuses:    []int{http.StatusNotFound},
		},
		// Phase 1-D: palace/narrative read surfaces
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Session export read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleSessionExport",
			Route:                  fmt.Sprintf("GET /sessions/%s/export", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Step7 health surface (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleSessionStep7Health",
			Route:                  fmt.Sprintf("GET /sessions/%s/step7-health", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Sessions compare / pairwise narrative evidence (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleSessionsCompare",
			Route:                  fmt.Sprintf("GET /sessions/compare?session_ids=%s,smoke-second-sid", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Canonical state layer read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleCanonicalStateLayer",
			Route:                  fmt.Sprintf("GET /canonical-state-layer/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Active scope read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleActiveScopeGet",
			Route:                  fmt.Sprintf("GET /session/%s/active-scope", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Momentum packet read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleMomentumPacket",
			Route:                  fmt.Sprintf("GET /momentum-packet/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Pending threads read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handlePendingThreads",
			Route:                  fmt.Sprintf("GET /pending-threads/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Storylines list (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleStorylinesGet",
			Route:                  fmt.Sprintf("GET /storylines/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Characters list (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleCharactersGet",
			Route:                  fmt.Sprintf("GET /characters/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Character detail read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleCharacterDetail",
			Route:                  fmt.Sprintf("GET /characters/%s/Alice", sessionID),
			AllowedHTTPStatuses:    []int{http.StatusOK, http.StatusNotFound},
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Character events read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleCharacterEvents",
			Route:                  fmt.Sprintf("GET /characters/%s/Alice/events", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "World rules list (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleWorldRulesGet",
			Route:                  fmt.Sprintf("GET /world-rules/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Inherited world rules read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleWorldRulesInherited",
			Route:                  fmt.Sprintf("GET /world-rules/%s/inherited", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Episodes list (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleEpisodesGet",
			Route:                  fmt.Sprintf("GET /episodes/%s", sessionID),
		},
		{
			Phase:                  "1-D",
			SourceFunctionsSummary: "Episode detail read (palace/narrative_read.py)",
			GoSurface:              "group_narrative.go: handleEpisodeDetail",
			Route:                  "GET /episodes/detail/1",
			AllowedHTTPStatuses:    []int{http.StatusOK, http.StatusNotFound},
		},
		// Phase 2-A: retrieval/search/memory read surfaces
		{
			Phase:                  "2-A",
			SourceFunctionsSummary: "Retrieval index runtime config read",
			GoSurface:              "group_memory.go: handleRetrievalIndexRuntimeConfigGet",
			Route:                  "GET /retrieval-index/runtime-config",
		},
		{
			Phase:                  "2-A",
			SourceFunctionsSummary: "Intent routing runtime config read",
			GoSurface:              "group_memory.go: handleIntentRoutingRuntimeConfigGet",
			Route:                  "GET /intent-routing/runtime-config",
		},
		{
			Phase:                  "2-A",
			SourceFunctionsSummary: "Retrieval index snapshot read",
			GoSurface:              "group_memory.go: handleRetrievalIndexSnapshot",
			Route:                  fmt.Sprintf("GET /retrieval-index/%s", sessionID),
		},
		{
			Phase:                  "2-A",
			SourceFunctionsSummary: "Retrieval index source row read",
			GoSurface:              "group_memory.go: handleRetrievalIndexSourceRow",
			Route:                  fmt.Sprintf("GET /retrieval-index/%s/source-row?document_id=memory:1", sessionID),
			AllowedHTTPStatuses:    []int{http.StatusOK, http.StatusNotFound},
		},
		{
			Phase:                  "2-A",
			SourceFunctionsSummary: "KG recall GET read-only path",
			GoSurface:              "group_memory.go: handleKGRecallGet",
			Route:                  fmt.Sprintf("GET /kg/recall?chat_session_id=%s", sessionID),
		},
		{
			Phase:                  "2-A",
			SourceFunctionsSummary: "Chroma shadow preflight read path",
			GoSurface:              "group_memory.go: handleChromaPreflight",
			Route:                  "GET /chroma-shadow/preflight",
		},
		// Phase 2-B: explorer read surfaces
		{
			Phase:                  "2-B",
			SourceFunctionsSummary: "get_chat_logs explorer read",
			GoSurface:              "group_memory.go: handleExplorerChatLogs",
			Route:                  fmt.Sprintf("GET /explorer/chat_logs?chat_session_id=%s", sessionID),
		},
		{
			Phase:                  "2-B",
			SourceFunctionsSummary: "get_memories explorer read",
			GoSurface:              "group_memory.go: handleExplorerMemories",
			Route:                  fmt.Sprintf("GET /explorer/memories?chat_session_id=%s", sessionID),
		},
		{
			Phase:                  "2-B",
			SourceFunctionsSummary: "get_direct_evidence explorer read",
			GoSurface:              "group_memory.go: handleExplorerDirectEvidence",
			Route:                  fmt.Sprintf("GET /explorer/direct-evidence?chat_session_id=%s", sessionID),
		},
		{
			Phase:                  "2-B",
			SourceFunctionsSummary: "get_kg_triples explorer read",
			GoSurface:              "group_memory.go: handleExplorerKGTriples",
			Route:                  fmt.Sprintf("GET /explorer/kg_triples?chat_session_id=%s", sessionID),
		},
		{
			Phase:                  "2-B",
			SourceFunctionsSummary: "get_chapter_summaries explorer read",
			GoSurface:              "group_memory.go: handleExplorerChapterSummaries",
			Route:                  fmt.Sprintf("GET /explorer/chapter_summaries?chat_session_id=%s", sessionID),
		},
		// Phase 3: metrics / diagnostic read surfaces
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1C memory footprint read",
			GoSurface:              "group_narrative.go: handleMetricsLC1C",
			Route:                  fmt.Sprintf("GET /metrics/lc1c/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1D integrity replay diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1D",
			Route:                  fmt.Sprintf("GET /metrics/lc1d/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1E KG triple diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1E",
			Route:                  fmt.Sprintf("GET /metrics/lc1e/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1F storyline diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1F",
			Route:                  fmt.Sprintf("GET /metrics/lc1f/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1G world rule diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1G",
			Route:                  fmt.Sprintf("GET /metrics/lc1g/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1H character state diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1H",
			Route:                  fmt.Sprintf("GET /metrics/lc1h/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1I pending thread diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1I",
			Route:                  fmt.Sprintf("GET /metrics/lc1i/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1J resume pack diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1J",
			Route:                  fmt.Sprintf("GET /metrics/lc1j/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1K memory diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1K",
			Route:                  fmt.Sprintf("GET /metrics/lc1k/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1L direct evidence diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1L",
			Route:                  fmt.Sprintf("GET /metrics/lc1l/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1M episode summary diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1M",
			Route:                  fmt.Sprintf("GET /metrics/lc1m/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1N active state diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1N",
			Route:                  fmt.Sprintf("GET /metrics/lc1n/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1O canonical state layer diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1O",
			Route:                  fmt.Sprintf("GET /metrics/lc1o/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1P storyline continuity diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1P",
			Route:                  fmt.Sprintf("GET /metrics/lc1p/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1Q freshness lag diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1Q",
			Route:                  fmt.Sprintf("GET /metrics/lc1q/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1R regression corpus manifest",
			GoSurface:              "group_narrative.go: handleMetricsLC1R",
			Route:                  "GET /metrics/lc1r/regression-corpus?limit=5",
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "LC1S step17 bundle closure diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsLC1S",
			Route:                  "GET /metrics/lc1s/step17-bundle-closure",
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "TM1D truth maintenance diagnostic",
			GoSurface:              "group_narrative.go: handleMetricsTM1D",
			Route:                  fmt.Sprintf("GET /metrics/tm1d/%s", sessionID),
		},
		{
			Phase:                  "3",
			SourceFunctionsSummary: "Momentum packet diagnostic surface",
			GoSurface:              "group_narrative.go: handleMomentumPacket",
			Route:                  fmt.Sprintf("GET /momentum-packet/%s", sessionID),
		},
		// Phase 4: generation packet / turn shadow surfaces
		{
			Phase:                  "4",
			SourceFunctionsSummary: "Prepare-turn generation packet read-shadow",
			GoSurface:              "group_turn.go: handlePrepareTurn",
			Route:                  "POST /prepare-turn",
			RequestBody:            fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"raw_user_input":"smoke user input","messages":[{"role":"user","content":"smoke user input"}],"settings":{"top_k":3,"max_injection_chars":1200,"max_input_context_chars":600}}`, sessionID),
			ContentType:            "application/json",
		},
		{
			Phase:                  "4",
			SourceFunctionsSummary: "Complete-turn writeback plan and side-effect boundary",
			GoSurface:              "group_turn.go: handleCompleteTurn",
			Route:                  "POST /complete-turn",
			RequestBody:            fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"user_input":"smoke user input","assistant_content":"smoke assistant output","context_messages":[{"role":"assistant","content":"smoke assistant output"}],"improvement_trace":{"source":"backend-surface-smoke"}}`, sessionID),
			ContentType:            "application/json",
		},
		{
			Phase:                  "4",
			SourceFunctionsSummary: "Repair replay shadow plan",
			GoSurface:              "group_turn.go: handleTurnsRepairReplay",
			Route:                  "POST /turns/repair-replay",
			RequestBody:            fmt.Sprintf(`{"chat_session_id":%q,"dry_run":true}`, sessionID),
			ContentType:            "application/json",
		},
		{
			Phase:                  "4",
			SourceFunctionsSummary: "Effective input save boundary",
			GoSurface:              "group_turn.go: handleEffectiveInputs",
			Route:                  "POST /effective-inputs",
			RequestBody:            fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"effective_input":"smoke effective input"}`, sessionID),
			ContentType:            "application/json",
		},
	}

	for i := range checks {
		sc := &checks[i]
		parts := strings.SplitN(sc.Route, " ", 2)
		method := parts[0]
		path := parts[1]

		var req *http.Request
		if sc.RequestBody != "" {
			req = httptest.NewRequest(method, path, strings.NewReader(sc.RequestBody))
			if sc.ContentType != "" {
				req.Header.Set("Content-Type", sc.ContentType)
			}
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		sc.HTTPStatus = rec.Code
		if rec.Code >= 200 && rec.Code < 300 {
			sc.Status = "present"
		} else if rec.Code == http.StatusNoContent {
			sc.Status = "present_no_content"
		} else if isAllowedHTTPStatus(rec.Code, sc.AllowedHTTPStatuses) {
			sc.Status = "present_not_found"
		} else {
			sc.Status = "missing_or_error"
			rep.Status = "degraded"
		}

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err == nil {
			sc.JSONValid = true
			for k := range body {
				sc.TopLevelKeys = append(sc.TopLevelKeys, k)
			}
			sort.Strings(sc.TopLevelKeys)
		} else {
			sc.JSONValid = false
			sc.MissingNotes = "Response body is not valid JSON"
		}

		sc.OpenGaps = openGapsForPhase(sc.Phase)
		rep.Checks = append(rep.Checks, *sc)
	}

	return rep
}

func isAllowedHTTPStatus(code int, allowed []int) bool {
	for _, candidate := range allowed {
		if code == candidate {
			return true
		}
	}
	return false
}

func openGapsForPhase(phase string) string {
	switch phase {
	case "1-D":
		return "Go route surface exists in noop mode; palace/narrative_read.py extraction and value-level compare evidence remain incomplete."
	case "2-A":
		return "Go retrieval read surface exists in noop mode; services/retrieval_read.py extraction, real Store/Vector data parity, and live Chroma evidence remain incomplete."
	case "2-B":
		return "Go explorer read surface exists in noop mode; services/explorer.py read extraction and Store-backed value-level compare evidence remain incomplete."
	case "3":
		return "Go metrics/diagnostic route surface exists in noop mode; real Store-backed metric parity and production baseline thresholds remain governed by separate value and runtime reports."
	case "4":
		return "Go turn packet route surface exists in noop mode with LLM/write side effects disabled; product default switch and post-switch replay remain separate gates."
	default:
		return "Go shadow returns heuristic/skeleton data using noop store; Python-side story_guidance.py, temporal_state.py, and resume_pack.py extraction remain incomplete."
	}
}
