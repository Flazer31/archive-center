#!/usr/bin/env python3
"""Unit tests for milvus_runtime_bootstrap.py."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

import milvus_runtime_bootstrap as bootstrap


class MilvusRuntimeBootstrapTests(unittest.TestCase):
    def test_default_runtime_dir_is_outside_repo(self) -> None:
        runtime_dir = bootstrap.default_runtime_dir()
        self.assertFalse(bootstrap.path_inside(runtime_dir, bootstrap.repo_root()))
        self.assertIn(".runtime-cache", str(runtime_dir))

    def test_runtime_python_windows_or_posix(self) -> None:
        runtime = Path("runtime")
        py = bootstrap.runtime_python(runtime)
        self.assertTrue(str(py).endswith("python") or str(py).endswith("python.exe"))

    def test_build_report_blocks_source_tree_runtime(self) -> None:
        args = mock.Mock(
            runtime_dir=str(bootstrap.repo_root() / "runtime" / "bad"),
            install=False,
            smoke=False,
            package=list(bootstrap.DEFAULT_PACKAGES),
            install_timeout=1,
            check_timeout=1,
            smoke_timeout=1,
        )
        report, exit_code = bootstrap.build_report(args)
        self.assertEqual(exit_code, 1)
        self.assertIn("runtime_dir_inside_source_tree", report["failures"])
        self.assertTrue(report["safety_flags"]["runtime_dir_inside_source_tree"])

    def test_missing_runtime_degrades_without_live_flags(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            args = mock.Mock(
                runtime_dir=str(Path(tmp) / "missing-runtime"),
                install=False,
                smoke=False,
                package=list(bootstrap.DEFAULT_PACKAGES),
                install_timeout=1,
                check_timeout=1,
                smoke_timeout=1,
            )
            report, exit_code = bootstrap.build_report(args)
        self.assertEqual(exit_code, 0)
        self.assertEqual(report["status"], "degraded")
        self.assertIn("managed_runtime_missing", report["warnings"])
        self.assertFalse(report["safety_flags"]["live_retrieval_enabled"])
        self.assertFalse(report["safety_flags"]["milvus_live_enabled"])


if __name__ == "__main__":
    unittest.main()
