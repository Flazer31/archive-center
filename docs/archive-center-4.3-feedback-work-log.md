# 4.3 선행 GitHub 피드백 수정

## 2026-09-09 — test.23 재분기·콜드스타트 수정

4.2 제보를 바탕으로 만든 격리 재현에서, 부모의 분기 원본 source revision이 저장되기
전에는 부모 후보를 찾고도 상속 범위를 열지 못했다. 또한 콜드스타트의 두 번째 병합이
Go의 상속 제외 결정을 무시하고, 정리 전 번역 표시문을 다시 덮어쓰는 결함을 확인했다.

Go의 기존 계보 판정에서 이미 관측한 부모 메시지 순서와 저장 턴/상속 경계의 대응을
사용하도록 수정했다. 부모 원본이 저장 대기 중이어도 기존 부모·조상 기억의 상속 범위를
확인한다. 미저장 응답의 기억을 새로 만들거나 부모 평론가를 강제 호출하는 작업은 아니다.
콜드스타트는 두 병합 모두 Go의 제외 결과를 따르고, 같은 원문 정리 함수를 적용한다.

수정 전 실패한 자료를 다시 실행하고 보존 ID·재발급 ID, 사용자/응답 분기점, 저장된
턴 번호의 차이, 다단계 재분기를 검사했다. [test.23 기록](archive-center-4.3-test-build-23.md)에
정확한 시험·패키지 결과를 남긴다. 실제 제보자의 과거 DB를 복원한 검증은 아니며,
사용자 환경의 최종 상태는 `implemented_unverified`다.

## 2026-09-08 — test.21 기본 기억 전달 보완

2026-09-08 [test.21](archive-center-4.3-test-build-21.md): 기본 기억 전달 폭과 주관 기억의
예산 경합, 현재 값 판정, 전처리 추가 검색의 출처 충돌을 한 묶음으로 수정했다.
분류별 개수는 핵심 우선 수로 사용하고, 남은 문자 예산에는 관련 세부사항을 담는다.
일반 주관 기억은 보유자·출처 턴과 함께 정상 선정에 참여한다. 현재 필드의 최신 출처
판정과 검색 순위를 분리하고, 보완 검색의 새 출처/값에는 별도 후보 참조를 부여한다.
AI 추천의 원문·순서와 추천 없음/부분 실패의 Go 기본 선정은 유지한다.
전체 Go 35개 패키지, 고정 자료의 OFF·AI·추천 없음·일부 실패, 새 패키지 검증을 통과했다.
실제 RisuAI의 출력 품질은 사용자 테스트 단계이며 백엔드는 자동 실행하지 않았다.

아래 선행 작업은 각 당시 빌드의 기록이다.

현재 기준(2026-09-08): 아래 기억·하이파 수정은 [4.3.0-test.18](archive-center-4.3-test-build-18.md)에
포함했다. [전체 현황](archive-center-4.3-status-summary.md)에서 전처리·HUD 작업도 확인한다.
공개 릴리스 기록은 4.2.0이며, 날짜별 검사 결과는 그때 확인한 범위를 보존한다.
상태는 `implemented_unverified`: 소스·패키지 확인과 실제 사용자 환경의 검증을 구분한다.

<a id="retrieval-score-retention-repair"></a>
## 2026-09-08 — 의미 검색 점수 전달과 중요도 이중 감점 수정

상태: `implemented_unverified`. 활성 Go 소스와 생산 함수 회귀를 확인했다.
후속 test.18 패키지 검증을 완료했다. 실행 중인 백엔드·실제 사용자 DB·로드된 RisuAI의 증거는 아니다.

4.1·4.2·현재 4.3·참고 1.0의 [비교 기록](../../_diagnostics/20260908-version-memory-review/README.md)에서
의미 검색에 성공한 과거 사건이 최종 선정에서 빠지는 경로를 재현했다. 사용자 제보 세션의
원인을 확정한 것은 아니며, 버전 전체의 품질 순위로 해석하지 않는다.

### 수정 범위

- `prepare_turn_priority_memory.go`의 기존 점수 소유자를 수정했다. aggregate Memory 검색의
  `vector_hit`과 점수가 seed → candidate → complete turn summary까지 이어진다.
  요약 점수는 기존 최고 개별 사실 점수와 원본 의미 점수로 계산한 점수 중 높은 값을 사용한다.
  개별 사실에는 원본 전체의 점수를 복제하지 않으며, 정밀 기억의 의미 점수는 기존대로
  일치하는 사실에만 적용한다. 단어 검색 점수를 vector 관측으로 취급하지 않는다.
- `priority_score.static.v4`는 `0.60 × 관련성 + 0.25 × 저장 중요도 + 0.15 × RP 최근성`
  및 기존 연속성·구조화 보정을 사용한다. 중요도에 최근성을 곱하던 이중 감점을 제거했다.
  최근성의 32턴 반감기·0.20 하한은 유지하며, 같은 조건이면 최근 기억이 유리하다.
  1~10 중요도 구간별 새 감쇠율이나 10점의 무조건 주입은 추가하지 않았다.
- 기존 trace 필드 `importance_after_turn_decay`는 보존하되 v4에서는 저장 중요도와 같은 값이다.
  정책은 `stored_importance_preserved_recency_separate`로 기록한다. 요약 trace의
  `score_source`, `source_vector_similarity`, `source_vector_similarity_observed`로 실제 사용 근거를 확인한다.
- 기존 후보 범위·자료별 K·문자 예산·관점/출처·AI 추천 원문과 순서는 유지한다.
  새 검색, AI 호출, 프롬프트 변경, DB 쓰기, 추가 거부 조건은 없다. 기본 장기 기억 조립에
  적용되며 다중 전처리나 출판사를 켤 필요가 없다. JS 변경은 **0줄 추가 / 0줄 삭제**다.

### 검증 결과

- 새 회귀를 수정 전 코드에서 실행해 과거 의미 검색 기억 누락(K=1/5), 중요도 감점의 실패를 확인했다.
  수정 후 동일 검사를 통과했다. 중요도 1~10 각각을 근거리·100턴·10,000턴 시점에서 확인하고,
  근거 원문 불변·관련 없는 개별 사실의 점수 분리·최근성의 독립 기여도 검사한다.
- 기억 선정 관련 상위 **46개**, 하위 **14개** 통과. 독립 K·예산·낮은 관련성의 비거부·
  과거 사건·관련 없는 고중요 기억·AI 추천 유지·Text/PDF 전달 관련 기존 회귀가 포함된다.
- 이전 비교 fixture를 변경하지 않고 재실행했다. aggregate 의미 검색 K=1/5와 정밀 의미 점수/
  명시 키워드 대조군이 모두 최종 주입문에 도달했다. 앞선 지식 연속성 6개 그룹도 통과했다.
  이 재검사는 상위 **8개**, 하위 **13개**다. 외부 vector 점수와 Store 경계는 통제된 입력이며
  실제 임베딩 생성이나 실 DB를 사용한 시험은 아니다.
- `go test ./... -count=1 -json`: **35개 패키지**, 상위 **3,847개**, 하위 **783개** 통과.
  실 MariaDB/Chroma/제공자 설정과 POSIX 권한 검사를 위한 **12개**는 생략됐다.
  로그와 작업 전 기준은 [이번 검증 폴더](../../_diagnostics/20260908-retrieval-score-repair/README.md)에 보관한다.

### 후속 범위

이번 수정은 검색에서 이미 찾은 자료의 선정 손실을 줄인다. 검색 자체가 놓친 기억,
단어 일치/의미 유사도 간 수치 보정, 상태 필드의 실제 변경 시점, 약속의 이행/취소 연결,
동일 ID 없이 표현이 달라진 정체 공개, 누락된 과거 인물별 지식의 복원까지 해결하지 않는다.
4.4 자료 통합 → 4.6 상태·약속 유효성 → 4.7 선정 → 4.8/4.9 관계 기반 회상으로 이어간다.
실제 확인은 동일 저장 자료·질문·K·예산으로 전처리/출판사를 끄고 검색 후보 → 선정 → 최종 입력을
비교한 뒤 출력의 사실·약속·인물 지식 연속성을 별도로 평가한다. source 선정과 표시 출력은 별개다.

<a id="knowledge-continuity-first-repair"></a>
## 2026-09-08 — 기억 연속성 1차 수정 및 재검사

상태: `implemented_unverified` — 활성 소스·생산 함수 회귀 확인, 실제 Host/DB/본문 영향 미검증.
앞선 읽기 전용 진단의 재현 기록은
[`_diagnostics/20260908-memory-cause-audit`](../../_diagnostics/20260908-memory-cause-audit/README.md)에 있다.
이번 단계는 그곳에서 재현한 지식 생성·조립 오류를 수정한다. 중요도/검색 점수,
선택 K, 추가 AI 호출, 서사 후처리나 4.4/4.6/4.7 시간 모델은 변경하지 않았다.

### 수정한 기존 처리

- `character_perspective.go`: 같은 종류의 비밀과 일반 주관 기억에 속한 별개 주장을
  각각 유지한다. 제공된 비밀 ID 또는 기록된 주장으로 구분한다. 실제 단일 상태를
  표현하는 일반 belief slot의 최신 값 선택은 유지한다.
- `turn_precise_memory.go`: 같은 사실의 `known`과 `revealed`를 서로 충돌하는 지식으로
  처리하지 않는다. 원래 지식 상태 라벨은 보존한다. 정체 매핑도 기존 보호 지식
  후보 생성·출처 연결·저장 함수를 사용해 해당 인물의 관측으로 만든다.
- `turn_extraction_private.go`, `turn_entity_identity.go`: 정체 매핑이 제공한 근거,
  공개 전이 및 식별자를 보존하고 명시된 지식 보유자를 기존 개체 처리에 연결한다.
  `prompts/critic_system.txt`에도 해당 근거/수신자 필드를 설명했다.
  사용자가 저장한 프롬프트를 덮어쓰거나 새로운 출력 거부 조건을 추가하지 않았다.
- `prepare_turn_memory.go`, `prepare_turn_assembly.go`: 임의의 POV를 자기 이름과
  비교해 정체를 아는 것으로 만들던 판정을 수정한다. 본인의 위장 정체와 다른 인물의
  정체를 알고 있는 경우를 구분한다. 이미 조회된 canonical 기록의 공개 상태를
  조립에 전달하여 같은 비밀의 지난 보호 안내를 해제하고, 이유와 공개 출처를
  기존 delivery lineage에 기록한다. 원본 및 별도의 공개 사건 투영은 유지한다.

### 재검사 증거

- 이전 진단 overlay의 재현 입력과 기대 결과를 그대로 재실행하여 모두 통과했다.
  새 `memory_knowledge_continuity_test.go`는 같은 재현을 회귀로 보존하고,
  독립된 비밀·기억, 같은 사실의 알려짐/공개, 알지 못하는 인물과 공개 수신자,
  entity 목록에는 없고 knowledge_scope에만 있는 수신자, 다른 세션,
  과거 공개/이후 비공개 및 해제 출처 기록을 추가 확인한다.
- 정체 항목은 실제 schema 처리 → protected-candidate 처리 → 정규화 → 저장 함수
  → 인물별 읽기 조립까지 확인했다. 저장 경계만 기존 기록형 Store 대역을 사용했다.
- `go test ./internal/httpapi -count=1 -json`: 상위 테스트 **2,709개**와 하위 사례
  **665개** 통과. 실제 제공자 설정이 필요한 `TestConfiguredProviderLiveSmoke` 1개는
  생략됐다. 리롤/같은 입력 수정 후 재생성/동일 문구의 새 입력/재시도/일반 저장과
  기존 관점·공개 범위 회귀가 포함된다.
- 마지막 근거 안내·수신자 연결 확인 후, 기존 진단과 새 기억 회귀 및 Critic 프롬프트
  검사를 함께 재실행해 상위 **23개**, 하위 **27개** 통과했다. 프롬프트의 기존
  16,000자 한도와 JSON 예제 검사도 유지한다. 처음 넓은 검사에서 달라진 소유자 안내
  문구를 기대하던 이전 시험 1개를 갱신했고, 처음 근거 안내 추가 시 초과한 길이는
  해당 설명을 간결하게 정리해 해결했다. 검사 한도를 늘리거나 조건을 제거하지 않았다.
- 로그: 위 진단 폴더의 `repair-httpapi-test.jsonl`, `repair-audit-recheck.jsonl`.
  `git diff --check`로 변경 부분의 공백 오류를 확인했다.

### 남는 범위

- 수정 당시에는 소스만 변경했다. 이후 test.18에 포함했으며 실행 중인 백엔드 적용은 별도다.
  실제 RisuAI/사용자 DB/제공자/최종 RP 출력에서의 확인은 별도다.
- 읽기 조립 수정은 존재하는 관측 기록에 적용된다. 과거에 아예 생성하지 못한
  인물별 지식 기록이나 이전 정규화에서 보존하지 않은 근거를 자동 복구하지 않는다.
- 동일 ID가 없는 과거 기록은 동일하게 기록된 내용까지만 연결한다. 서로 다른
  표현의 같은 비밀, 장기 상태의 필드별 시점, 계보를 건너는 의미상 동일성까지
  해결한 것은 아니다. 검색되지 않은 공개 기록을 새로 찾는 검색도 추가하지 않았다.
- 전처리·출판사를 추가 호출하지 않았고 실제 DB/설정/실행 프로세스/패키지/공개 GitHub는
  변경하지 않았다. 이번 단계 JavaScript 변경은 **0줄 추가 / 0줄 삭제**다.

## 이전 선행 피드백 기록

작업일: 2026-09-06. 기준: 공개 `v4.2.0` 및 활성 작업 트리
`work/4.2.0`의 `3e5e0dc99b3daafce5040418436b671f9a84ac78`.
사용자가 승인한 순서인 #5 → #6 → #4 → #10 → #7을 처리한다.
이 수정은 기본 동작에 적용되며 다중 AI 전처리의 활성화 여부와 관계없다.
선행 피드백을 처리한 당시에는 다중 AI 기능 A–F가 미구현이었다. 이후 구현과
test.12까지의 현재 상태는 [4.3 작업 현황 요약](archive-center-4.3-status-summary.md)을 따른다.
현재 전처리는 소스에 연결된 `implemented_unverified`이며 4.3 정식 완료를 뜻하지 않는다.

전체 판정: `implemented_unverified`. 소스·회귀·격리한 실제 MariaDB/Chroma
검증과 실제 PocketRisu에 수정본을 로드한 검증은 구분한다.
이 작업에서 사용자 DB, 채팅, 설치된 플러그인과 공개 GitHub 이슈 상태는 변경하지 않았다.

2026-09-06 사용자 요청의 [사전 테스트 재실행 결과](archive-center-4.3-pretest-20260906.md):
전체 35개 Go 패키지, 집중 회귀 13개, 실제 MariaDB·Chroma 시험 5개와 정적 검사가
통과했다. 현재 앱의 4.2.0 표기만으로 수정본 로드를 확인할 수 없으므로 실제 Host
검증은 미완료로 유지한다. 이번 재실행에서는 실행 코드를 변경하지 않았다.

이어 사용자 요청으로 [Windows 4.3.0-test.1 테스트 빌드](archive-center-4.3-test-build-1.md)를
생성했다. 플러그인을 테스트 버전으로 표시하고 최종 ZIP의 실제 실행기·DB·Chroma
기동과 기본 요청을 확인했다. 실제 Host·공급자 검증은 사용자가 이어서 진행할 단계다.
패키지 진단 도구의 오래된 세션 삭제 요청도 재현 후 현행 API 형식으로 수정했다.

<a id="hypa-original-import"></a>
## 하이파 가져오기: 원문 1개당 기억 1개 — 2026-09-08

상태: `implemented_unverified` — 활성 소스와 회귀 검사 범위. 실행 중인
수정 당시 실행 백엔드, 설치된 RisuAI, 사용자 DB와 기존 테스트 패키지는 변경하지 않았다.
이후 test.18에 소스를 포함했으며, 기존 가져오기 데이터의 자동 복원을 뜻하지 않는다.

사용자가 승인한 방향대로 각 하이파 요약의 원문을 별도 기억으로 저장한다.
기존 구현도 입력 요약별로 처리했으므로, 제보된 75개와 11개의 차이를
요약 병합 때문이라고 확정하지 않는다. 확인된 경로는 평론가 재요약으로
본문이 대체되는 것, 분석 실패 시 항목이 저장되지 않는 것, 가져오기 번호
충돌에 따른 생략, 음수 번호 기억이 Explorer 범위에서 제외되는 것이다.
제보자의 실제 데이터에서 각 원인이 차지하는 수는 확인하지 못했다.

- `group_audit_feedback_import.go`: 기존 가져오기 처리에서 원문을
  `turn_summary` 및 `hypamemory_import.original_text`에 보존한다. 순서,
  원래 번호, 태그, 분류, 분석 상태와 평론가 요약을 부가 정보로 남긴다.
  평론가가 반환한 나머지 추출 내용은 기존 저장 경로를 따른다.
- `turn_extraction_persist.go`: 명시적인 가져오기 원문을 AI JSON 응답으로
  재해석하거나 공백을 정규화하지 않는다. 원문을 기존 기억 writer에 전달하며
  음수 번호의 외부 가져오기와 정상 RP 턴의 source admission 구분을 유지한다.
- 평론가 설정이 없거나 분석에 실패해도 실제 원문을 저장한다. 분석 성공을
  꾸며내지 않으며 원문 저장 실패와 분석 실패를 별도로 집계한다. 저장은
  기존 동기 요청 안에서 항목 분석 후 이루어진다. 요청 중단 후 자동으로
  계속되는 백그라운드 작업은 아니다.
- Go가 점유된 번호를 확인해 다른 원문에는 비어 있는 음수 번호를 배정한다.
  같은 원문을 다시 가져오면 기존 항목으로 집계한다. 한 목록에 같은 텍스트가
  여러 번 있으면 각 원본 항목 수를 유지하고, 재가져오기 때 같은 수만큼
  기존 기록과 대응한다. 원문 메타데이터가 없는 과거 기억은 그대로 둔다.
- `group_turn_prepare.go`, `group_memory_explorer_read.go`: 세션 범위 안의
  음수 가져오기 기억/근거/KG를 조회에 포함하고 양수 턴의 분기 경계를 유지한다.
  `store/mariadb_chat_memory.go`의 기존 범위 SQL에 음수 import를 포함하며,
  양수 분기 조회 범위를 0부터로 넓히지 않는다.
  `source=hypamemory` 필터는 페이지를 나누기 전에 적용한다. 예전 import score
  표시가 남은 기억도 필터에서 볼 수 있다.
- `memory_search_text.go`: 원문 메타데이터 복사본은 공개 기억 투영에서
  제외한다. 기억 본문은 기존 공개/비공개 처리 경로를 사용한다. 저장된
  원문을 무조건 모두 주입하거나 현재 인물의 지식으로 승격시키는 변경은 없다.
- `Archive Center.js`: 가져오는 중/처리 완료를 실제 동기 요청 상태에 맞춰
  표시한다. 읽음·새로 저장·이미 저장됨·저장 실패·빈 항목 및 분석 결과를
  보여준다. 일부 실패 응답도 내역을 유지한다. 기억 목록에 전체/HypaMemory
  필터와 원문 펼치기를 추가하고 가져오기 후 해당 목록을 갱신한다.

검증:

- 실제 import handler와 저장 경계 기록을 연결한 검사: 75개 → 원문 75개,
  공백/줄바꿈/긴 원문/JSON 모양 원문 유지, 번호 충돌, 동일 텍스트의 복수
  원본, 재가져오기 중복 없음, 빈 항목, 저장 실패 후 다음 항목 처리.
- 실제 제공자 요청 경계를 대체한 검사: 평론가 실패와 원문 저장 성공의
  분리, 성공한 평론가의 요약/근거가 원문을 대체하지 않고 부가 정보로 남음.
- Explorer 필터/페이지 나누기, 음수 기억 조회, 양수 분기 범위 및 다른 세션
  분리, 원문 메타데이터가 공개 투영을 우회하지 않는 것을 검사.
- 실제 JS 함수 실행: 현재 챗의 하이파 요약만 전송, 75개 원문 보존,
  일부 실패 응답의 수치 유지, 필터 요청, 결과 HTML 이스케이프 및 미확인
  수치와 0의 구분. JavaScript 구문 검사 통과.
- `go test ./...` 통과: 테스트가 있는 35개 패키지, 테스트 파일이 없는
  2개 패키지. JS 실행 검사도 Node를 지정해 수행했다. 최초 전체 검사에서
  양수 분기 조회 범위가 넓어진 것을 기존 회귀가 검출했고, 범위 인자를
  원복하고 저장소 SQL에서 음수만 함께 조회하도록 수정한 뒤 전체 통과했다.
- 기존 분기 준비 요청 검사에 하이파 원문을 추가해 실제 `/prepare-turn`
  조립 결과에 포함됨을 확인했다. 부모의 분기 이후 기록과 자식의 복제된
  양수 prefix가 포함되지 않는 기존 부정 검증과 범위 인자 검증도 유지했다.
  정상 새 턴, 같은 사용자 행의 리롤/편집, 같은 텍스트의 새 사용자 행,
  중복 요청 관련 기존 source acceptance/idempotency 검사도 함께 통과했다.
- 기본 Go 실행에서는 외부 빌드 캐시/도구 경로 접근 문제가 발생했다.
  설치된 Go 1.26.6을 명시하고 작업 폴더의 빌드 캐시/임시 경로로 검사했다.
  프로젝트 의존성 버전이나 실행 중인 백엔드는 변경하지 않았다.

범위: 현재 챗의 `hypaV3Data.summaries`만 읽는다. 다른 분할 챗 탐색이나
사라진 요약 복원, 과거 가져오기 기록의 자동 재작성, 콜드 스타트 변경은
포함하지 않는다. 원문이 남아 있으면 새 방식으로 다시 가져올 수 있지만,
과거 압축된 기억은 자동 삭제하지 않으므로 함께 표시될 수 있다.
MariaDB/Chroma와 실제 RisuAI에 새 소스를 적용한 가져오기/회상 확인은 남아 있다.
이번 항목의 JS 증분은 작업 시작 시점 대비 91줄 추가·29줄 삭제다.
추가분은 Host 전송, 목록/UI와 한국어·영어·일본어 문구에 한정된다.

## 추가 제보: Vertex 평론가 처리 모드 — 2026-09-07

Vertex에 다른 제공자용 서비스 등급이 함께 전달되면 Google 호출 전에 실패하는
문제를 공통 Go 요청 조립 함수에서 수정했다. 저장값을 지우지 않으며 Vertex의
전용 Flex 설정으로 처리 방식을 정한다. 평론가 연결 테스트와 실제 추출 경로를
24개 설정 조합으로 검증했다. 수정 전 오류 재현과 제공자 경계 시험의 범위는
[4.3.0-test.9 기록](archive-center-4.3-test-build-9.md)에 정리했다.
실제 Google 계정 및 설치된 RisuAI 검증은 대기 상태다.

## 피드백별 처리

| 피드백 | 현재 처리와 근거 | 확인 범위 |
|---|---|---|
| [#5 대량 세션 삭제](https://github.com/Flazer31/archive-center/issues/5) | 4.2의 `enqueueKnownVectorDeletesTx`가 문서별로 삭제를 모으고 안정된 작업 키를 쓰는 것은 유지했다. 남아 있던 기존 중복 작업 정리의 반복 전수 스캔을 재현하여 `CoalesceInactiveMemoryVectorDeleteOperations`를 마지막 커밋 ID 이후 탐색으로 수정했다. 배치마다 쓰기 잠금을 해제하고, 기존 인덱스를 쓰는 쿼리 계획으로 변경했다. 중간 SQL 읽기 오류도 누락하지 않는다. | 새 삭제의 재시도 중복 없음, HTTP 응답, 실제 Chroma 삭제, 만료된 작업 점유 회수, 관련 없는 자료 보존을 검증했다. |
| [#6 긴 Critic 값 저장 실패](https://github.com/Flazer31/archive-center/issues/6) | `memory_subtype`, `relationship_key`, `reveal_condition`을 `LONGTEXT`로 넓혔다. 신규 설치 SQL·Go 호환성 경로와 기존 설치용 `013_precise_memory_text_fields.sql`을 일치시켰다. 4.2에 이미 있는 `pending` 상태의 Critic 결과 보존과 재사용을 유지한다. | 기존 크기에서 Error 1406 재현 → 원래 결과 JSON/hash 보존 → 마이그레이션 → 동일 결과로 저장 성공 → 중복 재실행을 실제 MariaDB에서 검증했다. |
| [#4 저장 결과 해시 불일치](https://github.com/Flazer31/archive-center/issues/4) | 4.2의 `storedMemoryAdmissionExtraction`은 저장된 JSON과 당시 버전으로 해시를 계산한다. 다시 파싱해 정규화한 JSON으로 해시를 바꾸는 구버전 결함은 현재 경로에 없다. 기존 구현을 유지하고 회귀 및 #6의 실제 DB 재실행으로 재확인했다. | 저장 결과의 배열/문자열 구조, 해시 변조 감지, Critic 재호출 없이 재투영을 확인했다. 부가 보고인 stderr는 현재 진입점이 `slog.NewJSONHandler(os.Stderr, nil)`을 쓰며, 제보자의 NSSM 수집 환경은 미검증이다. |
| [#10 PocketRisu HUD 리스너 누적](https://github.com/Flazer31/archive-center/issues/10) | 등록한 SafeElement·이벤트 종류·ID·옵션을 함께 보관하고 동일 인스턴스의 `removeEventListener`로 정리한다. 제거 실패 시 추적 정보를 보존하여 다음 기존 정리 동작에서 다시 제거한다. 별도 타이머는 없다. | 공식 v1.11.2의 인스턴스별 이벤트 API를 모사한 경계와 실제 Archive Center 함수를 연결했다. 100회 교체 후 리스너 1개, 외부 클릭 위치 조회 1회, 정리/unload 후 0개를 확인했다. 실제 브라우저 프리징의 전체 원인은 미확정이다. |
| [#7 실패 HUD의 반복 복구](https://github.com/Flazer31/archive-center/issues/7) | 사용할 수 없는 복구 동작의 409에 최신 HUD를 포함한다. JS는 실패 응답의 최신 화면 또는 기존 상태 API 1회 조회 결과를 사용한다. 사라진 요청은 오래된 카드를 닫는다. 준비 시 추정 턴과 저장 시 확정 턴이 다르면 Go HUD 표시를 정정한다. | 오래된 실패 화면 반복, 409/404, 뒤늦은 응답이 새 화면에 덮어쓰는 경우, 재생성의 시도 번호를 회귀 검증했다. 실제 PocketRisu에서 수정본으로 같은 동선을 밟는 검증은 남아 있다. |

## 삭제 검증의 정확한 범위

- 각 112개 source revision과 4,000개 문서를 가진 3개 세션을 실제 MariaDB에
  준비하고 총 12,000개 문서를 실제 Chroma 1.5.9에 넣었다. 생산 HTTP DELETE
  라우트 응답은 약 859 / 784 / 798ms였고 각각 `vector_cleanup=queued`였다.
  재시도해도 세션당 삭제 작업은 4,000개였다.
- 생산 worker를 시작하기 전 일부 작업 점유를 만료시켜 중단 후 회수를 검증했다.
  이후 12,000개 Chroma 문서의 삭제와 재조회 확인까지 약 31.19초였다.
  관련 없는 MariaDB source와 Chroma 문서는 남았다. 이 시간은 이 PC의 격리 시험
  측정값이며 모든 OS·사용자 DB 크기의 응답 시간을 보장하지 않는다.
- 같은 비활성 source revision/document의 기존 중복 112,000행에서는 기존 코드가
  2분 제한 안에 정리를 끝내지 못했다. 수정 후 111,000행의 중복 정리가 25.71초,
  남은 1,000개 SQL 작업의 완료가 8개 묶음 / 6.15초였다. 후자는 SQL 대기열
  시험이며 Chroma 호출 시간으로 보고하지 않는다.
- 같은 분포를 1,120,000행으로 늘린 시험도 통과했다. 중복 1,110,000행 정리에
  4분 50.52초, 남은 10,000개 SQL 작업을 79개 묶음으로 완료하는 데 2분 45.58초가
  걸렸다. 데이터 준비를 포함한 시험 전체는 486.83초였다. 제보자의 실제 DB를
  복제한 결과가 아니라 동일 source/document의 중복 규모를 확대한 시험이다.
- 기존 중복 정리는 이미 있는 관리자 재색인 경로에서 실행된다. 업데이트 시
  자동으로 모든 과거 대기열을 삭제하는 기능은 추가하지 않았다. 서로 다른 source
  revision을 임의로 하나로 합치지 않으며, 기존 점유·재시도·순서 조건을 유지한다.
- 세션 삭제는 기존 계약대로 원문·파생 본문을 지우고 관련 source/precise 이력을
  비활성화한다. 감사용 이력 행이 남는 사실을 미삭제된 활성 기억과 혼동하지 않는다.

## 회귀와 경계

- `go test ./... -count=1`: 전체 패키지 성공. opt-in 실제 DB 시험은 별도 실행했다.
- `go vet ./...`, `node --check "Archive Center.js"`: 성공.
- `git diff --check`: 성공. JavaScript 변경은 37줄 추가 / 15줄 삭제이며,
  Host SafeElement 정리와 Go HUD 응답 적용에 한정한다.
- `TestFeedback43LongPreciseValuesMariaDBIntegration`: 120/121/255/256/70,000 길이,
  한글·이모지 포함 세 필드의 값 일치. 자르기나 새로운 결과 거부 조건이 없다.
- `TestFeedback43UpgradeReplaysPreservedAdmissionMariaDBIntegration`: 기존 설치
  오류·원자적 투영 롤백·결과 보존·013 반복 적용·호환성 경로·기존 짧은 행 보존·재실행.
- `TestFeedback43HUDListenersUseRegisteringSafeElement`,
  `TestFeedback43HUDRecoveryRefreshesStaleView` 및 기존 전체 HUD 렌더러 시험:
  모의 Host 경계 + 실제 JS 함수 실행. 라이브 브라우저 시험으로 주장하지 않는다.
- 기존 생산 회귀의 provider retry, 같은 사용자 행 reroll, assistant 삭제 후
  같은 사용자 행 편집·재생성, 동일 본문인 새 사용자 행, 정상 새 턴을 유지했다.
- Go의 `creationSequence`는 HUD 요청 생성 순서만 나타낸다. 완료 턴 저장,
  reroll/edit 식별, 복구 대상 source revision의 결정을 바꾸지 않는다.
- Windows/POSIX 패키지 작성기는 migrations 디렉터리 전체를 복사하고 updater는
  신규 migration을 수용한다. 현재 수정으로 새 OS 패키지를 배포하거나 실제
  설치본을 업데이트한 것은 아니므로 설치·업데이트 완료 증거로 사용하지 않는다.

## 변경 위치

실행 코드는 `Archive Center.js`, `internal/httpapi/turn_workflow_hud.go`,
`internal/store/mariadb_memory_derivation.go`, `cmd/mariadb-schema/precise_memory_schema.go`,
`migrations/001`, `004`, `013`이다. Go 경로는 `go-service` 아래다.
통합 시험은 `cmd/mariadb-schema/feedback_43_mariadb_integration_test.go`에 있으며,
`AC_FEEDBACK_TEST_DISPOSABLE=YES`와 **기존 DB 이름 없는 전용 시험 DSN**을 명시해야
실행된다. 매 시험이 만든 독립 DB만 정리한다. 대규모 시험은
`AC_FEEDBACK_TEST_LARGE=YES`, 실제 Chroma 시험은 전용 `AC_FEEDBACK_TEST_CHROMA`를 쓴다.

## 추가 피드백 — RisuAI·PocketRisu 분기/재분기 연결

작업일: 2026-09-06. 사용자 승인: 두 Host를 함께 지원하는 부모·자식 연결 수정과
4.3 작업 기록. 상태: `implemented_unverified`. 앞의 #5/#6/#4/#10/#7 변경을 보존했다.

### 확인한 원인과 Host 근거

- 기본 [RisuAI `Chat.svelte`](https://github.com/kwaroran/RisuAI/blob/c454df882aaf32e02a22da26d3718c8cadc97814/src/lib/ChatScreens/Chat.svelte)은
  기준 메시지를 포함한 앞부분을 복사하면서 기존 메시지 ID를 유지한다.
- [PocketRisu `Chat.svelte`](https://github.com/PocketRisu/PocketRisu/blob/ca09a80746e74e5334145e5e78af47ce423e0eba/src/lib/ChatScreens/Chat.svelte)과
  [`chatClone.ts`](https://github.com/PocketRisu/PocketRisu/blob/ca09a80746e74e5334145e5e78af47ce423e0eba/src/ts/chatClone.ts)는
  같은 앞부분을 복사한 뒤 `reissueMessageIds`로 자식 ID를 새로 발급한다.
  함수의 원본→신규 ID 표는 로컬 변수이며 외부 플러그인용 영구 출처 표로 제공되지 않는다.
- 두 Host의 `branchedfrom` 마커는 **부모 채팅의 기준 메시지 ID**를 담는다.
  마커 클릭도 부모 채팅으로 이동한 뒤 그 ID의 메시지로 이동한다. 마커를 자식 ID로
  바꾸는 수정은 하지 않았다. AC가 마커 ID와 자식의 직전 메시지 ID를 같다고 가정한
  지점이 PocketRisu에서 `fork_source_message_conflict`를 만들었다.
- 두 Host의 공식 `getCharacterFromIndex` API를 확인했다. 지정 캐릭터 안에서
  Go가 요청한 채팅을 읽는다. 사용자 PC의 설치 버전이나 로드된 실제 플러그인을
  위 소스 SHA와 같다고 가정하지 않는다.

### 구현한 동작

1. 기존 Go `resolveRisuWorldlineObservation`에서 부모 ID와 자식 ID의 동등성 요구를
   제거했다. 부모 마커가 가리키는 source 및 부모 user anchor로 기존 원문 이력을 찾는다.
   기본 RisuAI의 같은 ID 경로도 그대로 사용한다.
2. 초기 output/pre-backfill 관찰은 기존 2–3행으로 유지한다. Go가
   `origin_read_request`를 반환하면 JS가 지정 캐릭터의 부모 채팅을 읽어 ID·역할·순서와
   분기 마커만 `risu_message_origins.v1`로 전달한다. 대화 본문·유사도 검색·LLM 호출은 없다.
3. Go가 보존된 ID 및 두 Host의 기준 메시지 포함 순서 복사 구조로 출처 대응을 만든다.
   기존 `session_fork_lineage.inherited_items_json`에 버전 있는 배열 항목으로 저장한다.
   이미 확정된 계보는 빈 출처 필드만 보충하며, 부모·분기 턴·source 좌표를 바꾸지 않는다.
   이 작업을 위한 새 DB 테이블·마이그레이션은 없다.
4. 재분기는 저장된 자식→부모 대응을 따라 조상의 원문을 찾는다. 직접 부모가 B이면
   원문이 A에 있어도 C의 부모는 B로 유지한다. 과거 B 계보에 출처 대응이 없으면,
   C를 관찰할 때 B의 분기 마커를 기존 Go 해석 경로로 전달하여 보충한다.
   읽기는 Go가 지정한 조상에 한정하며 기존 계보 깊이 32 안에서 끝난다.
5. C가 B의 상속 구간 안에서 더 일찍 분기했다면 회상 범위에 더 이른 C의 경계를
   유지한다. user 메시지 T에서 분기할 때 원문 식별에는 T를 쓸 수 있지만,
   부모의 완료된 기억을 상속하는 범위는 T−1까지다.
6. 출처 대응이 저장된 이후의 일반 관찰은 부모 앞부분을 다시 요청하지 않는다.
   Host 읽기나 선택적 추가 전송 실패는 최초 라우팅 응답을 유지한다. 마커/부모가
   나중에 없어져도 이미 확정된 계보를 지우지 않는다. 일반 출력·완료 턴 저장·reroll/edit
   수락 조건과 finality 소유자는 변경하지 않았다.

### 검증 결과

| 검증 | 결과와 범위 |
|---|---|
| `TestWorldline43PreservedAndReissuedIDsShareParentAuthority` | 두 ID 형식 × user/char 분기. 실제 완료 저장처럼 assistant ID가 없는 원문도 부모 user anchor로 해결한다. 마커의 부모 ID 보존과 저장된 계보 재사용을 확인했다. |
| `TestWorldline43NestedReissuedBranchUsesStoredOriginsAndEarlierCut` | ID를 다시 발급한 재분기에서 조상 원문을 찾고, 더 이른 user/char 경계 이후의 조상·부모 소유 기억을 포함하지 않는다. |
| `TestWorldline43NestedUserEndpointAndEmptyInheritedScope` | 첫 user 메시지에서 재분기할 때 원문 턴은 찾되 상속한 완료 기억은 0개다. |
| `TestWorldline43ExistingNestedBranchEnrichesParentWithoutOpeningIt` | 이전에 확정된 부모의 빈 출처 표를 자식 관찰 경로에서 보충하고 재분기 연결을 해결했다. |
| `TestWorldline43ConfirmedLineageOnlyEnrichesEmptyOriginMetadata` | 실제 SQL 저장 함수의 트랜잭션·재조회 경계를 모사했다. 빈 출처만 보충하고 기존 부모/분기 좌표와 이미 저장된 출처 표를 보존한다. |
| `TestWorldline43HostOriginReadAndRouting` | 실제 JS 함수 + 공식 Host/HTTP 모의 경계. 최초 읽기와 조상 보충, 본문 미전송, 고정 관찰 불변, 요청 중 캐릭터 전환에도 원래 캐릭터 유지, 읽기·전송 실패 시 원래 응답 유지, 저장 후 추가 읽기 없음, 일반 복사에서 마커를 만들지 않음을 확인했다. |
| `TestWorldline43ReissuedNestedBranchMariaDBIntegration` | **실제 격리 MariaDB + 생산 HTTP 라우트/Store**. 부모 계보 확정→출처 필드 보충→재발급 ID의 재분기 연결→매 요청 새 서버 인스턴스→마커 제거 뒤 재조회까지 통과했다. 단위 시험과 별도로 실행했고 2.03초였다. |
| 전체 검사 | 수정 후 `go test ./... -count=1`, `go vet ./...`, `node --check "Archive Center.js"` 성공. 기존 provider retry, 같은 사용자 행 reroll, assistant 삭제 후 같은 사용자 행 편집·재생성, 같은 본문인 새 사용자 행, 정상 새 턴의 생산 회귀를 포함한다. |

새 Go 소유자는 `internal/httpapi/worldline_message_origins.go`이며 기존
`group_turn_range_decision.go`, `group_step23_fork_lineage.go`, `group_turn_prepare.go`,
`internal/store/mariadb_narrative_state.go`에 연결했다. Go 경로는 `go-service` 아래다.
JS 증가가 필요한 이유는 두 Host의 공식 캐릭터/채팅 snapshot API가 플러그인에만
노출되기 때문이다. 이번 분기 수정의 JS는 **78줄 추가 / 1줄 삭제**다. 앞의 다섯
피드백과 이 분기 수정까지의 누적 JS 차이는 **115줄 추가 / 16줄 삭제**다.

### 남은 확인과 별도 범위

- 수정본을 실제 기본 RisuAI와 PocketRisu에 로드하여 분기·재분기·기억 조회까지
  확인하는 검증은 남아 있다. HTTP/DB 시험 성공을 실제 Host 완료로 보고하지 않는다.
- 순서 대응은 두 공식 분기 구현의 앞부분 복사 동작을 사용한다. 오래된 부모·자식에서
  메시지를 임의로 편집/재배열한 경우의 완전한 출처 복원은 보장하지 않는다. 복사
  위치가 달라진 앞부분은 보존 ID와 명시적 분기 끝점만 기록한다. 읽을 수 없는 부모나
  원문이 부족한 경우에는 기존 미해결/수동 연결 흐름이 남는다.
- 일반 챗 복사는 새 공식 분기 마커를 주지 않는다. 기존 라우팅/backfill 및 명시적
  DB 세션 복사를 유지한다. 이미 있던 분기 마커까지 복사되는 경우도 새 마커로 바꾸지 않는다.
- 상속 기억 전체의 독립 복사·수정 체크박스와 개체/주관 기억 상속 UI는 별도 기능이다.
  이번 변경으로 그 기능까지 구현했다고 기록하지 않는다.
- 사용자 DB·채팅·Host 파일을 수정하거나 GitHub에 배포하지 않았다. 시험은 기존
  사용자 DB와 다른 포트의 전용 MariaDB 및 매번 생성한 독립 DB에서 수행했다.

## 추가 피드백 — Gemini 3.8 Flash medium 선택

2026-09-06 사용자 승인으로 `gemini-3.8-flash`의 medium 선택·전송을 추가했다.
상태는 `implemented_unverified`다. [Google 공식 규격](https://ai.google.dev/gemini-api/docs/generate-content/latest-model?hl=en)의
지원값은 low/medium/high이고 기본값은 medium이다.

- JS `resolveGeminiThinkingLevelOptions`와 Go `proxyGeminiThinkingLevel`의 기존
  모델 목록에 3.8 Flash를 추가했다. 출판사·평론가의 선택지는
  **none / low / medium / high**이며, 선택한 medium을 실제 요청 본문에 유지한다.
- none의 의미와 프리셋 기본 선택은 바꾸지 않았다. Google 직접 연결의 none은
  설정 생략이고, 기존 중계 경로는 각 전송 규약에 따라 none을 전달한다.
- 수정 전 UI에서 medium이 누락되고 Go의 Google/LLM Gateway/OpenRouter 요청에서
  medium이 빠지는 실패를 생산 함수 시험으로 재현했다. 수정 후 해당 시험을 통과했다.
- `TestArchiveCenterJSReasoningControlsUseProviderAndEndpointRuntime`에서 Gemini,
  Vertex, LLM Gateway, OpenRouter의 선택·정규화·bridge 본문과 기존 모델 선택지를 확인했다.
  `TestProxyGemini38ThinkingLevelReachesPublisherAndCriticWire`는 출판사·평론가의
  none/low/medium/high 요청 본문을 확인하고, 기존 전송 시험은 두 중계 형식의 medium을 확인했다.
- `go test ./cmd/js-route-variant-smoke ./internal/httpapi -count=1` 및
  `node --check "Archive Center.js"` 성공. 외부 Host/HTTP 경계는 모의 환경이며 실제
  Google 계정 호출이나 수정본을 로드한 화면 확인은 수행하지 않았다.
- 이번 실행 코드 변경은 JS **1줄 추가 / 1줄 삭제**, Go **1줄 추가 / 1줄 삭제**다.
  이번 단계까지 작업 트리의 JS 누적 변경은 **116줄 추가 / 17줄 삭제**다.
