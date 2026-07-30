# Archive Center Windows 시작 안내

이 자동 설치 패키지에는 Go 백엔드, `Archive Center.js`, migrations,
prompts와 설치 도구가 들어 있습니다. MariaDB와 ChromaDB 실행 파일은
Archive Center 패키지에 포함하지 않습니다.

## 처음 실행

별도의 제한시간 값을 `.env.full.local`에 입력할 필요가 없습니다.
Windows 실행기는 3.5 관리형 실행기와 같은 설치·준비 확인 흐름을 사용합니다.
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

같은 PC에서는 Bridge URL로 `http://127.0.0.1:28080`을 사용합니다. 다른
PC나 모바일에서는 서버 PC의 LAN IP, Tailscale 주소 또는 HTTPS 프록시
주소를 사용합니다.

## 기존 사용자 데이터

Archive Center의 `.runtime` 데이터 폴더는 그대로 사용합니다. 이번 변경은
MariaDB 실행 파일의 배포 위치만 분리하며 기존 MariaDB 데이터나 Archive
Center DB를 이동하거나 초기화하지 않습니다.

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
