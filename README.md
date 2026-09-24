# Archive Center

**[Archive Center 4.6.1](https://github.com/Flazer31/archive-center/releases/tag/v4.6.1)**
Archive Center는 RisuAI 대화의 원문과 파생 기억을 로컬에 보존하고, 현재 장면에
관련된 기억과 원작 근거를 다음 요청에 전달하는 로컬 우선 기억 backend입니다.

`저장 확정 시점`의 기본값인 **현재 턴**은 응답 직후 평론가·저장을 진행합니다.
**이전 턴**을 선택하면 다음 새 입력에서 직전 최종 응답을 확정하며, 현재 응답 생성과
직전 평론가·저장을 두 진행 카드로 나누어 표시합니다.

## 현재 버전

이 소스와 패키지의 버전은 **4.6.1**입니다. JS·백엔드·HUD·설치 패키지를 같은 버전으로 사용하세요.
기본 기억은 전처리·출판사 없이도 사용할 수 있습니다. 기존 기억과 설정은 업데이트할 때 유지됩니다.

## 서비스 포트 변경

- Windows: `06_change_port_windows.bat`에서 서비스를 선택합니다.
- Termux 기본 설치: `sh ~/.archive-center/start.sh --configure-ports`.
- Linux/macOS: 기존 실행 명령에 `--configure-ports`를 추가합니다.

포트 입력을 비우고 Enter를 누르면 선택한 서비스의 기본값으로 복원합니다.
ChromaDB **8000**, MariaDB **3307**, Go 백엔드 **28080**입니다.
저장 후 평소 방법으로 재시작하면 DB 실행 포트와 Go 연결 설정에 함께 적용됩니다.
Go 포트를 바꾸면 RisuAI에 저장한 백엔드 URL의 포트도 변경해야 합니다.
기존 DB 위치와 데이터는 유지됩니다.
[최신 버전 다운로드](https://github.com/Flazer31/archive-center/releases/latest).


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

RisuAI에 설치된 `Archive Center.js`는 RisuAI의 플러그인 업데이트 기능이나 새 파일
가져오기로 함께 갱신하십시오. 백엔드 패키지 업데이트만으로 RisuAI에 이미 설치된
플러그인 코드가 교체되지는 않습니다.

Raw `git clone` is a source/operator path. It does not by itself configure
MariaDB, ChromaDB, package launchers, or live service env.

## Docker

릴리즈마다 `ghcr.io/flazer31/archive-center` 이미지가 linux/amd64와
linux/arm64로 빌드됩니다. `compose.yml`은 MariaDB, ChromaDB, 스키마
마이그레이션, Go 백엔드를 한 번에 올립니다.

```sh
docker compose up -d
```

백엔드는 `127.0.0.1:28080`에 바인딩되며 RisuAI의 백엔드 URL을
`http://127.0.0.1:28080`으로 지정합니다. DB 비밀번호는 같은 폴더의 `.env`에
`MARIADB_PASSWORD=...`로 바꿀 수 있습니다. 컨테이너 안에서는 패키지 업데이트가
비활성화됩니다. 새 버전은 이미지를 다시 받아(`docker compose pull`) 적용합니다.

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
