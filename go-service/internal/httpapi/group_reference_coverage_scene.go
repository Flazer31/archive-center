package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type referenceCoverageSceneSource struct {
	Location  string
	Text      string
	TurnIndex int
}

type referenceCoverageSceneContext struct {
	RecentCompletedTurn int
	RecentDialogue      []referenceCoverageSceneSource
	CurrentLocations    []referenceCoverageSceneSource
	ActiveRules         []referenceCoverageSceneSource
	Conversation        []referenceCoverageSceneSource
}

type referenceCoverageSceneSignalSummary struct {
	RecentCompletedTurn  *int `json:"recent_completed_turn,omitempty"`
	RecentDialogueCount  int  `json:"recent_dialogue_count"`
	CurrentLocationCount int  `json:"current_location_count"`
	ActiveRuleCount      int  `json:"active_rule_count"`
}

func referenceCoverageSceneMessages(sceneContext referenceCoverageSceneContext) []referenceCoverageMessage {
	sources := append([]referenceCoverageSceneSource{}, sceneContext.RecentDialogue...)
	sources = append(sources, sceneContext.CurrentLocations...)
	sources = append(sources, sceneContext.ActiveRules...)
	out := make([]referenceCoverageMessage, 0, len(sources))
	for _, source := range sources {
		normalized := referenceCoverageNormalize(source.Text)
		if normalized == "" {
			continue
		}
		out = append(out, referenceCoverageMessage{
			location:   source.Location,
			normalized: normalized,
		})
	}
	return out
}

func summarizeReferenceCoverageSceneSignals(sceneContext referenceCoverageSceneContext) referenceCoverageSceneSignalSummary {
	summary := referenceCoverageSceneSignalSummary{
		RecentDialogueCount:  len(sceneContext.RecentDialogue),
		CurrentLocationCount: len(sceneContext.CurrentLocations),
		ActiveRuleCount:      len(sceneContext.ActiveRules),
	}
	if sceneContext.RecentCompletedTurn > 0 {
		turn := sceneContext.RecentCompletedTurn
		summary.RecentCompletedTurn = &turn
	}
	return summary
}

func buildReferenceCoverageSceneContext(chatLogs []store.ChatLog, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, worldRules []store.WorldRule, ruleLimit int) referenceCoverageSceneContext {
	context := referenceCoverageSceneContext{}
	context.RecentCompletedTurn, context.RecentDialogue = referenceCoverageLatestCompletedDialogue(chatLogs)
	context.CurrentLocations = referenceCoverageCurrentLocations(activeStates, canonicalLayers)
	context.ActiveRules = referenceCoverageActiveRules(worldRules, ruleLimit)
	context.Conversation = referenceCoverageConversationSources(chatLogs)
	return context
}

func referenceCoverageConversationSources(chatLogs []store.ChatLog) []referenceCoverageSceneSource {
	sources := []referenceCoverageSceneSource{}
	for _, item := range chatLogs {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if text := strings.TrimSpace(item.Content); text != "" {
			sources = append(sources, referenceCoverageSceneSource{
				Location:  fmt.Sprintf("chat_log:turn:%d:%s:%d", item.TurnIndex, role, item.ID),
				Text:      text,
				TurnIndex: item.TurnIndex,
			})
		}
	}
	return referenceCoverageUniqueSceneSources(sources)
}

func referenceCoverageLatestCompletedDialogue(chatLogs []store.ChatLog) (int, []referenceCoverageSceneSource) {
	type turnDialogue struct {
		hasUser      bool
		hasAssistant bool
		sources      []referenceCoverageSceneSource
	}
	turns := map[int]*turnDialogue{}
	for _, item := range chatLogs {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		text := strings.TrimSpace(item.Content)
		if text == "" {
			continue
		}
		entry := turns[item.TurnIndex]
		if entry == nil {
			entry = &turnDialogue{}
			turns[item.TurnIndex] = entry
		}
		if role == "user" {
			entry.hasUser = true
		} else {
			entry.hasAssistant = true
		}
		entry.sources = append(entry.sources, referenceCoverageSceneSource{
			Location:  fmt.Sprintf("chat_log:turn:%d:%s:%d", item.TurnIndex, role, item.ID),
			Text:      text,
			TurnIndex: item.TurnIndex,
		})
	}
	latestTurn := 0
	for turn, entry := range turns {
		if entry.hasUser && entry.hasAssistant && turn > latestTurn {
			latestTurn = turn
		}
	}
	if latestTurn == 0 {
		return 0, nil
	}
	return latestTurn, referenceCoverageUniqueSceneSources(turns[latestTurn].sources)
}

func referenceCoverageCurrentLocations(activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer) []referenceCoverageSceneSource {
	candidates := []referenceCoverageSceneSource{}
	for _, item := range activeStates {
		if text := referenceCoverageLocationText(item.StateType, item.Content); text != "" {
			candidates = append(candidates, referenceCoverageSceneSource{
				Location:  fmt.Sprintf("active_state:%s:%d", item.StateType, item.ID),
				Text:      text,
				TurnIndex: item.TurnIndex,
			})
		}
	}
	for _, item := range canonicalLayers {
		if text := referenceCoverageLocationText(item.LayerType, item.Content); text != "" {
			turn := maxInt(item.TurnIndex, item.SourceTurn)
			turn = maxInt(turn, item.LastVerifiedTurn)
			candidates = append(candidates, referenceCoverageSceneSource{
				Location:  fmt.Sprintf("canonical_state:%s:%d", item.LayerType, item.ID),
				Text:      text,
				TurnIndex: turn,
			})
		}
	}
	latestTurn := 0
	for _, item := range candidates {
		if item.TurnIndex > latestTurn {
			latestTurn = item.TurnIndex
		}
	}
	latest := []referenceCoverageSceneSource{}
	for _, item := range candidates {
		if latestTurn > 0 && item.TurnIndex != latestTurn {
			continue
		}
		latest = append(latest, item)
	}
	return referenceCoverageUniqueSceneSources(latest)
}

func referenceCoverageLocationText(stateType, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	typeKey := referenceCoverageNormalize(stateType)
	directLocation := referenceCoverageLocationKey(typeKey)
	sceneContainer := typeKey == "scene" || typeKey == "scenestate"
	if !directLocation && !sceneContainer {
		return ""
	}
	var payload any
	if json.Unmarshal([]byte(content), &payload) == nil {
		locations := []string{}
		referenceCoverageCollectLocationValues(payload, &locations)
		if len(locations) > 0 {
			return strings.Join(referenceCoverageUniqueStrings(locations), " | ")
		}
		if directLocation {
			return content
		}
		return ""
	}
	if directLocation || typeKey == "scene" {
		return content
	}
	return ""
}

func referenceCoverageCollectLocationValues(value any, out *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if referenceCoverageLocationKey(referenceCoverageNormalize(key)) {
				if text := referenceCoverageValueText(item); text != "" {
					*out = append(*out, text)
				}
				continue
			}
			referenceCoverageCollectLocationValues(item, out)
		}
	case []any:
		for _, item := range typed {
			referenceCoverageCollectLocationValues(item, out)
		}
	}
}

func referenceCoverageLocationKey(key string) bool {
	switch key {
	case "location", "currentlocation", "scenelocation", "locationname", "place", "placename", "loc":
		return true
	default:
		return false
	}
}

func referenceCoverageValueText(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func referenceCoverageActiveRules(worldRules []store.WorldRule, limit int) []referenceCoverageSceneSource {
	rules := collapsePrepareTurnWorldRules(worldRules)
	if limit > 0 && len(rules) > limit {
		rules = rules[:limit]
	}
	sources := make([]referenceCoverageSceneSource, 0, len(rules))
	for _, item := range rules {
		text := strings.TrimSpace(strings.Join(nonEmptyStrings([]string{item.ScopeName, item.Category, item.Key, item.ValueJSON}), " "))
		if text == "" {
			continue
		}
		sources = append(sources, referenceCoverageSceneSource{
			Location:  fmt.Sprintf("world_rule:%d", item.ID),
			Text:      text,
			TurnIndex: item.SourceTurn,
		})
	}
	return referenceCoverageUniqueSceneSources(sources)
}

func referenceCoverageRenderedActiveRules(text string) []referenceCoverageSceneSource {
	text = strings.TrimSpace(text)
	if text == "" {
		return []referenceCoverageSceneSource{}
	}
	return []referenceCoverageSceneSource{{
		Location: "prepare_turn:world_rules",
		Text:     text,
	}}
}

func referenceCoverageUniqueSceneSources(values []referenceCoverageSceneSource) []referenceCoverageSceneSource {
	out := []referenceCoverageSceneSource{}
	seen := map[string]bool{}
	for _, value := range values {
		text := strings.TrimSpace(value.Text)
		key := value.Location + "|" + referenceCoverageNormalize(text)
		if text == "" || seen[key] {
			continue
		}
		seen[key] = true
		value.Text = text
		out = append(out, value)
	}
	return out
}

func referenceCoverageSceneSourcesMatchItem(item referenceRecallItem, scope referenceRecallScope, sources []referenceCoverageSceneSource) bool {
	for _, source := range sources {
		text := referenceCoverageNormalize(source.Text)
		if text == "" {
			continue
		}
		switch item.ReferenceKind {
		case "entity":
			if referenceCoverageContainsAnyName(text, referenceCoverageItemEntityNames(item, scope, item.SourceID)) {
				return true
			}
		case "claim":
			claim := scope.claims[item.SourceID]
			if referenceCoverageContainsNormalized(text, referenceCoverageNormalize(claim.ClaimText)) ||
				referenceCoverageContainsAnyName(text, referenceCoverageClaimSubjectNames(item, scope, claim.SubjectEntityID)) {
				return true
			}
		case "timeline":
			node := scope.nodes[item.SourceID]
			if referenceCoverageContainsNormalized(text, referenceCoverageNormalize(node.Label)) ||
				referenceCoverageContainsNormalized(text, referenceCoverageNormalize(node.NodeKey)) {
				return true
			}
		}
	}
	return false
}
