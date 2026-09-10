# Archive Center 4.3.1-test.4 — 입력 맥락 조립 지연 수정

2026-09-10. 검증했던 중복 확인 수정안을 **활성 Go 소스와 새 Windows 테스트 패키지에 적용했다.**
소스·회귀·패키지 검증은 완료했으며, 실행 중인 사용자 백엔드는 교체하거나 재시작하지 않았다.
빌드 당시 새 빌드의 실제 RisuAI 요청 완료·본문 기억 활용은 `implemented_unverified`로 남겼다.

후속 결정: 이 패키지로 식별되는 **4.3.1을 이후 기억 동작의 기준**으로 고정한다.
[보존 기준과 사용자 후속 관측 범위](archive-center-memory-recall-restoration-plan.md#memory-baseline-431)를 따른다.
아래 빌드 당시의 검사·해시·증거 범위는 유지하며 정식 배포나 새 빌드를 뜻하지 않는다.

## 사용

1. 기존 AC 백엔드를 평소 방법으로 종료한다.
2. `source/_dist/4.3.1-test.4/Archive Center 4.3.1-test.4 Windows Test/`의
   `01_start_archive_center_windows.bat`를 실행한다.
3. 같은 폴더의 `Archive Center.js`로 플러그인을 교체해 HUD 버전을 일치시킨다.
4. 기존과 같은 입력으로 `입력 맥락 조립`이 끝나는지 확인한다.

새 패키지의 포트 메뉴는 사용자가 변경한 이름인 `06_change_port_windows.bat`다.
설정한 데이터 위치와 포트를 기존 실행기가 그대로 사용한다. Go 포트를 변경한 경우에만
RisuAI의 백엔드 URL도 해당 포트에 맞춘다. 백엔드 자동 시작·종료, 운영 DB 변경, GitHub 업로드 없음.

## 적용한 수정

- `prepare_turn_recall.go::prepareTurnDistinctiveRecallTerms`에서 조사 확장 결과의
  중복 확인을 기존 목록 전체 재정규화·순회에서 함수 내부 `formSeen` 조회로 교체했다.
- 입력은 이미 소문자 단일 토큰이다. 최초 등장 순서, 한국어 조사 확장, 기존 anchor 제외,
  빈 결과 처리와 단어 빈도 분석을 유지한다. 단어 중복 확인을 없앤 것이 아니다.
- 검색 폭, 후보 수, 의미 점수, 중요도, 시간, 문자 예산, AI 추천 및 기존 추천 없음/실패 처리는
  변경하지 않았다. API·스키마·캐시·추가 호출·새 기억 거부 조건 없음.
- 기존 `memory_recall_restoration_test.go`에 긴 질의의 생산 함수 벤치마크를 추가했다.
  서로 다른 조사에서 공통 어간이 한 번만 남고 최초 순서가 유지되는지도 확인한다.
- 패키지 빌더의 출력 파일명과 동봉 안내문을 `06_change_port_windows.bat`로 맞췄다.
  내부 원본 BAT 경로는 `ops/full-package/06_change_chromadb_port_windows.bat`이며 내용은 그대로다.
- 이번 턴의 Go 실행 코드 차이 **+6/-2**, 벤치마크 **+31/-0**, JS 실행 소스 **+0/-0**.
  기존 dirty worktree의 기억 회수·포트 수정은 보존했다. 패키지 JS는 버전 표시만 다르다.

## 검증 결과

| 검사 | 결과와 범위 |
| --- | --- |
| 실제 세션 자료로 같은 조립 함수 실행 | 이전 45초 제한 초과 → 활성 수정 소스 **3.46초**, 사실 seed 580개 |
| 추출 단어·순서 비교 | 짧은 질의 2개와 실제 긴 대화 4개 × anchor 유무, **12개 모두 기존 결과와 일치** |
| 기억 회수·우선순위·전처리 및 조립 재현 | **52개 최상위 / 96개 세부 사례 통과**, 실제 AI 대신 통제된 외부 응답 사용 |
| 전체 Go `go test -mod=readonly ./... -count=1` | **35개 패키지 통과**, 3,874개 최상위 / 4,608개 세부 사례 통과; 환경 의존 검사 13개 건너뜀 |
| 패키지 | 관리 파일 **55개**의 크기·해시, ZIP 전체 내용, JS/실행기 버전, 프롬프트·마이그레이션·포트 helper 소스 일치 |
| 구문·차이 | 패키지 JS `node --check`, 빌더 PowerShell 구문, `git diff --check` 통과 |

전체 Go 검사의 건너뜀은 실제 MariaDB 8개, Chroma 2개, POSIX 실행 비트 1개,
공개 패키지 업그레이드 1개, 실제 제공자 1개다. 이 환경의 성공으로 계산하지 않았다.
전체 검사는 진단용 가상 파일 없이 실행했다. 조립·동등성 검사는 가상 테스트 파일만
추가했으며 **실행 대상 recall 소스의 overlay 대체는 없다.**

이전 원본과 현재 생산 함수를 같은 벤치마크로 각각 실행했다 (`-benchtime=1x`).

| 단서 수 | 이전 | 수정 후 | 이전 / 수정 후 할당 횟수 |
| --- | --- | --- | --- |
| 128 | 16.6ms | 0.393ms | 99,256 / 578 |
| 512 | 250ms | 1.46ms | 1,576,529 / 2,145 |
| 2,048 | 4.02초 | 4.24ms | 25,180,310 / 8,387 |

이는 해당 함수의 단일 실행 측정이다. 전체 턴 처리 시간이나 실제 기억 정확도를 뜻하지 않는다.
기존 함수는 결과 동등성 자체는 통과하지만, 단서 증가에 따라 중복 검사 비용이 급증한다.
성능 실패 증거는 이전 조립의 45초 timeout과 위 원본 비교다. 일반 테스트에 시간 임계값을 넣지 않았다.

## 패키지 식별

- ZIP: `source/_dist/4.3.1-test.4/Archive Center 4.3.1-test.4 Windows Test.zip`
- 크기: **18,103,731 bytes**
- ZIP SHA-256: `e3aee185edd26ac51af8752639c21a7283ee0d97f2fd809761739722db0f1cf6`
- Go SHA-256: `2bf0cb9c9bb65c3167dda2619385704e6249d632b430896a1f52658358b0d279`
- JS SHA-256: `bc2bfb76f30b2abff99172ca4d328b44161785ceba03573e884000745378384f`
- Go 1.26.6 / Windows amd64 / 기본 소스 버전 4.3.0 / 패키지 표시 4.3.1-test.4.
- **Go는 test.3과 다른 새 바이너리**다. updater와 schema helper는 test.3과 동일하다.
- 기존 test.3과 그 이전 패키지는 보존했다. 운영 데이터·실제 환경설정·API 키를 포함하지 않는다.

## 남은 확인과 증거

실제 세션 자료를 읽어 저장한 로컬 snapshot으로 조립 단계를 재현했다. DB 재조회·벡터 검색·
실제 전처리/출판사 호출·RisuAI 본문 출력까지 재실행한 것은 아니다. 따라서 새 패키지를
시작한 뒤 같은 요청의 완료와 최종 주입을 확인해야 한다. 실사용 기억 품질의 남은 한계는
[구버전 공통 재검사](archive-center-4.3.1-memory-revalidation.md)에 그대로 남는다.

증거: `_diagnostics/20260910-context-assembly-fix/`의 `targeted.jsonl`, `full-go.jsonl`,
`benchmark-before.log`, `benchmark-after.log`, `verification.json`, `turn.diff`, `package-build.log`.
`overlay-before.json`은 원본 비교 전용, `overlay-active-probes.json`은 현재 소스에 진단 테스트만 추가한다.
이전 지연 재현의 stack과 실패 로그는 `_diagnostics/20260910-context-assembly-stall/`에 보존한다.
사적 RP 자료는 진단 폴더에만 있고 테스트 패키지·소스 벤치마크에는 포함하지 않았다.
