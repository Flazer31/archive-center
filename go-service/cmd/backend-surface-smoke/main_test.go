package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRunSurfaceSmoke(t *testing.T) {
	rep := runSurfaceSmoke("test-session-42")

	if rep.Status != "ok" {
		t.Fatalf("report status=%s (expected ok when all routes present)", rep.Status)
	}

	if rep.CheckedAt == "" {
		t.Error("expected CheckedAt to be populated")
	}

	expectedPhases := map[string]int{
		"1-A": 0,
		"1-B": 0,
		"1-C": 0,
		"1-D": 0,
		"2-A": 0,
		"2-B": 0,
		"3":   0,
		"4":   0,
	}

	for _, c := range rep.Checks {
		if _, ok := expectedPhases[c.Phase]; !ok {
			t.Errorf("unexpected phase %q", c.Phase)
			continue
		}
		expectedPhases[c.Phase]++

		if !strings.HasPrefix(c.Status, "present") && c.Status != "present_no_content" {
			t.Errorf("phase %s route %s: expected present-ish status, got %q (http=%d)", c.Phase, c.Route, c.Status, c.HTTPStatus)
		}

		if c.HTTPStatus != http.StatusOK && c.HTTPStatus != http.StatusNoContent && !isAllowedHTTPStatus(c.HTTPStatus, c.AllowedHTTPStatuses) {
			t.Errorf("phase %s route %s: expected 200, 204, or an explicitly allowed status, got %d", c.Phase, c.Route, c.HTTPStatus)
		}

		if !c.JSONValid && c.HTTPStatus != http.StatusNoContent {
			t.Errorf("phase %s route %s: expected valid JSON, got invalid", c.Phase, c.Route)
		}

		if len(c.TopLevelKeys) == 0 && c.HTTPStatus != http.StatusNoContent {
			t.Errorf("phase %s route %s: expected at least one top-level JSON key", c.Phase, c.Route)
		}
	}

	for phase, count := range expectedPhases {
		if count == 0 {
			t.Errorf("expected at least one check for phase %s", phase)
		}
	}

	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON report")
	}
}
