# GLM 설계 리뷰 — 2.5 Output Quality Layer

Date: 2026-06-26
Author: GLM (glm-5.2:cloud)
Status: review note, not authoritative spec

검토 대상 문서:

- `README.md` (폴더 인덱스)
- `2.5-standalone-output-quality-layer-plan.md` (active anchor)
- `2.5-mdash-output-harness-plan.md` (historical/strategic reference)

본 문서는 위 세 문서를 읽고, 유저가 제시한 전제(입력 재작성 금지, bounded
patch, 보조 LLM 실패 시 원문 반환, Table Read는 검토 전용/메모리 write 금지)
를 기준으로 작성한 설계 리뷰다.

---

## 1. 문서 이해 요약

세 문서의 관계는 명확하다.

- `README.md`는 폴더 인덱스이며, `2.5-standalone-output-quality-layer-plan.md`
  를 **active anchor**로 지정하고, `2.5-mdash-output-harness-plan.md`를
  **historical/strategic reference**로 위치짓는다.
- **standalone plan**(active)은 RisuAI 단독 작동 우선, Archive Center는 선택
  어댑터, MDASH는 harness 패턴으로 해석. 핵심 파이프라인:
  `Input Enhance → Main LLM → Output Check → Table Read(조건부) → Output
  Enhance → Verifier → fallback`. 입력은 재작성 금지(support-only guidance),
  출력은 전체 재작성 금지(protected/mutable 분리 + bounded patch), 실패 시
  원문 반환.
- **mdash plan**(reference)은 같은 방향을 전제하되, **독립 위성 플러그인
  전략**(Guidance / Output Check / Table Read / Patch Guard 각각 별도 JS)과
  `connected_archive_center` / `standalone_risuai` 이중 모드, 위성별
  RunLedger, Full Table Read 호출 예산(`2C+7~9`)을 더 상세히 다룬다.

두 문서는 방향이 일치한다. standalone plan이 제품 경계·원칙·실행 모드·Done
Criteria를 잡고, mdash plan이 패키징·위성 분할·어댑터 계약·호출 예산을
보강하는 구조.

---

## 2. 현재 설계의 강점

- **원칙 선언이 명확하고 일관됨**: "유저 입력 재작성 금지", "전체 응답 재작성
  금지", "보조 LLM 실패 → 원문 반환", "Table Read는 메모리/DB write 금지, draft
  검토 전용"이 두 문서 모두에서 반복 강조된다. 이는 유저가 요구한 전제와
  정확히 부합.
- **protected/mutable/inspect-only 3단 분할**이 Output Enhance의 핵심 제어기로
  자리잡고 있고, Verifier가 protected 변경/길이 급감/언어변경/무출처 사실/
  사용자 에이전시 침해를 veto 조건으로 명시한 점은 실무적으로 탄탄하다.
- **실행 모드 계단**(off/guidance_only/check_only/light_patch/single_panel/
  full_table_read)와 **호출 예산표**가 있어 비용·레이턴시 기대치를 사용자가
  조절 가능. runtime이 스트리밍/비용/안전 이유로 자동 downgrade한다는 점도
  현실적.
- **fail-open이 원칙**이고, 보조 호출 실패가 메인 출력을 지우지 않는다는 규칙이
  Done Criteria에까지 들어가 있다.
- **Archive Center 의존성 격리**가 잘 정의됨: standalone 입력 목록과 optional
  입력 목록이 분리되어 있고, optional은 "adapter later, must not shape first
  implementation"으로 명시.
- **위성 분할 전략**(mdash plan)이 거대 단일 파일(`Archive Center.js` 재발)을
  막는 구조적 방어책으로 제시됨.

---

## 3. 현재 설계의 약점

- **스트리밍 처리가 선언만 있고 메커니즘이 없다.** "streaming mode must
  degrade safely" / "true pre-display interception may not be available"라고만
  되어 있고, (a) 스트리밍 중 segment 경계를 어떻게 감지할지, (b) patch 적용을
  post-stream으로 미룰지 pre-display 교체를 포기할지, (c) 청크 단위 보호 마커
  누적 파싱 전략이 전혀 없다. 이는 유저가 명시적으로 짚은 위험 항목.
- **protected segment 파서의 명세가 추상적이다.** "image tags, status windows,
  Chatindex, module tags, regex markers, JSON/code blocks"라고 나열만 되어 있고,
  RisuAI 실제 마커 문법(`{{...}}`, `[[...]]`, `<img ...>`, 상태창 블록,
  Chatindex 프레임, regex 슬롯 치환 전/후)에 대한 구체적 토큰화/경계 규칙이
  없다. 파서가 틀리면 보호 자체가 무의미.
- **OOC 오탐 제어가 부재하다.** `ooc_or_meta_separation` 필드는 있지만,
  OOC/메타 발언을 "유저 입력 일부"로 보호할지 "inspect-only"로 볼지, 그리고
  reviewer가 OOC를 캐릭터 발언 오류로 오탐하지 않게 할 분류 기준이 없다. 유저가
  지적한 항목.
- **helper/submodel 간섭 정의가 없다.** RisuAI의 auxiliary request
  bypass(memory/translate/image/module/submodel/Lightboard 등)는 "bypass한다"고만
  되어 있고, 이들이 2.5 레이어를 통과할 때 guidance/patch가 끼어들면 안 되는
  조건, 그리고 submodel 호출이 2.5의 extra call 예산과 충돌하지 않게 하는 격리
  규칙이 없다. 유저가 지적한 항목.
- **Verifier의 "suspiciously shorter" / "same language" 기준이 정량 미정.**
  임계값(예: 길이 비 < 0.7 → reject)이나 측정 단위(segment별? 전체?)가 없어
  구현자마다 달라진다.
- **Table Read와 Output Check의 역할 중복 경계가 흐리다.** Output Check의
  Character Reader/World Reader/Plot Reader와 Table Read의 Character Director/
  Individual Character Reader가 거의 같은 역할을 가리키는데, 언제 Output Check만
  으로 끝내고 언제 Table Read로 넘어가는지의 승격 조건이 명시되지 않았다.
  "when needed"로만 표기.
- **contract 컴파일 비용이 빠져 있다.** Input Enhance가
  user_intent/character/world/plot contract를 매번 LLM으로 빌드한다면
  guidance_only도 0–1 call이 아니라 1 call 고정 + 지연이 된다. 휴리스틱 우회
  조건이 불명확.
- **trace/ledger의 직렬화 포맷과 보존 주기가 없다.** RunLedger 필드는 나열되어
  있으나 저장 위치(plugin-local? 메모리만?), 크기 상한, 민감정보(프롬프트 원문)
  포함 여부 정책이 없다.
- **멀티모달/이미지 생성 태그와 일반 이미지 참조 태그의 구분이 없다.** `<img>`
  생성 태그와 캐릭터 카드 내 이미지 참조가 같은 "protected"로 묶여 있어, 생성
  태그는 보호하되 참조 태그는 inspect-only로 봐야 하는 경우 구분 규칙이 없다.
- **이름 정책이 미정.** "final product name not tied to Archive Center or MDASH"
  라고만 되어 있어 위성 파일명(`Archive Center Guidance.js` 등)과 충돌.
  standalone-first인데 파일명이 Archive Center 중심이다.

---

## 4. 반드시 추가할 문서 항목

아래 항목들을 standalone plan에 섹션으로 추가하거나, 별도 파일
(`2.5-streaming-and-protection-spec.md`,
`2.5-interference-and-ooc-policy.md`)로 분리할 것을 권장.

### 4.1 `Streaming Behavior Contract`

- 스트리밍 모드에서 2.5가 취할 수 있는 3가지 honest 모드 정의:
  (a) **audit-only**(스트림 그대로 송출, 사후 trace만),
  (b) **post-stream patch**(전체 수신 후 segment 분할 → patch → 별도 표시/
  재렌더),
  (c) **non-streaming opt-in**(사용자가 스트리밍을 끄면 pre-display 교체 허용).
- 각 모드에서 UI가 사용자에게 "이 응답은 스트리밍 원문이며 patch는 사후
  적용됨"을 어떻게 표시할지.
- 청크 누적 파서: 보호 마커가 청크 경계에 걸쳐 있을 때 부분 마커를 버퍼링하는
  규칙.

### 4.2 `Protected Segment Token Specification`

- RisuAI 실제 마커별 경계 규칙 표: `{{char}}`, `{{user}}`, `{{slot::...}}`,
  `[[...]]`, `<img ...>`, 상태창 블록 시작/종료 토큰, Chatindex 프레임, regex
  슬롯 치환 전/후, 모듈 태그(`<module:...>` 등 추정), JSON/code fence.
- 각 마커가 protected / inspect-only / mutable 중 어디로 분류되는지, 그리고
  regex 치환은 patch 적용 **전**에 고정해야 한다는 순서 규칙.

### 4.3 `OOC and Meta Separation Policy`

- OOC/메타 발언의 감지 휴리스틱(괄호/대괄호/접두어 `((OOC:))`, `//` 등)과,
  감지 시 해당 영역을 **inspect-only + user-input-equivalent**로 보호하는 규칙.
- reviewer가 OOC를 캐릭터 대사 오류로 오탐하지 않게 하는 분류 힌트를 Input
  Enhance가 `ooc_or_meta_separation` 필드로 제공하고, Output Check는 이 힌트를
  신뢰 우선한다는 규칙.

### 4.4 `Auxiliary Request and Submodel Interference Guard`

- bypass 대상 요청의 분류표(memory/translate/image/module/submodel/Lightboard/
  OtherAx)와, 이들이 2.5 파이프라인을 **건너뛰는 조건**.
- submodel/helper 호출이 2.5의 extra-call 예산에 포함되지 않음을 명시하고, 2.5가
  submodel 응답에 대해 guidance/patch를 시도하지 않는다는 규칙.
- 2.5 자체의 보조 호출이 RisuAI의 submodel 슬롯을 재귀적으로 트리거하지 않도록
  하는 루프 방지 조항.

### 4.5 `Verifier Thresholds`

- reject 임계값 표: 길이 비 < 0.7, protected 바이트 변경 > 0, 언어 코드 변경,
  포맷 마커 소실 개수, 무출처 신사실 도입, 사용자 요청 형식 제거.
- 모든 임계값은 설정에서 사용자가 조정 가능하되 기본값을 명시.

### 4.6 `Output Check → Table Read Promotion Rule`

- Output Check 결과에서 `needs_table_read` 승격 조건: 다수 캐릭터 교차 발언
  충돌, 비밀 누출 위험 high, 세계규칙 충돌 high, 사용자가 `full_table_read` 모드
  명시.
- 승격 시 Output Check의 findings를 Table Read 초기 agenda로 전달하여 중복
  검토를 줄이는 규칙.

### 4.7 `Contract Compilation Cost Policy`

- Input Enhance의 contract를 LLM 없이 휴리스틱으로 빌드하는 조건(최근 N턴 캐시
  재사용, 단순 응답형 요청, guidance_only 모드).
- LLM 기반 contract 빌드는 캐싱 키(request hash + 최근 context hash)를 두어
  동일 턴 재호출 방지.

### 4.8 `Trace Ledger Retention Policy`

- RunLedger 저장 위치(plugin-local memory, 디스크 아님 기본), 최대 보관 턴 수,
  프롬프트 원문 포함 여부 스위치(기본 off), 민감정보 마스킹 규칙.

---

## 5. MVP 범위 제안

첫 구현(standalone, Archive Center 없이)에 **반드시** 들어가야 할 것:

1. **플러그인 셸 + RunLedger + fail-open pass-through** — 보조 호출 전/후
   어디서든 실패해도 원문 반환.
2. **auxiliary request bypass 분류기** — memory/translate/image/module/submodel
   요청은 2.5 파이프라인 우회.
3. **protected segment 파서(구체적 토큰 명세 기반)** — 위 4.2 항목.
4. **`guidance_only` 모드(Input Enhance)** — support-only guidance, 원문 미변경,
   휴리스틱 우선.
5. **`check_only` 모드(Output Check)** — findings만, 출력 미변경, trace 기록.
6. **JS Verifier(임계값 표 기반, LLM 미사용)** — protected/길이/언어/포맷 검사.
7. **`light_patch` 모드(Output Enhance)** — mutable segment에만 bounded patch,
   verifier veto, 실패 시 원문.
8. **스트리밍 honest degradation** — audit-only 또는 post-stream patch 중 택1,
   UI 표시.
9. **trace UI** — requested/actual mode, roles run, calls, downgrade reason,
   findings, patches, fallback reason.

---

## 6. 2.5 이후로 미룰 항목

- **`single_panel` Table Read** — 한 호출로 다역할 시뮬레이션. MVP 직후 검증 후
  추가.
- **`full_table_read`** — `2C+7~9` 호출, 중요 장면 opt-in 전용.
- **Archive Center read-only adapter** — standalone 안정화 후.
- **독립 verifier 모델(별도 provider)** — JS verifier가 안정화된 뒤.
- **위성 분할 패키징**(Guidance/Output Check/Table Read/Patch Guard 각각 별도
  JS) — 단일 플러그인으로 먼저 증명한 뒤 분리. mdash plan의 위성 전략은 좋으나
  MVP에서는 단일 플러그인 내 모듈 분리로 시작하는 편이 위성 간 계약 비용을
  줄인다.
- **멀티모달 이미지 생성 태그 고도 보호**(생성 vs 참조 구분).
- **trace 영속화/디스크 저장**.

---

## 7. 수정된 구현 순서

standalone plan의 12단계와 mdash plan의 2.5-0~2.5-8을 통합하되, 위 약점을 먼저
메우는 순서로 재배치:

1. **플러그인 셸 + RunLedger + fail-open pass-through** (원본 1, 2.5-0 일부)
2. **auxiliary request bypass + submodel 간섭 방지** (원본 2, 2.5-1) — 4.4 항목
   먼저 확정
3. **protected segment 파서(구체적 토큰 명세)** (원본 3) — 4.2 항목 먼저 확정
4. **스트리밍 honest degradation 모드 정의** (신규) — 4.1 항목
5. **`guidance_only`(Input Enhance, 휴리스틱 우선 + 캐싱)** (원본 4) — 4.7 항목
6. **`check_only`(Output Check, findings + trace)** (원본 5)
7. **JS Verifier(임계값 표)** (원본 6) — 4.5 항목
8. **`light_patch`(Output Enhance, bounded patch + veto)** (원본 7, 8) — 4.6 승격
   규칙은 이 단계에서 초안
9. **trace UI** (원본 10) — 4.8 보존 정책
10. **`single_panel` Table Read** (원본 9) — 검증 후
11. **`full_table_read`** (원본 11)
12. **Archive Center read-only adapter** (원본 12)
13. **위성 분할 패키징** (mdash plan 2.5-0 위성화) — 단일 플러그인 증명 후

---

## 8. 최종 권고

문서의 **방향과 원칙은 유저 요구와 정확히 일치**하며, 특히 "입력 재작성 금지 /
bounded patch / fail-open / Table Read는 검토 전용"이라는 네 가지 핵심 전제가
Done Criteria까지 일관되게 내려와 있다. 이 부분은 그대로 유지해야 한다.

다만 현재 문서는 **원칙은 강하지만 메커니즘이 약하다**. 유저가 명시적으로 짚은
네 가지 위험(스트리밍, 이미지/상태창/모듈 태그 보호, OOC 오탐, helper/submodel
간섭)은 모두 "선언은 있고 명세는 없다" 상태다. 이것들이 구현 단계에서 해석이
갈리면 원칙이 무력화된다. 따라서:

- **4.1~4.4 항목을 standalone plan에 섹션으로 추가하거나 별도 spec 파일로
  분리**하여 구현자가 판단 없이 따를 수 있게 만들 것.
- **MVP는 단일 플러그인 + 모듈 분리**로 시작하고, mdash plan의 위성 분할은
  단일 플러그인이 안정화된 뒤 패키징 단계에서 적용할 것. 위성 계약 비용이 MVP
  복잡도를 불필요하게 올린다.
- **Verifier 임계값(4.5)과 protected 토큰 명세(4.2)는 코드를 한 줄 짜기 전에
  먼저 문서로 고정**할 것. 이 두 가지가 2.5 전체 안전성의 뼈대이므로.
- **파일/제품 명명 정책**을 standalone-first에 맞게 정리할 것. 현재 위성
  파일명이 `Archive Center *.js`인데 standalone-first 원칙과 모순되므로, neutral
  이름(예: `OutputQualityLayer.*`)으로 바꾸거나 명명 정책 섹션을 추가할 것.

이렇게 보완하면 2.5는 "더 많은 에이전트"가 아니라 "제어된 품질 파이프라인"이라는
차별점이 기존 multi-agent / risu_agents / Serial Gradation 대비 명확히 살아날
것이다.