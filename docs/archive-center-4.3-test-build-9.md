# Archive Center 4.3.0-test.9 — Vertex 평론가 설정 전달 수정

날짜: 2026-09-07. 상태: `implemented_unverified` — 소스 및 제공자 경계 시험 검증,
설치된 RisuAI·실제 Google 계정 검증 대기.

## 수정

다른 제공자의 Standard/Flex/Priority 값이 설정에 남아 있으면 Vertex 평론가의
연결 테스트와 실제 호출이 `llm_gateway_service_tier requires provider ...`로
Google 요청 전에 실패했다. Endpoint 형식이나 Vertex 지원 자체의 문제가 아니다.

기존 Go 요청 조립 함수 `proxyApplyLLMGatewayServiceTier()`에서 Vertex에는
해당 서비스 등급을 적용하지 않고 정상적으로 다음 요청 조립을 이어간다.
`VertexFlexMode`의 기존 전용 헤더가 Vertex 처리 방식을 결정한다. 다른 제공자에
사용할 저장값은 유지하며, Flex/Priority를 임의로 다른 처리 모드로 바꾸지 않는다.

기존 진단 항목 `llm_gateway_service_tier_applied=false`,
`llm_gateway_service_tier_skip_reason=vertex_uses_vertex_flex_mode`로 구분한다.
다른 제공자의 검증, 명시적인 추가 본문 설정, 상위 API 오류와 재시도는 변경하지 않았다.
JS에 제공자 판단을 추가하지 않았으며 기억 선정·평론가 결과 처리·저장은 수정하지 않았다.

## 검증

- `TestVertexRetainedServiceTierAcrossCriticPaths`: 등록된 `/config/update`,
  `/proxy/plugin-main?connection_test=critic`, 실제 `runCompleteTurnCritic()`을 실행한다.
- 두 호출 경로 × 공통 등급 4종(빈 값/standard/flex/priority) × Vertex 모드 3종
  (off/provisioned_then_flex/flex_only)의 24개 조합을 검사한다.
- 수정 전에는 비어 있지 않은 공통 등급의 18개 조합에서 제보 오류를 재현했다.
  수정 후 24개 조합이 통과했다. 일반 Vertex와 Flex 헤더, JSON 응답 정책,
  온도/출력 한도, 응답 본문의 평론가 요약, 호출 후 설정 보존을 확인했다.
- HTTP 대역은 OAuth 요청과 지정된 모델 URL만 허용한다. 각 실행의 OAuth 1회와
  생성 1회를 확인하며 다른 제공자용 tier 필드가 Google 본문에 없는지 검사한다.
- 관련 기존 Vertex·서비스 등급·Claude 옵션·전처리 회귀도 통과했다.
- `go test ./... -count=1` 전체 패키지, `go vet ./internal/httpapi`,
  `node --check "Archive Center.js"`, diff 공백 검사를 통과했다.

검증 자료: 작업 루트 `.tmp-feedback-43/vertex-tier-before.log`,
`vertex-tier-after.log`, `vertex-tier-all-tests.log`, `vertex-tier-vet.log`.
실제 Google 인증·모델 이용 권한·RisuAI 설치 환경은
검증하지 않았다. 실제 자격 증명이나 사용자의 RP 데이터를 시험에 사용하지 않았다.

## 패키지

생성 완료:
`_test-builds/4.3.0-test.9/Archive Center 4.3.0-test.9 Windows Auto Install Package.zip`.

사용자가 새 패키지의 백엔드를 실행해야 수정이 적용된다. 패키지 생성 과정에서
기존 백엔드를 시작·종료하거나 기존 설치 폴더와 RisuAI 플러그인을 교체하지 않는다.
활성 JS와 패키지 JS의 일치, test.8 대비 백엔드 변경, 패키지 버전 설정,
매니페스트의 53개 파일과 ZIP 내부 동일 파일의 SHA-256 및 외부 ZIP 체크섬을 확인했다.

- ZIP: `4810C8AA850346F759E6A09D561F27AD30782C0E8AA56BB8835DADD22305DBA6`
- 백엔드: `B3ABAAAB68624B927323D3BAE217C39A59490536848C120A746489627C523061`
- JS: `BEC4D654AB8BFA715432B9BAB9A4C9B27A996C2D55D8794822B770C00F602819`

검증 영수증: 작업 루트 `.tmp-feedback-43/vertex-tier-package-receipt.json`.
빌드 로그: 같은 폴더의 `vertex-tier-build.log`.

JavaScript 변경량: test.8 활성 소스 대비 **4줄 추가, 4줄 삭제**.
네 줄 모두 버전 표시이며 동작 코드는 변경하지 않았다.
