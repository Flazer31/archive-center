#!/usr/bin/env python3
"""Unit tests for milvus_http_live_read_probe.py."""

from __future__ import annotations

import argparse
import tempfile
import unittest
from pathlib import Path

import milvus_http_live_read_probe as probe


class MilvusHTTPLiveReadProbeTests(unittest.TestCase):
    def test_path_inside_detects_source_tree_child(self) -> None:
        root = probe.repo_root()
        self.assertTrue(probe.path_inside(root / "tools", root))
        self.assertFalse(probe.path_inside(probe.workspace_root() / ".runtime-cache", root))

    def test_query_docs_uses_query_set_shape(self) -> None:
        query_set = {
            "queries": [
                {
                    "source_id": "memories:1",
                    "embedding": [0.1, 0.2],
                    "tier": "memory",
                    "chat_session_id": "sess-1",
                    "source_table": "memories",
                    "source_row_id": "1",
                    "document_excerpt": "hello",
                }
            ]
        }
        docs = probe.query_docs(query_set, 5)
        self.assertEqual(len(docs), 1)
        self.assertEqual(docs[0]["id"], "memories:1")
        self.assertEqual(docs[0]["embedding"], [0.1, 0.2])
        self.assertEqual(docs[0]["source_table"], "memories")

    def test_first_query_requires_embedding(self) -> None:
        with self.assertRaises(ValueError):
            probe.first_query({"queries": [{"source_id": "memories:1"}]})

    def test_missing_runtime_returns_failed_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            query_set = Path(tmp) / "query-set.json"
            query_set.write_text('{"queries":[{"source_id":"memories:1","embedding":[0.1],"chat_session_id":"s"}]}', encoding="utf-8")
            args = argparse.Namespace(
                runtime_python=str(Path(tmp) / "missing-python"),
                data_root=str(Path(tmp) / "data"),
                go_cache=str(Path(tmp) / "go-cache"),
                query_set=str(query_set),
                milvus_port=0,
                go_port=0,
                doc_limit=5,
                result_limit=5,
                wait_timeout_s=0.1,
                go_timeout_s=0.1,
                http_timeout_s=0.1,
                cleanup=False,
            )
            report, exit_code = probe.run_probe(args)
        self.assertEqual(exit_code, 1)
        self.assertEqual(report["status"], "failed")
        self.assertIn("runtime_python_missing", report["failures"])

    def test_data_root_inside_source_tree_is_blocked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fake_python = Path(tmp) / "python"
            fake_python.write_text("", encoding="utf-8")
            query_set = Path(tmp) / "query-set.json"
            query_set.write_text('{"queries":[{"source_id":"memories:1","embedding":[0.1],"chat_session_id":"s"}]}', encoding="utf-8")
            args = argparse.Namespace(
                runtime_python=str(fake_python),
                data_root=str(probe.repo_root() / "bad-milvus-data"),
                go_cache=str(Path(tmp) / "go-cache"),
                query_set=str(query_set),
                milvus_port=0,
                go_port=0,
                doc_limit=5,
                result_limit=5,
                wait_timeout_s=0.1,
                go_timeout_s=0.1,
                http_timeout_s=0.1,
                cleanup=False,
            )
            report, exit_code = probe.run_probe(args)
        self.assertEqual(exit_code, 1)
        self.assertIn("data_root_inside_source_tree", report["failures"])


if __name__ == "__main__":
    unittest.main()
