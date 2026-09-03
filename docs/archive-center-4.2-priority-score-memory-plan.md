# Archive Center 4.2 Priority Memory and User-Selectable Finalization Plan

상태: `VERSION_ASSIGNED_PLAN`, `NOT_IMPLEMENTED`

정본 버전 배정:
[`4.1-9.0-integrated-roadmap.md`](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)

기준일: 2026-09-03

## 1. 사용자 체감 목표

4.2의 목표는 기억을 많이 넣거나 정확성 조건을 더 쌓는 것이 아니다. 이미 저장된 기억 후보의
관련성·중요도·최신성을 최종 선택까지 보존하고, 점수가 높은 핵심 사실이 낮은 점수의 반복적인
관계·프로필·상태 자료보다 먼저 실제 본문 모델에 도달하게 한다.

같은 버전의 후반 작업에서는 사용자가 `저장 확정 시점`을 선택할 수 있게 한다. 기존
`응답 직후`가 기본값이며, 명시적으로 선택한 `다음 사용자 입력 시` mode는 다음 요청의
`beforeRequest`에서 직전 RisuAI 사용자·assistant pair의 현재 최종본을 기존 `/complete-turn`으로
한 번 확정한다. 직전 Critic과 현재 Publisher·본문 출력은 같은 시간대에 진행하되 현재 본문은
직전 Critic 완료를 기다리지 않는다.

```text
기존: 검색 중간 점수 → 문자열 평탄화 → lane 순서로 예산 소진
4.2: 사실 후보와 점수 유지 → canonical identity → 전역 점수 순위 → 핵심 기억 K → payload
```

정확성·privacy·branch·revision은 기존 후보 사용 범위를 보호한다. 그 범위 안에서 어떤 기억을
먼저 전달할지는 하나의 설명 가능한 `final_score`가 결정한다. 점수는 정상 본문 출력이나 저장을
거부하는 조건으로 사용하지 않는다.

## 2. 현재 확인된 문제

### 2.1 점수가 최종 예산 선택까지 이어지지 않음

- Critic은 `importance_score`, `emotional_intensity`, `narrative_significance`를 저장한다.
- 현재 memory recall은 exact phrase·lexical/vector relevance·importance·recency를 일부
  정렬에 사용한다.
- query가 있으면 importance는 relevance 뒤의 비교 기준이며, memory 이외 typed surface 전체에
  적용되는 공통 최종 점수는 없다.
- 최종 `memory_delivery_plan.v1` 예산 소유자는 점수 객체가 아니라 lane별 문자열 배열을 받아
  고정된 class 순서로 추가한다.
- `core_objective_memory_max_items`는 objective event summary에만 적용되고 관계·인물·세계·
  thread 후보 전체의 최종 K가 아니다.

현재 근거:

- [`selectPrepareTurnMemoryLanesWithVectorHydrationSource()`](../go-service/internal/httpapi/prepare_turn_recall.go)
- [`prepareTurnMemoryLineMeta()`](../go-service/internal/httpapi/prepare_turn_memory.go)
- [`buildPrepareTurnMemoryDeliveryPlan()`](../go-service/internal/httpapi/prepare_turn_memory_budget.go)

### 2.2 큰 typed surface가 사실 단위 경쟁을 우회함

한 인물의 상태 JSON이나 관계·private-memory section이 하나의 큰 문자열 후보가 되면, 그 안의
현재 사실과 낡은 계획·중복 필드가 각각 점수 경쟁을 하지 않는다. 높은 점수의 작은 완료 사실이
앞 lane의 큰 문자열 뒤로 밀릴 수 있다.

### 2.3 저장·후보·점수·전달·본문 효과를 분리해야 함

“이미 완료한 일을 다시 하려는 출력”은 아래 어느 단계에서도 발생할 수 있다.

```text
displayed-final
  → Critic 추출
  → MariaDB 저장
  → prepare-turn 후보
  → priority score
  → selected
  → rendered
  → payload-applied
  → displayed effect
```

완료 사실이 저장되지 않은 경우는 ranking으로 해결하지 않는다. 저장됐지만 점수나 선택에서
밀린 경우에만 4.2 priority 경로가 직접 해결한다.

### 2.4 응답 직후 확정만으로는 편집·리롤 최종본과 체감 대기를 함께 해결할 수 없음

현재 4.1은 공식 `afterRequest` 결과를 받아 기존 `/complete-turn` 저장을 비동기로 시작한다.
따라서 응답 뒤 사용자가 본문을 고치거나 리롤하기 전에 Critic 처리가 이미 시작될 수 있다.
기존 동작은 그대로 필요한 사용자도 있으므로 이를 제거하지 않고, 저장 시점을 다음 사용자
입력으로 늦추는 mode를 같은 설정의 선택지로 추가한다.

## 3. 1.0에서 복원할 것과 복원하지 않을 것

역사적 1.0 검색은 다음 기본 합산 점수로 결과를 정렬했다.

```text
final_score = similarity × 0.60 + importance × 0.25 + recency × 0.15
```

4.2는 이 세 신호가 실제 최종 순위를 결정했다는 원칙을 복원한다. 수치는 첫 shadow/A-B의 시작
기준이며, fixture 결과 없이 특정 가중치를 runtime 정답으로 고정하지 않는다.

함께 복원할 강점:

- 높은 `final_score`가 실제 선택 순서를 결정함;
- 선택된 기억을 기존 `book_author`·`director` 역할의 행동·대사·서브텍스트로 연결함;
- 현재 장면에 필요한 기억을 짧고 읽을 수 있는 형태로 제공함.

복원하지 않을 것:

- Python 1.0 runtime 또는 별도 검색기;
- 병렬 Supervisor·Reviewer·두 번째 Publisher;
- 추가 provider 호출, 자동 retry 또는 숨은 fallback;
- JavaScript ranking·selection;
- 특정 이야기 문구·인물명·점수·순위를 hard-code한 분류.

## 4. 4.2 목표 계약

계약명과 version은 구현 전에 현재 DTO·호출자를 확인해 최종 확정한다. 아래 shape은 책임과
필수 관찰값을 고정하는 계획 표현이다.

```text
priority_memory_item
  canonical_fact_id
  source_refs
  lane
  complete_text
  relevance_score
  importance_score
  recency_score
  continuity_bonus
  final_score
  final_rank
  chars
  selection_status
  selection_reason
```

```text
turn_finalization_mode
  immediate_after_response  # 기본값, 현재 4.1 동작
  next_user_input           # 명시적 선택, 직전 Host pair의 현재 최종본 확정
```

### 4-A 점수 생산

- `relevance_score`: 기존 exact·lexical·vector 결과를 하나의 비교 가능한 범위로 정규화;
- `importance_score`: 이미 저장된 1~10 importance를 정규화하고 동점 표시에 그치지 않게 함;
- `recency_score`: 최신성 신호를 유지하되 오래됐다는 이유만으로 지속 상태·약속을 삭제하지 않음;
- `continuity_bonus`: 현재 상태, 완료 행동, 유효한 목표·약속, 직접 언급과 반복 오류 방지 가치를
  작은 명시적 가산점으로 표현;
- `final_score`: 각 구성값과 계산 version을 trace에 남기는 결정적 합산값.

4.2는 매 turn 별도 LLM을 호출해 importance를 다시 쓰지 않는다. 기존 importance의 생산·저장과
그 점수를 실제로 소비하는 문제를 먼저 분리해 검증한다.

### 4-B 점수 보존

후보는 최종 Go budget owner에 도달하기 전에 문자열만 남는 형태로 평탄화하지 않는다. 렌더링
문장은 후보의 score·rank·canonical/source identity와 함께 이동해야 한다. JavaScript는 완성된
Go plan만 적용한다.

### 4-C 전역 순위와 핵심 기억 K

- 현재 사용자 입력은 기억 후보가 아니며 기존 최고 권한을 유지한다.
- 명시적 사용자 정정·직접 근거·비밀 누출 방지 경로도 기존 권한과 동작을 유지한다.
- 그 밖의 event, current state, relationship, private memory, world, KG, storyline, pending thread
  기억 후보는 lane별 선착순이 아니라 공통 `final_score` 내림차순으로 경쟁한다.
- 핵심 기억 K는 objective event만이 아니라 canonical fact 단위의 전역 scored-memory K다.
- K와 최종 문자 예산 중 먼저 닿는 상한까지만 선택한다.
- K 선택 뒤 남은 문자 공간을 저득점 기억으로 다시 채우지 않는다.
- lane별 고정 개수·고정 quota를 만들지 않는다.

### 4-D 사실 단위 후보

인물 전체·관계 section 전체·거대한 JSON 전체에 점수 하나를 주지 않는다. 기존 typed source에서
현재 위치, 소유, 진행 상태, 완료 행동, 유효한 목표, 관계 변화처럼 독립적으로 이해 가능한 사실을
완전한 후보로 만든다.

후보를 임의 길이로 잘라 점수를 맞추지 않는다. 사실 단위로 만든 각 후보는 온전하게 선택하거나
defer하고, 원본 JSON과 source occurrence는 MariaDB·lineage에 보존한다.

### 4-E canonical identity와 요청 단위 현재 상태

- 별칭이 검토된 같은 인물은 `entity_id` 기준으로 같은 순위 집합에 둔다;
- 계획·시작·완료가 같은 사건 family라면 최신 완료/current 표현을 이번 요청의 primary로 둔다;
- 과거 계획은 삭제하지 않고 historical/support로 남긴다;
- 부정·방향·시점·관점·별도 occurrence는 같은 텍스트라는 이유로 합치지 않는다;
- identity가 불명확하면 서로 다른 후보로 보존하고 진단한다. 정상 출력·저장 거부로 연결하지
  않는다.

4.2의 resolution은 읽기·전달 projection이다. 계획·프로젝트·물건·약속의 durable
`planned → active → completed/cancelled/superseded` 쓰기는 4.5가 소유한다.

### 4-F 기존 Publisher 소비

기존 `publisher_plan.v2`만 사용한다. Publisher는 실제 selected/delivered source ref를 사용해
continuity anchor, must-account, 행동·대사·서브텍스트와 필요한 no-repeat guidance를 만든다.

```text
완료 사실: 작업은 이미 끝났다.
표현: 준비를 반복하지 말고 그 결과가 존재하는 현재 상태에서 이어간다.
```

Publisher는 truth writer가 아니며 전달되지 않은 기억, private knowledge 또는 과거 계획으로
사용자 입력을 덮지 않는다.

### 4-G 사용자 선택형 저장 확정 시점과 직전 Critic 파이프라이닝

- 설정은 `저장 확정 시점` 하나로 제공하고 `응답 직후`를 기본값으로 유지한다;
- `응답 직후` mode는 현재 4.1의 `beforeRequest`·`afterRequest`·`/complete-turn`·Critic·리롤·
  분기·재시도 동작을 그대로 사용한다;
- `다음 사용자 입력 시` mode의 `afterRequest`는 표시 후보와 request lineage만 관찰하고
  canonical 저장을 시작하지 않는다;
- 다음 사용자 행의 `beforeRequest`는 바로 앞의 완성된 Host 사용자·assistant pair를 읽어,
  사용자가 편집하거나 리롤한 뒤 실제로 남긴 최종본을 기존 `/complete-turn`에 전달한다;
- 같은 Host user row의 리롤과 assistant 삭제 후 편집·재생성은 마지막 assistant 하나만
  확정하고, 내용이 같더라도 새 Host user row는 새 턴으로 처리한다;
- 직전 `/complete-turn`과 현재 `/prepare-turn`·Publisher를 같은 `beforeRequest` 시간대에
  시작하되 현재 본문 요청은 직전 Critic을 기다리지 않는다;
- 현재 장기기억 선택은 직전 pending pair보다 앞의 마지막 확정 horizon을 사용하고, RisuAI의
  최근 대화는 직전 pair를 정상 context로 계속 제공한다;
- 마지막 응답 뒤 다음 입력이 없으면 RisuAI에는 남지만 Archive Center에는 `확정 대기`로
  표시한다. 종료나 세션 전환만으로 자동 저장하지 않는다;
- JavaScript는 Host pair 관찰·기존 API 호출·HUD 표시만 담당한다. mode, logical-turn identity,
  branch/session 범위, 확정 horizon, idempotency, Critic와 canonical persistence는 Go가 소유한다;
- 기존 `/prepare-turn`, `/complete-turn`, Critic, raw-pair 저장, 파생 저장과 vector outbox를
  재사용하고 두 번째 저장 경로·Critic·scheduler·숨은 retry를 만들지 않는다.

## 5. 버전 인계 경계

| 버전 | 소유 범위 |
|---|---|
| 4.2 | static priority score 복원, score lineage, canonical fact K, 요청 단위 current resolution, 제한적 자연어 사실 투영, 기존 Publisher 소비, 사용자 선택형 저장 확정 시점과 직전 Critic 파이프라이닝 |
| 4.3 | Memory·KG·상태·관계·thread 전체의 cross-surface 의미 통합과 대표 표현 선정 |
| 4.4 | 여러 고득점 사실의 source-linked atomic bundle과 scoped raw excerpt |
| 4.5 | item·project·plan·promise의 durable lifecycle과 status signal |
| 4.6 | 모델/context별 K·가중치·bundle token 효용의 adaptive 조정; static 4.2 score의 실측 기반 보정 |
| 5.1~5.7 | Actor별 접근·잠복·부분/완전 회상과 습관·감정·행동 표현 |
| 7.5 | 기존 Go Publisher 안의 capability-adaptive 고급 guidance·review depth |

4.2의 저장 확정 시점 mode는 직전 Host pair의 canonical 완료 시점을 선택하는 요청 수명주기다.
4.3의 전체 semantic consolidation, 4.4의 범용 bundle, 4.5의 물건·계획·약속 durable lifecycle
또는 4.6의 adaptive tuning을 미리 구현하지 않는다.

## 6. 구현 순서

1. 개인정보를 제거한 동일 의미 fixture로 현재 4.1 priority와 즉시 저장 baseline을 고정한다.
2. 저장→후보→현재 점수→최종 budget에서 점수가 사라지는 위치를 production trace로 고정한다.
3. 기존 Go owner 안에서 사실 단위 score-bearing candidate를 만든다.
4. canonical identity와 요청 단위 current/historical 표현을 연결한다.
5. 전체 scored-memory 후보를 `final_score`로 정렬하고 핵심 기억 K와 문자 상한을 적용한다.
6. 선택된 사실만 짧은 자연어로 렌더하고 score·rank·source lineage를 유지한다.
7. 기존 Publisher가 delivered ref만 소비하도록 확인한다.
8. Text와 Google/Vertex·Gateway·Provider Manager PDF가 동일한 plan/hash를 소비하는지 확인한다.
9. `저장 확정 시점` 설정과 Go-owned mode를 추가하고 기본 `응답 직후` parity를 고정한다.
10. `다음 사용자 입력 시`의 직전 pair 확정, 확정 horizon과 Critic 비대기 흐름을 기존
    `/complete-turn`·`/prepare-turn` owner에 연결한다.
11. 두 mode의 리롤·편집·동일 문장 새 행·재시도·분기·재시작·번역기 회귀와 실제 provider
    동시 실행 시간·usage를 확인한다.
12. 실제 RisuAI 4.1/4.2 A-B 뒤 Windows 4.2 테스트 package를 만든다.

## 7. 필수 회귀와 체감 완료 기준

- 관련성이 비슷할 때 importance 9 기억이 importance 4 기억보다 먼저 선택됨;
- 중요도만 높은 무관 기억이 현재 장면의 관련 기억 전체를 밀어내지 않음;
- 오래됐지만 현재도 유효한 고중요 상태·약속이 단순 recency 때문에 사라지지 않음;
- 최신이지만 낮은 중요도의 반복 관계 설명이 핵심 완료 사실보다 먼저 예산을 소진하지 않음;
- 같은 canonical fact의 여러 surface 복제본이 K를 여러 번 소비하지 않음;
- 같은 문장이라도 별도 occurrence·방향·관점이면 임의 병합되지 않음;
- 큰 character-state JSON이 통째로 고득점 후보가 되지 않고 필요한 완전한 사실만 경쟁함;
- 큰 문자 상한을 설정해도 K 선택 후 저득점 backfill로 상한을 억지로 채우지 않음;
- `final_score`, 구성 점수, rank, selected/deferred 이유가 Edit Check/trace에서 설명됨;
- Text와 네 PDF 표현이 동일한 selected fact·순서·hash를 사용함;
- 리롤·provider 재시도·분기·Say Nothing·Yumi Translator·저장·Critic·벡터 색인 회귀가 없음;
- 실제 “완료한 일을 다시 하려는” fixture에서 필요한 기억이 payload에 들어가고 displayed-final이
  그 완료 상태에서 이어짐.
- 기본 `응답 직후` mode의 저장·리롤·편집·재시도 동작이 4.1과 동일함;
- `다음 사용자 입력 시` mode에서 직전 pair의 편집·리롤 최종본만 정확히 한 번 저장되고 버린
  assistant 후보는 남지 않음;
- 새 사용자 입력의 `/prepare-turn`과 직전 `/complete-turn`이 같은 요청 시간대에 시작되더라도
  현재 장기기억은 pending pair 앞의 확정 horizon을 사용함;
- 직전 Critic 지연·실패가 현재 본문 출력을 기다리게 하거나 다른 저장 경로를 만들지 않음;
- 마지막 응답 뒤 다음 입력이 없는 상태와 재시작 뒤 확정 대기가 HUD·진단에서 구분됨.

## 8. 금지선

- 정확성·권위 조건을 여러 개 추가해 정상 기억 주입·본문 출력·저장을 거부하지 않는다;
- 중요도 하나만으로 무관 기억을 모든 장면에 강제하지 않는다;
- 고정 lane quota, lane별 K, 낮은 점수 filler 또는 문자 상한 채우기를 만들지 않는다;
- 점수 계산을 JavaScript, 외부 번역기 또는 Provider Manager에 두지 않는다;
- 새 검색기·새 canonical store·새 graph DB·새 Publisher 경로를 만들지 않는다;
- 지연 mode를 위해 두 번째 `/complete-turn`, 별도 Critic, broad chat sweep, background scheduler,
  종료·세션 전환 자동 저장 또는 JavaScript 저장 정책을 만들지 않는다;
- source row, raw evidence 또는 충돌 사실을 점수 때문에 삭제하거나 자동 병합하지 않는다;
- fixture의 특정 문장·인물명·점수·예상 순위를 runtime에 hard-code하지 않는다;
- 문서·source test·package 결과만으로 loaded RisuAI나 displayed-final 효과를 완료로 주장하지
  않는다.

## 9. 완료 증거 분리

다음 상태를 별도로 기록한다.

1. `SOURCE_IMPLEMENTED`
2. `REGRESSION_VERIFIED`
3. `PACKAGE_BUILT`
4. `LOADED_RISU_VERIFIED`
5. `REAL_MARIADB_CHROMA_VERIFIED`
6. `PROVIDER_PAYLOAD_VERIFIED`
7. `DISPLAYED_FINAL_EFFECT_VERIFIED`

4.2는 마지막 항목까지 확인하기 전에는 기억 품질 개선을 실환경 완료로 표시하지 않는다.
