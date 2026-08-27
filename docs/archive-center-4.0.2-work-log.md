# Archive Center 4.0.2 작업 기록

기준 태그: `v4.0.1..v4.0.2`  
관련 커밋:

- `0b63af785a93d5846ea578fc82c9f105e6b0c30c` — legacy lorebook `bookVersion` 허용
- `d80b2d55ad138534cb016692ffeaa63429d6a806` — durable memory admission JSON hash 일치
- `ed8fcba70bba93481f8be1c30c95d540c6d378f4` — 4.0.2 릴리스

## 목표와 적용 내용

### 로어북 snapshot 호환

- `bookVersion`을 JSON 숫자로만 받지 않고 숫자 문자열도 읽을 수 있게 했다.
- 비어 있거나 해석할 수 없는 선택 필드는 snapshot 전체를 거절하는 대신 `nil`로 정규화했다.
- 실제 `bookVersion: "..."` 요청의 회귀를 추가했다.

### 기억 admission hash 수렴

- 저장할 때의 map/slice와 JSON 재로딩 뒤의 `[]any`가 다른 형태로 정규화되어 `committed_derived_result_hash_mismatch`가 발생하던 문제를 수정했다.
- 저장과 재생 검증이 동일한 canonical result JSON 문자열을 해시하도록 통일했다.
- 변조 감지는 유지하고, hash 불일치를 무조건 정상 처리하는 fail-open은 추가하지 않았다.

## 변경하지 않은 것

- Voyage 또는 특정 embedding 모델 예외를 추가하지 않았다.
- DB schema와 새 테이블을 추가하지 않았다.
- 리롤·삭제·콜드 스타트 보호 상태를 추가하지 않았다.

## 근거

- Git 태그 diff: `v4.0.1..v4.0.2`
- 주요 소스: `group_lorebook_reference.go`, `turn_memory_admission.go`, `memory_reprocessing_worker.go`

