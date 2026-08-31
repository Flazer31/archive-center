# Archive Center 4.0.8 작업 기록

기록일: 2026-08-28

상태: 활성 source 수정·회귀 검증 완료, 기존 Windows 테스트 패키지는 후속 source 갱신 전 산출물

## 1. 기준과 범위

- 활성 소스는 `source/Archive Center.js`와 `source/go-service`이다.
- 4.0.1~4.0.7의 이전 피드백·수정 기록은
  `archive-center-4.0.1-4.0.7-integrated-work-record.md`와
  `archive-center-4.0.2-4.0.7-feedback-work-record.md`를 유지한다.
- 4.0.8은 해당 기록을 다시 쓰는 버전이 아니라, 4.0.7 이후 사용자 테스트에서
  확인된 아래 항목을 현재 활성 소스에 합쳐 고정하는 버전이다.
- DB 스키마와 새 테이블은 추가하지 않았다.
- 모델명 allowlist, 숨겨진 재시도, 항목 하나의 결함으로 전체 결과를 버리는
  보호 정책은 추가하지 않았다.

## 2. 사용자 피드백과 반영 내용

### 2.1 출판사·평론가 타임아웃 설정 연결

피드백:

- UI에서 타임아웃을 늘려도 출판사는 60초, 평론가는 90초로 계속 종료됐다.

확인된 원인:

- 출판사·평론가별 LLM 설정에는 millisecond 타임아웃이 있었지만, 별도의
  backend timeout 필드가 60초·90초 기본값으로 남아 실제 호출 제한을 다시
  고정하고 있었다.

반영:

- 출판사는 `pluginMainTimeoutMs`, 평론가는 `subLlmTimeoutMs`를 backend runtime
  config와 실제 평론가 호출에 전달한다.
- 중복된 공통 `supervisorTimeout`·`criticTimeout` 입력 UI는 제거했다.
- backend 소유 호출 제한은 유지하며, JS는 사용자가 저장한 값을 전달한다.

### 2.2 주관 기억 중요도·감정 가중치

피드백:

- 서로 다른 주관 기억이 모두 `imp 5.0`으로 저장되는 현상이 확인됐다.

확인된 원인:

- 직접 `subjective_entity_memories`의 점수는 보존할 수 있었지만,
  `belief_updates`에서 주관 기억을 파생할 때 항목별 점수를 복사하지 않아
  기본값으로 수렴했다.

반영:

- `belief_updates.importance_10`과 `emotional_weight`를 파생 주관 기억으로
  전달한다.
- 기존 별칭 `importance_score`·`emotional_intensity`도 항목 단위로 인정한다.
- 점수가 없는 항목만 기존 기본값 `5 / 0.5`를 사용한다.
- 점수 누락 항목 때문에 같은 턴의 다른 정상 항목을 버리지 않는다.
- 평론가 프롬프트의 주관 기억·belief 객체 예시에 선택 점수 필드를 추가했다.

### 2.3 KG의 시간 의미와 현재 상태 오인 방지

피드백:

- 과거의 `은 15냥` 같은 관계 지식이 모두 `현재 유효`로 표시되어 현재
  재산이나 상태처럼 오인될 수 있었다.

반영:

- `valid_to`가 비어 있는 KG는 `현재 유효`가 아니라 `종료 미기록`으로
  표시한다.
- 본문 모델에는 KG를 `현재 상태 권위`가 아닌 시간 정보가 붙은 관계·사건
  지원 이력으로 전달한다.
- 각 항목에 source turn과 valid range를 붙이며 branch 기준 턴 밖의 종료된
  항목과 아직 시작하지 않은 항목은 전달하지 않는다.
- 오래된 열린 약속·관계를 삭제하지 않으며, DB 이력도 변경하지 않는다.
- 돈·소유·위치·신분처럼 바뀌는 현재값은 평론가의 기존 `state_claims`
  표면으로 추출하도록 짧은 객체 계약을 보완했다.
- KG 의미 비교만으로 최신 사실을 추측해 강제로 종료하거나 통합하지 않는다.

### 2.4 NeuralWatt 출판사·평론가 Provider

피드백:

- NeuralWatt를 Custom으로 호출할 때 연결 테스트와 최종 텍스트 처리가
  불안정했고, RisuAI에서는 `service_tier=flex` 스트리밍을 사용할 수 있었다.

반영:

- 출판사와 평론가 Provider 선택에 `NeuralWatt`를 추가했다.
- 기본 endpoint는 `https://api.neuralwatt.com/v1`이며 기존 OpenAI 호환
  요청 계약을 사용한다.
- Standard는 기존 JSON 응답 경로를 유지한다.
- Flex는 `service_tier=flex`, `stream=true`, usage 포함 요청을 보내고 Go
  백엔드가 SSE의 본문·추론·usage·finish reason·service tier를 표준 응답으로
  조립한다.
- 추론 내용만 최종 평론가 JSON으로 오인하지 않는다.
- 완료 표시 없이 끊긴 스트림이나 completion chunk가 없는 스트림은 부분
  결과로 저장하지 않고 명시적으로 실패한다.
- Flex 실패 시 Standard로 숨겨서 재호출하지 않는다.

### 2.5 세계 규칙의 180자 강제 축약 제거

피드백:

- 실제 최종 입력에서 세계 규칙 두 항목이 JSON 문장 중간에서 잘린 상태로
  본문 모델에 전달됐다.

확인된 원인:

- 최종 lane 예산 선택은 항목 단위였지만, 그 이전 조립 단계에서 세계 규칙
  설명을 먼저 180자로 잘랐다. 따라서 예산에 여유가 있어도 이미 손상된
  항목이 전달됐다.

반영:

- 세계 규칙 조립의 고정 180자 축약을 제거했다.
- 세계 규칙은 원문 단위로 최종 예산 선택기에 전달된다.
- 최종 예산이 부족하면 기존처럼 항목 전체를 선택하거나 제외하며, 문장
  중간을 잘라 맞추지 않는다.
- 다른 기억 lane의 선택·예산·축약 정책은 변경하지 않았다.

### 2.6 DeepSeek V4 `low` 추론 강도 전달

피드백:

- DeepSeek 공식 API에는 `low` 추론 강도가 존재하지만 Archive Center의
  DeepSeek V4 설정에는 `none/high/max`만 표시됐다.
- Ollama, OpenCode Zen, NeuralWatt와 같은 비공식 제공 경로에서도 실제
  Provider가 DeepSeek V4의 `low`를 인식한다면 해당 값을 선택하고 그대로
  전달할 수 있어야 한다는 요청이 있었다.

확인된 원인:

- UI의 Direct DeepSeek 및 gateway용 DeepSeek V4 선택지가 `low`를 제외했다.
- Go 백엔드의 Direct DeepSeek와 공용 gateway 변환기가 입력된 `low`를
  호출 전에 `high`로 승격했다.
- Ollama OpenAI 호환 경로는 이미 `low`를 보존하고 있었으나 이 동작을
  직접 검증하는 회귀가 부족했다.
- Provider마다 실제 의미가 같지는 않았다. NeuralWatt의
  `deepseek-v4-pro`는 독립적인 `low`를 지원하지만
  `deepseek-v4-flash`와 `-flex` 별칭에는 light tier가 없어 `low`가
  `high` 의미로 처리된다.

확인한 공식 계약:

- DeepSeek Thinking Mode: <https://api-docs.deepseek.com/guides/thinking_mode/>
- Ollama Thinking API 및 DeepSeek V4 Pro 모델 설명:
  <https://docs.ollama.com/capabilities/thinking>,
  <https://ollama.com/library/deepseek-v4-pro>
- NeuralWatt Chat Completions의 모델별 `reasoning_effort` 매핑:
  <https://portal.neuralwatt.com/docs/api/chat-completions>
- OpenCode 모델 variant와 Zen endpoint:
  <https://opencode.ai/docs/models/>, <https://opencode.ai/docs/zen/>

반영:

- DeepSeek 공식 endpoint에서는 UI에 `low`를 표시하고
  `thinking.type=enabled`, `reasoning_effort=low`를 그대로 전송한다.
- LLM Gateway에서는 `reasoning_effort=low`, OpenRouter·Vercel에서는
  `reasoning.effort=low`를 전달한다. Archive Center가 먼저 `high`로 바꾸지
  않으며 실제 지원 여부는 upstream 응답으로 확인한다.
- OpenCode Zen과 같은 Custom OpenAI 호환 endpoint도 모델명이 DeepSeek V4로
  판정되고 사용자가 `low`를 선택하면 `reasoning_effort=low`를 전달한다.
- NeuralWatt `deepseek-v4-pro`는 `low`를 표시·전달한다.
- NeuralWatt `deepseek-v4-flash` 및 `-flex`는 실제 light tier가 없으므로
  `low`를 별도 단계로 표시하지 않고 문서화된 `high` 의미를 유지한다.
- Ollama DeepSeek V4의 기존 `low`·`medium` 전달과 저장값 호환을 유지한다.
- 지원하지 않는 Provider의 오류를 숨기지 않으며 다른 강도로 몰래 재호출하는
  fallback이나 숨겨진 재시도를 추가하지 않았다.

## 3. 주요 변경 파일

### RisuAI host adapter

- `Archive Center.js`
  - 4.0.8 버전·빌드 표기
  - 출판사·평론가 타임아웃 값 전달
  - KG `종료 미기록` 표시
  - NeuralWatt Provider와 설정 표시
  - Provider·모델 계약에 맞는 DeepSeek V4 `low` 선택 표시와 요청 전달

### Go backend

- `go-service/internal/httpapi/turn_extraction_private.go`
  - belief 기반 주관 기억의 점수 전달
- `go-service/internal/httpapi/prepare_turn_assembly.go`
  - KG 시간 지원 이력 조립
  - 세계 규칙 180자 강제 축약 제거
- `go-service/internal/httpapi/prepare_turn_recall.go`
  - KG 선택 진단 용어 정리
- `go-service/internal/httpapi/proxy_provider.go`
  - NeuralWatt Standard/Flex 요청 및 Flex SSE 조립
  - Direct·gateway·Custom OpenAI 호환 DeepSeek V4 `low` wire 값 보존
- `go-service/internal/config/config.go`
  - 기본 빌드 버전 4.0.8

### Prompt

- `prompts/critic_system.txt`
  - 선택적인 주관 기억 점수 예시
  - 변경 가능한 현재 상태용 `state_claims` 예시
  - KG가 현재 상태 권위가 아니라는 지침

### 회귀

- `turn_extraction_critic_test.go`
- `prepare_turn_prompt_hygiene_test.go`
- `group_turn_part03_test.go`
- `group_turn_part06_test.go`
- `group_proxy_test.go`
- `cmd/js-route-variant-smoke`의 관련 테스트
- config 및 server 버전 테스트

## 4. 유지한 경계

- Go 백엔드가 정책·조립·Provider 응답 해석을 소유한다.
- JavaScript는 RisuAI 설정 관측·전달과 UI 표시만 담당한다.
- 기억 DB 이력은 삭제하지 않는다.
- KG `valid_to=0`을 현재 사실이라고 추측하지 않는다.
- 오래됐다는 이유만으로 약속·관계를 제거하지 않는다.
- 주관 기억 점수가 없다고 항목이나 전체 턴을 삭제하지 않는다.
- 세계 규칙을 예산에 맞추기 위해 문자열 중간에서 자르지 않는다.
- NeuralWatt Flex 실패를 Standard로 숨겨 재시도하지 않는다.
- 로어북, 일반 기억, Publisher 출력 계약, DB 스키마는 이번 변경에서
  수정하지 않았다.

## 5. 검증 결과

- 활성 `Archive Center.js` Node 구문 검사: 통과
- 패키지 내부 `Archive Center.js` Node 구문 검사: 통과
- 관련 `internal/config`, `internal/httpapi` 회귀: 통과
- Go 전체 회귀 `go test ./... -count=1`: 통과
- `git diff --check`: 내용 오류 없음. Windows 줄바꿈 전환 경고만 존재
- Windows·POSIX 간편 설치 계약: 통과
- Windows 테스트 패키지 manifest: `green`
- 관리 파일 46개: 누락 0, 크기 불일치 0, SHA-256 불일치 0
- 활성 소스와 패키지의 `Archive Center.js` SHA-256 일치
- 활성 소스와 패키지의 `critic_system.txt` SHA-256 일치
- 실제 NeuralWatt 계정 Standard/Flex 호출: 미검증
- 실제 Direct DeepSeek·LLM Gateway·OpenRouter·OpenCode Zen의 `low` 적용:
  미검증
- 실제 RisuAI·MariaDB·ChromaDB 장기 세션: 미검증

## 6. 테스트 패키지

- output root:
  `_test-builds/Archive-Center-4.0.8-neuralwatt-memory-windows-test`
- 설치 폴더:
  `Archive Center 4.0.8 Windows Auto Install Package`
- ZIP:
  `Archive Center 4.0.8 Windows Auto Install Package.zip`
- ZIP 크기: `12,132,721 bytes`
- ZIP SHA-256:
  `ec29ec8802cc783cfb8784666dd612b461b1c1ab9b766f4fc62c5cff3ab63a6b`
- package status: `green`
- target version: `4.0.8`
- `release_ready=true`
- `automatic_update_apply=true`
- `direct_update_supported=true`
- Go toolchain: `go1.26.6 windows/amd64`
- 관리 파일: 46개
- 누락·크기·SHA-256 불일치: 모두 0개
- source/package `Archive Center.js` SHA-256:
  `772716bf5d90c3b658b5419a98544e6895fe9372b2ccb9f54d907ac7dfe24f09`
- source/package `critic_system.txt` SHA-256:
  `b03b6797619a70c67837107df49abf01a67aa7b89d1d1ab3421bfbfdd77b2c7d`

이 산출물은 Windows 실사용 테스트 패키지다. 전체 OS 4.0.8 정식 릴리스
자산 생성이나 GitHub 업로드를 완료했다는 의미는 아니다.

## 7. JavaScript 증감

현재 작업 트리에는 4.0.7 이후 여러 승인된 수정이 함께 있으므로 최종 diff와
이번 버전 고유 변경을 구분해 기록한다.

- 세계 규칙 축약 제거: JavaScript `+0 / -0`
- NeuralWatt 조각: JavaScript `+12 / -6`
- 4.0.8 버전 표기 교체: JavaScript `+6 / -6`
- DeepSeek V4 `low` 후속 조각: JavaScript `+11 / -7`
- 4.0.8 테스트 패키지 생성 당시 타임아웃 연결·KG 표시·NeuralWatt·버전
  표기를 포함한 JavaScript diff: `+33 / -45`
- 2026-08-28 후속 source 전체 JavaScript diff: `+42 / -39`

## 8. 아직 확인되지 않은 범위

- 실제 NeuralWatt API의 Standard 및 Flex 과금·usage·완료 이벤트
- 실제 Provider별 DeepSeek V4 `low` 수용·적용 결과와 응답 metadata
- 실제 RisuAI에서 저장한 타임아웃이 외부 장시간 모델 호출에 적용되는지
- 실제 사용자 DB에서 신규 주관 기억 점수가 다양하게 생성되는지
- 장기 세계선에서 열린 KG가 지원 이력으로 전달되고 현재 상태로 오인되지
  않는지
- 실제 패키지 설치·업데이트 및 전체 OS 자동 업데이트

소스 회귀와 Windows 테스트 패키지 검증은 위 실환경 증거와 동일하지 않다.
