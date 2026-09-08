# Archive Center 4.3.0-test.14 — JSON 형식 복구와 전처리 결과 활용

날짜: 2026-09-07. 상태: `implemented_unverified`.
test.13 이후 승인된 다섯 항목을 포함한 Windows 테스트 패키지다.
소스·회귀·패키지 검증과 실제 RisuAI/제공자/표시 출력 검증을 구분한다.

## 포함 내용

- 전처리·출판사·평론가의 공통 JSON 문법 복구. 명확한 다음 필드/객체 경계 앞의 배열
  닫는 괄호 누락을 복구하며 문자열 내부의 원문과 원래 제공자 응답을 보존한다.
- 전처리의 독립 필드 해석과 단일 문자열 목록 수용. 잘못된 이유 필드 뒤의 정상 선택·검색을
  읽고, 끊긴 목록에서 이미 받은 ID를 보존한다. 추가 AI 복구 호출이나 세 번째 분석은 없다.
- HUD의 형식 보정/부분 해석/추천 없음/실패 구분과 최종 기억 선택의 AI/Go 출처 표시.
- 짧은 F/S/L 참조·간결한 이유를 요청하는 기본 지침과 로어북 참조 옆 이름 표시.
- 현재 입력이 암묵적인 경우 최근 대화에 명시된 미해결 목표가 후보 생성 전에 빠지는 경로 수정.
  이미 설정된 최근 대화 질의 집합을 기존 목표 관련성 판정에 연결한다.

모델 크기나 제공자를 제한하지 않는다. AI가 선택한 원문·순서, 추천 부재 시 기존 Go 선정,
실패한 보충 호출의 첫 추천 보존과 사용자 이야기 결정권을 유지한다. 저장된 사용자 편집
프롬프트는 덮어쓰지 않는다. 최신 기본 지침을 사용하려면 해당 기본값 복원을 선택한다.
파서·HUD·목표 후보·로어북 이름 변경은 프롬프트 복원 여부와 별개로 적용된다.

## 검증

- 수정 전: 단일 문자열 검색 질문, 배열 닫는 괄호 누락, 최근 대화의 목표 누락 회귀가
  기존 production 파서/assembly에서 실패하는 것을 확인했다.
- 수정 후: `go test ./internal/httpapi ./internal/dto -count=1` 통과.
- `go test ./cmd/js-route-variant-smoke -count=1` 및 HUD 상태 렌더 회귀 통과.
- `go vet ./internal/httpapi ./internal/dto`, `node --check "Archive Center.js"` 통과.
- 실제 호출 함수 `runSupervisorLLM`과 `runCompleteTurnCritic`을 시험 제공자 HTTP 경계로
  실행해 호출 각 1회로 원래 가이드·추출이 복구되는 것을 확인했다. 실제 유료 AI 호출은 아니다.
- 전처리 생산 경로의 선택 순서·뒤 필드 보존·기존 1회 검색/2차 분석과 HUD 최종 선택 출처,
  로어북 이름/ID 대응 및 무관한 목표의 기존 제외를 확인했다.
- Publisher의 기존 `items:[,]` 진단 테스트는 같은 입력이 빈 목록으로 복구됨을 검증하도록
  갱신했다. 공통 파서의 기존 마지막 쉼표 복구에 해당하며 가이드 항목을 만들지 않는다.
  복구할 수 없는 `items:[?]`의 구문 위치 진단 검증도 유지한다.
- 실제 JS 렌더 함수로 280px 폭 HUD 검증 HTML을 생성했다. 브라우저 URL 정책이 로컬 파일
  열기를 차단하여 시각 검증은 수행하지 못했다. 렌더 회귀 통과가 실제 Host 화면 증거는 아니다.

JavaScript 변경은 작업 전 사본 대비 **+26/-7**이다. HUD 렌더·번역과 버전 표시가 해당하며,
판단·형식 복구·검색·선정은 Go 소유다. 세부 내용은 [전처리 기록](archive-center-4.3-preprocessing-work-log.md)을 따른다.

## 패키지

- [Windows 테스트 ZIP](../_test-builds/4.3.0-test.14/Archive%20Center%204.3.0-test.14%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.14/Archive%20Center%204.3.0-test.14%20Windows%20Auto%20Install%20Package/Archive%20Center.js)
- ZIP 크기: **18,063,283바이트**.
- Go 1.26.6으로 백엔드·업데이트 도구·스키마 도구를 활성 소스에서 빌드했다.
- 관리 파일 **53개**의 디스크 크기·해시, ZIP 내부 해시와 외부 ZIP 체크섬을 확인했다.
- 패키지 플러그인·Publisher 기본 프롬프트가 활성 소스와 일치하고, 백엔드 해시가 test.13과 다르다.
- 플러그인 표시·버전·BUILD_ID, 패키지 식별자와 `.env.full.example`은 `4.3.0-test.14`다.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `B8515131A75CEFC8701A3383963C2D552EDEAA63716EF5E0808FC7754A1516AD` |
| 백엔드 | `8D504E10B3E6AF49DDF48066C2A267BE0A515BF31D51C1CBC1EFD345F82C1554` |
| 플러그인 | `413B981E79CE13CE61FD9B779BF28D49D6DAE41E9E3312366A1800D7F17909E2` |
| Publisher 프롬프트 | `A8D33F560DB28DC88A3306F2ECD3471AE99BC58F95EE8FAB5AC8D7C04CE24FF9` |

처음 패키지 시도는 시스템 PATH의 Go 1.26.5 때문에 버전 검사에서 중단됐다. 이번 작업이
생성한 빈 test.14 대상만 다시 만들고, 시험과 같은 Go 1.26.6 경로로 빌드했다.
기존 test.13 이하 패키지와 사용자 백엔드 실행 상태·DB·키·설정은 변경하지 않았다.
RisuAI 플러그인 설치와 실제 제공자 재시험, 본문 일관성 확인은 사용자 환경 검증으로 남아 있다.

작업 루트 `.tmp-feedback-43`에 `test14-build.log`, `test14-go-tests.log`,
`test14-js-tests.log`, `test14-hud.html`, `verify-test14-package.ps1`,
`test14-package-receipt.json`을 남겼다. 소스 HEAD는
`3e5e0dc99b3daafce5040418436b671f9a84ac78`이며 기존 미커밋 4.3 작업을 포함한다.
