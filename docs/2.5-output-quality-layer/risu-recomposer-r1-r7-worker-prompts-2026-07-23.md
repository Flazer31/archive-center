# Risu Recomposer R1-R7 구현 인수인계

Date: 2026-07-23

Status: superseded implementation handoff

Current authority: `risu-recomposer-mdash-execution-plan-2026-07-24.md`.
R1-R5 were used to reach the current runtime baseline. Do not reuse this
document's old baseline, 240 KB limit, or R6-R7 sequence for new implementation
work. The current one-file hard limit is 500 KB.

Target runtime:

- `source/Risu Recomposer.js`
- current baseline: `0.1.7`
- current size at handoff: about 177 KB / 3,687 lines

## 1. 목표

RisuAI 메인 LLM의 출력을 `draft_zero`로 받아 여러 전문 AI가 역할별 재작성
후보를 만들고, Fusion Director와 Whole-Scene Composer가 후보를 결합하여 실제
최종 출력문을 개선한다.

```text
RisuAI draft_zero
-> protected / inspect_only / mutable segmentation
-> MDASH specialist rewrites
-> semantic Fusion Director
-> Whole-Scene Composer
-> structural JS Verifier
-> enhanced output return
```

이 플러그인은 검토 보고서나 조언을 남기는 도구가 아니다. 정상적인 성공 경로에서는
전문 역할이 재작성 후보를 만들고, Composer 또는 최고 후보가 실제 반환 출력에
적용되어야 한다.

## 2. 현재 구현 기준선

현재 전문 역할:

- `secret_pov_guard`
- `character_reader`
- `plot_continuity_reader`
- `world_reader`
- `style_reader`
- `agency_meta_guard`
- `whole_scene_composer`

현재 프리셋:

- `fast`: 전문 역할 2개 + Composer
- `balanced`: 전문 역할 4개 + Composer
- `quality`: 전문 역할 6개 + Composer

현재 핵심 함수:

- 저장: `loadSettings`, `saveSettings`, `storageGet`, `storageSet`
- 분할: `buildSegmentMap`, `assembleOutput`, `verifyOutput`
- 문맥: `collectContext`, `extractLorebookSummary`, `extractMemorySummary`
- 호출: `callProvider`, `callOllama`, `callAnthropic`, `callGemini`,
  `callVertex`, `callCustom`, `callRole`
- schema: `validateCandidateSchema`, `validateComposerSchema`
- 라우터: `detectSceneSignals`, `selectRoles`, `SIGNAL_ROLE_MAP`
- Fusion: `fusionDirector`
- Composer: `runComposer`
- 스케줄러: `scheduleRoles`
- Trace: `newTrace`, `traceTimeline`, `traceError`, `saveTrace`,
  `refreshTraceList`
- UI: `renderUI`, `openSettingsUI`, `collectSettingsFromUI`

현재 인메모리 테스트는 36개다. 기존 테스트를 삭제하거나 약화해서 통과시키면 안 된다.

## 3. 절대 제약

1. 런타임 수정 대상은 `source/Risu Recomposer.js` 한 파일뿐이다.
2. 새 런타임 파일, 외부 패키지, 빌드 단계, 백엔드 서버를 추가하지 않는다.
3. Archive Center, MariaDB, ChromaDB 연동은 이번 범위에서 구현하지 않는다.
4. Table Read와 인물별 주관 기억은 이번 범위에서 구현하지 않는다.
5. 현재 6개 전문 역할과 Composer 외에 새 AI 역할을 추가하지 않는다.
6. 예제 플러그인의 코드, 프롬프트, 상수, schema를 복사하지 않는다.
7. 특정 세션의 인물명, 비밀 설정, 문장, 세계관을 하드코딩하지 않는다.
8. `RisuAI.pluginStorage` 저장 경로와 API Key 유지/교체/삭제 계약을 보존한다.
9. API Key 원문을 DOM, placeholder, Trace, 오류 문자열에 기록하지 않는다.
10. protected 및 inspect-only segment는 byte-for-byte 보존한다.
11. 성공한 재작성 후보를 단순 findings나 조언으로만 남기는 경로를 만들지 않는다.
12. 실패하지 않은 후보를 막연한 “안전” 또는 “보수적” 판단으로 원문 반환하지 않는다.
13. 원문 반환은 호출 실패, 후보 없음, deadline, schema 불능, 구조 검증 실패 같은
    명시적 기술 사유로만 허용한다.
14. 새 guard, fallback, cache, watcher를 병렬로 덧붙이기 전에 기존 경로를 고친다.
15. 대규모 포맷 변경을 하지 않는다.
16. 총 파일 크기는 목표 220 KB 이하, 하드 상한 240 KB다. 상한을 넘기려면 작업을
    중지하고 추가 코드가 반드시 필요한 이유를 보고한다.
17. 각 단계의 순증은 원칙적으로 250줄 이하다. 기존 함수를 교체·단순화하는 방식을
    우선한다.

## 4. 공통 검증 명령

작업 PC에 설치된 번들 Node를 사용한다.

```powershell
& 'C:\Users\com12\.cache\codex-runtimes\codex-primary-runtime\dependencies\node\bin\node.exe' --check 'source\Risu Recomposer.js'
```

인메모리 테스트:

```powershell
& 'C:\Users\com12\.cache\codex-runtimes\codex-primary-runtime\dependencies\node\bin\node.exe' -e "require('./source/Risu Recomposer.js'); setTimeout(async()=>{const r=await globalThis.__recomposer.runInMemoryTests(); if(r.failed){process.exitCode=1;}},100);"
```

각 단계 완료 보고에 반드시 포함할 것:

- 변경한 함수 목록
- 추가한 schema 필드
- 새 테스트 이름과 결과
- 전체 테스트 통과 수
- 문법 검사 결과
- 변경 전후 파일 bytes 및 lines
- JavaScript lines added / removed
- 한 턴 최대 specialist 호출 수와 Composer 호출 수
- 원문 반환이 가능한 모든 사유
- API Key 관련 함수가 변경되었는지 여부

## 5. R1 - Semantic Fusion Director

### 목적

현재 `fusionDirector`는 여러 역할이 후보를 냈다는 사실과 후보 길이·단어 유사도를
주로 사용한다. 이것을 실제 문제 단위의 consensus, complementary improvement,
conflict, gap으로 바꾼다.

### 구현 요구

1. 후보 schema에 최소한 다음 의미 정보를 추가한다.
   - `issues`: 이 후보가 해결하는 정규화된 문제군 배열
   - `change_summary`: 무엇을 바꿨는지 짧은 설명
2. 허용 문제군은 지나치게 세분화하지 않는다.
   - `secret_leak`
   - `pov_violation`
   - `identity_continuity`
   - `character_voice`
   - `emotion`
   - `plot_continuity`
   - `scene_logic`
   - `world_rule`
   - `agency_takeover`
   - `meta_artifact`
   - `repetition`
   - `rhythm`
   - `transition`
   - `prose_clarity`
3. 기존 저장 프롬프트가 이전 schema를 반환해도 `tags`로부터 문제군을 유도하여
   후보를 버리지 않는다.
4. consensus는 같은 segment에서 겹치는 문제군을 서로 다른 역할이 발견했을 때만
   인정한다.
5. 서로 다른 문제군을 고치는 후보는 conflict가 아니라 complementary로 기록한다.
6. 같은 문제군인데 rewrite의 의미·길이·방향이 크게 다르면 conflict로 기록한다.
7. 후보 수 자체를 consensus로 계산하지 않는다.
8. Director 점수는 다음을 반영한다.
   - 역할 우선순위
   - 후보 confidence
   - 문제군 우선순위
   - 실제 consensus bonus
   - complementary bonus
   - 같은 문제군 conflict penalty
   - 원문과 사실상 같은 후보 penalty
   - 극단적인 길이 변화 penalty
9. `secret_leak`, `pov_violation`, `agency_takeover`, `plot_continuity`는 문체-only
   후보보다 높은 우선순위를 유지한다.
10. Director는 후보를 평가하지만, 정상 후보를 trace-only 조언으로 강등하는 별도
    경로를 만들지 않는다.

### 필수 테스트

- 같은 segment, 같은 issue, 다른 역할 2개 -> true consensus
- 같은 segment, 서로 다른 issue -> complementary, conflict 아님
- 같은 issue, 상반된 rewrite -> conflict
- 후보 여러 개지만 issue가 다름 -> false consensus
- 이전 schema의 `tags`만 있는 후보 -> 정상 정규화
- secret/POV 후보와 style 후보 충돌 -> secret/POV 우선
- 원문과 동일한 rewrite -> 낮은 점수
- 정상 후보가 `directorResult.ranked`에 보존

### 작업 프롬프트

```text
source/docs/2.5-output-quality-layer/risu-recomposer-r1-r7-worker-prompts-2026-07-23.md를 읽고 R1만 구현하라.

수정 대상은 source/Risu Recomposer.js 한 파일이다. 현재 fusionDirector,
validateCandidateSchema, buildRolePrompt, DEFAULT_ROLES의 기본 프롬프트와 관련
테스트만 필요한 범위에서 고쳐라.

후보 수를 합의로 오인하지 말고, segment + normalized issue를 기준으로 실제
consensus/complementary/conflict/gap을 계산하라. 기존 tags-only 응답은 유실 없이
정규화하라. 정상 후보는 실제 Composer 입력까지 전달되어야 한다.

R2 이후 작업, UI 재설계, Table Read, Archive Center 연동은 하지 마라. 완료 후 공통
보고 형식으로 결과를 제출하고 멈춰라.
```

## 6. R2 - Language-Independent Fugu Router

### 목적

현재 `detectSceneSignals`가 영어 단어 출현에 크게 의존한다. 한국어·일본어·혼합
언어 RP에서도 안정적으로 역할을 선택하고, 핵심 역할이 우연한 키워드 때문에
탈락하지 않도록 고친다.

### 구현 요구

1. `fast`, `balanced`, `quality`의 기본 역할 계약을 명확히 유지한다.
2. `balanced`의 핵심 역할은 신호 하나가 감지됐다는 이유로 탈락하지 않는다.
3. `quality`는 활성화되고 설정이 완성된 6개 전문 역할을 모두 실행한다.
4. Fugu식 신호는 역할 삭제보다 우선순위 조정과 조건부 역할 추가에 사용한다.
5. 구조 기반 신호를 우선한다.
   - 대화 비율
   - mutable segment 수와 길이
   - 문단 및 문장 반복
   - context/lorebook/memory 가용성
   - 인용부호와 화자 전환
   - 메타/목록형 구조
6. 언어별 표현을 사용할 경우 한국어와 영어를 최소한 동등하게 취급하고, 특정
   작품 문구를 하드코딩하지 않는다.
7. 선택된 역할과 선택 이유를 Trace에 기록한다.
8. Provider 설정이 불완전한 역할은 `not_configured`로 명확히 기록한다.
9. Quality에서 역할 수를 줄여 비용을 절감하려 하지 않는다. 비용 절감은 Fast와
   Balanced의 책임이다.

### 필수 테스트

- 동일 의미의 한국어/영어 장면에서 핵심 역할 집합이 비정상적으로 달라지지 않음
- Balanced에서 하나의 신호가 감지되어도 핵심 역할 유지
- Quality에서 설정된 전문 역할 6개 + Composer 선택
- Fast에서 정해진 핵심 역할 + Composer만 선택
- 비활성 역할 제외
- 설정 불완전 역할의 skip reason 기록
- router reason이 Trace에 남음

### 작업 프롬프트

```text
R1 검증이 끝난 source/Risu Recomposer.js를 기준으로 인수인계 문서의 R2만 구현하라.

detectSceneSignals, SIGNAL_ROLE_MAP, selectRoles와 관련 Trace/테스트만 수정하라.
Balanced와 Quality의 핵심 역할을 우연한 영어 키워드가 제거하지 못하게 하라.
Quality는 설정된 전문 역할 전부를 사용하고, Fugu 신호는 우선순위와 조건부 추가에
사용하라.

R3 이후 작업이나 새 역할 추가는 하지 마라. 완료 후 선택 역할, skip reason,
한국어/영어 테스트 결과를 포함해 보고하고 멈춰라.
```

## 7. R3 - Composer Application Contract

### 목적

Whole-Scene Composer가 후보를 단순 선택하는 대신 서로 다른 역할의 강점을 실제
장면 문장으로 합성하고, 모든 mutable segment의 처리 결과를 명확히 남기게 한다.

### 구현 요구

1. Composer 입력에 다음을 명확히 포함한다.
   - segment 원문
   - 후보별 role, issues, confidence, Director score
   - consensus, complementary, conflict, gap
   - read-only runtime context
2. Composer 응답은 허용된 mutable segment ID만 사용할 수 있다.
3. unknown ID, protected ID, inspect-only ID는 schema 단계에서 거부한다.
4. Composer가 일부 mutable segment만 반환하면:
   - 반환된 segment는 Composer 결과 사용
   - 누락된 segment는 Director 최고 후보 사용
   - 후보도 없으면 원문 사용
5. Composer가 성공했어도 원문과 동일한 segment는 `unchanged`로 기록한다.
6. Composer 전체 실패 때문에 성공한 specialist 후보를 버리지 않는다.
7. 문단 사이 연결을 개선할 수 있도록 전체 ordered segment 목록을 유지한다.
8. 직접 protected/inspect-only 내용을 생성하거나 수정하지 않는다.
9. `assembleOutput`은 최종 segment별 source를 `composer`, `top_candidate`,
   `original`로 정확히 남긴다.

### 필수 테스트

- Composer가 모든 mutable segment 재작성
- Composer partial response + 최고 후보 fallback
- Composer 실패 + specialist 후보 적용
- Composer unknown segment ID 거부
- Composer protected/inspect-only ID 거부
- Composer 동일문 반환 -> unchanged
- 여러 문제군 후보가 하나의 segment로 합성
- 최종 출력이 실제 draft_zero와 다름

### 작업 프롬프트

```text
R1과 R2가 검증된 source/Risu Recomposer.js를 기준으로 인수인계 문서의 R3만
구현하라.

buildRolePrompt, validateComposerSchema, runComposer, assembleOutput,
scheduleRoles와 직접 관련된 테스트만 수정하라. Composer partial/failure 시 성공한
specialist 후보를 실제 출력에 적용하라. unknown/protected/inspect-only ID는
거부하라.

조언이나 findings만 남기고 원문을 반환하는 성공 경로를 만들지 마라. R4 이후 작업은
하지 말고 공통 형식으로 보고한 뒤 멈춰라.
```

## 8. R4 - Applied Output Evidence Trace

### 목적

여러 AI를 호출한 결과가 실제 출력에 적용됐는지 Trace에서 즉시 확인한다.

### 구현 요구

1. 역할 호출 timeline:
   - role
   - provider/model
   - queued/start/end
   - elapsed
   - fulfilled/failed
   - retry/fallback
   - candidate count
2. Director evidence:
   - segment
   - issue groups
   - consensus/complementary/conflict
   - 후보 점수
   - 선택 후보와 선택 이유
3. Applied evidence:
   - segment ID
   - source role 또는 Composer
   - original preview
   - final preview
   - changed 여부
4. 최종 요약:
   - specialist calls
   - successful roles
   - candidate count
   - Composer state
   - changed segment count
   - final state와 reason
5. Trace 크기는 제한한다. 전체 원문·전체 프롬프트·전체 context를 중복 저장하지
   않는다.
6. API Key, Authorization header, extra header의 비밀값을 Trace에 기록하지 않는다.
7. UI는 기존 Trace 탭 안에서 정보를 읽기 쉽게 표시하되 전체 UI 재설계는 하지 않는다.

### 필수 테스트

- 성공 역할 timeline
- 실패 역할 timeline
- retry/fallback 표시
- candidate count
- Composer 적용 segment evidence
- top candidate 적용 segment evidence
- original 반환 reason
- Trace JSON에 API Key 원문 없음
- 긴 원문에서도 Trace 크기 상한 유지

### 작업 프롬프트

```text
R1-R3 검증이 끝난 source/Risu Recomposer.js를 기준으로 인수인계 문서의 R4만
구현하라.

newTrace, callRole, scheduleRoles, assembleOutput, onAfterRequest,
refreshTraceList와 관련 테스트만 수정하라. Trace의 중심은 “무슨 AI가 무엇을
제안했나”보다 “어떤 후보가 실제 어느 segment를 바꿨나”다.

원문, prompt, context 전체를 Trace에 복제하지 말고 preview와 집계만 저장하라.
API Key와 인증 header는 절대 기록하지 마라. R5 이후 작업은 하지 말고 보고 후
멈춰라.
```

## 9. R5 - Provider Reliability Gate

### 목적

Provider별 요청 형식, JSON 응답, reasoning, timeout, fallback, abort가 실제
파이프라인을 멈추지 않도록 정리한다.

### 참고 문서

- `source/docs/provider-request-overrides-flex-paygo-contract.md`

### 구현 요구

1. 기존 단일 `callRole -> callProvider` 경로를 유지한다.
2. Provider별 body와 응답 파싱을 명확히 분리한다.
3. Ollama local과 Ollama Cloud/OpenAI-compatible endpoint를 구분한다.
4. Anthropic, Gemini, Vertex의 인증 방식과 JSON 응답 위치를 혼합하지 않는다.
5. Vertex Flex는 Vertex에서만 적용한다.
6. reasoning adapter는 지원 Provider/model에만 필드를 붙인다.
7. JSON repair retry는 최대 1회 유지한다.
8. fallback은 최대 1회 유지한다.
9. timeout/abort 후 background 호출이 남지 않아야 한다.
10. 같은 Provider semaphore와 전체 max parallel을 모두 지킨다.
11. 한 역할 실패가 다른 역할 후보와 Composer fallback 경로를 제거하지 않는다.
12. 공식 계약에서 확인되지 않은 필드를 추측해서 추가하지 않는다.

### 필수 테스트

- OpenAI-compatible request/response mock
- Ollama local request/response mock
- Ollama Cloud request/response mock
- Anthropic request/response mock
- Gemini request/response mock
- Vertex request/response mock
- Vertex Flex가 Vertex에서만 적용
- reasoning-only content 분류
- invalid JSON -> 1회 repair retry
- primary 실패 -> fallback 1회
- deadline abort -> active call 0
- same provider concurrency 준수
- API Key와 Authorization 비밀값 Trace 미노출

### 작업 프롬프트

```text
R1-R4 검증이 끝난 source/Risu Recomposer.js와
source/docs/provider-request-overrides-flex-paygo-contract.md를 읽고 R5만 구현하라.

callProvider, 각 Provider caller, applyReasoningAdapter, fetchWithAbort,
callRole, semaphore/deadline과 관련 테스트만 수정하라. 실제 네트워크를 호출하지
말고 mock fetch로 요청 URL, header, body, 응답 파싱을 검증하라.

지원이 확인되지 않은 Provider field를 추측하지 마라. Archive Center 연동이나 R6
작업은 하지 말고 보고 후 멈춰라.
```

## 10. R6 - Connected Three-Turn Live Validation

R6는 주로 사용자와 Codex가 RisuAI에서 수행하는 실사용 검증이다. 구현 worker는
R1-R5 완료 상태에서 테스트 항목과 Trace 판독을 지원하되, 성공 증거 없이 코드를
추가하지 않는다.

### 실행 순서

1. 연결된 RP 장면 1턴: `balanced`
2. 연결된 RP 장면 2턴: `quality`
3. 연결된 RP 장면 3턴: `quality`

같은 장면을 삭제·재생성할 필요는 없다. 자연스럽게 이어지는 세 턴을 사용한다.

### 각 턴에서 수집할 것

- RisuAI 원래 초안 또는 Trace의 original preview
- 실제 반환된 최종 출력
- 전체 Trace JSON
- 선택된 역할
- provider/model/elapsed
- 후보 수
- Composer 상태
- changed segment 수
- final reason

### 판정 기준

- 캐릭터 말투와 감정 반응이 더 구체적인가
- 비밀/POV/정체성 정보가 현재 화자의 지식 범위를 넘지 않는가
- 사용자 agency를 대신 결정하지 않는가
- 이전 턴과 사건·물체·위치·관계가 이어지는가
- 반복, 기계적 요약, 번역체, 상투적 마무리가 줄었는가
- 이미지 태그, 상태창, 코드, JSON, RisuAI marker가 보존되는가
- Trace에서 실제 변경 segment와 적용 source를 확인할 수 있는가
- deadline 이후 무한 로딩이나 background 호출이 없는가

### 실패 분류

- `host_integration`
- `provider_request`
- `provider_response_schema`
- `router_selection`
- `candidate_generation`
- `fusion_ranking`
- `composer_integration`
- `verifier_rejection`
- `trace_evidence`
- `quality_regression`

실패 원인을 분류한 뒤 해당 owner 함수만 수정한다. 새 우회 경로를 추가하지 않는다.

### 검증 요청 프롬프트

```text
첨부한 세 턴의 원문/최종 출력/Trace JSON을 읽고 R6 판정만 수행하라.

각 턴별로 실제 변경 segment, 적용 source, 역할 호출, 후보 수, Composer 결과,
verifier 결과를 표로 정리하라. 개선이 없으면 모델 탓으로 추측하지 말고
router_selection, candidate_generation, fusion_ranking, composer_integration 중
어느 단계에서 막혔는지 Trace 증거로 분류하라.

코드는 수정하지 말고, 수정이 필요한 정확한 함수와 재현 조건만 보고하라.
```

## 11. R7 - Standalone Beta Gate

R7은 R6가 통과한 후에만 진행한다.

### Beta 조건

1. UI 설정 저장/재로딩이 RisuAI 실환경에서 확인됨
2. Balanced와 Quality 모두 실제 enhanced output을 반환한 턴이 있음
3. 한 역할 실패 후 다른 후보가 적용되는 실증이 있음
4. Provider timeout 후 다음 턴 진행 가능
5. protected/inspect-only 손상 0건
6. API Key 평문 노출 0건
7. Trace에서 실제 변경 증거 확인 가능
8. 인메모리 테스트 전체 통과
9. 문법 검사 통과
10. 파일 크기 240 KB 이하

### Beta 정리 작업

- 버전을 다음 beta 번호로 갱신
- UI에서 프리셋 설명을 짧고 명확하게 표시
- 기본값은 `balanced`
- Debug 전용 정보는 Trace 탭 안에 둔다
- 알려진 제한 사항을 이 문서 하단에 기록
- 배포물은 단일 `Risu Recomposer.js`로 유지
- 사용자가 요청하지 않으면 zip을 만들지 않는다

### Beta에서 보류할 항목

- Archive Center backend 연동
- MariaDB/ChromaDB 기억
- 인물별 주관 기억
- Full Table Read
- Provider-native sub-agent/tool calling

### 작업 프롬프트

```text
R6 세 턴 실사용 검증 자료와 R1-R5 구현 보고를 읽고 R7 Beta Gate를 판정하라.

10개 Beta 조건을 pass/block로 표시하고 근거를 적어라. blocker가 있으면 beta
버전을 붙이지 말고 정확한 owner 함수와 재현 조건만 보고하라. 전부 통과한 경우에만
source/Risu Recomposer.js의 버전과 최소 UI 문구를 정리하라.

새 기능, 새 역할, Table Read, Archive Center 연동, zip 생성은 하지 마라. 문법 검사,
전체 인메모리 테스트, 파일 bytes/lines를 보고하고 멈춰라.
```

## 12. 전체 작업을 한 AI에게 맡길 때 사용하는 Master Prompt

```text
다음 문서를 작업 계약으로 사용하라.

source/docs/2.5-output-quality-layer/risu-recomposer-r1-r7-worker-prompts-2026-07-23.md

필수 입력:
- source/AGENTS.md
- source/Risu Recomposer.js
- source/docs/2.5-output-quality-layer/2.5-mdash-fusion-operating-contract-2026-06-29.md
- source/docs/2.5-output-quality-layer/2.5-strong-fusion-enhancement-roadmap-2026-07-02.md
- R5에서만 source/docs/provider-request-overrides-flex-paygo-contract.md

R1부터 R5까지 순서대로 구현하되 각 단계가 끝날 때마다 해당 단계 테스트와 전체
회귀 테스트를 실행하라. 다음 단계가 앞 단계의 계약을 깨뜨리면 진행하지 말고 먼저
수정하라.

수정 대상은 source/Risu Recomposer.js 한 파일뿐이다. 역할 추가, Table Read,
Archive Center 연동, 백엔드 추가, 예제 코드 복사, 대규모 포맷 변경을 금지한다.
정상 후보를 조언으로만 남기지 말고 Composer 또는 최고 후보를 통해 실제 출력에
적용하라.

R6는 실사용 자료가 필요하므로 코드 구현 후 테스트 절차만 보고하고 멈춰라. R7은
실사용 자료 없이 beta 버전을 붙이지 마라.

최종 보고에는 단계별 변경 함수, 테스트, lines added/removed, bytes, 최대 호출 수,
원문 반환 사유, API Key 노출 점검을 포함하라.
```

## 13. Codex 검증 체크리스트

다른 AI가 완료했다고 보고하면 Codex는 보고문을 믿는 대신 실제 파일을 확인한다.

1. `node --check` 직접 실행
2. `runInMemoryTests` 직접 실행
3. 버전과 파일 bytes/lines 확인
4. 새 역할이나 중복 호출 경로가 생기지 않았는지 확인
5. `callRole -> callProvider` 단일 경로 유지 확인
6. Composer 실패 시 specialist 후보 적용 확인
7. Quality에서 설정된 전문 역할 전체 선택 확인
8. semantic consensus가 단순 후보 수로 계산되지 않는지 확인
9. Trace에 original/final segment evidence가 있는지 확인
10. API Key, Authorization, extra headers가 Trace/DOM에 노출되지 않는지 확인
11. protected/inspect-only exact preservation 테스트 확인
12. deadline 후 active call 0 확인
13. 파일 크기 상한 확인
14. Archive Center/Table Read 코드가 섞이지 않았는지 확인
15. R6 자료 없이 beta 표시를 붙이지 않았는지 확인

검증 실패 시 전체 작업을 다시 맡기지 않는다. 실패한 단계와 owner 함수만 잘라서
국소 수정 프롬프트를 만든다.
