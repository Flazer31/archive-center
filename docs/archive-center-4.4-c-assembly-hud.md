# 4.4-C — 기억 조립과 HUD 내부 경계 정리

## 후속 실사용 보고 — 2026-09-11

- 사용자는 test.3으로 실제 진행 중이며, 기존 설정 유지·전처리 모델 변경/저장·HUD 진행이 정상이라고 확인했다. 아래의 교체 전 보류 기록은 당시 상태다.
- 속도가 짧아진 체감은 없었다. 47턴 보고의 준비 시간은 약 302초, 보충 검색은 122.2초다. 네 후보 조립은 28.1/32.1/30.8/30.7초로 합계 121.7초이며 순서 대기가 누적된다. 검색별 임베딩·벡터 조회는 약 0.5초다.
- 이 수치로 후보 조립 구간의 지연은 확인되지만 함수 내부 연산/할당/GC 중 원인과 RAM 누수 여부는 확정하지 않는다. 대기 시간은 이미 앞선 조립 시간과 겹치므로 다시 합산하지 않는다.
- UI test.3 / HUD 4.3.1 표시는 패키징이 VERSION만 갱신하고 HUD의 BUILD_ID 리터럴을 남긴 결함이었다. BUILD_ID를 VERSION 참조로 수정했다. JS +1/-1, 422개 JS 회귀 통과. 사용자 서비스는 조작하지 않았다.
- 전달 자료에서 AI가 선택한 사실 43개와 요약 1개는 모두 백엔드 조립본에 남아 있었다. 다만 현재 장면의 집단 내 공개 지위에 해당하는 후보들이 1·2차 모두 제공됐어도 미선택됐다는 별도 내용상 약점이 확인됐다. 이 사실을 Go의 추천 제거나 저장 소실로 분류하지 않는다.
- 제공된 최종 입력의 전체 검증 상태는 `effective_user_text_not_observed`다. 조립본 포함과 실제 최종 제공자 입력/삭제된 출력의 반영을 구분하며 후자는 이번 자료만으로 확인되지 않았다. 세션별 원문/ID 대조는 저장소 상위 `_diagnostics/20260911-test3-status-memory/`에 둔다.

작성: 2026-09-11. 소스·격리 검증 기록이며, 실제 loaded RisuAI는 `implemented_unverified`다.
진행 중인 사용자의 4.3 세션·설정·백엔드를 교체하거나 중단하지 않았다.

## 시작 기준

- 초기 B 복원점 `0dfd9bf`, 전체 설정 로드 결함을 수정하고 검증한 B 복원점 `9dadb1b`.
- B의 `4.4.0-test.1`은 삭제된 helper 호출로 설정 복원 실패가 있어 사용하지 않는다.
  수정된 `test.2`의 전체 JS와 실제 Go 실행 파일로 조회→저장→새로고침→재열기를 통과한 뒤 C를 진행했다.
- `ops/settings-pair-smoke.cjs`는 호스트 API만 격리하며 실제 JS 저장·정규화·전송, 브라우저 저장소,
  Go 설정 조회/업데이트 경로를 실행한다. 사용자 DB·Chroma·유료 모델 검증과는 구분한다.

## 변경 내용과 보존 범위

| 항목 | 실제 변경 | 유지한 동작 |
| --- | --- | --- |
| RF04 | `prepareTurnAssemblyInput`, `prepareTurnAssemblyPerspective`, `prepareTurnMemorySelectionContext`로 긴 위치 인자와 내부 정책 값을 구분. 생산 호출부가 직접 필드를 구성 | 공개 perspective, 후보 생성/선정, 예산, 출처·공개 범위, DTO와 Store 인터페이스 |
| RF05 | `prepareTurnVectorRecallResult`에 공개 Trace와 JSON 제외된 precise hit를 분리. handler의 수동 private key 삭제 제거 | broad·aggregate·precise 각각의 검색/상태/부분 실패, 같은 벡터 재사용, canonical hydration과 공개 trace |
| RF06 | 같은 assembly owner에서 후보 생성 이후의 최종 주입문/표시 작업을 구분. 보충 검색은 `buildPrepareTurnSupplementCandidates`를 소비 | 후보·원문·순서·출처, 검색 횟수, 합류 후 2차, 요청별 mutex, AI 추천 및 무추천 Go 선정 |
| RF07 | `decodeTurnWorkflowHUDStreamLine`과 `readTurnWorkflowHUDNDJSON`으로 동일 decode/줄 분할/read 처리를 공유 | 현재/이전 request ID·watch token·abort controller·render owner, 기존 취소/종료, 카드·타이머·경고 |
| RF08 | A에서 완료한 SQL 독립 기대값과 mutation 검출을 재사용 | 실제 삭제 실행 없음. `DeleteSession` 이동은 필요가 확인되지 않아 수행하지 않음 |

후보 snapshot 생성은 `prepareTurnResolvePrioritySourcePool` 한 곳을 사용한다.
보충 검색의 반환값에는 사용하지 않던 최종 delivery plan을 만들지 않으며, 일반 준비와 최종 AI 추천 반영은
기존 최종 조립을 거친다. 새 저장소·검색·선정 경로, 영구 cache, 수신 거부 조건을 추가하지 않았다.
출판사·평론가·전처리의 프롬프트와 호출 설정도 변경하지 않았다.

## 검증과 발견한 시험 보완

- C 변경 전 등록 `/prepare-turn` 경로의 전처리 OFF/추천/빈 추천/실패/보충 실패 × 출판사 OFF/ON,
  10조합을 캡처했다. 변경 후 전체 구조를 비교해 후보·선정·원문·호출·payload 차이가 없었다.
  제외한 항목은 실행 시간, 실행 ID/시각, 격리 서버 임시 포트, 시간 필드 때문에 달라지는 입력 글자 수다.
  모델 입력 문자열 안의 JSON도 파싱해 시간 필드 외의 내용이 같은지 비교했다.
- `Test44SupplementCandidatesMatchFullAssembly`: 같은 공개/비공개 자료와 예산 0/100/60000,
  자동/사용자 예산 및 우선순위 활성/비활성에서 전체 조립과 보충 후보의 모든 필드를 비교한다.
- precise hit의 실제 내부 도착과 JSON 비노출, 등록 HTTP의 precise 점수→최종 plan 검증을 유지했다.
- `Test44HUDStreamChunksAndIndependentRequestOwners`: 한국어 UTF-8 바이트 분할, CRLF/빈 줄,
  마지막 줄의 개행 없음, 잘린 JSON, 요청 불일치, 대기 중 취소, 오래된 요청, 두 카드 동시 소비를 확인했다.
  기존 HUD EOF/연결/타이밍/카드 검증도 실행했다.
- 전체 검사 중 두 시험 보완이 필요했다. 원작 설정 시험은 B에서 제거된 helper 대신 현재 상수 fixture를
  제공하도록 수정했다. 조립 시험 변환 helper는 최근 대화의 원래 문자열 목록을 유지하도록 수정했다.
  기존 기대값을 완화하지 않았고, 실제 `pending_threads` 후보와 담당 입력까지 다시 검사했다.
- JavaScript 구문 검사 통과. C의 JS 변경량은 **+39/-55**다.
- 최종 `go test -mod=readonly ./... -count=1 -json`: **5,772개 통과, 실패 0, 건너뜀 13**.
  건너뛴 항목은 실제 MariaDB/Chroma/제공자/공개 배포 검증과 POSIX 전용 검사이며 통과로 계산하지 않았다.

## 성능 측정의 범위

동일한 작은 공개/비공개 fixture에서 전체 조립 후 후보 추출은 약 498KB/6010회 할당,
후보 전용 조립은 약 413KB/5364회였다. 할당량은 약 17%, 횟수는 약 11% 줄었다.
실행 시간은 전체 0.79~0.82ms, 후보 전용 0.68~1.08ms로 흔들림이 있어 고정적인 속도 향상 수치로 보지 않는다.
이 결과는 사용자가 보고한 후보 조립 24초나 전체 6분의 해소 증거가 아니다.

담당별 전체 정렬/최근 대화 재계산은 이번 표본에서 별도 개선 근거를 확보하지 않아 수정하지 않았다.
기존 snapshot 재사용, 요청별 조립 lock, AI 호출의 120초 제한을 제거하지 않았다.
실사용 성능은 실제 교체 후 동일 설정·유사 입력의 단계별 시간을 비교해야 한다.

## 증거 위치와 남은 확인

저장소 상위 `_diagnostics/` 아래:

- `20260910-44c-before/`: 변경 전 10조합 캡처와 기존 snapshot benchmark.
- `20260911-44c-rf04/`: 변경 후 캡처, `normalized-comparison.json`, 후보/보충/HUD 검사,
  `full-go-final.jsonl`, `supplement-before.txt`, `supplement-after.txt`.
- `20260910-44b-pair-fixed/`: C 착수 전 B 전체 JS/Go 설정 검증.
- `20260911-44c-pair/result.json`: **test.3 전체 JS + 실제 패키지 Go**로 설정 조회·저장·페이지
  새로고침·재열기 재검증 통과. HUD 호스트 API는 이 격리 시험에 없으므로 실제 RisuAI HUD 확인을 대신하지 않는다.

로컬 Windows 교체용은 `source/_dist/4.4.0-test.3/Archive Center 4.4.0-test.3 Windows Test.zip`이다.
JS와 Go 모두 `4.4.0-test.3`로 패키징했고 `01_start_archive_center_windows.bat`를 포함한다.
ZIP SHA256: `1b75151d4f6f2456aef27841873c12af95616d12d3cf35ae62b95fa200878b41`.
빌드 결과는 green이며 GitHub 업로드와 현재 실행 파일 덮어쓰기는 하지 않았다.

실제 PocketRisu의 플러그인 교체·설정 재열기·채팅 HUD 및 사용자 DB/Chroma/모델 검증은 남아 있다.
사용자가 현재 세션을 계속 쓰는 동안 이 실행은 보류한다. D의 의미 중복 통합과 E의 결합 검증은 착수하지 않았다.
