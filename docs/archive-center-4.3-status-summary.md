# Archive Center 4.3 작업 현황 요약

Current source: **4.3.0 stable**, published on GitHub. See [release record](archive-center-4.3.0-release-verification.md). Prior test-build entries below are historical evidence.

기준일: **2026-09-09**. 활성 소스: `source/Archive Center.js`와 `source/go-service`.
현재 소스 식별자는 **`4.3.0`**이며 정식 패키지·CI·이전 버전 업데이트 검사를 통과했다. main 반영과 정식 릴리스 공개를 완료했다. [검증 기록](archive-center-4.3.0-release-verification.md). 실제 RisuAI·제공자 검증은 별도로 남아 있다.
아래의 완료는 해당 소스 반영과 기록된 검증 범위의 완료를 뜻한다. 실제 RisuAI에
로드한 동일 플러그인·백엔드, 실제 제공자, 최종 표시 출력까지의 완료와 구분한다.
최초 현황 정리는 문서 작업이었다. 이후 소스 수정과 검증은 아래 후속 갱신에서 구분한다.

## 현재 기준: 4.3.0 정식 (test.23 동작 포함)

2026-09-09 [test.23](archive-center-4.3-test-build-23.md): 부모의 원본 저장 대기와
분기 위치 확인을 분리해 기존 부모·조상 기억의 상속 범위가 끊기는 경로를 수정했다.
기본 RisuAI의 보존 ID와 PocketRisu의 재발급 ID를 함께 검사했다. 콜드스타트의 두 번째
병합에서도 상속 제외와 번역 표시문 정리가 유지된다. 기존 저장 확정 시점은 유지하며,
미저장 응답에 대한 기억 생성은 원래 확정 절차를 따른다. 사용자 DB·챗·설정은 변경하지
않았고, 새 백엔드는 사용자가 실행한다. test.22의 전처리·기억 선정 보완을 포함한다.

## 선행 기준: test.22

2026-09-09 [test.22](archive-center-4.3-test-build-22.md): 로컬 기준 커밋 `6ef8f74` 이후
검색 질문 객체 형식, 담당별 추천 순서, 편집자 기본 프롬프트와 출처 표기를 보완했다.
기존 원문·선택 항목·보유자/공개 범위를 유지하며 노트의 반복 부분만 줄였다. 기존 Go
선정 폭과 중요도·예산 정책은 유지한다. 외부 AI 호출 없이 회귀와 20개 저장 응답을
검사했으며 다음 실제 턴에서 새 기본 프롬프트의 효과를 확인한다. 이전 test.21 패키지를
보관했고, 백엔드는 사용자가 실행한다. 사용자 프롬프트를 자동으로 덮어쓰지 않는다.

## 2026-09-08 계획 인계 — 기본 회상 우선

[통합 로드맵의 기본 회상 계획](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#good-memory-plan)에
사용자가 승인한 목표·버전별 결과·공통 비교 자료를 반영했다. 중요한 기억의 유지, 사소한 기억의
단서 기반 재회수, 시점·출처 구별을 전처리·출판사 OFF에서도 검증한다.
4.4~4.9는 중복·맥락 묶음·시점·검색·연관 회수를, 5.1-A~5.2는 공통 접근성·장면 단서를 맡는다.
5.1-B·5.3~5.7의 인물별 망각·기시감·부분 회상은 선택형 표현으로 분리한다.
여섯 공통 사례와 전처리/출판사의 네 조합, 추천 없음·부분 실패를 버전별로 이어서 비교한다.
이번 인계는 문서 변경이며 구현·빌드·실제 품질 검증을 추가하지 않았다. test.21의 상태는 그대로다.

## 선행 기준: test.21

2026-09-08 [test.21](archive-center-4.3-test-build-21.md): 기본 기억 전달 폭과 주관 기억의
예산 경합, 현재 값 판정, 전처리 추가 검색의 출처 충돌을 한 묶음으로 수정했다.
분류별 개수는 핵심 우선 수로 사용하고, 남은 문자 예산에는 관련 세부사항을 담는다.
일반 주관 기억은 보유자·출처 턴과 함께 정상 선정에 참여한다. 현재 필드의 최신 출처
판정과 검색 순위를 분리하고, 보완 검색의 새 출처/값에는 별도 후보 참조를 부여한다.
AI 추천의 원문·순서와 추천 없음/부분 실패의 Go 기본 선정은 유지한다.
전체 Go 35개 패키지, 고정 자료의 OFF·AI·추천 없음·일부 실패, 새 패키지 검증을 통과했다.
실제 RisuAI의 출력 품질은 사용자 테스트 단계이며 백엔드는 자동 실행하지 않았다.

2026-09-08 [test.20](archive-center-4.3-test-build-20.md): 모델 입력에서 현재 입력·최근 대화를
먼저 배치하고, 반복 출처만 묶어 원문·순서·공개 범위를 보존했다. 선택 사실·완성 요약과
담당 설명을 같은 F/S 참조로 연결하고, 출판사에 참조·출처 목록을 전달한다. 기존 2차의
이전 결과도 같은 참조를 사용하며 공통 기본 지침에 원문 대조를 보완했다.
추가 호출·추천 재선정·강제 K 절단·설정/DB 변경은 없다. 전체 Go 35개 패키지와 제공된
10개 입력 보존 검사, 새 백엔드·53개 관리 파일·ZIP 검증을 통과했다. 실제 모델 전후 비교는
외부 전송의 명시적 승인을 요구한 자동 승인 심사로 대기 중이며, RisuAI 출력 품질은 미검증이다.

2026-09-08 [test.19](archive-center-4.3-test-build-19.md): 다섯 전처리 담당을 기억 편집자로
정리하고, 공통·개별 프롬프트에 현재 장면과 기록된 변화의 연결을 보완했다. 공개 근거를
다른 담당에게 보낼 때 빠졌던 확인 목적을 `request_reason`으로 원문과 분리해 전달한다.
출판사 OFF·성공·실패 회귀, 전체 Go 및 패키지 검사를 통과했다. 기존 호출 횟수·선정·
비밀 범위·사용자 결정권을 유지한다. 여섯 기존 기본 프롬프트는 UI에서 백엔드 기본값
사용으로 저장했으며, 새 백엔드 적용과 실제 RP 품질 검증은 사용자 실행 이후 단계다.
세부 내용은 [전처리 작업 기록](archive-center-4.3-preprocessing-work-log.md)을 따른다.

2026-09-08 패키지 갱신: [test.18](archive-center-4.3-test-build-18.md)에 아래 HUD·검색
점수·기억 연속성·하이파 원문 가져오기 수정을 포함했다. 플러그인과 Go 백엔드를 함께
갱신했으며 관리 파일 53개와 ZIP 해시를 확인했다. 설치와 실행은 사용자가 직접 한다.

2026-09-08 HUD 표시 후속 수정: 활성 JS의 HUD 폭을 224px로 줄이고 준비 세부를
두 열의 작은 시간 박스로 정리했다. 생성·저장 항목도 같은 두 열 박스로 통일했다.
정상 완료 카드는 본체나 X로 닫고, 상세 펼치기/접기는 창을 유지한다. 경고·오류는 X 전용이다.
기존 시간/상태 계산과 저장 시점별 목록 표시 조건은 유지한다.
[UI 작업 기록](archive-center-4.3-preprocessing-work-log.md)에 격리 Edge 화면 검증을 남겼다.
test.18 패키지에 포함했다. 로드된 RisuAI 검증은 별도다.

2026-09-08 검색·선정 후속 수정: `priority_score.static.v4`에서 원본 Memory의 의미 검색
점수를 complete summary 선정에 이어주고, 저장 중요도에 최근성을 곱하던 이중 감점을 제거했다.
개별 사실의 관련성, 자료별 K·예산과 AI 추천 순서는 유지한다. 전처리·출판사가 없는 기본
조립에서도 적용된다. [작업·검증 기록](archive-center-4.3-feedback-work-log.md#retrieval-score-retention-repair):
이전 실패 비교 사례와 전체 Go 검사(35개 패키지)가 통과했다. 실제 서비스가 필요한 검사 등
12개 생략과 source/실환경 차이는 해당 기록을 따른다. **test.18 패키지에 포함했다.**
상태 시점·약속 상태·표현이 다른 기록의 관계 연결 및 실환경 출력 효과는 후속 검증 범위다.

2026-09-08 기억 연속성 1차 수정: 별개 비밀·주관 기억의 충돌, 알고 있음/공개받음의
잘못된 충돌, 정체를 모르는 POV의 지식 판정, 정체 매핑의 인물별 지식 생성 누락,
공개 이후 남는 과거 보호 안내를 기존 Go 경로에서 수정하고 재검사했다.
기본 장기 기억 경로에 적용되며 전처리·출판사 활성화가 필요하지 않다.
[수정 및 재검사 기록](archive-center-4.3-feedback-work-log.md#knowledge-continuity-first-repair)을
따른다. 이 변경은 test.18 패키지에 포함했으며 실제 백엔드 기동은 수행하지 않았다.
기존 DB의 누락된 관측 기록을 자동 복구하지 않는다.

2026-09-08 하이파 가져오기: 현재 챗에서 받은 비어 있지 않은 요약을 하나당 기억 하나로
보존한다. 평론가 요약은 보조 정보이며, 반복 가져오기와 음수 가져오기 턴의 이력 조회를
보완했다. 챗 분할로 이미 사라진 원문이나 과거 축약 결과를 복원하지는 않는다.
[하이파 기록](archive-center-4.3-feedback-work-log.md#hypa-original-import)을 따른다.

현재 설치·빌드 안내는 [README](../README.md)와 [test.20 기록](archive-center-4.3-test-build-20.md),
파일별 소유자는 [STRUCTURE](../STRUCTURE.md), 다음 작업은
[4.4 계획](archive-center-4.4-refactoring-plan.md)을 따른다.

## 이전 체크포인트와 결과 검토

후속 갱신: test.12 결과 검토 후 **기존 정보의 전달 정확성 보완을 활성 소스에 반영**했다.
누적 인물 상태의 갱신 턴과 개별 사실 시점을 구분해 설명하고, 최근 대화 설정의 양 차수
전달 및 출판사까지의 원문·수량 전달을 회귀로 확인했다. 저장된 사용자 프롬프트와
이야기 결정권은 보존한다. 세부 내용은 [전처리 작업 기록](archive-center-4.3-preprocessing-work-log.md)을
따른다. [test.13 패키지](archive-center-4.3-test-build-13.md)에 이 수정이 포함되었으며
53개 관리 파일의 해시를 확인했다. 실제 RisuAI와 표시 출력 검증은 별도다.

test.14 후속 갱신: 전처리·출판사·평론가가 공통 JSON 형식 복구를 사용하고, 전처리는
잘못된 필드 뒤의 정상 선택·검색 질문을 계속 해석한다. HUD에는 보정/부분 해석/추천 없음과
최종 AI/Go 선택 출처를 구분해 표시한다. 로어북 참조 옆 이름과 최근 대화에 명시된 미해결
목표의 후보 누락도 보완했다. 모델 크기나 제공자에 제한을 두지 않으며 추가 AI 호출은 없다.
백엔드·UI 회귀와 새 [test.14 패키지](archive-center-4.3-test-build-14.md)의 관리 파일 53개를
확인했다. 저장된 편집 프롬프트를 보존하므로 최신 기본 지침은 기본값 복원 시 사용할 수 있다.

test.15 후속 갱신: 기존 담당별 선택 이유와 미해결 질문을 기억원문과 구분해 출판사와
본문 AI 양쪽에 전달한다. 원문 참조와 인물별 공개 범위, 실제 채택 차수를 유지한다.
출판사를 끄거나 호출에 실패해도 본문용 해석은 유지되며 추가 AI 호출은 없다.
편집 확인의 **전처리 담당별 해석**에서 별도 내용과 추가 입력량을 확인할 수 있다.
[test.15 패키지와 검증](archive-center-4.3-test-build-15.md) 이후 아래 사용자 결과를 검토했다.
추가 개선을 4.3에 계속 확대하지 않는다. 서사 품질 향상과 완벽한 비밀 유지가 입증된 것은 아니다.

### test.17 — HUD 간소화

이전 턴 확정 모드의 현재 생성 카드에서 저장 건수표를 제거하고, 이전 턴 저장 카드에는
실제 건수를 접어서 표시한다. 전체·준비·응답 시간과 다섯 담당의 1차·2차·선정 표를
기본 화면에 남기고 검색·준비·단계 세부 항목을 접었다. 응답 수신 뒤 대기 중 표시도
현재 카드에서 정리했다. HUD 설명문 세 개와 전처리 Flex 설명문은 제거했다.

이는 Host UI 변경이며 Go의 실제 저장·검색·AI 호출·원래 상태는 유지한다.
JS 회귀와 로컬 데스크톱/모바일 화면 검증, 패키지 정보는
[test.17 기록](archive-center-4.3-test-build-17.md)을 따른다. 실제 RisuAI에서의 확인은 별도다.

### test.16 — 기존 공개 기억 전달 누락의 수정

`runMultiAgent()`가 실제 기억 후보의 `public_projection` 표시를 공개 전달에서
처리하도록 수정했다. test.15 이전부터 남아 있던 누락이며 기존 `public`·`general`
검증만으로는 발견하지 못했다. 실제 `/prepare-turn` 조립에서 생성된 후보가 다음 담당의
2차 HTTP 요청에 원문·참조·출처·턴 그대로 도착하는 회귀를 추가했다. 수정 전 세 Publisher
모드에서 실패, 수정 후 통과를 확인했다. 기존 개인 범위와 AI 선정·최종 주입은 유지한다.

세부 시험과 최신 패키지는 [test.16 기록](archive-center-4.3-test-build-16.md)을 따른다.
사용자 요청으로 기존 test.15 백엔드를 종료했으며 시작과 RisuAI 설치는 사용자가 진행한다.
실제 외부 제공자·Host·표시 출력 검증은 남아 있다. 아래 상태 시점 문제 및 이전의
최종 입력 관측 불일치·과다 선택·추가 입력 분량 문제를 이번 수정의 해결 범위로 세지 않는다.

### test.15 사용자 관측과 후속 상태 시점 작업 — 2026-09-07

사용자 제공 자료에서는 다섯 담당의 양 차수 호출과 최근 대화 5턴 전달, 선택 이유 65개와
미해결 질문 19개의 조립본 포함을 확인했다. 편집 확인의 `effective_user_text_not_observed`는
현재 코드상 보조 기억 반영 관측과 사용자 입력 일치 관측을 구분하는 결과이며, 최종 제공자
요청 전체가 검증됐다는 뜻은 아니다. 해당 자료 검토를 새 실환경 시험 완료로 확대하지 않는다.

**미해결:** `character_states:2709`의 현금 51냥 5푼과 자산 설명 속 61냥이 모두
`source_turn:115`로 공급됐다. 직전 대화의 잔액은 46냥 5푼이다. 현재 상태 병합은 기존
항목을 유지하고 새 기록 턴을 붙이며, 후보 구성은 그 턴을 개별 사실에 전달한다. 따라서
최근성 점수만 높여서는 이 항목들의 실제 시점 차이를 구분할 수 없다. 인물 상태 담당이
38개를 선택한 결과와 별개로, 백엔드의 저장·시점 부여·검색 후보 구성을 원인 범위로 추적한다.
과거 항목이 처음 생성된 정확한 시점과 기존 자료 전체의 복원 가능성은 아직 확인하지 않았다.

test.13의 시점 설명과 test.15의 해석 전달은 구현된 보완이며 위 문제의 해결은 아니다.
[통합 로드맵의 후속 계획](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#temporal-state-44647)에
**4.4 중복 정리·사례 준비 → 4.6 개별 항목 시점·현재성 수정 → 4.7 검색·선정 반영**으로
배정했다. 새 자료의 저장 개선과 누적 자료의 근거 기반 복원은 별도로 검증한다.
이 갱신은 문서 작업이며 runtime·DB·프롬프트·빌드를 바꾸거나 문제를 해결 완료로 표시하지 않는다.

## 기본 동작과 호환성에 반영한 작업

| 항목 | 현재 소스의 처리 | 근거와 경계 |
| --- | --- | --- |
| GitHub #5 대량 세션 삭제 | 기존 중복 벡터 삭제 작업 정리가 마지막 커밋 ID 뒤부터 진행하고 배치마다 쓰기 잠금을 해제한다. 기존 인덱스·점유·source 조건과 관리자 정리 경로를 유지한다. | [피드백 기록](archive-center-4.3-feedback-work-log.md). 격리 DB의 대량 삭제·중복 정리·재시도·무관 자료 보존을 검증했으며 새 자동 삭제 기능은 없다. |
| GitHub #6 긴 Critic 값 | `memory_subtype`, `relationship_key`, `reveal_condition`을 `LONGTEXT`로 통일했다. 신규 SQL, migration 013, Go 호환성 경로가 같은 값을 보존한다. | [사전 시험](archive-center-4.3-pretest-20260906.md). Error 1406 재현 뒤 기존 JSON/hash를 보존하고 같은 결과를 재처리한 격리 MariaDB 증거다. |
| GitHub #4 저장 결과 해시 | 4.2에 이미 수정된 `storedMemoryAdmissionExtraction()`의 저장 JSON·당시 버전 기반 해시와 결과 재사용을 재확인했다. | 새로 고친 결함으로 세지 않는다. Critic 재호출 없는 재투영·해시 변조 회귀와 #6 DB 재처리가 근거다. 제보자의 NSSM 로그 환경은 미확인이다. |
| GitHub #10 HUD 리스너 | 이벤트를 등록한 SafeElement와 종류·ID·옵션으로 제거하고 제거 실패의 추적 정보를 유지한다. | 실제 JS 함수/Host 대역에서 100회 교체 후 1개, 정리 후 0개. 실제 브라우저 프리징 전체 원인은 미확정이다. |
| GitHub #7 HUD 복구 | 409의 최신 Go HUD, 필요 시 기존 상태 조회 1회, 404의 오래된 카드 정리, 확정 턴에 맞춘 표시 정정을 연결했다. | 실제 Go/JS 회귀 범위다. HUD 생성 순서를 완료 턴 저장·reroll/edit의 식별자로 사용하지 않는다. |
| RisuAI·PocketRisu 분기/재분기 | 보존 ID와 재발급 ID를 함께 지원한다. Go가 지정한 부모의 ID·역할·순서·분기 마커를 Host가 `risu_message_origins.v1`로 관측하고 기존 계보 메타데이터에 저장한다. | [분기 기록](archive-center-4.3-feedback-work-log.md#추가-피드백--risuaipocketrisu-분기재분기-연결). 직접 부모, 조상 원문, 더 이른 재분기 경계를 분리한다. 실제 두 Host 로드 검증은 남아 있다. |
| Gemini 3.8 Flash medium | 기존 UI 선택지와 Go 요청 생성에서 `none/low/medium/high` 및 medium 전달을 연결했다. | [피드백 기록](archive-center-4.3-feedback-work-log.md#추가-피드백--gemini-38-flash-medium-선택). 출판사·평론가와 기존 중계 전송의 회귀이며 실제 계정 수락 증거는 아니다. |

분기 출처 관측은 본문 전체 전송·유사도 검색·LLM 호출을 하지 않는다. 기존
`session_fork_lineage.inherited_items_json`을 쓰며 새 출처 테이블은 없다.
**상속 기억 전체의 독립 복사·수정 및 개체/주관 기억 상속 UI는 구현한 기능이 아니다.**
일반 채팅 복사, 공식 분기, 상속 기억을 독립 편집하는 복사는 서로 다른 범위다.

## 선택형 전처리 다중 에이전트

[구현 기록](archive-center-4.3-preprocessing-work-log.md)의 다섯 역할이 실제
`handlePrepareTurn()`에 연결되어 있다. 추가 기능의 독립 항목이며 **기본 OFF**다.
화면 열기·설정 저장·다른 추가 기능 활성화로 켜지지 않으며 OFF에서는 전용 분석·검색을 하지 않는다.

| 역할 | 준비하는 근거 |
| --- | --- |
| `event_recent` | 사건·진행 이력, 인과·시점, 사실과 별도 턴 요약 |
| `character_objective` | 인물의 객관적 상태, 지속/임시 상태와 변화 |
| `subjective_relationship` | 개인 경험·관계·비밀, 앎·믿음·모름과 공개 범위 |
| `world_state` | 세계·장소·물건의 상태와 규칙, 선택적 AC 로어북 참조 |
| `unresolved_goal` | 미해결 목표·복선·약속의 진행/완료/취소와 근거 |

기존 Go 검색·관점별 후보 조립 → 활성 N개 역할의 1차 병렬 분석 → 요청된 보충 검색 →
필요한 M개 역할의 2차 병렬 분석 → 정본 원문 선정·렌더 → 기존 Publisher·최종 payload 계획 순서다.
정상 분석은 **N+M, 0≤M≤N≤5**이며 역할당 보충 검색 질문은 최대 1개다.
검색 내부 임베딩/저장소 호출과 실제 제공자 재진입은 별도 집계한다.
기존 Publisher는 해당 실행 조건에서 별도 1회이며 전처리 분석 횟수에 포함하지 않는다.

Go는 전달받은 정본 추천의 원문·순서를 자기 점수/K/분량 기준으로 다시 바꾸지 않는다.
기존 범위와 예산을 호출 전에 알리고, 받은 추천의 초과는 실제 사용량으로 관측한다.
추천이 없는 담당에는 해당 요청의 기존 Go 선정을 사용한다. 보충 호출 실패는 먼저 받은
추천을 유지하고, 정상 보충 응답이 빈 추천이면 Go 선정을 사용한다. 부분 JSON에서 받은
ID와 다른 담당의 결과를 보존하며 미해결 참조·파싱 실패·검색 실패를 성공으로 표시하지 않는다.
이 규칙은 기존 세션/분기/관점·공개 범위를 넓히거나 존재하지 않는 정본을 만드는 기능이 아니다.

각 역할은 개별 제공자·Endpoint·모델·키, 프롬프트, 온도·출력 토큰·시간·추론 설정을 가진다.
개별 연결이 기본이며 출판사 연결 공유는 선택 사항이다. 공유 중에도 역할 프롬프트와
역할별 온도·출력 토큰을 사용한다. 공통/역할 프롬프트 편집·기본값 복원과 실제 적용 내용 조회를 지원한다.
Flex는 기존 Go 전송을 사용한다: OpenAI 호환 `service_tier`, AI Studio `serviceTier`, Vertex 전용 헤더.
제공자를 바꿔 숨겨진 설정은 보존한다. 모델·계정별 실제 Flex 사용 가능 여부는 별도 확인 대상이다.

`GET/PUT /config/memory-preprocessing`가 `ARCHIVE_CENTER_DATA_DIR/memory-preprocessing.json`에
설정과 사용자 프롬프트를 저장한다. 번들 기본본과 사용자 편집본을 분리하며 재로드 회귀가 있다.
저장 키를 비밀번호 입력칸에 다시 채우고 명시적 빈 문자열은 삭제, 필드 생략은 보존한다.
응답은 `Cache-Control: no-store`이며 키를 모델 입력/HUD 시간 자료에 포함하지 않는다.
실제 설치 업데이트 전체에서 설정 보존까지 확인했다는 뜻은 아니다.

## test.10–12에서 보완한 입력·가이드·관측

- **근거 공급:** 사실·요약이 기존 후보 문자 한도를 공유하고 요청 내부 F/S/L 번호를
  정확한 원래 ID에 연결한다. 공개 `general` 근거의 역할 간 참조를 맞추고 사적 범위는 유지한다.
  선택 사실의 출처 턴을 표시하여 과거의 상대 시점 표현을 현재 사실로 오인하지 않도록 근거를 남긴다.
- **로어북:** 세계 담당의 `selected_lorebook_refs`는 기억 추천과 독립이다. 명시적 `[]`는
  이번 AC 추가 로어 주입 없음, 생략/실패는 기존 Go 또는 앞선 성공 선택 유지다.
  받은 원문·순서와 동일 내용 병합의 출처를 보존하며 Host 자체 로어 주입은 Host가 소유한다.
- **검색·전송:** Chroma 정상 JSON 응답의 1 MiB 잘림을 수정하고 HTTP 오류 응답 제한은 유지했다.
  Vertex에 남아 있는 다른 제공자용 유효 서비스 티어가 요청을 막지 않도록 기존 전송 소유자를 수정했다.
  Vertex 자체 Flex 헤더를 쓰며 저장값 변경·티어 치환·추가 재시도는 하지 않는다.
- **사용자 자유와 강도:** 기본 Publisher는 `current_arc`, `narrative_goal`, `next_beats`,
  `pressure_level`을 요청한다. 약하게/중간은 참고·권고, 강하게 이상은 사용자가 선택한
  방향의 구체적 실행·반응·결과를 요청한다. 방향·수정·진행 속도·사용자 결정권이 모든 강도에서 우선한다.
  강도와 pressure는 독립이며 프롬프트/렌더 정책을 출력 수락·저장 조건으로 사용하지 않는다.
- **시간/HUD:** 담당별 1차·보충 호출 및 오류 시간을 기존 HUD에 표시한다. Host가 관측한
  준비 시작→응답/본문 응답 대기는 응답 수신에 고정하고 이후 Critic·저장·다음 입력 대기를 제외한다.
  Effective Input은 백엔드 미리보기와 요청 직전 관측 및 불일치 사유를 구분한다.
- **test.12 검색 병렬화:** 독립 보충 검색을 동시에 수행하고 완료 순서와 무관하게 역할 순서로
  병합한 뒤 2차에 공급한다. 공유 입력의 원문 연결·조립만 요청 내부 mutex로 직렬화한다.
  최초 조립 때 보관한 렌더 변경 전 후보 사본을 재사용하며 중첩 배열과 사적 원문을 보존한다.
  사본은 요청 내부 비공개 필드로 영구 캐시가 아니다. 검색 전체 경과 시간과 검색별 7개 세부
  시간을 분리하며 겹치는 검색 시간이나 상위 `injection_assembly` 시간을 중복 합산하지 않는다.

이 범위는 **메인 응답 전의 기억 준비와 기존 Publisher 가이드 보완**이다.
초안 작성·재작성·출력 다듬기·후처리 AI 및 새로운 기억 쓰기 AI를 추가하지 않았다.

## 기록과 실제 검증의 경계

| 근거 | 확인된 범위 | 이 근거만으로 확인되지 않는 범위 |
| --- | --- | --- |
| 활성 소스·생산 함수 회귀 | 기능 연결, 추천/원문/순서, OFF·부분 실패, 설정, Host/제공자 대역과 최종 Go payload 계획 | 설치된 Host 적용, 실제 유료 제공자 수락·비용·글쓰기 품질 |
| [9월 6일 사전 시험](archive-center-4.3-pretest-20260906.md) | 당시 35개 Go 패키지, 집중 13개, 격리 실제 MariaDB/Chroma 시험 5개 | 이후 test.18 전체를 실제 DB/제공자로 다시 검증한 결과 |
| 선행 삭제 대규모 시험 | 12,000 Chroma 문서 및 과거 중복 112,000/1,120,000행 시험의 명시된 결과 | 사용자 DB 전체, 모든 OS, 40M 장기 이력 성능 |
| [test.1](archive-center-4.3-test-build-1.md)·[test.2](archive-center-4.3-test-build-2.md)·[test.3](archive-center-4.3-test-build-3.md) | 당시 Windows 패키지/백엔드 기본 동작과 저장소 준비, 승인된 Tailscale 접속 복구 기록 | 현재 test.22 실행 상태나 동일 코드의 RisuAI 재등록 |
| 사용자 기본 동작 관측 | 전처리 구현 전 테스트 빌드에서 사용자가 기본 기능에 문제가 없어 보인다고 확인하고 전처리 진행을 요청했다. | 정확한 로드 버전·Host별 검증표, 이후 전처리와 test.18의 실제 제공자/출력 검증 |
| test.4–12 패키지·UI 대역 기록 | 각 기록의 생성물·해시·로컬 화면/회귀. test.12 관리 파일 53개 디스크/ZIP 해시 일치 기록 | 공개 4.3 출시, 실제 설치·업데이트 완료, 최종 표시 출력 |
| [test.18](archive-center-4.3-test-build-18.md) | 당시 소스·플러그인·프롬프트·SQL 동일성, 관리 파일 53개와 ZIP 해시, Go 35개 패키지 및 Host 422개 회귀 | 실행 백엔드 교체·RisuAI 설치·실제 제공자 수락·최종 출력 효과 |
| [test.19](archive-center-4.3-test-build-19.md) | 편집자 프롬프트 검토, 수신 담당의 요청 이유·원문 분리 회귀, 전체 Go 35개 패키지, UI 기본값 사용 저장, 새 백엔드/53개 관리 파일/ZIP | test.19 실제 기동·모델 해석의 정확성·기억 및 최종 RP 품질 향상 |
| [test.20](archive-center-4.3-test-build-20.md) | 입력 순서·반복 출처·사실/요약별 참조·Publisher 출처 연결 회귀, 제공된 10개 입력 보존, Go 35개 패키지, 새 백엔드/53개 관리 파일/ZIP | 외부 전송 승인 대기 중인 실제 모델 전후 비교, 동일 빌드의 RisuAI 최종 출력 품질 |
| [test.21](archive-center-4.3-test-build-21.md) | 넓은 Go 전달·주관 기억·현재 출처·보완 검색 회귀와 고정 후보 비교, 패키지 | 사용자 DB 검색 및 생성 품질 전체 보증 |
| [test.22](archive-center-4.3-test-build-22.md) | 검색 질문 형식·담당 순서·출처 보존 회귀, 20개 저장 응답의 외부 AI 없는 재처리, 패키지 | 새 프롬프트에 따른 새로운 AI 판단·실제 출력 개선 |
| test.12 후보 전달 벤치마크 | 작은 시험 데이터의 후보 재계산과 사본 전달 함수 비교 | 전체 RP 지연 개선 폭, 제공자 동시 처리 능력, 40M 성능 |
| 실제 화면 관측 | test.8 HUD와 test.11 백엔드에서 세계 담당 1차 12.7초 실패·2차 19.1초 성공 기록 | 최초 `calls[].error`의 원인과 현재 빌드에서의 동일 원인 해결 여부 |

**남은 확인:** 동일 test.20 플러그인·백엔드의 실제 로드, 기본 RisuAI/PocketRisu 분기·재분기,
실제 제공자 1차/보충/Flex, 원문·비밀 범위·최종 payload 적용·표시 결과, 비용과 지연이다.
`world_state` 최초 오류는 **미해결 원인**으로 남긴다. test8의 원래 payload 불일치와
정밀 검색 오류의 동일 원인 여부도 원본 관측 자료가 없어 확정하지 않았다.
**race 검사는 CGO 비활성화와 gcc 부재로 미실행**이다. 동시 실행 회귀가 그 결과를 대신하지 않는다.

## 단계별 기록과 현재 소유자

| 기록 | 반영한 범위 |
| --- | --- |
| [test.1](archive-center-4.3-test-build-1.md) / [test.2](archive-center-4.3-test-build-2.md) | 선행 피드백·분기·Gemini 시험 패키지 / 다섯 역할 전처리 연결 |
| [test.3](archive-center-4.3-test-build-3.md) / [test.4](archive-center-4.3-test-build-4.md) | 설정창 선표시·접속 복구 / 전처리 카드·반응형 편집 화면 |
| [test.5](archive-center-4.3-test-build-5.md) / [test.6](archive-center-4.3-test-build-6.md) | 공통 프롬프트·개별 연결 / 제공자 선택·Flex·온도·출력 토큰 |
| [test.7](archive-center-4.3-test-build-7.md) / [test.8](archive-center-4.3-test-build-8.md) | 역할 기본 지침·키 편집 / 담당별 시간과 응답 시각 HUD |
| [test.9](archive-center-4.3-test-build-9.md) / [test.10](archive-center-4.3-test-build-10.md) | Vertex 전송 / 사용자 자유·근거/로어/검색·관측 보완 |
| [test.11](archive-center-4.3-test-build-11.md) / [test.12](archive-center-4.3-test-build-12.md) | 가이드 강도 / 보충 검색 병렬화·후보 재사용·시간 분리 |

| 책임 | 현재 소스와 대표 검증 소스 |
| --- | --- |
| Host 관측·적용·화면 | [Archive Center.js](../Archive%20Center.js): `observeRisuWorldlineMessageOrigins`, `loadMemoryPreprocessingPanel`, `turnWorkflowHUDTimingHTML`. [Host 회귀](../go-service/cmd/js-route-variant-smoke/main_worldline_origins_43_test.go), [HUD 회귀](../go-service/cmd/js-route-variant-smoke/main_hud_timing_43_test.go) |
| 설정·역할·추천·분석/검색 순서 | [prepare_turn_multi_agent.go](../go-service/internal/httpapi/prepare_turn_multi_agent.go): `handleMultiAgentSettings`, `callMultiAgent`, `runMultiAgent`. [설정/전달 회귀](../go-service/internal/httpapi/prepare_turn_multi_agent_test.go), [병렬 검색 회귀](../go-service/internal/httpapi/prepare_turn_multi_agent_search_test.go) |
| 기존 검색 연결·요청 조립 | [group_turn_prepare.go](../go-service/internal/httpapi/group_turn_prepare.go): `handlePrepareTurn`, 요청 내부 `searchAssemblyMu`. [등록 라우트 회귀](../go-service/internal/httpapi/prepare_turn_supplement_search_route_test.go) |
| 후보·선정·원문 전달 | [prepare_turn_priority_memory.go](../go-service/internal/httpapi/prepare_turn_priority_memory.go): `buildPrepareTurnPriorityMemoryDeliveryPlan`, `clonePrepareTurnPriorityCandidatePool`. [후보 회귀](../go-service/internal/httpapi/prepare_turn_candidate_pool_test.go), [입력 회귀](../go-service/internal/httpapi/prepare_turn_multi_agent_input_test.go) |
| 로어·출처·최종 가이드 | [prepare_turn_lorebook_reference.go](../go-service/internal/httpapi/prepare_turn_lorebook_reference.go), [prepare_turn_render.go](../go-service/internal/httpapi/prepare_turn_render.go), [group_proxy.go](../go-service/internal/httpapi/group_proxy.go): `publisherStrengthProfile`. [로어 회귀](../go-service/internal/httpapi/prepare_turn_lorebook_preprocessing_test.go), [Publisher 회귀](../go-service/internal/httpapi/publisher_plan_test.go) |
| 제공자 전송·시간 | [proxy_provider.go](../go-service/internal/httpapi/proxy_provider.go), [turn_workflow_hud.go](../go-service/internal/httpapi/turn_workflow_hud.go), [chroma.go](../go-service/internal/vector/chroma.go). [전송 회귀](../go-service/internal/httpapi/group_proxy_test.go), [HUD 회귀](../go-service/internal/httpapi/turn_workflow_hud_test.go) |
| 분기 출처·저장 | [worldline_message_origins.go](../go-service/internal/httpapi/worldline_message_origins.go), [mariadb_narrative_state.go](../go-service/internal/store/mariadb_narrative_state.go). [분기 회귀](../go-service/internal/httpapi/worldline_message_origins_test.go), [격리 DB 시험](../go-service/cmd/mariadb-schema/worldline_43_mariadb_integration_test.go) |
| 선행 저장·삭제 | [mariadb_memory_derivation.go](../go-service/internal/store/mariadb_memory_derivation.go), [memory_reprocessing_worker.go](../go-service/internal/httpapi/memory_reprocessing_worker.go), [migration 013](../migrations/013_precise_memory_text_fields.sql). [피드백 DB 시험](../go-service/cmd/mariadb-schema/feedback_43_mariadb_integration_test.go) |

[전처리 비교](archive-center-4.3-preprocessing-comparison.md)는 보관된 예제 6개와 현재
AC 호출/검색 구조를 읽기 전용으로 비교한 문서다. 같은 조건의 실측 우열이나 후처리 도입 근거가 아니다.
읽기 전용 리팩토링 점검을 바탕으로 한 [4.4 리팩토링 계획](archive-center-4.4-refactoring-plan.md)은
현재 소유자·실제 호출·시험 경계를 기준으로 한 **계획**이다. 이 요약에서 리팩토링 구현이나
검증 완료를 선언하지 않는다. 현재 상태의 안내는 이 문서와
[STRUCTURE](../STRUCTURE.md), 작업 제약은 [AI_GUARDRAILS](../AI_GUARDRAILS.md)를 함께 따른다.
