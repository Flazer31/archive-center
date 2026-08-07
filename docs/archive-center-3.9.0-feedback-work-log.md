# Archive Center 3.9.0 피드백 작업 내역

상태: `source_implemented / automated_regression_verified / package_not_built / live_not_verified`

기준일: 2026-08-06  
기준 source: `C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source`  
기준 HEAD: `6a2d27a4b7a545c5e290ce510fcf3fa7c0464ba5` (`release: prepare Archive Center 3.9.0`)

이 문서는 3.9.4 작업을 폐기하고 공식 3.9.0 source로 돌아온 뒤 반영한 사용자 피드백만 기록한다.
기능이 source에 연결됐다는 사실과 실제 설치본·RisuAI·MariaDB·Chroma·Voyage에서 확인됐다는 사실을 구분한다.

> [!CAUTION]
> **스트리밍 원문 인식 지원 불가 (`unsupported`)**
>
> Archive Center 3.9.0은 RisuAI 또는 PocketRisu가 화면에 표시하는 스트리밍 응답의 중간
> chunk/token을 실시간 원문으로 읽어 저장하거나 평론가에 전달하는 기능을 지원하지 않는다.
> RisuAI의 `addRisuChatListener("output", ...)`은 스트리밍이 모두 끝난 뒤 확정된 출력을
> 알리는 완료 이벤트이지 스트리밍 조각을 읽는 API가 아니다. PocketRisu에는 현재 동일한
> 공식 완료 이벤트 계약도 확인되지 않았다. 따라서 스트리밍 모드의 원문 저장·평론가 호출은
> 호스트 공통 지원 기능으로 보장하거나 배포 기능으로 표기하지 않는다.
>
> 이 제한을 우회하기 위한 DOM 감시, polling, 고정 대기 시간, 네트워크 가로채기 또는 별도
> fallback 저장 경로는 만들지 않는다. 스트리밍 중간 조각 인식이 필요하면 RisuAI와 PocketRisu
> 본체가 공식 plugin API로 해당 lifecycle 이벤트를 제공해야 한다.

## 1. 증거 단계

| 단계 | 의미 | 현재 상태 |
|---|---|---|
| `source_implemented` | 현재 production source에 구현 연결 | 완료 |
| `automated_regression_verified` | 단위·통합·smoke 회귀 검사 통과 | 완료 |
| `package_built` | 현재 변경으로 설치 패키지 또는 테스트 빌드 생성 | 미실행 |
| `loaded_artifact_verified` | 생성한 파일을 RisuAI에 실제 로드하여 확인 | 미실행 |
| `live_provider_verified` | 실제 Voyage API로 요청·응답 확인 | 미실행 |
| `real_db_vector_verified` | 실제 MariaDB·Chroma에 저장·검색 확인 | 미실행 |
| `release_verified` | 배포 artifact·hash·설치·업데이트 확인 | 미실행 |

## 2. 피드백별 상태 요약

| 피드백 | 내용 | 상태 |
|---|---|---|
| 1-A | 기존 `.runtime`의 MariaDB를 새 3.9.0 데이터 경로로 가져올 때 PowerShell 단일 후보 선택 실패 | source·smoke 완료 |
| 1-B | 후처리 플러그인이 있는 경우 본문 최종 출력 전에 평론가가 시작되는 문제 | 스트리밍 원문 인식은 `unsupported`; 비스트리밍·완료 후 처리와 혼동 금지 |
| 2 | `voyage-context-4` contextualized chunk embedding 완전 지원 | source·자동 회귀 완료, 실제 Voyage·DB·Chroma 확인 필요 |
| 3 | 저장된 reasoning effort가 UI 재진입 시 `none`으로 보이는 문제 | source·자동 회귀 완료, 실제 RisuAI 새로고침 확인 필요 |
| 4 | 수동 삽화 모듈 호출 때 턴 HUD의 `총 생성·저장`이 표시되는 문제 | 보류 — 닫힌 기존 HUD의 재표시 가능성, 재현 전까지 다음 항목으로 이동 |
| 5 | 설정의 `지금 업데이트` 한 번으로 OS·CPU 판별부터 교체·DB migration·재시작까지 자동 실행 | source·자동 회귀 완료, 이 변경으로 빌드한 다음 패키지부터 지원 |
| 6 | macOS 공개 런처가 필수 제한시간 값을 전달하지 않아 시작이 실패하는 문제 | source·자동 회귀 완료, 실제 macOS 확인 필요 |
| 7 | LLM endpoint를 비워 두면 설정은 저장되지만 UI에는 `저장 실패`가 표시되는 문제 | 원인·수정 방향 확인, 미수정 |
| 8 | 구 POSIX 설치본을 신규 설치로 교체할 때 기존 MariaDB·Chroma 데이터만 유지하는 방법과 Termux 준비 대기 실패 | 현재 간편 설치의 제한시간 전달 확인, legacy DB 수동 이전 절차 기록, Linux systemd 사용자 지정 경로 전달은 미처리 |

## 3. 피드백 1-A — 기존 DB 수동 적용

### 3.1 확인된 원인

Windows 전체 패키지 실행기의 `Import-LegacyRuntimeDataOnce`가 MariaDB 후보를 정확히 하나 찾았을 때
PowerShell 배열 원소를 문자열로 변환하는 식이 배열 인덱싱보다 먼저 평가될 수 있었다.

기존 식:

```powershell
Source = [string]$mariaCandidates[0]
```

수정 식:

```powershell
Source = [string](@($mariaCandidates)[0])
```

### 3.2 동작 계약

- 기존 패키지의 `.runtime` 위치는 탐색 대상으로 유지한다.
- MariaDB 후보가 하나면 그 경로 하나만 새 managed data root로 복사한다.
- 후보가 여러 개면 임의로 하나를 고르지 않는다.
- 원본 `.runtime`과 기존 DB는 삭제하거나 이동하지 않는다.
- DB schema와 저장 위치 계약은 바꾸지 않는다.
- 사용자 DB를 자동 초기화하거나 rollback하지 않는다.

### 3.3 변경 파일

- `ops/full-package/scripts/start-full-windows.ps1`

### 3.4 검증

실행:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File ops\legacy-data-import-smoke.ps1
```

결과:

- `archive-center.legacy-data-import-smoke.v1`: `status=ok`
- launcher와 external installer의 live-port 거부 검사 통과
- IPv4·IPv6 및 configured-port 거부 검사 통과
- offline atomic promotion 통과
- legacy source 보존 확인

실제 사용자 DB를 대상으로 한 수동 이전은 수행하지 않았다.

## 4. 피드백 1-B — 후처리 완료 이후 원문 저장과 평론가 호출

### 4.0 최종 지원 결정

**Archive Center 3.9.0의 스트리밍 원문 인식은 지원 불가다.** 사용자가 스트리밍 출력을
화면에서 계속 받는 동안 Archive Center가 그 중간 조각을 읽어 원문을 구성하는 기능은 없다.
스트리밍 완료 뒤 전달되는 최종 출력은 완료 후 관찰이며, 이를 스트리밍 실시간 인식 지원으로
표기하지 않는다.

아래 4.2~4.6은 `output` 완료 이벤트를 이용해 최종 출력만 받으려 했던 이전 구현 시도와 검증
기록이다. 스트리밍 chunk 인식이 구현되었거나 지원된다는 근거로 사용하지 않는다.

### 4.1 문제

`afterRequest` 직후에는 RisuAI의 본문 생성 결과가 존재하더라도 다른 후처리 플러그인이 최종 표시 문장을
아직 교체하지 않았을 수 있다. 이 값을 즉시 canonical raw로 저장하고 평론가를 호출하면 사용자가 최종적으로
본 출력과 Archive Center가 저장·분석한 출력이 달라진다.

### 4.2 이전 구현 시도 기록 — 지원 근거 아님

```text
beforeRequest
  -> 감독관/본문 요청 준비
  -> afterRequest에서 응답 후보만 기록
  -> RisuAI 공식 output 이벤트 대기
  -> output 이벤트가 가리키는 활성 assistant 메시지 확인
  -> 최종 표시 원문 확정
  -> raw 저장
  -> 평론가 호출
  -> derived memory·vector 처리
```

핵심 변경:

- `afterRequest`는 최종 저장을 시작하지 않고 `candidate_observed` 상태와 후보 hash만 기록한다.
- RisuAI의 `addRisuChatListener("output", ...)`가 전달한 실제 chat/message를 최종 출력으로 사용한다.
- 사용자 메시지 위치, assistant 메시지 위치, character/chat index와 요청 correlation을 확인한다.
- 같은 요청이 이미 저장을 시작했으면 중복 output 이벤트를 다시 처리하지 않는다.
- 후처리 플러그인 이름이나 출력 문구를 하드코딩하지 않는다.
- 기존 post-output 교체 예약 경로와 `afterRequest` 직접 확정 경로는 제거했다.
- unload 시 등록한 output listener를 해제한다.

### 4.3 유지한 경계

- 중복 요청 방지는 유지한다.
- `Archive Center.js`는 RisuAI hook과 표시 원문 관찰만 담당한다.
- canonical 저장·평론가·derived 처리 권한은 Go backend에 유지한다.
- DOM polling, 고정 대기 시간, plugin-name 예외, 별도 fallback 저장 경로는 추가하지 않았다.

### 4.4 변경 파일

- `Archive Center.js`
- `go-service/cmd/js-route-variant-smoke/main_part02_test.go`
- `go-service/cmd/js-route-variant-smoke/main_part11_test.go`
- `go-service/cmd/js-route-variant-smoke/main_part12_test.go`

현재 `Archive Center.js` diff: `+224 / -283`.

### 4.5 주요 회귀 검사

- `afterRequest`가 저장을 시작하지 않고 output 신호를 기다리는지
- 후처리된 최종 메시지를 output listener가 받는지
- 동일 output 이벤트가 정확히 한 번만 저장되는지
- 활성 chat을 별도로 재추정하지 않고 output event의 message를 사용하는지
- output final이 선택적 orchestration pending state 없이도 기존 요청과 연결되는지
- 기존 post-output secondary replacement 경로가 남지 않았는지

### 4.6 최종 검증 경계

- 현재 source는 `addRisuChatListener`가 실제 host에 존재할 때 listener를 등록한다.
- 실제 사용 중인 RisuAI/PocketRisu 빌드가 이 API와 `output` 이벤트를 제공하는지는 아직 로드 검증하지 않았다.
- API가 없는 host에서 사용할 대체 DOM watcher나 시간 기반 fallback은 만들지 않았다.
- 현재 source는 등록 요청·callback·등록 예외를 lifecycle trace로 기록하지만, API 자체가 없는 경우를 위한
  별도 stable error code는 아직 없다.
- 위 listener는 스트리밍 완료 후 최종 출력을 받는 수단일 뿐, 스트리밍 중간 조각을 읽지 않는다.
- RisuAI와 PocketRisu 공통 스트리밍 원문 인식 지원을 주장하지 않는다.
- 배포 문서와 테스트 결과에서 스트리밍 원문 인식을 `implemented`, `verified` 또는 `supported`로 표기하지 않는다.

## 5. 피드백 2 — Voyage Context 지원

### 5.1 적용 모델과 endpoint

- `voyage-context-*`: `/v1/contextualizedembeddings`
- 일반 Voyage 모델: 기존 `/v1/embeddings`
- context endpoint는 설정된 Voyage endpoint의 base 또는 `/embeddings` 경로에서 정확히 변환한다.
- endpoint가 비어 있을 때 임의의 기본 endpoint를 넣지 않는다.
- 일반 Voyage 요청 형식은 변경하지 않는다.

문서 저장 요청:

```json
{
  "model": "voyage-context-4",
  "inputs": [["chunk 1", "chunk 2", "chunk 3"]],
  "input_type": "document"
}
```

검색 요청:

```json
{
  "model": "voyage-context-4",
  "inputs": [["query"]],
  "input_type": "query"
}
```

### 5.2 다중 청크 문맥화

현재 턴의 동일 source revision에서 만들어진 다음 공개 검색 자료를 한 inner document group으로 보낸다.

- 턴 요약 memory
- 직접 근거
- 일반 공개 precise memory

각 자료를 하나씩 독립 호출하지 않는다. Voyage 응답의 outer document index와 inner chunk index를 검증하고,
반환 순서가 바뀌어도 원래 자료 위치에 다시 연결한다. 내용이 완전히 같은 두 청크도 텍스트 값으로 합치지 않고
서로 다른 index로 유지한다.

다음 자료는 일반 검색용 context group에 넣지 않는다.

- `owner_private`
- `restricted`
- `reveal_required`
- knowledge-holder 전용 자료
- 관점·주관 기억

### 5.3 저장과 재시도

- 성공한 contextualized embedding과 실제 응답 model을 memory admission 및 vector outbox에 전달한다.
- provider 호출이 실패했을 때도 outbox 문서에 전체 sibling chunk 목록과 대상 index를 보존한다.
- 재시도는 대상 청크 하나만 다시 보내지 않고 전체 sibling group을 다시 문맥화한다.
- Chroma metadata에는 재시도용 sibling 원문 목록을 남기지 않는다.
- Chroma가 설정되지 않은 환경에서도 canonical MariaDB memory용 요약 embedding은 생성한다.
- DB table·column·migration은 추가하지 않았다. context 전달 필드는 commit/outbox용 비영속 필드다.

### 5.4 검색·유지보수 연결

Context 모델 경로를 다음 기존 owner에 연결했다.

- prepare-turn memory recall query
- reference canon-base query
- reference recall query
- reference vector search query
- 현재 턴 memory/evidence/precise admission
- memory vector outbox materialization과 retry
- foreground admin memory reindex
- background admin memory reindex
- evidence/world-rule derived reindex
- reference material reindex
- session migration vector reindex
- 단건 status·world-rule 등 기존 document embedding 호출

### 5.5 변경 파일

Provider·현재 턴·outbox:

- `go-service/internal/httpapi/turn_extraction_vector.go`
- `go-service/internal/httpapi/turn_extraction_persist.go`
- `go-service/internal/httpapi/turn_memory_admission.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor.go`

검색·재색인·이전:

- `go-service/internal/httpapi/prepare_turn_recall.go`
- `go-service/internal/httpapi/group_reference_canon_base.go`
- `go-service/internal/httpapi/group_reference_recall.go`
- `go-service/internal/httpapi/group_reference_vectors.go`
- `go-service/internal/httpapi/group_admin_reindex.go`
- `go-service/internal/httpapi/group_admin_vector_maintenance.go`
- `go-service/internal/httpapi/group_session_migration.go`

Outbox 전달용 비영속 필드:

- `go-service/internal/store/precise_memory.go`
- `go-service/internal/store/mariadb_memory_admission.go`
- `go-service/internal/store/mariadb_precise_memory.go`

회귀 테스트:

- `go-service/internal/httpapi/group_proxy_test.go`
- `go-service/internal/httpapi/memory_admission_worker_test.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor_test.go`
- `go-service/internal/httpapi/character_perspective_test.go`

Voyage Context 작업의 JavaScript 증감: `+0 / -0`.  
Voyage Context 관련 Go·테스트 diff: `+840 / -33`.

### 5.6 주요 회귀 검사

- context 전용 endpoint 사용
- `inputs`가 한 문서의 여러 sibling chunk를 포함하는 중첩 배열인지
- document/query `input_type` 구분
- 역순 응답 index의 정확한 원위치 매핑
- 같은 텍스트를 가진 서로 다른 청크의 독립 매핑
- 일반 Voyage endpoint와 기존 payload 유지
- 현재 턴 memory·evidence·public precise가 provider 한 번에 전달되는지
- private/perspective precise가 일반 group에서 제외되는지
- Chroma가 없어도 MariaDB memory embedding이 생성되는지
- outbox retry가 전체 sibling group과 고정 index를 유지하는지
- retry 전용 context metadata가 Chroma에 노출되지 않는지

### 5.7 남은 검증 경계

- 실제 Voyage API key를 사용한 요청은 실행하지 않았다.
- 실제 Voyage rate/token 제한과 장문 archive의 provider 응답은 확인하지 않았다.
- 실제 MariaDB admission/outbox row와 Chroma embedding 문서를 함께 읽어보는 통합 검증은 하지 않았다.
- 실제 모델 전환 후 기존 DB 전체 재색인 시간·비용·검색 품질 비교는 하지 않았다.
- 현재 변경으로 테스트 빌드나 배포 패키지를 만들지 않았다.

## 6. 피드백 3 — reasoning effort UI 표시

이 피드백은 Voyage Context 피드백과 같은 대화에 포함됐지만 별도 문제로 분리한다.

### 6.1 실제 저장 위치

- reasoning effort를 포함한 플러그인 설정의 영속 원본은 MariaDB가 아니라 RisuAI `pluginStorage`다.
- 플러그인 시작 시 `loadSettings()`가 `pluginStorage`에서 설정을 복원한다.
- 저장 시 `saveSettings()`가 `pluginStorage`에 기록하고 `syncConfigToBackend()`로 현재 설정을 백엔드에 전달한다.
- 백엔드 `/config/update`는 설정을 실행 중 메모리에만 보관하며 응답도 `persisted: false`,
  `persistence: runtime_only`로 명시한다.
- API 키나 reasoning 설정을 MariaDB에 저장하는 변경은 하지 않았다.

관련 source:

- `Archive Center.js`: `loadSettings`, `saveSettings`, `syncConfigToBackend`
- `go-service/internal/httpapi/group_health.go`: `handleConfigUpdate`

### 6.2 직접 원인

출판사 `mo-pluginMainReasoningEffort`와 평론가 `mo-subLlmReasoningEffort`의 최초 HTML 선택 목록에는
`none`, `low`, `medium`, `high`, `enable`, `disable`만 있었다. `minimal`, `xhigh`, `max`가 없었기 때문에
`pluginStorage`에서 `max`를 정상 복원해도 브라우저가 해당 option을 찾지 못해 최초 값이 `none`으로 떨어졌다.

그 뒤 provider/model에 맞는 동적 목록이 만들어질 때는 이미 선택 요소에서 읽은 `none`이 다시 적용됐다.
이 상태에서 저장 버튼을 누르면 `pluginStorage`의 실제 값도 `none`으로 덮어쓸 수 있었다.

### 6.3 변경 내용

- 출판사와 평론가의 최초 reasoning effort 선택 목록에 `minimal`, `xhigh`, `max`를 추가했다.
- 각 option이 저장된 설정값을 직접 `selected`로 복원하도록 기존 방식과 동일하게 연결했다.
- 동적 provider/model 판정, 설정 저장, 백엔드 동기화, provider request 생성은 변경하지 않았다.
- 별도 fallback·watcher·cache·DB 설정 저장 경로를 추가하지 않았다.

변경 파일:

- `Archive Center.js`
- `go-service/cmd/js-route-variant-smoke/main_part10_test.go`

### 6.4 회귀 검사

`TestArchiveCenterJSReasoningEffortInitialSelectPreservesStoredMax`를 추가했다. 이 검사는 출판사와 평론가의
최초 선택 목록 모두에 전체 지원 option이 존재하고, 저장값이 `max`일 때 최초 렌더링에서 `max`가 직접
선택되는지 확인한다.

통과:

```text
node --check "Archive Center.js"
go test ./cmd/js-route-variant-smoke -run 'TestArchiveCenterJS(GLM52ReasoningEffortMarkers|ReasoningEffortInitialSelectPreservesStoredMax)$' -count=1
go test ./cmd/js-route-variant-smoke -count=1
```

실제 RisuAI에 현재 source를 로드한 뒤 `max` 저장 → 새로고침 → 재표시 → 다시 저장까지 확인하는 live 검증은
아직 실행하지 않았다.

## 7. 피드백 4 — 수동 삽화 모듈 호출과 턴 HUD

### 7.1 확인된 요청 타입 경계

Archive Center의 `beforeRequest`·`afterRequest`·저장·문맥 주입 경로에는 RisuAI 훅의 `type`이 비어 있거나
정확히 `model`인 요청만 들어간다. `submodel`, `otherAx` 등 다른 타입은 `onBeforeRequest()` 초입에서 원본
payload를 그대로 반환하므로 턴 HUD, 감독관, 저장, 평론가가 시작되지 않는다.

`primeTurnWorkflowHUD()`도 이 타입 검사 뒤에 호출된다. 따라서 수동 삽화 모듈 버튼을 누른 직후 새 턴 HUD가
열렸다면 다음 중 하나다.

1. 삽화 모듈이 보조 모델 설정을 사용하더라도 RisuAI 훅에는 요청을 `model` 또는 빈 타입으로 전달했다.
2. 삽화 모듈이 일반 채팅 생성을 실행했고, 이미 채팅에 들어간 user/assistant 쌍을 Archive Center 백필이
   별도의 완료 턴으로 처리하기 시작했다.

“보조 모델 슬롯을 사용한다”는 설정과 RisuAI replacer가 Archive Center에 전달하는 훅 `type`은 같은 정보가
아니다. 슬롯이 보조 모델이어도 호출 코드가 `mode: "model"`을 사용하면 Archive Center에는 본문 요청으로 보인다.

관련 source:

- `Archive Center.js`: `isNarrativeType`, `isSaveType`, `isContextInjectionType`
- `Archive Center.js`: `onBeforeRequest`, `primeTurnWorkflowHUD`
- `Archive Center.js`: `buildCompletedTurnPairsFromActiveChatMessages`, `ensureActiveChatCompletedTurnsBackfilled`

### 7.2 `sendChat`과 백필 경계

RisuAI 공식 Plugin API v3의 `sendChat(message)`는 사용자 메시지를 보낸 것처럼 일반 채팅 처리 흐름을 실행한다.
Archive Center의 active-chat 백필은 저장된 채팅에서 user/assistant 역할, 내용, 메시지 위치를 읽지만 그 쌍을 만든
플러그인의 원래 `runLLMModel` mode나 모듈 이름은 받을 수 없다.

따라서 삽화 모듈이 `sendChat`으로 실제 일반 user/assistant 메시지를 채팅에 추가하면, 다음 플러그인 시작,
본문 `beforeRequest`, 타임라인 새로고침 등의 백필 시점에 일반 완료 턴 후보가 될 수 있다. 이 경계는 현재 source에
실제로 존재한다.

반대로 삽화 결과가 채팅에 일반 user/assistant 쌍으로 추가되지 않고 호출 타입도 실제 `submodel`이라면 Archive
Center 턴 처리로 들어오지 않는다.

### 7.3 이번 확인의 판정

- 사용자 관찰을 다시 반영하면 삽화 요청 자체가 새 HUD를 연 것이 아니라, 닫았던 기존 HUD가 다른 작업에 걸려
  다시 열린 상황일 가능성도 있다.
- 진짜 `submodel` 호출에서 턴 HUD가 뜨는 source 결함은 현재 코드와 회귀 fixture에서는 재현되지 않았다.
- `model` 또는 빈 타입으로 들어온 삽화 호출은 현재 정상 본문 요청과 구분할 수 없다.
- `sendChat`으로 생성된 일반 채팅쌍은 나중에 백필될 수 있다.
- HUD가 표시됐다는 사실만으로 DB 저장까지 완료됐다고 판단할 수는 없다. HUD는 저장 전 `beforeRequest`에서 먼저 열린다.
- NAI 구독은 원인 판별에 필요하지 않다. 해당 삽화 모듈 source의 `runLLMModel.mode`/`sendChat` 사용 여부 또는
  한 번의 실제 `beforeRequest` type 기록이면 두 경로를 구분할 수 있다.
- 플러그인 이름이나 삽화 문구를 하드코딩해 차단하는 코드는 추가하지 않았다.

### 7.4 자동 확인

통과:

```text
TestArchiveCenterJSOnlyModelTypeEntersPersistence
TestBeforeRequestNonModelSkipsPrepareTurnRuntime
TestArchiveCenterJSActiveChatCompleteTurnBackfillMarkers
TestRisuMessageIndexesDriveLogicalTurnPairs
```

실제 `인레이 마개조 삽화 모듈` source와 실행 로그는 제공되지 않았으므로 그 모듈이 두 경로 중 어느 쪽을
사용하는지는 아직 확정하지 않았다. 새 HUD인지 닫힌 HUD의 재표시인지도 구분되지 않았으므로 피드백 4는
재현 자료가 생길 때까지 보류하고 다음 피드백으로 이동한다. 이번 항목에서는 production source를 변경하지 않았다.

## 8. 현재 통합 검증 결과

### 8.1 통과

```text
node --check "Archive Center.js"
go test ./internal/httpapi ./internal/store -count=1
go test ./cmd/js-route-variant-smoke -count=1
git diff --check
ops/legacy-data-import-smoke.ps1
```

`go test ./... -count=1` 첫 실행에서는 `node`가 PATH에 없어 `cmd/js-route-variant-smoke`의 세 검사가 환경 오류로
실패했다. 같은 패키지에 bundled Node 경로를 `ARCHIVE_CENTER_NODE_BINARY`로 지정해 다시 실행했으며 통과했다.
첫 실행에서 그 외 모든 Go 패키지는 통과했다.

### 8.2 미실행

- 실제 RisuAI/PocketRisu output event
- 후처리 플러그인을 켠 실제 본문 생성
- 실제 사용자 MariaDB 이전
- 실제 MariaDB·Chroma 저장 및 검색
- 실제 Voyage Context API 호출
- 실제 RisuAI에서 reasoning effort `max` 저장 후 새로고침·재저장
- Windows 설치 패키지 생성과 설치
- 업데이트·rollback·프로세스 종료 검증
- 공개 release artifact 생성

## 9. 현재 diff 범위

현재 worktree에는 피드백 1~5 변경이 함께 들어 있다. `Archive Center.js`와 일부 JS smoke 파일은 여러 피드백의
변경이 겹치므로 전체 `git diff --numstat`만으로 피드백별 줄 수를 다시 분리하지 않는다.

이번 updater 항목은 production/test 16개 파일에 연결됐다. updater 때문에 바뀐 JavaScript 부분만 따로 계산하면
`+20/-36`이며, 남은 JavaScript 역할은 UI 표시와 `/update/apply` 단일 전송이다. 플랫폼·CPU·asset·SHA·버전
선택, 패키지 적용, DB migration, commit·rollback은 Go와 managed launcher가 담당한다.

이 worktree는 아직 commit되지 않았다. 문서 변경은 production/test 파일 수에 포함하지 않는다.

이번 3.9.0 피드백 작업에서 변경하지 않은 영역:

- 턴 삭제·리롤 정책
- `/del`·`/cut` Archive Center DB 보존 정책
- 기존 DB table schema 자체
- 설치 구조 전면 개편
- API 키·reasoning 설정의 MariaDB 저장 정책
- provider endpoint 기본값
- 새로운 polling·watcher·cache·별도 fallback 경로

## 10. 다음 확인 순서

1. 현재 피드백 변경을 포함한 테스트 빌드를 생성한다.
2. 실제 RisuAI에서 reasoning effort `max` 저장 → 새로고침 → `max` 재표시 → 다시 저장을 확인한다.
3. output listener가 있는 실제 RisuAI/PocketRisu에서 일반 출력과 후처리 출력을 각각 확인한다.
4. raw 저장 후 평론가 호출 순서와 HUD를 확인한다.
5. 기존 `.runtime` DB 복사본으로 legacy import를 확인한다.
6. `voyage-context-4` 실제 key로 다중 청크 요청과 query 검색을 확인한다.
7. MariaDB embedding model·outbox와 Chroma 문서를 함께 확인한다.

## 11. 피드백 5 — 한 번 클릭하는 완전 자동 업데이트

### 11.1 지원 시작점

기존 3.9.0 패키지 중 `PACKAGE_FILE_MANIFEST.json`에 `package_version`이 없는 설치본을 위한 별도 예외
경로는 만들지 않았다. 그런 설치본은 한 번 수동 재설치가 필요할 수 있다. 이번 변경으로 새로 빌드되는 패키지는
버전·전체 관리 파일·DB migration 계약을 모두 포함하며, 그 패키지 이후 버전부터 설정의 `지금 업데이트`
버튼 한 번으로 자동 업데이트하는 것을 정식 계약으로 삼는다.

### 11.2 구현된 흐름

```text
지금 업데이트 1회
  -> POST /update/apply 1회
  -> 실행 중인 백엔드의 OS·CPU로 GitHub release 자산 선택
  -> SHA256 검증 후 package/.updates에 staging
  -> UI 응답을 완전히 전송
  -> 백엔드 exit 75
  -> 기존 managed launcher가 즉시 update transaction 재진입
  -> 전체 관리 파일 추가·교체·삭제
  -> migrations/*.sql 전체를 이름순 적용 + embedded schema 적용
  -> candidate /ready + exact /version
  -> commit
```

사용자가 업데이트 확인을 먼저 누르거나, 별도 명령을 실행하거나, 프로그램을 수동 재시작하거나, 두 번째 확인
버튼을 누르는 단계는 없다. JavaScript는 버튼과 단일 요청만 담당하며 플랫폼, CPU, asset, hash, 현재 버전을
선택하지 않는다.

백엔드는 다음 실행 환경을 구분한다.

- Windows x64 / Windows arm64
- Linux x64 / Linux arm64
- Ubuntu는 Linux ABI와 CPU를 사용하고 `/etc/os-release`의 배포판 ID를 별도로 표시
- macOS Intel / macOS Apple Silicon
- Termux arm64 (`android/arm64`)

클라이언트가 백엔드 실행 환경과 다른 플랫폼을 지정하면 다운로드 전에 거부한다. Windows x64 자산 선택은
arm64 ZIP을 허용하지 않는다.

### 11.3 패키지와 DB 계약

- candidate `PACKAGE_FILE_MANIFEST.json`의 `package_version`은 GitHub release target과 정확히 같아야 한다.
- 현재 updater binary는 현재 manifest에 기록된 size·SHA256과 일치해야 하며 POSIX에서는 실행 권한도 필요하다.
- candidate manifest 전체를 검증한 뒤 새 파일을 추가하고 변경 파일을 교체하며, 이전 manifest에만 있는 관리
  파일은 삭제한다.
- `.runtime`, `.updates`, 사용자 env, MariaDB·Chroma data와 secret은 관리 파일에 포함하지 않는다.
- 신규 POSIX 설치는 버전 패키지 밖의 install-level `data` 경로와 `start-archive-center.sh`를 소유한다.
  Linux systemd, macOS, Termux의 최초 실행과 이후 재실행은 모두 이 실행기를 통과하므로 패키지가 갱신되어도
  MariaDB·ChromaDB 경로가 package-local `.runtime`으로 바뀌지 않는다.
- migration inventory는 모든 `migrations/*.sql`뿐 아니라 embedded schema를 가진 `mariadb-schema` binary도
  포함한다.
- SQL은 파일 이름순으로 전부 실행하며 migration은 expand-first·rerunnable·이전 backend 호환이어야 한다.
- 실패 시 관리 패키지 파일만 되돌린다. DB rollback은 하지 않는다.
- rollback한 이전 backend가 forward-migrated DB에서 `/ready`와 이전 exact `/version`을 통과해야 한다.

### 11.4 신규 경로를 만들지 않은 항목

- fresh-install wrapper와 updater는 결합하지 않았다.
- polling, watcher, 별도 background updater, 별도 update queue를 추가하지 않았다.
- 기존 `/update/check`, `/update/download`, `/update/status`는 호환 API로 유지했지만 설정의 정상 업데이트 버튼은
  `/update/apply`만 한 번 호출한다.

### 11.5 자동 검증

통과:

```text
go test ./internal/httpapi -run 'Test(Update|SelectUpdate|WindowsX64|ParseOSRelease)' -count=1
go test ./cmd/archive-center-go ./internal/httpapi -count=1
go test ./cmd/js-route-variant-smoke -run 'Test(BackendOwnedLongOperations|ArchiveCenterJSImmediateUpdate)' -count=1
go test ./internal/packageupdate ./cmd/mariadb-schema -count=1
node --check "Archive Center.js"
PowerShell parser: Windows launcher and both package builders
sh -n ops/full-package-posix/start-full-posix.sh
scripts/test-simple-fresh-install.sh
ops/full-package/scripts/updater-e2e-smoke.ps1
git diff --check
```

실제 공개 GitHub release, 실제 Windows/Linux/Ubuntu/macOS/Termux 장치, 실제 MariaDB·Chroma, 실제 launcher
프로세스 재기동은 아직 검증하지 않았다. 설치 패키지와 테스트 빌드도 생성하지 않았다.

## 12. 피드백 6 — macOS 공개 런처의 제한시간 전달 누락

### 12.1 제보와 재검증

macOS 패키지에서 `sh "Start Archive Center macOS.command"`를 실행하면 다음 오류가 발생한다는 제보를
확인했다.

```text
ERROR: external operation requires --external-operation-timeout-seconds or AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS
```

패키지 빌더가 생성하던 공개 런처에는 package root와 profile만 있었고, 공통 POSIX 런처가 요구하는 외부 작업,
로컬 HTTP 요청, 준비 대기, 준비 확인 간격이 없었다. 간편 설치의 `1800` 값도 GitHub release helper에만
전달되고 최종 macOS 런처에는 전달되지 않았다. 기존 간편 설치 테스트는 실제 POSIX 런처 대신 `started`와
data root만 기록하는 가짜 런처를 사용했기 때문에 이 누락을 발견하지 못했다.

### 12.2 수정 범위

공통 POSIX 런처 안에 숨은 기본값이나 별도 fallback을 추가하지 않았다. 정상 사용자 진입점이 다음 값을
명시적으로 소유하고 기존 POSIX 런처에 전달하도록 수정했다.

```text
AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=1800
AC_REQUEST_TIMEOUT_SECONDS=30
AC_READINESS_TIMEOUT_SECONDS=180
AC_READINESS_POLL_INTERVAL_SECONDS=1
```

- 패키지 빌더가 생성하는 Linux·macOS·Termux 공개 런처에 네 기본값을 포함한다.
- 값이 이미 지정된 경우 `${NAME:-default}` 규칙으로 사용자 값을 유지한다.
- `install.sh`가 release helper와 최종 패키지 런처에 네 값을 전달한다.
- GitHub release helper의 직접 시작과 systemd unit에도 같은 값을 전달한다.
- 공통 `start-full-posix.sh`의 기존 명시적 입력 계약은 변경하지 않았다.
- `install.sh`와 release helper는 값을 전달만 하며 request/readiness 값 검증은 기존 `start-full-posix.sh`가
  계속 단독으로 담당한다.

### 12.3 자동 검증

통과:

```text
sh -n install.sh
sh -n scripts/install-github-release.sh
sh -n scripts/test-simple-fresh-install.sh
PowerShell parser: ops/build-posix-managed-packages.ps1
scripts/test-simple-fresh-install.sh
  - macOS Intel
  - macOS Apple Silicon
  - Linux x64
  - Linux arm64
  - Termux arm64
scripts/test-simple-fresh-install.ps1
  - macOS Apple Silicon production package build
  - generated Start Archive Center macOS.command --preflight
  - custom timeout override preservation
```

생성된 `.command`가 기본값 `1800/30/180/1`을 실제 하위 macOS 런처에 전달하고, 호출자가 다른 값을 지정하면
그 값을 덮어쓰지 않는 것까지 기존 Windows CI 회귀 안에서 매번 다시 확인한다. 패키지 출력은 workspace 내부의
고유 임시 경로에만 만들고 테스트 종료 시 제거한다. 실제 macOS 장치에서 Homebrew·MariaDB·ChromaDB를
설치하고 backend `/ready`까지 도달하는 실기기 검증은 아직 수행하지 않았다.

### 12.4 현재 문서 기준

3.9.4 작업 폐기와 3.9.0 재시작이라는 이후 사용자 지시에 따라 현재 피드백 기준 문서는 이 파일이다. 폐기된
3.9.4 작업을 전제로 한 `archive-center-3.9.0-to-3.9.4-consolidated-audit.md`는 새로 만들지 않는다.

## 13. 피드백 7 — LLM endpoint 누락 시 저장 결과 표시 불일치

### 13.1 제보

macOS 26.5.2의 RisuAI 플러그인 설정에서 출판사와 편집 검토·평론가 역할에 API key와 model만 입력하고
endpoint를 비워 둔 경우 다음 오류와 함께 `저장 실패`가 표시된다는 제보를 받았다.

```text
runtime_config_incomplete:main[endpoint];supervisor[endpoint];critic[endpoint]
```

endpoint를 직접 입력하면 오류가 발생하지 않았으며, 실패 표시가 나온 경우에도 RisuAI `pluginStorage`에는
설정값이 저장된 것으로 관찰됐다. 이 문제는 macOS 전용 코드가 아니라 공통 JavaScript 설정 UI와 Go runtime
검증 순서에서 발생한다.

### 13.2 확인된 원인

`saveSettings()`는 먼저 `pluginStorage`에 설정을 저장하고 그다음 백엔드 `/config/update` 동기화를 수행한다.
로컬 저장이 성공했어도 backend runtime trace가 endpoint 누락을 보고하면 함수가 `false`를 반환하므로 UI에는
전체 저장이 실패한 것처럼 표시된다.

Go의 `configMissingFieldsWithProvider()`는 provider·API key·endpoint·model을 실행 가능한 LLM 역할의 필수
필드로 검사한다. JavaScript는 main 설정 중 하나라도 채워지면 main과 supervisor를 검사 대상으로 삼고,
critic 설정 중 하나라도 채워지면 critic을 검사 대상으로 삼는다.

요청 코드 안에는 일부 provider의 기본 URL을 계산하는 함수가 있지만 빈 endpoint는 그 함수에 도달하기 전에
요청 검증에서 거부된다. 따라서 Archive Center backend의 supervisor·critic 호출이 endpoint 없이 정상
작동한다는 의미가 아니다. RisuAI가 직접 담당하는 메인 본문 생성만 별도로 작동해 보일 수 있다.

### 13.3 확정한 수정 방향

- endpoint는 실행 가능한 LLM 역할의 필수값으로 유지한다.
- provider별 endpoint를 자동 입력하거나 자동 보정하지 않는다.
- API key 또는 model을 입력한 역할은 저장 전에 endpoint 누락을 UI에서 검사한다.
- endpoint 누락 시 기존 설정을 저장한 뒤 일반적인 `저장 실패`로 표시하지 않고, 해당 endpoint가 필요하다는
  구체적인 안내를 표시한다.
- main endpoint는 같은 설정을 사용하는 출판사와 감독관에 적용하고, 평론가는 유효한 평론가 설정을 기준으로
  별도로 검사한다.
- endpoint가 채워진 경우 기존 `pluginStorage` 저장과 backend runtime 동기화 경로를 그대로 사용한다.
- 새로운 provider 기본값, fallback, watcher 또는 별도 저장 경로를 추가하지 않는다.

### 13.4 현재 상태

이번 항목에서는 원인과 수정 방향만 확인했다. production source와 테스트는 아직 변경하지 않았다.

## 14. 피드백 8 — POSIX 신규 설치에서 기존 DB만 유지

### 14.1 제보와 설치 실패 원인

Termux에서 예전 `install-archive-center.sh --repo ... --install-dir ... --start` 명령과 직접 받은 ZIP을 사용했을 때
필수 제한시간 값이 전달되지 않았고, MariaDB가 다음 오류로 시작되지 않았다는 제보를 받았다.

```text
Initializing MariaDB data directory
Starting MariaDB on 127.0.0.1:3307
ERROR: MariaDB did not become ready on 127.0.0.1:3307
```

공통 POSIX 실행기의 `readiness_polling_enabled()`는 readiness timeout과 poll interval이 모두 있을 때만 반복
확인을 활성화한다. 두 값이 없으면 `wait_port()`가 MariaDB 포트를 한 번 확인한 뒤 실패하므로, MariaDB가
정상적으로 InnoDB와 소켓을 초기화하는 중이어도 성급하게 실패할 수 있다.

현재 공식 신규 설치 진입점은 다음 한 줄이다.

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

현재 `install.sh`와 패키지 빌더가 생성하는 Linux·macOS·Termux 공개 런처는
`1800/30/180/1`의 external/request/readiness/poll 값을 명시적으로 전달한다. 따라서 정상 신규 설치 경로는
제보된 한 번 검사 문제를 해소한다. 공통 POSIX 실행기에 숨은 기본값이나 별도 fallback을 추가하지 않는다.

### 14.2 기존 DB만 보존하는 Termux 수동 절차

구 Termux 기본 데이터 위치는 `$HOME/.archive-center-2.0`이고 신규 간편 설치의 기본 데이터 위치는
`$HOME/.archive-center/data`다. 기존 Archive Center·MariaDB·ChromaDB를 모두 종료한 뒤 다음과 같이 기존
MariaDB·ChromaDB 디렉터리만 별도 데이터 위치로 복사하여 사용할 수 있다.

```sh
mkdir -p "$HOME/archive-center-preserved-data"

cp -a "$HOME/.archive-center-2.0/mariadb-data" \
  "$HOME/archive-center-preserved-data/"

[ ! -d "$HOME/.archive-center-2.0/chromadb-data" ] || \
  cp -a "$HOME/.archive-center-2.0/chromadb-data" \
  "$HOME/archive-center-preserved-data/"

curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | \
  ARCHIVE_CENTER_DATA_DIR="$HOME/archive-center-preserved-data" sh
```

새 설치 루트인 `$HOME/.archive-center`가 없어야 하며, 구 실행기·updater·`.updates`·package manifest·runtime
binary는 복사하지 않는다. 이 절차는 MariaDB와 ChromaDB 데이터만 보존한다.

### 14.3 Linux·Ubuntu 적용 범위

`ARCHIVE_CENTER_DATA_DIR`는 Termux 전용이 아니라 공통 POSIX runtime 계약이므로 Linux·Ubuntu·macOS에서도
사용할 수 있다. 다만 구 데이터 원본 경로는 설치 방식에 따라 다르다.

- Termux 구 기본값: `$HOME/.archive-center-2.0`
- Linux·Ubuntu 구 압축 패키지: 해당 패키지의 `.runtime`
- 신규 비-systemd Linux·macOS·Termux 기본값: `$HOME/.archive-center/data`
- 신규 systemd Linux·Ubuntu 기본값: `/opt/archive-center/data`

Linux·Ubuntu에서도 구 패키지의 `.runtime/mariadb-data`와 `.runtime/chromadb-data`만 별도 디렉터리에 복사한
뒤 같은 `ARCHIVE_CENTER_DATA_DIR` 방식으로 사용할 수 있다. 그러나 현재 `install.sh`의 systemd 분기는
release helper를 `sudo env`로 실행하면서 사용자 지정 `ARCHIVE_CENTER_DATA_DIR`를 명시적으로 전달하지 않는다.
따라서 systemd가 활성화된 Ubuntu·Linux에서는 이 명령을 현재 지원 완료로 표시하지 않는다. 해당 기존 전달
경로를 수정하고 systemd 설치 회귀 검사를 통과한 뒤 같은 절차를 공식 지원해야 한다.

### 14.4 검증 경계

- 현재 source에서 정상 간편 설치의 네 제한시간 전달과 공개 POSIX 런처 생성을 확인했다.
- 현재 실행 환경에 `sh`가 없어 이번 확인에서 셸 회귀 검사를 다시 실행하지 못했다.
- 실제 Termux·Linux·Ubuntu 장치에서 기존 MariaDB·ChromaDB를 이전하는 검증은 수행하지 않았다.
- 기존 Termux 데이터 위치를 신규 설치기가 자동 탐색·복사하는 기능은 구현하지 않았다.
- 이번 항목에서는 production source와 테스트를 변경하지 않았다.
