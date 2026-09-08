# Archive Center 4.3.0-test.6

2026-09-06 담당별 Flex·제공자 설정 및 생성 설정 화면 보완.
상태: `implemented_unverified` — 소스·시험·패키지 증거와 실제 RisuAI 검증을 구분한다.

## 변경

- 일반 설정의 제공자 목록을 재사용한 선택창과 담당별 모델·API 연결을 제공한다.
- OpenAI, LLM Gateway, Vercel, NeuralWatt, Custom 및 Gemini(AI Studio)에서는
  기본값/Standard/Flex/Priority를, Vertex에서는 기존 Flex 모드를 선택한다.
  해당 제공자가 지원하는 모델에서 사용해야 한다. Claude 직접 연결은 Flex 항목이 없다.
- 설정은 기존 `memory-preprocessing.json`의 담당별 `llm_gateway_service_tier`,
  `vertex_flex_mode`에 저장한다. 새 필드의 빈 값은 기존처럼 별도 처리 모드를 보내지 않는다.
- 제공자 전환으로 숨긴 값은 보존한다. Go는 개별 연결에 해당하는 처리 모드만 적용한다.
  출판사 연결을 공유하면 출판사의 처리 모드를 따르며 별도 담당 프롬프트는 유지한다.
- `callMultiAgent()` → 기존 `applyProxyOverridesFromLLMConfig()` → 제공자 전송 경로를
  사용한다. OpenAI 호환은 `service_tier`, AI Studio는 최상위 `serviceTier`, Vertex는
  기존 Flex 헤더다. AI Studio의 Standard는 `standard`로 변환한다.
- 온도와 최대 출력 토큰을 AI 연결 아래에 펼쳐 두었다. 기존 담당별 값을 1차·보충 호출에
  적용하며 출판사 연결 공유 중에도 조절할 수 있다. 대기 한도·추론 설정은 세부 설정에 있다.
- 기존의 추천 없음/호출 실패 시 Go 선정, 보충 실패 시 첫 추천 보존을 유지한다.
  Flex 실패를 Standard 재호출로 바꾸거나 새로운 모델 제한 목록을 추가하지 않았다.
- JavaScript: test.5 패키지 대비 **31줄 추가, 13줄 삭제**. 제공자 선택·조건부 표시·
  편집값 전달과 배치·버전 표시 변경이다. 설정 저장과 전송 정책은 Go가 담당한다.
- 백엔드와 플러그인 모두 변경했다. 새 패키지만 생성하고 실행·설치는 사용자가 담당한다.

## 공식 자료 확인

2026-09-06 확인. 제공자 옵션 표시와 모델별 이용 자격은 구분한다. 모델별 실시간 지원
목록을 조회하거나 모든 계정의 Flex 이용 가능 여부를 검증한 기능은 아니다.

- [OpenAI Docs Flex processing](https://developers.openai.com/api/docs/guides/flex-processing):
  지원 모델의 Chat Completions/Responses 요청에서 `service_tier=flex` 사용.
- [Google GenerateContent Flex](https://ai.google.dev/gemini-api/docs/generate-content/flex-inference)와
  [API 계약](https://ai.google.dev/api/generate-content): AI Studio Flex와 최상위 `serviceTier` 확인.
- [LLM Gateway 서비스 티어](https://docs.llmgateway.io/features/service-tiers): 제공자/모델
  매핑별 지원이며 미지원 조합의 Flex 요청은 공급자 오류가 될 수 있다.
- [Claude 서비스 티어](https://platform.claude.com/docs/en/api/service-tiers): 직접 API의
  Standard/Priority/Batch와 `auto`/`standard_only`는 Flex 지원과 다르다.

## 검증

- HTTP API와 JS 회귀 패키지 전체 통과:
  `go test ./internal/httpapi ./cmd/js-route-variant-smoke -count=1`.
- 추가한 공유 연결 사례의 집중 회귀와 `go vet ./internal/httpapi` 통과.
- 서로 다른 5개 역할이 자신의 서비스 티어를 1차·보충 분석에 보내는지 검사했다.
- HTTP 전송 대역에서 OpenAI/Gateway/NeuralWatt Flex, AI Studio Flex/Standard/기본값,
  Vertex Flex 전용/예약 우선/끄기, Claude에 부적합한 개별 티어 미전송을 확인했다.
  각 경로의 1차·보충 및 출판사 공유 호출에서 온도 0.6과 최대 출력 3072가 실제 요청
  본문에 전달되는지 확인했다. NeuralWatt는 기존 SSE 응답을 정상 추천으로 복원했다.
- 설정 저장과 새 서버 객체에서의 재로드에 Flex 두 필드가 보존되는지 확인했다.
- 실제 UI 함수의 로컬 브라우저 시험: 제공자 전환, Flex 항목 표시, 숨긴 값 보존,
  온도·토큰 편집과 저장, 공유 중 생성 설정 활성 유지, 1600/900/390px 배치 통과.
- `node --check "Archive Center.js"`, `git diff --check` 통과.

화면 및 시험 기록: 작업 루트 `.tmp-feedback-43/preprocessing-ui-test6/`.
실제 유료 제공자의 처리 티어·요금·지연 및 설치된 RisuAI의 최종 주입은 미검증이다.

## 패키지

`_test-builds/4.3.0-test.6/Archive Center 4.3.0-test.6 Windows Auto Install Package.zip`

같은 폴더의 `SHA256SUMS-4.3.0-test.6.txt`에 ZIP 해시를 기록한다.
기존 설치 폴더를 덮어쓰거나 사용자 백엔드를 기동·종료하거나 RisuAI에 설치하지 않는다.

패키지 생성 성공. 매니페스트 53개 파일의 해시, 활성 JS와 패키지의 일치, test.5와
백엔드 실행 파일의 차이 및 ZIP 체크섬을 확인했다.

- ZIP SHA-256: `2FB78D11647E393A05B91625717B006674CC1DCBE8F7869FFAAB4377B33C6826`
- 백엔드 SHA-256: `F494F8ADCCD20FFA7E461D1F079028AAB946159CE1FB62135CA3867716F7BFED`
- JS SHA-256: `413DE1DF5EB2FA4B116371BF1425B2D60134748F22AA7D08717AB7D8BCFFE7F7`

해시 검증 기록: 작업 루트 `.tmp-feedback-43/preprocessing-ui-test6/package-receipt.json`.
