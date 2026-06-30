import json
import tempfile
import unittest
from pathlib import Path

import final_adoption_replay as replay


class FinalAdoptionReplayTests(unittest.TestCase):
    def write_json(self, root: Path, rel: str, payload: dict) -> None:
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(payload), encoding="utf-8")

    def seed_green_foundation(self, root: Path) -> None:
        (root / "docs").mkdir(parents=True, exist_ok=True)
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
            },
        )
        self.write_json(
            root,
            "benchmarks/milvus-r2-product-read-rollback-decommission-2026-05-27-r2-97.json",
            {
                "status": "ok",
                "summary": {"search_result": "ok", "rollback_proof_ok": True},
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
                    "fallback_available": True,
                },
                "rollback_proof": {"status": "ok", "rolled_back": True},
            },
        )
        self.write_json(
            root,
            "benchmarks/backend-surface-smoke-r2-111-decomposition-phase3-phase4-summary.json",
            {"status": "ok", "total_checks": 56, "phase_counts": [{"3": 19}, {"4": 4}]},
        )

    def test_report_degraded_when_prerequisite_gates_and_signoff_are_open(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.seed_green_foundation(root)

            out = replay.build_report(root)

            self.assertEqual(out["status"], "degraded")
            self.assertFalse(out["final_adoption_green"])
            self.assertIn("2.0-5", out["prerequisite_open_gates"])
            self.assertIn("operator_signoff_missing", out["open_blockers"])

    def test_operator_signoff_requires_rollback_acceptance(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            signoff = root / "benchmarks" / "final-adoption-signoff.json"
            self.write_json(root, "benchmarks/final-adoption-signoff.json", {"status": "ok", "operator_signoff": True})

            check = replay.check_operator_signoff(signoff)

            self.assertFalse(check["ok"])
            self.assertIn("rollback_plan_not_accepted", check["failures"])

    def test_actual_scope_requires_green_gates_and_signoff(self) -> None:
        check = replay.check_actual_adoption_scope(
            {"ok": False, "prerequisite_open_gates": ["2.0-5"]},
            {"ok": False},
        )

        self.assertFalse(check["ok"])
        self.assertIn("product_gate_prerequisites_open", check["failures"])
        self.assertIn("operator_signoff_not_accepted", check["failures"])

    def test_actual_scope_accepts_green_gates_and_signoff(self) -> None:
        check = replay.check_actual_adoption_scope(
            {"ok": True, "prerequisite_open_gates": []},
            {"ok": True},
        )

        self.assertTrue(check["ok"])
        self.assertEqual(check["failures"], [])
        self.assertTrue(check["accepted_from_evidence"])

    def test_build_report_uses_evidence_green_scope_names(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.seed_green_foundation(root)
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
            self.write_json(
                root,
                "benchmarks/windows-live-write-smoke-r2-124.json",
                {
                    "status": "ok",
                    "ready": {"checks": {"store_mode": "mariadb_authority", "mariadb_authority": "enabled"}},
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
                "benchmarks/windows-single-file-bundle-r1-102.json",
                {"status": "ok"},
            )
            self.write_json(
                root,
                "benchmarks/platform-adoption-smoke-r2-121.json",
                {
                    "status": "ok",
                    "product_gate_green": True,
                    "scope": {
                        "normal_user_manual_mariadb_required": False,
                        "normal_user_manual_milvus_required": False,
                    },
                    "summary": {
                        "all_platform_requirements_ok": True,
                        "external_proof_accepted": 3,
                        "external_proof_total": 3,
                    },
                },
            )
            self.write_json(
                root,
                "benchmarks/final-adoption-signoff.json",
                {"status": "ok", "operator_signoff": True, "accepts_rollback_plan": True},
            )

            out = replay.build_report(root)
            scope = out["scope"]

            self.assertTrue(out["final_adoption_green"])
            self.assertTrue(scope["authority_switch_evidence_green"])
            self.assertTrue(scope["go_default_switch_evidence_green"])
            self.assertTrue(scope["milvus_live_switch_evidence_green"])
            self.assertNotIn("authority_switch", scope)
            self.assertFalse(scope["python_retirement"])
            self.assertTrue(scope["python_fallback_retained"])


if __name__ == "__main__":
    unittest.main()
