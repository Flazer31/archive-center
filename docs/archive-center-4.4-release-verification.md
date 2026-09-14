# Archive Center 4.4.0 배포 기록

2026-09-15. **정식 공개 및 실제 공개 업데이트 확인 완료**.

초기 Windows CI에서는 02 연결 확인의 `.env` 읽기가 시스템 기본 인코딩에 의존하여,
한글 데이터 경로에 저장한 백엔드 포트를 찾지 못했다. `smoke-live.ps1`의 읽기에
UTF-8을 명시했다. 이미 추가된 실제 URL 경계 회귀 검사가 해당 CI에서 실패했고,
수정 후 같은 CI가 성공했다. 초기 초안 자산은 공개 전에 모두 수정본으로 교체했다.

- 기준: test.17까지의 승인된 변경과 Windows UTF-8 경로 수정. 정식 표시 버전은 4.4.0.
- 개인 설정 관찰표·개인 접속 주소는 새 공개 변경에서 제외했다. DB·벡터·원문 자료·로그·실제 환경 파일·개인 키는 패키지에 포함하지 않았다.
- 기존 4.4 후보 검사: ZIP 7개, 이전 버전×플랫폼 28개 preflight, 이전 Windows updater 적용/복원, 격리된 Windows 01 시작·재시작을 통과했다.
- 4.3.1의 실제 버튼 콜백→업데이트 API→관리 실행기 재시작→4.4 준비/commit을 로컬 후보로 확인했다. RisuAI host와 GitHub release HTTP는 격리된 테스트 경계였으며 공개 배포 다운로드 증거와 구분한다.
- 아래 결과는 후속 정식 빌드·CI·공개 자산·실제 공개 다운로드에서 새로 확인했다.

사용 중인 사용자 DB·서비스·RisuAI 설정은 변경하지 않았다. 백엔드 업데이트는 RisuAI에 등록한 JS를 교체하지 않는다.

## 정식 릴리스와 소스

- [공개 릴리스 v4.4.0](https://github.com/Flazer31/archive-center/releases/tag/v4.4.0)
- 태그 커밋: `77e597f71afe5e967adee603ae409535c8359fd9`.
- `draft=false`, `prerelease=false`, 최신 정식 릴리스. JS·백엔드·HUD·설치기·ZIP은 **4.4.0**.
- ZIP 7개와 `SHA256SUMS-4.4.0.txt` 1개의 공개 크기·SHA-256이 최종 로컬 파일과 일치한다.
- GitHub raw `main/Archive Center.js`도 최종 소스와 일치했다. 과거 test 번호는 이력과 fixture에만 남는다.

로컬 개발 중간 커밋은 기존 작업 브랜치에 보존하고 공개할 변경을 릴리스 커밋으로 정리했다.
공개 기본 설정의 모델·개별 LLM endpoint·API 키는 빈 값이며 실사용 설정을 기본값으로 복사하지 않았다.
각 ZIP의 전체 파일을 관리 파일 목록과 대조하고 비밀키 형식 및 개인 파일 검사를 통과했다.
릴리스 전환 자체의 runtime JavaScript 변경은 버전·빌드 설명 **+4/-4**이다.
이번 전환에서 기억 정책·턴 처리·설정 의미를 추가 변경하지 않았다.

## 최종 검사

- 전체 `go test ./... -count=1`, `go vet ./...`, JS syntax 및 Git LF 기준 Go formatting 통과.
- [최종 CI](https://github.com/Flazer31/archive-center/actions/runs/34868740480): Windows·Ubuntu·macOS 설치/포트/업데이트 및 Go 전체 검사·ChromaDB v2 통합·취약점 검사, 4개 작업 성공.
- [별도 MariaDB migration 검사](https://github.com/Flazer31/archive-center/actions/runs/34868002223): 최초 릴리스 커밋에서 성공. 후속 수정은 Windows 연결 확인의 env 읽기와 문서뿐이다.
- 최종 ZIP 7개에 대해 4.1.0·4.2.0·4.3.0·4.3.1의 production preflight **28개 통과**.
- 관리 파일별 크기·체크섬, JS 일치, 누적 migration, OS/아키텍처 바이너리 헤더, POSIX 실행 권한 검사 통과.

## 실제 공개 4.3.1 → 4.4.0 업데이트

GitHub에서 공개된 **4.3.1 Windows 설치 ZIP을 다시 다운로드**하고 공개 digest/크기를 확인했다.
원본 JS·Go·updater·01 실행기를 수정하지 않고, 별도 데이터 경로와 포트에서 시작했다.
설치된 MariaDB/ChromaDB 실행 도구를 재사용하되 DB 내용은 독립된 시험 데이터였다.

1. 기존 01 실행기로 원본 4.3.1과 실제 MariaDB·ChromaDB의 준비 상태 확인.
2. MariaDB에 합성 확인용 행을 저장하고 환경 파일·저장된 포트의 비교 기준 확보.
3. UI가 사용하는 실제 `/update/check`·`/update/apply` 경로를 HTTP로 호출.
4. 원본 4.3.1 backend가 **실제 공개 GitHub API와 ZIP**에서 4.4.0을 확인·다운로드.
5. `accepted` 응답과 exit 75 후 관리 실행기가 자동 교체·재시작.
6. `/version=4.4.0`, store/vector ready, `/update/status=committed` 확인.
7. DB 확인용 행, `.env.full.local`, 세 서비스 포트 유지. 설치 파일 **58개**가 공개 배포 manifest와 일치.
8. 실제 다운로드 ZIP의 SHA-256도 공개 자산과 일치. 시험 프로세스·포트 정리 및 기존 사용자 서비스 유지 확인.

이 시험은 원본 backend와 공개 다운로드를 검증했다. RisuAI 화면에서 버튼을 누른 시험으로 서술하지 않는다.
[이전 버튼 검사](archive-center-4.3.1-to-4.4-ui-update-verification.md)는 원본 JS 콜백과 로컬 후보를 이용한 별도 증거다.
**백엔드 업데이트는 RisuAI에 등록한 JS를 교체하지 않으므로 플러그인도 별도 업데이트해야 한다.**

## 범위

신규 설치 진입점은 Windows PowerShell 한 줄 / POSIX 한 줄 / 패키지의 01 실행기를 유지한다.
설치 이후 RisuAI 플러그인 등록·백엔드 주소 지정은 별도다. 기존 설치는 UI 업데이트를 사용한다.
Native POSIX CI는 설치 계약과 updater를 검사하며 모든 Linux/macOS/Termux 실기기의 최초 런타임 설치를 증명하지 않는다.
실사용 기억 회수·시간·RAM의 관측 범위는 [요청 준비 기록](archive-center-request-preparation-20260914.md)을 따른다.
이번 배포 검증에서 사용 중인 세션을 진행하거나 유료 모델을 호출하지 않았다.

로컬 원본 증거: workspace `_diagnostics/20260915-440-release/`의
`assets-verified.json`, `published-release.json`, `final-verified-preflight.log`,
`public-update/verified-result.json`. 원문 로그와 시험 DB는 GitHub에 올리지 않는다.
