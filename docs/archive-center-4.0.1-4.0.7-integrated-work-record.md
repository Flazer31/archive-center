# Archive Center 4.0.1~4.0.7 통합 작업 기록

기준일: 2026-08-26 KST  
목적: 빠진 버전 기록을 보완하고 4.0.7 문제 수정 범위를 고정한다.

사용자 피드백별 상세 작업과 핵심 기능 감사표는 `docs/archive-center-4.0.2-4.0.7-feedback-work-record.md`를 기준으로 한다.

## 버전별 작업

| 버전 | 핵심 작업 | 근거 | 현재 처리 |
|---|---|---|---|
| 4.0.1 | 주관 기억 owner 별칭·belief 투영 복구, 로어북 오류 상세, Go 1.26.6 | `v4.0.0..v4.0.1`, 4.0.1 작업 기록 | 유지 |
| 4.0.2 | 문자열 `bookVersion`, canonical admission hash | `v4.0.1..v4.0.2`, 4.0.2 작업 기록 | 유지 |
| 4.0.3 | 사용자가 고정한 활성 약속 전달 | 4.0.3 작업 기록 | 유지 |
| 4.0.4 | Chroma→MariaDB embedding 수렴, delete key·중복 outbox·batch 수정 | 4.0.4 작업 기록, `4a30fb3` | 유지 |
| 4.0.5 | Publisher 입력 경량화·JSON 진단·gateway 호환 | 4.0.5 작업 기록, `9f1ba7e` | 유지 |
| 4.0.6 | Custom/Responses 응답 정규화, 이름 매칭, 사용자 입력만 삭제된 턴 유지 | 4.0.5 이후 diff와 보호 제거 전 4.0.7 패키지 | 유지 |
| 4.0.7 | 리롤 `replacement_pending`, 비동기 rollback vector 정리, bounded worker, HUD 종료 | 4.0.7 작업 기록과 보존 패키지 | `replacement_pending`만 제거, worker/HUD 유지 |

## 사용자가 지적한 4.0.7 결함

보호 제거 전 4.0.7은 `beforeRequest`에서 기존 source를 `replacement_pending`으로 기록하고 JavaScript도 이를 정상 대기 결과로 취급했다. 실제 테스트에서는 콜드 스타트가 턴을 복원한 뒤 자동 rollback이 실행되어 턴이 무효화되는 현상이 확인됐다.

이번 수정의 경계는 다음과 같다.

- 제거: Go의 `replacement_pending` decision·source-acceptance 기록·특수 binding, JS의 pending HUD·성공 취급, 관련 회귀
- 수정: 첫 관측에서 persisted ledger 길이만으로 자동 rollback하지 않음
- 수정: host signal의 독립적인 backend tail reconcile 2회 호출 제거
- 복원: 4.0.5까지 존재했던 blind-tail gap 제한
- 유지: 4.0.1~4.0.6의 모든 기능
- 유지: 4.0.7의 rollback HTTP 비동기 outbox, bounded worker, delete 공정성, worker stop timeout, 중단 HUD
- 유지: 4.0.0 이전부터 존재한 `/del`·`/cut`, 기존 rollback detector와 기타 보호·호환 경로
- 금지: `guard` 또는 `fallback` 문자열만 보고 범위를 확대해 삭제하는 행위

## 복구 기준과 검증

- 보호 제거 전 4.0.7 Windows 테스트 패키지의 `Archive Center.js`를 이전 작업 보존 기준으로 사용했다.
- 잘못 제거된 343줄을 복원한 뒤, 해당 기준과 비교해 `replacement_pending` 관련 19줄 삭제와 반환 1줄 변경만 남겼다.
- JavaScript 문법 검사: 통과
- Go `internal/httpapi`: 통과
- JavaScript route smoke: 통과
- 현재 소스로 Windows 테스트 패키지를 갱신했다. ZIP SHA-256은 `1466BCAFCFA01BCDDC0624CDD0DDAB598D0D81894A7F1499F2A827D9DD81F373`이며 전체 OS 정식 릴리스와는 구분한다.
