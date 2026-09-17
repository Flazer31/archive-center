import platform_adoption_smoke as smoke


def test_contract_requires_manual_install_flags_false():
    report = {
        "mariadb": {
            "normal_user_manual_mariadb_required": False,
            "provider_mode": "installer_package_required",
            "installer_managed_required": True,
            "required_action": "installer does it",
        },
        "chromadb": {
            "normal_user_manual_chromadb_required": False,
            "provider_mode": "installer_python_package_required",
            "installer_managed_required": True,
            "required_action": "installer does it",
        },
    }

    assert smoke.contract_from_report(report)["status"] == "ok"


def test_contract_fails_when_chromadb_manual_flag_missing():
    report = {
        "mariadb": {
            "normal_user_manual_mariadb_required": False,
            "provider_mode": "bundled_runtime",
            "installer_managed_required": False,
            "required_action": "use bundled",
        },
        "chromadb": {
            "provider_mode": "missing",
            "installer_managed_required": True,
            "required_action": "installer does it",
        },
    }

    contract = smoke.contract_from_report(report)

    assert contract["status"] == "failed"
    assert "normal_user_manual_chromadb_required_not_false" in contract["failures"]


def test_summarize_and_degraded_status_for_host_red_reports():
    targets = [
        {
            "syntax_status": "ok",
            "preflight_json_valid": True,
            "preflight_support_level": "red",
            "contract": {"status": "ok"},
        },
        {
            "syntax_status": "ok",
            "preflight_json_valid": True,
            "preflight_support_level": "red",
            "contract": {"status": "ok"},
        },
    ]
    external = [
        {"target": "linux", "status": "missing"},
        {"target": "termux", "status": "missing"},
        {"target": "macos", "status": "missing"},
    ]

    summary = smoke.summarize(targets, external)

    assert summary["all_contracts_ok"] is True
    assert summary["platform_red"] == 2
    assert summary["all_external_proofs_ok"] is False
    assert summary["all_platform_requirements_ok"] is False
    assert smoke.determine_status(summary) == "degraded"


def test_external_proof_requires_lifecycle_checks():
    report = {
        "target": "linux",
        "platform": "Linux",
        "platform_detail": "GNU/Linux",
        "support_level": "green",
        "preflight_status": "ok",
        "mariadb": {
            "normal_user_manual_mariadb_required": False,
            "provider_mode": "bundled_runtime",
            "installer_managed_required": False,
            "required_action": "use bundled",
        },
        "chromadb": {
            "normal_user_manual_chromadb_required": False,
            "provider_mode": "python_local_package",
            "installer_managed_required": False,
            "required_action": "use managed runtime",
        },
        "lifecycle": {
            "bootstrap": "ok",
            "install": "ok",
            "update": "ok",
            "repair": "ok",
            "uninstall": "ok",
        },
    }

    lifecycle = smoke.lifecycle_from_report(report)

    assert lifecycle["status"] == "failed"
    assert "rollback_proof_missing_or_not_ok" in lifecycle["failures"]


def test_external_proof_can_pass_with_real_platform_lifecycle():
    report = {
        "target": "linux",
        "platform": "Linux",
        "platform_detail": "GNU/Linux",
        "support_level": "green",
        "preflight_status": "ok",
        "mariadb": {
            "normal_user_manual_mariadb_required": False,
            "provider_mode": "bundled_runtime",
            "installer_managed_required": False,
            "required_action": "use bundled",
        },
        "chromadb": {
            "normal_user_manual_chromadb_required": False,
            "provider_mode": "python_local_package",
            "installer_managed_required": False,
            "required_action": "use managed runtime",
        },
        "lifecycle": {
            "bootstrap": "ok",
            "install": "ok",
            "update": "ok",
            "repair": "ok",
            "uninstall": "ok",
            "rollback": "ok",
        },
    }

    assert smoke.platform_match("linux", report) is True
    assert smoke.contract_from_report(report)["status"] == "ok"
    assert smoke.lifecycle_from_report(report)["status"] == "ok"


def test_macos_assumption_profile_can_be_gate_accepted(tmp_path):
    proof_dir = tmp_path
    (proof_dir / "macos-assumption-profile.json").write_text(
        """{
          "target": "macos",
          "status": "ok",
          "proof_kind": "assumption_profile",
          "conditional_support": true,
          "accepted_without_real_device": true,
          "support_level": "conditional",
          "preflight_status": "assumed",
          "mariadb": {
            "normal_user_manual_mariadb_required": false,
            "provider_mode": "homebrew_or_bundled_runtime_expected",
            "installer_managed_required": true,
            "required_action": "installer managed"
          },
          "chromadb": {
            "normal_user_manual_chromadb_required": false,
            "provider_mode": "managed_python_package_expected",
            "installer_managed_required": true,
            "required_action": "installer managed"
          },
          "assumptions": ["macOS conditional support"],
          "limitations": ["not real macOS execution"]
        }""",
        encoding="utf-8",
    )

    proof = smoke.load_external_proof(proof_dir, "macos")

    assert proof["status"] == "conditional"
    assert proof["gate_accepted"] is True
    assert proof["real_platform_proof_failures"] == ["real_platform_proof_missing"]


def test_linux_assumption_profile_can_be_gate_accepted(tmp_path):
    proof_dir = tmp_path
    (proof_dir / "linux-assumption-profile.json").write_text(
        """{
          "target": "linux",
          "status": "ok",
          "proof_kind": "assumption_profile",
          "conditional_support": true,
          "accepted_without_real_device": true,
          "support_level": "conditional",
          "preflight_status": "assumed",
          "mariadb": {
            "normal_user_manual_mariadb_required": false,
            "provider_mode": "linux_package_or_bundled_runtime_expected",
            "installer_managed_required": true,
            "required_action": "installer managed"
          },
          "chromadb": {
            "normal_user_manual_chromadb_required": false,
            "provider_mode": "managed_python_package_or_approved_vector_fallback_expected",
            "installer_managed_required": true,
            "required_action": "installer managed"
          },
          "assumptions": ["Linux conditional support for small-group adoption"],
          "limitations": ["not real Linux execution"]
        }""",
        encoding="utf-8",
    )

    proof = smoke.load_external_proof(proof_dir, "linux")

    assert proof["status"] == "conditional"
    assert proof["gate_accepted"] is True
    assert proof["real_platform_proof_failures"] == ["real_platform_proof_missing"]


def test_termux_assumption_profile_can_be_gate_accepted(tmp_path):
    proof_dir = tmp_path
    (proof_dir / "termux-assumption-profile.json").write_text(
        """{
          "target": "termux",
          "status": "ok",
          "proof_kind": "assumption_profile",
          "conditional_support": true,
          "accepted_without_real_device": true,
          "support_level": "conditional",
          "preflight_status": "assumed",
          "mariadb": {
            "normal_user_manual_mariadb_required": false,
            "provider_mode": "termux_pkg_or_linux_family_bootstrap_expected",
            "installer_managed_required": true,
            "required_action": "installer managed"
          },
          "chromadb": {
            "normal_user_manual_chromadb_required": false,
            "provider_mode": "managed_python_package_or_approved_vector_fallback_expected",
            "installer_managed_required": true,
            "required_action": "installer managed"
          },
          "assumptions": ["Termux is Linux-family mobile support"],
          "limitations": ["not real Android/Termux execution"]
        }""",
        encoding="utf-8",
    )

    proof = smoke.load_external_proof(proof_dir, "termux")

    assert proof["status"] == "conditional"
    assert proof["gate_accepted"] is True
    assert proof["real_platform_proof_failures"] == ["real_platform_proof_missing"]
