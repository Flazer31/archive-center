# 서비스 포트 변경 — 2026-09-10

미배포 로컬 변경. ChromaDB 포트 기능을 MariaDB와 Go HTTP 백엔드까지 확장했다.
공개 4.3.0에는 아직 포함되지 않는다. 실제 사용자 서버의 기동 확인은 별도다.
과거 Chroma 전용 기록은 [test.1](archive-center-4.3.1-test-build-1.md),
확장 패키지는 [test.3](archive-center-4.3.1-test-build-3.md)에서 확인한다.
[test.4](archive-center-4.3.1-test-build-4.md)부터 Windows 메뉴 파일명은 사용자가 변경한
`06_change_port_windows.bat`다. 서비스 포트 동작은 동일하다.

## 사용법

| 환경 | 설정 메뉴 |
| --- | --- |
| Windows | `06_change_port_windows.bat` 실행. test.3 원본 배포 파일명은 `06_change_chromadb_port_windows.bat` |
| Termux 기본 설치 | `sh ~/.archive-center/start.sh --configure-ports` |
| Termux ZIP | `sh install-and-start-termux.sh --configure-ports` |
| Linux/macOS | 기존 시작 명령에 `--configure-ports` 추가 |

1. 서비스를 선택한다: 1 ChromaDB / 2 MariaDB / 3 Go 백엔드.
2. 포트를 입력한다. **포트 입력을 비우고 Enter를 누르면 해당 서비스만 기본값으로 복원한다.**
3. 실행 중인 AC를 평소 방법으로 종료하고 기존 실행기로 다시 시작한다.

| 항목 | 기본 포트 | 함께 적용되는 값 |
| --- | --- | --- |
| ChromaDB | 8000 | 로컬 서버 실행 포트, Go의 `AC_CHROMA_ENDPOINT` |
| MariaDB | 3307 | 서버 실행·준비 확인·스키마 적용 포트, Go의 로컬 `AC_MARIADB_DSN` |
| Go 백엔드 | 28080 | `AC_BIND_ADDR`의 포트, 준비 확인 및 Windows `02` 연결 검사 |

서비스 선택 화면의 빈 입력/0은 종료다. **포트 입력란**의 빈 입력/EOF는 기본값 복원이다.
메뉴 자체는 서버를 시작하거나 종료하지 않는다. Go 백엔드 포트를 바꾸면
**RisuAI에 저장된 백엔드 URL의 포트도 맞춰야 한다.** JS는 저장된 주소를 임의로 바꾸지 않는다.
Go 항목은 별도의 DB가 아니라 HTTP 서버다.

POSIX의 `--chroma-port 8001`, `--mariadb-port 3308`, `--backend-port 28081`은 저장 후
일반 실행을 이어간다. `--configure-ports --port-service mariadb --mariadb-port 3308`처럼
설정만 할 수도 있다. 기존 `--configure-chroma-port`는 Chroma 입력란을 바로 연다.
Windows `scripts/start-full-windows.ps1`도 `-ChromaPort`, `-MariaDBPort`, `-BackendPort`,
`-ConfigurePorts -PortService chroma|mariadb|backend`를 지원한다.
`-ConfigureChromaPort`는 동일 소유자의 Chroma 입력란을 바로 여는 기존 진입점이다.

## 저장과 적용

- 기존 데이터 루트의 `chroma-port.txt`, `mariadb-port.txt`, `backend-port.txt`에 숫자만 저장한다.
  Windows 기본은 `%LOCALAPPDATA%\ArchiveCenter\data`, Termux 기본 명령 설치는
  `~/.archive-center/data`다. 사용자 지정 루트와 설치기의 기존 데이터 포인터를 따른다.
- 입력한 포트 → 저장된 포트 → 기존 환경 설정/기본값 순으로 적용한다.
  일반 시작 명령에 포트 옵션이 없으면 저장값을 유지한다. 메뉴의 빈 입력 복원과 구분한다.
- Windows에서 명시한 `-BindAddr`는 해당 실행의 전체 주소를 지정한다. `-BackendPort`도
  지정하면 그 호스트에 새 포트를 적용한다. `01`은 더 이상 28080을 강제로 전달하지 않는다.
- Windows MariaDB의 로컬 TCP DSN은 포트만 바꾸고 계정·암호·DB 이름·옵션은 보존한다.
  외부 MariaDB DSN은 변경하지 않는다. 메뉴의 대상은 AC가 관리하는 로컬 서비스다.
- 로컬 Chroma endpoint는 `http://127.0.0.1:<포트>`로 설정한다. `external`의 기존 endpoint와
  `off`/`fallback`의 비기동 동작은 유지한다. Go bind의 호스트도 유지한다.
- DB 폴더·컬렉션·스키마를 변경하지 않는다. Termux proot의 기존 DB 경로도 유지한다.
- 설정은 릴리스 관리 파일 목록 밖의 데이터 루트에 남는다. 새 패키지/업데이트에서
  같은 데이터 루트를 사용하면 그대로 읽는다. 자동 포트 탐색이나 점유 프로세스 종료는 없다.
- 잘못된 숫자는 기존 포트 유효성 검사에 따라 저장하지 않는다.

## 소유자 및 검증

Windows 실행기와 연결 검사는 `ops/full-package/scripts/service-ports.ps1`을 공유한다.
POSIX는 기존 `start-full-posix.sh::configure_service_ports`에서 처리한다. 구성 전용 모드는
설치·DB 준비·업데이트 적용 전에 끝나며 신규 설치에는 입력 단계를 추가하지 않는다.
기존 패키지 생성기가 공유 helper를 관리 파일 manifest에 포함한다.
Go의 환경 읽기와 JS의 저장 URL 사용을 유지한다. 이번 포트 변경의 JS 증감은 **+0/-0**.

`scripts/test-chromadb-port.ps1` / `.sh`는 기존 이름으로 세 서비스의 저장,
빈 입력 복원, 잘못된 입력, 재실행과 패키지 교체 후 유지, 프로세스/DSN/스키마 포트 일치,
DB 경로 보존을 검사한다. Windows `02`의 저장된 backend URL 적용도 검사한다.
프로세스 생성·소켓·네트워크는 격리된 시험 경계로 대체하므로 실제 사용자 서버,
Android/Linux/macOS의 네이티브 설치 또는 RisuAI 연결 성공의 증거는 아니다.
검사 결과와 패키지 식별은 [test.3 기록](archive-center-4.3.1-test-build-3.md)에 남긴다.
