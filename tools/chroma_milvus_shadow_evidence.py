#!/usr/bin/env python3
"""Generate Chroma-vs-Milvus Lite shadow evidence from a copied Chroma store.

This is an R1 evidence tool. It copies the 0.8 Chroma persist directory to a
temporary directory, reads the copy, mirrors sampled vectors into temporary
Milvus Lite, and writes JSON evidence. It does not modify the 0.8 runtime and
does not enable live retrieval.
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.metadata
import json
import shutil
import sqlite3
import sys
import tempfile
import time
from pathlib import Path
from typing import Any


QUERY_SET_SCHEMA = "archive-center.chroma-milvus-query-set.v1"
RESULT_SCHEMA = "archive-center.vector-shadow-results.v1"
PARITY_INPUT_SCHEMA = "archive-center.vector-shadow-parity-input.v1"
DEFAULT_COLLECTION = "archive_center_shadow"
DEFAULT_MILVUS_COLLECTION = "archive_center_lite_parity"


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def workspace_root() -> Path:
    return repo_root().parent


def default_chroma_dir() -> Path:
    return workspace_root() / "Archive Center Beta 0.8(fix)" / ".chroma_shadow"


def default_sqlite_db() -> Path:
    return workspace_root() / "Archive Center Beta 0.8(fix)" / "memory.db"


def package_version(name: str) -> str | None:
    try:
        return importlib.metadata.version(name)
    except importlib.metadata.PackageNotFoundError:
        return None


def path_inside(child: Path, parent: Path) -> bool:
    try:
        child.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def json_dump(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=True) + "\n", encoding="utf-8")


def normalize_embedding(value: Any) -> list[float]:
    if value is None:
        return []
    if hasattr(value, "tolist"):
        value = value.tolist()
    return [float(item) for item in value]


def normalize_metadata(value: Any) -> dict[str, Any]:
    return dict(value) if isinstance(value, dict) else {}


def vector_hash(vector: list[float]) -> str:
    payload = json.dumps(vector, separators=(",", ":"), ensure_ascii=True)
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def deterministic_embedding(text: str, dimension: int) -> list[float]:
    values: list[float] = []
    seed = text.encode("utf-8", errors="replace")
    counter = 0
    while len(values) < dimension:
        digest = hashlib.sha256(seed + counter.to_bytes(4, "big")).digest()
        for index in range(0, len(digest), 4):
            chunk = digest[index : index + 4]
            if len(chunk) < 4:
                continue
            integer = int.from_bytes(chunk, "big")
            values.append((integer / 0xFFFFFFFF) * 2.0 - 1.0)
            if len(values) >= dimension:
                break
        counter += 1
    return values


def safe_text(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, (dict, list)):
        return json.dumps(value, ensure_ascii=True, sort_keys=True)
    return str(value)


def parse_embedding_text(value: Any) -> list[float]:
    if value is None:
        return []
    if isinstance(value, bytes):
        value = value.decode("utf-8", errors="replace")
    if not isinstance(value, str):
        value = str(value)
    text = value.strip()
    if not text:
        return []
    try:
        parsed = json.loads(text)
    except json.JSONDecodeError:
        return []
    if not isinstance(parsed, list):
        return []
    out: list[float] = []
    for item in parsed:
        try:
            out.append(float(item))
        except (TypeError, ValueError):
            return []
    return out


def get_hit_source_id(hit: Any) -> str | None:
    if isinstance(hit, dict):
        entity = hit.get("entity")
        if isinstance(entity, dict):
            for key in ("source_id", "id"):
                value = entity.get(key)
                if value is not None:
                    return str(value)
        for key in ("source_id", "id"):
            value = hit.get(key)
            if value is not None:
                return str(value)
    return None


def chroma_client(path: Path) -> Any:
    import chromadb  # type: ignore

    try:
        from chromadb.config import Settings  # type: ignore

        return chromadb.PersistentClient(
            path=str(path),
            settings=Settings(anonymized_telemetry=False),
        )
    except Exception:
        return chromadb.PersistentClient(path=str(path))


def load_chroma_documents(collection: Any, max_docs: int) -> list[dict[str, Any]]:
    count = int(collection.count())
    if count <= 0:
        return []
    limit = min(count, max_docs)
    raw = collection.get(limit=limit, include=["embeddings", "metadatas", "documents"])
    ids = raw.get("ids") or []
    embeddings = raw.get("embeddings") or []
    metadatas = raw.get("metadatas") or []
    documents = raw.get("documents") or []

    docs: list[dict[str, Any]] = []
    for index, doc_id in enumerate(ids):
        embedding = normalize_embedding(embeddings[index] if index < len(embeddings) else None)
        if not embedding:
            continue
        metadata = normalize_metadata(metadatas[index] if index < len(metadatas) else None)
        document = documents[index] if index < len(documents) else ""
        docs.append(
            {
                "id": str(doc_id),
                "embedding": embedding,
                "metadata": metadata,
                "document": str(document or ""),
            }
        )
    return docs


def chroma_metric_name(metric_type: str) -> str:
    metric = metric_type.lower()
    if metric == "ip":
        return "ip"
    if metric == "cosine":
        return "cosine"
    return "l2"


def create_temp_chroma_collection(
    path: Path,
    collection_name: str,
    docs: list[dict[str, Any]],
    metric_type: str,
) -> Any:
    client = chroma_client(path)
    collection = client.get_or_create_collection(
        name=collection_name,
        metadata={"hnsw:space": chroma_metric_name(metric_type)},
    )
    collection.add(
        ids=[doc["id"] for doc in docs],
        embeddings=[doc["embedding"] for doc in docs],
        metadatas=[doc["metadata"] for doc in docs],
        documents=[doc["document"] for doc in docs],
    )
    return collection


def table_columns(conn: sqlite3.Connection, table: str) -> set[str]:
    return {str(row[1]) for row in conn.execute(f"pragma table_info({table})")}


def add_doc(
    docs: list[dict[str, Any]],
    *,
    table: str,
    row_id: Any,
    session_id: Any,
    text: str,
    embedding_dim: int,
    tier: str,
    source_turn: Any = "",
) -> None:
    normalized = " ".join((text or "").split())
    if not normalized:
        return
    source_id = f"{table}:{row_id}"
    docs.append(
        {
            "id": source_id,
            "embedding": deterministic_embedding(f"{source_id}\n{normalized}", embedding_dim),
            "metadata": {
                "tier": tier,
                "chat_session_id": safe_text(session_id),
                "source_table": table,
                "source_row_id": safe_text(row_id),
                "schema_version": "synthetic_embedding_from_sqlite_sample.v1",
                "source_turn": safe_text(source_turn),
            },
            "document": normalized,
        }
    )


def load_sqlite_sample_documents(db_path: Path, max_docs: int, embedding_dim: int) -> list[dict[str, Any]]:
    docs: list[dict[str, Any]] = []
    conn = sqlite3.connect(f"file:{db_path.as_posix()}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    try:
        tables = {
            row[0]
            for row in conn.execute("select name from sqlite_master where type='table'")
        }

        def remaining() -> int:
            return max_docs - len(docs)

        if "memories" in tables and remaining() > 0:
            for row in conn.execute(
                "select id, chat_session_id, turn_index, summary_json, evidence from memories "
                "order by id limit ?",
                (remaining(),),
            ):
                add_doc(
                    docs,
                    table="memories",
                    row_id=row["id"],
                    session_id=row["chat_session_id"],
                    text=f"{safe_text(row['summary_json'])} {safe_text(row['evidence'])}",
                    embedding_dim=embedding_dim,
                    tier="memory",
                    source_turn=row["turn_index"],
                )

        if "direct_evidence_records" in tables and remaining() > 0:
            for row in conn.execute(
                "select id, chat_session_id, turn_anchor, evidence_text from direct_evidence_records "
                "order by id limit ?",
                (remaining(),),
            ):
                add_doc(
                    docs,
                    table="direct_evidence_records",
                    row_id=row["id"],
                    session_id=row["chat_session_id"],
                    text=row["evidence_text"],
                    embedding_dim=embedding_dim,
                    tier="direct_evidence",
                    source_turn=row["turn_anchor"],
                )

        summary_specs = [
            ("episode_summaries", "summary_text", "episode"),
            ("chapter_summaries", "summary_text", "chapter"),
            ("arc_summaries", "arc_resume_text", "arc"),
            ("saga_digests", "saga_summary", "saga"),
        ]
        for table, text_column, tier in summary_specs:
            if table not in tables or remaining() <= 0:
                continue
            cols = table_columns(conn, table)
            if text_column not in cols:
                continue
            turn_column = "to_turn" if "to_turn" in cols else "id"
            for row in conn.execute(
                f"select id, chat_session_id, {turn_column} as source_turn, {text_column} as text "
                f"from {table} order by id limit ?",
                (remaining(),),
            ):
                add_doc(
                    docs,
                    table=table,
                    row_id=row["id"],
                    session_id=row["chat_session_id"],
                    text=row["text"],
                    embedding_dim=embedding_dim,
                    tier=tier,
                    source_turn=row["source_turn"],
                )

        if "kg_triples" in tables and remaining() > 0:
            for row in conn.execute(
                "select id, chat_session_id, source_turn, subject, predicate, object from kg_triples "
                "order by id limit ?",
                (remaining(),),
            ):
                add_doc(
                    docs,
                    table="kg_triples",
                    row_id=row["id"],
                    session_id=row["chat_session_id"],
                    text=f"{row['subject']} {row['predicate']} {row['object']}",
                    embedding_dim=embedding_dim,
                    tier="kg_triple",
                    source_turn=row["source_turn"],
                )

        if "chat_logs" in tables and remaining() > 0:
            for row in conn.execute(
                "select id, chat_session_id, turn_index, role, content from chat_logs "
                "order by id limit ?",
                (remaining(),),
            ):
                add_doc(
                    docs,
                    table="chat_logs",
                    row_id=row["id"],
                    session_id=row["chat_session_id"],
                    text=f"{row['role']}: {row['content']}",
                    embedding_dim=embedding_dim,
                    tier="chat_log",
                    source_turn=row["turn_index"],
                )
    finally:
        conn.close()
    return docs


def load_sqlite_real_embedding_documents(db_path: Path, max_docs: int) -> list[dict[str, Any]]:
    docs: list[dict[str, Any]] = []
    conn = sqlite3.connect(f"file:{db_path.as_posix()}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    try:
        tables = {
            row[0]
            for row in conn.execute("select name from sqlite_master where type='table'")
        }
        if "memories" not in tables:
            return docs
        for row in conn.execute(
            "select id, chat_session_id, turn_index, summary_json, evidence, embedding, "
            "embedding_model from memories "
            "where embedding is not null and trim(cast(embedding as text)) != '' "
            "order by id limit ?",
            (max_docs,),
        ):
            embedding = parse_embedding_text(row["embedding"])
            if not embedding:
                continue
            text = " ".join(
                (
                    safe_text(row["summary_json"]),
                    safe_text(row["evidence"]),
                )
            ).strip()
            source_id = f"memories:{row['id']}"
            docs.append(
                {
                    "id": source_id,
                    "embedding": embedding,
                    "metadata": {
                        "tier": "memory",
                        "chat_session_id": safe_text(row["chat_session_id"]),
                        "source_table": "memories",
                        "source_row_id": safe_text(row["id"]),
                        "schema_version": "sqlite_real_memory_embedding.v1",
                        "source_turn": safe_text(row["turn_index"]),
                        "embedding_model": safe_text(row["embedding_model"]),
                    },
                    "document": text or source_id,
                }
            )
    finally:
        conn.close()
    return docs


def query_chroma(collection: Any, queries: list[dict[str, Any]], result_limit: int) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for query in queries:
        raw = collection.query(
            query_embeddings=[query["embedding"]],
            n_results=result_limit,
            include=["distances", "metadatas", "documents"],
        )
        ids = [str(item) for item in (raw.get("ids") or [[]])[0]]
        out.append(
            {
                "query_id": query["query_id"],
                "source_id": query["source_id"],
                "chroma_ids": ids,
                "distances": (raw.get("distances") or [[]])[0],
            }
        )
    return out


def build_milvus_rows(docs: list[dict[str, Any]]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for index, doc in enumerate(docs, start=1):
        metadata = doc["metadata"]
        rows.append(
            {
                "id": index,
                "vector": doc["embedding"],
                "source_id": doc["id"],
                "text": doc["document"],
                "tier": str(metadata.get("tier") or ""),
                "chat_session_id": str(metadata.get("chat_session_id") or ""),
                "source_table": str(metadata.get("source_table") or ""),
                "source_row_id": str(metadata.get("source_row_id") or ""),
                "schema_version": str(metadata.get("schema_version") or ""),
            }
        )
    return rows


def query_milvus(
    docs: list[dict[str, Any]],
    queries: list[dict[str, Any]],
    result_limit: int,
    collection_name: str,
    metric_type: str,
) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    from pymilvus import MilvusClient  # type: ignore

    temp_dir = Path(tempfile.mkdtemp(prefix="archive-center-milvus-parity-"))
    db_path = temp_dir / "milvus_lite_parity.db"
    client = MilvusClient(str(db_path))
    dimension = len(docs[0]["embedding"])
    cleanup_warning: str | None = None
    try:
        try:
            client.create_collection(
                collection_name=collection_name,
                dimension=dimension,
                metric_type=metric_type,
            )
        except TypeError:
            client.create_collection(collection_name=collection_name, dimension=dimension)
        rows = build_milvus_rows(docs)
        insert_result = client.insert(collection_name=collection_name, data=rows)

        out: list[dict[str, Any]] = []
        for query in queries:
            raw_hits = client.search(
                collection_name=collection_name,
                data=[query["embedding"]],
                limit=result_limit,
                output_fields=[
                    "source_id",
                    "text",
                    "tier",
                    "chat_session_id",
                    "source_table",
                    "source_row_id",
                    "schema_version",
                ],
            )
            hits = raw_hits[0] if raw_hits else []
            ids = [source_id for source_id in (get_hit_source_id(hit) for hit in hits) if source_id]
            out.append(
                {
                    "query_id": query["query_id"],
                    "source_id": query["source_id"],
                    "milvus_ids": ids,
                    "raw_hit_count": len(hits),
                }
            )
        meta = {
            "db_path": str(db_path),
            "collection": collection_name,
            "dimension": dimension,
            "metric_type": metric_type,
            "insert_result": str(insert_result),
            "insert_count": len(rows),
        }
        return out, meta
    finally:
        try:
            close = getattr(client, "close", None)
            if callable(close):
                close()
        except Exception:
            pass
        for _ in range(5):
            try:
                shutil.rmtree(temp_dir)
                cleanup_warning = None
                break
            except PermissionError as exc:
                cleanup_warning = f"{type(exc).__name__}: {exc}"
                time.sleep(0.2)
            except OSError as exc:
                cleanup_warning = f"{type(exc).__name__}: {exc}"
                time.sleep(0.2)
        if cleanup_warning:
            print(f"warning: temp cleanup incomplete: {cleanup_warning}", file=sys.stderr)


def build_query_set(docs: list[dict[str, Any]], query_limit: int) -> list[dict[str, Any]]:
    queries: list[dict[str, Any]] = []
    for index, doc in enumerate(docs[:query_limit], start=1):
        metadata = doc["metadata"]
        embedding = doc["embedding"]
        queries.append(
            {
                "query_id": f"q{index}",
                "source_id": doc["id"],
                "embedding": embedding,
                "embedding_dim": len(embedding),
                "embedding_sha256": vector_hash(embedding),
                "tier": str(metadata.get("tier") or ""),
                "chat_session_id": str(metadata.get("chat_session_id") or ""),
                "source_table": str(metadata.get("source_table") or ""),
                "source_row_id": str(metadata.get("source_row_id") or ""),
                "document_excerpt": doc["document"][:160],
            }
        )
    return queries


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate Chroma/Milvus shadow evidence JSON")
    parser.add_argument("--chroma-persist-dir", default=str(default_chroma_dir()))
    parser.add_argument("--sqlite-fallback-db", default=str(default_sqlite_db()))
    parser.add_argument("--chroma-collection", default=DEFAULT_COLLECTION)
    parser.add_argument("--milvus-collection", default=DEFAULT_MILVUS_COLLECTION)
    parser.add_argument("--out-dir", default=str(repo_root() / "benchmarks"))
    parser.add_argument("--query-limit", type=int, default=5)
    parser.add_argument("--result-limit", type=int, default=5)
    parser.add_argument("--max-docs", type=int, default=500)
    parser.add_argument("--embedding-dim", type=int, default=64)
    parser.add_argument("--metric-type", default="L2", choices=["L2", "COSINE", "IP"])
    parser.add_argument("--allow-sqlite-fallback", action="store_true")
    parser.add_argument("--date-stamp", default="2026-05-25")
    args = parser.parse_args(argv)
    if args.query_limit <= 0:
        parser.error("--query-limit must be positive")
    if args.result_limit <= 0:
        parser.error("--result-limit must be positive")
    if args.max_docs <= 0:
        parser.error("--max-docs must be positive")
    if args.embedding_dim <= 0:
        parser.error("--embedding-dim must be positive")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    source_chroma = Path(args.chroma_persist_dir).expanduser().resolve()
    source_sqlite = Path(args.sqlite_fallback_db).expanduser().resolve()
    out_dir = Path(args.out_dir).expanduser().resolve()

    if not source_chroma.exists():
        raise SystemExit(f"Chroma persist dir not found: {source_chroma}")
    if path_inside(out_dir, workspace_root() / "Archive Center Beta 0.8(fix)"):
        raise SystemExit("Refusing to write evidence inside Archive Center Beta 0.8(fix)")

    temp_dir = Path(tempfile.mkdtemp(prefix="archive-center-chroma-copy-"))
    started = time.time()
    try:
        chroma_copy = temp_dir / "chroma_shadow_copy"
        shutil.copytree(source_chroma, chroma_copy)
        client = chroma_client(chroma_copy)
        collection = client.get_collection(name=args.chroma_collection)
        collection_count = int(collection.count())
        docs = load_chroma_documents(collection, args.max_docs)
        if not docs:
            if not args.allow_sqlite_fallback:
                raise SystemExit("No Chroma documents with embeddings were available for parity evidence")
            if not source_sqlite.exists():
                raise SystemExit(f"SQLite fallback DB not found: {source_sqlite}")
            sqlite_copy = temp_dir / "memory.copy.db"
            shutil.copy2(source_sqlite, sqlite_copy)
            docs = load_sqlite_real_embedding_documents(sqlite_copy, args.max_docs)
            if docs:
                source_mode = "sqlite_real_memory_embeddings"
            else:
                docs = load_sqlite_sample_documents(sqlite_copy, args.max_docs, args.embedding_dim)
                source_mode = "sqlite_canonical_sample_with_synthetic_embeddings"
            if not docs:
                raise SystemExit("SQLite fallback did not produce sample retrieval documents")
            chroma_copy = temp_dir / "generated_chroma_shadow"
            collection = create_temp_chroma_collection(
                chroma_copy,
                args.chroma_collection,
                docs,
                args.metric_type,
            )
            collection_count = int(collection.count())
        else:
            source_mode = "existing_chroma_shadow_copy"

        queries = build_query_set(docs, min(args.query_limit, len(docs)))
        chroma_results = query_chroma(collection, queries, args.result_limit)
        milvus_results, milvus_meta = query_milvus(
            docs,
            queries,
            args.result_limit,
            args.milvus_collection,
            args.metric_type,
        )

        query_path = out_dir / f"chroma-milvus-query-set-{args.date_stamp}.json"
        chroma_path = out_dir / f"chroma-shadow-results-{args.date_stamp}.json"
        milvus_path = out_dir / f"milvus-lite-results-{args.date_stamp}.json"
        parity_input_path = out_dir / f"chroma-milvus-parity-input-{args.date_stamp}.json"

        environment = {
            "python_version": sys.version.split()[0],
            "chromadb_version": package_version("chromadb"),
            "pymilvus_version": package_version("pymilvus"),
            "milvus_lite_version": package_version("milvus-lite"),
        }
        base_meta = {
            "generated_at": "2026-05-25",
            "source_chroma_persist_dir": str(source_chroma),
            "source_chroma_opened_from_temp_copy": True,
            "source_sqlite_db": str(source_sqlite) if source_mode.startswith("sqlite_") else None,
            "source_mode": source_mode,
            "synthetic_embeddings": source_mode == "sqlite_canonical_sample_with_synthetic_embeddings",
            "chroma_collection": args.chroma_collection,
            "chroma_collection_count": collection_count,
            "mirrored_doc_count": len(docs),
            "query_count": len(queries),
            "result_limit": args.result_limit,
            "metric_type": args.metric_type,
            "environment": environment,
            "non_goals": [
                "does_not_modify_archive_center_0_8",
                "does_not_enable_go_vector_store",
                "does_not_switch_live_retrieval",
                "does_not_create_source_tree_vector_persist_dir",
            ],
        }

        json_dump(
            query_path,
            {
                "schema_version": QUERY_SET_SCHEMA,
                **base_meta,
                "queries": queries,
            },
        )
        json_dump(
            chroma_path,
            {
                "schema_version": RESULT_SCHEMA,
                "engine": "chroma",
                **base_meta,
                "queries": chroma_results,
            },
        )
        json_dump(
            milvus_path,
            {
                "schema_version": RESULT_SCHEMA,
                "engine": "milvus_lite",
                **base_meta,
                "milvus": milvus_meta,
                "queries": milvus_results,
            },
        )
        parity_queries = []
        chroma_by_query = {item["query_id"]: item for item in chroma_results}
        milvus_by_query = {item["query_id"]: item for item in milvus_results}
        for query in queries:
            query_id = query["query_id"]
            parity_queries.append(
                {
                    "query_id": query_id,
                    "source_id": query["source_id"],
                    "chroma_ids": chroma_by_query[query_id]["chroma_ids"],
                    "milvus_ids": milvus_by_query[query_id]["milvus_ids"],
                }
            )
        json_dump(
            parity_input_path,
            {
                "schema_version": PARITY_INPUT_SCHEMA,
                **base_meta,
                "queries": parity_queries,
            },
        )

        summary = {
            "status": "ok",
            "duration_ms": round((time.time() - started) * 1000.0, 3),
            "query_set": str(query_path),
            "chroma_results": str(chroma_path),
            "milvus_results": str(milvus_path),
            "parity_input": str(parity_input_path),
            "query_count": len(queries),
            "mirrored_doc_count": len(docs),
        }
        print(json.dumps(summary, indent=2, ensure_ascii=True))
        return 0
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
