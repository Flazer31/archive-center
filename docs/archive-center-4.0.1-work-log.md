# Archive Center 4.0.1 작업 기록

기준 태그: `v4.0.0..v4.0.1`  
릴리스 커밋: `cf94346f0e2b71e8d9d45c6d85012c9eb14e84e1`  
빌드 보완 커밋: `13824ed1e07fde838b7873806798c48fe44cbfb8`

## 목표

- 실제 provider가 `belief_updates.owner` 또는 `owner_entity_name`을 반환해도 주관 기억을 잃지 않게 한다.
- 저장된 Critic 결과의 재생에서도 같은 owner 별칭을 해석한다.
- 로어북 snapshot 동기화 실패의 실제 HTTP·백엔드 상세를 UI에 보이게 한다.
- Windows와 관리형 패키지의 Go 도구 체인을 `1.26.6`으로 고정한다.

## 적용 내용

- Critic 관점 판정과 정규화에서 `owner`, `owner_entity_name`을 `perspective_owner` 별칭으로 인정했다.
- 명시적인 owner 기반 belief는 belief/claim을 주관 기억 본문으로 사용하고, listener 기반 전달 기억은 기존 evidence 본문을 유지했다.
- committed Critic replay에서도 `belief_updates`로부터 빠진 주관 기억 투영을 다시 만들 수 있게 했다.
- 평론가 프롬프트에 `subjective_entity_memories`와 `belief_updates`의 짧은 객체 모양을 복원했다.
- 로어북 snapshot 실패에 request path, HTTP 상태, backend code/detail 등을 표시하는 진단 UI를 추가했다.
- 로드맵 통합 문서와 설치·빌드 버전 표식을 갱신했다.

## 변경하지 않은 것

- 기억 종류, Critic pipeline 버전, DB schema, 모델/provider allowlist는 변경하지 않았다.
- 리롤·삭제·콜드 스타트 보호 상태를 추가하지 않았다.
- 하나의 잘못된 선택 항목 때문에 전체 Critic 결과를 버리는 정책을 추가하지 않았다.

## 근거

- Git 태그 diff: `v4.0.0..v4.0.1`
- 주요 소스: `turn_extraction_critic.go`, `turn_extraction_private.go`, `turn_precise_memory.go`, `critic_system.txt`, `Archive Center.js`

