# Archive Center Windows 시작 안내

## __ARCHIVE_CENTER_PACKAGE_VERSION__ 적용 확인

이 안내의 버전은 패키지를 만들 때 실제 빌드 번호로 설정됩니다.
설정 화면의 플러그인 버전과 연결된 백엔드 버전을 함께 확인하세요.
4.3 테스트판의 다중 AI 전처리는 **추가 기능 → 전처리**에서 선택적으로 사용합니다.
기능을 켜지 않아도 기본 장기 기억을 사용할 수 있습니다.

1. 실행 중인 기존 Archive Center는 해당 실행 창에서 `Ctrl+C` → `Y`로 종료합니다.
2. 이 폴더의 `01_start_archive_center_windows.bat`를 실행합니다.
3. RisuAI/PocketRisu에 이 폴더의 `Archive Center.js`를 임포트해 기존 AC를 교체합니다.
   AC 플러그인 두 개를 동시에 활성화하지 않습니다.
4. AC 설정 화면에서 패키지와 같은 버전 표시 및 백엔드 연결을 확인합니다.
   같은 PC의 기본 백엔드 주소는 `http://127.0.0.1:28080`입니다.
   다른 기기에서 사용한다면 실제 접속 가능한 주소를 UI 설정에 저장합니다.
5. 시험용 채팅에서 일반 대화 저장·기억 조회 → reroll → assistant 삭제 후 같은
   사용자 메시지 편집·재생성 → 분기·재분기 → HUD 닫기·복구 순서로 확인합니다.
   Gemini 3.8 Flash를 사용하는 경우 medium 선택·저장도 확인합니다.

기본 실행은 `%LOCALAPPDATA%\ArchiveCenter\data`의 기존 기억 데이터를 이어서
사용합니다. 새 패키지 폴더가 빈 시험 DB를 의미하지는 않습니다. 별도 데이터 경로를
사용하던 설치라면 기존 `ARCHIVE_CENTER_DATA_DIR` 설정도 유지해야 합니다.
삭제 동작은 지워도 되는 시험용 채팅/세션에서 확인하세요. 첫 실행 때 포함된 DB
마이그레이션을 적용하며, 긴 기억 항목용 필드 확장을 포함합니다.

상속 기억을 독립 복사·편집하는 체크박스는 이번 빌드에 포함하지 않습니다.

이 자동 설치 패키지에는 Go 백엔드, `Archive Center.js`, migrations,
prompts와 설치 도구가 들어 있습니다. MariaDB와 ChromaDB 실행 파일은
Archive Center 패키지에 포함하지 않습니다.

## 처음 실행

별도의 제한시간 값을 `.env.full.local`에 입력할 필요가 없습니다.
Windows 실행기가 필요한 런타임 설치와 연결 준비를 확인합니다.
로컬 전체판은 ChromaDB 연결을 필수로 하며, 설치·기동·endpoint와
upsert/readback/delete 검증이 실패하면 백엔드를 시작하지 않습니다.

1. `01_start_archive_center_windows.bat`를 더블클릭합니다.
2. MariaDB가 없으면 공식 MariaDB 12.3.2 ZIP을 직접 다운로드합니다.
3. Python/ChromaDB가 없으면 공식 CPython 설치 파일을 다운로드하고 ChromaDB
   1.5.9를 설치합니다.
4. MariaDB와 Python은 SHA-256을 검증하며, Python은 Python Software
   Foundation의 유효한 Authenticode 서명도 확인합니다.
5. 검증된 런타임만 `%LOCALAPPDATA%\ArchiveCenter\runtime`에 설치합니다.
6. 설치에는 관리자 권한이나 Windows 서비스 등록이 필요하지 않습니다.
7. 기본 `full_local`/`bundled` 설정에서 MariaDB와 ChromaDB를 함께 시작합니다.
8. 검은 콘솔 창을 열어 둔 상태로 사용합니다.
9. RisuAI에 이 폴더의 `Archive Center.js`를 플러그인으로 등록합니다.

## 종료 또는 종료 취소

백엔드가 실행 중일 때 `Ctrl+C`를 누르면 Windows 실행기가 Archive Center와
관리 중인 MariaDB·ChromaDB를 함께 종료할지 묻습니다.

- `N`을 입력하면 종료를 취소합니다. 백엔드와 관리 중인 서비스는 같은
  프로세스로 계속 실행됩니다.
- `Y`를 입력한 경우에만 전체 정리 절차를 실행합니다.
- 질문이 표시되기 전에 `cmd.exe`의 일반 배치 종료 질문으로 서비스를 먼저
  내리지 않습니다. 종료 여부는 PowerShell 실행기가 먼저 결정합니다.
- `Y`로 실제 종료를 확인한 뒤 Windows가 `Terminate batch job (Y/N)?`을
  추가로 표시할 수 있습니다. 이 두 번째 질문에는 `Y`를 입력해 BAT 창을
  닫습니다. 여기서 `N`을 입력해도 이미 확인한 서비스 종료를 되돌리지는 않습니다.
- 정상 종료 뒤 BAT는 별도의 `pause`에서 대기하지 않고 종료됩니다. 실행 오류가
  발생한 경우에만 종료 코드를 읽을 수 있도록 일시 정지합니다.

같은 PC에서는 Bridge URL로 `http://127.0.0.1:28080`을 사용합니다. 다른
PC나 모바일에서는 서버 PC의 LAN IP, Tailscale 주소 또는 HTTPS 프록시
주소를 사용합니다.

## 기존 사용자 데이터

기본 데이터 위치는 `%LOCALAPPDATA%\ArchiveCenter\data`입니다.
`ARCHIVE_CENTER_DATA_DIR`를 지정하면 그 경로를 사용합니다. 예전 패키지 안에
`.runtime` 데이터가 있는 경우에는 기존 실행기의 검증된 가져오기 절차를 사용하며
원본을 보존합니다. 설정·API 키·사용자 DB는 이 배포 ZIP에 들어 있지 않습니다.

## 선택 설정

이미 별도로 설치한 MariaDB ZIP 런타임을 사용하려면 `.env.full.local`에
다음을 지정할 수 있습니다.

```text
AC_MARIADB_RUNTIME_DIR=C:\path\to\MariaDB
```

해당 폴더에는 `mariadbd.exe`, `mariadb-install-db.exe`, `mariadb.exe`,
`mariadb-admin.exe`가 있어야 합니다.

별도로 운영하는 ChromaDB를 사용하려면 endpoint를 `AC_CHROMA_ENDPOINT`에
지정하고 runtime/vector profile을 `vector_external`/`external`로
설정합니다. Windows 로컬 백엔드에서는 ChromaDB를 끌 수 없으며,
`core_lite`/`fallback`은 지원하지 않습니다. ChromaDB 실행 파일은 어느
경우에도 이 ZIP에 들어 있지 않습니다.

## 주의

- 배포 ZIP에 MariaDB·Python·ChromaDB 실행 파일, 사용자 DB, ChromaDB persist data,
  API 키를 넣지 마세요.
- 최초 MariaDB·Python·ChromaDB 설치 시 인터넷 연결이 필요합니다.
- 다운로드 파일의 SHA-256 또는 Python 서명이 맞지 않으면 설치를 중단합니다.
- `.env.full.local.protected`는 같은 Windows 사용자 계정에서만 복호화됩니다.
