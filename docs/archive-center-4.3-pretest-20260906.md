# 4.3 피드백 수정 사전 테스트 — 2026-09-06

판정: **소스 회귀와 격리한 실제 MariaDB·Chroma 시험 통과. 실제 Host 검증은 남아 있으므로 `implemented_unverified` 유지.**

사용자 요청에 따라 현재까지 반영한 #5/#6/#4/#10/#7, 분기·재분기 호환성,
Gemini 3.8 Flash medium 수정분을 함께 시험했다. 다중 AI 전처리 A–F의
구현 완료나 4.3 정식 출시를 확인하는 시험은 아니다.

## 시험 대상과 환경

- 활성 소스: `source/Archive Center.js`, `source/go-service` 및 migrations.
- Git 기준: `work/4.2.0`, HEAD `3e5e0dc99b3daafce5040418436b671f9a84ac78`와
  [피드백 작업 기록](archive-center-4.3-feedback-work-log.md)의 미커밋 수정분.
- 플러그인 SHA-256: `AE89FB198CA06C68A935B29140CC37502355E2359A5152F5A0EF28E03782B298`.
- Windows, Go 1.26.6, Node 24.19.0, MariaDB 11.4.10, Chroma 1.5.9.
- 실제 저장소 시험은 전용 MariaDB `127.0.0.1:33186` 및 Chroma
  `127.0.0.1:8186`을 사용했다. 시험별 새 DB와 전용 컬렉션을 생성·정리했다.
  시험 종료 후 두 서비스가 종료된 것을 확인했다. 기존 Chroma 8000번 서비스는 유지했다.
- 이번 단계에서는 실행 코드와 설치된 플러그인·백엔드·사용자 채팅을 변경하지 않았다.
  JS 변경은 **추가 0줄 / 삭제 0줄**이다. 이전 피드백 작업을 포함한 HEAD 대비
  누적 JS 차이는 추가 116줄 / 삭제 17줄이다.

## 이번에 다시 실행한 결과

| 대상 | 결과 | 증거의 범위 |
|---|---|---|
| 전체 Go 회귀 | 시험이 있는 35개 패키지 통과 | `go test ./... -count=1`. opt-in 실제 DB 시험은 아래에서 별도 실행했다. |
| 피드백 집중 회귀 | 최상위 시험 13개 통과, 건너뜀·실패 없음 | 실제 JS/Go 함수 실행. Host API와 외부 HTTP는 모의 경계다. |
| #5 신규 대량 세션 삭제 | 통과 | 실제 MariaDB·Chroma, 3개 세션 × 112개 source revision × 세션당 4,000개 벡터. 생산 HTTP DELETE 응답은 약 820/790/800ms, 후속 worker의 12,000개 벡터 삭제·재조회는 32.73초. 재시도 중복 없음, 만료 점유 회수, 관련 없는 원문·벡터 보존을 확인했다. |
| #5 과거 중복 대기열 | 통과 | 실제 MariaDB 112,000행 중 중복 111,000행 정리 26.56초. 남은 1,000개 SQL 작업 완료는 8개 묶음/6.53초. 이 측정에는 Chroma 호출이 없다. |
| #6 긴 기억 항목 | 통과 | 실제 MariaDB에서 세 필드의 120/121/255/256/70,000 길이와 한글·이모지를 원래 값 그대로 저장·조회했다. |
| #6 업그레이드와 #4 저장 결과 재사용 | 통과 | 기존 필드 크기의 Error 1406 → 투영 롤백과 원래 결과 JSON/hash 보존 → migration 013 반복 적용 및 호환성 경로 → 보존 결과 재처리. 실제 MariaDB 시험이며 실제 Critic 공급자를 호출하지 않았다. |
| #10 HUD 이벤트 누적 | 회귀 통과 | 실제 JS 함수 + SafeElement 모의 경계에서 100회 교체 뒤 리스너 1개, 최종 정리 뒤 0개. 실제 PocketRisu 브라우저의 장시간 프리징 해소까지 확인한 것은 아니다. |
| #7 오래된 실패 HUD | 회귀 통과 | 409 최신 HUD, 상태 조회, 만료된 카드 정리, 늦은 응답의 화면 소유권 및 표시 턴 정정. 실제 앱에서 복구 버튼을 누르는 시험은 남아 있다. |
| 기본 RisuAI·PocketRisu 분기 형식 | 회귀 통과 | 보존 ID/재발급 ID, user/char 기준점, 재분기의 더 이른 경계, 부모의 빈 출처 대응 보충과 JS 지정 읽기. 실제 두 Host가 생성한 신규 채팅을 연결하는 시험은 남아 있다. |
| 재발급 ID의 재분기 저장 | 실제 DB 통과 | 생산 HTTP/Store + MariaDB. 매 요청 새 서버 인스턴스, 확정된 부모 보존, 출처 보충, 재분기 연결, 마커 제거 후 재조회. 1.93초. |
| Gemini 3.8 Flash medium | 회귀 통과 | 실제 UI 선택지/bridge 함수와 출판사·평론가 요청 생성 함수. Gemini/Vertex/LLM Gateway/OpenRouter의 선택·정규화 및 기존 전송 시험을 포함한다. 실제 공급자 수락·사용량은 미확인이다. |
| 정적 검사 | 통과 | `go vet ./...`, `node --check "Archive Center.js"`, `git diff --check`. |

실제 DB 시험 5개는 모두 건너뜀 없이 통과했고 전체 85.195초였다.
속도는 이 PC의 위 시험 자료에 대한 관측값이다. 이전 작업의 1,120,000행 시험은
이번에 재실행하지 않았으며, 과거 결과는 별도 작업 기록에 유지한다.

## 실제 앱에서 확인한 것과 남은 것

열린 PocketRisu의 플러그인 목록에는 **Archive Center 4.2.0**이 표시되었다.
현재 작업 소스도 버전 표기를 올리지 않았으므로 이 이름만으로 수정본의 적용 여부를
판별할 수 없다. 로드된 코드와 시험 소스의 일치, 현재 연결 백엔드의 일치는 확인하지
못했다. UI 조회 중 브라우저 자동화 연결이 `Debugger unattached`로 끊겨 추가 조회도
진행하지 못했다. 이것은 Archive Center 실행 오류를 재현한 결과가 아니다.

수정본을 적용한 실제 Host 시험에서는 다음을 확인해야 한다.

1. 실제 로드한 플러그인·백엔드가 이번 시험 대상과 일치하는지 확인한다.
2. 별도 시험용 채팅에서 일반 턴, 같은 행 reroll, assistant 삭제 후 같은 user 행
   편집·재생성, 동일 본문인 새 user 행을 실행하여 저장·교체 결과를 확인한다.
3. 기본 RisuAI와 PocketRisu 각각 분기 → 상속 구간의 재분기 → 기억 조회를 실행해
   직접 부모와 상속 범위를 확인한다. 일반 복사와 독립 편집용 기억 복사는 구분한다.
4. 실제 HUD의 반복 표시·닫기·실패 복구를 실행하여 이벤트 누적과 오래된 카드의
   재등장을 확인한다.
5. Gemini 3.8 Flash의 medium 선택·저장·재열기와 실제 요청 수락을 확인한다.

상속 기억 전체를 독립 복사·수정하는 체크박스와 개체/주관 기억 상속 UI는 미구현인
별도 기능이다. 이번 연결 시험의 통과가 그 기능의 구현을 뜻하지 않는다.
각 OS 신규 설치·자동 업데이트와 공개 GitHub 배포도 이번 시험에서 수행하지 않았다.

## 재현과 로그

작업 루트의 `.tmp-feedback-43/`에 이번 실행 로그가 있다. 로그는 로컬 시험 산출물이며
배포 자료에 포함하지 않는다.

- `pretest-go-all.log`: 전체 회귀.
- `pretest-focused.log`: 피드백 집중 시험의 개별 PASS와 하위 사례.
- `pretest-integration.log`: 실제 MariaDB·Chroma 5개 시험과 시간.
- `pretest-vet.log`: vet 오류 없음.
- `pretest-db.stdout.log`, `pretest-db.stderr.log`, `pretest-chroma.stdout.log`,
  `pretest-chroma.stderr.log`: 전용 서비스 실행·종료 기록.

Go 명령은 `source/go-service`에서 실행했다. 집중 시험은
`cmd/js-route-variant-smoke`, `internal/httpapi`, `internal/store`의
`TestFeedback43*`, `TestWorldline43*`,
`TestArchiveCenterJSReasoningControlsUseProviderAndEndpointRuntime`,
`TestProxyGemini38*`, `TestTurnWorkflowHUDUnavailableRecoveryReturnsCurrentSnapshot`,
`TestTurnWorkflowHUDCorrectsEstimatedTurnBeforeRecovery`를 선택했다.
Node 실행 파일 경로도 `ARCHIVE_CENTER_NODE_BINARY`로 명시했다.

실제 DB 시험은 `AC_FEEDBACK_TEST_DISPOSABLE=YES`, 기존 DB 이름이 없는 전용
`AC_FEEDBACK_TEST_DSN`, 전용 `AC_FEEDBACK_TEST_CHROMA`를 지정한 뒤 실행했다.
`cmd/mariadb-schema`의 `TestFeedback43.*MariaDBIntegration` 및
`TestWorldline43ReissuedNestedBranchMariaDBIntegration`을 선택했다.
재실행할 때도 사용자 설치의 기본 DB·Chroma 경로를 사용하지 않는다.
