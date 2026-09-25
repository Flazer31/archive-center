package vector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IsIndexLoadError distinguishes the reported persisted-index failure from
// connection, authentication and configuration failures. Those are not repaired
// by replacing a collection.
func IsIndexLoadError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "error loading hnsw index")
}

// IndexRecovery keeps the old collection intact. Rebuilding happens in a new
// collection; only a completed rebuild is promoted. The local journal makes the
// two Chroma name changes resumable after a process interruption.
type IndexRecovery interface {
	RecoverIndex(context.Context, string, func(VectorStore) error) (string, error)
	ResumeIndexRecovery(context.Context, string) error
	RecoverySnapshot(context.Context, string) ([]VectorDocument, int, error)
}

func (s *chromaStore) RecoverySnapshot(ctx context.Context, persistDir string) ([]VectorDocument, int, error) {
	found, err := s.lookupRecoveryCollection(ctx, s.collectionName)
	if err != nil {
		return nil, 0, err
	}
	return ReadChromaRecoverySnapshot(ctx, persistDir, found.ID)
}

type chromaRecoveryJournal struct {
	Collection  string `json:"collection"`
	OriginalID  string `json:"original_id"`
	Candidate   string `json:"candidate"`
	CandidateID string `json:"candidate_id"`
	Backup      string `json:"backup"`
}

func (s *chromaStore) lookupRecoveryCollection(ctx context.Context, name string) (chromaCollection, error) {
	var found chromaCollection
	status, err := s.doJSON(ctx, http.MethodGet, s.collectionLookupPath(name), nil, &found, http.StatusOK, http.StatusNotFound)
	if err == nil && status == http.StatusNotFound {
		return found, ErrNotFound
	}
	return found, err
}

func (s *chromaStore) renameRecoveryCollection(ctx context.Context, id, name string) error {
	_, err := s.doJSON(ctx, http.MethodPut, strings.TrimSuffix(s.collectionOperationPath(id, ""), "/"),
		map[string]any{"new_name": name}, nil, http.StatusOK, http.StatusNoContent)
	return err
}

func (s *chromaStore) ResumeIndexRecovery(ctx context.Context, journalPath string) error {
	data, err := os.ReadFile(journalPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal chromaRecoveryJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return fmt.Errorf("read Chroma recovery journal: %w", err)
	}
	if journal.Collection != s.collectionName {
		return fmt.Errorf("Chroma recovery journal belongs to another collection")
	}
	current, err := s.lookupRecoveryCollection(ctx, journal.Collection)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if current.ID == journal.OriginalID {
		if err := s.renameRecoveryCollection(ctx, journal.OriginalID, journal.Backup); err != nil {
			return fmt.Errorf("preserve original Chroma collection: %w", err)
		}
	} else if current.ID != "" && current.ID != journal.CandidateID {
		return fmt.Errorf("Chroma collection changed during recovery; saved collections are unchanged")
	}
	if current.ID != journal.CandidateID {
		if err := s.renameRecoveryCollection(ctx, journal.CandidateID, journal.Collection); err != nil {
			return fmt.Errorf("promote recovered Chroma collection (resume on next start): %w", err)
		}
	}
	s.mu.Lock()
	s.collectionRef = journal.CandidateID
	s.mu.Unlock()
	if _, err := s.Health(ctx); err != nil {
		return fmt.Errorf("recovered Chroma collection health: %w", err)
	}
	return os.Remove(journalPath)
}

func (s *chromaStore) RecoverIndex(ctx context.Context, journalPath string, rebuild func(VectorStore) error) (string, error) {
	original, err := s.lookupRecoveryCollection(ctx, s.collectionName)
	if err != nil {
		return "", err
	}
	// Names are below Chroma's length limit even for a long configured name.
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	base := s.collectionName
	if len(base) > 100 {
		base = base[:100]
	}
	candidateName := base + "-recovery-" + stamp
	backupName := base + "-backup-" + stamp
	raw, err := NewChromaStoreWithHTTPClient(s.endpoint, candidateName, s.apiPath, s.client)
	if err != nil {
		return "", err
	}
	candidate := raw.(*chromaStore)
	var created chromaCollection
	body := map[string]any{"name": candidateName, "get_or_create": false}
	if len(original.Metadata) > 0 {
		body["metadata"] = original.Metadata
	}
	if len(original.Configuration) > 0 {
		body["configuration"] = original.Configuration
	}
	if _, err := s.doJSON(ctx, http.MethodPost, s.collectionListPath(), body, &created, http.StatusOK, http.StatusCreated); err != nil {
		return "", err
	}
	candidate.collectionRef = created.ID
	if _, err := candidate.Health(ctx); err != nil {
		return "", err
	}
	if err := rebuild(candidate); err != nil {
		// Only the incomplete new collection is removed. The original is never
		// deleted, reset, or renamed when rebuilding fails.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = candidate.ResetAll(cleanupCtx)
		return "", err
	}
	if _, err := candidate.Health(ctx); err != nil {
		return "", err
	}
	journal := chromaRecoveryJournal{s.collectionName, original.ID, candidateName, candidate.collectionRef, backupName}
	data, err := json.Marshal(journal)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(journalPath), 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(journalPath), ".chroma-recovery-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	if err := os.Rename(file.Name(), journalPath); err != nil {
		return "", err
	}
	if err := s.ResumeIndexRecovery(ctx, journalPath); err != nil {
		return backupName, err
	}
	return backupName, nil
}
