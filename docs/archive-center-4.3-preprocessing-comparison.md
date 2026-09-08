# Archive Center 4.3 전처리 비교

> 2026-09-08 갱신: 아래 비교 표는 **test.12 당시 소스 비교 기록**이다.
> 현재 기준은 [test.18](archive-center-4.3-test-build-18.md)이며, 이후 공통 JSON 복구,
> 담당별 해석의 본문 전달, public projection 인계, 기본 기억 점수·지식 연결·하이파
> 원문 가져오기와 HUD 보완은 [4.3 현황](archive-center-4.3-status-summary.md)을 따른다.
> 외부 플러그인을 재실행하거나 같은 모델로 품질을 재측정한 비교는 아니다.

2026-09-07, `4.3.0-test.12` 소스 기준. Archive Center의 현재 Go 전처리와 보관된 외부 예제 6개를 비교한다. **소스 경로 비교이며 실제 RisuAI·provider 실행시간이나 응답 품질을 측정한 결과가 아니다.** 이 문서 작성에서는 테스트·빌드·패키징·라이브 호출을 수행하지 않았다.

범위는 메인 RP 응답 생성 전까지다. 후처리·번역·출력 다듬기·저장 단계는 제외한다. GRADIA와 Serial의 메인 생성 전 초안 작성/수정은 전처리 호출에 포함하되, AC의 기억 선택과 수행하는 일이 다름을 구분한다. 예제의 설명문보다 실제 호출부를 근거로 삼았다.

## 호출과 의존 구조

호출 수는 필요한 모델/API 설정을 마친 **정상 새 턴** 기준이다. provider 재시도, 오류 보정, 임베딩 요청, 검색 내부 요청 및 메인 RisuAI 생성은 별도다. 사용자 저장 설정에 따라 기본값과 달라질 수 있다.

| 구현 | 기본 전처리와 순서 | 설정/추가 작업에 따른 차이 | 기억과 검색 범위 |
| --- | --- | --- | --- |
| **Archive Center test.12** | 기능 기본 OFF. ON에서 5개 역할이 모두 활성·호출 가능한 경우 **첫 분석 5회 + 필요한 역할의 보충 분석 0–5회 = 5–10회**. 각 분석 라운드는 병렬. 보충 검색도 병렬이며, 모든 결과를 역할 순서로 병합한 뒤 보충 분석을 시작한다. | 역할당 검색 질문 최대 1개, 전체 최대 5개. 검색 안의 임베딩/저장소 요청 수와는 다르다. 검색 뒤 후보 조립은 요청 내부에서 직렬화하며 서비스 전체 잠금은 쓰지 않는다. **기존 Publisher는 실행 조건이 충족되면 별도 1회**이며 5–10회에 포함하지 않는다. | MariaDB 정본 이력과 Chroma 검색을 기존 Go 소유 경로로 사용한다. 기존 세션/분기/인물 식별/관점 범위를 유지하며, 역할 간 전달도 기존 공개 근거 범위를 따른다. |
| **GRADIA v0.24.18** | **4회 직렬**: SHADOW 초안 1회 → 인물/세계/플롯 AIDE 부분 수정 각 1회. | lightweight 기본. lite는 2회. heavyweight는 정상 8회/lite 4회. 입력 도우미·확장 응답 엔진·오류 보정은 별도다. | 기본 `risu_selected`로 RisuAI가 선택한 로어를 사용하며, 키 검색/Jaccard 확장 설정도 있다. Host 로어·Hypa 기록을 읽는다. SHADOW에서 준비한 공유 문맥을 AIDE가 재사용한다. |
| **☸에로스 타워** | **pre 역할 4회 직렬**: world → character → momentum → synthesis. 다음 역할이 앞선 노트를 받는다. | 활성 pre 역할 수에 따라 변한다. 그 전에 source ingest/cold-start가 조건부 실행되며 기본 chunk 한도는 2다. 따라서 요청 전체 전처리가 항상 4회인 것은 아니다. | 플러그인 상태 저장소의 기억·원문·관계 후보를 역할별로 검색한다. 외부 임베딩 재정렬은 선택 사항이다. 이 경로는 AC의 MariaDB/Chroma 정본 서비스 구조와 다르다. |
| **risu_agents.js** | 기본 **4개 행·4회 직렬**: world → plot → character → dialogue. | 같은 행에 배치한 역할들은 병렬, 행 사이는 순차다. 역할 추가/비활성화와 모델 프리셋 설정이 가능하다. 기본 provider 재시도 2회는 정상 호출 수와 별도다. | 실제 채팅·설정·요약과 역할별 pluginStorage 기억 스냅샷을 사용한다. 확인한 전처리 경로에는 모델이 질문하여 추가 검색하는 루프가 없다. |
| **Serial_Gradation_Agents_for_RP** | 기본 **7회 직렬**: SHADOW 1회 + AIDE 3개 × 분석/초안 2회. 각 단계가 이전 전체 초안을 다시 작성한다. | 기본 lite는 3회. 모두 단일 호출이면 normal 4회/lite 2회. SHADOW도 분리하면 normal 8회가 된다. 사용자 분석 역할·확장 엔진·오류 복구 호출은 별도다. | Host 인물/페르소나/모듈 로어/채팅 기억 snapshot과 활성 키·재귀 로어 선택을 사용한다. 외부 정본 기억 검색 서비스는 아니다. |
| **risu-multiagent.js** | **3회 직렬**: world → plot → character. 후속 역할이 앞 결과에 의존한다. | 역할 구조는 고정이고 공통 provider/model/문맥 크기를 설정한다. | 전달된 system/history/user 입력을 사용한다. 확인한 전처리에 독립 기억 저장소나 추가 검색은 없다. |
| **CocoA(gent) v1.1.5.4** | 기본 single 모드 **4개 행·4회 직렬**: world → plot → character → dialogue. | 같은 행은 병렬. multi 선택 시 역할별 1–12 step, 기본 3이며 도구 호출 한도는 기본 6/최대 40이다. 마지막 도구 결과 후 결론 호출 +1, 선택적 형식 수정 +1, provider 재시도는 별도이므로 “최대 3회 고정”이 아니다. | 기본 제공 문맥/역할 기억 외에 multi 도구로 로어·채팅·역할 기억·Hypa/HypaPlus·WygLore를 추가 조회한다. 선택적 semantic/hybrid 임베딩과 웹 검색도 지원한다. |

## 역할 설정과 실행 관측

| 구현 | 역할별 프롬프트/모델 | 소스에서 확인한 시간·진행·오류 기록 |
| --- | --- | --- |
| Archive Center | 5개 전문 역할의 지침과 모델 설정. Go가 문맥을 조립하고 받은 기억 선택/순서를 기존 전달 경로에 적용한다. Publisher가 전달된 기억을 창작 가이드에 사용한다. | 역할별 라운드 호출 시간/오류, 보충 검색 전체 시간과 질문별 상태/시간 및 검색 내부 구간. 기존 요청 HUD로 전달한다. |
| GRADIA | 단계별 프롬프트/모델 프리셋과 AIDE 순서 설정. | 공유 문맥 준비·단계 문맥 조립·모델·전체 시간, 진행 상태, 원응답과 실패 trace. |
| 에로스 타워 | 역할별 prompt/template/provider/model. | 역할별 `ms`, 검색 미리보기, 프롬프트, 원응답, 오류와 전처리 완료 토스트. |
| risu_agents | 역할별 모델 프리셋/프롬프트/문맥 포함 설정. | 역할별 소요 시간, 성공/실패/재시도, 실행 검사기. |
| Serial_Gradation | 단계별 프리셋/프롬프트/문맥/단일·분리 실행 설정. | stage trace의 시간, 프롬프트, 원응답, 실패와 보존된 이전 초안. |
| risu-multiagent | 역할별 고정 prompt 함수, 모델 설정은 공통. | 역할별 시간/오류, 전체 시간, 주입 여부와 마지막 실행 기록. |
| CocoA(gent) | 역할별 모델/프롬프트/도구/step 설정. | 행 벽시계, 모델 호출 수/토큰/시간, 도구 시간, 단계 진행, 실시간 검사기와 실패 기록. |

## 비교에서 확인되는 차이

- **AC의 강점은 정본 이력에서 현재 장면의 근거를 준비하는 연결에 있다.** 역할이 필요한 근거를 추천하고 기존 Go 검색·식별·관점 범위 안에서 보충하며, 받은 선택과 순서를 기존 전달 경로 및 Publisher로 연결한다. 기억은 이전 상태와 출처를 설명하고, 사용자가 정한 이야기 방향·수정·속도·결정이 창작의 권한을 가진다. 보충 분석이나 Publisher 제안이 정본 기록을 새로 쓰는 것은 아니다.
- **Coco의 강점은 역할별 탐색 도구와 여러 단계 작업의 유연성이다.** 같은 행의 병렬 실행과 도구 결과를 다시 읽는 경로가 있으며, 실제 호출량은 역할 수뿐 아니라 도구 사용·step·결론 호출에 따라 달라진다. 이는 AC의 한 번 보충 검색/분석 구조와 서로 다른 작업량이다. GRADIA/Serial의 초안 작성·수정량도 별도로 봐야 한다.
- **호출 수만으로 속도나 품질 순위를 정할 수 없다.** 독립 작업의 병렬화는 직렬 의존 경로와 다르지만 provider의 동시 처리, 입력/출력량, 검색량과 오류가 벽시계 시간을 바꾼다. 같은 장면·기억·모델·설정의 라이브 비교가 없으므로 어느 쪽이 더 빠르거나 더 좋은 RP를 만든다는 결론은 내리지 않는다. 현재 비교는 추가 기능이나 자동 재작성/재시도 도입을 약속하는 문서가 아니다.

## 소스 근거

라인은 이 문서 작성 시점 기준이며 경로와 함수명이 우선이다. 외부 예제는 실행 중인 플러그인으로 확인한 것이 아니라 보관 파일을 읽은 것이다.

- AC: [prepare_turn_multi_agent.go](../go-service/internal/httpapi/prepare_turn_multi_agent.go), `multiAgentRoles`/`multiAgentSharedPrompt` 22–38행, `runMultiAgent` 739행부터. 1차 병렬 호출 763–777행, 검색 질문 수 제한과 작업 구성 790행부터, 보충 검색 병렬/역할 순서 병합 826–889행, 보충 라운드 911–961행. [group_turn_prepare.go](../go-service/internal/httpapi/group_turn_prepare.go)의 `handlePrepareTurn`에서 기존 scope를 전달한 검색 및 요청 내부 `searchAssemblyMu` 1029행부터, Publisher 조건과 단일 호출 1426–1471행. 저장소/관점 근거는 같은 활성 경로의 `historyScope`, `prepareTurnVectorShadowWithPreciseCandidateLimits`, `assemblyPerspectiveContext`와 [prepare_turn_priority_memory.go](../go-service/internal/httpapi/prepare_turn_priority_memory.go)를 따른다.
- [GRADIA.js](../../_archive/reference/external-examples/example/GRADIA.js): 본체 전체가 **88행에 압축**되어 있다. `multi_pipeline_mode` 기본 `lightweight`; SHADOW `Bc`/`Hc`, AIDE `Xc → Yc`, `Zc`의 `integrated_serial_patch`/`analysis_then_serial_patch`, 순차 `await Xc`, `oa.pipelineTimings`, 공유 문맥 `_o → vo`를 확인했다. UI의 전체 초안 설명과 달리 최신 AIDE 경로는 JSON edit patch이므로 Serial과 구분한다.
- [☸에로스 타워.js](../../_archive/reference/external-examples/example/%E2%98%B8%EC%97%90%EB%A1%9C%EC%8A%A4%20%ED%83%80%EC%9B%8C.js): `defaultPipeline` 1844–1853행, `resolveAgentConf` 1901행, `loadState` 2428행, `runPsycheSourceIngest` 6495행, `runAutoColdStart` 7065행, `buildAgentContextPackMaybeEmbedded` 7442행, `runPrePipeline` 9061–9147행. `beforeRequest`의 ingest → cold-start → pre 순서는 13061–13065행이다.
- [risu_agents.js](../../_archive/reference/external-examples/example/risu_agents.js): `DEFAULT_AGENT_PRESETS` 2962–3045행, `resolveAgentConfig` 3399행, 기억 snapshot 4055행, `runPrePipeline`의 행 순차/역할 병렬 4827–4831행, 호출 4894행, 정렬·노트 전달 4979–4989행. 기본 provider retry는 86행이다.
- [Serial_Gradation_Agents_for_RP.js](../../_archive/reference/external-examples/example/Serial_Gradation_Agents_for_RP.js): `defaultExecutionModeForStage` 262행, 기본 단계 설정 1436행, 실제 `scopedSettingsForStage` 적용 1520행, `activeRisuLorebooks` 2183행, Host snapshot 2307행, `runTwoCallStage` 4220행, 직렬 실행 4790–4812행. 전역 `twoCallAide=true`만으로 세지 않고 SHADOW의 `draft_only` 적용을 포함했다.
- [risu-multiagent.js](../../_archive/reference/external-examples/example/risu-multiagent.js): 공통 `getConfig` 40행, `runAgentWithDiagnostics` 1601행, 입력 구성과 세 번의 의존 호출 1686–1709행, 실패 시 원입력 반환 1718–1725행.
- [cocoAgent v1.1.5.4.js](../../_archive/reference/external-examples/example/CocoA%28gent%29/v1.1.5.4/cocoAgent%20v1.1.5.4/cocoAgent%20v1.1.5.4.js): `DEFAULT_AGENT_PRESETS` 4369–4448행, single/multi 적용 6628–6646행, 모델 설정 6752행, `memory_search` 8615행, `hypa_search` 9054행, multi 도구 활성화 9296행, 도구 병렬/쓰기 순차 9755–9764행. `runAgentSteps`의 결론 추가 호출 10719–10776행, 선택적 형식 수정 10818–10846행, 모델/도구 분리 통계 10853–10862행. `runPrePipeline` 행/역할 실행 15678–15691행, 역할 호출 15785행, 정렬과 다음 행 전달 15895–15904행.
