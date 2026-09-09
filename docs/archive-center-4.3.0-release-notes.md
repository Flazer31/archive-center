# Archive Center 4.3.0

4.3은 기본 기억 전달을 보완하고, 선택형 **편집자(전처리 멀티 에이전트)**를 추가합니다.
전처리·출판사를 끈 사용자에게도 기본 기억 개선이 적용됩니다.

- 사건·진행 이력, 인물의 객관적 상태, 주관 기억·관계, 세계·사물 상태, 미해결 목표를 5개 담당이 분석합니다. 각 담당의 제공자·모델·프롬프트·온도·토큰·추론 설정을 개별 저장합니다.
- 1차 담당 분석은 병렬이며, 필요한 경우 추가 검색과 2차 보완 분석을 수행합니다. AI가 추천한 기억과 순서는 유지하고 추천이 없거나 호출에 실패한 분류는 Go의 기본 선정을 사용합니다.
- 분류별 핵심 기억을 먼저 확보한 뒤 문자 예산에 남은 관련 기억도 전달합니다. 의미 검색 점수와 중요도 처리, 일반 주관 기억, 현재 상태와 과거 기록 구분을 보완했습니다.
- 출판사 가이드 강도와 편집자 결과 전달, JSON 형식 보정·부분 결과 활용을 보완했습니다. 사용자의 입력과 창작 결정권을 유지합니다.
- 하이파 가져오기는 요약 원문 1개를 기억 1개로 보존합니다. 대량 턴 삭제, 긴 기억 저장, 분기·재분기와 PocketRisu 메시지 ID 재발급, 이전 턴 확정 전 분기, 콜드 스타트의 상속 구간·번역 제외 처리를 수정했습니다.
- HUD는 작은 카드와 펼칠 수 있는 상세 정보로 정리했습니다. 저장하지 않은 턴에는 빈 저장 통계를 표시하지 않습니다.
- Gemini 3.8 Flash medium 추론과 Vertex 설정 전달을 보완했습니다. **OpenCode Zen·Go**를 추가하고 기존 **OpenRouter** 연결도 회귀 검증했습니다.

## 설치와 업데이트

신규 사용자는 Windows ZIP의 `01_start_archive_center_windows.bat` 또는
[README의 OS별 한 줄 설치](../README.md#github-fresh-install)를 사용합니다.
기존 사용자는 Archive Center 설정에서 업데이트를 확인하고 적용합니다.
백엔드 업데이트 후 RisuAI의 플러그인 업데이트 또는 새 `Archive Center.js` 가져오기도 필요합니다.

배포 대상: Windows x64, Linux x64·arm64, macOS Intel·Apple Silicon, Termux arm64.
기존 데이터·개인 설정·API 키는 관리 파일 교체 대상에 포함하지 않습니다.

## OpenCode / OpenRouter

OpenCode Zen의 기본 Endpoint는 `https://opencode.ai/zen/v1`이며 API 키와 모델 ID를 입력합니다.
모델 ID는 `deepseek-v4-pro`, `glm-5.2`, `claude-sonnet-4-6`, `gemini-3.8-flash` 같은 API ID를 사용합니다.
Zen은 모델별 API 형식을 사용하므로 Go가 기존 Chat Completions·Responses·Claude·Gemini 호출기로 연결합니다.
전체 API Endpoint를 직접 입력하면 그 형식을 우선합니다.
OpenRouter는 `https://openrouter.ai/api/v1`과 `google/gemini-3.8-flash` 같은 OpenRouter ID를 사용합니다.
OpenCode Go는 별도 제공자 항목이며 기본 Endpoint는 `https://opencode.ai/zen/go/v1`입니다.
출판사·평론가·전처리에 동일하게 설정할 수 있고, 대화별 세션 헤더를 전달합니다.
Go의 MiniMax·Qwen은 Messages, GPT는 Responses, Kimi·GLM·DeepSeek는 Chat Completions로 연결합니다.
Go 공식 안내는 코딩 에이전트 트래픽을 대상으로 합니다. Archive Center는 자체 클라이언트명으로 요청하며,
RP 용도에 대한 서비스 수용 여부와 실제 구독 계정 호출은 검증하지 않았습니다.

근거: [OpenCode Zen API](https://opencode.ai/docs/zen/#endpoints),
[OpenCode Go API](https://opencode.ai/docs/go/),
[OpenRouter API](https://openrouter.ai/docs/quickstart) (2026-09-09 확인).
새 제공자의 실제 유료 API 호출은 별도 검증이며, 자동 검사는 외부 HTTP 응답 fixture를 사용합니다.

검증 결과는 [4.3.0 배포 검증 기록](archive-center-4.3.0-release-verification.md)에 기록합니다.
