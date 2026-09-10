# Archive Center 4.3.1 배포 검증

**정식 공개 완료: 2026-09-10 17:50 KST.**

- 릴리스: https://github.com/Flazer31/archive-center/releases/tag/v4.3.1
- 태그 소스: `0821d69e9be9bc406bd9f0092a03398498f23b5f`.
- 기능·버전 승격 커밋: `c45da095320387ae5bcbd752cdce0ca7ab7616f7`.
- CI: https://github.com/Flazer31/archive-center/actions/runs/34456545567 — 4개 작업 모두 통과.
- 비인증 공개 latest API에서 `v4.3.1`, draft=false, prerelease=false와 8개 asset을 확인했다.
- 공개 main과 태그의 `Archive Center.js` blob은 모두 `5c63afde3091b71501d5d1d069f6dd486609e447`.
  실제 raw 업데이트 URL에서도 4.3.1 버전을 확인했다.

## 배포 범위

검증한 4.3.1-test.4의 기억 회수 복원, 맥락 조립 성능 수정, 세 서비스 포트 설정을 유지한다.
정식 버전 문자열과 빌더 기본값, OpenCode 클라이언트 버전 표기를 4.3.1로 맞췄다.
기억 정책·프롬프트·스키마를 추가 변경하지 않았다. 마이그레이션은 013까지다.

- [사용자 릴리스 안내](archive-center-4.3.1-release-notes.md)
- [사전 설치·업데이트 검증](archive-center-4.3.1-install-update-verification.md)
- [기억 비교 검증](archive-center-4.3.1-memory-revalidation.md)

## 소스와 CI

JavaScript syntax와 Go vet를 통과했다. 전체 Go 검사에서 기존 버전 4.3.0을 기대하던
`TestArchiveCenterJSPluginVersionMarkers`, `TestHandleVersion` 두 곳을 4.3.1로 맞춘 뒤
해당 두 패키지 전체 재검사를 통과했다. 최종 GitHub CI에서도 전체 Go 검사를 다시 통과했다.

CI의 verify, Windows fresh install, Ubuntu fresh install, macOS fresh install 작업이 모두 통과했다.
여기에는 설치 entrypoint, 체크섬 수락·변조 거부, 포트 변경, native updater,
ChromaDB 1.5.9 API v2 통합 검사와 취약점 검사가 포함된다.
설치 검사의 외부 다운로드·시작 경계 fixture는 모든 실제 기기의 완전한 신규 설치 증거와 구분한다.

첫 CI의 macOS 포트 검사만 실패했다. IPv6 주소와 교체 패키지를 지정하는 테스트의
임시 환경변수를 subshell 안에 한정한 뒤 같은 값·포트 보존 assertion으로 macOS 검사를 통과했다.
이 수정은 `scripts/test-chromadb-port.sh`에만 적용했고 배포 실행기 코드는 바꾸지 않았다.
최종 태그에 이 테스트 수정도 포함하도록 공식 builder를 다시 실행했다.

## 패키지와 이전 버전 호환

공식 `ops/build-release-assets.ps1`로 Windows 설치·업데이트 및 Linux x64/arm64,
macOS Intel/Apple Silicon, Termux arm64 ZIP 7개와 외부 체크섬을 생성했다.

- 최종 Windows manifest의 source_commit은 태그와 일치하고 source_dirty=false, 관리 파일은 55개다.
  POSIX는 기존 manifest 형식을 유지하며 동일 builder 실행과 파일 대조로 출처를 확인했다.
- 모든 관리 파일 SHA-256·크기, 소스 JS·prompt·schema·migration 일치,
  세 바이너리의 CPU 형식, POSIX 실행 권한과 LF를 확인했다.
- 개인 `.env.full.local`, 데이터, 캐시, runtime, update 상태는 배포에 포함하지 않았다.
- 이전 공개 4.1·4.2·4.3의 ZIP 21개는 GitHub asset digest/크기와 일치함을 먼저 확인했다.
- 해당 이전 패키지와 최종 4.3.1의 OS별 조합 21개가 생산 `PreflightCandidate` 검사를 통과했다.
- 이전 Windows updater 세 버전 각각으로 최종 ZIP 적용→rollback→재적용→commit을 통과했다.
  실제 등록된 `POST /update/apply`를 사용하되 이 단계의 GitHub HTTP 경계는 최종 ZIP 바이트 fixture다.
- 새 `06_change_port_windows.bat`와 `scripts/service-ports.ps1`의 설치 및 rollback 시 제거,
  기존 JS 복원, 관리 파일 전체 hash를 검사했다. 가상 개인 설정·DB 디렉터리·편집자 설정·
  세 포트 파일 sentinel 7종은 유지됐다. 이 sentinel 검사를 실제 DB 레코드 검증으로 확대하지 않는다.

업로드 후 GitHub가 계산한 아래 8개 asset의 SHA-256·크기를 로컬 파일과 모두 대조했다.

| 파일 | SHA-256 |
| --- | --- |
| Archive.Center.4.3.1.Linux.arm64.Auto.Install.Package.zip | `fb439f6cc530e0a56eb4371a77d0b6e8d3d23de4b8cb7bead6c58499b038171e` |
| Archive.Center.4.3.1.Linux.x64.Auto.Install.Package.zip | `0caa0c81d063f66d0aa926a856dea3cfbea5ea625aefae565d9724301d8dd488` |
| Archive.Center.4.3.1.macOS.Apple.Silicon.Auto.Install.Package.zip | `60834e3aa74355e89b2df47009570edb48c83b36face2c0aa729a3f6672f526f` |
| Archive.Center.4.3.1.macOS.Intel.Auto.Install.Package.zip | `0c245afa4fc59342effbdf7ce10649390240a54305bd07a65b03dd99a16bf0d2` |
| Archive.Center.4.3.1.Termux.arm64.Auto.Install.Package.zip | `99dc3176093793ed740ae4b4022a19eed241fbd40d73337851d95a98bfc60105` |
| Archive.Center.4.3.1.Windows.Auto.Install.Package.zip | `b4a471975ff3f9fb6508d7c3c59ede2ad5f596c10d2f806aebef48b25fa727db` |
| Archive.Center.4.3.1.Windows.Update.Package.zip | `2961e795e0f30c312a317b9462f12c6e1ab585afd7b1e735f2702e186ba4a338` |
| SHA256SUMS-4.3.1.txt | `23cf99bcb963498c8b2ebe7fd36f44d8f9055d6656af5341ae2ae6e96515f39d` |

## 실제 공개 GitHub 업데이트

공개 이후 사용자 데이터와 분리한 Windows 환경에서 **공개 4.3.0 패키지**를 실행했다.
기존 공식 MariaDB·Chroma 런타임을 사용하되 DB 디렉터리는 새 테스트 디렉터리로 지정했다.
포트는 Go 28192, MariaDB 33192, ChromaDB 8192이며 사용자 서버 포트는 사용하지 않았다.

1. 이전 백엔드의 `/version=4.3.0`과 실제 MariaDB·Chroma 준비 상태를 확인했다.
2. `/update/check`가 공개 최신 4.3.1을 선택했다. UI가 사용하는 `POST /update/apply`는
   실제 GitHub ZIP을 다운로드하고 `accepted`로 응답했다. 이 단계에는 GitHub 응답 fixture가 없다.
3. 종료 코드 75 이후 기존 관리 실행기가 새 패키지를 적용하고 재시작했다.
   `/version=4.3.1`, `/ready`의 ready/store_ready/vector_ready=true, update status=committed를 확인했다.
4. 새 포트 메뉴·helper를 포함한 관리 파일 55개의 hash가 공개 패키지와 일치했다.
   개인 `.env.full.local` hash와 세 포트 설정 파일은 유지됐다.
5. 테스트 실행기만 종료했고 테스트 포트 세 개가 닫혔다. 시작 전 사용자 백엔드의 PID·시작 시각은 유지됐다.

이는 실제 공개 다운로드·API·updater·재시작 검증이다. RisuAI 화면에서 직접 버튼을 클릭한
증거나 모든 사용자 DB 레코드의 전후 비교, 모든 OS 실기기 설치 증거와는 구분한다.
4.1·4.2→4.3.1의 이번 검사는 위 최종 ZIP/이전 updater fixture 범위이며,
실제 공개 GitHub 다운로드까지 실행한 이번 경로는 4.3.0→4.3.1이다.

사전 후보의 Windows `01_start_archive_center_windows.bat` 실제 실행과 신규 격리 DB 초기화는
[사전 검사 기록](archive-center-4.3.1-install-update-verification.md)에 따로 정리했다.

## 사용자 적용 및 후속 확인

기존 관리형 설치는 Archive Center UI 업데이트를 사용한다. 신규 사용자는 Windows ZIP의 01 BAT
또는 [README의 OS별 한 줄 설치](../README.md#github-fresh-install)를 사용한다.
**패키지 내부 JS 갱신과 RisuAI에 등록된 플러그인 교체는 별개**이므로 RisuAI 플러그인도 4.3.1로 맞춘다.

최종 제공자 호출·본문 AI의 기억 활용과 모든 OS/CPU 실기기 장시간 운용은 위 검사로 보증하지 않는다.
후속 기억 변경은 4.3.1과 직전 검증 버전의 동작을 함께 비교한다.

이번 정식 승격의 JS 변경량은 +6/-6, 포함된 기존 기억 UI 변경량은 +1/-9로 합계 +7/-15다.
공개 후 검증 기록 갱신의 JS 변경량은 +0/-0이다.
