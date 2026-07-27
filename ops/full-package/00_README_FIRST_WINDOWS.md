# Archive Center Windows 시작 안내

이 자동 설치 패키지에는 Go 백엔드, `Archive Center.js`, migrations,
prompts와 설치 도구가 들어 있습니다. MariaDB와 ChromaDB 실행 파일은
Archive Center 패키지에 포함하지 않습니다.

## 처음 실행

1. `01_start_archive_center_windows.bat`를 더블클릭합니다.
2. MariaDB가 없으면 공식 MariaDB 11.4.10 ZIP을 직접 다운로드합니다.
3. SHA-256 검증에 성공한 파일만
   `%LOCALAPPDATA%\ArchiveCenter\runtime\MariaDB`에 설치합니다.
4. MariaDB 설치에는 관리자 권한이나 Windows 서비스 등록이 필요하지 않습니다.
5. 기본 `core_lite`/`fallback` 설정에서는 ChromaDB를 설치하거나 시작하지 않습니다.
6. 검은 콘솔 창을 열어 둔 상태로 사용합니다.
7. RisuAI에 이 폴더의 `Archive Center.js`를 플러그인으로 등록합니다.

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

Vector 검색을 사용하려면 별도로 운영하는 ChromaDB endpoint를
`AC_CHROMA_ENDPOINT`에 지정하고 runtime/vector profile을
`vector_external`/`external`로 설정합니다. ChromaDB는 이 ZIP에 들어 있지
않습니다.

## 주의

- 배포 ZIP에 MariaDB·ChromaDB 실행 파일, 사용자 DB, ChromaDB persist data,
  API 키를 넣지 마세요.
- 최초 MariaDB 설치 시 인터넷 연결이 필요합니다.
- 다운로드 파일의 SHA-256이 다르면 설치를 중단합니다.
- `.env.full.local.protected`는 같은 Windows 사용자 계정에서만 복호화됩니다.
