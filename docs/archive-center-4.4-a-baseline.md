# Archive Center 4.4-A — 비교 기준과 시험 보완

작성: 2026-09-10. 상태: **준비 작업 수행**. B/C 리팩터링, D 의미 통합, E 배포 검증은 미착수다.
실행 로직·프롬프트·기억 정책·DB schema·설정값·패키지는 변경하지 않았다. JavaScript 변경: **+0/-0**.
계획 권위는 [통합 로드맵](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md), 작업 범위는 [4.4 실행 계획](archive-center-4.4-refactoring-plan.md)이다.

## 1. 식별한 기준

| 구분 | 이번 확인 결과 |
|---|---|
| 활성 소스 | `source`, branch `work/4.2.0`, HEAD `026dcbf3b45adcf69b254673d439b24e943115b3` |
| 정식 기억 기준 | `v4.3.1^{commit}` = `0821d69e9be9bc406bd9f0092a03398498f23b5f` |
| 착수 전 차이 | 미커밋 `docs/archive-center-4.4-refactoring-plan.md` 보존. 태그와 HEAD의 runtime 경로 차이는 Go/ops README뿐이며 실행 소스는 동일 |
| 기준 파일 | 추적 파일 611개의 SHA-256·크기, 착수 diff, HEAD/tag를 로컬 artifact로 보존 |
| 실행 도구 | Go 1.26.6 windows/amd64, Node 24.19.0, 별도 진단용 Go cache/tmp |
| 승인된 Edge UI 관찰 | Archive Center **4.3.1-test.4**, 원격 백엔드 연결 (개인 주소 비공개) |
| 로컬 실행 파일 관찰 | `source/_dist/4.3.1-test.4/Archive Center 4.3.1-test.4 Windows Test/bin/archive-center-go.exe` |
| 관찰의 한계 | 로컬 프로세스 경로만으로 원격 주소가 응답한 바이너리 hash를 확정하지 않음. 사용자 HUD의 **4.3.0** 표기와 현재 UI 관찰을 동일 실행으로 간주하지 않음 |

설정창을 열어 읽고 닫았다. 채팅 생성·저장·연결 시험·키 표시·백엔드 재시작을 실행하지 않았다.
새 실제 MariaDB/Chroma 쓰기나 유료 모델 호출도 없다. 이전 정식 설치/업데이트 증거는
[4.3.1 배포 기록](archive-center-4.3.1-release-verification.md)을 그대로 인계하며, 이번 A의 새 실행 결과로 세지 않는다.

개인별 모델·추론·예산 설정의 관찰값은 공개 문서에 포함하지 않는다.
실사용 설정 관찰과 별도로 아래 시험은 통제된 fixture를 사용한다.

## 2. 변경한 시험과 재현 결과

- `memory_recall_restoration_matrix_test.go`: 기존 등록 Go prepare 경로의 독립 판정은 유지하고,
  선택적으로 요청·설정·40개 원본 기억·vector 후보·provider body/호출 수·검색 한도·전체 응답을 캡처한다.
  환경 변수 `ARCHIVE_CENTER_TEST_BASELINE_DIR`은 테스트에서만 읽는다. 실행 서버에는 새 기록 기능을 추가하지 않았다.
- 편집자 OFF/정상/빈 추천/실패/보완 실패 × 출판사 OFF/ON **10조합**을 같은 입력과 18,000자 예산으로 보존했다.
  이 matrix의 활성 편집자 담당은 사건·세계 두 개다. 다섯 담당 전체 검증은 별도 기존
  `Test43MultiAgentParallelRoundsAndPartialFailure` 등의 실행 근거로 구분한다.
- 원본의 습관 기억이 검색→후보→선정→memory text→payload plan에 도달하고 예산을 지키는지 독립적으로 검사한다.
  캡처된 결과를 그대로 정답으로 되읽어 통과시키지 않는다. provider/Store/vector는 외부 경계 fixture이며
  실제 모델 판단·실제 DB transaction·RisuAI 최종 전송의 증거는 아니다.
- `mariadb_logical_turn_replace_test.go`: production command builder에서 기대 목록을 얻거나
  `(?s).+`로 임의 SQL을 통과시키던 네 transaction 사례를 보완했다. **37개 SQL의 대상·범위·인자·순서**를
  독립 기대 목록으로 검사하고, legacy 물리 정리 두 문장 및 해당 턴/이후 턴 삭제 차이도 구분한다.
  정상 교체, 삭제 직후 재생성, 빈 세션 첫 턴 재생성, 반복 rollback을 실행한다.
- 진단용 Go overlay에서 기억 DELETE 누락, 잘못된 턴 인자, `>=`를 `>`로 변경하는 세 변형을 시험했다.
  **이전 rollback 시험은 셋 모두 통과했고, 보완 시험은 셋 모두 SQL 기대 불일치로 실패했다.**
  최종 helper 정리 후에도 세 실패를 재확인했다. production 파일에는 변형을 쓰지 않았다.

| 실행 로그 | 결과 | 범위 |
|---|---:|---|
| `before-targeted.jsonl` | 226 pass | 착수 전 선택 회귀 |
| `after-targeted.jsonl` | 260 pass | 캡처/SQL 보완 및 추가 lifecycle 선택 회귀 |
| `store-full.jsonl` | 369 pass | 최종 SQL helper 변경 뒤 store 전체 시험 |
| `additional-contracts.jsonl` | 296 pass | 전처리·예산·관점·로어북·분기·cold-start·설정·vector/config 관련 선택 시험 |

숫자는 하위 사례를 포함하며 로그 사이 중복이 있다. 고유 시험 수나 전후 성능 향상 수치로 합산하지 않는다.
위 네 로그에는 fail/skip이 없다. 일부 JS 시험은 source/VM 수준이며 버튼 실사용 검증으로 확대하지 않는다.

## 3. 33턴 준비 6분 19초 기록

사용자가 제공한 HUD는 `ARCHIVE CENTER 4.3.0`, 준비 379초, 백엔드 전체 377초다.
`본문 응답 기다리는 중`이므로 본문 모델의 생성 완료 시간은 포함되지 않은 기록이다.

| 백엔드 구간 | 초 | 포함 관계 |
|---|---:|---|
| 기억 검색 | 26.7 | 첫 검색 묶음. 보완 검색의 단일 vector 조회와 구분 |
| 기억 조립 | 327.8 | 편집자 두 round·보완 검색 포함 |
| 최종 조립 | 21.6 | 출판사 21.5초 포함 |
| 나머지 | 약 0.9 | 위 큰 구간과 전체 차이; 표기 반올림 포함 |

편집자는 각 round 안에서 병렬이지만 1차 완료→보완 검색/조립→2차→출판사는 순서가 있다.
세계·사물 담당은 1차 실패 120초, 2차 형식 보정 후 사용 82.6초로 각 round에서 가장 오래 걸렸다.
다른 담당의 시간을 모두 더하는 계산이 아니다. 120 + 74.5 + 82.6 = **약 277.1초**가
세 순차 구간의 큰 대기 시간을 설명한다. 기억 조립 327.8초와의 차이 약 50.7초는 아직 상세 귀속이
안 된 입력 구성/조립 등 시간이며 전부 Go CPU나 검증 비용이라고 단정하지 않는다.

| 보완 검색 | 실제 경과 | 후보 조립 | 조립 순서 대기 | vector 조회 |
|---|---:|---:|---:|---:|
| 주관 기억·관계 | 26.7초 | 26.2초 | 0초 | 0.27초 |
| 미해결 목표 | 50.5초 | 23.8초 | 26.2초 | 0.28초 |
| 사건·진행 | 74.5초 | 24초 | 49.9초 | 0.27초 |

조립 자체 26.2 + 23.8 + 24 = **74초**가 보완 검색 전체 74.5초의 대부분을 설명한다.
대기 열은 앞의 조립과 겹친다. 임베딩은 각각 0.23~0.30초다. 이 구간을 Chroma 서버 지연으로 돌리지 않는다.
전체 회귀 원인이나 현재 4.3.1 실행의 시간이라고 확정할 자료는 아직 없다.

현재 소스에서 연결한 후속 측정 대상:

1. `group_turn_prepare.go`의 `searchAssemblyMu`는 요청 안의 조립만 직렬화한다. 검색은 밖에서 겹친다.
   보완 질문마다 전체 `buildPrepareTurnInjectionAssemblyWithBudget()`를 만든 뒤
   `multiAgentCandidatePool()`만 취한다. **RF06에서 후보 구성과 쓰지 않는 최종 렌더링을 분리할 대상**이다.
2. `prepare_turn_priority_memory.go`의 fact 반복→`prepareTurnPriorityQuerySetRelevance()`→
   `prepareTurnPriorityRelevance()`는 같은 질의의 어휘 추출을 다시 실행한다. 요청 안에서 같은 결과를
   재사용할 수 있는지 비교한다. 조사 일치·어휘 순서·점수·후보 수를 줄이는 변경과 구분한다.
3. `prepare_turn_multi_agent.go`의 두 round 경계와 입력 구성 비용을 따로 본다. 현재 긴 모델 대기와
   후보 조립 비용을 한 원인으로 묶지 않는다. 추가 보완을 없애거나 시간 한도를 줄이는 작업은 이번 A에 없다.

기존 benchmark를 1회/3반복해 참고값도 보존했다. 질의 2,048단어 추출은 3.28~3.93ms,
작은 fixture의 후보 신규 구성은 89.7~96.5μs, 기존 request snapshot 복사는 2.7~10.1μs다.
한 번의 실제 준비 전체와 규모가 다르므로 이 숫자로 74.5초의 개선 폭을 예측하지 않는다.

## 4. 설정과 fallback 위험

- **확인한 helper 결함:** 실제 JS `withUiBridgeSettings()`를 추출해 독립 VM에서 두 비동기 실행을
  겹쳤다. A 시작→B 시작→A 완료→B 완료 때 최종 설정이 최초값 대신 A의 임시값으로 남았다.
  DOM/timeout 공급 경계만 stub했고 원래 helper를 실행했다. 실제 브라우저 설정 저장이나 잘못된
  network 전송을 재현한 것은 아니다. RF02의 기존 owner 수정 대상으로 인계하고 이번에는 수정하지 않았다.
- **SUPPORTED_RISK:** fallback `fakeVectorStore`의 Search/Health/Count 호출 기록 쓰기를
  `mutationFencedStore`의 RLock이 조회끼리 보호하지 않는다. 실제 fallback constructor와 wrapper를
  함께 쓰는 병렬 진단 overlay를 준비했다. 이 환경은 CGO 비활성/C compiler 부재로 `go test -race`가
  시작되지 않아 race 결과는 **미검증**이다. Chroma 사용 중인 실제 사용자에게 같은 실패가 있었다는 뜻이 아니다.
- fixture, no-op fallback, 옛 진단 필드를 운영 기억 오염이나 불안정의 증거로 단정하지 않는다.
  생산 경로와 사용자 자료에 미친 영향은 별도 증거가 필요하다. 새 fallback이나 수락 조건은 추가하지 않았다.

## 5. 재실행 자료와 인계

로컬 자료 폴더: [`_diagnostics/20260910-44a-baseline`](../../_diagnostics/20260910-44a-baseline/).
`source-baseline.json`, `preexisting.diff`, `ui-baseline-sanitized.json`, `verification-summary.json`,
`replay/*.json`, `user-hud-33-timing.json`, SQL 변형 overlay/로그, `ui-settings-overlap.cjs/json`, `fake-read-race-overlay.json/log`,
`benchmarks.txt`를 보존했다. 개인 채팅·DB·키·배포 ZIP을 fixture로 복사하지 않았다.

Go 명령은 `source/go-service`에서 실행한다. 이 환경에서는 `GOCACHE`/`GOTMPDIR`을 위 진단 폴더의
`go-cache`/`go-tmp`로 지정했다. 경계 결과 캡처는 `ARCHIVE_CENTER_TEST_BASELINE_DIR`에 출력 폴더를
지정하고 `go test -mod=readonly ./internal/httpapi -run '^TestMemoryRestorationBudgetAndOptionMatrix$' -count=1`로 재생성한다.
이는 녹화된 실제 AI 판단의 재생이 아니라 고정 fixture 응답을 생산 경로에 넣는 재실행이다.

- **B/C 비교:** 후보 참조·원문·순서·시점/관점·공개 범위·빈 추천/일부 실패 의미·payload·호출 수를 유지한다.
  소스 식별 hash, 자동/명시 예산, 독립/공유 provider 설정을 함께 비교한다. 동적 URL·시간·request ID는
  원본 캡처에 남기되 단순 파일 byte 동일성을 기능 동등성으로 사용하지 않는다.
- **D 준비:** 기존 priority 회귀의 같은 인물/장소의 다른 사건, 현재 필드와 과거 값, 관점별 사실을 인계한다.
  금액 혼재/이야기 시간의 알려진 한계는 개선 대상이며 4.3.1 출력이 정답이라는 oracle로 고정하지 않는다.
  신규 의미 통합 계약과 독립 기대값은 D에서 정의한다.
- **E 인계:** 포트 사용자값/빈 Enter 기본값, 새 파일 추가·삭제·DB migration 포함 업데이트, 01/한 줄 설치는
  기존 4.3.1 계약/배포 기록과 비교한다. 이번 A가 새 OS 패키지를 검증했다는 주장은 하지 않는다.
- **실사용 경계:** 현재 UI 버전/설정은 확인했다. 새 생성의 최종 provider payload·모델 활용·실제 DB 저장을
  검증한 것은 아니다. 이번 377초 자료의 실행 버전/원인별 CPU 시간과 race는 열린 검증 항목으로 남긴다.
