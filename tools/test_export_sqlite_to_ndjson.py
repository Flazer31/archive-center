#!/usr/bin/env python3
"""Tests for export_sqlite_to_ndjson.py."""

import json
import os
import sqlite3
import subprocess
import sys
import tempfile
import unittest

SCRIPT = os.path.join(os.path.dirname(__file__), "export_sqlite_to_ndjson.py")

# Import helpers directly for stronger unit assertions.
from export_sqlite_to_ndjson import _row_checksum, _table_checksum


class TestExportSQLiteToNDJSON(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.db_path = os.path.join(self.tmpdir.name, "test.db")
        conn = sqlite3.connect(self.db_path)
        conn.execute(
            """
            CREATE TABLE chat_logs (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                chat_session_id TEXT,
                turn_index INTEGER,
                role TEXT,
                content TEXT
            )
            """
        )
        conn.execute(
            """
            INSERT INTO chat_logs (chat_session_id, turn_index, role, content)
            VALUES ('sess-1', 1, 'user', 'hello')
            """
        )
        conn.execute(
            """
            CREATE TABLE custom_table (
                id INTEGER PRIMARY KEY,
                name TEXT
            )
            """
        )
        conn.execute("INSERT INTO custom_table (name) VALUES ('test')")
        conn.commit()
        conn.close()

    def tearDown(self):
        self.tmpdir.cleanup()

    def _run(self, *extra_args):
        out_dir = os.path.join(self.tmpdir.name, "out")
        cmd = [
            sys.executable,
            "-B",
            SCRIPT,
            "--db",
            self.db_path,
            "--out",
            out_dir,
        ] + list(extra_args)
        result = subprocess.run(cmd, capture_output=True, text=True)
        return result, out_dir

    def test_canonical_only_exports_chat_logs_and_skips_missing(self):
        result, out_dir = self._run("--canonical-only")
        self.assertEqual(result.returncode, 0, msg=result.stderr)

        # chat_logs should exist
        chat_logs_path = os.path.join(out_dir, "chat_logs.ndjson")
        self.assertTrue(os.path.exists(chat_logs_path))

        with open(chat_logs_path, encoding="utf-8") as fh:
            lines = fh.readlines()
        self.assertEqual(len(lines), 2)  # meta + 1 row

        meta = json.loads(lines[0])
        self.assertEqual(meta["_export_meta"]["table_name"], "chat_logs")
        self.assertEqual(meta["_export_meta"]["row_count"], 1)
        self.assertIn("table_checksum", meta["_export_meta"])
        self.assertIn("columns", meta["_export_meta"])

        row = json.loads(lines[1])
        self.assertEqual(row["chat_session_id"], "sess-1")
        self.assertIn("_row_checksum", row)

        # custom_table should NOT exist
        self.assertFalse(
            os.path.exists(os.path.join(out_dir, "custom_table.ndjson"))
        )

        # manifest should list skipped missing canonical tables
        with open(os.path.join(out_dir, "manifest.json"), encoding="utf-8") as fh:
            manifest = json.load(fh)
        self.assertEqual(manifest["mode"], "canonical-only")
        self.assertIn("chat_logs", manifest["tables_exported"])
        self.assertIn("chat_logs", manifest["row_counts"])
        self.assertIn("chat_logs", manifest["checksums"])
        missing = [t for t in manifest["skipped_missing_tables"] if t != "chat_logs"]
        self.assertTrue(len(missing) > 0)

    def test_all_exports_noncanonical_too(self):
        result, out_dir = self._run("--all")
        self.assertEqual(result.returncode, 0, msg=result.stderr)

        self.assertTrue(os.path.exists(os.path.join(out_dir, "chat_logs.ndjson")))
        self.assertTrue(os.path.exists(os.path.join(out_dir, "custom_table.ndjson")))

        with open(os.path.join(out_dir, "manifest.json"), encoding="utf-8") as fh:
            manifest = json.load(fh)
        self.assertEqual(manifest["mode"], "all")
        self.assertIn("custom_table", manifest["tables_exported"])
        self.assertEqual(manifest["skipped_missing_tables"], [])
        self.assertEqual(manifest["row_counts"]["custom_table"], 1)

    def test_default_is_canonical_only(self):
        # Neither --canonical-only nor --all passed; should behave like canonical-only.
        result, out_dir = self._run()
        self.assertEqual(result.returncode, 0, msg=result.stderr)

        self.assertTrue(os.path.exists(os.path.join(out_dir, "chat_logs.ndjson")))
        self.assertFalse(
            os.path.exists(os.path.join(out_dir, "custom_table.ndjson"))
        )

        with open(os.path.join(out_dir, "manifest.json"), encoding="utf-8") as fh:
            manifest = json.load(fh)
        self.assertEqual(manifest["mode"], "canonical-only")

    def test_row_checksum_excludes_id(self):
        # Prove that _row_checksum ignores the id field.
        row_a = {"id": 1, "chat_session_id": "sess-1", "turn_index": 1, "role": "user", "content": "hello"}
        row_b = {"id": 999, "chat_session_id": "sess-1", "turn_index": 1, "role": "user", "content": "hello"}
        self.assertEqual(_row_checksum(row_a), _row_checksum(row_b))

        # Different non-id data must produce a different checksum.
        row_c = {"id": 1, "chat_session_id": "sess-1", "turn_index": 2, "role": "user", "content": "hello"}
        self.assertNotEqual(_row_checksum(row_a), _row_checksum(row_c))

    def test_table_checksum_is_order_independent(self):
        # Prove that table checksum does not depend on row order.
        checksums = ["aaa", "bbb", "ccc"]
        self.assertEqual(_table_checksum(checksums), _table_checksum(list(reversed(checksums))))

    def test_mutually_exclusive_flags_rejected(self):
        result, _out_dir = self._run("--canonical-only", "--all")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not allowed with", result.stderr.lower())

    def test_missing_db_fails_cleanly(self):
        out_dir = os.path.join(self.tmpdir.name, "out_missing")
        cmd = [
            sys.executable,
            "-B",
            SCRIPT,
            "--db",
            os.path.join(self.tmpdir.name, "does_not_exist.db"),
            "--out",
            out_dir,
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("cannot open database", result.stderr.lower())

    def test_source_db_not_modified(self):
        # Verify that the source DB was not modified by checking its mtime
        # and that no journal/WAL files were created.
        before_mtime = os.path.getmtime(self.db_path)
        result, _out_dir = self._run("--all")
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        after_mtime = os.path.getmtime(self.db_path)
        self.assertEqual(before_mtime, after_mtime)

        for suffix in ("-journal", "-wal", "-shm"):
            self.assertFalse(
                os.path.exists(self.db_path + suffix),
                f"unexpected {suffix} file created",
            )


if __name__ == "__main__":
    unittest.main()
