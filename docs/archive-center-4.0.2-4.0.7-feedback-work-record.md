# Archive Center 4.0.2~4.0.7 피드백·작업 통합 기록

기준일: 2026-08-26 KST  
활성 소스: `source/Archive Center.js`, `source/go-service`  
목적: 각 버전 작업이 어떤 사용자 피드백에서 시작됐는지와 현재 유지·수정 상태를 한 문서에서 확인한다.

## 기록 원칙

- 사용자 피드백, 실제 수정, 소스 회귀, 실환경 검증을 서로 다른 증거로 기록한다.
- 테스트 통과만으로 실제 RisuAI·MariaDB·ChromaDB 동작을 확정하지 않는다.
- 4.0.0 이전 기능을 4.0.2~4.0.7 수정으로 잘못 분류하지 않는다.
- 이번 문서에서 `유지`는 현재 소스에 남겨야 하는 작업, `수정`은 후속 결함으로 고친 작업을 뜻한다.

## 피드백 → 작업 대응표

이 표는 단순 변경 목록이 아니라, 실제로 받은 피드백 때문에 어떤 작업을 했는지를 직접 연결한다. 아래 상세 절에는 원인과 유지·수정 상태를 추가로 기록한다.

| 버전 | 작업을 시작하게 만든 사용자 피드백 | 그 피드백에 대응해 수행한 작업 | 현재 판정 |
|---|---|---|---|
| 4.0.2 | 로어북의 문자열 `bookVersion`이 HTTP 400을 일으킴 | 숫자·문자열을 모두 수용하도록 로어북 요청 호환 처리 | 유지 |
| 4.0.2 | Voyage Context 4 재색인에서 `committed_derived_result_hash_mismatch`로 기억 색인이 막힘 | `[]string` 저장 전과 JSON 재로딩 후 `[]any`가 같은 canonical JSON 해시를 사용하도록 수정 | 유지 |
| 4.0.3 | Android Firefox에서 Backend URL·저장값이 `127.0.0.1`로 돌아감 | Firefox 전용 우회가 아니라 전체 브라우저의 설정 저장·복원 경로를 정리 | 유지 |
| 4.0.3 | 매일 순간이동 큐브로 합류하기 같은 고정 약속을 장기 진행 뒤 잊음 | 사용자가 고정한 활성 약속을 기억 전달에 남기는 최소 보강 | 유지, 전면 개선은 4.1 |
| 4.0.4 | force 재색인이 `missing_embedding`에서 완료되지 않음 | Chroma readback 뒤 MariaDB embedding·model과 outbox 완료를 함께 수렴 | 유지 |
| 4.0.4 | 112턴에서 삭제 대기열이 352,405건까지 증가함 | 삭제 operation key 수렴, 안전한 중복 삭제 `stale_rejected`, 제한 batch·공정성 적용 | 유지 |
| 4.0.5 | Grok·GLM의 출판사 호출이 즉시 reject 또는 malformed로 실패함 | 중복 지침·들여쓰기·감사 metadata를 모델 입력에서 줄이고 JSON 형식·파서 진단을 보존 | 유지 |
| 4.0.5 | 15턴·14턴 연속 삭제 뒤 이전 15턴을 평론가가 붙잡고, UI 수동 삭제도 실패함 | 연속 rollback과 수동 삭제가 source revision·logical turn 수명주기와 충돌하지 않도록 보강 | 유지 |
| 4.0.6 | NeuralWatt·OpenCode 호출량은 기록되지만 `custom returned no text content`로 실패함 | 문자열·텍스트 블록·legacy text·명시적 Responses API 응답을 정규화하고 종료·사용량 정보를 보존 | 유지 |
| 4.0.6 | 콜드 스타트 뒤 한국인 성씨가 계속 바뀜 | 기존 평론가 계약 안에 이름·성씨 대응 지침을 보강 | 유지 |
| 4.0.6 | 사용자 입력만 지웠는데 assistant 출력까지 포함한 턴이 삭제됨 | assistant message ID·generation ID·content hash 관측을 추가해 assistant 출력 존재 시 턴을 유지 | 유지 |
| 4.0.7 | 입력 수정·설정 토글 뒤 리롤하면 삭제 감지가 시작되고 초록 HUD가 장시간 멈춤 | rollback HTTP를 durable queue 경계에서 끝내고 worker batch·공정성·lease·중단 HUD를 제한 | 유지 |
| 4.0.7 | 새 `replacement_pending` 보호가 정상 수명주기를 흐림 | Go decision·source 상태와 JavaScript 성공 취급·HUD에서 `replacement_pending`을 제거 | 제거 완료 |
| 4.0.7 | 콜드 스타트가 턴을 복원한 직후 정상 턴을 스스로 rollback함 | 첫 관측 ledger 단독 삭제 금지, 중복 꼬리 비교 제거, blind-tail gap 제한 복원 | 수정 완료 |

## 4.0.2

### 받은 피드백

1. 4.0.1 사용자의 로어북 스냅샷 저장이 `bookVersion` 문자열 때문에 HTTP 400으로 거절됨.
2. Voyage Context 4 등을 사용한 재색인에서 `committed_derived_result_hash_mismatch`가 발생함.
3. 저장 전 `[]string`과 JSON 재로딩 후 `[]any`가 서로 다른 해시가 되는 정규화 불일치가 지적됨.

### 작업

- 로어북 `bookVersion`을 숫자·문자열 모두 수용하도록 호환 처리.
- 저장 시점과 검증 시점이 같은 canonical JSON을 해시하도록 수정.
- 변조 감지는 유지하고 해시 불일치를 무조건 성공 처리하는 우회는 추가하지 않음.

### 현재 상태

- 유지.
- 특정 임베딩 모델 예외나 모델 allowlist 없음.

## 4.0.3

### 받은 피드백

1. Android Firefox에서 저장값 또는 Backend URL이 다시 `127.0.0.1`로 돌아가는 브라우저 저장 호환 문제.
2. 순간이동 큐브로 매일 합류하기 같은 사용자가 고정한 반복 약속이 장기 진행 후 전달되지 않는 문제.

### 작업

- 브라우저별 저장 차이를 특정 Firefox 전용 분기로 덮지 않고 전체 브라우저 저장·URL 복원 경로를 정리.
- 사용자가 고정한 활성 약속을 기억 전달에서 유지하는 최소 보강.

### 현재 상태

- 유지.
- 반복 약속의 전면 개선은 4.1 작업 범위이며 4.0.3은 고정된 활성 약속 전달까지만 담당.

## 4.0.4

### 받은 피드백

1. force 재색인 후 `perspective_scoped_typed_delivery`가 `missing_embedding`으로 남아 완료 판정이 끝나지 않음.
2. 112턴 세션에서 삭제 작업이 약 352,405건까지 불어나고 대기열 처리 예상 시간이 수일로 늘어남.
3. 삭제 operation key가 사유·결과 해시에 따라 달라져 같은 revision·문서 삭제가 중복되는 정황.

### 작업

- ChromaDB upsert와 readback 성공 뒤 MariaDB embedding·model 갱신과 outbox 완료를 원자적으로 연결.
- delete operation key를 `delete + session + source_revision + document_id`로 수렴.
- 안전 조건을 만족하는 같은 revision·문서의 미완료 중복 삭제를 `stale_rejected`로 종료.
- worker claim·delete batch·재처리량을 제한하고 delete가 오래된 upsert 뒤에서 굶지 않게 공정성 추가.

### 현재 상태

- 유지.
- Vector 실패를 성공으로 처리하지 않으며 서로 다른 revision 삭제를 합치지 않음.

## 4.0.5

### 받은 피드백

1. LLM Gateway의 Grok·GLM 등에서 출판사 호출이 즉시 upstream reject 또는 malformed로 실패함.
2. 출판사 입력에 중복 출력 지침, 들여쓰기 JSON, 관리용 metadata가 많아 모델이 출력 계약을 지키기 어려움.
3. 연속으로 15턴과 14턴을 삭제한 뒤 새 14턴을 생성하면 평론가가 이전 15턴을 계속 붙잡는 현상.
4. 기억 관리에서 선택한 턴 삭제가 수동으로도 완료되지 않는 현상.

### 작업

- 출판사 사용자 payload의 중복 출력 지침 제거, compact JSON, 모델용 지원 패킷·실행 계약 경량화.
- provider JSON 형식과 실제 파서 오류·종료 사유·chars를 진단에 보존.
- OpenAI 계열 gateway의 JSON schema/json object 호환을 endpoint·응답 계약 기준으로 정리.
- 연속 삭제와 수동 턴 삭제가 기존 source revision·logical turn과 충돌하지 않도록 rollback decision과 source 수명주기 보강.

### 현재 상태

- 유지.
- 출판사 단일 호출, malformed fail-open, 숨겨진 재시도 금지 원칙 유지.

## 4.0.6

### 받은 피드백

1. NeuralWatt·OpenCode 같은 OpenAI 호환 endpoint가 사용량은 기록하지만 `custom returned no text content`로 연결 테스트에 실패함.
2. `content: null`, reasoning만 존재, `finish_reason=length` 응답을 단순 연결 실패로 오인함.
3. 명시적인 `/responses` endpoint와 문자열 content·텍스트 블록 배열·legacy `text` 응답이 처리되지 않음.
4. 콜드 스타트 뒤 인물 성씨가 바뀌는 사례에서 명시적 이름 대응 지침이 효과가 있다는 피드백.
5. 사용자 입력만 삭제되고 assistant 출력은 남은 턴이 통째로 삭제되는 문제.

### 작업

- Custom/OpenAI 호환 응답을 문자열, text block, legacy text, 명시적 Responses API 기준으로 정규화.
- finish reason, 사용량, reasoning token을 UI 진단까지 보존하고 reasoning만을 평론가 최종 JSON으로 사용하지 않음.
- 연결 테스트의 지나치게 작은 출력 예산을 제거하고 최종 출력 전 토큰 소진을 별도 오류로 표시.
- 평론가 이름 매칭 지침을 기존 출력 언어 계약을 바꾸지 않는 범위에서 보강.
- assistant message ID·generation ID·content hash를 전달해 사용자 입력만 사라진 턴은 유지하고 assistant 출력 삭제만 후보로 판정.

### 현재 상태

- 응답 정규화·이름 지침·사용자 입력만 삭제된 턴 유지: 유지.
- 4.0.6에서 현재 활성 채팅을 항상 `full_active_chat`으로 간주하고 4.0.5의 blind-tail 제한을 제거한 부분: 4.0.7 후속 수정에서 교정.

## 4.0.7

### 받은 피드백

1. 입력 수정 또는 설정 토글 뒤 리롤하면 삭제 감지가 시작되고 대규모 Vector outbox에서 HUD가 오랫동안 초록색으로 멈춤.
2. rollback HTTP가 Vector 정리를 끝까지 기다리고 worker 종료·lease 처리에도 제한이 부족함.
3. 4.0.7 테스트에서 콜드 스타트가 턴을 정상 복원한 직후 스스로 여러 턴을 rollback함.
4. 새 `replacement_pending` 보호가 정상 source 수명주기를 흐리고 콜드 스타트·리롤 판정을 더 복잡하게 함.

### 작업

- rollback HTTP는 MariaDB 무효화와 durable outbox 등록까지만 수행하고 `vector_cleanup=queued`로 종료.
- worker 처리량, delete 공정성, lease 시간, worker stop 대기를 제한하고 중단 HUD를 terminal·retryable로 표시.
- 새로 추가했던 `replacement_pending` decision·source 상태·JS 성공 취급·HUD를 제거.
- 첫 관측에서는 persisted ledger와 현재 창 길이만으로 rollback하지 않고 현재 실행의 기준 snapshot만 설정.
- 호스트 신호마다 백엔드 꼬리 비교를 앞뒤로 두 번 실행하던 호출을 제거하고 같은 실행의 메시지 변화 판정만 한 번 수행.
- 4.0.5까지 존재했던 blind-tail gap 제한을 복원.
- 부분적으로 보이는 현재 채팅을 전체 채팅으로 오인해 과거 source를 삭제하는 회귀 테스트 추가.

### 현재 상태

- 비동기 Vector 정리·bounded worker·terminal HUD: 유지.
- `replacement_pending`: 제거.
- 콜드 스타트 첫 관측 자동 rollback: 수정 및 소스 회귀 추가.

## 핵심 기능 감사 결과

| 사용 흐름 | 현재 소스 근거 | 회귀 근거 | 판정 |
|---|---|---|---|
| 사용자 입력·assistant 출력 원문 저장 | `/complete-turn`, `persistCompleteTurnRaw`, active-final source acceptance | complete-turn raw/idempotency/source revision 테스트 | 소스 확인 |
| 출력 기반 평론가와 기억 개체 생성 | `runCompleteTurnCritic`, `saveCriticExtractionArtifacts`, durable reprocessing worker | Critic 부분 보존, 파생 저장, 실패 queue/replay 테스트 | 소스 확인 |
| 기억·직접 근거·KG 및 서사 개체 수정·삭제 | Explorer PATCH/DELETE와 episode/storyline/world-rule/character/pending-thread/subjective-memory route | HTTP API 회귀와 JS edit/delete smoke | 소스 확인 |
| 세션 DB 연결·복사·이동·삭제 | routing attach, migrate preview/complete/reindex/lock/cleanup, session delete | migration parity·rollback·cleanup·delete 테스트 | 소스 확인 |
| 세계선 UI와 분기 계보 | `worldline_topology.viewmodel.v2`, Canvas·inspector·session action UI | Go topology와 JS presentation/gesture 테스트 | 소스 확인 |
| 콜드 스타트 실패 표시와 재실행 | session normalize, canonical raw replay, failed/deferred turn 목록, projection-complete 재개 | long-session resume, failed-turn progress, canonical replay 테스트 | 소스 확인 |
| 콜드 스타트 직후 자동 삭제 금지 | 첫 snapshot 단독으로 rollback 금지, blind tail 차단 | 신규 cold-start JS runtime 회귀 | 수정 완료 |

`소스 확인`은 실제 RisuAI·실제 MariaDB·실제 ChromaDB 검증과 동일하지 않다. 테스트 패키지에서는 위 전체 흐름을 다시 실사용 확인해야 한다.

## 테스트 패키지 실환경 확인 순서

1. 일반 1턴: user/assistant 원문 2건과 평론가 파생 개체 생성 확인.
2. 평론가 실패 1턴: 원문 유지, 파생 저장 0건, 실패 turn/reprocessing 표시 확인.
3. 같은 콜드 스타트 재실행: 성공 턴은 중복 호출하지 않고 실패 턴만 다시 후보가 되는지 확인.
4. 기억·직접 근거·KG·에피소드·세계 규칙·인물·주관 기억 각각 수정 후 재조회 확인.
5. 위 항목 삭제 후 UI·MariaDB·Vector outbox 수렴 확인.
6. 세션 연결·복사·이동·삭제를 각각 별도 빈 대상 세션으로 확인.
7. 일반 branch와 다중 branch에서 세계선 상속·턴 번호·노드 선택 상세 확인.
8. 콜드 스타트 직후 아무 입력 없이 기다려도 자동 rollback audit가 생기지 않는지 확인.
9. 실제 assistant 출력 삭제는 같은 실행 중 감지되고, 사용자 입력만 삭제하면 턴이 유지되는지 확인.
