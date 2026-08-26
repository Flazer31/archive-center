# Archive Center 4.0.3 작업 기록

기록일: 2026-08-25  
상태: source 구현·회귀 검증 및 Windows 테스트 패키지 생성 완료

## 1. 이번 작업의 단일 목표

장기적인 반복 일정·약속 모델을 새로 만들지 않고, 사용자가 직접 고정한
활성 약속을 현재 입력의 단어가 겹치지 않는다는 이유만으로 누락하지 않는다.

이번 4.0.3 보완은 임시 연속성 보강이다. 반복 주기 계산, 기한 판정,
아이템 기능·입수 경로 복원, 자동 고정, 새 기억 종류와 새 DB 스키마는
추가하지 않는다. 해당 확장은 4.1 이후의 정식 기억 개선 범위로 남긴다.

## 2. 적용된 동작

- `pending_threads` 중 `pinned=true`, `status=open`, `suppressed=false`인 항목은
  현재 입력과 직접적인 문자열 관련성이 없어도 기존 Pending Threads 전달
  후보에 포함한다.
- 전달량은 기존 일반 기억 예산과 조립 경로를 그대로 사용한다. 고정 항목을
  위한 별도 무제한 예산이나 두 번째 주입 경로를 만들지 않았다.
- 고정되지 않은 항목은 기존 관련성 선택 규칙을 유지한다.
- `paused`, `resolved`, `suppressed` 항목은 고정되어 있어도 위 우회를 받지 않는다.
- 평론가 재추출에 따른 자동 저장은 사용자가 수동으로 설정한 `pinned`,
  `suppressed`, `user_corrected` 값을 덮어쓰지 않는다.
- UI와 `Archive Center.js`에는 선택 정책을 추가하지 않았다. 정책과 저장은
  기존 소유자인 Go 백엔드가 담당한다.

## 3. 변경 파일

### 제품 코드

- `go-service/internal/httpapi/prepare_turn_assembly.go`
  - 고정된 활성 약속의 관련성 필터 우회
  - `pending_thread_pinned_active_selected` 및
    `pending_thread_suppressed_dropped` 진단 수치
- `go-service/internal/store/mariadb_runtime_state.go`
  - 자동 pending-thread 갱신에서 사용자 신뢰 플래그 덮어쓰기 제거

### 회귀 테스트

- `go-service/internal/httpapi/prepare_turn_prompt_hygiene_test.go`
  - 관련성 없는 고정 활성 약속은 남고, paused·suppressed 항목은 남지 않는지 검증
- `go-service/internal/store/store_test.go`
  - 자동 갱신 SQL이 사용자 고정·숨김·수정 플래그를 갱신하지 않는지 검증

## 4. 의도적으로 하지 않은 작업

- 매일·매주와 같은 반복 주기 해석
- 시간대나 `story_clock`에 따른 자동 재활성화
- 약속 완료·실패·기한 경과 판정
- 순간이동 큐브 같은 물건의 기능·소유권·입수 경로 묶음 복원
- 사용자가 고정하지 않은 약속의 강제 전달
- 새 API, 테이블, worker, 캐시, 숨겨진 재시도 또는 JavaScript 정책

## 5. source 검증

- 대상 회귀 테스트: 통과
- `go test ./... -count=1`: 통과
- `Archive Center.js` 구문 검사: 통과
- `git diff --check`: 통과
- 이번 약속 보완의 JavaScript 증감: `+0 / -0`

이 검증은 source와 자동 회귀 수준이다. 실제 RisuAI에 로드한 뒤 고정된
활성 약속이 무관한 다음 입력에도 전달되는지는 갱신된 테스트 패키지에서
확인해야 한다.

## 6. 테스트 패키지

2026-08-25에 기존 4.0.1 테스트 폴더를 덮어쓰지 않고 새 Windows 관리형
테스트 패키지를 생성했다.

- output root:
  `_test-builds/Archive-Center-4.0.3-pinned-promise-windows-test`
- 설치 폴더:
  `Archive Center 4.0.3 Windows Auto Install Package`
- ZIP:
  `Archive Center 4.0.3 Windows Auto Install Package.zip`
- ZIP SHA-256:
  `5b8a851e9a56423945392302c96236ecf70071f8164b836e177a15a3f2409453`
- 패키지 상태: `green`
- 패키지 버전: `4.0.3`
- 빌드 도구: `go1.26.6 windows/amd64`
- 패키지 내부 `Archive Center.js`의 `@version`, `VERSION`, `BUILD_ID`:
  모두 `4.0.3`
- manifest 기준 포함 파일 46개, 누락 파일 0개

빌드 뒤 `go1.26.6`과 명시적인 Node 실행 경로로 `go test ./... -count=1`을
다시 실행해 전체 통과했다. 첫 재검증에서는 시스템 `PATH`에 `node`가 없어
JavaScript runtime fixture 4건이 실행 준비 단계에서 실패했으며, 저장소가
요구하는 `ARCHIVE_CENTER_NODE_BINARY`를 지정한 동일 회귀에서 모두 통과했다.

### 실사용 확인 상태

이 문제는 수백 턴을 진행한 외부 사용자 세션에서 받은 제보이며, 해당 세션과
DB를 현재 확보하지 못했다. 이미 지나간 턴의 체감을 같은 조건으로 재현할 수
없으므로 이번 변경을 `실사용 완전 해결`로 판정하지 않는다.

- source 수정: 확인 완료
- 회귀 테스트: 확인 완료
- 4.0.3 테스트 패키지 반영: 확인 완료
- 원 제보자의 기존 장기 세션 체감 개선: 미확인
- 실제 RisuAI payload 전달과 본문 모델의 약속 준수: 미확인

따라서 현 상태는 `간접 해결 확인`으로 기록한다. 같은 증상이 다시 제보되면
그 시점의 편집 확인·payload 장부·Pending Threads 상태를 받아 4.1의 정식
기억 개선 작업에서 재검증한다. 과거 턴을 소급 재처리하거나 추측으로 추가
보호 규칙을 넣지는 않는다.
