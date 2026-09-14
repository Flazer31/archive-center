# 전처리 담당별 추론 설정 — 2026-09-11

소스 구현·격리 설정/요청 검증 기록. 실제 사용 중인 RisuAI·유료 제공자 응답은 `implemented_unverified`.
4.4-B 동등성 리팩터링 이후 사용자가 별도로 요청한 기능 보완이다.

## 결함과 변경

- 일반 텍스트였던 전처리 추론 입력을 기존 Go `POST /config/view-model`의 제공자·endpoint·모델별 선택지/예산 입력으로 바꿨다.
- 출판사 연결을 공유해도 추론 preset/budget/toggle을 모두 전달하지 않았다. `callMultiAgent`가 연결의 추론 값을 받고 담당의 effort/budget만 덮어쓴다. 담당별 모델·키·프롬프트·온도·출력 토큰·대기 시간은 기존 소유자를 유지한다.
- Kimi family 누락, GLM 5.3에 적용되던 이전 버전의 off/강도 변환, gateway GLM max→high 축소를 수정했다.
- DeepSeek V4.1/`deepseek-flash`를 감지한다. 직접 연결의 빈 effort는 제공자 기본값으로 유지하며 공식 별칭 변환을 적용한다. Ollama의 저장된 max가 화면 high/요청 none으로 갈리던 경우도 기존 wire의 high에 맞췄다.

## 모델별 계약

| 모델 | 설정 | 직접 연결 필드 |
| --- | --- | --- |
| GLM 4.7/5.1 | 켜기·끄기 | `thinking.type` |
| GLM 5.2 | none/high/max | `thinking.type` + `reasoning_effort` |
| GLM 5.3/5.3-flash | low/high/max, 항상 추론 | enabled + `reasoning_effort` |
| Kimi K2.5/2.6 | 켜기·끄기 | `thinking.type` |
| Kimi K2.7-code/highspeed | 기본 추론, 별도 강도 없음 | 선택 필드 생략 |
| Kimi K3 | low/high/max, 항상 추론 | `reasoning_effort` |
| DeepSeek V4/V4.1/flash | none/low/high/max | `thinking.type` + `reasoning_effort` |
| GPT·Gemini·Claude | 기존 버전별 강도/예산/자동 추론 | 기존 제공자 owner |

LLM Gateway는 `reasoning_effort`, OpenRouter는 `reasoning.effort`를 사용한다.
Ollama는 기존 지원 단계로 변환하고 기존 Neuralwatt 개별 제한도 유지한다.
Kimi는 모드별 기본 sampling을 사용하도록 온도 입력을 비활성화하고 temperature를 생략한다(Ollama 제외).
GLM 5.3의 예전 none/minimal/disable은 low로 읽는다. DeepSeek는 minimal→low, medium/xhigh→high, ultra→max다.
직접 API 계약과 gateway의 모델 공급/단계 지원은 별도다. 모델 목록 자동 조회나 검증용 유료 호출은 추가하지 않았다.

## 설정·소유권

- 기존 전처리 설정 JSON role에 optional `reasoning_budget_tokens`를 추가한다. SQL schema 변경은 없다.
- effort 빈칸은 연결 기본값. 출판사 공유 시 현재 출판사 값을, 독립 연결은 그 모델의 기본값을 사용한다.
- budget 미지정은 연결 값 유지, 양수는 담당 값, 0은 상속된 수치 예산 해제다. 0이 모든 모델에서 추론 끄기를 뜻하지 않는다. 모델별 기존 출력 토큰/추론 예산 제약은 유지한다.
- 공유 연결 미리보기는 Go가 실제 출판사 설정을 읽는다. 비활성화된 독립 모델 입력으로 옵션을 만들지 않는다.
- JS는 폼 값·이벤트·표시·저장 전송만 담당한다. 모델 판정·값 변환·API 요청은 Go 소유다.
- 같은 설정을 1차·2차에 적용한다. 프롬프트, AI 추천/순서, 빈 추천 Go 선정, 검색 횟수, timeout·재시도는 바꾸지 않았다.

## 공식 근거

2026-09-11 확인. 이후 제공자 변경은 재확인이 필요하다.

- [GLM 5.3](https://docs.z.ai/guides/llm/glm-5.3), [5.3 Flash](https://docs.z.ai/guides/vlm/glm-5.3-flash), [Thinking](https://docs.z.ai/guides/capabilities/thinking)
- [Kimi 모델 목록](https://platform.kimi.ai/docs/api/models-overview), [effort](https://platform.kimi.ai/docs/guide/use-reasoning-effort), [thinking](https://platform.kimi.ai/docs/guide/use-thinking-models), [K2.5 공식 예제](https://github.com/MoonshotAI/Kimi-K2.5#6-model-usage)
- [DeepSeek thinking/별칭](https://api-docs.deepseek.com/guides/thinking_mode/), [현재 모델 별칭](https://api-docs.deepseek.com/quick_start/pricing/)
- [LLM Gateway reasoning](https://docs.llmgateway.io/features/reasoning), [요청 계약](https://docs.llmgateway.io/v1_chat_completions), [OpenRouter reasoning](https://openrouter.ai/docs/guides/best-practices/reasoning-tokens)

## 검증

- `preprocessing_reasoning_test.go`: 실제 설정 저장/재조회 → 1차·2차 `callMultiAgent` → HTTP 요청 본문. 상류 응답만 fixture로 대체하고 GLM/Kimi/DeepSeek/Gemini/Claude/GPT 및 연결 상속을 검사한다.
- `llm_settings_view_test.go`: 기존 486개 표본을 보존한다. 승인된 모델 변경과 확인된 Ollama 표시/전송 차이만 독립 기대값으로 수정했으며 frozen JSON은 바꾸지 않았다.
- `ops/settings-pair-smoke.cjs --preprocessing-reasoning`: 전체 JS와 실제 Go, 격리 브라우저 저장소로 5개 담당의 옵션·저장·새로고침·재열기·출판사 상속을 검사한다.
- 증거: 저장소 상위 `_diagnostics/20260911-preprocessing-reasoning/`. 최종 검사·패키지 결과는 아래 완료 기록에 기재한다.

진행 중인 사용자의 4.3 서비스·플러그인을 교체하거나 GitHub에 업로드하지 않았다.

## 로컬 완료 기록

- 전체 `go test -mod=readonly ./... -count=1 -json`: **5,830 통과, 실패 0, 건너뜀 13**. 실제 DB/제공자/OS 환경이 필요한 검사는 통과에 포함하지 않았다.
- `node --check "Archive Center.js"`, prompt/API-key/Flex/저장 실패 보존 UI 검사 통과.
- **4.4.0-test.5 패키지 전체 JS + 실제 Go**의 설정 조회·5개 담당별 선택·저장·새로고침·재열기·출판사 연결 상속 통과. 증거 `ui-package-test5/result.json`과 화면 캡처.
- 최신 Windows 패키지: [`Archive Center 4.4.0-test.5 Windows Test.zip`](../_dist/4.4.0-test.5-hudfix/Archive%20Center%204.4.0-test.5%20Windows%20Test.zip). 01 실행기·07 오류 보고 파일을 포함한다. 후속 HUD 버전 수정으로 BUILD_ID도 VERSION을 따른다.
- 최신 ZIP SHA256: `48f57dce3861eeec98739f07529a5a1b2a3b077f0c5d1f928cf658bae5271cd2`. 최초 ZIP의 SHA256은 `f8ddfc9fb280d9f01672dc811feb267848c76ca1abff6f883dd693bb4d3bc7ff`였으며 HUD에 구버전 표기가 남아 있었다. 설정 검사는 이를 포착하지 못했다.
- 후속 버전 표시 수정은 JS +1/-1이며 전용 패키지 버전/HUD 검증을 포함한 422개 JS 회귀를 통과했다. 최신 패키지 JS 문법 통과, Go 바이너리는 최초 test.5와 hash가 동일하다.
- 이번 기능의 JS 변경량 **+27/-6**. 기존 DOM 필드 바인딩과 저장 전송에 필요한 변경이며 Go 모델 정책을 JS에 복제하지 않았다. 기존 진단/C 작업의 변경량은 포함하지 않았다.
- 실제 사용자 RisuAI 등록/유료 provider 성공, 사용자 DB/Chroma·다른 OS 실행은 이번 검사에 포함하지 않았다. 본 테스트 패키지는 로컬 검증용이며 정식 4.3.1 소스 버전/공개 릴리스는 변경하지 않았다.
