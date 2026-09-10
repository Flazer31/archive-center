# Archive Center 4.3.1-test.3 — 세 서비스 포트 변경

후속 테스트 빌드: [4.3.1-test.4](archive-center-4.3.1-test-build-4.md).
긴 대화의 입력 맥락 조립 지연 수정과 사용자 지정 포트 메뉴 파일명을 포함한다. 아래는 test.3 당시 기록이다.

2026-09-10 로컬 Windows 테스트 패키지. 설정 저장·실행 명령·연결 주소·패키지 검사를 통과했다.
실제 사용자 DB/서버 기동과 RisuAI 연결은 `implemented_unverified`이며 사용자 테스트 대기다.
GitHub 업로드와 사용자 백엔드의 시작·종료는 하지 않았다. 기존 test.1/test.2 패키지도 보존했다.

후속 [구버전 공통 기억 재검사](archive-center-4.3.1-memory-revalidation.md)에서 이전 누락 조건의
회복과 남은 한계를 구분했다. 검사한 현재 소스 재빌드와 이 패키지의 Go SHA-256이 일치했다.
기억 테스트를 이유로 이 패키지를 교체하거나 새 버전으로 다시 빌드하지 않았다.

## 사용

1. 새 폴더의 `06_change_chromadb_port_windows.bat`를 실행한다.
2. 1 ChromaDB / 2 MariaDB / 3 Go 백엔드 중 하나를 고르고 포트를 입력한다.
3. **포트 입력을 비우고 Enter를 누르면 해당 항목만 8000 / 3307 / 28080으로 복원한다.**
4. 기존 AC를 평소 방법으로 종료한 뒤 새 폴더의 `01_start_archive_center_windows.bat`를 실행한다.
5. Go 포트를 바꿨다면 RisuAI의 저장된 백엔드 URL도 같은 포트로 바꾼다.
   `02` 연결 검사는 저장된 새 Go 포트를 읽는다.

메뉴는 서버를 시작·종료하지 않는다. 서비스 선택 화면의 빈 입력/0은 메뉴 종료다.
일반 `01` 실행에서 포트 옵션을 생략하는 것은 저장한 값을 유지한다는 의미다.
같은 폴더의 `Archive Center.js`로 플러그인을 교체하면 UI에도 test.3 버전이 표시된다.
이번 포트 기능의 런타임 변경은 실행기에 있으며 Go/JS 소스 변경은 없다.

## 변경 범위

- 기존 `06_change_chromadb_port_windows.bat` 이름으로 세 서비스 메뉴 제공.
- Windows 공유 `scripts/service-ports.ps1`이 실행기와 연결 검사의 포트를 해석한다.
- `01`의 고정 28080 인자 제거. 저장한 Go 포트 적용 시 기존 bind 호스트 유지.
- MariaDB 서버·준비 확인·스키마 적용·Go DSN 포트를 일치시킨다.
  로컬 DSN의 계정/암호/DB/옵션과 외부 DB 주소는 보존한다.
- 데이터 루트의 서비스별 포트 파일에 저장한다. DB 경로·스키마·기억은 변경하지 않는다.
- POSIX도 `--configure-ports`와 서비스별 포트 옵션을 지원한다.
- [test.2 기억 회수 수정](archive-center-4.3.1-test-build-2.md)을 그대로 포함한다.
- [적용 규칙과 OS별 명령](chromadb-port-configuration.md).

## 패키지 식별

- 폴더: `source/_dist/4.3.1-test.3/Archive Center 4.3.1-test.3 Windows Test`
- ZIP: 같은 위치의 `Archive Center 4.3.1-test.3 Windows Test.zip`
- ZIP 크기: **18,103,785 bytes**
- ZIP SHA-256: `184eee278460bd5a08d943c7d83ce7e4c5f0c8edaa58a978324f9155a0bd190e`
- Go SHA-256: `2895eb46bae3f5d666391908d20340e0a96268440b4484fd130f9fd5ebfdf420`
- JS SHA-256: `23582deb16fa4b3a35a320b0c1f72120127575c5d12a631cb8c845fc41ca22fc`
- 관리 파일 **55개**, ZIP 안팎의 크기·SHA-256 일치 확인.
- Go 1.26.6, Windows amd64, `c0116aa` 이후 기존 dirty worktree를 포함한 관리형 빌드.
  새 helper는 기존 manifest 생성 경로에 포함된다. 운영 DB·실제 `.env`·API 키는 패키징하지 않았다.
- 기본 소스 버전 4.3.0 유지. 복사된 JS/실행기/환경 예제만 `4.3.1-test.3`으로 표시한다.
  **Go 바이너리는 test.2와 동일**하며 이번 기능은 실행 스크립트에서 작동한다. JS **+0/-0**.

## 검사 결과

| 검사 | 결과 |
| --- | --- |
| Windows 포트 회귀 | 통과: 실제 설정 진입점, 세 기본값 복원, 잘못된 입력 보존, DSN/주소/기동 명령, `02` URL |
| POSIX 포트 회귀 | 통과: Linux/macOS/Termux 프로필 설정, EOF 복원, 교체 후 유지, MariaDB/Chroma 실행 명령·DB 경로 |
| Windows 신규 설치 및 생성 POSIX 패키지 계약 | 통과 |
| POSIX 신규 설치 계약 | 통과 |
| Go `cmd/runtime-dependency-live-probe`, `internal/packageupdate` | 통과 |
| PowerShell/Bash 구문, `git diff --check`, 패키지 JS `node --check` | 통과 |
| test.3 실제 패키지 메뉴 | 통과: 세 서비스에서 새 값 저장 → 빈 입력 기본값 복원 |
| 관리 파일/ZIP/소스 사본/버전 | 통과 |

POSIX는 Windows Git Bash에서 검사했으며 네이티브 OS 실행 결과가 아니다.
프로세스·소켓·네트워크 경계는 시험 대역으로 대체했다. 실제 서버 포트의 개방과 Go↔DB 연결은
사용자의 재시작 후 확인할 항목이다. 새 패키지로 실제 공개 자동 업데이트를 실행하지는 않았다.
증거: `_diagnostics/20260910-service-ports/`의 검사 로그, `package-verification.json`, `package-menu.log`.
