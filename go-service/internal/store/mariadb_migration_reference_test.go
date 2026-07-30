package store

import (
	"context"
	"errors"
	"testing"
)

func TestCleanupSessionMigrationSourceRequiresEnabledMariaDBStore(t *testing.T) {
	m := &mariadbStore{}
	result, err := m.CleanupSessionMigrationSource(context.Background(), 7, "must not delete")
	if result != nil {
		t.Fatalf("cleanup result = %+v, want nil", result)
	}
	if !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("cleanup error = %v, want %v", err, ErrNotEnabled)
	}
}
