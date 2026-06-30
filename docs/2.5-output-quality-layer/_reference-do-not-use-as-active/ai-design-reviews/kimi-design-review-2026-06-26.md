# Kimi의 의견: 2.5 출력 품질 레이어 설계 리뷰

> 리뷰 대상 문서
> - `2.5-output-quality-layer/README.md`
> - `2.5-output-quality-layer/2.5-standalone-output-quality-layer-plan.md`
> - `2.5-output-quality-layer/2.5-mdash-output-harness-plan.md`
>
> 리뷰 기준: RisuAI용 독립형 출력 품질 레이어. Archive Center 연동은 후순위 선택 어댑터이며, 첫 구현은 Archive Center 없이 단독 작동해야 한다. 다만 MDASH의 기능적 장점은 포함해야 한다.

---

## 1. 문서 방향 요약

두 문서는 **RisuAI 플러그인 안에서 단독 작동하는 출력 품질 파이프라인**을 정의합니다.

- **핵심 목표**: Archive Center 백엔드 없이 RisuAI `beforeRequest`/`afterRequest` 훅만으로 동작하면서, MDASH의 "계약 기반 출력 검증 + 제한적 패치" 장점을 가져오는 것.
- **철학**: "더 많은 에이전트"가 아니라, **계약을 만들고 → 초안을 검증하고 → 안전한 가변 영역만 패치하고 → 증명되지 않으면 원문 그대로 fallback**하는 통제된 파이프라인.
- **3개 레인**:
  1. 입력 전처리 (pre-draft guidance)
  2. 출력 후검증/패치 (output check → mini table read → output enhance → verifier)
  3. 출력 자료 가공 (polish, material processing)
- **실행 모드**: `off` → `guidance_only` → `check_only` → `light_patch` → `single_panel` → `full_table_read`. 런타임에서 스트리밍/비용/지연/제공자 가용성에 따라 자동 다운그레이드.
- **fail-open**: 보조 LLM 실패, JSON 파싱 실패, 보호 마커 손상, 절단, 불안전 패치 시 무조건 원본 메인 LLM 출력 반환.

---

## 2. 강점

| 항목 | 근거 |
|------|------|
| **standalone-first 경계** | "첫 구현은 Archive Center 없이 단독 작동"이 명시되어 있어 제품 범위가 명확함. |
| **fail-open 계약** | 모든 보조 경로가 실패하면 원문 fallback. 이는 RisuAI 플러그인에서 필수적임. |
| **보호/가변/검사 전용 세그먼트 분리** | 이미지 태그, 상태창, Chatindex, 모듈 태그, 정규식 마커, 헤더는 보호; 산문/대화만 가변; 인용된 유저 텍스트/로그는 검사 전용. |
| **실행 모드 그라데이션** | 비용-품질 트레이드오프를 사용자가 선택할 수 있고, 런타임 다운그레이드 규칙도 있음. |
| **기존 구현 재활용 가능성** | `_dist/Archive Center.js`에 이미 `tryApplyTableReadOutputEnhance`, 세그먼트 가드, 패치 적용, 보호 봉투, 절단 가드가 구현되어 있음. |
| **차별점 명시** | `risu_agents.js`, `Serial_Gradation_Agents_for_RP.js`를 예시로 참조하면서도 "무제한 전체 출력 재작성"을 경계함. |

---

## 3. 약점 / 위험 / 미묘한 점

### 3.1 기존 구현과의 긴장

문서는 standalone-first를 외치지만, **실제 구현체 `_dist/Archive Center.js`는 Archive Center 백엔드에 강하게 결합**되어 있습니다.

- `tryApplyTableReadOutputEnhance`는 `/table-read/output-check`, `/table-read/mini-read`, `/table-read/output-enhance`를 `bridgeFetchWithRetry`로 호출.
- `loadTableReadPolishEntityBundles`는 `/subjective-entity-memories/entities`를 호출.
- Go 백엔드 `group_table_read.go`에 이미 전체 라우트가 구현됨.

**따라서 "Archive Center 없이 단독 작동"은 현재 코드베이스 기준으로는 아직 미구현 상태입니다.** 문서와 코드 사이의 간극이 큼.

### 3.2 스트리밍 / afterRequest 불확실성

`_dist/Archive Center.js`의 `onAfterRequest`는 이미 상당히 복잡합니다:

- native afterRequest vs synthetic afterRequest(storage poller 복구 경로) 구분.
- non-persistable fragment 처리, watcher, active chat fallback.
- `submodel`/`otherAx` 타입은 skip.

문서는 "스트리밍 문제"를 언급하지만, **afterRequest가 실제로 언제 호출되는지, fragment 단위로 들어올 때 어떻게 처리할지, 출력 품질 레이어를 적용할 타이밍이 명확하지 않습니다.** 특히 `displayContent`를 교체하는 시점과 persistence/save 시점의 차이가 중요한데, 문서에 그림이 없음.

### 3.3 보호 마커 계약의 모호함

문서는 "이미지 태그, 상태창, 모듈 태그 보호"를 요구하지만:

- **정확한 마커 목록**이 없음. RisuAI/사용자별로 `{{char}}`, `{{user}}`, `[System: ...]`, `<img ...>`, `## Status`, `{{#if ...}}` 등 다양함.
- **보호 봉투(envelope)의 식별자**가 충돌할 경우(예: 동일한 헤더가 본문에도 등장) 복원이 실패할 수 있음.
- **패치가 가변 영역을 벗어나 보호 영역을 건드렸을 때의 감지 기준**이 "정확히 일치"인지 "정규식 기반"인지 불분명.

### 3.4 OOC 오탐 / helper 간섭

- `onAfterRequest`는 이미 OOC turn을 save layer에서 skip.
- 하지만 **출력 품질 레이어가 OOC 출력을 "품질 낮음"으로 오판하여 불필요하게 패치하거나, 반대로 OOC 출력을 정상 narrative로 오판하여 보호 마커를 손상**할 가능성이 있음.
- `submodel`/`otherAx` skip 로직은 있지만, **helper 호출이 main context 없이 afterRequest에 들어올 때**(`auxiliaryTypedWithoutMainContext`) 출력 품질 레이어가 어떻게 동작해야 하는지 문서에 없음.

### 3.5 계약 객체의 실제 JSON 스키마 부재

문서는 "contract objects"를 언급하지만, **input contract, output contract, patch contract, verifier contract의 정확한 필드, 타입, 필수/선택이 정의되어 있지 않음.** 구현 시 각 LLM 프롬프트와 파서가 달라질 수 있음.

### 3.6 설정/LLM 구성 중복

`_dist/Archive Center.js`에는 이미 `pluginMainLlm`, `subLlm` 설정이 있고, Table Read용 LLM 설정(`tableReadLLMConfigured`)도 별도로 존재. standalone 출력 품질 레이어가 **기존 설정을 재사용할지, 새 설정을 둘지, 아니면 둘 다 지원할지** 문서가 결정하지 않음.

---

## 4. 반드시 문서에 추가해야 할 항목

### 4.1 Standalone 모드에서의 LLM 호출 경로 명세

- RisuAI 플러그인은 CORS 제약으로 인해 직접 OpenAI API를 호출하기 어려움. `_dist/Archive Center.js`의 `callPluginMainLlm`은 `/proxy/plugin-main` 백엔드 경유.
- **standalone 모드에서 보조 LLM을 어떻게 호출할 것인가?**
  - RisuAI 내장 LLM 호출 API가 있는가?
  - 사용자가 별도 proxy 서버를 띄워야 하는가?
  - 아니면 RisuAI의 `fetch`를 그대로 쓰는가?
- **이것이 설계의 가장 큰 미결정사항입니다.** 문서에 명확히 있어야 함.

### 4.2 afterRequest / 스트리밍 처리 시퀀스 다이어그램

- native afterRequest 호출 시점.
- synthetic afterRequest(storage poller) 호출 시점.
- fragment/non-persistable 처리 후 최종 출력이 확정된 시점.
- **출력 품질 레이어가 적용되는 단일 진입점**: `displayContent` 확정 직후, persistence 이전.
- 스트리밍 중에는 `afterRequest`가 여러 번 호출될 수 있음. 어떤 조건에서만 적용할지.

### 4.3 보호 마커 정의서

| 카테고리 | 예시 | 처리 규칙 |
|----------|------|----------|
| 이미지 태그 | `<img ...>`, `![...](...)` | 절대 수정/이동 금지 |
| 상태창 | `## Status`, `{{#if status}}...{{/if}}` | 절대 수정 금지 |
| 모듈 태그 | `{{module:...}}`, `[Module: ...]` | 절대 수정 금지 |
| Chatindex | `{{chat_index}}`, `{{len}}` | 절대 수정 금지 |
| 정규식 마커 | `{{regex::...}}` | 절대 수정 금지 |
| 헤더 | `## ...`, `---` 구분선 | 보호 또는 가변 정책 명시 |
| 인용 유저 텍스트 | `> ...` | 검사 전용, 수정 금지 |
| 로그/시스템 출력 | `[System: ...]` | 검사 전용 |

### 4.4 Patch Contract JSON Schema

```json
{
  "patch_id": "string",
  "target_segment_index": "number",
  "target_segment_type": "prose|dialogue",
  "operation": "replace|insert|delete",
  "old_text": "string (exact or regex)",
  "new_text": "string",
  "reason": "string",
  "confidence": "number 0-1"
}
```

- `old_text`가 exact match 실패 시 패치 거부.
- segment index가 가변 영역을 벗어나면 거부.
- protected envelope 복원 후 보호 마커가 변형되면 전체 결과 폐기.

### 4.5 Verifier Contract

```json
{
  "verdict": "accept|reject|retry",
  "reject_reasons": ["string"],
  "protected_marker_drift": "boolean",
  "truncation_detected": "boolean",
  "semantic_fidelity_score": "number 0-1"
}
```

### 4.6 OOC / Helper / Submodel 상호작용 규칙

- OOC turn: 출력 품질 레이어를 아예 skip.
- `submodel`/`otherAx`: skip.
- `syntheticAfterRequest`: 적용 가능하나, trace에 `syntheticAfterRequest: true` 기록.
- main context 없는 auxiliary afterRequest: skip.

### 4.7 설정 통합/분리 정책

- standalone 출력 품질 레이어가 사용할 LLM 설정의 이름과 위치.
- 기존 `pluginMainLlm`/`subLlm`과의 관계.
- Table Read LLM 설정과의 관계.

---

## 5. MVP 범위 (첫 구현에 반드시 들어갈 것)

1. **단일 afterRequest 진입점**: `Risuai.addRisuReplacer("afterRequest", ...)`에 연결.
2. **보호/가변 세그먼트 분리**: 최소한 이미지 태그, 상태창, 모듈 태그, Chatindex, 정규식 마커를 보호.
3. **check_only 모드**: LLM이 출력을 검토하고 문제점만 JSON으로 반환. 패치는 하지 않음.
4. **light_patch 모드**: JSON patch contract로 제한된 가변 영역만 교체.
5. **fail-open**: 보호 마커 손상, JSON 파싱 실패, LLM 호출 실패, 절단 감지 시 원문 반환.
6. **trace 기록**: 적용 여부, 거부 사유, 실행 모드, 소요 시간, LLM 사용량.
7. **설정 UI on/off 스위치**: 최소한 활성화/비활성화, 실행 모드 선택, LLM 설정.

---

## 6. 미뤄야 할 항목

1. **full_table_read / multi-panel table read**: 비용과 복잡도가 높음. MVP 후.
2. **입력 전처리(pre-draft guidance)**: `beforeRequest` 개입은 현재 `pluginMain` first-pass/second-pass가 이미 존재. 충돌 가능성.
3. **출력 자료 가공(polish, material processing)**: 별도 제품 영역.
4. **Archive Center 연동 어댑터**: 문서 자체가 "후순위 선택 어댑터"라고 명시.
5. **자동 다운그레이드의 모든 조건**: MVP에서는 수동 모드 선택 + 간단한 timeout/cost 기반 다운그레이드만.
6. **다국어 prompt 최적화**: 영어/한국어 먼저, 일본어 등은 이후.
7. **verifier를 별도 LLM 호출**: MVP에서는 rule-based verifier + lightweight LLM verifier(선택).

---

## 7. 수정된 구현 순서

| 단계 | 작업 | 산출물 |
|------|------|--------|
| 1 | **Standalone LLM 호출 경로 확정** | 문서 결정: RisuAI 내장 API vs 사용자 proxy vs 백엔드 필수 |
| 2 | 보호 마커 정의서 작성 | `docs/2.5-output-quality-layer/protected-markers.md` |
| 3 | afterRequest 진입점 및 세그먼트 분리 구현 | `src/standalone-output-quality/segment-guard.js` |
| 4 | check_only 모드 + output-check prompt 구현 | `src/standalone-output-quality/output-check.js` |
| 5 | light_patch 모드 + patch contract + 적용 구현 | `src/standalone-output-quality/patch-applier.js` |
| 6 | fail-open / verifier / trace 통합 | `src/standalone-output-quality/verifier.js` |
| 7 | 설정 UI 및 on/off 스위치 | `src/standalone-output-quality/settings.js` |
| 8 | 회귀 테스트: 보호 마커, OOC, helper skip, 절단 | `tests/standalone-output-quality/` |
| 9 | (후순위) Archive Center 어댑터 | `src/standalone-output-quality/archive-center-adapter.js` |
| 10 | (후순위) full_table_read / 입력 전처리 | 별도 문서화 후 구현 |

---

## 8. 최종 권고

### go/no-go

**Go — 단, 조건부.**

설계 방향은 타당하고, `_dist/Archive Center.js`에 이미 검증된 세그먼트 가드/패치 로직이 있어 기술적 위험은 낮습니다. 다만 **"standalone-first"를 진심으로 하려면 현재 Archive Center-coupled 코드를 재사용하지 말고 새 위성 플러그인 또는 별도 모듈로 시작**해야 합니다.

### 우선 수정할 문서

1. **`2.5-standalone-output-quality-layer-plan.md`에 standalone LLM 호출 경로 결정 추가** — 이것이 모든 설계의 전제.
2. **`2.5-standalone-output-quality-layer-plan.md`에 보호 마커 정의서 추가**.
3. **`2.5-mdash-output-harness-plan.md`에 afterRequest/스트리밍 시퀀스 다이어그램 추가**.
4. **새 문서 `2.5-output-quality-layer/contracts.md` 작성**: output-check, patch, verifier JSON schema.
5. **새 문서 `2.5-output-quality-layer/risuai-integration.md` 작성**: `beforeRequest`/`afterRequest` 진입점, OOC/helper/submodel skip 규칙.

### 구현 전략 권고

- **기존 `_dist/Archive Center.js`의 Table Read 로직을 복사-붙여넣기하지 말고**, 그 개념만 참조하여 새 파일에서 재구현.
- **MVP는 `afterRequest` output check + light patch만**.
- **Archive Center 연동은 어댑터 인터페이스만 정의해 두고 구현은 나중에**.

---

*작성: Kimi (GitHub Copilot), 2026-06-26*
