# 4.3.1 → 4.4 UI 업데이트 연속 검증

검사일: 2026-09-15. **4.3.1의 원본 업데이트 버튼 처리 코드를 사용한 격리 브라우저
시험에서, 클릭 한 번으로 백엔드 패키지가 4.4.0으로 적용·재시작·확정되는 것을 확인했다.**
이 문서는 [앞선 설치·업데이트 검사](archive-center-4.4-install-update-verification.md)의
파일 적용 검사 이후에 자동 재시작과 브라우저 클릭을 추가로 확인한 기록이다.

## 관측한 순서

1. 별도 MariaDB·ChromaDB와 실행한 이전 서버가 `/version=4.3.1`, store/vector ready를 반환했다.
2. 시험 화면의 **업데이트 확인**을 클릭했다. `current: 4.3.1`, `latest: 4.4.0`,
   `update: available`, `compatibility: compatible`, Windows Update ZIP 선택을 표시했다.
3. **지금 업데이트**를 한 번 클릭했다. 원본 4.3.1 callback이 `POST /update/apply`를
   호출했고, HTTP 200 / `accepted` / 종료 코드 75 전달을 확인했다.
4. 이전 관리 실행기가 종료 코드 75를 받아 보류 중인 패키지를 적용했다.
   수동 파일 복사, 별도 재실행 명령, 두 번째 업데이트 클릭을 하지 않았다.
5. 브라우저에서 실행 버전 확인을 눌러 `/version=4.4.0`, 업데이트 상태 `committed`를 확인했다.
   이 버튼은 결과 조회만 수행하며 적용·재시작을 실행하지 않는다.
6. 실제 MariaDB 시험 레코드, 환경 파일 SHA-256과 세 포트 파일이 유지됐다.
   적용된 관리 파일 **58개**의 해시가 후보 manifest와 일치했다.

서비스 포트는 Go 28197 / MariaDB 33197 / ChromaDB 8197, 시험 UI는 28198이었다.
시험 종료 후 모두 닫혔고 사용자의 기존 Go·MariaDB·Python PID와 시작 시각은 유지됐다.

## 공개 전 시험의 경계

4.4는 아직 GitHub에 공개하지 않았으므로, 다음 두 외부 경계를 시험용으로 연결했다.

- **GitHub 응답:** `v4.3.1`의 소스 커밋
  `0821d69e9be9bc406bd9f0092a03398498f23b5f`에서 진단용 Go 실행 파일을 만들었다.
  `updateHTTPClient`만 후보 릴리스 정보·ZIP 바이트를 반환하도록 Go overlay를 적용했다.
  버전 선택·사전 검사·다운로드 처리·스테이징·응답 flush·종료 요청·서버 종료 코드는 그대로다.
  따라서 배포된 4.3.1 Go 실행 파일 자체를 변경 없이 실행한 증거로 표현하지 않는다.
- **RisuAI 호스트:** 공개 4.3.1 ZIP의 JS에서 버튼 click callback, 업데이트 API 함수,
  설정 적용 함수와 결과 렌더링 함수를 그대로 추출해 시험 화면에 연결했다.
  실제 브라우저에서 버튼을 클릭했고, Risu bridge 전송만 별도 시험 서버로 연결했다.
  사용자 RisuAI에 설치된 플러그인을 교체하거나 실제 채팅 설정 화면에서 클릭한 것은 아니다.

**4.3.1의 관리 실행기·updater·JS 원본은 변경하지 않았다.** 진단용 Go 실행 파일에 맞춰
격리된 이전 패키지 manifest의 해당 파일 해시만 갱신했다. 실제 업데이트로 적용된 4.4
후보는 앞선 검사를 통과한 `4.4.0-verification-20260915-fixed` ZIP 그대로다.
진단 overlay나 시험 UI는 배포 후보에 들어 있지 않다.

이번 검사는 기존 updater를 수동 실행한 것에서 더 나아가, 브라우저 클릭 → 이전 서버의
업데이트 처리 → 종료 75 → 관리 실행기 → 새 서버의 정상 준비·확정까지 이어서 검사했다.
실제 GitHub 공개 다운로드와 RisuAI 전체 호스트 환경은 정식 공개 후 별도로 확인한다.

## 사용자에게 안내할 적용 범위

기존 **관리형 설치**는 Archive Center UI의 업데이트 확인 → 지금 업데이트로
백엔드 다운로드·적용·재시작을 이어간다. 수동으로 실행한 비관리형 Go 서버나 별도
systemd 구성은 이 Windows 관리 실행기 결과와 구분한다.

**RisuAI에 등록된 `Archive Center.js`는 이 버튼이 자동 교체하지 않는다.**
패키지 내부 JS는 4.4로 갱신되지만, 사용자는 RisuAI 플러그인 업데이트 또는 같은 버전의
JS 가져오기를 별도로 해야 한다. 따라서 “백엔드는 버튼으로 자동 업데이트”는 확인됐지만
“RisuAI 플러그인까지 버튼 하나로 전부 교체”라고 안내해서는 안 된다.

## 증거

`_diagnostics/20260915-431-ui-update/`:

- `summary.json`, `result.json`: 자동 재시작·commit·보존·정리 결과.
- `ui-requests.jsonl`: 확인 요청 1회, 적용 POST 1회, 이후 4.4 버전·commit 조회.
- `launcher-stdout.log`: 종료 코드 75와 보류 패키지 적용 및 새 서버 실행.
- `release-requests.jsonl`: 교체한 외부 HTTP 요청 기록. 실제 GitHub 통신으로 세지 않는다.
- `fixture-provenance.json`, `431-update-ui-extracted.js`, `overlay.json`: 원본과 시험 경계.
- `processes-before.json`, `processes-after.json`: 기존 사용자 서비스 보존.

이번 추가 검사는 제품 실행 코드를 변경하지 않았다. **JavaScript 추가 0줄 / 삭제 0줄.**
