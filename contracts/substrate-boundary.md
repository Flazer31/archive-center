# 2.0 Substrate Boundary

Status: R0 defined boundary, not implemented.

## Runtime Boundary

Go-primary means the backend service gradually moves behind the same RisuAI-facing JavaScript host adapter. It does not mean `Archive Center.js` is rewritten in Go.

The completed 0.8(fix) backend feature set is the migration source. 2.0 must preserve that behavior while changing the backend substrate to Go-primary, MariaDB canonical truth, and Milvus Lite local-first vector retrieval. This is not a scope reduction.

`Archive Center.js` is a host compatibility boundary. It must remain JavaScript for RisuAI and should not be changed during 2.0 backend migration unless the user explicitly opens a separate compatibility patch.

The Go service must eventually preserve:

- prepare/complete turn transport semantics
- trace/error vocabulary
- fail-open behavior
- local operator/security envelope
- Python fallback or coexistence window until parity is proven

## Truth Boundary

MariaDB is the target canonical truth store.

Truth authority must stay relational. Vector hits are retrieval candidates, not facts. Direct evidence, audit/replay, canonical state, and rollback behavior must be preserved before any authority cutover.

## Vector Boundary

Milvus Lite is the target local-first vector lane.

The vector lane supports retrieval acceleration only. It must support degraded fail-open behavior where relational truth remains usable if vector search is unavailable, stale, or rebuilding.

## Risk-Tier Boundary

- R0 may define contracts and collect metrics.
- R1 may run shadow write/read/compare while current primary remains active.
- R2 may perform one live cutover at a time.
- R3 may retire old paths only after post-cutover replay passes.

MariaDB truth cutover, Milvus live read switch, and Go default runtime switch must remain separate R2 events.
