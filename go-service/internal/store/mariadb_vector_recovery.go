package store

import (
	"context"
	"strings"
)

// VectorRecoveryCacheReader returns saved materializations, without re-admitting
// memories or changing outbox status. Callers match the saved document and model
// before reusing an embedding in an existing Chroma metadata snapshot.
type VectorRecoveryCacheReader interface {
	ReadVectorRecoveryCache(context.Context, []string) ([]string, error)
}

func (m *mariadbStore) ReadVectorRecoveryCache(ctx context.Context, ids []string) ([]string, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	result := []string{}
	for start := 0; start < len(ids); start += 200 {
		end := min(start+200, len(ids))
		args := make([]any, 0, end-start)
		for _, id := range ids[start:end] {
			args = append(args, id)
		}
		rows, err := m.db.QueryContext(ctx, `SELECT document_json FROM memory_vector_outbox
			WHERE operation = 'upsert' AND document_json IS NOT NULL AND document_id IN (`+
			strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")+`) ORDER BY updated_at, id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				rows.Close()
				return nil, err
			}
			result = append(result, data)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
