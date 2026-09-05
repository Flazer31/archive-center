# Archive Center 4.2 작업 기록

기준일: 2026-09-05

기준선:

- 공개 4.1 parent: `574c2d5b2295b0d50630474fb695055d5417777c`
- 로컬 4.2 계획 기준점: `48a63711e9acc79ab514a4bb0a1ea183dd5b65b3`
- 로컬 4.2 구현 기준점: `43bc20a19698d8e85c910c1b04f33b557eca42c7`
- 로컬 4.2 package 증거 기준점: `c2f1a2d51d39663c5c97091c24bb2e59d6e3d1f9`
- 활성 작업 branch: `work/4.2.0`

이 문서는 4.2-A부터 4.2-G까지의 구현과 증거 단계를 기록한다. 소스·자동 회귀·패키지·
로드된 RisuAI·실제 MariaDB/ChromaDB·provider payload·displayed-final은 서로 대신하지 않는다.

## 0. 최초 계획 이후 추가된 4.2 작업

최초 4.2 계획 기준점 `48a6371`과 첫 구현 기준점 `43bc20a`의 중심 범위는 A~F의
Priority Score·사실 단위 후보·canonical identity·전역 핵심 기억 K·Priority Memory Pack,
그리고 G의 사용자 선택형 저장 확정 시점이었다. 이후 실제 RisuAI와 장기 세션을 시험하면서
아래 작업이 4.2에 추가됐다. 현재 상세 계약은 이 추가 작업까지 포함하지만, 처음부터 있던 계획과
실측 뒤 확장된 범위를 혼동하지 않도록 여기서 분리한다.

| 추가 범위 | 최초 계획과 달라진 점 | 현재 경계 | 상세 기록 |
| --- | --- | --- | --- |
| 최근 완결 대화 기반 검색 문맥 | Chroma 결과 수와 결합돼 있던 최근 원문 깊이를 `최근 대화 참고 수`로 분리하고, 현재 입력과 최근 user+assistant 완결 대화를 각각 semantic query로 사용 | 원문은 검색에만 사용하고 본문 payload에 중복 주입하지 않음 | 2.3, 9.1, 9.2 |
| 사실 자체의 의미 점수 연결 | parent Memory 검색 점수를 모든 자식 사실에 복사하지 않고, 기존 `precise_memory_units` vector 점수를 일치하는 사실 하나에만 연결 | 새 검색기·새 저장소·JavaScript selector 없음 | 2.1~2.3 |
| RP 턴 기준 최신성과 lifecycle 후순위화 | 현실 시간이 아닌 source turn 거리로 importance와 recency를 감쇄하고, lifecycle 판정은 최종 점수·canonical winner·탈락 조건에서 제외 | 계획·진행·완료·후속 사실을 각각 점수 후보로 보존 | 2, 9.4 |
| 완성 턴 요약과 자료 분류별 K | 최초의 단일 전역 K 대신 완성 `turn_summary` 그룹과 각 scored fact 분류가 같은 UI 최대 수를 독립적으로 사용 | 남는 K는 이동하지 않고 기존 자료별·전체 문자 예산만 최종 상한으로 사용 | 3, 9.5 |
| 완료 사건 저장·재색인 수선 | 4.5의 범용 lifecycle을 앞당긴 것이 아니라, 같은 lifecycle key의 명시적 resolve가 기존 pending row를 닫고 재색인이 파생 상태를 중복 저장하지 않게 보정 | 새 schema·문구 기반 완료 추측·범용 약속 lifecycle 없음 | 9.3 |
| raw-only Critic 재개 | raw user/assistant pair만 있고 파생 자료가 없는 턴을 JavaScript가 중복으로 조기 종료하던 동작 제거 | 기존 `/complete-turn`이 raw 중복 없이 Critic과 파생 저장을 재개 | 9 |
| 지연 저장 정책 전달과 6+6 HUD | G의 저장 mode 자체에 더해, compact 결과의 정책 누락과 단일 12단계 HUD 전제를 실제 RisuAI에서 보정 | 현재 생성 1~6과 직전 Critic·저장 1~6은 같은 Go 12단계 장부의 UI projection | 9.6~9.8 |
| 구조화 배열 canonical 충돌 보정 | 서로 다른 source row의 `rules[0].rule` 같은 배열 순번이 같은 사실 ID로 충돌하지 않도록 `source_occurrence`를 identity에 유지 | 일반 객체의 current resolution과 저장·리롤·분기는 변경하지 않음 | 9.9 |
| Critic 자동 복구 HUD 닫기 | durable reprocessing 중인 `recovering` 카드에 X를 제공 | X는 화면 카드와 event stream만 닫고 복구 job은 계속 실행 | 9.10 |
| 설정 UI와 진단 정리 | 검색 수 세 항목 분리, PDF·저장 시점의 일반 설정 이동, compact query count 노출, 체크박스 줄바꿈과 설명 문구 정리 | 설정 내부 값과 backend 동작은 유지 | 9.1, 9.2, 9.11 |

이 추가 범위는 새 엄격 조건이나 다중 거부 gate를 만들기 위한 작업이 아니다. 실제로 너무 많은
후보가 들어오거나, 정상 기억이 전부 빠지거나, 다른 사실이 같은 identity로 합쳐지거나, 저장은
완료됐지만 HUD가 멈춰 보인 실측을 기존 owner에서 직접 고친 작업이다.

다음 항목은 4.2에서 회귀를 확인했지만 4.2 신규 기능으로 계산하지 않는다.

- 4.1의 Text·Google/Vertex·Gateway·Provider Manager PDF 전송;
- Yumi Translator 1.4.2 모델 원문 판정;
- provider timeout 동일 요청 재사용;
- 같은 Host user row의 리롤·편집 재생성 교체와 분기·Say Nothing 저장 규칙.

최신 Windows 패키지는 위 추가 범위를 모두 포함한다. 실제 RisuAI에서는 `최근 대화 참고 수`의
query count와 `다음 사용자 입력 시`의 한 턴 지연 저장이 확인됐다. 현재 110턴을 생성한 상태에서
109턴까지 저장된 관찰은 이 mode의 정상 결과다. 기억 품질, 실제 provider payload,
Critic 복구 X와 최신 UI 모양은 각각의 실환경 gate로 계속 분리한다.

## 1. 4.2-A — 저장→점수→payload 기준선

기존 production 조립 경로에서 저장 source, 검색 결과, lane 후보, 최종 예산, 렌더링,
`payload_application_plan.v1`과 output-fidelity 관찰을 이어 확인했다. 4.1 경로는 후보가 lane용
문자열로 평탄화된 뒤 일부 저장 점수와 source identity가 최종 예산까지 유지되지 않고,
objective event K만 전역 기억 상한처럼 사용되는 제한이 있었다.

4.2는 별도 검색기나 JavaScript selector를 만들지 않고 같은 Go 조립 경로에
`memory_delivery_plan.v2`를 연결한다. 현재 입력, 직접 근거, explicit correction, 비밀·관점,
branch/revision 권한은 점수 경쟁 밖의 기존 권한으로 유지한다.

## 2. 4.2-B/C — Priority Score와 사실 단위 후보

Go의 `prepare_turn_priority_memory.go`가 다음 값을 후보에서 최종 선택까지 보존한다.

- `canonical_fact_id`, canonical key, source ref/table/row/occurrence, lane
- 중간에서 자르지 않은 자연어 사실과 문자 수
- relevance, 저장 importance, 턴 감쇄 importance, RP-turn-distance recency, continuity bonus
- 진단 전용 lifecycle metadata와 `priority_score.static.v3` final score·결정적 rank
- selected/deferred/superseded 상태, reason, `superseded_by`

점수는 다음 정적 시작식이다.

```text
final_score = relevance * 0.60
            + (importance * turn_distance_recency) * 0.25
            + turn_distance_recency * 0.15
            + continuity_bonus
            + structured_bias
```

`importance_after_turn_decay`는 저장 importance에 RP 턴 최신성을 곱한 값이다. 관련성이 같으면 최근
사실이 직접 최신성 점수와 덜 감쇄된 importance를 함께 받아 앞선다. lifecycle transition은
canonical winner·final score·eligibility·저장 허용 여부에 사용하지 않고 진단으로만 남긴다.
최신성은 현실 시간이 아니라 RP 턴 거리만 사용한다.

Memory의 importance/emotional/narrative 신호, pending thread priority, storyline/canonical
confidence, Persona·private memory의 importance를 source metadata로 연결했다. 큰 JSON은
경로별 완전한 사실로 투영하되 원본 row·JSON은 바꾸거나 삭제하지 않는다. 별도 LLM 호출이나
DB migration은 추가하지 않았다.

### 2.1 사실 단위 보정

초기 4.2 구현은 일부 source를 최종 section 문자열에서 다시 분해했고, Memory parent row의
retrieval `selection_score`가 각 자식 사실의 relevance보다 높으면 그 값을 자식 relevance로
사용했다. 이 때문에 한 row 안의 관련 사실과 무관 사실이 같은 높은 점수를 받을 수 있었고,
payload budget의 `candidate_chars`가 0인 채 selected 문자가 존재하는 진단 모순도 남았다.

현재 source는 다음처럼 보정했다.

- 기존 eligibility/public-private projection을 바꾸지 않고 요청 단위 `PriorityFactSeed`를 생성;
- structured Memory item과 각 typed source line을 최종 section 조립 전 독립 사실로 투영;
- 각 사실에 source row/occurrence, lane/tier, visibility, perspective owner/viewers를 연결;
- relevance를 현재 사용자 입력과 관찰된 entity scope에 대해 사실별 계산;
- parent selection score는 `score_lineage`에 관찰값으로만 남기고 자식 relevance에는 사용하지 않음;
- fact identity 단위 current resolution과 실제 fact K 소비;
- 아직 seed가 없는 source는 `legacy_rendered_line` 호환 분해로 계속 전달 가능;
- `candidate_count`, `candidate_chars`, `selected_count`, `selected_chars`를 실제 fact pool과 최종
  전달값에서 계산.

visibility/관점 정보는 기존 projection이 이미 허용한 사실의 lineage이며, scorer가 다시 기억을
거부하는 조건으로 사용하지 않는다. 새 DB schema, 저장 경로, 검색기, Publisher, JavaScript
selector 또는 이중 보호 장치는 추가하지 않았다.

### 2.2 실측 일반 기억 0건 보정

실제 4.2 payload에서는 일반 기억이 `후보 165291 chars → 선택 504 chars`처럼 보였지만, 그
504자는 direct evidence와 protected memory였고 scored general fact 2,906개는
`canonical_current_resolution=574`, `no_current_context_affinity=2332`로 모두 제외되었다.

원인은 두 가지였다. 이미 읽힌 session Memory 전체를 기존 recall 결과와 무관하게 fact 후보로
넣어 후보 수를 부풀렸고, 그 뒤 relevance 0을 `no_current_context_affinity`로 일괄 defer하는 hard
gate가 semantic 점수를 받지 못한 일반 기억을 전부 제거했다.

현재 source는 aggregate Memory fact를 기존 `tier=memory` recall이 실제 선택한 parent row에서만
만든다. session 전체 Memory를 후보로 우회 편입하던 loop와 relevance 0 hard gate는 제거했다.
relevance·importance·recency·continuity는 전역 순위를 정하지만, 점수 하나가 0이라는 이유로 정상
기억 전달을 거부하지 않는다. K는 최종 fact 개수 상한이고 문자 예산을 채우는 목표가 아니다.
인물 blacklist, source별 차단, 새 visibility·관점 gate, 저장 거부 조건은 추가하지 않았다.

이 변경의 전체 회귀에서 기존 Persona capsule의 보호 지침이 세미콜론 때문에 별도 사실 여러 개로
분리되는 문제와 한국어 `책상이`/`책상` 같은 조사 차이가 함께 드러났다. 보호 지침은 원래 기억과
한 K 단위로 유지했고, 비ASCII 어절의 한 글자 끝 차이만 priority relevance에서 인식하도록 했다.
Persona 전용 강제 통과 조건은 만들지 않았다.

### 2.3 현재 장면 의미 관련도 보정

기존 4.2 사실 분해 뒤에도 실제 요청에서는 두 단절이 남아 있었다. `continuity_query`가 있어도
vector search가 `raw_user_input`을 우선할 수 있었고, 이미 색인되는 `precise_memory_units`의 사실
vector 점수가 일반 기억 최종 순위로 이어지지 않았다. 따라서 같은 parent Memory row의 무관한
자식은 lexical 관련도만으로 경쟁했고, 인물·장소를 primary query에 합친 보정은 서로 다른 사건을
구별하기 어려웠다.

역사 기록상 직전 assistant 원문을 기억 검색 질의에서 제거한 정확한 경계는
`594e00a1df837aa16e33e8eff9b441d988404afa` (`2026-07-23`, `Archive Center 3.4.0-dev`)다.
이보다 앞선 `097301f`의 `Recent Raw Turn` 제외는 RisuAI가 이미 보내는 대화 이력을 최종 payload에
다시 넣지 않기 위한 별도 변경이다. 4.2는 검색 앵커만 복원하며 최종 payload 중복은 복원하지 않는다.

현재 source는 다음 하나의 production 경로로 보정했다.

- `prepareTurnRetrievalQueries()`가 명시적인 `continuity_query` 또는 `raw_user_input`을 현재 질의로
  잡고, 별도 UI `recent_conversation_reference_count` 수만큼의 최근 완결 대화를 RisuAI 관찰
  메시지에서 역순으로 가져온다. 각 완결 대화는 사용자 입력과 최종 assistant 원문을 함께 가진
  하나의 semantic query이며, 여러 query에서 같은 Chroma 문서가 잡히면 가장 높은 유사도 관찰
  하나만 유지한다. 이 설정은 Chroma 결과 `top_k`와 최종 핵심 기억 최대 수를 변경하지 않는다;
- `prepareTurnEffectiveContinuityQuery()`는 같은 query set을 추적 가능한 진단 문자열로 투영하고,
  fact lexical scorer는 각 query에 대한 사실 관련도를 따로 계산해 최고값을 사용한다. 원문은 검색
  앵커일 뿐 최종 payload에 추가하지 않는다. 특정 continuation 문구 목록이나 `이어서 적어주세요`
  특수 조건은 없다;
- 기존 aggregate `tier=memory` 검색은 그대로 유지한다. 따라서 precise/evidence 문서가 aggregate
  Memory recall을 밀어내지 않는다;
- MariaDB `precise_memory_units`에서 source-active/public/general-memory이며 worldline history turn
  범위 안인 canonical unit snapshot과 session별 실제 개수를 먼저 얻는다;
- 동일 request query set에서 만든 query vector들을 aggregate와 precise 검색이 함께 재사용해 기존
  Chroma 문서의 `source_table=precise_memory_units`만 canonical 후보 수만큼 조회한다. final K=5 같은
  전달 설정이 사실 점수 관찰도 5개로 자르지 않는다. 별도 selector·저장 owner는 없다;
- precise hit의 내부 handoff는 hydration 뒤 public response에서 제거한다;
- fact vector similarity는 source turn과 fact identity가 맞는 사실 하나에만 연결한다. parent Memory
  retrieval score나 한 unit의 점수를 다른 자식 사실에 복사하지 않는다;
- 인물/화자 `0.04`, 장소 `0.05`, storyline `0.06`을 합계 `0.12` 이내의 독립 rank bias로 남긴다.
  이는 기억을 허용·거부·삭제하는 조건이 아니다;
- recency는 현재 RP turn과 source turn의 거리, 32-turn half-life, `0.20` floor로 계산한다. 현실
  시간이나 단순 최신 순번이 오래됐지만 계속 유효한 사실을 0으로 만들지 않는다;
- 이름·별칭·identity evidence는 `identity_metadata`로 추적하되 사건/current-state K를 소비하지
  않는다. 검토된 같은 canonical entity의 선택 사실에 한 번 붙을 수 있지만, metadata가 문자
  예산에 맞지 않아도 그 사실을 막지 않는다.

Publisher, PDF/Text representation, Yumi 원문 판정, 저장 확정 시점, Critic, 리롤, provider timeout
재시도, 분기와 JavaScript lifecycle은 이 보정에서 변경하지 않았다. DB migration, 새 저장 owner,
새 검색기, phrase classifier 또는 이중 eligibility gate도 추가하지 않았다.

최근 원문 `top_k` 복원 뒤 대상 테스트, `go test ./internal/httpapi -count=1`,
`cmd/js-route-variant-smoke`를 제외한 전체 Go package, `go vet ./...`가 통과했다. 전체
`go test ./... -count=1`에서는 현재 실행 환경의 PATH에 `node`가 없어
`cmd/js-route-variant-smoke`의 Node 실행 fixture 5개만 시작되지 못했다. 이번 변경은 JavaScript를
수정하지 않았지만, 최신 package 생성 전 Node fixture 재실행은 별도 gate로 남긴다.

## 3. 4.2-D/E — current resolution, 턴 요약·자료 분류별 순위와 K

요청 단위 projection에서 structured current-state source는 canonical field identity를 사용해
현재 revision을 primary로 고른다. 검토된 인물 alias는 character-state identity에만 사용한다.
자연어 사건·private/perspective occurrence는 같은 문구라는 이유로 합치지 않는다. 같은 occurrence와
lifecycle key를 공유해도 계획·진행·완료·후속 사실은 각각 전역 점수 후보로 유지한다. 완전히 같은
사실만 동일 identity로 dedupe하며, lifecycle terminal 여부는 primary 선택을 소유하지 않는다.

eligible scored-memory 사실은 각 자료 분류 안에서 `final_score` 순서로 경쟁한다. recalled
`memories.turn_summary`는 별도 그룹으로 분리하고 같은 row의 자식 사실 중 최고 점수를 턴 점수로
사용한다. 선택된 턴은 쪼개지 않은 완성 `turn_summary`로 렌더한다.

`core_objective_memory_max_items`는 기본값 5의 공통 최대값이며, 완성 턴 요약 그룹과 event,
character objective, subjective/relationship, world state, unresolved goal 사실 분류에 각각 독립
적용한다. 남는 K는 다른 그룹으로 넘기지 않는다. 직접 근거와 비밀 보호는 K 밖의 기존 권한이다.
한 문장 턴 요약과 동일한 자식 사실은 한 번만 전달하며 fact K를 소비하지 않는다.

새 설정은 추가하지 않았다. auto와 custom 모두 같은 `memory_delivery_plan.v2` owner를 사용한다.
custom mode는 기존 UI 자료별 주입 예산을 class 문자 상한으로 적용하고, 턴 요약과 event 사실은
`event_recent` 예산을 공유한다. 모든 항목은 자료별 예산과 전체 문자 예산을 함께 지키며 중간에서
자르지 않는다.

## 4. 4.2-F — Priority Memory Pack과 Publisher·전송 연결

선택된 사실만 source ref와 함께 짧은 자연어 pack으로 렌더한다. 기존
`publisher_plan.v2`는 새 Publisher를 만들지 않고 실제 selected/delivered ref를 사용한다.
Persona/private 후보가 선택된 경우 기존 해석·privacy guidance를 K를 소비하는 별도 기억으로
만들지 않고 해당 후보의 렌더링 guidance로 붙인다.

Text, Google AI Studio/Vertex PDF, LLM Gateway PDF, Provider Manager PDF는 모두 동일한 완성
`payload_application_plan.v1`의 selected fact ID·순서·렌더링 hash를 소비한다. PDF 경로가 별도
검색이나 순위를 만들지 않는다.

## 5. 4.2-G — 사용자 선택형 저장 확정 시점

일반 설정의 `임베딩 Timeout (초)` 바로 아래에 `저장 확정 시점`을 제공한다.

- `응답 직후` (`immediate_after_response`): 기본값이며 4.1 `afterRequest → /complete-turn`
  경로를 그대로 사용한다.
- `다음 사용자 입력 시` (`next_user_input`): 직전 표시 후보의 최소 Host 좌표와 hash만 pending
  marker로 보관하고, 실제 새 사용자 행의 `beforeRequest`에서 그 직전 최종 pair를 기존
  `/complete-turn`으로 보낸다.

Go가 `turn_finalization_policy.v1`로 mode를 확정한다. JavaScript는 Go 정책이 없으면 4.1 기본
동작을 사용하며 mode를 독자적으로 판정하지 않는다. 같은 user row의 리롤·편집 재생성은 pending
marker만 최종 assistant로 교체하고 저장하지 않는다. 내용이 같더라도 새 user row이면 직전 pair를
확정한다. 다른 branch/session은 marker를 소비하지 않는다.

직전 complete-turn은 현재 prepare-turn과 같은 시간대에 시작하지만 await하지 않는다. 느리거나
실패한 직전 Critic은 현재 본문 요청을 막지 않으며 두 번째 Critic, 저장 API, scheduler, hidden retry,
종료·세션 전환 auto-save를 만들지 않았다. 안정적인 직전 행을 확인할 수 없을 때는 marker를
남기고 현재 요청을 계속한다. 이 확인은 정상 출력 거부 조건이 아니라 잘못된 과거 pair 저장을
피하기 위한 pending 대상 identity 확인이다.

## 6. 자동 회귀 결과

2026-09-03 기존 4.2 package 기준으로 다음 검사가 통과했다.

- `node --check "Archive Center.js"`
- `go test ./internal/httpapi -count=1`
- `go test ./cmd/js-route-variant-smoke -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `scripts/test-simple-fresh-install.ps1`의 Windows checksum/install-pointer와
  Linux x64/arm64·macOS Intel/Apple Silicon·Termux arm64 한 줄 설치 계약

검증 범위에는 stored-score 전파, structured fact 분해, current resolution, 검토된 alias,
occurrence 보존, importance 우선순위, 무관 importance takeover 방지, 당시 전역 K, no-fill,
oversized top-K, direct-evidence/privacy 보존, Text/PDF plan 일치, 기본 즉시 저장, 같은 행 리롤,
편집 재생성, 동일 문장 새 행, 비대기 직전 Critic, 재시작 marker 복원, 다른 branch 격리가 포함된다.

사실 단위 보정 뒤 추가한 production-path 회귀는 다음을 고정한다.

- 한 Memory row의 두 structured 사실이 후보 2개가 되고 K=1이 관련 사실만 선택;
- parent selection score가 자식 relevance로 사용되지 않으며 score lineage에는 관찰값이 남음;
- owner-private 두 문장이 각각 사실 후보가 되고 동일 visibility/owner/viewers를 유지;
- seed가 없는 기존 section도 `legacy_rendered_line`으로 두 사실 모두 전달 가능;
- 문장 뒤 `source_turn=...` 표식은 별도 사실이 되지 않고 원래 사실에 붙음;
- 기존 active-interaction exact delivery, canonical current resolution, Text/Google/Vertex/Gateway/
  Provider Manager PDF의 동일 logical text/hash가 유지됨;
- session-wide Memory row가 기존 recall을 우회해 priority 후보로 들어오지 않음;
- relevance 0 fact도 hard rejection 없이 전역 점수와 K에서 처리되어 일반 기억 전체가 0건이 되지
  않음;
- 짧은 입력도 phrase classifier 없이 같은 effective continuity query 경로를 사용함;
- Persona 보호 기억·사용 지침이 한 단위로 유지되고 한국어 조사 차이에서도 기존 전달이 유지됨;
- `continuity_query`가 Chroma embedding과 fact scorer에 동일하게 전달되고 raw input은 이를
  덮어쓰지 않음;
- final K보다 많은 canonical precise fact를 기존 `source_table` metadata로 검색하고, MariaDB로
  확인한 `precise_memory_unit` similarity가 matching fact 하나에만 연결되며 parent/sibling fact에는
  복사되지 않음;
- 같은 인물·장소의 오래된 다른 사건보다 현재 장면 사건이 우선되고, 사용자가 오래된 사건을
  명시한 경우에는 그 사건의 의미 유사도가 turn-distance recency를 이김;
- speaker/location/storyline bias가 점수 lineage에 독립적으로 남고 rejection reason은 만들지 않음;
- 최신/과거/미상 source turn이 32-turn half-life와 `0.20` floor를 따름;
- 이름·별칭·identity evidence가 event/current-state fact K를 소비하지 않고 same-entity 선택 사실에
  metadata로 한 번만 붙음.

이전 4.2 검증 주기에는 `node --check "Archive Center.js"`와 Node production fixture를 포함한
`go test ./... -count=1`이 통과했다. 이후 최근 원문 `top_k` 검색 복원은 JavaScript를 변경하지
않았고 대상 Go 테스트와 `go vet ./...`를 다시 통과했다. 현재 실행 환경에는 Node가 없어 같은
JavaScript fixture의 재실행은 아직 별도 gate로 남아 있다.

## 7. 완료 증거 단계

| 단계 | 상태 | 증거 |
| --- | --- | --- |
| `SOURCE_IMPLEMENTED` | 완료 | 활성 Go/JavaScript/DTO contract 구현 |
| `REGRESSION_VERIFIED` | Go 완료 / Node 재확인 필요 | 최근 원문 `top_k` query-set 대상 회귀, HTTP API 전체, Go vet 통과. 현재 환경에 Node가 없어 JS runtime fixture는 실행되지 않음 |
| `PACKAGE_BUILT` | 완료 | 독립된 `최근 대화 참고 수`와 최근 user+assistant 완결 대화 query-set을 포함한 Windows 4.2 package 재생성, manifest·ZIP·격리 실행 검증 통과 |
| `LOADED_RISU_VERIFIED` | 미확인 | 실제 RisuAI에서 새 plugin hash 로드 필요 |
| `REAL_MARIADB_CHROMA_VERIFIED` | 미확인 | 기존 4.1 DB를 사용하는 package stack 확인 필요 |
| `PROVIDER_PAYLOAD_VERIFIED` | 미확인 | 실제 provider payload의 selected fact/hash 확인 필요 |
| `DISPLAYED_FINAL_EFFECT_VERIFIED` | 미확인 | 동일 DB/input 4.1/4.2 A-B 결과 확인 필요 |

마지막 네 실환경 단계가 끝나기 전에는 기억 품질 개선이나 4.2 릴리스 완료를 주장하지 않는다.

## 8. Windows 4.2 테스트 패키지

공식 `ops/build-full-package.ps1`로 다음 단일 테스트 패키지를 다시 생성했다. 현재 package는
ChromaDB 검색 수와 분리된 `최근 대화 참고 수`, 최근 user+assistant 완결 대화 query-set 및
일반 설정으로 이동한 PDF·저장 확정 시점 UI를 포함한다.

- 폴더: `_test-builds/Archive-Center-4.2.0-windows-test-20260903`
- 실행: `Archive Center 4.2.0 Windows Test Package/01_start_archive_center_windows.bat`
- package status: `green`, `release_ready=true`, automatic update apply 활성
- 관리 파일: 52개, 크기·SHA-256 불일치 0개
- package source: `c2f1a2d51d39663c5c97091c24bb2e59d6e3d1f9`, `source_dirty=true`
  - 현재 4.2 사실 단위·K·의미 관련도 보정이 아직 commit 전이므로 package manifest가 이를
    숨기지 않는다.
- Go toolchain: `go1.26.6 windows/amd64`
- ZIP 크기: `17,947,084 bytes`
- ZIP SHA-256: `de71ac45af9363023a4d344cf4a96dc95e707eb6bc4ebd2b7d39eacf80095ebb`
- `SHA256SUMS-4.2.0.txt` 일치
- source/package `Archive Center.js` SHA-256:
  `9cea4710bb561c6b27eed4d7a5ed20c339018485e47a8056d04af159412e2ed9`
- packaged `archive-center-go.exe` SHA-256:
  `3041a03d721c2da72fcab39d9a482ca48e5c43d98949d35c4e1638e47e0d27d2`
- package fresh-install smoke: `core_lite`/`off`, 별도 포트 `28180`, `/health`·`/ready`
  통과, failure 0. 현재 실행 환경에는 Node가 없어 package JavaScript 문법 검사는
  `node_not_available` 경고로 남겼다.

ZIP은 `.env.full.local`, `.runtime`, `.updates`, DB와 사용자 자료를 포함하지 않는다. 로컬 테스트
폴더의 `.env.full.local`, `.runtime`, `.updates`는 갱신 전 임시 백업 뒤 복원했다. 환경 파일과
복구 백업 해시는 일치하며, `.runtime/reports/fresh-install-smoke.json`만 현재 smoke 결과로 갱신됐다.
이전 4.1 테스트 빌드 폴더는 제거해 `_test-builds`에는 이 4.2 폴더 하나만 남겼다.

갱신 전에 남아 있던 해당 package launcher를 종료했고, Go 백엔드·MariaDB·ChromaDB의 대상 포트가
모두 비어 있는 것을 확인했다. package smoke는 실제 사용자 DB나 ChromaDB에
연결하지 않는 격리 모드로 수행했다. 따라서 현재 기록은 최신 package 바이너리의 실행·readiness
증거이며, 실제 RisuAI 재로드와 기존 MariaDB/ChromaDB를 사용한 `full_local` gate는 여전히 별도다.

## 9. raw-only 턴의 Critic 재개 보정

실제 RisuAI에서 `응답 직후` 저장 모드로 일반 요청을 수행했을 때 사용자 입력
`이어서 적어주세요.`는 `active_chat:196`의 정상 입력으로 캡처됐고, 모델 `afterRequest`도
정상 호출됐다. 그러나 turn 99의 raw user/assistant pair가 이미 있다는 이유로 JavaScript가
`duplicate complete-turn pair skipped`를 기록하고 `/complete-turn` 전에 반환했다. 이때 turn 99에는
Critic 파생 자료가 없었으므로 HUD가 `6/12 본문 응답 기다리는 중`에 머물렀다.

`Archive Center.js`에서 이 raw-pair 존재만 확인하던 조기 반환을 제거했다. 새로운 판정 조건이나
별도 저장 경로는 추가하지 않았다. 기존 Go `/complete-turn` owner가 다음 두 상태를 그대로 구분한다.

- raw pair만 존재: raw row를 중복 기록하지 않고 기존 turn에서 Critic과 파생 저장을 재개;
- raw와 파생 자료가 모두 존재: 기존 완료 replay로 멱등 종료.

대상 회귀로 JavaScript가 reserve 뒤 기존 `/complete-turn`을 호출하는지, Go가 raw-only 상태에서
Critic을 다시 실행하면서 raw log를 중복 기록하지 않는지, 완성된 replay는 파생 자료를 중복 기록하지
않는지를 확인했다. 이 회귀는 현재 `node --check`, Node production fixture를 포함한 전체
`go test ./...`, package fresh-install smoke에서 다시 통과했다.

이 보정과 최근 원문 `top_k` 검색 복원을 포함해 Windows 4.2 테스트 package와 ZIP을 다시 만들고
source/package JavaScript hash, 52개 관리 파일, checksum 및 격리 fresh-install smoke failure 0을
확인했다. smoke의 유일한 경고는 현재 환경의 `node_not_available`이다. 갱신 전 남아 있던 해당
package launcher만 종료했고 Go·MariaDB·ChromaDB 대상 포트가 비어 있음을 확인했다. 실제 사용자
DB를 사용하는 재시작은 자동 수행하지 않았고, 새 plugin 재임포트 뒤의 loaded-Risu 및 full-local
확인은 아직 남아 있다.

### 9.1 compact 응답의 최근 원문 검색 진단

실제 RisuAI 플러그인은 `/prepare-turn`에 `prepare_turn.production_compact.v1`을 요청하므로 full
`recall_result.vector_shadow`가 Network Response에서 보이지 않았다. 검색·선택 동작은 바꾸지 않고
compact `trace_preview.vector_recall_query`에 다음 비내용 진단값만 노출했다.

- `query_text_source`
- `query_text_count`
- `recent_conversation_query_limit`
- `recent_conversation_query_count`
- `query_vector_count`
- `query_history_embedding_error_count`

원문 query, embedding, vector 값과 full `recall_result`는 계속 compact 응답에서 제외된다. 따라서
이 변경은 최근 완결 대화가 실제 요청에서 몇 개 관찰·벡터화됐는지 확인하기 위한
진단 표면이며, 기억 검색·점수·K·저장·payload 적용 조건을 추가하거나 변경하지 않는다.

현재 이름의 여섯 필드는 생산 HTTP 회귀에서 full `recall_result` 없이 반환되는지를 검증한다.
이전 package 기록은 당시 assistant-only/`top_k` 결합 구현에 대한 증거이므로 아래 분리 변경의
package 증거로 재사용하지 않는다. 실제 RisuAI Network와 실제 MariaDB/Chroma 검색 결과는 새
package를 다시 만든 뒤 확인할 실환경 gate로 남긴다.

### 9.2 최근 대화 참고 수 분리와 일반 설정 배치

기존 `top_k`가 ChromaDB 결과 수와 최근 assistant 원문 깊이를 함께 결정하던 결합을 제거했다.
일반 설정의 기억 영역에는 다음 세 수를 독립적으로 표시한다.

- `ChromaDB 의미 기억 검색 수`: Chroma aggregate Memory 결과 수;
- `핵심 연관 기억 최대 수`: 최종 전달되는 점수 기반 기억 사실의 최대 수;
- `최근 대화 참고 수`: 현재 입력과 별도로 검색에 참고하는 최근 완결 대화 수.

최근 완결 대화 하나는 같은 대화의 사용자 입력과 최종 assistant 출력을 함께 가진 하나의 검색
query다. 현재 사용자 입력은 별도 query이므로 이 수를 소비하지 않고, 이 대화 원문은 최종 본문
payload에 다시 주입되지 않는다. RisuAI에서 관찰한 active-chat 복사본을 Yumi 1.4.2 원문 판정과
같은 Archive-only read copy로 전달하며 기존 payload, 저장, 리롤, PDF, Publisher 경로는 바꾸지
않는다. compact 진단의 `recent_conversation_query_limit/count`로 설정값과 실제 완결 대화 수를
확인할 수 있다.

`장기 기억 전달 방식`과 `저장 확정 시점`은 동작을 바꾸지 않고 고급 설정에서 꺼내 일반 설정의
`임베딩 Timeout (초)` 바로 아래로 옮겼다. 대상 Go 회귀, 전체 HTTP API 회귀 및 UI source 배치
회귀는 통과했다. 현재 환경에는 Node 실행 파일이 없어 JavaScript 문법 검사와 Node runtime fixture는
미확인이다.

이 변경까지 포함해 위 Windows 4.2 테스트 package와 ZIP을 다시 생성했다. source/package
`Archive Center.js` 해시가 일치하고, 52개 관리 파일 검증과 checksum 검증 및 격리
`core_lite`/`off` fresh-install smoke가 failure 0으로 통과했다. ZIP에는 `.env.full.local`, `.runtime`,
`.updates`가 없으며 로컬 테스트 폴더의 해당 상태는 갱신 전후 동일하게 복원했다. loaded-Risu,
실 MariaDB/Chroma 및 displayed-final 검증은 여전히 별도다.

### 9.3 완료 사건 lifecycle·재색인 수선

실제 100턴 콜드 스타트 자료에서 완료 사실과 `state_deltas.resolved_threads`가 존재해도 이전
`pending_threads`와 여러 `open/current` 표현이 남았고, 관리자 재색인이 저장된 Critic 결과를 전체
파생 저장 경로로 재생해 active/canonical state 중복을 늘리는 현상을 확인했다. 다음 범위만
수정했다.

- Critic이 같은 사건의 `state_claims`, `pending_threads`, opened/resolved thread에 동일한
  `lifecycle_key`를 유지하고 완료 시 `complete/resolve`를 직접 출력한다;
- Go current-state owner와 pending thread physical key가 같은 lifecycle identity를 사용한다;
- `resolved_threads`와 terminal state claim이 실제 open pending row를 `resolved`로 갱신한다;
- 4.2 사실 단위 전달은 같은 lifecycle key의 terminal fact가 다른 단계 사실을 선제 탈락시키지 않고,
  각 사실을 전역 점수 후보로 유지한다;
- 관리자 canonical reindex는 core memory/evidence/precise-memory vector admission 직후 종료해
  post-admission state·thread·storyline을 다시 저장하지 않는다;
- 이미 resolved인 pending row의 반복 저장은 새 중복 row를 만들지 않는다.

새 DB 컬럼·테이블, 새 검색 경로, 문구별 완료 판정, JavaScript 정책, Publisher/PDF/저장 시점 변경은
추가하지 않았다. 기존 key 없는 자료는 exact-title 호환 경로를 유지한다. 관련 대상 테스트와
`internal/httpapi`, `internal/store` 전체 회귀는 통과했다. `go test ./...`의 나머지 Go package도
통과했으나 `cmd/js-route-variant-smoke`의 Node fixture 5개는 현재 실행 환경 PATH에 Node가 없어
실행되지 않았다. 이번 변경에는 `Archive Center.js` 수정이 없다.

### 9.4 1.0형 전역 순위 복원과 lifecycle 후순위화

정밀한 4.2 사실 문장과 source/visibility/관점 lineage는 유지하면서, 1.0의 장점이었던 관련도·중요도·
최신성의 실질적인 전역 경쟁을 우선했다. 현실 시간 감쇠는 소설/RP의 휴식 기간을 기억 노후화로
오인하므로 복원하지 않았고, 기존 RP 턴 거리 반감기와 최저점을 유지했다.

- lifecycle key가 fact family나 canonical winner identity를 덮어쓰지 않는다;
- 같은 occurrence에서도 내용이 다른 계획·진행·완료·후속 사실은 각각 후보로 남는다;
- lifecycle key와 explicit transition은 점수에 영향을 주지 않는 진단 lineage만 제공한다;
- 완료 유사 문구는 continuity bonus나 lifecycle rank를 만들지 않는다;
- 최근 대화 원문은 Chroma와 사실 relevance의 검색 문맥으로만 쓰며 최종 memory payload에 다시
  넣지 않는다;
- JavaScript, Publisher, Text/PDF 전송, 저장 확정 시점, Critic 저장 lifecycle은 변경하지 않았다.

이 9.4 source 보정은 아래 2026-09-04 09:40 KST 패키지 기록 이후 변경이다. 현재 source 회귀와
패키지·loaded RisuAI 검증을 구분하며, 아래 해시의 기존 패키지에 9.4가 들어갔다고 보지 않는다.

같은 단일 Windows 테스트 package를 2026-09-04 09:40 KST에 갱신했다.

- package status `green`, `release_ready=true`, 관리 파일 52개;
- ZIP `17,954,387 bytes`, SHA-256
  `93e7b09b451f88e6e9f02e03198560aec9489f517928370dbd31890236716444`;
- `archive-center-go.exe` `31,971,328 bytes`, SHA-256
  `8bcdcac00bb52b111b9a9c8ceba592be56e230fa4a03afcb33132542e1dabcf9`;
- source/package Critic prompt hash 일치, `state_deltas.resolved_threads` 계약 포함;
- source/package `Archive Center.js` SHA-256는 기존과 같은
  `9cea4710bb561c6b27eed4d7a5ed20c339018485e47a8056d04af159412e2ed9`;
- 외부 ZIP checksum 일치;
- package `core_lite`/vector `off` 격리 fresh-install smoke에서 `/ready` 정상,
  failure 0; 현재 환경의 `node_not_available` 경고만 남음.

패키지 갱신을 위해 그 패키지에서 실행 중이던 Go, 관리형 MariaDB·ChromaDB와 launcher만 종료했고
다른 Python 프로세스는 건드리지 않았다. 현재 세 프로세스는 종료 상태다. 기존 오염 자료는 자동
삭제하지 않았으며, 새 package를 다시 실행한 뒤 해당 Archive Center 세션만 백업·삭제하고 Risu
원문에서 콜드 스타트를 한 번 수행해야 새 lifecycle 자료로 일관되게 재구성된다.

### 9.5 완성 턴 요약·자료 분류별 K와 UI 예산 연결

2026-09-04에는 사실 하나가 K 하나를 소비해 기억 전달 폭이 지나치게 좁아진 실측을 기준으로 다음
선택 계약을 같은 Go owner 안에서 보정했다.

- 기존 recall이 선택한 `memories` row만 완성 `turn_summary` 후보가 된다;
- 턴 요약 점수는 같은 row의 자식 사실 중 최고 `final_score`이며, 선택된 요약은 중간 분해 없이
  완성 문장으로 전달된다;
- UI `핵심 연관 기억 최대 수`는 턴 요약 그룹과 각 scored fact 자료 분류에 독립 적용되고 남는
  항목 수는 다른 그룹으로 넘어가지 않는다;
- 한 문장 요약과 정확히 같은 자식 사실은 한 번만 전달하며 fact K는 소비하지 않는다;
- auto/custom mode가 모두 `memory_delivery_plan.v2`를 사용한다. custom은 새 설정 없이 기존
  `자료별 주입 예산`을 class 문자 상한으로 사용하고, 턴 요약과 event fact는 `event_recent`
  예산을 공유한다;
- 직접 근거와 비밀 보호의 K 면제, 전체 문자 상한, 온전한 항목 단위 defer, Publisher/Text/PDF 및
  저장 lifecycle owner는 그대로 유지했다.

새 최소 점수, 다중 eligibility gate, lane 간 K 대여, JavaScript selector, DB migration 또는 저장
변경은 추가하지 않았다. 새 생산 경로 회귀 네 건, 기존 Priority Memory 대상 회귀,
`go test ./...`, `go vet ./...`, `node --check "Archive Center.js"`가 통과했다. 이 9.5 source 변경은 아직 package·loaded RisuAI·실
MariaDB/ChromaDB·provider payload·displayed-final 검증을 거치지 않았다.

### 9.6 다음 입력 확정 정책 전달과 이중 HUD

실제 RisuAI에서 `다음 사용자 입력 시`를 선택해도 HUD가 응답 직후 1~12단계를 모두 진행한 원인은
Go가 반환한 `turn_finalization_policy.v1`가 `tryPrepareTurn()` 뒤 `orchestrateTurnHelpers()` 결과에서
누락되어 `onAfterRequest()`가 기본 `immediate_after_response`로 되돌아간 것이었다. 저장 조건이나
Critic owner를 바꾸지 않고 compact·legacy orchestration 결과가 같은 요청의 Go 정책을 그대로
전달하도록 수정했다.

같은 모드에서 새 입력으로 직전 `/complete-turn`이 시작되면 HUD는 다음처럼 분리한다.

- 위: 현재 턴의 기억 검색·출판사·본문 응답 진행;
- 아래: 직전 턴의 평론가·원문/파생 저장·벡터 색인 진행.

아래 카드는 기존 `/turn-workflow/events`를 해당 직전 request ID로 한 번 더 구독해 Go ViewModel을
표시하는 UI 관찰이다. 새 저장 API, 새 Critic, 대기 조건, 자동 재시도, 폴링 또는 별도 persistence
경로를 추가하지 않았다. `응답 직후`는 기존 단일 HUD와 기존 `/complete-turn` 경로를 유지한다.

운영 `orchestrateTurnHelpers()`와 `tryCompleteTurn()`을 직접 실행하는 Node 회귀로 정책 전달, 모드별
HUD routing, 현재 카드 위·직전 카드 아래 순서, 직전 카드의 단일 request-scoped event stream을
확인했다. 기존 HUD RootDocument/stream/reroll 회귀와 JavaScript 문법 검사도 통과했다. 이 상태는
source/regression 증거이며 loaded RisuAI·실 MariaDB/ChromaDB 검증과는 구분한다.
정책 전달과 이중 HUD에 한정한 `Archive Center.js` 변경량은 `+341/-26`줄이다.

같은 단일 Windows 4.2 테스트 패키지를 2026-09-04 15:33 KST에 갱신했다.

- package status `green`, `release_ready=true`, automatic update apply 활성, 관리 파일 52개와
  크기·SHA-256 불일치 0개;
- source/package `Archive Center.js` SHA-256 일치:
  `8a71c27f225cf13de7de565d8dec5c9b67e76a6c606b4b2bacb5a7dfe12ec5e6`;
- ZIP `17,961,145 bytes`, SHA-256
  `498eac6e15bf676453dcbaa2a1f329a91c47d098be9c2a3b9935a6f67cba4a46`, 외부 checksum 일치;
- ZIP에 `.env.full.local`, `.runtime`, `.updates`가 없고, 로컬 테스트 폴더의 해당 상태는 보존했다;
- package `core_lite`/vector `off` 격리 fresh-install smoke에서 `/ready` 정상, warning/failure 0.

### 9.7 다음 입력 모드 HUD 6+6 분리와 직전 장부 보존

실제 RisuAI에서 위·아래 HUD가 모두 나타난 뒤에도 직전 카드가 `작업 중단 ·
superseded_by_new_request`로 바뀐 반면 MariaDB에는 해당 턴의 원문·기억·근거가 저장된 사실을 대조했다.
원인은 새 턴의 `/prepare-turn`이 같은 세션의 유일 active HUD를 교체한다는 기존 즉시 저장 전제를 지연
저장 모드에도 적용한 것이었다. 저장이나 Critic은 정상 완료됐지만 HUD 장부만 먼저 terminal invalidation되어
후속 stage 갱신을 제대로 표현하지 못했다.

수정 범위는 HUD 생명주기와 표현에 한정했다.

- `응답 직후`: 기존 단일 `1/12~12/12` HUD와 같은 세션의 이전 미완료 요청 supersession을 유지한다.
- `다음 사용자 입력 시`: 해당 턴 장부에 Go가 확정한 지연 저장 시점을 기록하고, 다음 요청이
  시작될 때 그 직전 장부를 보존한다. 그 사이 UI 설정을 바꿔도 이미 생성된 직전 장부의 소유권은
  바뀌지 않는다.
- 위 카드: 기존 backend stage 1~6을 `현재 턴 생성 1/6~6/6`으로 표시한다.
- 아래 카드: 기존 backend stage 7~12를 `직전 턴 평론가·저장 1/6~6/6`으로 표시한다.
- 지연 응답이 실제로 수락되면 완료된 위 카드는 닫히고, pending marker만 다음 입력을 위해 유지한다.

Go의 12단계 장부, `/complete-turn`, Critic, 원문·파생 저장, 리롤·재시도·분기·Say Nothing 규칙은
변경하지 않았다. 새 조건, 거부 gate, watcher, polling, 저장 API 또는 persistence owner도 추가하지 않았다.
backend ledger 단위 회귀, 실제 `/prepare-turn` production handler 회귀, JavaScript 6+6 projection과 모드별
routing fixture, JavaScript 문법 검사를 통과했다. 최신 package·loaded RisuAI·실 DB displayed-final 검증은
후속 상태로 구분한다.

같은 단일 Windows 4.2 테스트 패키지를 2026-09-04 16:37 KST에 다시 갱신했다.

- package status `green`, `release_ready=true`, automatic update apply 활성;
- 관리 파일 52개, 누락 0개, 크기·SHA-256 불일치 0개;
- source/package `Archive Center.js` SHA-256 일치:
  `f7686b6208167eebfe8d484f0fdadf47e80e53652b933adf32615c7e869e70ac`;
- packaged `archive-center-go.exe` SHA-256:
  `2f83553f704a3286c9c7ec94a9b3ae20e3310cbee8d6b9b0fd7a0e57ef44e942`;
- ZIP `17,962,472 bytes`, SHA-256
  `dd4b80a434db26d281b71defdacbe439f36a6c7df77e371df08be6de723909c5`, 외부 checksum 일치;
- ZIP에 `.env.full.local`, `.runtime`, `.updates`가 없으며 갱신 전 로컬 상태는 파일 수와 hash가
  일치하도록 복원했다;
- package JavaScript 문법 검사와 `core_lite`/vector `off` 격리 fresh-install smoke가 통과했고,
  `/ready` 정상, failure 0이다.

이는 `PACKAGE_BUILT`까지의 증거다. 이 최신 패키지를 실제 RisuAI에 다시 import한 뒤 6+6 HUD와
실 MariaDB/Chroma 저장 결과를 확인하기 전에는 `LOADED_RISU_VERIFIED` 또는 실환경 완료로 간주하지
않는다.

패키지 교체를 위해 경로로 확인한 패키지 Go 백엔드와 `01_start_archive_center_windows.bat`
launcher만 종료했다. 공용 사용자 runtime의 MariaDB·ChromaDB는 강제 종료 시 데이터 손상 위험이
있어 실행 상태를 유지했으며, 새 패키지는 자동 시작하지 않았다. loaded RisuAI의 실제 두 카드 표시와
실 MariaDB/ChromaDB 완료 동작은 사용자가 새 `Archive Center.js`를 import한 뒤 확인해야 한다.

### 9.8 완료된 생성 카드 종료와 직전 완료 카드 개별 닫기

실제 RisuAI에서 현재 본문이 표시되고 직전 평론가·저장도 완료된 뒤, 현재 카드는 여전히
`6/12 본문 응답 기다리는 중`으로 남고 직전 `저장 완료` 카드는 눌러도 사라지지 않는 현상을 확인했다.
저장과 평론가는 완료됐으며 문제 범위는 두 HUD 카드의 종료·dismiss 처리였다.

- `다음 사용자 입력 시`의 `afterRequest`가 본문을 수락하고 marker를 저장하면 현재 생성 stream을 끝내고
  현재 카드만 제거한다. marker와 다음 입력의 기존 `/complete-turn` 경로는 유지한다.
- 직전 카드가 terminal이면 현재 카드 유무와 관계없이 click listener를 연결한다.
- 직전 카드를 누르면 직전 watcher/ViewModel만 제거하고, 진행 중인 현재 카드는 그대로 둔다.
- 두 카드가 함께 있으면 현재 stage 1~6과 직전 stage 7~12를 각각 독립적인 `1/6~6/6`으로
  렌더링한다. 내부 12단계 Go 장부와 순서는 바꾸지 않는다.
- 저장, Critic, 리롤, 재시도, 분기, Say Nothing, 기억 선택 조건은 변경하지 않는다.

production JavaScript 함수를 실행하는 회귀 fixture에서 현재 카드 종료 후 직전 카드 보존과, 직전 완료 카드
제거 후 현재 카드 보존을 각각 확인한다. 최신 패키지 재생성 뒤 loaded RisuAI 확인은 별도 단계로 남긴다.

같은 단일 Windows 4.2 테스트 패키지를 2026-09-04 17:11 KST에 갱신했다. 이 HUD 종료·개별 닫기·독립
1~6 표시 slice의 `Archive Center.js` 변경량은 직전 테스트 패키지 대비 `+51/-27`줄이다.

- source/package `Archive Center.js` SHA-256:
  `557067398bff8a1e87114109c3311e89fc3949f2aa47440d385cc4f08da8d3d5`;
- packaged `archive-center-go.exe` SHA-256:
  `2f83553f704a3286c9c7ec94a9b3ae20e3310cbee8d6b9b0fd7a0e57ef44e942`;
- ZIP `17,962,478 bytes`, SHA-256
  `59d363a3899fe7a0b660078fd5a97643d74f8353423d42ee6f2e25a77ad3b9c1`, checksum 일치;
- 관리 파일 52개, 누락·크기/SHA 불일치 0, ZIP private-state 항목 0;
- source와 package JavaScript 문법 검사, 전체 Go test와 vet가 통과했다.

갱신 전에 있던 `.env.full.local`, `.runtime`, `.updates`는 파일 수와 hash를 대조해 테스트 폴더에 복원했다.
패키지 재빌드를 위해 경로가 일치한 Go backend만 종료했으며 새 패키지는 자동 시작하지 않았다. 따라서 최신
close/dismiss/1~6 표시의 loaded RisuAI 확인은 새 플러그인 import 후 남아 있다.

### 9.9 구조화 배열 canonical identity 충돌 보정과 패키지 갱신

2026-09-05 실측 `/prepare-turn` 자료에서 서로 무관한 `canonical_state_layers` 규칙들이 각 행의
`rules[0].rule`이라는 배열 순번 경로만으로 같은 `canonical_fact_id`를 받아
`canonical_current_resolution`로 탈락하는 문제를 확인했다. 배열 순번은 한 projection 내부의 표시
순서일 뿐 여러 source row에 걸친 사실 정체성이 아니므로, 배열 순번이 포함된 구조화 사실은 기존
`source_occurrence`를 canonical key에 유지하도록 기존 Go owner만 수정했다. 일반 객체의 동일 필드
갱신, 점수, lane K, Publisher, Text/PDF, 저장·리롤·분기 및 JavaScript는 변경하지 않았다.

운영 조립 함수를 사용하는 새 회귀는 수정 전 실패하고 수정 후 통과했으며, 기존 구조화 동일 필드
`planned -> completed` current resolution 회귀, Priority Memory 전체 대상 테스트, `go test ./...`,
`go vet ./...`가 통과했다. 같은 단일 Windows 4.2 테스트 패키지를 2026-09-05 00:51 KST에 갱신했다.

- package status `green`, `release_ready=true`, automatic update apply 활성;
- 관리 파일 52개, 누락·크기·SHA-256 불일치 0개;
- source/package `Archive Center.js` SHA-256 일치:
  `557067398bff8a1e87114109c3311e89fc3949f2aa47440d385cc4f08da8d3d5`;
- packaged `archive-center-go.exe` SHA-256:
  `196fcbf744cc0b56bb0cb7b0dca747db07b9bb0b66a9bc198611ceec3d8a06a3`;
- ZIP `17,962,420 bytes`, SHA-256
  `5a4aed18d4cb5c28d6fa12419fd1e6480b850086ff5a5d860f16c883e2e58757`, 외부 checksum 일치;
- migrations `001`부터 `012`까지 포함, ZIP private-state 항목 0;
- package JavaScript 문법 검사 통과;
- packaged backend `/version`은 `4.2.0`;
- `core_lite`/vector `off` 격리 fresh-install smoke에서 `/health`·`/ready` 정상, failure 0. Smoke의
  `node_not_available` 경고는 bundled Node로 별도 실행한 package JavaScript 문법 검사 통과로 보완했다.

갱신 전 해당 package 경로의 Go backend와 `01_start_archive_center_windows.bat` launcher만 종료했고,
MariaDB·ChromaDB 및 대상 포트는 이미 닫혀 있었다. 기존 `.env.full.local`, `.runtime`, `.updates`의
4개 파일은 갱신 후 hash 불일치 없이 복원했으며 fresh-install smoke report만 새 결과로 갱신했다.
새 package는 자동 시작하지 않았다. 따라서 이 기록은 `PACKAGE_BUILT`까지이며, 실제 RisuAI 재-import와
배상문 사례의 payload/displayed-final 확인은 별도 `LOADED_RISU_VERIFIED` 단계로 남는다.

### 9.10 Critic 자동 복구 HUD X 닫기

2026-09-05 실환경에서 Critic이 HTTP 200과 정상 종료를 반환했지만 JSON 괄호 불일치로
`CRITIC_JSON_PARSE_FAILED`가 발생했고, durable reprocessing이 예약된 `recovering` HUD에는 X가
표시되지 않는 문제가 확인됐다. 원인은 Go ViewModel이 모든 비종료 상태의 `dismissal_policy`를 `none`으로
동기화하고, JavaScript도 `terminal` presentation에만 닫기 listener를 연결하던 기존 표시 경로였다.

자동 복구 자체는 계속 비종료 상태와 기존 MariaDB job을 유지한다. Go는 `recovering`에 기존
`x_only` 정책을 보내고, JavaScript는 그 정책이 있는 복구 카드에만 X를 렌더링하고 연결한다. 단일 HUD와
`다음 사용자 입력 시`의 직전 턴 HUD를 모두 지원하며, X는 Host의 카드와 event stream만 닫는다. Critic
job을 취소하거나 삭제하지 않고 저장·재시도·리롤·분기·기억 선택에도 조건을 추가하지 않았다.

대상 Go HUD 회귀, 실제 JavaScript 함수를 실행하는 HUD smoke, 전체 `go test ./...`, `go vet ./...`,
JavaScript 문법 검사가 통과했다. 같은 Windows 4.2.0 테스트 패키지를 2026-09-05 01:34 KST에 갱신했다.

- package status `green`, `release_ready=true`, automatic update apply 활성;
- 관리 파일 52개, 누락·크기·SHA-256 불일치 0개;
- source/package `Archive Center.js` SHA-256 일치:
  `c6726f239662667e230823f6842375d226e7e33103f8e434437523e799d27e51`;
- packaged `archive-center-go.exe` SHA-256:
  `be376915dcb88a24e3f5275a60a31dbe7a5dad5a66aa3acac3be0b325a8e0043`;
- ZIP `17,962,459 bytes`, SHA-256
  `f8b56a363ea8c4f88778551da5492be29c724d4667a31010b51ba6f277f7c9d7`, 외부 checksum 일치;
- ZIP private-state 항목 0개;
- `core_lite`/vector `off` fresh-install smoke에서 ready 정상, failure 0.

갱신 전 실행 중이던 해당 package의 Go backend와 BAT launcher만 종료했다. 기존 `.env.full.local`,
`.runtime`, `.updates` 4개 파일은 hash 불일치 없이 복원했고 새 package는 자동 시작하지 않았다. 따라서
실제 복구 중 X 표시와 닫은 뒤 background reprocessing 완료는 새 플러그인 import 후 확인해야 한다.

### 9.11 공통 설정 문구와 체크박스 배치 정리

2026-09-05 실환경 설정 화면에서 `Web Risu 직접 연결 (실험)`의 안쪽 설명 label이 상위
`.mo-row label`의 고정 flex 폭을 다시 상속해 글자가 짧은 단위로 줄바꿈되는 문제를 확인했다. 체크박스
wrapper가 행의 남은 폭을 사용하고 안쪽 label이 그 폭을 채우도록 CSS만 보정했으며, 600px 이하에서도
체크박스와 설명이 같은 flex 행을 유지하도록 했다.

사용자 요청에 따라 `핵심 연관 기억 최대 수`와 `최근 대화 참고 수` 설명은 각각 첫 문장만 남겼다.
`저장 확정 시점` 선택지의 표시 문구는 `현재 턴`과 `이전 턴`으로 줄이고 하단 설명은 제거했다. 설정의
내부 값 `immediate_after_response`/`next_user_input`, 기본값, 저장 및 백엔드 전달 동작은 변경하지 않았다.
한국어·영어·일본어 표시를 함께 맞췄다. 이 UI slice의 JavaScript 변경은 `+16/-19`줄이다.

JavaScript 문법 검사와 실제 설정 UI가 포함된 `js-route-variant-smoke`가 통과했다. 같은 Windows 4.2.0
테스트 패키지를 다시 갱신했다.

- package status `green`, 관리 파일 52개, 누락·크기·SHA-256 불일치 0개;
- source/package `Archive Center.js` SHA-256 일치:
  `87ffa29e3836daa6e63c85c1e554ed643eab59bb67857adf925e8d3616295819`;
- packaged `archive-center-go.exe` SHA-256:
  `be376915dcb88a24e3f5275a60a31dbe7a5dad5a66aa3acac3be0b325a8e0043`;
- ZIP `17,961,925 bytes`, SHA-256
  `a40f88c3b68c5d0bab5727da5c25a0e94a23220bee4a8bb35f19706da6cf9aae`, 외부 checksum 일치;
- ZIP private-state 항목 0개.

갱신 전 해당 package의 Go backend와 BAT/PowerShell launcher만 종료했다. 기존 `.env.full.local`,
`.runtime`, `.updates` 4개 파일은 hash 불일치 없이 복원했고 새 package는 자동 시작하지 않았다. 실제
줄바꿈 모양은 새 플러그인을 RisuAI에 import한 뒤 확인해야 한다.

## 10. 4.2에서 다루지 않은 범위

- 전체 cross-surface 의미 통합: 4.3
- 범용 source-linked bundle과 선택 사건의 원인·결과·후속 사건 관계 점수 전파: 4.4
- 물건·계획·약속 전반의 일반화된 영구 lifecycle ontology: 4.5
- 모델/context별 adaptive 점수·K·예산: 4.6
- 인간형 기시감·부분·완전 회상: 5.x
- 기억 Review/Undo·quarantine 관리 UI: 5.8

## 11. 4.2.0 정식 릴리스 준비 (2026-09-05)

사용자 요청에 따라 4.1과 같은 Windows 설치/업데이트, Linux x64/arm64,
macOS Intel/Apple Silicon, Termux arm64의 7개 패키지를 공식 release builder로
생성했다. 플러그인 channel은 `release`로 맞추고 신규 설치와 기존 설치의 업데이트,
RisuAI 플러그인의 별도 갱신 절차를 README와 릴리스 안내에 명시했다.

- 현재 소스의 JavaScript 구문, 전체 Go test, vet, Git LF 기준 Go 형식 검사 통과.
- Windows 신규 설치 production-entrypoint/다운로드 검증과 POSIX 패키지 생성 계약 통과.
- 공개 GitHub checksum과 보관된 4.1 ZIP 7개 해시 일치.
- 4.2 ZIP 7개 manifest/hash, source plugin, 누적 migrations, CPU binary header,
  POSIX 실행 권한, private-state 미포함 검사 통과.
- 동일한 production packageupdate 코드로 7개 실제 4.1 패키지에서 4.2 후보의
  direct-update preflight 통과. 이것은 각 OS의 native runtime 실행 증거는 아니다.
- 공개 Windows 4.1 updater 바이너리로 두 Windows 패키지의 적용, 모든 관리 파일
  복구, 재적용, commit과 로컬 설정/runtime sentinel 4종 보존 통과.
- Windows 후보 패키지의 격리 core_lite/off fresh smoke: warning/failure 0.
- Ubuntu/macOS production POSIX 설치 계약과 native updater 회귀를 기존 CI에 추가.

릴리스 커밋 `4257081c217e57b7e570592fb1090b484255c013`의 원격 CI 네 작업이 모두
통과했고, 최종 7개 ZIP과 checksum을 `v4.2.0` 최신 안정판으로 공개했다.
공개 직후 실제 MariaDB/Chroma를 둔 격리 Windows 4.1에서 UI와 같은 업데이트 요청을
실행하여 4.2 재시작, `committed`, 대화·기억·벡터와 설정 보존을 확인했다.
구체적인 범위는 [공개 및 업데이트 검증 기록](archive-center-4.2.0-release-verification.md)을 따른다.
이 결과는 모든 OS 실기기 검증이나 loaded RisuAI·Provider 품질 검증을 대체하지 않는다.
