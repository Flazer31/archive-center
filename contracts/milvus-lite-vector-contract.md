# Milvus Lite Vector Contract ? Archive Center 2.0 R0

> Status: **R0 design contract**  
> Live Milvus Lite implementation is **explicitly banned** in R0/R1. This document freezes the vector store contract based on 0.8 Chroma shadow behavior analysis.

---

## 1. 0.8 Chroma Shadow Behavior Analysis

### 1.1 Storage
- **Client**: `chromadb.PersistentClient` with `anonymized_telemetry=False`.
- **Persist dir**: `.chroma_shadow/` (relative to backend root).
- **Collection name**: `archive_center_shadow` (configurable via `CHROMA_SHADOW_COLLECTION_NAME`).
- **DB file**: `.chroma_shadow/chroma.sqlite3` (~168 KB).

### 1.2 Collection Metadata
```json
{
  "contract_version": "c17.bootstrap.v1",
  "schema_version": "q1a.v1",
  "truth_authority": "sqlite",
  "session_partitioning": "session_partitioned"
}
```

### 1.3 Document Metadata (per upsert)
| Field | Type | Description |
|-------|------|-------------|
| `tier` | string | `"memory"`, `"episode"`, `"chapter"`, `"arc"`, `"saga"` |
| `chat_session_id` | string | Session partition key |
| `source_table` | string | SQLite source table name |
| `source_row_id` | string | SQLite source row ID |
| `schema_version` | string | Retrieval document schema version |

### 1.4 Upsert Pattern
```python
collection.upsert(
    ids=[f"{tier}:{doc_id}:{source_row_id}"],
    documents=[doc_text],
    embeddings=[embedding_vector],
    metadatas=[{"tier": tier, "chat_session_id": sid, ...}],
)
```

### 1.5 Query Pattern
```python
collection.query(
    query_embeddings=[embedding_vector],
    n_results=N,
)
```

### 1.6 Get Pattern (snapshot / count)
```python
collection.get(include=["documents", "embeddings", "metadatas"])
collection.get(include=["metadatas"])  # count by session
```

### 1.7 Delete Pattern
- Chroma shadow does **not** use `collection.delete()` directly.
- Rebuild drill creates a new collection, upserts validated rows, then swaps.
- Rollback is achieved by discarding the rebuilt collection.

---

## 2. Milvus Lite Mapping Contract

### 2.1 Storage
- **Engine**: Milvus Lite (local-first, no server required).
- **Persist dir**: `.milvus_lite/` (configurable via `MILVUS_LITE_PATH`).
- **Collection name**: `archive_center_lite` (configurable via `MILVUS_LITE_COLLECTION`).
- **Go SDK**: `github.com/milvus-io/milvus-sdk-go/v2` (future R1+).
- **Python SDK**: `pymilvus` (Milvus Lite mode).

### 2.2 Collection Schema

```go
// Milvus Lite collection schema (Go conceptual)
type VectorDocument struct {
    // Primary key ? composite string ID
    ID string `milvus:"primary_key;auto_id=false"`

    // Vector field ? float32 array
    Embedding []float32 `milvus:"dim=1536"`  // dim matches PROJECT_EMBEDDING_DIM

    // Scalar fields ? metadata
    Tier          string `milvus:"index"`
    ChatSessionID string `milvus:"index"`
    SourceTable   string `milvus:""`
    SourceRowID   string `milvus:""`
    SchemaVersion string `milvus:""`
    DocumentText  string `milvus:""`  // Chroma "documents" field
}
```

### 2.3 Milvus Collection Properties
```json
{
  "contract_version": "c17.bootstrap.v1",
  "schema_version": "q1a.v1",
  "truth_authority": "mariadb",
  "session_partitioning": "session_partitioned",
  "vector_engine": "milvus_lite"
}
```

---

## 3. Operation Mapping

### 3.1 Upsert

**Chroma**:
```python
collection.upsert(ids=ids, documents=docs, embeddings=embs, metadatas=mds)
```

**Milvus Lite**:
```python
# Python (Milvus Lite)
from pymilvus import MilvusClient
client = MilvusClient(".milvus_lite/archive_center.db")
client.upsert(
    collection_name="archive_center_lite",
    data=[{
        "id": id,
        "embedding": emb,
        "tier": md["tier"],
        "chat_session_id": md["chat_session_id"],
        "source_table": md["source_table"],
        "source_row_id": md["source_row_id"],
        "schema_version": md["schema_version"],
        "document_text": doc,
    }],
)
```

**Go (future R1+)**:
```go
// Conceptual ? milvus-sdk-go/v2
entities := []entity.Column{
    entity.NewColumnVarChar("id", ids),
    entity.NewColumnFloatVector("embedding", dim, embeddings),
    entity.NewColumnVarChar("tier", tiers),
    entity.NewColumnVarChar("chat_session_id", sessionIDs),
    // ...
}
client.Upsert(ctx, collectionName, "", entities...)
```

### 3.2 Search (Query)

**Chroma**:
```python
collection.query(query_embeddings=[emb], n_results=10)
```

**Milvus Lite**:
```python
# Python
client.search(
    collection_name="archive_center_lite",
    data=[emb],
    limit=10,
    output_fields=["tier", "chat_session_id", "source_table", "source_row_id", "document_text"],
    filter=f'chat_session_id == "{sid}"',
)
```

**Go (future R1+)**:
```go
sp := &entity.SearchParam{}
sp.WithExpr(fmt.Sprintf(`chat_session_id == "%s"`, sid))
results, err := client.Search(ctx, collectionName, nil, limit, "", []string{"id", "tier", "document_text"}, vector, "embedding", entity.L2, sp)
```

### 3.3 Get (Snapshot / Count)

**Chroma**:
```python
collection.get(include=["metadatas"])
```

**Milvus Lite**:
```python
# Count by session
client.query(
    collection_name="archive_center_lite",
    filter=f'chat_session_id == "{sid}"',
    output_fields=["id"],
    limit=100000,
)
```

### 3.4 Delete

**Chroma**: Not directly used; rebuild drill swaps collections.

**Milvus Lite**:
```python
# Delete by session (for rebuild / cleanup)
client.delete(
    collection_name="archive_center_lite",
    filter=f'chat_session_id == "{sid}"',
)
```

---

## 4. Fail-Open Behavior

### 4.1 Chroma Shadow Fail-Open
- If Chroma client is unavailable ? fallback to SQLite-only retrieval.
- If embedding mismatch ? skip shadow, continue with SQLite.
- If query returns no results ? fallback to SQLite search.
- Shadow read path changes do not block chat flow.

### 4.2 Milvus Lite Fail-Open (Future)
- If `.milvus_lite/` file is missing/corrupt ? recreate collection, backfill from MariaDB.
- If search returns no results ? fallback to MariaDB LIKE search on `memories.summary_json`.
- If upsert fails ? log to `audit_logs`, continue without blocking.
- If dimension mismatch ? reject with `ErrDimensionMismatch`, trigger re-embedding audit.

---

## 5. Rebuild / Backfill / Delete Contract

### 5.1 Backfill (from MariaDB ? Milvus Lite)
1. Query MariaDB canonical truth tables (`memories`, `episode_summaries`, etc.) by `chat_session_id`.
2. Build retrieval documents via `_build_retrieval_document_q1a` equivalent.
3. Batch upsert into Milvus Lite (batch size: 25 per tier).
4. Checkpoint per tier to resume on interruption.

### 5.2 Rebuild Drill
1. Create new collection `archive_center_lite_rebuild_{timestamp}`.
2. Backfill from MariaDB into rebuild collection.
3. Validate: sample query sanity check.
4. If valid ? atomic swap (rename old ? `_rollback`, rename rebuild ? active).
5. If invalid ? discard rebuild collection, keep original.

### 5.3 Delete by Session
```python
client.delete(
    collection_name="archive_center_lite",
    filter=f'chat_session_id == "{sid}"',
)
```

### 5.4 Full Reset
```python
# Drop and recreate collection
client.drop_collection("archive_center_lite")
_create_milvus_lite_collection()
```

---

## 6. Metadata Filter Contract

### 6.1 Supported Filters (Milvus Lite expr)
| Filter | Example | Notes |
|--------|---------|-------|
| Equality | `chat_session_id == "sess-123"` | Primary partition filter |
| Equality | `tier == "memory"` | Tier filter |
| IN | `tier in ["memory", "episode"]` | Multi-tier filter |
| AND | `chat_session_id == "sess-123" and tier == "memory"` | Composite filter |
| EXISTS | `has(source_table)` | Field presence check |

### 6.2 Unsupported Filters (R0)
- Range queries on `source_row_id` (string comparison unreliable).
- Full-text search on `document_text` (use MariaDB for text search; Milvus for vector search).
- Complex OR expressions (decompose into multiple queries).

---

## 7. Go Interface Skeleton (Disabled)

```go
package vectorstore

import "context"

var ErrNotEnabled = errors.New("milvus lite is not enabled in R0/R1")

// VectorStore defines the vector search contract.
type VectorStore interface {
    Upsert(ctx context.Context, docs []VectorDocument) error
    Search(ctx context.Context, sessionID string, vector []float32, limit int) ([]VectorDocument, error)
    Count(ctx context.Context, sessionID string) (int, error)
    DeleteSession(ctx context.Context, sessionID string) error
}

// VectorDocument maps to a single upserted row.
type VectorDocument struct {
    ID            string
    Embedding     []float32
    Tier          string
    ChatSessionID string
    SourceTable   string
    SourceRowID   string
    SchemaVersion string
    DocumentText  string
}

// OpenMilvusLite returns ErrNotEnabled in R0/R1.
func OpenMilvusLite(path string) (VectorStore, error) {
    return nil, ErrNotEnabled
}
```

---

## 8. R0 Shadow Status

| Component | Status | Notes |
|-----------|--------|-------|
| Milvus Lite client import | **Disabled** | No `pymilvus` or `milvus-sdk-go` in dependency tree. |
| Collection creation | **Disabled** | No `.milvus_lite/` dir created at runtime. |
| Upsert | **Disabled** | `OpenMilvusLite` returns `ErrNotEnabled`. |
| Search | **Disabled** | `OpenMilvusLite` returns `ErrNotEnabled`. |
| Backfill | **Disabled** | Shadow handler returns `writeShadowGuard`. |
| Rebuild drill | **Disabled** | Shadow handler returns `writeShadowGuard`. |
| Delete | **Disabled** | Shadow handler returns `writeShadowGuard`. |

---

## 9. Verification Checklist

Before live Milvus Lite implementation:

- [ ] `pymilvus` / `milvus-sdk-go` dependency added with version pin.
- [ ] `.milvus_lite/` path is configurable via `MILVUS_LITE_PATH` env.
- [ ] Collection schema matches Chroma shadow metadata + document fields.
- [ ] Embedding dimension matches `PROJECT_EMBEDDING_DIM` (default 1536).
- [ ] Backfill batch size is configurable (default 25 per tier).
- [ ] Rebuild drill creates a new collection, validates, then swaps atomically.
- [ ] Fail-open fallback to MariaDB is tested.
- [ ] Metadata filter expressions are validated before execution.
- [ ] H-4e release hygiene scan passes (no `.env`, no secrets).

---

*Contract version: R0-2026-05-22*  
*Reference: `Archive Center Beta 0.8(fix)/backend/services/chroma_shadow.py`, `backend/chroma_shadow_contracts.py`*
