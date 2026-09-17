package store

import (
	"context"
	"time"
)

const SourceDiscoveryContract = "source-discovery-pipeline.v1"

type SourceDiscoverySource struct {
	URL             string `json:"url"`
	SourceType      string `json:"source_type"`
	AccessClass     string `json:"access_class"`
	PolicyConfirmed bool   `json:"policy_confirmed"`
}

type SourceDiscoveryDomainPolicy struct {
	Domain          string `json:"domain"`
	SourceType      string `json:"source_type"`
	AccessClass     string `json:"access_class"`
	PolicyConfirmed bool   `json:"policy_confirmed"`
}

type SourceDiscoveryInput struct {
	WorkID             string                        `json:"work_id,omitempty"`
	ContinuityID       string                        `json:"continuity_id,omitempty"`
	WorkQuery          string                        `json:"work_query"`
	WorkTitle          string                        `json:"work_title,omitempty"`
	WorkType           string                        `json:"work_type,omitempty"`
	OriginalTitle      string                        `json:"original_title,omitempty"`
	Language           string                        `json:"language,omitempty"`
	EditionHint        string                        `json:"edition_hint,omitempty"`
	AllowedSourceTypes []string                      `json:"allowed_source_types"`
	RequestedDomains   []string                      `json:"requested_domains,omitempty"`
	DomainPolicies     []SourceDiscoveryDomainPolicy `json:"domain_policies,omitempty"`
	Sources            []SourceDiscoverySource       `json:"sources,omitempty"`
}

type SourceDiscoveryJob struct {
	Contract       string         `json:"contract"`
	JobID          string         `json:"job_id"`
	State          string         `json:"state"`
	Input          map[string]any `json:"input"`
	Result         map[string]any `json:"result"`
	CoverageReport map[string]any `json:"coverage_report"`
	Revision       uint64         `json:"revision"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type SourceDiscoveryStore interface {
	SaveSourceDiscoveryJob(context.Context, SourceDiscoveryInput, string, map[string]any, map[string]any) (*SourceDiscoveryJob, error)
	GetSourceDiscoveryJob(context.Context, string) (*SourceDiscoveryJob, error)
}

type SourceDiscoveryMutableStore interface {
	SourceDiscoveryStore
	UpdateSourceDiscoveryJob(context.Context, string, string, map[string]any, map[string]any) (*SourceDiscoveryJob, error)
}

type SourceDiscoveryQueryStore interface {
	SourceDiscoveryStore
	FindLatestSourceDiscoveryJob(context.Context, string, string) (*SourceDiscoveryJob, error)
}
