# Archive Center 기억 주입·서사 가이드 권한 검토

> 현재 지위: 1.0 비교, 권한 검토와 대안 채택·기각을 보존하는 역사적 review다.
> 4.1~9.0 버전 지도와 확장 제품 이름은
> [`4.1-9.0-integrated-roadmap.md`](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)를 우선한다.
> 신규 확장 제품은 `AC Ensemble Agent`이며 Recomposer 공동 사용 절은 현재 제품
> 이름이나 구현 완료 증거가 아니다.

상태: 조사 및 버전별 설계 제안

최초 작성: 2026-07-29

현재 기준: Archive Center 3.8-A~E source 구현·자동 회귀 완료 — 3.7 핵심
RisuAI lifecycle·HUD 고정 빌드 확인, 3.7-G 전체 환경 행렬과 3.8 loaded-artifact 검증 후속

구조 정리: 2026-07-31

버전 배정 정정: 2026-08-10

- 3.9는 현재 bounded Publisher의 provider-neutral 오류 안정화에서 끝낸다.
- 1.0의 단일 호출 `book_author + director` 역할과 coherent narrative-plan block은 4.0으로 이관한다.
- 4.0은 `none/weak/medium/strong/extreme/maximum` 서사 강도와
  `compact/standard/explicit` 모델 입력 표현을 독립된 축으로 정의한다.
- 기존 4.1 lifecycle·복구 범위는 4.0-H~N으로 합친다. 새 4.1은 기억 Locator·계층 탐색 후보다.
- 4.3은 Archive 기억과 RisuAI 로어북·캐릭터·페르소나를 역할별 문맥으로 만들고,
  감독관·나레이터에는 전체 배경 이야기와 공개 기억을, 연기자에는 배경 이야기와 해당
  인물의 주관 기억을 제공한다.
- 현재 Recomposer로 부르는 이름·역할명·packet 이름은 확정하지 않는다. 4.4~4.5는 단독형
  별도 플러그인의 Archive Center 연동 전처리, 연동 요청의 Archive Publisher 자동 OFF,
  메인 모델 1차 출력 이후 감독관·나레이터·연기자 협업과 후처리 최종문 lifecycle을 소유한다.
- 4.9는 4.0 계약을 교체하지 않고 `directed/reviewed/deep` 다단계 검토와 검증된 capability 기반
  format 자동 선택만 확장한다.
- 기존 4.3 이후 작업은 새 4.3 삽입에 따라 한 칸씩 이동해 5.0에서 끝난다. 본문의 과거 번호가 이 결정과 충돌하면
  통합 로드맵과 각 정본 문서를 우선한다.

범위: Archive Center 1.0(fix), 역사적 3.6 조사 기준선, 현재 3.8,
3.9~4.3 기억·기본 출판사 packet, 4.6~5.0 모델 적응형 RP 출판·시뮬레이션,
독립형 원작 DB, 이름 미확정 별도 플러그인의 4.3~4.5 연동 경계, RisuAI 로어북 항목의 4.0 참조 검색
계약

비범위: 런타임 구현, 배포 패키지, 실제 RisuAI 품질 검증, 독립형 원작 DB 구현 및 특정 출시 버전 배정, RisuAI 로어북 대체·발동 엔진 재구현

## 0. 문서 지위와 버전별 분할

3.9~5.0의 전체 실행 순서와 정본 문서 연결은
[`3.9-5.0-integrated-roadmap.md`](3.9-5.0-integrated-roadmap.md)를 통합 인덱스로
사용한다. 이 문서의 역사적 비교·권한 근거와 각 버전 정본의 세부 packet이 충돌하면
정본 packet과 더 엄격한 안전 하한선을 우선한다.

이 문서는 특정 버전 하나의 구현 명세가 아니다. 1.0의 기억·감독 체감에서 회수할 장점,
3.6에서 확인한 안전 하한선과 약화 지점, 현재 3.7에서 정리할 안정화 항목, 3.8 이후에
구현할 기억·회상·서사 지원을 한곳에서 비교하는 **교차 버전 설계 검토서**다.

3.6은 더 이상 현재 버전을 뜻하지 않는다. 본문에서 3.6을 언급하는 경우는 다음 두
역사적 의미로 제한한다.

1. 당시 소스에서 직접 확인한 동작·결함의 증거 기준선
2. 3.7 이후에도 완화하면 안 되는 source·time·branch·privacy·authority·fail-open 하한선

현재 구현 상태와 출시 판단은
[`3.6-4.1-precision-long-term-memory-roadmap.md`](3.6-4.1-precision-long-term-memory-roadmap.md)와
각 버전의 release evidence를 따른다. 이 문서에 계획이 적혀 있다는 사실만으로 구현 또는
실환경 검증 완료를 주장하지 않는다.

| 구분 | 이 문서에서 소유하는 내용 | 이 문서의 주요 절 |
|---|---|---|
| 역사적 1.0·3.6 기준선 | 1.0 체감 검증, 3.6 당시 약화·이상 동작, 보존할 안전 하한선 | 2~4장 |
| 3.7 기준선 | 3.6 이월 항목의 재검증, 기억 품질·관찰성·오류 분류 안정화 | 4장, 10장의 3.7 packet |
| 3.8 | 시간·현재/과거 상태·주관 지식과 인물별 관점 | 7장, 8.6의 시간·인식 계약, 10장의 3.8 packet |
| 3.9 | 관계·습관·프로필과 bounded Publisher 오류 안정화 | 5장, 8.6, 10장의 3.9 packet |
| 4.0 | 3.7-F 핵심 기억 K를 소비하는 복합 회상·최종 전달 증명, `book_author + director` 출판사 복구, 선택형 로어북 참조 검색 | 5~6장, 8.6의 회상 계약, 10장, 12장 |
| 4.1 | 기억 Locator·계층 탐색 후보 | 통합 로드맵과 4.1 정본 문서 |
| 4.3 | 역할별 배경 이야기·공개 기억과 인물별 주관 기억 | 통합 로드맵과 4.3 정본 문서 |
| 4.4~4.5 | 이름 미확정 별도 플러그인의 연동 전처리·Publisher 자동 OFF·역할 AI 후처리·최종문 lifecycle | 통합 로드맵과 4.3 정본 문서의 인계 계약 |
| 4.6 | 실행 지원 기반 계약과 source-linked 관계·개체 graph·UI | 8.6, 10장 |
| 4.7 | 인물·관계 실행 카드, 말투·지식·agency 투영 | 8장, 10장 |
| 4.8 | 세계·세력·thread 시뮬레이션과 비강제 진행 지원 | 5장, 8장, 10장 |
| 4.9 | 모델 능력 적응과 Recall Auditor→Director→Reviewer 고급 출판사 | 5장, 8장, 9장, 10장 |
| 5.0 | output fidelity 사후 진단, 실사용 검증과 release closure | 8장, 10장 |
| 버전 미배정 후속 | 독립형 원작 DB | 11장 |

버전 번호는 기능의 소유권과 선행 조건을 뜻한다. 뒤 버전의 아이디어를 앞 버전의
핵심 범위에 끼워 넣거나, 앞 버전의 release gate를 뒤 버전이 대신 통과한 것으로
해석하지 않는다.

과거 단일 범위 초안에 임시로 붙어 있던 4.2~4.7 단계 번호는 더 이상 버전 배정이 아니다.
새 통합 인덱스는 사용자 기억 문제를 4.1~5.0에 먼저 배치하고, 역할별 기억 packet과 별도
플러그인의 Integrated 실제 연극을 5.1~6.0에 배치한다.

이 문서 아래의 과거 Recomposer 공동 사용안 가운데 Archive Center Publisher plan을
별도 플러그인 전처리에 다시 전달하거나 같은 요청에서 둘을 fallback으로 교대시키는 설명은
현재 5.1~5.5 방향에 의해 대체된다. Integrated stage가 요청을 소유하면 Archive Center
Publisher는 자동 OFF이고, 연동하지 않은 요청에서만 Archive Center Publisher가 동작한다.

## 1. 결론

사용자가 기억하는 1.0의 체감은 **결과적으로 맞다**.

다만 실제 구조는 “Top K를 10으로 두면 최종 프롬프트에 언제나 정확히 10줄을 넣는다”가 아니었다. 더 정확한 설명은 다음과 같다.

```text
넓게 의미 검색
  → 짧은 한 줄 기억으로 정리
  → Supervisor가 더 넓은 기억 묶음을 읽음
  → Book Author / Director 지시로 다시 압축
  → 소수의 직접 기억 줄 + 장면 지휘가 본문 모델에 함께 전달
```

이 구조 때문에 사용자는 다음 효과를 얻었다.

- 핵심 기억은 데이터베이스 조회 결과처럼 짧고 분명했다.
- 직접 들어가지 못한 기억도 감독 지시에 간접 반영됐다.
- 기억, 이전 맥락, 세계·스토리라인 보조 자료가 역할별로 보였다.
- “무엇을 기억해야 하는가”뿐 아니라 “이번 장면에서 그 기억을 어떻게 써야 하는가”도 본문 모델이 받았다.

3.6 조사 기준선과 1.0의 가장 큰 차이는 다음 두 가지다.

1. 3.6 조사 당시 Go production 계약에서 `topK`가 최종 핵심 기억 개수가 아니라 `vector_memory_search_limit_only`가 됐다. 다만 JS 호환 경로와 reference recall에는 과거의 다른 소비 방식이 일부 남아 있었다.
2. weak/medium/strong이 감독 권한을 늘리지 않고 표현 보조 항목의 범위만 늘린다.

따라서 권장 목표는 다음과 같다.

> `없음`에서는 기억·출처·비밀·연속성 지원만 유지한다.
>
> `약함`부터는 진실 권한이 아닌 **응답 구성 권한**을 단계적으로 복원한다.
>
> 핵심 연관 기억 K와 보조 기억·상태·주관 기억은 서로 다른 전달 레인으로 분리한다.

Archive Center 출판사는 별도 플러그인을 사용하지 않는 요청을 위해 독립적으로 완성한다.
그러나 이름 미확정 별도 플러그인의 Archive 연동 전처리가 현재 요청을 소유하면 Archive
Center Publisher 호출·plan·direct guidance는 자동으로 0이 된다. 플러그인은 Archive
기억과 RisuAI 로어북·캐릭터·페르소나를 역할별 문맥으로 전처리하고, 메인 모델 1차 출력
뒤에는 감독관·나레이터·연기자 AI 협업으로 최종문 하나를 만든다. 이때도 Archive Center는
canonical truth·memory·relationship·thread·privacy와 최종 displayed-output admission
권한을 유지하며, 같은 요청에 두 전처리 계획을 적용하지 않는다.

3.9~5.0은 1.0의 감독을 그대로 복제하는 계획이 아니다. 3.9 Basic Publisher에서 먼저
1.0의 체감 가능한 응답 구성 지원을 복구하고, 4.6~4.8의 정밀한 인물·관계·세계·인과
기반 위에서 4.9 고급 출판사로 역할 분리를 확장하는 방향이 가장 적합하다.

이때 복원의 기준은 “1.0과 비슷해 보이는가”가 아니라 **3.6 기준선보다 절대적으로 강했던
능력만 추가하면서 3.6에서 정립해 3.7 이후가 상속한 안전 하한선을 그대로 유지하는가**다.
1.0의 넓은 회상, 약한 입력의 재진입, 적응형 이전 맥락, 계층 해상도 선택, 기억의
행동화와 **사용자가 계속 선택하는 동안만 유효한 가역적 장면 초점**은 회수 대상이다.
반대로 하나의 storyline을 계속 끌고 가는 지속 지휘, 출처 없는 지휘, raw 최근 대화 투입,
고정 문자열 휴리스틱, stale 지침 재사용, 감독 실패의 본문 차단은 회수 대상이 아니다.

특히 **회상·입력 편집·장면 지휘·2차 검토·사후 기억 정리를 역할별로 나눈 다층 체인**은
1.0의 핵심 강점으로 취급한다. 후속 설계는 호출 횟수 자체를 흉내 내는 것이 아니라 각
단계가 실제로 다른 오류를 잡는지, 최종 핵심 기억과 장면 지시의 누락을 줄이는지
ablation으로 증명해야 한다. 이 효과가 없으면 `deep` 호출 수를 늘리지 않으며, 효과가
있어도 3.7의 고정 eligible set과 권한 상한을 넓히지 않는다.

또한 이 문서는 아직 **설계·수용 조건 문서**다. 아래 계약이 실제 Go 경로에 구현되고 source → payload → displayed final → admission 연결과 실제 RisuAI 검증을 통과하기 전에는 “1.0의 강점을 가져왔다”고 완료 판정하지 않는다.

## 2. 사용자가 기억한 1.0 체감 검증

### 2.1 맞는 부분

1.0은 일반 기억을 한 항목당 짧은 한 줄로 포맷했다.

- 구 JS는 기억 요약을 한 줄로 만들고 항목당 최대 200자로 제한했다.
- 구 Python formatter도 기억 요약을 짧은 한 줄로 만들었다.
- `topK=10`이면 의미 검색 폭이 넓어져 핵심 기억을 놓치지 않는 체감이 생길 수 있었다.
- memory, episode, chapter, arc, saga가 별도 조회됐기 때문에 일반 기억 밖의 보조 계층도 존재했다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:17664-17687`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/roles/storyteller.py:14-106`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/services/prepare_turn_recall.py:85-169`

더 중요한 점은 Supervisor가 회수 기억을 직접 읽고 다음 구조를 만들었다는 것이다.

- `book_author.current_arc`
- `book_author.narrative_goal`
- `book_author.next_beats`
- `book_author.guardrails`
- `director.scene_mandate`
- `director.required_outcomes`
- `director.forbidden_moves`
- `director.pressure_level`

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/prompts/supervisor_system.txt:2-25`
- 같은 파일 `:40-83`

즉 1.0은 “기억을 검색하는 시스템”이면서 동시에 “검색된 기억을 장면 목표로 번역하는 시스템”이었다.

### 2.2 정확히는 다른 부분

1.0도 `Top K=10`을 최종 전달 10개로 보장하지 않았다.

- 기본값은 5였고 사용자가 10으로 바꿀 수 있었다.
- 같은 K가 여러 계층 검색에 각각 사용됐다.
- 일반 context profile에서는 최종 `Related Memories`가 먼저 약 2줄로 제한될 수 있었다.
- 전체 문자 예산에 따라 더 줄어들 수 있었다.
- ultra/extreme profile에서는 더 많은 줄이 직접 남았다.
- Supervisor는 직접 본문에 남지 않은 더 넓은 검색 결과도 볼 수 있었다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:110-125`
- 같은 파일 `:20272-20289`
- 같은 파일 `:20936-20993`

따라서 사용자 체감의 정확한 판정은 다음과 같다.

| 사용자 체감 | 판정 |
|---|---|
| 연관 기억 10개를 넓게 긁어왔다 | 대체로 맞음 |
| 각각 한 문장처럼 짧았다 | 맞음 |
| 최종 프롬프트에 항상 정확히 10개가 있었다 | 엄밀히는 아님 |
| 다른 자료가 핵심 기억을 덜 방해했다 | 체감상 맞음 |
| 감독이 기억을 다음 장면 지시로 바꿨다 | 핵심적으로 맞음 |

### 2.3 1.0의 다층 편집·감독 체인

“1.0에서는 Supervisor가 이중·삼중으로 동작했다”는 기억도 **체감상 맞다**. 다만
Supervisor class 자체를 한 턴에 세 번 직렬 호출한 구조는 아니다. 정상 설정에서는 본문
생성 전에 서로 다른 목적의 보조 LLM 판단이 최대 세 층으로 겹쳤다.

```text
Plugin Main 1차 입력 개선 ─┐
                           ├─ 병렬 준비
Supervisor 지시 생성 ─────┘
          ↓
Sub LLM 2차 검토
          ↓
RisuAI 본문 모델
          ↓
Critic 후처리·기억 추출
```

1. **Plugin Main 1차 편집:** 회수 기억과 persistent guidance를 보고 사용자 입력에 필요한
   기억을 1~3개로 압축해 `improved_input`과 keep/drop 후보를 만들었다.
2. **Supervisor 장면 지휘:** 별도 Supervisor 호출 한 번이 `book_author`와 `director`를
   같은 JSON packet으로 함께 만들었다. Book Author와 Director는 서로 다른 책임이지만
   별도 LLM 호출 두 번은 아니었다.
3. **Sub LLM 2차 검토:** 설정되어 있고 1차 결과가 있을 때 1차 편집본, wake-up context와
   Supervisor directive의 일부 preview를 보고 approve/partial/reject를 판정했다.
   `current_arc`, `narrative_goal`, `scene_mandate`, `pressure_level`은 짧은 text로 전달됐지만
   `next_beats`, `guardrails`, `required_outcomes`, `forbidden_moves`는 내용이 아니라 개수로
   축약됐으므로 전체 지침 충돌을 검토했다고 볼 수는 없다.
4. **본문 뒤 Critic:** 최종 응답 뒤 Critic이 별도 background call로 기억·직접 근거·상태
   추출을 수행했다. 이는 현재 응답을 지휘하는 세 번째 Supervisor가 아니라 다음 턴을 위한
   기억 정리 층이다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:11690-12169`
- 같은 파일 `:8041-8068`, `:14803-15032`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/roles/supervisor.py:25-119`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/prompts/supervisor_system.txt:2-25`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/services/complete_turn.py:325-345`

따라서 1.0의 강점은 단순히 “Supervisor 호출 횟수가 많았다”가 아니다. **회상·입력 편집,
장면 지휘, 제한된 2차 검토, 사후 기억 정리가 서로 다른 역할로 연결됐다**는 점이다.
Plugin Main과 Supervisor는 병렬로 준비되고, 이후 2차 검토가 1차 편집과 Supervisor의
일부 preview를 함께 본 구조였다. 이것이 한 번의 약한 표현 보조보다 강한 감독 체감을
만들었을 가능성이 높지만, Supervisor 배열 지침 전체에 대한 교차검증 품질까지 증명하지는
않는다.

반대로 그대로 복원하면 안 되는 부분도 분명하다.

- 보조 호출 수 자체를 품질로 간주하지 않는다.
- 같은 모델을 이름만 바꿔 반복 호출하거나 동일 자료를 세 번 읽히지 않는다.
- 1차 편집 실패나 2차 검토 부재가 본문 요청을 막지 않는다.
- 입력 개선 모델이 사용자 의도를 다시 쓰거나 memory candidate를 truth로 승격하지 않는다.
- post-response Critic의 추출 결과를 현재 턴의 지휘 근거로 소급 적용하지 않는다.

4.9에서 회수할 것은 **다층 역할 분리**다. Go의 결정론적 base support와 typed
candidate가 1차 기반을 만들고, 선택형 Director가 응답 방향을 고르며, 위험하거나 충돌이
있는 경우에만 별도 검증 역할을 호출한다. 모든 모델 호출이 없어도 기억 K·직접 근거·
주관 기억·비밀 guard는 유지돼야 하며, 호출이 늘어날수록 권한이 커지는 구조는 금지한다.

### 2.4 1.0의 지속 지휘가 만든 레일로딩 부작용

사용자가 1.0에서 받은 실제 사용 피드백에는 다음 두 문제가 있었다.

- 한 storyline에 집중하는 힘이 너무 강해, 그 사건 하나가 끝나면 작품 전체가 끝난 것처럼
  느껴졌다.
- 사용자가 다른 이야기로 넘어가려 해도 이전 storyline을 다시 끌어와 사건을 그쪽으로
  옮기는 경향이 있었다.

이 피드백은 소스 구조와 별개의 **사용자 운영 경험 증거**이며, 아직 동일 fixture로
재현·계측한 live evidence는 아니다. 그러나 설계 위험은 분명하다. 1.0의
`arc focus`, `active storyline`, beat queue와 carry target을 장점이라는 이유로 그대로
복원하면 continuity support가 현재 사용자 선택보다 높은 **서사 의무**로 바뀔 수 있다.

따라서 회수할 것은 storyline 고정이 아니라 **일시적인 초점 선택 능력**이다.

- current input이 계속 같은 thread를 선택할 때만 그 thread를 foreground로 둘 수 있다.
- 사용자의 명시적 장면 전환·새 목표·다른 인물 선택은
  `explicit_redirection`이며 이전 focus·arc·carry보다 항상 우선한다.
- 사용자가 반대하지 않았다는 사실, 짧은 답변, 빈 입력과 Supervisor 침묵은 focus 갱신
  동의가 아니다.
- 한 thread의 해결은 그 thread만 `resolved` 또는 `parked`로 바꾼다. 작품·세션 전체
  종결, 엔딩, 에필로그 전환의 근거가 아니다.
- 과거 thread는 사용자가 다시 부르거나 현재 사건의 독립적인 source-backed 결과가 실제로
  닿을 때만 후보로 돌아올 수 있다. continuity를 만들기 위해 새 사건을 과거 thread 쪽으로
  꺾지 않는다.

## 3. 역사적 비교 — 1.0, 3.6 조사 기준선, 4.6~5.0 계획

| 축 | 1.0 | 3.6 조사 기준선 | 4.6~5.0 로드맵과 현재 prerequisite |
|---|---|---|---|
| 핵심 기억 | 짧은 한 줄 기억, 넓은 검색 | 7개 전달 클래스를 문자 예산으로 조립 | 동적 장면·인물·세계 실행 카드 계획 |
| Top K | 검색 폭, 체감상 직접 기억 수와 가까움 | Go production은 벡터 후보 제한, 일부 JS 호환·reference 경로는 혼용 | 3.7-F의 별도 핵심 기억 K 계약을 그대로 소비 |
| 감독 역할 | Story Author + Director | source-backed 표현 보조 reviewer | 안전한 장면 후보 제안자 |
| 장면 목표 | 명확한 `scene_mandate` | 없음 | response obligation과 scene anchor |
| 다음 전개 | `next_beats`, stance, pressure | 의도적으로 폐기 | `may_advance`, `hold_allowed`, causal frontier |
| 약한 입력·재개 | 직전 assistant tail과 continuity pack으로 검색을 다시 깨움 | 현재 입력과 직전 완료 턴 중심, 재진입 전용 계약은 부족 | lifecycle 좌표가 붙은 reentry/continuity plan 필요 |
| 검색 질의 | 현재 입력 + 장면·관계·미해결 스레드 다중 경로 | 현재 입력·언급 entity·이전 요약 중심 | typed scene/relation/thread query seed로 확장 가능 |
| 이전 맥락 | 약한 입력·시간 질문·장면 전환에 따라 1~3 anchor 적응 선택 | 직전 완료 논리 턴 중심의 고정 anchor | accepted logical turn만 쓰는 적응형 context 계획 필요 |
| 계층 기억 | context profile에 따라 episode/chapter/arc/saga를 확대·억제 | 여러 계층이 같은 `event_recent` 레인에서 함께 경쟁 가능 | 현재 질문에 맞는 한 해상도를 고르는 hierarchy zoom 필요 |
| 관계·비밀 경계 | 비교적 거침 | 출처·비밀·권한 경계 강화 | 인물별 지식·관계·종결 범위를 더 정밀하게 계획 |
| 지속 계획 | persistent guidance와 beat queue. 연속성은 강했지만 storyline 고착 위험 | 3.6 조사 당시 production 감독에는 없음 | current user turn에 귀속된 짧은 focus lease와 비강제 carry 후보. 장기 plot lock 금지 |
| 감독 입력 | 최근 대화·wake-up·active storyline을 분리해 제공 | 현재 입력과 delivered memory 중심 | accepted recent context와 제한된 guidance support가 필요 |
| 기억 표현 | 기억을 직접 낭독하기보다 행동·압력·subtext로 번역 | source-backed 표현 힌트 중심이지만 재료와 지속성이 좁음 | 출처 연결 expression cue와 필드 간 의미 중복 억제 필요 |
| 실패 처리 | Supervisor 실패가 응답을 막을 수 있음 | fail-open 지향 | fail-open, stale plan 재사용 금지 계획 |
| 상태 | 과거 구현 | 역사적 조사 기준선 | 계획 초안, 미구현 |

## 4. 3.7 재검증 packet — 3.6에서 이월된 약화·이상 동작

아래 P1/P2는 3.6 소스에서 직접 확인한 조사 결과다. Archive Center가 현재 3.7이므로
이를 전부 “현재도 발생하는 결함”으로 자동 승격하지 않는다. 3.7 source·자동 회귀에서
해결됐는지 먼저 재검증하고, 남아 있는 항목만 3.7 안정화 작업으로 닫는다. loaded
artifact와 실제 RisuAI 증거가 없는 항목은 source 수정 여부와 관계없이 실환경 완료로
표시하지 않는다.

아래 기존 근거의 `source/...:line`은 2026-07-29의 3.6 조사 당시 좌표이며, 이동한 현재
3.7 line을 증명하는 좌표로 재사용하지 않는다. 현재 source 판정은
[`3.7-full-bug-audit-and-remediation-plan.md`](3.7-full-bug-audit-and-remediation-plan.md)의
3.7-F 구현·회귀 증거와 현재 계약을 따른다.

| 항목 | 3.7 source·자동 회귀 판정 | 3.7-G / 후속 판정 |
|---|---|---|
| P1-1 강도 의미 | 3.7-F에서 weak/medium/strong의 typed 표현 권한 차이와 공통 금지선을 구현 | 실제 guide 품질은 3.7-G, Basic Publisher는 3.9, 다층 Director 확장은 4.9 |
| P1-2 `topK` 혼동 | 3.7-F에서 Vector 후보 상한과 별도 핵심 기억 K를 분리하고 HUD 의미를 정렬 | loaded 설정·HUD 확인은 3.7-G |
| P1-3 compact cap | 3.7-F에서 UTF-8 rune·가능한 단어 경계 cap 구현 및 회귀 통과 | 실제 한글 payload 확인은 3.7-G |
| P1-4 감독 지원 표면 | 3.7-F는 현재 지침의 출처·권한을 강화했지만 넓은 Director 지원 표면은 의도적으로 미구현 | 3.9 bounded support, 4.9 고급 eligible support가 소유 |
| P1-5 주관 기억·HUD | 3.7-F에서 owner hard cap 제거, 관련도·coverage와 item lineage ViewModel 구현 | 실제 private 비노출·HUD 확인은 3.7-G |
| P1-6 hierarchy·Persona | 3.7-F에서 현재 요청 관련도와 단일 hierarchy zoom을 구현 | 실제 장기 세션 품질은 3.7-G 이후 |
| P1-7 Supervisor 오류 | 3.7-F에서 malformed/schema-invalid/valid-empty/unsupported/provider-fail을 분리 | 실제 provider 응답 확인은 3.7-G |
| P1-8·P2 설정/UI | 3.7-F에서 영어 키워드 추론 제거, 명시 mode와 Go ViewModel 기반 UI를 구현 | loaded 저장·reload·표시는 3.7-G |

### P1-1. [3.6 조사] weak/medium/strong에 감독 권한 차이가 없었다

3.6 조사 당시 강도는 다음 항목의 coverage만 바꿨다.

- weak: fidelity warning, portrayal
- medium: weak + pacing, scene emphasis, callback
- strong: medium + reversible option

3.6 조사 당시 모든 강도에서 다음은 동일하게 금지됐다.

- 사용자·주인공 행동 결정
- 새 사실·감정·지식 생성
- 관계 변경
- 사건 종결
- 장면 이동
- 비가역 결과
- canonical write

`book_author`, `director`, `may_advance`가 들어오면 validator가 폐기했다. 당시 테스트
이름도 `StrengthChangesCoverageNotAuthority`였다.

근거:

- `source/prompts/supervisor_system.txt:2-50`
- `source/go-service/internal/httpapi/group_proxy.go:289-365`
- `source/go-service/internal/httpapi/group_supervisor_boundary_test.go:9-114`
- `source/go-service/internal/httpapi/group_turn_part14_test.go:14-85`

판정:

> 3.6 조사 당시 strong은 “강한 감독”이 아니라 “표현 힌트 종류가 더 많은 보조자”였다.

### P1-2. [3.6 조사] `topK`는 사용자가 기대하는 최종 기억 수가 아니었다

3.6 조사 당시 Go는 `top_k_definition`을 `vector_memory_search_limit_only`로 명시했다.

- 벡터 검색에 성공하면 유효 hit 수가 K보다 적어도 무관한 기억으로 채우지 않았다.
- 벡터가 준비되지 않으면 lexical 경로는 별도 safety limit까지 후보를 만들 수 있었다.
- episode, chapter, arc, saga, canon, private, persona, KG 등은 K 밖의 별도 자료였다.
- 최종 전달 개수는 7개 category 문자 예산과 중복 제거가 결정했다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_assembly.go:45-48`
- `source/go-service/internal/httpapi/prepare_turn_recall.go:840-1263`
- `source/go-service/internal/httpapi/prepare_turn_memory_budget.go:11-29`

UI 도움말은 이를 ChromaDB 검색 수라고 설명했지만, 상세 HUD는
`total memory target`이라고 표시했다.

근거:

- `source/Archive Center.js:1067-1068`
- `source/Archive Center.js:19257`

판정:

> 3.6 조사 당시 `topK=10`은 “최종 핵심 기억 10개”가 아니었고 상세 trace/HUD의
> `total memory target` 표시도 backend 정의와 충돌했다. 3.7-F source에서는 별도
> `core_objective_memory_delivery.v1`과 K 설정, Vector 후보 상한 표시로 분리됐으며
> loaded artifact 확인만 남아 있다.

### P1-3. [3.6 조사] 한 줄 압축의 길이 제한이 작동하지 않았다

3.6 조사 당시 `prepareTurnMemorySummary`는 `compactPrepareTurnLine(summary, 220)`을
호출했지만 함수는 `limit`을 버리고 공백만 정리했다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_assembly.go:898-922`
- 같은 파일 `:937-940`

이 결함이 관련도 자체를 망쳤다는 증거는 아니었다. 그러나 다음 현상을 만들 수 있었다.

- 장문 한 항목이 category 예산의 대부분을 차지했다.
- 길이가 예산을 넘으면 높은 순위 항목이 통째로 defer됐다.
- 짧은 hierarchy·canon·KG 항목만 남아 “핵심 기억보다 보조 잡음이 들어온다”는 체감을 키울 수 있었다.
- 한글·이모지 포함 길이 보장이 없었다.

1.0의 짧고 데이터베이스 같은 기억 체감과 직접 대비되는 회귀다.

3.7-F source에서는 공백 정규화 뒤 양수 rune 상한과 가능한 단어 경계 절단을 구현했고
자동 회귀를 통과했다. 실제 RisuAI의 한글·이모지 전달 결과는 3.7-G 판정 대상이다.

### P1-4. [3.6 조사·3.9/4.9 설계 공백] 감독관이 지원 자료 전체를 같은 수준으로 보지 못했다

3.6 조사 당시 최종 본문 기억은 다음 7개 class로 조립됐다.

1. direct evidence
2. protected secret
3. event/recent
4. character objective
5. subjective/relationship
6. world state
7. unresolved goal

근거:

- `source/go-service/internal/httpapi/prepare_turn_memory_budget.go:11-29`
- 같은 파일 `:132-145`

그러나 3.6 조사 당시 Supervisor의 source-linked memory support는 최종 전달된
`store.memories` lineage 중심이었다. NPC private memory, persona, KG, canon,
hierarchy, storyline, world rule이 본문에는 들어가도 감독 callback이나 동기 제안의
동일한 item-level 근거가 되지 못할 수 있었다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_planner.go:185-205`
- `source/go-service/internal/httpapi/output_fidelity_lineage.go:28-103`

판정:

> 3.7-F는 기존 서사 가이드의 typed 권한과 오류를 고쳤지만 넓은 Director support를
> 구현한 버전은 아니다. 3.9에서 bounded 입력을 먼저 고정하고 다층 감독은 4.9에서 확장
> `eligible_support_refs`와 함께 도입해야 한다.

### P1-5. [3.6 조사] 주관 기억은 보이기 어렵고 owner당 1개로 너무 거칠게 제한됐다

3.6 조사 당시 NPC private memory는 입력·장면 관련도, 감정 가중치, 중요도,
최신성으로 정렬됐다. 이 필터는 좋은 안전장치였다.

하지만 한 owner의 첫 기억을 고르면 같은 owner의 나머지는 모두
`owner_repetition_capped`로 제거됐다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_recollection.go:534-688`
- 특히 같은 파일 `:628-678`

이것은 일반적인 garbage 생성 원인이라기보다 **정보 손실 원인**이었다.

- 같은 인물의 약속, 오해, 경계심이 동시에 관련돼도 하나만 남았다.
- 첫 항목이 미묘하게 빗나가면 보완 기억이 사라졌다.
- 주관 기억은 객관 기억 후보와 별도인데도 사용자는 최종 전달 여부를 쉽게 볼 수 없었다.

백엔드에는 `memory_delivery_lineage.v1`과 일부 recollection surface가 있었지만 JS HUD는
최종 항목별 선택·defer·drop reason을 투영하지 않았다. compact production trace는
실제 기억이 있어도 `memoryCount: 0`으로 기록했다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_memory.go:276-360`
- `source/go-service/internal/httpapi/group_turn_prepare.go:813`
- `source/Archive Center.js:25817-25832`

판정:

> “주관 기억이 로그에 잘 안 찍힌다”는 3.6 지적은 맞았다. 3.7-F source는 owner hard
> cap을 제거하고 관련도·coverage·item disposition ViewModel을 연결했다. 실제 loaded
> HUD와 private 비노출은 3.7-G에서 확인해야 한다.

### P1-6. [3.6 조사] hierarchy와 Persona가 잡음처럼 보일 수 있었다

- chapter/arc/saga는 장기 진행 구간과 active 상태에 따라 들어왔고, 당시 입력과 의미
  관련성이 약해도 `event_recent` 예산을 차지할 수 있었다.
- Persona recollection은 부착된 항목을 당시 요청 관련도로 다시 거르는 단계가 약했다.
- `subjective_relationship`과 `protected_secret`은 관점 보존을 위해 일반 dedupe에서
  제외됐지만, 완전히 같은 문장까지 중복될 수 있었다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_memory.go:812-895`
- `source/go-service/internal/httpapi/prepare_turn_recollection.go:12-55`
- `source/go-service/internal/httpapi/prepare_turn_memory_budget.go:153-163`

이것이 “가비지 메모리”의 유일한 원인이라고 단정할 수는 없었다. 3.7-F source는
Persona 내용 관련도, owner coverage 뒤 추가 선택과 가장 좁은 관련 hierarchy 한 단계
선택을 구현했다. 실제 장기 대화에서의 잡음 체감은 3.7-G 이후 final item lineage와 함께
측정한다.

### P1-7. [3.6 조사] Supervisor의 malformed 응답과 정상적인 빈 제안이 구분되지 않았다

3.6 조사 당시 Supervisor 응답 JSON 파싱이 실패하면 raw text로 감싼 뒤, 지원 항목이
0개인 `ready_no_supported_proposal`로 끝날 수 있었다.

근거:

- `source/go-service/internal/httpapi/group_proxy.go:201-216`
- 같은 파일 `:337-365`

3.7-F source는 `malformed_failed_open`, `supervisor_schema_invalid`, `valid_empty`,
`unsupported_rejected`, `supervisor_provider_failed_open`을 분리했다. 실제 provider
응답의 HUD 표시는 3.7-G에서 확인한다.

### P1-8. [3.6 조사] 자동 장르 모드가 영어 리터럴 키워드에 의존했다

3.6 조사 당시 `auto` 판정은 영어 `romance`, `kiss`, `battle`, `fight` 같은 단어를
직접 검사해 한국어 장면이 standard로 떨어질 수 있었다.

근거:

- `source/go-service/internal/httpapi/prepare_turn_planner.go:409-449`

3.7-F source는 `auto`를 언어별 본문 키워드 추론 없이 `standard`로 해석하고 다른 mode는
사용자의 명시적 설정만 사용하도록 고쳤다. loaded 저장·reload는 3.7-G에서 확인한다.

### P2. [3.6 조사] 설정과 HUD의 드리프트

- JS sanitizer는 narrative mode를 항상 `auto`로 덮었다.
- 수동 mode 상수는 남았지만 UI에서는 사실상 선택할 수 없었다.
- 존재하지 않는 mode display를 갱신하려는 코드가 남아 있었다.
- `narrative_stance`와 `takeover_mode` 흔적은 있었지만 active JS 경로는 stance를 보내지
  않고 takeover를 off로 고정했다.
- 잘못된 strength 값은 off가 아니라 weak로 정규화됐다.
- custom category budget의 0은 비활성화가 아니라 자동값이었다.

근거:

- `source/Archive Center.js:10379-10385`
- 같은 파일 `:13587-13597`
- 같은 파일 `:50688-50701`
- `source/go-service/internal/httpapi/prepare_turn_planner.go:461-469`
- `source/go-service/internal/httpapi/prepare_turn_memory_budget.go:55-79`

3.7-F source는 mode 저장·전달, none에서만 mode 비활성, Go ViewModel 기반 HUD,
핵심 기억 K와 Vector 후보 수의 분리를 구현했다. 이 절의 목록은 현재 결함 목록이 아니라
3.7-G loaded UI 회귀가 다시 확인할 역사적 재현 항목이다.

## 5. 3.9 Basic Publisher와 4.9 고급 출판사 — 권장 서사 가이드 의미

핵심은 Supervisor의 권한을 두 축으로 분리하는 것이다.

```text
truth_authority
  = 모든 강도에서 false

canonical_write
  = 모든 강도에서 false

response_shaping_authority
  = 없음   → none
  = 약함   → response_focus
  = 보통   → bounded_scene_direction
  = 강함   → bounded_story_and_scene_direction
```

권한 우선순위:

```text
RisuAI native system·Host 제약
  > 현재 사용자 입력·명시적 정정
  > 직접 근거와 현재 canonical state
  > Go의 must_preserve / must_account / must_not_assert
  > 제한된 Story Author / Director 계획
  > 표현·페이싱 힌트
```

여기서 현재 사용자 입력의 우선권은 서사 명령과 명시적 정정에 관한 것이며, native system 안전·형식 제약을 덮는다는 뜻이 아니다.

### 강도별 권장 동작

| 강도 | 감독 동작 | 계속 유지되는 지원 | 금지 |
|---|---|---|---|
| 없음 | Supervisor LLM과 서사 방향 지시 없음 | 핵심 기억, 직접 근거, 비밀 guard, 주관 기억, 상태·관계·세계, 직전 턴 연속성 | scene mandate, next beat, pressure 지시 |
| 약함 | 현재 응답의 초점, 반응 의무, 이미 발동한 인과, 금지 전개를 정리 | 없음과 동일 | 새 beat·관계 변화·offscreen 사건·scene jump |
| 보통 | 약함 + Go가 만든 `may_advance` 중 우선 후보 또는 `hold_allowed` 선택, 페이싱·콜백. `explicit_redirection`이면 이전 thread 선택 0 | 없음과 동일 | 허용 envelope 확대, 사용자 행동 실행, parked storyline 재활성화 |
| 강함 | 보통 + 현재 사용자가 계속 선택한 thread에 한한 response/scene 범위 focus lease, arc context, ending edge. Go가 `advance_allowed=true`로 확정하고 유효한 `may_advance` ref가 있을 때만 그중 하나를 선호 | 없음과 동일 | focus lease 자동 갱신, hold 부재·침묵을 진행 허가로 해석, source 밖 새 갈등·반전, 관계·종결 권한, 단일 예정 plot 강제 |

### 장면 초점과 레일로딩을 분리하는 계약

서사 가이드의 “방향 유지”는 장기 plot lock이 아니다. Director가 고를 수 있는 것은 현재
응답 또는 현재 장면의 가역적 초점뿐이며, 다음 조건을 모두 적용한다.

1. **사용자 방향 우선:** `explicit_redirection`이 관찰되면 이전
   `foreground_thread_ref`, `arc_anchor_ref`, `preferred_frontier_ref`와 그 focus lease를
   같은 요청에서 해제한다.
2. **짧은 lease:** focus는 `response_only` 또는 최대 현재 scene 범위다. 다음 요청에
   자동 갱신하지 않고 current input의 동일 thread 선택 근거가 다시 필요하다.
3. **비반대는 동의가 아님:** 빈 입력·짧은 반응·모호한 이어쓰기·Supervisor 침묵을 기존
   storyline 유지 동의로 해석하지 않는다. 불명확하면 여러 open thread를 후보로만
   유지하고 visible beat를 강제하지 않는다.
4. **국소 종결:** 한 thread가 해결돼도 해당 thread만 focus에서 해제한다. 작품 전체
   엔딩·에필로그·세션 종료는 현재 사용자의 명시적 의도 또는 별도 global closure 근거가
   없으면 제안하지 않는다.
5. **복귀는 원인 기반:** parked·resolved thread를 다시 foreground로 올리려면 현재 사용자의
   명시적 재선택 또는 현재 장면에 도달한 독립적인 source-backed consequence가 필요하다.
   단순 관련도, 과거 momentum, arc 일관성과 “회수할 떡밥”은 복귀 근거가 아니다.
6. **응답 끝과 작품 끝 분리:** `ending_edge`는 이번 응답을 어디에서 끊을지 정하는
   표현 힌트다. 사건·arc·작품의 closure 상태를 바꾸지 않는다.

### 1.0 역할 분리에서 착안한 안전한 다층 감독

1.0에서 회수할 강점은 단일 Supervisor prompt가 아니라 **서로 다른 목적의 보조 역할을
연결한 구조**다. 실제 1.0은 Plugin Main과 Supervisor를 병렬 실행한 뒤 Sub LLM이 두
결과의 제한된 표면을 검토했다. 아래 `Recall Auditor → Director → Reviewer`는 그 역할
분리에서 착안한 **새로운 안전 직렬 설계**이며 1.0의 실제 호출 topology를 그대로
복원한다는 뜻이 아니다. `guidance_strength`와 보조 LLM 호출 깊이도 하나의 설정으로 묶지
않는다.

- `guidance_strength`: Director가 최종 응답 구성에 행사할 수 있는 권한의 상한
- `supervision_depth`: 회상 검토·방향 제안·교차검토 중 실제로 실행한 품질 보조 단계

권장 실행 깊이는 다음과 같다. 3.9는 `base_only`와 `directed`까지만 소유하고, 4.9에서
같은 Publisher owner를 확장해 `reviewed`와 `deep`을 추가한다.

| 실행 깊이 | 최초 소유 버전 | 보조 단계 | 허용 결과 | 실패 시 |
|---|---|---|---|---|
| `base_only` | 3.9 이전부터 유지 | Go 결정론적 기억·constraint 조립 | 기억 K, 직접 근거, 주관 기억, 비밀 guard | 같은 base support 유지 |
| `directed` | 3.9 | 기존 Publisher/Supervisor 1회 | 현재 strength 안의 typed response plan | proposal 0, base support 유지 |
| `reviewed` | 4.9 | Director → 필수 Reviewer | Reviewer가 승인·축소한 plan | proposal 0, base support 유지 |
| `deep` | 4.9 | Recall Auditor → Director → 필수 Reviewer | 고정 eligible set의 coverage gap까지 점검한 검증 plan | proposal 0, base support 유지 |

`없음`은 항상 `base_only`이며 Publisher·Supervisor·Director·Reviewer 호출이 0이다.
3.9의 `약함` 이상은 `directed`를 사용할 수 있다. 4.9 이후에는 모델 가용성, 요청 위험,
비용·지연 예산에 따라 `reviewed` 또는 `deep`을 별도로 요청할 수 있다. 강함이 항상 세 번
호출된다는 뜻도 아니고, 세 번 호출됐다고 강함 권한을 얻는다는 뜻도 아니다.

모든 요청은 `requested_guidance_strength`, `requested_supervision_depth`,
`effective_supervision_depth`, `effective_plan_status`, `degradation_reason`을 분리해
기록한다. 모델 미설정·가용 모델 없음·예산 skip·필수 Reviewer 없음이면
`effective_supervision_depth=base_only`, `effective_plan_status=proposal_0`이다. 설정된
strength를 기본 지침 prose나 이전 성공 plan으로 바꾸지 않는다.

`reviewed` 또는 `deep`에서 필수 Reviewer가 timeout, malformed, provider 오류, budget
skip 또는 독립성 요건 불충족이면 effective plan은 proposal 0이며 base support만 유지한다.
실패 뒤 Director 결과를 `directed`로 소급 채택하지 않는다. 더 낮은 깊이는 보조 호출 전에
Go가 별도 versioned policy로 선택하고 trace에 기록한 경우에만 허용한다.

각 역할의 권한은 다음처럼 제한한다.

1. **Recall Auditor:** Go는 첫 보조 호출 전에 lifecycle·owner·knower·visibility·privacy·
   authority·budget 검사를 통과한 `eligible_support_refs`와 hash를 고정한다. Auditor는 이
   집합 안에서 누락 표시·부분집합·우선 ref만 반환한다. 새 query, 검색 lane, graph
   traversal, parent/child, 원문 materialization 또는 visibility 변경을 요청할 수 없고
   부족하면 `coverage_gap`만 기록한다.
2. **Director:** current input, accepted context, 전달 가능한 ref와 typed constraint만 보고
   현재 strength 안의 응답 방향을 제안한다.
3. **Reviewer:** Director 결과를 source coverage, 사용자 의도, agency, secret, stale fence와
   중복 관점에서 승인·축소·거절한다. 새 지시·새 ref·더 높은 권한을 추가하지 않는다.
4. **Go validator:** 각 단계 전후에 source·branch·generation·revision·visibility·budget과
   contract hash를 검사한다. 검사를 통과한 plan만 해당 턴의 `effective_turn_contract`
   후보로 고정하며, 선택된 main-guidance materialization owner만 이를 실제 payload에 적용한다.
5. **Post-response Critic:** 저장 후보를 추출하는 별도 다음-turn 준비 역할이다. 현재 응답의
   Director plan을 소급 수정하거나 current canonical truth를 직접 쓰지 않는다.

같은 모델을 여러 역할에 재사용할 수는 있지만 이를 독립 검토라고 표시하지 않는다.
trace에는 role별 provider/model, 입력 data class, 호출 여부, latency, 결과 상태와
`same_model_review` 여부를 남긴다. 동일 응답을 이름만 바꿔 여러 단계의 결과로 재사용하는
것도 금지한다. `same_model_review`는 독립 Reviewer가 필수인 요청의 성공으로 인정하지
않는다.

1.0의 Plugin Main에서 가져올 것은 기억 후보를 다시 살피는 역할이지 사용자 입력을 몰래
고쳐 쓰는 권한이 아니다. Archive Center의 다층 감독은 원본 current input을 보존하고,
개선 제안이 필요해도 별도 비권위 proposal로만 다룬다.

### 출판사 실행과 Recomposer 직접 적용 위임 — 목표 계약, `DESIGN_ONLY_NOT_IMPLEMENTED`

후속 공동 사용 계약에서는 Archive Center Go가 Director/Reviewer 결과 중 검증된
response-shaping projection만
`publisher_plan.v1`로 확정하는 단독 publisher execution owner다. 기억 K, 직접 근거,
주관 기억, privacy·agency·authority constraint와 상태 레인은 publisher plan 밖의
immutable base support로 유지한다. publisher plan에는 raw Recall Auditor·Director·
Reviewer 응답을 넣지 않고 Go validator가 승인한 projection만 넣는다. identity는
session, backend request ID, one-shot transaction ID, logical/source turn, branch,
generation, revision, raw/effective input SHA-256, payload plan ID/hash, publisher plan
ID/hash, eligible-support hash, source snapshot hash, requested/effective strength·depth,
committed route, effective contract digest와 server-issued `issued_at`·`expires_at`을
포함한다. `none`, `base_only`와 `proposal_0`에서는 `publisher_plan=0`이고 main-guidance
materialization도 0이지만, base support는 그대로 effective input에 남는다. 아래 owner
분리와 transaction은 현재 v1 source 동작 설명이 아니라 수용 목표다.

Recomposer는 두 번째 Director나 두 번째 출판사가 아니다. 두 기능을 함께 사용하는
경우의 역할은 다음과 같이 나눈다.

- `publisher_execution_owner=archive_center_go`: publisher 호출·선별·검증과
  `publisher_plan.v1` 확정
- `effective_contract_compiler=archive_center_go`: Recomposer input proposal까지 검증해
  최종 prompt-assembly 계약 확정
- `main_guidance_materialization_owner=archive_center|recomposer`: publisher plan을 main
  payload에 materialize하는 단 하나의 owner
- `post_rewrite_context_consumer=none|recomposer`: 이미 적용된 plan을 post-response
  rewrite constraint로 읽는 consumer이며 두 번째 publisher 적용 owner가 아님
- `final_prose_owner=risu_main|recomposer_composer`: 최종 visible prose를 작성해 반환하는
  단 하나의 owner
- Semantic Prover와 Reviewer는 검증·축소·거절만 하며 final prose나 새 publisher
  directive를 직접 작성하지 않는다.

Recomposer가 없거나 위임이 선택되지 않은 턴에는 Archive Center가 승인된 publisher
plan을 `output_guidance`로 정확히 한 번 전달한다. Recomposer가 같은 요청의 plan과
lifecycle fence를 검증해 Input Planner·Contract Judge의 통합 proposal을 반환하고,
Archive Center Go가 이를 검증해 하나의 `effective_turn_contract`로 확정한 경우에만
`main_guidance_materialization_owner=recomposer`가 된다. Recomposer는 확정 계약을 변경
없이 main payload에 materialize하고 Archive Center의 같은 publisher plan 직접 블록은
중복 적용하지 않는다.

위임은 원자적으로 commit 또는 abort해야 한다. Recomposer는 immutable publisher plan을
자체적으로 축소·보류·재정렬·삭제하지 않고, 충돌이 있으면 통합 proposal과 conflict를
Go에 반환한다. Go가 새 effective contract를 확정하지 못하면 transaction 전체를 abort하고
동일 publisher plan의 Archive direct 경로로 복귀한다. consume-on-commit, load order와
post-response 실패의 상세 상태 전이는 16장을 정본으로 한다.

Archive Center의 must-preserve, must-account, must-not-assert, privacy, agency와 lifecycle
hard guard와 publisher plan은 Recomposer가 변경할 수 없다. Input Planner와 Contract
Judge는 새 ref·사실·strength·authority를 확정하지 않고 Go 검증용 proposal만 반환한다.
억제 대상은 중복 publisher plan뿐이다. 핵심 기억 K, 직접 근거, 주관 기억, 비밀 guard,
관계·세계 상태와 authority fence는 Recomposer 유무에 관계없이 유지한다.

`guidance_strength=none`에서는 Recomposer 설치 여부와 무관하게 Archive Center publisher
호출과 `publisher_plan`이 0이다. Recomposer 존재만으로 supervision depth를 `deep`으로
올리지 않으며, Recomposer의 독립 전·후처리 활성 여부는 Recomposer 자체 설정과 계약을
따른다.

권장 UI 설명:

> 없음은 서사 감독을 끄지만 기억 주입과 출처·비밀·연속성 보호는 유지합니다. 약함부터
> 현재 입력과 검증된 기억 안에서 이번 응답의 초점을 보조합니다. 사용자가 다른 이야기로
> 전환하면 이전 초점은 즉시 해제됩니다. 강도가 높아져도 한 줄거리를 계속 강제하거나
> 사용자 행동, 새로운 사실, 관계 변화, 사건·작품 종결을 임의로 확정하지 않습니다.

### 3.7 이후가 상속할 3.6 비퇴행 하한선

1.0의 강점을 회수하는 모든 변경은 다음 3.6 계약을 **동일하거나 더 강하게** 유지해야 한다. 이 목록은 구현 편의를 위해 완화할 수 있는 권장사항이 아니라 출시 하한선이다.

1. **명령 권한:** 현재 사용자 입력과 명시적 정정만 서사 명령 권한을 가진다. 과거 대화, 기억, 원작 근거, Supervisor 지침은 현재 입력을 덮지 못한다.
2. **사실 권한:** `truth_authority=false`, `canonical_write=false`를 모든 강도에서 유지한다. 감독 출력은 memory·canon·relationship·thread 사실의 저장 근거가 아니다.
3. **수명주기:** 현재 branch·generation·revision에 속하고 승인된 `active_final` 관찰만 사용한다. streaming 후보, superseded reroll, 삭제된 턴, stale 결과는 검색·감독·저장 어느 경로에서도 부활하지 않는다.
4. **지식·비밀 경계:** protected/private/subjective 자료는 owner·knower·visibility·spoiler 검사를 통과해야 한다. raw private text를 Supervisor, 본문, trace, HUD에 노출하지 않는다.
5. **근거 가시성:** 감독이 실제로 사용한 보조 근거는 본문 모델이 볼 수 있는 기존 레인에 이미 있거나, 짧은 표현과 `source_ref`로 `guidance_support`에 승격돼야 한다. 전달할 수 없는 근거로 만든 지시는 폐기하고 이미 전달된 텍스트는 중복하지 않는다.
6. **실패 격리:** Supervisor timeout, malformed JSON, provider 오류, 예산 부족은 본문 응답을 막지 않는다. 이전 성공 plan이나 일반적인 기본 지침으로 대체하지 않고 proposal 0으로 fail-open한다.
7. **단일 정책 소유자:** 선택·랭킹·중복 제거·예산·계층·Supervisor 검증·prompt assembly는 Go가 소유한다. JavaScript에는 과거 1.0의 선택 정책을 복사하지 않는다.
8. **관찰 가능성:** requested/eligible/delivered/deferred/dropped와 이유가 payload lineage와 HUD에서 같은 item을 가리켜야 한다. 넓은 내부 검색을 최종 전달이나 실제 사용으로 오인하게 표시하지 않는다.
9. **기본 동작 보존:** 서사 가이드 `없음`과 Supervisor 장애에서도 핵심 기억, 직접 근거, 비밀 guard, 주관 기억, 관계·세계 상태와 연속성 지원은 유지한다.
10. **호스트 증거:** RisuAI가 노출하지 않은 lifecycle 사실을 콘텐츠 모양이나 prompt 문구로 추정하지 않는다. 필요한 관찰은 공식 Host 계약에서 `observed`로 받거나 `unobserved`로 남긴다.

기능별 수용 판정은 다음 두 질문을 모두 통과해야 한다.

- 동일한 유효 입력에서 1.0이 더 잘했던 회상·연속성·장면 지휘 지표가 3.6 조사 기준선보다 개선되는가?
- 위 10개 하한선에서 완화, 우회, 불투명한 fallback 또는 새 JavaScript 정책이 0인가?

첫 질문만 통과하면 “1.0식 기능 복원”일 수는 있어도 Archive Center 개선으로는 수용하지 않는다.

### 1.0의 proactive complication

1.0의 proactive는 기존 갈등에서 파생된 가역적 complication을 제안할 수 있었다. 이를 기본 strong에 섞으면 안 된다.

- 기존 strong 설정은 표현 coverage였으므로 새 사건 생성 동의로 간주할 수 없다.
- 3.9·4.9 공통 계약은 Publisher가 source 밖 새 pressure·event를 만들지 못하게 한다.
- 제안이 본문에 나오면 비정설 proposal이라도 사용자가 본 사건이 된다.

따라서 다음과 같이 별도 명시적 opt-in으로 분리하는 편이 안전하다.

| stance | 동작 |
|---|---|
| reactive | 현재 입력과 이미 발동한 인과만 처리 |
| balanced | 기존 `may_advance` 중 preferred frontier 선택 |
| proactive | strong과 함께 사용할 때만 가역적인 creative complication 후보를 별도 lane으로 허용 |

proactive proposal은 `source_refs`가 아니라 `constraint_refs`를 사용해야 한다. 기존 source가 새 사건의 발생을 증명하는 것이 아니라, 단지 창작 허용 경계를 제공하기 때문이다.

## 6. 3.7-F K 기반과 4.0 recall·delivery 확장

### 6.1 3.7-F에서 완료된 `topK`와 핵심 기억 K의 분리

기존 `topK`를 곧바로 최종 기억 수로 재해석하면 검색 recall과 reference recall까지
깨질 수 있다는 3.6 조사 결과를 바탕으로, 3.7-F source는 두 계약을 이미 분리했다.

3.7-F source 기준:

1. 기존 backend `top_k`는 `vector_memory_search_limit_only`로 유지한다.
2. 별도 `core_objective_memory_max_items`와
   `core_objective_memory_delivery.v1`을 **핵심 연관 기억 수 K**로 제공한다.
3. 요청에 새 필드가 없으면 기존 item-count 동작을 유지한다.
4. Go가 K 선택·중복 제거·예산·lineage를 소유하고 JavaScript는 설정과 ViewModel만
   전달·표시한다.

3.6 조사에서 분리 전에 확인할 위험으로 남겼던 항목은 다음과 같다.

- 3.6 조사 당시 Go production trace는 vector-only라고 선언했다.
- 일부 JS fallback/non-compact 경로는 `settings.topK`로 최종 기억 줄도 자른다.
- reference recall limit도 기본적으로 `topK`를 상속한다.

3.7-F는 source·자동 회귀 수준에서 이 분리를 닫았다. 3.7-G는 loaded 설정·HUD·실제
payload에서 의미가 다시 합쳐지지 않는지 확인한다. 4.0은 K 필드나 UI를 다시 만들지 않고
이 계약을 `memory_recall_plan.v2`와 `memory_injection_plan.v2`가 소비하도록 확장한다.

### 6.2 사용자에게 보이는 K의 의미

> 현재 요청과 실제로 관련된 서로 다른 객관 사건 기억의 한 문장 요약을 최대 K개 최종 전달한다. 관련 기억이 K개보다 적으면 적게 전달하며, 수를 채우려고 무관한 최근 기억을 넣지 않는다.

즉 K=10이면:

- 관련 있고 서로 다른 객관 기억이 10개 있으며 예산이 충분하면 10개 전달
- 관련 기억이 6개뿐이면 6개 전달
- 나머지 4개를 recent garbage로 채우지 않음
- 예산으로 2개가 빠지면 `적합 10 / 전달 8 / 예산 보류 2`를 표시

K가 세는 것:

- 객관 사건 기억의 compact one-line summary

K가 세지 않는 것:

- direct evidence
- protected secret guard
- subjective/private memory
- 관계·인물 상태
- 세계 규칙
- unresolved goal
- episode/chapter/arc/saga support
- 서사 가이드

이 자료들은 별도 support lane이며 K를 소비하지 않는다. 다만 안전 우선 자료 때문에 실제 전달 K가 줄면 silent omission이 아니라 gap reason을 남겨야 한다.

값 의미:

- 필드 미지정: legacy 동작 유지
- 명시적 숫자: 1 이상의 정수만 허용
- `0`: 자동·무제한·비활성 중 어느 의미로도 암묵 변환하지 않고 validation error 또는 명시적 enum 사용
- 자동/무제한이 필요하면 숫자 0이 아니라 별도 `mode`로 표현

적용 순서:

```text
후보 검색
  → source·branch·visibility·relevance 검사
  → 객관 사건 기억만 분리
  → 관련도 정렬·중복 제거
  → core K 적용
  → 문자 예산 적용
  → final lineage와 gap reason 기록
```

### 6.3 한 문장 기억 규칙

- 의미 있는 한 문장은 extraction/critic 단계에서 만든다.
- delivery 단계의 하드 cap은 rune 기준 안전망으로만 사용한다.
- 접두사·source metadata가 summary 본문 예산을 먹지 않게 분리한다.
- 중간 절단으로 주어·사건·부정 의미가 바뀌지 않게 한다.
- 긴 항목은 무음 drop하지 않고 `deferred_item_too_long`을 기록한다.
- raw evidence를 자동으로 덧붙여 compact summary를 다시 장문화하지 않는다.

권장 trace:

```json
{
  "contract_version": "core_objective_memory_delivery.v1",
  "requested_max_items": 10,
  "candidate_fetch_count": 24,
  "eligible_distinct_count": 10,
  "delivered_count": 8,
  "missing_to_k": 2,
  "gap_reasons": ["budget_deferred"],
  "garbage_fill": false
}
```

고정 candidate 배수는 두지 않는다. 후보 폭은 관찰된 hit rate와 설정 상한 안에서 Go가 결정한다.

## 7. 3.8 time·state·perspective — 주관 기억 권장 계약

주관 기억은 핵심 객관 기억 K와 별도 레인으로 유지한다.

권장 표현:

```text
[owner: NPC_A | about: Player | private]
NPC_A는 이번 도움을 호의로 보지만 아직 신뢰의 증거로 받아들이지는 않는다.
```

필수 metadata:

- `owner_entity_id`
- `about_entity_ids`
- `knower_scope`
- `visibility_boundary`
- `source_ref`
- `source_turn`
- `delivery_role=subjective_support`
- `delivered`, `deferred`, `dropped`
- drop reason

3.7-F에서 이미 구현된 선택 동작의 **상속 비퇴행 기준**:

1. 관련 owner별 첫 항목 1개를 우선해 관점 다양성을 보장한다.
2. 남은 주관 기억 예산에서 고관련·비중복 항목을 추가한다.
3. owner당 고정 1개가 아니라 장면 복잡도와 예산에 따른 1~N개를 허용한다.
4. 객관 사건 요약과 같은 문장을 반복하지 않는다.
5. raw secret은 일반 본문이나 HUD에 노출하지 않고 guard만 전달한다.
6. 해당 인물의 subtext·주저·오해·회피에만 영향을 주며 다른 인물의 지식으로 전파하지 않는다.
7. 최종 payload에 들어간 exact item과 제외 이유를 HUD에서 확인할 수 있어야 한다.

3.8은 위 관련도·owner coverage·1~N 선택·HUD를 다시 구현하지 않는다. 3.7-F 결과에
story-time, current/history, owner/knower, 지식 경로와 visibility 의미를 추가해 같은
선택 항목이 누구의 어느 시점 인식인지 정확히 표현하는 버전이다.

## 8. 4.6~5.0 모델 적응형 RP 출판·시뮬레이션 로드맵 평가

### 8.1 현재 상태

[`4.6-5.0-model-adaptive-rp-publishing-roadmap.md`](4.6-5.0-model-adaptive-rp-publishing-roadmap.md)는
버전 소유권을 확정한 **구현 전 계획**이다. 다음 계약은 아직 활성 구현이 아니다.

- `simulation_snapshot.v1`
- `character_execution_card.v1`
- `voice_behavior_projection.v1`
- `world_execution_slice.v1`
- `narrative_execution_brief.v1`
- `model_capability_profile.v1`
- `scene_guidance_proposal.v1`
- `output_fidelity_report.v1`

또한 3.5, 3.9 Basic Publisher와 4.1 lifecycle prerequisite가 각 요구 수준으로 검증되기
전에는 4.6~5.0 결과를 production payload에 적용하지 않는다.

근거:

- `source/docs/4.6-5.0-model-adaptive-rp-publishing-roadmap.md`의 0장과 4~9장

### 8.2 1.0보다 뛰어난 부분

4.6~5.0은 다음 면에서 1.0보다 우수하게 작동할 잠재력이 크다.

- 인물별 known/suspected/unknown/misinformed/hidden/revealed 분리
- 관계의 방향·domain·상호성·동의·지속성 분리
- 이미 실현된 사건과 다음 후보 분리
- `hold_allowed`와 장면 정체 구분
- 한 thread 해결과 작품 전체 종결 분리
- `explicit_redirection`에서 이전 focus lease를 즉시 해제해 storyline 강제 복귀 차단
- 현재 장면에 필요한 인물·세계·세력만 동적으로 선택
- 모델명 대신 관찰된 능력에 맞춘 compact/standard/deep 표현
- Supervisor 실패 시 결정론적 base support로 fail-open
- source laundering과 stale plan 방지

근거:

- `source/docs/4.6-5.0-model-adaptive-rp-publishing-roadmap.md`의 6~10장

### 8.3 1.0보다 약하거나 비어 있지만 전부 복원할 대상은 아닌 부분

4.6~5.0의 simulation 계약만으로는 다음의 안전한 1.0 감독 체감이 돌아오지 않는다.
따라서 3.9 Basic Publisher에서 최소 체감을 먼저 복구하고 4.9에서 다층 감독으로
확장해야 한다.

- 명확한 근거리 작가 목표
- 현재 사용자 선택에 귀속된 짧은 scene focus lease
- 명시적 opt-in이 있는 reactive/balanced/proactive 주도성
- “계속”·재개 입력에서 reentry 방향 선택
- 메인 모델이 즉시 이해할 짧은 tempo/pressure cue

반면 순서가 고정된 beat queue, 자동 갱신되는 arc focus와 지속 연출 의도는 1.0의 강한
지휘력을 만들었지만 레일로딩 피드백의 원인이기도 하다. 이를 “비어 있는 기능”으로 세어
복원하지 않는다. `scene_guidance_proposal.v1`이 단일 정답 전개를 강제하지 않는 것은
의도적인 비퇴행 성질이다.

### 8.4 종합 판정

| 평가 축 | 판정 |
|---|---|
| 1.0 감독 UX와의 유사성 | 부분적이며 의도적. 장면 후보·제약·짧은 focus는 회수하지만 persistent plot lock은 복원하지 않음 |
| 사실·비밀·관계 정확성 | 4.6~5.0이 설계상 우수 |
| 장면 추진력 | 단일 storyline 추진력은 1.0보다 약할 수 있으나 사용자 redirection과 다중 thread 자유도는 더 강해야 함 |
| 장기 확장성 | 4.6~5.0이 우수 |
| 현재 즉시 사용 가능성 | 낮음. 구현 전 계획이며 prerequisite 미완료 |
| 최선의 사용법 | 3.9 Basic Publisher를 기반으로 4.6~4.8 실행 표면을 추가하고 4.9에서 단계형 Director와 만료 가능한 focus lease를 확장 |

4.6~5.0에는 `guide OFF + simulation support ON` 조합이 정의돼 있다.

근거:

- `source/docs/4.6-5.0-model-adaptive-rp-publishing-roadmap.md`의 독립 activation 계약

이 조합은 사용자가 원하는 `없음 = 기억·시뮬레이션 보조만`과 정확히 맞는다. 다만 현재
3.7 source가 4.6~5.0 simulation support를 구현했다는 뜻은 아니다.

### 8.5 문서 드리프트

- 4.6~5.0 구현은 과거 초안의 `supervisor_scene_proposal.v2`를 별도 계약으로 되살리지
  않고, 현재 계약과 3.9 Basic Publisher의 versioned successor를 단일 owner로 사용해야 한다.
- 문서는 guide base와 4.6~5.0 support의 독립 조합을 설명하지만 3.6 조사 당시 production
  output guidance는 Supervisor 성공 결과 중심이었다. 3.7 이월 여부는 4장의 재검증 대상이다.
- `narrative_execution_brief.v1`의 migration과 version negotiation이 아직 없다.

따라서 3.9 Basic Publisher 구현에서 현재 계약과 목표 계약의 연결 방식을 먼저 명문화하고,
4.9는 이를 교체하지 않고 versioned successor로 확장해야 한다.

### 8.6 버전별 기반으로 회수할 1.0의 추가 강점

아래 항목은 처음 정리한 “짧은 기억 K + 단계형 Director”만으로는 충분히 회수되지 않는다. 1.0의 구현을 그대로 복사하지 않고, 각 장점을 3.6의 출처·수명주기·권한 계약으로 다시 설계해야 한다.

#### A. [4.0/4.1] 약한 입력과 재개의 continuity query

1.0은 빈 입력이나 재개 상황에서 직전 assistant 응답의 마지막 2~3문장을 뽑아 검색을 다시 깨웠다. 이후 continuity pack의 active storyline, relationship shift, latest episode, world constraint를 섞어 실제 장면 검색어를 보강했다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:10252-10275`
- 같은 파일 `:13083-13124`

이 능력은 사용자가 `계속`, 짧은 반응, 공백에 가까운 입력을 보냈을 때 literal 입력만 검색해 핵심 기억이 빠지는 현상을 줄였다. 다만 1.0의 다국어 고정 문자열 resume 판정은 그대로 복사하지 않는다.

권장 계약:

- Host가 실제로 관찰한 idle/reentry lifecycle과 현재 입력을 Go에 전달한다.
- Go가 현재 branch의 승인된 마지막 assistant `active_final` tail과 현재 활성 thread/state로 `continuity_query_plan`을 만든다.
- 텍스트만으로 재개 의도를 판단해야 하면 하드코딩한 단어 목록이 아니라 versioned semantic intent 결과를 사용하고, 판정 근거와 confidence를 기록한다.
- continuity query는 후보 검색 seed일 뿐 사실·명령·정사 근거가 아니다.
- 현재 입력에 명확한 새 방향이 있으면 continuity seed를 강등하거나 폐기한다.

필수 trace:

```json
{
  "mode": "reentry|weak_input|explicit_request|none",
  "source_turn_refs": [],
  "query_seed_refs": [],
  "current_input_precedence": true,
  "selected": false,
  "drop_reason": ""
}
```

#### B. [4.0] 장면·관계·미해결 스레드의 다중 검색

1.0은 current input 한 개만 검색하지 않았다. 장면의 위치·분위기, 관계 당사자·변화, 미해결 사안을 분리해 최대 3개의 보조 query로 만들고 중복 결과를 합쳤다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:10459-10588`
- 같은 파일 `:10595-10657`
- 현재 `source/go-service/internal/httpapi/prepare_turn_assembly.go:65-94`

권장 계약:

- Go가 `scene_seed`, `relationship_seed`, `thread_seed`, `current_input_seed`를 typed query로 만든다.
- query 원료는 현재 branch에서 유효한 state와 공개 가능한 compact 표현만 사용한다.
- private 원문은 query provider로 보내지 않는다.
- 여러 경로에서 같은 item이 잡힌 횟수는 관련도 보조 신호일 뿐 진실성·해결 가능성·정사 신뢰도를 높이지 않는다.
- 모든 경로 결과는 동일한 source·branch·revision·visibility·duplicate 검사를 거친 뒤에만 eligible pool로 들어간다.

이 계약은 검색 폭을 넓히지만 최종 핵심 K를 무관한 항목으로 채우는 허가가 아니다.

#### C. [4.0] 입력 성격에 따른 적응형 이전 맥락

1.0은 약한 입력, 시간 질문, 장기 공백 뒤 재개, 다중 인물·스레드 압력에서 이전 anchor 슬롯을 늘리고, 사용자가 장면을 명시적으로 전환하면 오래된 storyline과 중복 helper를 강등했다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:17074-17210`
- 같은 파일 `:22206-22240`
- 현재 `source/go-service/internal/httpapi/group_turn_prepare.go:542-555`

3.6 조사 당시 Go의 input context는 직전 완료 논리 턴 중심이었으므로, 1.0의 적응형
재정렬 감각을 별도 계약으로 회수할 가치가 있다.

권장 계약:

- 원료는 현재 branch의 승인된 논리 턴과 유효 state/thread만 허용한다.
- `weak_input`, `temporal_request`, `reentry`, `explicit_redirection`, `multi_entity`, `multi_thread`를 typed need signal로 표현한다.
- 신호별로 필요한 anchor family와 최대 슬롯·문자 예산을 Go가 결정한다.
- 현재 입력과 겹치는 문장, 이미 다른 레인에 전달된 helper, stale arc는 drop reason과 함께 제거한다.
- 고정 “최근 N개”를 raw message로 넣는 방식은 금지한다.

#### D. [3.8→4.0] 시간 의도에 맞는 실제 anchor

1.0은 “처음”, “중간”, “최근”, “N번째 턴” 요청을 감지해 episode/direct evidence/turn span에서 실제 시간 anchor를 구성했다. 단순 의미 유사도만으로 과거 질문을 처리하지 않았다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/services/prepare_turn_injection.py:470-607`
- 현재 `source/go-service/internal/httpapi/group_turn_contract_seq16_temporal.go:129-187`

3.6 조사 기준선은 temporal bucket과 validity-first reading으로 더 안전한 기반을
갖췄지만, 3.8의 시간 상태와 4.0의 질문별 anchor 선택을 최종 기억·감독 표면으로
연결하는 계약은 더 명확해져야 한다.

권장 계약:

- 시간 의도는 `earliest|middle|latest|turn_range|none`의 versioned 값으로 표현한다.
- active branch, accepted final, visibility, spoiler, knower 검사를 먼저 적용한 뒤 범위 안의 direct evidence와 객관 사건 기억을 선택한다.
- 증거가 없으면 임의의 가장 가까운 기억을 사실처럼 쓰지 않고 temporal gap을 표시한다.
- 시간 anchor는 일반 semantic ranking보다 우선할 수 있지만 현재 사용자의 명시적 수정과 현재 장면 사실보다 우선하지 않는다.

#### E. [4.0] hierarchy stack이 아니라 hierarchy zoom

1.0은 profile에 따라 memory, episode, chapter, arc, saga를 모두 같은 비중으로 쌓지 않았다. episode가 있으면 chapter를 줄이고, 넓은 saga가 필요한 profile에서만 더 큰 해상도를 허용하는 식으로 한 단계에 초점을 맞췄다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/Archive Center.js:20250-20289`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/services/prepare_turn_injection.py:610-629`
- 현재 `source/go-service/internal/httpapi/prepare_turn_memory_budget.go:132-145`

현재 여러 hierarchy 자료가 `event_recent` 안에서 함께 경쟁하면 짧은 보조 요약이 핵심 기억보다 많이 남아 garbage처럼 보일 수 있다.

권장 계약:

```text
현재 장면·짧은 회상    → event memory 또는 episode
여러 장면의 국면 질문  → chapter
장기 목표·방향 질문    → arc
작품 전체 재진입       → saga
```

- 기본은 현재 질문에 가장 좁으면서 충분한 한 해상도다.
- 더 넓은 계층을 선택하면 겹치는 좁은 계층을 반복 전달하지 않는다.
- hierarchy는 support-only이며 핵심 객관 기억 K를 소비하지 않는다.
- 선택한 수준, 가려진 수준, source turn range, overlap/defer reason을 기록한다.
- “항상 episode+chapter+arc+saga”와 “profile이 크면 무조건 전부 투입”은 금지한다.

#### F. [3.9→4.9] Publisher가 보는 승인된 최근 맥락

1.0 Supervisor는 `Recent_Conversation`, wake-up context, active storylines를 별도 표면으로 받았다. 이 덕분에 검색 기억만으로는 알기 어려운 직전 말투·행동·장면 끝을 감독 지시에 반영할 수 있었다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/roles/supervisor.py:238-281`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/prompts/supervisor_system.txt:35-37`
- 현재 `source/go-service/internal/httpapi/prepare_turn_planner.go:87-159`

1.0의 raw 최근 10개 투입은 복원하지 않는다. 대신 다음 표면을 사용한다.

- accepted `active_final` 논리 턴의 compact recent context
- 각 항목의 turn·branch·generation·revision·source ref
- 과거 user message는 continuity evidence일 뿐 현재 명령 권한은 없음
- private·deleted·superseded·streaming 항목 0
- 현재 입력과 충돌하면 현재 입력 우선

최근 맥락은 기억으로 저장되거나 사실 권한을 얻지 않고, 이번 응답의 연결과 표현을 위한 support-only surface다.

#### G. [3.9→4.9, 4.1] 지속 지휘 대신 focus lease와 비강제 carry

1.0은 arc focus, narrative drive, live tensions, beat queue, carry threads, scene drive, carry targets, blocked routes, tempo, handoff edge를 다음 요청으로 이어 갔다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/services/prepare_turn_supervisor.py:156-235`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/models.py:396-499`

이 지속성은 1.0 감독 체감의 핵심이지만, 사용자 피드백상 storyline을 너무 오래
foreground에 고정해 다른 이야기로의 전환을 되돌리는 원인이기도 했다. LLM이 쓴 자유문장
plan을 session+turn 키로 그대로 캐시하면 stale 지휘와 사실 세탁뿐 아니라
**레일로딩 carry**가 발생한다.

권장 `guidance_carry_state`:

- `state_id`
- `branch_id`, `request_generation`, `expected_revision`
- `source_refs`, `source_hash`
- `arc_anchor_ref`, `foreground_thread_ref`
- `focus_lease_scope=response_only|scene`
- `focus_lease_source_user_turn_ref`, `focus_lease_expires_turn`
- `renewal_required=true`, `release_on_explicit_redirection=true`
- `carry_refs`, `hold_refs`, `blocked_route_refs`
- `created_turn`, `last_confirmed_turn`
- `status=active|held|expired|invalidated`
- `transition_reason`

Go는 매 turn의 accepted state/thread에서 이 상태를 재구성한다. Supervisor는 typed 후보를
선택할 수 있지만 자유문장을 장기 사실로 저장하지 않는다. focus lease는 현재 사용자
입력이 같은 thread를 다시 선택한 증거가 있을 때만 갱신한다. `explicit_redirection`,
reroll, delete, branch change, revision mismatch에서는 즉시 해제·무효화하며, 검색 부재나
Supervisor의 침묵만으로 thread를 해결·만료하거나 lease를 갱신하지 않는다.

#### H. [4.8, 4.1 소비] thread focus radar와 momentum

1.0은 active storyline 외에도 next pressure, payoff/callback 후보, unresolved tension, beats to avoid를 별도 momentum packet으로 만들었다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/services/prepare_turn_momentum.py:69-115`
- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/prompts/supervisor_system.txt:92-96`

권장 계약:

- 각 thread는 안정적인 `thread_ref`를 가진다.
- Director가 할 수 있는 동작은 `foreground`, `carry`, `callback`, `hold`, `defer`다.
- `resolve`는 Go의 별도 `resolution_allowed`와 complete-turn evidence가 있을 때만 가능하다.
- 높은 retrieval score, 여러 query hit, 오래 등장하지 않음은 해결 증거가 아니다.
- `hold_allowed`인 장면에서 활성 thread가 있다는 이유만으로 가시적 beat를 강제하지 않는다.
- `foreground`는 focus lease가 있는 동안만 유효하고 `carry`는 향후 후보일 뿐 다음 응답의
  의무가 아니다.
- 사용자가 다른 thread·인물·장소·목표를 선택하면 기존 foreground는 `held` 또는
  `parked`로 내려가며, 새 사건을 만들어 사용자를 이전 thread로 돌려보내지 않는다.
- 한 thread의 `resolve`는 다른 open thread나 작품 전체를 함께 resolve하지 않는다.

#### I. [4.7~4.8, 3.9 소비] 관계·세계·캐릭터의 Director 실행 표면

1.0은 relationship shift, stable world constraint, scene-local world context, character voice와 guardrail을 Supervisor 재료로 사용했다. 3.6은 이 데이터를 더 안전하게 보관하지만 Director가 사용할 source-linked 실행 표면은 아직 충분히 연결되지 않았다.

권장 분리:

- 관계: 객관적 transition과 인물별 subjective posture를 분리하고 direction·domain·mutuality·consent를 유지
- 세계: stable world rule과 현재 장면 local constraint를 분리
- 캐릭터: voice·behavior·known/unknown·agency boundary를 분리
- 모든 항목: current/history/transition ref와 knower/visibility 제공

Director는 이 표면에서 항목을 선택할 뿐 새 관계 단계, 새 세계 규칙, 인물의 숨은 감정이나 지식을 작성하지 않는다. 관계·세계·정사 권한은 모든 강도에서 false다.

#### J. [4.7→4.9] 기억을 설명이 아니라 행동으로 표현

1.0 프롬프트는 기억을 직접 인용하거나 설명문으로 낭독하기보다 hesitation, warmth, wariness, pressure 같은 행동·subtext로 드러내도록 요구했다. 또한 Story Author와 Director 필드가 같은 hook을 반복해 바꾸어 말하지 않도록 구분했다.

근거:

- `_archive/reference/external-examples/example/Archive Center 1.0(fix)/backend/prompts/supervisor_system.txt:15-26`
- 같은 파일 `:40-96`

권장 계약:

- `expression_hints`는 반드시 memory/state `source_ref`를 가진다.
- 힌트는 말투·행동·반응 압력만 제안하고 숨은 비밀이나 새 사실을 외부화하지 않는다.
- `arc_anchor`, `response_focus`, `required_outcome`, `forbidden_move`, `pressure`, `ending_edge`는 서로 다른 의미 역할을 가져야 한다.
- 동일 source를 여러 필드가 쓰더라도 같은 문장을 반복하면 semantic dedupe한다.
- 본문에 기억 설명을 반드시 넣으라는 지시가 아니라, 관련될 때 자연스럽게 행동화하는 선택지를 준다.

#### K. [4.9] context profile에서 capability-aware 표현으로

1.0은 mid/wide/ultra/extreme에 따라 기억 줄 수와 Story Author/Director 상세도를 달리했다. 이 방식은 큰 context 모델에서 감독 재료가 지나치게 줄어드는 것을 막았지만, 모델명·profile별 고정 규칙과 계층 과투입 위험도 있었다.

회수할 것은 고정 profile 자체가 아니라 **검증된 동일 항목을 모델과 Host 예산에 맞게 얼마나 압축해 표현할지 조절하는 능력**이다.

- capability profile은 eligibility·truth·visibility·핵심 K의 의미를 바꾸지 않는다.
- compact/standard/deep는 동일한 item ref의 표현 길이와 optional explanation만 바꾼다.
- 작은 profile에서도 agency·secret·authority guard는 생략하지 않는다.
- 큰 profile도 hierarchy stack, raw chat 확대, 무제한 Supervisor 자료를 허용하지 않는다.
- 모델명 하드코딩 대신 관찰된 입력 한도·구조화 출력 성능·실패율을 사용한다.

#### 8.6.1 [4.0/4.1] 외부 장기 기억 사례로 보강하는 다섯 전달 원칙

`_archive/reference/external-examples/example/hayaku_locator_continuity.js`의 HAYAKU Locator Continuity v2.3.17은 별도 Critic·embedding 호출 없이, 응답 모델이 만든 구조화 패킷을 원장에 보관하고 모델 없는 검색 결과를 짧은 자연어 기억으로 다시 전달한다. 로컬 참조 파일의 SHA-256은 `B1224C35FD1E93D7C7EAFE157EAC84B4ED529CEE1FDEC0C82482E2708A8B7171`이다.

이 파일은 1.0 동작의 역사적 근거가 아니며 Archive Center에 그대로 이식할 구현도 아니다. 특히 HAYAKU의 단일 JavaScript 원장·선택·prompt 조립 소유권은 영구 Host/backend 경계와 맞지 않는다. 아래 다섯 항목은 **1.0의 짧고 관련성 높은 기억 전달 체감을 3.6의 안전 계약 위에서 더 확실히 구현하기 위한 행동·품질 기준으로만 참고**한다.

- HAYAKU의 함수, 정규식, 상수, prompt 문구, packet schema, 저장 키와 hook 코드를 복사하거나 번역 이식하지 않는다.
- HAYAKU의 JavaScript 원장·선택·검색·예산 경로를 Archive Center의 병렬 runtime 또는 fallback으로 추가하지 않는다.
- 먼저 Archive Center의 기존 Go owner와 계약으로 같은 요구를 충족할 수 있는지 확인하고, 재현된 결함과 수용 시험이 있을 때만 최소 변경한다.
- 아래 필드명과 trace명은 HAYAKU 코드의 호환 계약이 아니라 Archive Center가 독립적으로 정의할 설계 예시다.
- 모든 선택·랭킹·감사·렌더링·예산·lineage는 Go가 소유하고 JavaScript에는 정책을 추가하지 않는다.

##### 보강 1. 최종 전달 K와 실제 전달 증명

1.0의 체감은 검색 후보가 많았다는 사실보다 필요한 기억이 실제 본문 입력에 들어왔다는 결과에서 왔다. 따라서 다음 단계를 서로 다른 수치와 exact item ref로 남긴다.

```text
candidate_fetched
  → eligible
  → selected_for_core_k
  → rendered
  → delivered_to_payload
```

- 사용자-facing K는 `delivered_to_payload`만 센다.
- 관련 기억이 부족하면 적게 전달하고 filler를 넣지 않는다.
- 관련 기억과 예산이 충분한데 renderer·assembly에서 K보다 적게 들어가면 정상 축소가 아니라 `delivery_contract_gap`이다.
- budget, duplicate, item-too-long, privacy, stale fence 등 단계별 제외 이유를 같은 item ref에 연결한다.
- support·주관·비밀·직접 근거·계층 기억은 계속 별도 레인이며 K를 소비하지 않는다.

이는 6장의 `core_objective_memory_delivery.v1`을 대체하지 않고, 후보 수가 아니라 **실제 최종 주입 수**가 계약의 끝이라는 점을 강화한다.

##### 보강 2. 구조화 저장과 한 문장 전달 표현의 분리

장기 저장본은 source ref, actor, event, polarity, time scope, branch·generation·revision, owner·knower·visibility와 lifecycle을 잃지 않는다. 본문 모델에 전달할 때는 검증된 동일 의미를 한 문장 `delivery_text`로 표현한다.

- 의미 있는 한 문장은 admission/extraction/Critic 단계에서 만들고 source item에 결속한다.
- delivery renderer는 새 해석이나 사실을 작성하지 않고 승인된 한 문장과 결정적 경계 문구만 조립한다.
- score, 내부 ID, ranking breakdown, lifecycle metadata는 문장 본문에 섞지 않고 sidecar lineage로 유지한다.
- JSON·raw evidence·긴 계층 객체를 한 문장 기억 대신 직접 주입하지 않는다.
- 부정, 주체, 대상, 시간 범위를 보존할 수 없으면 자르지 말고 보류 또는 재추출한다.

이 분리는 데이터베이스처럼 정확한 구조를 유지하면서도 1.0처럼 기억이 짧고 읽기 쉬웠던 장점을 회수한다.

##### 보강 3. 최근·연속성·장기 기억의 다중 horizon

전역 관련도 순위 하나만 사용하면 최근 항목이 장기 핵심 사건을 밀어내거나 오래된 중요 기억이 현재 장면을 압도할 수 있다. 검색 결과를 request-scoped horizon으로 나눈 뒤 최종 K에서 다시 합친다.

- `recent`: 현재 장면과 가까운 accepted final·current state
- `continuity`: 관계 변화, 약속, 미해결 thread, 지속 제약
- `historical`: 오래된 사건, 실제 temporal anchor, 분쟁·변천 기록

horizon은 새 사실 상태나 별도 저장 권한이 아니라 검색·coverage view다. 각 항목은 먼저 3.6의 source·branch·generation·revision·visibility·privacy gate와 최소 관련도를 통과해야 한다. 관련 항목이 없는 horizon의 자리를 무관한 항목으로 채우지 않고 남은 K를 다른 관련 horizon에 돌린다. horizon 보장은 고정 3분할 quota가 아니라 **최근·장기 한쪽의 독식을 막는 적응형 coverage 규칙**이어야 한다.

##### 보강 4. 요약 누락 coverage audit와 제한 근거 복구

짧은 요약은 토큰 효율이 높지만 actor, 부정, 핵심 인물, 사건 anchor를 빠뜨리면 이후 검색에서 영구적으로 사라질 수 있다. compact summary를 승인하기 전 또는 전달 후보로 사용할 때 다음 coverage를 검사한다.

- accepted source에 있는 핵심 actor·target이 요약에 남았는가
- 사건·장소·관계의 canonical anchor 또는 portable ref가 남았는가
- 긍정·부정, 발생·미발생, 현재·과거 의미가 뒤집히지 않았는가
- 주관 기억의 owner·knower·visibility 경계가 유지됐는가
- summary가 가리키는 source ref가 현재 branch·revision에서 여전히 유효한가

누락 시 허용되는 순서는 `재추출 → 해당 summary 보류 → 승인된 짧은 direct evidence를 별도 support lane으로 제한 복구`다. raw 대화 덩어리를 core K로 승격하거나, private·stale·superseded 근거로 빈 요약을 채우지 않는다. fallback 근거도 본문 payload에 실제 전달된 경우에만 delivered로 기록한다.

##### 보강 5. 현재·과거·대체·분쟁 기억의 의도별 사용

같은 기억도 현재 상태 질문, 과거 회상, 변화 과정, 사실 충돌 질문에서 역할이 다르다. 새 heat 상태를 독립 진실로 만들지 않고 기존 3.6 lifecycle과 source fence를 사용해 request-scoped time intent를 적용한다.

- 현재 질문: `active_final`과 현재 유효 상태를 우선하고 superseded·deleted·no-longer-true 항목을 현재 사실로 전달하지 않는다.
- 과거 질문: 유효한 historical evidence를 시간 표지와 함께 허용하되 현재 상태처럼 표현하지 않는다.
- 변화 질문: 이전 상태와 이를 대체한 accepted transition을 함께 제시한다.
- 분쟁 질문: contested·unknown을 단일 확정 사실로 평탄화하지 않는다.
- 어떤 time intent도 branch·privacy·spoiler·authority gate를 우회하지 못한다.

이 다섯 원칙은 1.0의 기억 체감을 복원하는 동안 3.6의 장점을 약화시키지 않는다. HAYAKU를 통해 점검하는 것은 응답 모델의 패킷 작성이나 JavaScript 원장이 아니라 **짧은 전달 표현, horizon coverage, 누락 감지, lifecycle-aware recall, 실제 전달 증명이라는 설계 목표**다. 실제 구현은 Archive Center의 현재 데이터 모델과 Go 정책 소유권에 맞춰 독립적으로 설계한다.

#### 8.6.2 [공통 비퇴행] 세 외부 사례와의 추가 비교

다음 세 파일을 장기 기억·정사·멀티턴 연속성 관점에서 정적 분석했다.

| 참조 파일 | SHA-256 | 주된 구조 | 참고 가치 |
|---|---|---|---|
| `WygLoreLeaf-3.0.0.js` | `8A22A13E1184460F60CC2255249F82A279B0C1C8A723B9E4FC40790471F7B39A` | 가중 연상 그래프, 되감기 가능한 commit, 격리 후 삭제, 선택형 LLM·embedding | 비권위 Host 관측과 실제 삭제의 분리, 재빌드 가능한 retrieval sidecar, 외부 호출과 장기 작업의 관찰성 |
| `omninode-beta18.js` | `794C7321F2F5D6677338E979FD60D9DFDC49941AE8E2326AE9DF47BC3C0EF520` | 노드·관계 graph, keyword/vector 융합, 요약 계층, base+diff rollback, 수동 merge UI | 비확장 degraded, 비권위 학습 신호, 원자적 파생 쓰기, 완전한 rollback domain |
| `agents-pipeline-Living-Canon-1.0-멀티턴 (2).json` | `760404BDA459726048959D4199F503874068A9E88CBF43897905D7CA7AEC95EB` | 사실·지식·상태·offscreen·회상·사후 기억 정리를 나눈 멀티에이전트 preset | truth·assertion·knowledge path·epistemic state 분리, 상태 commit barrier, present bridge, locator형 회상 색인 |

세 파일 모두 1.0 동작의 역사적 근거가 아니고, 실제 RisuAI에서 검증한 품질 증거도 아니다. 함수, 정규식, prompt, schema, 저장 키, 점수, 임계값, agent 수를 복사하지 않는다. 특히 JavaScript 또는 자유문장 agent가 저장·검색·ranking·예산·canonical mutation을 소유하는 구조는 Archive Center의 영구 Host/backend 경계와 맞지 않는다.

##### 현재 구현, 문서화된 목표와 새 보강의 관계

| 외부 사례의 장점 | 현재 source | 문서화된 목표와 이번 보강 |
|---|---|---|
| keyword·embedding·관계 multi-hop 검색 | exact/lexical/vector 후보와 KG support 신호는 존재하지만 완성형 graph-neighborhood 복합 retrieval의 구현 증거는 아니다. | 4.0은 source-backed atomic item·기존 edge를 읽는 retrieval graph-neighborhood만 소유하고, 4.6은 stable identity·3.9 관계·4.0 recall을 소비하는 관계 graph projection·UI를 소유한다. similarity와 edge는 truth·importance·merge 권한이 아니다. |
| 구조화 기억과 짧은 전달문 분리 | 현재 기억 payload와 lineage가 있으나 8.6.1의 final K·승인된 `delivery_text` 전체 계약은 권장 목표다. | source sidecar, 한 문장 전달과 actual delivered K를 구현·검증해야 한다. |
| 계층 요약과 parent-child 기억 | hierarchy 자료는 존재하지만 request-scoped zoom의 완료 증거는 아니다. | 고정 계층 stack 대신 hierarchy zoom과 coverage reason을 쓰는 것이 이 문서의 목표다. |
| 되감기와 격리 | source revision fence, rollback과 durable vector outbox가 존재한다. | 모든 transitive projection의 stale quarantine, 비권위 Host read 격리와 완전 rollback manifest는 수용 목표다. |
| exact·pending·proposed와 owner·holder 구분 | 일부 `truth_scope`, `epistemic_mode`, state transition 경로가 존재한다. | exact/approximate/unknown, 계획 lifecycle, owner/holder와 방향별 관계 current/history는 정밀 장기 기억 로드맵 목표이며 아래 commit barrier가 수용 기준을 선명하게 한다. |
| last-known과 current 분리, offscreen no-replay | source acceptance와 stale result 차단 기반은 존재한다. | time intent, `turn_realization.v1`, read-only offscreen proposal과 present bridge는 후속 로드맵·수용 목표다. |
| source pointer와 한 주제 단위 | source row·revision pointer와 atomic 기억 기반이 일부 존재한다. | one-subject locator는 반복 검색 비용이 실제로 재현될 때만 기존 trace로 먼저 시도한다. |
| human merge UI | 현재 similarity 기반 memory dedup이 있으나 범용 human-reviewed merge의 완료 증거는 없다. | identity split/merge proposal과 명시적 Go mutation UI는 후속 목표다. 모든 요약이 아니라 위험 semantic merge만 review한다. |

아래 Obsidian식 관계 graph·시각화 표면의 소유 버전은 **4.6**이다. 4.0 recall은
기존 source-backed item·edge에서 비권위 검색 후보를 만드는 retrieval
graph-neighborhood와 planner만 제공하고, 4.1은 rebuild·rollback gate를 닫는다.
어느 쪽도 관계 원장·projection·graph UI를 앞당겨 구현하지 않는다.

여기서 graph 관련 중복은 사용자가 원한 **Obsidian식 노드 시각화**와 연결된다. 후속 목표의 시각화 표면은 인물·장소·사건·개념과 관계 assertion을 source·시점·branch·knower·공개 범위와 함께 탐색하게 하고, graph-neighborhood는 현재 요청의 후보를 넓히는 retrieval 보조 경로로 쓸 수 있다. 그러나 UI의 선이나 연상 edge 자체가 정사 관계를 만드는 것은 아니다. MariaDB의 source-backed atomic memory는 canonical provenance·authority 원장이고, 객관 truth 여부는 각 row의 authority class·epistemic mode·lifecycle로 판정한다. graph·Chroma는 삭제·수정 뒤 재구축 가능한 projection/index라는 경계를 유지한다.

따라서 이 사례들은 3.6의 source·time·branch·generation·revision·owner·knower·visibility·privacy·authority·fail-open 하한선을 대체하지 않는다. graph, agent, LLM 또는 embedding이 없어도 exact/lexical recall, accepted state, 핵심 K와 한 문장 전달은 독립적으로 작동해야 한다.

##### 보강 A. [공통] degraded는 비확장적이어야 한다

보조 모델, embedding, reranker, keyword 추출 또는 graph 확장이 실패했을 때 후보와 권한이 넓어지면 안 된다. OMNINODE처럼 관련도 하한 아래의 나머지 노드를 예산까지 다시 채우는 방식은 장애가 곧 garbage fill이 되는 반전이다.

- degraded 결과의 eligible·selected·delivered 집합은 정상 hard gate를 통과한 집합의 부분집합이어야 한다.
- 장애는 visibility, authority, privacy, branch 범위, K, hard budget 또는 candidate breadth를 늘릴 수 없다.
- exact/lexical 결과가 부족하면 적게 전달하고 `coverage_gap`을 남긴다. raw chat, 전체 노드, parent만 맞은 child 또는 stale cache로 채우지 않는다.
- model-free 경로는 보조 기능과 독립적으로 완성되며, 보조 기능 장애가 model-free 결과를 삭제하지도 않는다.

##### 보강 B. [4.0/4.1] 선택·재사용·응답 유사도는 비권위 feedback이다

OMNINODE는 주입된 항목의 activation을 올리고 다음 응답과 embedding이 비슷하면 utility를 높인 뒤, 장기간 높은 항목을 lore로 승격하거나 낮은 항목을 삭제한다. WygLoreLeaf의 Hebbian co-fire 강화도 같은 자기강화 위험을 가진다.

- `selected`, `delivered`, `reused`, `co_fired`, `response_similar`, 높은 activation 또는 조회 빈도는 truth·canon·importance·relationship·closure·삭제 증거가 아니다.
- 이 값은 필요하면 request-scoped ranking telemetry로만 쓰며 source evidence와 분리한다.
- ranking feedback은 canonical row, source authority, lifecycle, owner/knower, relationship state 또는 summary 내용을 직접 바꾸지 않는다.
- 같은 기억을 모델에 넣어 모델이 비슷한 내용을 출력했다는 사실을 독립적인 재확인 근거로 세지 않는다.

##### 보강 C. [4.1] 파생 쓰기의 원자성과 완전한 rollback domain

OMNINODE는 permissive JSON 복구 뒤 node create/update/delete/merge와 relationship operation을 항목별로 적용해 중간 실패 시 일부 변경이 남을 수 있다. base+diff rollback도 저장한 전체 상태 중 일부 metadata와 cache를 복원하지 않는다. 이 실패를 구현 방식으로 가져오지 않고, 무엇을 증명해야 하는지에 대한 반례로 사용한다.

- LLM·Critic·Supervisor 결과는 먼저 엄격한 versioned schema와 source/lifecycle/authority/ref 제약을 같은 의존성 mutation batch 전체에서 검증한다.
- 같은 불변식에 묶인 canonical mutation batch는 모두 유효할 때만 commit하고, 일부만 유효하면 그 batch의 canonical change는 0이다. 서로 독립적인 memory·state lane은 별도 검증 batch로 처리해 한 잘못된 선택 항목이 모든 유효한 기억을 버리게 하지 않는다.
- accepted raw/source 저장, deferred 재처리 기록과 독립 exact idempotent write는 잘못된 derived operation 때문에 유실되거나 rollback되지 않는다. raw-first·fail-open spine은 파생 graph mutation보다 먼저 보존된다.
- permissive repair 결과나 모델의 operation 순서를 저장 권한 또는 transaction 경계로 사용하지 않는다.
- 외부 vector index는 canonical transaction과 같은 물리 transaction일 필요는 없지만 durable outbox와 current-row fence로 stale 항목이 보이지 않아야 한다.
- rollback manifest는 raw/source, atomic memory, current/history, subjective memory, relation, hierarchy summary, vector/index, cache, guidance carry, proposal과 파생 search trace를 열거한다.
- rollback 뒤 canonical fingerprint와 파생 set/hash가 이전 checkpoint와 같거나, 동일 source에서 결정적으로 재구성됐음을 증명한다.
- 본문 응답 fail-open과 저장 write-open을 혼동하지 않는다. 원장 read/parse 무결성 실패 시 응답은 base 경로로 계속할 수 있지만 canonical write는 read-only degraded로 막고 손상본·last-known-good·복구 상태를 보존한다.

현재 `turn_extraction_persist.go`의 similarity 기반 memory dedup은 새 요약 row를 생략하면서 기존 memory importance를 올릴 수 있다. rollback이 source turn 이후 row를 지워도 이전 row에 가해진 중요도 변경까지 되돌리는지 별도 mutation lineage가 필요하다. 이는 코드가 잘못됐다고 이 문서만으로 확정하는 것이 아니라, semantic merge·importance update가 **원 source turn과 함께 되돌아가는지 반드시 증명해야 하는 구체적 점검 대상**이다.

##### 보강 D. [4.1 선택 기능] semantic merge·정리의 검토와 되돌림

WygLoreLeaf는 merge log와 제한된 undo를 두고, OMNINODE는 유사 node pair를 사용자에게 보여 준 뒤 선택하게 한다. 유용한 부분은 UI 모양이 아니라 파괴적 정리 전에 identity와 lineage를 확인하고 결과를 되돌릴 수 있게 하는 원칙이다.

- exact duplicate와 동일 source revision의 idempotent replay는 자동 no-op으로 처리할 수 있다.
- 다른 turn·entity·time scope·knower를 가로지르는 semantic merge는 similarity만으로 실행하지 않는다.
- merge proposal은 keep/remove 대상, source refs, 달라지는 요약·관계·visibility, 영향 받는 projection/index와 undo 가능 범위를 보여 준다.
- 승인된 merge도 원 source를 재작성하지 않고 mapping/revision과 rebuild lineage를 남긴다.
- dependent merge 때문에 단순 undo가 불가능하면 부분 복구하지 않고 영향 graph와 필요한 선행 undo를 보고한다.
- 자동 prune은 단순 저조회·오래됨·낮은 activation이 아니라 resolved/superseded/source-invalidated 근거와 보존 가능한 감사 이력을 요구한다.

모든 요약을 수동 확인하라는 뜻은 아니다. calibration 없는 cross-source semantic merge, identity merge, private/public 경계 변경처럼 복구 비용이 큰 mutation만 명시적 review 대상으로 삼는다.

##### 보강 E. [3.8/3.9/4.1] truth status, assertion kind, knowledge path, epistemic state와 상태 commit barrier

Living Canon의 가장 유용한 표현은 “명제가 참인가”, “어떤 형식의 발화·주장인가”, “누가 어떤 경로로 접했는가”, “그 인물이 지금 어떤 인식 상태인가”를 섞지 않은 점이다. 3.6 조사 기준선의 Go에는 일부 `truth_scope`, `epistemic_mode`와 owner/knower/visibility가 있었지만 안정된 최종 taxonomy와 Supervisor·delivery·HUD 연결이 완료됐다는 증거는 아니다. 새 DB를 복제하지 않고 versioned mapping과 다음 수용 불변식을 먼저 정의한다.

- `truth_status`: `confirmed_true`, `confirmed_false`, `disputed`, `unknown`처럼 명제의 객관적 확정 상태
- `assertion_kind`: narrator assertion, character utterance, question, proposal, hypothesis, quoted report처럼 source 표현의 형식과 speech act
- `knowledge_path`: direct perception, received communication, remembered event, public record처럼 인물별 획득 경로
- `epistemic_state`: known, believed, suspected, heard, unknown, misinformed처럼 현재 인식 수준
- deception·거짓말·신뢰도는 위 축에 끼워 넣지 않고 의도와 반증 근거가 있을 때만 별도 claim qualifier로 기록한다.
- 기억 검색에 성공했다는 사실은 어떤 인물이 그 사실을 안다는 뜻이 아니다.
- 맞아떨어진 추측도 accepted confirmation 전에는 지식이나 objective truth로 승격하지 않는다.

값 변경은 수용 시험에서 다음 결과를 구분한다.

- `exact`: 새 값과 적용이 accepted evidence로 확인됨
- `directional`: 증가·감소·손상 같은 방향만 확인됐으며 이전 exact baseline을 임의 숫자로 덮지 않음
- `pending`: trigger·요청·시도는 있었지만 적용 결과가 확인되지 않아 current 값을 바꾸지 않음
- `proposed`: 계획·제안·예상일 뿐 trigger도 확정되지 않음

소유자, 실제 보유자, 착용자, 임시 위탁·수리·감정 중 보관자와 현재 사용 가능 상태도 하나의 `ownership` 문자열로 평탄화하지 않는다. 이 분류는 4.1 coverage를 다시 발명하는 필드 목록이 아니라 pre-response 예상 결과가 current state를 선점하지 못하게 하는 commit barrier다.

##### 보강 F. [4.1→4.8] offscreen present bridge와 no-replay

마지막으로 확인된 offscreen 활동과 현재 본문에 영향을 주는 활동을 분리한다.

- `last_confirmed_ref`는 과거에 실제로 묘사·보고된 상태일 뿐 현재 반복 중이라는 뜻이 아니다.
- 현재 입력·장면에 영향을 주는 message, report, arrival, deadline, obligation 또는 직접 묘사된 secondary scene 같은 `present_bridge_ref`가 있어야 current support로 노출한다.
- 현재 branch에서 accepted된 진행 중 효과, 유효 기간이 남은 current-state projection 또는 확정된 즉시 이동·도착·상호작용 의무도 source와 시간 범위가 맞으면 bridge가 될 수 있다. 단순 목적지 이름이나 과거 계획만으로는 부족하다.
- bridge ref도 source·branch·generation·revision·visibility·knower와 시간 유효성을 통과해야 하며 stale·wrong-branch·private·wrong-knower 근거는 current support가 아니다.
- 미해결, 오래된 상세 묘사, 같은 직업·루틴, 높은 중요도만으로 present bridge를 만들지 않는다.
- 새 근거 없이 “계속 조사한다”, “계속 기다린다”를 반복하면 progression이 아니라 replay이므로 전달하지 않는다.
- present bridge 부재는 request-scoped 비노출 또는 `deferred`일 뿐 원 기억의 lifecycle을 바꾸지 않는다. `dormant` 전이는 별도 Go source-linked transition rule이 있을 때만 허용한다.
- private simulation은 canonical memory나 다음 요청의 last-known plan이 아니다. 4.8에서
  필요해도 bounded hypothesis로 격리하고 accepted depiction과 source path가 있을 때만
  일치 부분을 새로 admission한다.

##### 보강 G. [4.0/4.1] locator형 회상 색인과 검색 가능한 원자 단위

Living Canon의 recall index는 대화 사본이 아니라 “무엇을 어디서 찾았는가”의 pointer로 쓰인다. Archive Center에서는 먼저 기존 query trace와 atomic source pointer로 같은 효과를 낸다.

- 한 retrieval unit은 한 subject·claim·question에 답할 수 있어야 한다.
- locator는 session/scope digest, source revision, branch, query digest/intent, owner/knower/visibility-policy revision, eligibility contract, searchable lane coverage completeness, provider readiness, expiry, verified source refs와 found/not-found를 가질 수 있지만 canonical fact를 복제하지 않는다.
- negative search trace는 “사실이 없다”는 truth가 아니라 같은 revision에서 어떤 경로를 이미 찾았는지에 대한 비용 절감 정보다.
- session/scope digest, source revision, branch, query digest/intent, owner/knower/visibility-policy revision, eligibility contract, searchable lane coverage completeness, provider readiness 또는 expiry가 바뀌면 재사용하지 않는다.
- private-scope 또는 incomplete-lane `not_found`를 public-scope·다른 POV·complete-lane의 검색 억제 근거로 공유하지 않는다.
- 반복 검색 비용과 hit 품질 개선이 측정되기 전에는 새 테이블·상시 agent memory·전체 대화 색인을 추가하지 않는다.
- no-op, split, merge, dormant, prune은 정상적인 maintenance 결과이며 각각 source ref와 reason을 남긴다.

##### 보강 H. [3.7/4.1] 외부 호출 공개표와 장기 작업 breadcrumb

WygLoreLeaf는 역할별 외부 전송 목적·데이터·보존·제3자 여부를 사용자에게 설명하고, 장기 작업의 boot/run/cold-start breadcrumb와 단계별 지연을 남긴다. Archive Center에는 privacy hard gate가 이미 있지만 **provider × role별 실제 전송 공개표**는 더 명시적으로 보강할 가치가 있다.

외부 Critic, Supervisor, embedding, reranker, query expansion을 사용할 때 다음을 versioned manifest 또는 기존 HUD/trace projection으로 설명한다.

- 역할, 목적, provider/model과 credential owner
- 전송 가능한 data class와 명시적으로 제외되는 data class
- masking·truncation·source-ref-only 여부
- provider 측 보존·제3자 전송에 대해 Archive Center가 아는 범위와 모르는 범위
- request별 실제 실행·skip·degraded 상태와 전송량

raw private text를 manifest나 breadcrumb에 다시 기록하지 않는다. durable breadcrumb는 operation ID, source revision, 마지막 완료 단계, retryability, degradation reason과 recovery owner만 남긴다. provider 포화·timeout·restart 뒤에도 이전 payload나 stale 결과를 재사용하는 권한이 생기지 않는다.

##### [공통] 가져오지 않을 패턴

- JavaScript가 memory 원장, graph, ranking, prompt, budget, rollback과 provider key를 소유
- message count·chat ID·content hash만으로 reroll·branch·revision을 추정
- canonical/reference 문구가 accepted session narrative를 무조건 `OVERRIDE`
- `knownBy`를 privacy hard filter가 아니라 주입문 주석으로만 표시
- co-fire·주입 횟수·응답 유사도·오래된 시간으로 canon 승격 또는 자동 삭제
- 관련도 하한 아래 전체 node, raw chat, 선택되지 않은 parent의 child로 예산을 채움
- malformed LLM JSON을 추측 복구한 뒤 create/update/delete/merge를 부분 적용
- 저장 read 실패를 빈 원장으로 바꾸고 정상 save로 덮어씀
- timeout 뒤 stale graph·plan·cache를 현재 기억으로 계속 주입
- 한 번의 reroll이나 거절 출력에서 전역 사용자 선호를 확정
- web 결과 또는 private simulation을 story canon과 같은 memory mutation 경로에 연결
- 출처·만료·우선순위 없는 자유문장 agent note를 user tail에 넣고 “반드시 반영”을 강제
- 모든 agent에 전체 history·setting을 반복 전달하고 고정 agent 수를 품질 등급으로 사용
- 고정 개수 cap을 coverage·defer·gap reason 없이 적용

종합하면 WygLoreLeaf의 연상 그래프, OMNINODE의 graph·snapshot, Living Canon의 역할 분리는 좋은 아이디어를 보여 주지만 그 자체를 이식하는 것은 3.6의 장점을 약화시킨다. 가져올 것은 **비확장 실패, 비권위 feedback, 원자적 파생 쓰기와 완전 rollback, 위험 mutation의 review·undo, truth·assertion·knowledge·epistemic 축 분리, present bridge, locator형 색인, 외부 전송 관찰성**이다.

### 8.7 4.6~5.0이 1.0을 실제로 넘어서는 조건

4.6~5.0의 인물·관계·세계·thread 모델은 1.0보다 정밀하지만, 현재 문서의 필드만 존재해서는
안전한 장면 연속성과 가역적 focus가 작동하지 않는다. 다음 lifecycle이 함께 있어야 한다.

1. 각 상태의 생성 owner와 갱신 owner는 Go다.
2. 상태 변경은 accepted observation과 versioned transition rule에서만 발생한다.
3. 모든 상태는 source refs, branch, generation, revision, created/confirmed turn을 가진다.
4. `carry`, `hold`, `expire`, `resolve`, `invalidate`의 조건과 reason을 분리한다.
5. reroll·delete·branch 이동은 기존 proposal뿐 아니라 그 proposal에서 파생된 carry state도 무효화한다.
6. Supervisor 실패·침묵·검색 miss를 expire 또는 resolve로 해석하지 않는다.
7. Director가 선택한 관계·세계·thread 항목이 본문 모델의 기존 레인에 없으면 가이드에 쓰인 범위만 `guidance_support`로 승격한다.
8. complete-turn은 본문에 등장했다는 이유만으로 새 상태를 승인하지 않고 actor·causal·consent·closure 근거를 다시 검사한다.
9. foreground는 current user turn에 귀속된 만료 가능한 focus lease이며
   `explicit_redirection`에서 같은 요청에 해제한다.
10. 한 thread의 resolve를 다른 thread·arc·작품 전체 closure로 전파하지 않는다.

이 lifecycle이 구현되면 4.6~5.0은 1.0의 장면 연결·기억 행동화·근거리 Director 체감을
복원하면서도, 1.0보다 훨씬 강한 사용자 전환 자유와 출처·시점·관계·비밀·branch 안전성을
가질 수 있다. 반대로 만료·redirection 없이 필드와 prompt만 추가하면 “구조는 풍부하지만
한 줄거리에서 놓아주지 않는 감독”이 되어 1.0의 결함까지 되살린다.

## 9. 3.9 Basic Publisher와 4.9 고급 출판사 — 권장 versioned guidance 계약

새 API·테이블·JS 정책을 먼저 만들지 않고 기존 `/prepare-turn` → Go orchestration →
Supervisor endpoint → Go renderer 경로를 확장한다. `directed`는 Director stage 하나만,
`reviewed`와 `deep`은 같은 versioned envelope 안에서 제한된 추가 role을 실행한다. 역할별
별도 병렬 API나 JavaScript orchestration을 만들지 않는다.

아래 입력·출력 예시는 다층 체인 중 **Director stage**의 계약이다. Recall Auditor는 이
입력에 이미 고정된 eligible set 안에서 누락 표시·부분집합·우선 ref만 반환할 수 있고,
Reviewer는 아래 출력의 필드를 승인·축소·거절할 수 있을 뿐 새 지시나 ref를 추가할 수
없다. Supervisor 입력은 “검색 후보 전체”가 아니라 Go 검증을 통과한 제한 표면이어야 한다.

```json
{
  "execution_fence": {
    "contract_hash": "...",
    "chat_session_id": "...",
    "turn_index": 0,
    "request_generation": 0,
    "branch_id": "...",
    "expected_revision": 0
  },
  "current_input": {
    "source_ref": "...",
    "raw_text": "...",
    "role": "current_command",
    "turn_index": 0,
    "branch_id": "...",
    "request_generation": 0,
    "expected_revision": 0,
    "visibility_boundary": "current_request",
    "privacy_class": "request_visible",
    "authority": "current_user_intent"
  },
  "accepted_recent_context": [
    {
      "source_ref": "...",
      "compact_text": "...",
      "role": "continuity",
      "turn_index": 0,
      "branch_id": "...",
      "request_generation": 0,
      "expected_revision": 0,
      "visibility_boundary": "session_visible",
      "privacy_class": "approved_compact",
      "authority": "continuity_only"
    }
  ],
  "continuity_query_plan": {
    "mode": "none",
    "source_turn_refs": [],
    "query_seed_refs": []
  },
  "core_objective_memory": [
    {
      "source_ref": "...",
      "compact_text": "...",
      "role": "core_objective_memory",
      "turn_index": 0,
      "branch_id": "...",
      "request_generation": 0,
      "expected_revision": 0,
      "visibility_boundary": "session_visible",
      "privacy_class": "approved_compact",
      "authority": "source_backed_memory"
    }
  ],
  "eligible_guidance_support": [
    {
      "source_ref": "...",
      "compact_text": "...",
      "support_role": "thread|relationship|world|character|hierarchy",
      "turn_index": 0,
      "branch_id": "...",
      "request_generation": 0,
      "expected_revision": 0,
      "visibility_boundary": "...",
      "privacy_class": "approved_compact",
      "authority": "guidance_support_only"
    }
  ],
  "hierarchy_zoom": {
    "selected_level": "episode",
    "selected_items": [],
    "suppressed_levels": []
  },
  "thread_focus_candidates": [],
  "relationship_execution_items": [],
  "world_execution_items": [],
  "character_execution_items": [],
  "guidance_carry_state": {
    "arc_anchor_ref": "",
    "foreground_thread_ref": "",
    "focus_lease": {
      "lease_ref": "...",
      "scope": "response_only",
      "source_user_turn_ref": "...",
      "expires_turn": 0,
      "renewal_required": true,
      "release_on_explicit_redirection": true
    },
    "carry_refs": [],
    "hold_refs": [],
    "blocked_route_refs": []
  }
}
```

입력 계약의 필수 조건:

- 모든 ref-bearing item은 최소한 `source_ref`, 역할, lifecycle 좌표, visibility,
  privacy와 authority를 가진다. current input은 원본 `raw_text`를 보존하고 그 외 지원
  item은 Supervisor가 읽을 승인된 `compact_text`를 가진다. ref만 주고 Supervisor가 별도
  저장소를 임의 조회하게 하지 않는다.
- `eligible_guidance_support`에는 source·branch·revision·visibility·privacy 검사를 통과한 bounded item만 들어간다.
- 검색에서 탈락했거나 본문에 전달할 수 없는 raw 후보는 Supervisor에게도 주지 않는다.
- current input 이외의 과거 user text에는 command authority가 없다.
- carry state는 저장된 Supervisor 자유문장이 아니라 Go가 accepted state/thread에서 재구성한 typed ref 묶음이다.
- focus lease는 current user turn ref, 만료 turn과 scope를 가지며 비반대·침묵·과거
  storyline만으로 갱신되지 않는다. `explicit_redirection`이면 Director 호출 전에
  foreground·arc·frontier 후보에서 제외한다.
- Supervisor가 지원 항목을 실제 선택하면 본문 모델의 기존 전달 ref를 재사용하거나, 본문 예산 안에서 `guidance_support`로 승격할 수 있어야 한다.

Supervisor 출력 예시:

```json
{
  "contract_version": "supervisor_scene_proposal.v4",
  "truth_authority": false,
  "canonical_write": false,
  "response_shaping_authority": "bounded_scene_direction",
  "execution_fence": {
    "contract_hash": "...",
    "chat_session_id": "...",
    "turn_index": 0,
    "request_generation": 0,
    "branch_id": "...",
    "expected_revision": 0
  },
  "fidelity_warnings": [],
  "expression_hints": [],
  "used_support_refs": [],
  "director_plan": {
    "response_focus_refs": [],
    "must_account_refs": [],
    "continuity_anchor_ref": "",
    "foreground_thread_ref": "",
    "focus_lease_ref": "",
    "preferred_frontier_ref": "",
    "relationship_execution_refs": [],
    "world_guard_refs": [],
    "character_execution_refs": [],
    "must_not_refs": [],
    "pacing_intent": "",
    "ending_edge": ""
  },
  "story_author_plan": {
    "arc_anchor_ref": "",
    "near_term_direction_ref": "",
    "candidate_frontier_refs": []
  },
  "carry_state_delta": {
    "carry_refs": [],
    "hold_refs": [],
    "release_focus_refs": [],
    "expire_refs": [],
    "invalidate_refs": []
  }
}
```

Supervisor가 자유문으로 새 전개를 만든 뒤 적당한 `source_ref`를 붙이게 해서는 안 된다. Go가 먼저 다음 typed item을 만들고 Supervisor는 항목을 선택·정렬·표현해야 한다.

- `response_obligation`
- `must_account`
- `may_advance`
- `hold_allowed`
- `must_not_assert`
- `fidelity`
- `expression`

모든 proposal은 사용한 execution contract의 lifecycle 좌표와 hash를 되돌려 보내야 한다. `contract_hash`는 위 좌표를 포함한 canonical contract 직렬화에서 계산하고, Go는 현재 accepted fence와 좌표·hash가 모두 정확히 일치할 때만 적용한다. branch, generation, revision 중 하나라도 다르면 stale 결과를 적용하지 않는다.

`used_support_refs`는 최종 수락된 지시가 실제로 의존한 항목만 담는다. Go renderer는 각 ref의 compact 표현을 본문 모델에 함께 전달할 수 있는지 다시 검사한다. 이미 핵심 기억·직접 근거·reference 레인에 전달된 항목은 기존 ref를 가리키고 텍스트를 중복하지 않는다. support-only 항목만 `guidance_support`로 승격한다. 하나라도 전달할 수 없거나 현재 fence와 어긋나면 해당 ref에 의존하는 Director 필드만 폐기하며, 다른 독립 필드와 본문 요청은 fail-open한다.

`foreground_thread_ref`, `preferred_frontier_ref`, `arc_anchor_ref`는 유효한
`focus_lease_ref` 없이 적용하지 않는다. `explicit_redirection`이면 Go는
`release_focus_refs`를 먼저 결정하고 이전 focus에 의존한 Director·Story Author 필드를
폐기한다. `ending_edge`는 응답의 표현상 종단일 뿐 thread·arc·작품의 해결 또는 엔딩
권한이 아니다.

`carry_state_delta.expire_refs`와 `invalidate_refs`는 Supervisor의 단독 결정을 뜻하지 않는다. Go가 versioned transition rule과 complete-turn evidence를 확인하기 위한 요청값이며, 조건을 만족하지 않으면 거부한다. `resolve`는 proposal 출력에 두지 않고 Go의 별도 thread lifecycle만 소유한다.

3.9의 현재 Publisher 계약과 4.9의 `scene_guidance_proposal.v1` successor를 별도 병렬
계약으로 만들기보다 하나의 versioned 단일-owner 계약으로 정리해야 한다.

## 10. 버전별 구현 packet

이 장은 기능을 발견한 순서가 아니라 현재 3.7 이후의 실제 소유 버전으로 나눈다. 각
packet은 앞 버전의 release gate를 상속한다. 3.9 Basic Publisher는 관계·profile 전달과
같은 버전에서 bounded 응답 지원을 만들지만 canonical memory 권한을 얻지 않는다.
4.6~5.0 RP 지원은 3.8~4.1 기억과 3.9 Publisher 계약을 읽는 후속 소비자이며 앞 버전의
메인 범위를 대신하지 않는다.

| packet | 이 문서에서 실행할 핵심 | 다음 단계에 제공하는 것 |
|---|---|---|
| 3.7 | A~F source 구현을 기준선으로 고정하고 G에서 loaded 설정·HUD·실제 provider 품질 검증 | 믿을 수 있는 source/trace 기준선 |
| 3.8 | 시간·상태·주관 지식·owner/knower 경계 | story-time 좌표와 관점별 support |
| 3.9 | 방향성 관계·습관·stable/current/dynamic profile과 Basic Publisher | source-linked 캐릭터 기억과 1회 bounded response plan |
| 4.0 | 3.7-F K를 소비하는 복합 회상·final injection·delivery lineage, 승인 시 로어북 reference lane | 본문과 Director가 함께 볼 검증 ref |
| 4.1 | edit/delete/reroll/branch/reindex/restart와 live parity | stale 없는 carry·thread·projection 기반 |
| 4.6 | 기반 계약과 관계·개체 graph·UI | source-linked 실행 기반과 탐색 표면 |
| 4.7 | 인물·관계 실행 카드 | 인물별 지식·말투·관계 표현·agency 경계 |
| 4.8 | 세계·세력·thread simulation | 비구속 진행 후보·hold와 다중 thread 자유도 |
| 4.9 | 모델 능력 적응과 고급 출판사 | Recall Auditor→Director→Reviewer 검증 plan |
| 5.0 | output fidelity 사후 진단과 실사용 release closure | 진단 결과와 재현 가능한 실사용 증거 |

### 3.7 packet — A~F source 기준선 고정과 G 실환경 검증

3.7-F source·자동 회귀에서 다음은 구현 완료로 기록돼 있다.

1. `compactPrepareTurnLine`의 rune·가능한 단어 경계 cap
2. backend `topK`의 Vector 후보 상한 고정과 별도
   `core_objective_memory_delivery.v1` K 계약
3. compact trace의 memory count·Supervisor typed status와 Go ViewModel 연결
4. final item disposition·defer/drop reason의 HUD projection
5. hierarchy·Persona·주관 기억의 current-request relevance와 owner hard cap 제거
6. 영어 키워드 기반 auto 추론 제거와 mode 설정 저장·전달

3.7-G에서 닫을 것은 같은 기능을 다시 구현하는 일이 아니라 loaded `Archive Center.js`와
backend를 연결한 설정 저장·reload, 한글·K 미달/초과, private 비노출,
weak/medium/strong, malformed 실제 provider 응답과 HUD 표시다. source 증거만으로
실환경 품질을 완료 처리하지 않는다.

### 3.8 packet — 시간·상태·주관 지식 기반

1. 객관 사건과 주관 기억을 별도 lane으로 유지
2. `owner_entity_id`, `about_entity_ids`, `knower_scope`, `visibility_boundary`와 source ref 연결
3. story clock, 시간 불확실성, validity interval과 source-backed time coordinate
4. current exact, last confirmed, pending/proposed와 superseded state 분리
5. correction·reveal·recovery에서 source revision 기반 재구축
6. 3.7-F의 owner coverage·1~N 선택·HUD lineage를 그대로 상속하고 time·knower 의미만
   추가

3.8은 Director를 구현하지 않는다. 3.9 Publisher, 4.0 retrieval과 4.6~5.0 execution
surface가 읽을 수 있는 정확한 time·state·perspective 기반만 제공한다.

### 3.9 packet — 관계·캐릭터 기억과 Basic Publisher

실행 정본은
[`3.9-character-memory-basic-publisher-roadmap.md`](3.9-character-memory-basic-publisher-roadmap.md)다.

1. relation subject/object, domain, direction, reciprocity, consent와 current/history/transition 분리
2. 단일 행동을 습관·성격·말투로 자동 승격하지 않는 반복·counterevidence 계약
3. stable/current/dynamic profile과 trait-domain·voice evidence 분리
4. relation/profile/voice item을 exact entity·counterpart·knower·branch·revision 경계로 전달
5. 기존 Go Publisher/Supervisor owner를 한 번 확장해 단 하나의 typed guidance block 생성
6. `none`에서 base memory/evidence/secret guard를 유지하고 Publisher call·plan·guidance는 0
7. weak/medium/strong에서 response 범위의 source-linked focus·must-account·expression·hold
   또는 사전 승인된 `may_advance`만 허용
8. entity remap·split/merge, correction, reroll·delete·branch 뒤
   relation/profile/Publisher input을 함께 재구축
9. Publisher 없음·skip·timeout·malformed이면 stale/default/partial plan 없이 base support만 유지
10. 실제 RisuAI에서 첫 출력 채택, redirection, 장기 session, 장애 격리와 비용 검증

3.9는 “1.0 같은” 최소 체감을 복구하지만 이는 story truth나 persistent plot direction
권한이 아니다. canonical relation·profile admission은 3.9의 source evidence와 transition
rule만 소유한다. 4.0의 전체 recall coverage, 4.7~4.8 simulation과 4.9 다단계 검토를
선행 구현하지 않는다.

### 4.0 핵심 작업 — 3.7-F K를 복합 recall·final injection에 통합

정확한 `4.0-A~G` packet 소유권은
[`4.0-precision-recall-injection-roadmap.md`](4.0-precision-recall-injection-roadmap.md)를
따른다. 아래 두 절은 packet ID 재정의가 아니라 이 문서의 1.0 강점 복구 관점 요약이다.

1. 기존 `core_objective_memory_max_items`와
   `core_objective_memory_delivery.v1`의 의미·UI·legacy 동작을 그대로 소비
2. `memory_recall_plan.v2`에서 exact·lexical·vector와 기존 source-backed item·edge의
   비권위 retrieval graph-neighborhood 후보를 만들고
   recent·continuity·historical horizon을 조합
3. objective event summary만 K에 포함하고 support lane은 K를 소비하지 않음
4. `memory_injection_plan.v2`에서 구조화 저장본과 승인된 한 문장 `delivery_text`를
   분리하고 metadata는 sidecar lineage로 유지
5. horizon coverage 뒤 최종 K를 적용하되 무관련 quota filler 금지
6. actor·canonical anchor·부정·시간·owner 경계의 summary coverage audit와 제한
   direct-evidence 복구
7. current·historical·transition·dispute intent에 맞춰 기존 lifecycle을 적용하고
   superseded를 현재 사실로 전달하지 않음
8. candidate/eligible/selected/rendered/delivered와 exact item ref·gap reason 표시
9. 보조 기능 장애 시 정상 hard gate 집합의 부분집합만 전달하는 non-expansive degraded 계약
10. selected·delivered·co-fired·response-similar를 canonical 승격·importance·삭제 근거로
    사용하지 않음

4.0은 K 설정과 기본 객관 기억 cap을 재발명하지 않는다. 3.7-F의 K가 새 복합 검색과
final injection을 지나도 같은 의미와 비퇴행 결과를 유지하는지가 핵심이다.

### 4.0 보강 작업 — 회상과 이전 맥락의 1.0 강점 복구

1. Host observation과 accepted final tail로 `continuity_query_plan` 구성
2. current input, scene, relationship, unresolved thread의 typed multi-query
3. weak/reentry/temporal/redirection에 따른 적응형 이전 anchor 선택
4. earliest/middle/latest/turn-range temporal anchor
5. 현재 질문에 맞는 단일 hierarchy zoom
6. 모든 query/anchor/zoom의 selected·dropped reason과 lineage 표시

이 packet은 Supervisor 권한을 늘리지 않고 기억 recall과 continuity만 개선할 수 있다. 먼저 `없음` 모드에서도 독립적으로 검증해 기억 품질 개선과 감독 효과를 분리한다.

### 4.1 packet — lifecycle·복구·실환경 gate

1. edit·delete·reroll·branch·copy·move 뒤 memory, projection, index, delivery와 carry 동시 무효화
2. stale worker·outbox·retry·restart가 superseded source와 plan을 부활시키지 않는 fence
3. source → payload → displayed final → admission 및 selected → rendered → delivered lineage 연결
4. MariaDB canonical row와 Chroma/graph/locator projection의 재구축·no-resurrection 검증
5. Supervisor·Reviewer·Critic의 늦은 결과와 이전 성공 plan 재사용 차단
6. 실제 RisuAI, loaded artifact, MariaDB, ChromaDB와 updater의 fault·upgrade·long-session 검증
7. role별 외부 호출, 비용·latency와 degraded/fail-open 결과를 evidence manifest에 기록
8. provider × role별 외부 전송 data class·제외 항목·실행/skip/degraded 공개표
9. raw private text 없는 durable stage breadcrumb와 recovery owner
10. 파생 mutation batch의 strict validation·all-or-zero commit과 전체 rollback manifest
11. 선택형 cross-source semantic merge가 별도 승인·활성화된 경우에만
    proposal·review·undo와 projection/index rebuild
12. similarity memory dedup·importance update가 source rollback과 함께 되돌아감을 증명하고,
    증명하지 못하면 해당 자동 mutation을 비활성화하거나 exact idempotent 경로로 제한

4.1은 새 감독 능력을 추가하는 버전이 아니다. 3.8~4.0 기억 기반, 3.9 Publisher plan과 4.6~5.0이 소비할
lifecycle 좌표가 실제 수정·삭제·분기·재시작에서도 거짓이 되지 않음을 닫는 release
gate다.

### 4.6 packet — 기반 계약·관계 graph·탐색 UI

실행 정본은
[`4.6-5.0-model-adaptive-rp-publishing-roadmap.md`](4.6-5.0-model-adaptive-rp-publishing-roadmap.md)다.

1. stable `entity_identity.v1`, 3.9 방향성 관계와 4.0 `memory_recall_plan.v2`만 소비
2. `simulation_snapshot.v1`과 `narrative_execution_brief.v1`의 source·budget·OFF 회귀 기반 정의
3. 4.0의 retrieval graph-neighborhood와 구분되는 source
   ref·current/history·transition·privacy·branch 보존 관계 graph projection·UI
4. 언급 후보, tentative node/link와 canonical node/relation assertion을 분리
5. graph edge와 검색 적중을 truth·importance·merge 권한으로 사용하지 않음
6. UI mutation은 Go admission·revision·audit API를 우회하지 않음
7. rebuild·rollback·stale quarantine와 loaded UI 증거를 release gate로 연결

4.6은 새 Supervisor나 두 번째 recall planner를 만들지 않는다. `none`의 base support와
Publisher call 0은 이미 3.9 계약이며, 4.6 simulation support가 활성화되더라도
기억·직접 근거·비밀 guard·주관 기억·현재 상태는 동일하게 유지된다.

### 4.7 packet — 인물·관계 실행 카드

1. `character_execution_card.v1` 구현과 3.9 `voice_behavior_projection.v1` read-only 소비
2. 관계별 stable/current/dynamic profile, 지식·오해·비밀·동기·감정 원인과 agency 분리
3. `truth_status`, `assertion_kind`, 인물별 `knowledge_path`, `epistemic_state` 분리
4. trait-domain 밖 감정·말투·도덕성의 무근거 파생과 예시 대사 복사 금지
5. relationship direction/domain/current/history와 evidence-bound change envelope 투영
6. user 우대·자동 reciprocity·cross-domain·영속성 승격 차단
7. 활성 인물 selected/deferred/excluded reason과 source-linked execution refs
8. 행동·subtext 표현에 실제 사용한 ref만 semantic dedupe 뒤 기존 본문 ref 또는
   `guidance_support`로 전달

### 4.8 packet — 세계·세력·thread simulation

1. `world_execution_slice.v1`, thread focus candidates와 hold/defer
2. 세계 규칙·세력·자원·시간·공간·인과, actor/faction의 독립 목표·수단·대가
3. thread lifecycle과 `progression_change_envelope`, 모든 eligible thread의
   selected/related/deferred 이유
4. offscreen `last_confirmed_ref`와 현재 `present_bridge_ref`를 분리하고 bridge 없는 반복
   활동을 지시하지 않음
5. private offscreen simulation은 bounded hypothesis이며 accepted depiction 없이
   canonical write 금지
6. foreground를 response/scene 범위 focus lease로 제한하고 current user turn ref 없이는
   자동 갱신하지 않음
7. `explicit_redirection`에서 이전 foreground·arc·frontier와 의존 proposal을 같은 요청에 해제
8. 한 thread 해결과 작품·세션 전체 closure를 분리하고 `ending_edge`를 응답 표현으로만 사용
9. current input delta와 already-realized outcome을 분리해 replay·summary echo 차단

### 4.9 packet — 모델 적응형 고급 출판사

1. 3.9 Publisher의 endpoint·owner·strength·fail-open과 단일 guidance block 계약 상속
2. `model_capability_profile.v1`에 따라 표현의 압축·명시성·구조와 호출 깊이만 조절
3. weak: response focus와 must-account
4. medium: 사전 승인된 `may_advance` 하나 또는 hold
5. strong: 사용자가 계속 선택한 thread의 짧은 focus lease 안에서만 arc context,
   near-term option, foreground frontier와 ending edge
6. 모든 단계에서 truth/canonical/relationship/closure authority false
7. `reviewed`: Director plan을 필수 Reviewer가 승인·축소·거절
8. `deep`: 고정 eligible set에서만 Recall Auditor → Director → 필수 Reviewer 실행
9. Reviewer와 Recall Auditor는 새 지시·새 ref·검색 범위·visibility·예산·권한을 추가하지 않음
10. 같은 모델 재사용은 `same_model_review`로 표시하고 독립 검토 성공으로 주장하지 않음
11. `reviewed`/`deep`의 Reviewer 미실행·실패·timeout·malformed·독립성 실패는
    `base_only`·`proposal_0`이며 Director 결과를 낮은 depth로 소급 적용하지 않음
12. Reviewer는 `explicit_redirection`과 focus lease 만료를 확인하고 이전 storyline으로
    되돌리는 plan을 거절
13. 새 테이블·병렬 Supervisor API·두 번째 prompt block을 만들기 전에 기존 owner와
    versioned successor로 해결

### 5.0 packet — 출력 충실도·실사용 release closure

1. 3.5의 Host source→constraint→payload→final evidence와 HUD를 소비하고 같은
   preprocessor·fidelity owner·진단 UI를 병렬 재구현하지 않음
2. `output_fidelity_report.v1`을 우선 diagnostic-only로 구현
3. source→contract→payload→displayed final→admission과 Publisher plan의 E2E correlation
4. 인물 혼동·지식 누출·관계 급변·replay·stagnation·thread tunnel vision·문장 복사·
   세계 규칙 위반 검출
5. 사후 결과가 memory·relation·thread를 직접 수정하지 못하도록 admission 재검증
6. 공식 Host 보호 span과 draft 교체가 증명되기 전 자동 repair 활성화 금지
7. 이름 미확정 별도 플러그인은 5.0 필수 의존성이 아니지만, 연동 전처리를 소유한 요청의
   Archive Center Publisher는 자동 OFF하고 미사용 요청의 Publisher는 그대로 유지
8. 서로 다른 capability profile의 OFF/ON 맹검, 첫 출력 채택·리롤·latency·cost·품질 측정
9. OS·package·upgrade·fault·장기 session과 실제 RisuAI release evidence closure
10. proactive stance는 별도 명시적 opt-in과 source-backed envelope가 있는 경우만 평가하며
   기본 strong이나 5.0 완료에 자동 포함하지 않음

파생 mutation·rollback·semantic merge·locator/rebuild는 각각 4.0 recall과 4.1 lifecycle의
소유다. 4.6~5.0은 검증된 결과를 읽을 뿐 해당 원장이나 복구 경로를 구현하지 않는다.

## 11. 버전 미배정 후속 A — 독립형 원작 DB와 Archive Center 연결 강화

이 항목은 특정 Archive Center 버전에 배정하지 않는다. 현재 기억 전달·서사 가이드 복구 작업의 출시 범위에도 포함하지 않고, 독립 제품과 연결 계약을 설계할 때 다시 착수한다.

### 목표와 소유권

원작 DB는 Archive Center 안의 기억 종류가 아니라, 혼자서도 사용할 수 있는 **원작 근거 제공 시스템**으로 분리한다. 연결 상태에서는 두 저장소를 합치지 않고 현재 장면 문맥을 교환해 검색과 활용만 강화한다.

| 구성요소 | 소유하는 것 | 소유하지 않는 것 |
|---|---|---|
| 원작 DB | 작품·판본·연속성, 원문 출처, 인물·별칭·연표, 검토 상태, 스포일러 시점, 원작 근거 검색 | 세션 기억, 사용자 서사, 최종 프롬프트 조립, 장면 지휘 |
| Archive Center | 객관·주관·계층 기억, 현재 branch·turn 상태, 최종 관련도·예산·주입, 서사 가이드와 Director | 원작 문서의 정설 판정과 원문 저장 |

연결 상태의 흐름:

```text
Archive Center
  → 현재 장면·등장인물·진행 시점·부족한 원작 정보의 최소 문맥 전달
원작 DB
  → 출처와 시점이 붙은 짧은 원작 근거 묶음 반환
Archive Center
  → 세션 기억과 원작 근거를 별도 레인으로 검증·선별
  → 직접 주입 항목과 감독용 제한 보조 묶음을 구분
  → 최종 기억 주입과 서사 가이드 구성
```

원작 DB는 “원작에서 무엇이 사실인가”를 제공한다. “이번 응답을 어떻게 진행할 것인가”는 계속 Archive Center의 Director가 결정한다.

### 독립 실행과 연결 실행

| 상태 | 동작 |
|---|---|
| 독립 실행 | 작품·판본·인물·연표 검색, 시점·스포일러 제한, 출처가 붙은 요약과 근거 조회 |
| 연결 꺼짐 | Archive Center는 원작 DB 없이 현재 기억 기능을 동일하게 수행 |
| 연결·보충 | 현재 기억에서 빠진 원작 사실만 별도 예산으로 보충 |
| 연결·원작 중심 | 사용자가 명시적으로 선택한 원작 준수 세션에서만 더 넓은 원작 근거 예산을 사용하되 핵심 기억 K는 침범하지 않음 |

연결 실패·지연·계약 불일치가 발생해도 Archive Center의 일반 기억 회수와 본문 요청은 중단하지 않는다. 이전 연결 결과를 현재 turn·branch의 근거처럼 재사용하지도 않는다.

독립 실행에서는 원작 DB가 자체 화면이나 출력 경로를 가질 수 있다. 그러나 Archive Center와 연결된 동안에는 원작 DB의 직접 프롬프트 주입을 끄고, 출처가 붙은 읽기 전용 항목만 반환한다. 최종 순서·예산·중복 제거·프롬프트 조립의 단일 소유자는 Archive Center의 Go backend다.

### 핵심 기억 K 및 서사 가이드와의 관계

- `core_objective_memory_max_items`는 Archive Center가 최종 전달하는 세션의 객관 사건 기억만 센다.
- 원작 근거는 별도 후보 검색 상한과 별도 주입 문자 예산을 사용하며 핵심 기억 K를 소비하지 않는다.
- 원작 DB 내부 검색 후보 수는 사용자-facing 핵심 기억 K와 연결하지 않는다.
- 주관 기억, 계층 기억, 직접 근거, 비밀 guard도 기존처럼 각각 별도 레인에 둔다.
- Host 전체 예산이 부족하면 원작 근거를 먼저 보류하고 reference gap reason을 표시한다. 원작 근거 때문에 핵심 기억 K를 줄이지 않는다.
- 각 레인의 별도 예산을 합친 결과도 RisuAI Host의 전체 context 상한을 넘지 않아야 한다.

서사 가이드 강도는 원작 DB의 검색 권한이 아니라 Archive Center가 반환 근거를 **어떻게 사용하는가**를 바꾼다.

| 서사 가이드 | 원작 근거 활용 |
|---|---|
| 없음 | 사실·시점·스포일러·모순 방지에만 사용하며 장면 목표나 다음 전개를 만들지 않음 |
| 약함 | 현재 반응과 장면 초점을 보조하는 source-backed cue로 사용 |
| 중간 | 기존 causal frontier와 인물 동기를 고르는 Director 자료로 사용 |
| 강함 | 현재 사용자가 계속 선택한 thread의 짧은 focus lease 안에서 arc context와 foreground option을 고르는 자료로만 사용. 새 정설·관계 변화·종결 권한과 storyline 자동 갱신 없음 |

Director는 본문에 직접 들어가는 짧은 핵심 기억 K만 보는 것으로 제한하지 않는다. 1.0의 장점을 회수하려면 다음 두 표면을 구분한다.

- 직접 주입 표면: 최종 핵심 기억 K와 최종 reference 항목. 본문 모델이 그대로 받는 짧은 묶음
- 감독 지원 표면: 관련도·출처·분기·시점·공개 범위·비밀 검사를 통과한 제한된 보조 항목. 직접 주입보다 넓을 수 있음

감독은 탈락·미검증 후보를 볼 수 없다. 감독이 보조 항목을 실제 지시의 근거로 선택했다면 Archive Center는 먼저 해당 `source_ref`가 본문 모델의 기존 레인에 있는지 확인한다. 이미 있으면 그 ref를 재사용하고 텍스트를 반복하지 않는다. 없으면 짧은 표현과 `source_ref`를 `guidance_support` 레인으로 승격한다. 그 근거를 예산 안에 전달할 수 없으면 해당 감독 지시도 폐기한다. 이렇게 해야 본문에는 짧은 기억만 유지하면서도 감독은 1.0처럼 조금 더 넓은 기억 묶음을 읽을 수 있다.

### 오염 방지 경계

- 원작 근거를 세션의 객관 기억으로 자동 승격하지 않는다.
- 역할극에서 생긴 사건이나 모델 생성 요약을 검토 없이 원작 정설로 올리지 않는다.
- 원작 DB와 Archive Center가 같은 테이블·벡터 컬렉션·canonical row를 공동 소유하지 않는다.
- 캐시가 필요하면 원작 문서의 portable ID, 판본·연속성, source hash, 검토 상태를 보존하고 `reference_external`처럼 출처를 분리한다. 숫자 DB row ID나 내부 Chroma ID를 시스템 간 식별자로 쓰지 않는다.
- 세션의 raw 주관 기억·비밀·사용자 전용 정보는 원작 DB 조회에 보내지 않는다. 필요한 경우 Archive Center가 비공개 내용을 제거한 장면 요약만 전달한다.
- 원작 근거의 설명문이나 검색 결과를 Director 지시 자체로 취급하지 않는다.
- 현재 사용자 입력과 세션에서 실제로 발생한 branch·관계·상태가 원작보다 우선한다. 원작 근거는 현재 세션의 발생 사실을 덮어쓰지 않는다.

연결 요청에는 최소한 요청 ID, 현재 turn·branch·revision, 작품·연속성, 공개 상한, deadline, 후보·문자 상한을 포함한다. Archive Center는 늦게 도착했거나 현재 좌표와 일치하지 않는 응답, 출처 hash가 달라진 응답, 권한 표지가 없는 응답을 폐기한다.

### 1.0 장점 회수 여부 재검토

판정은 **조건부로 올바른 방향**이다. 독립화 자체가 1.0의 장점을 복원하는 것은 아니지만 다음 조건을 함께 지키면 1.0의 장점을 보존하면서 더 정확하게 확장할 수 있다.

| 1.0의 장점 | 이 설계의 회수 방법 | 실패 조건 |
|---|---|---|
| 짧고 관련성 높은 기억이 데이터베이스 결과처럼 보임 | 세션 핵심 기억은 한 문장 K로 유지하고 원작 근거도 출처가 붙은 짧은 항목으로 반환 | 원문 덩어리나 장문의 원작 요약이 핵심 기억을 밀어냄 |
| 이전 기억과 보조 자료가 핵심 기억을 방해하지 않음 | 세션 기억, 원작 근거, 주관 기억, 계층 기억을 별도 레인·예산으로 유지 | 원작 근거가 핵심 기억 K 또는 같은 문자 예산을 소비 |
| 감독이 더 넓은 기억 묶음을 읽고 장면 목표로 번역 | Director가 최종 기억과 검증된 제한 보조 묶음을 함께 읽고, 실제 사용한 보조 근거는 기존 본문 ref를 재사용하거나 `guidance_support`로 승격 | 원작 DB가 직접 장면 지시를 만들거나 Director가 탈락·미검증 후보를 보고 지휘함 |
| 설정·인물 일관성이 높음 | 판본·연속성·시점·출처가 명확한 근거로 모순과 스포일러를 차단 | 출처 없는 모델 요약을 원작 정설로 승격 |
| 원작 자료가 없어도 기본 기억 시스템이 작동 | 연결을 선택적 provider로 두고 실패 시 memory-only로 fail-open | 원작 DB 장애가 일반 기억 회수나 본문 응답을 차단 |

따라서 올바른 목표는 다음과 같다.

> 원작 DB를 Archive Center에서 떼어내는 것이 목적이 아니라, 원작 근거의 소유권을 분리하면서 Archive Center의 핵심 기억 선택과 Director 지휘력을 더 선명하게 만드는 것이 목적이다.

최종 수용 조건:

1. 연결 전후 동일한 입력에서 핵심 객관 기억 K의 의미와 전달 수가 변하지 않는다.
2. 연결 시에는 같은 핵심 기억에 필요한 원작 근거만 추가되고, 무관한 원작 정보가 늘지 않는다.
3. Director는 핵심 기억과 검증된 제한 보조 묶음을 함께 볼 수 있으며 모든 지시에 item-level 근거를 남긴다.
4. `없음`에서는 연결되어 있어도 감독 제안이 0이고 사실 지원만 유지된다.
5. `약함` 이상에서는 Archive Center만 응답 구성 권한을 가지며 원작 DB에는 서사 지휘 권한이 없다.
6. 원작 DB 연결 실패 시 핵심 기억, 주관 기억, 계층 기억과 본문 응답이 그대로 fail-open한다.
7. 세션 기억과 원작 정설 사이의 자동 상호 승격이 0이다.
8. 연결 상태에서는 원작 DB의 직접 주입이 0이고 동일 원작 항목의 중복 주입도 0이다.
9. 늦거나 stale인 원작 DB 응답과 본문에 함께 전달할 수 없는 감독 근거의 사용이 0이다.

## 12. 4.0-G 계획 packet — RisuAI 로어북 참조 검색 색인

상태: `source_implemented_regression_verified_live_gate_pending`

이 절은 정본 4.0 로드맵의 `4.0-G`가 채택되기 전 작성된 후보 검토를 현재 계약과 구현 경계에
맞게 정정한다. 2026-08-12 현재 G-1~G-6 source 구현과 자동 회귀는 통과했고 G-7의 실제
loaded RisuAI/PocketRisu·MariaDB·패키지 검증은 남아 있다. 과거의 전체 lorebook reader나 RisuAI
발동 엔진 복제안을 되살리지 않고, 좁은 read-only reference lane만 허용한다.

배치 위치는 3.8이나 3.9의 주기능이 아니라 **4.0 recall·delivery packet**이다.
3.8의 시간·상태·주관 지식과 3.9의 관계·습관·profile을 밀어내지 않는다. 3.9까지 확정된
stable entity와 session scope를 소비하고, 4.0에서 검색·선별·중복 제거·최종 전달을 구현한
뒤 같은 4.0 안에서 수정·삭제·모듈 전환·부분 관찰·재색인·실환경 lifecycle gate로 닫는다.
기능을 끈 사용자의 4.0 core 완료를 막지는 않는다.

현재 계획에서 허용할 기능은 다음 한 문장으로 제한한다.

> RisuAI 로어북은 계속 원본 작성·활성화·native 주입을 소유한다. Archive Center는 사용자
> 동의하에 항목별 읽기 전용 사본과 검색 색인을 유지하고, 현재 scope에서 관련된 항목을
> 출처가 붙은 참조 후보로만 제공한다.

### 공식 RisuAI 기준과 현재 관찰 한계

검토 기준은 2026-08-12의 공식 RisuAI `main`
`72ce721878d65b09baf4339638dfd221d1788261`과 PocketRisu
`85a65f3137b45c8de4a8d21a9887be213b1ac3fc`다.

- `plugins.md`의 API v3는 사용자 DB 접근 동의 후 `characters`, `modules`,
  `enabledModules`, `moduleIntergration`(공식 필드 철자)을 선택적으로 읽을 수 있다.
- `src/ts/storage/database.svelte.ts`의 `loreBook`은 key·second key·순서·본문·mode·
  always-active·selective·regex·entry ID 등을 가진다. character global lore와 chat local
  lore는 서로 다른 scope다.
- `src/ts/process/lorebook.svelte.ts`는 character global, current-chat local, enabled-module
  lore를 합친 뒤 scan depth, recursive scan, regex, additional/exclude key, priority,
  token budget, probability, greeting, sticky activation과 decorator를 반영한다.
- 공식 `beforeRequest` payload는 최종 system text는 보여도 그 span의 정확한 lorebook
  entry ID·scope·revision·activation을 일반적으로 제공하지 않는다. 현재 Archive Center의
  `prepare_turn_host_context.go`도 이 값들을 `not_exposed`로 유지한다.
- 현재 `Archive Center.js`는 공식 현재 로어북 snapshot과 character/chat/module 관찰값만
  backend에 전달한다. 별도 MariaDB snapshot 수명주기, exact/key/lexical 검색, 최종 표시
  중복 억제와 별도 참조 lane은 Go가 소유한다. 선택적 semantic 검색과 실환경 gate는 아직
  활성화·검증되지 않았다.
- `getCurrentLorebookEntries()`는 character global, current-chat local, enabled-module 원본을
  합친 snapshot이며 native activation과 token budget 적용 결과가 아니다. 합쳐진 결과에서
  항목별 출처를 확정할 수 없으면 내용을 보고 scope를 추측하지 않는다.
- 양쪽 공통 API에는 lorebook edit/delete, character/chat 전환, enabled-module 변경 event가
  없다. RisuAI `output` listener는 모델 출력 완료 신호일 뿐 로어북 변경 신호가 아니다.

따라서 “DB에서 항목을 읽을 수 있다”와 “RisuAI가 이번 요청에서 그 항목을 발동했다”를
같은 사실로 취급하지 않는다. activation이 노출되지 않으면 추측하지 않고
`activation_unknown`으로 둔다.

### 기존 폐기 경로와의 구분

정밀 장기 기억 로드맵이 폐기한 것은 전체 로어북 reader·DB/Chroma mirror와 RisuAI activation
재구현이다. 이 금지는 그대로 유지한다. 현재 4.0-G에서 허용하는 것은 다음의
더 좁은 경로다.

| 허용 | 계속 금지 |
|---|---|
| 사용자 동의 후 항목별 read-only snapshot | RisuAI 로어북을 Archive Center가 대신 편집·저장 |
| exact/key/lexical 검색과 선택적 semantic 후보 확장 | scan depth·recursive·decorator·확률·token budget 발동 복제 |
| character·chat·module scope hard filter | scope가 다른 로어북의 통합 검색 후 무차별 주입 |
| Explorer·trace의 출처가 붙은 검색 결과 | 검색 성공을 native activation·정사·세션 발생 사실로 승격 |
| 실제 payload native overlap의 중복 억제 | beforeRequest system text를 특정 entry라고 문구로 추측 |
| 별도 reference lane의 제한 전달 | 메인 기억 K·원작 Canon·관계·profile row로 자동 병합 |

### 논리 저장 계약

로어북 항목은 승인 원작 사실을 담는 `reference_claims`나 세션에서 발생한 일을 담는 메인
memory row에 넣지 않는다. 두 저장소와 lifecycle·authority가 다른
`lorebook_reference_entry.v1` 논리 namespace로 분리한다. 실제 table과 migration은 기존
store로 아래 lifecycle을 표현할 수 없는지 먼저 감사한 뒤 결정한다.

항목별 최소 필드는 다음과 같다.

- host/save identity와 관찰 contract version;
- `scope_kind`: `character_global`, `chat_local`, `enabled_module`; 합쳐진 공식 snapshot에서
  항목별 출처를 확정할 수 없으면 `current_host_aggregate`이며 내용을 보고 세 범위 중 하나로
  추측하지 않음;
- character ID, chat ID, module ID와 host entry ID. 노출되지 않은 ID는 생성해 사실처럼
  사용하지 않음;
- 원본 key·second key·comment·content·mode·regex/selective/always-active와 보존할
  decorator metadata;
- raw content hash, normalized search text hash, observed revision 또는 `not_exposed`;
- `catalog_current`, `stale`, `deleted`, `observation_unknown` lifecycle;
- `truth_authority=false`, `canonical_write=false`,
  `session_event_authority=false`, `relationship_authority=false`;
- first seen, last seen, superseded-by와 동기화 source provenance.

내용 hash는 revision·동기화·중복 확인용이지 stable lore identity나 정사 증거가 아니다.
항목이 수정되면 새 revision을 가리키고 이전 revision을 current 검색에서 제외하되 감사
이력을 보존한다. 한 번 읽지 못한 것, 권한 거부, timeout, 부분 DB 관찰을 삭제로 해석하지
않는다. 명시적 삭제 또는 같은 완전 snapshot에서의 부재가 검증되기 전에는
`observation_unknown`으로 둔다.

### 검색과 색인 계약

검색은 scope hard filter를 먼저 적용한 뒤 다음 순서로 확장한다.

1. declared key·entry ID·정확 문구;
2. comment·key·content의 lexical/full-text 검색;
3. exact·lexical 결과가 현재 요구를 충분히 지원하지 못할 때 선택적 embedding semantic 후보;
4. Go hydration 뒤 현재 revision·scope·lifecycle·privacy·중복 검증;
5. `search_candidate`와 실제 전달 `selected/delivered`를 분리해 trace.

exact/lexical 경로는 embedding 모델과 Chroma가 없어도 완전하게 작동해야 한다. semantic
색인은 후보 폭만 넓히며 truth, native activation, author priority, character knowledge,
relationship 또는 Director 권한을 올리지 않는다. 기존 vector abstraction을 재사용할 수
있어도 메인 memory·원작 Canon과는 별도 논리 lane과 필수 metadata filter를 가진다. 물리
collection을 공유할지 분리할지는 누락 filter를 제거·반전하면 실패하는 시험과 실제 query
plan·latency를 본 뒤 결정하며, unfiltered cross-lane query는 금지한다.
고정 개수나 고정 문자량을 채우기 위한 semantic filler는 만들지 않고, 하나의 고정 유사도
값으로 모든 후보를 일괄 제거하지 않는다. semantic 장애 시 exact·lexical 결과는 보존한다.

### 검색 상태와 전달 모드

| 상태 | 의미 | 허용 동작 |
|---|---|---|
| `catalog_current` | 현재 consented snapshot에서 항목을 읽음 | scope 안에서 검색 후보가 될 수 있음 |
| `search_candidate` | 현재 질의와 관련성이 있음 | Explorer·trace 또는 Go 선별 입력; 아직 주입 아님 |
| `native_present_observed` | actual payload에 정규화 후 동일한 전체 본문이 관찰됨 | Archive 로어북 참조의 반복 표시만 억제; 모호하면 특정 entry ID로 귀속하지 않음 |
| `activation_unknown` | Host가 발동 여부를 노출하지 않음 | native active라고 표시하거나 발동 복제하지 않음 |
| `stale/deleted` | 현재 revision에서 무효가 검증됨 | current 검색·전달 제외, 감사 이력만 유지 |

첫 배포 모드는 `search_only`가 적절하다. 항목별 DB 저장, exact/lexical 검색, Explorer와
trace까지만 연결하고 prompt 전달은 0으로 둔다. 이것은 최종 목표를 진단-only로 축소한다는
뜻이 아니라, Risu native activation과 Archive 검색 품질을 분리해 측정하는 4.0 내부
cutover 단계다.

후속 `reference_assist`는 사용자가 명시적으로 켠 경우에만 별도
`lorebook_reference` 예산으로 전달한다.

- 핵심 객관 기억 K를 소비하거나 밀어내지 않는다.
- `없음`에서도 factual reference 지원은 유지할 수 있지만 Supervisor·Director proposal은
  0이다.
- `약함` 이상에서 Director가 사용하는 로어북 근거는 이미 본문에 전달된 ref를 재사용하거나
  `guidance_support`로 함께 승격할 수 있을 때만 허용한다.
- 검색 후보 전체, 미전달 항목, 다른 scope, stale revision과 activation-unknown 자체는
  Director 지휘 근거가 아니다.
- RisuAI가 이미 actual payload에 넣은 동일 항목은 Archive Center가 다시 주입하지 않는다.
- index·DB·embedding 장애, 접근 거부와 계약 불일치는 RisuAI native lorebook과 메인 기억
  요청을 그대로 보존하고 해당 reference lane만 중단한다. 장애를 후보 확대나 다른 scope
  stale 자료 사용으로 보완하지 않는다.

### 기억·native 로어북과의 최종 전달 경계

로어북 참조와 Archive 기억은 저장·검색·후보 단계에서 서로 제거하지 않는다. 핵심 기억과
기타 기억의 최종 전달 계획을 먼저 확정한 뒤에만 Archive 로어북 참조의 반복 표시를 줄인다.

- CRLF/LF, 본문 앞뒤 공백과 연속 공백만 정규화한 전체 본문이 같거나, 같은 Host entry의 같은
  revision임이 확인되거나, 로어북 전체 본문이 actual payload에 그대로 있을 때만 확정
  중복으로 인정한다.
- Vector 유사도, 이름·key 일치, 일부 문장 겹침과 비슷한 요약만으로 제거하지 않는다.
- actual payload에 동일 본문이 있으면 Archive 로어북 참조만 생략한다.
- 실제 delivered 기억과 확실히 같으면 기억을 남기고 Archive 로어북 참조만 생략한다.
- current state·현재 관계·위치·소유 상태와 로어북 기본값의 충돌은 중복이 아니다. 최신
  accepted 기억을 남기며, 주관 기억도 객관 설정과 비슷하다는 이유로 생략하지 않는다.
- 판단이 모호하면 둘 다 유지한다. 하나를 생략하기 전에 다른 하나의 실제 payload 전달을
  확정하며, 둘 다 없어지는 결과를 금지한다.
- 중복 억제는 현재 요청의 표시만 줄이고 원본 snapshot, memory, evidence, relation, entity와
  subjective-memory 행을 수정·삭제하지 않는다.

핵심 기억을 먼저 확정하고 로어북 참조는 남은 전체 보조 입력 문자 예산을 사용하는 별도
lane에 둔다. 핵심 기억 K를 소비하거나 밀어내지 않고, 별도 lane도 전체 모델 입력 상한에서는
면제하지 않는다. 고정 개수·고정 chars를 채우지 않는다.

### Host/backend 소유권

JavaScript는 공식 RisuAI API에서 사용자 동의를 확인하고 현재 character/chat/module 식별자와
항목 snapshot을 관찰해 versioned packet으로 전송하는 일만 맡는다. 전체 DB polling,
activation 판정, ranking, stale/delete 판정, 예산, 중복 제거와 prompt 조립을 JavaScript에
추가하지 않는다.

Go는 snapshot 검증, hash/revision, MariaDB lifecycle, 검색·색인, scope hard filter,
candidate/selected/delivered 상태, native overlap, 예산과 최종 delivery plan을 소유한다.
공통 공식 변경 event가 없으므로 snapshot 갱신은 첫 활성화·현재 범위 snapshot 부재·사용자
수동 새로고침·관찰된 character/chat index 변경·동의를 받아 읽은 enabledModules 집합 변경·
명시적 복구/재색인으로 제한한다. 같은 범위 안의 edit/delete는 수동 새로고침으로 갱신한다.
일반 턴은 MariaDB만 검색하고 Host 전체 로어북을 다시 읽지 않는다. 관찰되지 않은 삭제를
polling, DOM 감시나 `output` listener로 추측하지 않는다.

### 버전 배치와 수용 판단

| 버전 | 이 기능에서 맡을 범위 |
|---|---|
| 3.8 | 새 로어북 기능을 넣지 않음. time·state·perspective의 기존 source/authority gate 보존 |
| 3.9 | stable entity와 character/chat/module scope를 검색 filter가 소비할 수 있게 함. lore를 profile·relation evidence로 자동 승격하지 않음 |
| 4.0 | 항목 snapshot admission·revision, exact/lexical search, optional semantic index, Explorer, 별도 reference delivery, native/기억 중복 경계, entry edit·delete·module 전환·부분 관찰·restart·reindex와 실제 RisuAI/PocketRisu lifecycle gate |
| 4.1 | 4.0 로어북 기능을 확장하지 않음. 4.0의 검증된 read-only 결과를 다른 4.1 기능이 필요할 때만 소비 |

따라서 이 기능은 3.8~4.0 사이에 분산하지 않고, 3.8·3.9의 identity/scope 결과를 소비하는
**4.0의 독립 recall packet**으로 둔다. 현재 상태는
`source_implemented_regression_verified_live_gate_pending`이다. source·schema·검색 fixture와
자동 회귀만으로 완료하지 않고 loaded RisuAI/PocketRisu·실제 MariaDB·패키지 lifecycle 검증
전에는 완료로 표시하지 않는다.

## 13. 공통 필수 회귀·품질 시험

### 기억 전달

- 기존 요청에 새 K 필드가 없으면 출력 불변
- `topK`만 바꾸면 최종 K 계약이 바뀌지 않음
- 핵심 K만 바꾸면 객관 사건 기억만 달라짐
- 관련 기억과 예산이 충분한 fixture에서 requested K와 실제 payload delivered count가 일치
- selected/rendered/delivered exact item ref가 payload lineage와 HUD에서 동일하고 eligible 항목의 무음 유실 0
- 구조화 저장본의 source·actor·polarity·time·owner metadata가 보존되지만 본문에는 승인된 완결 한 문장만 전달
- K를 채우기 위한 recent garbage 주입 0
- vector 성공 시 lexical garbage padding 0
- vector 장애 시 구조 근거 없는 이름 겹침 기억 0
- hierarchy, subjective, protected, world가 K를 소비하지 않음
- recent·continuity·historical 중 관련 항목이 있는 horizon은 coverage되고 빈 horizon을 무관한 기억으로 채우지 않음
- summary에서 actor·canonical anchor·부정·owner 경계를 빠뜨린 fixture가 coverage audit에 걸리고 승인된 재추출 또는 제한 direct evidence만 사용
- coverage fallback에 raw private·stale·superseded·다른 branch 근거 0
- 한글·이모지 포함 rune cap과 UTF-8 보존
- 길이·예산·중복 drop reason이 payload lineage와 HUD에서 일치
- embedding·reranker·keyword·graph·Supervisor 각각의 장애에서 delivered 집합이 정상 hard-gated 집합보다 넓어지지 않음
- degraded에서 raw chat·전체 node·무관련 child·stale cache filler 0
- selected·delivered·co-fired·response-similar 변화만으로 canonical row·importance·lifecycle 변화 0

### [기능 활성화 시] 4.0-G 로어북 참조 검색

- character global, chat local, enabled module의 동일 key·동일 content가 서로 다른 scope와
  provenance로 보존되고 현재 scope 밖 항목 전달 0
- 접근 동의 거부·timeout·부분 DB 관찰을 삭제로 해석하거나 기존 current entry를
  tombstone하는 경우 0
- 항목 edit·delete·module disable/전환 뒤 old revision이 current 검색·delivery에
  재등장하지 않고 감사 이력만 유지
- embedding·Chroma·reranker가 없어도 exact/key/lexical 검색과 Explorer가 동일 hard filter로
  작동
- semantic 검색은 lexical miss의 관련 후보를 보완하지만 native activation·truth·priority·
  knowledge·relationship 권한을 올리지 않음
- actual payload에 이미 존재하는 lore span과 Archive Center reference의 중복 본문 전달 0
- 실제 delivered 기억과 확정 중복인 Archive reference의 반복 표시는 0이지만 memory 원본과
  delivered 기억은 그대로 유지
- 부분 문장·이름·Vector 유사도만으로 기억이나 reference를 제거하는 경우 0
- 중복 억제 뒤 기억과 로어북 참조가 모두 payload에서 사라지는 경우 0
- accepted current state·현재 관계·주관 기억을 로어북 기본값과 비슷하다는 이유로 제거 0
- `activation_unknown` 항목을 native active로 표시하거나 RisuAI scan/decorator/probability
  결과를 재계산하는 경우 0
- `search_only`에서 prompt·Supervisor·Director로 전달되는 lorebook text 0
- `reference_assist`에서 selected/delivered ref와 실제 payload의 정규화한 전체 본문·source ref가 일치하고
  미전달 candidate를 Director가 사용하는 경우 0
- lorebook index·DB·embedding 장애가 핵심 기억 K, current input, RisuAI native lorebook과
  본문 응답을 차단하거나 delivered 집합을 확대하는 경우 0
- 일반 턴마다 Host 전체 로어북을 다시 읽거나 `output` listener·DOM·polling을 변경 신호로
  사용하는 경우 0

### 연속성·시간·계층 선택

- 빈 입력, 짧은 재개, 장기 공백 뒤 재개에서 현재 branch의 마지막 accepted final만 continuity seed로 사용
- 명확한 새 사용자 방향이 있으면 오래된 continuity/storyline seed 강등
- `explicit_redirection` fixture에서 이전 foreground·arc·frontier의 focus lease와 의존
  proposal이 같은 요청에 해제되고, 새 사건·우연·NPC 행동으로 이전 storyline에 되돌리는
  출력 0
- 빈 입력·짧은 반응·Supervisor 침묵만으로 이전 focus lease가 자동 갱신되는 경우 0
- scene·relationship·thread query가 서로 다른 관련 기억을 보완하지만 match count가 truth score나 closure readiness를 올리지 않음
- raw private text가 query provider에 전달되지 않음
- weak/reentry/temporal에서 필요한 anchor만 늘고 raw 최근 N개 투입은 0
- earliest/middle/latest/turn-range가 실제 유효 범위를 고르고 범위 증거가 없으면 gap 반환
- current 질문에 superseded·deleted 상태가 현재 사실로 전달되지 않고 historical 질문에서만 시간 표지와 함께 회수
- transition 질문은 이전·대체 상태를 source ref로 구분하고 dispute 질문은 contested·unknown을 확정 사실로 평탄화하지 않음
- episode/chapter/arc/saga 중 필요한 해상도만 선택되고 겹치는 계층 동시 주입 0
- hierarchy zoom이 핵심 K, source·branch·visibility 필터와 Host hard budget을 바꾸지 않음
- offscreen `last_confirmed_ref`만 있고 `present_bridge_ref`가 없는 fixture를 현재 활동·새 dialogue로 렌더링하지 않음
- bridge 없는 “계속 조사/대기/작업” summary echo와 replay 0
- accepted current-state projection·진행 중 효과·즉시 상호작용 의무는 유효 source/time 범위 안에서 bridge가 되지만 목적지 이름·과거 계획만으로는 bridge가 되지 않음
- stale·wrong-branch·wrong-generation·wrong-revision·private·wrong-knower·expired·not-yet-effective bridge ref의 current support 승격 0
- present bridge 부재가 원 기억의 dormant·resolved·삭제·canon 약화로 전파되지 않음

### 주관 기억

- 한 owner의 서로 다른 약속·오해·감정이 모두 관련된 fixture
- owner 간 coverage 우선 후 고관련 추가 선택
- raw secret이 Supervisor, main payload, trace, HUD에 0
- private interpretation이 다른 인물의 known fact로 전파되지 않음
- 동일 명제의 truth status가 같아도 narrator assertion/character utterance/question/proposal/hypothesis/quoted report의 assertion kind가 섞이지 않음
- 같은 assertion도 인물별 direct perception/received communication/remembered event/public record의 knowledge path와 known/believed/suspected/heard/unknown/misinformed epistemic state가 다르게 유지됨
- 검색된 사실이 retrieval 성공만으로 등장인물 knowledge에 추가되지 않음

### 감독 권한

- none에서 Supervisor 호출과 Director proposal 0
- none에서도 기억·상태·비밀 guard 유지
- weak/medium/strong에서 memory/reference hash 동일
- strength에 따라 output guidance만 변화
- user/player action 강제 0
- relation·closure envelope 밖 제안 0
- source ref 없는 항목 0
- `hold_allowed` 장면의 강제 진행 0
- malformed, empty, unsupported, budget skip 상태 분리
- Supervisor timeout·panic·provider 오류가 main response를 막지 않음
- 이전 turn/branch/revision proposal 재사용 0
- Supervisor recent context에 accepted logical turn만 있고 raw private·streaming·superseded message 0
- current input 외 과거 user text의 command authority 0
- 감독이 사용한 모든 support ref가 본문 기존 레인 또는 `guidance_support`에 존재하고 동일 텍스트 중복 0
- 예산·privacy·stale fence 때문에 승격할 수 없는 support ref 기반 지시 0
- carry state가 accepted state/thread에서 재구성되고 자유문장 Supervisor plan 캐시 0
- reroll·delete·branch·revision 변경 뒤 proposal과 carry state 동시 무효화
- focus lease가 `response_only|scene`, source user turn과 expiry를 가지며 만료 뒤 자동
  foreground 재사용 0
- current input이 다른 thread·인물·장소·목표를 선택했는데 이전 arc·momentum·callback을
  이유로 old thread를 foreground로 되돌리는 경우 0
- 한 thread의 resolve 뒤 다른 open thread가 유지되고, 명시적 global closure 근거 없이
  작품 엔딩·에필로그·세션 종료를 지시하는 경우 0
- `ending_edge` 변화가 thread·arc·작품 closure 상태를 변경하는 경우 0
- retrieval miss·Supervisor 침묵·높은 score만으로 thread expire/resolve 0
- 관계·세계·캐릭터 실행 표면에서 새 사실·감정·지식·관계 단계 생성 0
- expression hint가 source ref를 가지며 기억 설명 강제·비밀 외부화·필드 간 동일 문장 반복 0
- compact/standard/deep capability profile에서 eligibility·핵심 K·비밀·권한 판정 동일
- `base_only`에서 보조 LLM 호출 0이고 exact/lexical recall·핵심 K·직접 근거·주관 기억 유지
- `reviewed`에서 Reviewer가 Director plan을 승인·축소·거절할 수 있지만 새 지시·ref·권한 추가 0
- `deep`에서 Recall Auditor가 Go eligible support 밖 검색·visibility 확대·사실 생성 0
- 첫 보조 호출 전 고정한 `eligible_support_refs` hash와 최종 used refs가 일치하고
  Recall Auditor의 새 query·lane·graph traversal·materialization 요청 0
- Director·Reviewer 실패 또는 timeout 뒤 이전 성공 결과 재사용 0
- `reviewed`/`deep`의 필수 Reviewer가 성공하지 않았는데 Director plan 일부가 본문에 전달되는 경우 0
- weak 이상 요청에서 모델 없음·budget skip·필수 독립 Reviewer 없음이면
  `effective_supervision_depth=base_only`, `effective_plan_status=proposal_0`
- 위 degraded 상태에서 default guidance prose·이전 plan·감독 적용 성공 HUD 표시 0
- 한 단계 실패 뒤 최종 plan이 정상 성공 경로보다 넓어지는 경우 0
- role별 provider/model·입력 data class·호출/skip/degraded·latency와 `same_model_review` trace 일치
- 동일 LLM 응답을 Director와 Reviewer 결과로 중복 기록하거나 same-model 검토를 독립 검토로 표시하는 경우 0
- 같은 고정 eligible set과 동일 fixture에서 `base_only`·`directed`·`reviewed`·`deep`을
  비교하고, 각 추가 역할이 잡은 누락·무근거 지시·사용자 의도 충돌을 role별로 계측
- Recall Auditor가 관련 핵심 기억의 coverage gap을 잡는 fixture와 Reviewer가 지원되지
  않은 Director 지시를 거절하는 fixture에서 각 역할의 독립 기여를 확인
- 추가 역할이 plan 품질을 개선하지 않거나 같은 오류만 반복하는 요청에서 `deep`을
  무조건 호출하지 않고 typed need signal·비용·latency 근거로 skip
- 최종 본문 모델이 Go가 승인한 effective plan 이외의 Recall Auditor·탈락 Director·Reviewer raw text를 받는 경우 0
- 원본 current input을 보존하고 명시적 별도 기능 없이 다층 감독이 사용자 입력을 rewrite하는 경우 0
- `advance_allowed=true`와 유효한 `may_advance` ref 없이 strong plan이 사건·관계·
  사용자 행동·scene frontier를 전진시키는 경우 0

### [조건부: Recomposer 공동 사용 활성화] 단일 계약과 원자적 위임

- Recomposer 미설치·완전 비활성에서 유효 publisher plan이 있으면 Archive Center
  `output_guidance`로 정확히 1회 전달되고 Recomposer rewrite는 0. `proposal_0`과
  `guidance_strength=none`, `base_only`는 publisher plan과 main-guidance
  materialization 모두 0
- 공동 사용만 미선택됐지만 standalone Recomposer가 활성인 경우 Archive direct plan
  1회를 유지하고 `archive_direct_post_recompose`를 허용할 수 있으나, 같은 publisher
  plan의 standalone pre-contract 재 materialization은 0
- Recomposer 공동 사용에서는 Publisher LLM 실행이 route와 관계없이 1회 이하이고,
  같은 plan의 Archive 직접 블록과 Recomposer `effective_turn_contract` 동시 전달 0
- 정상 통합에서 Archive bridge, Recomposer snapshot, Draft Ledger, Composer, Prover와
  final trace가 같은 publisher plan ID/hash와 effective contract ID/digest를 가리킴
- Recomposer가 같은 요청의 publisher plan을 beforeRequest에서 소비하고 실제 main
  payload에 단일 contract를 materialize한 경우에만 `integrated_single_contract`로 기록
- 반대 load order 등 pre-response 통합을 증명하지 못한 순서는 integrated로 표시하지 않고
  `archive_direct` 또는 명시적 post-recompose fallback으로 기록
- `archive_direct_post_recompose`에서 Archive direct publisher plan 외 Recomposer
  standalone pre-contract 주입 0
- Input Planner·입력 Contract Judge·Go contract compiler·pre-dispatch commit 각각의
  throw, timeout, malformed, digest mismatch에서 부분 contract 0, 동일 publisher plan
  fallback 1회, Publisher 재호출 0
- commit receipt의 실제 main message array에서 태그된 Archive fallback 0,
  `effective_turn_contract` 1, identity/hash 일치. abort receipt에서는 각각 1과 0
- 위임 성공 전후 memory/reference/secret/privacy/agency/authority hash와
  `eligible_support_refs` 상한 변화 0
- Recomposer가 Archive hard guard 또는 immutable publisher plan을 축소·보류·재정렬·
  삭제·확대하는 경우 0. Input Planner·Contract Judge는 Go 검증용 proposal만 반환
- standalone Planner fragment와 publisher plan이 충돌하면 current input과 Go hard guard
  우선으로 Go가 최종 contract를 확정하며, 승인됐다는 사실만으로 publisher proposal을
  established fact로 승격하는 경우 0
- 단일 canonical contract를 사용하더라도 provider × role projection이 유지되고 허가되지
  않은 role request의 subjective/private/protected-secret sentinel 0
- 필수 Judge·Composer·Prover·repair 실패, required-role quorum 미충족 또는 final
  pipeline `failed|rejected`에서 반환 text digest가 실제 `draft_zero_digest`와 일치하고
  partial rewrite·탈락 후보·이전 cache 반환 0
- 선택적 specialist 실패는 해당 lane의 부분 출력을 폐기하고 남은 후보의 독립성·contract
  coverage·Judge·Composer·Prover 통과를 모두 증명한 경우에만 degraded 성공
- 같은 user text·draft라도 backend request, transaction, payload plan, publisher plan,
  committed route, branch, generation, revision, source snapshot, expiry 또는 effective
  contract digest가 다르면 output reuse cache hit 0
- reroll·edit·delete·branch·restart·중복 afterRequest·응답 역순 완료에서 claim, plan,
  contract, final receipt와 output이 서로 다른 request로 교차하는 경우 0
- current-turn complete-turn이 트리거하는 Critic 입력은 실제 displayed final 하나만 받고
  `draft_zero`, specialist 후보, Judge·Reviewer raw text, 실패한 Composer 결과 admission 0
- current-turn claim 뒤 final receipt가 없거나 receipt hash와 공식 displayed-final hash가
  다르면 사용자 응답은 반환되지만 complete-turn·Critic 호출과 canonical mutation은 0,
  상태는 `deferred_finality_unobserved`
- trace가 publisher call/plan 상태, requested/committed route,
  main-guidance materialization owner, post-rewrite context consumer, final prose owner,
  commit/abort reason, draft/final/critic input hash를 서로 구분

### 저장·정리·rollback

- strict schema가 아닌 malformed·부분 복구 모델 출력의 canonical mutation 0
- 같은 의존성 mutation batch 안의 한 operation이 실패하면 그 batch의 canonical create/update/delete/merge가 전부 0이고 독립 lane 결과는 자체 검증대로 유지
- malformed derived operation이 accepted raw/source 저장·deferred 재처리 기록을 취소하거나 유실시키지 않음
- exact duplicate replay는 idempotent no-op이지만 cross-source semantic merge는 source·entity·time·knower 검토 없이 실행되지 않음
- semantic merge·importance update를 만든 source turn rollback 뒤 대상 row와 파생 projection이 이전 fingerprint와 동일
- rollback manifest가 raw/source, atomic·subjective memory, current/history, relation, hierarchy, vector/index, cache, carry/proposal과 search trace를 모두 설명
- async vector cleanup 중에도 invalidated canonical row를 stale index가 payload에 다시 올리지 않음
- hidden/partial/unobserved/timeout Host read가 canonical 삭제·무효화를 만들지 않고 반복 관측만으로 accepted delete로 승격되지 않음
- versioned accepted delete/rollback observation만 source lifecycle을 변경
- storage read/parse 실패 뒤 빈 원장 또는 부분 상태가 last-known-good를 덮어쓰지 않음
- 요청·시도·제안·예상 결과가 current exact 값을 바꾸지 않고 directional 변화는 이전 exact baseline을 보존
- 임시 위탁·장착·수리·감정이 owner를 holder/custodian으로 자동 이전하지 않음
- reroll·rollback 뒤 pending/proposed application이 exact current로 남지 않음
- 한 번의 reroll·거절 출력만으로 session 밖 user preference가 생성·승격되지 않음
- provider × role 공개표와 실제 실행 trace의 data class·skip/degraded 상태가 일치하고 manifest 자체의 raw private text는 0
- durable breadcrumb의 raw source/private/request body 0, operation ID·source revision fence 없는 restart/retry payload·result 재사용 0
- locator/negative-search trace가 session/scope digest·source revision·branch·query digest/intent·owner/knower/visibility-policy revision·eligibility contract·searchable lane coverage completeness·provider readiness·expiry 경계 밖에서 재사용되지 않고 canonical fact로 주입되지 않음
- private/incomplete-lane `not_found`가 public·다른 POV·complete-lane 검색을 억제하지 않음

### complete-turn admission

- Supervisor 문장이 memory·canon·relationship evidence로 저장되지 않음
- 본문에 등장해도 actor·causal·consent·closure 근거가 없으면 상태 승격 거부
- reroll·delete·branch 뒤 stale proposal로 기억·관계·thread가 부활하지 않음

### 실제 RisuAI

source test만으로 “감독 기능이 부활했다”고 판정하면 안 된다.

- 지원 RisuAI tag/commit과 loaded artifact hash 기록
- none/weak/medium/strong의 실제 payload와 표시 본문 연결
- 한국어 입력
- streaming, reroll, edit, delete, branch, restart
- 사용자 행동 강제·비밀 누출·관계 급변 허용 0
- 첫 출력 채택률, 리롤률과 리롤 사유
- 사용자 redirection 뒤 이전 storyline 강제 복귀율과 `storyline을 놓아주지 않음` 리롤 사유
- 국소 thread 해결 뒤 잘못된 작품 엔딩·에필로그 전환율
- focus lease의 source user turn·만료·갱신·해제 reason과 실제 foreground 일치율
- Recomposer 공동 사용 시 두 플러그인의 양쪽 load order에서 정확히 하나의 guidance
  application route, 정확한 fallback 출력과 displayed-final admission 관찰
- p50/p95 지연과 입력·출력 비용

## 14. 공통 출시 중단 조건

아래 조건의 적용 범위는 기능 종류에 따라 다르다.

- Supervisor·Director·strength·guidance 조건은 해당 weak 이상 감독 기능의 release gate다.
- source acceptance, 기본 기억, K delivery, privacy, storage, rollback과 canonical admission 조건은 `none`을 포함한 모든 모드와 전체 memory release gate다.
- graph mutation, locator, semantic merge UI처럼 아직 선택적인 후속 기능의 고유 조건은 그 기능을 활성화하는 release gate다. 다만 현재 production 경로에 이미 존재하는 자동 dedup·importance mutation은 3.7 재검증 packet의 감사 대상이며 rollback fixture를 통과하지 못하면 비활성화 또는 제한하기 전까지 기억 품질 개선 완료를 선언하지 않는다.
- 로어북 참조 검색 시험·중단 조건은 정본 로드맵 변경이 승인되고 해당 기능을 활성화한
  release에만 적용한다. 승인 전에는 전체 memory release를 막지 않는다.
- Recomposer 공동 사용 시험은 해당 위임을 활성화한 release에만 적용한다. 다만 그
  integration 안의 단일 계약, 중복 적용 방지, role별 privacy, 원자적 fallback과
  displayed-final admission은 release-blocking 조건이다.

해당 범위에서 다음 중 하나라도 발생하면 출시를 중단한다.

- 기존 v3 의미를 versioning 없이 변경
- none에서 Supervisor가 호출되거나 Director proposal이 주입됨
- none이 기억·직접 근거·비밀 guard까지 끔
- 사용자·주인공 행동 강제
- private/secret/spoiler 누출
- unsupported proposal의 event·relation·thread·canon 승격
- reroll·delete·branch 후 stale plan 재사용
- Supervisor 실패가 main response를 막거나 이전 plan을 재사용
- Recall Auditor·Director·Reviewer 호출 수를 guidance authority나 품질 완료 증거로 사용
- Reviewer가 Director 결과에 새 지시·새 ref·더 높은 strength를 추가
- `reviewed`/`deep`의 Reviewer 성공 상태 없이 Director plan 또는 그 일부를 본문에 전달
- weak 이상에서 보조 모델 없음·skip·독립 Reviewer 부재인데 effective plan, default prose,
  이전 plan이 전달되거나 HUD가 감독 적용 성공으로 표시
- `none`·`base_only`·`proposal_0`에서 publisher plan 또는 main-guidance materialization이
  생기거나 HUD가 감독 성공으로 표시
- 보조 단계 실패가 더 넓은 candidate·plan·visibility·예산으로 fallback
- supervision depth가 pre-LLM eligible set, source·time·branch·generation·revision,
  visibility·authority·핵심 K 또는 hard budget 상한을 확대
- 동일 응답을 여러 role의 독립 검토로 기록하거나 role별 provider/model·data class·latency를 설명할 수 없음
- 다층 감독 경로가 current input을 무단 rewrite하거나 Reviewer raw text를 본문 payload에 직접 주입
- 같은 턴에 서로 다른 publisher plan·`effective_turn_contract` digest가 둘 이상 적용되거나
  Archive Center 직접 guidance와 Recomposer contract에 같은 plan이 함께 들어감
- Recomposer의 current-request contract commit 증거 없이 Archive Center publisher 직접
  적용을 억제하거나, 전처리 실패 뒤 부분 Recomposer contract와 Archive fallback을 함께 전송
- 위임 실패가 Publisher를 재호출하거나 부분 rewrite·이전 turn plan·standalone contract로
  무음 fallback하고, 실제 guided `draft_zero`와 다른 text를 반환
- Recomposer가 immutable publisher directive를 축소·보류·재정렬·삭제·확대하거나
  eligible ref·truth·privacy·agency·relationship·closure 권한을 바꾸고, publisher
  proposal을 established/canonical fact로 세탁
- publisher plan과 output reuse identity를 통합하기 전에 cache를 조회하거나
  branch·generation·revision·source snapshot·plan digest가 다른 결과를 재사용
- 단일 contract를 이유로 role/provider visibility를 평탄화하거나 raw private·secret
  plan 내용이 bridge·trace·cache·pluginStorage·HUD에 남음
- 사용자가 본 displayed final이 아닌 `draft_zero`·중간 rewrite를 Critic·complete-turn에
  제출하거나 final receipt 불일치 상태에서 canonical write를 실행
- Recomposer 설치 여부, 전역 테스트 객체, plugin 이름 또는 prompt marker만으로
  guidance route를 선택하고 current-request claim·contract 적용 관찰을 대체
- trace가 publisher owner, main-guidance materialization owner, post-rewrite context
  consumer, final prose owner, commit/abort, fallback과 draft/returned/displayed/critic
  hash를 설명하지 못함
- `advance_allowed=true`와 유효한 `may_advance` ref 없이 strong이 사건·관계·사용자 행동·
  scene frontier를 전진
- Top K 의미가 UI, HUD, lineage에서 불일치
- eligible·selected·rendered·delivered item ref가 어긋나거나 예산이 충분한 핵심 K가 무음 유실
- garbage-memory must-not-deliver fixture 실패
- summary coverage fallback이 raw/private/stale/superseded/다른 branch 근거를 주입
- current 질문에 superseded·deleted 기억을 현재 사실로 주입하거나 contested 기억을 확정 사실로 평탄화
- continuity/reentry 판정을 prompt 문자열·플러그인 이름·고정 다국어 단어 목록으로 새로 구현
- raw 최근 메시지, 탈락 검색 후보, 미전달 support를 Supervisor 지휘 근거로 사용
- multi-query match count를 진실성·정사·thread 해결 신호로 사용
- `explicit_redirection` 뒤 이전 foreground·arc·frontier 또는 carry가 계속 적용되거나
  새 사건을 만들어 사용자를 이전 storyline으로 되돌림
- current user turn의 동일 thread 선택 근거 없이 focus lease를 갱신하거나
  response/scene 범위를 넘어 단일 storyline을 기본 foreground로 고정
- 한 thread의 해결 또는 `ending_edge`를 작품 전체 엔딩·에필로그·세션 종료 근거로 사용
- 보조 모델·embedding·keyword·graph 장애가 candidate breadth, K, visibility, authority 또는 hard budget을 확대
- selected·delivered·co-fired·response-similar·activation을 canon 승격, importance, relation, closure 또는 삭제 근거로 사용
- malformed·부분 복구 LLM 결과를 같은 의존성 canonical mutation batch에 일부 적용
- source·entity·time·knower 검토와 되돌릴 lineage 없이 cross-source semantic merge 실행
- rollback 뒤 rollback 대상 이후의 invalidated tail 또는 wrong-branch canonical·derived·index·cache·carry가 재노출
- hidden/partial/unobserved/timeout Host read 또는 그 반복을 accepted delete·source invalidation 근거로 사용
- storage read/parse 실패를 빈 원장으로 치환한 뒤 정상 save로 덮어씀
- 활성 외부 호출의 provider × role 전송 data class·제외 범위·실행 상태를 설명할 수 없음
- durable breadcrumb에 raw source/private/request body를 기록하거나 operation ID·source revision fence 없이 restart/retry에서 이전 payload·result를 재사용
- 검색 결과를 등장인물 knowledge path로 자동 승격
- 로어북 catalog 조회 또는 semantic match를 native activation·원작 Canon·세션 사건·인물
  지식·관계·profile 근거로 자동 승격
- 로어북 접근 거부·timeout·부분 snapshot을 accepted delete 또는 source invalidation으로
  해석
- character/chat/module scope filter 없는 로어북 검색 또는 stale/deleted revision 전달
- RisuAI actual payload의 native lore와 Archive Center `lorebook_reference`의 중복 주입
- JavaScript 또는 Go에서 RisuAI scan depth·recursive·decorator·확률·token budget activation을
  병렬 재구현
- lorebook index·DB·embedding 장애가 RisuAI native lorebook, 핵심 기억 K 또는 main response를
  차단하거나 fallback 후보를 확대
- accepted evidence 없이 current exact 값을 바꾸거나 directional/pending/proposed를 exact applied로 평탄화
- owner와 holder/custodian/availability를 하나의 ownership current 값으로 평탄화
- `last_confirmed` offscreen 상태를 유효한 present bridge 없이 현재 활동·대사로 재생
- stale·wrong-branch·wrong-generation·wrong-revision·private·wrong-knower·expired·not-yet-effective bridge ref를 current support로 사용
- private simulation, web 결과 또는 자유문장 agent memory가 canonical source로 승격
- 한 번의 reroll·거절 출력만으로 전역 user preference를 확정
- 출처·만료·우선순위 없는 agent note를 user tail에 넣고 무조건 반영을 강제
- locator `not_found`를 session/scope digest·source revision·branch·query digest/intent·owner/knower/visibility-policy revision·eligibility contract·searchable lane coverage completeness·provider readiness·expiry 경계 밖에서 재사용해 새 검색을 억제
- episode/chapter/arc/saga를 관련도·해상도 선택 없이 동시에 주입
- reroll·delete·branch 뒤 guidance carry state 또는 파생 지침 재사용
- 검색 miss나 Supervisor 침묵으로 thread·관계·arc를 expire/resolve
- capability profile이 source·visibility·K·agency·secret·authority 판정을 변경
- compact profile에서 agency·secret·authority 경계 누락
- JavaScript에 선택·예산·감독 fallback 정책 추가
- source → payload → displayed final → admission lineage 단절
- source test만 있고 loaded artifact와 실제 RisuAI 증거가 없음
- 다층 감독을 호출 수만 늘린 채 role별 누락 감소·무근거 지시 차단의 독립 기여를
  ablation으로 증명하지 못함

## 15. 에이전트 교차검증 기록

GPT 하위 에이전트 4개가 다음 역할로 독립 검토했다.

- 1.0 실제 동작·기억·감독 조사
- 3.6 조사 기준선의 회귀·이상 동작과 현재 3.7 이월 여부 조사
- 강도별 권한·계약 설계
- 안전 위험·시험·출시 중단 조건 검토

2차 교차검증에서 네 가지 이견을 해결했다.

### 이견 A — K를 기존 topK로 재정의할 것인가

- 한쪽: 기존 topK는 vector 검색 깊이이므로 유지해야 한다.
- 다른 쪽: 사용자는 최종 핵심 기억 수를 원한다.
- 합의: 기존 backend 검색 limit는 유지하고 사용자-facing 핵심 기억 K를 별도
  final-delivery 계약으로 둔다. 이 합의는 3.7-F source의
  `core_objective_memory_delivery.v1`에 구현됐고, 4.0은 이를 재작성하지 않고
  `memory_recall_plan.v2`·`memory_injection_plan.v2`에서 소비한다.

### 이견 B — strong이 새 complication을 만들 수 있는가

- 한쪽: 1.0 체감을 위해 strong에 가역적 complication이 필요하다.
- 다른 쪽: 기본 strong에 넣으면 4.6~5.0 경계와 사용자 동의를 넘는다.
- 합의: strong은 현재 사용자가 계속 선택한 thread의 짧은 focus lease 안에서 기존 causal
  frontier 하나를 선호할 수 있다. 새 creative complication은 별도 명시적 proactive
  opt-in으로 분리한다.

### 이견 C — 지속 방향을 복원하면 레일로딩도 돌아오는가

- 1.0 사용 피드백은 한 storyline이 끝나면 작품 전체 엔딩처럼 느껴지고, 다른 이야기로
  넘어가려 해도 이전 사건 쪽으로 되돌아오는 문제가 있었다.
- 지속 carry를 강하게 유지하면 장면 연속성은 좋아지지만 current user input보다 과거
  arc·momentum이 높은 사실상의 plot 권한을 얻는다.
- 반대로 carry를 모두 없애면 1.0의 장면 연결·미해결 tension 관리 장점도 사라진다.
- 합의: persistent plot direction을 복원하지 않고, current user turn에 귀속된
  response/scene 범위 focus lease만 둔다. `explicit_redirection`은 같은 요청에서 이전
  focus와 의존 plan을 해제하며, 한 thread의 해결은 global closure로 전파하지 않는다.

### 이견 D — 별도 플러그인과 출판사의 전처리 소유권

- 별도 플러그인이 전처리를 소유한 요청에서 Archive Center Publisher까지 실행하면 두 계획과
  중복 지시가 생긴다.
- 플러그인을 사용하지 않는 요청에는 Archive Center의 독립 출판사가 필요하다.
- 현재 합의: 연동 전처리가 요청을 소유하면 Archive Center Publisher를 자동 OFF한다.
  연동하지 않은 요청에서만 Archive Center Publisher를 사용한다. Archive 기억 검색과
  privacy·authority는 Publisher와 별개이므로 연동 요청에서도 유지한다.

전체 합의:

- 1.0의 감독 체감은 복원할 가치가 있다.
- truth·canonical write 권한은 복원하지 않는다.
- none은 감독 OFF이지 기억 지원 OFF가 아니다.
- Archive Center와 Recomposer 공동 사용은 하나의 publisher plan, 하나의 effective
  turn contract와 하나의 main-guidance materialization owner만 사용한다.
- 4.6~5.0은 1.0 감독의 완제품이 아니라 더 안전한 기반이다.
- 검색 랭킹 전체가 garbage라고 단정할 증거는 아직 없다.
- compact limit, hierarchy/persona relevance, 고정 owner cap, HUD lineage는 3.7-F
  source에서 보강됐다. loaded artifact와 실제 장기 대화 품질은 3.7-G 이후 증거가 필요하다.

### 1.0 강점 추가 재검토와 3.6 비퇴행 합의

3차 검토에서는 “1.0의 장점을 더 가져오는 과정이 3.6의 강점을 약화시키지 않는가”를 중심으로 다시 대조했다.

추가로 회수해야 한다고 합의한 능력:

- 약한 입력·재개의 continuity query
- scene·relationship·unresolved thread 다중 검색
- 입력 성격에 따른 적응형 이전 맥락
- 실제 범위를 고르는 temporal anchor
- hierarchy stack을 줄이는 해상도 zoom
- Supervisor용 accepted recent context
- Go 재구성형이되 사용자 재선택이 필요한 짧은 guidance focus lease와 비강제 carry 후보
- thread focus radar와 momentum
- source-linked 관계·세계·캐릭터 실행 표면
- 기억의 행동·subtext 표현과 Director 필드 의미 중복 억제
- capability-aware 표현 예산

의견 충돌은 “Director가 최종 직접 주입 기억보다 넓은 자료를 볼 수 있는가”였다.

- 너무 좁으면 1.0처럼 누락된 기억을 장면 지침으로 보완하지 못한다.
- 검색 후보 전체를 주면 미검증·private·stale·본문 미전달 사실로 지휘할 수 있다.
- 합의는 source·branch·revision·visibility·privacy 검사를 통과한 bounded eligible support만 보게 하는 것이다.
- Director가 실제 사용한 항목은 본문 기존 ref를 재사용하거나 compact 표현과 `source_ref`를 `guidance_support`로 승격하며, 둘 다 불가능하면 해당 지시도 폐기한다.

최종 합의:

> 1.0의 장점은 기억 수나 강한 prompt 문구가 아니라 회상 폭, 재진입, 맥락 해상도,
> 사용자가 계속 선택하는 동안의 짧은 장면 초점, 기억의 행동화에서 가져온다. 단일
> storyline 고정과 과거 arc로의 강제 복귀는 장점이 아니라 제거할 결함이다. 3.6의
> 출처·시점·branch·비밀·권한·fail-open 경계는 이 기능들의 바깥 guard가 아니라 각
> 단계의 입장 조건으로 유지한다.

### 세 외부 사례 추가 교차검증

추가 검토에서는 GPT 하위 에이전트 4개가 WygLoreLeaf, OMNINODE, Living Canon을 각각 독립 분석하고, 별도 에이전트가 현재 문서·정밀 장기 기억·4.6~5.0 로드맵과의 중복을 대조했다.

공통 합의:

- graph, multi-hop, 계층 요약, 짧은 전달, source pointer, owner/knower 분리는 이미 Archive Center 쪽 계약이 더 강하거나 같은 방향이다.
- WygLoreLeaf에서 새로 강조할 것은 비권위 Host read와 확정 삭제의 분리, 격리·되돌림, 외부 호출·장기 작업의 관찰성이다.
- OMNINODE에서 새로 강조할 것은 non-expansive degraded, ranking feedback의 비권위성, all-or-zero 파생 write와 완전 rollback manifest다.
- Living Canon에서 새로 강조할 것은 truth status·assertion kind·knowledge path·epistemic state의 분리, 상태 commit barrier, offscreen present bridge, canonical fact가 아닌 locator형 회상 trace다.
- provider × role privacy 공개표와 one-subject pointer 관점은 현재 설계에 부분적으로만 명시되어 있어 문서 보강 가치가 크다.
- association edge, typed state certainty, present bridge, quarantine는 새 runtime을
  추가하기보다 3.7 이후와 4.6~5.0 계약의 수용 시험을 명확히 하는 용도로 사용한다.

이견은 human confirmation의 범위였다.

- 모든 요약·dedup에 사람 확인을 요구하면 자동 장기 기억의 비용과 지연이 커진다.
- similarity만으로 cross-source·identity·private/public merge를 실행하면 복구와 rollback이 불완전해질 수 있다.
- 합의는 exact idempotent duplicate는 자동 no-op, 위험 semantic merge만 proposal·영향 preview·review·undo 대상으로 두는 것이다.

최종 판정:

> 세 사례의 구조를 이식하지 않고 실패·feedback·mutation·rollback·privacy·epistemic exposure의 수용 조건만 가져오면 1.0의 기억 체감을 보강하면서 3.6의 안전성을 유지할 수 있다. 반대로 graph activation, agent 자유문장, LLM 직접 mutation 또는 JavaScript 원장을 가져오면 3.6보다 약해진다.

### 독립 원작 DB 방향 추가 재검토

원작 DB 독립화와 연결 강화 방향은 세 에이전트가 1.0 행동, 권한 경계, 위험·시험 관점에서 다시 검토했다.

확인된 결론:

- 특정 Archive Center 버전에 넣지 않고 별도 추후 작업으로 두는 것이 적절하다.
- 원작 DB 독립화는 1.0 감독의 대체물이 아니라 핵심 기억과 보조 자료의 분리를 강화하는 기반이다.
- 연결 상태에서 원작 DB는 후보 근거만 제공하고 Archive Center Go가 승인·예산·최종 주입을 단독 소유해야 한다.
- 핵심 기억 K를 먼저 보장하고 전체 예산이 부족하면 원작 근거를 먼저 보류해야 한다.
- Director가 최종 직접 주입 항목만 보면 1.0보다 감독 재료가 좁아진다.
- 반대로 검색 후보 전체를 보면 미검증·미전달 사실로 지휘할 수 있다.
- 합의안은 검증을 통과한 제한 보조 묶음을 Director에게 제공하고, 실제 지시에 사용한 근거가 본문 기존 레인에 없을 때만 `guidance_support`로 함께 전달하는 것이다.
- 현재 사용자 입력·명시적 수정·수용된 세션 결과가 원작 reference보다 우선한다.
- `없음`은 원작·기억 보조만 유지하고, `약함` 이상에서만 Archive Center의 단계형 응답 지휘 권한이 작동한다.

최종 판정:

> 이 경계를 모두 지키면 독립 원작 DB는 1.0의 짧은 핵심 기억, 보조 자료 분리, 넓은 기억 묶음을 읽는 감독이라는 장점을 더 안전하게 회수하는 방향이다. 독립화만 하고 감독 지원 표면을 연결하지 않으면 1.0의 감독 장점은 돌아오지 않는다.

### 파일명·버전 분할 이후 최종 교차검증

추가 교차검증은 파일명에서 3.6을 제거하고 현재 기준을 3.7로 바꾼 뒤, 역사적 결함과
현재 source 상태가 다시 섞이지 않는지 검토했다.

- 1.0은 Supervisor를 세 번 직렬 호출한 구조가 아니라 Plugin Main·Supervisor 병렬 준비,
  Sub LLM의 제한 preview 검토, 사후 Critic이 겹친 다층 역할 체인이었다.
- 새 `Recall Auditor → Director → Reviewer`는 이를 그대로 복원한 topology가 아니라,
  고정 eligible set 안에서 누락·지시·검토 책임을 분리한 안전한 후속 설계다.
- 3.7-F가 이미 compact cap, K/topK 분리, hierarchy·Persona·주관 기억 관련도,
  Supervisor 오류 분류와 HUD lineage를 source 수준에서 구현했음을 현재 상태로 반영했다.
- 3.8은 story-time·상태·관점 좌표, 4.0은 질문별 temporal anchor와 복합 recall,
  4.1은 mutation·rollback·rebuild·live gate, 4.6은 관계 graph·UI를 소유하도록
  중복 귀속을 제거했다.
- 초기 검토에서 좁은 read-only 후보로 제한했던 로어북 참조 색인은 이후 정본 로드맵의
  선택형 `4.0-G` 계획 packet으로 채택했다. 구현·실사용 검증 전에는 완료로 표시하지 않는다.
- 다층 감독의 성공 조건은 호출 수가 아니라 role별 독립 기여다. ablation에서 누락 감소,
  무근거 지시 차단 또는 의도 충돌 검출이 증명되지 않으면 `deep`을 기본화하지 않는다.

### 출판사·Recomposer 공동 사용 추가 교차검증

GPT 하위 에이전트 4개가 권한 구조, 실패·보안 시험, source-vs-design 상태와 문서 중복을
각각 독립 검토했다.

- 출판사를 전부 끄는 안은 단순하지만 Archive Center 단독 사용자와 공동 사용자의
  Recall Auditor·Director·Reviewer 기여를 함께 잃으므로 기각했다.
- 두 시스템이 각자 guidance를 직접 적용하는 안은 같은 publisher plan의 중복, 서로 다른
  계약 digest, 레일로딩과 fallback 불명확성을 만들 수 있어 기각했다.
- 합의안은 Archive Center가 publisher plan과 Recomposer input proposal을 검증해
  effective contract를 확정하고, 검증된 current-request Recomposer만 이를 변경 없이
  materialize하며 commit 뒤에만 Archive 직접 블록을 소비하는 방식이다.
- current source의 `archive_center.recomposer_enhancement.v1`은 단방향 read-only
  enhancement 기반이다. 역방향 capability claim, 입력용 Contract Judge, v2 publisher
  handoff와 transactional replacement는 아직 구현 증거가 없다.
- Recomposer가 Archive Center보다 먼저 실행한 순서의 afterRequest 결합은 post-response
  품질 향상에는 쓸 수 있지만, main 모델이 통합 pre-response contract를 받았다는 증거가
  아니다. 이 순서를 `integrated_single_contract`로 표시하지 않는다.
- 기존 문서의 “15분 만료” 설명은 producer/consumer source에 실제 expiry 좌표나 시간
  검사가 없어 철회한다. 현재 구현 증거는 session·turn·input binding, payload plan과
  lifecycle digest 검증까지로 제한한다.

최종 합의:

> 함께 사용한다는 것은 Archive Center 기억과 RisuAI Host 문맥을 별도 플러그인의 역할별
> 전처리에 연결한다는 뜻이다. 이 요청에서는 Archive Center Publisher를 자동 OFF하고,
> 메인 모델의 1차 출력 뒤 감독관·나레이터·연기자 AI가 최종문 하나를 만든다.

## 16. 폐기된 과거안 — Standalone Recomposer와 Archive Center 출판사 공동 사용 계약

> 이 절은 과거 비교 근거를 남기기 위한 기록이다. Archive Center publisher plan을 별도
> 플러그인에 넘기거나 두 전처리를 fallback으로 교대하는 아래 설계는 현재 6.1~7.0 계획이
> 아니다. 현재 정본은
> [`4.1-9.0-integrated-roadmap.md`](../../_archive/future-reference/4.1-9.0-integrated-roadmap.md)이며,
> [`4.3-character-subjective-memory-story-context-roadmap.md`](4.3-character-subjective-memory-story-context-roadmap.md)는
> 역사적 번호를 가진 상세 근거로만 사용한다.

### 목표

`Risu Recomposer.js`는 Archive Center 설치 여부와 관계없이 단독으로 작동하는 전·후처리
재작성 엔진이고, Archive Center 출판사도 Recomposer 설치 여부와 관계없이 독립적으로
완성돼야 한다. 공동 사용의 목표는 한쪽을 끄는 것이 아니라 두 역할을 한 번씩 연결하는
것이다.

- 단독 Recomposer는 RisuAI payload, 캐릭터, 페르소나, 현재 채팅, 관찰·검증 가능한
  로어북 active-or-injected 항목과 Supa/Hypa 계열 메모리 필드를 읽어 Input Planner,
  specialist, Semantic Judge, Composer와 Semantic Prover를 실행한다. activation을
  증명할 수 없는 후보는 unknown으로 유지한다.
- Archive Center 단독 경로는 Go가 기억·근거·권한을 선별하고 출판사가 만든 검증 plan을
  main payload에 직접 적용한다.
- 공동 경로는 Archive Center의 Recall Auditor·Director·Reviewer를 끄지 않는다. Go가
  승인한 `publisher_plan.v1`을 Recomposer Input Planner의 별도 비권위
  response-shaping proposal 입력으로 전달하고, Recomposer의 통합 proposal까지 Go가
  검증해 하나의 `effective_turn_contract`로 확정한 뒤 main draft와 post-response
  재작성을 같은 plan lineage로 묶는다. plan 자체는 사실 근거가 아니며 `support_refs`만
  Go가 검증한 근거를 가리킨다.
- Archive Center 연동은 Recomposer에 별도 truth·canon·memory·relationship·thread 권한을
  추가하지 않는다. 더 정밀한 장기 기억과 출판 방향을 기존 Composer/Prover 경로에
  공급하는 확장이다.

### 실행 모드와 권한축별 단일 owner

| 실행 상태 | publisher execution | main guidance materialization | final prose | 허용 설명 |
|---|---|---|---|---|
| Archive Center 미관여 | 없음 | Recomposer standalone contract | Recomposer Composer 또는 원래 RisuAI 출력 | `standalone_recomposer` |
| Archive Center 단독·Recomposer 미선택 | Archive Center Go | Archive Center direct 1회 | RisuAI main output | `archive_direct` |
| 같은 요청 pre-response 통합 성공 | Archive Center Go | Recomposer contract 1회 | proof를 통과한 Recomposer Composer | `integrated_single_contract` |
| pre-response 통합 불가, post만 안전하게 가능 | Archive Center Go | Archive Center direct 1회 | proof 성공 시 Recomposer Composer, no-change·bypass·fatal post failure 시 exact `guided_draft_zero` | `archive_direct_post_recompose` |

Archive Center가 현재 턴에 관여했는데 Recomposer 위임 claim·contract가 불일치한 경우를
Archive Center 미관여로 해석해 standalone preprocessor로 무음 전환하지 않는다.
`guidance_strength=none`은 Archive Center publisher plan 0을 뜻하며, Recomposer 자체
기능의 활성 여부를 대신 결정하지 않는다.

`guidance_strength=weak+`이고 유효한 Archive Center plan이 있으면 publisher plan은
정확히 하나이며, `none`·`base_only`·`proposal_0`·Archive Center 미관여에는 0개다. 정상 통합에는
`effective_turn_contract`가 정확히 하나이고 `archive_direct` 계열에는 Recomposer
pre-response effective contract가 0개다. 모든 모드에서 main guidance materialization
owner는 최대 하나다. `integrated_single_contract`에서는 Archive Center direct
`output_guidance`가 0이고, `archive_direct` 계열에서는 같은 plan을 Recomposer가 다시
pre-response contract로 중복 materialize하지 않는다. Recomposer의
specialist·Judge·Reviewer·Prover는 final prose를 직접 쓰지 않으며 Composer만 mutable
prose를 작성한다.

### 목표 전달 계약 — 미구현 설계

현재 v1 bridge의 의미를 바꾸지 않고 새 semantics는 별도 versioned contract로 추가해야
한다. 아래 `publisher_plan.v1`, Recomposer current-turn claim/receipt와 bridge v2 명칭은
목표 계약이며 현재 구현 완료를 뜻하지 않는다.

1. Go `prepare-turn`이 memory delivery, eligible support, Supervisor depth와
   `publisher_plan.v1`을 한 번 만들고 검증한다.
2. plan은 raw Auditor·Director·Reviewer text가 아니라 Go 승인 projection, authority
   flags, support refs와 lifecycle identity만 가진다.
3. `Archive Center.js`는 Recomposer readiness와 current-request claim을 공식 Host에서
   관찰 가능한 versioned capability로 전달할 뿐 route를 결정하지 않는다. 설치 파일명,
   prompt marker와 테스트용 전역 객체는 capability 증거가 아니다.
4. Go가 guidance route, direct fallback block ID/hash와 publisher plan digest를 정한다.
5. Archive Center adapter는 Go 결과를 재선택하지 않고 실제 RisuAI message array에 태그된
   direct fallback을 적용하고 current-turn handoff를 운반한다.
6. Recomposer Input Planner는 standalone fragment와 같은 publisher plan을 읽고, 향후
   입력용 Contract Judge는 충돌·중복·role projection에 대한 통합 proposal만 낸다.
7. Recomposer는 immutable publisher plan을 변경하지 않고 Planner·Judge proposal과
   conflict trace를 Archive Center Go에 반환한다.
8. Go contract compiler가 current input, hard guard, plan identity, role visibility와 ref
   coverage를 검사해 단 하나의 `effective_turn_contract`를 확정한다.
9. Recomposer는 태그된 Archive fallback 제거와 Go 확정 contract 1회 삽입을 하나의
   message-array replacement로 수행한다. 교체 뒤 실제 배열을 다시 읽어 fallback 0개,
   effective contract 1개, request identity와 hash 일치를 모두 확인한 경우에만
   pre-dispatch commit receipt를 발행한다.
10. claim·plan·proposal·contract·commit receipt는 같은 session, backend request, logical/source
   turn, branch, generation, revision, raw/effective input digest, payload plan,
   publisher plan, source snapshot과 유효 범위를 가리켜야 한다.

입력용 Contract Judge는 현재 존재하는 post-response Semantic Judge와 다른 역할이다.
Contract Judge도 최종 정책 owner가 아니며 Archive Center hard guard, eligible set 또는
publisher plan을 변경할 수 없다. Go가 승인한 publisher projection과 Recomposer Planner
fragment의 결합·drop·role projection은 Archive Center Go contract compiler가 최종
소유한다. Go round-trip과 새 contract 발행이 완료되지 않으면 통합 경로를 abort한다.

### 원자적 fallback과 load order

guidance 위임은 terminal state가 `committed|aborted` 중 정확히 하나인
consume-on-commit transaction이다. immutable effective contract와 mutable transaction
trace를 분리한다. trace에는 `commit_attempt`, 교체 후 검증 실패, rollback과 최종
`aborted`가 함께 남을 수 있지만 terminal state 둘을 동시에 성공 상태로 기록할 수 없다.

- Input Planner·Contract Judge·Go compiler·commit observation 중 하나라도 실패하면
  Recomposer는 부분 contract를 버리고 message array를 입력 snapshot의 태그된 Archive
  fallback 1개·effective contract 0개 상태로 복원한 뒤 `aborted`로 끝낸다.
- fallback은 Publisher 재호출, 이전 성공 plan, default prose, 더 넓은 eligible support나
  별도 standalone contract를 사용하지 않는다.
- main payload dispatch 뒤에는 Archive direct plan을 추가한 두 번째 요청으로 소급
  fallback하지 않는다.
- 필수 Judge·Composer·Prover·repair 실패, 후보 quorum 부족 또는 최종 pipeline
  reject에는 부분 rewrite나 cache 후보가 아니라 main 요청에서 실제 guidance 적용이
  관찰된 정확한 `guided_draft_zero`를 반환한다. 선택적 specialist 실패는 해당 lane을
  폐기하고도 남은 후보의 독립성, contract coverage와 Judge·Composer·Prover 통과를
  증명한 경우에만 degraded success가 될 수 있다. guidance 적용이 관찰되지 않은 원문은
  `unguided_draft_zero`이며 guided 성공으로 표시하지 않는다.
- Recomposer beforeRequest callback이 같은 요청에서 Archive Center callback이 적용한
  태그 fallback과 handoff를 실제로 전달받은 경우만 `integrated_single_contract` 후보로
  인정한다. 플러그인 설치·등록 순서만으로 callback 관찰 순서를 추정하지 않는다.
- Recomposer-first 순서에서 afterRequest가 Archive 봉투를 다시 읽는 현재 기능은
  post-response 보강 근거일 뿐 통합 pre-response contract의 증거가 아니다. 이 순서는
  future capability/ordering 계약이 없으면 `archive_direct_post_recompose` 또는
  `archive_direct`로 정직하게 기록한다.
- `archive_direct_post_recompose`는 Recomposer가 Archive 관여를 pre-dispatch에 확인해
  competing standalone pre-contract를 주입하지 않고 Archive direct plan만 받은 draft를
  후처리한 경우에만 허용한다. 이 억제를 증명할 capability가 없으면 공동 모드로 표시하지
  않고 `archive_direct`를 유지한다.

### 단일 effective contract에서 사용하는 Archive Center 레인

| Go 전달 표면 | Recomposer 의미 | 사용 위치 |
|---|---|---|
| `event_recent` | 객관적 사건 기억 | established facts, continuity |
| `character_objective` | 객관적 인물 상태 | established facts, character/plot |
| `subjective_relationship` | 특정 인물 관점의 주관 기억 | relationship state, character/plot |
| `world_state` | 객관적 세계 상태 | established facts, plot/world continuity |
| `protected_secret` | 작가만 아는 비밀 | writer-only secret, knowledge/POV guard |
| `unresolved_goal` | 미해결 목표·실마리 | unresolved hooks, scene direction |
| `direct_evidence` | 이전에 수용·검증된 직접 근거 | grounded facts, continuity |
| guidance trace / future `publisher_plan.v1` | 현재 턴의 Go 승인 출판 제안 | response directives, plot/style |

현재 `memory_delivery_plan.v1`의 classes에는 `output_guidance`가 없고 Supervisor 지침은
별도 `guidance_application_trace`에서 Recomposer lane으로 한 번 추가된다. future memory
class와 guidance trace 두 경로에 같은 guidance를 동시에 추가하지 않는다.

주관 기억은 객관 사실로 승격하지 않는다. `protected_secret`은 등장인물 지식이 아니라
writer-only constraint다. Publisher plan은 `truth_authority=false`,
`canonical_write=false`, `user_action_authority=false`,
`relationship_transition_authority=false`, `closure_authority=false`인 response-only
proposal이다. Recomposer는 plan을 established facts, character-visible knowledge,
required canonical outcome 또는 저장 근거로 복사하지 않는다.

canonical effective contract가 하나여도 외부 role 호출에는 Go data class와 role
permission에 따른 projection만 전달한다. protected/private raw text는 publisher plan,
bridge, pluginStorage, durable trace와 HUD에 넣지 않는다. redaction 뒤 근거가 사라진
directive는 public/default 문구로 대체하지 않고 함께 폐기한다.

### post-response final과 Critic admission

Archive Center Critic은 동일 afterRequest에서 Recomposer 문장을 다시 쓰는 역할이 아니다.
이번 턴에 사용할 수 있는 Critic 계열 자료는 이전에 수용·검증되어 `direct_evidence`에
들어온 근거뿐이고 `same_turn_critic_result_available=false`를 유지한다.

Recomposer가 current turn을 claim한 경우 Archive Center는 중간 afterRequest
`draft_zero`를 complete-turn·Critic·memory admission의 최종값으로 확정하지 않는다.
목표 계약은 Recomposer가 turn coordinates, draft hash, returned output hash와 결과
상태를 가진 final receipt를 게시하고, Archive Center가 공식 displayed-final observation과
receipt가 일치할 때 visible output 하나만 제출하는 것이다. receipt나 displayed-final을
확인할 수 없으면 사용자 응답은 fail-open하되 canonical write와 Critic admission은
deferred/read-only로 둔다.

다음 항목은 memory evidence가 아니다.

- Input Planner·Recall Auditor·Director·Reviewer·Contract Judge raw text
- `draft_zero`와 specialist 후보
- Semantic Judge·Composer 실패본과 Prover 거부본
- publisher plan 또는 해당 plan이 승인됐다는 사실 자체

### 현재 source에 구현된 기반

현재 source가 증명하는 범위는 다음과 같다.

- Go는 `archive_center.recomposer_enhancement.v1`을 read-only optional enhancement와
  standalone fallback requirement로 생산한다.
- `Archive Center.js`는 Go payload 적용 관찰 뒤
  `archive_center.recomposer_bridge.v1` transient 봉투를 게시한다.
- Recomposer는 계약 버전, Go owner, read-only·optional 상태, session·turn, input
  binding, payload plan과 lifecycle digest를 검증한다.
- Recomposer는 Archive memory classes와 별도 guidance trace를 typed lane과 Draft
  Ledger에 결합한다.
- Recomposer beforeRequest는 세 Input Planner fragment를 `fuseTurnContract`로
  결정론적으로 합쳐 자체 `turn_contract.v1`을 주입한다.
- Recomposer afterRequest는 Archive 봉투를 다시 확인해 context와 output rewrite
  lineage에 붙이고, specialist·Semantic Judge·Composer·Semantic Prover 경로를 실행한다.
- 기존 beforeRequest snapshot과 settings에 결합된 strict output reuse와 원본 fail-open
  기반은 존재한다. 그러나 publisher-plan-aware identity와 통합 이후 cache lookup은
  구현되지 않았다. 현재 Recomposer-first 경로는 Archive enhancement 통합보다 먼저
  cache를 확인한다.

현재 구현의 transient 봉투에는 실제 `published_at_ms`·`expires_at_ms`가 없고 reader도
시간 만료를 검사하지 않는다. 기존 “15분 수명” 설명은 source와 맞지 않으므로 삭제한다.
현재 수명 증거는 session·turn·input·payload-plan·lifecycle binding과 다음 요청·unload의
bridge clear 동작까지로 제한한다.

### 공동 사용 목표 중 아직 구현되지 않은 항목

| 항목 | 상태 |
|---|---|
| Go enhancement contract와 고정 lane semantics | `SOURCE_IMPLEMENTED` |
| Archive Center→Recomposer one-way bridge | `SOURCE_IMPLEMENTED` |
| Recomposer envelope 검증·typed ledger 결합 | `SOURCE_IMPLEMENTED` |
| 관련 Go/JS v1 fixture 회귀 | `SOURCE_TESTED_V1_ONLY` |
| 실제 양쪽 load order·DB·Supervisor 연동 | `LIVE_HOST_UNVERIFIED` |
| publisher plan 전용 versioned projection | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| Recomposer→Archive current-request capability/claim | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| 입력용 Contract Judge proposal과 Go effective contract compiler | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| pre-dispatch commit receipt·transactional fallback replacement | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| publisher-plan-aware output reuse identity와 통합 뒤 cache lookup | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| guided/unguided draft provenance | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| Archive data class provider별 role privacy projection | `DESIGN_ONLY_NOT_IMPLEMENTED` |
| final output receipt와 displayed-final-only Critic admission | `DESIGN_ONLY_NOT_IMPLEMENTED` |

현재 `archive_center.recomposer_enhancement.v1`은 **Archive Center 선택 정보를 Recomposer
자체 계약과 rewrite ledger에 붙이는 optional read-only enhancement**다. 위 공동 사용
목표는 v1의 완료 기능으로 소급 해석하지 않는다. 특히 잘못된 bridge를 standalone으로
무음 처리하는 현재 fail-open은 단독 Recomposer의 기존 동작 증거이지, Archive 위임을
claim한 뒤 정확한 Archive fallback으로 돌아가는 새 transaction의 증거가 아니다.

### 검증 증거와 남은 실사용 확인

2026-07-30 소스 기준으로 기존 v1 기반에 대해 기록된 증거:

- `Archive Center.js` 문법 검사 통과
- `Risu Recomposer.js` 문법 검사 통과
- Recomposer core/batch/input 회귀군 `63/63` 통과
- 기존 Recomposer enhancement 계약 Go 테스트 통과
- `go test ./internal/httpapi` 통과

이 결과를 새 공동 사용 계약의 검증으로 재해석하지 않는다. 실제 RisuAI에서 두 플러그인의
로드 순서를 각각 바꾼 실행, current-request claim과 exactly-one guidance route,
MariaDB/ChromaDB의 실제 주관·장기 기억 전달, Publisher 사용, pre/post failure별 정확한
fallback, displayed-final receipt와 다음 턴 Critic 회수는 아직 loaded-artifact 또는
live-host 증거가 아니다.

## 17. 범위와 증거 수준

이번 검토는 소스·테스트·로드맵의 읽기 전용 분석이다.

- 런타임 코드는 변경하지 않았다.
- JavaScript 런타임 증감: +0 / -0
- Go 런타임 증감: +0 / -0
- HAYAKU v2.3.17은 로컬 압축 소스의 구조·문자열·호출 연결과 JavaScript 문법만 정적으로 확인했으며 실제 RisuAI 장기 대화 적중률·패킷 준수율은 검증하지 않았다.
- WygLoreLeaf 3.0.0과 OMNINODE beta18은 JavaScript 문법과 정적 소스 구조만 확인했고, Living Canon preset은 JSON parse와 prompt·tool·memory 계약만 확인했다.
- 세 외부 사례의 실제 장기 대화 적중률, 비용, latency, rollback 복구율과 RisuAI host별 동작은 검증하지 않았다.
- 외부 사례의 함수·prompt·schema·저장 키·임계값·agent 수는 Archive Center 구현 근거로 복사하지 않았다.
- 실제 RisuAI, loaded artifact, MariaDB, ChromaDB 검증은 수행하지 않았다.
- RisuAI 로어북 검토는 공식 `main`
  `da9c514017dc254795bf5c306da5225a65791da4`의 plugin DB access, loreBook type와
  lorebook activation source를 읽은 설계 검토다. 실제 동기화·검색·중복 억제·수정/삭제
  lifecycle은 구현하거나 live 검증하지 않았다.
- 4.6~5.0은 계획 문서이므로 현재 구현처럼 표현하지 않는다.

따라서 이 문서는 구현 방향과 acceptance criteria의 근거이며, 실제 품질 개선의 완료 증거는 아니다.
