import unittest
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import check_h2e_runtime as h2e


class H2ERuntimeReportTests(unittest.TestCase):
    def test_report_exposes_stale_filter_and_export_gap(self) -> None:
        report = h2e.build_report(
            "sess-h2e",
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "count": 2,
                    "reference_turn": 10,
                    "storylines": [
                        {"name": "Fresh", "status": "active", "is_stale": False},
                        {"name": "Stale High", "status": "active", "is_stale": True},
                    ],
                },
            },
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "source": "shadow",
                    "supervisor_input_pack": {
                        "storyline_selection": {
                            "policy_version": "storyline_selection.h2d.go.v1",
                            "reference_turn": 10,
                            "total_active_count": 2,
                            "selected_count": 1,
                            "dropped_count": 1,
                            "stale_selected_count": 0,
                            "stale_dropped_count": 1,
                            "fresh_rows_take_priority": True,
                            "selected": [{"name": "Fresh", "is_stale": False}],
                            "dropped": [{"name": "Stale High", "is_stale": True}],
                        }
                    },
                },
            },
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "summary": {"chat_logs_count": 2, "storylines_count": 0},
                    "chat_logs": [{"turn_index": 1}, {"turn_index": 1}],
                    "storylines": [],
                },
            },
        )

        self.assertEqual(report["status"], "ok")
        self.assertTrue(report["checks"]["active_storyline_path_exercised"])
        self.assertTrue(report["checks"]["selection_pressure_exercised"])
        self.assertFalse(report["checks"]["stale_selected"])
        self.assertTrue(report["checks"]["stale_filter_effective"])
        self.assertFalse(report["checks"]["stale_contamination_risk"])
        self.assertFalse(report["checks"]["raw_chat_without_storyline_rows"])
        self.assertEqual(report["supervisor"]["selection"]["dropped_preview"], ["Stale High"])

    def test_report_flags_raw_chat_without_storyline_rows(self) -> None:
        report = h2e.build_report(
            "sess-gap",
            {"ok": True, "status_code": 200, "payload": {"status": "ok", "count": 0, "storylines": []}},
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "supervisor_input_pack": {
                        "storyline_selection": {"selected": [], "dropped": [], "selected_count": 0, "dropped_count": 0}
                    },
                },
            },
            {
                "ok": True,
                "status_code": 200,
                "payload": {"status": "ok", "summary": {"chat_logs_count": 2}, "chat_logs": [{"turn_index": 1}]},
            },
        )

        self.assertEqual(report["status"], "degraded")
        self.assertTrue(report["checks"]["raw_chat_without_storyline_rows"])

    def test_empty_session_is_not_runtime_pass(self) -> None:
        report = h2e.build_report(
            "empty",
            {"ok": True, "status_code": 200, "payload": {"status": "ok", "count": 0, "storylines": []}},
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "supervisor_input_pack": {
                        "storyline_selection": {
                            "selected_count": 0,
                            "dropped_count": 0,
                            "stale_selected_count": 0,
                            "stale_dropped_count": 0,
                            "selected": [],
                            "dropped": [],
                        }
                    },
                },
            },
            {"ok": True, "status_code": 200, "payload": {"status": "ok", "summary": {}, "chat_logs": [], "storylines": []}},
        )

        self.assertEqual(report["status"], "degraded")
        self.assertFalse(report["checks"]["active_storyline_path_exercised"])
        self.assertFalse(report["checks"]["selection_pressure_exercised"])

    def test_stale_selected_marks_contamination_risk(self) -> None:
        report = h2e.build_report(
            "risk",
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "storylines": [{"name": "Stale Selected", "status": "active", "is_stale": True}],
                },
            },
            {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "supervisor_input_pack": {
                        "storyline_selection": {
                            "total_active_count": 1,
                            "selected": [{"name": "Stale Selected", "is_stale": True}],
                            "dropped": [],
                        }
                    },
                },
            },
            {"ok": True, "status_code": 200, "payload": {"status": "ok", "summary": {"chat_logs_count": 0, "storylines_count": 1}}},
        )

        self.assertEqual(report["status"], "degraded")
        self.assertTrue(report["checks"]["stale_selected"])
        self.assertFalse(report["checks"]["stale_filter_effective"])
        self.assertTrue(report["checks"]["stale_contamination_risk"])


if __name__ == "__main__":
    unittest.main()
