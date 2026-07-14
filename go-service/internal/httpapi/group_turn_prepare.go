package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handlePrepareTurn(w http.ResponseWriter, r *http.Request) {
	timing := newBackendTimingTrace("prepare_turn.backend_timing.v1")
	decodeStartedAt := time.Now()
	var req dto.PrepareTurnRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	timing.addElapsed("request_decode", decodeStartedAt)

	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	migrationStartedAt := time.Now()
	if lock, err := s.sessionMigrationSourceLock(r.Context(), sid); err != nil {
		writeInternalError(w, err.Error())
		return
	} else if lock != nil {
		timing.addElapsed("migration_guard", migrationStartedAt)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                "ok",
			"source":                "shadow",
			"chat_session_id":       sid,
			"generated_at":          time.Now().UTC().Format(time.RFC3339),
			"request_type":          stringPtrValue(req.RequestType, "model"),
			"fallback_reason":       "source_session_migrated_away",
			"effective_user_input":  stringPtrValue(req.RawUserInput, ""),
			"injection_text":        "",
			"input_context_text":    "",
			"migration_source_lock": sessionMigrationLockPayload(lock),
			"read_excluded":         true,
			"read_exclusion_reason": "source_session_migrated_away",
			"target_session_id":     lock.TargetSessionID,
			"trace_preview":         map[string]any{"would_call_llm": false, "would_write": false, "migration_source_lock": sessionMigrationLockPayload(lock)},
			"recall_result":         map[string]any{"status": "skipped", "reason": "source_session_migrated_away"},
			"runtime_toggle":        map[string]any{"source_session_migrated_away": true, "target_session_id": lock.TargetSessionID},
			"warnings":              []string{"source_session_migrated_away: continue in target_session_id " + lock.TargetSessionID},
			"backend_timing":        timing.snapshot(),
		})
		return
	}
	timing.addElapsed("migration_guard", migrationStartedAt)

	// Resolve settings from the request/default DTO contract.
	defaultSettings := dto.PrepareTurnSettings{}
	defaultSettings.ApplyDefaults()
	maxInjectionChars := prepareTurnIntSetting(req.Settings.MaxInjectionChars, defaultSettings.MaxInjectionChars)
	maxInputContextChars := prepareTurnIntSetting(req.Settings.MaxInputContextChars, defaultSettings.MaxInputContextChars)
	injectionEnabled := true
	inputContextEnabled := true
	memoryTopK := prepareTurnIntSetting(req.Settings.TopK, defaultSettings.TopK)
	supportRecallLimit := prepareTurnSupportRecallLimit(memoryTopK)

	if req.Settings.InjectionEnabled != nil {
		injectionEnabled = *req.Settings.InjectionEnabled
	}
	if req.Settings.InputContextEnabled != nil {
		inputContextEnabled = *req.Settings.InputContextEnabled
	}
	rawUserInput := stringPtrValue(req.RawUserInput, "")
	turnIndex := intPtrValue(req.TurnIndex, 0)
	languageContext := completeTurnLanguageContextFromClientMeta(req.ClientMeta)
	perspectiveContext := prepareTurnPerspectiveContextFromRequest(req)

	// Read assembly from Store (no writes, no LLM).
	var memories []store.Memory
	var kgTriples []store.KGTriple
	var evidence []store.DirectEvidence
	var chatLogs []store.ChatLog
	var resumePack *store.ResumePack
	var storylines []store.Storyline
	var worldRules []store.WorldRule
	var charStates []store.CharacterState
	var charEvents []store.CharacterEvent
	var pendingThreads []store.PendingThread
	var activeStates []store.ActiveState
	var canonicalLayers []store.CanonicalStateLayer
	var episodeSums []store.EpisodeSummary
	var personaEntries []store.PersonaMemoryEntry
	var characterPrivateMemories []store.ProtagonistEntityMemory
	var narrativeCurrentValues []store.StatusCurrentValue

	readErrs := []error{}
	readsOK := 0
	sessionStateReads := map[string]bool{}

	storeReadsStartedAt := time.Now()
	if s.Store != nil {
		ctx := r.Context()
		if m, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			memories = m
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if k, err := s.Store.ListKGTriples(ctx, sid); err == nil {
			kgTriples = k
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if e, err := s.Store.ListEvidence(ctx, sid); err == nil {
			evidence = e
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if c, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			chatLogs = c
			readsOK++
			sessionStateReads["chat_logs"] = true
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if rp, err := s.Store.GetResumePack(ctx, sid, "prepare_turn"); err == nil {
			resumePack = rp
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			readErrs = append(readErrs, err)
		}
		if sl, err := s.Store.ListStorylines(ctx, sid); err == nil {
			storylines = sl
			readsOK++
			sessionStateReads["storylines"] = true
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if wr, err := s.Store.ListWorldRules(ctx, sid); err == nil {
			worldRules = wr
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if cs, err := s.Store.ListCharacterStates(ctx, sid); err == nil {
			charStates = cs
			readsOK++
			sessionStateReads["character_states"] = true
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if ce, err := s.Store.ListCharacterEvents(ctx, sid, ""); err == nil {
			charEvents = ce
			sessionStateReads["character_events"] = true
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if pt, err := s.Store.ListPendingThreads(ctx, sid, ""); err == nil {
			pendingThreads = pt
			readsOK++
			sessionStateReads["pending_threads"] = true
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if as, err := s.Store.ListActiveStates(ctx, sid, ""); err == nil {
			activeStates = as
			readsOK++
			sessionStateReads["active_states"] = true
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if cl, err := s.Store.ListCanonicalStateLayers(ctx, sid, ""); err == nil {
			canonicalLayers = cl
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if es, err := s.Store.ListEpisodeSummaries(ctx, sid, supportRecallLimit, 0, 0); err == nil {
			episodeSums = es
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if personaStore, ok := s.Store.(store.PersonaCapsuleStore); ok {
			if entries, err := personaStore.ListAttachedPersonaMemoryEntries(ctx, sid, supportRecallLimit); err == nil {
				for _, entry := range entries {
					if personaMemoryEntryIsCharacterPrivate(entry) {
						characterPrivateMemories = append(characterPrivateMemories, personaMemoryEntryAsCharacterPrivateMemory(entry, sid))
						continue
					}
					personaEntries = append(personaEntries, entry)
				}
				readsOK++
			} else if !errors.Is(err, store.ErrNotEnabled) {
				readErrs = append(readErrs, err)
			}
		}
		if entityStore, ok := s.Store.(store.ProtagonistEntityMemoryStore); ok {
			memories, err := entityStore.ListProtagonistEntityMemories(ctx, store.ProtagonistEntityMemoryFilter{
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: sid,
				Limit:               supportRecallLimit,
			})
			if err == nil {
				characterPrivateMemories = append(characterPrivateMemories, memories...)
				if len(memories) > 0 {
					readsOK++
				}
			} else if !errors.Is(err, store.ErrNotEnabled) {
				readErrs = append(readErrs, err)
			}
		}
		if valueStore, ok := s.Store.(store.StatusCurrentValueStore); ok {
			if values, err := valueStore.ListStatusCurrentValues(ctx, sid, "", "", narrativeStateStatusKey, 1000); err == nil {
				narrativeCurrentValues = values
				readsOK++
			} else if !errors.Is(err, store.ErrNotEnabled) {
				readErrs = append(readErrs, err)
			}
		}
	}
	timing.addElapsed("store_reads", storeReadsStartedAt)

	recollectionStartedAt := time.Now()
	recollectionRelevance := filterPrepareTurnEntityRecollections(rawUserInput, chatLogs, activeStates, canonicalLayers, personaEntries, &characterPrivateMemories)
	timing.addElapsed("recollection_filter", recollectionStartedAt)

	degraded := readsOK == 0
	fallbackReason := ""
	if degraded {
		fallbackReason = "store_unavailable"
	} else if len(readErrs) > 0 {
		fallbackReason = "partial_reads"
	}

	var storylineReferenceTurn *int
	if turnIndex > 0 {
		value := turnIndex
		storylineReferenceTurn = &value
	}
	storylineSelection := selectStorylinesForSupervisor(storylines, storylineReferenceTurn, supportRecallLimit)
	selectedStorylines := selectedStorylineItems(storylineSelection)

	profile := strings.TrimSpace(clientMetaString(req.ClientMeta, "context_window_profile"))
	if profile == "" {
		profile = "default"
	}
	injectionAssembly := prepareTurnInjectionAssembly{}
	documents := []map[string]any{}
	vectorStartedAt := time.Now()
	vectorShadow := s.prepareTurnVectorShadow(r.Context(), req, memoryTopK)
	timing.addElapsed("vector_recall", vectorStartedAt)
	injectionStartedAt := time.Now()
	if !degraded {
		documents = buildUnifiedRetrievalDocuments(sid, memories, evidence, kgTriples, episodeSums, resumePack, chatLogs)
		if injectionEnabled {
			assemblyPerspectiveContext := prepareTurnPerspectiveWithNarrativeState(perspectiveContext, narrativeCurrentValues, activeStates)
			injectionAssembly = buildPrepareTurnInjectionAssembly(memories, kgTriples, evidence, chatLogs, selectedStorylines, worldRules, charStates, pendingThreads, canonicalLayers, episodeSums, resumePack, personaEntries, characterPrivateMemories, memoryTopK, maxInjectionChars, rawUserInput, profile, documents, vectorShadow, languageContext, assemblyPerspectiveContext)
		}
	}
	timing.addElapsed("injection_assembly", injectionStartedAt)
	referenceRecallStartedAt := time.Now()
	referenceSceneContext := buildReferenceCoverageSceneContext(chatLogs, activeStates, canonicalLayers, worldRules, supportRecallLimit)
	referenceSceneContext.ActiveRules = referenceCoverageRenderedActiveRules(injectionAssembly.WorldRulesText)
	referenceRecall := s.buildSessionReferenceRecallWithSceneContext(r.Context(), sid, rawUserInput, memoryTopK, req.ClientMeta, req.Messages, referenceSceneContext)
	referenceInjectionEnabled := referenceRecall.LiveBindingCount > 0
	referenceInjectionText := ""
	if referenceInjectionEnabled {
		referenceBudget := maxInjectionChars
		if referenceBudget <= 0 {
			referenceBudget = 3000
		}
		remaining := referenceBudget - len(injectionAssembly.Text)
		if remaining > 0 {
			referenceInjectionText = formatReferenceRecallInjection(referenceRecall, remaining)
			if referenceInjectionText != "" {
				injectionAssembly.Text = strings.TrimSpace(injectionAssembly.Text + "\n\n" + referenceInjectionText)
			}
		}
	}
	timing.addElapsed("reference_recall", referenceRecallStartedAt)
	referenceInjectedCount := strings.Count(referenceInjectionText, "\n- [")
	injectionText := injectionAssembly.Text
	injectionTruncated := injectionAssembly.Truncated

	var inputContextText string
	var inputContextTruncated bool
	if inputContextEnabled && !degraded {
		inputContextText, inputContextTruncated = buildInputContextText(evidence, chatLogs, resumePack, activeStates, canonicalLayers, episodeSums, personaEntries, characterPrivateMemories, maxInputContextChars, supportRecallLimit)
	}
	responseAssemblyStartedAt := time.Now()
	currentStoryClock19 := resolveCurrentStoryClock(activeStates, chatLogs, canonicalLayers)
	temporalRelationLedger19 := buildTemporalRelationLedger(activeStates)
	temporalSupportPacket := buildTemporalSupportPacket(currentStoryClock19, temporalRelationLedger19)
	guideMode := resolveNarrativeGuideMode(stringPtrValue(req.Settings.GuideMode, "off"), nil, "", rawUserInput)
	narrativeStance := stringPtrValue(req.Settings.NarrativeStance, "balanced")
	continuityTriggerMode := stringPtrValue(req.ContinuityTriggerMode, "none")
	continuityQuery := stringPtrValue(req.ContinuityQuery, "")
	requestType := stringPtrValue(req.RequestType, "model")
	applyMode := stringPtrValue(req.Settings.ApplyMode, "shadow")
	promptAssembly := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	evidenceCounts := prepareTurnEvidenceCounts(memories, kgTriples, evidence, chatLogs, resumePack, storylines, worldRules, charStates, pendingThreads, activeStates, canonicalLayers, episodeSums)
	evidenceCounts["storyline_selected_count"] = len(storylineSelection.Selected)
	evidenceCounts["storyline_dropped_count"] = len(storylineSelection.Dropped)
	evidenceCounts["storyline_stale_dropped_count"] = storylineSelectionSummary(storylineSelection)["stale_dropped_count"]
	sectionSummary := prepareTurnSectionSummary(injectionText, inputContextText, injectionTruncated, inputContextTruncated)
	guideStrength := normalizeNarrativeGuideStrength(stringPtrValue(req.Settings.GuideStrength, "weak"))
	supervisorInputPack := buildSupervisorInputPack(
		sid,
		turnIndex,
		rawUserInput,
		guideMode,
		guideStrength,
		narrativeStance,
		continuityTriggerMode,
		continuityQuery,
		promptAssembly,
		evidenceCounts,
		sectionSummary,
		storylineSelection,
		degraded,
		fallbackReason,
		languageContext,
	)
	criticInputPack := buildCriticInputPack(sid, turnIndex, rawUserInput, promptAssembly, evidenceCounts, sectionSummary, degraded)
	injectionPack := buildInjectionPack(rawUserInput, inputContextText, injectionEnabled || referenceInjectionEnabled, inputContextEnabled, inputContextTruncated, injectionAssembly, temporalSupportPacket)
	injectionPack["reference_text"] = nilIfEmpty(referenceInjectionText)
	injectionPack["reference_applied"] = referenceInjectionText != ""
	injectionPack["reference_selected_count"] = referenceInjectedCount

	queryPreview := rawUserInput

	recallResult := buildRecallResult(
		sid,
		queryPreview,
		degraded,
		memories,
		evidence,
		kgTriples,
		episodeSums,
		chatLogs,
		resumePack,
		vectorShadow,
		storylines,
		worldRules,
		pendingThreads,
		profile,
		memoryTopK,
	)

	packetMode := "store_backed_shadow"
	if degraded {
		packetMode = "off"
	}

	var injectionOut any = nil
	if injectionText != "" {
		injectionOut = injectionText
	}
	var inputContextOut any = nil
	if inputContextText != "" {
		inputContextOut = inputContextText
	}

	sessionState := buildSessionState(sid, degraded, activeStates, storylines, charStates, charEvents, chatLogs, worldRules, pendingThreads, sessionStateReads)
	narrativeControl := buildNarrativeControl(degraded, storylines, worldRules, pendingThreads, charStates)
	continuityPack := buildContinuityPack(sid, queryPreview, degraded, resumePack, episodeSums, chatLogs, activeStates, canonicalLayers, supportRecallLimit)
	progressionLedger := buildProgressionLedger(sid, degraded, storylines, worldRules, pendingThreads, episodeSums, supportRecallLimit)
	personaRecollection := buildPersonaRecollectionSurface(sid, personaEntries, injectionAssembly.PersonaText, supportRecallLimit)
	characterPrivateRecollection := buildCharacterPrivateRecollectionSurface(sid, characterPrivateMemories, injectionAssembly.CharacterPrivateText, supportRecallLimit)

	// SEQ-16-P164/P165/P167/P168 contract surfaces.
	retrievalRoleBoundary := buildRetrievalRoleBoundary(sid, storylines, worldRules, charStates, activeStates, pendingThreads, chatLogs)
	retrievalIndexIR := buildRetrievalIndexIRSupportOnly(recallResult, memories, evidence, kgTriples, chatLogs, resumePack)
	retrievalExtendAuthority := buildRetrievalExtendAuthority(retrievalRoleBoundary)
	temporalReadValidityFirst := buildTemporalReadValidityFirst(chatLogs, episodeSums, len(chatLogs))

	// SEQ-16-P172~P175 contract surfaces.
	sessionMemoryBoundary := buildSessionMemoryBoundary(sid, activeStates, pendingThreads, chatLogs, storylines, worldRules, charStates)
	bridgePromotionEntry := buildBridgePromotionEntry(sid, pendingThreads, canonicalLayers)
	sessionFirstPermanentFallbackReadRule := buildSessionFirstPermanentFallbackReadRule(sid, sessionMemoryBoundary, retrievalRoleBoundary)
	promotionWaitVisibility := buildPromotionWaitVisibility(sid, pendingThreads, canonicalLayers, chatLogs)

	// SEQ-16-P179~P182 contract surfaces (IR normalized retrieval unit schema).
	retrievalUnitsIR := buildRetrievalUnitsIR(sid, memories, evidence, kgTriples, chatLogs, resumePack)
	directEvidenceDualRepresentation := buildDirectEvidenceDualRepresentation(evidence)
	sourceTaggedRetrievalUnitSurface := buildSourceTaggedRetrievalUnitSurface(memories, evidence, kgTriples, chatLogs, resumePack)
	rawTurnSpanMetadata := buildRawTurnSpanMetadata(chatLogs, episodeSums, memories, evidence, resumePack)

	// SEQ-16-P186~P189 contract surfaces (MS Multi-Signal Retrieval Contract).
	signalMixContract := buildSignalMixContract(sid, memories, evidence, kgTriples, chatLogs, episodeSums)
	queryClassRouting := buildQueryClassRouting(sid, memories, evidence, kgTriples, chatLogs, episodeSums)
	retrievalResultInspection := buildRetrievalResultInspection(sid, memories, evidence, kgTriples, chatLogs, episodeSums, supportRecallLimit)
	sparseTailRecall := buildSparseTailRecall(sid, memories, evidence, kgTriples, chatLogs, episodeSums)

	// SEQ-16-P193~P196 contract surfaces (TM Temporal Read Surface).
	validityWindowReading := buildValidityWindowReading(sid, chatLogs, episodeSums, evidence, memories)
	truthCoexistenceRules := buildTruthCoexistenceRules(sid, evidence, memories, chatLogs)
	temporalDisambiguationContract := buildTemporalDisambiguationContract(sid, chatLogs, episodeSums, evidence, memories)
	promotionLagInvisibilitySplit := buildPromotionLagInvisibilitySplit(sid, pendingThreads, canonicalLayers, chatLogs, evidence)

	// SEQ-16-P200~P205 contract surfaces (VX verify + replay).
	sessionPermanentAuthorityReplay := buildSessionPermanentAuthorityReplay(sid, retrievalRoleBoundary)
	normalizedUnitSupportOnlyReplay := buildNormalizedUnitSupportOnlyReplay(sid, retrievalUnitsIR)
	multiSignalRetrievalInspectionReplay := buildMultiSignalRetrievalInspectionReplay(sid, signalMixContract, retrievalResultInspection)
	validityWindowTemporalReplay := buildValidityWindowTemporalReplay(sid, temporalReadValidityFirst, validityWindowReading)
	sourceTaggedAuthorityAwareAssemblyReplay := buildSourceTaggedAuthorityAwareAssemblyReplay(sid, sourceTaggedRetrievalUnitSurface, retrievalRoleBoundary)
	criticTruncationSpilloverReplay := buildCriticTruncationSpilloverReplay(sid, rawTurnSpanMetadata, sparseTailRecall, retrievalUnitsIR)

	// SEQ-16-P209~P212 contract surfaces (backend test remigration evidence).
	indexSnapshot := retrievalIndexSnapshotFromDocuments(sid, documents)
	sessionPartitionedIndex := buildSessionPartitionedIndex(sid, documents, indexSnapshot)
	indexLifecycle := buildIndexLifecycle(sid, vectorShadow)
	sourceLookupAudit := buildSourceLookupAudit(sid, evidence, memories, kgTriples, chatLogs)
	runtimeToggle := buildRuntimeToggle(sid, degraded, injectionEnabled, inputContextEnabled, maxInjectionChars, maxInputContextChars)
	inputAnchorGovernor := buildInputAnchorGovernor(rawUserInput, inputContextText, inputContextTruncated, maxInputContextChars, chatLogs, resumePack, activeStates, canonicalLayers, episodeSums, pendingThreads, storylines)
	weakInputPlanner := buildWeakInputPlannerContract(rawUserInput, inputAnchorGovernor, languageContext, maxInputContextChars)
	plannerExecutionContract := buildPlannerExecutionContract(rawUserInput, narrativeStance, guideMode, guideStrength, inputAnchorGovernor, weakInputPlanner, selectedStorylines, pendingThreads, activeStates, canonicalLayers, worldRules, injectionAssembly, languageContext)
	progressionChoiceLedger := buildProgressionChoiceLedger(sid, turnIndex, rawUserInput, chatLogs, selectedStorylines, pendingThreads, episodeSums, inputAnchorGovernor, weakInputPlanner, plannerExecutionContract, progressionLedger)
	progressionLedger["progression_choice"] = progressionChoiceLedger
	step25ValidationGate := buildStep25ValidationGate(rawUserInput, weakInputPlanner, plannerExecutionContract, progressionChoiceLedger)
	supervisorInputPack["step25_validation_gate"] = step25ValidationGate
	if guidance := formatWeakInputPlannerGuidance(weakInputPlanner); guidance != "" {
		supervisorInputPack["weak_input_planner"] = weakInputPlanner
		if existing, _ := supervisorInputPack["persistent_guidance"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["persistent_guidance"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["persistent_guidance"] = guidance
		}
		if existing, _ := supervisorInputPack["final_guidance_suffix"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["final_guidance_suffix"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["final_guidance_suffix"] = guidance
		}
	}
	if guidance := formatPlannerExecutionContractGuidance(plannerExecutionContract); guidance != "" {
		supervisorInputPack["planner_execution_contract"] = plannerExecutionContract
		if existing, _ := supervisorInputPack["persistent_guidance"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["persistent_guidance"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["persistent_guidance"] = guidance
		}
		if existing, _ := supervisorInputPack["final_guidance_suffix"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["final_guidance_suffix"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["final_guidance_suffix"] = guidance
		}
	}
	if guidance := formatProgressionChoiceGuidance(progressionChoiceLedger); guidance != "" {
		supervisorInputPack["progression_choice_ledger"] = progressionChoiceLedger
		if existing, _ := supervisorInputPack["persistent_guidance"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["persistent_guidance"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["persistent_guidance"] = guidance
		}
		if existing, _ := supervisorInputPack["final_guidance_suffix"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["final_guidance_suffix"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["final_guidance_suffix"] = guidance
		}
	}
	helperBudgetGovernorTrace := buildHelperBudgetGovernorTrace(injectionAssembly, maxInjectionChars)

	tracePreview := map[string]any{
		"source":              "go_r1_read_shadow",
		"would_call_llm":      false,
		"would_write":         false,
		"prompt_source":       promptAssembly["prompt_source"],
		"evidence_counts":     evidenceCounts,
		"section_summary":     sectionSummary,
		"supervisor_status":   supervisorInputPack["status"],
		"critic_status":       criticInputPack["status"],
		"storyline_selection": supervisorInputPack["storyline_selection"],
	}
	for k, v := range progressionLedgerTracePreviewFields(progressionLedger) {
		tracePreview[k] = v
	}
	autonomyPlan := buildAutonomyPlan(degraded, guideMode, narrativeStance)
	microBeatProposal := buildMicroBeatProposal(degraded, pendingThreads, storylines, supportRecallLimit)
	sceneStepProposal := buildSceneStepProposal(degraded, activeStates, canonicalLayers, episodeSums, supportRecallLimit)
	combinedProposal := buildCombinedProposal(degraded, microBeatProposal, sceneStepProposal)
	writebackPreview := buildWritebackPreview(degraded)
	shadowCompareRecord := buildGenerationPacketShadowCompareRecord(injectionAssembly, inputContextText)
	inputTransparencyModel := buildPrepareTurnInputTransparencyRenderModel(sid, turnIndex, rawUserInput, inputContextText, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded, fallbackReason, injectionAssembly)
	effectiveInputPreview := buildPrepareTurnEffectiveInputPreview(sid, turnIndex, rawUserInput, requestType, applyMode, inputContextText, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded, fallbackReason, injectionAssembly)
	timing.addElapsed("response_assembly", responseAssemblyStartedAt)
	backendTiming := timing.snapshot()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"source":                   "shadow",
		"chat_session_id":          sid,
		"generated_at":             time.Now().UTC().Format(time.RFC3339),
		"request_type":             requestType,
		"fallback_reason":          fallbackReason,
		"effective_user_input":     rawUserInput,
		"injection_text":           injectionOut,
		"input_context_text":       inputContextOut,
		"supervisor_input_pack":    supervisorInputPack,
		"critic_input_pack":        criticInputPack,
		"injection_pack":           injectionPack,
		"language_context":         languageContext,
		"perspective_context":      perspectiveContext,
		"input_transparency_model": inputTransparencyModel,
		"effective_input_preview":  effectiveInputPreview,
		"backend_timing":           backendTiming,
		"trace_preview":            tracePreview,
		"recall_result":            recallResult,
		"reference_recall":         referenceRecall,
		"reference_injection": map[string]any{
			"enabled":        referenceInjectionEnabled,
			"applied":        referenceInjectionText != "",
			"selected_count": len(referenceRecall.Selected),
			"injected_count": referenceInjectedCount,
			"mode":           map[bool]string{true: "live", false: "shadow"}[referenceInjectionEnabled],
		},
		"session_state":                                 sessionState,
		"narrative_control":                             narrativeControl,
		"progression_ledger":                            progressionLedger,
		"retrieval_role_boundary":                       retrievalRoleBoundary,
		"retrieval_index_ir":                            retrievalIndexIR,
		"retrieval_extend_authority":                    retrievalExtendAuthority,
		"temporal_read_validity_first":                  temporalReadValidityFirst,
		"session_memory_boundary":                       sessionMemoryBoundary,
		"bridge_promotion_entry":                        bridgePromotionEntry,
		"session_first_permanent_fallback_read_rule":    sessionFirstPermanentFallbackReadRule,
		"promotion_wait_visibility":                     promotionWaitVisibility,
		"retrieval_units_ir":                            retrievalUnitsIR,
		"direct_evidence_dual_representation":           directEvidenceDualRepresentation,
		"source_tagged_retrieval_unit_surface":          sourceTaggedRetrievalUnitSurface,
		"raw_turn_span_metadata":                        rawTurnSpanMetadata,
		"signal_mix_contract":                           signalMixContract,
		"query_class_routing":                           queryClassRouting,
		"retrieval_result_inspection":                   retrievalResultInspection,
		"sparse_tail_recall":                            sparseTailRecall,
		"validity_window_reading":                       validityWindowReading,
		"truth_coexistence_rules":                       truthCoexistenceRules,
		"temporal_disambiguation_contract":              temporalDisambiguationContract,
		"promotion_lag_invisibility_split":              promotionLagInvisibilitySplit,
		"session_permanent_authority_replay":            sessionPermanentAuthorityReplay,
		"normalized_unit_support_only_replay":           normalizedUnitSupportOnlyReplay,
		"multi_signal_retrieval_inspection_replay":      multiSignalRetrievalInspectionReplay,
		"validity_window_temporal_replay":               validityWindowTemporalReplay,
		"source_tagged_authority_aware_assembly_replay": sourceTaggedAuthorityAwareAssemblyReplay,
		"critic_truncation_spillover_replay":            criticTruncationSpilloverReplay,
		"session_partitioned_index":                     sessionPartitionedIndex,
		"index_lifecycle":                               indexLifecycle,
		"source_lookup_audit":                           sourceLookupAudit,
		"runtime_toggle":                                runtimeToggle,
		"input_anchor_governor":                         inputAnchorGovernor,
		"weak_input_planner":                            weakInputPlanner,
		"planner_execution_contract":                    plannerExecutionContract,
		"progression_choice_ledger":                     progressionChoiceLedger,
		"step25_validation_gate":                        step25ValidationGate,
		"helper_budget_governor_trace":                  helperBudgetGovernorTrace,
		"helper_injection_budget_manager":               buildStep165HelperInjectionBudgetManager(maxInjectionChars, injectionAssembly),
		"input_context_slot_governor":                   buildStep165InputContextSlotGovernor(maxInputContextChars, inputContextTruncated),
		"transparency_preview_runtime_trace_extend":     buildStep165TransparencyPreviewRuntimeTraceExtend(inputContextText, inputContextTruncated, injectionAssembly),
		"handoff_anchor_metadata_alignment":             buildStep165HandoffAnchorMetadataAlignment(inputContextText, inputAnchorGovernor),
		"stale_arc_guard_carry_in_hooks":                buildStep165StaleArcGuardCarryInHooks(inputAnchorGovernor, helperBudgetGovernorTrace),
		"decision_adaptive_floor_ceiling":               buildStep165DecisionAdaptiveFloorCeiling(),
		"decision_max_slot":                             buildStep165DecisionMaxSlot(),
		"decision_runtime_token_hint":                   buildStep165DecisionRuntimeTokenHint(),
		"decision_saga_chapter_anchor_ladder":           buildStep165DecisionSagaChapterAnchorLadder(),
		"decision_explicit_user_input_specificity":      buildStep165DecisionExplicitUserInputSpecificity(),
		"step_16_8_baseline_compare":                    buildStep165Step168BaselineCompare(inputAnchorGovernor),
		"step_16_8_reason_visibility_guard_lane":        buildStep165Step168ReasonVisibilityGuardLane(),
		"step_17_direct_handoff_gate":                   buildStep165Step17DirectHandoffGate(),
		"step_17_evaluation_harness_baseline":           buildStep165Step17EvaluationHarnessBaseline(),
		"step_17_ops_trace_interpretation":              buildStep165Step17OpsTraceInterpretation(),
		"step_17_inspection_surface":                    buildStep165Step17InspectionSurface(),
		"stale_arc_ceiling":                             buildStep168StaleArcCeiling(inputAnchorGovernor),
		"scene_alignment":                               buildStep168SceneAlignment(rawUserInput, inputAnchorGovernor),
		"current_scene_evidence_min_criteria":           buildStep168CurrentSceneEvidenceMinCriteria(activeStates, evidence, chatLogs),
		"pending_threads_guard":                         buildStep168PendingThreadsGuard(pendingThreads),
		"reason_trace":                                  buildStep168ReasonTrace(inputAnchorGovernor),
		"failure_split":                                 buildStep168FailureSplit(inputAnchorGovernor),
		"packet_synthesis":                              buildStep168PacketSynthesis(storylines, pendingThreads),
		"callback_bias_ceiling":                         buildStep168CallbackBiasCeiling(storylines),
		"callback_scene_alignment":                      buildStep168CallbackSceneAlignment(storylines, activeStates),
		"stale_callback_suppression":                    buildStep168StaleCallbackSuppression(storylines),
		"old_arc_foreground_visibility":                 buildStep168OldArcForegroundVisibility(inputAnchorGovernor),
		"reason_code_vocabulary":                        buildStep168ReasonCodeVocabulary(),
		"preview_audit_transparency":                    buildStep168PreviewAuditTransparency(inputAnchorGovernor),
		"foreground_hijack_taxonomy":                    buildStep168ForegroundHijackTaxonomy(inputAnchorGovernor),
		"delayed_payoff_split":                          buildStep168DelayedPayoffSplit(storylines, episodeSums),
		"recall_gain_monopoly_split":                    buildStep168RecallGainMonopolySplit(inputAnchorGovernor),
		"stale_arc_revival_replay":                      buildStep168StaleArcRevivalReplay(inputAnchorGovernor),
		"tail_recall_hijack_gate":                       buildStep168TailRecallHijackGate(inputAnchorGovernor),
		"narrative_diversity_gate":                      buildStep168NarrativeDiversityGate(storylines, worldRules),
		"arc_monopoly_gate":                             buildStep168ArcMonopolyGate(inputAnchorGovernor),
		"js_continuity_rescue":                          buildStep168JSContinuityRescue(storylines, pendingThreads),
		"js_prompt_assembly_guard":                      buildStep168JSPromptAssemblyGuard(injectionAssembly),
		"js_trace_preview_transparency":                 buildStep168JSTracePreviewTransparency(inputContextText, injectionAssembly),
		"replay_corpus_baseline":                        buildStep168ReplayCorpusBaseline(inputAnchorGovernor),
		"backend_metadata_alignment":                    buildStep168BackendMetadataAlignment(storylines, pendingThreads),
		"evaluation_split":                              buildStep17EvaluationSplit(recallResult, 0.75),
		"ops_procedure_surface":                         buildStep17OpsProcedureSurface(),
		"inspection_lane_boundary":                      buildStep17InspectionLaneBoundary(),
		"adoption_gate":                                 buildStep17AdoptionGate(false),
		"release_hygiene":                               buildStep17ReleaseHygiene(),
		"retrieval_completeness_metric":                 buildStep17RetrievalCompletenessMetric(recallResult),
		"final_answer_quality_metric":                   buildStep17FinalAnswerQualityMetric(0.75),
		"failure_split_replay":                          buildStep17FailureSplitReplay(recallResult, 0.75),
		"regression_corpus":                             buildStep17RegressionCorpus(),
		"freshness_lag_metric":                          buildStep17FreshnessLagMetric(120, 80, 200),
		"promotion_backfill_rebuild":                    buildStep17PromotionBackfillRebuild(),
		"reembed_migration_health_probe":                buildStep17ReembedMigrationHealthProbe(),
		"failure_fallback_rollback":                     buildStep17FailureFallbackRollback(),
		"async_critic_delay":                            buildStep17AsyncCriticDelay(),
		"partial_write_retry":                           buildStep17PartialWriteRetry(),
		"explain_surface":                               buildStep17ExplainSurface(),
		"preview_audit_surface":                         buildStep17PreviewAuditSurface(),
		"dashboard_lane":                                buildStep17DashboardLane(),
		"display_guard":                                 buildStep17DisplayGuard(),
		"visibility_lane":                               buildStep17VisibilityLane(),
		"step_14_adoption_gate":                         buildStep17Step14AdoptionGate(),
		"step_15_adoption_gate":                         buildStep17Step15AdoptionGate(),
		"step_16_adoption_gate":                         buildStep17Step16AdoptionGate(),
		"bundle_regenerate_checklist":                   buildStep17BundleRegenerateChecklist(),
		"packaged_bundle_checklist":                     buildStep17PackagedBundleChecklist(),
		"freshness_silent_drop_gate":                    buildStep17FreshnessSilentDropGate(),
		"bundle_generation_evidence":                    buildStep17BundleGenerationEvidence(),
		"regression_corpus_green":                       buildStep17RegressionCorpusGreen(),
		"evaluation_split_smoke_check":                  buildStep17EvaluationSplitSmokeCheck(),
		"ops_dry_run_checklist_pass":                    buildStep17OpsDryRunChecklistPass(),
		"inspection_lane_boundary_review":               buildStep17InspectionLaneBoundaryReview(),
		"release_gate_complete":                         buildStep17ReleaseGateComplete(),
		"reaudit_backend_admin_owner":                   buildStep17ReauditBackendAdminOwner(),
		"reaudit_ops_doc_dry_run":                       buildStep17ReauditOpsDocDryRun(),
		"reaudit_root_runtime_read_only":                buildStep17ReauditRootRuntimeReadOnly(),
		"reaudit_release_gate_operator_evidence":        buildStep17ReauditReleaseGateOperatorEvidence(),
		"reaudit_admin_mutation_control_ui":             buildStep17ReauditAdminMutationControlUI(),
		"reaudit_release_execution_ui":                  buildStep17ReauditReleaseExecutionUI(),
		"reaudit_beta_0_8_closure_bundle":               buildStep17ReauditBeta08ClosureBundle(),
		"decision_completeness_metric_unit":             buildStep17DecisionCompletenessMetricUnit(),
		"decision_regression_corpus_mix":                buildStep17DecisionRegressionCorpusMix(),
		"decision_inspection_lane_default":              buildStep17DecisionInspectionLaneDefault(),
		"decision_adoption_gate_review_mode":            buildStep17DecisionAdoptionGateReviewMode(),
		"decision_bundle_regenerate_split":              buildStep17DecisionBundleRegenerateSplit(),
		"chroma_migration_preflight":                    buildStep17ChromaMigrationPreflight(),
		"chroma_shadow_bootstrap":                       buildStep17ChromaShadowBootstrap(),
		"chroma_backfill_dry_run":                       buildStep17ChromaBackfillDryRun(),
		"chroma_bulk_backfill":                          buildStep17ChromaBulkBackfill(),
		"chroma_reembed_discipline":                     buildStep17ChromaReembedDiscipline(),
		"chroma_divergence_health_probe":                buildStep17ChromaDivergenceHealthProbe(),
		"chroma_degraded_fallback_runbook":              buildStep17ChromaDegradedFallbackRunbook(),
		"chroma_rebuild_rollback_drill":                 buildStep17ChromaRebuildRollbackDrill(),
		"chroma_adoption_gate":                          buildStep17ChromaAdoptionGate(),
		"chroma_release_hygiene":                        buildStep17ChromaReleaseHygiene(),
		"chroma_migration_visibility_guard":             buildStep17ChromaMigrationVisibilityGuard(),
		"reset_admin":                                   buildResetAdmin(),
		"historical_content_preserved":                  buildHistoricalContentPreserved(),
		"reset_note_only":                               buildResetNoteOnly(),
		"step17_closure_gate":                           buildStep17ClosureGate(),
		"context_files_reviewed":                        buildContextFilesReviewed(),
		"prep_anchor_vrhy":                              buildPrepAnchorVRHY(),
		"historical_reference_only":                     buildHistoricalReferenceOnly(),
		"backend_prep_anchor":                           buildBackendPrepAnchor(),
		"routing_contract_prep_anchor":                  buildRoutingContractPrepAnchor(),
		"runtime_prep_scope":                            buildRuntimePrepScope(),
		"vr_scoped_verbatim_support_text":               buildVRScopedVerbatimSupportText(injectionAssembly.ScopedVerbatimSupport),
		"vr_policy_owner_block":                         buildVRPolicyOwnerBlock(),
		"vr_prompt_injection_strategy":                  buildVRPromptInjectionStrategy(),
		"vr_hierarchy_escape_hatch":                     buildVRHierarchyEscapeHatch(),
		"vr_backend_test_guard":                         buildVRBackendTestGuard(),
		"vr_runtime_transparency":                       buildVRRuntimeTransparency(),
		"vr_regression_bundle_green":                    buildVRRegressionBundleGreen(),
		"hy_semantic_rank_score":                        buildHYSemanticRankScore(),
		"hy_soft_bias":                                  buildHYSoftBias(),
		"hy_stopword_guard":                             buildHYStopwordGuard(),
		"hy_q1a_propagation":                            buildHYQ1aPropagation(),
		"hy_runtime_inspection":                         buildHYRuntimeInspection(),
		"hy_recurring_risk_guards":                      buildHYRecurringRiskGuards(),
		"hy_policy_registry":                            buildHYPolicyRegistry(),
		"hy_stop_at_18_2c":                              buildHYStopAt18_2c(),
		"hy_tail_budget_policy_owner":                   buildHYTailBudgetPolicyOwner(),
		"hy_tail_budget_rescue_pass":                    buildHYTailBudgetRescuePass(),
		"hy_tail_budget_rescue_trace":                   buildHYTailBudgetRescueTrace(),
		"hy_tail_budget_q1a_propagation":                buildHYTailBudgetQ1aPropagation(),
		"hy_tail_budget_regression":                     buildHYTailBudgetRegression(),
		"qr_query_class_contract":                       buildQRQueryClassContract(),
		"qr_query_class_taxonomy":                       buildQRQueryClassTaxonomy(),
		"qr_primary_class_selection":                    buildQRPrimaryClassSelection(),
		"qr_lexical_cue_block":                          buildQRLexicalCueBlock(),
		"qr_query_class_contract_test":                  buildQRQueryClassContractTest(),
		"qr_query_class_budget_policy":                  buildQRQueryClassBudgetPolicy(),
		"qr_q3c_budget_reuse":                           buildQRQ3cBudgetReuse(),
		"qr_temporal_profile_budget":                    buildQRTemporalProfileBudget(),
		"qr_budget_visibility":                          buildQRBudgetVisibility(),
		"qr_query_class_budget_test":                    buildQRQueryClassBudgetTest(),
		"qr_note_policy":                                buildQRNotePolicy(),
		"qr_scene_canon_no_pre_extract":                 buildQRSceneCanonNoPreExtract(),
		"qr_callback_resume_temporal_note_only":         buildQRCallbackResumeTemporalNoteOnly(),
		"qr_note_policy_fields":                         buildQRNotePolicyFields(),
		"qr_note_policy_test":                           buildQRNotePolicyTest(),
		"qr_route_policy":                               buildQRRoutePolicy(),
		"qr_route_families":                             buildQRRouteFamilies(),
		"qr_long_tail_route_candidates":                 buildQRLongTailRouteCandidates(),
		"qr_route_policy_fields":                        buildQRRoutePolicyFields(),
		"qr_route_policy_test":                          buildQRRoutePolicyTest(),
		"vx_hybrid_replay_gate":                         buildVXHybridReplayGate(),
		"vx_replay_threshold_reuse":                     buildVXReplayThresholdReuse(),
		"vx_hybrid_replay_states":                       buildVXHybridReplayStates(),
		"vx_hybrid_replay_test":                         buildVXHybridReplayTest(),
		"vx_heldout_completeness_gate":                  buildVXHeldoutCompletenessGate(),
		"vx_heldout_metrics":                            buildVXHeldoutMetrics(),
		"vx_heldout_threshold_reuse":                    buildVXHeldoutThresholdReuse(),
		"vx_heldout_completeness_test":                  buildVXHeldoutCompletenessTest(),
		"vx_latency_token_budget_gate":                  buildVXLatencyTokenBudgetGate(),
		"vx_latency_token_metrics":                      buildVXLatencyTokenMetrics(),
		"vx_latency_token_threshold_reuse":              buildVXLatencyTokenThresholdReuse(),
		"vx_latency_token_test":                         buildVXLatencyTokenTest(),
		"vx_truth_boundary_gate":                        buildVXTruthBoundaryGate(),
		"vx_truth_boundary_precedence":                  buildVXTruthBoundaryPrecedence(),
		"vx_truth_boundary_states":                      buildVXTruthBoundaryStates(),
		"vx_truth_boundary_test":                        buildVXTruthBoundaryTest(),
		"vx_truncation_summary_loss_gate":               buildVXTruncationSummaryLossGate(),
		"vx_truncation_summary_loss_metrics":            buildVXTruncationSummaryLossMetrics(),
		"vx_truncation_summary_loss_threshold_reuse":    buildVXTruncationSummaryLossThresholdReuse(),
		"vx_truncation_summary_loss_states":             buildVXTruncationSummaryLossStates(),
		"vx_truncation_summary_loss_test":               buildVXTruncationSummaryLossTest(),
		"post_chroma_top1_scoped_verbatim":              buildPostChromaTop1ScopedVerbatim(),
		"post_chroma_top2_hybrid_scoring":               buildPostChromaTop2HybridScoring(),
		"post_chroma_top3_temporal_relation":            buildPostChromaTop3TemporalRelation(),
		"post_chroma_top4_temporal_validity":            buildPostChromaTop4TemporalValidity(),
		"post_chroma_top5_entity_graph":                 buildPostChromaTop5EntityGraph(),
		"post_chroma_top6_selective_rerank":             buildPostChromaTop6SelectiveRerank(),
		"vr_raw_preserving_support":                     buildVRRawPreservingSupport(),
		"vr_hybrid_realism":                             buildVRHybridRealism(),
		"vr_soft_routing":                               buildVRSoftRouting(),
		"vr_latency_discipline":                         buildVRLatencyDiscipline(),
		"vr_truth_boundary_preserve":                    buildVRTruthBoundaryPreserve(),
		"vr_18_1a_raw_transcript":                       buildVR18_1aRawTranscript(),
		"vr_18_1b_source_tag":                           buildVR18_1bSourceTag(),
		"vr_18_1c_prompt_injection":                     buildVR18_1cPromptInjection(),
		"vr_18_1d_hierarchy_escape":                     buildVR18_1dHierarchyEscape(),
		"hy_18_2a_semantic_keyword":                     buildHY18_2aSemanticKeyword(),
		"hy_18_2b_soft_bias":                            buildHY18_2bSoftBias(),
		"hy_18_2c_score_inspection":                     buildHY18_2cScoreInspection(),
		"hy_18_2d_adaptive_top_k":                       buildHY18_2dAdaptiveTopK(),
		"qr_18_3a_query_class":                          buildQR18_3aQueryClass(),
		"qr_18_3b_retrieval_depth":                      buildQR18_3bRetrievalDepth(),
		"qr_18_3c_extract_before_read":                  buildQR18_3cExtractBeforeRead(),
		"qr_18_3d_long_tail_route":                      buildQR18_3dLongTailRoute(),
		"vx_18_4a_semantic_hybrid_replay":               buildVX18_4aSemanticHybridReplay(),
		"vx_18_4b_held_out_recall":                      buildVX18_4bHeldOutRecall(),
		"vx_18_4c_latency_token":                        buildVX18_4cLatencyToken(),
		"vx_18_4d_truth_boundary_replay":                buildVX18_4dTruthBoundaryReplay(),
		"vx_18_4e_top_k_truncation":                     buildVX18_4eTopKTruncation(),
		"pre_release_version_marker":                    buildPreReleaseVersionMarker(),
		"pre_release_bundle_authority":                  buildPreReleaseBundleAuthority(),
		"pre_release_artifact":                          buildPreReleaseArtifact(),
		"pre_release_vr_smoke":                          buildPreReleaseVRSmoke(),
		"pre_release_hy_smoke":                          buildPreReleaseHYSmoke(),
		"pre_release_qr_smoke":                          buildPreReleaseQRSmoke(),
		"pre_release_vx_review":                         buildPreReleaseVXReview(),
		"pre_release_raw_snippet":                       buildPreReleaseRawSnippet(),
		"pre_release_hybrid_bias":                       buildPreReleaseHybridBias(),
		"pre_release_query_class_rule":                  buildPreReleaseQueryClassRule(),
		"pre_release_retrieval_note":                    buildPreReleaseRetrievalNote(),
		"reset_admin_185":                               buildResetAdmin185(),
		"historical_content_preserved_185":              buildHistoricalContentPreserved185(),
		"reset_note_only_185":                           buildResetNoteOnly185(),
		"bounded_live_scope":                            buildBoundedLiveScope(),
		"sqlite_truth_preserve":                         buildSQLiteTruthPreserve(),
		"fail_open_safety":                              buildFailOpenSafety(),
		"operator_visibility":                           buildOperatorVisibility(),
		"silent_authority_drift_guard":                  buildSilentAuthorityDriftGuard(),
		"release_honesty":                               buildReleaseHonesty(),
		"live_chroma_toggle_config":                     buildLiveChromaToggleConfig(),
		"live_scope_memory_only":                        buildLiveScopeMemoryOnly(),
		"live_chroma_topk_cap":                          buildLiveChromaTopkCap(),
		"shadow_disabled_degrade_rule":                  buildShadowDisabledDegradeRule(),
		"chroma_identity_sqlite_hydration":              buildChromaIdentitySQLiteHydration(),
		"chroma_sqlite_dedupe_merge":                    buildChromaSQLiteDedupeMerge(),
		"canonical_precedence_formatting":               buildCanonicalPrecedenceFormatting(),
		"chroma_miss_fallback_preserve":                 buildChromaMissFallbackPreserve(),
		"operator_inspection_surface":                   buildOperatorInspectionSurface(),
		"live_limited_mode_toggle":                      buildLiveLimitedModeToggle(),
		"health_adoption_prerequisite":                  buildHealthAdoptionPrerequisite(),
		"narrow_rollout_rule":                           buildNarrowRolloutRule(),
		"chroma_enabled_smoke_check":                    buildChromaEnabledSmokeCheck(),
		"degraded_fail_open_replay":                     buildDegradedFailOpenReplay(),
		"sqlite_baseline_parity_replay":                 buildSQLiteBaselineParityReplay(),
		"truth_boundary_source_order_replay":            buildTruthBoundarySourceOrderReplay(),
		"release_note_honesty_checklist":                buildReleaseNoteHonestyChecklist(),
		// SEQ-18.5-P201~P205 release gate surfaces (dry-run evidence only; no actual artifact created)
		"bundle_release_gate_201":                    buildBundleReleaseGate201(),
		"limited_live_chroma_smoke_check_202":        buildLimitedLiveChromaSmokeCheck202(),
		"sqlite_fail_open_replay_pass_203":           buildSQLiteFailOpenReplayPass203(),
		"operator_visibility_fallback_checklist_204": buildOperatorVisibilityFallbackChecklist204(),
		"release_note_bundle_notes_complete_205":     buildReleaseNoteBundleNotesComplete205(),
		// SEQ-18.5-P209~P212 decision surfaces (operator-gated, dry-run only)
		"first_live_scope_decision_209":               buildFirstLiveScopeDecision209(),
		"chroma_candidate_merge_replace_decision_210": buildChromaCandidateMergeReplaceDecision210(),
		"degraded_threshold_decision_211":             buildDegradedThresholdDecision211(),
		"operator_visibility_scope_decision_212":      buildOperatorVisibilityScopeDecision212(),
		// SEQ-19-P9~P11 reset administration surfaces
		"reset_admin_19":                  buildResetAdmin19(),
		"historical_content_preserved_19": buildHistoricalContentPreserved19(),
		"reset_note_only_19":              buildResetNoteOnly19(),
		// SEQ-19-P15~P22 temporal state surfaces
		"temporal_state":                   buildTemporalState19(activeStates, chatLogs, canonicalLayers),
		"current_story_clock_resolution":   buildCurrentStoryClockResolution(activeStates),
		"precision_label_contract":         buildPrecisionLabelContract(),
		"invalid_unknown_degradation":      buildInvalidUnknownDegradation(),
		"temporal_split_rule":              buildTemporalSplitRule(),
		"story_clock_surface_guard":        buildStoryClockSurfaceGuard(),
		"step18_plus_19_regression_bundle": buildStep18Plus19RegressionBundle(),
		// SEQ-19-P30~P42 temporal relation ledger schema surfaces
		"temporal_relation_ledger_canonical": buildTemporalRelationLedgerCanonical(),
		"schema_phrase_ingress":              buildSchemaPhraseIngress(),
		"schema_owner_block":                 buildSchemaOwnerBlock(),
		"canonical_data_override_guard":      buildCanonicalDataOverrideGuard(),
		"locale_pack_split":                  buildLocalePackSplit(),
		"multilingual_deictic_parity":        buildMultilingualDeicticParity(),
		"active_locales_gating":              buildActiveLocalesGating(),
		"snake_case_camel_case_inspect":      buildSnakeCaseCamelCaseInspect(),
		"valid_from_to_turn_range":           buildValidFromToTurnRange(),
		"missing_anchor_degradation":         buildMissingAnchorDegradation(),
		"temporal_relation_ledger_complete":  buildTemporalRelationLedgerComplete(),
		// SEQ-19-P50~P57 elapsed-time normalization surfaces
		"sc19_elapsed_policy_owner":           buildElapsedPolicyOwner(),
		"elapsed_time_decision_extended":      buildElapsedTimeDecisionExtended(currentStoryClock19, temporalRelationLedger19),
		"clock_write_directive_extended":      buildClockWriteDirectiveExtended(currentStoryClock19, temporalRelationLedger19),
		"temporal_support_packet":             temporalSupportPacket,
		"temporal_write_discipline":           buildTemporalWriteDiscipline(),
		"elapsed_policy_compactness":          buildElapsedPolicyCompactness(),
		"temporal_guard_bundle":               buildTemporalGuardBundle(),
		"step18_plus_19_regression_bundle_57": buildStep18Plus19RegressionBundle57(),
		// SEQ-19-P66~P69 locale pack + replay surfaces
		"week_unit_support":                   buildWeekUnitSupport(),
		"temporal_replay_cases":               buildTemporalReplayCases(),
		"bounded_week_month_write_guard":      buildBoundedWeekMonthWriteGuard(),
		"step18_plus_19_regression_bundle_69": buildStep18Plus19RegressionBundle69(),
		// SEQ-19-P78~P81 mixed-lane VX replay surfaces
		"mixed_lane_precedence_contract":      buildMixedLanePrecedenceContract(),
		"mixed_lane_replay_cases":             buildMixedLaneReplayCases(),
		"mixed_lane_split_rule_outcome":       buildMixedLaneSplitRuleOutcome(),
		"step18_plus_19_regression_bundle_81": buildStep18Plus19RegressionBundle81(),
		// SEQ-19-P90~P93 degrade replay / VX coverage surfaces
		"missing_anchor_degrade_contract":       buildMissingAnchorDegradeContract(),
		"missing_anchor_exact_phrase_degrade":   buildMissingAnchorExactPhraseDegrade(),
		"low_precision_recalled_relation_guard": buildLowPrecisionRecalledRelationGuard(),
		"step18_plus_19_regression_bundle_93":   buildStep18Plus19RegressionBundle93(),
		// SEQ-19-P102~P105 temporal packet truth-boundary / precedence surfaces
		"temporal_packet_truth_boundary_contract": buildTemporalPacketTruthBoundaryContract(),
		"temporal_packet_mixed_precedence":        buildTemporalPacketMixedPrecedence(),
		"temporal_packet_clock_missing_boundary":  buildTemporalPacketClockMissingBoundary(),
		"step18_plus_19_regression_bundle_105":    buildStep18Plus19RegressionBundle105(),
		// SEQ-19-P114~P117 response-time validator helper cluster / trace-only surfaces
		"step19_validator_helper_cluster_contract":    buildStep19ValidatorHelperClusterContract(),
		"temporal_precedence_resolution_order":        buildTemporalPrecedenceResolutionOrder(),
		"temporal_deictic_warning_classes":            buildTemporalDeicticWarningClasses(),
		"temporal_deictic_trace_only_warning_surface": buildTemporalDeicticTraceOnlyWarningSurface(),
		// SEQ-19-P125~P128 classification / write-discipline surfaces
		"temporal_classification_write_discipline_surface": buildTemporalClassificationWriteDisciplineSurface(),
		"temporal_classification_exceptions":               buildTemporalClassificationExceptions(),
		"temporal_write_discipline_rules":                  buildTemporalWriteDisciplineRules(),
		"temporal_relation_entry_metadata_surface":         buildTemporalRelationEntryMetadataSurface(),
		// SEQ-19-P137~P139 locale-aware extraction / multilingual parity surfaces
		"locale_aware_extractor_owner_block":        buildLocaleAwareExtractorOwnerBlock(),
		"recalled_past_parity_surface":              buildRecalledPastParitySurface(),
		"current_scene_next_morning_parity_surface": buildCurrentSceneNextMorningParitySurface(),
		// SEQ-19-P140 activeLocales fail-open gating
		"active_locales_fail_open_gating_contract": buildActiveLocalesFailOpenGatingContract(),
		// SEQ-19-P288~P292 finish-line criteria surfaces
		"current_time_explicitness_contract": buildCurrentTimeExplicitnessContract(),
		"anchor_bound_relation_contract":     buildAnchorBoundRelationContract(),
		"bounded_ambiguity_contract":         buildBoundedAmbiguityContract(),
		"advance_discipline_contract":        buildAdvanceDisciplineContract(),
		"truth_boundary_preserve_contract":   buildTruthBoundaryPreserveContract(),
		// SEQ-19-P296~P299 sub-step 19-1 schema definition surfaces
		"current_story_clock_schema_define":               buildCurrentStoryClockSchemaDefine(),
		"session_state_timeline_anchor_precedence_define": buildSessionStateTimelineAnchorPrecedenceDefine(),
		"precision_label_define":                          buildPrecisionLabelDefine(),
		"current_scene_recalled_past_split_define":        buildCurrentSceneRecalledPastSplitDefine(),
		// SEQ-19-P303~P307 sub-step 19-2 schema definition surfaces
		"temporal_relation_schema_define":       buildTemporalRelationSchemaDefine(),
		"phrase_ingress_normalization_define":   buildPhraseIngressNormalizationDefine(),
		"temporal_relation_surface_define":      buildTemporalRelationSurfaceDefine(),
		"anchor_ambiguity_carry_forward_define": buildAnchorAmbiguityCarryForwardDefine(),
		"locale_parser_pack_boundary_define":    buildLocaleParserPackBoundaryDefine(),
		// SEQ-19-P311~P314 sub-step 19-3 schema definition surfaces
		"advance_trigger_define":               buildAdvanceTriggerDefine(),
		"scene_transition_define":              buildSceneTransitionDefine(),
		"elapsed_time_write_discipline_define": buildElapsedTimeWriteDisciplineDefine(),
		"temporal_support_packet_define":       buildTemporalSupportPacketDefine(),
		// SEQ-19-P318~P322 sub-step 19-4 VX replay surfaces
		"temporal_replay_define_19_4a":                            buildTemporalReplayDefine19_4a(),
		"current_scene_recalled_past_conflict_replay_define":      buildCurrentSceneRecalledPastConflictReplayDefine(),
		"missing_anchor_low_precision_degrade_replay_define":      buildMissingAnchorLowPrecisionDegradeReplayDefine(),
		"temporal_packet_truth_boundary_precedence_replay_define": buildTemporalPacketTruthBoundaryPrecedenceReplayDefine(),
		"response_time_deictic_validator_replay_define":           buildResponseTimeDeicticValidatorReplayDefine(),
		// SEQ-19-P323~P324 sub-step 19-4f/19-4g classification + multilingual replay surfaces
		"figurative_duration_planned_future_recalled_past_classification_replay_define": buildFigurativeDurationPlannedFutureRecalledPastClassificationReplayDefine(),
		"multilingual_parity_mixed_language_fail_open_replay_define":                    buildMultilingualParityMixedLanguageFailOpenReplayDefine(),
		// SEQ-19-P328~P332 Beta 1.0 release gate surfaces
		"beta_1_0_bundle_latest_root_runtime_define":   buildBeta10BundleLatestRootRuntimeDefine(),
		"story_clock_smoke_check_pass":                 buildStoryClockSmokeCheckPass(),
		"relative_time_normalization_smoke_check_pass": buildRelativeTimeNormalizationSmokeCheckPass(),
		"elapsed_time_advance_replay_pass":             buildElapsedTimeAdvanceReplayPass(),
		"ambiguity_precedence_review_checklist_pass":   buildAmbiguityPrecedenceReviewChecklistPass(),
		// SEQ-19-P333, P337~P344 Beta 1.0 release gate + decision surfaces
		"multilingual_temporal_parity_smoke_check_pass":                                 buildMultilingualTemporalParitySmokeCheckPass(),
		"current_story_clock_absolute_datetime_bounded_story_day":                       buildCurrentStoryClockAbsoluteDatetimeBoundedStoryDay(),
		"relative_time_normalization_numeric_offset_vocabulary_first":                   buildRelativeTimeNormalizationNumericOffsetVocabularyFirst(),
		"elapsed_time_advance_conservative_manual_scene_classifier":                     buildElapsedTimeAdvanceConservativeManualSceneClassifier(),
		"missing_anchor_degrade":                                                        buildMissingAnchorDegrade(),
		"locale_parsing_single_detector_active_locales_merge":                           buildLocaleParsingSingleDetectorActiveLocalesMerge(),
		"ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover":                  buildKoEnBootstrapExtractorLocalePackParserReplaceCutover(),
		"unspecified_time_fallback_no_advance_carry_forward_discipline":                 buildUnspecifiedTimeFallbackNoAdvanceCarryForwardDiscipline(),
		"relation_only_future_past_reference_current_scene_advance_evidence_gate_split": buildRelationOnlyFuturePastReferenceCurrentSceneAdvanceEvidenceGateSplit(),
		// SEQ-20 Preparatory reset/admin surfaces (P9 ~ P11)
		"seq20_reset_admin_note":             buildSeq20ResetAdminNote(),
		"seq20_historical_content_preserved": buildSeq20HistoricalContentPreserved(),
		"seq20_reset_note_only":              buildSeq20ResetNoteOnly(),
		// SEQ-20 q20a temporal query expansion surfaces (P21 ~ P28)
		"q20a_temporal_query_expansion_preparatory": buildQ20aTemporalQueryExpansionPreparatory(),
		"q20a_v1_temporal_query_expansion":          buildQ20aV1TemporalQueryExpansion(),
		"q20a_rule_surface_focus_range":             buildQ20aRuleSurfaceFocusRange(),
		"q20a_derives_from_sc19_relation_schema":    buildQ20aDerivesFromSc19RelationSchema(),
		"q20a_mirrored_at_recall_intent":            buildQ20aMirroredAtRecallIntent(),
		"q20a_current_clock_overlay_cue_pack":       buildQ20aCurrentClockOverlayCuePack(),
		"q20a_qr1a_lexical_routing_normalized":      buildQ20aQr1aLexicalRoutingNormalized(),
		"q20a_contract_only_groundwork":             buildQ20aContractOnlyGroundwork(),
		// SEQ-20 q20b temporal validity read policy surfaces (P36 ~ P40)
		"q20b_temporal_validity_read_policy_preparatory": buildQ20bTemporalValidityReadPolicyPreparatory(),
		"q20b_v1_temporal_validity_read_policy":          buildQ20bV1TemporalValidityReadPolicy(),
		"q20b_read_priority_modes":                       buildQ20bReadPriorityModes(),
		"q20b_mirrored_at_recall_intent_and_query_class": buildQ20bMirroredAtRecallIntentAndQueryClass(),
		"q20b_stops_before_later_tv_work":                buildQ20bStopsBeforeLaterTVWork(),
		// SEQ-20 q20c temporal event invalidation support surfaces (P47 ~ P51)
		"q20c_temporal_event_invalidation_preparatory": buildQ20cTemporalEventInvalidationPreparatory(),
		"q20c_v1_temporal_event_invalidation_support":  buildQ20cV1TemporalEventInvalidationSupport(),
		"q20c_invalidation_modes":                      buildQ20cInvalidationModes(),
		"q20c_mirrored_at_recall_intent":               buildQ20cMirroredAtRecallIntent(),
		"q20c_separate_from_promotion_lag":             buildQ20cSeparateFromPromotionLag(),
		// SEQ-20 q20d temporal promotion-lag support surfaces (P57 ~ P60)
		"q20d_temporal_promotion_lag_preparatory": buildQ20dTemporalPromotionLagPreparatory(),
		"q20d_v1_temporal_promotion_lag_support":  buildQ20dV1TemporalPromotionLagSupport(),
		"q20d_anchor_precedence":                  buildQ20dAnchorPrecedence(),
		"q20d_mirrored_at_recall_intent":          buildQ20dMirroredAtRecallIntent(),
		// SEQ-20 q20e temporal hot recall buffer surfaces (P66 ~ P69)
		"q20e_temporal_hot_recall_buffer_preparatory": buildQ20eTemporalHotRecallBufferPreparatory(),
		"q20e_v1_temporal_hot_recall_buffer":          buildQ20eV1TemporalHotRecallBuffer(),
		"q20e_bridge_source_set":                      buildQ20eBridgeSourceSet(),
		"q20e_mirrored_at_recall_intent":              buildQ20eMirroredAtRecallIntent(),
		// SEQ-20 q20f lightweight entity index surfaces (P76 ~ P81)
		"q20f_lightweight_entity_index_preparatory": buildQ20fLightweightEntityIndexPreparatory(),
		"q20f_v1_lightweight_entity_index":          buildQ20fV1LightweightEntityIndex(),
		"q20f_structured_state_surfaces":            buildQ20fStructuredStateSurfaces(),
		"q20f_mirrored_at_query_class":              buildQ20fMirroredAtQueryClass(),
		"q20f_stops_before_graph_like_support":      buildQ20fStopsBeforeGraphLikeSupport(),
		"q20f_token_boundary_structured_labels":     buildQ20fTokenBoundaryStructuredLabels(),
		// SEQ-20 q20g graph-like support signal surfaces (P89 ~ P93)
		"q20g_graph_like_support_signal_preparatory": buildQ20gGraphLikeSupportSignalPreparatory(),
		"q20g_v1_graph_like_support_signal":          buildQ20gV1GraphLikeSupportSignal(),
		"q20g_pair_sources_and_fail_open":            buildQ20gPairSourcesAndFailOpen(),
		"q20g_mirrored_at_query_class":               buildQ20gMirroredAtQueryClass(),
		"q20g_stops_before_inspection_formatting":    buildQ20gStopsBeforeInspectionFormatting(),
		// SEQ-20 q20h entity/graph boost inspection surface surfaces (P99 ~ P102)
		"q20h_entity_graph_boost_inspection_surface_preparatory": buildQ20hEntityGraphBoostInspectionSurfacePreparatory(),
		"q20h_v1_entity_graph_boost_inspection_surface":          buildQ20hV1EntityGraphBoostInspectionSurface(),
		"q20h_inspection_role_and_authority_notice":              buildQ20hInspectionRoleAndAuthorityNotice(),
		"q20h_mirrored_at_query_class":                           buildQ20hMirroredAtQueryClass(),
		// SEQ-20 q20i lagging current state boost surfaces (P109 ~ P112)
		"q20i_lagging_current_state_boost_preparatory": buildQ20iLaggingCurrentStateBoostPreparatory(),
		"q20i_v1_lagging_current_state_boost":          buildQ20iV1LaggingCurrentStateBoost(),
		"q20i_activation_and_precedence":               buildQ20iActivationAndPrecedence(),
		"q20i_mirrored_at_query_class":                 buildQ20iMirroredAtQueryClass(),
		// SEQ-20 q20j motive-shadow hint surfaces (P118 ~ P121)
		"q20j_motive_shadow_hint_preparatory": buildQ20jMotiveShadowHintPreparatory(),
		"q20j_v1_motive_shadow_hint":          buildQ20jV1MotiveShadowHint(),
		"q20j_truth_write_forbidden":          buildQ20jTruthWriteForbidden(),
		"q20j_mirrored_at_query_class":        buildQ20jMirroredAtQueryClass(),
		// SEQ-20 q20k motive-shadow non-escalation guard surfaces (P127 ~ P129)
		"q20k_motive_shadow_non_escalation_guard_preparatory": buildQ20kMotiveShadowNonEscalationGuardPreparatory(),
		"q20k_v1_motive_shadow_non_escalation_guard":          buildQ20kV1MotiveShadowNonEscalationGuard(),
		"q20k_mirrored_at_query_class":                        buildQ20kMirroredAtQueryClass(),
		// SEQ-20 q20l relation edge support ledger surfaces (P135 ~ P138)
		"q20l_relation_edge_support_ledger_preparatory": buildQ20lRelationEdgeSupportLedgerPreparatory(),
		"q20l_v1_relation_edge_support_ledger":          buildQ20lV1RelationEdgeSupportLedger(),
		"q20l_graph_truth_write_forbidden":              buildQ20lGraphTruthWriteForbidden(),
		"q20l_mirrored_at_query_class":                  buildQ20lMirroredAtQueryClass(),
		// SEQ-20 aggregate summary surfaces (P231 ~ P236)
		"seq20_validity_priority":         buildSeq20P231ValidityPriority(),
		"seq20_support_only_accelerator":  buildSeq20P232SupportOnlyAccelerator(),
		"seq20_ambiguity_reduction":       buildSeq20P233AmbiguityReduction(),
		"seq20_inspection_visibility":     buildSeq20P234InspectionVisibility(),
		"seq20_truth_precedence_preserve": buildSeq20P235TruthPrecedencePreserve(),
		"seq20_hot_bridge":                buildSeq20P236HotBridge(),
		// SEQ-20 q20m temporal ambiguity support note surfaces (P258 ~ P259)
		"q20m_temporal_ambiguity_support_note_preparatory": buildQ20mTemporalAmbiguitySupportNotePreparatory(),
		"q20m_v1_temporal_ambiguity_support_note":          buildQ20mV1TemporalAmbiguitySupportNote(),
		// SEQ-20 q20n alias/entity conflict disambiguation surfaces (P260 ~ P261)
		"q20n_alias_entity_conflict_disambiguation_preparatory": buildQ20nAliasEntityConflictDisambiguationPreparatory(),
		"q20n_v1_alias_entity_conflict_disambiguation":          buildQ20nV1AliasEntityConflictDisambiguation(),
		// SEQ-20 q20o temporal/entity support block source-tag rule surfaces (P262 ~ P263)
		"q20o_temporal_entity_source_tag_rule_preparatory": buildQ20oTemporalEntitySourceTagRulePreparatory(),
		"q20o_v1_temporal_entity_source_tag_rule":          buildQ20oV1TemporalEntitySourceTagRule(),
		// SEQ-20 q20p canonical-pending/stale-current conflict note surfaces (P264 ~ P265)
		"q20p_canonical_pending_stale_current_conflict_note_preparatory": buildQ20pCanonicalPendingStaleCurrentConflictNotePreparatory(),
		"q20p_v1_canonical_pending_stale_current_conflict_note":          buildQ20pV1CanonicalPendingStaleCurrentConflictNote(),
		// SEQ-20 q20q recall cue rescue rule surfaces (P266 ~ P267)
		"q20q_recall_cue_rescue_rule_preparatory": buildQ20qRecallCueRescueRulePreparatory(),
		"q20q_v1_recall_cue_rescue_rule":          buildQ20qV1RecallCueRescueRule(),
		// SEQ-20 q20r wide gather -> validity join rule surfaces (P268 ~ P269)
		"q20r_wide_gather_validity_join_rule_preparatory": buildQ20rWideGatherValidityJoinRulePreparatory(),
		"q20r_v1_wide_gather_validity_join_rule":          buildQ20rV1WideGatherValidityJoinRule(),
		// SEQ-20 q20s thin support tag fallback surfaces (P270 ~ P271)
		"q20s_thin_support_tag_fallback_preparatory": buildQ20sThinSupportTagFallbackPreparatory(),
		"q20s_v1_thin_support_tag_fallback":          buildQ20sV1ThinSupportTagFallback(),
		// SEQ-20 vx20a~vx20g validation replay gates (P286 ~ P299)
		"vx20a_temporal_validity_replay_gate":              buildVx20aTemporalValidityReplayGate(),
		"vx20b_entity_boost_false_positive_gate":           buildVx20bEntityBoostFalsePositiveGate(),
		"vx20c_graph_accelerator_degrade_gate":             buildVx20cGraphAcceleratorDegradeGate(),
		"vx20d_canonical_precedence_replay_gate":           buildVx20dCanonicalPrecedenceReplayGate(),
		"vx20e_promotion_blocked_freshness_replay_gate":    buildVx20ePromotionBlockedFreshnessReplayGate(),
		"vx20f_recall_cue_rescue_replay_gate":              buildVx20fRecallCueRescueReplayGate(),
		"vx20g_hot_buffer_wide_gather_non_regression_gate": buildVx20gHotBufferWideGatherNonRegressionGate(),
		// SEQ-20 Beta 1.1 release smoke gate surfaces (P312 ~ P316)
		"seq20_beta11_bundle_dry_run":                 buildSeq20P312Beta11BundleDryRun(),
		"seq20_temporal_validity_recall_smoke":        buildSeq20P313TemporalValidityRecallSmoke(),
		"seq20_entity_graph_accelerator_smoke":        buildSeq20P314EntityGraphAcceleratorSmoke(),
		"seq20_temporal_entity_disambiguation_smoke":  buildSeq20P315TemporalEntityDisambiguationSmoke(),
		"seq20_precedence_ambiguity_review_checklist": buildSeq20P316PrecedenceAmbiguityReviewChecklist(),
		// SEQ-20 final preserve summary surfaces (P330 ~ P333)
		"seq20_temporal_query_expansion_preserve": buildSeq20P330TemporalQueryExpansionPreserve(),
		"seq20_entity_index_preserve":             buildSeq20P331EntityIndexPreserve(),
		"seq20_graph_accelerator_preserve":        buildSeq20P332GraphAcceleratorPreserve(),
		"seq20_ambiguity_support_note_preserve":   buildSeq20P333AmbiguitySupportNotePreserve(),
		// SEQ-21 surfaces ??Beta 1.2 selective rerank + retrieval economics (P9 ~ P202)
		"seq21_reset_admin_note":                buildSeq21ResetAdminNote(),
		"seq21_historical_content_preserved":    buildSeq21HistoricalContentPreserved(),
		"seq21_reset_note_only":                 buildSeq21ResetNoteOnly(),
		"seq21_rerank_class_summary":            buildSeq21P181RerankClassSummary(),
		"seq21_budget_config_summary":           buildSeq21P182BudgetConfigSummary(),
		"seq21_failure_class_split_summary":     buildSeq21P183FailureClassSplitSummary(),
		"seq21_held_out_hygiene_summary":        buildSeq21P184HeldOutHygieneSummary(),
		"seq21_truth_boundary_preserve_summary": buildSeq21P185TruthBoundaryPreserveSummary(),
		"seq21_density_discipline_summary":      buildSeq21P186DensityDisciplineSummary(),
		"seq21_rerank_trigger_class":            buildSeq21P190RerankTriggerClass(),
		"seq21_rerank_support_only_schema":      buildSeq21P191RerankSupportOnlySchema(),
		"seq21_rerank_off_fallback":             buildSeq21P192RerankOffFallback(),
		"seq21_rerank_near_miss_trigger":        buildSeq21P193RerankNearMissTrigger(),
		"seq21_query_class_candidate_cap":       buildSeq21P197QueryClassCandidateCap(),
		"seq21_latency_budget_degrade":          buildSeq21P198LatencyBudgetDegrade(),
		"seq21_retrieval_cache_reuse":           buildSeq21P199RetrievalCacheReuse(),
		"seq21_failure_class_adaptive_cap":      buildSeq21P200FailureClassAdaptiveCap(),
		"seq21_dual_density_delivery_budget":    buildSeq21P201DualDensityDeliveryBudget(),
		"seq21_heavy_promotion_rule":            buildSeq21P202HeavyPromotionRule(),
		// SEQ-21 21-3 failure-class tuning loop surfaces (P206 ~ P209)
		"seq21_failure_taxonomy":           buildSeq21P206FailureTaxonomy(),
		"seq21_dev_split_tuning_loop":      buildSeq21P207DevSplitTuningLoop(),
		"seq21_held_out_confirmation_gate": buildSeq21P208HeldOutConfirmationGate(),
		"seq21_residual_long_tail_loop":    buildSeq21P209ResidualLongTailLoop(),
		// SEQ-21 21-4 validation/adoption gate surfaces (P213 ~ P219)
		"seq21_cost_vs_gain_replay":                    buildSeq21P213CostVsGainReplay(),
		"seq21_latency_token_envelope_replay":          buildSeq21P214LatencyTokenEnvelopeReplay(),
		"seq21_held_out_regression_gate":               buildSeq21P215HeldOutRegressionGate(),
		"seq21_post_chroma_default_promotion_criteria": buildSeq21P216PostChromaDefaultPromotionCriteria(),
		"seq21_cost_normalized_tail_recall_gate":       buildSeq21P217CostNormalizedTailRecallGate(),
		"seq21_density_mix_replay":                     buildSeq21P218DensityMixReplay(),
		"seq21_shared_runner_corpus_rule":              buildSeq21P219SharedRunnerCorpusRule(),
		// SEQ-21 Beta 1.2 release gate surfaces (P223 ~ P227)
		"seq21_beta12_bundle_dry_run":                 buildSeq21P223Beta12BundleDryRun(),
		"seq21_selective_rerank_trigger_smoke":        buildSeq21P224SelectiveRerankTriggerSmoke(),
		"seq21_candidate_budget_latency_smoke":        buildSeq21P225CandidateBudgetLatencySmoke(),
		"seq21_failure_class_tuning_review_checklist": buildSeq21P226FailureClassTuningReviewChecklist(),
		"seq21_held_out_cost_adoption_gate_complete":  buildSeq21P227HeldOutCostAdoptionGateComplete(),
		// SEQ-21 final preserve decision surfaces (P238 ~ P241)
		"seq21_bounded_trigger_classes_preserve":   buildSeq21P238BoundedTriggerClassesPreserve(),
		"seq21_query_class_candidate_cap_preserve": buildSeq21P239QueryClassCandidateCapPreserve(),
		"seq21_latency_degrade_path_preserve":      buildSeq21P240LatencyDegradePathPreserve(),
		"seq21_tuning_deferred_preserve":           buildSeq21P241TuningDeferredPreserve(),
		// SEQ-21.5 surfaces ??Backend structural closeout evidence (P416 ~ P432)
		"seq215_authority_frozen":           buildSeq215P416AuthorityFrozen(),
		"seq215_stale_history_rejected":     buildSeq215P417StaleHistoryRejected(),
		"seq215_turn_contracts_moved":       buildSeq215P418TurnContractsMoved(),
		"seq215_m3a_formatting_moved":       buildSeq215P419M3aFormattingMoved(),
		"seq215_proxy_config_moved":         buildSeq215P420ProxyConfigMoved(),
		"seq215_maintenance_queue_moved":    buildSeq215P421MaintenanceQueueMoved(),
		"seq215_chroma_c17_moved":           buildSeq215P422ChromaC17Moved(),
		"seq215_step17_helpers_extracted":   buildSeq215P423Step17HelpersExtracted(),
		"seq215_lc1_phase_a_moved":          buildSeq215P424LC1PhaseAMoved(),
		"seq215_lc1_phase_bcd_moved":        buildSeq215P425LC1PhaseBCDMoved(),
		"seq215_utility_services_moved":     buildSeq215P426UtilityServicesMoved(),
		"seq215_physical_baseline_recorded": buildSeq215P427PhysicalBaselineRecorded(),
		"seq215_wi14_removed":               buildSeq215P431WI14Removed(),
		"seq215_wi14_deletion_records":      buildSeq215P432WI14DeletionRecords(),
		// SEQ-21.5 core extraction / deferral / validation evidence (P436 ~ P448)
		"seq215_run_maintenance_pass_blocked": buildSeq215P436RunMaintenancePassBlocked(),
		"seq215_complete_turn_m4_extracted":   buildSeq215P437CompleteTurnM4Extracted(),
		"seq215_prepare_turn_extracted":       buildSeq215P438PrepareTurnExtracted(),
		"seq215_bundle_supervisor_reduced":    buildSeq215P439BundleSupervisorReduced(),
		"seq215_bundle_recall_reduced":        buildSeq215P440BundleRecallReduced(),
		"seq215_bundle_injection_reduced":     buildSeq215P441BundleInjectionReduced(),
		"seq215_lc1_remaining_moved":          buildSeq215P442LC1RemainingMoved(),
		"seq215_narrative_read_lock":          buildSeq215P443NarrativeReadLock(),
		"seq215_hypamemory_extracted":         buildSeq215P444HypamemoryExtracted(),
		"seq215_archive_center_js_deferral":   buildSeq215P445ArchiveCenterJSDeferral(),
		"seq215_or1e_rechecked":               buildSeq215P446OR1eRechecked(),
		"seq215_final_validation":             buildSeq215P447FinalValidation(),
		"seq215_step_complete":                buildSeq215P448StepComplete(),
		// SEQ-21.5 WI14 deletion slice evidence (P476 ~ P488)
		"seq215_authority_restate":                        buildSeq215P476AuthorityRestate(),
		"seq215_before_count":                             buildSeq215P477BeforeCount(),
		"seq215_exact_usage_search":                       buildSeq215P478ExactUsageSearch(),
		"seq215_delete_minimal_continuation_cues":         buildSeq215P479DeleteMinimalContinuationCues(),
		"seq215_delete_explicit_correction_markers":       buildSeq215P480DeleteExplicitCorrectionMarkers(),
		"seq215_delete_detect_input_mode":                 buildSeq215P481DeleteDetectInputMode(),
		"seq215_remove_build_weak_input_steering":         buildSeq215P482RemoveBuildWeakInputSteering(),
		"seq215_simplify_supervisor_planner":              buildSeq215P483SimplifySupervisorPlanner(),
		"seq215_keep_auto_advance_explicit":               buildSeq215P484KeepAutoAdvanceExplicit(),
		"seq215_py_compile_pass":                          buildSeq215P485PyCompilePass(),
		"seq215_focused_backend_tests":                    buildSeq215P486FocusedBackendTests(),
		"seq215_js_untouched":                             buildSeq215P487JSUntouched(),
		"seq215_after_count":                              buildSeq215P488AfterCount(),
		"seq215_js_authority":                             buildSeq215P556JSAuthority(),
		"seq215_backend_authority":                        buildSeq215P557BackendAuthority(),
		"seq215_no_root_standalone_pair":                  buildSeq215P558NoRootStandalonePair(),
		"seq215_backup_not_authority":                     buildSeq215P559BackupNotAuthority(),
		"seq215_deploy_not_authority":                     buildSeq215P560DeployNotAuthority(),
		"seq215_no_broad_split_before_narrow":             buildSeq215P561NoBroadSplitBeforeNarrow(),
		"seq215_stale_split_rejected_context":             buildSeq215P562StaleSplitRejectedContext(),
		"seq215_stale_split_rejected_progress":            buildSeq215P563StaleSplitRejectedProgress(),
		"seq215_beta08_metrics":                           buildSeq215P564Beta08Metrics(),
		"seq215_restate_guard":                            buildSeq215P565RestateGuard(),
		"seq215_promote_guard":                            buildSeq215P566PromoteGuard(),
		"seq215_turn_contracts_created":                   buildSeq215P589TurnContractsCreated(),
		"seq215_complete_turn_request_moved":              buildSeq215P590CompleteTurnRequestMoved(),
		"seq215_m4_complete_turn_request_moved":           buildSeq215P591M4CompleteTurnRequestMoved(),
		"seq215_m4_complete_turn_response_moved":          buildSeq215P592M4CompleteTurnResponseMoved(),
		"seq215_prepare_turn_settings_moved":              buildSeq215P593PrepareTurnSettingsMoved(),
		"seq215_prepare_turn_request_moved":               buildSeq215P594PrepareTurnRequestMoved(),
		"seq215_retrieval_document_q1a_moved":             buildSeq215P595RetrievalDocumentQ1AMoved(),
		"seq215_generation_packet_moved":                  buildSeq215P596GenerationPacketMoved(),
		"seq215_prepare_turn_response_moved":              buildSeq215P597PrepareTurnResponseMoved(),
		"seq215_moved_classes_imported_back":              buildSeq215P598MovedClassesImportedBack(),
		"seq215_route_decorators_stay":                    buildSeq215P599RouteDecoratorsStay(),
		"seq215_prepare_turn_stays":                       buildSeq215P600PrepareTurnStays(),
		"seq215_complete_turn_m4_stays":                   buildSeq215P601CompleteTurnM4Stays(),
		"seq215_public_route_paths_unchanged":             buildSeq215P602PublicRoutePathsUnchanged(),
		"seq215_response_fields_unchanged":                buildSeq215P603ResponseFieldsUnchanged(),
		"seq215_no_broad_tree_created":                    buildSeq215P604NoBroadTreeCreated(),
		"seq215_py_compile_turn_contracts":                buildSeq215P605PyCompileTurnContracts(),
		"seq215_focused_import_check":                     buildSeq215P606FocusedImportCheck(),
		"seq215_validation_record":                        buildSeq215P607ValidationRecord(),
		"seq215_phase1_validation_passed":                 buildSeq215P663Phase1ValidationPassed(),
		"seq215_prepare_turn_assembly_created":            buildSeq215P664PrepareTurnAssemblyCreated(),
		"seq215_format_memory_text_moved":                 buildSeq215P665FormatMemoryTextMoved(),
		"seq215_format_kg_text_moved":                     buildSeq215P666FormatKGTextMoved(),
		"seq215_format_episode_text_moved":                buildSeq215P667FormatEpisodeTextMoved(),
		"seq215_format_chapter_text_moved":                buildSeq215P668FormatChapterTextMoved(),
		"seq215_format_fallback_text_moved":               buildSeq215P669FormatFallbackTextMoved(),
		"seq215_clean_short_moved":                        buildSeq215P670CleanShortMoved(),
		"seq215_json_load_maybe_moved":                    buildSeq215P671JsonLoadMaybeMoved(),
		"seq215_predicate_matches_moved":                  buildSeq215P672PredicateMatchesMoved(),
		"seq215_world_rule_note_moved":                    buildSeq215P673WorldRuleNoteMoved(),
		"seq215_format_entity_digest_text_moved":          buildSeq215P674FormatEntityDigestTextMoved(),
		"seq215_format_entity_anchor_text_moved":          buildSeq215P675FormatEntityAnchorTextMoved(),
		"seq215_db_session_helpers_stay":                  buildSeq215P677DBSessionHelpersStay(),
		"seq215_core_logic_stay":                          buildSeq215P678CoreLogicStay(),
		"seq215_injection_pack_fields_unchanged":          buildSeq215P679InjectionPackFieldsUnchanged(),
		"seq215_py_compile_prepare_turn_assembly":         buildSeq215P680PyCompilePrepareTurnAssembly(),
		"seq215_focused_backend_tests_m3a":                buildSeq215P681FocusedBackendTestsM3a(),
		"seq215_m3a_validation_record":                    buildSeq215P682M3aValidationRecord(),
		"seq215_proxy_plugin_main_model_separated":        buildSeq215P730ProxyPluginMainModelSeparated(),
		"seq215_provider_ownership_split":                 buildSeq215P731ProviderOwnershipSplit(),
		"seq215_thin_proxy_route":                         buildSeq215P732ThinProxyRoute(),
		"seq215_config_service_split":                     buildSeq215P733ConfigServiceSplit(),
		"seq215_thin_config_route":                        buildSeq215P734ThinConfigRoute(),
		"seq215_routes_explicitly_wired":                  buildSeq215P735RoutesExplicitlyWired(),
		"seq215_public_paths_preserved":                   buildSeq215P736PublicPathsPreserved(),
		"seq215_compatibility_wrapper":                    buildSeq215P737CompatibilityWrapper(),
		"seq215_route_level_tests":                        buildSeq215P738RouteLevelTests(),
		"seq215_js_route_usage":                           buildSeq215P739JSRouteUsage(),
		"seq215_monolith_not_applicable":                  buildSeq215P740MonolithNotApplicable(),
		"seq215_prepare_turn_bundle_normal_use":           buildSeq215P776PrepareTurnBundleNormalUse(),
		"seq215_js_payload_mutation_owner":                buildSeq215P777JSPayloadMutationOwner(),
		"seq215_js_injection_budget_owner":                buildSeq215P778JSInjectionBudgetOwner(),
		"seq215_js_input_context_slotting_owner":          buildSeq215P779JSInputContextSlottingOwner(),
		"seq215_js_protection_blocks_owner":               buildSeq215P780JSProtectionBlocksOwner(),
		"seq215_js_hook_ui_integration_owner":             buildSeq215P781JSHookUIIntegrationOwner(),
		"seq215_js_offline_fail_open_owner":               buildSeq215P782JSOfflineFailOpenOwner(),
		"seq215_build_input_context_preserved":            buildSeq215P783BuildInputContextPreserved(),
		"seq215_assemble_injection_with_budget_preserved": buildSeq215P784AssembleInjectionWithBudgetPreserved(),
		"seq215_apply_context_injection_preserved":        buildSeq215P785ApplyContextInjectionPreserved(),
		"seq215_try_prepare_turn_takeover_off":            buildSeq215P786TryPrepareTurnTakeoverOff(),
		"seq215_js_node_check":                            buildSeq215P787JSNodeCheck(),
		"seq215_js_focused_contract_tests":                buildSeq215P788JSFocusedContractTests(),
		"seq215_p789_validation_record":                   buildSeq215P789ValidationRecord(),
		"seq215_runtime_split_status":                     buildSeq215P830RuntimeSplitStatus(),
		"seq215_backend_bundle_assisted":                  buildSeq215P831BackendBundleAssisted(),
		"seq215_plugin_only_modules":                      buildSeq215P832PluginOnlyModules(),
		"seq215_or1e_wording":                             buildSeq215P833OR1eWording(),
		"seq215_or1e_node_check":                          buildSeq215P834OR1eNodeCheck(),
		"seq215_validation_record_p835":                   buildSeq215P835ValidationRecord(),
		"seq215_phase1_complete":                          buildSeq215P869Phase1Complete(),
		"seq215_phase2_complete":                          buildSeq215P870Phase2Complete(),
		"seq215_phase3_complete":                          buildSeq215P871Phase3Complete(),
		"seq215_phase4_complete":                          buildSeq215P872Phase4Complete(),
		"seq215_context_readback":                         buildSeq215P873ContextReadback(),
		"seq215_progress_readback":                        buildSeq215P874ProgressReadback(),
		"seq215_stale_authority_search":                   buildSeq215P875StaleAuthoritySearch(),
		"seq215_false_backend_tree_search":                buildSeq215P876FalseBackendTreeSearch(),
		"seq215_no_backup_deploy_edited":                  buildSeq215P877NoBackupDeployEdited(),
		"seq215_changed_files_list":                       buildSeq215P878ChangedFilesList(),
		"seq215_validation_commands":                      buildSeq215P879ValidationCommands(),
		"seq215_additional_owner_split_bounded":           buildSeq215P880AdditionalOwnerSplitBounded(),
		"seq215_js_backend_offload_plugin_only":           buildSeq215P881JSBackendOffloadPluginOnly(),
		"seq215_master_checklist_open_zero":               buildSeq215P882MasterChecklistOpenZero(),
		"seq215_step_complete_p883":                       buildSeq215P883StepComplete(),
		"autonomy_plan":                                   autonomyPlan,
		"micro_beat_proposal":                             microBeatProposal,
		"scene_step_proposal":                             sceneStepProposal,
		"combined_proposal":                               combinedProposal,
		"writeback_preview":                               writebackPreview,
		"continuity_pack":                                 continuityPack,
		"persona_recollection":                            personaRecollection,
		"character_private_recollection":                  characterPrivateRecollection,
		"entity_recollection_relevance":                   recollectionRelevance,
		"generation_packet": map[string]any{
			"packet_mode":     packetMode,
			"degraded":        degraded,
			"fallback_reason": fallbackReason,
			"injection_text":  injectionOut,
			"prompt_assembly": promptAssembly,
			"trace_summary": map[string]any{
				"reads_ok":                      readsOK,
				"read_errors":                   len(readErrs),
				"memory_count":                  len(memories),
				"kg_count":                      len(kgTriples),
				"evidence_count":                len(evidence),
				"chat_log_count":                len(chatLogs),
				"resume_pack_present":           resumePack != nil,
				"storyline_count":               len(storylines),
				"storyline_selected_count":      len(storylineSelection.Selected),
				"storyline_dropped_count":       len(storylineSelection.Dropped),
				"storyline_stale_dropped_count": storylineSelectionSummary(storylineSelection)["stale_dropped_count"],
				"world_rule_count":              len(worldRules),
				"character_state_count":         len(charStates),
				"pending_thread_count":          len(pendingThreads),
				"active_state_count":            len(activeStates),
				"canonical_layer_count":         len(canonicalLayers),
				"narrative_current_state_count": len(narrativeCurrentValues),
				"episode_summary_count":         len(episodeSums),
				"max_injection_chars":           maxInjectionChars,
				"max_input_context_chars":       maxInputContextChars,
				"injection_truncated":           injectionTruncated,
				"input_context_truncated":       inputContextTruncated,
				"would_call_llm":                false,
				"would_write":                   false,
				"prompt_files_found":            promptAssembly["files_found"],
				"scoped_verbatim_support_count": injectionAssembly.ScopedVerbatimSupport.Count,
				"verbatim_support":              injectionAssembly.ScopedVerbatimSupport,
				"chapter_delivered":             strings.TrimSpace(injectionAssembly.ChapterText) != "",
				"chapter_text_chars":            len([]rune(strings.TrimSpace(injectionAssembly.ChapterText))),
				"chapter_consumed":              strings.TrimSpace(injectionAssembly.ChapterText) != "" && strings.Contains(strings.ToLower(injectionAssembly.Text), strings.ToLower(strings.TrimSpace(injectionAssembly.ChapterText))),
				"saga_delivered":                strings.TrimSpace(injectionAssembly.SagaText) != "",
				"saga_text_chars":               len([]rune(strings.TrimSpace(injectionAssembly.SagaText))),
				"saga_consumed":                 strings.TrimSpace(injectionAssembly.SagaText) != "" && strings.Contains(strings.ToLower(injectionAssembly.Text), strings.ToLower(strings.TrimSpace(injectionAssembly.SagaText))),
				"arc_delivered":                 strings.TrimSpace(injectionAssembly.ArcText) != "",
				"arc_text_chars":                len([]rune(strings.TrimSpace(injectionAssembly.ArcText))),
				"arc_consumed":                  strings.TrimSpace(injectionAssembly.ArcText) != "" && strings.Contains(strings.ToLower(injectionAssembly.Text), strings.ToLower(strings.TrimSpace(injectionAssembly.ArcText))),
				"hierarchy_escalation":          injectionAssembly.Counts["hierarchy_escalation"],
				"runtime_token_profile": map[string]any{
					"version":                "p61a.v1",
					"profile_source":         "client_meta_shadow",
					"context_window_profile": profile,
					"auto_optimized":         profile != "default",
					"status":                 "shadow_only",
				},
				"outbound_rewrite_guard": map[string]any{
					"version":         "p34a.v1",
					"status":          "ready",
					"rewrite_allowed": false,
					"reason":          "prepare_turn_read_only_assembly",
					"mode":            "shadow",
					"payload_mutated": false,
				},
			},
			"shadow_compare_record": shadowCompareRecord,
		},
		"note": "prepare-turn is a store-backed shadow assembly; no writes performed",
	})
}

func (s *Server) handleEffectiveInputs(w http.ResponseWriter, r *http.Request) {
	var req dto.SaveEffectiveInputRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := strings.TrimSpace(*req.ChatSessionID)
	if sid == "" {
		sid = "default"
	}

	// Store save boundary: active only when the configured mode allows writes.
	saveOK := false
	saveErr := "shadow_mode: save disabled in R0/R1"
	effectiveInputSaved := 0
	auditSaved := 0
	storeWriteAttempted := 0
	storeWriteErrors := 0

	now := time.Now().UTC()
	text := strings.TrimSpace(*req.EffectiveInput)
	writeSource := s.storeWriteSource()

	if s.usesShadowWriteStore() && text != "" {
		ctx := r.Context()
		storeWriteAttempted++
		if err := s.Store.SaveEffectiveInput(ctx, &store.EffectiveInput{
			ChatSessionID:  sid,
			TurnIndex:      req.TurnIndex,
			EffectiveInput: text,
			CreatedAt:      now,
		}); err != nil {
			storeWriteErrors++
		} else {
			effectiveInputSaved++
			saveOK = true
		}

		if saveOK {
			storeWriteAttempted++
			if err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
				ChatSessionID: sid,
				EventType:     "effective_input_saved",
				TargetType:    "turn",
				TargetID:      int64(req.TurnIndex),
				Summary:       fmt.Sprintf("effective input saved turn %d", req.TurnIndex),
				DetailsJSON:   fmt.Sprintf(`{"turn_index":%d,"length":%d}`, req.TurnIndex, len(text)),
				Source:        writeSource,
				CreatedAt:     now,
			}); err != nil {
				storeWriteErrors++
			} else {
				auditSaved = 1
			}
		}

		if storeWriteAttempted > 0 && storeWriteErrors == 0 {
			saveOK = true
			saveErr = ""
		}
	}

	note := "effective-inputs is a shadow skeleton; no live DB mutation performed"
	if s.usesShadowWriteStore() {
		if saveOK {
			note = "effective-inputs saved in " + writeSource + " mode"
		} else {
			note = "effective-inputs write attempted in " + writeSource + " mode but failed"
		}
	}

	inputTransparency := buildInputTransparency(sid, req.TurnIndex, text, s.usesShadowWriteStore(), writeSource)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"source":                writeSource,
		"turn_index":            req.TurnIndex,
		"chat_session_id":       sid,
		"id":                    nil,
		"save_ok":               saveOK,
		"save_error":            saveErr,
		"effective_input_saved": effectiveInputSaved,
		"audit_saved":           auditSaved,
		"store_write_attempted": storeWriteAttempted,
		"store_write_errors":    storeWriteErrors,
		"input_transparency":    inputTransparency,
		"trace_handoff": map[string]any{
			"shadow_mode": true,
			"store_mode":  string(s.Cfg.StoreMode),
		},
		"note": note,
	})
}
