# 4.4 설치·업데이트 검증

검사일: 2026-09-15. test.17의 활성 소스로 로컬 **4.4.0 후보**를 만들었다.
이번 작업은 설치·업데이트 검사와 그 과정에서 재현한 Windows 환경 파일 읽기 수정에 한정한다.
기억 선정, 전처리, 턴 처리, 모델 설정과 사용 중인 test.17 서버는 변경하지 않았다.
GitHub push·tag·release 공개 및 사용자 RisuAI 플러그인 교체는 하지 않았다.

같은 날 후속으로 [4.3.1 원본 버튼 코드의 브라우저 클릭 → 4.4 자동 재시작](archive-center-4.3.1-to-4.4-ui-update-verification.md)을
검사했다. 이전 서버의 종료 코드 75, 새 서버의 ready/committed, 실제 시험 DB 레코드 보존도
확인했다. 아래는 그에 앞서 수행한 파일 적용·설치 검사 범위이며, 후속 검사는 별도 문서를 따른다.

## 결과

| 항목 | 결과와 실제 확인 범위 |
| --- | --- |
| 배포 후보 | Windows 설치/업데이트, Linux x64/arm64, macOS Intel/Apple Silicon, Termux arm64의 7개 ZIP 생성 |
| 파일 검증 | ZIP SHA-256, 모든 관리 파일의 크기·해시, JS 버전과 활성 소스 일치, migration 원본 일치, 실행 파일 OS/CPU, POSIX 권한·LF 통과 |
| 구버전 패키지 사전 검사 | 4.1·4.2·4.3·4.3.1 × 7개 후보 = 28개 생산 `PreflightCandidate` 검사 통과 |
| Windows 실제 업데이트 실행기 | 4.2·4.3·4.3.1의 이전 updater로 4.4 적용 → 복구 → 재적용 → 확정 통과 |
| 사용자 파일 보존 | 환경 설정, 전처리 설정, MariaDB/Chroma 시험 파일, 세 서비스 포트 파일의 내용 유지 |
| 신규 설치 명령 | 실제 Windows/POSIX 설치 진입점·release helper의 신규 설치, 시작 요청, 기존 설치 무변경, checksum/실패 처리 통과 |
| Windows 실제 01 실행 | 한글·공백 설치 경로와 별도 신규 DB에서 시작·재시작 모두 `/version=4.4.0`, `/ready`의 store/vector 준비 완료 |
| 서비스 포트 | 저장·기본값 복원·패키지 교체 후 유지, MariaDB 프로세스/DSN/schema 및 Go/Chroma 주소 일치 통과 |

최종 후보는 `source/_release-builds/4.4.0-verification-20260915-fixed/`다.
처음 만든 `4.4.0-verification-20260915/`는 아래 수정 전 실패를 보존한 자료이며 배포 후보로 사용하지 않는다.
활성 JS의 개발 버전은 test.17을 유지했고, builder가 후보 복사본의 버전을 4.4.0으로 설정했다.

## 검사 중 재현·수정한 문제

`ops/full-package/scripts/start-full-windows.ps1`의 `Read-DotEnvContent`가 인코딩 없이
`Get-Content -Raw`로 UTF-8 환경 파일을 읽었다. Windows PowerShell 5.1에서 BOM 없는
UTF-8 파일의 `ARCHIVE_CENTER_DATA_DIR`에 한글이 있으면 잘못 해석되어
`Get-ArchiveDataRoot`의 `GetFullPath`에서 `Illegal characters in path`로 시작이 실패했다.

평문 환경 파일 읽기에 `-Encoding UTF8`을 지정했다. 암호화 환경 파일 경로, 저장 위치,
포트 선택, 프로세스 관리, 메모리 정책은 변경하지 않았다. 이것은 이번 자료에서 재현한
원인이며, 과거 사용자의 모든 Windows 시작 실패 원인으로 소급 확정하지 않는다.

`scripts/test-chromadb-port.ps1`에도 실제 launcher가 요구하는 `diagnostics.ps1`을 넣고
검사용 로그 위치를 격리했다. BOM 없는 UTF-8 환경 파일에 한글 데이터 경로를 넣은
동일 검사는 수정 전 실패·수정 후 통과했다. 진단 기록은 각각
`ports-unicode-before.log`와 `ports-unicode-after.log`다.

## 업데이트 검사 경계

UI의 `applyArchiveCenterUpdate()`가 호출하는 등록된 `POST /update/apply` 경로를 사용했다.
HTTP owner는 활성 Go 소스이며, 파일 적용·복구는 각 구버전 패키지의 실제 Windows
updater 실행 파일을 사용했다. **실제 RisuAI 화면의 버튼 클릭이나 구버전 Go 서버 프로세스
전체를 실행한 검사는 아니다.** GitHub 응답·다운로드 경계만 로컬 후보 ZIP으로 대체했다.

기존 릴리스 테스트의 4.3.0 고정 기대값은 검사용 Go overlay에서 후보 manifest로부터
읽도록 바꿨다. 모든 후보 관리 파일은 적용·확정 후 해시를 확인했다. 이전 JS·manifest·
실행 파일은 복구 후 원래 내용과 비교했다. 실제 런타임 소스를 overlay로 대체하지 않았다.
파일 추가·삭제, 사용자 파일 제외, 누적 migration, 적용 실패와 복구의 생산 코어 검사도 통과했다.
보존 검사 자료는 합성 sentinel이며 사용자 운영 DB 레코드를 읽거나 복제하지 않았다.

최종 후보의 28개 사전 검사와 3개 실제 updater 검사를 Windows 인코딩 수정 후 다시 실행했다.
코어 검사 71개 통과, Windows에서 POSIX 실행 비트 전용 1개 skip. HTTP 검사 28개 통과와
사전 검사 28개 사례(상위 테스트 포함 29개)를 구분한다. 초기 HTTP+사전 검사 기록은 57개다.
최종 실행 결과는 `tests-summary.json`에 기록했으며 재실행 결과를 합산해 검사 수를 부풀리지 않는다.

**백엔드 패키지 안의 JS 갱신과 RisuAI에 등록된 JS 교체는 별개다.** 정식 공개 후에는
백엔드 업데이트와 같은 버전의 플러그인 업데이트/가져오기를 함께 확인해야 한다.

## 실제 Windows 시작과 검사 한계

변경하지 않은 후보의 `01_start_archive_center_windows.bat`를 실행했다.
기존 MariaDB 11.4.10·ChromaDB 1.5.9 실행 파일을 명시적으로 재사용하되,
데이터는 진단 폴더의 `새 데이터 live-data-runtime`에서 새로 초기화했다.
Go 28196 / MariaDB 33196 / ChromaDB 8196 포트를 사용했고, 시작과 재시작 모두 세 서비스가
정상 준비됐다. 환경 파일은 패키지 예제에서 생성한 합성 설정이다.
재시작 후에도 포트 파일과 시험 데이터 파일이 유지됐다.

검사 도구의 실행 권한 제한 때문에 설치된 Chroma Python의 하위 프로세스 시작이 한 차례
막혔다. 권한을 확장한 같은 런타임 버전 검사는 1.5.9를 반환했고 실제 시작·재시작도 통과했다.
이 제한은 패키지 결함으로 집계하지 않았다. 모든 검사용 프로세스를 정리한 뒤 사용 중이던
Go 23352, MariaDB 10976, Python 20272/20612의 PID와 시작 시각이 유지된 것을 확인했다.

Windows/POSIX 한 줄 설치 검사는 다운로드·서비스 경계에 기록 가능한 시험 응답을 사용한다.
POSIX 스크립트는 Windows Git Bash와 실제 Python으로 실행했다. 따라서 Linux/macOS/Termux
실기기 신규 설치, 빈 OS에서 의존성 전체 다운로드, 공개된 4.4 자산 다운로드 완료로 확대하지 않는다.
배포 후 공개 ZIP 체크섬·링크 및 실제 RisuAI 등록 버전 확인은 이번 로컬 검사와 구분한다.

## 증거와 변경 범위

진단 위치: `_diagnostics/20260915-44-install-update-check/`.

- `packages.json`, `baseline-431.json`, `matrix.json`: 후보·이전 자산의 해시와 28개 조합.
- `go-update-core.jsonl`, `go-update-http-preflight.jsonl`, `platform-preflight-fixed.jsonl`,
  `upgrade-fixed-4.2.0.jsonl`, `upgrade-fixed-4.3.0.jsonl`, `upgrade-fixed-4.3.1.jsonl`.
- `fresh-windows.log`, `fresh-posix-python.log`, `ports-posix.log`, `ports-unicode-before/after.log`.
- `runtime-live/windows-live-result.json`: 실제 01 시작/재시작의 버전·준비 상태·포트·프로세스 보존.
- `processes-before.json`, `processes-after.json`, `tests-summary.json`, `git-before/after.txt`.

런타임 변경: Windows launcher의 UTF-8 읽기 1줄. 테스트 fixture와 검증 문서를 갱신했다.
**JavaScript 추가 0줄 / 삭제 0줄.** Go 기억·전처리·저장 코드는 이번 작업에서 변경하지 않았다.
