# Archive Center 4.3.1-test.1 — 포트 변경 확인용

2026-09-09 로컬 Windows 테스트 패키지. GitHub 업로드·태그·공개 릴리스는 진행하지 않았다.
실행 중인 사용자 백엔드는 시작·종료하지 않았다.

## 패키지

- 폴더: `source/_dist/4.3.1-test.1/Archive Center 4.3.1-test.1 Windows Test`
- ZIP: 같은 위치의 `Archive Center 4.3.1-test.1 Windows Test.zip` (18,106,182 bytes)
- SHA-256: `de302a89d8a788ad9390190144dbcca6cef36f3c2144eff663481e9df64fbf53`
- 활성 소스에서 Go 1.26.6으로 만든 Windows 관리형 패키지다. 기존 기본 데이터
  루트와 설치된 MariaDB/ChromaDB 런타임을 이어 사용한다. DB를 별도로 복사하지 않았다.
- Go 기본 소스 버전은 공개 4.3.0으로 유지한다. 패키지 실행기가 설정하는
  `AC_BUILD_VERSION`, 패키지 JS의 메타데이터와 `VERSION`은 `4.3.1-test.1`로 일치한다.

## 사용자 확인

1. 실행 중인 Archive Center를 평소 방법으로 종료한다.
2. 테스트 폴더의 `06_change_chromadb_port_windows.bat`를 열고 `8001`을 입력한다.
3. 같은 폴더의 `01_start_archive_center_windows.bat`를 실행한다.
4. 실행 창에 ChromaDB 주소가 `http://127.0.0.1:8001`로 표시되고,
   백엔드가 정상 시작하며 RisuAI에서 연결되는지 확인한다.
5. 종료 후 `01`을 다시 실행해 8001이 유지되는지 확인한다.

다시 기본 포트를 쓰려면 `06`에서 `8000`을 저장하고 재시작한다. RisuAI의
백엔드 접속 주소(기본 28080)는 그대로 둔다. 포트만 시험할 때 기존 플러그인도
사용할 수 있으며, 테스트 버전 표시를 맞추려면 패키지의 `Archive Center.js`를 사용한다.

## 검증과 범위

- 관리 파일 54개의 크기·해시와 ZIP 내부 내용 일치, 새 `06` 파일 포함 확인.
- 플러그인 표시명·메타데이터·내부 버전, 실행기와 환경 템플릿의 버전 일치 확인.
- 패키지 JS 구문 검사 통과.
- 패키지의 실제 구성 전용 진입점을 격리한 데이터 폴더로 실행해 8001 저장 확인.
  이 검사는 서버를 시작하지 않는다.
- 이전 턴의 Windows/POSIX 포트 설정, 신규 설치, 업데이트·스키마 도구 회귀 결과는
  [포트 기능 기록](chromadb-port-configuration.md)에 연결한다.
- 이번 작업에서 Windows 빌더가 시험 버전에도 JS 메타데이터·내부 버전을 반영하도록
  기존 버전 치환 조건을 확장했다. 활성 `Archive Center.js` 변경 **+0/-0**.
- 사용자 환경에서의 실제 ChromaDB 포트 기동·연결·재시작 유지 확인은 사용자 시험 대기다.
  공개 UI 업데이트, Android 기기, 외부 AI 호출·기억 품질은 이번 패키지 검증 범위가 아니다.
