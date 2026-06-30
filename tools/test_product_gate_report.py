import json
import tempfile
import unittest
from pathlib import Path

import product_gate_report as report


class ProductGateReportTests(unittest.TestCase):
    def write_json(self, root: Path, rel: str, payload: dict) -> None:
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(payload), encoding="utf-8")

    def test_report_counts_known_green_gates(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "architecture-runtime-handoff-freeze.md").write_text("ok", encoding="utf-8")
            self.write_json(root, "benchmarks/runtime-startup-rss-r2-110-architecture-handoff.json", {"status": "ok"})
            self.write_json(root, "benchmarks/baseline-capture-r2-110-architecture-handoff.json", {"status": "ok"})
            self.write_json(
                root,
                "benchmarks/managed-mariadb-e2e-r2-108-authority-cutover-replay.json",
                {
                    "status": "ok",
                    "authority_cutover_replay": {
                        "status": "ok",
                        "authority_switch": True,
                        "fallback_available": True,
                    },
                    "route_write_smoke": {"status": "ok"},
                    "rollback_proof": {"status": "ok", "rolled_back": True},
                    "safety_flags": {
                        "authority_switch": True,
                        "mariadb_authority_default_enabled": True,
                        "mariadb_product_read_persisted": True,
                    },
                },
            )
            self.write_json(
                root,
                "benchmarks/windows-live-write-smoke-r2-124.json",
                {
                    "status": "ok",
                    "ready": {
                        "checks": {
                            "store_mode": "mariadb_authority",
                            "mariadb_authority": "enabled",
                        }
                    },
                    "write_smoke": {"status": "ok"},
                },
            )
            self.write_json(
                root,
                "benchmarks/windows-live-stack-smoke-r2-125.json",
                {
                    "status": "ok",
                    "ready": {
                        "checks": {
                            "shadow_mode": "active",
                            "store_mode": "mariadb_authority",
                            "mariadb_authority": "enabled",
                            "milvus": "configured",
                            "milvus_live_enabled": "enabled",
                        }
                    },
                    "write_smoke": {"status": "ok"},
                    "milvus_smoke": {
                        "status": "ok",
                        "summary": {
                            "persisted_milvus_live_enabled": True,
                            "persisted_live_retrieval_enabled": True,
                            "bounded_shadow_route_only": False,
                            "search_result": "ok",
                        },
                    },
                },
            )
            self.write_json(
                root,
                "benchmarks/milvus-r2-product-read-rollback-decommission-2026-05-27-r2-97.json",
                {
                    "status": "ok",
                    "summary": {
                        "search_result": "ok",
                        "rollback_proof_ok": True,
                        "persisted_milvus_live_enabled": True,
                        "persisted_live_retrieval_enabled": True,
                        "bounded_shadow_route_only": False,
                    },
                    "rollback_proof": {"rolled_back": True},
                    "chroma_decommission_proof": {"status": "ok"},
                },
            )
            self.write_json(
                root,
                "benchmarks/managed-mariadb-e2e-r2-107-default-runtime-actual-switch.json",
                {
                    "status": "ok",
                    "default_runtime_switch": {
                        "status": "ok",
                        "go_default_switch": True,
                        "persistent_switch": True,
                        "fallback_available": True,
                    },
                    "rollback_proof": {"status": "ok", "rolled_back": True},
                    "safety_flags": {"go_default_switch": True},
                },
            )
            self.write_json(
                root,
                "benchmarks/backend-surface-smoke-r2-111-decomposition-phase3-phase4-summary.json",
                {"status": "ok", "total_checks": 56, "phase_counts": [{"3": 19}, {"4": 4}]},
            )

            out = report.build_report(root)
            self.assertEqual(out["green_count"], 5)
            self.assertEqual(out["product_cutover_readiness_percent"], 62)
            self.assertEqual(out["status"], "degraded")
            self.assertIn("2.0-5", out["open_gates"])
            self.assertIn("2.0-6", out["open_gates"])

    def test_report_counts_low_resource_gate_when_combined_stack_passes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "architecture-runtime-handoff-freeze.md").write_text("ok", encoding="utf-8")
            self.write_json(root, "benchmarks/runtime-startup-rss-r2-110-architecture-handoff.json", {"status": "ok"})
            self.write_json(
                root,
                "benchmarks/low-resource-go-only-r2-114.json",
                {"status": "ok", "checks": {"startup_ok": True, "idle_rss_ok": True}},
            )
            self.write_json(
                root,
                "benchmarks/combined-stack-low-resource-r2-115.json",
                {
                    "status": "ok",
                    "checks": {
                        "mariadb_ready": True,
                        "milvus_ready": True,
                        "go_ready": True,
                        "go_rss_ok": True,
                        "combined_rss_ok": True,
                        "startup_ok": True,
                        "read_only_latency_ok": True,
                    },
                },
            )

            out = report.build_report(root)
            self.assertEqual(out["green_count"], 1)
            self.assertEqual(out["product_cutover_readiness_percent"], 12)
            self.assertNotIn("2.0-0", out["open_gates"])

    def test_report_counts_packaging_gate_when_real_platform_proof_passes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.write_json(root, "benchmarks/windows-single-file-bundle-r1-102.json", {"status": "ok"})
            self.write_json(
                root,
                "benchmarks/platform-adoption-smoke-r2-121.json",
                {
                    "status": "ok",
                    "product_gate_green": True,
                    "scope": {"runs_installation": True},
                    "summary": {"all_external_proofs_ok": True, "external_proof_conditional": 0},
                },
            )

            out = report.build_report(root)
            self.assertEqual(out["green_count"], 1)
            self.assertNotIn("2.0-5", out["open_gates"])

    def test_report_only_switches_do_not_count_as_product_green(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.write_json(
                root,
                "benchmarks/managed-mariadb-e2e-r2-108-authority-cutover-replay.json",
                {
                    "status": "ok",
                    "authority_cutover_replay": {
                        "status": "ok",
                        "authority_switch": True,
                        "fallback_available": True,
                    },
                    "route_write_smoke": {"status": "ok"},
                    "rollback_proof": {"status": "ok", "rolled_back": True},
                    "safety_flags": {
                        "authority_switch": False,
                        "mariadb_authority_default_enabled": False,
                        "mariadb_product_read_persisted": False,
                    },
                },
            )
            self.write_json(
                root,
                "benchmarks/windows-live-write-smoke-r2-124.json",
                {
                    "status": "ok",
                    "ready": {
                        "checks": {
                            "store_mode": "mariadb_authority",
                            "mariadb_authority": "enabled",
                        }
                    },
                    "write_smoke": {"status": "ok"},
                },
            )
            self.write_json(
                root,
                "benchmarks/managed-mariadb-e2e-r2-107-default-runtime-actual-switch.json",
                {
                    "status": "ok",
                    "default_runtime_switch": {
                        "status": "ok",
                        "go_default_switch": True,
                        "persistent_switch": False,
                        "fallback_available": True,
                    },
                    "rollback_proof": {"status": "ok", "rolled_back": True},
                    "safety_flags": {"go_default_switch": False},
                },
            )
            self.write_json(
                root,
                "benchmarks/milvus-r2-product-read-rollback-decommission-2026-05-27-r2-97.json",
                {
                    "status": "ok",
                    "summary": {
                        "search_result": "ok",
                        "rollback_proof_ok": True,
                        "persisted_milvus_live_enabled": False,
                        "persisted_live_retrieval_enabled": False,
                        "bounded_shadow_route_only": True,
                    },
                    "rollback_proof": {"rolled_back": True},
                    "chroma_decommission_proof": {"status": "ok"},
                },
            )
            self.write_json(
                root,
                "benchmarks/platform-adoption-smoke-r2-121.json",
                {
                    "status": "ok",
                    "product_gate_green": True,
                    "scope": {"runs_installation": False},
                    "summary": {
                        "all_external_proofs_ok": False,
                        "external_proof_conditional": 3,
                    },
                },
            )
            self.write_json(
                root,
                "benchmarks/final-adoption-replay-r2-122.json",
                {
                    "status": "ok",
                    "final_adoption_green": True,
                    "prerequisite_open_gates": [],
                    "scope": {
                        "report_kind": "report_only_legacy",
                        "authority_switch_evidence_green": False,
                        "go_default_switch_evidence_green": False,
                        "milvus_live_switch_evidence_green": False,
                        "python_retirement": False,
                        "python_fallback_retained": True,
                    },
                },
            )

            out = report.build_report(root)
            self.assertEqual(out["green_count"], 0)
            self.assertIn("2.0-2", out["open_gates"])
            self.assertIn("2.0-3", out["open_gates"])
            self.assertIn("2.0-4", out["open_gates"])
            self.assertIn("2.0-5", out["open_gates"])
            self.assertIn("2.0-6", out["open_gates"])


if __name__ == "__main__":
    unittest.main()
