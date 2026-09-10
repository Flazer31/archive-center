# Archive Center

Archive Center는 RisuAI 대화의 원문과 파생 기억을 로컬에 보존하고, 현재 장면에
관련된 기억과 원작 근거를 다음 요청에 전달하는 로컬 우선 기억 backend입니다.

4.3.1은 검색한 과거 기억이 최종 선정 전에 빠지는 경로를 고치고, 입력 단서와
요약 중요도를 보존합니다. 선택형 전처리 편집자와 기본 기억에 함께 적용되며,
긴 입력의 맥락 조립 지연과 서비스 포트 설정도 개선합니다. 출판사·평론가·편집자의
제공자와 모델을 따로 설정할 수 있으며 OpenRouter와 OpenCode Zen·Go도 지원합니다.

`저장 확정 시점`의 기본값인 **현재 턴**은 응답 직후 평론가·저장을 진행합니다.
**이전 턴**을 선택하면 다음 새 입력에서 직전 최종 응답을 확정하며, 현재 응답 생성과
직전 평론가·저장을 두 진행 카드로 나누어 표시합니다.

변경 사항은 [4.3.1 릴리스 안내](docs/archive-center-4.3.1-release-notes.md),
검증 결과는 [4.3.1 배포 기록](docs/archive-center-4.3.1-release-verification.md)에 정리합니다.

## 현재 버전과 작업 문서

2026-09-10 **[4.3.1 정식 버전](https://github.com/Flazer31/archive-center/releases/tag/v4.3.1)**을 공개했습니다.
검증한 test.4의 기억 회수·서비스 포트·맥락 조립 수정을 유지합니다.
[구현·검증 범위](docs/archive-center-memory-recall-restoration-plan.md)와
[설치·이전 버전 업데이트 검사](docs/archive-center-4.3.1-install-update-verification.md)를 참고하세요.
OS별 ZIP 7개와 체크섬, Windows·Linux·macOS CI 4개 작업을 확인했습니다.

**이후 기억 작업의 기준은 4.3.1로 고정합니다.** [보존 기준](docs/archive-center-memory-recall-restoration-plan.md#memory-baseline-431)에
회수 폭·검색 근거·선정·중요도·전처리 선택 동작을 정리합니다. 기본 기억만 사용할 때도 이 기준을
유지하며 이후 버전은 4.3.1 및 직전 검증 버전과 비교합니다. 현재/과거 상태와 해석 문제는 계속 개선합니다.

4.1·4.2·4.3 이전 updater에서 최종 4.3.1 패키지의 적용·복원을 검사했습니다.
공개 후에는 별도 Windows 환경에서 실제 GitHub 4.3.0→4.3.1 다운로드·적용·재시작도 확인했습니다.
아래 설치·업데이트 명령은 GitHub의 최신 공개 릴리스를 사용합니다.

- [4.3 현황과 남은 확인](docs/archive-center-4.3-status-summary.md)
- [test.23 파일·적용 안내·검증 기록](docs/archive-center-4.3-test-build-23.md)
- [소스 구조와 기능 소유자](STRUCTURE.md), [작업 규칙](AI_GUARDRAILS.md)
- [4.1–9.0 통합 로드맵](../_archive/future-reference/4.1-9.0-integrated-roadmap.md), [4.4 실행 계획](docs/archive-center-4.4-refactoring-plan.md)
- [검사 위치](tests/README.md), [빌드·설치 도구](ops/README.md)

4.3에는 선택적으로 사용하는 5개 전처리 담당, 기본 기억의 의미 점수 전달·중요도 보존,
인물별 지식 연결 보정, 하이파 원문별 가져오기와 간결한 HUD가 포함된다. test.21은
분류별 핵심 우선 수를 확보한 뒤 남은 문자 예산에 세부사항을 담고, 일반 주관 기억을
정상 선정으로 돌린다. 현재 필드의 출처 시점과 검색 순위를 분리하며, 전처리 추가 검색의
새 출처를 보존한다. 기본 기억 보완은 전처리·출판사를 끈 상태에도 적용된다.
test.22는 검색 질문 형식 보정, 담당별 추천 순서, 편집자 기본 프롬프트와 반복 출처 표기를
보완한다. 기존 설정의 사용자 프롬프트는 유지한다. 패키지·회귀 검증과 실제 RisuAI의
기억·출력 효과 검증은 각 기록에서 구분한다.

4.4 이후 구성은 [4.3.1 기억 기준과 후속 인계](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#memory-baseline-431)를
따릅니다. 4.4는 동작 보존 리팩터링·동일 사실 통합, 4.5는 맥락 묶음, 4.6은 시점·상태,
4.7은 검색·예산, 4.8~4.9는 관계 회수를 맡습니다. 기본 기억은 전처리·출판사 없이도
검증하며, 5.1~6.0의 재회상·선택형 인물 기억 표현을 먼저 진행합니다.
[출력 개선 계획](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#output-improvement-plan)은
기존 순서인 6.1~8.0에 두고 **추가 기능 → 출력 개선 → 끔 / 기본형 / 복합형**으로 정리합니다.
6.1~7.0 기본형은 본문 초안→전문 AI 보완→최종 출력, 7.1~8.0 복합형은 나레이터·등장인물별
AI의 장면 생성입니다. 별도 AC Ensemble Agent Integrated 제작·연동 계획은 폐기했습니다.
전처리 편집자와 독립적으로 선택하며 8.1~9.0 Living World의 순서도 유지합니다.
이 내용은 향후 계획이며 이번 문서 갱신으로 기능이 구현된 것은 아닙니다.

## 서비스 포트 변경 (4.3.1)

- Windows: `06_change_port_windows.bat`에서 서비스를 선택합니다.
- Termux 기본 설치: `sh ~/.archive-center/start.sh --configure-ports`.
- Linux/macOS: 기존 실행 명령에 `--configure-ports`를 추가합니다.

포트 입력을 비우고 Enter를 누르면 선택한 서비스의 기본값으로 복원합니다.
ChromaDB **8000**, MariaDB **3307**, Go 백엔드 **28080**입니다.
저장 후 평소 방법으로 재시작하면 DB 실행 포트와 Go 연결 설정에 함께 적용됩니다.
Go 포트를 바꾸면 RisuAI에 저장한 백엔드 URL의 포트도 변경해야 합니다.
기존 DB 위치와 데이터는 유지됩니다.
[사용법과 적용 범위](docs/chromadb-port-configuration.md) ·
[4.3.1 다운로드](https://github.com/Flazer31/archive-center/releases/tag/v4.3.1).


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
