# Archive Center 4.3.0-test.16 — 공개 기억 전달 누락 수정

날짜: 2026-09-07. 상태: `implemented_unverified`.
test.15와 이전 수정들을 포함한 Windows 테스트 빌드다.

## 수정 내용과 이전 누락의 원인

실제 기억에서 생성한 전처리 후보에는 `public_projection`이라는 공개 범위 표시가 붙는다.
`runMultiAgent()`는 `public`·`general`만 처리해 이 후보를 다른 담당에게 넘기지 못했다.
test.15 이전부터 있던 구현 누락이며 기존 테스트가 실제 생성 형식을 포함하지 못했다.

기존 공개 전달 분기에 `public_projection`을 포함했다. 해당 근거의 원문·짧은 참조·정본 ID·
출처 표·출처 참조·턴은 다음 담당의 2차 `related_evidence`로 유지된다. 관점 소유자·열람자·
주관 기억의 기존 범위는 유지한다. 다른 분류의 전달 근거는 참고 자료이며 수신 담당의
선정 후보로 재분류하거나 선택을 강제하지 않는다.

Go 런타임 증분은 **+2/-1**(기존 분기와 주석), JavaScript는 **+4/-4**(버전 표시만)다.
사용자 설정·키·프롬프트·DB·추가 AI 호출·시간 관계·검색 순위는 변경하지 않는다.

## 검증

- 수정 전 등록된 `/prepare-turn` 경로에서 실제 기억 조립·후보 생성을 거쳐 문제를 재현했다.
  출판사 OFF·성공·실패 세 경우 모두 다음 담당의 2차 요청에 근거가 없었고
  `related_reference_not_public` 진단이 남았다. 수정 후 같은 회귀를 통과했다.
- 시험 HTTP 제공자의 2차 요청에서 원문·ID/ref·출처·턴 일치를 확인했다. 두 활성 담당은
  기존 두 차수(4회)로 실행됐고, 공개 전달 누락 진단은 사라졌다.
- 선정된 원문은 최종 기억, 본문 주입 계획, 출판사 지원 자료에 유지된다. Go가 별도 기억으로
  바꾸지 않았으며 출판사 OFF·실패에서도 기존 전달이 유지된다.
- 기존 `public`·`general` 전달, 비공개/소유자/열람자/주관 범위를 확인했다.
  `public_projection` 표시와 소유자·열람자·주관 범위가 함께 있는 사례도 포함했다.
- `go test ./... -count=1`: **35개 패키지 통과**, 2개 패키지는 시험 파일 없음.
  첫 전체 실행에서 test.15를 기대하던 버전 표시 검증이 실패해 test.16으로 갱신하고
  전체를 다시 실행해 통과했다.
- `go vet ./internal/httpapi`와 `node --check "Archive Center.js"` 통과.

이는 source/fixture 검증이다. Store·벡터·제공자 외부 경계는 시험 대역을 사용하며,
실제 사용자 DB·Chroma·외부 AI·로드된 RisuAI와 최종 서사 품질을 검증한 것은 아니다.
최종 입력 관측 불일치, 과다 선택, 해석 분량과 상태 시점 문제는 이 수정의 해결 항목이 아니다.

## 패키지

- [Windows 테스트 ZIP](../_test-builds/4.3.0-test.16/Archive%20Center%204.3.0-test.16%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.16/Archive%20Center%204.3.0-test.16%20Windows%20Auto%20Install%20Package/Archive%20Center.js)

활성 소스와 Go **1.26.6**으로 백엔드·업데이트 도구·스키마 도구를 빌드했다.
ZIP 크기는 **18,069,675바이트**다. 관리 파일 **53개**의 디스크 크기·해시와 ZIP 내부
해시, 외부 ZIP 체크섬을 확인했다. 플러그인·Publisher 프롬프트는 활성 소스와 일치하며,
SQL **13개**는 현재 소스와 일치하고 `013_precise_memory_text_fields.sql`을 포함한다.
패키지와 `.env.full.example`의 버전은 test.16이고 백엔드 해시는 test.15와 다르다.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `C71A3095E2C495D03B0DEB25AF679BBFE8514B3C48DA2F0995EF1431278F23A2` |
| 백엔드 | `FED9DD818AF7BC81E3DF38CF8E107572236ED33FD095EE63DA5D810F814B608B` |
| 플러그인 | `D73368CCD948607E1E62BA9BB9068BBE91FE0FC4E8D8616110B5864E16A2CA20` |
| Publisher 프롬프트 | `7DD94804731C3BA18FEC4402C178D8781A3EC0A2CCD6D0AF10A016649B7AFA42` |

소스 HEAD는 `3e5e0dc99b3daafce5040418436b671f9a84ac78`이며 기존 미커밋 4.3 작업과
이번 수정을 포함한다. 이전 패키지를 덮어쓰거나 GitHub에 업로드하지 않았다.

## 종료 상태와 사용자 시험

사용자 요청으로 실행 중이던 test.15 `archive-center-go.exe`(PID 21832)를 경로 확인 후
종료했다. 종료 직후 남은 Archive Center Go 백엔드는 0개였다. 새 백엔드는 시작하지 않는다.
ZIP을 풀고 `01_start_archive_center_windows.bat`로 실행하며, 같은 패키지의 플러그인을
RisuAI에 적용한다. 기존 사용자 데이터는 일반 실행 경로를 통해 사용한다.

새 턴에서는 공개 근거를 요청한 경우 수신 담당의 2차 `related_evidence`에 같은 ID와
원문이 도착하는지 확인한다. 공개 `public_projection`이 다시 비공개로 진단되는지 구분하고,
담당이 최종 선택한 원문은 편집 확인의 기억 주입과 대조한다. AI가 참고 자료를 선택하지
않은 경우와 백엔드가 전달을 누락한 경우는 별개의 결과다.

증거 파일은 작업 루트 `.tmp-feedback-43`의 `test16-before-fix.log`,
`test16-targeted-tests.log`, `test16-go-all-final.log`, `test16-go-vet.log`,
`test16-build.log`, `verify-test16-package.ps1`, `test16-package-receipt.json`에 남긴다.
변경 전 사본은 `public-handoff-baseline`이며 기존 미커밋 작업을 보존했다.
[작업 기록](archive-center-4.3-preprocessing-work-log.md),
[현황](archive-center-4.3-status-summary.md), [STRUCTURE](../STRUCTURE.md),
[AI_GUARDRAILS](../AI_GUARDRAILS.md), 통합 로드맵에 같은 범위를 반영한다.
