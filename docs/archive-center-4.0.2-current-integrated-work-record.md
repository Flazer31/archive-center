# Archive Center 4.0.2 이후 통합 작업 기록

기준일: 2026-08-27 KST  
활성 소스: `source/Archive Center.js`, `source/go-service`  
현재 소스 표기 버전: `4.0.8`

이 문서는 4.0.2부터 현재 4.0.8 작업 트리까지 흩어져 있던 버전별 기록을
한곳에 합친 기준 문서다. 각 작업을 시작하게 만든 사용자 피드백, 확인된
원인, 실제 반영 내용, 유지한 경계, 검증 수준과 남은 확인 사항을 함께
기록한다.

기존 버전별 문서는 당시 패키지 해시와 세부 회귀를 보존하는 역사 자료로
남긴다. 작업 범위와 현재 상태가 서로 다르게 적힌 경우에는 실제 커밋,
현재 활성 소스, 이 통합 문서 순으로 다시 대조한다.

## 1. 기록 범위와 증거 구분

### 포함 범위

- 공식 `v4.0.2` 릴리스
- 4.0.3~4.0.5 Windows 테스트 빌드 작업
- 별도 태그가 없었던 4.0.6 복원 구간
- 4.0.7 리롤·삭제·Vector worker 작업과 후속 결함 제거
- 4.0.7 이후 콜드 스타트 개체 투영 복구
- 현재 4.0.8 소스의 타임아웃, 주관 기억 점수, KG 시간 의미,
  NeuralWatt, 세계 규칙 전달 및 기본 설정 상향

### 증거 단계

| 단계 | 의미 |
|---|---|
| 공식 릴리스 | Git 태그와 배포 자산이 존재함 |
| 패키지 검증 | 관리 파일·manifest·문법·자동 회귀가 통과한 테스트 패키지가 존재함 |
| 소스 검증 | 활성 소스와 자동 회귀가 통과했으나 실제 사용자 환경을 뜻하지 않음 |
| 실사용 확인 | 실제 RisuAI, 사용자 DB, Provider에서 관찰됨 |
| 미확인 | 소스 또는 패키지 밖의 실제 동작 증거가 아직 없음 |

테스트 통과만으로 실제 RisuAI·MariaDB·ChromaDB·외부 Provider·전체 OS
업데이트가 모두 검증됐다고 기록하지 않는다.

## 2. 현재 기준점

- 마지막 공식 태그: `v4.0.2` (`ed8fcba`)
- 4.0.4 이전 복구점: `4a30fb3`
- 4.0.5 이전 복구점: `9f1ba7e`
- 4.0.7 상태 및 콜드 스타트 개체 투영 수정 복구점: `f18b532`
- 현재 작업 트리: 위 복구점 이후 4.0.8 변경이 미커밋 상태로 함께 존재함
- 현재 `Archive Center.js` 표기: `4.0.8`
- 현재 정책 소유자: Go 백엔드
- `Archive Center.js` 역할: RisuAI 관찰, 설정 전달, 실제 payload 적용,
  표시와 UI

## 3. 피드백과 작업 요약

| 구간 | 작업을 시작하게 만든 피드백 | 반영한 작업 | 현재 상태 |
|---|---|---|---|
| 4.0.2 | 문자열 `bookVersion` 때문에 로어북 snapshot이 HTTP 400 | 숫자·숫자 문자열 호환 정규화 | 유지, 공식 릴리스 |
| 4.0.2 | 재색인 시 `committed_derived_result_hash_mismatch`로 색인 중단 | 저장 전 `[]string`과 재로딩 뒤 `[]any`가 같은 canonical JSON을 해시 | 유지, 공식 릴리스 |
| 4.0.3 | Android Firefox에서 Backend URL·저장 설정이 `127.0.0.1`로 돌아감 | 브라우저 공통 영속 저장·검증·복원 경로 정리 | 유지, 소스·회귀 |
| 4.0.3 | 매일 합류 같은 사용자가 고정한 약속을 장기 진행 뒤 잊음 | 고정·활성 pending thread를 일반 기억 예산 안에서 계속 후보로 유지 | 유지, 전면 개선은 후속 버전 |
| 4.0.4 | force 재색인이 `missing_embedding`으로 완료되지 않음 | Chroma readback 뒤 MariaDB embedding/model과 outbox 완료를 원자적으로 수렴 | 유지 |
| 4.0.4 | 112턴에서 삭제 대기열이 352,405건까지 증가 | delete operation key 수렴, 안전한 중복 종료, 제한 batch와 공정성 | 유지 |
| 4.0.5 | Grok·GLM Publisher가 즉시 reject 또는 malformed | Publisher 입력 경량화, portable JSON 형식, parser·schema 진단 보강 | 유지 |
| 4.0.5 | 15턴·14턴 연속 삭제 뒤 이전 15턴을 평론가가 붙잡음 | source revision·logical turn 기반 연속 rollback과 수동 삭제 수명주기 보강 | 유지 |
| 4.0.6 | NeuralWatt·OpenCode 요청량은 기록되지만 `custom returned no text content` | 문자열·text block·legacy text·명시적 Responses API 응답 정규화 | 유지 |
| 4.0.6 | 콜드 스타트 뒤 한국인 성씨가 섞임 | 명시된 이름 매칭 표를 평론가 identity 지침에 반영 | 유지 |
| 4.0.6 | 사용자 입력만 지웠는데 assistant 출력이 있는 턴도 삭제됨 | assistant message/generation/content hash 관측으로 출력 존재 턴 유지 | 유지 |
| 4.0.7 | 입력 수정·설정 토글 뒤 리롤하면 삭제 감지와 초록 HUD가 장시간 정지 | rollback HTTP를 durable queue 경계에서 종료, bounded worker·공정성·중단 HUD 적용 | 유지 |
| 4.0.7 | 새 `replacement_pending` 보호가 정상 턴까지 무효화 | 해당 상태·판정·HUD를 제거하고 기존 교체 계보로 복귀 | 제거 완료 |
| 4.0.7 | 콜드 스타트가 복원 직후 정상 턴을 스스로 rollback | 첫 관측 rollback 금지, blind-tail 제한 복원, 중복 tail 비교 제거 | 수정 완료 |
| 4.0.7 후속 | 콜드 스타트 후 주관 기억은 있으나 개체 정보가 0건 | `character_name`/`character` 별칭을 `name`으로 투영하고 실패한 committed projection을 재개 가능하게 수정 | 수정 완료 |
| 4.0.8 | UI timeout을 늘려도 출판사 60초·평론가 90초로 고정 | 각 LLM의 millisecond 설정을 backend 실제 호출 제한에 직접 연결 | 소스·회귀 완료 |
| 4.0.8 | 주관 기억 중요도가 전부 `5.0`으로 보임 | belief 기반 주관 기억에도 항목별 중요도·감정 가중치 전달 | 신규 데이터부터 적용 |
| 4.0.8 | 과거의 은 15냥 같은 KG가 모두 `현재 유효` | 열린 KG를 `종료 미기록` 이력으로 표시·전달하고 현재 상태 권위와 분리 | 부분 해결, 완전한 supersession은 후속 |
| 4.0.8 | NeuralWatt Flex를 출판사·평론가에서 사용하고 싶음 | 전용 Provider, endpoint, service tier, SSE 응답 조립 추가 | 소스·회귀 완료, 실계정 미확인 |
| 4.0.8 | 세계 규칙이 180자에서 문장 중간 절단 | 조립 전 고정 축약 제거, 완전한 항목을 최종 예산 선택기로 전달 | 소스·회귀 완료 |
| 4.0.8 후속 | 긴 세션에서 기존 기본값이 부족해짐 | 새 기본값을 timeout 120초, completion 30,000, 일반 기억 18,000자로 상향 | 현재 소스 반영, 패키지 재생성 필요 |

## 4. 버전별 상세 기록

### 4.0.2 — 로어북 요청 호환과 admission hash 수렴

#### 피드백

- 4.0.1 사용자의 로어북 snapshot 요청에서 문자열 `bookVersion`이 Go의
  `int64` 역직렬화에 실패했다.
- Voyage Context 4 등을 사용한 재색인에서 모든 턴이
  `committed_derived_result_hash_mismatch`로 막힌다는 제보가 있었다.

#### 원인과 수정

- `bookVersion`을 숫자만 허용하던 요청 계약을 숫자 문자열까지 수용하도록
  정규화했다. 비어 있거나 해석할 수 없는 선택 필드는 snapshot 전체를
  실패시키지 않고 `nil`로 처리한다.
- 저장 직전 Go slice와 JSON 재로딩 후 `[]any`가 서로 다른 정규화 과정을
  거치던 해시 계산을 동일한 canonical result JSON 문자열 기준으로 통일했다.
- 변조 감지는 유지했다. 해시 불일치를 무조건 성공 처리하지 않았고 Voyage
  또는 특정 embedding 모델 예외도 만들지 않았다.

#### 근거

- `0b63af7` — legacy lorebook `bookVersion` 허용
- `d80b2d5` — durable memory admission JSON hash 일치
- `ed8fcba`, 태그 `v4.0.2` — 공식 4.0.2 릴리스

### 4.0.3 — 브라우저 설정 영속성과 고정 활성 약속

#### 피드백

- Android Firefox에서 저장 또는 Backend URL 설정이 다시 loopback 기본값으로
  돌아간다는 제보가 있었다.
- 수백 턴 세션에서 “매일 순간이동 큐브로 합류” 같은 정기 약속과 도구의
  사용 맥락이 장기 진행 뒤 전달되지 않는 사례가 있었다.

#### 원인과 수정

- Firefox 전용 분기 대신 plugin storage, device-local storage와 검증된 복원
  순서를 브라우저 공통 경로로 정리했다. 쓰기 성공을 확인할 수 없는 저장은
  성공으로 표시하지 않는다.
- `pending_threads` 가운데 사용자가 직접 고정했고 `open`이며 숨기지 않은
  항목은 현재 입력과 단어가 직접 겹치지 않아도 전달 후보에 남긴다.
- 별도 무제한 예산을 만들지 않았으며 기존 일반 기억 예산 안에서 선택한다.
- `paused`, `resolved`, `suppressed` 항목은 고정됐더라도 이 경로로 강제하지
  않는다.
- 자동 평론가 갱신이 사용자의 `pinned`, `suppressed`, `user_corrected` 값을
  덮어쓰지 않게 했다.

#### 경계

- 반복 주기 계산, 약속 이행·누락·취소 판단, 아이템 기능·입수 경로의 완전한
  묶음 복원은 구현하지 않았다.
- 위 전면 개선은 후속 기억 단계의 범위다.

### 4.0.4 — 재색인 수렴과 Vector 삭제 대기열 안정화

#### 피드백

- 공개 기억이 ChromaDB에는 있어도 MariaDB `memories.embedding`이 비어
  force 재색인이 완료되지 않았다.
- 롤백·재생성 뒤 같은 문서 삭제가 사유와 결과 해시마다 새 operation으로
  쌓여 112턴에서 352,405건까지 증가했다.

#### 수정

- 임베딩 생성 → Chroma upsert → 정확 readback 뒤 같은 MariaDB 트랜잭션에서
  기억 행의 embedding/model 갱신과 outbox 완료를 처리한다.
- delete key를 `delete + session + source_revision + document_id`로 수렴했다.
  upsert는 내용 변경을 구분하기 위해 기존 결과 해시를 유지한다.
- 같은 세션·revision·document, 비활성 source, 미완료, 활성 lease 없음 조건을
  모두 만족하는 중복 delete만 `stale_rejected`로 종료한다.
- delete claim 최대 128건, 정리 transaction 512건 단위와 delete lane
  공정성을 적용했다.

#### 경계

- ChromaDB를 정본으로 승격하지 않았다.
- 서로 다른 revision, 완료 이력, 실행 중 lease를 합치거나 물리 삭제하지 않았다.
- 자동 시작 시 전체 DB를 무조건 정리하지 않는다.

### 4.0.5 — Publisher 경량화·JSON 호환과 연속 삭제

#### 피드백

- LLM Gateway의 Grok·GLM 등에서 Publisher 요청이 매우 빠르게 reject되거나
  malformed 응답으로 실패했다.
- 모델 입력에 중복 출력 요구, 들여쓰기 JSON, 관리용 metadata가 많았다.
- 15턴 삭제 후 14턴도 삭제하고 새 14턴을 생성했을 때 평론가가 이전 15턴을
  계속 사용했으며 기억 관리의 수동 삭제도 완료되지 않았다.

#### Publisher 수정

- 사용자 payload의 중복 `required_output`을 제거하고 시스템 프롬프트와
  Provider 형식이 출력 계약을 소유하게 했다.
- 모델 입력을 compact JSON으로 바꾸고 `publisher_strength_profile`, 고정
  count/status, 중복 감사 metadata를 모델용 복사본에서만 제외했다.
- 실제 전달 텍스트, source reference, privacy·authority guard와 `must_*`
  지침은 유지했다.
- direct OpenAI는 기존 strict `publisher_output.v3` schema를 유지하고,
  여러 상류 모델을 중계하는 gateway에는 portable `json_object`를 사용했다.
- parser 오류, syntax 위치, 최상위 객체 수, 종료 사유, 제한된 raw preview와
  prompt chars를 진단에 남겼다.

#### 삭제 수정

- 연속 삭제와 UI 수동 삭제가 이전 source revision·logical turn과 충돌하지
  않도록 rollback decision과 source 수명주기를 정리했다.
- 새 결과는 기존 logical turn 교체 경로를 사용하고 이전 revision을
  `superseded`, 새 revision만 `active_final`로 유지한다.

#### 유지한 계약

- Publisher 단일 호출
- malformed 시 본문 요청 fail-open
- 숨겨진 재시도 없음
- 기억·로어북 선택과 DB schema 변경 없음

### 4.0.6 — OpenAI 호환 응답, 이름 매칭, 사용자 입력만 삭제된 턴

4.0.6은 별도 릴리스 태그가 없다. 4.0.5 복구점 이후와 4.0.7 작업 전 소스,
보존 패키지와 회귀에서 사후 복원한 작업 구간이다.

#### Custom/OpenAI 호환 응답

- 문자열 `content`, text block 배열, legacy `text`를 공통 텍스트로 읽는다.
- endpoint가 명시적으로 `/responses`인 경우 Responses API의 `output_text`를
  처리한다.
- `finish_reason`, termination kind, 입력·출력·reasoning token을 연결 테스트
  진단까지 보존한다.
- reasoning만 있고 최종 텍스트가 없는 length 종료는 연결 실패와 구분하며,
  reasoning을 평론가 최종 JSON으로 오인하지 않는다.
- 연결 테스트의 지나치게 작은 출력 예산을 실제 응답 가능한 값으로 바꿨다.

#### 이름 매칭

- 사용자가 명시한 이름 표가 있을 때 given name과 surname 대응을 identity
  필드에 보존한다.
- 다른 행의 성을 섞거나 새 성을 만들지 않도록 평론가 지침을 보강했다.
- 이름 표 자체를 새 인물·사건의 근거로 사용하지 않는다.

#### 사용자 입력만 삭제된 턴

- assistant 출력이 남아 있으면 사용자 입력만 사라졌다는 이유로 턴 전체를
  삭제하지 않는다.
- assistant message ID, generation ID, content hash를 backend active source와
  대조한다.
- assistant 출력이 실제로 삭제됐을 때의 canonical 삭제는 유지한다.

### 4.0.7 — 리롤 오인 삭제, 비동기 rollback과 후속 보호 제거

#### 피드백

- 입력 수정 또는 설정 토글 뒤 리롤하면 삭제 감지가 시작되고 대규모 Vector
  outbox에서 HUD가 오랫동안 초록색 상태로 멈췄다.
- 최초 4.0.7 테스트에서는 콜드 스타트로 턴을 정상 복원한 뒤 여러 턴을 다시
  자동 rollback했다.

#### 유지되는 수정

- rollback HTTP는 MariaDB source revision 무효화와 durable Vector outbox
  등록까지만 완료하고 `vector_cleanup=queued`로 끝난다.
- 한 wake에서 재처리 4건, Vector group 8개, delete group 128건, deferred
  upsert sibling 32건으로 제한한다.
- Vector group 4개마다 delete lane에 처리 기회를 주어 오래된 upsert 뒤에서
  delete가 계속 굶지 않게 한다.
- worker stop 대기는 5초 제한을 가지며 timeout·연결 중단은 무한 진행 대신
  중단·재시도 가능 HUD로 끝난다.

#### 제거한 잘못된 보호

- 새로 만들었던 `replacement_pending` decision, source 상태, JavaScript 성공
  취급과 HUD를 모두 제거했다.
- 정상 리롤은 기존 `ReplaceLogicalTurn` 경로만 사용한다.
- 첫 Host 관측에서는 persisted ledger 길이만으로 rollback하지 않고 현재
  snapshot 기준만 잡는다.
- 같은 host signal에서 backend tail을 앞뒤로 두 번 비교하던 호출을 제거했다.
- 4.0.5까지 존재했던 blind-tail gap 제한을 복원했다.

#### 건드리지 않은 것

- 4.0.0 이전부터 존재한 `/del`, `/cut`의 명시적 삭제
- 사용자 입력만 삭제된 턴 유지
- assistant 출력 실제 삭제의 canonical rollback
- 기억 선택, 로어북, Publisher/Critic 계약과 DB schema

### 4.0.7 후속 — 콜드 스타트 개체·상태 투영 복구

#### 피드백

- 콜드 스타트 뒤 주관 기억은 생성되지만 기억 화면의 개체 정보가 0건이었다.

#### 원인과 수정

- 일부 Provider의 `character_deltas`가 `name` 대신 `character_name` 또는
  `character`, 구조화된 `status` 대신 `change`·`delta_type`을 반환했다.
- 평론가 결과 자체는 committed됐지만 개체·상태 투영기가 `missing_name`으로
  건너뛰었고, 정상화는 이미 완료된 projection으로 오인해 다시 시도하지 않았다.
- Go 정규화기가 이름·상태 별칭을 기존 표준 필드로 투영한다.
- committed result의 JSON과 hash는 다시 쓰지 않는다. replay용 로컬 복사본만
  정규화해 기존 평론가 결과로 누락된 파생 상태를 재생한다.
- 과거 trace에 recoverable `character_deltas/missing_name`이 있고 committed
  결과에 이름 별칭이 실제로 존재할 때만 projection을 미완료로 판정한다.
- 진짜 이름이 없는 항목은 무한 재처리하지 않는다.

복구점 `f18b532`에 이 수정과 회귀가 포함돼 있다.

### 4.0.8 — 설정 연결과 기억 의미 보강

#### 출판사·평론가 timeout 연결

- 중복된 backend 60초·90초 설정이 각 LLM의 millisecond 설정을 덮던 경로를
  제거했다.
- 출판사는 `pluginMainTimeoutMs`, 평론가는 `subLlmTimeoutMs`를 runtime config와
  실제 호출에 그대로 전달한다.
- 공통 설정의 중복 timeout 입력은 제거했다.

#### 주관 기억 중요도·감정 가중치

- `belief_updates`에서 주관 기억을 만들 때 `importance_10`,
  `emotional_weight`와 기존 별칭을 항목별로 전달한다.
- 점수가 없는 항목만 기존 기본값 `5 / 0.5`를 사용한다.
- 하나의 점수 누락 때문에 다른 정상 항목이나 턴 전체를 버리지 않는다.
- 기존 DB의 `5 / 0.5`를 근거 없이 자동 변경하지 않는다.

#### KG 시간 의미

- `valid_to`가 비어 있는 KG를 `현재 유효`가 아니라 `종료 미기록`으로 표시한다.
- 본문 모델에는 KG를 현재 상태 권위가 아닌 source turn과 유효 범위가 붙은
  관계·사건 지원 이력으로 설명한다.
- 아직 시작하지 않은 항목과 기준 턴에서 이미 종료된 항목은 전달하지 않는다.
- 오래된 열린 약속·관계는 단순히 오래됐다는 이유로 삭제하지 않는다.
- 돈·소유·위치·신분처럼 변하는 값은 기존 `state_claims`로 보내도록 평론가
  객체 예시를 보완했다.
- KG끼리의 의미를 추측해 같은 predicate를 자동 종료하거나 통합하지 않는다.

#### NeuralWatt Provider

- 출판사·평론가 Provider에 `NeuralWatt`를 추가하고 기본 endpoint를
  `https://api.neuralwatt.com/v1`로 제공한다.
- Standard는 OpenAI 호환 JSON 경로를 사용한다.
- Flex는 `service_tier=flex`, `stream=true`, usage 요청을 보내며 Go가 SSE의
  본문·reasoning·usage·finish reason·service tier를 표준 응답으로 조립한다.
- completion chunk 또는 완료 표시 없이 끊긴 스트림을 부분 결과로 저장하지
  않으며 Standard로 숨겨 재호출하지 않는다.

#### 세계 규칙 전달

- 조립 초기에 세계 규칙을 180자로 잘라 문장 중간이 손상되던 코드를 제거했다.
- 완전한 세계 규칙 항목을 최종 예산 선택기에 전달한다.
- 예산이 부족하면 기존 항목 단위 선택·제외를 사용하고 문자열 중간을 잘라
  맞추지 않는다.
- 다른 기억 lane의 선택 정책은 변경하지 않았다.

### 4.0.8 후속 현재 소스 — 기본 설정 상향

긴 세션 테스트 결과에 따라 새 설치·설정 누락 시 기본값을 다음으로 올렸다.

| 설정 | 이전 기본값 | 현재 기본값 |
|---|---:|---:|
| 출판사 timeout | 60,000 ms | 120,000 ms |
| 평론가 timeout | 90,000 ms | 120,000 ms |
| 출판사 completion token | 1,024 | 30,000 |
| 평론가 completion token | 1,024 | 30,000 |
| 일반 기억 예산 | 9,000 chars | 18,000 chars |

- 원작 DB 3,000 chars와 로어북 3,000 chars 기본값은 변경하지 않았다.
- 사용자가 이미 저장한 timeout, token, 기억 예산은 자동으로 덮어쓰지 않는다.
- 새 설치, 설정 초기화 또는 해당 필드가 없는 경우에만 새 기본값을 사용한다.
- 별도 backend fallback이나 보호 정책을 추가하지 않고 기존 runtime config
  동기화 경로를 사용한다.

## 5. 핵심 기능별 현재 감사 상태

| 기능 | 현재 소유 경로 | 자동 회귀 | 현재 판정 |
|---|---|---|---|
| user/assistant 원문 저장 | `/complete-turn`, raw persist, active-final acceptance | raw·idempotency·revision 회귀 | 소스 확인 |
| 출력 기반 평론가와 파생 기억 | Critic 호출, admission, artifact saver, durable reprocessing | 부분 보존·실패 queue·replay 회귀 | 소스 확인 |
| 기억·직접 근거·KG·상태 편집/삭제 | Explorer PATCH/DELETE와 각 Go writer | HTTP·JS edit/delete 회귀 | 소스 확인 |
| 세션 연결·이동·복사·삭제 | routing/migration/session delete owner | parity·rollback·cleanup 회귀 | 소스 확인 |
| 세계선과 branch 계보 | `worldline_topology.viewmodel.v2`와 Go lineage | topology·presentation 회귀 | 소스 확인 |
| 콜드 스타트 중단 재개 | canonical raw replay와 failed/deferred turn 목록 | long-session resume·중복 방지 회귀 | 소스 확인 |
| 사용자 입력만 삭제된 턴 유지 | assistant identity와 active revision 대조 | user-only deletion 회귀 | 소스 확인 |
| assistant 출력 실제 삭제 | canonical rollback과 durable outbox | rollback·queue 회귀 | 소스 확인 |
| 콜드 스타트 개체 투영 | character delta 별칭 정규화·committed replay | missing-name 재개 회귀 | 소스 확인 |

`소스 확인`은 실제 장기 사용자 DB에서 모든 항목이 정상이라는 뜻이 아니다.

## 6. 패키지와 릴리스 기록

| 버전 | 산출물 | 상태 | 비고 |
|---|---|---|---|
| 4.0.2 | 공식 태그·릴리스 | 공식 릴리스 | 로어북·hash 수정 포함 |
| 4.0.3 | Windows 관리형 테스트 ZIP | green | 고정 약속 보강, 장기 사용자 체감 미확인 |
| 4.0.4 | Windows 관리형 테스트 ZIP | green | source 회귀 완료, 실제 대규모 outbox 미확인 |
| 4.0.5 | Windows 관리형 테스트 ZIP | green | 실제 Grok/LLM Gateway 품질은 별도 |
| 4.0.6 | 별도 태그 없음 | 작업 구간 복원 | 독립 릴리스 완료로 간주하지 않음 |
| 4.0.7 | Windows 관리형 테스트 ZIP | green | 최종 보호 제거·cold-start rollback 수정 포함 |
| 4.0.8 | Windows 관리형 테스트 ZIP | green | NeuralWatt·기억 의미·세계 규칙 수정까지 포함 |

4.0.8 테스트 ZIP 기록:

- 위치: `_test-builds/Archive-Center-4.0.8-neuralwatt-memory-windows-test`
- ZIP SHA-256: `ec29ec8802cc783cfb8784666dd612b461b1c1ab9b766f4fc62c5cff3ab63a6b`
- 생성 당시 패키지 `Archive Center.js` SHA-256:
  `772716bf5d90c3b658b5419a98544e6895fe9372b2ccb9f54d907ac7dfe24f09`

방금 적용한 기본 설정 상향 뒤 현재 활성 `Archive Center.js` SHA-256은
`d413092c6e9a27bc993df7fac105886a3eea3bea921900233b06967bb585ba66`이다.
따라서 기존 4.0.8 테스트 ZIP에는 새 120초·30,000 token·18,000 chars 기본값이
아직 포함되지 않는다. 패키지를 다시 갱신하기 전에는 현재 소스와 기존 ZIP을
동일한 산출물로 취급하면 안 된다.

## 7. 전 구간에서 유지한 원칙

- 모델명 allowlist를 만들지 않는다.
- Go 백엔드가 정책, 파싱, 실패 판정, 기억 선택, 예산, 수명주기와 저장을
  소유한다.
- `Archive Center.js`에 병렬 기억 정책을 추가하지 않는다.
- Publisher와 Critic 실제 호출에 숨겨진 재시도를 넣지 않는다.
- 잘린 JSON을 합성 복구해 정상 결과처럼 저장하지 않는다.
- 항목 하나의 누락이나 오류 때문에 정상 항목과 전체 결과를 함께 버리지 않는다.
- 과도한 보호 상태, 병렬 fallback, 새 캐시·watcher·queue를 추측으로 추가하지
  않는다.
- DB schema와 새 테이블은 재현된 필요 없이 추가하지 않는다.
- 기존 기억 종류, 출력 언어 계약과 branch lineage를 바꾸지 않는다.
- 과거 기억과 KG 이력은 삭제하지 않고 현재 전달 의미만 구분한다.
- 사용자 저장값과 수동 수정값을 자동 작업이 임의로 덮어쓰지 않는다.

## 8. 아직 남아 있거나 실환경 확인이 필요한 부분

### 현재 바로 남은 운영 작업

1. 새 기본값이 포함되도록 4.0.8 테스트 패키지를 다시 갱신해야 한다.
2. 갱신 뒤 패키지와 활성 `Archive Center.js` hash 일치를 다시 확인해야 한다.
3. 전체 OS 정식 4.0.8 자산과 GitHub 업로드는 아직 완료로 기록하지 않는다.

### 실환경 확인

- 실제 NeuralWatt Standard/Flex 호출의 usage·finish event·과금 표시
- 사용자 설정 timeout이 느린 실제 Provider 호출에 그대로 적용되는지
- 신규 주관 기억 점수가 실제 세션에서 다양하게 생성되는지
- 기존 `5 / 0.5` 데이터의 명시적 재처리 시 중복 없이 투영되는지
- 긴 branch에서 KG가 현재 상태가 아닌 시간 지원 이력으로 전달되는지
- 실제 MariaDB·ChromaDB 대규모 재색인과 delete backlog 수렴
- Android Firefox를 포함한 실제 브라우저별 설정 저장·복원

### 후속 버전으로 남긴 의미 개선

- KG 사이의 canonical identity와 명시적 supersession을 사용한 완전한 최신
  사실 판정은 후속 관계·상태 작업이 필요하다.
- 반복 약속의 예정·이행·누락·취소와 아이템 기능·입수 경로 결합은 4.0.3의
  최소 고정 전달 범위를 넘어선다.
- 간헐적인 `PUBLISHER_PLAN_NO_VALID_ITEMS`는 Provider가 응답했지만 유효한
  Publisher item이 없었던 경우다. 현재는 본문 fail-open을 유지하며, 유효하지
  않은 항목을 억지로 출력하거나 저장하는 fallback은 추가하지 않았다.

## 9. 실사용 재검증 순서

1. 새 세션 1턴에서 user/assistant 원문 2건과 평론가 파생 개체를 확인한다.
2. 평론가 실패 턴에서 원문은 남고 파생 저장 0건과 재처리 표시가 남는지 본다.
3. 콜드 스타트를 중단·재실행해 성공 턴은 건너뛰고 실패·누락 턴만 재개하는지
   확인한다.
4. 콜드 스타트 후 개체 정보와 주관 기억이 함께 생성되는지 확인한다.
5. 기억·직접 근거·KG·인물·상태를 각각 수정하고 재조회한다.
6. 동일 항목을 삭제하고 MariaDB·Vector outbox가 수렴하는지 확인한다.
7. 사용자 입력만 삭제한 경우 턴이 유지되고 assistant 출력 삭제에서는 실제
   rollback이 실행되는지 확인한다.
8. 입력 수정 리롤과 설정 토글 리롤에서 이전 revision이 `superseded`, 새 출력이
   `active_final`이 되는지 확인한다.
9. 일반 branch와 다중 branch에서 부모 기억 상속, 기준 턴과 표시 노드를 확인한다.
10. 출판사·평론가 timeout 120초와 completion 30,000 설정이 backend HUD와
    Provider 호출 장부에 같은 값으로 남는지 확인한다.
11. 일반 기억 기본값 18,000 chars가 원작 DB·로어북 별도 예산을 바꾸지 않는지
    확인한다.

## 10. 근거 문서와 소스

역사 자료:

- `docs/archive-center-4.0.2-work-log.md`
- `docs/archive-center-4.0.3-work-log.md`
- `docs/archive-center-4.0.4-work-log.md`
- `docs/archive-center-4.0.5-work-log.md`
- `docs/archive-center-4.0.6-work-log.md`
- `docs/archive-center-4.0.7-work-log.md`
- `docs/archive-center-4.0.7-current-state-handoff.md`
- `docs/archive-center-4.0.8-work-log.md`
- `docs/archive-center-4.0.2-4.0.7-feedback-work-record.md`

주요 활성 소스:

- `Archive Center.js`
- `go-service/internal/httpapi/group_admin_rescan.go`
- `go-service/internal/httpapi/group_turn_range_decision.go`
- `go-service/internal/httpapi/group_turn_rollback.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor.go`
- `go-service/internal/httpapi/prepare_turn_assembly.go`
- `go-service/internal/httpapi/prepare_turn_recall.go`
- `go-service/internal/httpapi/proxy_provider.go`
- `go-service/internal/httpapi/turn_extraction_critic.go`
- `go-service/internal/httpapi/turn_extraction_private.go`
- `go-service/internal/httpapi/turn_extraction_character_state.go`
- `prompts/critic_system.txt`

이 문서는 작업을 없던 일로 만들거나 과거 문서를 삭제하기 위한 요약이 아니다.
4.0.2 이후 어떤 피드백으로 무엇을 바꿨고, 무엇을 다시 제거했으며, 현재 무엇이
소스·패키지·실사용 중 어느 단계까지 검증됐는지를 한 번에 이어 보기 위한 기준이다.
