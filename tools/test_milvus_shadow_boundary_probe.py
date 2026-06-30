#!/usr/bin/env python3
"""Unit tests for milvus_shadow_boundary_probe.py."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import milvus_shadow_boundary_probe as probe


class BoundaryProbeTests(unittest.TestCase):
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

    def test_build_boundary_payload(self) -> None:
        payload = probe.build_boundary_payload(self.sample_query_set(), max_docs=2, query_index=0, limit=3)
        self.assertEqual(payload["chat_session_id"], "sess-1")
        self.assertTrue(payload["dry_run"])
        self.assertTrue(payload["allow_shadow_boundary"])
        self.assertEqual(len(payload["docs"]), 2)
        self.assertEqual(payload["docs"][0]["id"], "memories:1")
        self.assertEqual(payload["docs"][0]["embedding"], [0.1, 0.2])
        self.assertEqual(payload["query_vector"], [0.1, 0.2])
        self.assertEqual(payload["limit"], 3)

    def test_build_report_ok_requires_disabled_live_flags(self) -> None:
        request_payload = probe.build_boundary_payload(self.sample_query_set(), max_docs=1, query_index=0, limit=5)
        report, exit_code = probe.build_report(
            query_set_path=Path("sample.json"),
            query_set=self.sample_query_set(),
            endpoint="http://127.0.0.1:28080/milvus-shadow/backfill-compare",
            request_payload=request_payload,
            http_status=200,
            response_payload={
                "status": "ok",
                "live_retrieval_enabled": False,
                "milvus_live_enabled": False,
                "boundary_exercised": True,
                "backfill_doc_count": 1,
            },
            transport_error=None,
            elapsed_ms=1.2,
        )
        self.assertEqual(exit_code, 0)
        self.assertEqual(report["status"], "ok")
        self.assertEqual(report["source_evidence"]["source_mode"], "sqlite_real_memory_embeddings")

    def test_build_report_fails_if_live_flag_is_true(self) -> None:
        request_payload = probe.build_boundary_payload(self.sample_query_set(), max_docs=1, query_index=0, limit=5)
        report, exit_code = probe.build_report(
            query_set_path=Path("sample.json"),
            query_set=self.sample_query_set(),
            endpoint="http://127.0.0.1:28080/milvus-shadow/backfill-compare",
            request_payload=request_payload,
            http_status=200,
            response_payload={
                "status": "ok",
                "live_retrieval_enabled": True,
                "milvus_live_enabled": False,
                "boundary_exercised": True,
                "backfill_doc_count": 1,
            },
            transport_error=None,
            elapsed_ms=1.2,
        )
        self.assertEqual(exit_code, 1)
        self.assertEqual(report["status"], "fail")
        self.assertIn("live_retrieval_not_false", report["failures"])

    def test_load_query_set_rejects_empty_queries(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "bad.json"
            path.write_text(json.dumps({"queries": []}), encoding="utf-8")
            with self.assertRaises(ValueError):
                probe.load_query_set(path)


if __name__ == "__main__":
    unittest.main()
