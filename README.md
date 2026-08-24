# Archive Center 4.0.0

Archive Center는 RisuAI 대화의 원문과 파생 기억을 로컬에 보존하고, 현재 장면에
관련된 기억과 원작 근거를 다음 요청에 전달하는 로컬 우선 기억 backend입니다.

3.5 누적 릴리스에는 Canon Pack과 Source Discovery, RisuAI source lifecycle 추적,
자료 종류별 독립 기억 예산, 출력 계보와 플로팅 진행 HUD가 포함됩니다. 완료 범위,
실제 UI 제공 상태, 추가 검증이 필요한 부분과 3.6 이후 인계는
[`docs/3.1-3.5-consolidated-release.md`](docs/3.1-3.5-consolidated-release.md)에
정리되어 있습니다. 3.6부터 4.0까지의 정밀 장기 기억 구현 범위는
[`docs/3.6-4.0-precision-long-term-memory-roadmap.md`](docs/3.6-4.0-precision-long-term-memory-roadmap.md)
한 문서만 권위 로드맵으로 사용합니다.

## Runtime Architecture

- `Archive Center.js`: RisuAI hook 관찰, backend 통신, 실제 payload 적용과 HUD/UI
- Go backend: 기억 선택, 예산 조립, source·turn 판정, 저장과 orchestration
- MariaDB: canonical 원문·기억·상태
- ChromaDB: 삭제·재구축 가능한 벡터 검색 보조 계층

JavaScript는 두 번째 backend가 아니며, 기억 정책과 저장 판단은 Go가 소유합니다.

## License

Except where a file or third-party notice states otherwise, Archive Center
source code is licensed under the Mozilla Public License Version 2.0. See
[`LICENSE`](LICENSE) for the complete terms and
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) for separately licensed
dependencies and bundled runtimes.

Binary releases provide the corresponding Archive Center source through the
matching GitHub release tag. User `.env` files, databases, vector collections,
original-work documents, secrets, and other user-provided data are not part of
the project license or source release.

## GitHub Fresh Install

For a new installation, use the one-line entry point for your platform.

POSIX:

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

Updates use a separate path; these entry points never update or overwrite an
existing install. See [`docs/simple-fresh-install.md`](docs/simple-fresh-install.md).

Raw `git clone` is a source/operator path. It does not by itself configure
MariaDB, ChromaDB, package launchers, or live service env. See
`docs/2.3-github-install-update-contract.md`.

## Package and Data Safety

Release packages provide Windows x64, Linux x64/arm64, macOS Intel/Apple
Silicon and Android Termux arm64 builds. Platform packages are cross-built and
inspected here; real-device installation, update and recovery proof remains a
separate release gate where the matching device is unavailable.

User `.env`, API keys, MariaDB or SQLite databases, ChromaDB collections,
original-work documents, chats, logs, caches and runtime state must never be
included in a source or binary release. Only reviewed example configuration is
shipped. Existing user configuration and data are preserved during update.

## Development Validation

From the active source tree:

```powershell
node --check "Archive Center.js"
cd go-service
go test ./... -count=1
```

Runtime ownership rules are documented in
[`docs/permanent-risu-host-backend-boundary.md`](docs/permanent-risu-host-backend-boundary.md).
