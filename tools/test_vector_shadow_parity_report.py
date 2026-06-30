#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("vector_shadow_parity_report.py")
SPEC = importlib.util.spec_from_file_location("vector_shadow_parity_report", MODULE_PATH)
assert SPEC is not None
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class VectorShadowParityReportTests(unittest.TestCase):
    def test_extract_ids_accepts_chroma_nested_ids(self) -> None:
        payload = {"ids": [["memory:1", "episode:2"]]}
        self.assertEqual(MODULE.extract_ids(payload), ["memory:1", "episode:2"])

    def test_extract_ids_accepts_milvus_search_dicts(self) -> None:
        payload = [[{"id": "memory:1"}, {"id": "episode:2"}]]
        self.assertEqual(MODULE.extract_ids(payload), ["memory:1", "episode:2"])

    def test_build_report_passes_on_overlap_threshold(self) -> None:
        payload = {
            "queries": [
                {
                    "query_id": "q1",
                    "chroma_ids": ["a", "b", "c"],
                    "milvus_ids": ["a", "b", "c"],
                }
            ]
        }
        report, code = MODULE.build_report(payload, 0.8, 20)
        self.assertEqual(code, 0)
        self.assertEqual(report["status"], "ok")
        self.assertEqual(report["queries"][0]["jaccard"], 1.0)

    def test_build_report_fails_on_low_overlap(self) -> None:
        payload = {
            "queries": [
                {
                    "query_id": "q1",
                    "chroma_ids": ["a", "b", "c"],
                    "milvus_ids": ["x", "y", "z"],
                }
            ]
        }
        report, code = MODULE.build_report(payload, 0.8, 20)
        self.assertEqual(code, 1)
        self.assertEqual(report["status"], "fail")
        self.assertEqual(report["queries"][0]["status"], "fail")

    def test_cli_writes_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            input_path = Path(tmp) / "input.json"
            output_path = Path(tmp) / "report.json"
            input_path.write_text(
                json.dumps(
                    {
                        "queries": [
                            {
                                "query_id": "q1",
                                "chroma": {"ids": [["a", "b"]]},
                                "milvus": [[{"id": "a"}, {"id": "b"}]],
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            completed = subprocess.run(
                [
                    sys.executable,
                    "-B",
                    str(MODULE_PATH),
                    "--input",
                    str(input_path),
                    "--out",
                    str(output_path),
                ],
                check=False,
                text=True,
                capture_output=True,
            )
            self.assertEqual(completed.returncode, 0, completed.stderr)
            report = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "ok")


if __name__ == "__main__":
    unittest.main()
