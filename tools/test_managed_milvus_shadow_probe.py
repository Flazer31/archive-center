#!/usr/bin/env python3
"""Unit tests for managed_milvus_shadow_probe.py."""

from __future__ import annotations

import argparse
import json
import tempfile
import unittest
from pathlib import Path

import managed_milvus_shadow_probe as probe


class ManagedMilvusShadowProbeTests(unittest.TestCase):
    def sample_query_set(self) -> dict:
        return {
            "source_mode": "sqlite_real_memory_embeddings",
            "synthetic_embeddings": False,
            "mirrored_doc_count": 2,
            "query_count": 2,
            "metric_type": "L2",
            "queries": [
                {
                    "query_id": "q1",
                    "source_id": "memories:1",
                    "embedding": [0.1, 0.2],
                    "tier": "memory",
                    "chat_session_id": "sess-1",
                    "source_table": "memories",
                    "source_row_id": "1",
                    "document_excerpt": "one",
                },
                {
                    "query_id": "q2",
                    "source_id": "memories:2",
                    "embedding": [0.3, 0.4],
                    "tier": "memory",
                    "chat_session_id": "sess-1",
                    "source_table": "memories",
                    "source_row_id": "2",
                    "document_excerpt": "two",
                },
            ],
        }

    def test_build_rows(self) -> None:
        rows = probe.build_rows(self.sample_query_set(), max_docs=2)
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]["id"], 1)
        self.assertEqual(rows[0]["source_id"], "memories:1")
        self.assertEqual(rows[0]["vector"], [0.1, 0.2])

    def test_build_search_queries(self) -> None:
        queries = probe.build_search_queries(self.sample_query_set(), query_limit=1)
        self.assertEqual(len(queries), 1)
        self.assertEqual(queries[0]["query_id"], "q1")
        self.assertEqual(queries[0]["vector"], [0.1, 0.2])

    def test_get_hit_source_id_accepts_entity_shape(self) -> None:
        self.assertEqual(
            probe.get_hit_source_id({"entity": {"source_id": "memories:1"}}),
            "memories:1",
        )

    def test_jaccard(self) -> None:
        self.assertEqual(probe.jaccard(["a", "b"], ["b", "c"]), 1 / 3)
        self.assertEqual(probe.jaccard([], []), 1.0)

    def test_rebuild_parity_detects_stable_runs(self) -> None:
        runs = [
            {"comparisons": [{"query_id": "q1", "milvus_ids": ["a", "b"]}]},
            {"comparisons": [{"query_id": "q1", "milvus_ids": ["a", "b"]}]},
        ]
        report = probe.build_rebuild_parity(runs)
        self.assertEqual(report["status"], "ok")
        self.assertEqual(report["mismatch_count"], 0)

    def test_rebuild_parity_detects_mismatch(self) -> None:
        runs = [
            {"comparisons": [{"query_id": "q1", "milvus_ids": ["a", "b"]}]},
            {"comparisons": [{"query_id": "q1", "milvus_ids": ["b", "a"]}]},
        ]
        report = probe.build_rebuild_parity(runs)
        self.assertEqual(report["status"], "mismatch")
        self.assertEqual(report["mismatch_count"], 1)

    def test_exception_report_is_safe_fail_open(self) -> None:
        args = argparse.Namespace(
            query_set="missing.json",
            result_limit=5,
            metric_type="L2",
            rebuild_runs=2,
        )
        report = probe.build_exception_report(args, ValueError("bad query set"))
        self.assertEqual(report["status"], "failed")
        self.assertFalse(report["fail_open_checks"]["live_retrieval_enabled"])
        self.assertTrue(report["fail_open_checks"]["invalid_input_safe"])

    def test_persist_path_inside_source_tree_is_blocked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            query_set_path = Path(tmp) / "query_set.json"
            query_set_path.write_text(json.dumps(self.sample_query_set()), encoding="utf-8")
            args = argparse.Namespace(
                query_set=str(query_set_path),
                out=None,
                collection="test",
                max_docs=2,
                query_limit=1,
                result_limit=1,
                metric_type="L2",
                persist_path=str(probe.repo_root() / "bad.db"),
                rebuild_runs=1,
            )
            report, exit_code = probe.build_report(args)
        self.assertEqual(exit_code, 1)
        self.assertIn("persist_path_inside_source_tree", report["failures"])


if __name__ == "__main__":
    unittest.main()
