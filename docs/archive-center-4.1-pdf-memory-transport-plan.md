# Archive Center 4.1 PDF 기억 전달 작업 계획

상태: `VERSION_ASSIGNED_PLAN` · `IMPLEMENTATION_NOT_STARTED`

기준일: 2026-09-02 KST

계획 정본: [`4.1-9.0-integrated-roadmap.md`](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)

과거 실험 근거:

- [`gemini3-pdf-memory-transport-experiment-audit-and-handoff.md`](gemini3-pdf-memory-transport-experiment-audit-and-handoff.md)
- [`pdf-memory-stage1-generator.md`](../../pdf-memory-experiment/docs/pdf-memory-stage1-generator.md)
- [`pdf-memory-stage2-6-implementation-report.md`](../../pdf-memory-experiment/docs/pdf-memory-stage2-6-implementation-report.md)

## 0. 결정

Archive Center 4.1의 범위를 현재 기억 주입 기준선·중복 계보에서 **선택된 장기기억의
PDF 전달 Preview와 실제 provider 비교**까지 넓힌다.

이 기능은 기억을 PDF 파일로 보관하거나 PDF에서 새 기억을 검색하는 기능이 아니다. Go가
이번 요청에 이미 선택한 `long_term_memory` 문장을 검색·선택 순서 그대로 네이티브 텍스트
PDF로 만들고, 본문 모델이 그 PDF 안의 텍스트를 문서 입력으로 읽게 하는 물리적 전달 방식이다.

```text
기존 기억 검색·선택·예산
  -> 선택된 long_term_memory 문장
  -> Go 검색 가능 PDF 생성
  -> RisuAI 최종 provider body에 PDF 삽입
  -> 최종 body에서는 같은 long_term_memory 텍스트를 제거
  -> Gemini가 PDF의 네이티브 텍스트를 읽음
```

4.2 이후의 claim·event 정합성, 의미 중복 통합과 compact bundle 작업은 그대로 후속 버전이
소유한다. PDF 전달은 그 결과를 기다리지 않고 현재 4.1 선택 결과를 그대로 운반한다.

## 1. 현재 증거와 출발점

### 현재 4.1 활성 소스

- Go의 `payload_application_plan.v1`은 `original_work`, `long_term_memory`,
  `lorebook_reference`, `output_guidance` lane의 text·hash·budget·source ref를 구분한다.
- `Archive Center.js`는 Go가 만든 전체 `auxiliary_text`를 RisuAI 보조 시스템 메시지로 적용한다.
- 활성 `Archive Center.js`에는 PDF body interceptor가 아직 없다.
- 현재 기억 검색, 선택, 예산, 저장, 리롤, 분기, MariaDB와 Chroma 동작은 PDF 계획과 별개로
  이미 존재하는 owner가 계속 소유한다.

### 과거 PDF 실험

`4.0.2-pdfexp.2` 실험에는 검색 가능한 한글 PDF 생성기, `memory_transport_plan.v1`,
현재 요청용 PDF payload, Gemini body interceptor와 Windows 실험 빌드가 존재했다. 그러나
마지막 증거는 `pdf_pre_adapter_staged`였으며 실제 Google 요청 body, provider 수락, 실제
usage와 장기기억 회수 품질은 확인되지 않았다.

따라서 과거 코드를 현재 소스로 간주하거나 그대로 복사하지 않는다. 현재 4.1 owner와 계약에
대조해 필요한 부분만 이식하고, 실제 loaded RisuAI와 provider 증거를 새로 만든다.

## 2. 4.1에 포함하는 범위

### 2.1 기억 내용

- PDF 대상은 Go가 이미 선택한 `long_term_memory` lane 하나다.
- 문장 내용, 순서, source ref, visibility, 예산과 선택 결과를 PDF 전송 단계에서 다시 계산하지
  않는다.
- `original_work`, `lorebook_reference`, `output_guidance`, 현재 사용자 입력과 RisuAI 최근
  대화는 기존 텍스트 경로를 유지한다.
- PDF mode가 실제 최종 body에 적용된 요청에서는 같은 `long_term_memory`를 텍스트와 PDF로
  동시에 보내지 않는다.

### 2.2 지원 전송 경로

| 사용자 선택 전송 방식 | 최종 요청 형식 | 4.1 대상 |
| --- | --- | --- |
| 기존 Text | 현재 `payload_application_plan.v1` 보조 텍스트 | 기존 기본 동작 유지 |
| Google Gemini PDF | Gemini `inlineData` + `application/pdf` | Google AI Studio·Google Vertex |
| LLM Gateway Gemini PDF | OpenAI 호환 `type: file` + base64 `file_data` | LLM Gateway의 document-capable Gemini |

전송 방식은 사용자가 하나를 선택한다. Archive Center는 모델 표시 이름, prompt 문구나 응답
문구로 provider를 추측하지 않는다. Google AI Studio와 Vertex는 동일한 Gemini body 형식을
사용하므로 하나의 Google PDF adapter가 처리하고, LLM Gateway는 별도 body 형식만 처리한다.
LLM Gateway의 현재 공식 document route는 Google AI Studio의 document-capable Gemini를
대상으로 하므로, gateway 경로를 Vertex 직접 경로와 같은 usage·과금으로 간주하지 않는다.

### 2.3 수명과 재시도

- PDF bytes와 base64는 현재 요청의 transient 자료이며 MariaDB, ChromaDB와 plugin 영구
  storage에 저장하지 않는다.
- 현재 4.1의 request-owned provider retry 문맥이 같은 논리 요청에서 같은 Go plan과 PDF를
  재사용한다.
- PDF 전송은 새 `/prepare-turn`, 새 기억 검색, 새 Publisher, 새 request ID, 별도 provider
  재시도나 모델 전환을 만들지 않는다.
- 최종 provider body를 PDF 형식으로 바꾸기 전까지 기존 텍스트 payload를 그대로 둔다.
  Host가 PDF adapter를 호출하지 못하면 기존 텍스트 요청이 유지되며 Archive Center가 숨은
  재시도를 추가하지 않는다.

## 3. 영구 owner 경계

### Go backend

- 선택된 `long_term_memory` 원문과 논리 hash 소유
- 검색 가능·복사 가능한 한글 PDF 생성
- filename, MIME, page count, bytes, base64 size와 transport 계획 생성
- `long_term_memory`를 뺀 나머지 auxiliary text 생성
- text/PDF 논리 내용 동일성, 전달량과 진단 ViewModel 소유

과거 `memory_transport_plan.v1`을 현재 4.1 계약에 그대로 재사용할 수 있는지 먼저 대조한다.
호환된다면 이름과 의미를 유지하며, 단지 새 버전 번호를 만들기 위해 계약을 승격하지 않는다.

### Archive Center.js

- 사용자가 고른 전송 방식 관찰과 Go 전달
- RisuAI body interceptor 등록·해제
- Go가 만든 PDF를 Google 또는 LLM Gateway 최종 body 형식으로 적용
- 적용된 현재 요청의 transport 관찰과 HUD/UI 표시
- provider retry에서 현재 request-owned PDF plan 재사용

JavaScript는 기억을 다시 선택하거나 PDF에 넣을 문장을 재조립하지 않는다. PDF 전송 여부는
완료 턴 저장, 리롤 교체, 출력 수락이나 canonical 판정의 추가 조건이 아니다.

## 4. 작업 순서

### `4.1-PDF-A` — 현재 소스 호환성 대조

- 과거 `pdfmemory` generator·DTO·plan·interceptor를 현재 4.1 owner에 symbol 단위로 대조
- 현재 RisuAI 공식 body interceptor signature와 실제 Google/OpenAI-compatible body 확인
- 재사용할 코드, 폐기할 역사 코드, 새로 연결할 production caller 목록 확정
- 소스 수정 전 현재 text mode의 payload·retry·complete-turn 기준 fixture 고정

산출물: 호환성 표와 정확한 변경 파일 목록. 이 단계는 runtime 동작을 바꾸지 않는다.

### `4.1-PDF-B` — Go PDF 생성기 복원

- 한글 폰트를 내장한 searchable/copyable PDF 생성
- 선택된 장기기억 문장과 순서를 바꾸지 않는 페이지·열 배치
- 처음·중간·끝 marker, 한글, 숫자, 부정문, 관계 방향과 다중 페이지 순서 회귀
- PDF bytes·page count·base64 size·생성 시간 관찰

과거 2pt·4열 A4는 출발 fixture로 사용하되, 현재 Gemini 3에서 회수 품질을 다시 측정한 뒤
페이지 배치를 확정한다.

### `4.1-PDF-C` — Go 전송 계획 연결

- 현재 `payload_application_plan.v1`의 선택 결과를 소비하는 PDF transport plan 생성
- PDF 논리 내용과 `long_term_memory` lane의 source/hash/문자 수 계보 연결
- 다른 auxiliary lane이 그대로 남는 text projection 제공
- 현재 요청이 끝나면 PDF payload를 폐기하는 transient 수명 연결

### `4.1-PDF-D` — RisuAI provider body 적용

- Google AI Studio·Vertex용 `inlineData` PDF 적용
- LLM Gateway용 OpenAI `file` block 적용
- 최종 body에서 `long_term_memory` 텍스트를 PDF로 교체하고 다른 lane·사용자 입력 보존
- 현재 4.1 retry context에서 같은 PDF plan 재사용
- unload·요청 종료에서 interceptor와 transient 자료 정리

### `4.1-PDF-E` — production 회귀와 진단

- 기존 text mode의 payload, retry, reroll, branch, Say Nothing, complete-turn 비퇴행
- Google body와 LLM Gateway body 각각 PDF 1개 및 장기기억 text 중복 0 확인
- PDF가 없는 요청, 빈 장기기억, provider retry와 중복 `afterRequest` 확인
- HUD/진단에 logical chars, PDF pages/bytes, 실제 전송 방식과 적용 상태 표시
- PDF base64와 사용자 원문은 로그·진단·support bundle에 출력하지 않음

### `4.1-PDF-F` — loaded RisuAI·provider·Windows 검증

- 실제 RisuAI에서 body interceptor 등록과 callback 실행 확인
- Google AI Studio, Vertex, LLM Gateway 요청 본문을 각각 확인
- Gemini 3.1 Pro와 사용 중인 Gemini 3.x Flash에서 한국어 기억 회수 비교
- 동일 기억의 text/PDF A/B로 처음·중간·끝, 숫자, 부정, 소유·관계 방향, 지연과 usage 비교
- provider가 받은 최종 body에 PDF 1개, 장기기억 text 중복 0, 다른 lane 보존 확인
- 검증된 소스로 4.1 Windows 테스트 패키지 갱신 후 한 벌만 기동해 runtime readiness 확인

## 5. 증거 수준과 완료 판정

| 증거 | 확인하는 내용 | 확인하지 못하는 내용 |
| --- | --- | --- |
| Source | Go/JS owner와 실제 caller가 연결됨 | loaded Host callback·provider 수락 |
| Regression | production 함수가 text/PDF body를 예상대로 만듦 | 실제 RisuAI 변환·모델 회수 |
| Package | Windows artifact가 현재 source를 포함함 | 사용자가 로드한 plugin·provider 동작 |
| Loaded RisuAI | 실제 interceptor와 retry lifecycle이 실행됨 | provider가 PDF를 읽었는지 |
| Provider body/usage | PDF 전달·text 비중복·실제 usage | RP에서 기억을 올바르게 사용했는지 |
| Displayed final | 기억이 최종 출력에 정확히 반영됨 | 장기 세션 전체 품질의 일반화 |

Source와 회귀만 끝난 상태는 `IMPLEMENTED_UNVERIFIED`다. Google AI Studio·Vertex·LLM
Gateway의 실제 요청 및 회수 A/B가 끝나기 전에는 PDF token 절감이나 지원 완료를 주장하지
않는다.

## 6. 범위 밖

- 기억 검색·ranking·dedupe·selection 변경
- claim·event·entity·relationship 정합성 통합
- 새로운 기억 DB, PDF archive, PDF 검색기 또는 PDF vector index
- MariaDB schema와 Chroma collection 변경
- PDF 내용을 Publisher·Critic용 별도 기억으로 복제
- 모델명 whitelist와 자동 provider 추측
- Archive Center 자체 provider retry·fallback·모델 전환
- PDF 지원을 정상 출력·저장·리롤 수락의 조건으로 사용
- PocketRisu 지원을 실제 Host capability 확인 없이 완료로 선언

## 7. 문서 갱신 범위

구현을 시작할 때 이 문서를 작업 체크리스트로 사용한다. runtime owner, contract, hook order와
transport 관찰이 실제로 바뀌는 구현 change에서는 `STRUCTURE.md`, `AI_GUARDRAILS.md`, 4.1
작업 기록과 Windows 테스트 빌드 기록을 같은 변경에서 갱신한다.

현재 이 문서는 범위와 작업 순서를 확정한 계획 문서이며 runtime 구현 완료 증거가 아니다.

## 8. 공식 외부 계약 기준

- Google Gemini API document understanding:
  <https://ai.google.dev/gemini-api/docs/document-processing>
- Google Gemini 3 developer guide:
  <https://ai.google.dev/gemini-api/docs/gemini-3>
- Google Cloud/Vertex document understanding:
  <https://docs.cloud.google.com/vertex-ai/generative-ai/docs/multimodal/document-understanding>
- LLM Gateway document reading:
  <https://docs.llmgateway.io/features/documents>
- RisuAI Plugin API v3 guide:
  <https://github.com/kwaroran/RisuAI/blob/main/plugins.md>

모델과 provider 문서는 바뀔 수 있으므로 구현 시작과 live 검증 시점에 다시 확인한다. 공식
문서의 지원 표는 Archive Center가 실제 RisuAI body에 PDF를 적용했다는 증거를 대신하지 않는다.
