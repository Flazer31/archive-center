# 오류 기록과 진단 보고서 — 2026-09-11

상태: 구현 및 격리 검증. 현재 사용자가 실행 중인 4.3 계열과 제보자의 PC를 교체하거나 재시작하지 않았다. 새 코드는 로컬 `4.4.0-test.4` 진단 테스트 패키지에 포함한다. GitHub 배포는 하지 않는다.

## 사용 방법

- 플러그인 **설정 → 오류·진단 보고서**에서 최근 오류를 펼쳐 확인하고 **보고서 저장**을 누른다. 서버 로그와 기기의 전송 오류가 하나의 JSON 파일로 저장된다.
- 백엔드에 연결할 수 없으면 기기의 전송 오류 보고서는 계속 저장할 수 있다. 화면에 서버 로그가 포함되지 않았다는 상태를 표시한다.
- Windows 서버에서는 **07_export_diagnostics_windows.bat**을 실행한다. 바탕 화면에 보고서를 저장한다.
- Linux·macOS·Termux에서는 패키지 폴더에서 **`sh 07_export_diagnostics.sh`**를 실행한다. 홈 폴더에 보고서를 저장한다. 기존 설치에 필요한 Python의 표준 라이브러리만 사용한다.
- 07 파일은 백엔드 실행, DB 연결, 재시작을 하지 않는다. 실행 파일이 없거나 실행이 차단되어도 남아 있는 로그를 읽는다. 사용자 선택으로 파일을 내려받으며 자동 전송은 없다.
- 로그 경로는 시작 창에 표시된다. 기본값은 Windows `%LOCALAPPDATA%\ArchiveCenter\data\logs`, Linux/macOS 패키지 `.runtime/logs`, Termux `~/.archive-center-2.0/logs`이다. `ARCHIVE_CENTER_DATA_DIR` 또는 진단용 `AC_LOG_DIR`이 지정되어 있으면 실제 표시 경로를 따른다.
- 최초 설치가 패키지 완성 전에 실패하면 Windows `%LOCALAPPDATA%\ArchiveCenter-install.log`, POSIX `~/ArchiveCenter-install.log`에 남는다. 이미 설치된 경로를 거절하는 기존 fresh-install 경로는 계속 파일을 변경하지 않고 오류를 표시한다.

이전 버전에서 기록하지 않은 과거 오류를 복원하는 기능은 아니다. 새 실행기·백엔드·JS로 교체한 이후의 기록이 대상이다.

## 기록 범위와 소유자

| 경로 | 기록 내용 |
| --- | --- |
| Go 시작 | 구성 오류, DB/Chroma 준비 실패, 포트 바인드 실패, 종료 사유 |
| Go 런타임 | JSON slog 파일, net/http 서버 오류, 응답 쓰기·JSON 인코딩 오류, 처리되지 않은 goroutine panic/stack |
| HTTP | 오류 응답의 경로·메서드·상태·시간·제한된 원인. 정상 응답 본문과 요청 본문은 수집하지 않음 |
| AI | 제공자·모델·작업·시간·HTTP 상태·실제 실패. 전처리는 담당과 1/2차 구분, 보정/부분 사용도 기록 |
| 출판사·평론가 | 제공자 오류뿐 아니라 응답 해석·스키마 실패를 해당 소유자에서 기록 |
| 기억/저장 | 검색·임베딩·Chroma 오류, Effective Input 저장 오류, 작업자 오류, DB audit 저장 실패 자체 |
| Windows 실행기 | PowerShell 시작/준비 로그, MariaDB·Chroma·Go stdout/stderr를 동시에 파일로 배출. Go 비정상 종료 시 최근 오류와 보고서 경로 표시 |
| POSIX 공통 실행기 | Linux/macOS/Termux의 stdout/stderr 로그, 종료 코드 및 최근 Go 오류. stdout과 stderr를 분리 유지하여 preflight JSON을 손상시키지 않음 |
| JS | 호스트/전송 오류의 최소 메타데이터를 장치 저장소에 최근 40개 보관. 성공 후에도 이전 오류 기록 유지 |

새 Go `GET /diagnostics/report`는 기존 인증·CORS·역방향 프록시 경로 아래에 있다. DB를 사용하지 않는다. Go CLI `archive-center-go diagnostics`도 설정 검증과 DB 초기화 전에 보고서를 출력한다. 07 지원 도구는 구버전 실행 파일의 미지원 인수가 정상 시작으로 해석될 수 있으므로 **Go CLI를 호출하지 않는다**.

Go 로그/보고서는 `internal/diagnostics`, 요청 오류는 `internal/httpapi/diagnostics.go`가 담당한다. 원래 호출 결과·상태 코드·취소·재시도·저장 확정·턴 판정·기억 선정에는 새 조건을 추가하지 않는다. `http.Flusher`/ResponseController 연결은 유지한다. 기존 오류를 응답에 남기는 1단계 수정은 [앞선 기록](archive-center-error-preservation-20260911.md)에 있다.

## 보관과 민감 정보

- Go 주 로그는 파일당 4 MiB, 현재 파일과 이전 3개를 순환 보관한다. panic 로그는 시작 시 이전 파일을 회전시킨다.
- Windows 자식 프로세스 스트림도 4 MiB씩 회전한다. 출력 파이프는 별도 비동기 배출로 함께 읽으며 전체 출력을 RAM에 쌓지 않는다.
- 실행기/설치 로그는 재시작 시 이전 파일을 보존한다. POSIX 외부 DB/도구 로그는 서비스 자체의 파일 쓰기 동작을 유지하므로 장기 무중단 실행 동안 파일 크기가 증가할 수 있다.
- 보고서는 허용된 로그 파일의 마지막 64 KiB씩만 수집하고 잘림 여부를 표시한다. DB, `.env`, 전체 설정, 대화 원문, 프롬프트, 전체 모델 응답은 수집 대상으로 열지 않는다. 실패 메시지에 포함된 외부 서비스의 진단 문구는 포함될 수 있다.
- API 키/토큰/비밀번호/DSN 패턴 및 해당 호출의 알려진 키를 가린다. 로컬 외부 프로그램 원시 로그 자체와 보고서 수집 시 마스킹을 구분한다. 보고서는 사용자가 확인 후 필요한 곳에 첨부한다.
- 디스크 권한/용량 문제는 stderr와 보고서의 로그 오류 상태에 남는다. 진단 파일 저장 실패를 채팅 실패로 바꾸지 않는다. OS 강제 종료·전원 차단 이전에 프로그램이 내보내지 못한 정보까지 기록한다고 주장하지 않는다.

## 검증

- Go 회귀: HTTP 오류 보존/정상 SSE 유지, 역방향 프록시 보고서, AI 호출 실패 원인, 디스크 실패, 순환 보관, 동시 쓰기, 비밀 마스킹, 큰 파일 제한 읽기, 실제 자식 프로세스 goroutine panic, 무시된 audit 저장 실패.
- `ops/settings-pair-smoke.cjs ... --diagnostics`: 실제 새 Go + 전체 JS + Edge 격리 화면에서 설정 조회/저장/재열기, 온라인 보고서 다운로드, 백엔드 종료 후 오프라인 보고서, 페이지 재열기 후 기기 오류 유지, 키 제외. Risu 호스트 API만 시험용 경계이며 유료 AI/사용자 DB는 호출하지 않는다.
- `ops/diagnostics-windows-smoke.ps1`: 한글·공백 경로에서 실제 Go 구성 실패를 발생시켜 운영 실행기의 프로세스 시작·대기·stderr 배출·보고서 생성을 검사한다.
- `ops/diagnostics-posix-smoke.sh`: 실제 셸 출력 분리·종료 코드·파일 기록과 Go 없이 네이티브 보고서를 검사한다.
- 기존 Windows/POSIX fresh-install 계약과 POSIX 서비스 포트 변경 계약도 검사한다. Windows 호스트의 Git sh 검사 및 OS별 프로파일 경계 검증은 실제 Linux/macOS/Android DB 설치 검증과 구분한다.
- Windows 및 Linux x64/arm64, macOS Intel/Apple Silicon, Termux arm64 패키지 빌드를 검사한다. 실제 사용자 RisuAI와 모든 OS의 네이티브 서비스 실행은 아직 검증하지 않았다.

증거 위치: 작업 폴더 `_diagnostics/20260911-diagnostic-report/`. 이번 JS 변경은 호스트 전송 오류 보관 및 보고서 UI에 필요한 **+115/-1**이며, 이전 작업의 변경량과 구분한다.

최종 결과: 관련 Go/JS 회귀 **5,437 통과·0 실패·5 환경 의존 검사 건너뜀**.
새 테스트 패키지의 전체 JS/Go 조합에서도 보고서의 온라인/오프라인 다운로드와 설정 저장·재열기를
통과했다. Windows 네이티브 보고서는 백엔드 실행 파일이 없는 폴더에서도 한글 경로와 원인을
보존했다. 6개 OS/CPU 패키지의 진단 파일이 자동 업데이트용 파일 목록에 포함되는 것을 확인했다.
