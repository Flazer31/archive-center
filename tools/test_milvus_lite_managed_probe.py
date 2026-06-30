#!/usr/bin/env python3
"""Unit tests for milvus_lite_managed_probe.py."""

from __future__ import annotations

import unittest

import milvus_lite_managed_probe as probe


class MilvusLiteManagedProbeTests(unittest.TestCase):
    def sample_query_set(self) -> dict:
        return {
            "result_limit": 3,
            "queries": [
                {
                    "query_id": "q1",
                    "source_id": "memories:1",
                    "embedding": [0.1, 0.2],
                    "tier": "memory",
                    "chat_session_id": "session-a",
                    "source_table": "memories",
                    "source_row_id": "1",
                    "document_excerpt": "first memory",
                },
                {
                    "query_id": "q2",
                    "id": "kg:2",
                    "embedding": [0.3, 0.4],
                    "tier": "kg",
                    "chat_session_id": "session-a",
                    "source_table": "kg_triples",
                    "source_row_id": "2",
                },
            ],
        }

    def test_build_docs_preserves_reference_source_id(self) -> None:
        docs = probe.build_docs(self.sample_query_set())

        self.assertEqual(len(docs), 2)
        self.assertEqual(docs[0]["id"], "memories:1")
        self.assertEqual(docs[0]["source_id"], "memories:1")
        self.assertEqual(docs[1]["id"], "kg:2")
        self.assertEqual(docs[1]["source_id"], "kg:2")

    def test_build_search_plan_keeps_source_id_for_self_check(self) -> None:
        plan = probe.build_search_plan(self.sample_query_set(), max_queries=2)

        self.assertEqual(plan[0]["source_id"], "memories:1")
        self.assertEqual(plan[1]["source_id"], "kg:2")
        self.assertEqual(plan[0]["limit"], 3)

    def test_get_hit_source_id_prefers_source_id_over_milvus_pk(self) -> None:
        self.assertEqual(
            probe.get_hit_source_id({"id": 1, "entity": {"id": 1, "source_id": "memories:1"}}),
            "memories:1",
        )
        self.assertEqual(probe.get_hit_source_id({"id": 2}), "2")


if __name__ == "__main__":
    unittest.main()
