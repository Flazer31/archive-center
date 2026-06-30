#!/usr/bin/env python3
"""Dry-run exporter: SQLite -> NDJSON.

Reads a SQLite database in WAL-safe read-only mode and exports each selected
table as a newline-delimited JSON file with stable checksums.
"""

import argparse
import hashlib
import json
import os
import sqlite3
import sys
from datetime import datetime, timezone
from pathlib import Path

CANONICAL_TABLES = [
    "chat_logs",
    "effective_input_logs",
    "memories",
    "direct_evidence_records",
    "kg_triples",
    "audit_logs",
    "critic_feedback",
    "character_events",
]


def _quote_ident(name: str) -> str:
    """Return a quoted SQLite identifier with embedded quotes escaped."""
    return '"' + name.replace('"', '""') + '"'


def _canonical_json(obj: dict) -> str:
    """Return a stable canonical JSON string for checksum purposes."""
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def _row_checksum(row_dict: dict) -> str:
    """SHA-256 of the canonical JSON of the row, excluding auto-increment `id`."""
    data = {k: v for k, v in row_dict.items() if k != "id"}
    return hashlib.sha256(_canonical_json(data).encode("utf-8")).hexdigest()


def _table_checksum(row_checksums: list[str]) -> str:
    """SHA-256 of sorted row checksums (order-independent)."""
    joined = "\n".join(sorted(row_checksums))
    return hashlib.sha256(joined.encode("utf-8")).hexdigest()


def _get_columns(conn: sqlite3.Connection, table_name: str) -> list[str]:
    cursor = conn.execute(f"PRAGMA table_info({_quote_ident(table_name)})")
    return [row[1] for row in cursor.fetchall()]


def _list_user_tables(conn: sqlite3.Connection) -> list[str]:
    cursor = conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
    )
    return [row[0] for row in cursor.fetchall() if not row[0].startswith("sqlite_")]


def _export_table(conn: sqlite3.Connection, table_name: str, source_db_path: str):
    columns = _get_columns(conn, table_name)
    cursor = conn.execute(f"SELECT * FROM {_quote_ident(table_name)}")
    rows = cursor.fetchall()

    row_checksums = []
    export_rows = []

    for row in rows:
        row_dict = dict(zip(columns, row))
        rcs = _row_checksum(row_dict)
        row_checksums.append(rcs)
        out_row = dict(row_dict)
        out_row["_row_checksum"] = rcs
        export_rows.append(out_row)

    tcs = (
        _table_checksum(row_checksums)
        if row_checksums
        else hashlib.sha256().hexdigest()
    )

    meta = {
        "_export_meta": {
            "table_name": table_name,
            "export_timestamp": datetime.now(timezone.utc).isoformat(),
            "source_db_path": source_db_path,
            "row_count": len(rows),
            "columns": columns,
            "table_checksum": tcs,
        }
    }

    return meta, export_rows


def main():
    parser = argparse.ArgumentParser(
        description="Export SQLite tables to NDJSON (dry-run)."
    )
    parser.add_argument("--db", required=True, help="Path to SQLite database")
    parser.add_argument("--out", required=True, help="Output directory")

    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--canonical-only",
        dest="canonical_only",
        action="store_true",
        help="Export only the canonical 8 tables (default)",
    )
    group.add_argument(
        "--all",
        dest="all_tables",
        action="store_true",
        help="Export all user tables",
    )
    args = parser.parse_args()

    mode = "all" if args.all_tables else "canonical-only"

    os.makedirs(args.out, exist_ok=True)

    # Open read-only via URI to avoid creating/writing the source DB.
    db_uri = Path(args.db).resolve().as_uri() + "?mode=ro"
    try:
        conn = sqlite3.connect(db_uri, uri=True)
    except sqlite3.OperationalError as exc:
        print(f"error: cannot open database read-only: {exc}", file=sys.stderr)
        sys.exit(1)

    conn.row_factory = sqlite3.Row

    user_tables = _list_user_tables(conn)

    if mode == "canonical-only":
        tables_to_export = [t for t in CANONICAL_TABLES if t in user_tables]
        skipped = [t for t in CANONICAL_TABLES if t not in user_tables]
    else:
        tables_to_export = user_tables
        skipped = []

    manifest = {
        "source_db_path": args.db,
        "export_timestamp": datetime.now(timezone.utc).isoformat(),
        "mode": mode,
        "tables_exported": [],
        "skipped_missing_tables": skipped,
        "row_counts": {},
        "checksums": {},
    }

    for table_name in tables_to_export:
        meta, export_rows = _export_table(conn, table_name, args.db)
        manifest["tables_exported"].append(table_name)
        manifest["row_counts"][table_name] = meta["_export_meta"]["row_count"]
        manifest["checksums"][table_name] = meta["_export_meta"]["table_checksum"]

        out_path = os.path.join(args.out, f"{table_name}.ndjson")
        with open(out_path, "w", encoding="utf-8") as fh:
            fh.write(json.dumps(meta, ensure_ascii=False) + "\n")
            for row in export_rows:
                fh.write(json.dumps(row, ensure_ascii=False) + "\n")

    manifest_path = os.path.join(args.out, "manifest.json")
    with open(manifest_path, "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, indent=2, ensure_ascii=False)

    conn.close()
    print(f"Exported {len(tables_to_export)} table(s) to {args.out}")
    if skipped:
        print(f"Skipped missing table(s): {', '.join(skipped)}")


if __name__ == "__main__":
    main()
