# Archive Center 4.4 리팩터링·검증 및 후속 인계

상태: A 준비·B/C 및 test.17 반영 / E 내부 조합·격리 설정 검증과 한정된 실사용 시간·RAM 확인 / 정식 설치·업데이트 검증 잔여 / D 의미 통합 **DEFERRED**
작성일: 2026-09-07 / 기준 갱신: 2026-09-14 — test.17 실사용 시간·RAM과 기존 통합 검사 기록 재확인, 4.5·4.6 인계 유지

범위: 4.4의 반영 작업과 남은 검증, 이후 버전의 경계. B 복원점·격리 설정 검증 후 C를 진행했다.
로컬 테스트 패키지와 실사용 관측·정식 배포는 구분한다. 아래 날짜별 기록은 당시 상태다.

## 2026-09-14 — 현재 범위와 마무리 기준

[통합 로드맵의 현재 배정](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#version-handoff-20260914)과
[작업 현황·검증·후속 인계](archive-center-4.4-status-and-handoff-20260914.md)를 따른다.

- 현재 로컬 패키지는 **4.4.0-test.17**이다. A/B/C 이후 오류 보고·추론·원작 수집 품질,
  전처리 효율/묶음 호출, 다중 입력, HUD·자동 예산 및 요청 단위 조립 재사용을 포함한다.
- [실사용 관측과 검사 범위](archive-center-request-preparation-20260914.md#live-performance-test17):
  추가 검색 1.499/1.454초, 후보 조립 0.847/0.758초, 리롤 후 RAM 반환 확인.
  기존 10조합 비교·HTTP API 4,687 통과와 격리 설정 검사를 반복할 필요는 없다.
  정식 설치·UI 업데이트·자료 보존은 최종 배포물로 확인한다. 문서 갱신에 유료 재현을 요구하지 않는다.
- 사용자 첨부에서 일반 기억 16,000자 중 직접 근거 3,086자와 다른 다섯 분류의 전달,
  사건/세계의 남은 공간 재사용을 확인했다. 역할별 usage와 최종 제공자 요청 전체는 미확인이다.
- **D의 큰 폭 의미 통합은 보류한다.** E는 실제로 반영된 변경의 결합과 영향 범위를 검증하며,
  D 구현을 4.4 완료 조건으로 요구하지 않는다. 새 버전 자동 배정도 하지 않는다.
- 완료 약속의 open 잔존·필드별 시점은 개별 문구 보정을 반복하지 않고 **4.6**의 진행 이력과
  기존 자료 복원으로 다룬다. 정상 저장된 완료 정보가 전달 중 사라지는 별도 재현 결함은
  해당 범위에서 고칠 수 있다. **4.5**는 출처·주체·범위·근거가 함께 읽히는 전달 단위를 맡는다.
- 삭제 후 기존 DB 번호 정리, OCI/간헐 지연 제보, 장시간 RAM·실제 비용, OS별 최종 업데이트
  검증은 현황 문서의 미해결/미검증 목록을 유지한다. 사용자에게 모든 내부 검증을 대신 맡기지 않는다.

이 갱신은 설계·상태 문서만 변경한다. 기능 구현·빌드·업로드·사용자 데이터 변경은 하지 않는다.

## 2026-09-12 — test.10 전처리 설정 공유 및 요청 HUD

[최신 작업 기록](archive-center-preprocessing-efficiency-20260911.md)에 따라 다른 담당의 연결·
생성 설정을 함께 따르는 선택 기능과 실제 묶음 요청의 동시 진행 HUD를 추가했다. 각 담당의
프롬프트/사용 여부/원래 개별 설정을 보존하며 기존 출판사 공유 의미도 유지한다. Ollama
DeepSeek의 추론 빈칸이 `none`으로 변하던 부분을 고치되 기존 출력 한도는 유지했다.

5개 담당을 LLM Gateway 2역할 / Ollama 3역할의 2개 동시 요청으로 처리하는 Go 생산 경로,
설정 저장·재열기·기준 담당 수정 전파·개별 설정 복원을 검증했다. Go 5,881 pass / 14 skip /
실패 0, 세 가지 대조 실패, 좁은 HUD와 설정 UI 검사를 통과했다. JS +71/-23은 UI/Host 표시다.
Test.10 Windows 패키지와 세부 증거는 위 기록을 따른다. 실제 RisuAI/제공자 검증은
`implemented_unverified`이며 공개 배포·기존 설치 교체는 하지 않았다.

턴 삭제/입력 그룹/리롤 소스는 이 작업에서 다시 바꾸지 않았다. test.9의 승인 범위 회귀검사는
통과했지만 임의의 중간 삭제와 고립 출력 정리까지 완료한 것은 아니다. D/E 및 나머지 4.4
작업을 완료 처리하거나 순서를 변경하지 않는다.

## 2026-09-12 — 다중 입력 턴 연결 및 전처리 묶음 호출 소스 반영

[최신 구현·검증 기록](archive-center-preprocessing-efficiency-20260911.md)에 따라 승인된 두 범위만
수정했다. 연속 입력 본문/개별 참조와 Go 턴 관측을 통일하고 기존 수락·다음 입력 저장·복구
소유자에 연결했다. 전처리는 같은 최종 연결/모델/옵션의 담당을 회차별로 묶고 동일 입력을
한 번 전달한다. 역할 프롬프트·기억 후보·원문·선정·빈 추천/부분 실패 의미는 유지한다.
묶음 출력 상한은 역할 한도 합계, 추론/timeout은 동일한 기존 요청 값으로 정리했다.

전체 Go 5,872 pass/14 skip/실패 0, 생산 JS 10개 관측 시나리오 및 네 가지 대조 실패를 확인했다.
실제 Host/제공자·사용자 DB·과금 검증 전이므로 `implemented_unverified`이다. 후속 요청에 따라
4.4.0-test.9 Windows 테스트 ZIP을 생성하고 관리 파일 58개·ZIP 파일 61개, 버전 표시,
패키지 JS·Go의 격리 설정 조회·저장·재열기 및 다중 입력 관측을 검증했다. 사용자 설치 교체와
공개 업로드는 하지 않았다. JS +122/-26, 이번 패키징 +0/-0.
이 작업으로 D/E나 4.4 전체를 완료 처리하지 않는다.
서로 다른 모델의 선행 담당/별도 공통 요약 호출, 새로운 삭제/리롤 시스템, 기존 DB 자동 재번호화는
적용하지 않았다. 기존 4.4 작업 순서와 4.3.1 기억 전달 기준은 유지한다.

## 2026-09-12 — test.7 실사용 절감 부족에 대한 후속 수정

[56턴 분석과 후속 계약](archive-center-preprocessing-efficiency-20260911.md): 전체 최근 대화의
2차 재전송과 동일 선정의 이유 재작성까지 보완한다. 각 담당이 1차에서 전체 N을 읽고 C 원문
구간을 골라 2차에 이어받으며, 2차가 명시한 이전 설명 재사용을 Go가 적용한다. 완전한 최종
선정 목록·AI 순서·원문/공개 범위·기존 실패/빈 추천 동작을 보존한다. 설정 N 자동 감소나
새 호출은 없다. test.7의 부분 절감을 전체 작업 완료로 해석하지 않는다. 새 모델의 실제
구간 선택·과금·기억 품질 검증은 후속 테스트에 남고 D/E 전체 완료와 구분한다.

## 2026-09-12 — 4.3.1 OCI 연동 중단 제보 보관

[제보 및 1차 확인 기록](archive-center-4.3.1-oci-prepare-stall-report.md): OCI의 PocketRisu(Docker)와
AC(systemd), Yumi·프로바이더 매니저 조합에서 4.3.1 업데이트 후 본문 요청 준비에 멈춘다는 제보다.
4.2.0/4.3.1 태그를 비교했지만 원인과 실제 중단 위치는 미확정이다. 브라우저 응답·첫 오류·최종 입력 관측의 추가 답변을 기다린다.
이번에는 문서만 기록했으며, 별도 test.7 최적화나 B/C 검증으로 이 제보가 해결됐다고 판단하지 않는다.

## 2026-09-12 — 전처리 효율 후속 검증 / test.7

[현재 작업 기록](archive-center-preprocessing-efficiency-20260911.md)의 52·53턴 분석을 반영했다.
Go 회수의 원문/질문 반복 해석을 요청 안에서 재사용하고, 모델 출처 범위 중복과 2차 이유 재전송을 줄였다.
1차 선정 원문은 2차 예산 재배분 전에 보존하며 로어북의 남는 공간 재보충만 생략한다.
다른 담당의 검색으로 찾은 주변 근거가 빠지는 후보 축소/순위 변경 실험은 제외했다.
기존 전체 검색 공유, 최종 추천 계약 및 Go 기본 선정은 유지한다. 차분 응답 계약은 구현하지 않았다.
전체 Go 5,842 통과/14 건너뜀/실패 0, 10조합 최종 주입 동등성과 실제 19개 모델 입력의 원문·범위 보존을 확인했다.
추가 질문 5개 조립 합성 시험은 2.319초 → 0.434초, 실제 기록 serializer는 전체 입력 문자 약 4–6% 감소다.
실제 모델/사용자 DB의 시간·비용·선정 품질은 test.7 사용자 검증에 남긴다. 이 결과로 D/E나 4.4 전체를 완료 처리하지 않는다.
test.7 managed 패키지 58개 파일 검증과 실제 패키지 JS·Go의 격리 설정 저장/재열기 및 진단 다운로드도 통과했다. 사용자 백엔드 교체나 공개 업로드는 하지 않았다.

## 2026-09-11 — 승인된 전처리 효율 개선 / test.6

[작업 기록](archive-center-preprocessing-efficiency-20260911.md): 질문별 반복 계산 및 정밀 근거 대조 색인,
요청 안의 공통 원문 준비, 기존 ID의 추가 질문 일치 보존, 2차 입력 집중 및 출처 형식 중복을 개선했다.
질의 `[]string` 누락 복구와 2차 후보 구성은 승인된 동작 수정이며 B/C의 순수 이동과 구분한다.
Go 5,838개 통과/14개 건너뜀/실패 0, 등록 준비 경로 10조합의 최종 기억·ID·출처·검색 한도 동등성을 확인했다.
실제 test.6 JS·Go의 설정/추론 옵션 저장·재열기 및 온라인/오프라인 진단 보고서 검증이 통과했다.
사용자 제공 10개 호출은 원문/범위를 보존하며 형식 중복만으로 입력 약 10.1%를 줄였다(문자 기준).
실제 RisuAI의 준비 시간/과금 토큰 및 새 2차 분석 결과는 사용자 테스트 대기다. D/E는 여전히 별도다.

## 2026-09-11 — B 고정 후 C 내부 경계 정리

후속 사용자 요청으로 [오류 로그·일괄 보고](archive-center-diagnostic-report-20260911.md)와
[전처리 모델별 추론 설정](archive-center-preprocessing-reasoning-20260911.md)을 추가했다.
아래 B/C의 당시 동등성 기록과 이 기능 변경을 구분한다. 전처리의 과거 effort-only 범위는
담당별 예산과 출판사 연결 추론 상속으로 확장됐으며, 모델 정책은 계속 Go가 소유한다.
이 보완으로 D 의미 중복 통합·E 실환경 검증을 완료한 것은 아니다.
추론 설정을 포함한 test.5의 전체 JS·Go 저장/재열기와 전체 Go 검사(5,830 통과/0 실패/13 건너뜀)를
통과했다. 사용자 세션 교체, RAM 누적·삭제 감지 피드백의 해결, 실제 provider 검증을 의미하지 않는다.

B의 초기 복원점은 `0dfd9bf`, 전체 설정 복원 문제까지 수정한 복원점은 `9dadb1b`다.
수정한 test.2의 전체 JS·실제 Go로 설정 조회/저장/새로고침/재열기를 확인했다.
사용자는 4.3 세션을 계속 사용하므로 실행 중인 호스트를 교체하지 않았다.
[C 작업·검증 기록](archive-center-4.4-c-assembly-hud.md)에 RF04 typed 조립 입력,
RF05 private hit 분리, RF06 미사용 최종 조립 생략, RF07 공통 스트림 I/O와 보존 범위를 정리했다.
RF08 SQL 보강은 A에서 완료했다. 조건부 DeleteSession 이동과 근거가 부족한 역할별 계산 최적화는 수행하지 않았다.
등록 준비 경로의 10조합을 변경 전후 비교하며, D/E와 실제 RisuAI 검증은 별도로 남긴다.
최종 Go 검사는 5,772 통과/0 실패/13 건너뜀이다. 로컬 test.3의 전체 JS·Go 설정 저장·재열기도
통과했으며 현재 4.3을 실행하는 사용자 환경에는 적용하지 않았다.

## 2026-09-10 — B의 확인된 공통 부분과 보존할 차이

[호출 경계 검증 기록](archive-center-4.4-b-provider-boundaries.md): override helper의 생산 호출부 11곳을 확인했다.
RF01은 동일한 Publisher/Critic/audit 추론 복사 세 곳만 분리했고, 전처리와 다른 reference 호출의
필드 범위는 확대하지 않았다. 실제 역할 요청 10개는 변경 전후 동일하다.
RF02는 겹친 UI 시험의 전역 설정 덮어쓰기를 재현해 request별 bridge 설정 전달로 수정했다.
RF02의 동일한 Publisher/Critic 제공자 옵션·추론 바인딩도 공통화했다. 전처리의 빈 제공자 선택과
독립 연결·effort-only 설정은 별도로 유지한다. RF03은 `POST /config/view-model` 읽기 전용 계약과
기존 proxy의 `reasoning_input` 관측값으로 이전했다. JS 모델 정책은 제거했다.
486개 변경 전 JS 표본의 화면/필드/실제 provider wire, 실제 폼의 로컬 브라우저 검증을 통과했다.
로드된 RisuAI 교체·실제 계정 검증은 수행하지 않았으므로 배포·실기 완료로 취급하지 않는다.

## 2026-09-10 — 4.4-A 실행 기록

[A 기준선·시험 보완 결과](archive-center-4.4-a-baseline.md)에 착수 HEAD/tag/dirty diff,
611개 파일 hash, 등록 prepare 경로 10조합의 입력/후보/provider 요청/최종 주입 캡처,
삭제/교체 SQL 독립 기대값과 세 오류 변형 검출 결과를 기록했다. 현재 Edge 설정은 읽기 승인 후
`4.3.1-test.4`로 확인했고 설정을 저장하거나 새 응답을 생성하지 않았다.
사용자의 33턴 HUD(4.3.0 표기, 준비 379초/백엔드 377초)는 별도 성능 관측이다.
보완 검색 74.5초 중 후보 조립 세 건이 약 74초이며, 세계·사물의 120초/82.6초 모델 대기와 구분한다.
RF06의 반복 후보 조립과 질의 어휘 추출을 후속 비교 대상으로 인계한다. 기억 폭·AI 추천·round 수는 변경하지 않았다.
`withUiBridgeSettings`의 임시 설정 복원 오류는 실제 helper를 VM에서 실행해 재현했다.
fallback vector race는 진단 harness를 준비했지만 CGO/C compiler 환경 제약으로 미검증이다.
실제 DB 쓰기·유료 provider 재호출·새 빌드/배포 없이 수행한 준비 작업이며 JavaScript +0/-0이다.

## 2026-09-10 — 4.3.1 기억 기준선 인계

[기본 기억 회수·후보 선정 복원](archive-center-memory-recall-restoration-plan.md)은 별도로 구현한
`implemented_unverified` 작업이다. 검색당 후보 확보량과 합친 결과의 재절단, 사실 후보 구성 순서,
어휘 회수, 요약 전체 오감점, 공개/전처리 후보 연결을 수정했다. 현재 기준 패키지는 정식 `v4.3.1`이다.
[배포 검증 기록](archive-center-4.3.1-release-verification.md)의 태그 소스 `0821d69`는 test.4의 기억 동작을 승계한다.
공개 배포와 기억 품질의 실제 세션별 검증은 별도로 판단한다.
사용자 결정으로 [4.3.1 보존 기준](archive-center-memory-recall-restoration-plan.md#memory-baseline-431)을 고정한다.
구버전 공통 재검사와 test.4의 긴 질의 조립·단어 순서 검사도 함께 인계한다.
이 의도된 동작 수정을 4.4-A~C의 동작 보존 리팩터링으로 섞지 않는다. 세 `memory_recall_restoration*_test.go`의
검색·후보·최종 주입, 10가지 옵션/실패 조합, 조회량 비교 자료를 4.4-A 기준으로 인계한다.
실제 RisuAI 결과는 별도 확인하며, 재현된 결함 자체를 보존할 정답으로 고정하지 않는다.
4.4 이후의 기능 순서는 유지한다. 이 문서의 4.4 구현 완료를 뜻하지 않는다.

### 4.6으로 인계하는 이야기 시간 과제

수개월·수년 전 사건을 “어제”로 표현하는 경험은 [4.6 이야기 시간 전달](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#story-time-delivery-46)에 기록했다.
사건 시점·기록 시점·현재 장면 시점과 확인 가능한 경과를 기본 기억 및 선택형 AI 입력에 전달하는
후속 기능이다. 4.4-A에서는 비교 사례로 남기며, 4.4-A~C의 동작 보존 리팩터링에 섞지 않는다.

## 1. 출발점과 버전 경계

- 계획 권위는 [통합 로드맵의 4.4](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)이며, 이 문서는 실행 항목을 구체화한다.
- 기억 동작의 고정 비교 출발점은 **정식 4.3.1(test.4 기억 동작 승계)**이다. 공개 `v4.3.0` / `51d901b`의 test.23·OpenCode Zen/Go·배포 결과는 별도 공개 기반으로 승계한다.
- [정식 인계표](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#stable-43-handoff)가 완료 기반/미래 기능을 구분한다. [배포 기록](archive-center-4.3.0-release-verification.md)의 패키지·업데이트 증거와 실제 RisuAI 기억 품질은 별개다.
- [test.12 기록](archive-center-4.3-test-build-12.md)의 병렬 보충검색·pristine 후보 재사용·검색 시간 분리는 이미 반영된 기준선이다.
- [현재 상태 요약](archive-center-4.3-status-summary.md)에서 구현 범위·잔여 검증과 착수 소스를 확인한다. 착수 시점이 바뀌어도 4.3.1 기억 기준과의 비교를 유지한다.
- [4.3 전처리 작업 기록](archive-center-4.3-preprocessing-work-log.md)과 test.7~23 및 정식 릴리스 기록은 근거 위치를 찾는 색인으로 사용한다.
- 실제 4.4 시작 시 HEAD, dirty diff, 적용 설정, 활성 package/loaded Host 식별을 다시 기록한다. 문서 작성 시점의 상태를 재사용하지 않는다.
- 4.4-A~C의 리팩터링은 같은 입력에 대한 기존 결과·부작용·실패 의미 보존이 목적이다.
- 4.4-D의 cross-surface 의미 통합은 **보류한 의도적 동작 변경**이다. 아래 설계는 이력으로 보존하며 구현 대기열이나 4.4 완료 조건으로 읽지 않는다.
- 4.4-E는 반영된 B/C와 후속 수정의 결합을 확인한다. 소스·패키지·사용자 관측·실제 제공자/OS 검증을 구분한다.

### 4.3.1이 승계한 4.3 정식의 기억 동작 (2026-09-08 수정 포함)

test.18부터 포함된 `priority_score.static.v4`의 의미 점수 전달·독립 중요도/최근성,
인물별 지식·공개 기록 연결, 하이파 원문 하나당 기억 하나의 가져오기, 224px HUD와
완료 카드의 펼치기/닫기는 4.3의 기존 동작이다. 4.4에서 새로 구현할 항목으로 세지 않는다.
test.21의 분류별 핵심 우선 수·남은 문자 예산 전달, 일반 주관 기억, 현재 필드 출처 판정과
보완 검색의 별도 증거 참조도 이미 반영된 기준선이다. 4.4-A는 이 동작과 예산·AI 추천 순서·
출처 범위를 기준 사례에 포함한다. 과거의 K 최대 개수 절단을 동등성 기준으로 복원하지 않는다.
4.4-C의 UI 정리는 현재 박스 배치·접기·이전 턴 저장 표시·클릭 동작을 보존한다.
각 수정의 파일/검증은 [4.3 현황](archive-center-4.3-status-summary.md)과
[test.21 기록](archive-center-4.3-test-build-21.md)을 따른다.
점수 보정은 상태의 실제 시점이나 표현이 다른 사실의 연결을 해결한 것으로 취급하지 않는다.

### 4.3 공개 기반과 4.3.1 보완의 4.4 배정

| 보존할 기능 | 4.4 담당과 검증할 동작 |
|---|---|
| 4.3.1의 검색 근거 합치기·후보 선구성·조사 회수·오감점 제거·조립 비용 개선 | A 기준, C 처리 동등성, D 동일 사실 통합 후 회수/원문 보존. 같은 자료에서 기본 기억과 편집자 경로를 함께 비교 |
| test.21의 기본 기억 폭·일반 주관 기억·출처/순위 분리·보완 검색 새 참조 | A 기준, C 내부 전달, D 동일 사실 비교. 기본 OFF에서도 원문·세부사항·시점/관점 유지 |
| test.22의 JSON 부분 활용·검색 질문 형식·담당별 순서·편집자 프롬프트·반복 출처 축소 | A의 저장 응답 재사용. C의 입력 정리 뒤 사실/요약별 순서, 사용자 편집 프롬프트 보존 |
| 다섯 편집자·선택형 Publisher, 1차 병렬→보충 검색 합류→필요한 2차 | B 호출 옵션, C 후보/렌더. 정상 추천·추천 없음·부분 실패·보완 실패와 전처리/출판사 네 조합 비교 |
| Endpoint·키·온도·토큰·추론/Flex·Vertex·Gemini medium·OpenCode Zen/Go·OpenRouter | B에서 같은 provider body/headers·UI 저장 의미 유지. 독립 연결/Publisher 공유를 구분 |
| 하이파 원문별 가져오기·긴 필드 migration 013·대량 삭제/벡터 정리 | A의 저장/삭제 기대값, E의 기존 자료·원문 보존. 같은 의미를 묶는 D는 DB 원문 병합이 아님 |
| test.23 pending 부모·보존/재발급 ID 분기·콜드스타트 상속/번역 제외 | A/E lifecycle. 미저장 응답 기억 생성과 부모 좌표 확인 구분, 일반 복사 독립성 유지 |
| 좁은 카드 HUD·완료 펼치기·현재 생성/이전 저장·실제 경과 시간·저장 통계 | C의 RF07은 공통 I/O만 정리. 사용하지 않는 기능과 미저장 턴의 빈 통계 비표시 유지 |
| 정식 OS 패키지·01/한 줄 신규 설치·UI 업데이트·개인 자료 보존 | E의 영향 범위별 회귀. 새 실행기나 설치 경로를 리팩터링 산출물로 추가하지 않음 |

4.3의 연속성 보완은 필드별 유효 시점이나 일반 관계 그래프의 완성을 뜻하지 않는다.
4.4는 반영된 효율·HUD·예산 수정을 검증하며 큰 폭의 의미 통합은 보류한다. 맥락 묶음은 4.5,
진행 이력과 기존 자료 복원을 포함한 실제 항목 시점/약속은 4.6, 검색 후보/순위는
4.7, 근거 관계와 실제 확장 회수는 4.8~4.9가 이어받는다. 5.x 이후 선택형 표현·연기는
기본 기억의 필수 조건으로 만들지 않는다.

2026-09-09 [출력 개선 재편](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#output-improvement-plan):
별도 AC Ensemble Agent Integrated 제작·연동은 폐기하고 Archive Center의
`추가 기능 → 출력 개선 → 끔 / 기본형 / 복합형`으로 계획한다. 4.4~6.0을 먼저 진행하고
6.1~7.0 공통 기반/기본형, 7.1~8.0 복합형, 8.1~9.0 Living World 순서를 유지한다.
4.4의 제공자·설정·조립 정리는 이후 내부 기능이 재사용할 기반이며, 이 버전의 작업 범위에
새 후처리 호출·배역 실행을 추가하지 않는다. 기존 bridge/workbench는 이번 문서 변경으로 삭제하지 않는다.

### 기본 회상 우선 목표와 공통 비교 자료

[통합 계획의 목표·버전 지도·여섯 비교 사례](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#good-memory-plan)를 따른다.
test.16은 분류별 공간과 남은 예산 공유를 복원했다. 큰 폭의 의미 중복 정리는 보류한다.
후속 4.5에서도 다른 사건·시점·관점의 차이와 수신 AI 추천을 보존하고,
중복량 감소와 기억 품질 개선을 따로 평가한다.

4.4-A는 오래된 중요 사건·작은 세부사항·유사 사건·상태 변화·다른 실마리·일반 RP의 비교 입력을
이어받는다. 전처리 OFF/ON × 출판사 OFF/ON, 추천 없음·담당 일부 실패·보완 실패를 같은 자료로
확인한다. 구조 정리는 기존 결과의 동등성을, 의미 통합은 필요한 사실의 회수·전달과 변경 목적을
각각 검증한다. 검색 후보·선정·실제 입력·출력 활용을 분리해 기록한다.

5.1-A~5.2의 공통 재활성화와 5.1-B·5.3~5.7의 선택형 인물 표현은 후속 계획이다. 4.4에 새
연상 엔진이나 망각 정책을 넣지 않으며, 선택 기능 OFF의 기본 회상을 이후에도 기준선으로 유지한다.

### 2026-09-07 추가 인계 — 상태의 시점과 검색 후보 혼재

주 계획은 [통합 로드맵의 4.4·4.6·4.7 공통 인계](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md#temporal-state-44647)다.
사용자 test.15 자료에서 같은 인물 기록의 현금 51냥 5푼과 자산 설명 속 61냥이 모두
115턴 후보로 공급됐고, 직전 대화에는 46냥 5푼이 있다. 누적 상태의 갱신 턴을 개별 항목에
전달하는 현재 소스와 연결되는 관측이다. 이전 시점 안내·프롬프트 보완이 이 문제를 해결한 것은 아니다.

- **4.4-A:** 변하지 않은 항목의 시점, 다른 필드에 남은 과거 금액, 직전 대화의 지출을
  구분하는 비교 사례를 준비한다. 원문·저장 행·항목 출처·점수·AI 입력의 현재 경로를 기록한다.
- **4.4-C:** 자료 전달을 정리하면서 시점의 의미를 임의로 바꾸지 않는다. 항목 시점 보존은
  별도 기능 변경이므로 동등성 리팩터링에 숨겨 넣지 않는다.
- **보류한 4.4-D:** 같은 사실의 중복 표현을 줄이려던 설계는 이력으로 유지한다. 시점이 다른 금액을 합치지 않는 비교 기준은 4.5/4.6에도 인계한다.
  최신 저장 revision만으로 현재 유효한 금액이 확정된 것으로 보지 않는다.
- **4.6 인계:** 현금·자산·재고까지 개별 변경·유효 시점을 보존하는 저장/후보 구성 수정과
  원문이 남은 기존 자료의 복원을 구분해 수행한다. 주 대상은 기존 상태 병합·저장·사실 분해 소유자다.
- **4.7 인계:** 바로잡힌 시점을 현재/과거 질의의 검색 후보·점수·AI 입력에 반영하고 각각 비교한다.

이는 **PLANNED** 범위다. 4.4 완료만으로 상태 혼재 해결을 선언하지 않으며, 4.3에
추가 프롬프트 조정·호출을 반복하는 작업으로 되돌리지 않는다. 모든 과거 자료의 완전 복원과
모든 모델의 시간 해석을 보장하는 목표는 아니다. 세부 사례와 증거 기준은 통합 문서를 따른다.

## 2. 모든 단계에서 보존할 계약

1. 사용자의 이야기 방향·수정·속도·선택이 우선이다. strength는 기존 표현 지침이며 수신 결과의 거부 조건이 아니다.
2. 수신한 유효 AI 추천의 원문·참조·순서를 유지한다. 추천 없는 영역은 기존 Go 선택을 사용하고 그 이유를 구분한다.
3. 부분 실패·보완 실패 시 기존 첫 추천 보존과 lorebook의 명시적 빈 선택/미평가 구분을 유지한다.
4. Go는 정책·선택·예산·조립·저장, JS는 Host 관측·전달·실제 payload 적용·표시 확인·UI를 소유한다.
5. MariaDB canonical source, Chroma 파생 검색, source revision·분기·시점·private owner/viewers·삭제/reroll 범위를 보존한다.
6. 새 거부 조건·2차 조건·retry·fallback·watcher·queue·병렬 정책 경로를 리팩터링 편의로 추가하지 않는다.
7. 검색 후보 수·분류별 핵심 우선 수·남은 문자 예산을 구분해 보존한다. 과거 최대 개수 절단을 복원하지 않는다. 진단 필드는 출력·저장 허용 여부를 결정하지 않는다.
8. 변경 비교에서 요구하는 동등성은 개발 검증 기준이다. 이를 runtime 출력 수락 조건으로 구현하지 않는다.
9. 파일 이동·공통화만을 위한 새 프레임워크나 서비스 계층을 만들지 않는다. 기존 함수·파일·ViewModel을 우선 정리한다.
10. [AGENTS](../AGENTS.md), [AI_GUARDRAILS](../AI_GUARDRAILS.md), [Host/Go 경계](permanent-risu-host-backend-boundary.md), [4.0 기억 계약](4.0-memory-restoration-work-contract.md)을 적용한다.

## 3. 단계·실행 순서

| 단계 | 목적 | 작업 | 완료 판단 |
| --- | --- | --- | --- |
| 4.4-A | 실제 기준선·검증 공백 확보 | 기존 결과 캡처, 테스트 실호출 여부, RF08의 기대값 독립성 검토 | 비교 입력/결과/부작용과 미검증 범위가 재현 가능 |
| 4.4-B | provider·설정 UI 중복 정리 | RF01 → RF02 → RF03 | 역할별 request와 UI 저장/시험 동작 동등 |
| 4.4-C | 기억 조립·내부 전달·HUD 정리 | RF04 → RF05, 측정 후 RF06, 독립 slice RF07 | source·추천·payload·진단 보존 및 변경 이유 확인 |
| 4.4-D | DEFERRED | 과거 의미 통합 설계 보존; 다른 버전으로 자동 이관하지 않음 | 4.4 필수 완료 조건에서 제외 |
| 4.4-E | 통합·실환경 확인 | B/C 및 실제 반영한 후속 수정, Host/provider·설치/업데이트 영향 비교 | 증거 단계별 결과와 잔여 한계 명시 |

RF08의 저장 함수 위치 정리는 A의 테스트 보강과 별개이며, 삭제 계약과 관련 검증이 준비된 뒤에만 작은 독립 작업으로 다룬다.
`beforeRequest`, `orchestrate`, final 저장/reroll은 초기 공통화 대상에서 제외하고 마지막에 실제 중복과 영향 범위가 확인된 항목만 다시 검토한다.

## 4. 4.4-A — 기준선·검증 공백

다음은 A 착수 때 사용한 기준이다. A 수행 결과는 위 실행 기록을 따르며, 새로 A/B/C를
처음부터 반복하라는 지시가 아니다. 변경 영향이 생긴 비교만 후속 E에서 갱신한다.

정식 [4.3.1 배포 식별 기록](archive-center-4.3.1-release-verification.md), 승계한
[test.4 검사 기록](archive-center-4.3.1-test-build-4.md)과 실제 착수 HEAD·dirty diff를 함께 기록한다. 공개 4.3.0은 이전 배포 비교, 과거 1.0/3.9.9/4.1/4.2/4.3은 특정 장점·퇴행 사례의
역사적 비교 자료다. 후속 버전은 4.3.1과 직전 검증 버전에 모두 비교한다. 다음 자료를 준비한 뒤 B/C를 시작한다.

| 비교 묶음 | 실행 자료와 판단 |
|---|---|
| 기본 기억 / 출판사만 / 편집자만 / 둘 다 | 같은 원문·입력·설정·문자 예산. 후보·선정 ID/순서·주입 원문·가이드의 역할을 분리 |
| 정상·빈 추천·일부 실패·보완 실패·로어북 빈 선택 | 기존 실제 처리 함수에 저장 응답을 재생해 fallback과 부분 결과 사용 비교 |
| 검색 경쟁·한국어 조사·혼합 공개 근거·정리 힌트·긴 질의 | 4.3.1의 세 회수 회귀 파일과 test.4 검사 재사용. 검색 근거→사실 후보→최종 전달을 비교하고 비용 개선에 따른 후보 축소 여부 확인 |
| 시점·관점·작은 사실·유사 거래 | 유지할 사실·출처·시점 차이를 독립 기대값으로 구분. 보류한 D와 후속 4.5/4.6/4.7의 과제는 별도 표시 |
| 같은 턴 재생성 / 새 턴 진행 | 직전 실패/리롤 결과가 DB에 남은 여부와 현재/이전 저장 모드를 기록. 다른 시도끼리 오염된 입력을 동일 조건으로 간주하지 않음 |
| 설정·공급자·lifecycle | 사용자 프롬프트/키·Endpoint, 요청 body/headers, 변경/삭제/분기·cold-start의 기존 결과 보존 |

외부 AI를 다시 부르기 전에 저장 응답·경계 fixture로 처리 경로를 검증한다. 후보나 프롬프트가
바뀐 경우 이전 녹화 응답의 성공은 새 모델 판단의 증거가 아니다. 실제 모델/Host 검증은
변경된 기능의 대표 사례에 집중하고 호출 범위·목적·비용을 별도로 기록한다.
동일 원문 전달이 동일한 창작 문장을 보장하지는 않으므로 최종 출력은 사실 활용과 사용자 지시를 함께 본다.

아래 줄 번호는 최초 소스 조사 시점의 위치 참고다. 현재 파일·함수·호출자가 식별 기준이며,
4.4 착수 때 이동된 위치를 다시 확인한다. 줄 번호가 달라졌다는 이유로 기능이나 시험을 새로 만들지 않는다.

- 활성 `Archive Center.js`·`go-service`에서 경로와 파일을 확인하고 기존 dirty 변경의 작성자를 구분한다. 정리 목적으로 기존 변경을 되돌리지 않는다.
- 동일 입력·설정에서 provider request, 선택 ID/순서/원문, final memory text, payload plan, source lineage, 오류 코드·호출 횟수를 캡처한다.
- OFF, 독립/공유 connection, 빈 추천, 일부 역할 실패, 보완 실패, private/lorebook, auto/custom 예산 사례를 포함한다.
- 테스트를 source-string 검사, 실제 JS/Go 함수 실행, 등록 API+경계 fixture, 실제 DB/provider, loaded Host로 구분한다.
- `main_settings_loading_43_test.go:10`처럼 이벤트 연결을 stub한 테스트를 저장·시험 버튼 실행 증거로 사용하지 않는다.
- [vector/fake.go](../go-service/internal/vector/fake.go#L38)의 실제 `fakeVectorStore`는 `Search()`·`Health()`·`Count()`에서 공유 호출 기록 필드를 변경한다(38/105/117).
- [mutation_fence.go](../go-service/internal/vector/mutation_fence.go#L49)의 이 세 조회는 `RLock`을 사용하므로 조회 사이의 기록 변경은 서로 직렬화되지 않는다(49/73/79). 동시성 문제의 확정 전 상태는 `SUPPORTED_RISK`이다.
- **생산 연결:** [config.go:385](../go-service/internal/config/config.go#L385)의 기본 mode와 `VectorPolicySatisfied()`(514)는 core_lite fallback/off를 지원한다. [NewServer](../go-service/internal/httpapi/server.go#L91)는 비활성 Chroma/빈 endpoint 분기와 Chroma 생성 오류 분기에서 `NewFakeVectorStore()`를 선택하고 105~106에서 mutation fence로 감싼다.
- 실제 설정·startup 검증 결과에 따른 접근 가능성을 구분하고, 이 생산 대체 store의 동시 조회를 재현한다. 재현된 공유 기록 변경만 `vector/fake.go`의 기존 owner에서 좁게 수정하며 `Search` 무결과·Health 상태·Count 의미·기존 startup 오류를 유지한다.
- `vector/vector_test.go`·`vector/mutation_fence_test.go`의 실제 delegate/wrapper와 `config/config_test.go`의 core_lite/vector profile 사례를 사용한다. HTTP 테스트 fixture인 `group_memory_part03_test.go`를 수정 대상으로 대신 잡지 않는다.
- 이 위험을 운영 더미 기억 오염으로 단정하지 않는다. 실제 메모리 오염이나 서비스 실패는 별도 재현 증거가 있어야 하며 새 fallback이나 출력 거부 조건을 추가하지 않는다.
- 원래 body·호출 횟수·SQL 기대값과 독립적인 실패 검출을 확보한다. production 함수를 복제해 기대값을 만드는 테스트는 보강한다.
- 정식 배포 기록의 소스/CI/패키지/실제 Windows 업데이트 증거를 유지한다. loaded RisuAI·실제 provider 품질·모든 native 기기·race는 각각 최신 실행 기록으로 판정하고, 예전 미확인 목록을 일괄 재사용하지 않는다.
- 아래 JS `main_*_test.go`의 위치는 `go-service/cmd/js-route-variant-smoke/`이다.
- Go HTTP 테스트는 `go-service/internal/httpapi/`, SQL 테스트는 `go-service/internal/store/`의 활성 파일을 사용한다.
- 알려진 검사 목록이 모두 통과한다는 가정은 하지 않는다. 실제 4.4 시작 시 기존 실패와 새 실패를 구분한다.
- 변경 전후 비교 입력과 기대 결과는 한 쌍으로 보존하고, 이후 의미 통합 사례의 기대값과 섞지 않는다.

## 5. 4.4-B — provider·설정 UI

### RF01 — 기존 LLM 설정→proxy 요청 매핑 공통화

- **현재:** [turn_extraction.go:884](../go-service/internal/httpapi/turn_extraction.go#L884)의 `applyProxyOverridesFromLLMConfig()`가 extra headers/body·Flex·tier·cache를 전달한다.
- **대상:** [group_proxy.go:294](../go-service/internal/httpapi/group_proxy.go#L294), [turn_extraction_critic.go:587](../go-service/internal/httpapi/turn_extraction_critic.go#L587) 및 867의 반복 reasoning 매핑.
- **확인 후 구현:** 기존 override helper는 11곳이 사용하고 필드 범위가 서로 다르다. 동일한 추론 복사만 인접한 `applyProxyReasoningFromLLMConfig`로 분리해 세 호출부의 중복을 제거했다. 기존 override helper는 확대하지 않았다. 역할별 message/purpose·token 기본값·timeout·retry budget은 호출 owner에 남긴다.
- `prepare_turn_multi_agent.go::callMultiAgent()`는 기존 helper를 사용한다. 독립 연결과 Publisher 공유 연결의 서로 다른 기본값을 유지한 상태에서 공통 필드만 연결한다.
- **호출 보존:** Publisher 단일 요청, Critic 본 추출/세계규칙 audit, specialist 양 round 모두 기존 `performProxy...` 경로를 사용한다.
- **검증:** `group_proxy_test.go`, `group_proxy_part02_test.go`, `prepare_turn_multi_agent_test.go`의 실제 요청 body/headers를 비교한다.
- 빈 값·미지정·명시적 0, temperature/token 값, service tier·Vertex Flex·Claude cache·extra overrides, 오류/횟수/취소를 포함한다.
- OpenCode Zen과 Go는 별도 제공자다. 기존 model-native 경로와 명시적 Endpoint 우선, OpenRouter 기본값, 비-Claude Messages의 JSON 요청 형식을 보존한다.
- OpenCode Go의 같은 대화 세션 헤더를 Publisher·Critic·편집자 두 round·직접 프록시에서 유지한다. 사용자 extra headers 우선과 Archive Center 자체 클라이언트 표기를 보존하며 provider 요청 식별자를 기억/턴 identity로 옮기지 않는다.
- 정식 제공자 회귀는 실제 요청을 받는 HTTP fixture 증거다. 이 결과를 모든 모델·구독 계정의 실제 수락으로 보고하지 않는다.
- helper 단위 검사만으로 끝내지 않고 `TestProxyReasoningContractIsSharedByPublisherAndCritic`과 등록 config/connection-test 경로를 확인한다.

### RF02 — provider 설정 폼·이벤트의 반복 표현 정리

- **대상:** [Archive Center.js](../Archive%20Center.js)의 provider 옵션(84), 전처리 폼(51582), Publisher(52011), Critic(52133), `renderSettingsPanel()`(51742)·`attachSettingsEvents()` 연결(52679).
- **방법:** 실제로 같은 옵션·필드 표현만 기존 UI 생성/바인딩 방식으로 묶고 각 역할의 저장 키·DOM ID·기본값·변경 이벤트를 명시적으로 연결한다.
- 전처리와 일반 설정의 Flex·추론 표시 차이는 실제 Go 전송/설정 의미와 대조한다. 모양이 비슷하다는 이유로 표시 범위를 통합하지 않는다. 저장된 백엔드 주소를 그대로 사용한다.
- **검증:** `ops/preprocessing-ui-smoke.cjs`와 실제 `renderSettingsPanel()`/이벤트 함수를 실행한다. desktop/mobile, 편집 중 재렌더, 복원·저장·재열기·명시적 빈 값·숨겨진 값 보존을 확인한다.
- `main_settings_loading_43_test.go`의 loading 검사는 유지하되 버튼 이벤트 stub을 저장/connection-test 검증으로 계산하지 않는다.
- **재현 후 수정:** 기존 `withUiBridgeSettings()`는 async 작업 중 전역 `settings`를 임시 변경했다. 겹친 시험과 일반 요청의 주소/timeout 영향을 실제 버튼·transport를 실행한 로컬 회귀로 확인했다. 현재는 폼 값의 request별 객체를 기존 `bridgeFetch`에 전달한다.
- 동일한 Publisher/Critic 옵션 목록은 `renderLlmProviderOptions`, 추론 바인딩은 `bindLlmSettingsView`와 명시적인 두 role prefix로 묶었다. DOM ID·저장 키·기본값·제공자 순서는 유지한다. 전처리의 다른 목록/필드 의미는 합치지 않았다.
- 실제 폼·저장/복원/시험 이벤트를 격리된 desktop/mobile 브라우저에서 실행했다. 미저장 조회, 역순 응답, 조회 중 새 편집, 실패 시 입력 보존, 숨긴 Flex 값, 빈 Endpoint/API 키, 온도 0·토큰·역할 분리를 확인했다. 저장소/HTTP는 fixture 경계이며 loaded RisuAI의 전체 저장/재열기 증거는 별도다.
- 이 확인을 이유로 전역 설정 체계·요청 queue·새 bridge fallback을 재설계하지 않는다.

### RF03 — reasoning UI와 Go 정책의 경계 정리

- **이전 대상:** JS `detectReasoningFamily`, `resolveReasoningTransport`, `resolveReasoningControls`는 제거했다. `applyReasoningFieldsToPayload`는 모델 정책 없이 사용자 입력만 관측한다.
- **Go owner:** [llm_settings_view.go](../go-service/internal/httpapi/llm_settings_view.go)의 표시/관측값 변환과 기존 `proxy_provider.go`의 transport·family·모델별 요청 변환. 표시 문구/기본값은 `llm_settings_presentation.go`.
- **구현:** config trace와 config update는 조회 수단이 아니므로, 미저장 입력을 받는 읽기 전용 `POST /config/view-model` 계약을 추가했다. 저장소·runtime config·외부 provider를 사용하지 않는다. 모델 전용 reasoning endpoint는 만들지 않았다. JS는 응답의 선택지/라벨/다음 표시값을 적용하며 기존 모델 정책 사본을 제거했다.
- 본 호출과 연결 시험은 기존 `/proxy/plugin-main`에 `reasoning_input` 관측값을 보낸다. Go가 기존 flat DTO 필드로 확장하므로 본 호출 전 ViewModel 조회나 LLM 호출이 늘지 않는다. 기존 Go 역할 요청·flat HTTP 요청은 그대로다. paired JS/backend가 필요하며 새 배포는 E에서 검증한다.
- Gemini 3.8 Flash medium, native/gateway none 차이, GLM toggle/effort/Ollama 범위, Claude budget/adaptive, DeepSeek endpoint 차이를 486개 변경 전 실행 표본과 실제 요청으로 확인했다. 인식이 넓은 Go custom-GLM transport를 UI 추론 토글로 확대하지 않았다.
- 기존 동작이 서로 다르면 먼저 어떤 실제 경로가 어느 값을 보내는지 기록한다. 지원 모델·값·기존 오류 의미를 임의로 통일하지 않는다.
- **검증:** `ops/settings-view-ui-smoke.cjs`의 실제 설정/시험 버튼, `llm_settings_view_test.go`의 변경 전 486개 표시/필드/wire 표본과 Critic 시험 cap, `group_proxy_test.go`의 기존 wire/family/공유 reasoning 계약을 함께 확인했다.
- 정상 요청 body·UI 선택 가능 값·미저장 편집·backend 오류 표시를 비교했다. RF01~RF03 소스와 로컬 검증은 완료했지만 실제 RisuAI/계정 경계는 `implemented_unverified`다. C~E로 자동 확대하거나 새 패키지를 배포하지 않는다.

## 6. 4.4-C — 기억 조립·내부 결과·HUD

### RF04 — 23개 위치 인자와 perspective 내부 전달 정리

- **대상:** [prepare_turn_assembly.go:59](../go-service/internal/httpapi/prepare_turn_assembly.go#L59)의 `buildPrepareTurnInjectionAssemblyWithBudget()`와 `group_turn_prepare.go:1022/1062` 호출.
- **방법:** 기존 입력을 역할이 드러나는 request-local 내부 입력 구조로 치환한다. 23개 인자와 `assemblyPerspectiveContext`의 내부 정책·질의·semantic facts 전달을 함께 명시한다.
- `prepare_turn_assembly.go:1030~1044`의 재포장과 `prepare_turn_priority_memory.go:1804~1811`의 문자열 key 해석을 순서대로 정리한다.
- 외부 request DTO·공개 perspective shape·Store interface는 이 작업만으로 바꾸지 않는다. 관측과 내부 정책을 같은 필드로 재분류하지 않는다.
- **호출 보존:** `/prepare-turn` → 기본 조립 → 선택적 `runMultiAgent()` → 기존 priority plan → Publisher/payload.
- **검증:** `prepare_turn_priority_memory_test.go`의 생산 조립·분류별 핵심 우선 수·남은 예산·auto/custom·private metadata, `prepare_turn_candidate_pool_test.go`의 pristine/deep-copy/JSON 비노출.
- `group_turn_perf_test.go:466` 현재 logical turn 이전 generation 제외와 `group_turn_part14_test.go:83` confirmed worldline 범위를 유지한다.

### RF05 — vector 내부 hit 전달과 공개 trace 구분

- **대상:** [prepare_turn_recall.go:140](../go-service/internal/httpapi/prepare_turn_recall.go#L140)의 vector shadow 생성, 427의 private precise hit, 530의 hydration, handler 572/1060의 삭제.
- **방법:** 기존 검색 결과의 내부 hit 전달을 공개 진단 맵과 구분한다. 같은 검색 owner 안에서 타입·반환 경계를 정리해 수동 private key 삭제 의존을 줄인다.
- broad·aggregate Memory·precise 검색을 합치거나 추가하지 않는다. 각 상태/결과 소유권, canonical hydration, 검색 필터·횟수·부분 실패를 유지한다.
- aggregate hydration은 이미 자신의 `memory_search_result`를 읽는다(2501). 과거 교차 상태 결함의 재수정 항목으로 등록하지 않는다.
- **검증:** `prepare_turn_priority_memory_test.go:1217`의 등록 HTTP→precise score→final plan 및 private hit JSON 비노출을 사용한다.
- aggregate/broad의 서로 다른 성공·실패, wrong-session·stale/current-turn source, private scope, OFF와 무결과를 기존 생산 owner로 비교한다.
- 공개 trace의 필드·스크러빙·카운트는 기존 소비자와 대조한다. 진단 오류를 출력 거부 조건으로 바꾸지 않는다.

### RF06 — 측정 후 보충 조립·역할별 입력 반복 처리 축소

2026-09-10 관찰 기록: 업데이트 뒤 전처리가 약 2분에서 5분으로 늘었다는 사용자 제보가 있었다.
제보자의 정확한 버전·입력량·단계별 시간이 없어 원인은 미확정이다. 사용자 지시에 따라 점검
항목으로만 남기며, 즉시 수정하거나 4.4 작업 순서를 앞당기지 않는다. 기존 A의 측정과 C의
RF06에서 AI 호출 시간·추가 검색·조립/대기를 구분해 확인한다. 이 기록은 해결 판정이 아니다.

- **현재:** `group_turn_prepare.go:1062~1066`은 보충검색에서 전체 assembly를 만든 뒤 pristine facts/summaries만 소비한다.
- **대상:** `prepare_turn_assembly.go:1046~1222`의 최종 plan/표시·진단 단계와 `prepare_turn_multi_agent.go:643~720/920~941`의 전체 pool 정렬·길이·recent 대화 반복 계산.
- **방법:** 기존 타이밍/benchmark로 비중을 확인한 뒤 후보 생성에 필요한 처리와 미사용 최종 표시 작업의 경계를 같은 owner 안에서 정리한다.
- request 안에서 불변인 계산만 재사용한다. 후보를 임의로 줄이거나 영구 cache·새 검색·선택 경로를 만들지 않는다.
- test.12의 후보 재해결 제거·검색 병렬화는 다시 구현하지 않는다. request-local mutex 제거를 성능 목표로 삼지 않는다.
- **검증:** `prepare_turn_supplement_search_route_test.go:84`, `prepare_turn_multi_agent_search_test.go:17`, `prepare_turn_multi_agent_input_test.go:16/52/132/178`.
- 모든 검색 합류 후 round two, 역할 순서의 병합과 test.22의 담당별 추천 순서, 별도 summary 순서, F/S/L alias, 전체 항목 shared cap, AI 원문·빈 추천·부분 실패를 비교한다. 추가 검색의 다른 출처/값이 같은 필드라는 이유로 사라지지 않아야 한다.
- `prepare_turn_candidate_pool_test.go`의 benchmark는 측정 도구로 사용한다. 실제 지연 개선량은 측정 전 수치나 완료 기준으로 주장하지 않는다.

### RF07 — HUD 스트림의 공통 I/O만 정리

- **대상:** JS current/previous 상태(14521~14536), cancel(15835/15851), NDJSON line 소비(15909/15955), reader loop(15933/15982), start(16099/16187).
- **방법:** 이미 같은 읽기·줄 분할·decode·reader 종료 처리만 공통화한다. current와 previous의 request ID·watch token·abort controller·카드 생명주기는 분리해 보존한다.
- `applyTurnWorkflowHUDStack()`과 기존 timer/event stream을 사용한다. 새 watcher·수신 수락 규칙·저장 판단을 추가하지 않는다.
- 정식의 224px HUD·박스/펼치기·완료 카드 클릭·저장 없는 턴의 통계 비표시를 유지한다. wall time·역할 호출 합계를 섞지 않고, 현재 생성/이전 저장의 카드/시간/건수도 분리한다.
- **검증:** `main_part11_test.go:795`, `main_priority_memory_42_test.go:319`, `main_hud_timing_43_test.go:11/143` 및 `turn_workflow_hud_test.go`.
- 잘린 NDJSON·다중 chunk·취소·오래된 request·current/previous 동시 표시·OFF·timer 종료를 실제 함수로 확인한다.
- `main_part12_test.go:1634/1990/2175`의 hook/persistence 경계도 유지한다. HUD 편의를 위해 request identity를 합치지 않는다.

### RF08 — SQL 회귀 기대값 독립성과 조건부 파일 위치 정리

- **테스트 대상:** [mariadb_logical_turn_replace_test.go:203](../go-service/internal/store/mariadb_logical_turn_replace_test.go#L203)의 `TestMariaDBRollbackCanonicalTailIsAtomicAndIdempotent`.
- 221은 생산 `canonicalTailDeleteCommands()`의 개수로 기대 호출을 만들고, 222는 임의 SQL 정규식으로 받는다. 빠진 대상·잘못된 SQL/인자를 놓칠 수 있는 검증 공백이다.
- **방법:** 필요한 삭제 대상·SQL·인자·순서를 독립적으로 명시한다. 생산 목록에서 명령이 빠지거나 대상/범위가 바뀌면 테스트가 실패해야 한다.
- [mariadb_rollback_test.go:163](../go-service/internal/store/mariadb_rollback_test.go#L163)의 `TestMariaDBDeleteSession`은 이미 SQL을 명시한다. 두 테스트의 보강 이유를 구분한다.
- **조건부 runtime 위치 대상:** [mariadb_status.go:1186](../go-service/internal/store/mariadb_status.go#L1186)의 `DeleteSession()`을 기존 [mariadb_delete.go](../go-service/internal/store/mariadb_delete.go)로 옮기는 것은 삭제 owner 정리의 필요가 확인될 때만 별도 slice로 수행한다.
- signature·SQL 순서·transaction·revision tombstone·vector cleanup handoff를 그대로 유지한다. 새 삭제 helper/framework나 삭제 범위 확대를 만들지 않는다.
- **검증:** `mariadb_logical_turn_replace_test.go`의 원자성/재실행(203), 삭제 실패 rollback(241), lifecycle history(271), hierarchy 범위(312)를 생산 owner로 확인한다.
- 조건부 `DeleteSession()` 이동은 `mariadb_rollback_test.go`의 정상/실패 rollback(163/269/293)과 실제 Store/route의 relational rollback·vector outbox 연결을 별도로 확인한다.
- 이 계획은 실제 데이터 삭제·초기화 실행 허가가 아니다. 실제 저장소 검증은 격리된 fixture와 해당 실행 승인의 범위를 구분한다.

## 7. 보류한 4.4-D — cross-surface 의미 중복 통합 설계 이력

**2026-09-14: DEFERRED.** 아래 항목은 과거 설계의 보존이며 현재 실행 지시가 아니다.
4.4 필수 완료 조건에서 제외한다. 4.5의 출처 기반 맥락 구성과 구분하며, 이 통합을
다른 버전에 자동 배정하거나 이미 구현됐다고 가정하지 않는다.

- **동작 계약:** Memory·KG·상태·관계·thread·근거의 같은 claim/event를 delivery family로 묶고 대표 문장과 support/source refs를 구분한다.
- 같은 사실의 surface 점수를 단순 합산하지 않는다. 대표 점수와 통합 전·후 점수/source coverage의 연결을 기록한다.
- 대표 표현은 source authority·최신 revision·구체성·부정·방향을 보존한다. 다른 약속·회차·시점·관점·충돌은 유지한다.
- `merged`, `retained_conflict`, `not_equivalent`를 전달 lineage로 설명한다. prompt에서 선택되지 않았다는 사실을 canonical 삭제로 해석하지 않는다.
- **예정 owner:** `prepare_turn_priority_memory.go`의 source identity/사실 후보·점수, `prepare_turn_memory.go`의 출처 occurrence, `prepare_turn_memory_budget.go`의 최종 예산, `output_fidelity_lineage.go`의 관측 연결.
- `prepare_turn_assembly.go`·`prepare_turn_render.go`는 그 결과를 기존 최종 plan/payload로 전달한다. Critic canonical writer나 Chroma를 의미 통합 저장소로 바꾸지 않는다.
- **4.4의 새 family 접점은 후속 계약 과제:** 4.3 편집자 연결 자체는 구현됐다. 새 통합을 이미 수신한 AI 추천 뒤에 적용해 문장·순서를 조용히 대체하지 않는다.
- family/구성 원문을 AI 선택 전에 제시하는 방향으로 계약을 구체화한다. canonical member/source ref·F/S/L alias·추천 순서·원문·추천 없음의 Go 선택이 기존 후보와 어떻게 대응하는지 먼저 명시한다.
- 이 후보 입력 변경은 RF04/RF06의 동등성 리팩터링에 숨겨 넣지 않는다. 상세 계약/consumer 검증 전에는 구현된 기능이나 확정된 DTO로 기재하지 않는다.
- **검증:** 기존 priority/기억 예산/lineage 생산 함수 테스트에 같은 사건의 다중 surface와 별개 사건의 유사 문장을 대조하는 사례를 추가한다.
- 부정·역방향 관계·다른 source occurrence·시점·owner/viewer·충돌·AI가 명시적으로 고른 서로 다른 기억을 함께 검사한다.
- 같은 입력·예산에서 문자/token·사실 수·source coverage·필요 사실 recall을 통합 전후 비교한다. 중복 감소만으로 성공 판정하지 않는다.
- source-linked bundle 압축은 4.5 범위다. 이후 typed relation·local-graph 작업의 배정은 통합 로드맵 4.8–4.9를 따른다. 리팩터링에서 관계 점수 전파를 새로 구현하지 않는다.

## 8. 4.4-E — 통합 검증·보고

1. 각 RF와 후속 수정의 diff를 확인하고 B/C의 기존 출력 동등성과 test.16까지의 승인된 동작 변경을 구분한다. 보류한 D의 구현 완료를 요구하지 않는다.
2. 실제 production 함수를 호출하는 Go/JS 회귀를 먼저 확인하고 등록 API의 provider/Store 경계 fixture로 호출·payload를 연결한다.
3. JS 변경이 있으면 활성 source에서 `node --check "Archive Center.js"`를 수행한다. 이는 문법 증거이며 Host 실행 증거가 아니다.
4. source/regression, package, loaded RisuAI, MariaDB/Chroma, 실제 provider, payload-applied, displayed-final을 각각 기록한다.
5. 해당 변경의 실환경 검증이 남으면 `implemented_unverified`로 보고한다. 4.3 정식 공개 사실과 새로운 4.4 기능의 검증 상태를 구분하며 테스트 성공만으로 일반적인 기억 품질 완료를 선언하지 않는다.
6. 파일·함수별 변경 이유, JS 추가/삭제 줄 수, 검사 실행/미실행, 성능 측정의 입력·범위·한계와 남은 항목을 기록한다.
7. architecture·ownership·contract·hook order·저장·검색·fallback 의미가 바뀌는 구현 slice는 `STRUCTURE.md`와 `AI_GUARDRAILS.md`를 함께 갱신한다.

패키지 확인에는 기존 [Windows 빌더](../ops/build-full-package.ps1)·[POSIX 빌더](../ops/build-posix-managed-packages.ps1),
[Windows 설치](../install-windows.ps1)·[POSIX 설치](../install.sh)와 OS별 실행기를 사용한다.
Windows의 `01` 시작 파일과 Linux·macOS·Termux의 기존 한 줄 설치/실행 경로,
업데이트 뒤 DB·키·프롬프트·역할 설정 보존을 해당 환경별로 기록한다. 신규 설치 두 진입점과 기존 설치의 별도 업데이트를 유지한다. 공개 4.3 배포 기록의 증거 범위를 이어받고 4.4 변경 영향만 추가 검증한다. 교차 빌드·Windows 시험만으로
모든 OS 동작을 완료 처리하지 않는다. 설치기 수정은 리팩터링 자체의 필수 산출물이 아니며,
확인된 패키지 영향이 있을 때 기존 소유자를 수정한다.

대형 hook·`orchestrate`·저장/reroll 정리는 위 결과로 실제 필요한 범위를 입증한 뒤 후속으로 결정한다.
legacy 전체 삭제, 기존 acceptance 강화, 새로운 자동 복구 경로는 이 계획의 실행 방법에 포함하지 않는다.

## 9. 이번 문서 작업의 증거 수준

- 2026-09-09: 공개 `v4.3.0`/소스 `51d901b`와 정식 현황·배포 기록을 기준으로 4.4 이후 구성을 갱신했다. 활성 소스의 점수/전달 계약과 주요 조립·provider helper·UI 함수 위치도 확인했다.
- 통합 로드맵이 버전 소유권 정본이며 이 문서는 4.4의 파일별 실행 상세다. 4.3 전체 기반과 미래 계획을 분리하고, 녹화 응답/실제 호출·네 조합 비교와 배포 보존을 반영했다.
- 이번 정렬은 문서 변경뿐이다. 코드 변경·테스트 실행·빌드·새 유료 AI 호출·프로세스 조작은 없다.

- 2026-09-07 활성 소스의 위치·호출 관계·기존 테스트 내용을 읽어 계획에 반영했다.
- 구조적 중복과 결합은 소스 근거이며, `withUiBridgeSettings`·생산 대체 `vector/fake.go`의 조회 기록 동시성은 재현 전 `SUPPORTED_RISK`이다.
- 이 문서 작성에서는 runtime·테스트·설정·프로세스를 변경하거나 테스트·빌드·패키지·live 호출을 실행하지 않았다.
- 2026-09-07 문서 작성 당시 4.4 구현은 전부 `PLANNED`였다. 현재 상태는 문서 상단의 2026-09-14 갱신을 따른다.
