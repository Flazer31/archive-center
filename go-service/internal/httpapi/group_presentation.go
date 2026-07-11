package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const presentationViewModelContractVersion = "presentation.viewmodel.v1"

type presentationViewModelRequest struct {
	Timeline presentationTimelineInput `json:"timeline"`
	Explorer presentationExplorerInput `json:"explorer"`
}

type presentationTimelineInput struct {
	Items             []map[string]any `json:"items"`
	PendingItems      []map[string]any `json:"pending_items"`
	Sessions          []map[string]any `json:"sessions"`
	SelectedSessionID string           `json:"selected_session_id"`
	CurrentSessionID  string           `json:"current_session_id"`
	Meta              map[string]any   `json:"meta"`
	LifecycleByID     map[string]any   `json:"lifecycle_by_id"`
	NowMS             int64            `json:"now_ms"`
	Loading           bool             `json:"loading"`
	LoadingMore       bool             `json:"loading_more"`
	HasMore           bool             `json:"has_more"`
	Error             string           `json:"error"`
}

type presentationExplorerInput struct {
	Sessions            []map[string]any `json:"sessions"`
	SelectedSessionID   string           `json:"selected_session_id"`
	ActiveChatSessionID string           `json:"active_chat_session_id"`
	ActiveTab           string           `json:"active_tab"`
	Totals              map[string]any   `json:"totals"`
	Trust               map[string]any   `json:"trust"`
	WorldGraph          map[string]any   `json:"world_graph"`
	Entities            map[string]any   `json:"entities"`
	SessionsLoading     bool             `json:"sessions_loading"`
}

type presentationViewModelResponse struct {
	ContractVersion string                    `json:"contract_version"`
	Status          string                    `json:"status"`
	Timeline        presentationTimelineModel `json:"timeline"`
	Explorer        presentationExplorerModel `json:"explorer"`
}

type presentationTimelineModel struct {
	Items         []map[string]any            `json:"items"`
	Groups        []presentationTimelineGroup `json:"groups"`
	Sessions      []presentationSessionRow    `json:"sessions"`
	SelectedID    string                      `json:"selected_session_id"`
	Summary       map[string]any              `json:"summary"`
	EmptyState    string                      `json:"empty_state"`
	LoadMoreState string                      `json:"load_more_state"`
}

type presentationTimelineGroup struct {
	Key         string           `json:"key"`
	TurnText    string           `json:"turn_text"`
	CreatedText string           `json:"created_text"`
	Kind        string           `json:"kind"`
	Preview     string           `json:"preview"`
	Counts      map[string]int   `json:"counts"`
	Items       []map[string]any `json:"items"`
	ItemCount   int              `json:"item_count"`
}

type presentationSessionRow struct {
	SessionID  string         `json:"session_id"`
	Selected   bool           `json:"selected"`
	Current    bool           `json:"current"`
	Deleted    bool           `json:"deleted"`
	Status     string         `json:"status"`
	Label      string         `json:"label"`
	CanAttach  bool           `json:"can_attach"`
	CanCopy    bool           `json:"can_copy"`
	CanMigrate bool           `json:"can_migrate"`
	Counts     map[string]int `json:"counts"`
	LastActive any            `json:"last_activity,omitempty"`
}

type presentationExplorerModel struct {
	Sessions   []presentationSessionRow  `json:"sessions"`
	Tabs       []presentationExplorerTab `json:"tabs"`
	ActiveTab  string                    `json:"active_tab"`
	SyncState  string                    `json:"sync_state"`
	SelectedID string                    `json:"selected_session_id"`
}

type presentationExplorerTab struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

func (s *Server) registerPresentationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /presentation/view-model", s.handlePresentationViewModel)
}

func (s *Server) handlePresentationViewModel(w http.ResponseWriter, r *http.Request) {
	var req presentationViewModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_presentation_snapshot"})
		return
	}
	writeJSON(w, http.StatusOK, buildPresentationViewModel(req))
}

func buildPresentationViewModel(req presentationViewModelRequest) presentationViewModelResponse {
	return presentationViewModelResponse{
		ContractVersion: presentationViewModelContractVersion,
		Status:          "ok",
		Timeline:        buildPresentationTimelineModel(req.Timeline),
		Explorer:        buildPresentationExplorerModel(req.Explorer),
	}
}

func buildPresentationTimelineModel(input presentationTimelineInput) presentationTimelineModel {
	nowMS := input.NowMS
	if nowMS <= 0 {
		nowMS = time.Now().UnixMilli()
	}
	visible := make([]map[string]any, 0, len(input.PendingItems)+len(input.Items))
	for _, item := range input.PendingItems {
		if !presentationPendingVisible(item, input.SelectedSessionID, nowMS) {
			continue
		}
		visible = append(visible, presentationTimelineItem(item))
	}
	for _, item := range input.Items {
		visible = append(visible, presentationTimelineItem(item))
	}

	groups := make([]presentationTimelineGroup, 0)
	groupIndex := map[string]int{}
	for _, item := range visible {
		view := presentationMap(item["view"])
		key := presentationString(view["display_turn_key"])
		idx, exists := groupIndex[key]
		if !exists {
			groups = append(groups, presentationTimelineGroup{
				Key: key, TurnText: presentationString(view["turn_text"]),
				CreatedText: presentationString(item["created_at"]), Kind: "turn",
				Counts: map[string]int{}, Items: []map[string]any{},
			})
			idx = len(groups) - 1
			groupIndex[key] = idx
		}
		group := &groups[idx]
		rawType := strings.ToLower(presentationString(item["type"]))
		if rawType == "" {
			rawType = presentationString(view["normalized_type"])
		}
		group.Counts[rawType]++
		group.Items = append(group.Items, item)
		if group.CreatedText == "" {
			group.CreatedText = presentationString(item["created_at"])
		}
	}
	for i := range groups {
		group := &groups[i]
		if group.Key == "turn:-1" {
			group.Kind = "starter"
		}
		group.ItemCount = len(group.Items)
		group.Preview = presentationTimelineGroupPreview(group.Items)
	}

	total := len(visible)
	if metaTotal := presentationInt(input.Meta["total_unpaged"]); metaTotal > 0 {
		total = metaTotal
	}
	emptyState := "ready"
	if len(visible) == 0 {
		switch {
		case input.Loading:
			emptyState = "loading"
		case strings.TrimSpace(input.Error) != "":
			emptyState = "error"
		default:
			emptyState = "empty"
		}
	}
	loadMoreState := "none"
	if len(visible) > 0 {
		switch {
		case input.LoadingMore:
			loadMoreState = "loading"
		case input.HasMore:
			loadMoreState = "available"
		default:
			loadMoreState = "complete"
		}
	}
	return presentationTimelineModel{
		Items: visible, Groups: groups,
		Sessions:   buildPresentationSessionRows(input.Sessions, input.SelectedSessionID, input.CurrentSessionID, input.LifecycleByID),
		SelectedID: input.SelectedSessionID,
		Summary:    map[string]any{"turns": len(groups), "items": len(visible), "total": total, "source_counts": input.Meta["source_counts"]},
		EmptyState: emptyState, LoadMoreState: loadMoreState,
	}
}

func presentationTimelineItem(source map[string]any) map[string]any {
	item := presentationCloneMap(source)
	rawType := strings.ToLower(presentationString(item["type"]))
	role := strings.ToLower(presentationString(item["role"]))
	normalized := "user"
	switch rawType {
	case "chat_log":
		if role == "assistant" {
			normalized = "assistant"
		}
	case "kg_triple", "episode":
		normalized = "episode"
	case "direct_evidence", "evidence", "memory":
		normalized = "memory"
	case "pending_artifacts":
		normalized = "pending"
	}
	id := presentationFirst(item["id"], item["source_id"])
	itemKey := presentationFirstString(rawType, "item") + ":" + presentationString(id)
	turn := presentationTurnText(item)
	title := presentationString(item["title"])
	titleCode := ""
	if title == "" {
		switch rawType {
		case "chat_log":
			title = presentationFirstString(role, "chat_log")
		case "memory":
			titleCode = "memoryTitle"
		case "episode":
			titleCode = "episodeTitle"
		case "kg_triple":
			titleCode = "kgTitle"
		case "evidence", "direct_evidence":
			titleCode = "evidenceTitle"
		case "pending_artifacts":
			titleCode = "pendingTitle"
		default:
			titleCode = "itemFallback"
		}
	}
	displayTurnKey := "item:" + itemKey
	if turn != "" {
		displayTurnKey = "turn:" + turn
	}
	editableType := ""
	switch rawType {
	case "memory":
		editableType = "mem"
	case "evidence", "direct_evidence":
		editableType = "de"
	case "kg_triple":
		editableType = "kg"
	}
	item["view"] = map[string]any{
		"key": itemKey, "normalized_type": normalized, "raw_type": rawType,
		"title": title, "title_code": titleCode, "preview": presentationReadableText(presentationFirst(item["preview"], item["summary"], item["content"], item["summary_json"], item["summary_text"], item["evidence_text"])),
		"turn_text": turn, "display_turn_key": displayTurnKey, "source_id": id,
		"editable_type": editableType, "can_edit": editableType != "" && id != nil,
	}
	return item
}

func presentationTimelineGroupPreview(items []map[string]any) string {
	if len(items) == 0 {
		return ""
	}
	priority := func(item map[string]any) int {
		t := strings.ToLower(presentationString(item["type"]))
		r := strings.ToLower(presentationString(item["role"]))
		switch {
		case t == "memory" || t == "evidence" || t == "direct_evidence":
			return 1
		case t == "episode":
			return 2
		case t == "chat_log" && r == "assistant":
			return 3
		default:
			return 4
		}
	}
	ordered := append([]map[string]any(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool { return priority(ordered[i]) < priority(ordered[j]) })
	for _, item := range ordered {
		view := presentationMap(item["view"])
		text := presentationFirstString(presentationString(view["preview"]), presentationString(view["title"]))
		if text != "" {
			return presentationCompactText(text, 72)
		}
	}
	return ""
}

func buildPresentationExplorerModel(input presentationExplorerInput) presentationExplorerModel {
	active := presentationFirstString(input.ActiveTab, "chat_logs")
	tabs := []presentationExplorerTab{
		{Key: "chat_logs", Count: presentationInt(input.Totals["chat_logs"])},
		{Key: "memories", Count: presentationInt(input.Totals["memories"])},
		{Key: "direct_evidence", Count: presentationInt(input.Totals["direct_evidence"])},
		{Key: "kg_triples", Count: presentationInt(input.Totals["kg_triples"])},
		{Key: "episodes", Count: presentationInt(input.Totals["episodes"]) + presentationInt(input.Totals["chapters"]) + presentationInt(input.Totals["arcs"]) + presentationInt(input.Totals["sagas"])},
		{Key: "trust", Count: presentationSliceLen(input.Trust["storylines"]) + presentationSliceLen(input.Trust["world_rules"]) + presentationSliceLen(input.Trust["hooks"])},
		{Key: "world", Count: presentationWorldCount(input.WorldGraph)},
		{Key: "entities", Count: presentationSliceLen(input.Entities["characters"]) + presentationSliceLen(input.Entities["locations"]) + presentationSliceLen(input.Entities["items"])},
	}
	syncState := "unknown"
	if input.ActiveChatSessionID != "" {
		if input.SelectedSessionID == input.ActiveChatSessionID {
			syncState = "current"
		} else {
			syncState = "different"
		}
	}
	return presentationExplorerModel{
		Sessions: buildPresentationSessionRows(input.Sessions, input.SelectedSessionID, input.ActiveChatSessionID, nil),
		Tabs:     tabs, ActiveTab: active, SyncState: syncState, SelectedID: input.SelectedSessionID,
	}
}

func buildPresentationSessionRows(sessions []map[string]any, selectedID, currentID string, lifecycleByID map[string]any) []presentationSessionRow {
	rows := make([]presentationSessionRow, 0, len(sessions))
	for _, session := range sessions {
		sid := presentationFirstString(presentationString(session["chat_session_id"]), presentationString(session["session_id"]))
		if sid == "" {
			continue
		}
		lifecycle := presentationMap(lifecycleByID[sid])
		deleted := presentationBool(lifecycle["deleted"])
		placeholder := sid == "char_-1"
		canMove := currentID != "" && sid != currentID && !placeholder && !deleted
		rows = append(rows, presentationSessionRow{
			SessionID: sid, Selected: sid == selectedID, Current: sid == currentID,
			Deleted: deleted, Status: presentationFirstString(presentationString(lifecycle["status"]), "active"), Label: presentationString(lifecycle["label"]),
			CanAttach: canMove, CanCopy: canMove, CanMigrate: canMove,
			Counts:     map[string]int{"chat_logs": presentationInt(session["chat_logs_count"]), "memories": presentationInt(session["memories_count"]), "kg_triples": presentationInt(session["kg_triples_count"])},
			LastActive: session["last_activity"],
		})
	}
	return rows
}

func presentationPendingVisible(item map[string]any, selectedID string, nowMS int64) bool {
	expires := int64(presentationFloat(item["expires_at_ms"]))
	if expires > 0 && expires < nowMS {
		return false
	}
	sid := presentationFirstString(presentationString(item["chat_session_id"]), presentationString(item["session_id"]))
	return sid == "" || selectedID == "" || sid == selectedID
}

func presentationReadableText(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text != "" && (strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")) {
			var parsed any
			if json.Unmarshal([]byte(text), &parsed) == nil {
				return presentationReadableText(parsed)
			}
		}
		return typed
	case []any:
		parts := []string{}
		for _, item := range typed {
			if text := strings.TrimSpace(presentationReadableText(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"summary", "text", "content", "memory", "description", "value"} {
			if text := strings.TrimSpace(presentationReadableText(typed[key])); text != "" {
				return text
			}
		}
		encoded, _ := json.MarshalIndent(typed, "", "  ")
		return string(encoded)
	default:
		return fmt.Sprint(value)
	}
}

func presentationCompactText(value string, limit int) string {
	clean := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "**", ""), "#", " ")), " ")
	if len([]rune(clean)) <= limit {
		return clean
	}
	runes := []rune(clean)
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func presentationTurnText(item map[string]any) string {
	return presentationString(presentationFirst(item["turn_index"], item["turn_anchor"], item["source_turn"], item["to_turn"]))
}
func presentationMap(value any) map[string]any {
	if out, ok := value.(map[string]any); ok && out != nil {
		return out
	}
	return map[string]any{}
}
func presentationCloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input)+1)
	for key, value := range input {
		out[key] = value
	}
	return out
}
func presentationFirst(values ...any) any {
	for _, value := range values {
		if value != nil && presentationString(value) != "" {
			return value
		}
	}
	return nil
}
func presentationFirstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func presentationString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
func presentationFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		n, _ := typed.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(typed, 64)
		return n
	}
	return 0
}
func presentationInt(value any) int   { return int(presentationFloat(value)) }
func presentationBool(value any) bool { typed, ok := value.(bool); return ok && typed }

func presentationSliceLen(value any) int {
	if items, ok := value.([]any); ok {
		return len(items)
	}
	return 0
}

func presentationWorldCount(value map[string]any) int {
	if count := presentationSliceLen(value["all_rules"]); count > 0 {
		return count
	}
	return presentationSliceLen(value["rules"])
}
