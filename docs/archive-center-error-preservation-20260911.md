# 오류 원문 보존 수정 — 2026-09-11

상태: 소스 수정·회귀 검사·격리된 JS/Go 실행 검증 완료. 실제 사용자의 RisuAI 및 제보자의 DB 환경에서는 `implemented_unverified`.

## 요청과 범위

전체 오류 수집 감사 후 사용자가 먼저 해결하도록 요청한, **이미 받은 오류 원인이 처리 도중 사라지는 경로**를 수정했다. 기존 4.4-B/C 미커밋 변경은 유지했다. 기억 선정·AI 추천·재시도·턴 교체·저장 정책을 바꾸지 않았다.

| 기존 소유 경로 | 변경 |
|---|---|
| `internal/vector/chroma.go::doJSON` | 성공/실패 HTTP 응답의 body 읽기 오류를 `errors.Is`로 추적할 수 있게 반환한다. HTTP 상태와 실패 응답에서 이미 읽힌 원인 문구를 유지한다. 정상 JSON은 기존처럼 처리한다. |
| `internal/httpapi/group_turn_prepare.go::handleEffectiveInputs` | DB 오류를 카운트만 올리고 버리던 부분을 수정한다. `save_error`, `store_write_error_details`와 Go 오류 출력에 작업·원인을 남긴다. 실제 입력 저장 성공과 감사 저장 실패는 기존처럼 별도로 유지한다. |
| `internal/httpapi/memory_reprocessing_worker.go::processMemoryWorkerWake` | 재처리 drain, Vector drain, 다음 작업 시각 조회의 반환 오류를 기록한다. 기존 루프 종료·다음 실행 시각·호출 횟수는 유지한다. |
| `cmd/archive-center-go/main.go` | 기존 JSON stderr logger를 기본 slog logger로도 연결하여 위 오류가 같은 출력 형식을 사용하도록 한다. |
| `Archive Center.js::bridgeFetch` | body를 한 번 읽고 그 문자열에서 JSON을 해석한다. 평문 HTTP 오류나 200 응답의 JSON 오류에서도 본문을 보존한다. 기존 JSON-only 응답과 web-direct의 빈 응답 동작을 유지한다. HTTP 500을 서버가 꺼졌다는 설명으로 연결하던 힌트도 제거했다. |
| `Archive Center.js::auditFetch/renderAuditSection` | 조회 실패를 빈 감사 기록으로 표시하지 않는다. 실패 문구를 HTML escape하여 표시하며 다음 정상 조회에서는 오류 상태를 해제한다. 한국어·영어·일본어 문구를 추가했다. |

DB/worker 기록은 기존 `completeTurnPersistenceDiagnostics`의 키·DSN 마스킹과 길이 제한을 사용한다. 새 원문/프롬프트 저장소, RAM 이력, 큐, watcher, 자동 재시도는 추가하지 않았다. JS는 응답 읽기와 UI 표시만 변경했다. 이번 변경의 JS 증감은 **+40/-45줄**이다.

## 검증

다음은 실제 제품 함수 또는 등록된 API를 실행한다. 외부 DB/HTTP/호스트 경계만 fixture로 대체한다.

- `TestChromaResponseReadFailurePreservesCause`: 비어 있는 200 body, 잘린 JSON, 완성 JSON 뒤 read 오류, 500 body 중단. 원인 wrapping·HTTP 상태·외부 호출 횟수를 확인한다.
- `TestEffectiveInputFailurePreservesSafeCause`: 등록된 `/effective-inputs`에 입력 저장 실패/감사 저장 실패/정상을 전달한다. 원인 반환·Go 출력·키 마스킹·부분 저장 결과·감사 호출 횟수를 확인한다.
- `TestMemoryWorkerReadFailurePreservesSafeCause`: 실제 worker wake를 실행한다. 세 실패 지점의 원인과 키 마스킹, 정상 idle의 무오류, 추가 호출·스케줄 부재를 확인한다.
- `TestErrorPreservationBridgeAndAudit`: 실제 JS 함수와 표준 Response를 사용한다. native/web-direct의 429·평문 500·잘못된 JSON 200·body 중단·JSON-only·정상·빈 direct 응답을 확인한다. 감사 조회 실패/빈 정상/기록 있는 정상과 오류 문구 escape도 검사한다.
- 수정 전 소스를 Go overlay 및 JS root fixture로 읽어 같은 신규 검사를 실행했다. Chroma/DB/worker/브라우저 원문 보존 검사가 예상대로 실패했다. 제품 파일을 되돌려 덮어쓰지는 않았다. 감사 화면의 수정 전 빈 목록 오표시는 앞선 감사에서 별도로 재현되어 있다.

관련 네 패키지의 전체 검사:

```text
go test ./internal/vector ./internal/httpapi ./cmd/js-route-variant-smoke ./cmd/archive-center-go -count=1 -json
```

최초 실행에서 기존 JS transport fixture 하나가 실패했다. 해당 fixture는 `json()`이 정상 객체를 반환하는데 `text()`는 빈 문자열을 반환했다. 표준 Response 하나로 대체하여 동일 응답 본문을 제공하도록 수정했으며, 경로/timeout/binary 제한에 대한 기존 검증은 유지했다. 현재 공식 API 참조 checkout `9b2606944538f37b6a1d0089c787465a71197beb`의 `apiV3/risuai.d.ts`도 `nativeFetch(...): Promise<Response>`를 선언한다. 이 로컬 참조가 현재 사용자 호스트 버전이라는 주장은 하지 않는다.

JS 전체 패키지를 다시 실행한 최종 합산 결과: **4,993 통과, 실패 0, 환경 의존 검사 4개 생략**. 통과 수에는 하위 테스트가 포함된다. 생략된 검사는 실제 Chroma 연결 2건, 공개 릴리스 업그레이드 1건, 실제 제공자 호출 1건이다. JS `node --check`도 통과했다.

기존 등록 경로·기억 준비·기본/AI 선정·provider·턴 저장/리롤/교체 검사도 httpapi 패키지 전체 실행에 포함되었다. 실제 사용자 기억 품질이나 실사용 화면 결과를 이 검사만으로 보증하지 않는다.

## 한글/유니코드 경로 확인

사용자가 점검 중 추가로 문의한 Windows 한글 이름 문제도 확인했다.

- Windows launcher는 `.NET ProcessStartInfo`/`Process.Start`를 사용하고, 문자열을 받는 Windows job API는 `CharSet.Unicode`를 사용한다.
- 선행 진단에서는 `fixture-어드민 with spaces` 경로에서 실제 Go의 실패 stderr 수집을 검증했다.
- 이번에는 최신 Go를 빌드해 `검증 어드민/패키지` 아래에 두고 `ops/settings-pair-smoke.cjs`로 전체 JS와 실제 Go를 함께 실행했다.
- **기동 → 설정 조회 → 저장 → 페이지 새로고침 → 재열기 통과**. 브라우저 localStorage와 Go 설정 경로는 실제로 실행했다. 호스트 API만 격리하고 사용자 DB/LLM은 사용하지 않았다.
- 따라서 한글·공백 경로에서 이 실행·설정 흐름이 무조건 실패하지는 않는다. 제보자의 실제 Windows 계정/권한·MariaDB·Chroma 의존성 조합 또는 다른 경로 처리 문제가 없다는 뜻은 아니다. 제보자의 `exit code 1` 직접 원인은 아직 그 환경의 stderr가 필요하다.

## 남은 별도 작업

이번 수정은 **원인을 전달하고 기존 오류 출력에서 잃지 않도록 하는 단계**다. Windows/Linux/macOS/Termux 런처의 지속 파일 수집, 실행별 보관, provider 전체 실패 이력, 공통 API 로그, 조회·내보내기는 구현하지 않았다. 현재 브라우저의 경로별 마지막 오류 Map도 영속 이력으로 바꾸지 않았다.

기존 사용자의 백엔드를 종료하거나 새 빌드로 바꾸지 않았다. 검증용 실행 파일만 격리 디렉터리에 생성했으며, 테스트 배포 패키지나 GitHub 릴리스는 만들지 않았다.

상세 실행 증거는 프로젝트 루트의 `_diagnostics/20260911-error-preservation-fix/`에 있다: 수정 전 비교 사본, `negative-go.txt`, `negative-js.txt`, 최초 `regression.jsonl`, 수정 후 `js-regression.jsonl`, 한글 경로의 설정 검증 결과. 최초 로그의 fixture 실패는 후속 JS 전체 통과 기록과 함께 읽어야 한다.
