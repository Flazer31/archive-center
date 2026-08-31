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
	MemoryPublicProjectionIndex        = "memory_public_projection.v1"
)

var (
	ErrSourceRevisionConflict   = errors.New("source revision conflicts with the active logical turn")
	ErrSourceRevisionStale      = errors.New("source revision is no longer active")
	ErrLeaseExpired             = errors.New("work lease is expired or no longer owned")
	ErrMemoryReprocessingLeased = errors.New("memory reprocessing job has an active lease")
)

type memoryAdmissionVectorReplayContextKey struct{}

type memoryAdmissionVectorReplayOptions struct {
	Refresh              bool
	ReconcileEligibility bool
}

// WithMemoryAdmissionVectorReplay marks one canonical admission replay as a
// vector-maintenance pass. The option is request-local only: it does not add a
// second queue or persist administrative state. ReconcileEligibility lets the
// admission transaction cancel stale aggregate upserts and enqueue deletes;
// Refresh also reactivates the exact existing outbox operations.
func WithMemoryAdmissionVectorReplay(ctx context.Context, refresh, reconcileEligibility bool) context.Context {
	return context.WithValue(ctx, memoryAdmissionVectorReplayContextKey{}, memoryAdmissionVectorReplayOptions{
		Refresh:              refresh,
		ReconcileEligibility: reconcileEligibility,
	})
}

func memoryAdmissionVectorReplayFromContext(ctx context.Context) memoryAdmissionVectorReplayOptions {
	if ctx == nil {
		return memoryAdmissionVectorReplayOptions{}
	}
	options, _ := ctx.Value(memoryAdmissionVectorReplayContextKey{}).(memoryAdmissionVectorReplayOptions)
	return options
}

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

// SourceRevisionHistoryLister is the read-only recovery view of source
// revisions for one explicitly selected session.  Normal turn processing and
// rollback continue to use ActiveSourceRevisionLister; session normalization
// may additionally inspect inactive revisions so a deleted user side can be
// restored without another LLM call. Explicit branch-lineage repair may use
// the same bounded session history to recover an exact fork coordinate; it
// must not reactivate or rewrite any revision.
type SourceRevisionHistoryLister interface {
	ListSourceRevisions(
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

// MemoryReprocessingWakeScheduleStore exposes only the next durable wake time
// for the existing reprocessing queue. The worker uses it to restore its
// one-shot timer after a backend restart without polling or claiming work early.
type MemoryReprocessingWakeScheduleStore interface {
	NextMemoryReprocessingWakeAt(context.Context) (time.Time, error)
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

// MemoryVectorOutboxLaneStore lets the bounded authority worker reserve fair
// service for deletes without changing the canonical outbox contract used by
// other stores and tests.
type MemoryVectorOutboxLaneStore interface {
	ClaimMemoryVectorOperationsByOperation(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration, operation string) ([]*MemoryVectorOutboxItem, error)
}

// MemoryVectorMaterialization is the verified public memory vector that must
// converge into MariaDB before its outbox operation can be completed.
type MemoryVectorMaterialization struct {
	ChatSessionID  string
	SourceRevision string
	DocumentID     string
	SourceRowID    int64
	EmbeddingJSON  string
	EmbeddingModel string
}

type MemoryVectorMaterializedCompletionStore interface {
	CompleteMemoryVectorMaterializedOperation(
		ctx context.Context,
		outboxID int64,
		leaseOwner string,
		now time.Time,
		materialization MemoryVectorMaterialization,
	) error
}

type MemoryVectorOutboxMaintenanceStore interface {
	CoalesceInactiveMemoryVectorDeleteOperations(
		ctx context.Context,
		chatSessionID string,
		now time.Time,
	) (staleRejected int64, err error)
}
