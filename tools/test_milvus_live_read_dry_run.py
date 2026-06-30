#!/usr/bin/env python3
"""Unit tests for milvus_live_read_dry_run.py."""

from __future__ import annotations

import argparse
import json
import tempfile
import unittest
from pathlib import Path

import milvus_live_read_dry_run as dryrun


class LiveReadDryRunTests(unittest.TestCase):
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

    def test_state_tracker_initial_false(self) -> None:
        state = dryrun.LiveReadDryRunState()
        self.assertFalse(state.milvus_live_enabled)
        self.assertFalse(state.live_retrieval_enabled)
        self.assertEqual(len(state.state_log), 0)

    def test_state_tracker_simulate_and_rollback(self) -> None:
        state = dryrun.LiveReadDryRunState()
        state.record("initial")
        state.simulate_enable()
        self.assertTrue(state.milvus_live_enabled)
        self.assertTrue(state.live_retrieval_enabled)
        self.assertEqual(state.state_log[-1]["phase"], "simulated_enable")
        state.rollback()
        self.assertFalse(state.milvus_live_enabled)
        self.assertFalse(state.live_retrieval_enabled)
        self.assertEqual(state.state_log[-1]["phase"], "rollback")

    def test_state_tracker_verify_final_ok(self) -> None:
        state = dryrun.LiveReadDryRunState()
        ok, failures = state.verify_final()
        self.assertTrue(ok)
        self.assertEqual(failures, [])
        self.assertEqual(state.state_log[-1]["phase"], "final_verify")
        self.assertTrue(state.state_log[-1]["verified"])

    def test_state_tracker_verify_final_fails_if_enabled(self) -> None:
        state = dryrun.LiveReadDryRunState()
        state.milvus_live_enabled = True
        ok, failures = state.verify_final()
        self.assertFalse(ok)
        self.assertIn("milvus_live_enabled_not_false_after_rollback", failures)

    def test_build_rows(self) -> None:
        rows = dryrun.build_rows(self.sample_query_set(), max_docs=2)
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]["id"], 1)
        self.assertEqual(rows[0]["source_id"], "memories:1")
        self.assertEqual(rows[0]["vector"], [0.1, 0.2])
        self.assertIsInstance(rows[0]["id"], int)

    def test_build_search_plan(self) -> None:
        plan = dryrun.build_search_plan(self.sample_query_set(), max_queries=1)
        self.assertEqual(len(plan), 1)
        self.assertEqual(plan[0]["query_id"], "q1")
        self.assertEqual(plan[0]["vector"], [0.1, 0.2])
        self.assertEqual(plan[0]["source_id"], "memories:1")

    def test_report_with_rollback_failure_is_failed(self) -> None:
        state = dryrun.LiveReadDryRunState()
        state.simulate_enable()
        report, exit_code = dryrun.build_report(
            query_set_path=Path("sample.json"),
            query_set=self.sample_query_set(),
            deps={},
            run_result={"status": "ok"},
            state=state,
            rollback_ok=False,
            rollback_failures=["milvus_live_enabled_not_false_after_rollback"],
            warnings=[],
            failures=["milvus_live_enabled_not_false_after_rollback"],
        )
        self.assertEqual(report["status"], "failed")
        self.assertFalse(report["rollback_verified"])
        self.assertIn("milvus_live_enabled_not_false_after_rollback", report["rollback_failures"])
        self.assertEqual(exit_code, 1)

    def test_report_ok_when_clean(self) -> None:
        state = dryrun.LiveReadDryRunState()
        state.record("initial")
        state.simulate_enable()
        state.rollback()
        report, exit_code = dryrun.build_report(
            query_set_path=Path("sample.json"),
            query_set=self.sample_query_set(),
            deps={},
            run_result={"status": "ok"},
            state=state,
            rollback_ok=True,
            rollback_failures=[],
            warnings=[],
            failures=[],
        )
        self.assertEqual(report["status"], "ok")
        self.assertTrue(report["rollback_verified"])
        self.assertEqual(report["dry_run_scope"], "tool_only")
        self.assertTrue(report["authority_unchanged"])
        self.assertEqual(exit_code, 0)

    def test_persist_path_inside_source_tree_is_blocked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            query_set_path = Path(tmp) / "query_set.json"
            query_set_path.write_text(json.dumps(self.sample_query_set()), encoding="utf-8")
            temp_dir = dryrun.repo_root() / "bad_temp"
            self.assertTrue(dryrun.path_inside(temp_dir, dryrun.repo_root()))

    def test_non_goals_include_safety_items(self) -> None:
        report, _ = dryrun.build_report(
            query_set_path=Path("sample.json"),
            query_set=self.sample_query_set(),
            deps={},
            run_result={"status": "ok"},
            state=dryrun.LiveReadDryRunState(),
            rollback_ok=True,
            rollback_failures=[],
            warnings=[],
            failures=[],
        )
        non_goals = report["non_goals"]
        self.assertIn("does_not_persist_live_flag", non_goals)
        self.assertIn("does_not_retire_chroma", non_goals)
        self.assertIn("does_not_authorize_mariadb_truth", non_goals)
        self.assertIn("does_not_enable_go_default_runtime", non_goals)

    def test_load_query_set_rejects_empty_queries(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "bad.json"
            path.write_text(json.dumps({"queries": []}), encoding="utf-8")
            with self.assertRaises(ValueError):
                dryrun.load_query_set(path)

    def test_get_hit_source_id_entity_shape(self) -> None:
        self.assertEqual(
            dryrun.get_hit_source_id({"entity": {"source_id": "memories:1"}}),
            "memories:1",
        )

    def test_get_hit_source_id_flat_shape(self) -> None:
        self.assertEqual(
            dryrun.get_hit_source_id({"source_id": "memories:1"}),
            "memories:1",
        )

    def test_get_hit_source_id_none(self) -> None:
        self.assertIsNone(dryrun.get_hit_source_id(None))


if __name__ == "__main__":
    unittest.main()
