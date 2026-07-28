package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryLifecycleTestStore struct {
	Store
	registerErr error
	registered  int
}

func (f *memoryLifecycleTestStore) MemoryDerivationLifecycleEnabled() bool { return true }

func (f *memoryLifecycleTestStore) RegisterAcceptedSourceRevision(context.Context, *MemorySourceRevision) (SourceRevisionRegistration, error) {
	f.registered++
	if f.registerErr != nil {
		return SourceRevisionRegistration{}, f.registerErr
	}
	return SourceRevisionRegistration{Inserted: true}, nil
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
