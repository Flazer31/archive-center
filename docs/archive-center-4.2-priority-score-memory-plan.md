# Archive Center 4.2 Priority Memory and User-Selectable Finalization Plan

상태: `SOURCE_IMPLEMENTED`, `REGRESSION_VERIFIED`, `PACKAGE_BUILT`; 최근 대화 query count와
`다음 사용자 입력 시`의 한 턴 지연 저장은 실제 RisuAI에서 확인됐으며, 기억 품질·실 provider
payload·Critic 복구 X·최신 UI 모양은 각각 별도 live gate로 남음

정본 버전 배정:
[`4.1-9.0-integrated-roadmap.md`](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)

기준일: 2026-09-05

후속 기준(2026-09-08): 아래는 4.2 구현·패키지의 당시 기록이다. 활성 4.3 소스는
[`priority_score.static.v4`](archive-center-4.3-feedback-work-log.md#retrieval-score-retention-repair)로
원본 의미 검색 점수를 완전한 턴 요약 선정에 반영하고 중요도와 최근성의 기여를 분리했다.
이 후속 source/회귀 결과를 기존 4.2 배포본이나 test.17 패키지의 작동으로 해석하지 않는다.

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
4.2: 사실 후보와 점수 유지 → canonical identity → 완성 턴 요약 점수 → 자료 분류별 순위/K → 자료별·전체 문자 예산 → payload
```

정확성·privacy·branch·revision은 기존 후보 사용 범위를 보호한다. 그 범위 안에서 어떤 기억을
먼저 전달할지는 하나의 설명 가능한 `final_score`가 결정한다. 점수는 정상 본문 출력이나 저장을
거부하는 조건으로 사용하지 않는다.

1.0에서 복원할 것은 관련도·중요도·최신성이 실제 선택 순위에서 경쟁하는 원리다. 현실 시간 감쇠는
복원하지 않는다. 최신성은 오직 현재 RP 턴과 source turn의 거리로 계산하므로 사용자가 며칠 쉬어도
같은 이야기 위치의 기억 점수는 변하지 않는다. 현재 4.2의 사실 단위 문장·출처·visibility·관점
상세는 그대로 유지한다.

### 1.1 최초 계획 이후 확정된 범위 변경

최초 계획 기준점 `48a6371`은 하나의 전역 핵심 기억 K와 현재 입력 중심 검색 문맥을 전제로 했다.
첫 구현 뒤 실제 RisuAI·DB·payload를 대조하면서 다음 변경을 4.2 범위로 승인하고 현재 계약에
반영했다.

- `최근 대화 참고 수`를 ChromaDB 검색 수와 분리하고, 최근 user+assistant 완결 대화를 현재 입력과
  나란히 검색 query로 사용한다;
- 기존 `precise_memory_units`의 의미 유사도를 일치하는 사실 하나에 직접 연결하고 parent row
  점수를 자식에게 복사하지 않는다;
- 최신성은 RP 턴 거리로만 계산하고 lifecycle 판정은 점수·canonical winner·후보 탈락을 소유하지
  않는다;
- 하나의 전역 K를 완성 턴 요약 그룹과 각 scored fact 자료 분류가 독립적으로 사용하는 K로
  변경하고 기존 자료별 주입 예산을 함께 적용한다;
- 기존 완료 사건의 명시적 resolve와 재색인 멱등성만 제한적으로 수선한다. 4.5의 범용 물건·계획·
  약속 lifecycle은 가져오지 않는다;
- raw-only Critic 재개, 지연 저장 정책 전달, 현재/직전 6+6 HUD, 완료 카드 제거, Critic 복구 X와
  설정 UI·compact 진단을 실측 결함 보정으로 포함한다;
- 구조화 배열 순번의 canonical identity 충돌을 `source_occurrence` 보존으로 수정한다.

상세 원인, 변경 경계, 회귀와 패키지 증거는
[`archive-center-4.2-work-log.md`](archive-center-4.2-work-log.md)의 `0`, `2.1~2.3`, `9~9.11`을
정본으로 사용한다. PDF, Yumi Translator, timeout 재시도, 리롤·분기·Say Nothing의 기존 계약은
4.1에서 이어받은 비퇴행 대상이며 4.2 신규 기능으로 세지 않는다.

## 2. 최초 계획 당시 확인된 문제

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

역사적 1.0의 일반 기억 검색은 다음 기본 합산 점수로 결과를 정렬했다. 여기서 일반 기억의
`recency`는 현실 시간 감쇠였고, 일부 계층 기억에는 별도의 턴 기반 최신성이 있었다.

```text
final_score = similarity × 0.60
            + importance × 0.25
            + recency × 0.15
```

4.2는 이 세 신호가 실제 최종 순위를 결정했다는 원칙을 복원하되, 소설/RP에 맞게 현실 시간을
사용하지 않는다. 대신 저장 중요도에도 RP 턴 거리 감쇄를 적용한다.

```text
final_score = relevance × 0.60
            + (importance × turn_distance_recency) × 0.25
            + turn_distance_recency × 0.15
            + continuity_bonus
            + structured_bias
```

관련성이 비슷하면 최근 기억이 직접 최신성 점수와 감쇄되지 않은 중요도를 함께 받아 먼저 온다.
오래된 기억은 중요도 영향이 줄지만, 사용자가 그 사건을 직접 요청하면 높은 relevance로 다시
올라올 수 있다. 수치는 첫 shadow/A-B의 시작 기준이며 fixture 결과 없이 이야기별 조건을 추가하지
않는다.

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

계약명과 version은 현재 DTO·production 호출자에 맞춰 확정했다. 구현·검증 증거는
[`archive-center-4.2-work-log.md`](archive-center-4.2-work-log.md)에 기록한다.

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

- `relevance_score`: 현재 `continuity_query` 또는 `raw_user_input`과, UI
  `recent_conversation_reference_count` 수만큼의 최근 완결 대화를 하나의 Go-owned query set으로
  사용한다. 각 완결 대화는 사용자 입력과 최종 assistant 원문을 함께 가진 하나의 독립 query이며,
  설정값이 5이면 최근 완결 대화 5개를 각각 embedding/search하고 같은 문서의 최고 유사도를 사실
  점수에 연결한다. 이 수는 Chroma 결과 `top_k` 및 최종 핵심 기억 최대 수와 독립적이다.
  lexical scorer도 각 query의 사실 관련도를 따로 계산해 최고값을 사용한다. 원문은 검색 앵커일 뿐 최종 Payload의
  `Recent Raw Turn` 중복 블록으로 다시 전달하지 않는다. 특정 문구 목록으로
  `이어서 적어주세요`를 판별하지 않는다;
- `importance_score`: 이미 저장된 1~10 importance를 정규화하고 동점 표시에 그치지 않게 함;
- `recency_score`: 현재 RP 턴과 source turn의 거리로 계산하고 32턴 반감기와 `0.20` 최저점을
  유지한다. 현실 시각은 최신성 점수 owner가 아니며, 오래됐다는 이유만으로 지속 상태·약속을
  삭제하지 않음;
- `continuity_bonus`: 기존 lane/source 역할에 따른 작은 고정 보정이다. 사실 문구나 lifecycle
  완료 여부를 판정해 가산하지 않는다;
- `structured_bias`: 인물/화자 `0.04`, 장소 `0.05`, storyline `0.06`을 서로 독립적으로 계산하고
  합계 `0.12`까지만 rank를 보정한다. 이 값은 기억 admission·rejection 조건이 아니다;
- `final_score`: 각 구성값과 계산 version을 trace에 남기는 결정적 합산값.

4.2는 매 turn 별도 LLM을 호출해 importance를 다시 쓰지 않는다. 기존 importance의 생산·저장과
그 점수를 실제로 소비하는 문제를 먼저 분리해 검증한다.

### 4-B 점수 보존

후보는 최종 Go budget owner에 도달하기 전에 문자열만 남는 형태로 평탄화하지 않는다. 렌더링
문장은 후보의 score·rank·canonical/source identity와 함께 이동해야 한다. JavaScript는 완성된
Go plan만 적용한다.

### 4-C 턴 요약·자료 분류별 순위와 핵심 기억 K

- 현재 사용자 입력은 기억 후보가 아니며 기존 최고 권한을 유지한다.
- 명시적 사용자 정정·직접 근거·비밀 누출 방지 경로도 기존 권한과 동작을 유지한다.
- recall된 `memories.turn_summary`는 자식 사실의 최고 `final_score`를 턴 점수로 사용하고, 선택 시
  잘게 자르지 않은 완성 요약으로 전달한다.
- 그 밖의 event, current state, relationship, private memory, world, KG, storyline, pending thread
  사실은 현재 자료 분류 안에서 `final_score` 내림차순으로 경쟁한다.
- UI `핵심 연관 기억 최대 수`는 완성 턴 요약 그룹과 각 scored fact 분류에 같은 값으로 각각
  독립 적용한다. 한 그룹의 남는 K는 다른 그룹으로 넘기지 않는다.
- K와 해당 자료별 문자 예산 및 최종 전체 문자 예산 중 먼저 닿는 상한까지만 선택한다.
- K는 채워야 하는 문자량 목표가 아니라 각 그룹의 최대치다. K 뒤의 후보로 남은 문자 상한을
  억지로 채우지 않는다.
- 한 문장 턴 요약과 그 자식 사실이 완전히 같으면 한 번만 전달하고 fact K를 소비하지 않는다.
- relevance는 그룹 안의 순위를 정하는 점수이며, 0 또는 관찰 불가라는 이유만으로 기억을 거부하는
  별도 조건으로 사용하지 않는다. 후보 범위는 기존 검색·source projection owner가 정한다.
- 직접 근거와 비밀 보호는 기존대로 K를 소비하지 않는다.
- 새 K 설정은 만들지 않는다. 자동·사용자 지정 모두 같은 `memory_delivery_plan.v2`를 사용하며,
  사용자 지정은 기존 UI 자료별 주입 예산을 실제 class 문자 상한으로 사용한다. 턴 요약과
  event fact는 기존 `event_recent` 예산을 공유한다.

### 4-D 사실 단위 후보

인물 전체·관계 section 전체·거대한 JSON 전체에 점수 하나를 주지 않는다. 기존 typed source에서
현재 위치, 소유, 진행 상태, 완료 행동, 유효한 목표, 관계 변화처럼 독립적으로 이해 가능한 사실을
완전한 후보로 만든다.

후보를 임의 길이로 잘라 점수를 맞추지 않는다. 사실 단위로 만든 각 후보는 온전하게 선택하거나
defer하고, 원본 JSON과 source occurrence는 MariaDB·lineage에 보존한다.

구현 보정 계약:

- 이미 eligibility/visibility projection을 통과한 각 source가 최종 section 문자열로 합쳐지기 전에
  요청 단위 `PriorityFactSeed`를 만든다;
- seed에는 완전한 사실 문장, source row/occurrence, lane/tier, visibility, perspective owner와
  allowed viewers, stored importance를 함께 둔다;
- relevance는 검색에 사용한 바로 그 effective continuity query를 각 사실 문장에 직접 대조해
  계산한다. 인물·장소·storyline을 긴 primary query에 덧붙이지 않고 별도 작은 bias로 계산한다;
- aggregate Memory 사실은 기존 `tier=memory` 검색이 선택한 parent row에서만 만든다. 이미 읽힌
  session Memory 전체를 priority 후보에 다시 넣는 우회 경로는 만들지 않는다;
- MariaDB의 source-active, public/general-memory이며 history turn 범위 안인 정본
  `precise_memory_unit` 수를 session별로 먼저 확인한다. 같은 effective continuity query에서 이미
  생성한 query vector를 재사용해 기존 Chroma 문서의 `source_table=precise_memory_units`만 그 실제
  후보 수만큼 조회한다. final K를 이 검색 상한으로 쓰지 않으며, 새 query·embedding·selector·저장
  owner를 만들지 않는다;
- 위 hit는 요청 내부에서만 전달하고 public response 전 제거한다. 정본 unit과 확인된 사실만 unit
  자체 vector similarity를 relevance로 사용한다;
- 한 precise hit는 source turn과 canonical fact identity가 맞는 한 사실에만 연결한다. 앞 단계 parent
  row의 retrieval/selection score는 진단 lineage로만 보존하며 자식 relevance를 올리거나 형제
  사실에 복사하지 않는다;
- stored importance처럼 실제 source가 소유한 값은 `importance_source`와 함께 유지하고,
  `importance_after_turn_decay = importance × turn_distance_recency`도 별도로 기록한다. 이것을 parent
  relevance와 혼동하지 않는다;
- fact 분류의 canonical dedupe와 K는 row/section/문자 수가 아니라 이렇게 만들어진 사실 identity와
  실제 사실 개수를 사용한다. 별도 턴 요약 그룹은 parent Memory row 하나를 한 항목으로 세고 자식
  사실의 최고 점수를 사용한다. lifecycle key는 canonical winner identity가 아니며 서로 다른 단계의
  사실을 하나로 접지 않는다;
- 아직 seed 생산으로 전환하지 못한 source는 `legacy_rendered_line`으로 표시한 완전한 문장 분해를
  호환 fallback으로 사용한다. seed나 score가 없다는 이유로 기억을 버리거나 본문을 막지 않는다;
- per-fact visibility/관점은 기존 source projection 결정의 lineage다. 4.2 scorer가 같은 값을 다시
  검사하는 두 번째 eligibility gate를 만들지 않는다.
- 이름·별칭·identity evidence는 `identity_metadata`로 분리한다. event/current-state K를 소비하지
  않으며, 검토된 같은 canonical entity의 선택 사실에 필요할 때 한 번 붙는다. metadata가 문자
  예산에 맞지 않아도 사실 선택은 막지 않는다;
- 보호 기억의 본문 뒤에 붙는 사용 지침은 독립 사실이 아니므로 본문과 한 단위로 유지한다.
  한국어 조사처럼 비ASCII 어절 끝 한 글자가 달라진 경우는 사실 관련도 계산에서 인식하되,
  이를 영문 prefix 일반 일치로 확대하지 않는다.

### 4-E canonical identity와 요청 단위 현재 상태

- 별칭이 검토된 같은 인물은 `entity_id` 기준으로 같은 순위 집합에 둔다;
- 계획·시작·완료·후속 사건은 같은 `lifecycle_key`를 공유해도 각각 독립된 점수 후보로 유지한다;
- 같은 source occurrence 아래 완전히 같은 사실만 dedupe하고, 내용이 달라진 사실은 해당 분류 점수에서
  관련도·중요도·RP 턴 최신성으로 경쟁한다;
- lifecycle은 AI가 잘못 판정할 수 있는 진단 lineage다. transition은 final score에 포함하지 않으며,
  terminal 여부가 후보를 선제 탈락시키거나 canonical winner를 고르지 않는다. 문구에 `완료`,
  `이미`, `계획` 등이 들어갔다는 이유로 lifecycle을 추측하지 않는다;
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
| 4.2 | static priority score 복원, score lineage, 완성 턴 요약·자료 분류별 fact K, 요청 단위 current resolution, 최근 완결 대화 검색 문맥, 제한적 자연어 사실 투영, 기존 Publisher 소비, 사용자 선택형 저장 확정 시점과 직전 Critic 파이프라이닝 |
| 4.3 | Memory·KG·상태·관계·thread 전체의 cross-surface 의미 통합과 대표 표현 선정 |
| 4.4 | 여러 고득점 사실의 source-linked atomic bundle, 선택 사건의 원인·결과·후속 사건 관계 점수 전파와 scoped raw excerpt |
| 4.5 | item·project·plan·promise의 durable lifecycle과 status signal |
| 4.6 | 모델/context별 K·가중치·bundle token 효용의 adaptive 조정; static 4.2 score의 실측 기반 보정 |
| 5.1~5.7 | Actor별 접근·잠복·부분/완전 회상과 습관·감정·행동 표현 |
| 7.5 | 기존 Go Publisher 안의 capability-adaptive 고급 guidance·review depth |

4.2의 저장 확정 시점 mode는 직전 Host pair의 canonical 완료 시점을 선택하는 요청 수명주기다.
4.3의 전체 semantic consolidation, 4.4의 범용 bundle, 4.5의 일반화된 물건·계획·약속
durable lifecycle ontology 또는 4.6의 adaptive tuning을 미리 구현하지 않는다. 다만 4.2 실측에서
확인된 기존 완료 사건의 저장 상태를 닫기 위해, Critic의 동일 `lifecycle_key` 전달, 명시적
`complete/resolve`, 실제 `pending_threads` 종료, vector-only 재색인만 호환성 수선으로 포함한다.
기억 전달에서는 lifecycle 단계들을 각각 보존하고 점수에 영향을 주지 않는 진단으로만 사용한다. 새
테이블·새 검색 경로·문구 기반 판정은 추가하지 않는다.

## 6. 구현 순서

1. 개인정보를 제거한 동일 의미 fixture로 현재 4.1 priority와 즉시 저장 baseline을 고정한다.
2. 저장→후보→현재 점수→최종 budget에서 점수가 사라지는 위치를 production trace로 고정한다.
3. 기존 Go owner 안에서 사실 단위 score-bearing candidate를 만든다.
4. canonical identity와 요청 단위 current/historical 표현을 연결한다.
5. recalled `memories.turn_summary`를 최고 자식 점수로 정렬하고 완성 요약 K를 적용한다.
6. 나머지 scored-memory fact를 자료 분류별 `final_score`로 정렬하고 같은 K를 각 분류에 독립 적용한다.
7. 기존 UI 자료별 주입 예산과 전체 문자 상한을 적용해 완성 항목만 렌더하고 score·rank·source lineage를 유지한다.
8. 기존 Publisher가 delivered ref만 소비하도록 확인한다.
9. Text와 Google/Vertex·Gateway·Provider Manager PDF가 동일한 plan/hash를 소비하는지 확인한다.
10. `저장 확정 시점` 설정과 Go-owned mode를 추가하고 기본 `응답 직후` parity를 고정한다.
11. `다음 사용자 입력 시`의 직전 pair 확정, 확정 horizon과 Critic 비대기 흐름을 기존
    `/complete-turn`·`/prepare-turn` owner에 연결한다.
12. 두 mode의 리롤·편집·동일 문장 새 행·재시도·분기·재시작·번역기 회귀와 실제 provider
    동시 실행 시간·usage를 확인한다.
13. 실제 RisuAI 4.1/4.2 A-B 뒤 Windows 4.2 테스트 package를 만든다.

## 7. 필수 회귀와 체감 완료 기준

- recalled Memory의 자식 사실 최고 점수가 턴 요약 점수가 되고, 선택된 턴은 완성된
  `turn_summary` 한 항목으로 전달됨;
- 완성 턴 요약 그룹과 각 fact 자료 분류가 UI의 같은 K를 독립적으로 지키고 남는 K를 서로
  넘기지 않음;
- 사용자 지정 mode도 `memory_delivery_plan.v2`를 유지하며 기존 UI 자료별 주입 예산을 실제
  class 문자 상한으로 사용함;
- 완성 턴 요약과 event fact가 `event_recent` 문자 예산을 공유하고, 어느 항목도 중간 절단되지 않음;
- 관련성이 비슷할 때 importance 9 기억이 importance 4 기억보다 먼저 선택됨;
- 중요도만 높은 무관 기억이 현재 장면의 관련 기억 전체를 밀어내지 않음;
- 오래됐지만 현재도 유효한 고중요 상태·약속이 단순 recency 때문에 사라지지 않음;
- 최신이지만 낮은 중요도의 반복 관계 설명이 핵심 완료 사실보다 먼저 예산을 소진하지 않음;
- 같은 canonical fact의 여러 surface 복제본이 K를 여러 번 소비하지 않음;
- 같은 문장이라도 별도 occurrence·방향·관점이면 임의 병합되지 않음;
- 큰 character-state JSON이 통째로 고득점 후보가 되지 않고 필요한 완전한 사실만 경쟁함;
- 큰 문자 상한을 설정해도 K 선택 후 저득점 backfill로 상한을 억지로 채우지 않음;
- aggregate Memory 검색에 선택되지 않은 session-wide row가 priority 후보로 우회 편입되지 않음;
- relevance 0 또는 semantic 관찰 불가가 전체 일반 기억을 0개로 만드는 hard rejection으로 쓰이지
  않으며, 실제 후보는 점수순으로 K 안에서 선택됨;
- `continuity_query`가 Chroma embedding과 최종 fact scorer에 글자까지 동일하게 전달되며, 짧은
  입력도 별도 문구 분류 없이 같은 경로를 사용함;
- `precise_memory_units`의 사실 vector 검색 수가 final K와 독립적이며, 기존 `source_table`
  metadata로 조회한 점수가 해당 사실 하나의 `relevance_source`로 이어지고 같은 parent Memory row의
  무관한 자식에게 복사되지 않음;
- 같은 인물·장소가 등장하는 현재 사건과 오래된 다른 사건이 경쟁할 때 현재 장면과 의미가 가까운
  사실이 이기며, 사용자가 오래된 사건을 명시적으로 물으면 의미 유사도가 recency를 이김;
- speaker/location/storyline bias가 각각 trace에 보이고, 어느 것도 후보 admission/rejection reason을
  만들지 않음;
- source turn 0, 최근 turn, 먼 과거 turn의 recency가 RP turn distance와 floor를 따름;
- 이름·별칭·identity evidence가 metadata로 추적되지만 event/current-state 사실 K를 소비하지 않음;
- Persona 보호 기억 본문과 사용 지침이 하나의 K 단위로 유지되며 한국어 조사 차이로 누락되지 않음;
- `final_score`, 구성 점수, rank, selected/deferred 이유가 Edit Check/trace에서 설명됨;
- `candidate_count`는 실제 경쟁 사실 수, `candidate_chars`는 그 사실들의 전체 문자 수를 나타내며
  후보 0인데 선택 문자가 존재하는 모순이 없음;
- 한 parent row의 관련 사실과 무관 사실이 서로 다른 relevance를 가지며 parent retrieval score가
  두 자식에 복사되지 않음;
- 각 사실이 source ref, visibility, perspective owner/viewers를 유지하되 이 metadata가 새 거부
  조건으로 쓰이지 않음;
- Text와 네 PDF 표현이 동일한 selected fact·순서·hash를 사용함;
- 리롤·provider 재시도·분기·Say Nothing·Yumi Translator·저장·Critic·벡터 색인 회귀가 없음;
- 실제 “완료한 일을 다시 하려는” fixture에서 필요한 기억이 payload에 들어가고 displayed-final이
  그 완료 상태에서 이어짐.
- 같은 lifecycle의 계획·진행·완료·후속 사실이 각각 점수 후보로 남고 terminal 표식이 다른 후보를
  선제 탈락시키지 않으며, 저장 측 `resolved_threads`는 같은 key의 실제 pending row를 `resolved`로
  갱신함;
- 현실 시간이 흘러도 같은 RP turn distance의 recency 점수는 변하지 않음;
- 강제 벡터 재색인을 반복해도 active/canonical state, pending thread, storyline 수가 늘지 않음;
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
- 사용자 설정과 무관한 별도 K·최소 점수·낮은 점수 filler 또는 문자 상한 채우기를 만들지 않는다;
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
