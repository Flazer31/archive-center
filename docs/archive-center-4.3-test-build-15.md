# Archive Center 4.3.0-test.15 — 담당별 해석의 독립 전달

날짜: 2026-09-07. 상태: `implemented_unverified`.
test.14를 포함한 Windows 테스트 패키지다. 이번 승인 범위까지만 구현하고
추가 개선은 사용자의 실제 출력 평가 이후에 판단한다.

## 포함 내용

- 기존 전처리의 선택 이유와 미해결 질문을 기억원문과 구분해 출판사와 본문 AI에 모두 전달한다.
  담당·원문 참조·공개 범위·관점 소유자·열람자 정보와 실제 채택된 분석 차수를 함께 표시한다.
- 보충 호출이 실패하면 기존 첫 추천과 해석을 유지한다. 기억과 로어북이 서로 다른 차수의
  결과를 채택한 경우 각각 맞는 이유를 붙인다. Go 선정에 AI의 이유를 새로 만들어 넣지 않는다.
- 출판사 OFF·호출 실패에도 본문용 담당 해석은 유지된다. 출판사 기본 지침은 담당 해석을
  근거와 함께 읽고 사용자의 의도·설정 변경을 우선해 장면·반응·연출에 연결하도록 설명한다.
- 편집 확인에 **전처리 담당별 해석** 구역을 표시하고, 분량 표에는 **추가 전달** 글자 수를
  표시한다. 기억원문이나 가이드를 줄여 그 자리를 채우지 않으므로 전체 입력량은 늘 수 있다.

새 AI 호출·차수·후처리·설정 항목은 없다. 기존 사용자 편집 프롬프트는 유지하며,
이미 받은 이유·질문은 기본값 복원 없이도 전달된다. 실제로 이유·질문이 없는 결과에는
추가할 해석문이 없다. 모델이 해석을 얼마나 활용하는지와 최종 이야기 품질은 실제 시험 대상이다.

## 검증

- 수정 전 `/prepare-turn` 회귀는 이유가 본문 계획과 출판사 지원 자료에 없어서 실패했다.
- 수정 후 `go test ./internal/httpapi ./internal/dto -count=1` 통과.
- `go test ./cmd/js-route-variant-smoke -count=1` 통과.
- `go vet ./internal/httpapi ./internal/dto`, `node --check "Archive Center.js"` 통과.
- 시험 HTTP 제공자를 사용한 실제 `/prepare-turn` 경로에서 출판사 OFF·성공·실패를 실행했다.
  성공은 `applied`, 실패는 `failed_open`을 확인했고 양쪽에서 본문 해석을 유지했다.
  활성 출판사 호출은 기존처럼 1회이며 실제 요청 본문에 담당별 해석이 포함된다.
- 기존 선정·조립 함수로 보충 성공·실패·부분 오류·정상 빈 추천, 독립 로어북 차수,
  순서와 인물 공개 범위 메타데이터를 확인했다. 추가 분석 호출은 없다.
- 실제 JS 적용·편집 확인 함수에서 해석문이 최종 입력 관측에 유지되고 별도 제목으로
  한 번 표시되는 것을 검증했다. 실제 RisuAI 화면이나 유료 모델의 출력 검증은 아니다.

이 시험은 source/fixture 증거다. 실제 사용자의 DB·Chroma·AI 제공자와 연결한 한 턴,
사용자 의도·기억 일관성·비밀 유지·표시된 서사의 품질까지 검증한 것으로 보지 않는다.
JavaScript 변경은 작업 전 사본 대비 **+14/-7**이며 버전 표시·번역·렌더에 해당한다.

## 패키지

- [Windows 테스트 ZIP](../_test-builds/4.3.0-test.15/Archive%20Center%204.3.0-test.15%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.15/Archive%20Center%204.3.0-test.15%20Windows%20Auto%20Install%20Package/Archive%20Center.js)
- ZIP 크기: **18,069,655바이트**.
- Go 1.26.6으로 백엔드·업데이트 도구·스키마 도구를 활성 소스에서 빌드했다.
- 관리 파일 **53개**의 디스크 크기·해시, ZIP 내부 해시와 외부 ZIP 체크섬을 확인했다.
- 플러그인과 Publisher 프롬프트는 활성 소스와 일치하며 백엔드도 test.14와 다른 새 빌드다.
- 플러그인 표시·버전·BUILD_ID, 패키지 식별자와 `.env.full.example`은 `4.3.0-test.15`다.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `549BA97421D3442B3F80876F129E1049115854D3BEF992DFA1C6C18DAABC7896` |
| 백엔드 | `8A5FE59DBE2D93076E6C4878C2F6989E3E16F9952CDC5124BBE9B20689F56A18` |
| 플러그인 | `FA4C4D3CB036541AE06F1E5D238830981C378872271502211EA2A19E0E5DAABB` |
| Publisher 프롬프트 | `7DD94804731C3BA18FEC4402C178D8781A3EC0A2CCD6D0AF10A016649B7AFA42` |

기존 실행 중인 백엔드의 시작·종료·교체나 RisuAI 플러그인 설치는 하지 않았다.
DB, API 키, 사용자 설정, 이전 테스트 패키지를 수정하지 않았다. GitHub 업로드도 하지 않았다.
이번 패키지의 백엔드와 플러그인을 함께 사용하고, 한 턴 뒤 편집 확인에서 기억원문 →
전처리 담당별 해석 → 출판사 안내를 읽은 뒤 최종 출력과 비교하면 된다.

작업 루트 `.tmp-feedback-43`에 `test15-build.log`, `test15-go-tests.log`,
`test15-js-tests.log`, `verify-test15-package.ps1`, `test15-package-receipt.json`을 남겼다.
소스 HEAD는 `3e5e0dc99b3daafce5040418436b671f9a84ac78`이며 기존 미커밋 4.3 작업을 포함한다.
범위와 소유자는 [전처리 작업 기록](archive-center-4.3-preprocessing-work-log.md),
[STRUCTURE](../STRUCTURE.md), [AI_GUARDRAILS](../AI_GUARDRAILS.md)에 함께 반영했다.
