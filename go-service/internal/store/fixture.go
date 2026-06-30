package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// fixtureStore is a read-only R1 evidence store backed by sqlite-export NDJSON.
// It is not a product authority mode; writes remain disabled.
type fixtureStore struct {
	chatLogs             []ChatLog
	effectiveInputs      []EffectiveInput
	memories             []Memory
	evidence             []DirectEvidence
	kgTriples            []KGTriple
	auditLogs            []AuditLog
	criticFeedback       []CriticFeedback
	characterEvents      []CharacterEvent
	storylines           []Storyline
	worldRules           []WorldRule
	sessionActiveScopes  []SessionActiveScope
	characterStates      []CharacterState
	pendingThreads       []PendingThread
	activeStates         []ActiveState
	canonicalStateLayers []CanonicalStateLayer
	episodeSummaries     []EpisodeSummary
	guidancePlanStates   []GuidancePlanState
}

// NewFixtureStoreFromExportDir loads a read-only Store from sqlite-export NDJSON.
func NewFixtureStoreFromExportDir(exportDir string) (Store, error) {
	if strings.TrimSpace(exportDir) == "" {
		return nil, fmt.Errorf("fixture store requires export dir")
	}
	info, err := os.Stat(exportDir)
	if err != nil {
		return nil, fmt.Errorf("fixture store stat export dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fixture store export dir is not a directory")
	}

	fs := &fixtureStore{}
	loaders := map[string]func(map[string]any){
		"active_states":           fs.loadActiveState,
		"audit_logs":              fs.loadAuditLog,
		"canonical_state_layers":  fs.loadCanonicalStateLayer,
		"character_events":        fs.loadCharacterEvent,
		"character_states":        fs.loadCharacterState,
		"chat_logs":               fs.loadChatLog,
		"critic_feedback":         fs.loadCriticFeedback,
		"direct_evidence_records": fs.loadDirectEvidence,
		"effective_input_logs":    fs.loadEffectiveInput,
		"episode_summaries":       fs.loadEpisodeSummary,
		"guidance_plan_states":    fs.loadGuidancePlanState,
		"kg_triples":              fs.loadKGTriple,
		"memories":                fs.loadMemory,
		"pending_threads":         fs.loadPendingThread,
		"session_active_scopes":   fs.loadSessionActiveScope,
		"storylines":              fs.loadStoryline,
		"world_rules":             fs.loadWorldRule,
	}
	for table, load := range loaders {
		if err := loadNDJSONTable(filepath.Join(exportDir, table+".ndjson"), load); err != nil {
			return nil, err
		}
	}
	fs.sortRows()
	return fs, nil
}

func loadNDJSONTable(path string, load func(map[string]any)) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("fixture store open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return fmt.Errorf("fixture store parse %s: %w", filepath.Base(path), err)
		}
		if _, isMeta := row["_export_meta"]; isMeta {
			continue
		}
		delete(row, "_row_checksum")
		load(row)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("fixture store scan %s: %w", filepath.Base(path), err)
	}
	return nil
}

func (f *fixtureStore) sortRows() {
	sort.Slice(f.chatLogs, func(i, j int) bool {
		if f.chatLogs[i].ChatSessionID == f.chatLogs[j].ChatSessionID {
			if f.chatLogs[i].TurnIndex == f.chatLogs[j].TurnIndex {
				return f.chatLogs[i].ID < f.chatLogs[j].ID
			}
			return f.chatLogs[i].TurnIndex < f.chatLogs[j].TurnIndex
		}
		return f.chatLogs[i].ChatSessionID < f.chatLogs[j].ChatSessionID
	})
	sort.Slice(f.memories, func(i, j int) bool {
		if f.memories[i].ChatSessionID == f.memories[j].ChatSessionID {
			if f.memories[i].TurnIndex == f.memories[j].TurnIndex {
				return f.memories[i].ID < f.memories[j].ID
			}
			return f.memories[i].TurnIndex < f.memories[j].TurnIndex
		}
		return f.memories[i].ChatSessionID < f.memories[j].ChatSessionID
	})
	sort.Slice(f.kgTriples, func(i, j int) bool { return f.kgTriples[i].ID < f.kgTriples[j].ID })
	sort.Slice(f.evidence, func(i, j int) bool { return f.evidence[i].ID < f.evidence[j].ID })
	sort.Slice(f.auditLogs, func(i, j int) bool { return f.auditLogs[i].ID > f.auditLogs[j].ID })
	sort.Slice(f.storylines, func(i, j int) bool { return f.storylines[i].ID < f.storylines[j].ID })
	sort.Slice(f.worldRules, func(i, j int) bool { return f.worldRules[i].ID < f.worldRules[j].ID })
	sort.Slice(f.characterStates, func(i, j int) bool {
		if f.characterStates[i].TurnIndex == f.characterStates[j].TurnIndex {
			return f.characterStates[i].ID > f.characterStates[j].ID
		}
		return f.characterStates[i].TurnIndex > f.characterStates[j].TurnIndex
	})
	sort.Slice(f.pendingThreads, func(i, j int) bool {
		if f.pendingThreads[i].Pinned != f.pendingThreads[j].Pinned {
			return f.pendingThreads[i].Pinned
		}
		if f.pendingThreads[i].SourceTurn == f.pendingThreads[j].SourceTurn {
			return f.pendingThreads[i].ID > f.pendingThreads[j].ID
		}
		return f.pendingThreads[i].SourceTurn > f.pendingThreads[j].SourceTurn
	})
	sort.Slice(f.activeStates, func(i, j int) bool { return f.activeStates[i].ID < f.activeStates[j].ID })
	sort.Slice(f.canonicalStateLayers, func(i, j int) bool { return f.canonicalStateLayers[i].ID < f.canonicalStateLayers[j].ID })
	sort.Slice(f.episodeSummaries, func(i, j int) bool {
		if f.episodeSummaries[i].ToTurn == f.episodeSummaries[j].ToTurn {
			return f.episodeSummaries[i].ID > f.episodeSummaries[j].ID
		}
		return f.episodeSummaries[i].ToTurn > f.episodeSummaries[j].ToTurn
	})
}

func (f *fixtureStore) SaveChatLog(ctx context.Context, log *ChatLog) error { return ErrNotEnabled }
func (f *fixtureStore) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SaveMemory(ctx context.Context, m *Memory) error { return ErrNotEnabled }
func (f *fixtureStore) SaveEvidence(ctx context.Context, e *DirectEvidence) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SaveKGTriple(ctx context.Context, t *KGTriple) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SaveAuditLog(ctx context.Context, a *AuditLog) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SaveCriticFeedback(ctx context.Context, cf *CriticFeedback) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error {
	return ErrNotEnabled
}

func (f *fixtureStore) SaveEntity(ctx context.Context, e *Entity) error { return ErrNotEnabled }
func (f *fixtureStore) SaveTrust(ctx context.Context, t *Trust) error   { return ErrNotEnabled }

func (f *fixtureStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]ChatLog, error) {
	out := []ChatLog{}
	for _, item := range f.chatLogs {
		if matchesSession(item.ChatSessionID, sid) && matchesTurn(item.TurnIndex, fromTurn, toTurn) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) GetEffectiveInput(ctx context.Context, sid string, turnIndex int) (*EffectiveInput, error) {
	for _, item := range f.effectiveInputs {
		if item.ChatSessionID == sid && item.TurnIndex == turnIndex {
			cp := item
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fixtureStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]Memory, error) {
	out := []Memory{}
	for _, item := range f.memories {
		if matchesSession(item.ChatSessionID, sid) && matchesTurn(item.TurnIndex, fromTurn, toTurn) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) ListEvidence(ctx context.Context, sid string) ([]DirectEvidence, error) {
	out := []DirectEvidence{}
	for _, item := range f.evidence {
		if matchesSession(item.ChatSessionID, sid) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) ListKGTriples(ctx context.Context, sid string) ([]KGTriple, error) {
	out := []KGTriple{}
	for _, item := range f.kgTriples {
		if matchesSession(item.ChatSessionID, sid) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) ListAuditLogs(ctx context.Context, sid string, eventType string, limit int) ([]AuditLog, error) {
	out := []AuditLog{}
	for _, item := range f.auditLogs {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if strings.TrimSpace(eventType) != "" && item.EventType != eventType {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fixtureStore) CountAuditLogs(ctx context.Context, sid string, eventType string) (int, error) {
	total := 0
	for _, item := range f.auditLogs {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if strings.TrimSpace(eventType) != "" && item.EventType != eventType {
			continue
		}
		total++
	}
	return total, nil
}

func (f *fixtureStore) ListCriticFeedback(ctx context.Context, sid string, targetType string, targetID int64) ([]CriticFeedback, error) {
	out := []CriticFeedback{}
	for _, item := range f.criticFeedback {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if targetType != "" && item.TargetType != targetType {
			continue
		}
		if targetID > 0 && item.TargetID != targetID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fixtureStore) ListCharacterEvents(ctx context.Context, sid string, characterName string) ([]CharacterEvent, error) {
	out := []CharacterEvent{}
	for _, item := range f.characterEvents {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if characterName != "" && item.CharacterName != characterName {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fixtureStore) Stats(ctx context.Context) (StatsResult, error) {
	return StatsResult{ChatLogs: int64(len(f.chatLogs)), Memories: int64(len(f.memories)), KgTriples: int64(len(f.kgTriples))}, nil
}

func (f *fixtureStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	counts := map[string]struct {
		chatLogs     int
		memories     int
		kgTriples    int
		lastID       int64
		lastActivity time.Time
	}{}
	for _, item := range f.chatLogs {
		c := counts[item.ChatSessionID]
		c.chatLogs++
		if item.ID > c.lastID {
			c.lastID = item.ID
		}
		if item.CreatedAt.After(c.lastActivity) {
			c.lastActivity = item.CreatedAt
		}
		counts[item.ChatSessionID] = c
	}
	for _, item := range f.memories {
		c := counts[item.ChatSessionID]
		c.memories++
		if item.CreatedAt.After(c.lastActivity) {
			c.lastActivity = item.CreatedAt
		}
		counts[item.ChatSessionID] = c
	}
	for _, item := range f.kgTriples {
		c := counts[item.ChatSessionID]
		c.kgTriples++
		if item.CreatedAt.After(c.lastActivity) {
			c.lastActivity = item.CreatedAt
		}
		counts[item.ChatSessionID] = c
	}
	out := make([]SessionSummary, 0, len(counts))
	for sid, c := range counts {
		out = append(out, SessionSummary{
			ChatSessionID:  sid,
			ChatLogsCount:  c.chatLogs,
			MemoriesCount:  c.memories,
			KGTriplesCount: c.kgTriples,
			LastActivity:   c.lastActivity,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastActivity.Equal(out[j].LastActivity) {
			return out[i].LastActivity.After(out[j].LastActivity)
		}
		return out[i].ChatSessionID < out[j].ChatSessionID
	})
	return out, nil
}

func (f *fixtureStore) GetResumePack(ctx context.Context, sid string, trigger string) (*ResumePack, error) {
	return nil, nil
}

func (f *fixtureStore) ListStorylines(ctx context.Context, sid string) ([]Storyline, error) {
	out := []Storyline{}
	for _, item := range f.storylines {
		if matchesSession(item.ChatSessionID, sid) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) SaveStoryline(ctx context.Context, s *Storyline) error { return ErrNotEnabled }

func (f *fixtureStore) ListWorldRules(ctx context.Context, sid string) ([]WorldRule, error) {
	out := []WorldRule{}
	for _, item := range f.worldRules {
		if matchesSession(item.ChatSessionID, sid) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) SaveWorldRule(ctx context.Context, w *WorldRule) error { return ErrNotEnabled }
func (f *fixtureStore) SaveCharacterState(ctx context.Context, c *CharacterState) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SavePendingThread(ctx context.Context, p *PendingThread) error {
	return ErrNotEnabled
}
func (f *fixtureStore) SaveActiveState(ctx context.Context, a *ActiveState) error {
	return ErrNotEnabled
}

func (f *fixtureStore) SaveCanonicalStateLayer(ctx context.Context, item *CanonicalStateLayer) error {
	return ErrNotEnabled
}

func (f *fixtureStore) ListInheritedWorldRules(ctx context.Context, sid string, activeScope, scopeName string) ([]WorldRule, error) {
	items, err := f.ListWorldRules(ctx, sid)
	if err != nil {
		return nil, err
	}
	activeScope = strings.TrimSpace(activeScope)
	if activeScope == "" {
		if saved, err := f.GetActiveScope(ctx, sid); err == nil && saved != nil {
			activeScope = saved.ActiveScope
			if strings.TrimSpace(scopeName) == "" {
				scopeName = saved.ScopeName
			}
		}
	}
	if activeScope == "" {
		activeScope = "root"
	}
	chain := WorldRuleScopeChain(activeScope)
	chainOrder := map[string]int{}
	for i, scope := range chain {
		chainOrder[scope] = i
	}
	out := []WorldRule{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		if _, ok := chainOrder[item.Scope]; !ok {
			continue
		}
		if item.Scope == activeScope {
			if strings.TrimSpace(scopeName) != "" && strings.TrimSpace(item.ScopeName) != strings.TrimSpace(scopeName) {
				continue
			}
			if strings.TrimSpace(scopeName) == "" && strings.TrimSpace(item.ScopeName) != "" {
				continue
			}
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := chainOrder[out[i].Scope], chainOrder[out[j].Scope]
		if left != right {
			return left < right
		}
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		for _, cmp := range []int{strings.Compare(out[i].Category, out[j].Category), strings.Compare(out[i].Key, out[j].Key)} {
			if cmp != 0 {
				return cmp < 0
			}
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fixtureStore) GetGuidancePlanState(ctx context.Context, sid string) (*GuidancePlanState, error) {
	for _, item := range f.guidancePlanStates {
		if matchesSession(item.ChatSessionID, sid) {
			cp := item
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fixtureStore) UpsertGuidancePlanState(ctx context.Context, item *GuidancePlanState) error {
	return ErrNotEnabled
}

func (f *fixtureStore) GetActiveScope(ctx context.Context, sid string) (*SessionActiveScope, error) {
	for _, item := range f.sessionActiveScopes {
		if matchesSession(item.ChatSessionID, sid) {
			cp := item
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fixtureStore) UpsertActiveScope(ctx context.Context, item *SessionActiveScope) error {
	return ErrNotEnabled
}

func (f *fixtureStore) ListCharacterStates(ctx context.Context, sid string) ([]CharacterState, error) {
	out := []CharacterState{}
	for _, item := range f.characterStates {
		if matchesSession(item.ChatSessionID, sid) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fixtureStore) GetCharacterState(ctx context.Context, sid, characterName string) (*CharacterState, error) {
	for _, item := range f.characterStates {
		if item.ChatSessionID == sid && item.CharacterName == characterName {
			cp := item
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fixtureStore) ListPendingThreads(ctx context.Context, sid, status string) ([]PendingThread, error) {
	out := []PendingThread{}
	for _, item := range f.pendingThreads {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		switch {
		case strings.TrimSpace(status) == "":
			if item.Status != "open" && item.Status != "paused" {
				continue
			}
		case status != "all" && item.Status != status:
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fixtureStore) ListActiveStates(ctx context.Context, sid, stateType string) ([]ActiveState, error) {
	out := []ActiveState{}
	for _, item := range f.activeStates {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if stateType != "" && item.StateType != stateType {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fixtureStore) ListCanonicalStateLayers(ctx context.Context, sid, layerType string) ([]CanonicalStateLayer, error) {
	out := []CanonicalStateLayer{}
	for _, item := range f.canonicalStateLayers {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if layerType != "" && item.LayerType != layerType {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fixtureStore) ListEpisodeSummaries(ctx context.Context, sid string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	out := []EpisodeSummary{}
	for _, item := range f.episodeSummaries {
		if !matchesSession(item.ChatSessionID, sid) {
			continue
		}
		if fromTurn > 0 && item.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && item.FromTurn > toTurn {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fixtureStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	for _, item := range f.episodeSummaries {
		if item.ID == episodeID {
			cp := item
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

// RollbackStore stubs for fixtureStore (read-only, returns ErrNotEnabled).
func (f *fixtureStore) DeleteChatLogs(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteEffectiveInputs(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteMemories(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteEvidence(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteKGTriples(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteCriticFeedback(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteCharacterEvents(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteEntities(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteTrustStates(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteStorylines(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteWorldRules(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteCharacterStates(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeletePendingThreads(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteActiveStates(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteCanonicalStateLayers(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteEpisodeSummaries(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteGuidancePlanState(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteChapterSummaries(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteArcSummaries(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteSagaDigests(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteSessionActiveScopes(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteProtagonistEntityMemories(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteConsequenceRecords(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeletePsychologyBranches(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteThemeOffscreenCarries(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteCaptureVerificationRecords(ctx context.Context, sid string, fromTurn int) error {
	return ErrNotEnabled
}
func (f *fixtureStore) DeleteSession(ctx context.Context, sid string) error { return ErrNotEnabled }

func matchesSession(rowSID, filterSID string) bool {
	return filterSID == "" || rowSID == filterSID
}

func matchesTurn(turn, fromTurn, toTurn int) bool {
	if fromTurn > 0 && turn < fromTurn {
		return false
	}
	if toTurn > 0 && turn > toTurn {
		return false
	}
	return true
}

func (f *fixtureStore) loadChatLog(row map[string]any) {
	f.chatLogs = append(f.chatLogs, ChatLog{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), TurnIndex: intField(row, "turn_index"), Role: stringField(row, "role"), Content: stringField(row, "content"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadEffectiveInput(row map[string]any) {
	f.effectiveInputs = append(f.effectiveInputs, EffectiveInput{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), TurnIndex: intField(row, "turn_index"), EffectiveInput: stringField(row, "effective_input"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadMemory(row map[string]any) {
	f.memories = append(f.memories, Memory{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), TurnIndex: intField(row, "turn_index"), SummaryJSON: stringField(row, "summary_json"), Embedding: stringField(row, "embedding"), EmbeddingModel: stringField(row, "embedding_model"), Importance: floatField(row, "importance"), EmotionalBoost: floatField(row, "emotional_boost"), Evidence: stringField(row, "evidence"), EmotionalIntensity: floatField(row, "emotional_intensity"), NarrativeSignificance: floatField(row, "narrative_significance"), PlaceWing: stringField(row, "place_wing"), PlaceRoom: stringField(row, "place_room"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadDirectEvidence(row map[string]any) {
	f.evidence = append(f.evidence, DirectEvidence{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), EvidenceKind: stringField(row, "evidence_kind"), EvidenceText: stringField(row, "evidence_text"), SourceTurnStart: intField(row, "source_turn_start"), SourceTurnEnd: intField(row, "source_turn_end"), TurnAnchor: intField(row, "turn_anchor"), SourceMessageIDsJSON: stringField(row, "source_message_ids_json"), SourceHash: stringField(row, "source_hash"), ArchiveState: stringField(row, "archive_state"), CaptureStage: stringField(row, "capture_stage"), CaptureVerification: stringField(row, "capture_verification"), CommittedGate: stringField(row, "committed_gate"), LineageJSON: stringField(row, "lineage_json"), RepairNeeded: boolField(row, "repair_needed"), Tombstoned: boolField(row, "tombstoned"), SupersededByID: int64Field(row, "superseded_by_id"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadKGTriple(row map[string]any) {
	f.kgTriples = append(f.kgTriples, KGTriple{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), Subject: stringField(row, "subject"), Predicate: stringField(row, "predicate"), Object: stringField(row, "object"), ValidFrom: intField(row, "valid_from"), ValidTo: intField(row, "valid_to"), SourceTurn: intField(row, "source_turn"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadAuditLog(row map[string]any) {
	f.auditLogs = append(f.auditLogs, AuditLog{ID: int64Field(row, "id"), CreatedAt: timeField(row, "created_at"), EventType: stringField(row, "event_type"), ChatSessionID: stringField(row, "chat_session_id"), TargetType: stringField(row, "target_type"), TargetID: int64Field(row, "target_id"), Summary: stringField(row, "summary"), DetailsJSON: stringField(row, "details_json"), Source: stringField(row, "source")})
}

func (f *fixtureStore) loadCriticFeedback(row map[string]any) {
	f.criticFeedback = append(f.criticFeedback, CriticFeedback{ID: int64Field(row, "id"), CreatedAt: timeField(row, "created_at"), ChatSessionID: stringField(row, "chat_session_id"), TargetType: stringField(row, "target_type"), TargetID: int64Field(row, "target_id"), FeedbackValue: stringField(row, "feedback_value"), FeedbackNote: stringField(row, "feedback_note"), Source: stringField(row, "source")})
}

func (f *fixtureStore) loadCharacterEvent(row map[string]any) {
	f.characterEvents = append(f.characterEvents, CharacterEvent{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), CharacterName: stringField(row, "character_name"), TurnIndex: intField(row, "turn_index"), EventType: stringField(row, "event_type"), DetailsJSON: stringField(row, "details_json"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadStoryline(row map[string]any) {
	f.storylines = append(f.storylines, Storyline{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), Name: stringField(row, "name"), Status: stringField(row, "status"), EntitiesJSON: stringField(row, "entities_json"), CurrentContext: stringField(row, "current_context"), KeyPointsJSON: stringField(row, "key_points_json"), OngoingTensionsJSON: stringField(row, "ongoing_tensions_json"), Confidence: floatField(row, "confidence"), EvidenceCount: intField(row, "evidence_count"), LastEvidenceTurn: intField(row, "last_evidence_turn"), FirstTurn: intField(row, "first_turn"), LastTurn: intField(row, "last_turn"), Pinned: boolField(row, "pinned"), Suppressed: boolField(row, "suppressed"), UserCorrected: boolField(row, "user_corrected"), CreatedAt: timeField(row, "created_at"), UpdatedAt: timeField(row, "updated_at")})
}

func (f *fixtureStore) loadWorldRule(row map[string]any) {
	f.worldRules = append(f.worldRules, WorldRule{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), Scope: stringField(row, "scope"), ScopeName: stringField(row, "scope_name"), Category: stringField(row, "category"), Key: stringField(row, "key"), ValueJSON: stringField(row, "value_json"), Genre: stringField(row, "genre"), SourceTurn: intField(row, "source_turn"), Pinned: boolField(row, "pinned"), Suppressed: boolField(row, "suppressed"), UserCorrected: boolField(row, "user_corrected"), CreatedAt: timeField(row, "created_at"), UpdatedAt: timeField(row, "updated_at")})
}

func (f *fixtureStore) loadSessionActiveScope(row map[string]any) {
	f.sessionActiveScopes = append(f.sessionActiveScopes, SessionActiveScope{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), ActiveScope: stringField(row, "active_scope"), ScopeName: stringField(row, "scope_name"), UpdatedAt: timeField(row, "updated_at")})
}

func (f *fixtureStore) loadCharacterState(row map[string]any) {
	f.characterStates = append(f.characterStates, CharacterState{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), CharacterName: stringField(row, "character_name"), AppearanceJSON: stringField(row, "appearance_json"), PersonalityJSON: stringField(row, "personality_json"), StatusJSON: stringField(row, "status_json"), RelationshipsJSON: stringField(row, "relationships_json"), SpeechStyleJSON: stringField(row, "speech_style_json"), TurnIndex: intField(row, "turn_index"), CreatedAt: timeField(row, "created_at"), UpdatedAt: timeField(row, "updated_at")})
}

func (f *fixtureStore) loadPendingThread(row map[string]any) {
	title := stringField(row, "title")
	threadType := stringField(row, "thread_type")
	details := stringField(row, "details_json")
	confidence := floatField(row, "confidence")
	lastSeenTurn := intField(row, "last_seen_turn")
	f.pendingThreads = append(f.pendingThreads, PendingThread{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), ThreadKey: title, Description: title, Status: stringField(row, "status"), CreatedTurn: intField(row, "source_turn"), ResolvedTurn: lastSeenTurn, SourceTurn: intField(row, "source_turn"), Priority: int(confidence * 100), HookType: threadType, HookMetadataJSON: details, ThreadType: threadType, Title: title, Owner: stringField(row, "owner"), Target: stringField(row, "target"), LastSeenTurn: lastSeenTurn, Confidence: confidence, DetailsJSON: details, ResolutionNote: stringField(row, "resolution_note"), Pinned: boolField(row, "pinned"), Suppressed: boolField(row, "suppressed"), UserCorrected: boolField(row, "user_corrected"), CreatedAt: timeField(row, "created_at"), UpdatedAt: timeField(row, "updated_at")})
}

func (f *fixtureStore) loadActiveState(row map[string]any) {
	f.activeStates = append(f.activeStates, ActiveState{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), StateType: stringField(row, "state_type"), Content: stringField(row, "content"), TurnIndex: intField(row, "turn_index"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadCanonicalStateLayer(row map[string]any) {
	f.canonicalStateLayers = append(f.canonicalStateLayers, CanonicalStateLayer{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), LayerType: stringField(row, "layer_type"), Content: stringField(row, "content"), SourceStateType: stringField(row, "source_state_type"), TurnIndex: intField(row, "turn_index"), SourceTurn: intField(row, "source_turn"), SourceRecord: int64Field(row, "source_record"), LastVerifiedTurn: intField(row, "last_verified_turn"), Confidence: floatField(row, "confidence"), CreatedAt: timeField(row, "created_at")})
}

func (f *fixtureStore) loadGuidancePlanState(row map[string]any) {
	f.guidancePlanStates = append(f.guidancePlanStates, GuidancePlanState{
		ID:            int64Field(row, "id"),
		ChatSessionID: stringField(row, "chat_session_id"),
		StoryPlanJSON: stringField(row, "story_plan_json"),
		DirectorJSON:  stringField(row, "director_json"),
		StateStatus:   stringField(row, "state_status"),
		LastTurn:      intField(row, "last_turn"),
		WarningsJSON:  stringField(row, "warnings_json"),
		CreatedAt:     timeField(row, "created_at"),
		UpdatedAt:     timeField(row, "updated_at"),
	})
}

func (f *fixtureStore) loadEpisodeSummary(row map[string]any) {
	f.episodeSummaries = append(f.episodeSummaries, EpisodeSummary{ID: int64Field(row, "id"), ChatSessionID: stringField(row, "chat_session_id"), FromTurn: intField(row, "from_turn"), ToTurn: intField(row, "to_turn"), SummaryText: stringField(row, "summary_text"), KeyEntities: stringField(row, "key_entities"), KeyEvents: stringField(row, "key_events"), OpenLoopsJSON: stringField(row, "open_loops_json"), RelationshipChangesJSON: stringField(row, "relationship_changes_json"), EmbeddingVector: stringField(row, "embedding_vector"), EmbeddingModel: stringField(row, "embedding_model"), CreatedAt: timeField(row, "created_at")})
}

func stringField(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func intField(row map[string]any, key string) int {
	return int(int64Field(row, key))
}

func int64Field(row map[string]any, key string) int64 {
	v, ok := row[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func floatField(row map[string]any, key string) float64 {
	v, ok := row[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		n, _ := t.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(t, 64)
		return n
	default:
		return 0
	}
}

func boolField(row map[string]any, key string) bool {
	v, ok := row[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	default:
		return false
	}
}

func timeField(row map[string]any, key string) time.Time {
	raw := stringField(row, key)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	naiveLayouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	kst := time.FixedZone("KST", 9*60*60)
	for _, layout := range naiveLayouts {
		if t, err := time.ParseInLocation(layout, raw, kst); err == nil {
			return t
		}
	}
	return time.Time{}
}
