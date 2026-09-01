# Archive Center 4.1.0 작업 기록

기록 기준: 2026-09-01 KST  
범위: 완료 턴의 리롤·편집 재생성 식별 복구, Windows 테스트 패키지 갱신, 오염 세션 꼬리 복구  
증거 구분: 소스·자동 회귀·패키지·실제 MariaDB/Chroma 세션 복구는 확인됨. 갱신된 4.1 플러그인의 실제 RisuAI 리롤 검증은 남아 있음.

## 1. 수정한 문제

완료 턴의 영구 식별이 편집할 때 달라질 수 있는 사용자 내용 hash·시각·인덱스를
함께 사용해, 같은 RisuAI 사용자 메시지 행의 리롤과 편집 후 재생성을 새 턴으로
잘못 추가할 수 있었다.

`complete_turn_source_acceptance.go`의 영구 논리 턴 ID만 다음 계약으로 복구했다.

- 관찰된 안정적인 `UserMessageChatID`가 있으면 같은 사용자 행의 리롤·편집 재생성은 기존 턴을 교체한다.
- 실제로 새 사용자 행이 생기면 내용이 이전 행과 같아도 새 턴을 추가한다.
- 안정적인 ID를 관찰할 수 없을 때만 기존 인덱스·시각·내용 hash 좌표를 fallback으로 유지한다.
- 진짜 새 입력의 `latestCanonicalTurn+1` 경로는 유지한다.

## 2. 변경하지 않은 범위

- timeout 뒤 동일 요청 문맥 재사용과 HUD 재시도 표시
- 정상 `beforeRequest` / `afterRequest`
- 분기, Say Nothing, rollback
- 기억 검색·주입과 벡터 정책
- 기존 완료 턴 acceptance·terminal 처리
- JavaScript runtime 코드

JavaScript 줄 변경: 추가 0, 삭제 0.

## 3. 자동 검증

production 소유 함수를 사용해 다음 사례를 각각 검증했다.

- 정상 새 사용자 행은 새 턴 추가
- 같은 사용자 행 리롤은 기존 턴 교체
- assistant 삭제 뒤 같은 사용자 행을 편집·재생성하면 기존 턴 교체
- 내용이 같은 새 사용자 행은 새 턴 추가
- 같은 요청의 provider 재시도와 중복 `afterRequest`는 중복 저장하지 않음

검증 결과:

- 대상 `internal/httpapi` 5개 회귀: 통과
- fail-once provider와 동일 요청 재사용 JavaScript production-path 회귀 2개: 통과
- `go test ./internal/httpapi -count=1`: 통과
- 번들 Node를 사용한 `go test ./cmd/js-route-variant-smoke -count=1`: 통과
- 전체 `go test ./... -count=1`: Node 실행 파일을 자동 발견하지 못한 기본 환경의 한 패키지만 실패했고, 같은 패키지를 번들 Node 경로로 재실행해 통과
- source와 package `Archive Center.js` 문법 검사: 통과

## 4. Windows 4.1 테스트 패키지

정식 full-package 빌더로 기존 4.1 테스트 패키지를 갱신했다. 기존 로컬 설정과
관리형 runtime·update 상태는 빌드 전후 보존했다.

- 디렉터리: `_test-builds/Archive-Center-4.1.0-windows-test-20260901-ctrlc-fixed`
- package status: `green`, `release_ready=true`
- manifest 파일: 47개, 크기·SHA-256 불일치 0개
- backend toolchain: Go 1.26.6
- ZIP 크기: `12,277,751 bytes`
- ZIP SHA-256: `cb3d0cb9760e737f0c3ff410dd9a23b72e563ddcc8c7fa957e42a520092a889f`
- 외부 `SHA256SUMS-4.1.0.txt`와 실제 ZIP hash 일치

## 5. 오염 세션 복구 결과

전체 DB가 아니라 대상 세션 export만 복구 전·후 각각 로컬 runtime backup으로
보존했다. 백업 파일과 실제 대화 내용·세션 ID는 저장소 문서에 넣지 않았다.

현재 RisuAI 활성 채팅과 MariaDB를 dry-run으로 비교한 결과, 처음 예상한 91턴이
아니라 90턴부터 불일치가 시작됨을 확인했다. 89턴은 일치했다.

기존 backend 소유 복구 경로만 사용했다.

1. manual rollback decision이 해당 세션의 90턴 소유권과 정확한 suffix를 승인했다.
2. MariaDB canonical tail transaction으로 90턴 이후만 제거하고 vector cleanup을 durable outbox에 넣었다.
3. 현재 RisuAI에 실제 남은 세 사용자/assistant 쌍을 90·91·92턴으로 한 번씩 복원했다.
4. 세션 정상화로 누락 파생 자료를 재생성하고 벡터 색인을 한 번 실행했다.
5. 최종 dry-run에서 Risu 완료 쌍 92, DB 턴 92, raw missing 0, raw mismatch 0, derived suspect 0, unresolved 0, processable 0을 확인했다.

수동 SQL 삭제, 일반 rollback 추정, 별도 복구 API, 새 strict gate는 사용하거나
추가하지 않았다.

## 6. 남은 실환경 gate

현재 RisuAI에 로드된 플러그인은 4.0.9였으며 세션 복구에는 기존 관리 기능만
사용했다. 따라서 4.1 테스트 패키지의 `Archive Center.js`를 실제 RisuAI에 로드한 뒤
다음 최소 확인이 필요하다.

- 같은 사용자 행 리롤이 같은 Archive Center 턴을 교체하는지
- 사용자 행을 편집하고 재생성해도 같은 턴을 교체하는지
- 내용이 같은 새 사용자 행은 새 턴을 추가하는지
- fail-once provider 재시도 뒤 한 번만 저장되는지

이 live gate가 끝나기 전 상태는 `implemented_unverified`이며 정식 release 완료로
표시하지 않는다.

## 7. 2026-09-02 범위 확장 결정 — PDF 기억 전달 Preview

4.1의 남은 범위를 현재 기억 주입 기준선·중복 계보 검증에서, 그 기준선이 측정하는 동일한
장기기억을 PDF로 전달하는 opt-in Preview까지 확장했다.

정확한 기능 의미는 다음과 같다.

1. 기존 Go 검색·선택·예산 결과에서 `long_term_memory` 문장을 받는다.
2. Go가 그 문장을 순서와 내용 변경 없이 searchable/copyable 한글 PDF의 네이티브 텍스트로
   만든다.
3. Google AI Studio·Vertex에는 Gemini `inlineData`, LLM Gateway에는 OpenAI 호환 `file`
   block으로 전달한다.
4. PDF가 적용된 최종 provider body에서는 같은 장기기억 text를 제거하고 다른 auxiliary lane과
   사용자 입력은 보존한다.
5. 현재 4.1 request-owned retry 문맥에서 같은 Go plan과 PDF를 재사용한다.

이 결정은 PDF 구현 완료 기록이 아니다. 현재 상태는 `VERSION_ASSIGNED_PLAN`,
`IMPLEMENTATION_NOT_STARTED`다. 작업 정본은
[`archive-center-4.1-pdf-memory-transport-plan.md`](archive-center-4.1-pdf-memory-transport-plan.md)이며,
과거 `4.0.2-pdfexp.2` 소스·테스트·패키지는 호환성 대조용 역사 자료로만 사용한다.

다음 경계는 변경하지 않는다.

- 기억 검색·ranking·selection·budget·privacy와 source lineage
- MariaDB·ChromaDB·Critic·완료 턴 저장
- 리롤·편집 재생성·분기·Say Nothing·rollback
- 기존 text mode와 현재 timeout/provider retry lifecycle
- 별도 provider retry, 모델 전환과 자동 provider 추측

실제 구현 뒤에는 source/regression만으로 완료를 선언하지 않는다. Google AI Studio, Vertex,
LLM Gateway의 실제 final request body, PDF 수락, text 비중복, usage, 한국어 처음·중간·끝 회수와
displayed final을 구분해 검증한 후 Windows 4.1 테스트 패키지를 갱신한다.
