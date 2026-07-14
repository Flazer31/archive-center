package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

func referenceGroundedSourceExcerpt(source, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	if index := strings.Index(source, candidate); index >= 0 {
		return truncateRunes(strings.TrimSpace(source[index:index+len(candidate)]), 800)
	}

	sourceRunes := []rune(source)
	normalizedSource, sourcePositions := referenceCollapseWhitespaceWithPositions(sourceRunes)
	normalizedCandidate, _ := referenceCollapseWhitespaceWithPositions([]rune(candidate))
	if len(normalizedCandidate) == 0 || len(normalizedCandidate) > len(normalizedSource) {
		return ""
	}
	for start := 0; start+len(normalizedCandidate) <= len(normalizedSource); start++ {
		matched := true
		for offset := range normalizedCandidate {
			if normalizedSource[start+offset] != normalizedCandidate[offset] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		rawStart := sourcePositions[start]
		rawEnd := sourcePositions[start+len(normalizedCandidate)-1] + 1
		return truncateRunes(strings.TrimSpace(string(sourceRunes[rawStart:rawEnd])), 800)
	}
	return ""
}

func referenceCollapseWhitespaceWithPositions(values []rune) ([]rune, []int) {
	normalized := make([]rune, 0, len(values))
	positions := make([]int, 0, len(values))
	for index, value := range values {
		if unicode.IsSpace(value) {
			if len(normalized) == 0 || normalized[len(normalized)-1] == ' ' {
				continue
			}
			normalized = append(normalized, ' ')
			positions = append(positions, index)
			continue
		}
		normalized = append(normalized, value)
		positions = append(positions, index)
	}
	if len(normalized) > 0 && normalized[len(normalized)-1] == ' ' {
		normalized = normalized[:len(normalized)-1]
		positions = positions[:len(positions)-1]
	}
	return normalized, positions
}

func referenceCoverageGroundedSource(scope referenceRecallScope, kind, sourceID string) (string, string, *int) {
	metadataJSON := ""
	excerpt := ""
	documentID := ""
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "entity":
		item, ok := scope.entities[sourceID]
		if !ok {
			return "", "", nil
		}
		metadataJSON = item.MetadataJSON
	case "timeline":
		item, ok := scope.nodes[sourceID]
		if !ok {
			return "", "", nil
		}
		metadataJSON = item.MetadataJSON
	case "claim":
		item, ok := scope.claims[sourceID]
		if !ok {
			return "", "", nil
		}
		metadataJSON = item.MetadataJSON
		excerpt = strings.TrimSpace(item.EvidenceExcerpt)
		documentID = strings.TrimSpace(item.DocumentID)
	default:
		return "", "", nil
	}

	metadata := map[string]any{}
	if strings.TrimSpace(metadataJSON) != "" {
		_ = json.Unmarshal([]byte(metadataJSON), &metadata)
	}
	if grounded, _ := metadata["evidence_grounded"].(bool); !grounded {
		return "", "", nil
	}
	if excerpt == "" {
		if raw, ok := metadata["evidence_excerpt"]; ok && raw != nil {
			excerpt = strings.TrimSpace(fmt.Sprint(raw))
		}
	}
	if documentID == "" {
		if raw, ok := metadata["document_id"]; ok && raw != nil {
			documentID = strings.TrimSpace(fmt.Sprint(raw))
		}
	}
	var chunkIndex *int
	if raw, ok := metadata["chunk_index"]; ok {
		value := intFromAny(raw, 0)
		chunkIndex = &value
	}
	if excerpt == "" {
		return "", documentID, chunkIndex
	}
	return excerpt, documentID, chunkIndex
}
