package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryLifecycleTestStore struct {
	Store
	registerErr  error
	registered   int
	admissionErr error
	admissions   int
}

func (f *memoryLifecycleTestStore) MemoryDerivationLifecycleEnabled() bool { return true }
func (f *memoryLifecycleTestStore) MemoryAdmissionWritesEnabled() bool     { return true }

func (f *memoryLifecycleTestStore) CommitMemoryAdmission(context.Context, *MemoryAdmission) (MemoryAdmissionResult, error) {
	f.admissions++
	if f.admissionErr != nil {
		return MemoryAdmissionResult{}, f.admissionErr
	}
	return MemoryAdmissionResult{MemoryInserted: true}, nil
}

func (f *memoryLifecycleTestStore) RegisterAcceptedSourceRevision(context.Context, *MemorySourceRevision) (SourceRevisionRegistration, error) {
	f.registered++
	if f.registerErr != nil {
		return SourceRevisionRegistration{}, f.registerErr
	}
	return SourceRevisionRegistration{Inserted: true}, nil
}

func TestMemoryAdmissionDualWriteKeepsPrimaryAuthorityAndReportsShadowFailure(t *testing.T) {
	shadowFailure := errors.New("shadow admission failed")
	primary := &memoryLifecycleTestStore{Store: NewNoopStore()}
	shadow := &memoryLifecycleTestStore{Store: NewNoopStore(), admissionErr: shadowFailure}
	dual := NewDualWriteStore(primary, shadow)
	writer, ok := dual.(MemoryAdmissionWriter)
	if !ok {
		t.Fatal("dual store did not expose memory admission writer")
	}
	result, err := writer.CommitMemoryAdmission(context.Background(), &MemoryAdmission{})
	if err != nil || !result.MemoryInserted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	reporter := dual.(ShadowStatusReporter)
	failures, lastErr := reporter.ShadowStatus()
	if primary.admissions != 1 || shadow.admissions != 1 ||
		failures != 1 || !errors.Is(lastErr, shadowFailure) {
		t.Fatalf("primary=%d shadow=%d failures=%d last=%v",
			primary.admissions, shadow.admissions, failures, lastErr)
	}
}

func (f *memoryLifecycleTestStore) IsSourceRevisionActive(context.Context, string, string) (bool, error) {
	return true, nil
}

func (f *memoryLifecycleTestStore) GetSourceRevision(context.Context, string, string) (*MemorySourceRevision, error) {
	if f.registered == 0 {
		return nil, ErrNotFound
	}
	return &MemorySourceRevision{SourceRevision: "registered"}, nil
}

func (f *memoryLifecycleTestStore) InvalidateSourceRevisions(context.Context, string, int, string, string, time.Time) error {
	return nil
}

func TestMemoryDerivationLifecycleAvailabilityHonorsNoopReadOnlyAndShadow(t *testing.T) {
	noopDual := NewDualWriteStore(NewNoopStore(), NewNoopStore())
	availability, ok := noopDual.(MemoryDerivationLifecycleAvailability)
	if !ok || availability.MemoryDerivationLifecycleEnabled() {
		t.Fatalf("noop dual availability=%v ok=%v", availability, ok)
	}

	readOnly := NewReadOnlyStore(&memoryLifecycleTestStore{Store: NewNoopStore()})
	if _, ok := readOnly.(SourceRevisionStore); ok {
		t.Fatal("read-only store exposed source revision writes")
	}

	shadowFailure := errors.New("shadow source write failed")
	shadow := &memoryLifecycleTestStore{Store: NewNoopStore(), registerErr: shadowFailure}
	dual := NewDualWriteStore(NewNoopStore(), shadow)
	writer := dual.(SourceRevisionStore)
	result, err := writer.RegisterAcceptedSourceRevision(context.Background(), &MemorySourceRevision{})
	if err != nil || result.Inserted {
		t.Fatalf("shadow-only result=%+v err=%v", result, err)
	}
	reporter := dual.(ShadowStatusReporter)
	failures, lastErr := reporter.ShadowStatus()
	if failures != 1 || !errors.Is(lastErr, shadowFailure) || shadow.registered != 1 {
		t.Fatalf("failures=%d lastErr=%v registered=%d", failures, lastErr, shadow.registered)
	}
}
