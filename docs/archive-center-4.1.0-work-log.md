# Archive Center 4.1.0 작업 기록

기록 기준: 2026-09-03 KST
범위: request-owned provider 재시도, 완료 턴의 리롤·편집 재생성 식별 복구, Windows Ctrl+C
종료 취소, 테스트 패키지·오염 세션 꼬리 복구, PDF 기억 전달 Preview, Yumi Provider Manager
PDF 실험 경로, Yumi Translator 1.4.2 원문 읽기 호환

작업 상태: `4.1 WORK_SCOPE_COMPLETE`, `IMPLEMENTATION_COMPLETE`
증거 구분: 소스·자동 회귀·Windows 패키지·실제 MariaDB/Chroma 세션 복구와 `/prepare-turn`
PDF 생성은 확인했다. 실제 RisuAI의 Provider Manager 경로에서는 표식 1개와
`application/pdf` 변환 및 본문 출력까지 관찰했다. 직접 Vertex Gemini 3.1 Pro 경로에서도
`gemini_base` body interceptor 진입, `google_pdf` 적용, 동일 장기기억 Text 블록 1개 제거를
관찰했다. Google AI Studio·LLM Gateway의 최종 body, 전체 PDF 블록 수, 회수·usage A/B와
갱신된 리롤·번역 플러그인 순서의 loaded-host 확인은 별도 실환경 증거로 남긴다.

## 1. 수정한 문제

완료 턴의 영구 식별이 편집할 때 달라질 수 있는 사용자 내용 hash·시각·인덱스를
함께 사용해, 같은 RisuAI 사용자 메시지 행의 리롤과 편집 후 재생성을 새 턴으로
잘못 추가할 수 있었다.

`complete_turn_source_acceptance.go`의 영구 논리 턴 ID만 다음 계약으로 복구했다.

- 관찰된 안정적인 `UserMessageChatID`가 있으면 같은 사용자 행의 리롤·편집 재생성은 기존 턴을 교체한다.
- 실제로 새 사용자 행이 생기면 내용이 이전 행과 같아도 새 턴을 추가한다.
- 안정적인 ID를 관찰할 수 없을 때만 기존 인덱스·시각·내용 hash 좌표를 fallback으로 유지한다.
- 진짜 새 입력의 `latestCanonicalTurn+1` 경로는 유지한다.

### 1.1 동일 논리 요청의 provider 재시도

RisuAI의 provider timeout·retry·fallback은 같은 논리 요청 안에서 `beforeRequest`를 다시 호출할
수 있다. 4.1은 두 번째 호출이 같은 Host 턴으로 관찰되면 첫 호출에서 준비한 request context,
Archive request ID, raw input, Host 좌표, `/prepare-turn` 결과와 payload application plan을
재사용한다.

- 새 `/prepare-turn`, Publisher, 검색, request ID와 HUD를 만들지 않는다.
- 보조 컨텍스트는 같은 정확한 블록을 한 번만 유지하고, 깨끗한 retry payload에는 기존 Go
  계획만 다시 적용한다.
- 성공 `afterRequest`는 기존 context를 한 번만 소비하며 중복 callback이 `/complete-turn`을
  다시 호출하지 않는다.
- HUD는 6/12 `본문 응답 기다리는 중`을 유지하며 재시도 횟수를 표시하고, 성공 출력 뒤 같은
  request ID로 7/12를 시작한다.
- Archive Center가 관찰하지 못한 실패 원인을 `timeout`으로 단정하지 않는다.

## 2. 리롤 식별 수정에서 변경하지 않은 범위

- 이미 구현된 timeout 뒤 동일 요청 문맥 재사용과 HUD 재시도 표시
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
- manifest 파일: 52개, 크기·SHA-256 불일치 0개
- backend toolchain: Go 1.26.6
- ZIP 크기: `17,849,693 bytes`
- ZIP SHA-256: `2c117d914aa545da5474d232b5ff7be5fd45f2d5f128099482a9e095467f1265`
- 외부 `SHA256SUMS-4.1.0.txt`와 실제 ZIP hash 일치

### 4.1 Windows Ctrl+C 종료 취소

공개 BAT로 실행한 서비스에서 Ctrl+C 뒤 첫 Archive Center 확인에 `N`을 눌러도 Go backend가
이미 signal을 받아 종료되는 문제를 수정했다. PowerShell launcher가 관리형 Go·MariaDB·
ChromaDB child를 Ctrl+C 전파에서 격리하고, `N`은 같은 child PID를 그대로 유지하며 대기로
돌아간다. `Y`, launcher/BAT parent 상실 또는 비정상 종료만 기존 bounded cleanup과 Job Object
정리를 실행한다. 종료가 정상 완료된 BAT는 불필요한 `pause`를 남기지 않는다.

격리된 콘솔의 `N → Y` 회귀와 실제 4.1 Windows package에서 확인했다. `N` 뒤 세 서비스 PID와
readiness가 유지됐고, 이어서 `Y`를 선택하면 process tree와 28080·3307·8000 listener가
정리됐다.

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

## 6. 구현 완료 뒤 남은 실환경 증거

현재 RisuAI에 로드된 플러그인은 4.0.9였으며 세션 복구에는 기존 관리 기능만
사용했다. 따라서 4.1 테스트 패키지의 `Archive Center.js`를 실제 RisuAI에 로드한 뒤
다음 최소 확인이 필요하다.

- 같은 사용자 행 리롤이 같은 Archive Center 턴을 교체하는지
- 사용자 행을 편집하고 재생성해도 같은 턴을 교체하는지
- 내용이 같은 새 사용자 행은 새 턴을 추가하는지
- fail-once provider 재시도 뒤 한 번만 저장되는지

이 항목들은 4.1의 구현 범위를 다시 여는 미완성 기능이 아니라, 실제로 로드된 Host에서
source/regression 결과를 재확인하는 후속 증거다. 확인 전에는 해당 동작만 `LIVE_VERIFIED`로
과장하지 않는다.

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

현재 구현 상태는 `IMPLEMENTATION_COMPLETE`다. 작업 정본은
[`archive-center-4.1-pdf-memory-transport-plan.md`](archive-center-4.1-pdf-memory-transport-plan.md)이며,
과거 `4.0.2-pdfexp.2` 소스·테스트·패키지는 호환성 대조용 역사 자료로만 사용한다.

다음 경계는 변경하지 않는다.

- 기억 검색·ranking·selection·budget·privacy와 source lineage
- MariaDB·ChromaDB·Critic·완료 턴 저장
- 리롤·편집 재생성·분기·Say Nothing·rollback
- 기존 text mode와 현재 timeout/provider retry lifecycle
- 별도 provider retry, 모델 전환과 자동 provider 추측

### 7.1 구현 및 자동 회귀

- Go 네이티브 생성기는 A4 2pt·4열 searchable/copyable PDF를 만들고, 문장을 정규화·요약·
  재정렬하지 않는다. 공식 Google Fonts `Noto Sans KR` 가변 TTF를 내장했으며 실제 세션에서
  필요했던 `油`, `菜`, `種`의 추출 회귀를 고정했다.
- Go는 기존 `payload_application_plan.v1`의 `long_term_memory` 선택 결과만 사용해
  `memory_transport_plan.v1`과 현재 응답 전용 `memory_transport_payload.v1`을 만든다. PDF 생성
  실패·빈 선택은 기존 Text를 유지하며 저장·검색·재시도를 새로 만들지 않는다.
- `Archive Center.js`는 사용자가 고른 `text`, `google_pdf`, `llm_gateway_pdf`를 Go에 전달한다.
  등록된 body interceptor는 Google AI Studio·Vertex에 `inlineData`, LLM Gateway에 OpenAI
  호환 `file` block을 적용하고, 정확히 같은 장기기억 Text만 제거한다. 다른 lane·사용자 입력·
  요청 소유권은 유지한다.
- 같은 논리 요청의 provider 재시도는 기존 request-owned plan과 PDF를 재사용한다. PDF base64는
  현재 `/prepare-turn` 응답과 request context에만 있으며 complete-turn·로그·지원 bundle·DB·
  Chroma에는 저장하지 않는다.
- PDF 생성, DTO/HTTP, 실제 production JavaScript body 변환, Text 비퇴행, 재시도·리롤·분기·
  Say Nothing·저장 회귀와 전체 `go test ./... -count=1`, source/package JavaScript 문법 검사를
  통과했다.

### 7.2 Windows 패키지와 실제 backend 검증

최종 패키지를 위 4절 경로와 hash로 다시 만들고 기존 `.env.full.local`, `.runtime`, `.updates`를
동일한 파일 수·크기로 복원했다. 이전 Go·MariaDB·ChromaDB를 확인 후 종료하고 새 패키지 한
벌만 기동했다. `/health`, `/ready`, Chroma heartbeat와 28080·3307·8000 단일 listener를
확인했다.

복원된 실제 MariaDB 세션에 read-only `/prepare-turn`을 Text·Google PDF·Gateway PDF로 각각
실행한 결과는 다음과 같다.

- 세 모드의 `payload_application_plan.v1` 동일
- 세 모드의 논리 장기기억 17,979자와 SHA-256 동일
- Google/Gateway 모두 `pdf_ready`, 1 page, 99,514 bytes, `%PDF-` header
- `pdf_base64`는 각 PDF 응답의 `memory_transport_payload` 한 곳에만 존재
- Go·MariaDB·ChromaDB는 각각 한 세트만 실행 중

### 7.3 실제 Host/provider 확인 상태

실제 RisuAI에는 갱신된 `Archive Center.js`가 로드됐고, 아래 7.5의 Provider Manager opt-in
경로가 메인 출력까지 도달하는 것을 관찰했다. 직접 Vertex Gemini 3.1 Pro에서는
`memory PDF transport context attached` 뒤 `gemini_base` JSON body interceptor가 활성 요청
context와 envelope를 받은 뒤 `google_pdf`를 적용했고, 대응하는 Text 블록 1개를 제거했다.
뒤이어 관찰된 `meta_gemini` 진입은 usage/model metadata callback이며 두 번째 PDF 적용 증거는
아니다. Google AI Studio·LLM Gateway의 최종 body, 요청 전체의 PDF 블록 수와
Gemini 3.1 Pro/3.x Flash의 한국어 처음·중간·끝 회수·지연·usage A/B는 남아 있다. 따라서
Vertex 직접 전달은 `LIVE_TRANSPORT_OBSERVED`, provider별 성능·절감 효과는 `LIVE_PENDING`이며,
확인 전에는 지원 품질이나 token 절감을 주장하지 않는다.

### 7.4 실제 Host 전송 단절 진단 빌드

실제 RisuAI에서 body interceptor 등록 로그만 보이고 PDF 적용 로그와
`memory_transport_observation`이 확인되지 않은 상태를 추측 없이 구분하기 위해, 기존
`attachFinalConfirmationMemoryTransport()`와 `onMemoryTransportBodyInterceptor()`에 디버그 모드
진단만 추가했다. 출력·저장·허용 여부를 바꾸는 조건, fallback, retry, 별도 상태 경로는 추가하지
않았다.

진단은 다음 항목만 기록하며 사용자 입력, 기억 본문, PDF base64는 기록하지 않는다.

- PDF plan/payload가 request-owned context에 연결됐는지
- body interceptor가 실제 호출됐는지와 RisuAI가 전달한 request type
- 활성 요청 context와 memory transport envelope 존재 여부
- 기존 조기 반환 사유와 Text 제거·PDF 첨부 적용 결과
- Gemini metadata callback에서 usage/model status의 존재 여부

JavaScript 진단 변경은 추가 74줄·삭제 7줄이다. 번들 Node 문법 검사와 production 함수를 실행하는
`TestArchiveCenterPDFMemoryTransport*` 회귀가 통과했다.

기존 단일 Windows 4.1 테스트 패키지를 같은 경로에 재빌드하고 `.env.full.local`, `.runtime`,
`.updates`를 복원했다. 생성 package status는 `green`, `release_ready=true`, 관리 파일 52개다.

- ZIP 크기: `17,847,689 bytes`
- ZIP SHA-256: `947df5f3248d3615911c48812f4c77c5a25704614f1f055689972305c68f9509`
- 외부 `SHA256SUMS-4.1.0.txt`와 실제 ZIP hash 일치
- Go·MariaDB·ChromaDB를 새 패키지 한 벌로 기동하고 28080·3307·8000 단일 listener 확인
- `/health` 정상, `/ready`의 Store·Vector 모두 ready

이 진단 빌드로 직접 Google PDF 모드에서는 `memory PDF transport context...` 뒤 body
interceptor 진입 여부를 확인할 수 있게 했다. 이후 `[PM]` 모델 경로에서 body interceptor가
호출되지 않는다는 실제 관찰이 7.5의 Provider Manager opt-in 표현 경로를 선택한 근거가 됐다.
직접 Vertex body 적용은 관찰됐고, Google AI Studio·LLM Gateway body 관찰은 계속 별도 live
증거로 남는다.

### 7.5 Yumi Provider Manager v1.15.3 실험 경로

실제 `[PM]` 모델 요청에서는 Archive Center의 RisuAI body interceptor가 호출되지 않는다는
진단 결과에 따라, 외부 Provider Manager를 수정하지 않는 별도 opt-in 표현 경로를 추가했다.
Provider Manager v1.15.3이 공개적으로 처리하는 `<pm-pdf>...</pm-pdf>` 수동 지정 구간을
사용하며, Archive Center는 모델명이나 플러그인 상태를 자동 판정하지 않는다.

- 설정 값: `provider_manager_pdf` (`Yumi Provider Manager PDF (실험)`)
- Go는 기존 `long_term_memory` 선택과 나머지 auxiliary projection을 그대로 제공한다.
- 이 모드에서는 Go PDF/base64를 만들지 않는다. PDF 생성과 provider 첨부는 Provider Manager가
  담당하므로 같은 기억의 PDF를 두 번 만들지 않는다.
- JavaScript는 기존 Text 기준선을 적용·관찰한 뒤 선택된 장기기억만 표식 한 쌍으로 바꾼다.
  원작, lorebook, 출력 안내와 현재 사용자 입력은 기존 Text 메시지로 남긴다.
- 같은 요청 재시도에서는 기존 표식 표현을 기준선으로 복원한 뒤 한 번만 다시 적용한다.
- Provider Manager 파일, IPC 권한, 모델 설정, Archive Center 저장·리롤·분기·검색 경로는
  변경하지 않았다.

자동 검증은 Go 계획이 두 번째 PDF를 만들지 않는지, HTTP 응답에 transient base64가 없는지,
표식 한 쌍·장기기억 한 번·나머지 lane/사용자 입력 보존, 재시도 후 중복 없음, 기존
Text/Google/Gateway 경로 비변경을 production 함수로 확인했다. 번들 Node 문법 검사와 대상 Go/JS
회귀는 통과했다.

실제 사용 전 Provider Manager 모델에서 다음 두 설정을 사용자가 직접 켜야 한다.

1. `텍스트 변환: Gemini PDF`
2. 변환 설정의 `수동 지정 기능 사용`

Archive Center는 이 외부 설정을 읽거나 바꾸지 않는다.

동일 소스로 기존 단일 Windows 4.1 테스트 패키지를 갱신했다. 로컬 `.env.full.local`, `.runtime`,
`.updates`는 교체 전에 보존했고 파일 수·크기가 같은 상태로 복원했다. 빌드 중 사용한 이전 패키지
롤백 폴더, 스테이징 폴더와 Go 테스트 캐시는 새 패키지 기동 확인 후 삭제해 `_test-builds`에는 기존
이름의 테스트 빌드 폴더 하나만 남겼다.

- package status: `green`, `release_ready=true`
- 관리 파일: 52개, 크기·SHA-256 불일치 0개
- backend toolchain: Go 1.26.6
- ZIP 크기: `17,849,693 bytes`
- ZIP SHA-256: `2c117d914aa545da5474d232b5ff7be5fd45f2d5f128099482a9e095467f1265`
- 외부 `SHA256SUMS-4.1.0.txt`와 실제 ZIP hash 일치
- package `Archive Center.js` 문법 검사 통과
- 새 패키지에서 `/health=ok`, `/ready.ready=true`, Store·Vector·reference vector ready
- 28080·3307·8000은 각각 새 Go·관리형 MariaDB·ChromaDB 한 프로세스만 사용

### 7.6 Yumi Provider Manager 실제 관찰

실제 loaded RisuAI에서 Provider Manager v1.15.3 모델의 두 opt-in 설정을 켜고 한 턴을 실행해
다음을 관찰했다.

- Archive Center 콘솔: `memory Provider Manager PDF marker transport applied`
- `markerApplied: true`, `markerBlockCount: 1`
- `logicalMemoryChars: 17999`
- `providerPDFCreation: provider_manager_runtime_pending`
- Provider Manager 변환 기록: `Token Convert: gemini-pdf`
- 변환된 입력에 `mime_type: application/pdf`와 `%PDF-1.7`로 해석되는 base64 header 존재
- 같은 요청에서 메인 모델의 본문 출력 완료

이는 `Archive Center 선택 결과 → <pm-pdf> 표식 1개 → Provider Manager PDF 변환 → 메인 모델
출력` 경로가 실제 Host에서 이어졌다는 증거다. 개발자 도구에 최종 provider request body 전체가
노출되지 않아 총 PDF block 수, 동일 장기기억 Text 제거 여부, 처음·중간·끝 회수와 usage A/B는
독립적으로 확정하지 않았다. 따라서 이 경로는 `LIVE_PATH_OBSERVED_PARTIAL`이며, 구현 완료와
성능 검증 완료를 구분한다.

## 8. Yumi Translator 1.4.2 원문 읽기 호환

### 8.1 문제와 기준

Yumi Translator 1.4.2는 assistant 본문을
`<!-- yumi-tr:v1:<id>:start -->...<!-- yumi-tr:v1:<id>:end -->`로 감싸고, 화면에는 번역문을
표시하면서 원문을 active chat의 `$__yumi_tr.<id>` script state에 보존한다. Archive Center가
화면용 번역문만 Publisher·연속성·기억 추출의 입력으로 읽으면 번역 결과를 다시 요약하거나
원문의 어조·고유명사·관계를 잃을 수 있었다.

4.1은 외부 번역기나 표시 내용을 수정하지 않고, Archive Center 내부 읽기 사본에서만 원문을
복원하도록 했다.

### 8.2 구현 경계

- `buildYumiV1ArchiveReadContext()`가 정확한 v1 marker와 같은 active chat의
  `$__yumi_tr.<id>` record를 연결한다.
- plain, `u:`와 `z:` record를 읽고, 일치한 assistant 메시지만 저장된 모델 원문으로 바꾼
  `archiveReadMessages`를 만든다.
- Publisher, continuity 판단, `/prepare-turn`의 Archive-owned source 읽기와 recent context
  조립만 이 사본을 사용한다.
- 화면 번역문, active chat, RisuAI가 메인 provider에 보내는 payload, raw Host observation은
  변경하지 않는다.
- record가 없거나 손상됐으면 현재 보이는 번역문을 그대로 읽고 기존 요청을 계속한다. 이를
  출력·저장 거부 조건으로 사용하거나 별도 번역·재시도 경로를 만들지 않는다.
- Yumi Translator 파일과 설정은 수정하지 않았다.

### 8.3 검증과 상태

production JavaScript 함수를 실행하는 회귀에서 plain/`u:`/`z:` 원문 복원, 누락·손상 record,
화면/active payload 비변경과 내부 읽기 사본의 provider payload 비유출을 확인했다. JavaScript
문법 검사와 Windows 패키지 포함도 통과했다.

이 호환 작업은 `SOURCE_REGRESSION_COMPLETE`다. 실제 loaded RisuAI에서 Translator와 Archive
Center의 hook 순서, Publisher가 읽은 원문과 최종 표시 번역문의 분리를 추적하는 검증은
`LIVE_PENDING`이다. 해당 실환경 증거가 없어도 구현 범위는 완료됐지만, 확인 전에는 실제 번역
품질 개선을 `LIVE_VERIFIED`로 주장하지 않는다.

## 9. 4.1 완료 판정

4.1에서 승인된 구현 범위는 2026-09-02 기준으로 완료했다.

- 동일 논리 요청의 provider 재시도는 기존 request context와 준비 결과를 재사용한다.
- 같은 Risu 사용자 메시지 행의 리롤·편집 재생성은 기존 턴을 교체하고, 실제 새 행만 새 턴을
  추가한다.
- 기존 Text 전달을 유지하면서 Google/Vertex, LLM Gateway와 Provider Manager용 PDF opt-in
  경로를 추가했다.
- Yumi Translator 1.4.2가 감싼 번역문 대신 보존된 모델 원문을 Archive-owned 읽기 경로에서
  사용할 수 있게 했다.
- 기억 검색·선택·예산·저장·Critic·분기·Say Nothing·rollback의 기존 owner와 동작은 별도
  경로로 교체하지 않았다.
- Windows 4.1 테스트 패키지와 manifest/hash/runtime readiness를 갱신했다.

따라서 roadmap 상태는 `4.1 WORK_SCOPE_COMPLETE`로 닫는다. 남은 loaded-host/provider별 항목은
후속 검증 자료이며, 4.2에서는 4.1 전달 방식을 다시 만드는 대신 선택된 기억의 중요도·간결성·
실제 서사 반영을 개선한다. 같은 버전의 후반 작업은 기존 `응답 직후` 저장을 기본값으로
보존하면서, 사용자가 선택한 `다음 사용자 입력 시` mode에서 직전 편집·리롤 최종본을 확정하고
직전 Critic과 현재 본문 출력을 겹쳐 실행하는 수명주기 절편을 추가한다.

## 10. 공개 릴리스 사전 검증

2026-09-03에 공개 4.1.0 후보에 다음 검증을 추가했다.

- Windows·POSIX 신규 설치 helper가 선택한 ZIP의 공개 SHA-256 레코드를 추출 전에 정확히
  검증하고, 변조 fixture에서는 설치 pointer를 만들지 않는 회귀 통과
- UI의 `지금 업데이트`가 플랫폼·파일명을 고르지 않고 `POST /update/apply`를 정확히 한 번
  호출하는 JavaScript production-function 회귀 통과
- production updater CLI의 관리 파일 추가·교체·삭제, 성공 commit, 명시 rollback, 중단 후
  복구, 후보 readiness 실패 복구 통과
- 위 모든 update 시나리오에서 사용자 DB/runtime/secret sentinel 보존 통과
- Windows Ctrl+C 격리 시험에서 `N` 뒤 동일 child PID 유지, 다음 `Y`에서 정리 진입 확인
- 전체 `go test ./...`, `go vet ./...`, 핵심 분리 회귀, JavaScript 문법 검사 통과
- 현재 공개 대상과 262개 Git commit 이력의 Gitleaks 8.30.1 검사 통과
- Windows 2개, Linux 2개, macOS 2개, Termux 1개의 ZIP과 하나의 통합 SHA-256 목록 생성 확인

GitHub 공개 여부, 공개 자산에서의 설치, 공개 4.0.9→4.1.0 UI 갱신은 source/package 검증과
구분해 업로드 이후 별도로 확인한다. 최종 자산은 clean 공개 commit에서 다시 생성하며, 이
사전 후보의 `source_dirty=true` manifest나 이전 작업 기록의 ZIP hash를 공개 자산으로 사용하지
않는다.

## 11. 공개 후 관찰 — 연속 사용자 메시지의 턴 표시 불일치

2026-09-03에 4.1.0 사용자가 assistant 응답 사이에 RisuAI 사용자 입력 행을 두 개 연속으로
만든 세션에서 `Host N / Backend N+1`과 `backend_turn_ahead_of_host`를 관찰했다. 이어진 출력이
Backend의 `N+1` 턴으로 표시·저장됐다는 사용자 확인이 함께 있었다. 처음에는 후처리 플러그인의
보조모델 호출이 원인 후보였으나, 사용자가 연속 입력 두 개를 확인한 뒤 같은 현상을 그 입력
배치와 연결했다.

현재 증거는 사용자 재현 보고와 HUD 기록이며 production fixture로 재현한 증거는 아니다. Host의
완료된 user/assistant pair 수와 사용자 메시지 행·index/ordinal을 서로 다른 경로에서 턴 좌표로
해석할 때 한 칸 차이가 생길 수 있는 호환성 관찰로 기록한다. 이 절에서는 runtime, 턴 식별,
저장·리롤·정상화 동작을 변경하지 않으며 새 조건이나 자동 보정도 추가하지 않는다. 상태는
`DOCUMENTED_UNFIXED`다.
