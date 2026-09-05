# Archive Center 4.2.0

Archive Center는 RisuAI 대화의 원문과 파생 기억을 로컬에 보존하고, 현재 장면에
관련된 기억과 원작 근거를 다음 요청에 전달하는 로컬 우선 기억 backend입니다.

4.2.0은 사실별 관련성·중요도·RP 턴 최신성을 최종 기억 선택에 반영합니다. 현재
입력과 최근 완결 대화를 검색에 함께 참고하고, 완성 턴 요약과 각 기억 자료 분류에
`핵심 연관 기억 최대 수`를 독립적으로 적용합니다. 완료 사건·재색인·평론가 재개와
HUD 표시도 보완했습니다.

`저장 확정 시점`의 기본값인 **현재 턴**은 응답 직후 평론가·저장을 진행합니다.
**이전 턴**을 선택하면 다음 새 입력에서 직전 최종 응답을 확정하며, 현재 응답 생성과
직전 평론가·저장을 두 진행 카드로 나누어 표시합니다.

변경 사항은 [4.2.0 릴리스 안내](docs/archive-center-4.2.0-release-notes.md), 구현과
검증 이력은 [4.2 작업 기록](docs/archive-center-4.2-work-log.md)에 정리되어 있습니다.

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

기존 관리형 설치는 Archive Center 설정의 **업데이트 확인 → 지금 업데이트**를 사용합니다.
백엔드가 해당 OS·CPU의 패키지를 선택하고, 실행기가 교체·재시작과 준비 상태 확인을
진행합니다. 위 한 줄 명령은 신규 설치 전용이며 기존 설치를 덮어쓰지 않습니다.
[신규 설치 안내](docs/simple-fresh-install.md)를 참고하십시오.

RisuAI에 설치된 `Archive Center.js`는 RisuAI의 플러그인 업데이트 기능이나 새 파일
가져오기로 함께 갱신하십시오. 백엔드 패키지 업데이트만으로 RisuAI에 이미 설치된
플러그인 코드가 교체되지는 않습니다.

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
