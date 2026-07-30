# Ops

Status: Archive Center 3.6 development packaging, preflight and recovery workspace.

This directory is reserved for local deployment, shakedown, packaging, and recovery recipes.

Runtime data directories, logs, caches, secrets, and backups must stay outside the source workspace.

## Current Scripts

- `install-termux.sh --preflight`: emits a JSON Termux preflight report only. It checks MariaDB provider resolution and ChromaDB Python package availability, but does not install packages, initialize MariaDB, start ChromaDB, switch authority, or modify the 0.8 worktree.
- `install-linux.sh --preflight`: emits a JSON Linux preflight report only. It checks MariaDB provider resolution and ChromaDB Python package availability, but does not install packages, initialize MariaDB, start ChromaDB, switch authority, or modify the 0.8 worktree.
- `install-macos.sh --preflight`: emits a JSON macOS preflight report only. It mirrors the Linux/Termux managed MariaDB/ChromaDB contract and does not initialize runtime state.
- `platform-proof.sh --target auto --out benchmarks/platform-proofs/<target>-adoption-proof.json`: run on a real Linux, Termux, or macOS target to generate the adoption proof consumed by `tools/platform_adoption_smoke.py`. It performs temp bootstrap/install/update/repair/rollback/uninstall lifecycle checks and refuses to become green unless the matching platform preflight is green.
- `tools/platform_adoption_smoke.py --targets linux,termux,macos --out <report.json>`: runs report-only Linux/Termux/macOS bootstrap syntax and preflight contract checks, then imports real proof files or explicit conditional profiles from `benchmarks/platform-proofs/`. It verifies that reports keep `normal_user_manual_mariadb_required=false`; ChromaDB is an optional vector runtime for profile-aware packaging, not a `core_lite` requirement.
- `benchmarks/platform-proofs/linux-assumption-profile.json`: conditional Linux support profile for small-group use when real Linux or Docker-based proof is too heavy on the current PC. It is accepted as conditional support but does not claim real Linux execution proof.
- `benchmarks/platform-proofs/macos-assumption-profile.json`: conditional macOS support profile for small-group use when no macOS device is available. It is accepted as conditional support but does not claim real macOS execution proof.
- `benchmarks/platform-proofs/termux-assumption-profile.json`: conditional Termux support profile for small-group use when no Android/Termux device is available. It treats Termux as Linux-family mobile support but does not claim real Android/Termux execution proof.
- `install-windows.ps1 -Preflight`: emits a JSON Windows preflight report only. It checks managed data paths, Go backend binary/tool availability, local ports, and MariaDB provider resolution without initializing MariaDB, switching authority, or modifying the 0.8 worktree.
- `install-windows.ps1 -InstallMariaDBRuntime -InstallDir <path>`: downloads the pinned official MariaDB 12.3.2 Windows ZIP, verifies its SHA-256, and installs it outside the Archive Center package using the same blocking installer flow as 3.5. No administrator rights or system service registration is required.
- `install-windows.ps1 -StageMariaDBProvider -ProviderArchive <zip> -InstallDir <path>`: legacy/offline path for staging a user-supplied MariaDB archive outside the source and package trees.
- `install-windows.ps1 -VerifyBundle -BundlePath <zip> -Out <report.json>`: verifies that a Windows Archive Center distribution contains the Go backend and required ChromaDB runtime while excluding MariaDB binaries. MariaDB is prepared separately on first start.
- `build-full-package.ps1 -PackageVersion 3.6.0-dev -PackageKind managed -Zip -ForceRefresh`: creates the Windows auto-install package under `_dist`. It builds the Go backend and copies the host adapter, prompts, migrations, notices, and launch scripts, but does not bundle MariaDB, Python, ChromaDB, DB, `.env.full.local`, logs, caches, or user data. First launch downloads the verified official MariaDB and CPython runtimes, installs pinned ChromaDB into the per-user runtime directory, and starts the full local stack. The historical script name is retained so updater and operator paths do not fork.
- `build-posix-managed-packages.ps1 -PackageVersion 3.6.0-dev -Zip -ForceRefresh`: cross-builds the Linux x64/arm64, macOS Intel/Apple Silicon and Android Termux arm64 managed packages.
- `build-live-test-pack.ps1 -ForceRefresh`: creates a lightweight local experiment folder for MariaDB + ChromaDB source validation. It copies only source, prompts, migrations, `Archive Center.js`, and live-test scripts; it excludes runtime data, caches, DB files, generated binaries, backups, release, and deploy outputs.
- `docs/2.0.1-lightweight-runtime-profiles.md`: defines and now partially implements the 2.0.1 profile-based lightweight runtimes across Windows, Linux, macOS, Termux, NAS/Docker, and remote RisuAI clients. Full local ChromaDB is opt-in for low-resource targets.
- `docs/2.0.1-feedback-rollup-and-stabilization.md`: tracks the broader 2.0.1 feedback backlog, including RisuAI save churn from the 1.2-second rollback idle watcher, rollback `partial_error` diagnostics, current-chat session attach/rebind, readiness false-green handling, Windows Defender/package hygiene, and provider configuration clarity.
- `start-windows-live.ps1`: source/dev-only launcher for Windows. It uses a separately supplied MariaDB provider, initializes managed data outside the source tree, applies the schema, then starts `archive-center-go` with `AC_STORE_MODE=mariadb_authority`. It is not a release packaging path.

Note: `start-windows-live.ps1` is retained for source/dev verification. For packaged verification, use the managed auto-install package generated by `build-full-package.ps1`, or the source-only live-test pack generated by `build-live-test-pack.ps1`.

MariaDB provider resolution is installer-owned. Preflight may report `installer_managed_required=true`, but `normal_user_manual_mariadb_required` must remain `false` for the normal product path.
ChromaDB runtime resolution remains profile-aware on POSIX packages. The Windows managed full package is stricter: every local backend profile requires bundled or external ChromaDB, and `client_only` is the only Windows profile that starts no local vector service.
