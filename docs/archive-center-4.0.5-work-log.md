# Archive Center 4.0.5 작업 기록

기준일: 2026-08-26 KST

## 1. 작업 기준과 복구점

- 4.0.4 전체 상태를 먼저 로컬 커밋 `4a30fb3`으로 고정했다.
- 이번 변경은 Publisher 모델 입력 경량화, 실패 진단, Publisher 전용 JSON
  Schema 충돌 판정, 4.0.5 버전 표식과 Windows 테스트 패키지 생성으로
  제한했다.
- 기억·로어북 검색/선택/예산, 최종 본문 Payload 레인과 순서, DB와
  Schema, `publisher_output.v3`, `publisher_plan.v2`, 정확한 `source_ref`
  검증, 단일 호출, 숨겨진 재시도 금지, malformed fail-open은 변경하지
  않았다.

## 2. Publisher 모델 입력 경량화

- 사용자 Payload의 중복 `required_output`을 제거하고 출력 계약은 시스템
  프롬프트와 Provider JSON Schema가 소유하도록 했다.
- 모델 입력 JSON을 들여쓰기 없는 JSON으로 변경했다.
- `publisher_strength_profile`은 내부 trace와 UI용 상태에는 유지하되 모델
  입력에서는 제거했다.
- 내부 `supervisor_support_packet.v2`와
  `response_execution_contract.v1`은 그대로 유지한다. 모델에게 보내는
  복사본에서만 고정 count/status/감사 필드와 중복 visibility 정보를
  제외했다.
- 현재 입력, 실제 전달 텍스트, `source_ref`/`source_refs`, 보호·개인정보
  guard, authority, class/kind/source scope와 `must_*` 지침은 유지한다.

## 3. 진단과 JSON Schema 경계

- Publisher JSON 실패 trace에 제한된 `raw_preview`, 실제 parser 오류,
  최상위 객체 수, 미완성 여부, 가능한 경우 syntax offset과 중복 키 이름을
  기록한다. API key와 일반 secret 표지는 scrub하며 전체 프롬프트는 저장하지
  않는다.
- 호출 장부에 지원 자료의 실제 텍스트/메타데이터 chars, 실행 지침/메타데이터
  chars, 실제 적용된 JSON 형식과 Schema 출처를 추가했다.
- 직접 OpenAI Publisher 호출은 백엔드의 `publisher_output.v3` strict
  JSON Schema를 기본값으로 유지하고, 사용자 `json_object`가 이를 약화시키는
  경우 실제 Provider 호출 전에 명시적 설정 충돌로 판정한다.
- OpenRouter/LLM Gateway/Vercel처럼 여러 모델과 상류 Provider를 중계하는
  경로는 특정 모델의 strict JSON Schema 지원을 가정하지 않고 portable
  `json_object`를 기본값으로 사용한다. 정확히 일치하는 명시적
  `publisher_output.v3` JSON Schema가 들어온 경우에는 그대로 유지한다.
- strict Schema 내부의 선택적 `level` 필드는 표준 항목과
  `pressure_level` 항목을 분리한 두 객체 변형으로 표현했다. 따라서 strict
  검증 규칙을 지키면서도 기존 Publisher 결과 의미와 Go 파서는 바뀌지 않는다.
- 사용자 `json_schema`, Claude `output_config.format.schema`, Gemini
  `responseJsonSchema`는 Publisher Schema와 일치할 때만 유지한다.
- Custom/Ollama Publisher의 기존 `json_object` 호환과 Publisher 외 호출
  정책은 유지한다. 모델명 allowlist는 추가하지 않았다.

## 4. 회귀 검증

- `go test ./internal/httpapi -count=1`: 통과
- `go test ./... -count=1`: 통과
- LLM Gateway가 strict Schema를 HTTP 400으로 거절하는 재현 응답을 사용한
  회귀에서 `json_object` 단일 호출로 200 응답을 받는 경로: 통과
- Publisher strict Schema의 표준 항목/pressure 항목 분리 검증: 통과
- 번들 Node를 지정한 `cmd/js-route-variant-smoke`: 통과
- source `Archive Center.js`의 `node --check`: 통과
- 패키지 내부 `Archive Center.js`의 `node --check`: 통과
- `git diff --check`: 내용 오류 없음. Windows 작업 트리의 LF→CRLF 안내만 존재
- KiMi, GLM 구현 작업자와 DeepSeek 독립 검토자는 현재 ChatGPT 계정에서
  해당 cloud 모델을 지원하지 않아 실행되지 않았다. 이 공백은 전체 Go 회귀와
  국소 회귀로 대체했으며 외부 Provider 실호출 증거로 간주하지 않는다.

이번 변경의 `Archive Center.js` 증감은 버전·빌드 시각 문자열 교체만
`+6 / -6`이다.
Publisher 조립·검증·진단은 Go 백엔드가 소유한다.
이번 Provider 호환성 보완 조각의 `Archive Center.js` 증감은 `+0 / -0`이다.

## 5. Windows 테스트 패키지

- output root:
  `_test-builds/Archive-Center-4.0.5-publisher-compact-windows-test`
- 설치 폴더:
  `Archive Center 4.0.5 Windows Auto Install Package`
- ZIP:
  `Archive Center 4.0.5 Windows Auto Install Package.zip`
- ZIP SHA-256:
  `82576e8fc00b3af2d1c9e84d3e386661a30a35385fe0986a2adb1bbbde4e5e10`
- package status: `green`
- package version: `4.0.5`
- automatic update apply metadata: `true`
- Go toolchain: `go1.26.6 windows/amd64`
- manifest 포함 파일: 46개
- 누락 파일: 0개
- 관리 파일 46개 manifest SHA-256 전수 검증: 불일치 0개
- source와 패키지의 `Archive Center.js`, `critic_system.txt`,
  `supervisor_system.txt` hash 일치

이 테스트 패키지는 실제 RisuAI 표시 결과, 외부 Grok/LLM Gateway 호출,
사용자 MariaDB·ChromaDB 데이터와 전체 OS 배포 패키지를 검증한 증거는 아니다.
