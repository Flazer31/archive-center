package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func statusSchemaDefinitionsFromApprovedProposal(proposal store.StatusSchemaProposal) ([]store.StatusSchemaDefinition, error) {
	if strings.TrimSpace(proposal.ProposalState) != "approved" {
		return nil, errors.New("proposal must be approved before registry import")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(proposal.SchemaJSON), &root); err != nil || root == nil {
		return nil, errors.New("schema_json must be a JSON object")
	}
	stats := sliceFromAny(root["stats"])
	if len(stats) == 0 {
		stats = sliceFromAny(root["status_definitions"])
	}
	if len(stats) == 0 {
		return nil, errors.New("schema_json must include non-empty stats or status_definitions array")
	}
	definitions := make([]store.StatusSchemaDefinition, 0, len(stats))
	seen := map[string]bool{}
	for idx, raw := range stats {
		item := mapFromAny(raw)
		if len(item) == 0 {
			return nil, errors.New("status definition at index " + strconv.Itoa(idx) + " must be an object")
		}
		if statusSchemaHasExecutableFormulaField(item) {
			return nil, errors.New("status definition " + strconv.Itoa(idx) + " uses formula/script/code fields that are not enabled")
		}
		key := strings.TrimSpace(firstNonEmptyStringLocal(stringFromMap(item, "status_key"), stringFromMap(item, "key")))
		if !statusSchemaValidKey(key) {
			return nil, errors.New("status definition " + strconv.Itoa(idx) + " has invalid status_key")
		}
		if seen[key] {
			return nil, errors.New("duplicate status_key " + key)
		}
		seen[key] = true
		ownerScope := statusSchemaNormalizeOwnerScope(firstNonEmptyStringLocal(stringFromMap(item, "owner_scope"), stringFromMap(item, "scope")))
		if ownerScope == "" {
			return nil, errors.New("status definition " + key + " requires owner_scope")
		}
		valueKind := statusSchemaNormalizeValueKind(firstNonEmptyStringLocal(stringFromMap(item, "value_kind"), stringFromMap(item, "kind"), stringFromMap(item, "type")))
		if valueKind == "" {
			return nil, errors.New("status definition " + key + " requires value_kind")
		}
		boundsJSON, err := statusSchemaCompactOptionalValue(item["bounds"], "bounds")
		if err != nil {
			return nil, errors.New("status definition " + key + ": " + err.Error())
		}
		optionsJSON, err := statusSchemaCompactOptionalValue(item["options"], "options")
		if err != nil {
			return nil, errors.New("status definition " + key + ": " + err.Error())
		}
		defaultValue := item["default_value"]
		if defaultValue == nil {
			defaultValue = item["default"]
		}
		defaultValueJSON, err := statusSchemaCompactOptionalValue(defaultValue, "default_value")
		if err != nil {
			return nil, errors.New("status definition " + key + ": " + err.Error())
		}
		definitions = append(definitions, store.StatusSchemaDefinition{
			ChatSessionID:    proposal.ChatSessionID,
			SourceProposalID: proposal.ID,
			SchemaName:       proposal.SchemaName,
			RulesetLabel:     proposal.RulesetLabel,
			StatusKey:        key,
			Label:            firstNonEmptyStringLocal(stringFromMap(item, "label"), key),
			OwnerScope:       ownerScope,
			ValueKind:        valueKind,
			BoundsJSON:       boundsJSON,
			OptionsJSON:      optionsJSON,
			DefaultValueJSON: defaultValueJSON,
			RegistryState:    "active",
		})
	}
	return definitions, nil
}

func statusSchemaHasExecutableFormulaField(item map[string]any) bool {
	for _, key := range []string{"formula", "script", "code", "expression"} {
		if hasMeaningfulPayload(item[key]) {
			return true
		}
	}
	return false
}

func statusSchemaValidKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

func statusSchemaNormalizeOwnerScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "character", "party", "faction", "world", "entity", "session":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusSchemaNormalizeValueKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "scalar", "number", "numeric":
		return "scalar"
	case "resource":
		return "resource"
	case "enum", "choice":
		return "enum"
	case "boolean", "bool":
		return "boolean"
	case "clock", "time":
		return "clock"
	case "tags", "tag_list":
		return "tags"
	case "note", "text":
		return "note"
	case "derived":
		return "derived"
	default:
		return ""
	}
}

func statusNormalizeEventKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "set", "change", "reaffirm", "reversal", "recovery", "correction", "reveal", "resolve", "uncertain", "clear", "event_observed", "increase", "decrease", "effect_applied", "effect_expired", "effect_cleared":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusNormalizeEffectKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "temporary", "temporary_effect":
		return "temporary_effect"
	case "buff", "debuff", "injury", "cooldown":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusNormalizeEffectState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending", "active", "expired", "cleared":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusNormalizeOptionalEffectState(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	state := statusNormalizeEffectState(raw)
	if state == "" {
		return "", errors.New("effect_state must be one of pending, active, expired, cleared")
	}
	return state, nil
}

func statusNormalizeAuthorityMode(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto", nil
	case "archive", "canonical", "archive_canonical", "archive-current", "archive_current":
		return "archive_canonical", nil
	case "external", "external_runtime", "runtime", "lua", "lua_runtime":
		return "external_runtime", nil
	default:
		return "", errors.New("authority_mode must be one of auto, archive_canonical, external_runtime")
	}
}

func statusNormalizeProjectionDensity(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto", nil
	case "full":
		return "full", nil
	case "light", "tag":
		return "light", nil
	default:
		return "", errors.New("projection_density must be one of auto, full, light")
	}
}

func statusSchemaOptionalOwnerScope(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	scope := statusSchemaNormalizeOwnerScope(raw)
	if scope == "" {
		return "", errors.New("owner_scope is invalid")
	}
	return scope, nil
}

func statusSchemaNormalizeOptionalRegistryState(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	switch raw {
	case "active", "deprecated", "disabled":
		return raw, nil
	default:
		return "", errors.New("registry_state must be one of active, deprecated, disabled")
	}
}

func statusSchemaNormalizeInputChannel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "bootstrap", "schema_bootstrap":
		return "bootstrap"
	case "direct", "direct_json", "settings", "settings_json":
		return "direct_json"
	case "import", "portable_import", "schema_import":
		return "portable_import"
	default:
		return ""
	}
}

func statusSchemaNormalizeOptionalProposalState(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw == "pending" {
		raw = "pending_review"
	}
	switch raw {
	case "pending_review", "approved", "rejected", "needs_revision":
		return raw, nil
	default:
		return "", errors.New("proposal_state must be one of pending_review, approved, rejected, needs_revision")
	}
}

func statusSchemaNormalizeReviewState(raw string) (string, error) {
	state, err := statusSchemaNormalizeOptionalProposalState(raw)
	if err != nil {
		return "", err
	}
	if state == "" || state == "pending_review" {
		return "", errors.New("proposal_state must be one of approved, rejected, needs_revision")
	}
	return state, nil
}

func statusSchemaCompactJSONObject(raw json.RawMessage, field string) (string, error) {
	compact, err := statusSchemaCompactRawJSON(raw, field)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(compact), &obj); err != nil || obj == nil {
		return "", errors.New(field + " must be a JSON object")
	}
	return compact, nil
}

func statusSchemaCompactOptionalJSONObject(raw json.RawMessage, field string) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	return statusSchemaCompactJSONObject(raw, field)
}

func statusSchemaCompactOptionalRawJSON(raw json.RawMessage, field string) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	if !json.Valid(trimmed) {
		return "", errors.New(field + " must be valid JSON")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return "", errors.New(field + " must be valid JSON")
	}
	return buf.String(), nil
}

func statusSchemaCompactOptionalValue(raw any, field string) (string, error) {
	if !hasMeaningfulPayload(raw) {
		return "", nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", errors.New(field + " must be JSON serializable")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return "", errors.New(field + " must be valid JSON")
	}
	return buf.String(), nil
}

func statusSchemaCompactRawJSON(raw json.RawMessage, field string) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", errors.New(field + " is required")
	}
	if !json.Valid(trimmed) {
		return "", errors.New(field + " must be valid JSON")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return "", errors.New(field + " must be valid JSON")
	}
	return buf.String(), nil
}

func firstNonEmptyQuery(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyStringLocal(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func statusSchemaBoundedLimit(raw string, fallback, minValue, maxValue int) int {
	value := fallback
	if strings.TrimSpace(raw) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			value = parsed
		}
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
