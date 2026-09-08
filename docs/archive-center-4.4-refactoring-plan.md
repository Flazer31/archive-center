# Archive Center 4.4 리팩터링·의미 통합 실행 계획

상태: `PLANNED` / `VERSION_ASSIGNED_PLAN`  
작성일: 2026-09-07 / 기준 갱신: 2026-09-08  
범위: 4.4-A~E의 향후 파일별 작업과 검증 순서. 이 문서 작성은 구현·테스트·배포를 수행하지 않는다.

## 1. 출발점과 버전 경계

- 계획 권위는 [통합 로드맵의 4.4](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)이며, 이 문서는 실행 항목을 구체화한다.
- 현재 문서 기준은 활성 `source/`와 로컬 test.21 패키지이다. 4.3 전체 완료나 live 검증 완료를 전제하지 않는다.
- [test.12 기록](archive-center-4.3-test-build-12.md)의 병렬 보충검색·pristine 후보 재사용·검색 시간 분리는 이미 반영된 기준선이다.
- [4.3 상태 요약](archive-center-4.3-status-summary.md)에서 구현 범위와 잔여 검증을 확인한 뒤 실제 4.4 기준선을 다시 확정한다.
- [4.3 전처리 작업 기록](archive-center-4.3-preprocessing-work-log.md)과 test.7~21 기록은 근거 위치를 찾는 색인으로 사용한다.
- 실제 4.4 시작 시 HEAD, dirty diff, 적용 설정, 활성 package/loaded Host 식별을 다시 기록한다. 문서 작성 시점의 상태를 재사용하지 않는다.
- 4.4-A~C의 리팩터링은 같은 입력에 대한 기존 결과·부작용·실패 의미 보존이 목적이다.
- 4.4-D의 cross-surface 의미 통합은 전달 표현과 중복 처리의 **의도된 동작 변경**이다. 리팩터링 동등성 검사와 별도 계약·사례로 검증한다.
- 4.4-E는 두 작업의 결합을 확인한다. 어느 한쪽의 성공으로 다른 쪽의 미검증 항목을 완료 처리하지 않는다.

### 2026-09-08 기준선 추가

test.18부터 포함된 `priority_score.static.v4`의 의미 점수 전달·독립 중요도/최근성,
인물별 지식·공개 기록 연결, 하이파 원문 하나당 기억 하나의 가져오기, 224px HUD와
완료 카드의 펼치기/닫기는 4.3의 기존 동작이다. 4.4에서 새로 구현할 항목으로 세지 않는다.
test.21의 분류별 핵심 우선 수·남은 문자 예산 전달, 일반 주관 기억, 현재 필드 출처 판정과
보완 검색의 별도 증거 참조도 이미 반영된 기준선이다. 4.4-A는 이 동작과 예산·AI 추천 순서·
출처 범위를 기준 사례에 포함한다. 과거의 K 최대 개수 절단을 동등성 기준으로 복원하지 않는다.
4.4-C의 UI 정리는 현재 박스 배치·접기·이전 턴 저장 표시·클릭 동작을 보존한다.
각 수정의 파일/검증은 [4.3 현황](archive-center-4.3-status-summary.md)과
[test.21 기록](archive-center-4.3-test-build-21.md)을 따른다.
점수 보정은 상태의 실제 시점이나 표현이 다른 사실의 연결을 해결한 것으로 취급하지 않는다.

### 기본 회상 우선 목표와 공통 비교 자료

[통합 계획의 목표·버전 지도·여섯 비교 사례](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#good-memory-plan)를 따른다.
4.4의 의미 중복 정리는 같은 예산에서 필요한 세부 기억을 유지할 공간을 확보하는 작업이다.
다른 사건·시점·관점의 차이와 수신 AI 추천을 보존하고, 중복량 감소와 기억 품질 개선을 따로 평가한다.

4.4-A는 오래된 중요 사건·작은 세부사항·유사 사건·상태 변화·다른 실마리·일반 RP의 비교 입력을
이어받는다. 전처리 OFF/ON × 출판사 OFF/ON, 추천 없음·담당 일부 실패·보완 실패를 같은 자료로
확인한다. 구조 정리는 기존 결과의 동등성을, 의미 통합은 필요한 사실의 회수·전달과 변경 목적을
각각 검증한다. 검색 후보·선정·실제 입력·출력 활용을 분리해 기록한다.

5.1-A~5.2의 공통 재활성화와 5.1-B·5.3~5.7의 선택형 인물 표현은 후속 계획이다. 4.4에 새
연상 엔진이나 망각 정책을 넣지 않으며, 선택 기능 OFF의 기본 회상을 이후에도 기준선으로 유지한다.

### 2026-09-07 추가 인계 — 상태의 시점과 검색 후보 혼재

주 계획은 [통합 로드맵의 4.4·4.6·4.7 공통 인계](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#temporal-state-44647)다.
사용자 test.15 자료에서 같은 인물 기록의 현금 51냥 5푼과 자산 설명 속 61냥이 모두
115턴 후보로 공급됐고, 직전 대화에는 46냥 5푼이 있다. 누적 상태의 갱신 턴을 개별 항목에
전달하는 현재 소스와 연결되는 관측이다. 이전 시점 안내·프롬프트 보완이 이 문제를 해결한 것은 아니다.

- **4.4-A:** 변하지 않은 항목의 시점, 다른 필드에 남은 과거 금액, 직전 대화의 지출을
  구분하는 비교 사례를 준비한다. 원문·저장 행·항목 출처·점수·AI 입력의 현재 경로를 기록한다.
- **4.4-C:** 자료 전달을 정리하면서 시점의 의미를 임의로 바꾸지 않는다. 항목 시점 보존은
  별도 기능 변경이므로 동등성 리팩터링에 숨겨 넣지 않는다.
- **4.4-D:** 같은 사실의 중복 표현을 줄이되 시점이 다른 금액을 하나의 사실로 합치지 않는다.
  최신 저장 revision만으로 현재 유효한 금액이 확정된 것으로 보지 않는다.
- **4.6 인계:** 현금·자산·재고까지 개별 변경·유효 시점을 보존하는 저장/후보 구성 수정과
  원문이 남은 기존 자료의 복원을 구분해 수행한다. 주 대상은 기존 상태 병합·저장·사실 분해 소유자다.
- **4.7 인계:** 바로잡힌 시점을 현재/과거 질의의 검색 후보·점수·AI 입력에 반영하고 각각 비교한다.

이는 **PLANNED** 범위다. 4.4 완료만으로 상태 혼재 해결을 선언하지 않으며, 4.3에
추가 프롬프트 조정·호출을 반복하는 작업으로 되돌리지 않는다. 모든 과거 자료의 완전 복원과
모든 모델의 시간 해석을 보장하는 목표는 아니다. 세부 사례와 증거 기준은 통합 문서를 따른다.

## 2. 모든 단계에서 보존할 계약

1. 사용자의 이야기 방향·수정·속도·선택이 우선이다. strength는 기존 표현 지침이며 수신 결과의 거부 조건이 아니다.
2. 수신한 유효 AI 추천의 원문·참조·순서를 유지한다. 추천 없는 영역은 기존 Go 선택을 사용하고 그 이유를 구분한다.
3. 부분 실패·보완 실패 시 기존 첫 추천 보존과 lorebook의 명시적 빈 선택/미평가 구분을 유지한다.
4. Go는 정책·선택·예산·조립·저장, JS는 Host 관측·전달·실제 payload 적용·표시 확인·UI를 소유한다.
5. MariaDB canonical source, Chroma 파생 검색, source revision·분기·시점·private owner/viewers·삭제/reroll 범위를 보존한다.
6. 새 거부 조건·2차 조건·retry·fallback·watcher·queue·병렬 정책 경로를 리팩터링 편의로 추가하지 않는다.
7. 최종 K·예산·출처/보안 정책을 단순화 명목으로 바꾸지 않는다. 진단 필드는 출력·저장 허용 여부를 결정하지 않는다.
8. 변경 비교에서 요구하는 동등성은 개발 검증 기준이다. 이를 runtime 출력 수락 조건으로 구현하지 않는다.
9. 파일 이동·공통화만을 위한 새 프레임워크나 서비스 계층을 만들지 않는다. 기존 함수·파일·ViewModel을 우선 정리한다.
10. [AGENTS](../AGENTS.md), [AI_GUARDRAILS](../AI_GUARDRAILS.md), [Host/Go 경계](permanent-risu-host-backend-boundary.md), [4.0 기억 계약](4.0-memory-restoration-work-contract.md)을 적용한다.

## 3. 단계·실행 순서

| 단계 | 목적 | 작업 | 완료 판단 |
| --- | --- | --- | --- |
| 4.4-A | 실제 기준선·검증 공백 확보 | 기존 결과 캡처, 테스트 실호출 여부, RF08의 기대값 독립성 검토 | 비교 입력/결과/부작용과 미검증 범위가 재현 가능 |
| 4.4-B | provider·설정 UI 중복 정리 | RF01 → RF02 → RF03 | 역할별 request와 UI 저장/시험 동작 동등 |
| 4.4-C | 기억 조립·내부 전달·HUD 정리 | RF04 → RF05, 측정 후 RF06, 독립 slice RF07 | source·추천·payload·진단 보존 및 변경 이유 확인 |
| 4.4-D | 기존 의미 중복 통합 기능 | family·대표 표현·출처 coverage·충돌 보존 계약 | 의도한 전달 변경과 보존할 차이가 사례별 확인 |
| 4.4-E | 통합·실환경 확인 | B/C 동등성, D 변경 결과, 실제 Host/provider 비교 | 증거 단계별 결과와 잔여 한계 명시 |

RF08의 저장 함수 위치 정리는 A의 테스트 보강과 별개이며, 삭제 계약과 관련 검증이 준비된 뒤에만 작은 독립 작업으로 다룬다.
`beforeRequest`, `orchestrate`, final 저장/reroll은 초기 공통화 대상에서 제외하고 마지막에 실제 중복과 영향 범위가 확인된 항목만 다시 검토한다.

## 4. 4.4-A — 기준선·검증 공백

- 활성 `Archive Center.js`·`go-service`에서 경로와 파일을 확인하고 기존 dirty 변경의 작성자를 구분한다. 정리 목적으로 기존 변경을 되돌리지 않는다.
- 동일 입력·설정에서 provider request, 선택 ID/순서/원문, final memory text, payload plan, source lineage, 오류 코드·호출 횟수를 캡처한다.
- OFF, 독립/공유 connection, 빈 추천, 일부 역할 실패, 보완 실패, private/lorebook, auto/custom 예산 사례를 포함한다.
- 테스트를 source-string 검사, 실제 JS/Go 함수 실행, 등록 API+경계 fixture, 실제 DB/provider, loaded Host로 구분한다.
- `main_settings_loading_43_test.go:10`처럼 이벤트 연결을 stub한 테스트를 저장·시험 버튼 실행 증거로 사용하지 않는다.
- [vector/fake.go](../go-service/internal/vector/fake.go#L38)의 실제 `fakeVectorStore`는 `Search()`·`Health()`·`Count()`에서 공유 호출 기록 필드를 변경한다(38/105/117).
- [mutation_fence.go](../go-service/internal/vector/mutation_fence.go#L49)의 이 세 조회는 `RLock`을 사용하므로 조회 사이의 기록 변경은 서로 직렬화되지 않는다(49/73/79). 동시성 문제의 확정 전 상태는 `SUPPORTED_RISK`이다.
- **생산 연결:** [config.go:385](../go-service/internal/config/config.go#L385)의 기본 mode와 `VectorPolicySatisfied()`(514)는 core_lite fallback/off를 지원한다. [NewServer](../go-service/internal/httpapi/server.go#L91)는 비활성 Chroma/빈 endpoint 분기와 Chroma 생성 오류 분기에서 `NewFakeVectorStore()`를 선택하고 105~106에서 mutation fence로 감싼다.
- 실제 설정·startup 검증 결과에 따른 접근 가능성을 구분하고, 이 생산 대체 store의 동시 조회를 재현한다. 재현된 공유 기록 변경만 `vector/fake.go`의 기존 owner에서 좁게 수정하며 `Search` 무결과·Health 상태·Count 의미·기존 startup 오류를 유지한다.
- `vector/vector_test.go`·`vector/mutation_fence_test.go`의 실제 delegate/wrapper와 `config/config_test.go`의 core_lite/vector profile 사례를 사용한다. HTTP 테스트 fixture인 `group_memory_part03_test.go`를 수정 대상으로 대신 잡지 않는다.
- 이 위험을 운영 더미 기억 오염으로 단정하지 않는다. 실제 메모리 오염이나 서비스 실패는 별도 재현 증거가 있어야 하며 새 fallback이나 출력 거부 조건을 추가하지 않는다.
- 원래 body·호출 횟수·SQL 기대값과 독립적인 실패 검출을 확보한다. production 함수를 복제해 기대값을 만드는 테스트는 보강한다.
- 4.3의 loaded RisuAI, 실제 provider 응답 품질/시간, 전체 native OS, race-detector 미검증을 각각 유지한다.
- 아래 JS `main_*_test.go`의 위치는 `go-service/cmd/js-route-variant-smoke/`이다.
- Go HTTP 테스트는 `go-service/internal/httpapi/`, SQL 테스트는 `go-service/internal/store/`의 활성 파일을 사용한다.
- 알려진 검사 목록이 모두 통과한다는 가정은 하지 않는다. 실제 4.4 시작 시 기존 실패와 새 실패를 구분한다.
- 변경 전후 비교 입력과 기대 결과는 한 쌍으로 보존하고, 이후 의미 통합 사례의 기대값과 섞지 않는다.

## 5. 4.4-B — provider·설정 UI

### RF01 — 기존 LLM 설정→proxy 요청 매핑 공통화

- **현재:** [turn_extraction.go:884](../go-service/internal/httpapi/turn_extraction.go#L884)의 `applyProxyOverridesFromLLMConfig()`가 extra headers/body·Flex·tier·cache를 전달한다.
- **대상:** [group_proxy.go:294](../go-service/internal/httpapi/group_proxy.go#L294), [turn_extraction_critic.go:587](../go-service/internal/httpapi/turn_extraction_critic.go#L587) 및 867의 반복 reasoning 매핑.
- **방법:** 동일한 선택적 config 필드 매핑을 기존 helper로 옮기고 세 호출부의 중복을 제거한다. 역할별 message/purpose·token 기본값·timeout·retry budget은 호출 owner에 남긴다.
- `prepare_turn_multi_agent.go`의 `callMultiAgent()`는 현재 helper를 523에서 사용한다. 독립 연결과 Publisher 공유 연결의 서로 다른 기본값을 유지한 상태에서 공통 필드만 연결한다.
- **호출 보존:** Publisher 단일 요청, Critic 본 추출/세계규칙 audit, specialist 양 round 모두 기존 `performProxy...` 경로를 사용한다.
- **검증:** `group_proxy_test.go`, `group_proxy_part02_test.go`, `prepare_turn_multi_agent_test.go`의 실제 요청 body/headers를 비교한다.
- 빈 값·미지정·명시적 0, temperature/token 값, service tier·Vertex Flex·Claude cache·extra overrides, 오류/횟수/취소를 포함한다.
- helper 단위 검사만으로 끝내지 않고 `TestProxyReasoningContractIsSharedByPublisherAndCritic`과 등록 config/connection-test 경로를 확인한다.

### RF02 — provider 설정 폼·이벤트의 반복 표현 정리

- **대상:** [Archive Center.js](../Archive%20Center.js)의 provider 옵션(84), 전처리 폼(51582), Publisher(52011), Critic(52133), `renderSettingsPanel()`(51742)·`attachSettingsEvents()` 연결(52679).
- **방법:** 실제로 같은 옵션·필드 표현만 기존 UI 생성/바인딩 방식으로 묶고 각 역할의 저장 키·DOM ID·기본값·변경 이벤트를 명시적으로 연결한다.
- 전처리 Flex 표시(51615)와 일반 설정 표시(53508~53591)의 Gemini 차이는 현재 의미를 가진다. 모양이 비슷하다는 이유로 표시 범위를 통합하지 않는다.
- **검증:** `ops/preprocessing-ui-smoke.cjs`와 실제 `renderSettingsPanel()`/이벤트 함수를 실행한다. desktop/mobile, 편집 중 재렌더, 복원·저장·재열기·명시적 빈 값·숨겨진 값 보존을 확인한다.
- `main_settings_loading_43_test.go`의 loading 검사는 유지하되 버튼 이벤트 stub을 저장/connection-test 검증으로 계산하지 않는다.
- **조건부 위험 확인:** `withUiBridgeSettings()`(53350)는 async 작업 중 전역 `settings`를 임시 변경하고 `bridgeFetch()`(14222)가 이를 읽는다.
- 겹친 UI 시험/일반 요청의 영향은 아직 확정 결함이 아니다. 재현 후 영향이 확인된 호출에 한해 기존 transport의 요청별 설정 전달로 좁혀 수정한다.
- 이 확인을 이유로 전역 설정 체계·요청 queue·새 bridge fallback을 재설계하지 않는다.

### RF03 — reasoning UI와 Go 정책의 경계 정리

- **대상:** JS `detectReasoningFamily()`(11212), `resolveReasoningTransport()`(11254), `resolveReasoningControls()`(11315), `applyReasoningFieldsToPayload()`(11568).
- **Go owner:** [proxy_provider.go:138](../go-service/internal/httpapi/proxy_provider.go#L138)의 transport 및 173~223의 family/요청 변환, `runtime_config.go`와 기존 config ViewModel 응답.
- **방법:** provider/model/endpoint 정책은 Go의 기존 결정을 사용하도록 정리하고 JS는 선택지·표시·사용자 입력 관측을 맡는다. 필요한 UI 정보는 기존 config ViewModel에서 제공한다.
- 새 reasoning endpoint나 JS 정책 사본을 만들지 않는다. ViewModel 정보 범위와 편집 중 미저장 값 처리는 B의 request 비교로 구체화한다.
- 기존 동작이 서로 다르면 먼저 어떤 실제 경로가 어느 값을 보내는지 기록한다. 지원 모델·값·기존 오류 의미를 임의로 통일하지 않는다.
- **검증:** JS의 실제 시험 버튼(53877~53913, 53963~53996), `main_part10_test.go:152/260`, Go `group_proxy_test.go`의 wire/family/공유 reasoning 계약을 함께 확인한다.
- 정상 요청 body·UI 선택 가능 값·미저장 편집·backend 오류 표시를 비교한다. 정책 이전의 준비 작업은 `preparatory`로 보고한다.

## 6. 4.4-C — 기억 조립·내부 결과·HUD

### RF04 — 23개 위치 인자와 perspective 내부 전달 정리

- **대상:** [prepare_turn_assembly.go:59](../go-service/internal/httpapi/prepare_turn_assembly.go#L59)의 `buildPrepareTurnInjectionAssemblyWithBudget()`와 `group_turn_prepare.go:1022/1062` 호출.
- **방법:** 기존 입력을 역할이 드러나는 request-local 내부 입력 구조로 치환한다. 23개 인자와 `assemblyPerspectiveContext`의 내부 정책·질의·semantic facts 전달을 함께 명시한다.
- `prepare_turn_assembly.go:1030~1044`의 재포장과 `prepare_turn_priority_memory.go:1804~1811`의 문자열 key 해석을 순서대로 정리한다.
- 외부 request DTO·공개 perspective shape·Store interface는 이 작업만으로 바꾸지 않는다. 관측과 내부 정책을 같은 필드로 재분류하지 않는다.
- **호출 보존:** `/prepare-turn` → 기본 조립 → 선택적 `runMultiAgent()` → 기존 priority plan → Publisher/payload.
- **검증:** `prepare_turn_priority_memory_test.go`의 생산 조립·독립 K·auto/custom·private metadata, `prepare_turn_candidate_pool_test.go`의 pristine/deep-copy/JSON 비노출.
- `group_turn_perf_test.go:466` 현재 logical turn 이전 generation 제외와 `group_turn_part14_test.go:83` confirmed worldline 범위를 유지한다.

### RF05 — vector 내부 hit 전달과 공개 trace 구분

- **대상:** [prepare_turn_recall.go:140](../go-service/internal/httpapi/prepare_turn_recall.go#L140)의 vector shadow 생성, 427의 private precise hit, 530의 hydration, handler 572/1060의 삭제.
- **방법:** 기존 검색 결과의 내부 hit 전달을 공개 진단 맵과 구분한다. 같은 검색 owner 안에서 타입·반환 경계를 정리해 수동 private key 삭제 의존을 줄인다.
- broad·aggregate Memory·precise 검색을 합치거나 추가하지 않는다. 각 상태/결과 소유권, canonical hydration, 검색 필터·횟수·부분 실패를 유지한다.
- aggregate hydration은 이미 자신의 `memory_search_result`를 읽는다(2501). 과거 교차 상태 결함의 재수정 항목으로 등록하지 않는다.
- **검증:** `prepare_turn_priority_memory_test.go:1217`의 등록 HTTP→precise score→final plan 및 private hit JSON 비노출을 사용한다.
- aggregate/broad의 서로 다른 성공·실패, wrong-session·stale/current-turn source, private scope, OFF와 무결과를 기존 생산 owner로 비교한다.
- 공개 trace의 필드·스크러빙·카운트는 기존 소비자와 대조한다. 진단 오류를 출력 거부 조건으로 바꾸지 않는다.

### RF06 — 측정 후 보충 조립·역할별 입력 반복 처리 축소

- **현재:** `group_turn_prepare.go:1062~1066`은 보충검색에서 전체 assembly를 만든 뒤 pristine facts/summaries만 소비한다.
- **대상:** `prepare_turn_assembly.go:1046~1222`의 최종 plan/표시·진단 단계와 `prepare_turn_multi_agent.go:643~720/920~941`의 전체 pool 정렬·길이·recent 대화 반복 계산.
- **방법:** 기존 타이밍/benchmark로 비중을 확인한 뒤 후보 생성에 필요한 처리와 미사용 최종 표시 작업의 경계를 같은 owner 안에서 정리한다.
- request 안에서 불변인 계산만 재사용한다. 후보를 임의로 줄이거나 영구 cache·새 검색·선택 경로를 만들지 않는다.
- test.12의 후보 재해결 제거·검색 병렬화는 다시 구현하지 않는다. request-local mutex 제거를 성능 목표로 삼지 않는다.
- **검증:** `prepare_turn_supplement_search_route_test.go:84`, `prepare_turn_multi_agent_search_test.go:17`, `prepare_turn_multi_agent_input_test.go:16/52/132/178`.
- 모든 검색 합류 후 round two, 역할 순서의 첫 중복 소유권, F/S/L alias, 전체 항목 shared cap, AI 원문/순서·빈 추천·부분 실패를 비교한다.
- `prepare_turn_candidate_pool_test.go`의 benchmark는 측정 도구로 사용한다. 실제 지연 개선량은 측정 전 수치나 완료 기준으로 주장하지 않는다.

### RF07 — HUD 스트림의 공통 I/O만 정리

- **대상:** JS current/previous 상태(14521~14536), cancel(15835/15851), NDJSON line 소비(15909/15955), reader loop(15933/15982), start(16099/16187).
- **방법:** 이미 같은 읽기·줄 분할·decode·reader 종료 처리만 공통화한다. current와 previous의 request ID·watch token·abort controller·카드 생명주기는 분리해 보존한다.
- `applyTurnWorkflowHUDStack()`(15509), 기존 timer/event stream을 사용한다. 새 watcher·수신 수락 규칙·저장 판단을 추가하지 않는다.
- **검증:** `main_part11_test.go:795`, `main_priority_memory_42_test.go:319`, `main_hud_timing_43_test.go:11/143` 및 `turn_workflow_hud_test.go`.
- 잘린 NDJSON·다중 chunk·취소·오래된 request·current/previous 동시 표시·OFF·timer 종료를 실제 함수로 확인한다.
- `main_part12_test.go:1634/1990/2175`의 hook/persistence 경계도 유지한다. HUD 편의를 위해 request identity를 합치지 않는다.

### RF08 — SQL 회귀 기대값 독립성과 조건부 파일 위치 정리

- **테스트 대상:** [mariadb_logical_turn_replace_test.go:203](../go-service/internal/store/mariadb_logical_turn_replace_test.go#L203)의 `TestMariaDBRollbackCanonicalTailIsAtomicAndIdempotent`.
- 221은 생산 `canonicalTailDeleteCommands()`의 개수로 기대 호출을 만들고, 222는 임의 SQL 정규식으로 받는다. 빠진 대상·잘못된 SQL/인자를 놓칠 수 있는 검증 공백이다.
- **방법:** 필요한 삭제 대상·SQL·인자·순서를 독립적으로 명시한다. 생산 목록에서 명령이 빠지거나 대상/범위가 바뀌면 테스트가 실패해야 한다.
- [mariadb_rollback_test.go:163](../go-service/internal/store/mariadb_rollback_test.go#L163)의 `TestMariaDBDeleteSession`은 이미 SQL을 명시한다. 두 테스트의 보강 이유를 구분한다.
- **조건부 runtime 위치 대상:** [mariadb_status.go:1186](../go-service/internal/store/mariadb_status.go#L1186)의 `DeleteSession()`을 기존 [mariadb_delete.go](../go-service/internal/store/mariadb_delete.go)로 옮기는 것은 삭제 owner 정리의 필요가 확인될 때만 별도 slice로 수행한다.
- signature·SQL 순서·transaction·revision tombstone·vector cleanup handoff를 그대로 유지한다. 새 삭제 helper/framework나 삭제 범위 확대를 만들지 않는다.
- **검증:** `mariadb_logical_turn_replace_test.go`의 원자성/재실행(203), 삭제 실패 rollback(241), lifecycle history(271), hierarchy 범위(312)를 생산 owner로 확인한다.
- 조건부 `DeleteSession()` 이동은 `mariadb_rollback_test.go`의 정상/실패 rollback(163/269/293)과 실제 Store/route의 relational rollback·vector outbox 연결을 별도로 확인한다.
- 이 계획은 실제 데이터 삭제·초기화 실행 허가가 아니다. 실제 저장소 검증은 격리된 fixture와 해당 실행 승인의 범위를 구분한다.

## 7. 4.4-D — 기존 cross-surface 의미 중복 통합

- **동작 계약:** Memory·KG·상태·관계·thread·근거의 같은 claim/event를 delivery family로 묶고 대표 문장과 support/source refs를 구분한다.
- 같은 사실의 surface 점수를 단순 합산하지 않는다. 대표 점수와 통합 전·후 점수/source coverage의 연결을 기록한다.
- 대표 표현은 source authority·최신 revision·구체성·부정·방향을 보존한다. 다른 약속·회차·시점·관점·충돌은 유지한다.
- `merged`, `retained_conflict`, `not_equivalent`를 전달 lineage로 설명한다. prompt에서 선택되지 않았다는 사실을 canonical 삭제로 해석하지 않는다.
- **예정 owner:** `prepare_turn_priority_memory.go`의 source identity/사실 후보·점수, `prepare_turn_memory.go`의 출처 occurrence, `prepare_turn_memory_budget.go`의 최종 예산, `output_fidelity_lineage.go`의 관측 연결.
- `prepare_turn_assembly.go`·`prepare_turn_render.go`는 그 결과를 기존 최종 plan/payload로 전달한다. Critic canonical writer나 Chroma를 의미 통합 저장소로 바꾸지 않는다.
- **4.3 접점은 미구현 계약 과제:** 통합을 이미 수신한 AI 추천 뒤에 적용해 문장·순서를 조용히 대체하지 않는다.
- 후보 family를 AI 선택 전에 제시하는 방향을 채택한다면 canonical member/source ref와 alias, 추천 순서, 원문, no-recommendation Go 선택의 의미부터 합의하고 계약으로 명시한다.
- 이 후보 입력 변경은 RF04/RF06의 동등성 리팩터링에 숨겨 넣지 않는다. 합의 전에는 구현된 기능이나 확정된 DTO로 기재하지 않는다.
- **검증:** 기존 priority/기억 예산/lineage 생산 함수 테스트에 같은 사건의 다중 surface와 별개 사건의 유사 문장을 대조하는 사례를 추가한다.
- 부정·역방향 관계·다른 source occurrence·시점·owner/viewer·충돌·AI가 명시적으로 고른 서로 다른 기억을 함께 검사한다.
- 같은 입력·예산에서 문자/token·사실 수·source coverage·필요 사실 recall을 통합 전후 비교한다. 중복 감소만으로 성공 판정하지 않는다.
- source-linked bundle 압축은 4.5 범위다. 이후 typed relation·local-graph 작업의 배정은 통합 로드맵 4.8–4.9를 따른다. 리팩터링에서 관계 점수 전파를 새로 구현하지 않는다.

## 8. 4.4-E — 통합 검증·보고

1. 각 RF를 작은 diff로 확인하고 B/C의 기존 출력 동등성과 D의 의도된 변경을 별도 비교표로 남긴다.
2. 실제 production 함수를 호출하는 Go/JS 회귀를 먼저 확인하고 등록 API의 provider/Store 경계 fixture로 호출·payload를 연결한다.
3. JS 변경이 있으면 활성 source에서 `node --check "Archive Center.js"`를 수행한다. 이는 문법 증거이며 Host 실행 증거가 아니다.
4. source/regression, package, loaded RisuAI, MariaDB/Chroma, 실제 provider, payload-applied, displayed-final을 각각 기록한다.
5. 해당 실환경 검증이 남으면 `implemented_unverified`로 보고한다. 테스트 성공이나 문서 완료만으로 4.3/4.4 전체 완료를 선언하지 않는다.
6. 파일·함수별 변경 이유, JS 추가/삭제 줄 수, 검사 실행/미실행, 성능 측정의 입력·범위·한계와 남은 항목을 기록한다.
7. architecture·ownership·contract·hook order·저장·검색·fallback 의미가 바뀌는 구현 slice는 `STRUCTURE.md`와 `AI_GUARDRAILS.md`를 함께 갱신한다.

패키지 확인에는 기존 [Windows 빌더](../ops/build-full-package.ps1)·[POSIX 빌더](../ops/build-posix-managed-packages.ps1),
[Windows 설치](../install-windows.ps1)·[POSIX 설치](../install.sh)와 OS별 실행기를 사용한다.
Windows의 `01` 시작 파일과 Linux·macOS·Termux의 기존 한 줄 설치/실행 경로,
업데이트 뒤 DB·키·프롬프트·역할 설정 보존을 해당 환경별로 기록한다. 교차 빌드·Windows 시험만으로
모든 OS 동작을 완료 처리하지 않는다. 설치기 수정은 리팩터링 자체의 필수 산출물이 아니며,
확인된 패키지 영향이 있을 때 기존 소유자를 수정한다.

대형 hook·`orchestrate`·저장/reroll 정리는 위 결과로 실제 필요한 범위를 입증한 뒤 후속으로 결정한다.
legacy 전체 삭제, 기존 acceptance 강화, 새로운 자동 복구 경로는 이 계획의 실행 방법에 포함하지 않는다.

## 9. 이번 문서 작업의 증거 수준

- 2026-09-07 활성 소스의 위치·호출 관계·기존 테스트 내용을 읽어 계획에 반영했다.
- 구조적 중복과 결합은 소스 근거이며, `withUiBridgeSettings`·생산 대체 `vector/fake.go`의 조회 기록 동시성은 재현 전 `SUPPORTED_RISK`이다.
- 이 문서 작성에서는 runtime·테스트·설정·프로세스를 변경하거나 테스트·빌드·패키지·live 호출을 실행하지 않았다.
- 4.4 구현은 전부 `PLANNED`이다. 현재 4.3 사용자 경험과 잔여 live 검증은 계속 별도로 관리한다.
