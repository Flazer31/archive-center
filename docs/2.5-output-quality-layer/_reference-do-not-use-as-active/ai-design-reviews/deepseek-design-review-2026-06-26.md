# 2.5 Output Quality Layer — 설계 리뷰 (DeepSeek Validator)

Status: external design review, 2026-06-26

이 문서는 `README.md`, `2.5-standalone-output-quality-layer-plan.md`,
`2.5-mdash-output-harness-plan.md`를 읽고 수행한 설계 리뷰다.

---

## 1. 문서 이해 요약

현재 설계는 **RisuAI용 독립형 출력 품질 파이프라인**을 구축하는 것이 목표다. 핵심 구조는:

- **입력 전처리 (Input Enhance MDASH)**: 유저 입력을 절대 재작성하지 않고, support-only guidance(의도 계약, 캐릭터 계약, 세계 계약, 장면 압력 등)를 생성하여 메인 모델에 보조 컨텍스트로 주입한다.
- **메인 LLM 드래프트**: RisuAI의 정상적인 프롬프트/컨텍스트 조립 후 메인 모델이 응답을 생성한다.
- **출력 검토 (Output Check MDASH)**: Character Reader, World Reader, Plot Reader, Continuity Reader, Secret Guard, Structure Guard가 드래프트를 검사하고 findings를 반환한다. 직접 응답을 재작성하지 않는다.
- **Table Read MDASH**: 고비용 모드에서 캐릭터들이 드래프트를 읽고 토론한다. 새 장면을 쓰는 것이 아니라, 이미 나온 draft를 검토한다.
- **출력 패치 (Output Enhance MDASH)**: 검토 findings를 bounded patch proposals로 변환하고, protected segment는 건드리지 않고 mutable prose만 교체한다.
- **Verifier**: 결정론적 JS 검사 + 선택적 LLM 보조 검증으로 패치 안전성을 확인한다. 실패 시 원문 반환.
- **실행 모드**: off → guidance_only → check_only → light_patch → single_panel → full_table_read 의 6단계. 사용자가 최대 모드를 선택하고 런타임이 상황에 따라 하향 조정할 수 있다.
- **아키텍처**: StandaloneOutputCore + RisuAIAdapter + ProviderRegistry + OptionalArchiveAdapter. Archive Center 연동은 후순위 읽기 전용 어댑터.

---

## 2. 현재 설계의 강점

| 강점 | 설명 |
|---|---|
| **Fail-open 일관성** | 모든 레이어에서 "실패 시 원문 반환"이 일관되게 적용됨. 보조 LLM 실패가 메인 출력 실패로 전파되지 않음. |
| **Protected segment 개념** | 이미지 태그, 상태창, Chatindex, 모듈 태그, JSON/code block을 protected로 분류하고 패치 대상에서 제외하는 설계가 명확함. |
| **Bounded patch 방식** | "전체 응답 재작성"이 아닌 segment_id 기반의 최소 교체. 이건 기존 multi-agent 예제들과의 결정적 차별점이다. |
| **모드 하향 조정 사다리** | off → guidance_only → check_only → light_patch → single_panel → full_table_read. 사용자 선택을 존중하면서도 스트리밍/비용/지연 시간에 따라 자동 하향 조정 가능. |
| **Contract 객체화** | User Intent Contract, Character Contract, World Contract, Plot Contract, Protected Output Map을 명시적 중간 표현으로 정의. LLM 호출의 입출력이 구조화됨. |
| **위성 플러그인 전략** (MDASH 문서) | 5개 독립 플러그인으로 분할하여 단일 거대 런타임화를 방지. 각 플러그인이 standalone/connected 이중 모드 지원. |
| **유저 입력 불변성** | "raw user input stays unchanged"가 입력 전처리의 제1원칙으로 명시됨. |
| **Table Read의 역할 경계** | "새 장면을 쓰는 기능이 아니라 이미 나온 draft를 검토하는 기능"이라고 명확히 경계 설정. |

---

## 3. 현재 설계의 약점

| 약점 | 심각도 | 설명 |
|---|---|---|
| **스트리밍 시나리오 설계 부재** | 🔴 Critical | "Streaming mode must degrade safely"라고만 언급되고 **어떻게** 감지하고 **어떻게** 하향 조정할지 구체적 메커니즘이 없음. RisuAI에서 스트리밍이 기본 모드인데, pre-display interception이 불가능할 때 audit/trace로 degrade한다는 언급만 있음. 이건 MVP에서 가장 큰 현실 충돌 지점. |
| **Protected segment 파싱 전략 미지정** | 🔴 Critical | 이미지 태그, 상태창, 모듈 태그를 "protected"로 분류한다고 하지만, **어떤 정규식/휴리스틱으로** 감지할지 전혀 명시되지 않음. 중첩 태그, malformed 태그, 커스텀 포맷에 대한 처리 전략 없음. |
| **OOC(Out-of-Character) 오탐 문제** | 🟠 High | Secret Guard가 "meta leakage"를 언급하지만, 모델이 출력 중간에 `(sorry, let me redo that)` 같은 OOC 텍스트를 삽입하는 문제는 별도 카테고리. Structure Guard나 별도 OOC Detector가 필요. |
| **Helper/submodel 요청 분류 기준 모호** | 🟠 High | "auxiliary request bypass"가 필요하다고 하지만, 어떤 요청이 auxiliary인지 판별하는 구체적 기준(프롬프트 패턴? depth? 특정 태그?)이 없음. 메인 RP 턴과 헬퍼 호출의 경계가 불분명하면 오작동 위험. |
| **Contract 추출의 신뢰성** | 🟠 High | Character Contract, World Contract 등을 LLM이 추출한다고 하지만, 캐릭터 카드/로어북에서 **환각 없이** 계약을 추출하는 프롬프트 엔지니어링이 명시되지 않음. 계약 추출 실패 시 파이프라인 전체가 무의미해짐. |
| **단일 모델 편향 (single_panel)** | 🟡 Medium | 하나의 모델이 Character Director, Plot Reader, World Reader 등을 모두 시뮬레이션할 때 동일 모델의 편향이 모든 역할에 복제됨. 이 위험에 대한 인식이나 완화책이 없음. |
| **비용/지연 시간 UX** | 🟡 Medium | full_table_read 모드에서 2C+7~9회 추가 호출. 사용자에게 이 지연을 어떻게 설명하고 진행 상황을 어떻게 표시할지 UX 설계 없음. |
| **비영어/혼합 언어 처리** | 🟡 Medium | 다국어 출력이나 한글+영문 혼합 시 segmenter, verifier가 어떻게 동작할지 언급 없음. |
| **Verifier의 JS/LLM 경계** | 🟡 Medium | "deterministic and optional model-assisted"라고 하지만, 어떤 검사가 JS로 가능하고 어떤 검사가 LLM을 필요로 하는지 구분이 없음. |
| **두 문서 간 아키텍처 긴장** | 🟡 Medium | standalone-plan은 단일 플러그인(StandaloneOutputCore)을, MDASH-plan은 5개 위성 플러그인을 제안. 어느 쪽이 MVP인지 결정되지 않음. |

---

## 4. MDASH 핵심 기능 반영 평가

| MDASH 개념 | 반영 상태 | 평가 |
|---|---|---|
| Input Enhance MDASH | ✅ 잘 반영 | support-only guidance, 유저 입력 불변 원칙 명확 |
| Output Check MDASH | ✅ 잘 반영 | 6개 Reader 역할, findings 반환, 직접 재작성 금지 |
| Table Read MDASH | ✅ 잘 반영 | 캐릭터 토론 모드, draft 검토 목적, 새 장면 작성 금지 |
| Output Enhance MDASH | ✅ 잘 반영 | bounded patch proposal, segment_id 기반 교체 |
| Verifier | ✅ 잘 반영 | veto 권한, protected segment 보호, 실패 시 원문 |
| Memory Read MDASH | ⬜ 의도적 제외 | Archive Center 연동 시 adapter로 후순위 |
| Memory Write MDASH | ⬜ 의도적 제외 | "Table Read는 메모리 저장/DB write 하지 않는다" 명시 |
| Committer | ⬜ 의도적 제외 | 후순위 |
| 위성 독립성 | 🟡 부분 반영 | standalone-plan은 단일 플러그인 지향, MDASH-plan은 5개 위성. **MVP에서 어느 쪽으로 갈지 결정 필요** |
| standalone/connected 이중 모드 | 🟡 부분 반영 | standalone-plan에서 OptionalArchiveAdapter로 언급되나 구체적 전환 메커니즘 부재 |

**총평**: MDASH의 기능적 본질(역할 분리 검토 → 구조화된 findings → bounded patch → verifier veto)은 standalone-plan에 충실히 반영되어 있다. 다만 위성 독립성과 이중 모드 전환은 두 문서 간에 해소되지 않은 긴장이 있다.

---

## 5. 기존 multi-agent / risu_agents / Serial Gradation 대비 차별점

| 비교 축 | 기존 예제들 | 2.5 설계 | 차별점 평가 |
|---|---|---|---|
| **출력 변경 방식** | afterRequest 전체 교체, unrestricted post-agent polish | bounded patch (segment_id + mutable only) | ✅ 강한 차별점 |
| **실패 처리** | agent 실패 시 빈 응답 또는 오류 | 항상 원문 반환 (fail-open) | ✅ 강한 차별점 |
| **메모리 권한** | plugin-local memory를 canonical truth로 취급 | 메모리 write 금지, Table Read는 read-only | ✅ 강한 차별점 |
| **역할 분리** | Serial Gradation에서 차용 가능 | 6개 Reader + Director + Guard로 체계화 | ✅ 확장된 차별점 |
| **구조적 출력 보호** | 없음 | Protected Output Map + Structure Guard | ✅ 독창적 차별점 |
| **계약 기반 검증** | 없음 | User Intent / Character / World / Plot Contract | ✅ 독창적 차별점 |
| **모드 하향 조정** | 없음 | 6단계 모드 + 자동 하향 | ✅ 독창적 차별점 |
| **프롬프트 엔지니어링** | 예제별 산발적 | Contract 컴파일러로 체계화 시도 | 🟡 설계는 좋으나 구현 구체성 부족 |

**결론**: 2.5 설계는 "에이전트를 더 많이 돌리는" 접근이 아니라 "구조화된 계약 → 검증 → 최소 패치 → 안전 장치"라는 **품질 파이프라인** 접근을 취한다는 점에서 기존 예제들과 본질적으로 다르다. 이 차별점은 문서에 잘 드러나 있다.

---

## 6. 반드시 추가할 문서 항목

### 6.1 `streaming-degradation-design.md` (신규 문서 제안)

```
# Streaming Degradation Design

## Problem Statement
RisuAI의 기본 출력 모드는 streaming이다. pre-display interception이
불가능할 때 Output Check / Table Read / Patch가 어떻게 동작해야 하는가?

## Detection
- RisuAI 스트리밍 모드 감지 방법 (plugin API / 설정 / 휴리스틱)
- 부분 청크만 도착한 상태에서 segmenter 동작 여부 판단

## Degradation Ladder for Streaming
- full_table_read → streaming 시 자동 off (비용+지연 과다)
- single_panel → streaming 시 check_only로 하향
- light_patch → streaming 시 check_only로 하향 (패치 불가)
- check_only → streaming 시 audit/trace 모드 (findings 기록만, 출력은 원문)
- guidance_only → streaming 시 정상 동작 (입력 전처리는 출력 전 완료)

## Honest UI
- "Streaming mode: output review limited to audit only"
- "Patch mode unavailable in streaming"
- 사용자에게 원문이 먼저 표시되고 사후 검토 결과가 별도 표시됨을 명시
```

### 6.2 `protected-segment-detection-spec.md` (신규 문서 제안)

```
# Protected Segment Detection Specification

## Detection Targets (우선순위 순)
1. Image generation tags: <image ...>, ![ ... ], <img ...>, data:image/...
2. Status windows: 【...】, [Status:...], <status ...>, ┌─...─┐ 블록
3. Chatindex / timestamp: [Chatindex:...], [Turn:...], ⏰...
4. Module tags: <module:...>, {{...}}, <lb-...>, <OtherAx:...>
5. JSON/code blocks: ```json ... ```, ```python ... ``` 등
6. Command-like blocks: /command ..., !command ...
7. User-requested fixed templates: 설정에서 지정된 regex 패턴

## Parsing Strategy
- 1차: 정규식 기반 라인 단위 분류 (빠름, JS only)
- 2차: 경계 모호한 세그먼트는 inspect-only로 보수적 분류
- 3차: 중첩 태그 처리 (가장 바깥쪽 태그 기준)

## Edge Cases
- Malformed tags: 보수적으로 protected 처리
- Inline tags within prose: 해당 paragraph 전체를 inspect-only로
- Multi-line protected blocks: 시작/끝 마커 기반
- Custom formats: plugin settings에서 사용자 정의 regex 허용
```

### 6.3 `ooc-and-meta-leakage-detection.md` (신규 문서 제안)

```
# OOC and Meta-Leakage Detection

## OOC (Out-of-Character) Text
- 패턴: ( ... ), [OOC: ...], (OOC: ...), *out of character: ...*
- 패턴: "sorry, let me redo", "let me try again", "as an AI..."
- 패턴: 메타 코멘트 ("I think this response is...", "that was a good try")
- 처리: OOC 세그먼트는 mutable로 분류, Secret Guard가 감지

## Meta-Leakage (비의도적 메타 정보 노출)
- 캐릭터가 알 수 없는 시스템 정보 언급
- 캐릭터가 자신의 "설정"을 인식하는 발언
- 4차 벽 붕괴 (캐릭터가 플레이어/사용자에게 직접 말함)
- 처리: Secret Guard findings → Patch Planner가 제거 제안

## False Positive Risk
- 의도적인 4차 벽 붕괴 (설정상 허용된 메타 캐릭터)
- 인용문 내 OOC-like 텍스트
- 해결: Character Contract에 "meta_allowed": true/false 필드 추가
```

### 6.4 `auxiliary-request-classification.md` (신규 문서 제안)

```
# Auxiliary Request Classification

## Classification Rules (우선순위 순)
1. 명시적 bypass marker: 설정에서 지정된 태그/prefix
2. Non-RP intent: "translate", "summarize", "explain this code", "what is..."
3. Module/helper calls: <lb-...>, <OtherAx:...>, Lightboard 연동
4. Image generation requests: "generate an image of...", <image prompt>
5. Memory/recall requests: "what do you remember about..."
6. System/configuration: "change setting", "enable mode"

## Classification Method
- 1단계: 빠른 키워드/패턴 매칭 (JS, 무비용)
- 2단계: 애매한 경우 Context Reader에게 분류 위임 (1회 LLM 호출)
- 3단계: 분류 불확실 시 보수적으로 quality layer 적용 (누락보다 과잉 적용이 안전)

## Bypass Behavior
- auxiliary로 분류된 요청: 전체 quality layer 우회
- 원본 요청/응답 그대로 통과
- RunLedger에 "bypass: auxiliary_request" 기록
```

### 6.5 기존 문서에 추가할 문단

`2.5-standalone-output-quality-layer-plan.md`의 **Safety Rules** 섹션에 추가:

```markdown
## Streaming Safety

When RisuAI is in streaming mode and pre-display interception is not available:

- `light_patch`, `single_panel`, `full_table_read` modes are automatically
  downgraded to `check_only`.
- `check_only` runs post-hoc: findings are recorded in trace but the original
  streamed output is not modified.
- The UI must show a clear indicator: "Streaming active — output review is
  audit-only. Switch to non-streaming for patch modes."
- If the provider supports non-streaming override, the plugin may force
  non-streaming when patch modes are requested, with user consent.

## Image Tag and Structure Preservation

The Structure Guard must detect and protect:

- Image generation tags: `<image ...>`, `![...](...)`, `<img ...>`, base64 data URIs
- Status windows: `【...】`, `[Status:...]`, box-drawing character blocks
- Chatindex/timestamp frames: `[Chatindex:...]`, `[Turn:...]`
- Module tags: `<module:...>`, `{{...}}`, `<lb-...>`, `<OtherAx:...>`
- JSON/code blocks: fenced code blocks (```)
- Command-like blocks: `/command ...`, `!command ...`
- User-defined regex patterns from plugin settings

Detection uses a layered approach:
1. Fast regex line classifier (JS, zero cost)
2. Ambiguous boundaries default to inspect-only (conservative)
3. Nested tags resolved by outermost marker

## OOC and Meta-Leakage Handling

The Secret Guard role must detect:

- OOC text: parenthetical asides `(...)`, `[OOC:...]`, self-corrections,
  AI-self-references
- Meta-leakage: characters referencing system concepts, settings awareness,
  unintended fourth-wall breaks
- False positive mitigation: Character Contract `meta_allowed` flag for
  settings where meta-awareness is intentional

## Helper/Submodel Interference Prevention

Auxiliary requests (translation, image generation, module calls, memory
queries, system commands) must bypass the quality layer entirely.

Classification uses:
1. Fast keyword/pattern match (JS)
2. Ambiguous cases delegated to Context Reader (1 LLM call)
3. Conservative default: when uncertain, apply quality layer (over-apply is
   safer than under-apply)
```

---

## 7. MVP 범위 제안

### MVP에 반드시 포함

| 항목 | 이유 |
|---|---|
| **Plugin shell + RunLedger** | 모든 것의 기반. 모드 표시, 호출 추적, 실패 기록. |
| **Auxiliary request bypass** | 없으면 헬퍼/번역/이미지 생성 호출이 품질 레이어를 통과하며 오작동. MVP 안정성의 전제조건. |
| **Protected segment parser** | Structure Guard의 핵심. 이미지 태그/상태창/모듈 태그 보호 없이는 어떤 패치도 위험. |
| **`guidance_only` 모드 (Input Enhance)** | 가장 안전한 모드. 출력을 건드리지 않고 입력 보조만 제공. MVP에서 가장 먼저 가치를 증명할 수 있는 기능. |
| **`check_only` 모드 (Output Check)** | findings 생성까지만. 패치 없이도 "무엇이 문제인지" 보여주는 것만으로 가치 있음. |
| **JS Verifier (deterministic only)** | protected segment 무결성, 길이 비율, 언어 변경 등 기본 검사. LLM 호출 없이 JS로만 동작. |
| **스트리밍 감지 + 자동 하향** | MVP 사용자의 90%+가 스트리밍 모드일 것. 이걸 처리하지 않으면 MVP가 현실에서 작동 안 함. |
| **Trace UI (모드/호출/하향 이유/verifier 결과)** | 디버깅과 신뢰 구축에 필수. 사용자가 "왜 패치가 적용 안 되었는지" 알 수 있어야 함. |

### 2.5 이후로 미룰 항목

| 항목 | 이유 |
|---|---|
| **`light_patch` 모드** | segmenter + verifier + patch planner가 모두 안정화된 후에야 안전. MVP에서는 check_only로 findings만 보여주는 것으로 충분. |
| **`single_panel` Table Read** | 2~3회 추가 LLM 호출. 비용/지연 설계가 선행되어야 함. |
| **`full_table_read`** | 2C+7~9회 호출. MVP 범위를 크게 벗어남. 중요 씬 전용이라는 전제와 맞음. |
| **LLM-assisted Verifier** | JS verifier만으로도 핵심 안전 검사 가능. LLM verifier는 신뢰성 검증 후 추가. |
| **Archive Center adapter** | standalone 동작이 증명된 후에나 의미 있음. |
| **Contract compiler (LLM 기반)** | MVP에서는 heuristic + lightweight LLM 추출로 충분. 완전한 contract 체계는 check_only가 안정화된 후. |
| **Character Contract per character** | MVP에서는 단일 "Character Director" 관점으로 충분. 개별 캐릭터 계약은 full_table_read와 함께. |

---

## 8. 수정된 구현 순서

```
Phase 1: Foundation (MVP)
─────────────────────────
1. Plugin shell + RunLedger + settings UI
   - 모드 선택, provider 설정, on/off 토글
   - RunLedger: requested mode, actual mode, calls, downgrade reason, verifier result

2. Auxiliary request classifier + bypass
   - 키워드/패턴 기반 빠른 분류 (JS)
   - bypass 시 원문 통과, RunLedger에 기록

3. Protected segment parser (Structure Guard)
   - 정규식 기반 라인 분류: protected / mutable / inspect-only
   - 이미지 태그, 상태창, Chatindex, 모듈 태그, JSON/code block, command block
   - 사용자 정의 regex 지원

4. Streaming detection + mode degradation
   - RisuAI 스트리밍 모드 감지
   - 자동 하향: patch 모드 → check_only, check_only → audit/trace
   - UI 표시: "Streaming active — patch unavailable"

5. guidance_only mode (Input Enhance MDASH)
   - Context Reader: 현재 채팅/캐릭터/로어북 읽기
   - Guidance Writer: support-only guidance 생성
   - 유저 입력 불변 보장

6. check_only mode (Output Check MDASH)
   - 6개 Reader 역할 (단일 LLM 호출로 통합 실행)
   - findings JSON 반환
   - OOC / meta-leakage 감지 포함

7. JS Verifier (deterministic)
   - protected segment 무결성 검사
   - 길이 비율 검사 (원문 대비 50% 미만 시 거부)
   - 언어/형식 변경 감지
   - 사용자 요청 콘텐츠 제거 감지

8. Trace UI
   - 모드, 호출 횟수, 하향 이유, findings 요약, verifier 결과
   - 실패 시 원문 반환 이유 표시

Phase 2: Enhancement (Post-MVP)
───────────────────────────────
9. light_patch mode
   - Patch Planner: findings → bounded patch proposals
   - Mutable segment에만 패치 적용
   - JS Verifier + optional LLM Verifier

10. single_panel Table Read
    - 단일 LLM이 Character Director + Plot Reader + World Reader +
      Continuity Reader + Secret Guard + Structure Guard 시뮬레이션
    - "simulated panel" 명시

11. Contract compiler 개선
    - User Intent / Character / World / Plot Contract를 구조화된 JSON으로 추출
    - LLM 기반 추출 + heuristic fallback

Phase 3: Advanced (Future)
───────────────────────────
12. full_table_read
    - Character Director + Individual Character Readers + cross-character round
    - 2C+3~9 호출, 중요 씬 전용

13. LLM-assisted Verifier
    - JS verifier 통과 후 의미적 검증 (unsourced facts, hidden memory leak)

14. Archive Center read-only adapter
    - OptionalArchiveAdapter: memory packet, entity memory, world rules
    - standalone 검증 완료 후에만 추가
```

---

## 9. 위험 설계 가정 및 해결 방안

### 9.1 스트리밍 문제

**위험**: "pre-display interception이 가능하다"는 암묵적 가정. RisuAI에서 스트리밍 시 afterRequest 훅이 호출되는 시점에는 이미 사용자가 부분 출력을 봤을 수 있음.

**문서화 제안**: `streaming-degradation-design.md` (위 6.1 참조)를 신규 문서로 추가하고, standalone-plan의 Safety Rules에 "Streaming Safety" 문단을 추가.

### 9.2 이미지 태그 / 상태창 / 모듈 태그 보호

**위험**: "protected segment" 개념은 있지만 실제 파싱 전략이 없음. 잘못된 정규식이 이미지 태그를 손상시키면 사용자 경험에 치명적.

**문서화 제안**: `protected-segment-detection-spec.md` (위 6.2 참조)를 신규 문서로 추가하고, standalone-plan에 "Image Tag and Structure Preservation" 문단을 추가.

### 9.3 OOC 오탐

**위험**: OOC 감지가 과도하면 의도된 메타 발언이 잘리고, 너무 느슨하면 진짜 OOC가 통과함. 특히 `(...)` 패턴은 정상적인 대사에도 등장할 수 있음 (예: "I think (no, I know) you're wrong").

**문서화 제안**: `ooc-and-meta-leakage-detection.md` (위 6.3 참조)를 신규 문서로 추가. Character Contract에 `meta_allowed` 필드를 두어 설정별로 OOC 허용 여부를 제어.

### 9.4 Helper/submodel 간섭

**위험**: Lightboard, OtherAx, 번역, 이미지 생성 등 보조 호출이 quality layer를 통과하면 의도치 않은 패치/거부가 발생할 수 있음. 반대로 메인 RP 턴을 auxiliary로 오분류하면 quality layer가 아예 동작하지 않음.

**문서화 제안**: `auxiliary-request-classification.md` (위 6.4 참조)를 신규 문서로 추가. 분류가 애매할 때는 "quality layer 적용" 쪽으로 보수적 결정을 내리는 원칙을 명시.

### 9.5 Contract 추출 신뢰성

**위험**: Character Contract, World Contract 등을 LLM이 추출할 때 환각이 발생하면, 그 환각에 기반해 출력을 "교정"하는 2차 피해가 발생할 수 있음.

**문서화 제안**: standalone-plan의 Contract Objects 섹션에 아래 문단 추가:

```markdown
## Contract Extraction Reliability

Contracts are support-only guidance. A hallucinated contract constraint is
less dangerous than a hallucinated patch, but it can still misdirect the main
model or the reviewers.

Mitigations:
- Contracts must cite source evidence (character card line, lorebook entry,
  recent chat line) when possible.
- The Verifier must cross-check patch justifications against the original
  draft, not against the contract alone.
- When contract confidence is low, the Guidance Writer should mark the
  contract field as `confidence: low` and reviewers should treat it as
  advisory, not binding.
```

### 9.6 단일 모델 편향 (single_panel)

**위험**: 하나의 LLM이 모든 Reader 역할을 수행하면, 그 모델의 고유한 맹점이 모든 역할에 복제됨.

**문서화 제안**: standalone-plan의 Execution Modes 섹션에:

```markdown
## Single-Panel Bias Risk

In `single_panel` mode, one model simulates all reader roles. This means the
model's inherent biases (genre preference, character favoritism, cultural
assumptions) affect every role simultaneously.

Mitigations:
- The trace must mark findings as `source: single_panel_simulated` to
  distinguish from independent multi-agent findings.
- `single_panel` confidence scores should be discounted by a configurable
  factor (default 0.8) before patching decisions.
- Critical findings (secret leak, world rule violation) should trigger a
  recommendation to upgrade to `full_table_read` for that turn.
```

---

## 10. 최종 권고

1. **MVP는 `guidance_only` + `check_only` + JS Verifier로 제한하라.** 패치 없이도 "무엇이 문제인지" 보여주는 것만으로 충분한 가치가 있다. `light_patch`는 segmenter와 verifier가 충분히 검증된 후에 추가하라.

2. **스트리밍 대응을 Phase 1에 포함하라.** RisuAI 사용자의 대부분이 스트리밍 모드다. 이걸 MVP에서 빠뜨리면 현실에서 아무도 쓸 수 없는 제품이 된다.

3. **두 문서의 아키텍처 긴장을 해소하라.** standalone-plan은 단일 플러그인, MDASH-plan은 5개 위성. MVP는 **단일 플러그인 + 내부 모듈 분리**로 시작하고, 위성 분리는 Post-MVP로 미루는 것이 현실적이다.

4. **Protected segment parser를 가장 먼저 구현하고 가장 많이 테스트하라.** 이 파서가 실수로 이미지 태그를 깨뜨리면 사용자의 신뢰를 단번에 잃는다. 다양한 RisuAI 출력 샘플을 수집해서 파서를 검증해야 한다.

5. **신규 문서 4건을 작성하라:** streaming-degradation-design.md, protected-segment-detection-spec.md, ooc-and-meta-leakage-detection.md, auxiliary-request-classification.md. 이 4건이 없으면 MVP 구현 중에 설계 공백으로 인한 재작업이 반복될 것이다.

6. **Contract 추출은 heuristic-first로 시작하라.** MVP에서는 LLM 기반 contract compiler 대신, 키워드/패턴 기반으로 빠르게 추출하고 부족한 부분만 LLM으로 보완하는 hybrid 접근이 더 안전하다.

7. **"실패 시 원문 반환"이 실제로 작동하는지 모든 오류 경로에서 테스트하라.** LLM 타임아웃, JSON 파싱 실패, segmenter 예외, verifier 거부 등 모든 실패 경로에서 원문이 소실되지 않는지 검증하는 테스트 스위트를 Phase 1에 포함하라.
