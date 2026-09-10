# 4.3.1 설치·UI 업데이트 사전 검증

검사일: 2026-09-10. 활성 소스로 만든 **로컬 4.3.1 검증 후보**를 검사했다.
사전 검사 단계에서는 GitHub 공개·push·tag 변경을 하지 않았고 latest는 **v4.3.0**이었다.
이후 **v4.3.1을 정식 공개**했다. 최종 ZIP·CI·공개 다운로드 및 실제 Windows 업데이트 결과는
[4.3.1 배포 검증](archive-center-4.3.1-release-verification.md)에 기록했다. 아래는 사전 검사 당시의 상세 범위다.

## 결론과 사용자 작업

- 관리형 설치의 Archive Center UI 업데이트 버튼은 백엔드 패키지의 다운로드·적용·재시작 경로다.
  이번 후보를 이전 4.1·4.2·4.3 패키지에 적용하는 검사에서 차단 결함을 발견하지 않았다.
- 패키지에는 새 `06_change_port_windows.bat`와 `scripts/service-ports.ps1`도 포함된다.
  기존 사용자 설정과 데이터 디렉터리는 관리 파일 교체 대상과 구분된다.
- **패키지 안의 JS 갱신과 RisuAI에 등록된 플러그인 교체는 별개다.**
  RisuAI의 플러그인 업데이트 또는 같은 버전의 `Archive Center.js` 가져오기도 필요하다.
- 신규 설치는 아래 명령이 설치와 시작을 이어서 요청한다. 기존 설치에 신규 설치 명령을
  다시 실행하면 변경 없이 중단하고 별도 업데이트 경로를 안내한다.

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

Linux / macOS / Termux:

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

백엔드 설치·실행 후 RisuAI의 플러그인 등록과 백엔드 주소 설정은 별도다.
Go 서버 포트를 변경했다면 RisuAI에 저장된 주소의 포트도 맞춘다.

## 검증 후보와 파일 검사

공식 `ops/build-release-assets.ps1 -PackageVersion 4.3.1`을 현재 작업 트리에서 실행했다.
출력은 `source/_release-builds/4.3.1-verification/`이며 사용자 test.4를 교체하지 않았다.
활성 소스의 기본 버전 표시는 여전히 4.3.0이고, builder가 후보 복사본에 4.3.1을 적용했다.
이 후보는 사전 검사용이며 공개 배포 소스 커밋 확정을 대신하지 않는다.

- Windows Auto Install / Update, Linux x64·arm64, macOS Intel·Apple Silicon,
  Termux arm64 **7개 ZIP**과 `SHA256SUMS-4.3.1.txt` 생성.
- ZIP 체크섬, 모든 관리 파일의 크기·SHA-256, 4.3.1 JS 표시와 버전 표시 외 활성 JS 일치 확인.
- Go·updater·schema 세 실행 파일의 OS/CPU 헤더, POSIX 실행 권한·LF 확인.
- migration 전체와 활성 소스의 바이트 일치, `013` 포함 확인.
- 새 Windows 포트 메뉴/helper 및 POSIX 포트 설정 함수 포함 확인.
- 개인 `.env`, 데이터·런타임·업데이트 상태를 ZIP에 포함하지 않은 것 확인.
- 이전 4.1·4.2·4.3의 각 7개 로컬 ZIP도 공개 GitHub 자산의 SHA-256·크기와 대조: **21개 일치**.

## UI 업데이트와 이전 updater

현재 UI의 `applyArchiveCenterUpdate()`는 설정된 백엔드의 `POST /update/apply`를 호출한다.
이번 검사는 같은 등록 HTTP 경로와 실제 구버전 updater 실행 파일을 사용했다.
실제 RisuAI 화면에서 버튼을 클릭한 검사는 아니다.

| 검사 | 결과 | 증거 범위 |
| --- | --- | --- |
| 이전 3개 버전 × 후보 7개 ZIP | 21개 사전 적용 검사 통과 | 생산 `PreflightCandidate`, OS별 필수 파일·manifest·누적 migration 계약 |
| Windows 4.1 → 4.3.1 | 적용·복구·재적용·확정 통과 | 공개 원본과 동일한 4.1 updater 실행 |
| Windows 4.2 → 4.3.1 | 적용·복구·재적용·확정 통과 | 공개 원본과 동일한 4.2 updater 실행 |
| Windows 4.3 → 4.3.1 | 적용·복구·재적용·확정 통과 | 공개 원본과 동일한 4.3 updater 실행 |
| 새 파일 | 적용 후 존재·해시 일치, 복구 후 제거 | `06_change_port_windows.bat`, `scripts/service-ports.ps1` |
| 기존 파일 보존 | 7종 sentinel 동일 | 개인 환경 설정, MariaDB/Chroma 데이터 위치, 전처리 설정, 세 서비스의 포트 파일 |

후보 버전은 ZIP manifest에서 읽었으며 테스트의 4.3.0 고정 기대값을 사용하지 않았다.
적용·확정 후에는 일부 파일만이 아니라 후보의 모든 관리 파일 해시를 대조했다.
구버전의 JS도 복구 후 원래 바이트와 일치했다.

외부 GitHub 응답·다운로드만 4.3.1 후보 ZIP 바이트로 대체했다. 따라서 위 결과는
**로컬 후보의 생산 업데이트 경로 검사**이며, 미공개 4.3.1을 실제 GitHub에서 내려받았다는 뜻이 아니다.
데이터 보존 자료는 임시 sentinel 파일이다. 사용자 운영 DB의 레코드 보존을 새로 검증한 것은 아니다.

기존 업데이트 코어 검사 46개 최상위/25개 하위, HTTP 검사 15개 최상위/14개 하위도 통과했다.
파일 추가·삭제·관리 대상 변경, 사용자 데이터 보존, 누적 migration, 실패·복구 경로를 포함한다.
Windows에서 POSIX 실행 비트 전용 검사 1개는 건너뛰었다. 기본 HTTP 실행의 ZIP 미지정 skip은
위 실제 후보 ZIP을 지정한 3회 검사로 별도 수행했다.

## 신규 설치 명령과 포트 설정

- `scripts/test-simple-fresh-install.ps1`: 실제 Windows 설치 진입점의 신규 경로,
  기존 설치 무변경·무다운로드, 시작 요청, 실패 정리, release helper의 checksum 수용/거부 통과.
  공식 builder로 만든 5개 POSIX 패키지의 실행 파일·권한·실행기 전달값도 검사했다.
- `scripts/test-simple-fresh-install.sh`: 실제 POSIX 설치/helper의 신규·기존 설치 구분,
  checksum 확인, 안정적인 데이터 위치 및 Linux/macOS/Termux 실행기 경로 통과.
  Windows Git Bash에서 실행했고 실제 Python을 연결했다.
- `scripts/test-chromadb-port.ps1` / `.sh`: 세 서비스의 포트 저장, 빈 입력 기본값 복원,
  재실행·패키지 교체 후 보존, 로컬 프로세스·DSN·schema·연결 확인 값의 일치 통과.
- 위 설치 계약 검사의 다운로드·서비스 시작 경계는 시험용 응답이다.
  모든 OS 실기기에서 전체 의존성을 새로 내려받아 설치한 증거로 확대하지 않는다.

## 실제 Windows 01 BAT 실행

4.3.1 Auto Install ZIP을 진단 폴더에 새로 풀고 **변경하지 않은 01 BAT**를 실행했다.
별도 데이터 루트와 저장 포트 28191 / 33191 / 8191을 사용했다.
개인 환경 파일은 패키지 예제에서 만든 시험용 설정이며, 사용자의 설정 파일은 읽지 않았다.

- 새 MariaDB 데이터 디렉터리 초기화, 관리 계정 확인 및 migration 적용 성공.
- 새 ChromaDB 데이터 위치로 실행 성공.
- `/version`: **4.3.1**.
- `/ready`: **ready / store_ready / vector_ready = true**, `full_local`, `bundled`.
- Go·MariaDB·ChromaDB가 각각 저장한 포트에서 수신하고 포트 파일도 유지됨.
- 검사 뒤 테스트 실행기만 종료하고 해당 세 포트가 모두 닫힌 것 확인.
- 검사 전부터 실행 중이던 사용자 백엔드 PID 10168과 시작 시각 유지 확인.

이미 설치된 MariaDB/ChromaDB 실행 파일을 명시적으로 재사용했다. 새 데이터 초기화·실제 실행의
증거이며, Python/MariaDB/ChromaDB 전체 런타임을 처음부터 다운로드한 검사는 아니다.

## 공개 전에 남는 범위

4.3.1 정식 소스 버전·커밋·배포 자산 확정 및 GitHub 공개는 아직 하지 않았다.
공개 후 latest 다운로드와 체크섬은 실제 공개 파일로 다시 확인해야 한다.
실제 RisuAI 버튼 클릭, 등록된 JS 교체, 모든 OS 실기기 설치 성공은 이번 결과와 구분한다.

이번 작업의 런타임 수정 **없음**, JavaScript 실행 소스 **+0/-0**.
현재 기억 선정·프롬프트·스키마·설치/업데이트 정책을 변경하지 않았다.
테스트의 버전 일반화와 후보 파일·포트 보존 확인은 진단용 Go overlay에만 두었다.

## 증거

`_diagnostics/20260910-431-install-update-check/`:

- `public-assets.json`, `packages.json`: 공개 구버전 자산 대조와 후보 패키지 검사.
- `prepare_checks.py`, `matrix.json`, `overlay.json`, `group_update_431_overlay_test.go`: 검사 재현 입력.
- `go-update-core.jsonl`, `go-update-http.jsonl`, `platform-preflight.jsonl`, `upgrade-*.jsonl`, `tests-summary.json`.
- `fresh-windows.log`, `fresh-posix-3.log`, `ports-windows.log`, `ports-posix.log`.
- `check_windows_start.ps1`, `windows-live-result.json`, `windows-01-stdout.log`, `windows-01-stderr.log`.
- `candidate-build-2.log`: 공식 7개 패키지 생성 기록.

초기 Go cache 접근 제한, builder 출력 위치 제한, Git Bash PATH/Python 준비 실패도 별도 로그로
보존했다. 위 최종 성공 실행과 구분하며 제품 결함으로 계산하지 않는다.
