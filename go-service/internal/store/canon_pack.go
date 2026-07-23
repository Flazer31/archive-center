package store

import (
	"context"
	"time"
)

const CanonPackLifecycleContract = "canon_pack_lifecycle.v1"

type CanonPackInstallInput struct {
	ManifestJSON         []byte
	ManifestSHA256       string
	ArchiveSHA256        string
	ValidationReportJSON []byte
}

type CanonPackInstall struct {
	Contract          string         `json:"contract"`
	InstallID         string         `json:"install_id"`
	PackID            string         `json:"pack_id"`
	PackVersion       string         `json:"pack_version"`
	InstallGeneration uint64         `json:"install_generation"`
	WorkID            string         `json:"work_id"`
	EditionRowID      string         `json:"edition_row_id"`
	StableWorkID      string         `json:"stable_work_id"`
	EditionID         string         `json:"edition_id"`
	Title             string         `json:"title"`
	PackStatus        string         `json:"pack_status"`
	ReviewStatus      string         `json:"review_status"`
	TrustStatus       string         `json:"trust_status"`
	LifecycleStatus   string         `json:"lifecycle_status"`
	ManifestSHA256    string         `json:"manifest_sha256"`
	ArchiveSHA256     string         `json:"archive_sha256,omitempty"`
	CoverageReport    map[string]any `json:"coverage_report,omitempty"`
	RecordCounts      map[string]int `json:"record_counts,omitempty"`
	IndexStatus       string         `json:"index_status"`
	IndexResult       map[string]any `json:"index_result,omitempty"`
	InstalledAt       time.Time      `json:"installed_at"`
	ActivatedAt       *time.Time     `json:"activated_at,omitempty"`
	RemovedAt         *time.Time     `json:"removed_at,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type CanonPackLifecycleResult struct {
	Contract          string         `json:"contract"`
	Action            string         `json:"action"`
	InstallID         string         `json:"install_id"`
	LifecycleStatus   string         `json:"lifecycle_status"`
	PreviousInstallID string         `json:"previous_install_id,omitempty"`
	IndexStatus       string         `json:"index_status"`
	IndexResult       map[string]any `json:"index_result,omitempty"`
}

type CanonRegistryItem struct {
	InstallID       string         `json:"install_id"`
	PackID          string         `json:"pack_id"`
	PackVersion     string         `json:"pack_version"`
	LifecycleStatus string         `json:"lifecycle_status"`
	WorkID          string         `json:"work_id"`
	EditionRowID    string         `json:"edition_row_id"`
	StableWorkID    string         `json:"stable_work_id"`
	EditionID       string         `json:"edition_id"`
	Continuities    []string       `json:"continuities"`
	Title           string         `json:"title"`
	MatchedTitles   []string       `json:"matched_titles"`
	ReviewStatus    string         `json:"review_status"`
	TrustStatus     string         `json:"trust_status"`
	SourceCount     int            `json:"source_count"`
	ConflictCount   int            `json:"conflict_count"`
	UncertainCount  int            `json:"uncertain_count"`
	CoverageReport  map[string]any `json:"coverage_report"`
}

type CanonPackDiagnostics struct {
	Contract        string           `json:"contract"`
	InstallID       string           `json:"install_id"`
	LifecycleStatus string           `json:"lifecycle_status"`
	Sources         []map[string]any `json:"sources"`
	Conflicts       []any            `json:"conflicts"`
	Uncertainties   []any            `json:"uncertainties"`
	CoverageReport  map[string]any   `json:"coverage_report"`
	Quality         map[string]any   `json:"quality"`
}

type CanonOverlayInput struct {
	WorkID          string `json:"work_id"`
	EditionRowID    string `json:"edition_row_id"`
	TargetKind      string `json:"target_kind"`
	TargetID        string `json:"target_id"`
	Action          string `json:"action"`
	ReplacementKind string `json:"replacement_kind,omitempty"`
	ReplacementID   string `json:"replacement_id,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type CanonOverlayRule struct {
	OverlayRuleID       string    `json:"overlay_rule_id"`
	WorkID              string    `json:"work_id"`
	EditionRowID        string    `json:"edition_row_id"`
	TargetLogicalFactID string    `json:"target_logical_fact_id,omitempty"`
	TargetEntityID      string    `json:"target_entity_id,omitempty"`
	TargetNodeID        string    `json:"target_node_id,omitempty"`
	Action              string    `json:"action"`
	ReplacementClaimID  string    `json:"replacement_claim_id,omitempty"`
	ReplacementEntityID string    `json:"replacement_entity_id,omitempty"`
	ReplacementNodeID   string    `json:"replacement_node_id,omitempty"`
	Status              string    `json:"status"`
	Reason              string    `json:"reason,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CanonPackStore is optional because pack installation is available only when
// MariaDB is the authority. Shadow and fixture stores must not claim that a
// local pack was durably installed.
type CanonPackStore interface {
	InstallCanonPack(context.Context, CanonPackInstallInput) (*CanonPackInstall, error)
	ListCanonPackInstalls(context.Context, string) ([]CanonPackInstall, error)
	GetCanonPackInstall(context.Context, string) (*CanonPackInstall, error)
	SetCanonPackLifecycle(context.Context, string, string) (*CanonPackLifecycleResult, error)
}

// CanonRegistryStore owns pack catalog, diagnostics, and local overlay policy.
// It is intentionally separate from session memory storage.
type CanonRegistryStore interface {
	SearchCanonRegistry(context.Context, string, int) ([]CanonRegistryItem, error)
	GetCanonPackDiagnostics(context.Context, string) (*CanonPackDiagnostics, error)
	CreateCanonOverlay(context.Context, CanonOverlayInput) (*CanonOverlayRule, error)
	ListCanonOverlays(context.Context, string, string) ([]CanonOverlayRule, error)
}
