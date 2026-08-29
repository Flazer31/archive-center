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

- source commit: `31fe12ac60a457ed92e7f859af7c53f73368844f`
- package source dirty: `false`
- package status: `green`, `release_ready=true`
- `automatic_update_apply=true`, `direct_update_supported=true`
- source/package `Archive Center.js` SHA-256:
  `aca07cdce8f95cd6421f0269ee8d60ed21ea632877fb01722187add45a73d86f`
- ZIP size: `12,159,556 bytes`
- ZIP SHA-256:
  `f7bab74439269f6288065232a54be9805c422a12844cf53f5089ee7f98860cab`
- 외부 checksum과 실제 ZIP hash 일치

패키지·소스 동일성과 자동 업데이트 manifest는 확인됐다. 갱신된 패키지를
실제 RisuAI에 로드한 HUD, Provider, MariaDB/Chroma 동작은 별도 실환경 검증이다.
