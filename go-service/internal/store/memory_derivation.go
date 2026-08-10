package store

import (
	"context"
	"errors"
	"time"
)

const (
	MemoryDerivationDependencyContract = "memory_derivation_dependency.v1"
	MemoryReprocessingJobContract      = "memory_reprocessing_job.v1"
	MemorySourceRevisionContract       = "memory_source_revision.v1"
	MemoryVectorOutboxContract         = "memory_vector_outbox.v1"
)

var (
	ErrSourceRevisionConflict   = errors.New("source revision conflicts with the active logical turn")
	ErrSourceRevisionStale      = errors.New("source revision is no longer active")
	ErrLeaseExpired             = errors.New("work lease is expired or no longer owned")
	ErrMemoryReprocessingLeased = errors.New("memory reprocessing job has an active lease")
)

// MemorySourceRevision is the durable Host-observed raw turn pair. BranchID is
// nullable because source_acceptance_observation.v1 does not expose branch
// identity; callers must not borrow it from another lifecycle contract.
type MemorySourceRevision struct {
	ID                           int64
	ContractVersion              string
	SourceRevision               string
	ChatSessionID                string
	LogicalTurnID                string
	TurnIndex                    int
	SourceMessageID              string
	SourceGenerationID           string
	BranchID                     string
	BranchState                  string
	UserContent                  string
	AssistantContent             string
	CombinedContentHash          string
	UserObservedContentHash      string
	AssistantObservedContentHash string
	HashAlgorithm                string
	HostObservedAtMS             int64
	LifecycleState               string
	SupersededByRevision         string
	InvalidationReason           string
	DerivedAdmissionState        string
	DerivedAdmissionVersion      string
	DerivedExtractorVersion      string
	DerivedIndexVersion          string
	DerivedResultHash            string
	DerivedResultJSON            string
	DerivedAdmittedAt            time.Time
	CriticInputSnapshotJSON      string
	CriticInputSnapshotHash      string
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

type SourceRevisionRegistration struct {
	Inserted   bool
	Idempotent bool
}

// SourceRevisionStore owns accepted-source registration and the durable
// compare-at-commit fence used by derived writers and background workers.
type SourceRevisionStore interface {
	RegisterAcceptedSourceRevision(context.Context, *MemorySourceRevision) (SourceRevisionRegistration, error)
	GetSourceRevision(ctx context.Context, chatSessionID, sourceRevision string) (*MemorySourceRevision, error)
	IsSourceRevisionActive(ctx context.Context, chatSessionID, sourceRevision string) (bool, error)
	InvalidateSourceRevisions(ctx context.Context, chatSessionID string, fromTurn int, lifecycleState, reason string, invalidatedAt time.Time) error
}

// CriticInputSnapshotStore preserves the exact bounded dynamic input selected
// for an accepted source revision. Reprocessing reads this snapshot instead of
// rebuilding context from mutable session state.
type CriticInputSnapshotStore interface {
	SaveCriticInputSnapshot(
		context.Context,
		string,
		string,
		string,
		string,
		time.Time,
	) error
}

type MemoryDerivationLifecycleAvailability interface {
	MemoryDerivationLifecycleEnabled() bool
}

type ActiveSourceRevisionLister interface {
	ListActiveSourceRevisions(
		context.Context,
		string,
		int,
		int,
	) ([]MemorySourceRevision, error)
}

type MemoryDerivationDependency struct {
	ID                 int64
	ContractVersion    string
	ChatSessionID      string
	SourceRevision     string
	RootSourcePointer  string
	ChildArtifactType  string
	ChildArtifactID    string
	ParentArtifactType string
	ParentArtifactID   string
	DerivationVersion  string
	ExtractorVersion   string
	IndexVersion       string
	LifecycleState     string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type MemoryReprocessingJob struct {
	ID                int64
	ContractVersion   string
	IdempotencyKey    string
	ChatSessionID     string
	SourceRevision    string
	SourceContract    string
	DerivationVersion string
	ExtractorVersion  string
	IndexVersion      string
	Status            string
	Attempts          int
	RetryAfter        time.Time
	LeaseOwner        string
	LeaseUntil        time.Time
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type MemoryReprocessingJobStore interface {
	EnqueueMemoryReprocessingJob(context.Context, *MemoryReprocessingJob) (inserted bool, err error)
	ClaimMemoryReprocessingJob(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*MemoryReprocessingJob, error)
	CompleteMemoryReprocessingJob(ctx context.Context, jobID int64, leaseOwner string, now time.Time) error
	FailMemoryReprocessingJob(ctx context.Context, jobID int64, leaseOwner string, now, retryAfter time.Time, permanent bool, failure string) error
}

// MemoryReprocessingJobReopener is an optional administrative capability. It
// reopens the exact idempotent job and resets only its active source revision's
// committed admission snapshot. Raw source content and projected secondary
// rows remain untouched for operator-reviewed recovery.
type MemoryReprocessingJobReopener interface {
	ReopenMemoryReprocessingJob(
		context.Context,
		string,
		string,
		string,
		time.Time,
	) (reopened bool, err error)
}

type MemoryVectorOutboxItem struct {
	ID                  int64
	ContractVersion     string
	OperationKey        string
	Operation           string
	ChatSessionID       string
	SourceRevision      string
	DocumentID          string
	DocumentJSON        string
	EmbeddingReady      bool
	RequiredSourceState string
	Status              string
	Attempts            int
	RetryAfter          time.Time
	LeaseOwner          string
	LeaseUntil          time.Time
	LastError           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type MemoryVectorOutboxStore interface {
	EnqueueMemoryVectorOperation(context.Context, *MemoryVectorOutboxItem) (inserted bool, err error)
	ClaimMemoryVectorOperations(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) ([]*MemoryVectorOutboxItem, error)
	CompleteMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now time.Time) error
	FailMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now, retryAfter time.Time, permanent bool, failure string) error
}
