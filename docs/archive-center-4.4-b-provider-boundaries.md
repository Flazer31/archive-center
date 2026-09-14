# 4.4-B — 호출별 차이 확인과 제한된 공통화

작성: 2026-09-10

## B 복원점과 전체 JS·Go 조합 검증

- 로컬 복원점: `0dfd9bfe544f4acda14f96593c90488e68e85ac3`. 외부 로드맵 사본·해시는 `_diagnostics/20260910-44b-checkpoint/`에 보관했다. GitHub에는 올리지 않았다.
- 전체 JS 실행에서 삭제된 `getAllowedReasoningPresetsForProvider`의 설정 로드/원작 검색 설정 호출 4곳을 발견했다. 저장값을 읽다가 예외가 나서 기본값으로 돌아가는 재현이다. 부분 DOM fixture는 `sanitizeSettings`를 실행하지 않아 놓쳤다.
- 이미 동일한 전체 enum으로 정규화하는 설정 로드의 중복 검사 3곳을 제거하고, 원작 검색 설정의 선택지는 기존 `REASONING_PRESET_OPTIONS`를 사용했다. 제공자 정책이나 허용값을 넓히지 않았다.
- `ops/settings-pair-smoke.cjs`는 **패키지 전체 JS + 실제 Go 실행 파일**을 사용한다. 실제 설정 DOM, 저장/정규화/읽기, bridge transport, `/config/view-model`, `/config/update`를 대체하지 않는다. RisuAI 호스트 API만 격리하고 브라우저 localStorage를 실제로 읽고 쓴다. 구 패키지에서 로드 실패, 수정 패키지에서 조회→저장→페이지 새로고침→재열기 통과를 확인했다.
- 수정된 패키지는 `4.4.0-test.2`다. `test.1`은 이 결함이 있어 교체용으로 사용하지 않는다. 별도 Go는 shadow/noop 모드이며 사용자 DB·Chroma·모델 호출을 사용하지 않았다.
- 증거: `_diagnostics/20260910-44b-pair/failure.json`, `_diagnostics/20260910-44b-pair-fixed/result.json`. 개별 키·빈 Endpoint·온도·토큰·추론 예산, 읽기 중 무저장, Go runtime sync ACK와 재열기 복원을 확인했다.
- 사용자는 4.3 세션을 계속 진행하므로 현재 PocketRisu를 교체하지 않는다. 실제 loaded RisuAI 검증은 `implemented_unverified`로 분리한다. 새 조합의 격리 검증 후 승인된 C를 진행한다.

상태: **RF01~RF03 소스 구현·로컬 회귀 확인 완료**.
실사용 검증 완료를 의미하지 않는다. 새 동작의 loaded RisuAI 검증은
`implemented_unverified`다. 초기 RF01~RF03 당시에는 빌드하지 않았으며, 이후 위의 test.2 격리 검증을 수행했다. 배포·실행 중인 백엔드 변경은 하지 않았다.

사용자 기준: 비슷하게 보인다는 이유로 합치지 않고 실제 설정·요청 차이부터 확인한다.
착수 HEAD는 `026dcbf3b45adcf69b254673d439b24e943115b3`이며 A의 미커밋 변경을 보존했다.
4.3.1 기억 기준, 역할별 모델/키/Endpoint/프롬프트/온도/토큰/시간 제한과 호출 횟수를 유지한다.

## 1. 공통 helper의 실제 호출 범위

`applyProxyOverridesFromLLMConfig`의 생산 호출부는 **11곳**이다.
문서의 세 주요 역할만 보고 helper 전체를 확대하면 다른 기능에도 영향을 준다.

| 호출 소유자 | 확인한 기존 차이 | 이번 처리 |
| --- | --- | --- |
| `group_proxy.go::runSupervisorLLM` | effort/preset/양쪽 budget 필드/GLM type 복사, 출력 토큰 기본 30000, Publisher 단일 호출 | 복사 부분만 분리 |
| `turn_extraction_critic.go::runCompleteTurnCriticWithInputPolicy` | 동일 추론 필드 복사, Critic 고유 기본값/JSON 계약/오류 처리 | 복사 부분만 분리 |
| `turn_extraction_critic.go::runCompleteTurnWorldRuleAudit` | 동일 추론 필드 복사, 자체 입력과 토큰 처리/JSON 계약 | 복사 부분만 분리 |
| `prepare_turn_multi_agent.go::callMultiAgent` | 역할의 effort만 복사. 공유 연결이어도 preset/budget/GLM type을 일괄 복사하지 않음. 역할별 토큰·온도·timeout·프롬프트 유지 | 기존 호출 그대로 |
| `group_audit_feedback_import.go` 1곳 | 추론 복사가 비슷하나 피드백 import의 별도 호출 | 변경하지 않음 |
| `group_narrative_hierarchy_generation.go` 2곳 | override helper만 사용, 자체 기본값 | 변경하지 않음 |
| `group_reference_library.go` 3곳 | 원작 추출/검토/시간순서 각각 자체 요청, override helper 사용 | 변경하지 않음 |
| `group_source_discovery.go` 1곳 | 후보 추출의 자체 토큰 우선순위와 호출 경로 | 변경하지 않음 |

기존 override helper는 headers/body, Vertex Flex, gateway tier, Claude cache만 계속 복사한다.
새 `turn_extraction.go::applyProxyReasoningFromLLMConfig`는 동일성이 확인된 위 세 곳만 호출한다.
기존 비어 있음/공백 판정, 양수 budget의 두 별칭, 원래 문자열 값과 request별 복사를 보존한다.
역할을 하나의 설정 객체로 합치거나 전처리를 Publisher 설정으로 바꾸는 작업이 아니다.

## 2. RF02 — UI 시험의 전역 설정 덮어쓰기 재현과 수정

기존 `withUiBridgeSettings`는 await 중 `settings.bridgeUrl`, `requestTimeoutMs`,
`webDirectBridgeEnabled`를 바꿨다. 시험 A와 B가 겹치면 일반 요청이 B의 주소를 사용하고,
A/B의 복원 순서에 따라 임시 값이 남을 수 있었다. A에서 확인한 helper 재현을 확장해
실제 UI 건강 확인 버튼, `testBridgeHealth`, `bridgeFetch`까지 실행한 로컬 회귀로 확인했다.

수정 후 같은 helper가 폼 값을 작은 request별 객체로 전달한다. `bridgeFetch`는 해당 객체의
주소/시간 제한/전송 모드를 사용한다. 객체를 받지 않은 일반 호출은 저장된 설정을 사용한다.
건강 확인의 `/health → /ready`, LLM 시험 2곳, wakeup, stats, update check/apply,
Critic ledger probe에 기존 경로를 따라 전달한다. 별도 큐/잠금/재시도/라우팅 정책은 추가하지 않았다.

확인 범위:

- A 먼저/B 먼저 완료하는 두 순서, 서로 다른 native/direct 전송과 timeout.
- 시험 중 일반 요청의 저장 주소 유지, 시험 중 새로 저장한 주소가 완료 후에도 유지.
- 빈 주소는 기존 저장 주소 사용, 명시적 timeout 0인 업데이트 요청 유지.
- 실제 건강 확인 및 Update Now 버튼 바인딩 실행. 네트워크/타이머만 외부 경계로 대체.
- timeout 결과는 기존 null/진단 동작 유지, 실패 진단에 해당 요청 주소/timeout 기록.

기존 두 source-marker 테스트는 함수 호출 인자와 임시 전역 대입 형태에 결합돼 있었다.
업데이트 단일 호출 검사는 인자가 있는 호출도 세도록 수정했고, 전역 대입을 요구하던 두 문구는
제거했다. 위 실제 실행 회귀가 전역 변경 재발을 검출한다. 폼 렌더러·저장 키·모델 선택 목록·
API 키 저장 방식은 수정하지 않았다. 전체 설정 저장/재열기의 실제 RisuAI 증거로 과장하지 않는다.

## 3. RF02·RF03 후속 구현 — 표시 계약과 요청 소유자

착수 시 JS의 `resolveReasoningControls`는 미저장 provider/model/endpoint의 표시 모드를
계산했고, `applyReasoningFieldsToPayload`가 모델별 필드를 구성했다. 현재는 두 정책을
Go의 `llm_settings_view.go`로 옮겼다. JS 함수의 이름을 남긴 `applyReasoningFieldsToPayload`는
`reasoning_input`에 사용자 입력 세 값만 담으며 모델 판단을 하지 않는다.

확인한 차이:

- Gemini 3.8 Flash UI는 none/low/medium/high를 표시한다. native의 none은 JS에서 필드를
  생략하고 gateway 경로는 명시적 none을 전송한다. 하나의 규칙으로 평탄화하지 않는다.
- GLM은 구형 toggle, 5.2+ effort, Ollama/gateway가 서로 다른 선택/변환을 갖는다.
- preset/budget은 전처리와 Publisher/Critic의 복사 범위부터 다르다.
- `runtime_config.go`의 trace는 설정 진단이며, 임의 미저장 입력을 받는 추론 UI ViewModel이 아니다.
  등록된 `POST /config/update`는 `updateRuntimeConfig`로 런타임 값을 변경한다.
  `GET /config/memory-preprocessing`도 저장된 전처리 설정/프롬프트 조회이며 범용 미저장 모델
  선택지 API가 아니다. 기존 계획의 “config ViewModel 사용”은 아직 구현된 수단으로 볼 수 없다.

기존 계획에서 예정한 config ViewModel의 실체가 없었으므로, 읽기 전용 표시 계약을 이번에
구체화했다. 별도의 모델별 reasoning 서비스나 JS 정책 사본을 만든 것이 아니다.

| 경계 | 현재 계약 |
| --- | --- |
| `POST /config/view-model` 입력 | `provider`, `model`, `endpoint`, `preset`, `currentEffort`, `currentBudget`, `previousSyncKey`, `isFirstSync` |
| 표시 응답 | `contract_version: llm_settings_view.v1`, `controls`, `allowedPresets`, `guideText`, `syncKey`, `nextEffort`, `nextBudget`, `family`, `presetInfo` |
| 조회 부작용 | 저장소 읽기/쓰기, 런타임 설정 변경, 외부 모델 호출 없음. API 키는 입력/응답 항목에 없음 |
| 실제 호출 | 기존 `POST /proxy/plugin-main`의 선택적 `reasoning_input: {preset, effort, budget}`. HTTP owner가 기존 DTO 필드로 변환 |
| 기존 호출 | `reasoning_input` 없는 flat HTTP 요청과 Go의 Publisher/Critic/audit/전처리 owner는 기존 경로 유지 |
| 적용 순서 | host 입력 확장 → 기존 Endpoint 처리 → 기존 Critic 연결 시험 1024토큰/추론 budget 0 처리 → 기존 provider 호출 |

ViewModel은 설정 화면에서만 조회한다. 본 LLM 요청은 기존 proxy 1회를 사용하므로
조회/LLM 왕복이 추가되지 않는다. 표시 정책은 기존 Go의 family·transport·버전별 helper를
사용하되, Go의 느슨한 custom GLM 별칭 인식을 새 UI 토글로 확대하지 않는다. `glmfoo`/`glm`은
변경 전처럼 UI가 추론 필드를 추가하지 않으며, 기존 provider transport의 처리도 바꾸지 않는다.

Publisher/Critic의 동일한 13개 제공자 목록·순서를 `renderLlmProviderOptions`로 묶었다.
추론 바인딩은 `bindLlmSettingsView`와 두 명시적인 prefix로 연결한다. 전처리는 빈 제공자 선택,
effort-only 필드, 담당별 설정, Publisher 공유 선택을 유지한다. 모양이 다른 설정 전체를 합치지 않았다.

조회 중 새 편집은 보존하고, 역순 도착한 예전 조회는 현재 폼을 덮어쓰지 않는다. 이는 해당 폼의
일시적 표시 순서이며 본문/저장/복구를 제한하는 조건이 아니다. 조회 실패 시 기존 입력을 유지하고
짧은 조회 실패 표시만 남긴다. 설정 저장 API를 조회에 사용하거나 JS 모델 정책으로 되돌아가지 않는다.

이 계약은 paired JS/backend 변경이다. 구형 백엔드는 새 ViewModel과 `reasoning_input` 확장을
지원하지 않으므로 JS 파일만 따로 배포할 수 없다. 이번에는 활성 백엔드나 설치된 플러그인을
교체하지 않았으며, 정식 패키지·자동 업데이트 검증은 E의 범위다.

## 4. RF01·RF02 첫 묶음 검증 자료

로컬 자료: `../../_diagnostics/20260910-44b-provider-boundaries/`.

- `baseline.json`, `preexisting.diff`, `before/`: A의 변경을 포함한 착수 상태와 대상 원문 hash.
- `wire-before/`, `wire-after/`: 실제 역할 owner가 만든 10개 요청의 URL/method/headers/body가 JSON 기준 동일.
  Publisher/Critic/audit/공유 전처리 1·2차 × 빈 추론/명시한 추론. 합성 키만 사용한다.
- `Test44RoleOwnersPreserveProxyMapping`: 외부 제공자만 429로 응답하며 실제 요청과 호출 1회를 검사.
  성공 응답, 독립 5역할, Flex/cache/extra override, 여러 provider transport는 기존 생산 회귀로 보완.
- `Test44OptionalReasoningMappingPreservesPresenceAndIsolation`: 공백/미지정/0 이하 budget,
  명시적 none, 별칭 값, request 간 독립성, 기존 override helper 범위 유지.
- `omit-publisher-reasoning.log`: 복사를 제거하면 effort와 토큰 검사가 실패.
- `overbroad-shared-reasoning.log`: 기존 helper에 추론을 과하게 합치면 전처리 budget 검사가 실패.
- `ui-before.log`: 수정 전 전역 설정 덮어쓰기가 실제 실행 회귀에서 실패.
- `go-provider.jsonl`: 410개 pass, 실패/skip 0. `js-smoke.jsonl`: 423개 pass, 실패/skip 0.
- `go test ./...`: **4,797개 pass, 실패 0, skip 13** (`go-full-native-temp.jsonl`).
  최초 실행의 `TestChromaPreflightValues`는 이 작업에서 지정한 `GOTMPDIR`의 `/`와
  `filepath.Abs`가 반환한 Windows `\`를 문자열로 비교해 실패했다. 임시 경로만 `Resolve-Path`의
  native 표기로 전달한 뒤 해당 검사와 전체 검사가 통과했다. 제품 코드/테스트 기대값을 바꾸지 않았다.
  최초 결과는 `go-full.jsonl`, 최종 집계/파일 hash는 `test-summary.json`, `artifact-manifest.json`에 남긴다.
- skip 13개는 실제 MariaDB 8개, Chroma 2개, POSIX executable bit 1개,
  공개 패키지 업그레이드 1개, 실제 provider smoke 1개다. 실제 설치/외부 계정 검증으로 계산하지 않는다.
  집계에는 subtest가 포함되며 여러 실행의 테스트는 겹치므로 합쳐서 고유 테스트 수로 보고하지 않는다.
- `node --check "Archive Center.js"`, `git diff --check` 통과.

실제 유료 모델 호출·실제 MariaDB/Chroma 조작·로드된 플러그인 교체는 수행하지 않았다.
이 첫 묶음의 JavaScript 변경: **+34 / -37줄**. 증가 부분은 UI 폼 값의 request별 bridge transport 전달이며,
기억 선정·추론 정책·설정 저장 소유자를 JS에 추가하지 않았다.

## 5. RF02·RF03 완료 검증 자료

로컬 자료: `../../_diagnostics/20260910-44b-settings-view/`.

- `before.js`, `preexisting.diff`, `capture-before.cjs`: 이번 묶음 착수 시 원문과 기존 JS를 실행한
  캡처. 정규화한 JS SHA-256은 `4fa7b2be1d92460468df58ff4226c7e56de83e906b9490452cf53a238cd59736`.
- 저장소의 `go-service/internal/httpapi/testdata/llm-settings-44-before.json`은 **486개** 변경 전 표본이다.
  Gemini 2.5/3 계열, GPT/O 시리즈, Claude budget/adaptive, GLM toggle/effort, DeepSeek 직접/중계,
  Ollama, 알려지지 않은 모델과 느슨한 별칭, preset 변경, none/빈 값/budget을 포함한다.
- `Test44SettingsViewAndRequestPreservePreMigrationMatrix`: 표시 모드·선택지·기본값·설명·동기화 키와
  실제 host 입력에서 확장되는 flat 필드를 변경 전 JS 결과와 비교한다.
- `Test44HostReasoningAndLegacyRequestsHaveIdenticalWire`: 각 표본을 등록된 proxy HTTP 경로로 보내,
  변경 전 flat 요청과 새 host-input 요청의 최종 URL/headers/body를 비교한다. 제공자 응답과 Vertex
  토큰 발급만 합성 HTTP 응답이다. 기존 Endpoint 충돌 오류도 그대로다.
- `Test44DraftSettingsViewDoesNotWriteOrCallProvider`: 저장소 없는 Server에서 조회하며, 저장된
  두 역할 모델이 변하지 않고 모델 호출이 없음을 확인한다. Gemini 3.8 medium은 독립 기대값으로 검사한다.
- `Test44HostReasoningPreservesCriticConnectionTestCap`: 신규 host 입력을 보낸 연결 시험도
  실제 Claude 요청에서 1024토큰과 thinking budget 제거를 유지한다.
- `ops/settings-view-ui-smoke.cjs`: 실제 `renderSettingsPanel`과 해당 설정/저장/복원/연결시험
  바인딩을 실제 Edge DOM에서 실행한다. 두 역할의 제공자 순서, 역순 조회, 조회 중 새 budget,
  조회 실패, 저장/재열기/복원, 빈 키/Endpoint, 숨은 Flex 값, 온도 0/각자 토큰, Publisher/Critic 시험,
  모바일 화면을 확인했다. 저장소/HTTP/관계없는 패널 부문은 fixture 경계다.
- `ops/preprocessing-ui-smoke.cjs`: 5역할의 독립/공유 설정, 저장·키·프롬프트 복원, Flex 표시,
  desktop/mobile 회귀를 통과했다. 전처리 폼의 다른 계약을 보존했다.
- 기존 JS 정책 테스트는 Go의 고정 표본/실제 wire 검사로 이전했다. 실제 UI 소비자와 얇은 입력
  관측 검사를 유지했다. provider 옵션의 옛 인라인 문법을 요구하던 검사도 공통 렌더러 호출로 옮겼다.
- 전체 Go 회귀 **5,770개 pass, 실패 0, skip 13**. 최신 JS 전체 회귀 **420개 pass, 실패/skip 0**.
  subtest를 포함하며 두 실행은 중복되므로 합산하지 않는다. skip 경계는 위 4절과 같다.
  초기 실패 로그도 보관한다: fixture URL 환경 누락·Vertex 합성 인증/빈 Custom Endpoint·제거한
  JS 테스트의 미사용 import를 수정했고, 확대 표본에서 발견한 loose GLM 별칭의 UI/transport 차이는
  원래 동작을 유지하도록 수정했다. 마지막 범위 검토에서 공통 UI 함수의 지역 `$` 의존성도
  직접 DOM 조회로 수정했다. fixture의 `$`도 실제처럼 이벤트 함수 안에만 두어 수정 전에는
  `ui-closure-before.log`의 `$ is not defined` 실패가 재현되고 수정 후 통과함을 확인했다.
  실패를 기대값 변경으로 통과시키지 않았다.
- `node --check "Archive Center.js"`, `git diff --check` 통과. 전체 B JavaScript **+140 / -673줄**.

RF01~RF03의 소스와 로컬 검증을 마쳤다. 실제 RisuAI에 변경된 JS/backend를 함께 로드한 결과,
실제 DB/모델 계정, 배포 패키지 검증은 이 결과에 포함하지 않는다. C~E는 이번에 진행하지 않았다.
