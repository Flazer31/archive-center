# Archive Center 4.0.7 작업 기록

기준일: 2026-08-26 KST

## 1. 작업 범위

- 입력 수정·설정 변경 뒤 리롤할 때 `beforeRequest`의 일시적인 assistant 공백을 실제 삭제로 오인하던 경로를 수정했다.
- 롤백 HTTP가 대규모 vector outbox를 동기적으로 끝까지 비우며 HUD와 요청을 붙잡던 경로를 수정했다.
- 작업 범위는 리롤 교체 계보, 롤백 응답 경계, vector worker의 제한·공정성·lease 시간, 중단 HUD, 4.0.7 버전과 테스트 패키지로 제한했다.
- 기억 선택·예산, 로어북, Publisher/Critic 계약, DB schema와 새 테이블은 변경하지 않았다.

## 2. 리롤과 삭제 수명주기

- 최초 구현의 `replacement_pending` 상태는 콜드 스타트 실사용 검사에서 정상 복원된 턴을 뒤이은 자동 rollback과 분리하지 못했으므로 제거했다.
- Go의 `replacement_pending` decision·source-acceptance 기록·특수 binding과 JavaScript의 pending HUD·성공 취급을 함께 제거했다.
- 보호 제거 전에 잘못 삭제했던 기존 rollback detector와 4.0.6 사용자 입력만 삭제된 턴 유지 코드는 보존 테스트 패키지 기준으로 전부 복원했다.
- 새 최종 출력의 정상 교체는 기존 `ReplaceLogicalTurn` 경로를 계속 사용한다. 이전 revision은 `superseded`, 새 revision만 `active_final`이 된다.
- 4.0.0 이전부터 존재한 `/del`·`/cut` 처리와 기존 detector 보호 조건은 이번 수정 범위가 아니며 변경하지 않았다.

## 3. 롤백과 vector worker

- canonical rollback은 MariaDB source revision 무효화와 durable vector outbox 등록까지만 HTTP 요청에서 완료한다.
- 응답은 `vector_cleanup=queued`, `drain_attempted=false`를 반환하며 Chroma provider를 동기 호출하지 않는다.
- worker는 한 번 깨어날 때 재처리 4건과 vector group 8개까지만 처리하고, 4개 group마다 delete lane을 우선 확인한다.
- delete group은 최대 128건, deferred upsert group은 최대 32건이다. 같은 document의 선행 작업 순서는 유지한다.
- 각 claim과 완료/실패 시각을 새로 계산하고 retry는 최소 1초 뒤 다시 eligible이 된다.
- vector provider timeout은 lease 종료 전에 DB 완료/실패 기록 시간을 남길 수 있도록 lease보다 짧게 제한한다.
- source worker 종료 대기는 5초 제한을 가지며, 초과하면 canonical mutation 전에 `interrupted`, `retryable=true`로 종료한다.

## 4. Host 및 UI 경계

- 확인한 공식 RisuAI 기준은 로컬 audit checkout의 `72ce721878d65b09baf4339638dfd221d1788261`이다.
- `beforeRequest`는 provider 요청 전 관측이며 저장된 새 최종 출력을 증명하지 않는다.
- `afterRequest` 성공과 저장 후 output-listener 경계는 별개이므로 `beforeRequest`에서 삭제를 확정하지 않는다.
- `Archive Center.js`는 Host 관측 전달, backend 응답 적용, HUD 표시만 담당한다.
- rollback transport는 설정된 요청 제한을 사용한다. 연결 중단·재시작·worker stop timeout은 무한 초록 진행 대신 실패/중단·재시도 가능 상태로 끝난다.

## 5. 회귀 검증

- `replacement_pending` decision·source-acceptance 기록·JS 처리 경로가 남지 않는 검증
- 입력 문구가 바뀐 리롤이 기존 logical turn을 교체하는 기존 검증
- 새 final의 canonical replacement 및 기존 revision supersession 기존 검증
- 사용자 입력만 삭제되고 assistant 출력이 남은 턴을 유지하는 검증
- 보호 제거 전 4.0.7 패키지에 있던 기존 rollback detector 함수와 실행 회귀를 복원한 검증
- 첫 콜드 스타트 관측에서 persisted ledger만으로 rollback하지 않는 검증
- 일부만 보이는 활성 채팅과 backend tail 차이가 blind rollback을 실행하지 않는 검증
- host signal이 독립적인 backend tail reconcile을 앞뒤로 반복하지 않는 검증
- rollback HTTP가 vector provider를 호출하지 않고 queued 응답을 반환하는 검증
- 오래된 upsert 뒤의 delete에 reserved service가 배정되는 검증
- MariaDB delete lane과 upsert/delete group 제한 검증
- source worker 종료 제한 검증

## 6. 소스 검증과 갱신된 테스트 패키지 상태

- Go 전체 패키지에 대해 `go test ./... -count=1`을 통과했다.
- 활성 소스와 패키지의 `Archive Center.js`는 Node 문법 검사를 통과했다.
- JavaScript route smoke와 Go `internal/httpapi`, `internal/store` 회귀를 통과했다.
- 전체 Go 패키지는 번들 Node 경로를 명시한 상태에서 다시 통과했다.
- 현재 수정 소스로 Windows managed 테스트 패키지를 같은 위치에 갱신했다.
- 패키지 `Archive Center.js` SHA-256은 활성 소스와 같은 `1068BC9B635243238705C27C2F8E5764047265458D9878E5A439F9128C94D92A`다.
- ZIP 크기는 `12,121,593 bytes`, SHA-256은 `1466BCAFCFA01BCDDC0624CDD0DDAB598D0D81894A7F1499F2A827D9DD81F373`다.
- 패키지 상태는 `green`, `release_ready=true`, `automatic_update_apply=true`, 누락 파일 0개다.
- 이 패키지는 Windows 실사용 검증용이며 전체 OS 정식 릴리스 완료를 뜻하지 않는다.

## 7. 아직 실환경에서 확인해야 할 것

- 실제 RisuAI/PocketRisu에서 입력 수정+리롤, 설정 토글+리롤, 생성 취소·실패 후 삭제의 Host 관측 순서
- 대규모 MariaDB outbox에서 delete 처리량과 메모리 사용량
- 실제 ChromaDB 중단·복구와 backend 재시작 중 HUD 표시
- 전체 OS 자동 업데이트 및 실제 사용자 데이터 upgrade

소스·단위 회귀·테스트 패키지는 실 RisuAI, 실 MariaDB/ChromaDB, 외부 provider, 전체 OS release 증거와 구분한다.
