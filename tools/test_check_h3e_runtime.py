import json
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import check_h3e_runtime as h3e


class H3ERuntimeReportTests(unittest.TestCase):
    def test_mode_bounds_match_current_contract(self) -> None:
        reactive = h3e.build_initiative_mode_bounds("reactive")
        balanced = h3e.build_initiative_mode_bounds("balanced")
        proactive = h3e.build_initiative_mode_bounds("proactive")

        self.assertEqual(reactive["max_new_beats"], 0)
        self.assertEqual(balanced["max_new_beats"], 1)
        self.assertEqual(proactive["max_new_beats"], 1)
        self.assertFalse(reactive["allow_scene_jump"])
        self.assertIn("observation before action", reactive["emphasis"])
        self.assertIn("bounded steering", proactive["emphasis"])
        self.assertIn("suggest rather than execute", h3e.build_initiative_mode_suffix("balanced"))

    def test_partial_report_keeps_per_mode_timeout_visible(self) -> None:
        report = h3e.build_report(
            "sess-h3e",
            "same input",
            "off",
            5,
            {"ok": True, "status_code": 200, "payload": {"status": "ok"}},
            {
                "reactive": {"ok": False, "status_code": None, "payload": {}, "error": "timed out"},
                "balanced": {
                    "ok": True,
                    "status_code": 200,
                    "payload": {
                        "status": "ok",
                        "source": "shadow",
                        "supervisor_input_pack": {
                            "narrative_stance_bounds": h3e.build_initiative_mode_bounds("balanced")
                        },
                        "trace_summary": {
                            "narrative_stance": "balanced",
                            "narrative_stance_suffix_present": True,
                            "narrative_stance_bounds_present": True,
                            "narrative_stance_summary": {"mode": "balanced", "max_new_beats": 1},
                        },
                    },
                },
                "proactive": {"ok": False, "status_code": None, "payload": {}, "error": "timed out"},
            },
        )

        self.assertEqual(report["status"], "partial")
        self.assertTrue(report["checks"]["health_ok"])
        self.assertTrue(report["checks"]["request_bounds_differ"])
        self.assertFalse(report["checks"]["mode_comparison_available"])
        self.assertEqual(report["comparison"]["ok_modes"], ["balanced"])
        self.assertEqual(set(report["comparison"]["failed_modes"]), {"reactive", "proactive"})
        self.assertEqual(report["modes"]["reactive"]["error"], "timed out")

    def test_ok_report_detects_directive_difference(self) -> None:
        mode_probes = {}
        for mode in h3e.MODES:
            mode_probes[mode] = {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "source": "runtime_llm",
                    "would_call_llm": True,
                    "supervisor_result": {
                        "directive": {
                            "story_author": {
                                "current_arc": f"{mode}_arc",
                                "narrative_goal": f"{mode}_goal",
                            },
                            "director": {
                                "pressure_level": "normal",
                                "required_outcomes": [f"{mode}_outcome"],
                                "forbidden_moves": ["mode-specific guard"],
                            },
                        }
                    },
                    "supervisor_input_pack": {
                        "narrative_stance_bounds": h3e.build_initiative_mode_bounds(mode)
                    },
                    "trace_summary": {
                        "narrative_stance": mode,
                        "narrative_stance_suffix_present": True,
                        "narrative_stance_bounds_present": True,
                        "narrative_stance_summary": {"mode": mode, "max_new_beats": 0 if mode == "reactive" else 1},
                    },
                },
            }

        report = h3e.build_report(
            "sess-h3e",
            "same input",
            "off",
            5,
            {"ok": True, "status_code": 200, "payload": {"status": "ok"}},
            mode_probes,
        )

        self.assertEqual(report["status"], "ok")
        self.assertTrue(report["checks"]["mode_comparison_available"])
        self.assertTrue(report["checks"]["directive_diff_observable"])
        self.assertTrue(report["checks"]["request_bounds_differ"])
        self.assertEqual(report["h3e_checks"]["successful_modes"], ["reactive", "balanced", "proactive"])
        self.assertFalse(report["h3e_checks"]["exact_identical"])
        self.assertEqual(report["h3e_checks"]["fallback_suspected_modes"], [])
        self.assertEqual(report["modes"]["reactive"]["request_bounds"]["max_new_beats"], 0)
        self.assertEqual(report["modes"]["proactive"]["trace"]["summary_mode"], "proactive")

    def test_h3e_checks_detect_fallback_and_identical_directives(self) -> None:
        mode_probes = {}
        for mode in h3e.MODES:
            mode_probes[mode] = {
                "ok": True,
                "status_code": 200,
                "payload": {
                    "status": "ok",
                    "source": "shadow",
                    "would_call_llm": False,
                    "supervisor_result": {
                        "directive": {
                            "story_author": {
                                "current_arc": "same_arc",
                                "narrative_goal": "same_goal",
                            }
                        }
                    },
                    "supervisor_input_pack": {
                        "narrative_stance_bounds": h3e.build_initiative_mode_bounds(mode)
                    },
                    "trace_summary": {"narrative_stance": mode},
                },
            }

        report = h3e.build_report(
            "sess-h3e",
            "same input",
            "off",
            5,
            {"ok": True, "status_code": 200, "payload": {"status": "ok"}},
            mode_probes,
        )

        self.assertEqual(report["status"], "degraded")
        self.assertEqual(report["h3e_checks"]["fallback_suspected_modes"], ["reactive", "balanced", "proactive"])
        self.assertTrue(report["h3e_checks"]["exact_identical"])
        self.assertIn("[H-3e Checks]", h3e.format_h3e_checks(report))
        self.assertIn("successful_modes=reactive,balanced,proactive", h3e.format_h3e_checks(report))

    def test_main_appends_jsonl_and_prints_checks(self) -> None:
        original_run = h3e.run
        try:
            h3e.run = lambda *args, **kwargs: {
                "status": "partial",
                "h3e_checks": {
                    "successful_modes": ["balanced"],
                    "failed_modes": ["reactive", "proactive"],
                    "fallback_suspected_modes": [],
                    "exact_identical": False,
                    "request_bounds_differ": True,
                    "directive_diff_observable": False,
                    "mode_comparison_available": False,
                },
            }
            with tempfile.TemporaryDirectory() as tmp:
                jsonl = Path(tmp) / "h3e_runtime_log.jsonl"
                with redirect_stdout(StringIO()):
                    exit_code = h3e.main(["--append-jsonl", str(jsonl), "--json-only"])
                self.assertEqual(exit_code, 0)
                records = jsonl.read_text(encoding="utf-8").strip().splitlines()
                self.assertEqual(len(records), 1)
                self.assertEqual(json.loads(records[0])["h3e_checks"]["successful_modes"], ["balanced"])
        finally:
            h3e.run = original_run


if __name__ == "__main__":
    unittest.main()
