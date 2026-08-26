# Archive Center 4.0.4 작업 기록

기록일: 2026-08-26  
상태: source 구현·회귀 검증 및 Windows 테스트 패키지 생성 완료

## 1. 이번 작업의 목표

4.0.2에서 제보된 강제 재색인 수렴 실패와 삭제 outbox 증폭을 기존 Go
저장 소유자 안에서 수정한다.

- ChromaDB에 정상 반영된 공개 기억 임베딩을 MariaDB의
  `memories.embedding`, `embedding_model`에도 수렴시킨다.
- 같은 source revision·문서의 삭제 의도가 사유·결과 해시 때문에 여러
  operation key로 늘어나지 않게 한다.
- 이미 쌓인 미완료 삭제 중 안전 조건이 같은 중복만 세션 단위로
  `stale_rejected` 처리한다.
- 한 번의 삭제 claim과 ChromaDB 삭제 요청 크기를 제한한다.

## 2. 적용된 동작

### 공개 기억 임베딩 수렴

벡터 worker는 임베딩 생성, ChromaDB upsert, 정확 readback 검증 후 공개
`memories` 문서에 한해 다음을 같은 MariaDB 트랜잭션에서 수행한다.

1. outbox lease와 문서·세션·source revision을 다시 확인
2. source revision이 여전히 active인지 확인
3. 기억 행의 turn과 source revision의 turn이 같은지 확인
4. `memories.embedding`, `embedding_model` 갱신
5. outbox를 `completed`로 변경

중간 실패 시 outbox는 완료되지 않는다. source revision이 바뀐 경우에는
기존 보상 삭제를 유지한다. 공개 기억 벡터가 없는 주관 기억 전용 행은 이
경로에 들어오지 않는다.

### 삭제 operation key

- delete: `delete + session + source_revision + document_id`
- upsert: 기존 결과 해시를 포함하는 키 유지
- 삭제 사유는 operation key가 아니라 outbox의 JSON 감사 정보에 보존
- 문서별 causal guard는 다른 세션의 같은 legacy document ID가 막지 않도록
  `chat_session_id` 범위를 함께 사용

### 기존 중복 대기열 정리

관리자 재색인이 실행된 해당 세션에서만 다음 조건을 모두 만족하는 delete를
정리한다.

- 같은 세션·source revision·document ID
- `required_source_state=inactive`이고 실제 source revision도 비활성
- 미완료 상태
- 묶음 안에 현재 유효한 lease가 없음
- 실행 가능한 delete가 최소 한 건 존재

가장 오래된 실행 가능한 한 건을 남기고 나머지만 `stale_rejected`로 종료한다.
서로 다른 revision, 완료 이력, 활성 lease는 건드리지 않으며 행을 물리적으로
삭제하지 않는다. 작업은 512건 단위의 짧은 트랜잭션으로 반복한다.

### worker 처리량

- 한 번의 delete claim 최대 128건
- ChromaDB 삭제 요청 전에 동일 document ID 중복 제거
- 다음 batch에서 계속 처리
- 같은 세션·문서의 기존 causal 순서 유지

## 3. 변경 경계

- 새 DB 테이블·마이그레이션: 없음
- 새 JavaScript 정책: 없음
- 모델·provider별 예외: 없음
- ChromaDB를 정본으로 승격: 하지 않음
- `memory_rows_missing_embedding` 완료 조건 제거: 하지 않음
- 자동 시작 시 전체 DB 정리: 하지 않음
- 완료 이력 물리 삭제 또는 서로 다른 revision 병합: 하지 않음

## 4. 회귀 검증

- 깨지는 회귀를 먼저 확인:
  - 비동기 공개 기억 임베딩이 materialized MariaDB 완료를 호출하지 않음
  - delete key가 결과 해시와 사유에 따라 달라짐
- 추가 검증:
  - MariaDB 기억 임베딩 갱신과 outbox 완료가 한 트랜잭션에 있음
  - 112턴을 네 가지 사유·결과 해시로 반복해도 delete key는 112개로 제한
  - force replay 두 번이 동일 operation key·기존 행을 재사용
  - inactive·unleased·같은 revision·문서만 중복 종료
  - delete sibling claim SQL에 128건 상한 적용
  - 서로 다른 revision의 delete key는 계속 구분

## 5. 검증 단계 구분

- source 회귀: 완료
- 전체 Go 테스트: `go test ./... -count=1` 통과
- JavaScript 구문: source와 패키지 내부 파일 모두 통과
- `git diff --check`: 통과
- Windows 테스트 패키지: 생성·manifest 검증 완료
- 실제 MariaDB·ChromaDB 장기 대기열: 미확인
- 원 제보자의 Voyage Context 4 세션 재색인: 미확인
- 전체 OS 자동 업데이트 패키지: 이번 테스트 빌드 범위 아님

이번 변경의 JavaScript 증감은 버전 문자열 교체만 `+5 / -5`이며, 메모리
정책·대기열·완료 판정은 전부 Go 백엔드가 소유한다.

## 6. Windows 테스트 패키지

- output root:
  `_test-builds/Archive-Center-4.0.4-reindex-outbox-windows-test`
- 설치 폴더:
  `Archive Center 4.0.4 Windows Auto Install Package`
- ZIP:
  `Archive Center 4.0.4 Windows Auto Install Package.zip`
- ZIP SHA-256:
  `da6c8a06f9cfc01d3fd4cd7d10b8c3b6a833560814a6efd10040df7012e9a24d`
- package status: `green`
- package version: `4.0.4`
- automatic update apply metadata: `true`
- Go toolchain: `go1.26.6 windows/amd64`
- manifest 포함 파일: 46개
- 누락 파일: 0개
- 패키지 내부 `Archive Center.js`의 `VERSION`, `BUILD_ID`: `4.0.4`

첫 패키징 시도는 시스템 기본 Go가 1.26.5여서 빌드 도구가 중단했다. 저장소가
요구하는 `GOTOOLCHAIN=go1.26.6`으로 다시 실행해 정상 생성했으며, 실패한
첫 시도는 배포 산출물을 만들지 않았다.
