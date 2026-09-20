package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Settings describe an optional fictional model for automatically found women. Observed body facts
// remain in the existing source-bound state/history owner.
type bodyTrackingConfig struct {
	CycleTrackingEnabled      bool                  `json:"cycle_tracking_enabled"`
	AutomaticPregnancyEnabled bool                  `json:"automatic_pregnancy_enabled"`
	Characters                []bodyCharacterConfig `json:"characters"`
	SimulationSeed            string                `json:"simulation_seed,omitempty"`
}

type bodyCharacterConfig struct {
	EntityID        string        `json:"entity_id"`
	OriginEntityID  string        `json:"origin_entity_id,omitempty"`
	CharacterName   string        `json:"character_name"`
	Species         string        `json:"species"`
	WorldRule       string        `json:"world_rule"`
	CycleEnabled    bool          `json:"cycle_enabled"`
	CanConceive     bool          `json:"can_conceive"`
	Cycle           bodyCycleSpec `json:"cycle"`
	CycleViability  float64       `json:"cycle_viability"`
	ConditionalPeak float64       `json:"conditional_peak"`
	GestationDays   int           `json:"gestation_days"`
}

func defaultBodyCharacterConfig() bodyCharacterConfig {
	return bodyCharacterConfig{
		Species: "human", CycleEnabled: true, CanConceive: true,
		Cycle:          bodyCycleSpec{ReferenceTime: map[string]any{}, ReferenceKind: "author_setting", CycleDays: 28, VariationDays: 2, PeriodDays: 5, LutealMinDays: 10, LutealMaxDays: 16},
		CycleViability: .4, ConditionalPeak: .7, GestationDays: 266,
	}
}

// Missing fields use the editable template. Explicit zero/invalid model values
// are retained; the calculator reports uncertainty instead of rejecting a save.
func (c *bodyCharacterConfig) UnmarshalJSON(data []byte) error {
	type wire bodyCharacterConfig
	next := wire(defaultBodyCharacterConfig())
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}
	*c = bodyCharacterConfig(next)
	return nil
}

type bodyTrackingSettingsFile struct {
	ContractVersion string                        `json:"contract_version"`
	Sessions        map[string]bodyTrackingConfig `json:"sessions"`
}

func bodyTrackingSettingsPath() (string, error) {
	path, err := multiAgentSettingsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "body-tracking.json"), nil
}

func readBodyTrackingSettings() (bodyTrackingSettingsFile, error) {
	out := bodyTrackingSettingsFile{ContractVersion: "body_tracking_settings.v1", Sessions: map[string]bodyTrackingConfig{}}
	path, err := bodyTrackingSettingsPath()
	if err != nil {
		return out, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	if out.Sessions == nil {
		out.Sessions = map[string]bodyTrackingConfig{}
	}
	return out, nil
}

func writeBodyTrackingSettings(settings bodyTrackingSettingsFile) error {
	path, err := bodyTrackingSettingsPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".body-tracking-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = json.NewEncoder(tmp).Encode(settings); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (s *Server) storedBodyTrackingConfig(sid string) (bodyTrackingConfig, bool, error) {
	s.RuntimeConfigMu.RLock()
	defer s.RuntimeConfigMu.RUnlock()
	settings, err := readBodyTrackingSettings()
	c, exists := settings.Sessions[sid]
	if c.Characters == nil {
		c.Characters = []bodyCharacterConfig{}
	}
	return c, exists, err
}

func (s *Server) loadBodyTrackingConfig(sid string) (bodyTrackingConfig, error) {
	c, _, err := s.storedBodyTrackingConfig(sid)
	return c, err
}

// Character snapshots own gender/species; the settings file holds model parameters,
// not a manual membership list. Neither age nor a human menopause cutoff is read.
func (s *Server) resolveBodyTrackingConfig(ctx context.Context, sid string, c bodyTrackingConfig) bodyTrackingConfig {
	configured := map[string]bodyCharacterConfig{}
	for _, character := range c.Characters {
		configured[character.EntityID] = character
	}
	roster := s.bodyTrackingRoster(ctx, sid, c)
	deleted := map[string]bool{}
	if values, err := s.bodyTrackingCurrentValues(ctx, sid); err == nil {
		for _, value := range values {
			deleted[value.OwnerID] = parseJSONMap(value.ValueJSON)["deleted"] == true
		}
	}
	c.Characters = []bodyCharacterConfig{}
	if s.Store == nil {
		return c
	}
	states, err := s.Store.ListCharacterStates(ctx, sid)
	if err != nil {
		return c
	}
	byName := map[string]store.CharacterState{}
	for _, state := range states {
		if state.ChatSessionID != "" && state.ChatSessionID != sid {
			continue
		}
		name := s.canonicalCharacterName(ctx, sid, state.CharacterName)
		key := comparableEntityKey(name)
		if prior, exists := byName[key]; !exists || state.TurnIndex > prior.TurnIndex {
			byName[key] = state
		}
	}
	for _, person := range roster {
		if deleted[stringFromMap(person, "entity_id")] {
			continue
		}
		state := byName[comparableEntityKey(stringFromMap(person, "character_name"))]
		appearance := parseJSONMap(state.AppearanceJSON)
		if strings.ToLower(strings.TrimSpace(stringFromMap(appearance, "gender"))) != "female" {
			continue
		}
		id := stringFromMap(person, "entity_id")
		character, exists := configured[id]
		if !exists {
			character = defaultBodyCharacterConfig()
			character.EntityID, character.OriginEntityID = id, id
			character.Species = stringFromMap(appearance, "species")
		}
		character.CharacterName = stringFromMap(person, "character_name")
		c.Characters = append(c.Characters, character)
	}
	return c
}

func (s *Server) effectiveBodyTrackingConfig(ctx context.Context, sid string) (bodyTrackingConfig, error) {
	c, err := s.loadBodyTrackingConfig(sid)
	if err != nil || !c.CycleTrackingEnabled && !c.AutomaticPregnancyEnabled {
		return c, err
	}
	return s.resolveBodyTrackingConfig(ctx, sid, c), nil
}

// Called by explicit settings saves and accepted-source projection, never by a
// read. The independently drawn initial phase is a model value, not period evidence.
func (s *Server) initializeAutomaticBodyTracking(ctx context.Context, sid string) (bodyTrackingConfig, error) {
	stored, err := s.loadBodyTrackingConfig(sid)
	if err != nil || !stored.CycleTrackingEnabled && !stored.AutomaticPregnancyEnabled {
		return stored, err
	}
	if stored.SimulationSeed == "" {
		stored, err = s.saveBodyTrackingConfig(sid, stored)
		if err != nil {
			return stored, err
		}
	}
	effective := s.resolveBodyTrackingConfig(ctx, sid, stored)
	var clockValues []store.StatusCurrentValue
	if reader, ok := s.Store.(store.StatusCurrentValueStore); ok {
		clockValues, _ = reader.ListStatusCurrentValues(ctx, sid, storyClockOwnerScope, storyClockOwnerID, storyClockStatusKey, 1)
	}
	anchor := bodyCycleDayCoordinate(resolveCurrentStoryClock(nil, nil, nil, clockValues))
	bodyValues, _ := s.bodyTrackingCurrentValues(ctx, sid)
	before := mustCompactJSON(stored)
	positions := map[string]int{}
	for index, character := range stored.Characters {
		positions[character.EntityID] = index
	}
	for index := range effective.Characters {
		character := &effective.Characters[index]
		if len(character.Cycle.ReferenceTime) == 0 {
			observedReference := false
			for _, value := range bodyValues {
				if value.OwnerID == character.EntityID && len(mapFromAny(parseJSONMap(value.ValueJSON)["cycle_reference"])) > 0 {
					observedReference = true
				}
			}
			span := storyTimeBounds(anchor)
			if !observedReference && span.valid && span.minimum == span.maximum && character.Cycle.CycleDays > 0 {
				origin := extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
				phase, _ := bodyPregnancyInteger(stored.SimulationSeed, origin, "automatic_initial_cycle.v1", 0, "initial_phase", 0, character.Cycle.CycleDays-1)
				character.Cycle.ReferenceTime = storyTimeRelative(anchor, map[string]any{"anchor": "source_observation", "offset": -phase, "unit": "day"})
				character.Cycle.ReferenceKind = "model_initialization"
			}
		}
		if position, exists := positions[character.EntityID]; exists {
			stored.Characters[position] = *character
		} else {
			positions[character.EntityID] = len(stored.Characters)
			stored.Characters = append(stored.Characters, *character)
		}
	}
	if before != mustCompactJSON(stored) {
		if _, err = s.saveBodyTrackingConfig(sid, stored); err != nil {
			return effective, err
		}
	}
	return effective, nil
}

func (s *Server) saveBodyTrackingConfig(sid string, c bodyTrackingConfig) (bodyTrackingConfig, error) {
	s.RuntimeConfigMu.Lock()
	defer s.RuntimeConfigMu.Unlock()
	settings, err := readBodyTrackingSettings()
	if err != nil {
		return c, err
	}
	c.SimulationSeed = settings.Sessions[sid].SimulationSeed
	origins := map[string]string{}
	for _, previous := range settings.Sessions[sid].Characters {
		origins[previous.EntityID] = extractionFirstNonEmpty(previous.OriginEntityID, previous.EntityID)
	}
	for index := range c.Characters {
		c.Characters[index].OriginEntityID = extractionFirstNonEmpty(origins[c.Characters[index].EntityID], c.Characters[index].EntityID)
	}
	// Parameters of an absent/currently non-female character survive source
	// rollback and OFF. Effective membership is read from current character state.
	incoming := map[string]bool{}
	for _, character := range c.Characters {
		incoming[character.EntityID] = true
	}
	for _, previous := range settings.Sessions[sid].Characters {
		if !incoming[previous.EntityID] {
			c.Characters = append(c.Characters, previous)
		}
	}
	if c.SimulationSeed == "" {
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return c, err
		}
		c.SimulationSeed = hex.EncodeToString(seed)
	}
	if c.Characters == nil {
		c.Characters = []bodyCharacterConfig{}
	}
	settings.Sessions[sid] = c
	return c, writeBodyTrackingSettings(settings)
}

// Copy/import preserve the original deterministic model seed, unlike an
// explicit settings save which never accepts a replacement seed from the UI.
func (s *Server) restoreBodyTrackingConfig(sid string, c bodyTrackingConfig) error {
	s.RuntimeConfigMu.Lock()
	defer s.RuntimeConfigMu.Unlock()
	settings, err := readBodyTrackingSettings()
	if err != nil {
		return err
	}
	settings.Sessions[sid] = c
	return writeBodyTrackingSettings(settings)
}

func (s *Server) copyBodyTrackingConfig(sourceID, targetID string, entityIDs map[string]string) (bool, error) {
	s.RuntimeConfigMu.Lock()
	defer s.RuntimeConfigMu.Unlock()
	settings, err := readBodyTrackingSettings()
	if err != nil {
		return false, err
	}
	c, exists := settings.Sessions[sourceID]
	if !exists {
		return false, nil
	}
	c.Characters = append([]bodyCharacterConfig(nil), c.Characters...)
	for index := range c.Characters {
		character := &c.Characters[index]
		character.OriginEntityID = extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
		if mapped := entityIDs[character.EntityID]; mapped != "" {
			character.EntityID = mapped
		}
	}
	settings.Sessions[targetID] = c
	return true, writeBodyTrackingSettings(settings)
}

func (s *Server) bodyTrackingRoster(ctx context.Context, sid string, config bodyTrackingConfig) []map[string]any {
	byID := map[string]map[string]any{}
	if s.Store != nil {
		if reader, ok := s.Store.(store.EntityIdentityCatalogReader); ok {
			if identities, err := reader.ListActiveEntityIdentities(ctx, sid); err == nil {
				for _, item := range identities {
					if item.ChatSessionID == sid && item.EntityKind == "character" {
						id := s.characterIdentityRoot(ctx, sid, item.StableEntityID)
						if _, exists := byID[id]; !exists || id == item.StableEntityID {
							byID[id] = map[string]any{"entity_id": id, "character_name": item.CanonicalLabel}
						}
					}
				}
			}
		}
	}
	for _, item := range config.Characters {
		if _, exists := byID[item.EntityID]; !exists {
			byID[item.EntityID] = map[string]any{"entity_id": item.EntityID, "character_name": item.CharacterName}
		}
	}
	items := make([]map[string]any, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := stringFromMap(items[i], "character_name"), stringFromMap(items[j], "character_name")
		if a == b {
			return stringFromMap(items[i], "entity_id") < stringFromMap(items[j], "entity_id")
		}
		return a < b
	})
	return items
}

// Settings-only diagnostics read the saved trial. They never sample a new trial
// or add evidence, probabilities, or internal random draws to the AI reading.
func bodyTrackingExposureSettingsReading(projection, clock map[string]any) map[string]any {
	observation := mapFromAny(mapFromAny(projection["observed_facts"])["conception_exposure"])
	latest := mapFromAny(projection["latest_model_result"])
	out := map[string]any{"recorded": len(observation) > 0, "status": "not_recorded"}
	if len(observation) == 0 {
		return out
	}
	out["status"] = "not_evaluated"
	out["occurrence_time"] = observation["occurred_at"]
	out["current_relation"] = storyTimeRelation(mapFromAny(observation["occurred_at"]), clock)
	out["exposure"] = observation["exposure"]
	out["partners"] = observation["partners"]
	out["evidence_excerpt"] = mapFromAny(observation["source"])["evidence_excerpt"]
	out["source_turn"] = mapFromAny(observation["source"])["source_turn"]
	if len(latest) > 0 {
		out["status"], out["outcome"], out["reason"] = latest["status"], latest["outcome"], latest["reason"]
	}
	probabilities := []map[string]any{}
	for _, raw := range sliceFromAny(latest["cycles"]) {
		cycle := mapFromAny(raw)
		probabilities = append(probabilities, map[string]any{
			"ovulation_time": cycle["ovulation_time"],
			"probability":    cycle["single_day_probability_given_latent_ovulation"],
		})
	}
	out["probabilities"] = probabilities
	out["authority"] = "fiction_simulation"
	return out
}

func (s *Server) bodyTrackingSettingsView(ctx context.Context, sid string, c bodyTrackingConfig) map[string]any {
	stored := c
	c = s.resolveBodyTrackingConfig(ctx, sid, c)
	var values []store.StatusCurrentValue
	if reader, ok := s.Store.(store.StatusCurrentValueStore); ok {
		values, _ = reader.ListStatusCurrentValues(ctx, sid, storyClockOwnerScope, storyClockOwnerID, storyClockStatusKey, 1)
	}
	clock := resolveCurrentStoryClock(nil, nil, nil, values)
	bodyValues, _ := s.bodyTrackingCurrentValues(ctx, sid)
	currentByEntity := map[string]store.StatusCurrentValue{}
	for _, current := range bodyValues {
		currentByEntity[current.OwnerID] = current
	}
	bodyStates := make([]map[string]any, 0, len(c.Characters))
	modelReadings := make([]map[string]any, 0, len(c.Characters))
	exposureReadings := make([]map[string]any, 0, len(c.Characters))
	estimates := make([]map[string]any, 0, len(c.Characters))
	for _, character := range c.Characters {
		exposureReadings = append(exposureReadings, map[string]any{"entity_id": character.EntityID, "reading": bodyTrackingExposureSettingsReading(parseJSONMap(currentByEntity[character.EntityID].ValueJSON), clock)})
		if current, exists := currentByEntity[character.EntityID]; exists {
			projection := bodyTrackingTermProjection(character, parseJSONMap(current.ValueJSON))
			bodyStates = append(bodyStates, map[string]any{"entity_id": character.EntityID, "state": map[string]any{"observed_facts": projection["observed_facts"], "pregnancy": projection["pregnancy"], "recovery": projection["recovery"], "paternity": bodyTrackingPaternityReading(projection, clock)}})
			if c.AutomaticPregnancyEnabled {
				if reading := bodyTrackingModelReading(projection, clock); len(reading) > 0 {
					modelReadings = append(modelReadings, map[string]any{"entity_id": character.EntityID, "reading": reading})
				}
			}
		}
		estimate := bodyTrackingCycleReading(character, parseJSONMap(currentByEntity[character.EntityID].ValueJSON), clock)
		estimates = append(estimates, map[string]any{
			"entity_id": character.EntityID, "character_name": character.CharacterName,
			"tracking_enabled": c.CycleTrackingEnabled && character.CycleEnabled, "estimate": estimate,
		})
	}
	c.SimulationSeed = ""
	return map[string]any{
		"status": "ok", "chat_session_id": sid, "settings": c,
		"roster": c.Characters, "default_character": defaultBodyCharacterConfig(),
		"target_mode": "automatic_female", "age_policy": "not_used",
		"additional_memory_budget_chars": map[bool]int{true: bodyTrackingAdditionalBudgetChars, false: 0}[c.CycleTrackingEnabled || c.AutomaticPregnancyEnabled],
		"last_confirmed_story_clock":     storyClockPromptProjection(clock), "cycle_estimates": estimates,
		"body_states":       bodyStates,
		"model_readings":    modelReadings,
		"exposure_readings": exposureReadings,
		"data_management":   s.bodyTrackingDataView(ctx, sid, stored, bodyValues),
		"model_note":        "Female characters are automatic targets, without human age or menopause cutoffs. Initial cycle phases are independently randomized model values, not observed facts.",
	}
}

func (s *Server) handleBodyTrackingSettings(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	var c bodyTrackingConfig
	var err error
	if r.Method == http.MethodPut {
		var payload json.RawMessage
		if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body_tracking_settings", "Settings must be a JSON object.")
			return
		}
		var envelope struct {
			RestoreSnapshot *struct {
				ContractVersion string             `json:"contract_version"`
				Config          bodyTrackingConfig `json:"config"`
			} `json:"restore_snapshot"`
		}
		if err = json.Unmarshal(payload, &envelope); err == nil && envelope.RestoreSnapshot != nil {
			c = envelope.RestoreSnapshot.Config
			err = s.restoreBodyTrackingConfig(sid, c)
		} else if err = json.Unmarshal(payload, &c); err == nil {
			c, err = s.saveBodyTrackingConfig(sid, c)
			if err == nil {
				c, err = s.initializeAutomaticBodyTracking(r.Context(), sid)
			}
		} else {
			writeError(w, http.StatusBadRequest, "invalid_body_tracking_settings", "Settings must be a JSON object.")
			return
		}
	} else {
		c, err = s.loadBodyTrackingConfig(sid)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "body_tracking_settings_unavailable", "Body tracking settings could not be read or saved.")
		return
	}
	writeJSON(w, http.StatusOK, s.bodyTrackingSettingsView(r.Context(), sid, c))
}
