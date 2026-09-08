# Archive Center 4.3.0-test.13 — 기존 기억 전달 정확성

날짜: 2026-09-07. 상태: `implemented_unverified`.
test.12의 기능과 이후 [전달 정확성 보완](archive-center-4.3-preprocessing-work-log.md)을 포함한
Windows 테스트 패키지다. 실제 RisuAI/제공자/표시 출력의 검증은 별도다.

## 포함 내용

- 누적 인물 상태의 갱신 턴과 개별 사실의 발생 시점을 구분하는 출처 설명 및 렌더 표시.
- 전처리 후보와 공개 역할 간 전달에서 원문·출처 종류·턴을 함께 유지.
- 확인된 완료와 아직 모르는 세부 내용을 구분하는 공통 기본 전처리/출판사 프롬프트 보완.
- 기존 '최근 대화 참고 수'에 따른 사용자 입력·AI 응답 쌍이 두 차수에 전달되는 회귀 확인.
- 사용자 편집 프롬프트, AI 추천의 원문·순서, 추천 부재 시 Go 선정과 기존 검색 경로 유지.

이번 빌드 작업의 JavaScript 변경은 버전·빌드 설명 `+5/-5`다. 전처리와 기억 정책의
백엔드 소유권은 유지하며 새 AI 호출이나 상태 판정 기능을 추가하지 않았다.
기본 지침 변경은 저장된 사용자 편집 프롬프트를 덮어쓰지 않는다. 사용자가 기본값 복원을
선택하면 최신 기본 지침을 사용할 수 있다.

## 패키지와 확인 결과

- [Windows 테스트 ZIP](../_test-builds/4.3.0-test.13/Archive%20Center%204.3.0-test.13%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.13/Archive%20Center%204.3.0-test.13%20Windows%20Auto%20Install%20Package/Archive%20Center.js)
- ZIP 크기: 18,055,541바이트.
- Go 1.26.6으로 백엔드·업데이트 도구·스키마 도구를 활성 소스에서 새로 빌드했다.
- 관리 대상 53개 파일의 디스크/ZIP 내부 해시·크기와 외부 ZIP 체크섬 일치.
- 플러그인과 기본 Publisher 프롬프트의 활성 소스/패키지 해시 일치.
- 백엔드 해시가 test.12와 다름. 플러그인 표시/버전/BUILD_ID 모두 `4.3.0-test.13`.
- 실행 스크립트의 패키지 버전과 `.env.full.example`도 `4.3.0-test.13`.
  기존 실행 스크립트는 보존된 사용자 환경 파일보다 패키지의 빌드 식별자를 우선한다.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `6D0ECCD43461D6A5150034619DAD44CC75A6BC9424FE75A874A950D52AE0B2E9` |
| 백엔드 | `017077C67915DFA4072C49FF81FE9EC0998640706A40CCD98AA576B1DC1D377E` |
| 플러그인 | `196B615E6A82371B2616FBC5BA220F578DF97F0CF76691ADCF806D00D7DF1D6F` |
| Publisher 프롬프트 | `A8D33F560DB28DC88A3306F2ECD3471AE99BC58F95EE8FAB5AC8D7C04CE24FF9` |

## 검증 범위와 실행 경계

전달 정확성 수정에서 `go test ./internal/httpapi ./internal/vector ./internal/dto -count=1`,
`go vet ./internal/httpapi`를 통과했다. 이번 버전 갱신 후 활성 `Archive Center.js`의
문법 검사와 `go test ./cmd/js-route-variant-smoke -count=1`도 통과했다.

사용자 백엔드 시작/종료, DB/키/저장 설정 수정, RisuAI 플러그인 설치를 수행하지 않았다.
기존 test.12 패키지를 보존하고 test.13 폴더와 ZIP을 새로 생성했다. 패키지 검증은
실제 AI의 의미 이해나 본문 일관성을 보장하는 증거가 아니다.

작업 루트 `.tmp-feedback-43`의 `test13-build.log`, `test13-js-tests.log`,
`verify-test13-package.ps1`, `test13-package-receipt.json`에 빌드·검증 기록을 남겼다.
소스 HEAD는 `3e5e0dc99b3daafce5040418436b671f9a84ac78`이며 기존 미커밋 4.3 작업을 포함한다.
