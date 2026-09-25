package vector

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// ReadChromaRecoverySnapshot reads the metadata and remaining embedding log of
// the packaged Chroma 1.5.9 without opening its damaged HNSW files. No SQLite
// writes or new memory interpretation are performed. Missing vectors stay empty
// so the caller cannot mistake a partial reconstruction for a complete index.
func ReadChromaRecoverySnapshot(ctx context.Context, persistDir, collectionID string) ([]VectorDocument, int, error) {
	path, err := filepath.Abs(filepath.Join(persistDir, "chroma.sqlite3"))
	if err != nil {
		return nil, 0, err
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, 0, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var dimension sql.NullInt64
	if err := tx.QueryRowContext(ctx, "SELECT dimension FROM collections WHERE id = ?", collectionID).Scan(&dimension); err != nil {
		return nil, 0, err
	}
	var segmentID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM segments WHERE collection = ? AND scope = 'METADATA'", collectionID).Scan(&segmentID); err != nil {
		return nil, 0, err
	}
	var checkpoint int64
	err = tx.QueryRowContext(ctx, "SELECT seq_id FROM max_seq_id WHERE segment_id = ?", segmentID).Scan(&checkpoint)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, err
	}
	metadata := map[string]map[string]any{}
	rows, err := tx.QueryContext(ctx, `SELECT e.embedding_id, m.key, m.string_value, m.int_value, m.float_value, m.bool_value
		FROM embeddings e LEFT JOIN embedding_metadata m ON m.id = e.id WHERE e.segment_id = ?`, segmentID)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var id string
		var key, text sql.NullString
		var integer, boolean sql.NullInt64
		var number sql.NullFloat64
		if err := rows.Scan(&id, &key, &text, &integer, &number, &boolean); err != nil {
			rows.Close()
			return nil, 0, err
		}
		if metadata[id] == nil {
			metadata[id] = map[string]any{}
		}
		switch {
		case text.Valid:
			metadata[id][key.String] = text.String
		case integer.Valid:
			metadata[id][key.String] = integer.Int64
		case number.Valid:
			metadata[id][key.String] = number.Float64
		case boolean.Valid:
			metadata[id][key.String] = boolean.Int64 != 0
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	// Older retained operations supply vectors, but may not resurrect IDs absent
	// from the current metadata snapshot. Only operations after its checkpoint
	// change membership or metadata.
	embeddings := map[string][]float32{}
	rows, err = tx.QueryContext(ctx, `SELECT seq_id, operation, id, vector, encoding, metadata
		FROM embeddings_queue WHERE topic LIKE ? ORDER BY seq_id`, "%/"+collectionID)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var seq int64
		var op int
		var id string
		var raw []byte
		var encoding, rawMeta sql.NullString
		if err := rows.Scan(&seq, &op, &id, &raw, &encoding, &rawMeta); err != nil {
			rows.Close()
			return nil, 0, err
		}
		if op == 3 {
			delete(embeddings, id)
			if seq > checkpoint {
				delete(metadata, id)
			}
			continue
		}
		if seq > checkpoint {
			_, exists := metadata[id]
			if (op == 0 && exists) || (op == 1 && !exists) {
				continue
			}
			if !exists {
				metadata[id] = map[string]any{}
			}
			if rawMeta.Valid {
				var changes map[string]any
				if err := json.Unmarshal([]byte(rawMeta.String), &changes); err != nil {
					rows.Close()
					return nil, 0, fmt.Errorf("decode Chroma recovery metadata: %w", err)
				}
				for key, value := range changes {
					if value == nil {
						delete(metadata[id], key)
					} else {
						metadata[id][key] = value
					}
				}
			}
		}
		if len(raw) > 0 {
			if encoding.String != "FLOAT32" || len(raw)%4 != 0 {
				rows.Close()
				return nil, 0, fmt.Errorf("unsupported saved Chroma vector encoding %q", encoding.String)
			}
			values := make([]float32, len(raw)/4)
			for i := range values {
				values[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
			}
			embeddings[id] = values
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	docs := make([]VectorDocument, 0, len(metadata))
	for id, meta := range metadata {
		text := stringFromAny(meta["chroma:document"])
		for key := range meta {
			if strings.HasPrefix(key, "chroma:") {
				delete(meta, key)
			}
		}
		doc := vectorDocumentFromChroma(id, text, meta)
		doc.Embedding = embeddings[id]
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs, int(dimension.Int64), nil
}
