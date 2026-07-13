# Archive Center 2.1 Windows - 먼저 읽기

이 패키지는 Windows용 풀패키지입니다.

MariaDB, ChromaDB, Go 백엔드, Archive Center.js, migrations, prompts가 함께 들어 있습니다.
일반 사용자는 MariaDB나 ChromaDB를 따로 설치하지 않아도 됩니다.

## 처음 실행

1. `01_start_archive_center_windows.bat`를 더블클릭하세요.
2. 검은 콘솔 창이 열린 상태로 유지되면 서버가 실행 중인 것입니다.
3. RisuAI에 이 폴더의 `Archive Center.js`를 플러그인으로 등록하세요.
4. 같은 PC에서는 Bridge URL 기본값 `http://127.0.0.1:28080`을 사용하세요.
5. 다른 PC/모바일에서는 `localhost`가 아니라 브라우저가 접근 가능한 서버 PC의 IP 또는 도메인을 Bridge URL에 넣으세요.

## 정상 동작 확인

서버가 켜진 상태에서 `02_smoke_test_windows.bat`를 더블클릭하면 기본 동작 검사를 실행합니다.

## env 보호

`.env.full.local`에 API 키를 넣었다면 `04_protect_env_windows.bat`를 더블클릭하세요.
이 파일은 `.env.full.local`을 Windows 현재 사용자 계정 전용 DPAPI 암호화 파일인 `.env.full.local.protected`로 바꾸고, 평문 `.env.full.local`을 제거합니다.

설정을 다시 수정해야 할 때만 `05_unprotect_env_windows.bat`로 임시 복호화한 뒤, 수정 후 다시 `04_protect_env_windows.bat`를 실행하세요.

## 다른 PC에서 접속할 때

`01_start_archive_center_windows.bat` 하나로 같은 PC와 다른 PC/모바일 접속을 모두 처리합니다.
Go backend는 `0.0.0.0:28080`으로 열리므로 원격 브라우저도 접근할 수 있습니다.

원격 RisuAI의 Archive Center Bridge URL 예:

```text
http://서버_PC_IP_또는_도메인:28080
```

서버 주소는 같은 공유기/LAN IP, 직접 연결 IP, VPN/Tailscale IP, 포트포워딩된 공인 IP, 도메인 중 브라우저에서 실제로 접근 가능한 값을 사용하세요.
RisuAI 페이지가 HTTPS라면 브라우저가 HTTP 요청을 막을 수 있습니다. 그 경우에는 Tailscale Serve나 프록시로 28080을 HTTPS URL에 연결한 뒤 그 HTTPS 주소를 Bridge URL로 사용하세요.

## 주의

- `.runtime` 폴더는 사용자 PC에 생성되는 로컬 DB/런타임 데이터입니다.
- 배포 zip에는 사용자 DB, ChromaDB persist data, API 키가 들어 있으면 안 됩니다.
- `.env.full.local.protected`는 같은 Windows 사용자 계정에서만 복호화됩니다. 다른 PC나 다른 계정으로 복사하면 그대로 사용할 수 없습니다.
