package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/diagnostics"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

var errRecoveryEmbeddingSettingsRequired = errors.New("Chroma recovery needs embedding settings; waiting for the plugin settings sync; original collection preserved")

// Start the same recovery operation at startup or once on changed Host settings.
// The HTTP management service stays available throughout; no polling worker is used.
func (s *Server) retryIndexRecoveryAfterConfigSync() {
	state := s.indexRecoveryState.Load()
	if state == 0 || state == 2 || !s.indexRecoveryState.CompareAndSwap(state, 2) {
		return
	}
	go func() {
		ctx := s.indexRecoveryContext
		if ctx == nil {
			ctx = context.Background()
		}
		if err := s.recoverStartupVectorIndex(ctx, s.Vector.(vector.IndexRecovery)); err != nil {
			if errors.Is(err, errRecoveryEmbeddingSettingsRequired) {
				s.indexRecoveryState.Store(1)
				// A Host sync may finish just after the missing-settings read,
				// while this attempt still owned state 2. Consume those settings
				// now rather than require the user to change them a second time.
				if s.completeTurnExtractionConfig(nil).Embedder.hasConfig() {
					s.retryIndexRecoveryAfterConfigSync()
				}
			} else {
				s.indexRecoveryState.Store(3)
			}
			slog.Warn("Chroma automatic index recovery remains incomplete", "error", err)
			return
		}
		s.indexRecoveryState.Store(0)
		s.StartMemoryWorkers(ctx)
	}()
}

func (s *Server) indexRecoveryJournalPath() string {
	identity := sha256.Sum256([]byte(s.Cfg.ChromaEndpoint + "\n" + s.Cfg.ChromaAPIPath + "\n" + s.Cfg.ChromaCollection))
	return filepath.Join(diagnostics.Directory(), "chroma-recovery-"+hex.EncodeToString(identity[:8])+".json")
}

func (s *Server) recoverStartupVectorIndex(ctx context.Context, recovery vector.IndexRecovery) error {
	if err := recovery.ResumeIndexRecovery(ctx, s.indexRecoveryJournalPath()); err != nil {
		return err
	}
	if _, err := s.Vector.Health(ctx); err == nil {
		return nil
	}
	endpoint, err := url.Parse(s.Cfg.ChromaEndpoint)
	if err != nil {
		return err
	}
	switch endpoint.Hostname() {
	case "127.0.0.1", "localhost", "::1":
	default:
		return fmt.Errorf("Chroma index recovery needs access to the server's local data directory")
	}
	dataDir := os.Getenv("ARCHIVE_CENTER_DATA_DIR")
	if dataDir == "" {
		return fmt.Errorf("Chroma index recovery needs ARCHIVE_CENTER_DATA_DIR from the managed launcher")
	}
	slog.Warn("Chroma index load failed; starting recovery with saved vectors")
	docs, dimension, err := recovery.RecoverySnapshot(ctx, filepath.Join(dataDir, "chromadb"))
	if err != nil {
		return fmt.Errorf("read saved Chroma index data: %w", err)
	}
	if err := s.fillRecoveryEmbeddings(ctx, docs); err != nil {
		return err
	}
	providerDocuments, err := s.generateMissingRecoveryEmbeddings(ctx, docs)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if len(doc.Embedding) == 0 {
			return fmt.Errorf("Chroma recovery embedding is still missing; original collection preserved")
		} else if dimension > 0 && len(doc.Embedding) != dimension {
			return fmt.Errorf("saved recovery vector dimension does not match the original collection")
		}
	}
	backup, err := recovery.RecoverIndex(ctx, s.indexRecoveryJournalPath(), func(candidate vector.VectorStore) error {
		for start := 0; start < len(docs); start += 100 {
			end := min(start+100, len(docs))
			if err := candidate.Upsert(ctx, "", docs[start:end]); err != nil {
				return err
			}
			ids := make([]string, end-start)
			for i, doc := range docs[start:end] {
				ids[i] = doc.ID
			}
			read, err := candidate.(vector.ExactDocumentReader).GetDocuments(ctx, ids)
			if err != nil {
				return err
			}
			if len(read) != len(ids) {
				return fmt.Errorf("recovered Chroma batch readback is incomplete")
			}
			slog.Info("Chroma index recovery progress", "restored", end, "total", len(docs))
		}
		if len(docs) > 0 {
			_, err := candidate.Search(ctx, docs[0].ChatSessionID, docs[0].Embedding, 1, "")
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	slog.Info("Chroma index recovery completed", "documents", len(docs), "backup_collection", backup, "reembedded_documents", providerDocuments)
	return nil
}

func (s *Server) generateMissingRecoveryEmbeddings(ctx context.Context, docs []vector.VectorDocument) (int, error) {
	groups := map[string][]int{}
	for i, doc := range docs {
		if len(doc.Embedding) == 0 {
			model := strings.TrimSpace(extractionStringFromAny(doc.Metadata["embedding_model"]))
			groups[model] = append(groups[model], i)
		}
	}
	if len(groups) == 0 {
		return 0, nil
	}
	cfg := s.completeTurnExtractionConfig(nil).Embedder
	if !cfg.hasConfig() {
		return 0, errRecoveryEmbeddingSettingsRequired
	}
	generated := 0
	for model, indices := range groups {
		groupCfg := cfg
		if model != "" {
			groupCfg.Model = model
		}
		for start := 0; start < len(indices); start += 32 {
			batch := indices[start:min(start+32, len(indices))]
			texts := make([]string, len(batch))
			for i, index := range batch {
				texts[i] = docs[index].DocumentText
				if strings.TrimSpace(texts[i]) == "" {
					return generated, fmt.Errorf("saved Chroma document has no text for embedding recovery")
				}
			}
			values, _, err := callDocumentEmbeddings(ctx, groupCfg, texts)
			if err != nil {
				return generated, fmt.Errorf("Chroma recovery embedding request failed: %w", err)
			}
			if len(values) != len(batch) {
				return generated, fmt.Errorf("Chroma recovery embedding response is incomplete")
			}
			for i, index := range batch {
				docs[index].Embedding = parseFloat32JSONList(values[i])
			}
			generated += len(batch)
			slog.Info("Chroma recovery embedding progress", "regenerated", generated)
		}
	}
	return generated, nil
}

func (s *Server) fillRecoveryEmbeddings(ctx context.Context, docs []vector.VectorDocument) error {
	missing := map[string]int{}
	ids := []string{}
	for i, doc := range docs {
		if len(doc.Embedding) == 0 {
			missing[doc.ID] = i
			ids = append(ids, doc.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if cache, ok := s.Store.(store.VectorRecoveryCacheReader); ok {
		saved, err := cache.ReadVectorRecoveryCache(ctx, ids)
		if err != nil {
			return fmt.Errorf("read MariaDB vector recovery cache: %w", err)
		}
		for _, raw := range saved {
			var cached vector.VectorDocument
			if json.Unmarshal([]byte(raw), &cached) != nil {
				continue
			}
			if i, ok := missing[cached.ID]; ok && cached.DocumentText == docs[i].DocumentText && len(cached.Embedding) > 0 &&
				strings.TrimSpace(extractionStringFromAny(cached.Metadata["embedding_model"])) == strings.TrimSpace(extractionStringFromAny(docs[i].Metadata["embedding_model"])) {
				docs[i].Embedding = cached.Embedding
			}
		}
	}
	// Aggregate memories also persist the successful embedding in their own row.
	sessions := map[string]bool{}
	for _, doc := range docs {
		if len(doc.Embedding) == 0 && doc.SourceTable == "memories" {
			sessions[doc.ChatSessionID] = true
		}
	}
	for sid := range sessions {
		memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil {
			return err
		}
		for _, mem := range memories {
			id := fmt.Sprintf("memory:%s:%d", sid, mem.ID)
			if i, ok := missing[id]; ok && len(docs[i].Embedding) == 0 && reindexMemoryDocumentText(mem) == docs[i].DocumentText {
				storedModel := strings.TrimSpace(extractionStringFromAny(docs[i].Metadata["embedding_model"]))
				if storedModel == "" || storedModel == strings.TrimSpace(mem.EmbeddingModel) {
					docs[i].Embedding = parseFloat32JSONList(mem.Embedding)
				}
			}
		}
	}
	return nil
}
