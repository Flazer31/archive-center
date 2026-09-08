# Archive Center 4.3.0-test.17 — HUD 간소화

날짜: 2026-09-07. 상태: `implemented_unverified`.
test.16과 이전 수정을 포함한 Windows 테스트 빌드다.

## 변경

- 이전 턴 확정 모드의 현재 생성 카드에는 저장 건수표를 표시하지 않는다.
  이전 턴 저장 카드에는 실제 건수를 접힌 항목으로 표시한다. 즉시 저장 모드는 건수를
  유지하며, 실제 저장 결과가 0건일 때도 그 결과는 확인할 수 있다.
- 전체·준비·응답 시간을 세 칸으로, 전처리 다섯 담당을 1차·2차·AI/Go 표로 표시한다.
  보정/부분 상태는 짧은 표시와 전체 상태 도움말을 함께 유지한다.
- 추가 검색, 백엔드 준비, 전체 진행 단계, 저장 세부 건수는 기본적으로 접는다.
  개별 호출 합계는 담당 이름 도움말로 옮긴다. 카드 폭은 268px이며 화면 폭에 맞춰 제한한다.
- Host가 응답을 수신한 현재 카드에는 대기 단계도 수신 완료로 표시한다.
  완료·오류 카드의 상세 항목을 눌러도 닫히지 않으며 X로 닫는다. 기존 안내 카드는 유지한다.
- HUD의 합산 관련 설명문 세 개와 전처리 설정의 Flex 설명문을 제거한다.

JavaScript **+136/-100**, Go 런타임 **+0/-0**. JS 변경은 기존 RisuAI DOM 표시,
언어별 문구, Host 시각의 표시와 빌드 식별자다. 백엔드 상태·저장·검색·AI 추천·프롬프트·
제공자 기능·설정 전달은 변경하지 않는다. 현재 생성 카드에서 건수가 빠지는 것은
`projectTurnWorkflowHUDPhaseView()`의 표시 변환이며 Go 저장 동작을 생략하는 코드가 아니다.

## 검증

- 수정 전 생성 단계에 저장 건수가 그대로 남는 회귀를 재현했고 수정 후 통과했다.
- `go test ./cmd/js-route-variant-smoke -count=1` 및 `node --check "Archive Center.js"` 통과.
  실제 UI 함수와 Host 대역에서 생성/저장 분리, 실제 0건, 응답 수신 표시, 원본 backend 상태
  보존, X 닫기·카드 상세 탐색·리스너 정리와 기존 정보성 안내의 닫기를 확인했다.
- 실제 HUD 렌더러에서 만든 화면을 Chrome의 1440px·390px 폭으로 확인했다.
  다섯 담당이 정상인 현재 생성 카드는 약 **383px** 높이다. 생성 단독·이전 저장 동시·
  보정/부분/실패 사례에서 기본 접힘, 상세 펼치기와 가로 넘침 없음을 확인했다.
- 기존 전처리 UI 브라우저 시험도 통과했다. Flex 설명 제거와 함께 제공자 설정·역할별
  프롬프트·키·온도·토큰·저장 실패 편집 보존·모바일 레이아웃의 기존 동작을 확인했다.

소스/Host 대역/격리 브라우저 검증이다. 설치된 RisuAI, 실제 사용자 DB·외부 AI에는
연결하지 않았다. 실제 플러그인을 적용한 뒤의 모습은 사용자 확인이 남아 있다.

## 패키지

- [Windows 테스트 ZIP](../_test-builds/4.3.0-test.17/Archive%20Center%204.3.0-test.17%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.17/Archive%20Center%204.3.0-test.17%20Windows%20Auto%20Install%20Package/Archive%20Center.js)

활성 소스와 Go 1.26.6으로 빌드했다. ZIP은 **18,069,994바이트**이며 관리 파일
**53개**의 디스크 크기·해시, ZIP 내부 해시와 외부 체크섬을 확인했다.
플러그인·Publisher 프롬프트·SQL 13개는 활성 소스와 일치한다. 백엔드 실행 파일은
**test.16과 같은 해시**다. UI만 바뀌었으므로 기존 test.16 백엔드에 새 플러그인을
적용해 HUD를 확인할 수 있다. 전체 패키지의 버전·실행용 환경 예제는 test.17이다.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `776A74F7B323E84472CF30F16E2A503CB839F5D1CA05D0D3F1AF08EC14F14DE6` |
| 백엔드 | `FED9DD818AF7BC81E3DF38CF8E107572236ED33FD095EE63DA5D810F814B608B` |
| 플러그인 | `BF1B72A9AED7D070778E770777CFF7B091621E630386A1AC9659B2D930A65FAA` |
| Publisher 프롬프트 | `7DD94804731C3BA18FEC4402C178D8781A3EC0A2CCD6D0AF10A016649B7AFA42` |

기존 미커밋 4.3 작업을 포함한 소스 HEAD는
`3e5e0dc99b3daafce5040418436b671f9a84ac78`이다. GitHub 게시와 커밋은 하지 않았다.

## 사용자 확인과 증거

이전 턴 확정 설정으로 새 턴을 받아 현재 카드에 저장 0건 목록이 사라지는지 확인한다.
이전 턴의 저장 카드는 별도로 표시되고, 실제 건수는 해당 카드에서 펼쳐 확인한다.
완료된 현재 카드에는 전체·준비·응답 시간과 담당별 1차·2차·선정이 보여야 한다.
상세 펼치기를 누르면 카드가 유지되고 X로 닫혀야 한다.

이번 작업에서 실행 중인 백엔드를 종료하거나 새 백엔드를 시작하지 않았다.
새 패키지를 별도 경로에 만들며 기존 test.16 패키지를 덮어쓰지 않는다.

작업 루트 `.tmp-feedback-43`의 `test17-before-fix.log`, `test17-js-tests-final.log`,
`test17-hud-browser.json`과 `test17-hud-*.png`, `test17-preprocessing-browser.log`,
`test17-build.log`, `test17-package-receipt.json`을 증거로 남긴다.
변경 전 사본은 `hud-compact-baseline`이다.
[작업 기록](archive-center-4.3-preprocessing-work-log.md),
[현황](archive-center-4.3-status-summary.md), [STRUCTURE](../STRUCTURE.md),
[AI_GUARDRAILS](../AI_GUARDRAILS.md), 통합 로드맵을 함께 갱신했다.
