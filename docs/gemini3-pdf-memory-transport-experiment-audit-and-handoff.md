# Gemini 3 PDF 장기기억 전달 실험 감사 및 인계

상태: `audit_complete_implementation_not_started`

기준일: 2026-08-24

활성 소스:

`C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source`

이 문서는 사용자가 제공한 `Archive Center 4.0 — Gemini 3 계열용 PDF 장기기억 전달 실험` 작업안을 현재 Archive Center 소스, RisuAI 공식 소스, 각 제공자의 공식 문서와 대조한 결과다. 이 감사에서는 런타임 구현을 변경하지 않았다.

## 1. 최종 판단

실험 방향은 기술적으로 타당하다. 특히 Gemini 3의 PDF 처리에서는 PDF에 네이티브로 포함된 텍스트를 모델에 제공하면서 그 추출 텍스트에서 발생한 토큰에는 과금하지 않는다고 Google 공식 문서가 설명한다. 따라서 동일한 장기기억을 텍스트와 검색 가능한 텍스트 PDF로 각각 전달하여 비용·지연·회수 정확도를 비교할 가치가 있다.

다만 현재 상태에서 바로 전체 런타임에 붙이면 안 된다. PDF API 자체보다 다음 경계가 먼저 해결되어야 한다.

1. RisuAI body interceptor는 실제 요청 body와 interceptor type만 전달하며 URL, provider, model ID, request ID를 함께 전달하지 않는다.
2. Google AI Studio와 Vertex AI는 RisuAI에서 같은 `gemini_base` 또는 `gemini_base_stream` interceptor type을 사용한다. custom Google-compatible URL도 같은 경로를 탈 수 있다.
3. 현재 Archive Center의 `payload_application_plan.v1`은 장기기억을 포함한 여러 lane을 하나의 보조 시스템 메시지로 합친다. PDF가 적용된 경우 장기기억 lane만 정확히 제거하고 다른 lane은 텍스트로 남겨야 한다.
4. 한 페이지를 비정상적으로 길게 만드는 PDF는 최대 3072 x 3072로 비율을 유지한 채 축소될 수 있다. 비용 절감과 별개로 가운데·끝 부분의 가독성과 회수 정확도가 떨어질 수 있다.
5. PDF base64는 브라우저 메모리와 네트워크 payload를 줄이지 않는다. 바이너리보다 약 4/3 크기의 base64 문자열과 변환 중 복사본이 생길 수 있다.

따라서 첫 작업은 독립 PDF 생성·동일성·장문 회수 실험이어야 한다. 그 결과가 통과한 뒤에만 Go 전달 계약과 얇은 JavaScript body interceptor를 연결한다.

## 2. 제공자 지원 판정

### 2.1 확정 구현 대상

다음 두 경로는 이번 실험의 확정 구현 대상이다.

| 경로 | 공식 PDF 입력 | Gemini 3 비용 특성 | RisuAI 요청 형식 | 구현 대상 |
| --- | --- | --- | --- | --- |
| Google AI Studio 직접 | 확인됨 | 확인됨 | Gemini `contents[].parts[].inlineData` | 예 |
| Vertex AI 직접 | 확인됨 | 실제 usage를 별도 검증해야 함 | 같은 Gemini body 계열, Vertex endpoint·인증은 별도 | 예 |

여기서 “확정”은 구현 대상으로 확정했다는 뜻이다. 실제 외부 호출 성공, 토큰 절감, 장문 회수 정확도는 아직 검증되지 않았다.

Google AI Studio와 Vertex를 같은 결과라고 가정하면 안 된다. RisuAI 소스상 두 경로는 같은 Gemini body 생성기를 사용하지만 endpoint, 인증, region, usage 응답은 다르다.

### 2.2 조건부 포함 후보

다음 경로는 API 차원에서 PDF 입력이 공식 확인되지만 현재 RisuAI 플러그인 계약만으로 안전하게 자동 식별할 수 없어 첫 구현에는 포함하지 않는다.

| 경로 | API 가능성 | 현재 제외 이유 | 포함 조건 |
| --- | --- | --- | --- |
| LLM Gateway 종량제 | Google Gemini 문서 입력을 Google AI Studio로 전달하고 OpenAI `file` block을 공식 지원 | `openai_basic`/`openai_streaming` callback에서 endpoint를 알 수 없어 다른 OpenAI 호환 서버와 구분 불가 | 요청 시점의 정규화 provider·endpoint·model이 신뢰 가능한 계약으로 관찰되고 실제 routing·usage가 확인될 것 |
| LLM Gateway DevPass | 문서 요청 자체는 가능할 수 있음 | 실제 upstream provider 고정과 동일 과금 특성을 보장할 수 없음 | 실제 routing·usage 및 PDF 수락을 외부 요청으로 확인할 것 |
| OpenRouter | PDF `file` block과 native/parser 경로 공식 지원 | endpoint 식별 불가, native가 아니면 PDF parser가 개입하여 Gemini 3 직접 경로와 의미·과금이 달라질 수 있음 | OpenRouter endpoint, `native` PDF engine, Google AI Studio 또는 Google Vertex provider pin, fallback 비활성화, usage를 검증할 것 |

위 조건이 충족되면 추가할 수 있다. 단순 모델명 문자열, 플러그인 표시 이름, 응답 문구로 제공자를 추측해서는 안 된다.

### 2.3 이번 실험에서 텍스트 유지

다음 제공자도 PDF API를 지원할 수 있지만 Gemini 3 PDF 비용 실험과 다른 계약이다.

| 경로 | 공식 확인 | 이번 판정 |
| --- | --- | --- |
| OpenAI Responses API | `input_file` PDF에서 텍스트와 페이지 이미지를 모델에 전달 | 다른 body·토큰 계약이므로 텍스트 유지 |
| Anthropic Claude | URL, base64, Files API PDF 입력 지원 | 텍스트 및 페이지 이미지 토큰 비용이 발생하므로 텍스트 유지 |
| 일반 OpenAI 호환 endpoint | 공통 PDF 표준을 보장하지 않음 | 텍스트 유지 |
| Ollama·DeepSeek·GLM·Kimi·MiniMax·Grok의 임의 endpoint | 모델·서버별 파일 계약이 다름 | 명시된 공식 계약과 요청 식별이 모두 없으면 텍스트 유지 |

이 표는 해당 모델이 PDF를 절대 처리할 수 없다는 뜻이 아니다. Archive Center가 현재 RisuAI 요청 경계에서 무조건 안전하게 적용할 수 있다는 증거가 없다는 뜻이다.

### 2.4 다른 프로바이더에서 Gemini 3 이상을 사용하는 경우

Gemini 3의 PDF 네이티브 텍스트 특성은 사용자가 보는 서비스 이름이 아니라 실제 upstream Gemini 요청이 문서를 어떻게 받는지에 달려 있다. 따라서 다른 프로바이더도 다음 조건을 모두 만족하면 포함할 수 있다.

1. 요청 모델이 Gemini 3 이상임을 요청 body 또는 신뢰 가능한 host capability에서 확인한다.
2. PDF가 중간 서비스에서 OCR·markdown·일반 text로 변환되지 않고 Gemini native document input으로 전달된다.
3. 실제 upstream이 Google AI Studio 또는 Vertex AI임을 고정하거나 응답 routing metadata로 확인한다.
4. provider fallback이 다른 비-Google 경로나 parser 경로로 바뀌지 않는다.
5. 실제 usage metadata에서 PDF page/image modality와 입력 비용을 확인한다.
6. RisuAI body interceptor가 해당 endpoint를 다른 OpenAI 호환 endpoint와 혼동하지 않는다.

현재 판정은 다음과 같다.

- LLM Gateway pay-as-you-go: 포함 가능성이 높다. 공식 문서가 Gemini document input을 Google AI Studio로 전달한다고 명시한다. 다만 RisuAI에서 LLM Gateway endpoint를 요청 전에 식별하는 계약과 실제 usage 검증이 먼저 필요하다.
- LLM Gateway DevPass: 아직 확정할 수 없다. 공식 문서상 provider pinning을 지원하지 않고 smart routing과 provider fallback을 사용한다.
- OpenRouter: 조건부로 포함할 수 있다. `file-parser`의 `native` engine을 강제하고, provider를 `google-vertex` 등 실제 Google endpoint로 제한하며, `allow_fallbacks=false`로 고정한 외부 검증이 필요하다.
- 임의 OpenAI 호환 Gemini endpoint: endpoint별 공식 file block 변환 계약과 실제 upstream 증거가 있어야 한다.

현재 RisuAI plugin callback이 URL을 전달하지 않으므로 위 경로를 자동 판별하는 구현은 아직 확정되지 않았다. 구현 전에 다음 두 선택지 중 하나를 사용자에게 보고해야 한다.

- RisuAI가 provider·endpoint·model을 노출하는 공식 capability를 기다리거나 upstream에 추가한다.
- Archive Center에 명시적 실험용 provider binding을 두고, body의 model·plan hash까지 모두 일치할 때만 적용한다.

표시 이름이나 model 문자열 하나만으로 자동 적용하는 선택지는 허용하지 않는다.

## 3. 현재 Archive Center 소스에서 확인된 경로

### 3.1 Go가 최종 전달 내용을 소유한다

- `go-service/internal/httpapi/prepare_turn_render.go:42`
  - `buildPrepareTurnPayloadApplicationPlan`이 `payload_application_plan.v1`을 만든다.
- `go-service/internal/httpapi/prepare_turn_render.go:101`
  - `original_work`, `long_term_memory`, `output_guidance` lane을 별도로 만든다.
- `go-service/internal/httpapi/prepare_turn_render.go:128`
  - 계약 owner는 `go`, 적용 규칙은 `apply_exact_text_without_reassembly`다.
- `go-service/internal/httpapi/prepare_turn_render.go:164`
  - 로어북은 `lorebook_reference` lane으로 별도 부착된다.
- `go-service/internal/httpapi/group_turn_prepare.go:1391`
  - 현재 기억과 상태 조립 결과를 payload plan에 전달한다.
- `go-service/internal/httpapi/group_turn_prepare.go:1423`
  - lane별 후보·선택·최종 chars 장부를 plan에 붙인다.

현재 plan은 lane마다 `text`, `content_hash`, `source_refs`, `applied`를 가지고 있으므로 장기기억 lane의 논리적 원문과 해시는 이미 구분된다. 그러나 최종 `auxiliary_text`는 적용된 lane을 `\n\n`으로 합친 하나의 문자열이다.

### 3.2 JavaScript는 현재 텍스트 시스템 메시지만 적용한다

- `Archive Center.js:31277`
  - `injectAuxiliaryBlock`이 `[Archive Center — Auxiliary Context]` 시스템 메시지를 삽입한다.
- `Archive Center.js:31525`
  - `applyGoPayloadApplicationPlan`이 Go plan을 읽어 재조립 없이 적용한다.
- `Archive Center.js:4817`
  - RisuAI `input`, `beforeRequest`, `afterRequest` hook을 등록한다.
- 현재 `Archive Center.js`에는 `registerBodyIntercepter` 등록이 없다.

PDF 적용에는 RisuAI의 실제 provider body가 만들어진 뒤 파일 part를 넣어야 하므로 body interceptor 사용은 피할 수 없다. 이는 JavaScript 정책 추가가 아니라 RisuAI host의 실제 요청 body 변형 책임에 해당한다.

### 3.3 현재 host 관찰에는 provider·endpoint·model이 없다

- `Archive Center.js:15183`
  - `buildRisuRequestObservation`은 request type, lifecycle, observed role만 보낸다.
- `Archive Center.js:15174`
  - 현재 host capability는 final request payload를 `not_exposed`로 기록한다.
- RisuAI 공식 `getDatabase()` 허용 key에는 현재 선택된 `aiModel`, provider, endpoint가 포함되지 않는다.
- RisuAI 공식 `getRuntimeInfo()`는 API version, platform, save method만 반환한다.

Archive Center 내부의 Publisher·Critic provider 설정은 RisuAI 본문 모델 provider 관찰값이 아니다. 이를 대신 사용하면 안 된다.

### 3.4 현재 lineage 계약과의 충돌

- `go-service/internal/httpapi/output_fidelity_lineage.go:176`
  - 공식 Risu request ID는 노출되지 않는 것으로 기록한다.
- `go-service/internal/httpapi/output_fidelity_lineage.go:282`
  - final provider payload는 `not_exposed`로 계획된다.
- `go-service/internal/httpapi/output_fidelity_lineage.go:341`
  - complete-turn 검증은 final provider payload state가 `not_exposed`가 아니면 현재 계약에서 거절한다.

body interceptor가 실제 provider body 적용을 관찰하게 되면 이 계약을 조용히 우회해서는 안 된다. 새로운 버전의 전송 적용 관찰 계약을 만들고 기존 텍스트 경로의 lineage를 그대로 보존해야 한다.

## 4. RisuAI 공식 소스 감사

감사 기준:

- 저장소: `https://github.com/kwaroran/Risuai`
- commit: `f4290c6cc8765c8bfe53e925471df658d3dabcc1`
- commit 시각: 2026-08-22

### 4.1 beforeRequest만으로는 PDF part를 표현할 수 없다

`src/ts/plugins/apiV3/risuai.d.ts`의 `OpenAIChat.content`는 `string`이다. 따라서 기존 `beforeRequest` replacer만으로 provider-native PDF part를 안전하게 표현할 수 없다.

### 4.2 공식 body interceptor는 존재한다

같은 타입 정의에는 다음 공식 API가 있다.

`registerBodyIntercepter(callback: (body: any, type: string) => any)`

공식 철자는 `Intercepter`다. replacer permission이 필요하며, callback에는 body와 type만 전달된다.

### 4.3 Google AI Studio와 Vertex는 같은 interceptor type을 쓴다

`src/ts/process/request/google.ts`에서 다음을 확인했다.

- Google AI Studio와 Vertex 모두 Gemini `contents[].parts[]` body를 사용한다.
- `inlineData` part는 `mimeType`과 base64 `data`를 받는다.
- streaming은 `gemini_base_stream`, non-streaming은 `gemini_base`다.
- 모델 ID는 body가 아니라 URL에 들어간다.
- Google AI Studio URL과 Vertex URL은 서로 다르다.
- custom Google-compatible URL도 같은 body/interceptor 경로를 사용할 수 있다.

`fetchNative`에 넘길 때 body는 JSON 문자열이다. 따라서 interceptor는 object만 가정하지 말고 문자열 parse·동일성 검증·문자열 재직렬화를 명시적으로 처리해야 한다.

### 4.4 OpenAI 호환 경로는 endpoint를 알 수 없다

`src/ts/process/request/openAI/requests.ts`에서 streaming은 `openai_streaming`, non-streaming은 `openai_basic`을 사용한다. body에는 model이 있을 수 있지만 callback은 실제 URL을 받지 않는다. 따라서 같은 OpenAI body를 쓰는 LLM Gateway, OpenRouter, 사용자 지정 endpoint를 안전하게 구분할 수 없다.

## 5. 공식 제공자 문서에서 확인된 사실

### 5.1 Gemini API

공식 문서:

`https://ai.google.dev/gemini-api/docs/document-processing`

확인 사항:

- PDF inline data와 Files API를 지원한다.
- PDF는 최대 50 MB 또는 1,000 pages다.
- 큰 페이지는 최대 3072 x 3072로 종횡비를 유지해 축소한다.
- Gemini 3는 PDF 네이티브 텍스트를 추출해 모델에 제공한다.
- 추출 네이티브 텍스트에서 생긴 token은 과금하지 않는다고 명시한다.
- PDF page 처리 token은 usage metadata에서 IMAGE modality로 보고된다.

### 5.2 Vertex AI

공식 문서:

`https://cloud.google.com/vertex-ai/generative-ai/docs/multimodal/document-understanding`

확인 사항:

- PDF를 Gemini request에 포함할 수 있다.
- 공식 예시는 `fileData`/`FileData`를 사용한다.
- 실제 Archive Center 방식은 외부 파일 URI가 아니라 inline base64이므로 Vertex REST의 정확한 inline field와 media resolution 위치를 구현 전에 다시 고정해야 한다.
- token·과금 효과는 Google AI Studio와 같다고 추정하지 않고 실제 `usage_metadata`로 측정한다.

### 5.3 LLM Gateway

공식 문서:

`https://docs.llmgateway.io/features/documents`

확인 사항:

- Google Gemini 문서 입력을 Google AI Studio 경로로 지원한다.
- OpenAI 형식의 `type: file`과 `file.file_data` base64 data URL을 사용한다.
- pay-as-you-go API는 provider-specific routing과 no-fallback 설정을 제공한다.
- DevPass는 provider pinning을 제공하지 않는다.
- API 기능은 확인됐지만 현재 RisuAI interceptor에서 endpoint를 확정할 수 없으므로 자동 적용은 보류한다.

### 5.4 OpenRouter

공식 문서:

`https://openrouter.ai/docs/guides/overview/multimodal/pdfs`

확인 사항:

- PDF URL 또는 base64 `file` block을 지원한다.
- native 지원 모델이면 provider에 전달하고, 그렇지 않으면 parser를 사용할 수 있다.
- provider order/only와 fallback 비활성화로 Google Vertex endpoint를 제한할 수 있다.
- parser가 개입하면 이 실험의 Gemini 3 native PDF 비용 계약과 동일하지 않다.

### 5.5 OpenAI와 Anthropic

OpenAI 공식 문서:

`https://developers.openai.com/api/docs/guides/file-inputs`

Anthropic 공식 문서:

`https://platform.claude.com/docs/en/build-with-claude/pdf-support`

두 API 모두 PDF 입력을 지원한다. 그러나 각자 텍스트·페이지 이미지 처리 및 token 비용 계약을 가진다. Gemini 3의 네이티브 PDF 텍스트 무과금 실험으로 확장해서는 안 된다.

## 6. 반드시 보존할 기준선

이 실험은 다음을 변경하지 않는다.

- 기억 저장·검색·선별·중복·중요도 정책
- branch 계보와 공개 범위
- 주관 기억의 owner와 visibility
- 일반 기억, 원작 DB, 로어북, Publisher 안내의 예산 레인
- Publisher와 Critic 호출 형식
- Actual User Input, Recent Chat, Language Context
- 로어북 참조와 원작 자료의 텍스트 전달

PDF 후보는 `long_term_memory` lane의 최종 전달 text 하나뿐이다. `original_work`, `lorebook_reference`, `output_guidance`는 텍스트로 남긴다.

## 7. 안전한 적용 원칙

### 7.1 text-first fail-open

`beforeRequest` 단계에서는 기존 text baseline을 우선 삽입한다. body interceptor에서 다음 조건을 모두 만족했을 때만 장기기억 원문을 정확히 제거하고 PDF part를 삽입한다.

- Go transport plan ID와 현재 pending plan이 일치한다.
- body에 존재하는 Archive Center 보조 블록 hash가 계획 hash와 일치한다.
- `long_term_memory` lane의 exact text와 hash가 일치한다.
- PDF bytes와 logical text hash가 일치한다.
- 파일 part 삽입 후 장기기억 전체 text가 중복으로 남지 않았음을 관찰한다.

하나라도 실패하면 body를 변경하지 않는다. 그러면 기존 text 기억이 그대로 전달된다. 이는 두 번째 LLM 호출이나 숨은 retry가 아니라 같은 요청에서의 명시적 text transport 유지다.

새로운 보호 정책으로 기억 항목을 탈락시키지 않는다. 실패 판정 단위는 기억 선택이 아니라 전송 변환 전체다.

### 7.2 요청 상관관계

전역 pending plan 하나를 “다음 요청”에 무조건 적용하면 Publisher, Critic, 번역, 재생성, 동시 요청에 잘못 붙을 수 있다. provider 이름이나 prompt 문구가 아니라 다음 값으로 결합해야 한다.

- prepare-turn correlation ID
- payload plan ID
- exact auxiliary hash
- long-term memory lane hash
- 적용 가능한 lifecycle와 request type

공식 Risu request ID가 노출되지 않는 현재 한계는 진단에 그대로 표시한다.

### 7.3 한 페이지 PDF는 실험 변수

한 페이지 PDF를 최초 기준으로 시험하되 성공을 가정하지 않는다.

검사 항목:

- PDF text extraction의 처음·중간·끝 동일성
- 3072 x 3072 축소 후 한글 가독성
- 처음·중간·끝 사실 회수
- 검색·복사 가능 여부
- embedded font 또는 font subset에 따른 파일 크기
- base64 변환 전후 브라우저 peak memory

한 페이지가 실패해도 이번 감사 단계에서 임의로 다중 페이지나 이미지 PDF로 정책을 바꾸지 않는다. 결과를 먼저 보고한다.

## 8. 새 세션 작업 순서

### 단계 0 — 이 문서와 경계 재확인

다음 파일을 먼저 읽는다.

1. `AGENTS.md`
2. `docs/permanent-risu-host-backend-boundary.md`
3. `docs/4.0-memory-restoration-work-contract.md`
4. 이 문서

첫 세션의 단일 목표는 단계 1만 수행하는 것이다.

### 단계 1 — 독립 PDF 생성기와 동일성 시험

런타임 JS를 바꾸지 않는다.

- Go가 받은 확정 `long_term_memory` text를 재작성 없이 PDF로 만든다.
- 합성 한국어 자료만 사용한다.
- 검색 가능한 text PDF인지 확인한다.
- PDF에서 추출한 Unicode text와 기준 text를 비교한다.
- 처음·중간·끝 marker를 검증한다.
- 한 페이지 크기, PDF bytes, base64 chars, 생성 시간, peak memory를 기록한다.
- 전체 한글 font를 무조건 asset으로 추가하지 말고 font subset 또는 기존 배포 자산 가능성을 먼저 측정한다.

통과 조건이 충족되기 전에는 provider 호출과 Archive Center.js body interceptor를 만들지 않는다.

### 단계 2 — Go 전송 계획 계약

단계 1 통과 후 진행한다.

- 기존 `payload_application_plan.v1`을 조용히 재해석하지 않는다.
- 논리적 lane과 물리적 transport를 분리한 versioned contract를 만든다.
- Go가 text baseline, PDF bytes 또는 data, logical text hash, filename, page count, transport reason을 소유한다.
- 적용 실패 시 장기기억을 비우지 않는다.
- DB table이나 기억 schema를 추가하지 않는다.

### 단계 3 — 얇은 RisuAI body interceptor

- `gemini_base`와 `gemini_base_stream`만 최초 대상으로 한다.
- JSON string/object를 모두 처리한다.
- exact plan/hash match에서만 장기기억 text를 PDF `inlineData` part로 교체한다.
- 다른 lane은 기존 text로 유지한다.
- 적용 결과를 Go와 Effective Input에 보고한다.
- unload 시 interceptor를 반드시 해제한다.
- JavaScript에 provider 정책, 기억 정책, PDF 선별 정책을 넣지 않는다.

Google direct·Vertex direct를 구분할 신뢰 가능한 host 관찰값이 없으면 자동 추측하지 않는다. 이 경우 명시적 실험 설정 또는 RisuAI API 보강 중 하나를 설계 선택으로 보고하고 사용자 승인을 받는다.

### 단계 4 — Google AI Studio 외부 검증

동일한 합성 기억으로 text와 PDF를 비교한다.

- final HTTP body
- PDF 수락 여부
- prompt/input 및 modality token
- 비용
- 지연
- 처음·중간·끝 회수
- 관계·상태·owner 유지

### 단계 5 — Vertex AI 외부 검증

Google 결과를 재사용하지 않고 같은 항목을 별도로 측정한다.

- Vertex endpoint와 인증
- inline PDF field
- media resolution
- usage metadata
- token·비용·지연·회수

### 단계 6 — 조건부 provider 재평가

LLM Gateway와 OpenRouter를 통해 Gemini 3 이상을 사용할 때에는 다음이 모두 확인된 경우에만 포함한다.

- RisuAI가 실제 provider와 endpoint를 요청 시점에 신뢰 가능하게 제공
- model ID와 문서 capability 확인
- native PDF transport 확인
- Google AI Studio 또는 Vertex upstream 고정
- provider fallback 비활성화 또는 동일 Google upstream 안에서만의 fallback 증명
- exact body 적용과 중복 제거 확인
- 실제 외부 요청 성공
- usage와 upstream routing 확인

OpenAI와 Claude는 별도 실험 작업으로 분리한다.

### 단계 7 — UI·lineage·패키지

외부 provider 검증 후에만 진행한다.

- Effective Input에 logical memory text는 계속 표시
- `memory_transport`, filename, pages, logical chars, file block applied, duplicate text를 표시
- 실제 provider body 적용 관찰을 versioned lineage로 기록
- source test, Go regression, `node --check "Archive Center.js"`, loaded RisuAI, package evidence를 구분
- 테스트 패키지는 모든 소스 검증 후 갱신

## 9. 단계 1 작업 중지 조건

다음 중 하나면 단계 1 결과만 보고하고 런타임 통합으로 넘어가지 않는다.

- PDF 추출 text가 기준 text와 구조적으로 달라짐
- 한글 font가 깨지거나 검색·복사가 불가능함
- 한 페이지 축소로 중간·끝 기억 회수가 불안정함
- PDF/base64 peak memory가 브라우저 안정성을 위협함
- 배포 패키지에 과도한 font 또는 PDF dependency가 필요함
- text baseline을 남기지 않고서는 안전한 fail-open을 만들 수 없음

## 10. 근거 상태표

| 항목 | 상태 | 근거 |
| --- | --- | --- |
| Go가 기억 선택·lane·budget을 소유 | VERIFIED | 현재 Go 소스 |
| JS가 현재 텍스트 보조 블록을 적용 | VERIFIED | 현재 `Archive Center.js` |
| RisuAI body interceptor 존재 | VERIFIED | RisuAI 공식 API v3 타입·구현 |
| Google direct·Vertex가 같은 Gemini interceptor type 사용 | VERIFIED | RisuAI 공식 `google.ts` |
| interceptor가 URL·provider·model을 전달 | FALSE | callback은 body와 type만 받음 |
| Gemini 3 PDF 네이티브 추출 text token 무과금 | VERIFIED_DOC | Google 공식 문서 |
| Vertex direct PDF 입력 | VERIFIED_DOC | Google Cloud 공식 문서 |
| 한 페이지 장문 PDF가 정확하고 저렴함 | UNKNOWN | 외부 실험 필요 |
| Google AI Studio 실제 호출 성공 | UNVERIFIED_LIVE | 미실행 |
| Vertex 실제 호출 성공 | UNVERIFIED_LIVE | 미실행 |
| LLM Gateway를 RisuAI 요청 시점에 안전하게 식별 | UNVERIFIED_HOST | endpoint 미노출 |
| OpenRouter를 RisuAI 요청 시점에 안전하게 식별 | UNVERIFIED_HOST | endpoint 미노출 |
| 실제 RisuAI loaded plugin과 본문 기억 효과 | UNVERIFIED_LIVE | 미실행 |

## 11. 이번 감사의 변경 범위

- 런타임 Go 변경: 없음
- `Archive Center.js` 변경: 없음
- JavaScript 추가 줄: 0
- JavaScript 삭제 줄: 0
- DB schema/table 변경: 없음
- 외부 provider 호출: 없음
- 추가 파일: 이 문서 1개

## 12. 새 세션 시작 요청문

다음 요청을 새 세션에 그대로 전달할 수 있다.

> 활성 소스 `C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source`에서 `AGENTS.md`, `docs/permanent-risu-host-backend-boundary.md`, `docs/4.0-memory-restoration-work-contract.md`, `docs/gemini3-pdf-memory-transport-experiment-audit-and-handoff.md`를 전부 읽어라. 이번 세션에서는 인계 문서의 단계 1인 독립 Go PDF 생성기와 동일성 시험만 구현하라. Archive Center.js, provider 요청, 기억 선택·예산·저장 정책, DB schema를 변경하지 마라. 합성 한국어 장문으로 PDF text 추출의 처음·중간·끝, 검색·복사 가능성, bytes/base64 chars, 생성 시간과 메모리 비용을 검증하고 결과를 보고하라. 기존 dirty 변경을 보존하고 작업 전후 diff 범위를 증명하라.
