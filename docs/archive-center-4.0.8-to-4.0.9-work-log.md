# Archive Center 4.0.8 → 4.0.9 통합 작업 기록

기록 기준: 2026-08-29 KST  
범위: 4.0.8 공개 뒤 접수된 피드백부터 현재 4.0.9 테스트 소스까지  
증거 구분: 소스·자동 테스트는 확인됨, 실제 사용자 DB·Provider·RisuAI 재검증은 별도

## 1. 피드백과 처리 결과

### 1.1 사용자 입력만 삭제된 채팅의 콜드 스타트 누락

피드백:

- 원본 메시지는 179개인데 입력·출력이 모두 남은 15턴만 콜드 스타트 후보가 됐다.
- `active raw` 수치는 맞아도 입력 없는 assistant 출력은 파생 기억이 생성되지 않았다.

처리:

- assistant 출력 자체를 턴 관측 기준으로 삼고 `paired`,
  `stored_pair_recovered`, `assistant_only`를 Go가 판정한다.
- DB에 원래 입력이 남아 있으면 LLM 없이 재사용하고, 없으면 명시적
  정상화에서만 assistant-only Critic을 실행한다.
- 가짜 사용자 입력을 만들지 않으며 출력에서 확인되는 파생 항목만 각각 저장한다.
- 한 항목이 부족해도 다른 정상 항목을 폐기하지 않는다.

### 1.2 36~38턴 삭제가 3턴부터 rollback된 문제

심각도: `critical` — 정상적인 화면 삭제를 더 이른 턴의 삭제로 오판해
해당 지점 이후의 원문 계보·파생 기억·Vector를 대량 무효화할 수 있는 데이터
수명주기 결함이다.

피드백:

- JavaScript 누적 turn counter와 화면 assistant 개수의 차이가 앞쪽 보정으로
  오해돼 36~38번 삭제가 3턴부터의 대규모 삭제로 변했다.
- Critic timeout이 counter drift를 쌓아 문제를 증폭했다.

처리:

- `trackedTurnIndex - assistantMessageCount` 계산을 canonical 삭제 기준에서 제거했다.
- 실제 사라진 assistant source revision의 canonical turn을 Go가 비교한다.
- 사용자 입력만 사라지고 assistant 출력이 남으면 턴·기억·Vector를 유지한다.
- Critic 실패나 timeout은 턴 증가·삭제 근거로 쓰지 않는다.

### 1.3 리롤·삭제 뒤 진행 HUD 정지

피드백:

- 원문·파생 저장은 끝났는데 HUD가 `본문 응답 기다리는 중` 또는
  `기억 복구 중`에 남았다.
- 저장된 retry 시각이 지나도 새 외부 이벤트가 없으면 worker가 다시 실행되지 않았다.

처리:

- Go workflow ledger의 실제 request ID를 HUD 조회 기준으로 사용한다.
- 재처리 worker는 durable `retry_after` 시각에 one-shot으로 다시 깨어난다.
- 백엔드가 재시작돼도 MariaDB에서 가장 이른 미래 재시도·lease 만료 시각을
  다시 읽어 기존 one-shot timer를 복원한다.
- 시도 수·최대 시도·다음 시각·scheduled/exhausted를 HUD ViewModel에 남긴다.
- 마지막 자동 시도 실패 시 무한 초록 상태가 아니라 실패와 수동 재처리로 끝난다.

### 1.4 NeuralWatt 524 뒤 1초 재호출

피드백:

- Cloudflare 524 응답이 `retry_after: 120`을 주었지만 자동 Critic 재처리가
  1초 간격으로 이어졌다.

처리:

- Critic 자동 재처리 기본 간격 설정을 추가했다. 기본 30초, 범위 1~3600초다.
- Go가 설정 간격과 Provider 대기 지시 중 긴 값을 해당 작업에 기록한다.
- `Retry-After` 헤더와 JSON `retry_after`를 provider 이름과 무관하게 읽는다.
- 잘못된 힌트는 힌트만 무시하고 설정 간격을 사용한다.
- Vector·DB 큐와 Provider 자체 호출 횟수 정책은 바꾸지 않았다.

### 1.5 `CRITIC_OUTPUT_TOKEN_EXHAUSTED` 진단 부족

피드백:

- 토큰 소진 오류 이름만 보이고 실제 요청 한도, 종료 사유, 추론 토큰을 알 수 없었다.

처리:

- provider response ledger에 native finish reason과 요청한 출력 한도를 보존한다.
- 실제 턴 HUD와 자동 복구 HUD가 가능한 토큰 필드를 각각 독립 표시한다.
- 한 필드가 없다고 다른 토큰 정보나 오류 본문을 지우지 않는다.
- 토큰 소진·잘린 JSON은 합성 저장하지 않고 기존 재처리 계약을 유지한다.

### 1.6 풀네임·애칭이 서로 다른 인물로 갈라지는 문제

피드백:

- 같은 인물이 `아벨슈타인`과 애칭 `아벨`로 각각 저장돼 인물 카드가 둘로
  갈라졌다.
- 26턴의 새 무기 상태가 풀네임 쪽에만 남아, 28턴에 애칭으로 불린 인물이
  이전 상태를 읽을 가능성이 있었다.
- 사용자가 인물 탭에서 별명·애칭을 확인하고 직접 같은 인물로 합치거나
  잘못 합친 연결을 해제할 방법이 필요했다.

처리:

- 사용자가 대표 인물과 연결할 인물을 직접 고르는 미리보기·연결·해제 API를
  Go 백엔드에 추가했다.
- 병합은 기존 `entity_identity_links`의 검토된 `canonical_equivalence` 연결만
  저장한다. 기존 인물 상태·사건·관계 지식·주관 기억·원문·벡터는 삭제하거나
  일괄 재작성하지 않는다.
- 인물 상태·사건, 관계 지식, 장비·물품, 주관 기억, 이름 표면의 영향 건수를
  lane별로 미리 보여준다. 한 lane을 읽지 못해도 읽은 lane의 수치를 버리지 않는다.
- 연결 후보 하나가 더 이상 활성 상태가 아니거나 저장에 실패해도 다른 정상
  후보의 연결은 유지하고 후보별 결과를 반환한다.
- 검토된 연결은 A→B→C 형태도 최종 대표 인물로 수렴하며, 연결 해제는 사용자가
  지정한 직접 연결 하나만 비활성화한다.
- 인물 카드에서 `별명·애칭` 목록을 표시하고 `다른 인물과 합치기`, `별명 관리`를
  제공한다. 이름 유사도만으로 자동 병합하지 않는다.
- 인물 카드, 신규 평론가 저장, 주관 기억 조회, 관계 지식·물품 조회, 본문 기억
  조립은 같은 Go 동일성 해석기를 사용한다. JavaScript는 선택·요청·표시만 담당한다.

범위 경계:

- DB 스키마와 테이블을 추가하지 않았다.
- 기존 주관 기억 강제 합치기 기능을 변경하지 않았다.
- 평론가 재호출, 자동 재색인, 이름 유사도 기반 자동 병합, 다른 세션 병합을
  추가하지 않았다.

## 2. 유지한 경계

- Go가 턴 판정, 삭제 범위, Critic 재처리 시각, HUD ViewModel을 소유한다.
- `Archive Center.js`는 host 관측, 설정 전달, 화면 표시만 담당한다.
- 기존 raw/derived/source revision/outbox를 자동 삭제하지 않는다.
- 새 DB 테이블, 새 fallback queue, polling watcher, 모델 allowlist를 추가하지 않는다.
- 재시작 복원은 기존 MariaDB queue의 예약 시각 조회이며, 별도 queue나
  server-lifetime polling을 만들지 않는다.
- 하나의 누락·오류를 이유로 다른 정상 파생 항목을 전체 탈락시키지 않는다.
- branch·복사·CID가 다르다는 이유만으로 다른 세션을 자동 병합하지 않는다.

## 3. 자동 검증

- assistant-only 관측·복구·멱등성
- 36~38 삭제 시 `from_turn=36`
- 사용자 입력만 삭제된 턴 유지
- branch·복사 세션 source 범위 격리
- 재처리 완료·terminal HUD 전환
- Provider retry hint 숫자·문자열·헤더·오류 본문 보존
- 설정 간격과 Provider 힌트의 긴 값 선택
- DB에 저장된 미래 재시도 시각의 worker 재시작 복원
- 토큰 진단 필드별 독립 보존
- JavaScript 설정 저장·Go runtime sync marker
- 아벨슈타인/아벨 병합 전 두 카드와 병합 후 한 카드·별명 표시
- 병합 뒤 애칭 검색·신규 저장·주관 기억·KG 읽기의 대표 인물 수렴
- 기존 상태·사건·KG·주관 기억 행 수 불변과 연결 해제 복원
- A→B→C 연결 수렴과 지정한 직접 연결만 해제
- 다른 세션 인물 연결 차단과 정상 후보별 부분 성공
- 미리보기 lane 하나 실패 시 다른 lane 결과 보존

현재 자동 검증:

- `go test ./internal/httpapi -count=1`: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 통과
- 번들 Node `--check Archive Center.js`: 통과
- 전체 `go test ./... -count=1`: 통과

## 4. 실환경에서 남은 확인

- 실제 NeuralWatt 524/429에서 HUD가 Provider 지정 시간까지 기다리는지
- 연속 실패 시 시도 수가 증가하고 마지막에 수동 재처리로 끝나는지
- token exhausted 응답에서 Provider가 실제 usage를 제공한 필드가 HUD에 보이는지
- 179개 메시지 사용자 DB 정상화를 두 번 실행해 중복 저장되지 않는지
- 실제 36~38 삭제에서 MariaDB·Chroma가 36턴 이후만 무효화하는지
- 실제 장기 세션에서 풀네임·애칭 연결 전후의 본문 기억 주입과 신규 평론가 저장
- 실제 MariaDB·ChromaDB에서 병합 전후 기존 행·벡터 수가 변하지 않는지
- 실제 RisuAI 인물 탭에서 미리보기·연결·해제·별명 표시가 의도대로 보이는지

## 5. 테스트 패키지

기존 4.0.9 테스트 패키지와 같은 위치에서 정식 빌더로 갱신했다. 새 이름의
병렬 패키지를 만들거나 패키지 내부 파일을 손으로 고치지 않았다.

- source commit: `435e4e5ab58cf5bfa25421dfcbdec20e5344baf3`
- package source dirty: `false`
- package status: `green`, `release_ready=true`
- `automatic_update_apply=true`, `direct_update_supported=true`
- source/package `Archive Center.js` SHA-256:
  `4ab240dac0a0aed89cffaff080109db441545bc13186372498597feb0c3c1d47`
- ZIP size: `11,990,050 bytes`
- ZIP SHA-256:
  `c42fa27e6de9f179bc95cbc58c25f86c42dcd7f09954206376fe841d89c08054`
- 외부 checksum과 실제 ZIP hash 일치
- 기존 테스트 패키지의 `.env.full.local`은 빌드 전후 SHA-256
  `ec1e29c260549b2ff7475d23c32af9406deb22ccb370ccb40d671b32bb920cc2`로
  동일하게 복원했다.

패키지·소스 동일성과 자동 업데이트 manifest는 확인됐다. 갱신된 패키지를
실제 RisuAI에 로드한 HUD, Provider, MariaDB/Chroma 동작은 별도 실환경 검증이다.

## 6. 4.0.8 중대 데이터 무효화 사고 기록

### 6.1 사용자 체감

접수된 표현은 다음과 같다.

> 아카이브 센터 초기화 뭔데 크아아악
>
> 갑자기 저장된 정보들 다 날라가서 놀랐네
>
> 다시 정상화 드간다

이 표현만으로 실제 `전체 DB 초기화`, 잘못된 자동 rollback, CID·세션 매핑
분리를 하나로 단정해서는 안 된다. 다만 사용자가 전체 DB 초기화 확인 문구를
직접 입력하지 않았고 `auto_rollback` 감사 기록의 `from_turn`이 실제 삭제 턴보다
이르다면, 4.0.8의 잘못된 rollback 기준점 결함과 일치한다.

### 6.2 확인된 4.0.8 원인

4.0.8 JavaScript의 `buildRollbackTurnLedgerOr1f`는 다음 차이를 현재 화면 앞쪽의
숨은 완료 턴 수로 취급했다.

```text
completedTurnFloor = trackedTurnIndex - assistantMessageCount
```

Critic timeout·저장 실패 등으로 누적 counter가 증가했지만 assistant 출력 수가
같이 증가하지 않으면 이 차이가 drift로 누적됐다. 이후 실제 36~38턴 출력을
삭제해도 클라이언트 ledger는 삭제 시작점을 3턴처럼 앞당겨 보낼 수 있었다.
그 결과 잘못된 이른 턴부터 source revision과 파생 자료가 무효화돼 UI에서는
저장 정보가 거의 전부 사라진 것처럼 보일 수 있었다.

이것은 단순 표시 문제가 아니다. 잘못된 rollback이 승인되면 해당 범위의
기억·직접 근거·KG·상태·Vector 수명주기에 영향을 줄 수 있으므로
`critical` 결함으로 유지한다.

### 6.3 다른 초기화 기능과의 구분

- `전체 DB 초기화`: 디버그 UI에서 별도 경고를 확인하고 `전체 DB 초기화`라는
  문구를 정확히 입력한 뒤에만 `/admin/database-reset`을 호출한다. 실행되면
  MariaDB application row와 ChromaDB Vector가 실제 삭제된다.
- `세션 라우팅 상태 초기화`·`현재 세션 매핑 강제 초기화`: pin·alias migration·
  runtime cache를 다시 잡지만 DB row를 삭제하지 않는다. CID가 갈리면 기존
  자료가 현재 화면에서 사라진 것처럼 보일 수 있다.
- 잘못된 `auto_rollback`: 사용자가 전체 DB 초기화를 확인하지 않아도 발생할 수
  있었던 4.0.8 결함이다. `from_turn`, request source, source revision 감사 기록으로
  판별한다.

### 6.4 4.0.9 수정과 검증 상태

- `trackedTurnIndex - assistantMessageCount` 값을 canonical 삭제 기준에서 제거했다.
- JavaScript의 턴 수치는 host 관측 순서 힌트로만 전달한다.
- Go가 현재 보이는 assistant 출력과 기존 source revision을 직접 비교해 실제로
  사라진 가장 이른 canonical 턴을 결정한다.
- 사용자 입력만 사라지고 assistant 출력이 남은 턴은 삭제하지 않는다.
- Critic timeout·저장 실패는 턴 증가나 삭제의 증거로 사용하지 않는다.
- 회귀에서 잘못된 클라이언트 기준점 `3`을 보내도 실제 삭제된 source revision의
  시작점 `36`으로 교정되는 것을 확인한다.

현재 증거 수준:

- 현재 소스와 회귀 테스트: 확인됨
- 4.0.9 Windows 테스트 패키지 포함: 확인됨
- 문제가 발생한 실제 사용자 DB 복사본에서 동일 장기 세션 재현: 미확인
- 실제 MariaDB·ChromaDB에서 36턴 이후만 무효화되는지: 미확인

따라서 소스 수정을 `완료`로 기록하되, 실제 사용자 환경까지 완전히 닫힌 것으로
과장하지 않는다. 같은 제보가 다시 들어오면 다음 세 자료를 먼저 받는다.

1. Activity/Historical Queue의 `auto_rollback`, `from_turn`, 발생 시각
2. 전체 DB 초기화 완료 알림 또는 `/admin/database-reset` 호출 여부
3. 세계선의 기존 세션 잔존 여부와 현재 CID·기존 CID

## 7. 인물·물품 동일성 연결 후속 작업

### 7.1 인물 동일성 연결

- 대표 인물과 연결할 인물을 사용자가 직접 선택한다.
- 검토된 `canonical_equivalence` 연결만 저장하며 기존 원문·기억·상태·KG·
  주관 기억·Vector를 삭제하거나 일괄 재작성하지 않는다.
- 인물 카드에서 대표 이름과 별명·애칭을 함께 보여준다.
- 합치기 미리보기는 lane별 영향 건수를 반환하며, 한 lane을 읽지 못해도 다른
  정상 lane의 결과를 폐기하지 않는다.
- 연결 해제는 사용자가 지정한 직접 연결 하나만 비활성화한다.
- 인물 연결 목록은 기본으로 접고 `연결 목록 N개`를 펼쳤을 때만 상세 행과
  연결 해제 버튼을 보여준다.
- 대상·후보 선택 같은 로컬 UI 조작에서는 전체 Explorer 자료를 다시 요청하지
  않고 현재 화면만 다시 그린다. 서버 호출은 미리보기·저장·해제 시점에만 한다.

### 7.2 물품 동일성 연결

- 인물과 물품의 동일성 목록을 분리했다. 인물 화면에는 인물 링크만, 물품
  화면에는 물품 링크만 표시한다.
- `/items/{session}` 읽기와 물품용 미리보기·연결·해제 경로를 추가했다.
- `맑은 이슬 → 중급 소주`처럼 사용자가 같은 물품이라고 확인한 경우 대표
  물품과 별칭으로 수렴해 조회한다.
- 다른 종류의 개체를 물품에 연결하려는 후보는 그 후보만 실패로 반환한다.
  같은 요청의 정상 물품 후보는 유지한다.
- 기존 KG row는 다시 쓰거나 삭제하지 않고 동일성 연결을 읽기 투영에 적용한다.
- 평론가 재호출과 자동 Vector 재색인은 수행하지 않는다.
- 물품 연결 목록도 기본으로 접힌 상태로 표시한다.
- 기존 JavaScript의 KG 200건 재조회·물품 추출을 제거하고 Go의 `/items` 결과를
  표시하도록 바꿨다. 인물·물품·세계 규칙·주관 기억 요청은 병렬로 유지한다.

### 7.3 검증과 남은 실환경 확인

2026-08-29 현재 다시 실행한 회귀:

- `TestCharactersGetShowsOnlyCharacterIdentityLinks`: 통과
- `TestItemIdentityMergeStaysItemOnlyAndConvergesReadProjection`: 통과
- `TestItemsGetDoesNotExposeCharacterIdentityLinks`: 통과
- `TestRollbackDecisionHandlerIgnoresPoisonedCounterAnchorAndUsesDeletedSourceTurn`: 통과
- 번들 Node `--check Archive Center.js`: 통과

현재 동일성 UI 작업의 JavaScript 변경량은 기준 commit `2fef745` 대비
`+160 / -32`다. 이 증가는 물품 UI, 연결 목록 접기, 로컬 선택 시 불필요한
Explorer 재요청 제거에 사용됐으며 턴 판정·병합 정책·DB 저장 정책을
JavaScript에 추가하지 않았다.

남은 확인:

- 실제 장기 세션에서 인물 합치기 선택 반응 속도
- 실제 MariaDB에서 인물·물품 연결 전후 기존 행 수 불변
- 실제 기억 주입에서 애칭과 물품 별칭이 대표 ID 자료를 함께 읽는지
- 연결 해제 후 기존 두 개체 표시가 정상 복구되는지

## 8. 최신 4.0.9 테스트 패키지 기록

앞의 5절은 인물 동일성 연결 직후의 깨끗한 source snapshot이다. 물품 동일성
연결까지 포함해 같은 테스트 패키지 위치에서 다시 갱신한 최신 기록은 다음과
같다.

- generated at: `2026-08-29T10:15:50Z`
- source commit: `2fef745871de97b7099cd1fb52906bdf0c5585ee`
- package source dirty: `true` — 물품 동일성 후속 변경이 아직 별도 commit으로
  고정되기 전 빌드였음을 뜻한다.
- package status: `green`, `release_ready=true`
- `automatic_update_apply=true`
- source/package `Archive Center.js` SHA-256:
  `f11e1f72253e201e958ee94673badefe7c4bb08a3e73cc7fe877f7d48ef411d3`
- ZIP size: `12,222,910 bytes`
- ZIP SHA-256:
  `987df2eae457c9820912d35ee21d91d323fbf124c7ba3d82a2806a35b5335fbf`

패키지 안의 JavaScript에 인물·물품 연결 화면과 `/items` 호출 표식이 포함되고
소스 JavaScript와 hash가 일치하는 것은 확인했다. `green` 패키지는 빌드·manifest
검증 결과이며, 실제 사용자의 RisuAI·MariaDB·ChromaDB 동작을 대신 증명하지
않는다.

## 9. 삭제 인식·관계 지식 지연·누락 인물 ID 후속 수정

### 9.1 접수된 피드백

- 한 턴을 생성한 뒤 출력을 삭제했지만 세계선을 다시 열어도 삭제가 인식되지
  않았다.
- 관계 지식 탭은 처음뿐 아니라 탭을 왕복할 때마다 약 5초씩 걸렸다. 같은
  세션의 canonical KG 672건 자체는 약 25ms에 조회됐다.
- 인물 상태 카드 중 일부는 `stable_entity_id`가 없어 `다른 인물과 합치기`와
  별명 관리 버튼이 표시되지 않았다.

### 9.2 삭제 인식 결함과 수정

4.0.9 중간 작업에서 입력 훅과 `beforeRequest` 시작점에 rollback 확인을 추가한
것이 원인이었다. 출력 생성 전의 사용자 입력만 있는 상태가 호스트 서명으로
저장됐고, 출력 생성 뒤 그 출력이 삭제되어 같은 상태로 돌아오면 세계선 첫
로드도 이미 확인한 서명으로 오판해 백엔드 판정을 건너뛰었다. 백엔드 연결
실패 때에도 서명과 snapshot을 소비해 다음 새로고침 재시도가 막혔다.

이후 `beforeRequest` 대조 자체를 제거한 것은 올바른 최종 해결이 아니었다.
그 상태에서는 세계선을 열기 전에 `출력 삭제 → 즉시 리롤 또는 다음 요청`을
실행하면 삭제된 source revision이 본문 기억 조립에 남을 수 있었다. 또한 진행
상태가 전역 Promise와 boolean 하나였기 때문에 A 세션의 대조가 진행되는 동안
B 세션의 대조가 생략될 수 있었다.

수정 내용:

- 입력 훅에서는 삭제 대조를 실행하지 않는다.
- `beforeRequest`에서는 Go의 현재 입력 판정이 `eligible`로 확정된 뒤, 런타임
  설정 동기화와 전체 `/prepare-turn`보다 먼저 정확히 한 번 대조한다.
- 세계선 첫 로드에서도 같은 대조 경로를 유지한다.
- JavaScript는 전체 활성 채팅에서 assistant 메시지 ID·generation ID·내용 hash·
  위치와 최종 상태만 관측해 보낸다. 삭제 시작 턴은 계산하지 않는다.
- Go가 현재 active source revision과 관측값을 직접 비교해 실제로 사라진 가장
  이른 canonical 턴만 결정한다.
- 4.0.2의 요청 직전 인식 시점은 복원하지만,
  `trackedTurnIndex - assistantMessageCount`와 snapshot·ledger 기반 삭제 턴 추정은
  복원하지 않는다.
- 이전 호스트 상태와 같다는 이유로 순차 판정을 생략하던 두 서명 캐시는
  제거했다. 출력 생성 전 상태로 돌아오는 즉시 리롤과 같은 턴 재생성 후 재삭제도
  매 명시적 신호마다 Go에서 다시 검증한다.
- 동시에 겹친 같은 세션 호출만 세션별 Promise로 합친다. 서로 다른 A·B 세션은
  각자 고정된 session ID와 Host 좌표로 독립 처리하며 서로의 결과를 빌리지 않는다.
- 판정 또는 DELETE가 실패해도 성공 상태로 소비하지 않으며 다음 명시적 신호에서
  다시 시도할 수 있다.
- `beforeRequest`와 세계선 갱신은 각각 `before_request_*`,
  `worldline_refresh_*`로 진단을 구분한다.
- 새 queue, timer, watcher, 자동 삭제 fallback은 추가하지 않았다.

회귀는 요청 단계 순서, 출력 유지, 사용자 입력만 삭제, 출력 삭제, 첫 판정 연결
실패 뒤 재시도, 출력 생성 전과 같은 모양으로 돌아오는 즉시 리롤, 같은 턴을
재생성한 뒤 다시 삭제, A/B 세션 동시 대조를 실제 JavaScript 함수로 검증한다.
Go 회귀는 36~38턴 삭제의 시작점을 36으로 교정하고 기존 `/rollback/decision`의
source revision 비교와 one-use decision token을 그대로 사용한다.

### 9.3 관계 지식 지연 수정

원인은 Explorer가 KG 전체를 정렬하기 전에 672개 관계의 subject/object를 각각
`ResolveUniqueActiveEntityIdentityBySurface`로 조회한 것이었다. 해당 resolver의
DB 조회와 상태 fallback이 KG 건수에 비례해 반복됐다.

수정 후 순서:

1. 기존 방식으로 KG 이력을 한 번 조회한다.
2. 기존 최신순 정렬을 수행한다.
3. 요청된 20건 페이지를 먼저 선택한다.
4. 페이지에 포함된 실제 소유 세션별 인물 identity·surface·검토 연결을 각각
   한 번 읽는다.
5. 요청 메모리 안에서만 별명→대표 이름 표를 만든다.
6. 선택된 20건의 subject/object만 변환한다.

1,000개 KG 회귀에서 페이지는 20건만 반환되고 identity·surface·link 조회는
요청당 각각 1회였다. 같은 탭을 두 번 요청해도 조회 횟수는 요청마다 1회씩만
증가했다. 합쳐진 이름은 대표 이름으로 표시하고, 합치지 않았거나 모호한
동일 표면은 원문 그대로 유지한다. KG 원본 행은 수정·삭제하지 않는다.

### 9.4 누락 인물 동일성 복구

GET 인물 조회 중 DB를 수정하지 않는다. 사용자가 명시적으로 실행하는 세션
정상화에 `character_identity_repair` 단계를 추가했다.

- 기존 인물 상태의 세션·정확한 이름·턴을 읽는다.
- 같은 세션·같은 턴의 유일한 활성 source revision을 사용한다.
- 해당 정확한 이름의 인물 identity 또는 display surface가 없을 때만 보충한다.
- 평론가 재호출, 기존 인물 상태·사건·기억·KG 재작성, Vector 생성, 유사 이름
  자동 병합은 수행하지 않는다.
- 동일 이름에 여러 기존 identity가 있거나 source revision을 정확히 고를 수
  없는 항목은 그 항목만 `skipped_items`에 남기고 다른 정상 인물은 계속
  처리한다.
- 저장 실패도 해당 항목만 `errors`로 남긴다. 이미 성공한 다른 identity를
  되돌리거나 폐기하지 않는다.
- dry-run과 반복 실행 멱등성을 검증했다.

### 9.5 검증 결과

- `go test ./...`: 통과
- 번들 Node `--check Archive Center.js`: 통과
- `git diff --check`: 통과
- 삭제 인식 lifecycle·즉시 리롤·동일 턴 재삭제 회귀: 통과
- A/B 세션 동시 대조와 같은 세션 동시 호출 합치기 회귀: 통과
- 사용자 입력만 삭제된 턴 유지·assistant-only 콜드 스타트 회귀: 통과
- 잘못된 클라이언트 기준점 대신 source revision의 실제 삭제 턴 사용 회귀: 통과
- 1,000개 KG 페이지·일괄 동일성 조회 회귀: 통과
- 모호한 동일 표면 원문 유지 회귀: 통과
- 세션 정상화 누락 인물 identity dry-run·저장·반복 실행 회귀: 통과

이번 후속 작업 자체의 JavaScript 변경은 요청 직전 호출 복원과 기존 전역
진행 상태·순차 서명 생략 제거에 국한된다. 새 UI 계산이나 저장 정책을
JavaScript에 추가하지 않았다. canonical 삭제 범위와 DB 변경은 계속 Go가
소유한다.

### 9.6 최종 삭제 후속 수정 이전의 4.0.9 Windows 테스트 패키지

기존 패키지와 같은 위치를 정식 빌더로 갱신했다. 패키지를 붙잡고 있던 기존
백엔드와 Windows 실행기만 종료했고 MariaDB·ChromaDB는 종료하거나 초기화하지
않았다. 기존 `.env.full.local`은 빌드 전 임시 보존 후 같은 경로에 복원했다.

- package root:
  `_test-builds/Archive-Center-4.0.9-web-risu-direct-windows-test`
- ZIP: `Archive Center 4.0.9 Windows Auto Install Package.zip`
- package: `green`, `release_ready=true`
- managed files: `46`
- source/package `Archive Center.js` SHA-256:
  `1db85e5af7790244df815ad9541caba4aaf28b158f1c127f5eda9f732562e87c`
- ZIP size: `12,238,661 bytes`
- ZIP SHA-256:
  `78689a8b33f12715dfe314e46765bf7f9f4e26118537c11d82986d2c54bd0fc0`
- 외부 `SHA256SUMS-4.0.9.txt`와 실제 ZIP hash 일치

주의: 위 패키지 증거는 요청 직전 삭제 대조·세션별 in-flight·순차 서명 제거
후속 수정 전의 빌드 기록이다. 현재 소스 후속 수정은 아직 패키지에 반영됐다고
간주하지 않는다. 패키지 갱신은 별도 요청과 검증 뒤 진행한다.

현재 후속 수정의 증거는 소스와 자동 회귀까지다. 문제가 발생한 실제 사용자
MariaDB·ChromaDB 복사본은 제공되지 않았으므로, 해당 장기 세션의 정확한 삭제
범위와 실측 응답 시간은 갱신된 패키지로 별도 확인해야 한다.

## 10. 4.0.9 전체 재감사

재감사 기준: 2026-08-29 KST

비교 범위: tag `v4.0.8` 이후 13개 commit과 현재 미커밋 작업

현재 기준 commit: `2fef745871de97b7099cd1fb52906bdf0c5585ee`

판정 원칙: 소스 존재, 자동 회귀 통과, 테스트 패키지 포함, 실제 RisuAI·DB·Provider
확인을 서로 다른 증거 단계로 기록한다.

13개 중 `a750e0c`, `61820cd`, `2fef745`는 각각 앞선 작업의 패키지·문서 증거를
기록한 문서 전용 commit이다. 기능 구현 commit 수에 중복해서 세지 않는다.

이번 재감사에서 기존 1~9절이 assistant-only 복구, 정확한 rollback, Critic HUD,
Provider 대기 지시, 인물·물품 동일성 작업은 자세히 기록했지만 4.0.9 초반의 설정·
전송 작업과 세션 수명주기·분기 작업 일부를 빠뜨린 것을 확인했다. 아래 항목을
4.0.9 전체 변경 지도로 추가한다.

### 10.1 Web Risu·Provider 설정·기억 중복 제거

commit `8d6819b`에 다음 작업이 함께 고정돼 있다.

- 공식 Web Risu에서 전역 직접 요청 설정을 바꾸지 않고 Archive Center 백엔드
  요청만 `risuFetch`로 보내는 `Web Risu 직접 연결 (실험)` 모드를 추가했다.
  브라우저가 접근 가능한 HTTPS Bridge URL 전용이며 실시간 HUD stream은 이
  실험 경로에서 지원하지 않는다. 이것은 정식 Web Risu 지원 완료 선언이 아니다.
- 출판사·평론가 Endpoint가 비어 있으면 Go의 `proxyProviderBaseURL`이 선택한
  Provider의 공식 기본 Endpoint를 사용하고, 사용자가 입력한 주소가 있으면 그
  값을 우선한다. `custom`과 Embedding은 자동 추정하지 않는다.
- 장황하거나 실제 구성과 맞지 않던 출판사·평론가·Vertex·LLM Gateway·Vercel·
  NeuralWatt 설명 문구를 줄이고 Endpoint의 자동/직접 입력 우선순위를 표시했다.
- 원작 DB 검색 LLM 설정에 전용 저장 버튼을 복구했다.
- 일반 기억의 `Distinct`는 공백을 정리한 뒤 **완전히 같은 문장만** 제거한다.
  turn 표식이 다르거나 JSON 필드 순서가 다른 문장, 내용이 비슷할 뿐 동일하지
  않은 기억은 합치지 않는다. 주관·관계 lane에서도 실제로 같은 문장만 두 번째
  이후 항목을 제외하고 진단의 중복 건수에 남긴다.

증거 수준:

- 소스와 자동 회귀: 확인됨
- 현 테스트 패키지의 과거 snapshot 포함: 확인됨
- 공식 Web Risu + 실제 HTTPS bridge: 미확인
- 각 Provider의 빈 Endpoint 실호출: 자동 테스트 외 실계정 확인 필요

### 10.2 DeepSeek V4 `low`와 부분 복구

commit `460d791`에서 다음을 처리했다.

- Direct·LLM Gateway·OpenRouter·Vercel·Ollama·Custom OpenAI 호환 경로에서
  Provider가 허용하는 DeepSeek V4 `low`를 보존한다. NeuralWatt V4 Flash처럼
  실제 light tier가 없는 경로만 기존 지원값을 사용한다.
- Repair Replay는 한 턴의 user 또는 assistant 원문이 충돌하더라도 읽을 수 있는
  다른 턴·역할을 모두 폐기하지 않는다. 충돌 턴과 실패 역할을 결과에 남기고
  정상 항목은 계속 처리한다.
- 사용자가 HUD의 재처리를 누른 시점부터 상태를 `recovering`으로 전환하고,
  재처리 job이 실패하거나 끝났을 때 terminal 상태로 닫는 기반을 보완했다.

이 시점의 입력 훅 rollback 호출은 이후 현장 회귀를 일으켜 현재 소스에서는
그대로 유지하지 않았다. 최종 삭제 인식 구조는 9.2절과 10.7절을 기준으로 한다.

### 10.3 assistant-only·Critic 복구·토큰 진단

commit `52ab01f`, `dd28a76`, `31fe12a`의 결과는 1.1~1.5절과 12~13절의 기존
통합 기록에 대응한다.

- 입력이 삭제된 assistant 출력도 명시적 콜드 스타트·정상화 후보가 된다.
- DB에 원래 입력이 있으면 재사용하고, 없을 때만 `assistant_only`로 평론가를
  실행한다. 일반 실시간 `/complete-turn`을 assistant-only fallback으로 바꾸지
  않았다.
- 실제 누락된 assistant source revision을 기준으로 rollback 시작점을 정한다.
- Critic 재처리의 예약·시도·소진 상태와 Provider 토큰 종료 정보를 HUD에
  전달한다.
- Provider `Retry-After`와 오류 JSON의 `retry_after`를 allowlist 없이 읽고,
  사용자 설정 간격보다 긴 지시만 우선한다.

소스와 회귀는 통과했지만 179개 메시지 사용자 DB, 실제 NeuralWatt 524,
연속 token exhausted 환경은 여전히 실환경 재검증 대상이다.

### 10.4 세션 고정 처리·연결·이동·복사·삭제

commit `27add61`에서 접수된 세션 혼합과 관리 동작 문제를 다음처럼 정리했다.

- `beforeRequest`가 시작될 때 session ID와 host chat 좌표를 함께 캡처한다.
  A 세션에서 시작한 준비, 출력 확정, 평론가 저장, 정상화, 로어북 동기화는
  사용자가 도중에 B 세션으로 이동해도 A의 캡처된 소유 문맥을 사용한다.
- RisuAI 목록에서 세션이 보이지 않는다는 이유로 DB를 자동 삭제하던 JavaScript
  delete ledger와 자동 DELETE 경로를 제거했다.
- 백엔드 세션 DELETE는 명시적인 세계선 `삭제` 요청만 허용한다. 실제 삭제는
  MariaDB 한 transaction 안에서 수행하며 중간 실패 시 일부 table만 삭제된
  상태로 남기지 않는다.
- 연결은 현재 RisuAI 채팅의 route를 사용자가 고른 기존 Archive Center 세션에
  명시적으로 붙인다. 기존 채팅이 화면에서 사라졌다는 이유만으로 DB 자료를
  지우지 않는다.
- 이동은 DB copy가 성공한 뒤 대상 route를 확정한다. route 확정이 실패하면
  이미 끝난 DB migration을 되돌리거나 다시 복사하지 않고 `route_pending`과
  재연결 동작을 표시한다.
- `memory_source_revisions`처럼 실제 DB와 manifest의 column 순서만 다른 경우를
  schema mismatch로 오판하지 않는다. 이름 집합의 누락·추가·중복은 계속
  실제 오류로 판정한다.

이 작업은 자동 삭제를 없앤 것이며 수동 삭제 기능을 막은 것이 아니다. 연결·
이동·복사·삭제의 실제 장기 DB 검증은 사용자 DB 복사본에서 별도로 확인해야 한다.

### 10.5 분기 계보와 복사 미리보기

commit `eee1b5c`, `553bae3`에서 다음을 처리했다.

- 현재 active revision에서 분기 원본을 찾지 못하면 같은 부모 세션의 기존·
  정상화·재처리 source revision 이력 안에서 실제 branch 표식과 일치하는
  canonical 턴을 찾는다.
- 유일한 턴이면 부모와 분기점만 복구한다. 자식 자료 삭제, 부모 기억 복사,
  평론가 재호출, Vector 중복 생성은 하지 않는다.
- 후보가 여러 개면 자동 확정하지 않고 부모 후보와 분기 턴을 UI에 표시해
  사용자가 `분기 계보 복구`를 실행하게 한다.
- 복사·이동 미리보기와 실제 실행이 같은 MariaDB manifest, 참조 바인딩,
  background job, Chroma vector 점유 판정을 사용한다. 미리보기 통과 뒤
  `target session is not empty`가 뒤늦게 나오는 기준 불일치를 줄였다.
- 같은 요청에서 처음 확정된 worldline ViewModel을 다시 읽지 않고 즉시 턴
  경계 계산에 사용한다. 첫 분기 요청과 기존 unresolved 복구 요청에서도 부모
  상속 범위가 그 응답부터 적용된다.

일반 세션, 미확정·충돌 계보, 서로 다른 부모 후보를 넓게 연결하는 내용 기반
fallback은 추가하지 않았다.

### 10.6 예전 기본값의 교체

commit `1a22a46`은 새 값을 별도 우회 기본값으로 추가한 것이 아니라 과거
기본값 owner를 교체했다.

- 출판사 기본 출력 상한: `30,000`
- 평론가 기본 출력 상한: `30,000`
- 일반 기억 기본 예산: `18,000 chars`
- 원작 DB·로어북 기본 예산: 각각 `3,000 chars` 유지
- 출판사·평론가 timeout 기본값: `120초` 유지

모델 family preset에 남아 있던 `1,024`, `20,000`, `24,000` 출력 상한과 Go의
`1,200`, `1,600` fallback을 제거했다. 일반 기억의 과거 `6,000/9,000` 기본
profile은 `18,000` profile로 migration한다. 사용자가 명시적으로 저장한 다른
유효값까지 매번 18,000으로 덮어쓰지는 않는다.

### 10.7 인물·물품·Explorer와 최신 삭제 인식

commit `435e4e5`, `2fef745`와 현재 미커밋 후속 작업은 1.6절, 7절, 9절에
대응한다.

- 사용자가 확인한 인물·물품만 기존 `canonical_equivalence`로 연결하고 원본
  기억·상태·KG·Vector는 보존한다.
- 인물과 물품 후보·연결 목록을 서로 분리하고, 긴 연결 목록은 기본으로 접는다.
- 인물·물품 선택만 바꿀 때 Explorer 전체를 다시 요청하지 않는다.
- 관계 지식은 전체 KG의 subject/object를 건별 DB 해석하지 않고 20건 page를
  먼저 선택한 뒤 실제 소유 세션별 identity catalog를 한 번씩 읽는다.
- 세션 정상화는 정확한 이름의 identity가 누락된 인물만 항목별로 보충한다.
  모호하거나 실패한 한 인물 때문에 다른 정상 인물을 되돌리지 않는다.
- 최신 삭제 인식은 요청 직전과 세계선 새로고침의 실제 assistant 관측을 같은
  Go `/rollback/decision`에 전달한다. 서로 다른 세션은 session별 in-flight로
  독립 처리하며, 순차 상태 signature가 같다는 이유로 다음 명시적 검사를
  생략하지 않는다.
- 확인된 decision token의 request source를 rollback 실행과 HUD까지 동일하게
  사용한다. 판정과 실제 mutation이 일치한 경우에만 삭제 성공으로 표시한다.

최신 삭제 후속의 JavaScript 변화는 기존 전역·서명 기반 경로를 줄이는 방향이다.
현재 미커밋 JavaScript diff는 `+246 / -515`이고, tag `v4.0.8`부터 현재까지의
누적 JavaScript diff는 `+1,471 / -1,168`이다. JavaScript에 canonical 삭제 턴,
동일성 정책, DB mutation 정책을 새로 넣지 않았다.

현재 물품 API production 파일 `group_items.go`와 누락 인물 동일성 복구 회귀
`group_admin_character_identity_repair_test.go`는 아직 Git untracked 상태다.
현재 작업 폴더의 전체 테스트에는 포함되지만 `HEAD`만 새로 checkout하면 재현되지
않으므로, 최종 고정 commit 전에는 완료 산출물로 보지 않는다.

### 10.8 현재 확인된 미완료·미검증 항목

1. **RELEASE BLOCKER — 현재 소스와 테스트 패키지가 다르다.** 현재 `Archive Center.js` SHA-256은
   `e8ad208ac6dac05cdba3d289ba33eb1b7aca0727e65c011ca1f614d7de7a3f5a`,
   현 4.0.9 Windows 테스트 패키지는
   `1db85e5af7790244df815ad9541caba4aaf28b158f1c127f5eda9f732562e87c`다.
   따라서 9.2의 최신 요청 직전 삭제 대조·세션별 in-flight·서명 제거는 아직
   패키지 또는 실제 RisuAI로 검증됐다고 할 수 없다.
2. **RELEASE BLOCKER — 첫 branch 요청의 backfill 순서 경합이 남아 있다.**
   `beforeRequest`는 분기 채팅의 기존 완료 턴 backfill을 fire-and-forget으로
   시작한 뒤 full prepare로 진행한다. 같은 요청에서 새 세계선이 확정돼도
   backfill이 늦으면 첫 요청만 `current_only` 자료로 조립될 수 있다. Go의
   같은-request 세계선 경계 회귀는 통과하지만 실제 `onBeforeRequest` 경합을
   포함한 회귀는 아직 없다.
3. **RELEASE BLOCKER — 세션 이동 source lock이 기존 파생 worker claim까지
   막지 못한다.** 이동 단계는 현재 complete-turn worker를 취소하고 source
   lock을 저장하지만, 기존 memory reprocessing·Vector outbox lease SQL은
   `session_migration_locks`를 확인하지 않는다. lock 뒤 오래된 job이 다시 lease돼
   원본 세션을 변경할 수 있는 parity 위험이 있으며 해당 회귀가 없다. 실제
   사용자 DB에서 재현됐다는 뜻은 아니지만 현재 코드 계약상 닫히지 않았다.
4. **P1 — 삭제 대조 transport 실패 뒤 full prepare가 계속된다.** 요청 직전
   rollback 대조가 `false`를 반환해도 본문 요청은 fail-open으로 진행한다.
   DB를 잘못 삭제하지는 않지만, 실제 출력 삭제 직후 백엔드 연결이 실패하면
   그 요청 한 번에는 삭제 전 canonical 기억이 주입될 수 있다.
5. **주관 기억 탭 재진입 지연은 미완료다.** 현재 주관 기억 읽기는 항목마다
   인물 대표 이름 resolver를 다시 호출할 수 있다. 관계 지식처럼 요청 단위
   일괄 catalog를 사용하는 성능 수정과 대량 회귀는 아직 없다. 데이터 손실
   결함으로 확인된 것은 아니지만 P1 성능 항목으로 남긴다.
6. **물품 읽기와 과거 물품 ID 복구도 후속 성능·호환 작업이 남아 있다.** 현재
   물품 목록은 항목별 대표 이름 해석을 반복할 수 있고, 세션 정상화의 누락 ID
   복구는 인물만 대상으로 한다. 새 물품 동일성 연결 자체와는 별개다.
7. **자동 plugin-init backfill과 명시적 콜드 스타트의 범위가 다르다.** plugin
   초기 backfill은 아직 입력·출력 pair만 사용한다. 사용자 입력이 모두 삭제된
   assistant-only 복구는 사용자가 실행하는 콜드 스타트·세션 정상화·명시적
   rescan에서만 작동한다. 이는 일반 실시간 fallback을 만들지 않기 위한 현재
   경계지만 UI와 문서에서 혼동하지 않아야 한다.
8. **중첩 branch Vector 검색은 비용 경계가 완전하지 않다.** SQL hydration은
   각 세계선의 turn 경계를 다시 적용하지만 Chroma 후보 검색은 segment의
   session ID 전체를 조회할 수 있다. 다른 턴 자료가 최종 전달되는 결함은 현재
   회귀로 막지만 불필요한 후보·비용이 늘 수 있다.
9. **일부 토큰 소진 응답의 오류 분류가 덜 정확하다.** Claude/Gemini가 최종
   text 없이 `MAX_TOKENS` 계열 종료 사유만 반환하면 provider 정규화 단계에서
   `CRITIC_EMPTY_RESPONSE`로 분류될 수 있다. 토큰 소진 자체의 재처리는 되지만
   HUD 원인명이 정확하지 않을 수 있다.
10. **CID 변경 뒤 기존 세션 자동 재식별은 설계 기록까지만 있다.** 현재 명시적
   연결과 분기 계보 복구는 있지만, CID가 바뀐 모든 채팅을 내용 hash만으로
   자동 병합하는 기능은 구현하지 않았다.
11. **실사용 DB 검증이 남아 있다.** 179개 메시지 assistant-only 정상화 2회,
   36~38 실제 삭제, A→B→C 중첩 분기, 이동·복사·연결·수동 삭제, 인물·물품
   연결 전후 MariaDB·Chroma 행 수를 복사한 실DB로 확인해야 한다.
12. **Provider 실호출이 남아 있다.** Web Risu HTTPS bridge, NeuralWatt 524와
   `retry_after`, token exhausted usage, DeepSeek V4 `low`, 빈 Endpoint 자동
   설정을 실제 계정에서 확인해야 한다.
13. **죽은 구형 rollback helper가 남아 있다.** persisted-ledger fallback helper는
   현재 production 호출 경로에서는 사용되지 않지만 정의와 과거 테스트가 남아
   있다. 현재 canonical 삭제에는 관여하지 않는다. 별도 정리 시에는 먼저 실제
   참조 0건과 회귀 범위를 다시 확인해야 하며 이번 감사에서 삭제하지 않았다.

### 10.9 재감사 결론

- 4.0.9에서 접수된 주요 데이터 손실·assistant-only·세션 혼합·분기·재처리·
  identity·KG 지연 피드백에는 현재 소스와 자동 회귀 기준의 대응 경로가 있다.
- 이번 재감사에서 정상 자료 하나가 틀렸다는 이유로 전체 항목을 삭제하거나,
  다른 세션을 내용 유사도로 자동 연결하거나, 새 watcher·무제한 retry·숨은
  Provider fallback을 추가한 경로는 확인하지 못했다.
- 그러나 첫 branch 요청 경합과 migration lock 뒤 background worker fencing은
  소스에서도 닫히지 않았다. 현재 소스도 현 테스트 패키지보다 앞서 있으므로
  **4.0.9 전체가 완료됐거나 release 가능한 상태라고 판정할 수는 없다.**
  두 소스 blocker를 먼저 고친 뒤 패키지를 갱신하고 실환경 항목을 확인해야 한다.

### 10.10 현재 snapshot 자동 검증

2026-08-29 22:37 KST, 기준 `2fef745 + dirty`에서 다시 실행했다.

- 번들 Node `--check Archive Center.js`: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 통과
- `go test ./internal/httpapi -count=1`: 통과
- `go test ./... -count=1`: 통과
- `git diff --check`: 통과

이 통과 결과에는 현재 Git untracked인 `group_items.go`와
`group_admin_character_identity_repair_test.go`도 작업 폴더 파일로 포함된다.
또한 테스트 통과는 10.8의 branch lifecycle 경합, migration worker fencing,
실제 Provider·RisuAI·MariaDB·Chroma 검증을 대신하지 않는다.

### 10.11 현재 소스의 4.0.9 Windows 테스트 패키지 갱신

2026-08-29 22:49 KST에 기존 테스트 패키지를 새 이름으로 복제하지 않고 같은
위치에서 현재 source snapshot으로 다시 빌드했다.

- package root:
  `_test-builds/Archive-Center-4.0.9-web-risu-direct-windows-test`
- ZIP: `Archive Center 4.0.9 Windows Auto Install Package.zip`
- package status: `green`
- managed files: `46`
- 누락 / 크기 불일치 / SHA-256 불일치: `0 / 0 / 0`
- source/package `Archive Center.js` SHA-256:
  `e8ad208ac6dac05cdba3d289ba33eb1b7aca0727e65c011ca1f614d7de7a3f5a`
- ZIP size: `12,234,523 bytes`
- ZIP SHA-256:
  `ffefd51113436c2afb6d2b0f6673c5f6e26f868f8e4fbdb5b9453456a425f0ea`
- 외부 `SHA256SUMS-4.0.9.txt`와 실제 ZIP hash: 일치
- update contract: target `4.0.9`, minimum source `3.9.9`,
  `direct_update_supported=true`, `automatic_update_apply=true`
- ZIP 안 사용자 `.env.full.local`, DB, runtime, log: `0건`

패키지를 점유하던 해당 4.0.9 Go backend와 launcher만 종료했다. MariaDB와
ChromaDB는 종료·초기화하지 않았다. 기존 `.env.full.local`은 패키지 밖에
보존한 뒤 원래 위치로 복원했고, 복원 전후 SHA-256
`ec1e29c260549b2ff7475d23c32af9406deb22ccb370ccb40d671b32bb920cc2`가
일치했다.

빌드 직전에 번들 Node 구문 검사와 `go test ./... -count=1`을 다시 통과했다.
현재 package manifest의 source commit은 `2fef745`이며 `source_dirty=true`다.
따라서 현재 작업 폴더의 미커밋·미추적 production 파일까지 포함한 시험용
snapshot이지만, 깨끗한 checkout에서 같은 결과를 재현할 수 있는 정식 release
산출물은 아니다. 빌더가 기록한 `release_ready=true`는 필수 payload가 들어간
패키지 무결성 상태이며, 10.8의 branch 첫 요청 경합과 migration worker fencing이
해결됐다는 뜻은 아니다.

### 10.12 과도한 차단 정책 재감사와 제거

2026-08-30 KST, 기존 4.0.9 작업을 tag `v4.0.2`와 다시 대조했다. 이 재감사는
사용자가 반복해서 금지한 다음 구현이 실제로 들어갔다는 피드백 때문에 수행했다.

- 세계선 항목 하나가 미확정이라는 이유로 현재 요청 전체를 건너뜀
- 오래 남은 pending 출력 하나가 실제 삭제 증거까지 전부 막음
- worker 완료 시 migration lock을 다시 잠가 기존 drain·lease fence와 중복됨
- 별도 테스트 실행기가 위 전역 차단을 올바른 결과로 고정함

수정 전 상태 전체는 local checkpoint `f35f32d`로 보존했다. 그 뒤 새 보호 조건을
추가하지 않고 기존 소유 경로에서 다음 조건만 제거·분리했다.

1. `beforeRequest`의 세계선 사전 확인은 과거 턴 backfill 판정에만 사용한다.
   `worldline_ownership_unresolved`여도 현재 `/prepare-turn`과 기억 주입은 계속된다.
2. `onRisuOutput`의 정확한 A 세션 좌표 고정은 유지한다. 세계선 진단 실패·미확정은
   A 세션의 최종 출력 원문 저장과 평론가 진입을 막지 않는다.
3. 정확한 요청 소유자를 찾지 못한 `afterRequest`는 현재 세션으로 대체 저장하지
   않는다. 무한 회전 `watching` 대신 기존 `deferred` 상태로 종료하고 공식 출력
   callback이 캡처된 요청 문맥을 사용하게 둔다.
4. JavaScript의 `pending_output_guard` 전송과 Go의 전역 차단을 제거했다. 실제
   assistant 관측, source revision, lifecycle action, 캡처된 route와 one-use
   decision token이 삭제 여부를 계속 결정한다.
5. memory reprocessing·Vector worker 완료 transaction 안의 migration lock 재검사
   세 곳과 helper를 제거했다. claim/wake의 migration 제외, source worker drain,
   source lifecycle fence, 최종 active lease 및 relational/Vector parity 검사는
   유지했다.
6. `PrepareSessionMigrationSourceLock`가 동시에 non-nil provisional과 error를
   반환한다고 가정한 도달 불가능 분기를 제거했다. prepare 성공 뒤 drain·parity·
   lease 단계가 실패할 때 pending fence를 해제하는 실제 경로는 유지했다.
7. 운영 경로와 분리되어 잘못된 `/prepare-turn` 0회 결과를 고정하던
   `archive-center-runtime.test.cjs`를 제거했다. 회귀는 기존
   `cmd/js-route-variant-smoke`에서 실제 `Archive Center.js` 함수를 추출해 실행하고
   실제 Go HTTP 판정 함수를 호출하는 경로로 통합했다.

Repair Replay에서 한 role이 충돌한 턴을 그대로 user+assistant pair로 재검사하면
충돌한 원문과 새로 복구한 원문이 잘못 짝지어질 수 있다. 따라서 conflict 턴의
후속 rescan 제외는 이번에 무조건 제거하지 않았다. Go의 role별 정상 원문 저장은
그대로 유지하며, role별 rescan 계약이 별도로 마련되기 전에는 턴 전체를 억지로
재검사하지 않는다.

이번 수정의 운영 코드 변화량은 다음과 같다.

- `Archive Center.js`: `+29 / -44`
- Go 운영 코드: `+6 / -51`
- 중복 독립 테스트 파일: `-268`
- 새 API·table·queue·timer·watcher·fallback: `0`

검증 결과:

- 번들 Node `--check Archive Center.js`: 통과
- `go test ./cmd/js-route-variant-smoke -count=1`: 통과
- `go test ./internal/httpapi -count=1`: 통과
- `go test ./internal/store -count=1`: 통과
- `go test ./... -count=1`: 통과
- `git diff --check`: 통과

위 결과는 소스와 자동 회귀 증거다. 실제 RisuAI에서 A 요청 중 B 세션 이동,
미확정 branch 첫 요청, 출력 삭제 뒤 즉시 리롤, 실제 MariaDB·Chroma migration을
사용한 실환경 검증은 테스트 패키지 갱신 뒤 별도로 확인해야 한다. 이 절은 10.8의
첫 branch 요청 및 worker claim blocker, 10.9의 release 불가 판정, 10.11의 이전
패키지 상태를 현재 소스 기준으로 갱신한다.
