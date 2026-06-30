package dto

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 1. Invalid JSON
// ---------------------------------------------------------------------------

func TestDecodeWithDefaults_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{not valid json`)
	var req ArcGenerateRequest
	err := DecodeWithDefaults(body, &req)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Logf("error message: %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 2. Empty body ({})
// ---------------------------------------------------------------------------

func TestDecodeWithDefaults_EmptyBody(t *testing.T) {
	body := strings.NewReader(`{}`)
	var req ArcGenerateRequest
	if err := DecodeWithDefaults(body, &req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After DecodeWithDefaults, ApplyDefaults should have been called.
	if req.ChatSessionID == nil || *req.ChatSessionID != "" {
		t.Errorf("ChatSessionID expected default empty string, got %v", req.ChatSessionID)
	}
	if req.Force == nil || *req.Force != false {
		t.Errorf("Force expected default false, got %v", req.Force)
	}
	if req.FromTurn == nil || *req.FromTurn != 0 {
		t.Errorf("FromTurn expected default 0, got %v", req.FromTurn)
	}
	if req.ToTurn == nil || *req.ToTurn != 0 {
		t.Errorf("ToTurn expected default 0, got %v", req.ToTurn)
	}
}

// ---------------------------------------------------------------------------
// 3. Explicit zero preserved
// ---------------------------------------------------------------------------

func TestDecodeWithDefaults_ExplicitZeroPreserved(t *testing.T) {
	body := strings.NewReader(`{"from_turn":0,"to_turn":10}`)
	var req ArcGenerateRequest
	if err := DecodeWithDefaults(body, &req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// from_turn was explicitly 0 in JSON: must remain &0, not be replaced.
	if req.FromTurn == nil || *req.FromTurn != 0 {
		t.Errorf("explicit zero for FromTurn should be preserved, got %v", req.FromTurn)
	}
	// to_turn was explicitly 10 in JSON: must remain &10.
	if req.ToTurn == nil || *req.ToTurn != 10 {
		t.Errorf("explicit 10 for ToTurn should be preserved, got %v", req.ToTurn)
	}
	// ChatSessionID and Force were absent: must be defaulted.
	if req.ChatSessionID == nil || *req.ChatSessionID != "" {
		t.Errorf("ChatSessionID expected default empty string, got %v", req.ChatSessionID)
	}
	if req.Force == nil || *req.Force != false {
		t.Errorf("Force expected default false, got %v", req.Force)
	}
}

// ---------------------------------------------------------------------------
// 4. Nil target guard
// ---------------------------------------------------------------------------

func TestDecodeWithDefaults_NilTarget(t *testing.T) {
	body := strings.NewReader(`{}`)
	err := DecodeWithDefaults(body, nil)
	if err == nil {
		t.Fatal("expected error for nil target, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("error should mention nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 5. Non-ApplyDefaulter type (no panic)
// ---------------------------------------------------------------------------

func TestDecodeWithDefaults_NonDefaulter(t *testing.T) {
	body := strings.NewReader(`{"active_scope":"global"}`)
	var req ActiveScopeRequest // no ApplyDefaults method
	if err := DecodeWithDefaults(body, &req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ActiveScope != "global" {
		t.Errorf("ActiveScope = %q, want %q", req.ActiveScope, "global")
	}
}

// ---------------------------------------------------------------------------
// 6. Empty reader
// ---------------------------------------------------------------------------

func TestDecodeWithDefaults_EmptyReader(t *testing.T) {
	body := bytes.NewReader(nil)
	var req ArcGenerateRequest
	err := DecodeWithDefaults(body, &req)
	if err == nil {
		t.Fatal("expected error for empty reader, got nil")
	}
}
