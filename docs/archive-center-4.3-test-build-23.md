# Archive Center 4.3.0-test.23

기준일: 2026-09-09. 상태: **implemented_unverified**.
소스 회귀와 Windows 패키지를 검증했으며, 실제 RisuAI와 사용자 DB에서의 확인은 남아 있다.
공개 GitHub 릴리스는 4.2.0을 유지한다. 이전 test.22 ZIP은 변경하지 않았다.

## 테스트 파일

- [Windows 자동 설치 패키지](../_test-builds/4.3.0-test.23/Archive%20Center%204.3.0-test.23%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.23/Archive%20Center%204.3.0-test.23%20Windows%20Auto%20Install%20Package/Archive%20Center.js)
- [Go 백엔드](../_test-builds/4.3.0-test.23/Archive%20Center%204.3.0-test.23%20Windows%20Auto%20Install%20Package/bin/archive-center-go.exe)

이번에는 **플러그인과 백엔드를 함께** 적용한다. 백엔드 실행은 사용자가 한다.
설정·API 키·저장 프롬프트·기존 DB·대화는 변경하지 않았다. test.22의 전처리 보완을 포함하며,
이번 작업에서는 프롬프트, 기억 점수, 선정 폭, 예산, 출판사 동작을 변경하지 않았다.

## 수정 내용

1. **부모 원본이 저장 대기 중인 재분기**
   기존에는 부모 세션은 찾더라도 분기 원본의 source revision이 아직 없으면
   `parent_fork_source_history_unresolved`로 남아 부모·조상 기억의 범위가 끊겼다.
   Go의 기존 계보 판정이 이미 관측한 부모 메시지 순서와 저장된 턴/상속 경계의 대응을
   사용하도록 수정했다. 첫 문자 인사, 분기 마커와 사용자/응답 분기점의 차이도 반영한다.
   저장 좌표가 있는 경우 화면의 순번보다 그 좌표를 기준으로 삼는다.
   부모의 평론가를 강제로 호출하거나 자식 입력으로 부모의 저장 대기를 확정하지 않는다.
   **기존 기억을 상속할 수 있게 하는 수정이며, 미저장 응답의 기억까지 생성한 것은 아니다.**
2. **콜드스타트의 상속 구간 재진입**
   두 번째 응답 병합에서 Go의 `skip_pre_route_visible_pair`를 무시하던 부분을 고쳤다.
   제외 결과에 양수 턴 번호가 있어도 상속 구간을 자식의 복구 항목에 다시 넣지 않는다.
3. **콜드스타트의 번역문 재진입**
   첫 단계에서 정리한 텍스트를 두 번째 병합이 정리 전 응답으로 덮던 문제를 고쳤다.
   기존 원문 정리 함수를 같은 지점에 적용한다. GigaTrans 원문과 번역이 함께 있는 자료에서
   원문만 복구 항목에 들어간다. 기존 불완전 태그 처리와 Host 식별 해시는 유지한다.
   제보자의 별도 정규식 설정은 받지 못했으므로 모든 사용자 정규식의 적용을 검증한 것은 아니다.

Go 변경 소유자는 `group_turn_range_decision.go::resolveRisuWorldlineObservation()`과
`worldline_message_origins.go::risuWorldlineObservedSourceTurn()`이다. 새 API·DB 스키마·저장 작업은
없다. JS는 기존 `computeActiveChatRescanDryRunPlan()`의 백엔드 결과 적용과 텍스트 정리,
분기 상태 번역문, 버전 식별자만 변경했다.

## 검증

| 항목 | 결과 / 범위 |
| --- | --- |
| 콜드스타트 4개 재현 | 일반 문장, GigaTrans 표시, 사용자 행 삭제, 전체 상속 구간. 수정 전 실패 → 수정 후 통과 |
| 저장 대기 재분기 12개 경우 | ID 보존/재발급, 사용자/응답 기준점, 저장 턴 오프셋, 5개 세션 연쇄 분기. 수정 전 Go 소스 overlay에서는 실패 → 수정 후 통과 |
| 지연 저장 소유권 | 자식 입력은 부모의 pending marker를 소비하지 않음. 부모의 새 사용자 행에서만 기존 확정 경로가 실행됨 |
| 기존 라이프사이클 | 같은 요청 재시도, 같은 사용자 행 리롤, 응답 삭제 후 사용자 행 편집·재생성, 같은 문장의 새 사용자 행, 정상 새 턴 회귀 통과 |
| 전체 Go | 35개 패키지, 최상위 시험 3,862개와 하위 사례 806개 통과. 실패 0 |
| 실행 환경 의존 시험 | MariaDB/Chroma/실제 제공자/POSIX 권한 관련 12개 미실행. 통과 수에 포함하지 않음 |
| JavaScript | 활성 소스와 패키지의 구문 검사 통과 |
| 패키지 | 관리 파일 53개 크기·해시·ZIP 내용 일치. 활성 JS·프롬프트·SQL과 패키지 비교 통과 |

콜드스타트 시험은 실제 JS 생산 함수를 읽어 실제 Go 라우팅 HTTP 처리기와 연결했다.
Host 메시지 읽기와 Store 입출력만 fixture다. 사용자 DB, 실제 콜드스타트 저장 작업,
평론가·출판사·외부 AI 호출은 실행하지 않았다. 상속 범위 확인은 실제 모델이 그 기억을
사용했다는 증거와 구분한다. 기존에 잘못 저장된 중복/번역 데이터의 일괄 수정도 하지 않았다.

2026-09-09 확인한 고정 upstream:

- [RisuAI Chat.svelte, c454df8](https://github.com/kwaroran/RisuAI/blob/c454df882aaf32e02a22da26d3718c8cadc97814/src/lib/ChatScreens/Chat.svelte): 기준 메시지를 포함한 prefix 복사와 부모 ID 마커.
- [PocketRisu Chat.svelte, ca09a80](https://github.com/PocketRisu/PocketRisu/blob/ca09a80746e74e5334145e5e78af47ce423e0eba/src/lib/ChatScreens/Chat.svelte): prefix 복사 후 메시지 ID 재발급, 마커에는 부모 ID 유지.

이는 해당 커밋의 소스 확인이다. 현재 사용자의 브라우저에 그 커밋이 로드되었다는 뜻은 아니다.
현재 제보된 장애와 같은 경로를 재현했지만 제보자의 과거 세션 전체를 복원한 시험은 아니다.

## 패키지 식별과 변경량

ZIP 크기: **18,101,051바이트**.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `ca9b049eadd94e3151a9802484b13861f71dcacc6ae1fa4781b46fb49fbc0f47` |
| Go 백엔드 | `2bddca88823443024b05ca14e3dbca064512a5d604f82900c67876b7ee7cc873` |
| 플러그인 | `d4fb9a85b99e1e992315fbc70538ce104caed812cdf5e520b14341da9a017f90` |

이번 작업 직전 test.22 소스 대비 JS **+12/-7**: 버전·빌드 정보 여섯 줄 교체, 상태 설명 세 줄,
기존 병합의 결과 적용·원문 정리 수정이다. Go 운영 파일은 **+101/-1**이며,
턴 좌표 판정은 Go가 소유한다. 이전부터 남아 있던 adapter 정책 부채는 이번에 확장하지 않았다.
manifest는 로컬 기준 커밋 `6ef8f74`와 `source_dirty=true`를 함께 기록한다.

증거: `_diagnostics/20260909-worldline-coldstart/`의 수정 전 재현과
`_diagnostics/20260909-worldline-coldstart-fix/`의 `pending-before-test.txt`,
`expanded-targeted-test.txt`, `full.jsonl`, `suite-summary.json`, `runtime-change.diff`,
`package-verification.json`. 구조 문서·작업 지침·4.3 현황·피드백 기록·로드맵을 함께 갱신했다.

다음 실제 확인은 이전 턴 저장 모드에서 부모의 마지막 응답 직후 재분기하고, 부모·조상
기억 범위가 이어지는지 보는 것이다. 콜드스타트에서는 상속 구간이 복구 대상에서 빠지고
해당 번역 형식의 원문이 유지되는지를 확인한다. 백엔드는 이번 작업에서 실행하지 않았다.
