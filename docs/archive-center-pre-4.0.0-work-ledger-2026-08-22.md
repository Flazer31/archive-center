# Archive Center 4.0.0 종합 작업 원장

상태: `current_worktree_record / 4.0.0 source·regression·test package 갱신 / live gate 진행 전`

- 기록 기준일: 2026-08-24 KST
- 기록 대상: 현재 `source` 작업 트리, 단계 0–9, UI·세계선·기억·공급자·복구·예산·로어북·시험 패키지 작업
- 보존 체크포인트: `eed62ae checkpoint: preserve pre-4.0 stages 0-7`
- 현재 HEAD: `eae0b3f diagnose reversible status lock failures`
- 현재 source 제품 표기: `Archive Center 4.0.0` (2026-08-23 정식판 identity 승격)

파일명에는 기존 문서 링크의 안정성을 위해 `pre-4.0.0`이 남아 있지만, 문서 제목과 최신 판정의
대상은 현재 `4.0.0`이다. 과거 Pre 단계의 기록과 해시는 역사적 증거로 보존한다.

이 문서는 지금까지의 작업을 다시 추적할 수 있게 만든 종합 원장이다. 기존 3.6–3.9 장부,
3.9 피드백 작업 로그, 4.0 세계선 설계, 4.0 기억 복원 계약을 대체하지 않는다. 각 문서에
분산된 작업과 그 이후의 단계 0–9 작업을 한곳에 연결하고, 다음 UI 작업의 출발점을 고정한다.

이 문서에 `완료`라고 적힌 항목도 아래 증거 등급을 함께 읽어야 한다. 소스가 구현됐다는 사실은
시험 패키지 반영, 실제 RisuAI 로드, 실제 공급자 성공, 실제 MariaDB·Chroma 저장까지 자동으로
증명하지 않는다.

## 1. 증거 등급

| 등급 | 의미 |
|---|---|
| `DOCUMENTED` | 요구사항·설계·결정이 문서 또는 작업 대화에 기록됨 |
| `SOURCE` | 현재 production source의 실제 경로에 연결됨 |
| `REGRESSION` | 해당 소스의 자동 회귀 또는 문법 검사가 통과함 |
| `PACKAGE` | 지정 시험 패키지에 소스·바이너리·프롬프트가 포함됨 |
| `LIVE` | 실제 RisuAI, backend, MariaDB, Chroma 또는 외부 공급자에서 관찰됨 |
| `USER_OBSERVED` | 사용자가 실제 화면·공급자·턴 실행에서 보고한 결과 |
| `INFERRED` | 소스·로그 정합성으로 추론했으나 독립 실사용 증거가 부족함 |
| `OPEN` | 미구현, 미검증, 또는 재현·판정이 남음 |

`selected`, `hydrated`, `delivered`, 실제 메인 모델 payload, RisuAI에 표시된 최종 출력은 서로
다른 단계다. 기억 품질을 완료로 판정하려면 최종 payload와 displayed-final 계보까지 확인해야 한다.

## 2. 현재 스냅샷

### 2.1 소스와 Git

- 활성 소스: `<archive-center-root>\source`
- 단계 0–7 보존 체크포인트: `eed62ae` (2026-08-21 19:35:40 KST)
- 현재 HEAD: `eae0b3fba828df70c10ac9aad0ddef31fa040fdb`
- HEAD 의미: reversible status lock failure 진단을 고정한 로컬 checkpoint
- 현재 상태: HEAD 이후 단계 8–9, UI, 호출 장부, 로어북, 4.0 identity와 후속 수정이 dirty worktree에 있음
- 2026-08-24 이번 원장 갱신 직전 tracked diff: 46개 파일, `+2,329 / -516`
- 같은 시점 `Archive Center.js`: `+244 / -20`
- 확인된 시험 산출물: `_test-builds/Pre-4.0.0-stage9-critic-compact-windows-test`
- 이 원장 갱신에서는 runtime source, test, prompt를 변경·정리·되돌림·포맷하지 않았다.

### 2.2 최신 시험 패키지

- 외부 패키지 루트:
  `_test-builds/Pre-4.0.0-stage9-critic-compact-windows-test`
- 실제 4.0.0 폴더:
  `Archive Center 4.0.0 Windows Auto Install Package`
- 실제 4.0.0 ZIP:
  `Archive Center 4.0.0 Windows Auto Install Package.zip`
- manifest 생성 시각: `2026-08-23T16:00:21.7968232Z` (`2026-08-24 01:00:21 KST`)
- manifest 상태: `green`, `release_ready=true`, `automatic_update_apply=true`
- package profile: `windows_managed_full_local_auto_install`
- canonical store: MariaDB
- vector engine: ChromaDB
- managed files: 46개, manifest hash 불일치 0개
- 소스와 패키지 `Archive Center.js` SHA-256:
  `9C69DEA34D4E82AE5FFD6FE82B6ED61AB666ED05B62762F82DB8F676ADA839BF`
- packaged `bin/archive-center-go.exe` SHA-256:
  `CB2525BB196DD7E19490EF6598EF949D2B985FF9D07F2D6197071F484231EC65`
- ZIP SHA-256:
  `DA3EA75D35A567433F332F96E7362010003BE46D01DBFBC5816C1E104506CCD1`
- `SHA256SUMS-4.0.0.txt`: ZIP과 일치
- 기존 `.env.full.local`: 재빌드 전 보존, 재빌드 후 동일 hash로 복원
- backend와 package launcher: 갱신 후 종료 상태

`release_ready=true`는 관리 대상 파일이 갖춰진 build manifest 판정이다. 공개 배포 완료 또는 모든
공급자·OS·장기 세션 검증 완료라는 의미는 아니다. 이번 패키지는 빌드·manifest 단계까지 검증됐고,
package-launched `/ready`와 실제 RisuAI 플러그인 재로드·턴 실행은 아직 별도 `LIVE` gate다.

### 2.3 최근 실사용 기준선

2026-08-22의 같은 작업 흐름에서 세션
`char_11_cid_94257838-f133-471b-aeee-d752dea69990`의 최신 턴을 확인했다.

| 턴 | 평론가 결과 | 핵심 저장 | Vector | 언어 | 판정 |
|---:|---|---|---|---|---|
| 13 | `critic_pipeline.v6`, admission committed | 기억 1, 직접 근거 13, KG 8, 인물 상태 3, 활성 상태 3, pending thread 1 | outbox 21건, 1회 시도로 완료 | 기억 요약 한국어 | `LIVE`, 부분 항목만 국소 제외 |
| 14 | `critic_pipeline.v6`, admission committed | 기억 1, 정밀 기억 18, 직접 근거 16, KG 10, 개체 ID 14, 별명 6, ID 연결 25, 인물 상태 3, 서사 상태 21 | outbox 35건, 1회 시도로 완료 | 기억 요약 한국어 | `LIVE`, 전체 pipeline 정상 |

14턴 HUD에서 출판사 호출은 약 6.69초, 본문 응답은 약 20.8초, 평론가는 약 44초였다.
최근 네 턴 11–14는 `critic_ingest_trace.pipeline_complete=true`가 연속으로 확인됐다. 다만 성공
감사 행에는 공급자·모델·`reasoning_effort`가 영속되지 않으므로, 나중에 특정 GLM 설정으로 성공했다고
DB만으로 역증명할 수는 없다.

국소 제외 사유도 보존했다.

- 13턴: 원문에서 정확히 한 번만 대응되는 span이 없는 정밀 기억 후보 1건 제외
- 13턴: 안정된 방향 ID가 없는 관계 상태 후보 2건 제외
- 13턴: 지지 근거가 부족한 말투·행동 후보 1건 제외
- 14턴: 안정된 방향 ID가 없는 관계 상태 후보 2건 제외
- 14턴: 제목이 없는 pending thread 후보 2건 제외

이는 후보 하나가 불완전하다고 전체 평론가 결과를 폐기하지 않고, 정상 후보를 계속 저장하는 현재
부분 수용 계약의 실제 예다. 반대로 두 턴 모두 관계 상태 저장이 0건이라는 점은 계속 관찰해야 한다.

## 3. 변하지 않는 전체 원칙

사용자와 합의한 단계 0–9의 고정 원칙은 다음과 같다.

1. 모델명 allowlist를 만들지 않는다.
2. Go backend가 정책, 파싱, 실패 판정, 저장, 재처리와 ViewModel을 소유한다.
3. `Archive Center.js`는 RisuAI 관찰, 설정 표시, 요청 전달, 실제 payload 적용, DOM 렌더링만 맡는다.
4. 출판사와 평론가의 실제 호출에 숨겨진 재시도를 추가하지 않는다.
5. 잘린 JSON을 합성 복구해 정상 결과처럼 저장하지 않는다.
6. 출판사는 단일 호출과 fail-open을 유지한다. 출판사 실패가 본문 생성을 막지 않는다.
7. 평론가 실패 시 파생 저장은 0건이고, 재처리 대상은 1건으로 남긴다.
8. 이번 안정화에 새 DB schema나 새 table을 추가하지 않는다.
9. 기존 기억 종류, 비밀·관점 경계, 출력 언어 계약을 바꾸지 않는다.
10. 한 후보나 선택 필드가 틀렸다는 이유로 전체 추출 결과를 버리지 않는다.
11. 유용한 RP 연속성을 과도한 보호 조건으로 침묵시키지 않는다.
12. 특정 공급자·모델 문구를 평론가 본체 정책에 하드코딩하지 않는다.
13. dirty 변경을 보존하고 기존 owner의 정확한 함수 단위로 고친다.
14. 시험 패키지는 소스 검증 뒤에 기존 지정 빌드를 갱신한다.
15. 기억의 정확도가 파일 크기나 줄 수 감소보다 우선한다.

영구 경계는
[`permanent-risu-host-backend-boundary.md`](permanent-risu-host-backend-boundary.md),
4.0 기억 복원 경계는
[`4.0-memory-restoration-work-contract.md`](4.0-memory-restoration-work-contract.md)를 따른다.

## 4. 시간순 작업 기록

정확한 날짜가 별도 commit·manifest·로그에 없는 초기 UI 대화는 순서를 보존하되 임의의 날짜를
붙이지 않았다.

| 시점 | 작업 묶음 | 결과 |
|---|---|---|
| 초기 Pre-4.0 UI 작업 | 네온형 화면을 near-black 기반 제품 UI로 재구성, 상단 정보구조 변경, 세계선 canvas 도입 | `SOURCE`, 전면 UI 재정비는 계속 필요 |
| 초기 branch 실사용 | branch 턴 번호 재시작, 다중·하위 branch 고립 노드, 현재 세션 필터 문제 재현 | lineage·turn owner를 Go에서 수정 |
| 기억 UI 후속 | 기억 재조립, 수정 기능 점검, 보조 참조·로어북 20개 조회, 중복 재호출 제거 | `SOURCE`; 대규모 DOM 렌더링은 다음 UI 단계에서 개선 |
| 2026-08-08 이전/당일 | 평론가 고정 프롬프트 1차 정리와 입력 과다 축소, 재처리 snapshot·schema 오류 반복 방지 | 기존 3.9 로그와 prompt 백업에 기록 |
| 2026-08-19 전후 | Publisher·Critic malformed JSON, 공급자 응답 정규화, DeepSeek 토큰·추론 문제, 기억 recall owner 점검 | 단계 0–4의 직접 배경 |
| 2026-08-21 | 단계 0–7 구현·검증 후 로컬 checkpoint | commit `eed62ae` |
| 2026-08-21~22 | recovery source revision 충돌, HypaMemory, 로어북, branch ancestry, 출력 언어, provider reasoning 후속 수정 | 소스·회귀·실사용을 각기 분리 확인 |
| 2026-08-22 | 단계 8 정리, 단계 9 평론가 compact·부분 수용·reasoning adapter, GLM 세대별 설정 | 현재 dirty worktree |
| 2026-08-22 | 최신 Windows 시험 패키지 같은 경로로 갱신 | source/package JS hash 일치 |
| 2026-08-22 | 최근 턴 13–14 MariaDB·Vector·HUD 검사 | 평론가 저장 정상, 관계 상태 lane은 관찰 필요 |
| 2026-08-22 | 로어북 보조 참조를 설정의 On/Off로 단순화 | `플로팅 UI` 바로 위에 배치, 기존 로어북 선택창 제거 |
| 2026-08-22 | 출판사 동작 설정을 공통 설정에서 분리 | 별도 `출판사 설정`을 공통 설정보다 위에 배치 |
| 2026-08-23 | DB 경합 진단, 독립 payload/provider 장부, 관계 기억 과주입 제한, 로어북 선택 실험 | sections 26–34에 source·regression·package 증거 기록 |
| 2026-08-23 | source identity를 `4.0.0` release로 승격 | JS·Go·builder·설치 문서 정렬 |
| 2026-08-24 | 로어북의 기존 보수 조건을 제거한 뒤 관련도 frontier로 재작성, 새 기본 상한 3,000 적용 | `SOURCE`, `REGRESSION` 완료 |
| 2026-08-24 | 기존 최신 시험 빌드 경로를 실제 `4.0.0` 폴더·ZIP으로 갱신 | `PACKAGE` 완료, `LIVE`는 미실행 |

## 5. UI와 정보구조 작업

### 5.1 디자인 방향

사용자가 지정한 디자인은 사이버펑크·게임 UI가 아니라 프리미엄 B2B 금융 SaaS 계열이다.

- 배경과 카드는 여러 단계의 near-black으로 분리한다.
- 포인트 컬러는 전체 면적의 5–10% 이하로 제한한다.
- glow, gradient, glass는 선택·중요 CTA·작은 floating panel에만 사용한다.
- 박스형 상단 탭을 버리고 선택 탭 아래의 밝은 선으로 상태를 표시한다.
- 불필요한 설명 문장을 제거해 작업 공간을 확보한다.
- 현재 상단 정보구조는 `세계선 | 기억 | 기억 관리 | 추가 기능 | 설정`을 기준으로 한다.
- 원작 DB·페르소나 캡슐·로어북 관리 같은 확장 기능은 기억 열람과 분리한다.

### 5.2 세계선

- 기존 `타임라인` 중심 이름과 화면을 `세계선` 중심으로 바꿨다.
- 각 턴을 하나의 node box로 표시하고 branch를 연결선으로 나타낸다.
- canvas는 확대, 축소, 현재 위치, 전체 맞춤을 지원한다.
- 선택한 세션이 바뀌면 모든 탭의 대상도 같은 세션으로 바뀌어야 하며 다른 세계선의 항목이 섞이면 안 된다.
- 사용자는 현재 세계선의 큰 틀은 괜찮다고 판정했다.

아직 만족하지 못한 핵심 상호작용은 node 확장 방식이다. 목표는 세계선의 턴 박스를 누르면 그
박스 아래로 기억·근거·관계 등의 항목이 펼쳐지고, 항목을 누르면 같은 흐름 안에서 상세 영역이
확장되는 것이다. 별도의 작은 고정 높이 창을 만들어 다시 위아래로 스크롤시키는 구조는 최종안이 아니다.

### 5.3 기억 화면

- 좌측 기억 종류와 우측 목록이라는 이전 구조를 한 차례 재조립했다.
- `대화 원문`, `기억 요약`, `직접 근거`, `관계 지식`, `계층 요약`, `신뢰 제어`,
  `세계 규칙`, `개체 정보`, `보조 참조` 등의 표면을 현재 세션 범위로 표시한다.
- 잘못 생성된 기억을 사용자가 수동 수정할 수 있어야 한다.
- 직접 근거를 포함한 수정·검토·삭제 경로를 점검하고 관련 UI·backend API를 연결했다.
- 이미 backend에서 받은 목록을 펼칠 때 다시 요청하지 않도록 했다.
- 로어북은 20개 단위로 조회하고 `더 보기` 시 누적한다.
- UI를 닫았다는 이유로 받은 목록을 매번 20개부터 다시 요청하지 않는 현재 cache 방식을 유지했다.

### 5.4 기억 관리와 추가 기능

- DB 전체 관리의 `연결 | 이동 | 복사 | 삭제`는 `기억 관리`로 모으는 방향을 확정했다.
- 로어북 동기화, reference DB, 페르소나 캡슐 등은 `추가 기능` 또는 기억 관리의 명시적 하위 영역으로 분리한다.
- 현재 기능 경계는 일부 연결됐지만, 이동·복사·삭제의 최종 UI와 전체 정보구조는 다음 UI 단계의 작업이다.

### 5.5 브라우저 메모리 원칙

- backend ViewModel과 페이지 단위 결과를 렌더링하고 JS에서 같은 데이터를 다시 계산하지 않는다.
- 닫힌 상세 항목의 전체 DOM을 계속 모두 mount하지 않는다.
- 이미 받은 작은 페이지는 재사용하되, 모든 장기 세션 원문을 무제한 JS 배열과 DOM에 누적하지 않는다.
- canvas node에는 요약만 두고 상세 데이터는 선택 시 사용한다.
- event listener, observer, cache, timer를 화면을 다시 열 때 중복 등록하지 않는다.
- UI 편의를 이유로 branch·기억 선택·예산 정책을 JS로 옮기지 않는다.

### 5.6 2026-08-22 로어북 보조 참조 설정 정리

- `추가 기능 > 로어북`의 `검색만/본문 참조 보조` 선택창을 제거했다.
- 설정 화면에서 `로어북 보조 참조` 체크박스를 `플로팅 UI` 체크박스 바로 위에 배치했다.
- 체크 시 기존 backend 값 `reference_assist`, 해제 시 `search_only`로 저장한다.
- 해제해도 로어북 저장·동기화·조회와 추가 기능 화면의 수동 새로고침은 유지한다.
- `플로팅 진행 UI` 명칭은 한국어·영어·일본어 모두 짧은 `플로팅 UI` 계열로 변경했다.
- 이 bounded slice의 JavaScript 증감은 `+24 / -49`, 회귀 테스트 증감은 `+12 / -2`다.
- Node 문법 검사와 로어북·플로팅 UI 설정 표적 smoke 3종이 통과했다. 실제 RisuAI 화면 확인과
  시험 패키지 갱신은 아직 수행하지 않았다.

### 5.7 2026-08-22 출판사 설정 분리

- `서사 가이드 모드`, `서사 가이드 강도`, `출판사 안내 표현 형식`, `서사 안내 예산 (chars)`을
  공통 설정의 기억 카드에서 제거했다.
- 네 항목을 한 번만 렌더링하는 별도 `출판사 설정` 카드로 묶어 공통 설정 바로 위에 배치했다.
- 기존 setting key, 저장 처리, backend 전달 계약은 변경하지 않았다.
- 이 bounded slice의 JavaScript 증감은 `+48 / -40`, 회귀 테스트 증감은 `+25 / -0`이다.
- Node 문법 검사와 위치·단일 렌더링·기존 설정 계약 표적 smoke 2종이 통과했다. 실제 RisuAI 화면
  확인과 시험 패키지 갱신은 아직 수행하지 않았다.

## 6. 세계선·branch 정합성 작업

### 6.1 재현된 문제

1. 8턴에서 branch한 뒤 각 line을 1턴 진행했을 때 child가 1턴을 10턴처럼 평론가에 전달했다.
2. 다음 턴은 2턴을 11턴으로 전달했다.
3. branch에서 다시 branch하면 11턴 node가 부모 연결 없이 고립됐다.
4. branch 화면에는 새 턴 9–10만 보이고 상속된 1–8턴 기억이 없는 것처럼 보였다.
5. 상단 세션 선택과 다른 탭의 세션 범위가 일치하지 않아 다른 세계선이 함께 보였다.

### 6.2 적용한 계약

- RisuAI host는 raw chat/session observation만 JS에서 보낸다.
- `is_branch`, 부모 session, fork turn, 상속 범위, child의 다음 논리 턴은 Go가 판정한다.
- branch 이름 문자열이나 화면 순서를 정본 ID로 사용하지 않는다.
- 공식 RisuAI의 `branchedfrom` marker 형태를 versioned host observation으로 해석한다.
- parent는 fork point까지 소유하고 child는 그 다음 논리 턴부터 진행한다.
- parent의 fork 이전 기억은 child 검색·표시 범위에 포함한다.
- child의 fork 이후 기억은 parent나 sibling에 역류하지 않는다.
- reroll과 edit는 branch 생성과 구분한다.
- descendant branch도 동일한 lineage 규칙을 재귀적으로 따른다.

상세 설계와 미지원 host 경계는
[`4.0-risu-worldline-observation-design.md`](4.0-risu-worldline-observation-design.md)에 기록돼 있다.

### 6.3 남은 live gate

- 장기 parent → child → grandchild에서 턴 번호가 계속 단조 증가하는지
- branch from user와 branch from assistant의 fork ownership 차이
- RisuAI 재시작 후 lineage 재구성
- malformed 또는 누락된 `branchedfrom` marker의 conflict 표시
- delete, edit, reroll 뒤 branch ancestry와 source revision 정합성
- 모든 기억 화면에서 상속 턴과 현재 branch 전용 턴이 시각적으로 구분되는지

## 7. 기억·로어북·HypaMemory 작업

### 7.1 branch 기억

branch는 새 세션이지만 기억이 없는 빈 세션이 아니다. fork 이전 parent 기억을 검색 대상으로 포함하고,
UI도 상속 범위를 숨기지 않아야 한다. 초기 화면에서 1–8턴이 보이지 않던 문제는 ancestry 포함 방향으로
수정했다. 이 계약은 일반 세션의 과거 기억 검색과 같은 Go retrieval owner를 사용하며, branch만을 위한
두 번째 기억 시스템을 만들지 않는다.

### 7.2 로어북

- 로어북은 주력 사용자 요청을 대체하는 입력이 아니라 별도 `reference_assist` lane이다.
- 현재 캐릭터·채팅·활성 모듈 범위의 저장된 로어북을 backend가 검색한다.
- 명시적 로어북 직접 참조가 없다는 이유로 로어북 기능 전체를 끄지 않는다.
- 활성화된 로어북만 참조한다.
- 일반 턴마다 RisuAI 원본 로어북 전체를 다시 읽지 않는다.
- UI의 `보조 참조`는 backend에 이미 저장된 내용을 열람한다.
- 검색만 하는 `search_only`와 본문 보조까지 허용하는 `reference_assist`를 구분한다.
- 로어북이 일반 기억·사용자 요청·세계 진실을 덮어쓰지 않도록 별도 lane과 예산을 유지한다.

### 7.3 HypaMemory

- 현재 RisuAI chat의 HypaMemory 요약을 읽어 기존 import endpoint로 전달한다.
- 가져온 요약은 평론가가 importance, KG triple, embedding을 생성하는 재처리 흐름으로 들어간다.
- 중간의 임의 조건으로 기능이 조용히 off되지 않도록 host API, chat, Hypa 데이터 없음은 명시적 오류로 표시한다.
- import가 정상 시작됐다는 사실과 평론가 파생 저장 완료는 별도의 상태다.

### 7.4 오래된 원문 문맥

현재 일반 세션과 branch 모두 과거 기억을 검색해 선택하는 기능은 있다. 다만 선택된 오래된 기억의
주변 원문 여러 턴을 세계선까지 따라가 메인 모델과 평론가에 자동으로 복원해 붙이는 별도 범용 기능은
현재 완료 계약으로 보지 않는다. 현재 기억 종류와 예산이 충분한 동안은 이 기능을 새 owner로 추가하지
않기로 했다.

## 8. 출력 언어 계약

사용자가 최종 확정한 기준은 `출력 언어만 따진다`이다.

- 평론가가 생성하는 자연어 기억 필드는 실제 displayed final output의 언어를 따른다.
- 캐릭터 이름, lorebook 언어, 과거 기억 언어, UI 언어를 출력 언어 대신 우선하지 않는다.
- 한국어 본문인데 일본어 평론·기억이 저장되던 문제를 이 기준으로 수정했다.
- 현재 `critic_system.txt`도 `summary_language` 또는 `session_output_language`와
  `Language_Context_JSON`의 관찰된 최종 출력 언어 계약을 따르도록 명시한다.
- 최근 확인한 13–14턴 기억 요약은 한국어였고 일본어 혼입은 확인되지 않았다.

다국어 장기 세션에서 한 턴 안에 본문 언어가 실제로 전환되는 경우와 code block·고유명사가 많은
경우는 계속 live 표본을 늘려야 한다.

## 9. 출판사·평론가 JSON 안정화: 단계 0–4

단계 번호는 이번 Pre-4.0 안정화 작업에서 합의한 작업 순서를 가리킨다.

| 단계 | 목적 | 적용 결과 | 상태 |
|---:|---|---|---|
| 0 | 실패 기준선과 계약 고정 | malformed JSON, timeout, HTTP 오류, 빈 content, schema 오류를 구분하고 저장 불변식 고정 | `DOCUMENTED` |
| 1 | 공급자 응답 정규화 | 문자열 content, text content part, `choice.text` 등 정상 응답 표면을 공통 텍스트로 정규화 | `SOURCE`, `REGRESSION` |
| 2 | 공통 JSON 추출·파싱 | code fence·주변 텍스트·정상 JSON 객체를 공통 parser로 처리하고 mismatched/truncated는 명시적 실패 | `SOURCE`, `REGRESSION` |
| 3 | 부분 수용과 실패 판정 | 독립 후보는 국소 검증·제외하고, 전체 JSON을 신뢰할 수 없을 때만 전체 평론가 실패 | `SOURCE`, `REGRESSION`, 최근 `LIVE` |
| 4 | 재처리·source revision | 평론가 실패 시 파생 저장 0, 재처리 1; stale·중복 source가 current를 덮지 못함 | `SOURCE`, `REGRESSION`, 일부 `LIVE` |

### 9.1 출판사

- 호출은 정확히 한 번이다.
- 실패해도 본문 요청은 계속하는 fail-open을 유지한다.
- malformed 응답을 기본 지시문으로 위장하지 않는다.
- `PUBLISHER_LLM_MALFORMED_FAILED_OPEN`, timeout, HTTP 오류, empty content를 구분한다.
- 특정 모델만 통과하도록 모델명 allowlist를 만들지 않는다.
- 현재 3.9 bounded Publisher의 안정화와 4.0 `book_author + director` 역할 복원은 별도다.

### 9.2 평론가

- HTTP 200이어도 bracket가 맞지 않거나 필수 root를 읽을 수 없으면 parse 실패다.
- 잘린 JSON 뒤를 추측해 닫거나 없는 값을 만들어 저장하지 않는다.
- root가 유효하면 nested 후보를 각각 검증해 정상 후보는 보존한다.
- 선택 항목이 없다는 이유로 전체 memory 생성을 0으로 만들지 않는다.
- 실제 displayed final 원문을 기준으로 저장하고 hidden reasoning을 원문으로 사용하지 않는다.
- 최초 호출에서 사용한 bounded input snapshot을 같은 source revision 재처리에 재사용한다.

### 9.3 관찰된 대표 오류

- Vertex Gemini HTTP 200 + `CRITIC_JSON_PARSE_FAILED: critic_json_mismatched_brackets`
- Publisher JSON malformed + 본문 계속 진행
- LLM Gateway GPT-5.6-Luna 502 / context deadline exceeded
- Ollama DeepSeek V4 90초 timeout

앞의 두 오류는 응답 형식 문제이고, 뒤의 두 오류는 provider-call timeout이다. timeout을 JSON parser
실패로 기록하거나, JSON 구조를 고쳤으니 timeout도 해결됐다고 판정하면 안 된다.

## 10. 단계 5–7: 공급자 중립 통합과 체크포인트

| 단계 | 작업 | 결과 |
|---:|---|---|
| 5 | Publisher·Critic 공통 provider 응답·오류 경계 통합 | 공급자별 wire option과 공통 본체 parser를 분리 |
| 6 | 재처리·recovery·HUD와 source revision 연결 | 동일 입력 재현, stale 결과 차단, recovery 상태 표시 |
| 7 | 회귀 행렬과 로컬 보존 | 표적·통합 테스트 후 `eed62ae` commit 생성 |

이 단계에서도 GLM, DeepSeek, Kimi, Gemini, MiniMax, Grok, GPT, Claude 이름을 평론가 본체
정책 allowlist로 사용하지 않았다. 공급자마다 다른 request option은 adapter가 처리하고, 응답 text와
JSON 의미 계약은 공통 경로가 처리한다.

### 10.1 recovery 충돌

실사용 복구 요청에서 다음 오류가 재현됐다.

```text
409 recovery_source_not_unique
exactly one active source revision is required for this turn
```

수정 범위는 다음 세 부분이었다.

1. 복구 대상 턴의 active source revision을 정확히 하나로 정규화
2. `/turn-workflow/recovery`가 성공·실패 모두 HUD ViewModel 계약에 맞게 응답
3. 재처리 job이 정확한 source revision과 저장 input snapshot을 사용하도록 연결

재처리 시 외부 LLM Gateway 자체의 HTTP·timeout 오류와 Archive Center의 source-not-unique 오류는
서로 다른 실패다.

## 11. 단계 8: 정리와 기존 owner 축소

단계 8의 목표는 새 기능 추가가 아니라 단계 0–7 뒤에 남은 중복·불용 경로를 제거하고 기존 owner를
한 경로로 유지하는 것이었다.

- `group_turn.go`, `group_turn_complete.go`, world-rule backfill의 확인된 불필요 경로를 삭제했다.
- Publisher·Critic의 중복 helper와 중복 조건 일부를 기존 함수 안에서 정리했다.
- JS에 backend 정책 fallback을 새로 만들지 않았다.
- 사용자 지적 뒤 `group_proxy.go`가 커졌다는 사실만으로 정리 완료를 선언하지 않고, production과
  test 증감을 따로 확인했다.

현재 checkpoint 이후 전체 production+test diff는 순감소가 아니다. 테스트가 크게 늘었기 때문이다.
단계 8의 성공 기준은 저장·파싱 경로의 단일 owner와 불용 production 경로 삭제이며, 저장소 전체 줄 수
감소가 아니다. 확인되지 않은 코드를 줄 수를 맞추기 위해 삭제하지 않았다.

## 12. 단계 9: 평론가 경량화와 정확도 보존

### 12.1 적용한 작업

- `critic_system.txt`의 중복 설명과 장황한 예시를 압축했다.
- checkpoint 대비 prompt diff는 `+92 / -185`, 순 `-93`줄이다.
- 현재 prompt는 물리 113줄, 14,722자다.
- 현재 턴 원문은 보존하고, 과거 전체 chat을 매번 보내지 않는다.
- 직전 canonical 문맥과 backend가 선택한 관련 기억·근거·관계·상태·세계 규칙만 bounded input으로 조립한다.
- 선택적 배열은 sparse하게 반환할 수 있으며, 없는 선택 항목을 억지로 만들지 않는다.
- 각 후보를 독립 검증해 정상 후보 저장을 유지한다.
- 다중 평론가 호출은 도입하지 않았다.

### 12.2 핵심 다섯 파일의 checkpoint 이후 diff

| 파일 | 추가 | 삭제 | 의미 |
|---|---:|---:|---|
| `group_proxy_test.go` | 211 | 61 | 다중 provider 요청·응답·추론 회귀 확장 |
| `proxy_provider.go` | 189 | 113 | 공급자·endpoint별 wire option 정규화 |
| `turn_extraction_critic.go` | 150 | 227 | 평론가 parser·부분 수용·입력 경계 정리 |
| `turn_extraction_critic_test.go` | 323 | 35 | malformed·부분 수용·언어·회귀 확대 |
| `critic_system.txt` | 92 | 185 | 고정 지시 압축 |

이 다섯 파일은 총 `+965 / -621`이므로 파일 총량이 줄지 않았다. production 평론가 본체와 prompt는
줄었지만 provider 기능과 회귀 테스트가 늘었다. 따라서 단계 9의 성과를 단순 파일 크기 감소라고
설명하면 부정확하다. 정확한 표현은 `요청·prompt 압축 + 부분 수용 안정화 + 공급자 wire 호환 회귀 확대`다.

### 12.3 아직 남은 비용

- 최근 성공 평론가도 약 44초가 걸렸다.
- Ollama DeepSeek V4와 LLM Gateway Luna는 90초 전후 timeout 표본이 있다.
- prompt 축소만으로 provider의 내부 reasoning 시간을 완전히 강제할 수 없다.
- 다중 호출로 나누면 각 요청은 작아질 수 있지만 비용, 불일치, 부분 실패, 합성 owner가 늘어난다.

따라서 현재는 단일 평론가를 유지한다. 다중 호출은 단일 호출의 입력·추론 제어를 충분히 검증한 뒤에도
여러 공급자에서 반복 timeout이 재현될 때 별도 설계 항목으로 다시 검토한다.

## 13. 공급자·endpoint별 추론 제어

### 13.1 공통 원칙

- 모델명 allowlist가 아니라 `provider + normalized endpoint + protocol capability + model family/version`
  조합으로 wire 설정을 정한다.
- UI는 backend가 제공한 선택 가능 값과 저장값을 표시한다.
- unsupported 값을 모든 OpenAI-compatible endpoint에 무조건 보내지 않는다.
- 설정을 생략하는 것이 `추론 끄기`와 같은지 provider별로 구분한다.
- reasoning과 final output이 같은 token pool을 쓰는 모델은 둘의 예산을 함께 고려한다.

### 13.2 DeepSeek

- DeepSeek V4는 reasoning과 final output이 같은 `max_tokens`를 공유할 수 있다.
- thinking 사용 시 completion과 reasoning 예산을 합쳐 요청 상한을 계산한다.
- 별도 `reasoning_budget_tokens` 필드를 지원하지 않는 endpoint에는 보내지 않는다.
- Ollama의 지원 형태와 DeepSeek 공식 endpoint의 지원 형태를 같은 것으로 가정하지 않는다.
- 프롬프트의 “과도한 추론을 멈춰라” 지시는 보조 수단이며 provider의 실제 reasoning budget을 보장하지 않는다.

### 13.3 GLM

- GLM 5.2 이상 직접 endpoint: `thinking.type`과 지원되는 `reasoning_effort` 형태를 사용한다.
- 이전 GLM 세대: 세밀한 low/medium 대신 enabled/disabled toggle 계약을 사용한다.
- Ollama의 GLM 5.2 이상: 실제 노출 capability에 맞춰 none/high 계열을 표시한다.
- Ollama의 이전 GLM: enable/disable 계열로 처리한다.
- 모든 GLM 5.0, 5.1, 4.7이 5.2와 같은 effort 값을 지원한다고 가정하지 않는다.

### 13.4 다른 공급자

GPT, Claude, Gemini, Kimi, MiniMax, Grok, LLM Gateway, OpenRouter 등도 공통 parser를 사용한다.
각 공급자의 reasoning, service tier, JSON response format, max completion option은 기존 provider adapter가
지원 capability에 맞춰 보낸다. 특정 모델에서 한 번 성공했다는 결과를 다른 모델의 성공 증거로 사용하지 않는다.

## 14. 토큰과 context 관리

현재 문서화된 동작은 다음과 같다.

- 출판사와 평론가는 각각 별도의 max-completion·reasoning 설정과 호출 예산을 가진다.
- 기억 주입은 Go backend가 lane별 char budget으로 관리한다.
- RisuAI가 이미 사용한 prompt context를 관찰하고 남은 범위 안에서 기억을 조립한다.
- 평론가는 현재 visible turn과 bounded support context만 받으며 과거 전체 chat을 반복 전송하지 않는다.
- 출판사, 본문, 평론가 등 모든 호출을 합산한 장기 누적 context 총량 상한은 아직 별도 제품 계약으로 없다.

마지막 항목은 provider의 단일 요청 context limit과 다른 개념이다. 지금은 각 호출을 독립적으로 제한하며,
세션 전체 비용·누적 token을 하나의 quota로 차단하는 기능은 `OPEN`이다.

## 15. 수정·삭제·원본 계보

- canonical 기준은 RisuAI에 실제 표시·저장된 visible user/assistant 원문이다.
- provider hidden reasoning이나 평론가 sanitize 결과를 canonical 원문으로 대체하지 않는다.
- reroll·edit·delete는 active source revision과 displayed-final 계보를 유지해야 한다.
- 직접 근거 수정은 원문 근거와 상태를 함께 보존하는 기존 API를 사용한다.
- 삭제는 감사가 필요한 행을 무조건 물리 삭제하기보다 tombstone·active 상태와 파생 정리를 사용한다.
- 잘못된 기억을 사용자가 고칠 수 있어야 하지만, UI가 임의로 DB 관계를 다시 계산하지 않는다.

수정 기능은 관련 memory·direct evidence 표면에서 점검했으나, 모든 기억 종류의 실제 UI 편집 →
MariaDB 반영 → Chroma 재색인 → 다음 턴 전달까지를 한 행렬로 재검증하는 작업은 남아 있다.

## 16. 검증 기록

### 16.1 통과 기록

이번 단계 묶음에서 기록된 주요 검사는 다음과 같다.

- `node --check "Archive Center.js"`: 통과
- `go test ./cmd/js-route-variant-smoke`: 통과
- `go test ./internal/httpapi`: 통과
- Publisher 단일 호출·fail-open·empty/malformed 분류 표적 회귀: 통과
- Critic common response normalization·JSON parse·부분 수용 표적 회귀: 통과
- source revision·recovery·reprocessing 표적 회귀: 통과
- output-language 평론가 계약 회귀: 통과
- DeepSeek·Ollama·GLM 요청 option 회귀: 통과
- 소스와 최신 패키지 `Archive Center.js` SHA-256 일치: 통과
- 최신 패키지 ZIP 외부 SHA-256 대조: 통과
- 최근 MariaDB admission과 비동기 Vector outbox 완료: 실사용 확인

### 16.2 전체 Go suite 주의

전체 Go suite에서는 `runtime-dependency-live-probe`가 환경의 `Get-FileHash` 부재로 실패한 기록이 있다.
이는 평론가 parser 실패로 분류하지 않는다. 표적 `internal/httpapi` 회귀 통과와 전체 운영 스크립트
통과를 같은 증거로 합치지 않는다.

### 16.3 아직 충분하지 않은 검증

- 공식 DeepSeek, Ollama DeepSeek, GLM 여러 세대, GPT, Claude, Gemini, Kimi, MiniMax, Grok의 동일 장기 행렬
- 30턴 이상에서 평론가 입력 크기·지연·비용 추세
- provider timeout 뒤 수동 recovery와 재처리의 실제 성공·중복 호출 0 확인
- 관계 상태가 정상 장면에서 계속 0이 아닌지
- branch → descendant branch → edit/reroll/delete → 재시작 조합
- 모든 편집 가능한 기억 종류의 저장·재색인·다음 턴 전달
- loaded plugin이 최신 package JS hash인지 RisuAI 내부에서 직접 확인
- 공개 배포와 OS별 신규 설치·업데이트

## 17. 의도적으로 하지 않은 작업

- 평론가를 두 번 이상 자동 호출하는 구조
- 다른 모델로 자동 fallback
- malformed·truncated JSON 합성 복구
- 실제 공급자 호출의 hidden retry
- 특정 모델명 allowlist
- 새 기억 DB·새 graph DB·새 table
- JS의 turn 계산·branch 판정·기억 선택·예산 정책
- 로어북을 일반 사용자 요청의 대체 본문으로 사용
- 로어북 직접 언급이 없으면 기능 전체를 off
- 하나의 선택 항목이 없을 때 전체 평론가 결과 폐기
- 줄 수 감소를 위한 검증되지 않은 production 삭제
- 4.0의 `book_author + director`를 3.9 안정화에 섞는 작업

## 18. 현재 알려진 결함·한계

| 항목 | 현재 상태 | 다음 판정 |
|---|---|---|
| 평론가 지연 | 성공 턴도 44초, 일부 provider 90초 timeout | 같은 입력·설정의 여러 provider 표본 수집 |
| 관계 상태 0건 | 최근 13–14턴에서 0 | 실제 관계 변화 장면과 안정 방향 ID 검사 |
| 성공 감사의 provider 정보 | model·provider·reasoning effort를 성공 행에서 역추적하기 어려움 | 새 schema 없이 기존 trace에서 보존 가능한지 별도 검토 |
| 다중 branch | 기본 lineage 수정됨 | grandchild·재시작·edit/reroll/delete live 행렬 필요 |
| UI | 세계선은 수용 가능, 다른 화면은 여전히 이전 틀이 큼 | 다음 전면 UI 재정비 |
| JS 크기·메모리 | checkpoint 이후 JS `+147/-39` | DOM mount·listener·cache 실측 후 UI 함수 교체 중심 정리 |
| 편집 | 관련 경로 점검 | 모든 기억 종류 end-to-end 행렬 필요 |
| 장기 누적 token quota | 없음 | 비용 HUD 또는 backend quota는 별도 제품 결정 |
| aggregate Event recall | 별도 owner 혼선 결함이 4.0 계약에 기록됨 | 승인되지 않은 preview patch를 임의 적용하지 않음 |
| 외부 공급자 범위 | 일부 실제 성공·timeout 표본만 존재 | provider별 성공을 독립 검증 |

## 19. 단계 8–9 기존 구현 변경 파일 원장

다음 수치는 이 문서 작성을 시작하기 직전의 `eed62ae` 대비 runtime·test·prompt tracked diff다.
이번 종합 원장과 기존 상태 장부의 링크 5줄은 포함하지 않았다. 테스트 파일 증가는 production 무게
증가와 구분해서 읽는다.

| 파일 | + | - |
|---|---:|---:|
| `Archive Center.js` | 147 | 39 |
| `go-service/cmd/js-route-variant-smoke/main_part10_test.go` | 85 | 10 |
| `go-service/internal/httpapi/character_profile_voice_test.go` | 20 | 11 |
| `go-service/internal/httpapi/complete_turn_source_acceptance_test.go` | 2 | 2 |
| `go-service/internal/httpapi/complete_turn_source_revision_test.go` | 10 | 6 |
| `go-service/internal/httpapi/group_admin_part03_test.go` | 2 | 1 |
| `go-service/internal/httpapi/group_admin_rescan_queue_test.go` | 2 | 1 |
| `go-service/internal/httpapi/group_admin_test.go` | 18 | 14 |
| `go-service/internal/httpapi/group_admin_world_rule_backfill.go` | 0 | 1 |
| `go-service/internal/httpapi/group_narrative_seq15_part02_test.go` | 1 | 1 |
| `go-service/internal/httpapi/group_proxy.go` | 44 | 56 |
| `go-service/internal/httpapi/group_proxy_part02_test.go` | 12 | 25 |
| `go-service/internal/httpapi/group_proxy_test.go` | 211 | 61 |
| `go-service/internal/httpapi/group_supervisor_boundary_test.go` | 101 | 55 |
| `go-service/internal/httpapi/group_turn.go` | 0 | 1 |
| `go-service/internal/httpapi/group_turn_complete.go` | 0 | 4 |
| `go-service/internal/httpapi/group_turn_part02_test.go` | 15 | 33 |
| `go-service/internal/httpapi/group_turn_part03_test.go` | 10 | 10 |
| `go-service/internal/httpapi/group_turn_part04_test.go` | 12 | 5 |
| `go-service/internal/httpapi/group_turn_part07_test.go` | 4 | 4 |
| `go-service/internal/httpapi/group_turn_part08_test.go` | 1 | 1 |
| `go-service/internal/httpapi/group_turn_part12_test.go` | 2 | 2 |
| `go-service/internal/httpapi/group_turn_part13_test.go` | 27 | 63 |
| `go-service/internal/httpapi/group_turn_test.go` | 3 | 1 |
| `go-service/internal/httpapi/habit_evidence_test.go` | 17 | 9 |
| `go-service/internal/httpapi/memory_admission_worker_test.go` | 4 | 5 |
| `go-service/internal/httpapi/output_fidelity_guide_efficacy_test.go` | 13 | 11 |
| `go-service/internal/httpapi/proxy_provider.go` | 189 | 113 |
| `go-service/internal/httpapi/publisher_plan_test.go` | 40 | 53 |
| `go-service/internal/httpapi/rescan_long_session_test.go` | 4 | 4 |
| `go-service/internal/httpapi/reversible_state_test.go` | 2 | 4 |
| `go-service/internal/httpapi/story_clock_test.go` | 2 | 3 |
| `go-service/internal/httpapi/turn_extraction_critic.go` | 150 | 227 |
| `go-service/internal/httpapi/turn_extraction_critic_test.go` | 323 | 35 |
| `prompts/critic_system.txt` | 92 | 185 |
| `prompts/supervisor_system.txt` | 17 | 24 |

## 20. 다음 UI 작업의 출발점

다음 작업은 backend 계약을 다시 뜯는 단계가 아니라 현재 backend ViewModel을 더 가볍고 명확하게
표현하는 UI 재정비다.

권장 순서는 다음과 같다.

1. 상단 `세계선 | 기억 | 기억 관리 | 추가 기능 | 설정` shell과 공통 session scope를 고정한다.
2. 사용자가 수용한 세계선 canvas는 유지하고 node 선택·인라인 확장만 다시 설계한다.
3. 기억 화면을 요약 목록 → 선택 항목 인라인 확장 → 편집 상태의 세 단계로 단순화한다.
4. 기억 관리는 연결·이동·복사·삭제와 DB 상태를 한곳에 모은다.
5. 로어북·reference DB·페르소나 캡슐·HypaMemory는 추가 기능으로 분리한다.
6. 열린 항목만 상세 DOM을 만들고, 닫힌 항목은 작은 요약 ViewModel만 유지한다.
7. backend 재호출, DOM node 수, event listener 수, JS heap을 전후 비교한다.
8. 기능별로 교체한 뒤 기존 UI path를 같은 작업에서 삭제한다. 병렬 UI를 누적하지 않는다.

UI 완료 판정은 단순 색상 변경이 아니다. 화면 구조가 달라지고, node에서 세부 항목이 직접 확장되며,
세션 범위가 모든 탭에서 일치하고, 수정 가능한 값이 실제 저장되며, 긴 세션에서도 브라우저 메모리가
통제돼야 한다.

## 21. 후속 기록 규칙

이 원장은 과거 기록을 덮어써서 상태를 좋게 보이게 만들지 않는다.

1. 새 작업은 날짜, 목적, 변경 파일·함수, `+/-`, 테스트, package, live 결과를 각각 기록한다.
2. 실패도 provider, HTTP status, pipeline stage, source revision, 재처리 결과와 함께 남긴다.
3. 이전 결론이 틀렸다면 원문을 삭제하지 말고 `정정` 항목으로 이유와 새 증거를 붙인다.
4. source 검증과 live 검증을 같은 완료 표시로 합치지 않는다.
5. 새 package를 만들거나 기존 package를 갱신하면 폴더·ZIP·핵심 JS·prompt·binary hash를 남긴다.
6. UI 작업은 JS 추가·삭제 줄 수와 backend로 남긴 정책이 있는지 반드시 보고한다.
7. 외부 공급자 성공은 공급자·endpoint·model·reasoning 설정·elapsed·결과를 확인할 수 있을 때만 기록한다.

## 22. 연결 문서

- [Archive Center 3.6–3.9 구현·문서·개인 테스트 상태 장부](3.6-3.9-implementation-and-personal-test-status.md)
- [Archive Center 3.9.0 피드백 작업 로그](archive-center-3.9.0-feedback-work-log.md)
- [Archive Center 4.0 RisuAI 세계선 관찰 설계](4.0-risu-worldline-observation-design.md)
- [Archive Center 4.0 기억 복원 작업 계약](4.0-memory-restoration-work-contract.md)
- [영구 RisuAI Host / Backend 경계](permanent-risu-host-backend-boundary.md)
- [Provider request override 계약](provider-request-overrides-flex-paygo-contract.md)
- [3.7 평론가 파생 기억 재처리 runbook](3.7-critic-derived-memory-reprocessing-runbook.md)

이 문서는 현재 작업을 재개하기 위한 정본 기록이지, Pre-4.0.0 공개 릴리스 완료 선언이 아니다.

## 23. 기억 관리 세션 작업 공간 1차 재구성

### 목적

기억 관리의 분리된 세션 선택창과 유지보수 카드 묶음을 다음 구조로 교체했다.

- 왼쪽: backend Presentation ViewModel의 세션을 작은 카드로 표시하는 세션 레일
- 오른쪽: 선택한 세션 이름·건수·연결·복사·이동·삭제와 유지보수 도구를 한 작업 공간에 표시
- 세션 카드를 누르면 기존 `selectWorkspaceSession(..., "memory_admin")` 경로로 범위 변경
- 세션 정상화는 우측 선택 세션 작업 공간의 첫 유지보수 항목으로 이동
- 정상화 기본 상태는 접힘이며 실행·결과·오류 상태에서는 기존 상태가 패널을 열어 진행 상황을 표시
- 세션 레일에는 별도의 내부 스크롤 영역을 만들지 않았고 세계선 canvas도 복제하지 않음

### 소유권과 변경 범위

- Go backend의 세션 capability, 연결·복사·이동·삭제·정상화 API와 저장 의미는 변경하지 않았다.
- `Archive Center.js`는 기존 ViewModel을 카드와 작업 공간으로 투영하고 기존 action을 전달하는 DOM 역할만 맡는다.
- 새 API, 새 backend 상태, 새 테이블, 새 세션 정책은 추가하지 않았다.
- `세계선`, `기억` 열람 화면, 추가 기능의 데이터 계약은 변경하지 않았다.

### 파일과 변경량

이 UI slice 직전 작업 상태와 비교한 값이다.

| 파일 | + | - |
|---|---:|---:|
| `Archive Center.js` | 104 | 31 |
| `go-service/cmd/js-route-variant-smoke/main_part09_test.go` | 13 | 5 |

### 검증

- `node --check "Archive Center.js"`: 통과
- `go test ./cmd/js-route-variant-smoke -run "TestArchiveCenterJS(TimelineWorldlineCanvasPreservesCompactOperations|MemoryWorkspaceAndEditRuntime)$" -count=1`: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 통과
- `git diff --check`: 새 whitespace 오류 없음

### 아직 검증하지 않은 항목

- 실제 RisuAI에 이 JS를 다시 불러온 화면의 시각 검수
- 매우 많은 세션을 가진 실제 DB에서의 세로 길이·DOM/heap 측정
- test package 갱신 또는 backend 재빌드·재시작

이 항목은 source와 회귀 검증 완료 기록이며 package/live 완료 선언이 아니다.

## 24. 입력 개선 LLM의 검토 후 입력 재작성 복구

### 목적과 사용자 동작

새 재작성 체계를 추가하지 않고 기존 `beforeRequest` 입력 개선 경로를 다시 사용할 수 있게 했다.
설정의 출판사 설정 구역에서 `입력 개선 LLM (선택)`을 다음 세 모드로 선택한다.

- `꺼짐`: 입력 개선 LLM을 호출하지 않는다.
- `검토만`: 입력 개선 결과와 trace를 만들지만 메인 모델 요청은 원래 사용자 입력을 유지한다. 기본값이다.
- `검토 후 입력 재작성`: 이 선택 자체를 명시적 허용으로 간주한다. 기존 판정이
  `approve`, `partial`, `first-pass-only` 중 하나이고 실제 개선문이 있을 때만 메인 모델로 보낼
  마지막 사용자 메시지를 교체한다.

정본 내부 이름은 `pluginMainRewriteOptIn`이다. 사용자 설정으로 따로 노출하지 않으며 설정 정규화 시
`reviewed_apply` 선택에서만 `true`가 되고, `꺼짐` 또는 `검토만`에서는 항상 `false`로 정리된다.
이전 저장 설정의 폐기된 이름은 정규화 과정에서 제거하며 그 값은 선택된 apply mode를 덮어쓰지
않는다. 따라서 숨겨진 기본값 때문에 사용자가 재작성 모드를 골라도 `검토만`으로 되돌아가던
상태를 제거했다.

### 호스트와 backend 경계

- RisuAI에 저장된 원문 채팅은 수정하지 않는다.
- `Archive Center.js`는 실제 메인 모델 호출 직전의 outgoing payload에서 마지막 user 메시지만
  기존 `rewriteLastUserMessage`로 교체한다.
- 공식 RisuAI Plugin API의 고정 커밋
  [`846c897`](https://github.com/kwaroran/RisuAI/blob/846c897bf56e3e0121ef12e7ba07f5c3912dfa78/src/ts/plugins/apiV3/risuai.d.ts#L1677-L1706)에서
  `beforeRequest` replacer가 `OpenAIChat[]`을 받아 수정된 `OpenAIChat[]`을 반환하는 계약을
  2026-08-22에 다시 확인했다.
- 이번 slice는 기존 입력 개선 호출, 판정, trace와 fail-open 경로를 재사용했다. 새 LLM 호출,
  새 backend API, 새 저장 필드, 새 테이블, 새 fallback 또는 hidden retry는 추가하지 않았다.
- 재작성 실패나 빈 개선문에서는 원래 payload를 유지한다.

### UI 배치

입력 개선 모드는 공통 설정에서 빼고 서사 가이드 모드·강도·표현 형식·예산과 같은
`출판사 설정` 카드의 마지막 항목으로 옮겼다. 재작성 시 저장 원문과 outgoing 요청의 차이를
사용자가 오해하지 않도록 한·영·일 안내 문구를 추가했다.

### 파일과 변경량

이 slice 직전 작업 상태와 비교한 값이다.

| 파일 | + | - |
|---|---:|---:|
| `Archive Center.js` | 19 | 17 |
| `go-service/cmd/js-route-variant-smoke/main_test.go` | 5 | 0 |
| `go-service/cmd/js-route-variant-smoke/main_part02_test.go` | 40 | 0 |
| `go-service/cmd/js-route-variant-smoke/main_part09_test.go` | 1 | 0 |

JavaScript production 변경은 `+19/-17`, 순증가 2줄이다. Go production은 변경하지 않았다.

### 검증

- `node --check "Archive Center.js"`: 통과
- 입력 개선 mode·runtime·payload rewrite·UI 위치 표적 회귀 5개: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 통과

runtime 회귀는 production의 `getApplyModeGate`와 `applyModeAllowsApply`를 직접 추출해 다음을
검증한다.

- `reviewed_apply`: 개선 호출 실행, 허용 verdict만 재작성 가능
- `shadow`: 개선 호출 실행, 승인 verdict여도 재작성 불가
- `off`: 개선 호출 자체를 건너뜀
- 내부 허용값이 없는 `reviewed_apply`: 재작성 불가

### 아직 검증하지 않은 항목

- 실제 RisuAI 설정 화면에서 모드를 바꾸고 다시 열었을 때의 시각·저장 검수
- 실제 공급자 입력 개선 호출 뒤 메인 모델이 받은 마지막 user 메시지의 live trace 대조
- test package 갱신 또는 backend 재빌드·재시작

이 항목은 source와 회귀 검증 완료 기록이며 package/live 완료 선언이 아니다.

### 24.1 재작성 내부 이름 정리

입력 재작성을 정식 선택 기능으로 복구한 뒤 내부 정본 이름에서도 `Legacy`를 제거했다.

- 정본 설정 필드: `pluginMainRewriteOptIn`
- 기존 저장 키: 정규화 과정의 삭제 대상으로만 1회 참조하며 실행 판정에는 사용하지 않음
- 기존 저장 키의 값과 관계없이 `reviewed_apply` 선택만 정본 opt-in을 결정
- 설정을 다시 저장하면 폐기된 키는 영속 설정에서도 제거됨

이 후속 slice의 production JavaScript 변경량은 `+9/-8`이다. Go production 변경은 없다.
표적 회귀와 전체 `js-route-variant-smoke`를 다시 통과했으며, production 소스에서 폐기된 이름이
정리 대상 1곳 외에 남으면 실패하는 회귀를 추가했다.

## 25. 로어북 목록을 추가 기능으로 이관

### 화면 구조와 동작

- `기억`의 좌측 항목에서 `보조 참조`를 제거했다. 이전 저장 UI 상태가 `lorebook` 탭을 가리켜도
  backend Presentation ViewModel이 `대화 원문`으로 정규화한다. 최신 JS가 구버전 backend와
  일시적으로 함께 실행돼 `lorebook` 탭 행을 받더라도 기억 화면의 UI projection에서 제외한다.
- `추가 기능 → 로어북` 상단에 공통 세션 선택창을 배치했다. 선택 변경은 세계선·기억과 같은
  inspection session 상태를 사용한다.
- 같은 화면에서 선택 세션의 저장 건수, 최근 관찰 시각, 20개 단위 목록, 항목 펼치기와 추가
  불러오기를 제공한다.
- 현재 RisuAI 채팅은 `로어북 새로고침`으로 Host를 다시 관찰할 수 있다. 보관된 과거 세션은
  Host 현재 범위를 덮어쓰지 않고 DB에 이미 저장된 목록만 열람하므로 새로고침 버튼을 비활성화한다.
- 저장된 로어북이 없는 세션은 조회 오류가 아니라 0건의 빈 목록으로 표시한다.

### backend 소유권

- 기존 `GET /sessions/{chat_session_id}/lorebook-reference/current`에 UI 열람용
  `scope_mode=latest_session`을 추가했다. MariaDB가 선택 세션에서 가장 최근에 관찰·저장된
  scope를 고르고 기존 bounded exact-scope reader로 20개씩 읽는다.
- 메인 모델과 평론가의 prepare-turn 경로는 기존 exact Host scope를 계속 사용한다. 이번 변경은
  로어북의 기억 주입 조건, 검색 정책, 저장 수명 주기 또는 출력 언어 계약을 바꾸지 않았다.
- 새 route, table, schema, cache, hidden retry 또는 병렬 로어북 저장소를 추가하지 않았다.
- `Archive Center.js`는 선택한 session ID와 paging 값만 전달하고 backend가 반환한 ViewModel을
  DOM으로 표시한다. character/chat/module scope를 JavaScript에서 재구성하지 않는다.

### 파일과 변경량

이 slice 직전 작업 상태와 비교한 값이다.

| 파일 | + | - |
|---|---:|---:|
| `Archive Center.js` | 111 | 85 |
| `go-service/internal/store/lorebook_reference.go` | 1 | 0 |
| `go-service/internal/store/mariadb_lorebook_reference.go` | 33 | 0 |
| `go-service/internal/httpapi/group_lorebook_reference.go` | 27 | 9 |
| `go-service/internal/httpapi/group_presentation.go` | 10 | 1 |
| `go-service/internal/store/mariadb_lorebook_reference_test.go` | 51 | 0 |
| `go-service/internal/httpapi/group_lorebook_reference_test.go` | 77 | 0 |
| `go-service/internal/httpapi/group_presentation_test.go` | 15 | 3 |
| `go-service/cmd/js-route-variant-smoke/main_part09_test.go` | 38 | 8 |

Production JavaScript는 `+111/-85`, 순증가 26줄이다. 남은 JavaScript 역할은 RisuAI Host의
현재 로어북 관찰·동기화, session ID 전달, 목록 paging 요청, DOM 상호작용뿐이다. 선택 scope와
빈 상태 판정은 backend가 소유한다.

### 검증

- `node --check "Archive Center.js"`: 통과
- `go test ./internal/store ./internal/httpapi -count=1`: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 통과
- exact-scope 조회, 선택 세션 최신 scope 조회, 빈 세션 0건 ViewModel, 잘못된 scope mode 거부,
  기억 탭 제거, 구버전 ViewModel의 로어북 행 UI 차단과 오래된 활성 탭 정규화 회귀: 통과
- `git diff --check`: 새 whitespace 오류 없음

### 아직 검증하지 않은 항목

- 실제 RisuAI에서 여러 보관 세션을 바꿨을 때의 시각·상호작용 검수
- 실제 MariaDB에서 서로 다른 scope가 여러 번 저장된 세션의 최신 목록 대조
- 갱신한 JS의 실제 RisuAI 재불러오기와 backend 재시작

### 25.1 기존 stage9 시험 패키지 갱신

새 패키지 이름을 추가하지 않고 기존
`_test-builds/Pre-4.0.0-stage9-critic-compact-windows-test`를 같은 경로에서 현재 source로
재생성했다.

- package manifest: `green`, `release_ready=true`
- managed manifest: 46개 파일, hash 불일치 0개
- source/package `Archive Center.js` SHA-256:
  `643B29681E0D7F20AF51D5A9AB37C246EACFBB9129F9E5CE17EC48B5D47AB3AE`
- packaged `archive-center-go.exe` SHA-256:
  `6898F861128D1D54A4ECC63194F527C79F2175E355D5949BBEFE8D4E0D08F196`
- ZIP SHA-256:
  `8D76E7241F58C61F197AFA078D14F9A143EEA58CAEC5BE0D2FA378FA80E8A6B5`
- 외부 `SHA256SUMS-Pre-4.0.0.txt`: ZIP과 일치
- package JS 문법 검사: 통과
- package JS의 `explorer.tabs.lorebook.label`: 0개
- 구버전 Presentation ViewModel의 `lorebook` 행을 기억 UI에서 거르는 projection: 1개
- 기존 `.env.full.local`: 빌드 전 보존 후 동일 hash로 복원

폴더를 점유하던 종료 잔여 `01_start_archive_center_windows.bat` launcher PID만 종료했다. 패키지
갱신 뒤 backend는 다시 시작하지 않았으므로 이 기록은 `PACKAGE` 완료이며 `LIVE` 완료 선언이 아니다.

## 26. 2026-08-23 DB 경합 진단과 최소 안정화

### 처리한 범위

3.9.11 사용자가 제보한 간헐적 MariaDB deadlock 가능성을 근거 없이 구조 변경하지 않고 먼저
식별할 수 있도록, `ApplyReversibleStatusTransition`에서 발생하는 MySQL 1213과 1205 오류에 다음
식별자를 붙이도록 했다.

- `mysql_error`, `sql_state`
- `source_revision`, `source_unit_id`
- `status_key`
- `owner_scope`, `owner_id`

오류는 원래 `*mysql.MySQLError`를 `%w`로 감싸므로 `errors.Is`/`errors.As` 계열 판정이 유지된다.
식별자 문자열은 로그가 비정상적으로 커지지 않도록 120 rune으로 제한한다.

### 변경과 고정점

- local checkpoint commit: `eae0b3fba828df70c10ac9aad0ddef31fa040fdb`
- `go-service/internal/store/mariadb_status.go`: `+49/-6`
- `go-service/internal/store/mariadb_status_test.go`: `+55/-0`
- 합계: `+98/-6`
- production 함수: `reversibleStatusTransitionLockDiagnostic`,
  `boundedStatusLockDiagnosticValue`
- 회귀: 1213 deadlock과 1205 lock wait timeout 모두에서 MySQL 오류 번호와 source/status/owner
  식별자가 보존되는지 검증

각 reversible transition이 여는 기존의 작은 트랜잭션 경계는 유지했다. 이미 성공한 항목을 포함한
전체 습관 근거를 하나의 거대한 트랜잭션으로 합치지 않았고, schema나 table도 추가하지 않았다.

### 의도적으로 하지 않은 변경

실제 MariaDB에서 1213이 재현됐다는 증거가 아직 없으므로 다음은 적용하지 않았다.

- `wakeMemoryWorkers()`의 호출 위치 이동
- `ApplyReversibleStatusTransition` 내부의 새 자동 재시도
- 습관 근거 전체의 일괄 transaction화
- lock 순서나 worker 구조의 추측성 변경

현재 `group_turn_complete.go`는 원문과 source revision이 durable해진 직후 여전히
`wakeMemoryWorkers()`를 호출한다. 따라서 이번 commit은 진단과 오류 식별의 최소 slice이며,
deadlock 자체의 해결 완료 선언이 아니다.

### 남은 실 DB 증거

실제 문제가 다시 발생하면 같은 시각의 application log와 `SHOW ENGINE INNODB STATUS`에서 다음을
같이 확보해야 한다.

1. 1213인지 1205인지
2. 위 source revision/unit/status/owner 식별자
3. 맞은편 transaction과 잠근 index/row
4. worker wake 시점

1213이 확인된 경우에만 조기 wake 이동 또는 항목 단위의 제한된 transaction 재시도를 별도
slice로 검토한다.

## 27. 독립 참조 예산과 전체 payload 장부

### 정정된 예산 구조

일반 기억 9,000 chars 안에 원작 DB와 로어북을 억지로 합치는 구조가 아니다. 세 레인은 서로
독립된 ceiling을 가진다.

| 레인 | 기본 설정 상한 | 의미 |
|---|---:|---|
| 일반 기억 | 9,000 chars | 기억·근거·관계·상태 등 Archive Center 기억 |
| 원작 DB | 3,000 chars | 원작 자료 참조 전용 |
| 로어북 | 3,000 chars | RisuAI 기본 주입을 보완하는 추가 참조 전용 |

사용자가 일반 기억 설정을 12,000으로 올린 세션에서는 전체 설정 상한이
`12,000 + 3,000 + 3,000 + 출판사 안내 상한`으로 계산된다. 이는 기본값 자체를 12,000으로
바꿨다는 뜻이 아니다. 각 값은 최대한 채워야 하는 목표량이 아니라 넘지 말아야 하는 상한이다.

2026-08-23의 6,000자 실사용 trace와 당시 장부 수치는 아래에 역사적 관찰로 유지한다. 새 기본값
3,000은 2026-08-24 source와 package부터 적용된다. 이미 저장된 명시적 6,000과 명시적 0은
사용자 값을 보존하며, 기존 저장값이 과거 기본인지 사용자 선택인지 추측해 자동 덮어쓰지 않는다.

### Go 소유 장부

`prepare_turn_render.go`의 `attachPrepareTurnPayloadBudgetLedger`가
`payload_budget_ledger.v1`을 만든다. 장부는 각 레인에 대해 다음을 기록한다.

- 설정 상한과 실제 활성 상한
- 후보 건수·chars
- 선택 건수·chars
- 최종 전달 건수·chars
- 제외 건수와 reason code별 수치
- 제목, lane 사이 구분자, 바깥 구분자의 조립 비용
- 최종 auxiliary system message의 정확한 계획 chars

장부의 정본 범위는 `main_model_auxiliary_system_message`이고 delivery state는
`planned_exact_payload_text`다. `Archive Center.js`는 이 수치를 다시 계산하지 않고 편집 확인과
debug UI에 표시한다.

### 실사용 관찰

사용자가 제공한 첫 trace에서는 일반 기억 설정을 12,000으로 둔 상태에서 다음이 관찰됐다.

- 본문 auxiliary 계획: `18,731 / 설정 상한 25,500`, 활성 상한 22,500
- 일반 기억: `12,008 / 12,000`
- 원작 DB: `0 / 3,000`
- 로어북: `5,794 / 6,000`
- 출판사 안내: `887 / 4,500`
- 조립 비용: 42

합계 `12,008 + 0 + 5,794 + 887 + 42 = 18,731`로 backend 장부와 일치한다. 다만 일반 기억이
설정 12,000보다 8 chars 크게 표시된 것은 strict ceiling 완료 증거가 아니다. 현재 source의
`prepareTurnMemoryPayloadBudgetStats`는 기본 기억 delivery plan 뒤에 reversible-state 보충분을
별도 합산할 수 있으므로, 이 8 chars가 그 보충분인지 실제 cap 위반인지 다음 동일-turn trace에서
확인해야 한다. 이는 원작 DB나 로어북 예산이 일반 기억 예산에 합쳐졌다는 뜻은 아니다.

이 관찰은 backend가 만든 계획 수치의 자기 일관성을 입증하지만, Host가 실제 전송한 최종 payload의
검증 완료를 뜻하지는 않는다. 뒤의 trace에서 pre-request payload verification mismatch가 별도로
관찰됐기 때문이다.

## 28. 출판사·평론가 호출별 예산 장부

### 계약과 소유권

본문 payload와 서로 다른 LLM 호출인 출판사와 평론가는 별도의
`provider_call_budget_ledger.v1`로 기록한다. 새 production 파일
`go-service/internal/httpapi/provider_call_budget_ledger.go`는 142줄이며 다음 함수를 소유한다.

- `newProviderCallBudgetLedger`: system/user/final prompt chars와 구성 요소 계산
- `observeProviderCallBudgetResult`: 성공·실패 stage, HTTP status, termination, provider usage 기록
- `safeProviderCallBudgetLedger`: 재처리 trace에 내용 원문 없이 안전한 수치만 전달

### 기록 항목

- 전체 system prompt, user prompt, final prompt chars
- 현재 turn, auxiliary memory, 언어 context chars
- 원작 DB·로어북의 전달 chars 또는 `not_in_call_contract`
- JSON 출력 요구문의 별도/내장 accounting 상태
- 조립 비용
- 성공 또는 실패 stage와 code
- provider가 실제 보고한 input/output/reasoning/cached/total tokens

provider가 usage를 보고하지 않은 경우 토큰을 chars에서 추정하지 않고 `unreported`로 둔다.
평론가 JSON parse 실패와 provider timeout도 실제 실패 stage에 장부를 붙이며, 재처리 queue에는
프롬프트 원문이 아니라 허용된 수치와 상태만 보존한다.

`group_proxy.go`와 `turn_extraction_critic.go`가 각각 출판사와 평론가 장부를 연결한다.
`Archive Center.js`는 backend 장부를 호출별 UI로 표시할 뿐 provider별 계산이나 실패 판정을
소유하지 않는다.

### 현재 검증 경계

- `publisher_plan_test.go`: 실제 usage 보고와 미보고를 구분하고 미보고 토큰을 추정하지 않음
- `turn_extraction_critic_test.go`: 평론가 장부와 실패 stage 연결
- `memory_admission_worker_test.go`: 재처리 trace의 안전한 장부 보존
- `js-route-variant-smoke`: 출판사·평론가 장부 표시

source와 회귀 검증은 완료됐다. 실제 provider가 usage를 보고하는 성공 호출, usage 미보고 성공
호출, timeout, JSON parse 실패를 모두 UI에서 대조하는 live matrix는 아직 닫지 않았다.

## 29. 관계 기억 과주입의 제한 복원

### 적용한 범위

3.9.11에서 사용하던 `single_endpoint_only` 관계/KG 제외 규칙을 현재 4.0 경로에 복원했다.
현재 입력과 KG triple이 두 endpoint 중 하나만 공유하면 관계 자체를 관련 기억으로 보지 않는다.

- `prepare_turn_recall.go`: `prepareTurnKGRecallEligible`이 `single_endpoint_only` 반환
- `prepare_turn_assembly.go`: 해당 후보를 제외하고
  `kg_single_endpoint_only_dropped` 진단 수치 증가
- `group_turn_part06_test.go`, `group_turn_part05_test.go`,
  `prepare_turn_prompt_hygiene_test.go`: 제외와 count 회귀

branch 상속, 출력 언어, 독립 예산, 개별 기억 구조, 로어북 관찰 계약은 되돌리지 않았다. 중복 제거
전체를 3.9.11로 되돌리지도 않았다.

### 같은 세션의 사용자 관찰 비교

| 항목 | 복원 전 | 복원 후 |
|---|---:|---:|
| 전체 auxiliary 계획 | 18,731 | 14,463 |
| 일반 기억 | 12,008 | 7,512 |
| 주관 기억·관계 lane | 4,354 | 1,343 |
| 로어북 | 5,794 | 5,794 |

일반 기억은 4,496 chars, 주관 기억·관계 lane은 3,011 chars 감소했다. 같은 trace에서 원작 DB는
0, 출판사 안내는 1,115, 조립 비용은 42였고
`7,512 + 0 + 5,794 + 1,115 + 42 = 14,463`으로 장부와 일치했다.

이 결과는 관계 레인의 불필요한 단일 endpoint 후보가 감소했다는 `USER_OBSERVED` 증거다. 다만
trace 상단에 다음 경고가 있었으므로 실제 최종 RisuAI payload parity는 아직 `OPEN`이다.

> The pre-request payload verification did not match.

따라서 이 수치를 “backend 계획과 UI 표시가 감소했다”로 기록하며, Host가 실제 전송한 최종
payload까지 검증됐다고 확대 해석하지 않는다.

## 30. 로어북 보조 참조의 당시 방향 — `SUPERSEDED 2026-08-24`

이 section은 2026-08-23의 중간 선택 정책과 그 근거를 보존한 역사 기록이다. 아래의
`Always Active` 일괄 Host 보류, lexical overlap 2 이상, weak lexical 보류, 6,000자 상한은 현재
source 정책이 아니다. 사용자가 기존 보수 조건 전체를 먼저 삭제하도록 지시한 뒤 선택기를 다시
작성했으며, 최신 정본은 sections 36–41을 따른다. 과거 trace와 실패 원인을 잃지 않기 위해 본문을
삭제하거나 현재형으로 다시 쓰지 않는다.

### 문제를 보여준 trace

관계 기억 감소 뒤에도 로어북 수치는 그대로였다.

- 후보: 104,948 chars
- 선택: 28,486 chars
- 최종: 5,794 / 6,000 chars
- 제외: 84건
  - `lorebook_char_budget=34`
  - `lorebook_duplicate_suppressed=50`

실제 출력에는 현재 조선 배경과 관계없는 일본 전국시대 항목도 포함됐다. 따라서 6,000 chars를
상한이 아니라 채우는 목표처럼 쓰는 기존 선택 결과를 그대로 둘 수 없었다.

### 확정한 제품 의미

RisuAI는 활성 로어북을 자체적으로 본문 요청에 주입한다. Archive Center의 로어북 경로는 그
정본 주입을 복제하는 주력 기억 경로가 아니라, 빠졌을 수 있는 관련 정보만 보충하는 보조 참조다.

이에 따라 현재 source의 `finalizePrepareTurnLorebookReference`는 다음 순서로 동작한다.

1. 현재 user input, RisuAI Host message, 이미 조립된 Archive Center context와 완전 동일한 항목을
   `suppressed_native_duplicate`로 제외한다.
2. `Always Active`가 포함된 동일 내용 그룹은 RisuAI Host 소유로 간주하고
   `deferred_native_always_active_owned_by_host`로 보류한다.
3. 일반 항목은 key가 직접 맞거나 lexical token overlap이 2 이상일 때만 보조 후보로 전달한다.
4. 한 token만 겹치는 약한 lexical 후보는 `deferred_weak_lexical_relevance`로 보류한다.
5. 활성화된 보조 후보에 대해서만 6,000 chars 상한을 적용한다. 빈 공간을 채우기 위한 후보 승격은
   하지 않는다.

### 관찰 계약

`prepare_turn_lorebook_reference.go`의 `lorebook_selection_observation.v1`은 다음을 기록한다.

- candidate count와 candidate chars
- key match와 matched key
- lexical overlap
- Always Active candidate/activated/delivered count
- native duplicate source별 count
- 최종 disposition과 delivered 여부
- budget 또는 activation 때문에 보류된 수치

JS는 이 backend 관찰값을 그대로 표시한다. 선택 조건, 우선순위, budget은 Go가 소유한다.

### 회귀와 잔여 위험

- key 직접 활성화: 유지
- strong lexical 보조 참조: 유지
- weak lexical 보류: 검증
- Always Active candidate는 관찰되지만 activated/delivered 0: 검증
- native exact duplicate 억제: 유지
- 별도 schema, table, cache, hidden retry: 추가하지 않음

현재 tokenizer는 모든 영숫자 token을 동일하게 세므로 `the`, `is` 같은 흔한 두 token만 겹치는
영문 항목이 strong lexical로 오인될 가능성은 남아 있다. 그러나 이 가능성만으로 stopword 목록,
희소도 점수, 언어별 사전, 추가 단계 규칙을 넣지 않는다. 최신 source로 실제 한 turn trace를 먼저
확인하고, false positive가 재현될 때에만 기존 lexical 조건 한 곳을 최소 수정한다.

이것이 2026-08-23 현재 확정 방향이다. “최대한 채우기”도 하지 않고, “조금 불확실하면 모두
제외”하는 과도한 보호 정책도 추가하지 않는다.

## 31. 2026-08-23 누적 변경량과 검증

### `eae0b3f` 이후 production 누적 diff

다음은 여러 최근 slice가 같은 dirty worktree에 연결돼 있으므로, 개별 slice 수치로 잘못
분할하지 않고 DB 진단 checkpoint `eae0b3f` 대비 현재 누적치로 기록한다.

| 파일 | + | - | 주 역할 |
|---|---:|---:|---|
| `Archive Center.js` | 242 | 15 | backend 장부·로어북 관찰 UI 표시와 Host 전달 |
| `group_proxy.go` | 67 | 15 | 출판사 호출 장부 |
| `group_reference_budget.go` | 4 | 4 | 원작 참조 독립 예산 |
| `group_reference_canon_base.go` | 21 | 0 | 원작 참조 후보 수치 |
| `group_turn_prepare.go` | 35 | 9 | 독립 lane·장부·호출 trace 연결 |
| `memory_reprocessing_worker.go` | 3 | 0 | 안전한 평론가 호출 장부 보존 |
| `output_fidelity_lineage.go` | 1 | 1 | payload lineage 연결 |
| `prepare_turn_assembly.go` | 6 | 1 | single endpoint 제외 count |
| `prepare_turn_lorebook_reference.go` | 220 | 76 | 보조 선택과 관찰 계약 |
| `prepare_turn_memory_budget.go` | 27 | 0 | 기억 후보·선택·제외 수치 |
| `prepare_turn_recall.go` | 14 | 1 | 관계 recall eligibility |
| `prepare_turn_render.go` | 255 | 0 | 전체 payload 장부 |
| `turn_extraction_critic.go` | 26 | 6 | 평론가 호출 장부 |
| `provider_call_budget_ledger.go` | 142 | 0 | 새 호출별 장부 owner |

production JavaScript 누적 변경은 `+242/-15`다. 이번 문서 작업에서는 JavaScript를 변경하지
않았다. 남은 JavaScript 역할은 backend 수치 표시, RisuAI payload 적용, Host lifecycle 관찰이다.

관련 contract/DTO freeze와 회귀도 함께 갱신돼 있다. `docs/3.0.md`를 포함한 기존 dirty 변경은
사용자 작업으로 보존했으며 이번 기록 작업에서 건드리지 않았다.

### 2026-08-23 현재 재검증

- `go test ./internal/store ./internal/httpapi ./cmd/js-route-variant-smoke -count=1`: 통과
- `node --check "Archive Center.js"`: 통과
- 최초 Go test는 기본 Windows Go cache의 `Access is denied`로 실행 전에 실패했으며, workspace
  local `GOCACHE`/`GOTMPDIR`로 다시 실행해 위 세 package가 모두 통과했다.
- 검증용 local cache/temp 폴더는 통과 후 제거했다.

이 검증은 production 함수와 adapter 회귀가 현재 source에서 통과했다는 뜻이다. 실제 MariaDB,
실제 provider, 로드된 RisuAI 최종 payload 또는 최신 package 반영을 대신하지 않는다.

## 32. 시험 패키지 최신 snapshot과 정정

기존 경로
`_test-builds/Pre-4.0.0-stage9-critic-compact-windows-test`는 2026-08-23 14:14 KST에 다시
생성됐다.

- source commit: `eae0b3fba828df70c10ac9aad0ddef31fa040fdb`, dirty source
- package manifest: `green`, `release_ready=true`
- managed files: 46개
- source/package `Archive Center.js` SHA-256:
  `C1C6B7F2EEF632059B6C2F44DDC513C51DAF1F1DC15DDEDE14180DA30E329AE7`
- packaged `bin/archive-center-go.exe` SHA-256:
  `92B98900CC5E32D8B87828FB342A546E8D63E4104568D73962B36D5893EDC344`
- ZIP SHA-256:
  `ED14090381BFEABC2B4CA6F26123D05BCBD3DCCF4E4F522FBE5C95616F137992`
- 외부 `SHA256SUMS-Pre-4.0.0.txt`: ZIP과 일치

이 snapshot은 25.1의 이전 hash 기록을 삭제하지 않고 최신 package 기록으로 정정한다. 독립 예산,
payload/provider 장부, `single_endpoint_only` 복원은 build 전에 source에 있었으므로 포함된다.

그러나 최신 로어북 `Always Active` Host 소유·weak lexical 보류 patch는 16:36–16:40 KST에
적용돼 이 package binary보다 늦다. 따라서 JS hash는 현재 source와 같지만 Go binary는 최신
로어북 선택 patch를 포함하지 않는다. 현재 package 상태는 `PACKAGE_PARTIAL`이며, 최신 로어북
동작의 package/live 완료 선언이 아니다.

## 33. 당시 남은 작업 — `HISTORICAL SNAPSHOT`

이 목록은 2026-08-23 시점의 gate다. package 갱신과 로어북 정책은 이후 다시 변경됐으므로 완료
여부와 현재 우선순위는 section 41에서 판정한다. 특히 아래의 6,000자·Always Active·weak lexical
검증 문구를 현재 source 계약으로 사용하지 않는다.

### 바로 닫아야 할 검증

1. 최신 로어북 source를 기존 stage9 시험 패키지에 다시 반영한다. 새 이름의 병렬 package를
   만들지 않고 기존 package를 갱신하고 JS, Go binary, ZIP hash를 다시 기록한다.
2. 갱신된 package로 같은 세션에서 한 turn을 실행해
   `lorebook_selection_observation.v1`을 확인한다.
   - `always_active_candidate_count`는 관찰값으로 남을 수 있음
   - `always_active_activated_count=0`
   - `always_active_delivery_count=0`
   - native duplicate와 weak lexical disposition
   - key/strong lexical 실제 전달 항목
   - 최종 chars가 6,000을 채우기 위해 증가하지 않는지
3. 같은 turn의 backend `planned_exact_payload_text`와 RisuAI pre-request 실제 payload를 대조해
   기존 verification mismatch를 닫는다. 일치하기 전에는 UI의 “실제 전달” 수치를 live verified로
   기록하지 않는다.
4. 출판사·평론가 호출별 장부를 실제 provider 성공/실패 사례에서 확인한다. usage 미보고를 0 또는
   추정 토큰으로 오표시하지 않는지, timeout과 JSON parse 실패 stage가 정확한지 대조한다.
5. 일반 기억 감소가 다음 turn에도 유지되는지 확인하되 직접 근거·최근 사건·보호 기억이 사라지지
   않았는지도 함께 본다. 12,000 설정에서 12,008로 보인 reversible-state 보충분도 exact text로
   대조해 strict ceiling을 넘는지 판정한다. 목표는 적게 넣는 것 자체가 아니라 불필요한 관계
   후보만 줄이는 것이다.

### 실제 재현이 생길 때만 진행할 작업

6. MariaDB 경합이 다시 발생하면 1213/1205와 `SHOW ENGINE INNODB STATUS`를 확보한다. 실제
   1213 상대 transaction이 확인될 때에만 wake 위치 이동 또는 제한 재시도를 검토한다.
7. 최신 로어북 trace에서 흔한 token 두 개만으로 무관 항목이 실제 전달되는 경우에만 lexical 조건
   한 곳을 최소 조정한다. 재현 전에는 stopword/IDF/언어별 규칙을 추가하지 않는다.

### 별도 후속 backlog

8. 최신 package를 RisuAI에서 다시 불러온 뒤 예산·호출 장부 UI와 긴 세션의 DOM/heap을 실제로
   검수한다.
9. section 20의 전체 UI 재정비는 backend 계약을 다시 만들지 않고, 현재 ViewModel을 가볍게
   표현하는 별도 작업으로 이어간다.

현재 우선순위는 1–5다. 6–7은 증거 조건부이며, 8–9는 이 backend 안정화 검증이 끝난 뒤의 별도
UI 작업이다.

## 34. 2026-08-23 최신 로어북 source 시험 패키지 갱신

section 32에서 `PACKAGE_PARTIAL`로 남겼던 기존
`_test-builds/Pre-4.0.0-stage9-critic-compact-windows-test`를 새 이름으로 복제하지 않고 같은
경로에서 2026-08-23 19:25 KST에 갱신했다.

### 갱신 절차

- 해당 package의 `archive-center-go.exe` PID 3252 종료
- package 폴더를 점유하던 잔여 `01_start_archive_center_windows.bat` launcher PID 21648 종료
- `.env.full.local`을 package 밖 같은 output root에 임시 보존
- `ops/build-full-package.ps1`을 같은 output root·package name에 `-ForceRefresh -Zip`으로 실행
- 갱신 뒤 `.env.full.local`을 원래 위치에 복원하고 hash 대조
- 임시 env 사본과 workspace-local Go build cache/temp 제거
- backend는 다시 시작하지 않음

첫 빌드는 backend 종료 뒤에도 launcher가 폴더를 점유해 target 삭제 단계에서 중단됐다. launcher를
정확히 식별·종료한 다음 같은 명령을 다시 실행해 성공했다. 실패한 첫 시도는 package 완료 증거로
사용하지 않는다.

### 갱신 결과

- package manifest: `green`, `release_ready=true`
- Go toolchain: `go1.26.5 windows/amd64`
- source commit: `eae0b3fba828df70c10ac9aad0ddef31fa040fdb`, dirty source
- managed manifest: 46개 파일
- 실제 managed 파일 hash 불일치: 0개
- source/package `Archive Center.js` SHA-256:
  `C1C6B7F2EEF632059B6C2F44DDC513C51DAF1F1DC15DDEDE14180DA30E329AE7`
- packaged `bin/archive-center-go.exe` SHA-256:
  `54E8F7D18A83FF8D3AC1F94ECA195091437869301DD63306D059AB067B3E5D43`
- ZIP SHA-256:
  `137D6707B567DE97C773347967242F9158D713D929A4A6587566870BF39572DC`
- 외부 `SHA256SUMS-Pre-4.0.0.txt`: ZIP hash와 일치
- 복원한 `.env.full.local` SHA-256:
  `DC89554F1CF1F0196B0DEC2C6F0EA7DFA23F76E963ABFDA1365880AFEA03C81A`
- package `Archive Center.js` 문법 검사: 통과

이 binary는 section 30의 최신 `Always Active` Host 소유, native exact duplicate 억제, weak
lexical 보류와 `lorebook_selection_observation.v1` source를 포함해 빌드됐다. 따라서 section 33의
남은 작업 1번 package 갱신은 `PACKAGE` 단계에서 완료됐다.

아직 남은 우선 검증은 section 33의 2–5번이다. 즉, 새 package를 실제 RisuAI에서 불러 한 turn을
진행하고 로어북 선택 관찰, 실제 pre-request payload parity, provider 호출 장부, 일반 기억의
12,000/12,008 경계를 확인해야 한다. 이번 기록은 package 갱신 완료이며 `LIVE` 완료 선언이 아니다.

## 35. 2026-08-23 4.0.0 정식판 identity 승격

활성 source의 제품 identity를 `Pre-4.0.0`에서 `4.0.0` 정식판으로 승격했다. 과거 시험 패키지명과
Pre 단계의 검증 기록은 역사적 증거이므로 이 문서와 로드맵에서 일괄 치환하지 않았다.

### 정렬한 현재 소유 지점

- `Archive Center.js`: display name, `VERSION`, `BUILD_ID`, `BUILD_CHANNEL=release`, build notes
- Go 기본 `/version`: `4.0.0`
- Windows/POSIX package builder 기본값과 빈 값 fallback: `4.0.0`
- root 및 Windows package env example: `AC_BUILD_VERSION=4.0.0`
- README·신규 설치/이전 안내의 현재 대상 버전: `4.0.0`
- JS HUD와 Go `/version` 회귀 기대값: `4.0.0`

직접 업데이트 최소 원본 버전 `3.9.9`, Canon Pack의 버전 호환성 경계, 과거 Pre 시험 산출물 기록은
현재 제품 identity가 아니므로 변경하지 않았다.

### SOURCE 검증

- `node --check "Archive Center.js"`: 통과
- Windows/POSIX package builder PowerShell AST 구문 검사: 통과
- `go test ./... -count=1`: 통과
- 변경 대상 `git diff --check`: 통과

이번 단계는 `SOURCE` identity 승격 완료다. 정식 출시 완료를 선언하려면 별도 `4.0.0` 패키지를 새로
생성한 뒤 manifest/hash, package-launched `/version`·`/ready`, 실제 RisuAI의 새 플러그인 로드와 한 turn
처리를 검증해야 한다. 기존 Pre 시험 패키지의 이름이나 manifest만 고쳐 출시 증거로 사용하지 않는다.

## 36. 2026-08-24 현재 상태와 이전 판정의 대체 관계

이 section 이후가 현재 `4.0.0` source와 최신 시험 패키지의 정본 판정이다. 앞의 section은 삭제하지
않고 당시의 결정, 실패, trace와 hash를 보존한다.

| 이전 기록 | 현재 판정 |
|---|---|
| section 27의 로어북 기본 6,000 | 새·누락·잘못된 설정의 기본값은 3,000. 기존 명시값 6,000과 0은 보존 |
| section 30의 Always Active Host 보류·overlap 2·weak lexical 규칙 | `SUPERSEDED`; 해당 보수 조건은 현재 선택기에 없음 |
| sections 32·34의 Pre package hash | 역사적 package snapshot; 최신 hash는 section 40 |
| section 33의 남은 작업 | 역사적 gate; 현재 gate는 section 41 |
| section 35 마지막의 “4.0.0 package 없음” | section 40의 실제 4.0.0 폴더·ZIP 생성으로 `PACKAGE` 단계에서 종료 |

이 대체 관계는 과거 작업이 없었다는 뜻이 아니다. 보수적 선택기가 실제로 로어북 0개를 만들고,
다시 완화하면 6,000자를 채우는 진동을 보였기 때문에 삭제 후 재작성했다는 원인과 순서를 보존한다.

## 37. 로어북 기존 조건 삭제와 선택기 재작성

### 37.1 삭제한 현재 비정본 조건

사용자는 로어북 부분만 대상으로 기존의 보호적·보수적 조건을 먼저 제거한 뒤 다시 작성하도록
지시했다. 현재 `prepare_turn_lorebook_reference.go`에는 다음 중간 정책이 남아 있지 않다.

- `Always Active`라는 이유만으로 Archive Center 보조 참조를 일괄 차단
- lexical overlap 2 이상을 요구하는 고정 threshold
- overlap 1을 무조건 weak로 분류하는 별도 gate
- 관련성이 약한 후보를 남는 예산만큼 승격해 상한을 채우는 동작
- 조금 불확실하다는 이유로 로어북 lane 전체를 off 또는 0개로 만드는 fallback

`Always Active`는 관찰 count로 남지만 단독 활성화 근거도, 단독 차단 근거도 아니다. RisuAI Host를
수정하거나 Host의 원래 로어북 주입을 대신하지 않는다.

### 37.2 현재 Go 소유 선택 순서

현재 `prepareTurnLorebookReferenceSearch`와 `finalizePrepareTurnLorebookReference`의 순서는 다음과
같다.

1. 현재 세션·캐릭터·활성 모듈 scope에서 받은 catalog만 후보로 사용한다.
2. 관련성 검색 text는 저장된 `NormalizedSearch`를 우선하고, 비어 있으면
   `Key + SecondKey + Comment + Content`를 사용한다.
3. 직접 key match와 문맥 token overlap을 관찰하되, exact payload presence를 no-context보다 먼저
   판정한다.
4. 현재 user input, RisuAI request message 또는 이미 조립된 Archive Center context에 같은 본문이
   있으면 `excluded_already_present`로 기록한다.
5. 남은 후보 중 문맥 근거가 전혀 없는 항목만 `excluded_no_context_match`로 기록한다.
6. 동일 본문은 하나의 group으로 합치고 모든 source ref를 보존한다.
7. 직접 key match group이 하나라도 있으면 모든 직접 key group만 relevance frontier로 삼는다.
8. 직접 key group이 없으면 가장 높은 양수 `ContextOverlap` 동점 group만 frontier로 삼는다.
9. 더 낮은 tier는 `excluded_below_relevance_frontier`로 기록하고 예산이 남아도 선택하지 않는다.
10. char 상한은 frontier가 확정된 뒤에만 적용한다. 한 항목을 중간에서 잘라 넣지 않는다.

따라서 3,000은 목표량이 아니라 마지막 hard ceiling이다. 다만 직접 key group 또는 최고 overlap 동점
group이 매우 많으면 같은 frontier 안에서 3,000에 가까워질 수 있다. 이 잔여 한계를 없애려고 고정
항목 수나 임의 threshold를 추가하면 다시 유용한 보조 참조가 0개가 될 수 있으므로, 실제 trace 전에는
추가 규칙을 넣지 않는다.

### 37.3 관찰과 UI

- 후보·선택·최종 전달 count/chars는 서로 분리한다.
- 낮은 tier 제외 수는 `FinalDispositionCounts[excluded_below_relevance_frontier]`로 남긴다.
- payload 장부에는 `lorebook_below_relevance_frontier`로 투영한다.
- exact 중복은 `lorebook_already_present_in_payload`로 구분한다.
- `Archive Center.js`는 이 수치와 disposition을 표시할 뿐 선택·예산을 다시 계산하지 않는다.
- Publisher support와 execution contract에는 실제 전달된 lorebook source만 들어간다.

## 38. 로어북 3,000자 독립 상한

### 38.1 변경한 기본값 owner

새·누락·잘못된 설정의 로어북 기본 상한을 6,000에서 3,000으로 변경했다.

- `Archive Center.js`: `DEFAULT_SETTINGS.lorebookReferenceMaxChars=3000`
- `go-service/internal/dto/types_gen.go`: DTO default와 `ApplyDefaults` 3,000
- `go-service/internal/httpapi/group_turn_prepare.go`: 방어적 Go fallback 3,000
- `contracts/openapi-schema-freeze.json`: OpenAPI default 3,000
- `contracts/go-dto-mapping-plan.json`·`.md`: DTO freeze default 3,000
- `contracts/turn-contract-freeze.md`: 독립 lorebook cap과 relevance frontier 계약
- `docs/3.0.md`: 현재 예산·선택 설명

일반 기억 9,000, 원작 DB 3,000, 로어북 3,000은 서로 빌리거나 빌려주지 않는 독립 상한이다.
출판사와 평론가는 별도 LLM 호출이므로 main-model payload 합계에 억지로 합치지 않는다.

### 38.2 기존 저장값을 자동 변경하지 않은 이유

정상 플러그인 요청은 RisuAI plugin storage의 설정값을 매번 명시 전송한다. 이미 저장된 6,000이
과거 기본값인지 사용자가 의도적으로 고른 값인지 현재 저장 형식으로 구분할 수 없다. 따라서
`saved == 6000`이라는 이유만으로 3,000으로 덮는 migration을 만들지 않았다.

- 새 사용자·설정 key 누락·잘못된 값·기본값 복원: 3,000
- 기존 저장값 6,000: 그대로 6,000
- 사용자가 명시한 0: 그대로 0
- 그 밖의 유효한 사용자 값: 그대로 보존

기존 설치에서 새 상한을 시험하려면 UI에서 3,000을 저장하거나 기본값 복원 후 저장해야 한다.

## 39. 2026-08-24 source·regression 검증

### 39.1 회귀 범위

다음 동작을 production path 회귀로 고정했다.

- 누락 기본값 3,000
- 명시적 6,000 보존
- 명시적 0 보존
- `Content` fallback으로 얻은 최고 관련 후보 전달
- 큰 잔여 예산이 있어도 약한 context tier 제외
- 직접 key group이 있으면 context-only group 제외
- 관련성 0이어도 payload에 이미 있는 본문은 `already_present`로 분류
- frontier 제외 사유의 payload budget ledger 반영
- 동일 본문 coalescing과 source ref 보존
- oversized 항목 부분 절단 금지
- `Always Active` 단독 강제 활성화 금지
- scope, Publisher support, guide eligibility와 execution source ref 유지

### 39.2 실행 결과

- `go test ./internal/httpapi -run 'Lorebook|PayloadBudget' -count=1`: 통과
- `go test ./internal/httpapi -count=1`: 통과
- `go test ./internal/dto -count=1`: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 번들 Node 경로 지정 후 통과
- 번들 Node `--check "Archive Center.js"`: 통과
- `openapi-schema-freeze.json`, `go-dto-mapping-plan.json`: JSON parse 통과
- `gofmt -d` 대상 Go 파일: 출력 없음
- `git diff --check`: 통과

처음 JS smoke 실행은 시스템 `PATH`에 `node`가 없어 production fixture 네 건이 실행 전 실패했다.
`ARCHIVE_CENTER_NODE_BINARY`에 workspace 번들 Node를 지정한 재실행은 전체 통과했으므로 코드 실패로
분류하지 않는다.

이번 로어북 slice에서 `Archive Center.js`는 기존 기본값 literal 한 줄만 `+1/-1`, net 0이다. 새 JS
선택 정책은 없다. DB schema, table, API field, hidden retry, parallel cache도 추가하지 않았다.

## 40. 2026-08-24 최신 4.0.0 시험 패키지 갱신

### 40.1 갱신 절차

새 병렬 시험 package를 만들지 않고 기존 최신 외부 경로
`_test-builds/Pre-4.0.0-stage9-critic-compact-windows-test`를 갱신했다. 외부 경로명은 기존 사용자
동선과 link를 유지하기 위한 역사적 container이고, 실제 package identity는 4.0.0이다.

- 해당 package의 backend PID `24156` 종료
- 같은 package의 `01_start_archive_center_windows.bat` launcher PID `22632` 종료
- `.env.full.local`을 외부 output root의 임시 위치에 보존
- workspace-local `GOCACHE`·`GOTMPDIR` 사용
- `powershell.exe -NoProfile -ExecutionPolicy Bypass`로 같은 output root에
  `-PackageKind managed -PackageVersion 4.0.0 -Zip -ForceRefresh` 실행
- build 성공 뒤 `.env.full.local` 복원과 SHA-256 동일성 확인
- 임시 env 사본과 이번 Go cache/temp 제거
- backend와 launcher는 다시 시작하지 않음

### 40.2 패키지 결과

- 실제 folder: `Archive Center 4.0.0 Windows Auto Install Package`
- 실제 ZIP: `Archive Center 4.0.0 Windows Auto Install Package.zip`
- build 출력: `Status: green`
- manifest: `package_version=4.0.0`, `release_ready=true`
- release status: `automatic_update_apply=true`
- source commit: `eae0b3fba828df70c10ac9aad0ddef31fa040fdb`, dirty source
- managed files: 46개
- 실제 manifest hash 불일치: 0개
- source/package `Archive Center.js` SHA-256:
  `9C69DEA34D4E82AE5FFD6FE82B6ED61AB666ED05B62762F82DB8F676ADA839BF`
- packaged `bin/archive-center-go.exe` SHA-256:
  `CB2525BB196DD7E19490EF6598EF949D2B985FF9D07F2D6197071F484231EC65`
- ZIP SHA-256:
  `DA3EA75D35A567433F332F96E7362010003BE46D01DBFBC5816C1E104506CCD1`
- `SHA256SUMS-4.0.0.txt`: ZIP과 일치
- ZIP size: 11,848,718 bytes
- package local env: 복원 확인

이 증거로 section 35의 “4.0.0 package를 별도로 생성해야 한다”는 항목은 `PACKAGE` 단계에서
종료됐다. 그러나 package-launched backend, `/ready`, MariaDB·Chroma, 실제 updater 적용, 로드된
RisuAI와 한 turn은 이번 갱신에서 실행하지 않았으므로 `LIVE` 완료는 아니다.

## 41. 현재 통합 상태와 남은 gate

### 41.1 지금까지의 작업 묶음

| 영역 | 현재 결과 | 증거 | 현재 경계 |
|---|---|---|---|
| 영구 JS/Go 경계 | Go가 정책·선택·예산·저장·ViewModel, JS가 Host 관찰·적용·UI | sections 3, 23–30, 37–39 | 현재 source 정본 |
| UI·정보구조 | near-black 제품 UI, 세계선·기억·기억 관리·추가 기능·설정, 설정 재배치 | sections 5, 20, 23–25 | 실제 긴 세션 DOM/heap 검수 남음 |
| 세계선·branch | 현재 세션 필터, turn node, branch ancestry·다중 branch 정합성 | sections 6–7 | branch/reroll/delete 장기 live matrix 남음 |
| 기억 관리·수정 | 세션 작업 공간, DB 연결·이동·복사·삭제, 수동 수정과 원본 계보 | sections 15, 23 | 실제 대량 DB UI 반응 검수 남음 |
| 기억·로어북 열람 성능 | 20개 page, 누적 append, stale request 차단, 이미 받은 펼침 내용 재사용, session별 lorebook scope | sections 7, 18, 25 | 장시간 누적 DOM/heap과 다중 session 혼합 방지 live 확인 남음 |
| HypaMemory | 현재 Risu chat의 Hypa 요약을 명시 import하고 Critic·기억·근거·KG 경로에 연결 | section 7.3 | 실제 Risu Hypa data shape별 import 남음 |
| 입력 재작성 | 검토 후 입력 재작성, 표시·추적·legacy 이름 정리 | section 24 | provider별 live 사용 확인 남음 |
| 출력 언어 | 최종 출력 언어만 평론가 기억 언어 계약에 사용 | section 8 | 다국어 장기 세션 확인 계속 |
| 출판사·평론가 단계 0–9 | provider-neutral parsing, 부분 수용, fail-open, 재처리, compact·추론 제어 | sections 9–13, 28 | 실제 provider 성공·timeout·JSON 실패 matrix 남음 |
| recovery·HUD | recovery source revision 충돌 처리와 성공/실패 HUD 표시 | sections 10, 16 | 실제 여러 실패 유형 UI 확인 남음 |
| DB 경합 | source revision/status lock 진단 정보와 최소 retry 경계 | section 26 | 실제 1213/1205와 InnoDB 상대 transaction 필요 |
| 전체 payload 장부 | 기억·원작·로어북·안내·조립 비용과 candidate→selected→final 분리 | sections 27–29 | Host 실제 pre-request payload parity 남음 |
| 로어북 보조 참조 | 기존 조건 삭제, frontier 재작성, 기본 3,000, 빈 공간 비채움 | sections 36–39 | 최신 package로 실제 2–3턴 trace 필요 |
| updater | running backend version 정본, 누적 migration inventory, 직접 3.9.9→4.x 계약 | sections 21, 35, 40 | 게시된 4.0.0 asset으로 3.9.11 UI 적용 확인 남음 |
| 4.0 identity·package | source release identity, managed folder·ZIP·manifest·checksum | sections 35, 40 | package-launched live와 실제 updater 적용 남음 |

### 41.2 다음 실행 순서

1. 기존 RisuAI plugin storage에 6,000이 남아 있으면 3,000으로 저장하거나 기본값 복원 후 저장한다.
2. section 40 package의 `Archive Center.js`를 RisuAI에서 다시 불러온다.
3. 같은 세션으로 2–3턴을 실행해 다음 수치를 보존한다.
   - catalog·candidate count/chars
   - selected count/chars
   - final delivery count/chars
   - `lorebook_below_relevance_frontier`
   - `lorebook_already_present_in_payload`
   - `lorebook_no_context_match`
4. 관련 항목이 있을 때 다시 0개가 되지 않는지, 무관한 낮은 tier로 3,000을 채우지 않는지 확인한다.
5. backend `planned_exact_payload_text`와 실제 RisuAI pre-request payload를 대조한다.
6. 출판사·평론가 호출 장부를 실제 provider 성공, usage 미보고, timeout, JSON parse 실패에서 확인한다.
7. 게시된 4.0.0 bundle이 준비되면 3.9.11 UI updater에서 실제 갱신·DB·설정 보존을 확인한다.
8. MariaDB 경합이 재현될 때만 1213/1205, source revision/status key/owner와
   `SHOW ENGINE INNODB STATUS`를 수집해 wake 이동 또는 제한 재시도를 판단한다.
9. backend 안정화 live gate 뒤에 전체 UI·긴 세션 DOM/heap 개선을 별도 작업으로 이어간다.

최고 관련도 동점 또는 직접 key group 자체가 매우 많으면 로어북이 3,000에 가까워질 수 있다. 실제
trace에서 이 경우가 재현되기 전에는 고정 개수, stopword allowlist, 언어별 사전, IDF, 별도 LLM
판정기를 추가하지 않는다. 현재 목표는 “무조건 적게”가 아니라 Host 기본 로어북에서 빠질 수 있는
관련 정보만 보조하면서, 낮은 관련도로 빈 공간을 채우지 않는 것이다.
