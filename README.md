# Archive Center 4.1.0

Archive Center는 RisuAI 대화의 원문과 파생 기억을 로컬에 보존하고, 현재 장면에
관련된 기억과 원작 근거를 다음 요청에 전달하는 로컬 우선 기억 backend입니다.

4.1.0은 같은 논리 요청의 provider 재시도 문맥 재사용, RisuAI 사용자 행 기준
리롤·편집 재생성 교체, Windows Ctrl+C 종료 취소, 선택형 PDF 기억 전달,
Yumi Provider Manager PDF 표식 경로와 Yumi Translator 1.4.2 원문 읽기 호환을
포함합니다. 세부 구현·검증 범위는
[`docs/archive-center-4.1.0-work-log.md`](docs/archive-center-4.1.0-work-log.md)에
정리되어 있습니다.

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

Windows 릴리스 ZIP을 직접 받은 사용자는 빈 폴더에 압축을 풀고
`01_start_archive_center_windows.bat`를 실행하면 필요한 런타임 준비와 백엔드
시작이 이어집니다.

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
