# Archive Center 4.0.6 작업 복원 기록

상태: 독립 태그·커밋·당시 전용 문서가 없어 사후 복원  
복원 근거: 4.0.5 체크포인트 `9f1ba7e`, 보호 제거 전 4.0.7 테스트 패키지, 현재 함수·회귀 diff

## 이 기록의 한계

4.0.6이라는 별도 릴리스 커밋은 존재하지 않는다. 아래 내용은 4.0.5 체크포인트 이후, 4.0.7 리롤·worker 작업 전에 진행된 사용자 요청을 소스와 보존 패키지에서 구분한 것이다. 따라서 정식 배포 완료 기록이 아니라 작업 구간 복원 기록이다.

## 복원된 작업 내용

### Custom/OpenAI 호환 응답

- Custom/OpenAI 호환 endpoint의 문자열 content, content block 배열과 응답 metadata를 정규화했다.
- 명시적으로 `/responses` endpoint를 사용한 경우 Responses API의 `output_text`를 읽는 경로를 추가했다.
- `finish_reason`, termination kind, 입력·출력·추론 token 정보를 연결 테스트 ViewModel까지 보존했다.
- reasoning만 있고 최종 text가 없는 length 종료를 일반 연결 실패와 구분했다.
- 연결 테스트는 `Reply with exactly: OK`와 실제 응답 가능한 예산을 사용한다.
- provider·model allowlist와 숨겨진 재시도는 추가하지 않았다.

### 평론가 이름 매칭 지침

- 사용자가 명시한 이름 매칭 표가 있을 때에만 해당 표의 성·이름을 identity 필드에 정확히 사용하도록 했다.
- given name을 우선 보존하고 다른 행의 성을 섞거나 새 성을 만들어내지 않도록 했다.
- 이름 표는 인물·사건을 새로 만드는 근거로 사용하지 않으며 출력 언어 계약도 바꾸지 않는다.

### 사용자 입력만 삭제된 턴의 유지

- assistant 출력 순서가 그대로라면 사용자 입력만 사라진 것을 턴 삭제로 취급하지 않도록 했다.
- assistant 출력 삭제는 assistant message/generation identity와 content hash 관측을 백엔드 active source revision과 대조하도록 확장했다.
- 이 작업은 사용자 입력 삭제 때문에 이미 만들어진 턴 전체가 사라지는 문제를 막기 위한 것이며, assistant 출력이 실제로 삭제된 경우의 canonical 삭제는 유지한다.

## 변경하지 않은 것

- 기억 선택·예산, 로어북 선택기, Publisher/Critic 저장 계약, DB schema는 변경하지 않았다.
- `replacement_pending` 상태는 이 구간의 작업이 아니다.

