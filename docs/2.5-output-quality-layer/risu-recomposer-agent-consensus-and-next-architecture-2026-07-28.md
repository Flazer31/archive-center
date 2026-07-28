# Risu Recomposer 에이전트 합의 및 차기 아키텍처

- 작성일: 2026-07-28
- 대상 런타임: `source/Risu Recomposer.js`
- 현재 확인 버전: `0.1.22`
- 성격: 현재 구현 감사, 외부 예제 비교, 차기 구현 순서의 통합 기준
- 코드 참조 원칙: 외부 예제의 강점과 설계 원리만 분석하며 코드, 프롬프트, 상수는 복사하지 않는다.

## 1. 문서의 결론

Risu Recomposer에는 이미 실제 후처리 재작성 경로가 있다. 초안을 분절하고, 여러 역할이 후보를 만들며, Director가 후보를 정렬하고, Composer가 최종 문장을 반환한다.

그러나 현재의 `Enhanced=true`는 다음 두 조건만 강하게 증명한다.

1. 원문과 다른 문장이 반환되었다.
2. 보호 구조와 기본 형식 검사를 통과했다.

아직 다음을 증명하지는 못한다.

- 원문보다 문학적 완성도가 높아졌는가
- 중요한 장면 정보와 감정 비트가 보존되었는가
- 추가된 설정이 근거를 갖는가
- 비밀, 정체성, 시점, 인물 지식 경계가 지켜졌는가
- 사용자 캐릭터의 agency를 침범하지 않았는가
- 압축이나 확장이 장면에 실제로 도움이 되었는가
- 여러 모델의 서로 다른 통찰이 최종 문장에 결합되었는가

따라서 다음 단계는 역할이나 옵션을 더 추가하는 일이 아니다. 현재의 휴리스틱 후보 선택과 기계적 `Enhanced` 판정을 교체하여, **장면 전체 재작성 후보 -> 의미 판정 -> 명시적 Fusion 계획 -> 장면 Composer -> 의미 증명** 경로를 완성해야 한다.

## 2. 이번 협의 범위

다음 세 관점을 합쳤다.

1. 현재 `Risu Recomposer.js`의 실행 경로와 최신 Trace 감사
2. 다음 외부 예제의 출력 강화 구조 비교
   - `Serial_Gradation_Agents_for_RP.js`
   - `risu_agents.js`
   - `risu-multiagent.js`
   - `multiagent-full-v0.8.1`
   - `☸에로스 타워.js`
3. 향후 Archive Center와 연결할 때의 소유권, 문맥 전달, Table Read 경계

하위 에이전트들은 읽기 전용으로 분석했다. 예제 코드는 가져오지 않았으며 현재 런타임 파일도 수정하지 않았다.

## 3. 현재 구현에 대한 합의

### 3.1 이미 작동하는 부분

- `afterRequest`에서 RisuAI 메인 모델의 출력을 `draft_zero`로 받는다.
- 이미지 태그, 상태창, 코드 블록 등 보호 또는 읽기 전용 구간과 수정 가능 구간을 나눈다.
- RisuAI의 캐릭터, 페르소나, 최근 대화, 로어북, 메모리 자료를 읽기 전용 문맥으로 수집한다.
- 장면 신호를 바탕으로 역할을 선택한다.
- 역할별 모델과 Provider 설정을 사용할 수 있다.
- 역할 호출을 Provider 및 endpoint 단위로 스케줄링한다.
- 역할 하나가 실패해도 성공한 후보는 남긴다.
- Composer가 여러 후보를 받아 실제 문장 교체를 수행한다.
- 보호 구간, 빈 출력, 코드 fence, 태그, 장문 중복 등의 구조 검사가 있다.
- Trace에서 호출 모델, 소요 시간, 실패 이유, 적용 구간을 확인할 수 있다.

즉, 현재 구현은 “조언만 기록하는 검토 플러그인”은 아니다. 실제 후처리 재작성 엔진의 기본 골격은 존재한다.

### 3.2 현재 품질 향상이 불확실한 이유

#### `Enhanced` 판정의 의미가 좁다

현재 판정은 Composer가 한 개 이상의 실질 구간을 충분히 다르게 바꾸고 구조 검사를 통과하면 성공으로 볼 수 있다. 사실 보존, 장면 비트 보존, 시점, agency, 문체 우위는 성공 조건에 포함되지 않는다.

#### Materiality가 품질 대신 거리만 측정한다

문장이 짧아지거나 토큰 배열이 달라지면 material rewrite로 통과하기 쉽다. 따라서 장면을 단순 압축한 결과도 “강하게 바뀐 출력”으로 인정될 수 있다.

#### Director가 의미 판정기가 아니다

현재 Director 점수는 모델 자신이 제출한 confidence, 역할 우선순위, issue label, 길이와 표면 유사도에 크게 의존한다. 여러 역할이 같은 issue label을 제출한 것을 consensus로 볼 수 있지만, 실제 주장과 사실이 서로 일치한다는 뜻은 아니다.

#### 후보 계약에 근거와 보존 단위가 부족하다

현재 후보는 주로 `segment_id`, `rewrite`, `confidence`, `issues`, `change_summary`, `tags`로 구성된다. 다음 정보가 강제되지 않는다.

- 안정적인 candidate ID
- 어떤 원문 근거를 바탕으로 바꾸었는지
- 어떤 사실과 장면 비트를 반드시 남겼는지
- 어떤 신규 묘사나 해석을 추가했는지
- 다른 후보와 중복되는지

#### Verifier가 구조만 검사한다

현재 검사는 보호 구간 보존과 문법적 구조에는 유효하다. 그러나 다음 오류는 잡을 수 없다.

- 근거 없는 사건 또는 관계 추가
- 이미 존재하던 장면 비트 삭제
- 비밀 누출과 정체성 분리
- POV 이탈
- 사용자 행동이나 감정 확정
- 문체 평탄화
- 부자연스러운 표현과 시대 오류

### 3.3 최신 실제 결과의 해석

최근 전후 비교에서는 첫 구간이 주로 압축되었고, 두 번째 구간에는 인물 반응과 관계의 밀도가 추가되었다. 동시에 오탈자가 남고, 어색한 표현과 근거가 불명확한 과거 사건이 추가되었다.

따라서 현재 상태는 다음과 같이 평가한다.

- 실제 변경: 확인됨
- 구조 보존: 대체로 확인됨
- 공격적인 재작성: 일부 확인됨
- 여러 역할의 통찰 결합: 제한적으로 확인됨
- 의미 충실성: 미검증
- 원문 대비 품질 우위: 미검증
- Beta 품질 판정: 아직 불가

## 4. 외부 예제에서 확인한 강점

### 4.1 Serial Gradation Agents

가장 강한 장점은 **각 작성 단계가 완전한 장면 전체를 소유한다**는 점이다.

- 첫 작성자가 한 턴의 완성된 초안을 만든다.
- Character AIDE가 같은 장면을 인물 기준으로 전면 재작성한다.
- World AIDE가 현재 전체 초안을 세계와 연속성 기준으로 다시 쓴다.
- Plot AIDE가 같은 턴 안에서 인과, 긴장, 속도, 다음 턴 개방성을 강화한다.
- 단계 실패 시 마지막 완전한 초안을 유지한다.

사용자가 변화를 체감하기 쉬운 이유는 각 고비용 작성 호출이 반드시 완성된 장면을 변환하기 때문이다.

### 4.2 risu_agents

가장 강한 장점은 **구성 가능한 실행 토폴로지와 역할별 설정**이다.

- 독립적인 사전 분석은 병렬로 실행할 수 있다.
- 의존하는 단계는 행 단위로 직렬 실행한다.
- 역할마다 Provider, 모델, 프롬프트, 문맥, 기억, 재시도를 설정한다.
- 후처리 에이전트는 현재 전체 응답을 직렬로 다시 쓰고 최종 응답을 직접 반환한다.

다만 기본 후처리 프롬프트에는 요청된 부분 외에는 유지하려는 성향이 있어, Risu Recomposer의 목표에 그대로 적용해서는 안 된다.

### 4.3 risu-multiagent

가장 강한 장점은 **작고 명확한 인과적 전달**이다.

- World 분석이 Plot에 전달된다.
- World와 Plot이 Character에 전달된다.
- 세 결과가 메인 모델의 입력으로 합쳐진다.

최종 후처리 재작성기는 아니지만, 역할 간 정보 의존성을 최소 구조로 보여준다.

### 4.4 multiagent-full

가장 강한 장점은 **호스트 플러그인과 오케스트레이션의 경계**다.

- RisuAI 플러그인은 요청 관찰과 주입만 맡는다.
- 실제 분석은 sidecar API에 위임한다.
- strict와 fail-open 정책을 분리한다.

단독 한 파일을 우선하는 현재 Recomposer에는 그대로 적용하지 않는다. 추후 규모가 커져 분리가 필요할 때의 운영 대안으로만 남긴다.

### 4.5 에로스 타워

가장 강한 장점은 **검색된 문맥의 품질과 최종 출력 기준의 상태 확정**이다.

- 세계, 인물, 추진력, 종합 역할을 분리한다.
- 로어북과 기억을 예산과 관련도에 따라 선별한다.
- 인물 지식 경계를 고려한다.
- 사전 단계의 결과는 제안이며, 받아들여진 최종 출력만 상태와 기억의 근거가 된다.
- 후처리 단계는 현재 전체 응답을 직접 교체할 수 있다.

장기 기억은 Archive Center가 담당하게 될 예정이므로, Recomposer가 별도의 영구 기억 저장소를 만드는 방식으로 가져오면 안 된다.

## 5. 결합해야 할 핵심 원리

예제의 강점은 다음 형태로 결합한다.

```text
하나의 문맥 증거 묶음
-> 독립 전문 역할의 병렬 장면 후보
-> 별도 의미 판정
-> 하나의 명시적 Fusion 계획
-> 하나의 장면 전체 Composer
-> 구조 검사
-> 별도 의미 증명
-> 검증된 최종 장면 직접 반환
```

이 구조에서 역할은 다음과 같이 나뉜다.

- 전문 역할: 서로 다른 완전한 재작성 가능성을 제시한다.
- Judge: 무엇을 채택하고 버릴지 판정한다.
- Director: Judge의 판정을 결정적인 Fusion 계획으로 변환한다.
- Composer: 최종 장면 전체를 소유한다.
- Prover: 결과가 계약을 만족하는지 독립적으로 증명한다.

한 시점에 최종 문장을 소유하는 모델은 하나여야 한다. 여러 모델의 문장을 단순 연결하거나, 서로 충돌하는 부분 패치를 순서대로 덮어쓰지 않는다.

## 6. 확정할 제품 방향

### 6.1 출력은 초안이다

RisuAI 메인 모델의 출력은 완제품이 아니라 `draft_zero`다. Recomposer의 목적은 오탈자 몇 개를 고치는 것이 아니라, 장면의 핵심 사실과 제약을 지키면서 다음을 실제로 재구성하는 것이다.

- 인물 목소리와 감정의 층위
- 관계의 긴장과 반응
- 대사 하위 의미
- 장면 인과와 공간적 동작
- 사건의 속도와 긴장
- 묘사의 선택과 리듬
- 시점과 정보 공개
- 다음 턴으로 이어지는 장면의 힘

### 6.2 변화를 억제하는 항목과 훼손을 막는 항목을 분리한다

다음은 창작을 약화시키는 제한이므로 정상 Quality 경로에서 제거한다.

- 원문 어휘와 문장 구조를 최대한 유지
- 최소 수정 우선
- 확실한 오류만 수정
- 나머지 문장은 그대로 유지
- 원문과 비슷할수록 높은 점수
- 낮은 severity면 적용하지 않음

다음은 강한 재작성을 하더라도 지켜야 하는 **하드 불변조건**이다.

- 이미지 태그, 상태창, 코드 블록, JSON, RisuAI 특수 마커의 정확한 보존
- 확정된 세계관과 사건 사실
- 비밀, 정체성, 별칭, 공개 상태
- POV 인물이 현재 알 수 있는 정보
- 사용자 캐릭터의 미지정 행동, 감정, 결론
- 현재 턴과 장면의 경계
- 명시된 출력 언어와 형식

### 6.3 Material과 Quality를 분리한다

- `material_rewrite`: 원문과 실질적으로 다르게 재구성되었다.
- `semantic_verified`: 사실, 비밀, POV, agency, 연속성, 장면 비트를 통과했다.
- `quality_preferred`: 별도 비교 판정에서 원문보다 최종문이 우수하다고 판정되었다.

`Enhanced=true`는 적어도 `material_rewrite + semantic_verified`를 만족해야 한다. 실제 Beta 품질 지표에서는 `quality_preferred`도 별도로 기록한다.

## 7. 목표 Standalone 파이프라인

### 7.1 Turn Context

한 턴 동안 변경되지 않는 문맥 묶음을 한 번 만든다.

- 최신 사용자 입력
- RisuAI 메인 모델에 전달된 system 및 최근 대화
- 캐릭터와 페르소나
- 현재 채팅
- 활성 로어북
- Supa/Hypa/현재 채팅 메모리
- 출력 언어와 길이 요구
- 보호 구조
- 사실, 비밀, POV, agency 제약

모든 역할은 같은 snapshot을 사용한다. 역할마다 다시 로어북이나 채팅 전체를 수집하지 않는다.

### 7.2 Draft Ledger

`draft_zero`에서 반드시 보존하거나 판정해야 할 단위를 추출한다.

- established facts
- scene beats
- unresolved hooks
- character intentions
- relationship state
- speaker and POV
- secrets and reveal state
- user-owned decisions
- protected output structures
- candidate factual additions

Ledger는 원문 문장을 그대로 유지시키는 장치가 아니다. Composer가 문장과 장면 구조를 크게 바꾸더라도 의미 단위를 누락하거나 왜곡하지 않게 하는 계약이다.

### 7.3 Scene-wide Specialist Variants

Quality 기본 경로에서는 세 개 안팎의 전문 작성자를 장면에 맞게 선택한다.

1. Character and Relationship Rewriter
   - 목소리, 심리, 관계, 지식 경계, 대사 하위 의미를 중심으로 장면 전체 후보를 작성한다.
2. Plot and World Rewriter
   - 인과, 설정, 시간, 공간, 사건 진행, 장면의 개방성을 중심으로 장면 전체 후보를 작성한다.
3. Style and Dramatic Rewriter
   - 문장 리듬, 이미지, 반복 제거, 감각 밀도, 긴장, 대사와 서술의 균형을 중심으로 장면 전체 후보를 작성한다.

Secret/POV와 Agency/Meta는 일반 재작성 후보 역할에서 분리하여 Judge와 Prover의 하드 판정 기준으로 이동한다.

모든 역할은 전체 ordered segment와 동일한 Draft Ledger를 받는다. 역할당 한 번 호출하며 mutable segment 수에 따라 호출 수가 늘어나지 않는다.

### 7.4 Semantic Judge

Judge는 prose를 작성하지 않는다. 후보별로 다음을 판정한다.

- 근거가 있는 개선 요소
- 반드시 채택할 요소
- 서로 보완되는 요소
- 서로 충돌하는 요소
- 원문보다 약해진 요소
- 삭제된 사실이나 장면 비트
- 근거 없이 추가된 사실
- 비밀, POV, identity, agency 위반
- 문체와 장면 효과의 실제 향상

Judge의 출력은 `semantic_judgment.v1`로 고정한다. 모델의 self-confidence는 Trace 참고값일 뿐 채택의 주된 근거가 아니다.

### 7.5 Deterministic Fusion Plan

Director는 임의의 점수 경쟁 대신 Judge 결과를 결정적인 `fusion_plan.v1`로 변환한다.

- required facts and beats
- must preserve constraints
- accepted candidate elements
- rejected candidate elements and reasons
- conflicts and their resolution
- permitted creative additions
- prohibited additions
- target scene arc
- target voice and pacing
- expected segment coverage

이 단계는 LLM 후보를 이어 붙이지 않는다. Composer가 사용할 장면 설계도를 만든다.

### 7.6 Whole-scene Composer

Composer가 최종 출력의 유일한 작성 소유자다.

- `draft_zero`, Draft Ledger, 후보, Judge 판정, Fusion Plan을 받는다.
- 모든 실질 mutable 구간을 하나의 장면으로 다시 쓴다.
- 문장, 문단, 대사, 묘사, 속도, 강조의 순서를 크게 바꿀 수 있다.
- 보호 및 inspect-only 구간은 건드리지 않는다.
- 후보 문장을 그대로 고르는 대신 여러 후보의 좋은 요소를 새 문장으로 통합한다.
- 같은 턴의 장면 경계를 넘어서 후속 장면을 임의로 진행하지 않는다.

일반 성공 경로는 Composer 결과 하나다. Specialist 후보를 최종 출력에 직접 적용하는 fallback은 제거한다.

### 7.7 Recomposition Extent

재작성 범위 검사는 다음 용도로만 사용한다.

- 원문을 거의 그대로 복사한 소극적 결과 감지
- 설명 없이 지나치게 압축된 결과 감지
- 요구 길이와 장면 비트 범위 이탈 감지

이 검사는 품질을 판정하지 않는다.

### 7.8 Structural Verifier

JS 검사는 결정적으로 확인할 수 있는 구조만 담당한다.

- protected와 inspect-only exact preservation
- 빈 출력
- Markdown/code fence 균형
- HTML/tag 균형
- 중복 장문 블록
- 요청 길이의 심각한 이탈
- 모든 필수 mutable segment 반환

### 7.9 Semantic Prover

Composer와 다른 모델이 최종 출력과 Draft Ledger를 비교한다.

- 확정 사실 보존
- 장면 비트 보존
- 비밀, 정체성, 별칭, 공개 상태
- POV와 인물 지식
- 사용자 agency
- 플롯과 세계 연속성
- 근거 없는 사실 추가
- 문체와 장면 효과

실패 시 위반 항목만 전달하여 Composer에 한 번 수정하게 하고 다시 증명한다. 두 번째에도 실패하면 `draft_zero`를 반환하고 `recomposition_rejected`로 기록한다.

## 8. 현재 코드에서 교체할 소유자

파일 크기가 이미 500KB 제한에 가까우므로 새 경로를 옆에 추가하지 않는다.

| 현재 소유자 | 차기 처리 |
|---|---|
| 현재 segment 단위 `validateCandidateSchema()` | 장면 전체 후보, evidence, retained beats, proposed additions를 갖는 계약으로 교체 |
| confidence 중심 `fusionDirector()` | Semantic Judge 결과를 소비하는 deterministic Director로 교체 |
| issue label 기반 consensus/conflict | 실제 주장과 사실 단위의 합의/충돌로 교체 |
| `textSimilarity()` 중심 후보 가치 평가 | 중복 제거와 재작성 범위 진단에만 제한 |
| `rewriteMateriality()` 성공 판정 | `recomposition_extent` 진단으로 축소 |
| specialist 직접 적용 fallback | 제거 |
| `top_candidate_assembled` 정상 성공 경로 | 제거 |
| `candidateEligibleForApplication()` | Composer-only 반환 경로로 교체하면서 제거 |
| 구조 중심 `verifyOutput()` | 구조 부분 유지, 의미 판정은 Prover로 분리 |
| 기계적 `classifyAppliedOutput()` | material, semantic, comparative quality 상태로 교체 |
| Quality의 5개 이상 유사 역할 | 장면별 3개 안팎의 상호 구분되는 재작성 역할로 재구성 |

기존 API key, Provider adapter, reasoning adapter, endpoint 스케줄러, 문맥 수집기, 보호 구간 처리, UI 저장 경로는 재사용한다.

## 9. 권장 호출 예산

### Balanced

- Specialist 2
- Judge 1
- Composer 1
- Prover 1
- 필요 시 Repair 1
- 정상 5회, 최대 6회

### Quality

- Specialist 3
- Judge 1
- Composer 1
- Prover 1
- 필요 시 Repair 1
- 정상 6회, 최대 7회

현재 Quality가 Specialist 5회와 Composer 1회를 호출하면서도 의미 판정이 없는 것보다, 같은 수준의 호출 예산을 **후보 다양성, 판정, 합성, 증명**에 나누는 편이 목표에 더 가깝다.

한 Provider나 endpoint에 호출이 몰리면 그 endpoint만 직렬 또는 제한 병렬로 실행한다. 다른 endpoint 그룹은 서로 독립적으로 병렬 실행할 수 있다.

## 10. Archive Center 연동 시 역할 분담

Standalone Recomposer를 먼저 완성한다. Archive Center 연결은 별도의 후속 단계다.

### Archive Center 소유

- MariaDB의 canonical truth
- ChromaDB의 후보 검색과 MariaDB 기반 검증
- 장기 플롯, 사건, 상태 기억
- 인물 identity와 alias
- 관계 상태
- 비밀과 reveal 상태
- 인물별 주관 기억
- 출처와 provenance
- 저장, 수정, 삭제, revision 관리

### Recomposer 소유

- 현재 턴의 read-only context packet 소비
- `draft_zero`와 현재 턴 문맥의 Draft Ledger 생성
- 전문 재작성 후보 생성
- 의미 판정과 Fusion Plan
- 최종 장면 재구성
- 구조 및 의미 검증
- 최종 출력과 Trace 반환

Recomposer는 MariaDB 또는 ChromaDB에 직접 쓰지 않는다.

### 연결 계약

Archive Center는 버전이 있는 read-only packet을 제공한다.

```text
archive_narrative_context.v1
- request/session/revision fence
- public scene state
- immutable canonical facts
- writer-only secret ledger
- character knowledge and subjective memory
- relationship state
- plot and open threads
- scene objectives
- world rules
- provenance refs
- unavailable or uncertain fields
- context budgets
```

연결 모드에서는 Archive Center packet과 Standalone 수집기를 동시에 진실 공급자로 사용하지 않는다.

- packet이 완전하면 Recomposer의 중복 lore/memory/planner 수집을 우회한다.
- packet이 일부 비어 있으면 명시된 unavailable 필드만 보완한다.
- 동일한 서사 가이드나 비밀 정보를 두 번 주입하지 않는다.
- 입력 Planner는 검색과 장기 기억 판정을 반복하지 않고 현재 턴 계약의 빈칸만 채운다.

Archive Center 연결은 Recomposer의 문체 품질을 자동으로 해결하지 않는다. 먼저 Standalone의 Writer, Judge, Composer, Prover 경로가 검증되어야 한다.

## 11. Table Read의 위치

Table Read는 Archive Center 연결 이후 가장 큰 효과를 낸다.

### 작동 원칙

- 현재 장면에 실제로 관여하거나 중요한 인물만 선택한다.
- 각 인물 에이전트는 공용 장면 정보와 자기에게 공개된 정보만 받는다.
- 각 인물의 주관 기억과 관계 상태는 그 인물의 관점으로 제한한다.
- writer-only secret ledger는 Moderator, Judge, Prover만 볼 수 있다.
- 인물 에이전트는 “이 인물이 지금 무엇을 알고, 무엇을 오해하고, 어떤 대사와 반응이 자연스러운가”를 제출한다.
- 인물 에이전트가 canonical memory를 직접 쓰지 않는다.
- Composer가 Table Read의 충돌과 반응을 최종 장면에 통합한다.

### 호출 구조

```text
Archive Center context packet
-> active cast router
-> per-character private view
-> parallel character table
-> Moderator conflict summary
-> Semantic Judge
-> Fusion Plan
-> Composer
-> Prover
```

`single_panel`은 Standalone에서도 사용할 수 있다. `full_table_read`는 Archive Center의 인물별 주관 기억이 연결된 뒤 중요 장면용 opt-in으로 구현한다.

## 12. 구현 순서

### Phase A. 성공 의미 교정

1. `Enhanced`를 기계적 변경과 분리한다.
2. Trace에 `material_rewrite`, `semantic_verified`, `quality_preferred`를 별도 표시한다.
3. 기존 materiality 검사를 품질 판정에서 제거한다.

완료 조건:

- 압축만 한 결과가 품질 향상으로 기록되지 않는다.
- 구조 통과와 의미 통과를 구분할 수 있다.

### Phase B. Draft Ledger와 후보 계약

1. Draft Ledger schema를 만든다.
2. scene-wide candidate schema로 교체한다.
3. candidate ID, evidence, retained beats, additions를 강제한다.
4. 중복 후보를 Korean-aware n-gram과 의미 판정 입력 기준으로 정리한다.

완료 조건:

- 후보마다 무엇을 보존했고 무엇을 추가했는지 추적할 수 있다.
- 빈 줄 segment나 동일 후보가 Director 입력을 부풀리지 않는다.

### Phase C. 역할 교체

1. Character/Relationship Rewriter를 장면 전체 작성자로 전환한다.
2. Plot/World Rewriter를 장면 전체 작성자로 전환한다.
3. Style/Dramatic Rewriter를 장면 전체 작성자로 전환한다.
4. Secret/POV와 Agency/Meta 규칙을 Judge/Prover로 이동한다.
5. 역할당 한 번 호출을 유지한다.

완료 조건:

- 모든 고비용 specialist 호출이 완성된 장면 후보를 만든다.
- Quality 정상 경로는 장면에 따라 3개 안팎의 구분되는 후보를 얻는다.

### Phase D. Semantic Judge와 Fusion Plan

1. `semantic_judgment.v1`을 구현한다.
2. 기존 confidence 중심 Director를 제거한다.
3. `fusion_plan.v1`을 구현한다.
4. 합의, 보완, 충돌을 주장과 사실 단위로 계산한다.

완료 조건:

- self-confidence가 높은 후보가 자동으로 이기지 않는다.
- 근거 없는 매력적인 문장이 채택 대상에서 빠진다.
- Composer가 무엇을 결합하고 무엇을 거부해야 하는지 명확히 받는다.

### Phase E. Composer-only 적용

1. Composer가 모든 실질 mutable 구간의 최종 소유자가 되게 한다.
2. specialist 직접 patch fallback을 제거한다.
3. top-candidate 정상 반환 경로를 제거한다.
4. 재작성 범위가 부족하면 Composer에 한 번 명시적으로 재구성하도록 요구한다.

완료 조건:

- 최종 출력은 항상 하나의 통합 장면이다.
- 서로 다른 specialist 문장이 접합된 흔적이 남지 않는다.

### Phase F. Semantic Prover

1. `semantic_proof.v1`을 구현한다.
2. 사실, 비밀, identity, POV, agency, beat coverage, unsupported additions를 검사한다.
3. 한 번의 targeted repair와 재검증을 허용한다.
4. 실패 시 원문과 구체적인 거부 이유를 반환한다.

완료 조건:

- 단순 구조 통과 결과를 `Enhanced`로 표시하지 않는다.
- 근거 없는 과거 사건, 잘못된 인물 지식, 장면 비트 삭제가 실패로 잡힌다.

### Phase G. 실사용 품질 평가

1. 같은 세션에서 연결된 장면을 3턴 이상 진행한다.
2. 원문과 최종문을 build 정보 없이 블라인드 비교한다.
3. 다음 항목을 별도 평가한다.
   - character voice
   - relationship and emotion
   - plot causality
   - world continuity
   - prose rhythm and imagery
   - POV and secret integrity
   - agency
   - factual fidelity
   - overall preference
4. 단순 변경률이 아니라 최종문 선호율과 치명적 위반율을 기록한다.

Beta 후보 조건:

- 정상 턴에서 Composer와 Prover가 지속적으로 완료된다.
- 치명적 사실/비밀/agency 위반이 원문보다 늘지 않는다.
- 블라인드 비교에서 최종문이 원문보다 일관되게 선호된다.
- 실패한 턴은 이유가 Trace에 명확히 남고 무한 대기가 없다.

### Phase H. Archive Center 연결

1. `archive_narrative_context.v1`을 Go backend 계약으로 설계한다.
2. Recomposer에 read-only adapter를 추가한다.
3. 중복 standalone 수집과 Planner 호출을 연결 모드에서 제거한다.
4. 단독형과 연결형의 최종 출력 경로는 동일하게 유지한다.

### Phase I. Table Read

1. active cast router
2. per-character private context view
3. single panel
4. Moderator
5. important-scene full table read opt-in
6. Composer와 Prover 연결

## 13. 문서 충돌 정리

현재 일부 이전 문서에는 다음 방향이 남아 있다.

- 전체 응답 대체를 피한다.
- 최소 patch만 허용한다.
- reviewer는 직접 재작성하지 않는다.
- 변경이 작을수록 좋다.

이 규칙들은 초기 보호 중심 실험과 `light_patch` 경로의 역사적 규칙이다. 정상 `fusion_enhance` Writer 경로에는 더 이상 적용하지 않는다.

계속 유효한 부분:

- 원래 사용자 입력을 변조하지 않는다.
- protected와 inspect-only 구간을 정확히 보존한다.
- auxiliary/helper/submodel 요청은 우회한다.
- 실패 시 무한 대기하거나 깨진 출력을 반환하지 않는다.
- 비밀, POV, identity, agency, 확정 사실을 하드 불변조건으로 취급한다.

후속 문서 정리에서는 이 문서와
`risu-recomposer-mdash-execution-plan-2026-07-24.md`를 정상 Writer 경로의 기준으로 삼고, 이전 bounded-patch-only 문구를 역사/보조 모드로 명시해야 한다.

## 14. 하지 않을 일

- 역할 수를 14개로 다시 늘려 품질 문제를 덮지 않는다.
- 기존 Director 옆에 두 번째 Director를 추가하지 않는다.
- 기존 materiality 옆에 또 다른 quality flag를 쌓지 않는다.
- 예제 플러그인의 코드, 프롬프트, 상수를 복사하지 않는다.
- Recomposer 안에 Archive Center의 장기 기억 저장소를 다시 만들지 않는다.
- Archive Center 연결 전에 full Table Read를 핵심 경로로 만들지 않는다.
- `Enhanced=true`를 실제 품질 우위라고 홍보하지 않는다.

## 15. 최종 합의

Risu Recomposer가 다른 멀티 에이전트 플러그인보다 강해질 수 있는 지점은 호출 수가 아니다.

1. 각 전문 모델이 같은 장면을 서로 다른 전문성으로 **완전하게 재작성**한다.
2. 별도 Judge가 근거, 사실, 비밀, POV, agency와 품질을 판정한다.
3. Director가 판정을 명시적인 Fusion Plan으로 만든다.
4. 하나의 Composer가 여러 후보의 장점을 새로운 완성 장면으로 통합한다.
5. 별도 Prover가 결과를 의미 수준에서 검증한다.
6. Archive Center 연결 후에는 장기 기억과 인물별 주관 기억을 정확히 공급받되, 최종 문장 작성 권한은 Recomposer가 유지한다.

다음 구현 작업은 **Phase A와 Phase B를 한 묶음으로 진행하는 것**이다. 성공 판정의 의미를 먼저 바로잡고 Draft Ledger 및 후보 계약을 교체해야, 이후 Judge와 Composer가 잘못된 입력 구조 위에 다시 쌓이지 않는다.
