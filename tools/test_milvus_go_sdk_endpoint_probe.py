#!/usr/bin/env python3
"""Unit tests for milvus_go_sdk_endpoint_probe.py."""

from __future__ import annotations

import argparse
import tempfile
import unittest
from pathlib import Path

import milvus_go_sdk_endpoint_probe as probe


class MilvusGoSDKEndpointProbeTests(unittest.TestCase):
    def test_path_inside_detects_source_tree_child(self) -> None:
        root = probe.repo_root()
        self.assertTrue(probe.path_inside(root / "tools", root))
        self.assertFalse(probe.path_inside(probe.workspace_root() / ".runtime-cache", root))

    def test_find_free_port_returns_connectable_integer(self) -> None:
        port = probe.find_free_port()
        self.assertIsInstance(port, int)
        self.assertGreater(port, 0)

    def test_missing_runtime_returns_failed_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            args = argparse.Namespace(
                runtime_python=str(Path(tmp) / "missing-python"),
                data_root=str(Path(tmp) / "data"),
                go_cache=str(Path(tmp) / "go-cache"),
                port=0,
                dimension=4,
                wait_timeout_s=0.1,
                go_timeout_s=0.1,
                cleanup=False,
                query_set=None,
            )
            report, exit_code = probe.run_probe(args)
        self.assertEqual(exit_code, 1)
        self.assertEqual(report["status"], "failed")
        self.assertIn("runtime_python_missing", report["failures"])

    def test_data_root_inside_source_tree_is_blocked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fake_python = Path(tmp) / "python"
            fake_python.write_text("", encoding="utf-8")
            args = argparse.Namespace(
                runtime_python=str(fake_python),
                data_root=str(probe.repo_root() / "bad-milvus-data"),
                go_cache=str(Path(tmp) / "go-cache"),
                port=0,
                dimension=4,
                wait_timeout_s=0.1,
                go_timeout_s=0.1,
                cleanup=False,
                query_set=None,
            )
            report, exit_code = probe.run_probe(args)
        self.assertEqual(exit_code, 1)
        self.assertIn("data_root_inside_source_tree", report["failures"])


if __name__ == "__main__":
    unittest.main()
