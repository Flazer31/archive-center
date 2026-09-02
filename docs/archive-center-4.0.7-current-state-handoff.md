# Archive Center 4.0.7 현재 작업 상태 인계서

기준 시각: 2026-08-26 KST  
활성 소스: `<archive-center-root>\source`
활성 브랜치: `agent/fix-voyage-context-batching`

이 문서는 4.0.7 리롤·삭제·Vector 정리 작업과 실사용 후속 수정을 다른 작업과 섞지 않고 이어가기 위한 현재 상태 기록이다. `replacement_pending` 보호와 콜드 스타트 자동 rollback 결함은 제거·수정됐고 소스 회귀와 Windows 테스트 패키지가 갱신됐다.

## 1. 기준점과 작업 트리 주의사항

- 현재 HEAD는 `9f1ba7eaf0a087162f5da53841ef7106c9de0deb`이다.
- 커밋 제목은 `checkpoint: preserve Archive Center 4.0.5 before turn deletion repair`이다.
- 4.0.7 변경은 아직 별도 커밋으로 고정하지 않은 작업 트리 변경이다.
- 현재 작업 트리에는 이번 4.0.7 작업뿐 아니라 이전 Publisher, Custom provider, 평론가 이름 정규화, 로드맵 작업 등 기존 미커밋 변경도 함께 있다.
- 따라서 다음 작업자는 전체 파일을 되돌리거나 과거 체크포인트를 덮어쓰면 안 된다. 함수와 파일 단위로 이번 범위를 구분해야 한다.
- `_test-builds`는 생성물 위치이며 활성 런타임 소스의 기준은 계속 `source`이다.

## 2. 이번 문제의 확정 원인

입력 수정 또는 설정 변경 뒤 리롤할 때 `beforeRequest` 시점의 임시 상태를 실제 턴 삭제로 판정하는 문제와, 콜드 스타트 직후 일부만 보이는 활성 채팅을 `full_active_chat`으로 간주해 backend tail과 비교하는 문제가 함께 있었다. 4.0.6 작업에서 4.0.5까지 있던 blind-tail 제한이 빠져 backend gap만으로 rollback이 실행될 수 있었다. 대기열이 큰 환경에서는 이어지는 rollback HTTP가 Vector 정리를 동기적으로 오래 수행해 HUD가 초록색 진행 상태로 멈춘 것처럼 보일 수 있었다.

`beforeRequest`는 provider 요청 전 관측일 뿐 새 최종 출력의 저장이나 기존 출력의 영구 삭제를 증명하지 않는다. 공식 RisuAI 확인 기준은 로컬 audit checkout의 커밋 `72ce721878d65b09baf4339638dfd221d1788261`이다.

또한 동일 턴 리롤을 사용자 입력 문자열로 판정하면 안 된다. 입력 수정 리롤에서는 문자열 자체가 달라지기 때문이다. 이번 수정은 세션, 관측 턴 위치, logical turn, source revision과 generation 계보를 사용한다.

## 3. 구현 완료 범위

### 3.1 리롤 교체 수명주기

- 4.0.7에서 새로 추가했던 `replacement_pending` decision, source-acceptance 기록, pending HUD와 JS 성공 취급은 제거했다.
- 새 최종 출력이 확정되면 기존 `ReplaceLogicalTurn` 경로를 사용한다.
- 이전 revision은 `superseded`, 새 revision만 `active_final`이 된다.
- 4.0.6까지의 사용자 입력만 삭제된 턴 유지와 assistant source 대조를 보존했다.
- 첫 관측은 persisted ledger와 현재 창 길이만으로 rollback하지 않고 snapshot 기준만 설정한다.
- host signal은 같은 실행의 메시지 변화 판정을 한 번만 수행하며 독립적인 backend tail reconcile을 앞뒤로 반복하지 않는다.
- 4.0.5까지 있던 blind-tail gap 제한을 복원했다.
- 보호 제거 과정에서 범위를 잘못 넓혀 빠졌던 기존 detector 343줄은 보호 제거 전 패키지와 대조해 전부 복원했다.
- 4.0.0 이전부터 존재한 `/del`·`/cut`과 기존 rollback 보호는 수정하지 않았다.

주요 소스:

- `go-service/internal/httpapi/group_turn_range_decision.go`
- `go-service/internal/httpapi/complete_turn_source_acceptance.go`
- `go-service/internal/httpapi/group_turn_rollback.go`

### 3.2 롤백 HTTP와 Vector 정리

- rollback HTTP는 MariaDB canonical/source revision 무효화와 durable Vector outbox 등록까지만 수행한다.
- HTTP 요청 안에서 ChromaDB 정리를 끝까지 기다리지 않는다.
- 응답은 `vector_cleanup=queued`, `drain_attempted=false`, `processed=0`을 사용한다.
- source worker가 제한 시간 안에 멈추지 않으면 canonical mutation 전에 `interrupted`, `retryable=true`로 종료한다.
- source worker 종료 대기 제한은 현재 5초다.

주요 소스:

- `go-service/internal/httpapi/group_turn_rollback.go`
- `go-service/internal/httpapi/complete_turn_source_acceptance.go`

### 3.3 Worker 제한과 공정성

- 한 번의 wake에서 재처리는 최대 4건, Vector group은 최대 8개를 처리한다.
- Vector group 4개마다 delete lane에 우선 처리 기회를 준다.
- delete claim은 최대 128건, deferred upsert sibling은 최대 32건이다.
- 같은 document의 선행 작업 순서는 그대로 유지한다.
- claim, 완료, 실패마다 현재 시각을 다시 계산한다.
- 실패 작업은 최소 1초 뒤 다시 claim할 수 있다.
- Vector provider timeout은 lease보다 짧게 제한해 DB 완료·실패 상태를 기록할 시간을 확보한다.
- 한 wake의 제한에 도달하면 다음 wake를 예약하여 작업을 계속한다.

주요 소스:

- `go-service/internal/httpapi/memory_reprocessing_worker.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor.go`
- `go-service/internal/store/memory_derivation.go`
- `go-service/internal/store/mariadb_memory_derivation.go`
- `go-service/internal/store/dualwrite.go`

### 3.4 HUD와 JavaScript 경계

- `Archive Center.js`는 Host 관측 전달, 백엔드 응답 적용, HUD 표시만 담당한다.
- 리롤·삭제 정책과 logical turn 판정은 JavaScript에 추가하지 않았다.
- `replacement_pending` 전용 표시는 제거했다.
- rollback transport는 설정된 timeout을 사용한다.
- 백엔드 재시작, 연결 중단, 응답 누락, worker stop timeout은 무한 초록색이 아니라 중단·재시도 가능 상태로 끝난다.
- 버전 표시는 `4.0.7`로 갱신했다.

주요 소스:

- `Archive Center.js`

## 4. 변경하지 않은 영역

이번 범위에서는 다음을 의도적으로 변경하지 않았다.

- 일반 기억 검색·선택·예산
- 원작 DB와 로어북 선택·예산
- Publisher와 Critic의 프롬프트 및 출력 계약
- 기억 종류와 출력 언어 계약
- DB schema와 새 테이블
- 모델명 또는 provider allowlist
- 숨겨진 LLM 재시도
- 전체 Vector 작업의 fail-open 또는 무조건 정상 처리

## 5. 회귀 테스트 범위

다음 동작을 회귀로 고정했다.

- `replacement_pending` decision과 source-acceptance 기록이 더 이상 존재하지 않음
- 입력 문자열이 바뀐 리롤도 같은 logical turn을 교체함
- 새 final이 active가 되고 이전 revision이 superseded가 됨
- generation/final confirmation 중 삭제가 차단됨
- rollback HTTP가 Vector provider를 동기 호출하지 않고 queued로 응답함
- 오래된 upsert 뒤의 delete가 reserved service를 받음
- MariaDB delete lane과 upsert/delete batch 제한이 적용됨
- source worker 종료 대기가 제한 시간 뒤 오류로 종료됨
- JS가 pending, transport 중단, backend 응답 누락을 terminal HUD로 정리함

관련 테스트 파일:

- `go-service/internal/httpapi/group_turn_range_decision_test.go`
- `go-service/internal/httpapi/complete_turn_source_acceptance_test.go`
- `go-service/internal/httpapi/group_turn_part08_test.go`
- `go-service/internal/httpapi/memory_admission_worker_test.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor_test.go`
- `go-service/internal/store/mariadb_memory_derivation_test.go`
- `ops/js-route-smoke.js`

## 6. 완료된 검증

- `go test ./... -count=1`: 전체 통과
- 활성 `Archive Center.js` Node 문법 검사: 통과
- JavaScript route smoke: 통과
- Go `internal/httpapi`, `internal/store` 회귀: 통과
- `git diff --check`: 내용 오류 없음. Windows 줄바꿈 경고만 존재
- 공식 RisuAI lifecycle 소스 확인: 완료

이 결과는 소스와 단위 회귀 증거다. 실제 RisuAI, 실제 MariaDB/ChromaDB, 대규모 사용자 데이터, 전체 OS 자동 업데이트의 실환경 증거와는 구분한다.

## 7. 현재 4.0.7 테스트 패키지

Windows managed 테스트 패키지:

`<archive-center-root>\source\_test-builds\Archive-Center-4.0.7-reroll-worker-windows-test\Archive Center 4.0.7 Windows Auto Install Package.zip`

현재 검증 결과:

- 버전: `4.0.7`
- `release_ready=true`
- `automatic_update_apply=true`
- ZIP 크기: `12,121,593 bytes`
- ZIP SHA-256: `1466BCAFCFA01BCDDC0624CDD0DDAB598D0D81894A7F1499F2A827D9DD81F373`
- 관리 파일: 46개
- 누락: 0개
- 크기 불일치: 0개
- SHA-256 불일치: 0개
- 패키지의 `Archive Center.js` SHA-256은 활성 소스와 같은 `1068BC9B635243238705C27C2F8E5764047265458D9878E5A439F9128C94D92A`다.
- `replacement_pending` 제거와 콜드 스타트 첫 관측·blind-tail 수정이 포함돼 있다.

현재 생성된 것은 Windows 테스트 패키지다. 전체 OS 정식 릴리스 패키지 생성과 GitHub 업로드를 완료했다는 의미는 아니다.

## 8. 다음 실환경 확인 순서

1. 일반 리롤이 기존 턴을 superseded하고 새 출력만 active_final로 남는지 확인
2. 사용자 입력 수정 후 리롤 확인
3. 설정 토글 후 리롤 확인
4. 생성 실패와 사용자 취소 후 assistant가 실제로 없을 때 삭제 검증 확인
5. 대규모 Vector outbox 환경에서 rollback HTTP가 즉시 queued로 끝나는지 확인
6. delete가 오래된 upsert 뒤에서 계속 밀리지 않는지 확인
7. backend 중단·재시작과 ChromaDB 연결 실패에서 HUD가 재시도 가능 상태로 종료되는지 확인
8. MariaDB에서 source revision의 `active_final`/`superseded` 계보와 outbox 상태 확인

## 9. 다음 작업자가 지켜야 할 중단선

- 실환경 재현 없이 리롤·삭제 경로에 새로운 fallback이나 보호 조건을 덧붙이지 않는다.
- `beforeRequest`를 삭제 확정 신호로 다시 사용하지 않는다.
- 사용자 입력 문자열 일치로 동일 턴을 판정하지 않는다.
- rollback HTTP에서 Vector outbox 전체를 동기 drain하지 않는다.
- 완료 판정을 빠르게 만들기 위해 Vector 오류를 무조건 성공 처리하지 않는다.
- 이번 문제를 이유로 기억 선택, 로어북, Publisher, Critic을 함께 수정하지 않는다.
- 정식 패키지 또는 GitHub 업로드는 실환경 확인 뒤 별도 작업으로 진행한다.

## 10. 관련 기록

- `docs/archive-center-4.0.7-work-log.md`
- `docs/archive-center-4.0.2-4.0.7-feedback-work-record.md`
- `STRUCTURE.md`
- `AI_GUARDRAILS.md`
- `docs/permanent-risu-host-backend-boundary.md`

이 인계서가 현재 4.0.7 작업 상태의 기준이다. 이후 변경은 검증 근거와 함께 이 문서 또는 후속 버전 문서에 추가한다.
