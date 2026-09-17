# Archive Center Repository Structure

This map describes the public 4.5.0 source. Internal research, third-party example
analysis, work journals, test-run reports and separate plugin projects are kept
outside the public source tree. See [public release scope](docs/public-release-scope.md).

## Development history and publication

Historical investigation, version comparisons and recovery use local Git
commits/tags and backup bundles. The active development repository keeps its
ancestry locally and does not push to GitHub. A separate publication repository
contains only reviewed release snapshots; never merge development ancestry or
restore old public refs. Follow the [publication rules](docs/public-release-scope.md).

## Runtime ownership

- [Archive Center.js](Archive%20Center.js): RisuAI observations, backend transport,
  payload application, displayed-output confirmation and settings/HUD rendering.
- [go-service](go-service/): memory retrieval and selection, prompt/budget assembly,
  turn lifecycle, provider orchestration, persistence and backend ViewModels.
- MariaDB holds canonical chat, memory and state. ChromaDB is a rebuildable vector index.
- [Host/backend boundary](docs/permanent-risu-host-backend-boundary.md) defines ownership.

## Go service

- [cmd/archive-center-go](go-service/cmd/archive-center-go/): backend entrypoint.
- [internal/httpapi](go-service/internal/httpapi/): registered HTTP routes, preparation,
  completion, memory delivery, settings, provider calls and workflow diagnostics.
- [internal/store](go-service/internal/store/): persistence and migration-facing storage.
- [internal/vector](go-service/internal/vector/): vector service integration.
- [internal/diagnostics](go-service/internal/diagnostics/): console/file diagnostics.
- [cmd/archive-center-updater](go-service/cmd/archive-center-updater/): managed updates.
- [cmd/mariadb-schema](go-service/cmd/mariadb-schema/): SQL bootstrap/migrations.

## Data and contracts

- [migrations](migrations/): complete numbered SQL migration inventory.
- [prompts](prompts/): versioned Publisher and Critic default prompts.
- [contracts](contracts/): API/storage contracts and schema tooling inputs.
- [Canon Pack manifest](docs/canon-pack-manifest-v1.md): format, schema and synthetic example.
- [.env.example](.env.example): public configuration template, without user credentials.

## Installation and release

- [install.sh](install.sh) and [install-windows.ps1](install-windows.ps1): public fresh-install entrypoints.
- [scripts](scripts/): installation, bootstrap and regression verification helpers.
- [ops](ops/): package launchers, builders and operational tools.
- [ops/build-release-assets.ps1](ops/build-release-assets.ps1): Windows and POSIX release archives/checksums.
- [.github/workflows](.github/workflows/): CI and installation/update verification.
- [Fresh install](docs/simple-fresh-install.md), [ports](docs/chromadb-port-configuration.md),
  [Oracle/systemd](docs/oracle-cloud-systemd-update-guide.md) and
  [diagnostics](docs/archive-center-4.5-detailed-logging.md): user-facing operation guides.

## Validation and licensing

- [tests/README.md](tests/README.md): source tests and evidence boundaries.
- Production Go tests remain beside their owners; [testdata](testdata/) and
  [tests/fixtures](tests/fixtures/) contain regression inputs, not user conversations.
- [tools](tools/): schema, migration and platform validation tools.
- [LICENSE](LICENSE), [NOTICE](NOTICE), [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
  and [licenses](licenses/) remain part of source/package distribution.

The public source cleanup changes no JavaScript or Go runtime behavior (JS +0/-0).
The seven published 4.5.0 installation/update ZIPs retain their original hashes.
Their build-source commit remains recorded in the [release verification](docs/archive-center-4.5-release-verification.md).
