package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBodyTrackingGlobalMigrationIndependentUnionAndExactBackup(t *testing.T) {
	for _, raw := range []string{
		`{"contract_version":"body_tracking_settings.v1","sessions":{}}`,
		`{"sessions":{"a":{"cycle_tracking_enabled":false,"automatic_pregnancy_enabled":false}}}`,
		`{"sessions":{"a":{"cycle_tracking_enabled":true},"b":{"cycle_tracking_enabled":false}}}`,
		`{"sessions":{"a":{"automatic_pregnancy_enabled":true},"b":{}}}`,
		`{"sessions":{"a":{"cycle_tracking_enabled":true},"b":{"automatic_pregnancy_enabled":true},"last":{"cycle_tracking_enabled":false,"automatic_pregnancy_enabled":false}}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			var legacy bodyTrackingSettingsFile
			if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
				t.Fatal(err)
			}
			character := defaultBodyCharacterConfig()
			character.EntityID, character.OriginEntityID = "model-id", "origin-id"
			character.Cycle.ReferenceTime = map[string]any{"date": "1423-01-01"}
			legacy.Sessions["model"] = bodyTrackingConfig{Characters: []bodyCharacterConfig{character}, SimulationSeed: "original-seed"}
			original, err := json.MarshalIndent(legacy, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			path, _ := bodyTrackingSettingsPath()
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			cycle, pregnancy := 0, 0
			for _, c := range legacy.Sessions {
				if c.CycleTrackingEnabled {
					cycle++
				}
				if c.AutomaticPregnancyEnabled {
					pregnancy++
				}
			}
			s := &Server{}
			view := bodySettingsRequest46(t, s, "new-session", http.MethodGet, "")
			settings := mapFromAny(view["settings"])
			if settings["cycle_tracking_enabled"] != (cycle > 0) || settings["automatic_pregnancy_enabled"] != (pregnancy > 0) {
				t.Fatalf("independent legacy union lost: %#v", settings)
			}
			migrated, err := readBodyTrackingSettings()
			if err != nil {
				t.Fatal(err)
			}
			m := migrated.Migration
			if migrated.ContractVersion != "body_tracking_settings.v2" || m == nil || m.Rule != "any_session_on_per_toggle" || m.SessionCount != len(legacy.Sessions) || m.CycleOnSessionCount != cycle || m.PregnancyOnSessionCount != pregnancy {
				t.Fatalf("missing migration evidence: %+v", migrated)
			}
			backup, err := os.ReadFile(filepath.Join(filepath.Dir(path), m.BackupPath))
			if err != nil || !bytes.Equal(backup, original) {
				t.Fatal("original backup differs", err)
			}
			for sid, before := range legacy.Sessions {
				after := migrated.Sessions[sid]
				if !reflect.DeepEqual(before.Characters, after.Characters) || before.SimulationSeed != after.SimulationSeed {
					t.Fatal("migration changed session model", sid)
				}
			}
			persisted, _ := os.ReadFile(path)
			var disk struct {
				Sessions map[string]map[string]any `json:"sessions"`
			}
			if err := json.Unmarshal(persisted, &disk); err != nil {
				t.Fatal(err)
			}
			for sid, c := range disk.Sessions {
				if _, exists := c["cycle_tracking_enabled"]; exists {
					t.Fatal("session cycle override retained", sid)
				}
				if _, exists := c["automatic_pregnancy_enabled"]; exists {
					t.Fatal("session pregnancy override retained", sid)
				}
			}
			bodySettingsRequest46(t, s, "new-session", http.MethodPut, `{"cycle_tracking_enabled":false,"automatic_pregnancy_enabled":false}`)
			for sid := range legacy.Sessions {
				c, err := (&Server{}).loadBodyTrackingConfig(sid)
				if err != nil || c.CycleTrackingEnabled || c.AutomaticPregnancyEnabled {
					t.Fatal("restart remigrated old ON", sid, err)
				}
			}
			again, _ := readBodyTrackingSettings()
			if !reflect.DeepEqual(m, again.Migration) {
				t.Fatal("migration evidence changed after save/restart")
			}
			backups, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "body-tracking.v1-*.bak"))
			if len(backups) != 1 {
				t.Fatal("migration repeated", backups)
			}
		})
	}
}

func TestBodyTrackingGlobalLegacyRoutesPreserveModelsAndState(t *testing.T) {
	s, st := automaticBodyFixture(t, 2)
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"cycle_tracking_enabled":true,"automatic_pregnancy_enabled":true}`)
	before, err := s.loadBodyTrackingConfig("auto")
	if err != nil {
		t.Fatal(err)
	}
	stateBefore := mustCompactJSON(st.returnStatusCurrent)
	for _, sid := range []string{"auto", "new-chat", "other-rp", "unconfigured-branch"} {
		view := bodySettingsRequest46(t, s, sid, http.MethodGet, "")
		c := mapFromAny(view["settings"])
		if c["cycle_tracking_enabled"] != true || c["automatic_pregnancy_enabled"] != true || view["toggle_scope"] != "global" {
			t.Fatal("common ON missing", sid, c)
		}
	}
	// An explicit save through an old session route turns both off everywhere.
	bodySettingsRequest46(t, s, "other-rp", http.MethodPut, `{"cycle_tracking_enabled":false,"automatic_pregnancy_enabled":false}`)
	// A delayed automatic model write and a stale exported snapshot must not
	// re-enable common preferences. Both exercise their production write owners.
	if _, err := s.saveBodyTrackingSettings("auto", before, false); err != nil {
		t.Fatal(err)
	}
	bodySettingsRequest46(t, s, "imported", http.MethodPut, mustCompactJSON(map[string]any{"restore_snapshot": map[string]any{"contract_version": "body_tracking_settings.v1", "config": before}}))
	if _, err := s.copyBodyTrackingConfig("auto", "copied", nil); err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"auto", "new-chat", "other-rp", "imported", "copied"} {
		c, err := s.effectiveBodyTrackingConfig(context.Background(), sid)
		if err != nil || c.CycleTrackingEnabled || c.AutomaticPregnancyEnabled {
			t.Fatal("common OFF lost", sid, err)
		}
	}
	after, _ := s.loadBodyTrackingConfig("auto")
	if !reflect.DeepEqual(before.Characters, after.Characters) || before.SimulationSeed != after.SimulationSeed {
		t.Fatal("toggle changed original models")
	}
	if stateBefore != mustCompactJSON(st.returnStatusCurrent) || len(st.savedStatusCurrent)+len(st.savedStatusEvents)+len(st.savedCharacterStates) != 0 {
		t.Fatal("toggle or model portability wrote story state")
	}
}
