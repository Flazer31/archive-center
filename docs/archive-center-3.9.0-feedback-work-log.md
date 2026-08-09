  # Archive Center 3.9.9 피드백 통합 작업 기록

상태: `release_candidate_work_in_progress / source_mostly_implemented / automated_regression_partial / 3.9.9_package_pending`

기준일: 2026-08-09
기준 source: `C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source`  
과거 checkpoint: `b8a912d`, `c2aae3c`
현재 배포 목표: `3.9.9`
현재 패키지: `3.9.5-1st-test`는 중간 테스트 기록이며 3.9.9 완성판이 아님

이 문서는 3.9.4 작업을 폐기하고 공식 3.9.0 source로 돌아온 뒤 받은 피드백과 그 후속 작업을 3.9.9
배포 목표까지 통합한다. 3.9.0·3.9.5라는 이름은 당시 source·테스트 빌드의 역사적 이름이며, 현재 완료
판정은 이 문서 앞부분의 3.9.9 기준표를 따른다. 뒤의 상세 절은 작업 당시의 증거를 보존한 연대기 기록이다.
기능이 source에 연결됐다는 사실과 실제 설치본·RisuAI·MariaDB·Chroma·Voyage에서 확인됐다는 사실을 구분한다.

## 1. 현재 결론과 문서 읽는 법

### 1.1 3.9.9 완료 전에 남은 비실사용 작업

1. endpoint 누락 시 설정 저장 성공과 runtime 설정 불완전을 UI에서 구분한다. endpoint 필수 정책은 유지한다.
2. Linux·Ubuntu systemd 신규 설치가 사용자 지정 `ARCHIVE_CENTER_DATA_DIR`를 release helper에 전달하게 한다.
3. 외부 표시가 `Publisher LLM`으로 바뀐 뒤에도 `Supervisor`를 찾는 낡은 JS 회귀 assertion 한 건을 정정한다.
4. 전체 자동 회귀를 다시 통과시키고 3.9.9 OS 6종 패키지·ZIP·manifest·SHA-256을 새로 만든다.
5. 공개 배포 전 실제 MariaDB·Chroma 업데이트와 POSIX 대상 OS 설치·종료·업데이트를 검증한다.

### 1.2 이번 버전에 넣지 않는 항목

- A-3 `retry_after` timer: 필요성이 낮다고 판단하여 보류했으며 3.9.9 완료 조건으로 세지 않는다.
- 직전 완료 턴 부분 수정: 별도 집중 작업으로 이관했다.
- 스트리밍 중간 chunk 실시간 저장: 공식 host API가 없어 `unsupported`다.
- RisuAI 삭제 즉시 HUD: 신뢰할 공식 삭제 lifecycle이 없어 실패 구현을 제거했다.
- 1.0 방식 `book_author + director` 출판사 복구: 4.0 작업이다.
- 수동 삽화 모듈 HUD: 재현 자료가 없어 보류한다.

### 1.3 증거 단계

| 단계 | 의미 | 3.9.9 현재 상태 |
|---|---|---|
| `source_implemented` | production source에 구현 연결 | 대부분 완료; endpoint UI와 systemd 사용자 지정 data root 전달 미처리 |
| `automated_regression_verified` | 단위·통합·smoke 회귀 | 핵심 범위 통과; `Supervisor` 옛 표시를 요구하는 assertion 1건 남음 |
| `package_built` | 현재 source로 설치 패키지 생성 | 3.9.5 중간 빌드만 존재; 3.9.9 패키지 미생성 |
| `loaded_artifact_verified` | 생성한 plugin/package를 실제 RisuAI에 로드 | 3.9.5에서 일부 확인; 3.9.9 미실행 |
| `live_provider_verified` | 실제 provider 요청·응답 | Luna 평론가 토큰 4턴과 일부 출판사 호출 확인; 반복 품질 검증 필요 |
| `real_db_vector_verified` | 실제 MariaDB·Chroma 저장·검색 | 3턴·리롤 MariaDB 읽기 감사만 완료; Voyage·Chroma·업데이트 E2E 미완료 |
| `release_verified` | 공개 artifact·hash·설치·업데이트 | 미완료 |

### 1.4 3.9.0 사용자의 3.9.9 전환 계약

3.9.0과 개발 중간 테스트판 3.9.5는 UI 자동 업데이트 출발점이 아니다. 이 설치본에서 3.9.9로 전환할 때는
예외 없이 한 번의 완전 신규 설치를 사용한다. 기존 실행기나 updater를 보완하거나 이어서 사용하지 않는다.

신규 설치에서 폐기하는 것:

- 기존 Archive Center 프로그램 파일
- 기존 backend·실행기·updater·`.updates`
- 기존 package manifest와 package-local runtime binary

신규 설치로 넘기는 것:

- MariaDB 데이터
- ChromaDB 데이터가 있으면 해당 데이터
- `.env.full.local` 등 사용자가 직접 만든 로컬 설정과 secret
- RisuAI `pluginStorage`에 저장된 plugin 설정

DB·설정 보존은 기존 프로그램 파일을 새 설치에 섞는 호환 경로가 아니다. 모든 Archive Center·MariaDB·ChromaDB
프로세스를 종료하고 사용자 데이터를 별도로 보존한 뒤, 새 3.9.9 package와 실행기를 설치하고 보존한 데이터만
새 stable data root에 연결하거나 검증 복사한다. 원본 DB는 새 설치의 `/ready`, `/version`, 저장·조회 확인이
끝나기 전까지 삭제하지 않는다.

OS별 현재 증거와 남은 경계:

- Windows: package-local `.runtime`의 MariaDB·Chroma를 stable data root로 검증 복사하고 원본을 보존하는 source와
  자동 검사가 있다. 실제 사용자 3.9.0 DB를 사용한 전환 검증은 남아 있다.
- macOS·Termux·비-systemd POSIX: `ARCHIVE_CENTER_DATA_DIR`를 사용한 수동 data-root 이전 절차가 있다. 실제 대상
  장치 검증은 남아 있다.
- Linux·Ubuntu systemd: 사용자 지정 `ARCHIVE_CENTER_DATA_DIR`가 release helper까지 전달되지 않는 현재 미처리
  항목을 닫기 전에는 기존 DB 전환을 지원 완료로 표시하지 않는다.

현재 3.9.9 최종 package는 아직 생성되지 않았으므로 사용자가 지금 실행할 수 있는 3.9.0 → 3.9.9 전환은 없다.
3.9.9 최종 package가 공개된 뒤 위 신규 설치를 한 번 완료한 설치본부터, 이후 4.0·4.2 등 지원 package로의
이동을 설정의 `지금 업데이트` 한 번으로 처리한다.

## 2. 전체 피드백·작업 통합표

상태 표기에서 `완료`는 source와 자동 회귀까지를 뜻한다. `실환경 필요`가 붙은 항목은 완료를 취소하는
표시가 아니라 source 증거와 실제 제품 증거를 구분하는 표시다.

| ID | 피드백 또는 작업 | 처리 결과 | 현재 상태 | 상세 기록 |
|---|---|---|---|---|
| F01 | Windows 기존 `.runtime` DB 수동 이전 실패 | PowerShell 단일 MariaDB 후보 선택식을 수정하고 원본 보존 | 완료, 사용자 DB 확인 필요 | §3 |
| F02 | 후처리보다 평론가가 먼저 시작하거나 본문을 놓침 | 현재는 `afterRequest` 완료 응답만 기존 complete-turn으로 전달; 별도 output listener 제거 | 제한 지원, 스트리밍 chunk 비지원 | §4, §16 |
| F03 | `voyage-context-4` 미지원 | 동일 source revision의 공개 memory·근거·정밀 기억을 한 문서의 다중 chunk로 호출하고 검색·재시도·재색인 연결 | 완료, 실제 Voyage·DB·Chroma 필요 | §5 |
| F04 | reasoning effort `max`가 UI에서 `none`으로 보임 | 초기 option에 전체 값을 넣고 저장값을 그대로 재선택 | 완료, 실제 RisuAI 새로고침 필요 | §6 |
| F05 | 수동 삽화 모듈 호출 때 턴 HUD 표시 | submodel 자체 결함은 재현되지 않았고 `model`·`sendChat`·기존 HUD 재표시 가능성만 확인 | 보류 | §7 |
| F06 | 설치가 지나치게 복잡함 | Windows 실행기와 POSIX 한 줄 설치, stable launcher, 준비된 dependency 재사용 | 완료, 실제 OS 필요 | §11, §12, §30 |
| F07 | UI 업데이트 버튼이 알림만 하고 전체 패키지를 확실히 적용하지 못함 | 3.9.9 이후 설치본에 OS·CPU 후보 검증, 전체 managed file, 누적 migration, restart·health·commit·package rollback 계약 연결; 3.9.0·3.9.5에는 적용하지 않음 | source 완료, 실제 POSIX·DB 업데이트 필요 | §1.4, §11, §31 |
| F08 | macOS·Termux가 timeout/readiness 미선언으로 시작 실패 | 공개 launcher에 external/request/readiness/poll 값을 전달 | 완료, 실제 장치 필요 | §12, §14 |
| F09 | endpoint 없이 저장하면 실제 저장과 UI `저장 실패`가 모순 | endpoint 필수 유지와 수정 방향만 확정 | **미처리** | §13 |
| F10 | POSIX 신규 설치에서 기존 MariaDB·Chroma만 유지 | 수동 data-root 이전 절차와 stable data root 구현; systemd가 사용자 지정 data root를 helper에 넘기는 경로는 남음 | **부분 완료** | §14 |
| F11 | 평론가가 `allbefore` 전체를 받아 토큰·시간 폭증 | 현재 턴 전체 + 직전 canonical 1턴 + 관련 DB 자료만 기존 동적 예산으로 선택; 고정 프롬프트 중복 제거 | 완료, 장기 품질 확인 필요 | §17, §22, §29.1 |
| F12 | `character_deltas.name` 누락으로 인물 상태가 `missing_name` 폐기 | JSON 예시에 `name`을 복구하고 인물 상태 저장 회귀 추가; 직접 근거 정책은 건드리지 않음 | 완료 | §33 |
| F13 | 평론가 JSON 자료형 오류와 같은 요청 반복 호출 | `evidence_excerpts` 문자열 배열 계약 명시, 수동 HUD 재시도 복구, 동일 schema 오류 자동 반복 중단 | 완료, 실제 실패 재현 필요 | §25, §26 |
| F14 | 최초 평론가와 재처리 입력이 달라짐 | bounded input·언어·prompt hash를 source revision에 보존하고 같은 입력만 재사용 | 완료, 실제 재처리 확인 필요 | §26 |
| F15 | `retry_after`가 외부 wake 없이는 실행되지 않음 | 필요성 검토 후 구현하지 않기로 결정 | 보류, 3.9.9 제외 | §29.1 |
| F16 | 같은 인물·개체가 새 ID로 분리됨 | 기존 3.9.0 canonical ID를 유지하고 같은 정식 이름+종류+세션의 신규 occurrence만 연결 | 완료 | §20, §23, §24 |
| F17 | 콜드 스타트 파생 저장 MariaDB 1213 deadlock | 충돌 구간 직렬화와 1213 전체 transaction 재시도 | 완료, 실제 동시 실행 필요 | §21 |
| F18 | 리롤 후 이전 세계 규칙 유실 | 턴별 world-rule 버전 저장과 최신 source-turn 조회로 이전 값 복원 | 완료, 실제 리롤 확인 필요 | §23.3 |
| F19 | 리롤·DB 저장 결과와 고아 데이터 의심 | 3턴·리롤 실제 MariaDB 감사, superseded fence 확인; outbox 무한 증가 가능성은 별도 유지보수 후보로 기록 | 부분 실환경 확인 | §23.1, §23.5 |
| F20 | 모델 입력에 SHA·UUID 내부 ID가 노출됨 | 모델용 문자열에서만 내부 식별자를 제거하고 DB ID·lineage는 유지 | 완료, 실제 payload 확인 필요 | §27.1 |
| F21 | 감독관·출판사 명칭과 실패 원인이 혼란스러움 | 외부 명칭을 `출판사 LLM`으로 통일하고 timeout·empty·4xx·unavailable 등을 구분 | 완료 | §23.4, §27.2 |
| F22 | Ollama Cloud 출판사가 HTTP 200인데 빈 응답 | 저장된 reasoning/max-token을 실제 요청에 전달하고 Ollama `none`을 생략하지 않음 | 완료, provider 반복 확인 필요 | §28 |
| F23 | 1.0 출판사의 `book_author + director` 역할이 약화됨 | 3.9에서는 오류 수정까지만 하고 역할 복구는 4.0 단일 Publisher 계획으로 이관 | 4.0 이관 | §28.3, §29.4 |
| F24 | Historical Queue 누적·중복 표시·복구 버튼 무반응 | pending/transport 용량 분리, queue 종류·상태·reason 집계, terminal 정리, backend recovery HUD 연결 | 완료, pluginStorage·MariaDB 동시 확인 필요 | §15.2, §29.2 |
| F25 | Ctrl+C·창 종료 뒤 Go·MariaDB·Chroma 잔류 | Windows·POSIX 기존 launcher가 자신이 시작한 자식 process를 종료하고 제한시간 뒤 강제 종료 | 완료, 실제 OS 종료 확인 필요 | §15.2 |
| F26 | 삭제 감지 HUD가 UI를 열어야 보임 | 공식 삭제 event 없이 만든 observer/click 구현을 제거; 화면 기록 삭제는 Archive Center 기억 삭제와 분리 | 비지원·정책 확정 | §18, §29.3 |
| F27 | 리롤과 삭제 후 리롤의 logical turn 연결 | 기존 host observation과 backend logical-turn replacement 유지, 리롤 전 파생 결과 supersede | 완료, 실제 PocketRisu 확인 필요 | §23.1, §29.3 |
| F28 | Host turn과 Backend turn이 크게 어긋나 저장·확정이 실패 | RisuAI message index는 관찰값으로만 사용하고 새 canonical turn은 DB tail+1, 리롤은 기존 logical turn으로 해석 | 완료, 실제 장기 세션 확인 필요 | §34 |
| F29 | 피드백 완성판 버전과 향후 update 기준 혼재 | 3.9.0·3.9.5는 DB·설정만 보존하는 1회 신규 설치 대상으로 두고 제품·패키지·자동 업데이트 floor를 3.9.9로 변경 | source 완료, 3.9.9 패키지 대기 | §1.4, §31, §32 |

### 2.1 버전·checkpoint·패키지 흐름

| 시점 | 결과 | 의미 |
|---|---|---|
| 3.9.4 폐기 후 | 공식 3.9.0 source에서 피드백 작업 재시작 | 폐기된 3.9.4 구현을 현재 근거로 사용하지 않음 |
| `b8a912d` | 최초 피드백 묶음 checkpoint | source만 기록; `.runtime`·secret·테스트 패키지 제외 |
| `c2aae3c` | 후속 runtime 교정 checkpoint | 이후 작업은 현재 dirty worktree에 이어짐 |
| `3.9.5-1st-test` | 작업 단계마다 같은 폴더를 반복 갱신 | 당시 실사용·DB·provider 확인용 중간 artifact |
| 2026-08-09 | 제품·plugin·패키지 기본값과 update floor를 3.9.9로 변경 | 최초 정식 자동 업데이트 출발점을 3.9.9로 확정 |
| 현재 | 3.9.9 package 미생성 | 남은 source·회귀 작업 뒤 새 6종 package 필요 |

### 2.2 평론가 입력량 실측 기록

동일한 GPT-5.6-Luna 평론가에서 사용자가 gateway로 확인한 값이다. 모델 속도와 출력량이 섞이므로 token
감소를 곧바로 latency 보장으로 해석하지 않는다.

| 단계 | 논리 턴 | 호출당 token | 판정 |
|---|---|---:|---|
| 변경 전 | 단일 호출 | 29.8K | `allbefore` 전체 전달이 있던 기준값 |
| A-1 1차 축소 후 | 1턴 표본 | 16.5K | 기준 대비 약 44.6% 감소 |
| 프롬프트 중복 정리 후 | 1턴 | 11.5K | 약 21초 |
| 프롬프트 중복 정리 후 | 2턴 | 14.1K | 호출당 gateway 값 |
| 프롬프트 중복 정리 후 | 3턴 | 14.3K | 호출당 gateway 값 |
| 같은 3턴 삭제 후 리롤 | 리롤 | 약 15.3K | 2회 합계 29.6K에서 기존 14.3K를 뺀 역산값 |
| 후속 실사용 | 9턴 | 12.6K | 사용자 보고 단일 호출값 |
| 후속 실사용 | 신규 턴 | 14.7K | 사용자 보고 단일 호출값 |

7턴의 `CRITIC_SCHEMA_INVALID` 사례에서는 최초 1회 뒤 동일 재처리가 4회 더 실행되어 총 5회 호출됐다.
§25에서 자료형 계약을 명확히 했고 §26에서 동일 schema 오류의 자동 반복을 중단했다.

> [!CAUTION]
> **스트리밍 원문 인식 지원 불가 (`unsupported`)**
>
> Archive Center 3.9.9는 RisuAI 또는 PocketRisu가 화면에 표시하는 스트리밍 응답의 중간
> chunk/token을 실시간 원문으로 읽어 저장하거나 평론가에 전달하는 기능을 지원하지 않는다.
> RisuAI의 `addRisuChatListener("output", ...)`은 스트리밍이 모두 끝난 뒤 확정된 출력을
> 알리는 완료 이벤트이지 스트리밍 조각을 읽는 API가 아니다. PocketRisu에는 현재 동일한
> 공식 완료 이벤트 계약도 확인되지 않았다. 따라서 스트리밍 모드의 원문 저장·평론가 호출은
> 호스트 공통 지원 기능으로 보장하거나 배포 기능으로 표기하지 않는다.
>
> 이 제한을 우회하기 위한 DOM 감시, polling, 고정 대기 시간, 네트워크 가로채기 또는 별도
> fallback 저장 경로는 만들지 않는다. 스트리밍 중간 조각 인식이 필요하면 RisuAI와 PocketRisu
> 본체가 공식 plugin API로 해당 lifecycle 이벤트를 제공해야 한다.

## 3. 피드백 1-A — 기존 DB 수동 적용

### 3.1 확인된 원인

Windows 전체 패키지 실행기의 `Import-LegacyRuntimeDataOnce`가 MariaDB 후보를 정확히 하나 찾았을 때
PowerShell 배열 원소를 문자열로 변환하는 식이 배열 인덱싱보다 먼저 평가될 수 있었다.

기존 식:

```powershell
Source = [string]$mariaCandidates[0]
```

수정 식:

```powershell
Source = [string](@($mariaCandidates)[0])
```

### 3.2 동작 계약

- 기존 패키지의 `.runtime` 위치는 탐색 대상으로 유지한다.
- MariaDB 후보가 하나면 그 경로 하나만 새 managed data root로 복사한다.
- 후보가 여러 개면 임의로 하나를 고르지 않는다.
- 원본 `.runtime`과 기존 DB는 삭제하거나 이동하지 않는다.
- DB schema와 저장 위치 계약은 바꾸지 않는다.
- 사용자 DB를 자동 초기화하거나 rollback하지 않는다.

### 3.3 변경 파일

- `ops/full-package/scripts/start-full-windows.ps1`

### 3.4 검증

실행:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File ops\legacy-data-import-smoke.ps1
```

결과:

- `archive-center.legacy-data-import-smoke.v1`: `status=ok`
- launcher와 external installer의 live-port 거부 검사 통과
- IPv4·IPv6 및 configured-port 거부 검사 통과
- offline atomic promotion 통과
- legacy source 보존 확인

실제 사용자 DB를 대상으로 한 수동 이전은 수행하지 않았다.

## 4. 피드백 1-B — 후처리 완료 이후 원문 저장과 평론가 호출

### 4.0 최종 지원 결정

**Archive Center 3.9.0의 스트리밍 원문 인식은 지원 불가다.** 사용자가 스트리밍 출력을
화면에서 계속 받는 동안 Archive Center가 그 중간 조각을 읽어 원문을 구성하는 기능은 없다.
스트리밍 완료 뒤 전달되는 최종 출력은 완료 후 관찰이며, 이를 스트리밍 실시간 인식 지원으로
표기하지 않는다.

아래 4.2~4.6은 `output` 완료 이벤트를 이용해 최종 출력만 받으려 했던 이전 구현 시도와 검증
기록이다. 스트리밍 chunk 인식이 구현되었거나 지원된다는 근거로 사용하지 않는다.

### 4.1 문제

`afterRequest` 직후에는 RisuAI의 본문 생성 결과가 존재하더라도 다른 후처리 플러그인이 최종 표시 문장을
아직 교체하지 않았을 수 있다. 이 값을 즉시 canonical raw로 저장하고 평론가를 호출하면 사용자가 최종적으로
본 출력과 Archive Center가 저장·분석한 출력이 달라진다.

### 4.2 이전 구현 시도 기록 — 지원 근거 아님

```text
beforeRequest
  -> 감독관/본문 요청 준비
  -> afterRequest에서 응답 후보만 기록
  -> RisuAI 공식 output 이벤트 대기
  -> output 이벤트가 가리키는 활성 assistant 메시지 확인
  -> 최종 표시 원문 확정
  -> raw 저장
  -> 평론가 호출
  -> derived memory·vector 처리
```

핵심 변경:

- `afterRequest`는 최종 저장을 시작하지 않고 `candidate_observed` 상태와 후보 hash만 기록한다.
- RisuAI의 `addRisuChatListener("output", ...)`가 전달한 실제 chat/message를 최종 출력으로 사용한다.
- 사용자 메시지 위치, assistant 메시지 위치, character/chat index와 요청 correlation을 확인한다.
- 같은 요청이 이미 저장을 시작했으면 중복 output 이벤트를 다시 처리하지 않는다.
- 후처리 플러그인 이름이나 출력 문구를 하드코딩하지 않는다.
- 기존 post-output 교체 예약 경로와 `afterRequest` 직접 확정 경로는 제거했다.
- unload 시 등록한 output listener를 해제한다.

### 4.3 유지한 경계

- 중복 요청 방지는 유지한다.
- `Archive Center.js`는 RisuAI hook과 표시 원문 관찰만 담당한다.
- canonical 저장·평론가·derived 처리 권한은 Go backend에 유지한다.
- DOM polling, 고정 대기 시간, plugin-name 예외, 별도 fallback 저장 경로는 추가하지 않았다.

### 4.4 변경 파일

- `Archive Center.js`
- `go-service/cmd/js-route-variant-smoke/main_part02_test.go`
- `go-service/cmd/js-route-variant-smoke/main_part11_test.go`
- `go-service/cmd/js-route-variant-smoke/main_part12_test.go`

현재 `Archive Center.js` diff: `+224 / -283`.

### 4.5 주요 회귀 검사

- `afterRequest`가 저장을 시작하지 않고 output 신호를 기다리는지
- 후처리된 최종 메시지를 output listener가 받는지
- 동일 output 이벤트가 정확히 한 번만 저장되는지
- 활성 chat을 별도로 재추정하지 않고 output event의 message를 사용하는지
- output final이 선택적 orchestration pending state 없이도 기존 요청과 연결되는지
- 기존 post-output secondary replacement 경로가 남지 않았는지

### 4.6 최종 검증 경계

- 현재 source는 `addRisuChatListener`가 실제 host에 존재할 때 listener를 등록한다.
- 실제 사용 중인 RisuAI/PocketRisu 빌드가 이 API와 `output` 이벤트를 제공하는지는 아직 로드 검증하지 않았다.
- API가 없는 host에서 사용할 대체 DOM watcher나 시간 기반 fallback은 만들지 않았다.
- 현재 source는 등록 요청·callback·등록 예외를 lifecycle trace로 기록하지만, API 자체가 없는 경우를 위한
  별도 stable error code는 아직 없다.
- 위 listener는 스트리밍 완료 후 최종 출력을 받는 수단일 뿐, 스트리밍 중간 조각을 읽지 않는다.
- RisuAI와 PocketRisu 공통 스트리밍 원문 인식 지원을 주장하지 않는다.
- 배포 문서와 테스트 결과에서 스트리밍 원문 인식을 `implemented`, `verified` 또는 `supported`로 표기하지 않는다.

## 5. 피드백 2 — Voyage Context 지원

### 5.1 적용 모델과 endpoint

- `voyage-context-*`: `/v1/contextualizedembeddings`
- 일반 Voyage 모델: 기존 `/v1/embeddings`
- context endpoint는 설정된 Voyage endpoint의 base 또는 `/embeddings` 경로에서 정확히 변환한다.
- endpoint가 비어 있을 때 임의의 기본 endpoint를 넣지 않는다.
- 일반 Voyage 요청 형식은 변경하지 않는다.

문서 저장 요청:

```json
{
  "model": "voyage-context-4",
  "inputs": [["chunk 1", "chunk 2", "chunk 3"]],
  "input_type": "document"
}
```

검색 요청:

```json
{
  "model": "voyage-context-4",
  "inputs": [["query"]],
  "input_type": "query"
}
```

### 5.2 다중 청크 문맥화

현재 턴의 동일 source revision에서 만들어진 다음 공개 검색 자료를 한 inner document group으로 보낸다.

- 턴 요약 memory
- 직접 근거
- 일반 공개 precise memory

각 자료를 하나씩 독립 호출하지 않는다. Voyage 응답의 outer document index와 inner chunk index를 검증하고,
반환 순서가 바뀌어도 원래 자료 위치에 다시 연결한다. 내용이 완전히 같은 두 청크도 텍스트 값으로 합치지 않고
서로 다른 index로 유지한다.

다음 자료는 일반 검색용 context group에 넣지 않는다.

- `owner_private`
- `restricted`
- `reveal_required`
- knowledge-holder 전용 자료
- 관점·주관 기억

### 5.3 저장과 재시도

- 성공한 contextualized embedding과 실제 응답 model을 memory admission 및 vector outbox에 전달한다.
- provider 호출이 실패했을 때도 outbox 문서에 전체 sibling chunk 목록과 대상 index를 보존한다.
- 재시도는 대상 청크 하나만 다시 보내지 않고 전체 sibling group을 다시 문맥화한다.
- Chroma metadata에는 재시도용 sibling 원문 목록을 남기지 않는다.
- Chroma가 설정되지 않은 환경에서도 canonical MariaDB memory용 요약 embedding은 생성한다.
- DB table·column·migration은 추가하지 않았다. context 전달 필드는 commit/outbox용 비영속 필드다.

### 5.4 검색·유지보수 연결

Context 모델 경로를 다음 기존 owner에 연결했다.

- prepare-turn memory recall query
- reference canon-base query
- reference recall query
- reference vector search query
- 현재 턴 memory/evidence/precise admission
- memory vector outbox materialization과 retry
- foreground admin memory reindex
- background admin memory reindex
- evidence/world-rule derived reindex
- reference material reindex
- session migration vector reindex
- 단건 status·world-rule 등 기존 document embedding 호출

### 5.5 변경 파일

Provider·현재 턴·outbox:

- `go-service/internal/httpapi/turn_extraction_vector.go`
- `go-service/internal/httpapi/turn_extraction_persist.go`
- `go-service/internal/httpapi/turn_memory_admission.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor.go`

검색·재색인·이전:

- `go-service/internal/httpapi/prepare_turn_recall.go`
- `go-service/internal/httpapi/group_reference_canon_base.go`
- `go-service/internal/httpapi/group_reference_recall.go`
- `go-service/internal/httpapi/group_reference_vectors.go`
- `go-service/internal/httpapi/group_admin_reindex.go`
- `go-service/internal/httpapi/group_admin_vector_maintenance.go`
- `go-service/internal/httpapi/group_session_migration.go`

Outbox 전달용 비영속 필드:

- `go-service/internal/store/precise_memory.go`
- `go-service/internal/store/mariadb_memory_admission.go`
- `go-service/internal/store/mariadb_precise_memory.go`

회귀 테스트:

- `go-service/internal/httpapi/group_proxy_test.go`
- `go-service/internal/httpapi/memory_admission_worker_test.go`
- `go-service/internal/httpapi/memory_vector_outbox_processor_test.go`
- `go-service/internal/httpapi/character_perspective_test.go`

Voyage Context 작업의 JavaScript 증감: `+0 / -0`.  
Voyage Context 관련 Go·테스트 diff: `+840 / -33`.

### 5.6 주요 회귀 검사

- context 전용 endpoint 사용
- `inputs`가 한 문서의 여러 sibling chunk를 포함하는 중첩 배열인지
- document/query `input_type` 구분
- 역순 응답 index의 정확한 원위치 매핑
- 같은 텍스트를 가진 서로 다른 청크의 독립 매핑
- 일반 Voyage endpoint와 기존 payload 유지
- 현재 턴 memory·evidence·public precise가 provider 한 번에 전달되는지
- private/perspective precise가 일반 group에서 제외되는지
- Chroma가 없어도 MariaDB memory embedding이 생성되는지
- outbox retry가 전체 sibling group과 고정 index를 유지하는지
- retry 전용 context metadata가 Chroma에 노출되지 않는지

### 5.7 남은 검증 경계

- 실제 Voyage API key를 사용한 요청은 실행하지 않았다.
- 실제 Voyage rate/token 제한과 장문 archive의 provider 응답은 확인하지 않았다.
- 실제 MariaDB admission/outbox row와 Chroma embedding 문서를 함께 읽어보는 통합 검증은 하지 않았다.
- 실제 모델 전환 후 기존 DB 전체 재색인 시간·비용·검색 품질 비교는 하지 않았다.
- 현재 변경으로 테스트 빌드나 배포 패키지를 만들지 않았다.

## 6. 피드백 3 — reasoning effort UI 표시

이 피드백은 Voyage Context 피드백과 같은 대화에 포함됐지만 별도 문제로 분리한다.

### 6.1 실제 저장 위치

- reasoning effort를 포함한 플러그인 설정의 영속 원본은 MariaDB가 아니라 RisuAI `pluginStorage`다.
- 플러그인 시작 시 `loadSettings()`가 `pluginStorage`에서 설정을 복원한다.
- 저장 시 `saveSettings()`가 `pluginStorage`에 기록하고 `syncConfigToBackend()`로 현재 설정을 백엔드에 전달한다.
- 백엔드 `/config/update`는 설정을 실행 중 메모리에만 보관하며 응답도 `persisted: false`,
  `persistence: runtime_only`로 명시한다.
- API 키나 reasoning 설정을 MariaDB에 저장하는 변경은 하지 않았다.

관련 source:

- `Archive Center.js`: `loadSettings`, `saveSettings`, `syncConfigToBackend`
- `go-service/internal/httpapi/group_health.go`: `handleConfigUpdate`

### 6.2 직접 원인

출판사 `mo-pluginMainReasoningEffort`와 평론가 `mo-subLlmReasoningEffort`의 최초 HTML 선택 목록에는
`none`, `low`, `medium`, `high`, `enable`, `disable`만 있었다. `minimal`, `xhigh`, `max`가 없었기 때문에
`pluginStorage`에서 `max`를 정상 복원해도 브라우저가 해당 option을 찾지 못해 최초 값이 `none`으로 떨어졌다.

그 뒤 provider/model에 맞는 동적 목록이 만들어질 때는 이미 선택 요소에서 읽은 `none`이 다시 적용됐다.
이 상태에서 저장 버튼을 누르면 `pluginStorage`의 실제 값도 `none`으로 덮어쓸 수 있었다.

### 6.3 변경 내용

- 출판사와 평론가의 최초 reasoning effort 선택 목록에 `minimal`, `xhigh`, `max`를 추가했다.
- 각 option이 저장된 설정값을 직접 `selected`로 복원하도록 기존 방식과 동일하게 연결했다.
- 동적 provider/model 판정, 설정 저장, 백엔드 동기화, provider request 생성은 변경하지 않았다.
- 별도 fallback·watcher·cache·DB 설정 저장 경로를 추가하지 않았다.

변경 파일:

- `Archive Center.js`
- `go-service/cmd/js-route-variant-smoke/main_part10_test.go`

### 6.4 회귀 검사

`TestArchiveCenterJSReasoningEffortInitialSelectPreservesStoredMax`를 추가했다. 이 검사는 출판사와 평론가의
최초 선택 목록 모두에 전체 지원 option이 존재하고, 저장값이 `max`일 때 최초 렌더링에서 `max`가 직접
선택되는지 확인한다.

통과:

```text
node --check "Archive Center.js"
go test ./cmd/js-route-variant-smoke -run 'TestArchiveCenterJS(GLM52ReasoningEffortMarkers|ReasoningEffortInitialSelectPreservesStoredMax)$' -count=1
go test ./cmd/js-route-variant-smoke -count=1
```

실제 RisuAI에 현재 source를 로드한 뒤 `max` 저장 → 새로고침 → 재표시 → 다시 저장까지 확인하는 live 검증은
아직 실행하지 않았다.

## 7. 피드백 4 — 수동 삽화 모듈 호출과 턴 HUD

### 7.1 확인된 요청 타입 경계

Archive Center의 `beforeRequest`·`afterRequest`·저장·문맥 주입 경로에는 RisuAI 훅의 `type`이 비어 있거나
정확히 `model`인 요청만 들어간다. `submodel`, `otherAx` 등 다른 타입은 `onBeforeRequest()` 초입에서 원본
payload를 그대로 반환하므로 턴 HUD, 감독관, 저장, 평론가가 시작되지 않는다.

`primeTurnWorkflowHUD()`도 이 타입 검사 뒤에 호출된다. 따라서 수동 삽화 모듈 버튼을 누른 직후 새 턴 HUD가
열렸다면 다음 중 하나다.

1. 삽화 모듈이 보조 모델 설정을 사용하더라도 RisuAI 훅에는 요청을 `model` 또는 빈 타입으로 전달했다.
2. 삽화 모듈이 일반 채팅 생성을 실행했고, 이미 채팅에 들어간 user/assistant 쌍을 Archive Center 백필이
   별도의 완료 턴으로 처리하기 시작했다.

“보조 모델 슬롯을 사용한다”는 설정과 RisuAI replacer가 Archive Center에 전달하는 훅 `type`은 같은 정보가
아니다. 슬롯이 보조 모델이어도 호출 코드가 `mode: "model"`을 사용하면 Archive Center에는 본문 요청으로 보인다.

관련 source:

- `Archive Center.js`: `isNarrativeType`, `isSaveType`, `isContextInjectionType`
- `Archive Center.js`: `onBeforeRequest`, `primeTurnWorkflowHUD`
- `Archive Center.js`: `buildCompletedTurnPairsFromActiveChatMessages`, `ensureActiveChatCompletedTurnsBackfilled`

### 7.2 `sendChat`과 백필 경계

RisuAI 공식 Plugin API v3의 `sendChat(message)`는 사용자 메시지를 보낸 것처럼 일반 채팅 처리 흐름을 실행한다.
Archive Center의 active-chat 백필은 저장된 채팅에서 user/assistant 역할, 내용, 메시지 위치를 읽지만 그 쌍을 만든
플러그인의 원래 `runLLMModel` mode나 모듈 이름은 받을 수 없다.

따라서 삽화 모듈이 `sendChat`으로 실제 일반 user/assistant 메시지를 채팅에 추가하면, 다음 플러그인 시작,
본문 `beforeRequest`, 타임라인 새로고침 등의 백필 시점에 일반 완료 턴 후보가 될 수 있다. 이 경계는 현재 source에
실제로 존재한다.

반대로 삽화 결과가 채팅에 일반 user/assistant 쌍으로 추가되지 않고 호출 타입도 실제 `submodel`이라면 Archive
Center 턴 처리로 들어오지 않는다.

### 7.3 이번 확인의 판정

- 사용자 관찰을 다시 반영하면 삽화 요청 자체가 새 HUD를 연 것이 아니라, 닫았던 기존 HUD가 다른 작업에 걸려
  다시 열린 상황일 가능성도 있다.
- 진짜 `submodel` 호출에서 턴 HUD가 뜨는 source 결함은 현재 코드와 회귀 fixture에서는 재현되지 않았다.
- `model` 또는 빈 타입으로 들어온 삽화 호출은 현재 정상 본문 요청과 구분할 수 없다.
- `sendChat`으로 생성된 일반 채팅쌍은 나중에 백필될 수 있다.
- HUD가 표시됐다는 사실만으로 DB 저장까지 완료됐다고 판단할 수는 없다. HUD는 저장 전 `beforeRequest`에서 먼저 열린다.
- NAI 구독은 원인 판별에 필요하지 않다. 해당 삽화 모듈 source의 `runLLMModel.mode`/`sendChat` 사용 여부 또는
  한 번의 실제 `beforeRequest` type 기록이면 두 경로를 구분할 수 있다.
- 플러그인 이름이나 삽화 문구를 하드코딩해 차단하는 코드는 추가하지 않았다.

### 7.4 자동 확인

통과:

```text
TestArchiveCenterJSOnlyModelTypeEntersPersistence
TestBeforeRequestNonModelSkipsPrepareTurnRuntime
TestArchiveCenterJSActiveChatCompleteTurnBackfillMarkers
TestRisuMessageIndexesDriveLogicalTurnPairs
```

실제 `인레이 마개조 삽화 모듈` source와 실행 로그는 제공되지 않았으므로 그 모듈이 두 경로 중 어느 쪽을
사용하는지는 아직 확정하지 않았다. 새 HUD인지 닫힌 HUD의 재표시인지도 구분되지 않았으므로 피드백 4는
재현 자료가 생길 때까지 보류하고 다음 피드백으로 이동한다. 이번 항목에서는 production source를 변경하지 않았다.

## 8. 피드백 1~6 반영 당시 통합 검증 결과

### 8.1 통과

```text
node --check "Archive Center.js"
go test ./internal/httpapi ./internal/store -count=1
go test ./cmd/js-route-variant-smoke -count=1
git diff --check
ops/legacy-data-import-smoke.ps1
```

`go test ./... -count=1` 첫 실행에서는 `node`가 PATH에 없어 `cmd/js-route-variant-smoke`의 세 검사가 환경 오류로
실패했다. 같은 패키지에 bundled Node 경로를 `ARCHIVE_CENTER_NODE_BINARY`로 지정해 다시 실행했으며 통과했다.
첫 실행에서 그 외 모든 Go 패키지는 통과했다.

### 8.2 미실행

- 실제 RisuAI/PocketRisu output event
- 후처리 플러그인을 켠 실제 본문 생성
- 실제 사용자 MariaDB 이전
- 실제 MariaDB·Chroma 저장 및 검색
- 실제 Voyage Context API 호출
- 실제 RisuAI에서 reasoning effort `max` 저장 후 새로고침·재저장
- Windows 설치 패키지 생성과 설치
- 업데이트·rollback·프로세스 종료 검증
- 공개 release artifact 생성

## 9. 피드백 1~6 반영 당시 diff 범위

현재 worktree에는 피드백 1~5 변경이 함께 들어 있다. `Archive Center.js`와 일부 JS smoke 파일은 여러 피드백의
변경이 겹치므로 전체 `git diff --numstat`만으로 피드백별 줄 수를 다시 분리하지 않는다.

이번 updater 항목은 production/test 16개 파일에 연결됐다. updater 때문에 바뀐 JavaScript 부분만 따로 계산하면
`+20/-36`이며, 남은 JavaScript 역할은 UI 표시와 `/update/apply` 단일 전송이다. 플랫폼·CPU·asset·SHA·버전
선택, 패키지 적용, DB migration, commit·rollback은 Go와 managed launcher가 담당한다.

이 worktree는 아직 commit되지 않았다. 문서 변경은 production/test 파일 수에 포함하지 않는다.

이번 3.9.0 피드백 작업에서 변경하지 않은 영역:

- 턴 삭제·리롤 정책
- `/del`·`/cut` Archive Center DB 보존 정책
- 기존 DB table schema 자체
- 설치 구조 전면 개편
- API 키·reasoning 설정의 MariaDB 저장 정책
- provider endpoint 기본값
- 새로운 polling·watcher·cache·별도 fallback 경로

## 10. 피드백 1~6 당시의 다음 확인 순서

1. 현재 피드백 변경을 포함한 테스트 빌드를 생성한다.
2. 실제 RisuAI에서 reasoning effort `max` 저장 → 새로고침 → `max` 재표시 → 다시 저장을 확인한다.
3. output listener가 있는 실제 RisuAI/PocketRisu에서 일반 출력과 후처리 출력을 각각 확인한다.
4. raw 저장 후 평론가 호출 순서와 HUD를 확인한다.
5. 기존 `.runtime` DB 복사본으로 legacy import를 확인한다.
6. `voyage-context-4` 실제 key로 다중 청크 요청과 query 검색을 확인한다.
7. MariaDB embedding model·outbox와 Chroma 문서를 함께 확인한다.

## 11. 피드백 5 — 한 번 클릭하는 완전 자동 업데이트

### 11.1 지원 시작점

기존 3.9.0 패키지 중 `PACKAGE_FILE_MANIFEST.json`에 `package_version`이 없는 설치본을 위한 별도 예외
경로는 만들지 않았다. 3.9.0과 개발 중간 테스트판 3.9.5는 기존 프로그램 파일을 폐기하고 DB·설정만 보존하는
3.9.9 완전 신규 설치를 한 번 거쳐야 한다. 선택 사항이나 실패 시 대체 절차가 아니며, 이 두 버전에서 UI의
`지금 업데이트`를 직접 사용하는 경로는 지원하지 않는다. 3.9.9 package는 버전·전체 관리 파일·DB migration
계약을 모두 포함하며, 정상 설치와 데이터 확인이 끝난 3.9.9 설치본부터 이후 버전을 설정의 `지금 업데이트`
버튼 한 번으로 자동 업데이트하는 것을 정식 계약으로 삼는다.

### 11.2 구현된 흐름

```text
지금 업데이트 1회
  -> POST /update/apply 1회
  -> 실행 중인 백엔드의 OS·CPU로 GitHub release 자산 선택
  -> SHA256 검증 후 package/.updates에 staging
  -> UI 응답을 완전히 전송
  -> 백엔드 exit 75
  -> 기존 managed launcher가 즉시 update transaction 재진입
  -> 전체 관리 파일 추가·교체·삭제
  -> migrations/*.sql 전체를 이름순 적용 + embedded schema 적용
  -> candidate /ready + exact /version
  -> commit
```

사용자가 업데이트 확인을 먼저 누르거나, 별도 명령을 실행하거나, 프로그램을 수동 재시작하거나, 두 번째 확인
버튼을 누르는 단계는 없다. JavaScript는 버튼과 단일 요청만 담당하며 플랫폼, CPU, asset, hash, 현재 버전을
선택하지 않는다.

백엔드는 다음 실행 환경을 구분한다.

- Windows x64 / Windows arm64
- Linux x64 / Linux arm64
- Ubuntu는 Linux ABI와 CPU를 사용하고 `/etc/os-release`의 배포판 ID를 별도로 표시
- macOS Intel / macOS Apple Silicon
- Termux arm64 (`android/arm64`)

클라이언트가 백엔드 실행 환경과 다른 플랫폼을 지정하면 다운로드 전에 거부한다. Windows x64 자산 선택은
arm64 ZIP을 허용하지 않는다.

### 11.3 패키지와 DB 계약

- candidate `PACKAGE_FILE_MANIFEST.json`의 `package_version`은 GitHub release target과 정확히 같아야 한다.
- 현재 updater binary는 현재 manifest에 기록된 size·SHA256과 일치해야 하며 POSIX에서는 실행 권한도 필요하다.
- candidate manifest 전체를 검증한 뒤 새 파일을 추가하고 변경 파일을 교체하며, 이전 manifest에만 있는 관리
  파일은 삭제한다.
- `.runtime`, `.updates`, 사용자 env, MariaDB·Chroma data와 secret은 관리 파일에 포함하지 않는다.
- 신규 POSIX 설치는 버전 패키지 밖의 install-level `data` 경로와 `start-archive-center.sh`를 소유한다.
  Linux systemd, macOS, Termux의 최초 실행과 이후 재실행은 모두 이 실행기를 통과하므로 패키지가 갱신되어도
  MariaDB·ChromaDB 경로가 package-local `.runtime`으로 바뀌지 않는다.
- migration inventory는 모든 `migrations/*.sql`뿐 아니라 embedded schema를 가진 `mariadb-schema` binary도
  포함한다.
- SQL은 파일 이름순으로 전부 실행하며 migration은 expand-first·rerunnable·이전 backend 호환이어야 한다.
- 실패 시 관리 패키지 파일만 되돌린다. DB rollback은 하지 않는다.
- rollback한 이전 backend가 forward-migrated DB에서 `/ready`와 이전 exact `/version`을 통과해야 한다.

### 11.4 신규 경로를 만들지 않은 항목

- fresh-install wrapper와 updater는 결합하지 않았다.
- polling, watcher, 별도 background updater, 별도 update queue를 추가하지 않았다.
- 기존 `/update/check`, `/update/download`, `/update/status`는 호환 API로 유지했지만 설정의 정상 업데이트 버튼은
  `/update/apply`만 한 번 호출한다.

### 11.5 자동 검증

통과:

```text
go test ./internal/httpapi -run 'Test(Update|SelectUpdate|WindowsX64|ParseOSRelease)' -count=1
go test ./cmd/archive-center-go ./internal/httpapi -count=1
go test ./cmd/js-route-variant-smoke -run 'Test(BackendOwnedLongOperations|ArchiveCenterJSImmediateUpdate)' -count=1
go test ./internal/packageupdate ./cmd/mariadb-schema -count=1
node --check "Archive Center.js"
PowerShell parser: Windows launcher and both package builders
sh -n ops/full-package-posix/start-full-posix.sh
scripts/test-simple-fresh-install.sh
ops/full-package/scripts/updater-e2e-smoke.ps1
git diff --check
```

실제 공개 GitHub release, 실제 Windows/Linux/Ubuntu/macOS/Termux 장치, 실제 MariaDB·Chroma, 실제 launcher
프로세스 재기동은 아직 검증하지 않았다. 설치 패키지와 테스트 빌드도 생성하지 않았다.

## 12. 피드백 6 — macOS 공개 런처의 제한시간 전달 누락

### 12.1 제보와 재검증

macOS 패키지에서 `sh "Start Archive Center macOS.command"`를 실행하면 다음 오류가 발생한다는 제보를
확인했다.

```text
ERROR: external operation requires --external-operation-timeout-seconds or AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS
```

패키지 빌더가 생성하던 공개 런처에는 package root와 profile만 있었고, 공통 POSIX 런처가 요구하는 외부 작업,
로컬 HTTP 요청, 준비 대기, 준비 확인 간격이 없었다. 간편 설치의 `1800` 값도 GitHub release helper에만
전달되고 최종 macOS 런처에는 전달되지 않았다. 기존 간편 설치 테스트는 실제 POSIX 런처 대신 `started`와
data root만 기록하는 가짜 런처를 사용했기 때문에 이 누락을 발견하지 못했다.

### 12.2 수정 범위

공통 POSIX 런처 안에 숨은 기본값이나 별도 fallback을 추가하지 않았다. 정상 사용자 진입점이 다음 값을
명시적으로 소유하고 기존 POSIX 런처에 전달하도록 수정했다.

```text
AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=1800
AC_REQUEST_TIMEOUT_SECONDS=30
AC_READINESS_TIMEOUT_SECONDS=180
AC_READINESS_POLL_INTERVAL_SECONDS=1
```

- 패키지 빌더가 생성하는 Linux·macOS·Termux 공개 런처에 네 기본값을 포함한다.
- 값이 이미 지정된 경우 `${NAME:-default}` 규칙으로 사용자 값을 유지한다.
- `install.sh`가 release helper와 최종 패키지 런처에 네 값을 전달한다.
- GitHub release helper의 직접 시작과 systemd unit에도 같은 값을 전달한다.
- 공통 `start-full-posix.sh`의 기존 명시적 입력 계약은 변경하지 않았다.
- `install.sh`와 release helper는 값을 전달만 하며 request/readiness 값 검증은 기존 `start-full-posix.sh`가
  계속 단독으로 담당한다.

### 12.3 자동 검증

통과:

```text
sh -n install.sh
sh -n scripts/install-github-release.sh
sh -n scripts/test-simple-fresh-install.sh
PowerShell parser: ops/build-posix-managed-packages.ps1
scripts/test-simple-fresh-install.sh
  - macOS Intel
  - macOS Apple Silicon
  - Linux x64
  - Linux arm64
  - Termux arm64
scripts/test-simple-fresh-install.ps1
  - macOS Apple Silicon production package build
  - generated Start Archive Center macOS.command --preflight
  - custom timeout override preservation
```

생성된 `.command`가 기본값 `1800/30/180/1`을 실제 하위 macOS 런처에 전달하고, 호출자가 다른 값을 지정하면
그 값을 덮어쓰지 않는 것까지 기존 Windows CI 회귀 안에서 매번 다시 확인한다. 패키지 출력은 workspace 내부의
고유 임시 경로에만 만들고 테스트 종료 시 제거한다. 실제 macOS 장치에서 Homebrew·MariaDB·ChromaDB를
설치하고 backend `/ready`까지 도달하는 실기기 검증은 아직 수행하지 않았다.

### 12.4 현재 문서 기준

3.9.4 작업 폐기와 3.9.0 재시작이라는 이후 사용자 지시에 따라 현재 피드백 기준 문서는 이 파일이다. 폐기된
3.9.4 작업을 전제로 한 `archive-center-3.9.0-to-3.9.4-consolidated-audit.md`는 새로 만들지 않는다.

## 13. 피드백 7 — LLM endpoint 누락 시 저장 결과 표시 불일치

### 13.1 제보

macOS 26.5.2의 RisuAI 플러그인 설정에서 출판사와 편집 검토·평론가 역할에 API key와 model만 입력하고
endpoint를 비워 둔 경우 다음 오류와 함께 `저장 실패`가 표시된다는 제보를 받았다.

```text
runtime_config_incomplete:main[endpoint];supervisor[endpoint];critic[endpoint]
```

endpoint를 직접 입력하면 오류가 발생하지 않았으며, 실패 표시가 나온 경우에도 RisuAI `pluginStorage`에는
설정값이 저장된 것으로 관찰됐다. 이 문제는 macOS 전용 코드가 아니라 공통 JavaScript 설정 UI와 Go runtime
검증 순서에서 발생한다.

### 13.2 확인된 원인

`saveSettings()`는 먼저 `pluginStorage`에 설정을 저장하고 그다음 백엔드 `/config/update` 동기화를 수행한다.
로컬 저장이 성공했어도 backend runtime trace가 endpoint 누락을 보고하면 함수가 `false`를 반환하므로 UI에는
전체 저장이 실패한 것처럼 표시된다.

Go의 `configMissingFieldsWithProvider()`는 provider·API key·endpoint·model을 실행 가능한 LLM 역할의 필수
필드로 검사한다. JavaScript는 main 설정 중 하나라도 채워지면 main과 supervisor를 검사 대상으로 삼고,
critic 설정 중 하나라도 채워지면 critic을 검사 대상으로 삼는다.

요청 코드 안에는 일부 provider의 기본 URL을 계산하는 함수가 있지만 빈 endpoint는 그 함수에 도달하기 전에
요청 검증에서 거부된다. 따라서 Archive Center backend의 supervisor·critic 호출이 endpoint 없이 정상
작동한다는 의미가 아니다. RisuAI가 직접 담당하는 메인 본문 생성만 별도로 작동해 보일 수 있다.

### 13.3 확정한 수정 방향

- endpoint는 실행 가능한 LLM 역할의 필수값으로 유지한다.
- provider별 endpoint를 자동 입력하거나 자동 보정하지 않는다.
- API key 또는 model을 입력한 역할은 저장 전에 endpoint 누락을 UI에서 검사한다.
- endpoint 누락 시 기존 설정을 저장한 뒤 일반적인 `저장 실패`로 표시하지 않고, 해당 endpoint가 필요하다는
  구체적인 안내를 표시한다.
- main endpoint는 같은 설정을 사용하는 출판사와 감독관에 적용하고, 평론가는 유효한 평론가 설정을 기준으로
  별도로 검사한다.
- endpoint가 채워진 경우 기존 `pluginStorage` 저장과 backend runtime 동기화 경로를 그대로 사용한다.
- 새로운 provider 기본값, fallback, watcher 또는 별도 저장 경로를 추가하지 않는다.

### 13.4 현재 상태

이번 항목에서는 원인과 수정 방향만 확인했다. production source와 테스트는 아직 변경하지 않았다.

## 14. 피드백 8 — POSIX 신규 설치에서 기존 DB만 유지

### 14.1 제보와 설치 실패 원인

Termux에서 예전 `install-archive-center.sh --repo ... --install-dir ... --start` 명령과 직접 받은 ZIP을 사용했을 때
필수 제한시간 값이 전달되지 않았고, MariaDB가 다음 오류로 시작되지 않았다는 제보를 받았다.

```text
Initializing MariaDB data directory
Starting MariaDB on 127.0.0.1:3307
ERROR: MariaDB did not become ready on 127.0.0.1:3307
```

공통 POSIX 실행기의 `readiness_polling_enabled()`는 readiness timeout과 poll interval이 모두 있을 때만 반복
확인을 활성화한다. 두 값이 없으면 `wait_port()`가 MariaDB 포트를 한 번 확인한 뒤 실패하므로, MariaDB가
정상적으로 InnoDB와 소켓을 초기화하는 중이어도 성급하게 실패할 수 있다.

현재 공식 신규 설치 진입점은 다음 한 줄이다.

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

현재 `install.sh`와 패키지 빌더가 생성하는 Linux·macOS·Termux 공개 런처는
`1800/30/180/1`의 external/request/readiness/poll 값을 명시적으로 전달한다. 따라서 정상 신규 설치 경로는
제보된 한 번 검사 문제를 해소한다. 공통 POSIX 실행기에 숨은 기본값이나 별도 fallback을 추가하지 않는다.

### 14.2 기존 DB만 보존하는 Termux 수동 절차

구 Termux 기본 데이터 위치는 `$HOME/.archive-center-2.0`이고 신규 간편 설치의 기본 데이터 위치는
`$HOME/.archive-center/data`다. 기존 Archive Center·MariaDB·ChromaDB를 모두 종료한 뒤 다음과 같이 기존
MariaDB·ChromaDB 디렉터리만 별도 데이터 위치로 복사하여 사용할 수 있다.

```sh
mkdir -p "$HOME/archive-center-preserved-data"

cp -a "$HOME/.archive-center-2.0/mariadb-data" \
  "$HOME/archive-center-preserved-data/"

[ ! -d "$HOME/.archive-center-2.0/chromadb-data" ] || \
  cp -a "$HOME/.archive-center-2.0/chromadb-data" \
  "$HOME/archive-center-preserved-data/"

curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | \
  ARCHIVE_CENTER_DATA_DIR="$HOME/archive-center-preserved-data" sh
```

새 설치 루트인 `$HOME/.archive-center`가 없어야 하며, 구 실행기·updater·`.updates`·package manifest·runtime
binary는 복사하지 않는다. 이 절차는 MariaDB와 ChromaDB 데이터만 보존한다.

### 14.3 Linux·Ubuntu 적용 범위

`ARCHIVE_CENTER_DATA_DIR`는 Termux 전용이 아니라 공통 POSIX runtime 계약이므로 Linux·Ubuntu·macOS에서도
사용할 수 있다. 다만 구 데이터 원본 경로는 설치 방식에 따라 다르다.

- Termux 구 기본값: `$HOME/.archive-center-2.0`
- Linux·Ubuntu 구 압축 패키지: 해당 패키지의 `.runtime`
- 신규 비-systemd Linux·macOS·Termux 기본값: `$HOME/.archive-center/data`
- 신규 systemd Linux·Ubuntu 기본값: `/opt/archive-center/data`

Linux·Ubuntu에서도 구 패키지의 `.runtime/mariadb-data`와 `.runtime/chromadb-data`만 별도 디렉터리에 복사한
뒤 같은 `ARCHIVE_CENTER_DATA_DIR` 방식으로 사용할 수 있다. 그러나 현재 `install.sh`의 systemd 분기는
release helper를 `sudo env`로 실행하면서 사용자 지정 `ARCHIVE_CENTER_DATA_DIR`를 명시적으로 전달하지 않는다.
따라서 systemd가 활성화된 Ubuntu·Linux에서는 이 명령을 현재 지원 완료로 표시하지 않는다. 해당 기존 전달
경로를 수정하고 systemd 설치 회귀 검사를 통과한 뒤 같은 절차를 공식 지원해야 한다.

### 14.4 검증 경계

- 현재 source에서 정상 간편 설치의 네 제한시간 전달과 공개 POSIX 런처 생성을 확인했다.
- 현재 실행 환경에 `sh`가 없어 이번 확인에서 셸 회귀 검사를 다시 실행하지 못했다.
- 실제 Termux·Linux·Ubuntu 장치에서 기존 MariaDB·ChromaDB를 이전하는 검증은 수행하지 않았다.
- 기존 Termux 데이터 위치를 신규 설치기가 자동 탐색·복사하는 기능은 구현하지 않았다.
- 이번 항목에서는 production source와 테스트를 변경하지 않았다.

## 15. 2026-08-07 로컬 checkpoint와 이전 작업 기록

### 15.1 checkpoint

지금까지 source에 반영한 3.9.0 피드백 작업은 다음 로컬 commit으로 묶었다.

```text
b8a912d feat: checkpoint Archive Center 3.9.0 feedback work
```

이 commit은 66개 파일의 변경을 기록한다. `_test-builds/`, `.env`, `.runtime`은 commit에 넣지 않았다.
따라서 사용자 데이터·비밀값·생성된 테스트 패키지는 source checkpoint와 별개다.

### 15.2 checkpoint에 포함된 작업 묶음

- Windows legacy `.runtime` MariaDB 후보 선택 오류 수정과 기존 데이터 보존 경로
- `voyage-context-4`의 다중 청크 contextualized embedding, 저장·재시도·검색·유지보수 연결
- reasoning effort의 `minimal`·`xhigh`·`max` UI 재표시와 저장값 보존
- Windows·POSIX 한 줄 신규 설치 진입점과 설치 회귀 검사
- 다음 패키지부터 사용하는 managed updater, OS·CPU 판별, 패키지 교체, schema migration, DB 보존 계약
- macOS·Linux·Ubuntu·Termux 공개 실행기의 필수 제한시간 전달
- Windows와 POSIX 실행기 종료 시 Go backend·MariaDB·ChromaDB 자식 process 정리
- source acceptance·logical turn 연결, 현재·과거 queue 표시, 파생 기억·vector outbox·서사 상태 정리 관련 source와 회귀 검사
- 삭제·리롤 HUD 문구와 backend workflow notice 계약
- 위 항목들의 작업 문서와 자동 회귀 fixture

이 목록은 commit에 들어간 source 범위다. 실제 Windows 외 OS, 실제 RisuAI/PocketRisu, 실제 provider,
사용자 MariaDB·ChromaDB와 공개 release artifact에서 동작했다는 증거로 사용하지 않는다.

### 15.3 checkpoint 당시 검증

- `git diff --check`: 통과
- `Archive Center.js` Node syntax 검사: 통과
- Go 전체 package 검사: 통과
- Node 경로를 명시한 JavaScript route smoke: 통과

`_test-builds/3.9.5-1st-test`는 source 외 생성물이다. 2026-08-07 재빌드 결과와 포함 여부 검사는
19절에 별도로 기록한다.

## 16. 피드백 1-B 현재 source 정정

4.2절은 `addRisuChatListener("output", ...)`을 이용했던 이전 구현 시도 기록이다. 현재 source에는
`addRisuChatListener`·`removeRisuChatListener`·`onRisuChatOutput`이 없다. 따라서 4.6절의
"현재 source가 output listener를 등록한다"는 문장은 현재 동작 설명으로 사용하지 않는다.

현재 source의 실제 경로는 다음과 같다.

```text
beforeRequest에서 요청 관측
  -> RisuAI afterRequest가 돌려준 완료 응답을 승인
  -> 기존 complete-turn 저장 경로 예약
  -> raw 저장
  -> 평론가·파생 기억·vector 처리
```

- 스트리밍 중간 chunk를 읽지 않는다.
- `afterRequest`가 받은 완료 응답은 처리하지만, 다른 플러그인의 모든 후처리가 끝난 최종 chat commit이라고
  RisuAI 계약상 보장하지 않는다.
- 현재 source는 후처리 완료를 기다리는 별도 DOM watcher·polling·고정 대기·fallback 저장 경로를 두지 않는다.
- 이 정정은 문서의 현재 동작 설명을 source와 맞춘 것이며, 새 기능 구현이 아니다.

## 17. 평론가 입력 과다 1차 축소

### 17.1 변경 전 문제

canonical complete-turn의 평론가 입력에도 host의 `context_messages`가 그대로 들어갔다. RisuAI의
`allbefore`가 사실상 전체 이전 대화를 포함하면 턴이 늘어날수록 평론가 입력도 계속 커졌다. 사용자가 실제
gateway에서 관측한 단일 평론가 호출 입력은 GPT-5.6-Luna 기준 약 29.8K token이었다.

### 17.2 현재 선택 계약

현재 턴의 사용자 입력과 assistant 출력은 기존 `Latest_Turn` 필드에 **전체 원문으로 유지**한다. 그 밖의
보조 문맥만 다음처럼 제한한다.

1. 바로 직전 canonical DB 턴의 user·assistant 원문 전체
2. 현재 입력·출력과 관련도가 높은 과거 DB 기억 최대 3개
3. 선택된 기억의 source turn user·assistant 원문은 메시지당 최대 1,200자
4. 선택된 기억 요약은 항목당 최대 600자

canonical 경로에서는 host가 넘긴 전체 `context_messages`를 평론가 문맥으로 사용하지 않는다. DB에서 읽은
직전 턴과 관련 기억의 source turn만 사용하며, 현재·미래 턴, 관련 없는 기억, 공개되지 않은 보호 기억은
선택에서 제외한다. 이 보조 문맥은 새 사건이나 새 직접 근거가 아니라 support-only로 표시한다.

### 17.3 변경하지 않은 범위

- 현재 턴 원문과 평론가 JSON 출력 구조
- canonical raw 저장, 파생 기억 admission, vector 처리
- `critic_system.txt`
- 활성 세계 규칙 입력량
- 최초 호출 입력 snapshot의 재처리 보존
- `retry_after` 시각에 worker를 깨우는 타이머

따라서 이번 단계는 `allbefore` 전체 전달 제거와 제한된 DB 문맥 선택까지다. 평론가 시스템 프롬프트 중복,
재처리 재현성, 자동 재시도 타이머를 해결했다고 기록하지 않는다.

### 17.4 변경·검증

- production: `go-service/internal/httpapi/turn_extraction_critic.go` `+109 / -0`
- tests: `turn_extraction_critic_test.go` `+42 / -0`, `group_turn_part02_test.go` `+4 / -0`
- focused canonical context 회귀: 통과
- `go test ./internal/httpapi -count=1`: 통과
- 실제 provider token·latency 재측정: 미실행

### 17.5 테스트 빌드 1턴 실사용 측정

2026-08-07 사용자가 갱신된 `3.9.5-1st-test`로 한 턴을 진행한 뒤 LLM gateway에서 확인한
GPT-5.6-Luna 평론가 입력량은 `16.5K token`이었다. 변경 전 같은 평론가 모델의 단일 호출 측정값
`29.8K token`과 비교하면 `13.3K token`, 약 `44.6%` 감소다.

이는 `allbefore` 전체 전달 제거와 제한된 DB 문맥 선택이 실제 provider 입력량을 줄였다는 1회 실사용
증거다. 아직 여러 턴 누적, 오래된 관련 기억 회수, 평론가 출력 품질, 호출 시간과 재처리 호출은 반복
검증하지 않았으므로 평론가 최적화 전체 완료로 판정하지 않는다.

## 18. 턴 삭제 즉시 감지 지원 폐기

### 18.1 확인 결과

PocketRisu 1.8.1 plugin API 구현에는 `input`, `output`, `display`, `process`, `beforeRequest`, `afterRequest` handler만
있고 채팅 삭제, 메시지 삭제 또는 채팅 변경 lifecycle 신호는 없다. 평론가 호출은 `afterRequest`를 사용할 수
있지만 RisuAI UI에서 실행한 턴 삭제에는 같은 종류의 공식 신호가 없다.

### 18.2 제거한 실패 구현

`SafeMutationObserver`를 사용한 첫 구현과 main DOM click을 사용한 두 번째 구현은 모두 자동 검사에서는
통과했지만 실제 RisuAI에서는 탐색 UI를 열기 전 삭제를 감지하지 못했다. 두 구현과 전용 테스트 주장을
production source에서 제거했다. polling, 상시 watcher, timer, DOM selector 또는 별도 삭제 경로로 교체하지
않는다.

### 18.3 유지되는 동작

기존 backend 삭제 판정, rollback, 해당 턴 이후 DB 정리, `/del`·`/cut` 보존, HUD ViewModel과 탐색 UI를 열 때
수행되는 정합성 확인은 변경하지 않았다. 공식 host 신호가 추가되기 전에는 RisuAI 일반 UI 삭제 직후의 DB
삭제와 HUD 표시는 지원 기능으로 표기하지 않는다.

### 18.4 폐기 반영 테스트 빌드

실패 구현 제거 후 기존 `_test-builds/3.9.5-1st-test`를 같은 이름으로 다시 생성했다.

- Linux x64/arm64, macOS Intel/Apple Silicon, Termux arm64, Windows x64의 6개 package 폴더·ZIP 생성
- 6개 package 폴더와 6개 ZIP의 `PACKAGE_FILE_MANIFEST.json` size·SHA-256 오류 0건
- 6개 package 모두 실패한 mutation observer·main DOM click 삭제 감지 표식 0건
- Windows `.runtime`과 `.env.full.local` 복원 및 `.env.full.local` SHA-256 일치
- 외부 `SHA256SUMS-3.9.5.txt`에 6개 ZIP 기록 및 실제 hash 대조 오류 0건
- 빌드 종료 후 28080·3307·8000 listener 없음

## 19. `3.9.5-1st-test` 테스트 빌드 갱신

### 19.1 결과 경로

```text
C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source\_test-builds\3.9.5-1st-test
```

현재 source로 다음 6개 폴더와 ZIP을 다시 만들었다.

- Windows x64
- Linux x64
- Linux arm64
- macOS Intel
- macOS Apple Silicon
- Termux arm64

Windows package 생성 시각은 `2026-08-07T08:39:01Z`이며, POSIX package들은
`2026-08-07T08:33:35Z`부터 `08:34:45Z` 사이에 생성됐다.

### 19.2 실행 중 process와 사용자 데이터 처리

처음 확인했을 때 28080·3307 포트는 닫혀 있었지만 기존 Windows 테스트 package에서 시작한
`archive-center-go.exe`, MariaDB, ChromaDB Python과 실행기 cmd/PowerShell process가 남아 있었다.
이 process들이 package 실행 파일과 폴더 handle을 잡고 있어 첫 Windows 교체 시도가 거부됐다.

해당 테스트 package에서 시작한 backend·MariaDB·ChromaDB를 종료하고 10초 이내 종료되지 않은 process를
강제 종료한 뒤, 남은 테스트 실행기 cmd/PowerShell만 종료했다. Codex process와 Windows Terminal 자체는
종료하지 않았다. 빌드 후 서버는 다시 시작하지 않았다.

Windows package의 다음 local 항목은 교체 전에 별도 staging으로 이동하고 빌드 완료 또는 실패 시 항상
원래 package 폴더로 되돌렸다.

- `.runtime`
- `.env.full.local`
- 존재할 경우 `.runtime-cache`, `.updates`, `.env.full.local.protected`

최종 확인에서 `.runtime`과 `.env.full.local`이 모두 존재하고 임시 보존 폴더가 남지 않았음을 확인했다.

### 19.3 빌드 중 확인된 중단과 재실행

- POSIX 첫 시도는 Windows 기본 Go build cache 접근 거부로 중단됐다. source 내부 전용 `GOCACHE`와
  `GOTMPDIR`를 지정한 뒤 5개 POSIX package를 처음부터 다시 생성했다.
- Windows 첫 두 시도는 남아 있던 backend executable handle과 실행기 current-directory handle 때문에
  기존 package 삭제 단계에서 중단됐다. 각 실패 때 local data를 즉시 복원했고, 관련 process를 종료한 뒤
  전체 Windows package와 ZIP을 다시 생성했다.

### 19.4 artifact 포함 여부와 무결성 검사

각 6개 package에서 다음을 직접 확인했다.

- `PACKAGE_FILE_MANIFEST.json`의 모든 관리 파일이 존재하며 SHA-256이 일치
- package version이 모두 `3.9.5`
- package `Archive Center.js`에 `requestPluginPermission("mainDom")` 포함
- package `Archive Center.js`가 `body`에 기존 삭제 observer를 등록
- 각 OS backend binary에 `canonical_previous_plus_relevant_memory_sources` 평론가 문맥 선택 계약 포함
- 6개 ZIP 재생성 시각과 크기 확인
- Windows ZIP과 외부 `SHA256SUMS-3.9.5.txt` 생성

이는 source 변경이 생성된 artifact에 들어갔다는 검증이다. 실제 RisuAI에 새 `Archive Center.js`를 로드하고
UI를 열지 않은 삭제 HUD, 평론가 provider token, 실제 MariaDB·Chroma 결과를 실행한 검증은 아직 수행하지 않았다.

## 20. 신규 개체 ID 연속성 보강

### 20.1 적용 기준

- 기존 DB의 개체 ID를 일괄 재작성하거나 새 번호로 바꾸지 않는다.
- `mapping_revision=2` 같은 별도 매핑 세대를 추가하지 않는다. 기존 연결 계약의 `mapping_revision=1`만 유지한다.
- 3.9.0 DB에 저장된 기존 표면 범위 `source_turn`과 업데이트판이 새로 기록하는 `source_turn_current`를 모두
  동일 개체 후보 조회에 사용한다.
- 업데이트 후 새 턴에 개체가 다시 등장하면, 현재 턴의 정확한 동일성 근거와 같은 세션의 유일한 활성 후보를
  확인한 뒤 새 source occurrence의 canonical target을 기존 3.9.0 ID A로 기록한다.
- 새 source occurrence에는 원문 출처를 보존하기 위한 별도 occurrence ID가 남지만 기존 ID A를 대체하지 않는다.
  canonical 조회와 후속 연결은 계속 A를 반환한다.
- 과거 DB 전체를 스캔하거나 재처리하지 않는다. 개체가 새 턴에 다시 등장한 시점에만 기존 A를 조회한다.
- 이름이 같다는 이유만으로 합치지 않는다. 현재 턴의 정확한 `identity_evidence_excerpt`가 있고, 현재 범위에서
  같은 표면이 하나의 활성 ID로만 확인될 때 새 발생 ID를 최초 새 ID에 연결한다.
- 같은 규칙을 인물, 장소, 물건, 집단에 공통 적용한다. 동명이인·동명 개체 또는 근거 없는 후보는 계속
  독립된 source occurrence ID로 남긴다.

### 20.2 변경 범위

- Go의 기존 entity identity projection과 MariaDB surface resolver만 수정했다.
- MariaDB resolver는 `source_turn`과 `source_turn_current` 두 기존 범위만 명시적으로 읽는다. 다른 임의 범위는
  연결 후보로 넓히지 않는다.
- 평론가 JSON 예시에 장소·물건·집단의 `aliases`, `identity_evidence_excerpt`를 명시하고, 반복 개체에도
  동일한 근거 규칙을 사용하도록 기존 한 문장을 보강했다.
- 새 테이블, 새 API, JavaScript 경로, fallback, watcher, background worker는 추가하지 않았다.
- `Archive Center.js` 변경량: `+0 / -0`.

### 20.3 자동 검증

- 3.9.0의 기존 ID A가 업데이트 후 canonical target과 조회 결과로 그대로 유지됨: 통과
- 수정 이후 동일 인물의 근거 기반 연결: 통과
- 수정 이후 동일 물건의 공통 연결 경로: 통과
- 동명이인, 근거 없는 이름 동일성, reversible state 분리: 통과
- 기존 전체 이름·별칭 연결 회귀: 통과
- `go test ./internal/httpapi ./internal/store`: 통과
- 번들 Node를 지정한 `go test ./...`: 통과

자동 테스트는 source 동작 검증이다. 실제 MariaDB가 연결된 테스트 빌드에서 새 개체가 여러 턴에 걸쳐 하나의
canonical ID로 조회되는지는 별도의 loaded-artifact/live DB 검증이 남아 있다.

## 21. MariaDB 파생 기억 저장 데드락 완화

### 21.1 확인된 사실과 확정할 수 없는 부분

사용자 피드백의 다음 오류는 MariaDB가 실제 transaction lock cycle을 감지해 한 transaction을 중단했다는
의미다.

```text
derived_persist_failed: operation=CommitMemoryAdmission;
cause=Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
```

기존 source에서는 foreground 파생 기억 admission, 백그라운드 재처리, source revision·logical turn 교체,
vector outbox·재처리 job 상태 변경, 전체 초기화가 서로 다른 transaction으로 같은 파생 기억 테이블 묶음을
동시에 변경할 수 있었다. 특히 `CommitMemoryAdmission`은 source revision을 잠근 뒤 기억·근거·정밀 기억·의존성·
vector outbox를 한 transaction에서 변경했고, `ResetAll`은 다른 순서로 같은 테이블을 삭제했다. 이 구조에는
서로 반대 순서의 lock을 기다릴 가능성이 있었다.

다만 제보가 발생한 순간의 `SHOW ENGINE INNODB STATUS` 또는 MariaDB deadlock log가 없으므로, 실제 상대
transaction이 reset, 재처리 worker, vector outbox claim 중 정확히 어느 것이었는지는 확정할 수 없다.
기존 설명처럼 vector outbox 하나가 원인이라고 단정하지 않는다.

“삭제하고 다시 하니 정상화됐다”는 추가 피드백은 삭제·초기화 과정에서 기존 source revision, 재처리 job 또는
outbox 상태가 정리된 뒤 충돌이 사라졌다는 해석과 맞는다. 그러나 이것도 당시 lock log가 없는 상태에서는
원인 transaction을 특정하는 직접 증거로 사용하지 않는다.

### 21.2 변경한 동작

- 한 Go backend process 안에서 source revision, 파생 기억 admission, 재처리 job, vector outbox, 정밀 기억,
  logical turn 교체·rollback, `ResetAll`의 짧은 MariaDB 쓰기 transaction이 서로 겹치지 않도록 기존
  `mariadbStore`에 하나의 공통 쓰기 lane을 사용한다.
- `CommitMemoryAdmission`은 transaction isolation을 해당 경로에만 `READ COMMITTED`로 지정한다.
- MariaDB 오류 번호 `1213`일 때만 transaction 전체를 새 transaction으로 다시 실행한다.
- 최대 시도 횟수는 최초 시도를 포함해 3회이며 대기는 25ms, 50ms다. context가 취소되면 즉시 중단한다.
- lock wait timeout `1205`, provider 오류, validation 오류 또는 다른 DB 오류는 이 재시도 대상으로 넓히지 않는다.
- 세 번 모두 1213이면 기존 오류를 호출자에게 반환한다. 실패를 성공으로 숨기거나 새 fallback 저장 경로를
  사용하지 않는다.
- JavaScript, HUD, API, DB schema, table, background worker는 추가하거나 변경하지 않았다.

### 21.3 변경 파일과 실제 diff

- `go-service/internal/store/mariadb.go`: 공통 쓰기 lane과 `ResetAll` 연결, `+6 / -0`
- `go-service/internal/store/mariadb_memory_admission.go`: `READ COMMITTED`와 1213 transaction 재시도,
  `+46 / -1`
- `go-service/internal/store/mariadb_memory_derivation.go`: 기존 9개 mutation 진입점에 lane 연결,
  `+18 / -0`
- `go-service/internal/store/mariadb_precise_memory.go`: 정밀 기억 저장 연결, `+2 / -0`
- `go-service/internal/store/mariadb_logical_turn_replace.go`: logical turn 교체·rollback 연결, `+4 / -0`
- `go-service/internal/store/mariadb_memory_admission_test.go`: 1213 성공 재시도, 재시도 한도, rollback 뒤 새 ID
  재구성 검사
- `go-service/internal/store/mariadb_rollback_test.go`: reset이 진행 중인 파생 기억 쓰기와 겹치지 않는지 검사
- 이번 데드락 작업의 `Archive Center.js` 변경량: `+0 / -0`

`mariadb_memory_derivation.go`가 `+1216 / -1198`로 보였던 표시는 현재 실제 Git diff가 아니다. 현재 index와
worktree를 다시 비교한 결과는 `+18 / -0`이고, 기존 1,198줄은 보존된 채 `Lock`/`Unlock` 9쌍만 추가됐다.

### 21.4 자동 검증과 남은 실환경 검증

다음 자동 검증은 현재 source에서 통과했다.

- admission 1213 뒤 새 transaction 전체 재실행 및 commit
- 연속 1213 세 번 뒤 오류 반환
- 첫 transaction rollback에서 받은 memory·evidence·precise ID가 재시도 결과에 섞이지 않음
- `ResetAll`과 파생 기억 쓰기 lane의 상호 배제
- `go test ./internal/store -count=1`
- `go test ./internal/httpapi -count=1`
- 번들 Node를 지정한 `go test ./... -count=1`
- `go vet ./internal/store`
- `node --check "Archive Center.js"`
- `git diff --check`

SQL mock 회귀 검사는 transaction 재시도 계약과 호출 순서를 확인하지만 InnoDB의 실제 lock scheduler를
재현하지는 않는다. 실제 MariaDB에서 foreground 저장, 콜드 스타트 재처리, reset·삭제가 겹치는 실행을
반복해 1213과 HUD의 반복 저장 표시가 사라지는지 확인하는 loaded-artifact/live DB 검증은 남아 있다.

20절 개체 ID 및 21절 데드락 변경은 22절의 갱신된 `3.9.5-1st-test`에 포함됐다.

## 22. 평론가 고정 프롬프트 중복 정리와 테스트 빌드 재갱신

### 22.1 작업 전 checkpoint

평론가 프롬프트를 다시 변경하기 전에 당시의 source 상태를 먼저 다음 로컬 commit으로 고정했다.

```text
c2aae3c fix: checkpoint 3.9.0 feedback runtime corrections
```

`_test-builds`는 사용자 설정·런타임·DB가 섞일 수 있는 비추적 시험 산출물이므로 commit에 포함하지 않았다.

### 22.2 중복 정리 범위

기존 canonical 평론가 요청은 `critic_system.txt`에 추출 계약을 두면서도 Go가 만드는 user prompt에 같은 JSON
구조와 추출 규칙을 다시 붙였다. 이 단계에서는 평론가가 받는 자료나 저장 구조를 바꾸지 않고 고정 지시의 소유
위치만 하나로 정리했다.

- `prompts/critic_system.txt`가 JSON 표면, 직접 근거, 개체 동일성, 화자, 관계, 세계 규칙, 시간·장소,
  주관 기억, persona capsule 등 고정 추출 계약을 단독 소유한다.
- Go의 `buildCompleteTurnCriticPromptWithLanguageContext`는 다음 동적 값만 직렬화한다.
  - 현재 사용자 입력과 assistant 최종 출력
  - 제한된 최근 문맥
  - deterministic preview
  - critic archive ledger와 활성 세계 규칙
  - 출력 언어 override와 language context
- Go user prompt에서 중복 고정 지시 44개 항목을 제거했다.
- `critic_system.txt` 파일은 checkpoint blob 기준 17,791 bytes에서 25,293 bytes로 늘었지만, 기존에 매 호출마다 user prompt에
  중복 삽입되던 더 큰 고정 블록을 제거했다.
- 작은 fixture에서 동적 user prompt가 2,000자 미만이며 고정 JSON 구조·민감도 규칙 문구가 다시 들어가지
  않는 회귀 검사를 추가했다.
- 세계 규칙 누락 때만 실행되는 별도 `complete_turn_world_rule_audit` 요청은 main critic system prompt를
  사용하지 않는 독립 JSON 계약이므로 이번 중복 제거 대상으로 합치지 않았다.
- JavaScript production 변경량은 `+0 / -0`이며, 새 fallback·watcher·API·DB schema를 추가하지 않았다.

### 22.3 source 자동 검증

다음 검사가 통과했다.

- 평론가 prompt·인물 profile·습관·group-turn 표적 Go 회귀
- `go test ./... -count=1`
- `go vet ./internal/httpapi ./internal/store`
- 번들 Node의 `node --check "Archive Center.js"`
- `git diff --check`

이는 JSON 예시가 파싱되고 기존 추출 계약이 system+dynamic prompt 조합에서 유지된다는 source 증거다. 실제
provider 입력 token과 지연 감소는 ZDR 환경의 사용자 측 provider 사용량으로 다시 측정해야 한다.

### 22.4 설치·업데이터 계약 재확인

평론가 수정과 별개로 이전 피드백의 설치 간소화와 이후 버전 자동 업데이트가 패키지 갱신 중 깨지지 않았는지
다시 검사했다.

- Windows 한 줄 신규 설치와 기존 설치 거부·보존: 통과
- POSIX 한 줄 신규 설치와 stable data root 유지: 통과
- Linux·macOS·Termux shell 문법 검사: 통과
- macOS 공개 런처의 `1800/30/180/1` 제한시간 전달과 사용자 override 유지: 통과
- 3.9.0 legacy MariaDB data의 offline copy·검증과 원본 보존: 통과
- 실제 package updater의 apply/status/commit/rollback: 통과
- 중단된 apply 복구, 변조 ZIP의 변경 전 거부: 통과
- updater 적용·rollback·거부 동안 `.runtime`, MariaDB·Chroma data, env secret 보존: 통과

실제 공개 GitHub release를 올린 뒤 각 OS 장치에서 `지금 업데이트`를 누르는 release/live 검증은 수행하지
않았다. 이번 자동 검사는 생성된 Windows updater binary와 임시 package fixture를 사용했다.

### 22.5 `3.9.5-1st-test` 갱신과 무결성

다음 기존 폴더를 새 이름으로 늘리지 않고 그대로 갱신했다.

```text
C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source\_test-builds\3.9.5-1st-test
```

- Linux x64/arm64, macOS Intel/Apple Silicon, Termux arm64 ZIP: `2026-08-07T12:17:57Z`부터
  `12:18:50Z` 사이 재생성
- Windows x64 ZIP: `2026-08-07T12:20:59Z` 재생성
- 6개 package 폴더와 6개 ZIP 존재
- 모든 `PACKAGE_FILE_MANIFEST.json` 관리 파일의 size·SHA-256 일치
- 6개 package version 모두 `3.9.5`
- 6개 폴더와 6개 ZIP의 `prompts/critic_system.txt`가 source SHA-256
  `64adb50f1e6c6467820ad683b4473b1b9ce37e119b3b0f3bdcb650cda24f2e55`와 일치
- 외부 `SHA256SUMS-3.9.5.txt`에 6개 ZIP 전부 기록하고 실제 hash와 대조
- Windows package fresh smoke: `status=ok`
- Windows package updater E2E: `status=ok`

교체 전에 해당 test build에서 실행 중이던 Go backend, MariaDB, ChromaDB와 잔류 launcher `cmd.exe`만
종료했다. `.runtime`과 `.env.full.local`은 별도 staging에 보관한 뒤 원위치로 복원했으며 임시 보관 폴더는
남지 않았다. 서버는 자동으로 다시 시작하지 않았다.

이 artifact에는 20절 개체 ID 연속성, 21절 MariaDB 데드락 완화, 22절 평론가 중복 정리가 모두 포함된다.
실제 RisuAI, 실제 평론가 provider, 실제 MariaDB·Chroma에서의 기능·품질 검증은 별도 단계다.

### 22.6 갱신된 테스트 빌드의 Luna 실측

사용자가 같은 `3.9.5-1st-test`에서 GPT-5.6-Luna 평론가를 사용해 일반 턴을 진행한 뒤 gateway에 표시된
호출당 token 사용량을 다음과 같이 확인했다.

| 논리 턴 | Luna token 사용량 | 평론가 체감 시간 |
|---|---:|---:|
| 1턴 | 11.5K | 약 21초 |
| 2턴 | 14.1K | 미기록 |
| 3턴 | 14.3K | 미기록 |
| 3턴 삭제 후 리롤 | 약 15.3K | 미기록 |

3턴 삭제 후 리롤 값은 gateway에 2회 합계 `29.6K`로 표시된 값에서 삭제 전 3턴 `14.3K`를 뺀
역산값이다. 표시값 반올림 오차가 있을 수 있으므로 직접 측정값이 아닌 약 `15.3K`로 기록한다.

2턴은 1턴보다 2.6K 높고, 삭제 전 3턴은 2턴보다 0.2K 높다. 삭제 후 같은 3턴을 다시 생성한 리롤은
삭제 전 3턴보다 약 1.0K 높다. 리롤 값은 4턴 증가 표본이 아니므로 턴이 지날수록 입력량이 증가한다는
판정에는 사용하지 않는다. 이후 일반 턴과 리롤을 구분해 동일 모델·동일 reasoning 설정 측정값을 비교한다.
감독관은 별도 Ollama Cloud 모델 호출이므로 이 Luna 평론가 token 측정에 포함하지 않는다.

## 23. 3턴·리롤 DB 감사와 후속 정합성 수정

### 23.1 실제 MariaDB 결과 확인

`3.9.5-1st-test`로 진행한 현재 세션의 1~3턴과 3턴 리롤 결과를 읽기 전용으로 확인했다.

- 원문 로그: 턴마다 사용자 1건·assistant 1건
- 최종 입력: 턴마다 1건
- 턴 기억: 턴마다 1건
- 직접 근거: 1턴 15건, 2턴 15건, 3턴 17건
- KG triple: 1턴 9건, 2턴 6건, 3턴 8건
- canonical state layer: 1턴 3건, 2턴 4건, 3턴 4건
- 활성 정밀 기억: 1턴 16건, 2턴 17건, 3턴 15건

빈 원문·빈 최종 입력·손상된 기억 JSON·빈 직접 근거·빈 KG 구성요소·활성 source revision을 벗어난 활성 파생
결과는 발견되지 않았다. 리롤 전 3턴 source revision은 `superseded`, 그 파생 정밀 기억과 dependency는 무효화됐고
새 3턴만 활성 결과로 남아 있었다.

### 23.2 동일 개체 ID 연결 누락 수정

평론가가 2턴의 `강한얼` 근거 문장에서 원문의 `치러`를 `치뤄`로 바꿔 반환했다. 기존 연결 코드는 근거 문장 전체가
원문과 정확히 같지 않으면 연결을 중단했다. 2턴에서 연결이 끊긴 뒤 3턴에는 같은 이름 후보가 둘이 되어 모호성으로
다시 끊겼다.

기존 identity projection 안에서 다음 조건만 보강했다.

- 평론가 근거가 비어 있으면 기존처럼 거부한다.
- 전체 근거 문장이 달라도 현재 개체 이름 자체가 현재 원문에 정확히 존재해야 한다.
- 이전 canonical 후보가 정확히 하나일 때만 그 기존 ID를 잇는다.
- 저장 근거는 평론가의 불일치 문장이 아니라 원문에 실제 존재하는 정확한 이름만 사용한다.
- alias 전용 연결, 과거 DB 일괄 재연결, 새 mapping revision은 추가하지 않는다.

현재 DB의 과거 누락 ID를 소급 변경하지 않으며, 수정 이후 새로 들어오는 동일 개체부터 기존 canonical ID를 이어 간다.

### 23.3 리롤 시 이전 세계 규칙 유실 수정

`SaveWorldRule`은 다음 턴에 같은 key가 다시 나오면 기존 행을 갱신하면서 `source_turn`도 새 턴으로 바꾸고 있었다.
그 상태에서 새 턴을 리롤하면 tail 정리가 그 행을 삭제해 이전 턴에서 이미 성립한 세계 규칙까지 사라졌다.

별도 테이블이나 fallback 없이 기존 `world_rules` 저장·조회만 다음처럼 수정했다.

- 같은 턴의 같은 규칙 재시도는 같은 행을 갱신한다.
- 다른 턴의 같은 규칙은 새 버전 행으로 저장한다.
- 일반 조회, 상속 조회, 단일 transaction 세션 snapshot은 논리 key별로 가장 큰 `source_turn`을 읽고,
  같은 턴이면 가장 나중 ID를 읽는다. 따라서 과거 턴 재처리가 늦게 끝나도 현재 턴 규칙을 덮지 않는다.
- 리롤이 새 버전을 삭제하면 이전 턴 버전이 자동으로 다시 최신값이 된다.

### 23.4 감독관 실패 원인 표시

감독관 실패 후 본문을 계속 진행하는 기존 fail-open 동작은 유지했다. 다만 기존에는 모든 공급자 오류가
`publisher_llm_failed_open` 하나로 바뀌어 9초 실패나 빈 응답의 원인을 HUD에서 구분할 수 없었다.

기존 `runSupervisorLLM` 반환 trace와 현재 턴 workflow reason에 다음 구분을 전달한다.

- `publisher_llm_timeout`
- `publisher_llm_empty_content`
- `publisher_llm_upstream_rejected`
- `publisher_llm_upstream_unavailable`
- `publisher_llm_request_invalid`
- `publisher_llm_request_canceled`
- 분류되지 않은 `publisher_llm_provider_error`

trace에는 비밀값을 제거한 실제 오류 문구와 upstream HTTP status도 남긴다. API 키는 응답과 workflow trace에
노출하지 않는다. 새 재시도·fallback·차단 조건은 추가하지 않았다.

### 23.5 불필요 공간과 고아 데이터 감사

현재 DB에서 활성 artifact가 비활성 source revision을 가리키거나, source가 없는 entity surface·binding·speaker
artifact는 발견되지 않았다. `superseded` source revision과 무효화된 precise/dependency는 리롤 fence와 감사 이력이라
현재 결과를 오염시키는 고아 데이터로 보지 않았다.

반면 `memory_vector_outbox`는 정리 정책 없이 계속 증가하고 있었다.

- 완료 45,099건: delete 42,131건, upsert 2,968건
- permanent 562건, stale_rejected 18건
- 약 17.14MB data + 28.05MB index, 총 약 45MB
- 완료 upsert의 `document_json`만 약 4.90MB

이 행은 `operation_key` 중복 방지와 source revision 외래 키에 쓰이므로 처리 직후 전부 삭제하면 안 된다. 그러나 보존
기한이 없는 현재 상태도 장기적으로 무한 증가한다. 이번 범위에서는 live DB를 삭제하지 않았고, 후속 유지보수에서는
충분히 오래된 `completed/stale_rejected`만 제한적으로 정리하며 `pending/leased/retryable/needs_embedding/permanent`와 최근
중복 방지 구간은 유지해야 한다.

`memory_reprocessing_jobs`는 완료 15건·permanent 41건으로 크기가 약 0.11MB였으며, 당장 공간 문제의 주원인은 아니었다.

DB 밖의 source 작업 폴더도 확인했다. source root와 `go-service` 아래의 `.tmp-go-*`·`.tmp-*` 개발용 폴더는
87개, 합계 약 20.38GB였다. 대부분 자동 테스트와 package build가 만든 Go build cache·temporary work directory이며
Archive Center의 사용자 DB나 실행 데이터가 아니다. 가장 큰 단일 cache는 약 2.31GB, 1.71GB, 1.36GB였다.
이번 요청에서는 존재와 크기만 확인했고, 이전 작업이 공유하는 cache일 가능성이 있어 임의 삭제하지 않았다.

### 23.6 source 회귀 검증

- 동일 개체의 정확한 현재 이름 + 유일한 과거 canonical 후보 연결: 통과
- 근거가 없거나 alias만 불일치하는 연결의 기존 거부: 유지
- 세계 규칙의 다른 턴 새 버전 저장·같은 턴 중복 방지·최신 버전 조회: 통과
- 감독관 4xx 거절, 빈 응답, timeout 구분과 비밀값 제거: 통과
- `go test ./internal/httpapi -count=1`: 통과
- `go test ./internal/store -count=1`: 통과
- `git diff --check`: 통과
- 이번 절의 `Archive Center.js` 변경량: `+0 / -0`

이 검증은 source와 현재 MariaDB snapshot 기준이다. 갱신된 package 로드, 실제 Ollama Cloud 실패, 다음 신규 턴의
canonical ID 연속성, 리롤 후 세계 규칙 복원은 테스트 빌드에서 다시 확인해야 한다.

### 23.7 `3.9.5-1st-test` 재빌드

기존 `_test-builds/3.9.5-1st-test` 폴더를 새 이름으로 늘리지 않고 현재 source로 갱신했다.

- POSIX 5종 최종 manifest 생성: `2026-08-07T16:09:39Z`~`16:10:12Z`
- Windows 최종 manifest 생성: `2026-08-07T16:10:49Z`
- Linux x64/arm64, macOS Intel/Apple Silicon, Termux arm64, Windows x64의 6개 폴더·6개 ZIP 존재
- 6개 package의 `PACKAGE_FILE_MANIFEST.json` 전체 파일 size·SHA-256 오류 0건
- 6개 package version과 plugin metadata 모두 `3.9.5`
- 6개 backend binary 모두 신규 개체 연결, 세계 규칙 버전 조회, 감독관 실패 분류 표식 포함
- 외부 `SHA256SUMS-3.9.5.txt` 6개 ZIP 전부 대조 오류 0건
- Windows `.runtime`과 `.env.full.local` 원위치 복원
- 임시 보존 폴더 0건
- 빌드 종료 후 28080·3307·8000 listener 0건

처음 두 Windows 교체 시도는 기존 package의 `archive-center-go.exe`와 launcher `cmd.exe`가 파일·폴더 handle을
잡고 있어 삭제 전에 중단됐다. PID 27848의 backend와 PID 10184의 해당 package launcher만 종료한 뒤 재빌드했고,
각 실패에서도 `finally`로 `.runtime`과 `.env.full.local`을 원위치에 복원했다. Codex와 Windows Terminal은 종료하지 않았다.

## 24. 정확한 정식 이름의 기존 개체 ID 재사용

### 24.1 확정 범위

동일 세션에서 `정식 이름 + 개체 종류 + 세션 영역`이 모두 정확히 같은 경우에만 가장 먼저 저장된 기존
`stable_entity_id`를 이후 턴에서도 그대로 사용한다. 일반 이름·Vector 기억 검색, 주입, 평론가 호출 횟수와
프롬프트는 변경하지 않았다.

- `display_name`이 아닌 별칭만 같은 경우에는 ID를 자동 재사용하지 않는다.
- 개체 종류 또는 세션 영역이 다르면 ID를 자동 재사용하지 않는다.
- 과거 ID를 일괄 병합하거나 새 mapping revision을 만들지 않는다.
- 추가 LLM 호출이나 별도 동일성 판정 경로를 만들지 않는다.

### 24.2 구현과 검증

MariaDB의 기존 surface resolver가 같은 정식 이름의 과거 행을 여러 개 발견하더라도, 모든 후보의 정식 이름·개체
종류·세션 영역이 동일하면 가장 이른 ID를 반환하도록 수정했다. 새 턴 저장은 반환된 세 요소가 현재 개체와 정확히
같을 때만 해당 ID를 재사용한다. 이후 턴 삭제·리롤로 해당 surface가 제거되면 남은 surface의 마지막 턴으로
`last_seen_turn`도 복원한다.

- 같은 세 요소의 중복 과거 행에서 최초 ID 선택: 통과
- 세션 영역 불일치: 기존 ambiguous 처리 유지
- 개체 종류 불일치: 기존 ambiguous 처리 유지
- 별칭 surface만 같은 중복 ID: 기존 ambiguous 처리 유지
- 삭제·리롤 정리 후 재사용 ID의 `last_seen_turn` 복원: 통과
- `go test ./internal/store ./internal/httpapi -count=1`: 통과
- 번들 Node를 지정한 `go test ./... -count=1`: 전체 통과
- `git diff --check`: 통과
- `Archive Center.js` 변경량: `+0 / -0`

### 24.3 `3.9.5-1st-test` 갱신

기존 `_test-builds/3.9.5-1st-test`를 같은 이름으로 현재 source에서 다시 빌드했다.

- POSIX 5종과 Windows 1종, 총 6개 폴더·ZIP 생성
- 6개 `PACKAGE_FILE_MANIFEST.json`의 version `3.9.5`, 파일 size·SHA-256 오류 0건
- 외부 `SHA256SUMS-3.9.5.txt`에 6개 ZIP 기록 및 대조 오류 0건
- Windows `.runtime`과 `.env.full.local` 보존
- 갱신 전 실행 중이던 해당 테스트 빌드의 Go·MariaDB·ChromaDB·launcher만 종료
- 빌드 종료 후 28080·3307·8000 listener 0건

## 25. 평론가 문자열 스키마 불일치와 HUD 수동 재시도 복구

### 25.1 실사용 실패와 반복 호출 원인

현재 테스트 세션의 7턴에서 평론가 공급자는 HTTP 200과 JSON 객체를 반환했지만,
`evidence_excerpts[0]`가 문자열이 아니어서 backend의 기존 스키마 검증에서
`CRITIC_SCHEMA_INVALID`로 거절됐다. 공급자 네트워크 재시도는 없었고 trace의
`provider_retry`도 `null`이었다.

최초 실패 뒤 같은 source revision의 재처리 job `#75`가 동일 입력으로 네 번 더 평론가를 호출했다.
호출 시각은 2026-08-08 16:39:38, 16:39:58, 16:40:19, 16:40:39 KST였고 네 번 모두 같은
스키마 오류가 났다. 따라서 이 턴의 평론가 pipeline 호출은 최초 1회와 재처리 4회를 합쳐 총 5회였다.
재처리마다 같은 입력과 같은 프롬프트를 사용했으며 오류를 정정하는 다른 계약이 없었기 때문에 같은
자료형 오류가 반복됐다.

직접 원인은 prompt와 backend 계약의 불일치였다. `critic_system.txt`의 JSON 예시는
`"evidence_excerpts": []`라서 원소 자료형을 보여 주지 않았지만, backend는 이 배열의 모든 원소를
문자열로 요구한다. LLM Gateway 요청은 엄격한 중첩 JSON Schema가 아니라 `json_object` 형식만
요구하므로 문법상 정상인 객체 원소를 막지 못했다.

### 25.2 제한된 수정

- JSON 예시를 `"evidence_excerpts": ["exact source excerpt"]`로 바꿨다.
- 기존 목록 자료형 설명을 고쳐 `evidence_excerpts`가 JSON 문자열 배열임을 직접 명시했다.
- 직접 근거의 선택, 원문 일치 검증, 저장, 폐기 또는 재처리 정책은 변경하지 않았다.
- 자동 재처리 횟수 차단, fallback, 별도 보정 호출은 추가하지 않았다.

### 25.3 HUD 재시도 버튼

HUD는 RisuAI main DOM에 표시되지만 기존 재시도 버튼은 플러그인 설정 iframe의 `document`에
확인 모달을 만들었다. 설정 UI가 닫혀 있으면 사용자는 확인창을 볼 수 없고 Promise도 끝나지 않아
`/turn-workflow/recovery` 요청이 전송되지 않았다. 기존 JS smoke는 확인 모달을 항상 성공하는 mock으로
대체해 이 경계를 검증하지 못했다.

재시도 버튼만 기존 코드베이스에서 이미 쓰고 있는 동기식 브라우저 `confirm`을 사용하게 바꿨다.
확인 뒤 기존 `/turn-workflow/recovery` 요청, 같은 source revision 재개, HUD 상태 갱신 경로는 그대로다.
이번 수정의 `Archive Center.js` 변경량은 `+4 / -4`이며 다른 JS 경로는 변경하지 않았다.

### 25.4 검증과 테스트 빌드

- 번들 Node `node --check Archive Center.js`: 통과
- HUD recovery JS runtime smoke: 확인 1회, backend recovery 요청 1회 통과
- prompt JSON 예시 parse와 기존 backend schema 검증: 통과
- backend의 대상 턴 job reopen과 실행 중 job 비중단 검사: 통과
- 번들 Node를 지정한 `go test ./... -count=1`: 전체 통과
- 기존 `_test-builds/3.9.5-1st-test`의 POSIX 5종과 Windows 1종을 같은 이름으로 재빌드
- 6개 폴더의 managed manifest 전체 size·SHA-256 오류 0건
- 6개 ZIP 내부의 managed manifest 전체 size·SHA-256 오류 0건
- Windows `.runtime`과 `.env.full.local` 4개 파일 원위치 복원 및 hash 불일치 0건
- 실행 중이던 해당 테스트 빌드의 Go backend, MariaDB, ChromaDB와 남은 launcher 종료

자동 검증은 source와 생성 package의 계약을 증명한다. 실제 LLM Gateway가 다음 턴에서 문자열 배열을
반환하고 HUD 버튼 클릭이 실제 RisuAI main 화면에서 확인창과 재처리 요청으로 이어지는지는 갱신된
테스트 빌드의 live 확인 항목이다.

## 26. A-2 동일 입력 재처리와 동일 스키마 오류 반복 중단

### 26.1 수정 범위

이번 작업은 평론가의 정상 최초 호출 입력을 바꾸지 않고, 최초 호출에서 이미 선택된 동적 입력을 같은
source revision의 재처리에 그대로 사용하는 범위로 한정했다.

- 현재 사용자 원문과 assistant 원문 전체
- 직전 1턴 전체와 A-1에서 선택된 관련 DB 보조 자료
- 선택된 archive ledger와 활성 세계 규칙
- 출력 언어 override와 language context
- deterministic preview와 당시 입력 예산 정책

이 묶음을 `critic_reprocessing_input.v1` JSON과 SHA-256으로 `memory_source_revisions`에 최초 1회 저장한다.
같은 hash의 재저장은 idempotent하게 허용하고, 다른 입력으로 덮어쓰는 것은 source revision conflict로
거부한다. 재처리는 최신 채팅 로그, 최신 ledger, 최신 세계 규칙 또는 최신 언어 설정을 다시 선택하지
않고 이 저장본만 읽는다. 스냅샷이 없거나 hash·source revision·현재 원문이 맞지 않으면 provider를
호출하지 않고 `CRITIC_INPUT_SNAPSHOT_INVALID`로 끝낸다. 다른 입력을 만드는 fallback은 추가하지 않았다.
저장 당시의 critic pipeline version과 `critic_system.txt` hash도 현재 실행값과 같아야 한다. 프롬프트가
바뀐 경우에는 새 프롬프트로 재처리하지 않고 동일한 오류로 provider 호출 전에 끝낸다.
스냅샷 DB 저장 자체가 실패해도 정상 최초 평론가 provider 호출은 중단하지 않는다. 이 경우 trace에
`persist_failed`를 남기며, 저장본이 없으므로 이후 재처리는 provider 호출 전에 종료한다.

MariaDB 신규 설치와 기존 설치 모두를 위해 다음 두 컬럼과 `009_critic_input_snapshot.sql`을 추가했다.

- `critic_input_snapshot_json`
- `critic_input_snapshot_hash`

관리자가 canonical raw replay를 새로 시작하는 경우에는 과거 최초 호출 스냅샷이 존재할 수 없으므로,
그 관리 재처리의 첫 입력 선택을 스냅샷으로 저장한 뒤 같은 호출에서 사용한다. 일반 worker 재처리는 이
생성 모드를 사용하지 않는다. `group_admin_rescan.go`의 production 변경은 이 모드 전달 1줄뿐이다.

### 26.2 `CRITIC_SCHEMA_INVALID` 반복 호출

최초 평론가 호출이 `CRITIC_SCHEMA_INVALID`로 끝나면 source revision과 오류 기록은 보존하지만 job 상태를
`permanent`로 저장해 worker가 자동으로 같은 입력을 반복 호출하지 않게 했다. HUD의 기존 수동 재시도는
이 job을 다시 열 수 있다. 수동 재시도 1회도 같은 스키마 오류로 끝나면 즉시 다시 terminal/permanent가
되며 추가 자동 호출은 없다. provider 재시도, schema 보정 호출 또는 새 queue는 추가하지 않았다.

A-2 원래 계약에 포함된 focused world-rule audit가 실패한 경우에는 최초 평론가 결과만 성공으로 저장하지
않고 전체 derivation을 `CRITIC_WORLD_RULE_AUDIT_FAILED` 재처리 가능 오류로 남긴다. 정상 audit와 이미
추출된 world rule 저장 경로는 변경하지 않았다.

### 26.3 변경하지 않은 경계

- `Archive Center.js`: 이번 작업 변경 `+0 / -0`
- `critic_system.txt`: 이번 작업 변경 없음
- 직접 근거 선택·원문 일치 검증·저장·폐기 조건: 변경 없음
- 개체 ID, 기억 검색·주입, 삭제·리롤·원문 인식: 변경 없음
- A-3 `retry_after` 타이머: 구현하지 않음
- 정상 평론가 provider 호출과 정상 결과 저장 계약: 변경 없음

공통 queue 함수 인자를 늘려 explorer·HUD 호출부까지 수정했던 중간 구현은 제거했다.
최종 production diff에서 `group_memory_explorer_write.go`와 `turn_workflow_hud.go` 변경은 0이며,
`group_admin_rescan.go`는 위의 최초 스냅샷 생성 모드 1줄만 남는다.

### 26.4 검증과 `3.9.5-1st-test` 갱신

- 최초 호출과 재처리의 최종 평론가 user prompt 완전 일치: 통과
- 재처리 사이에 채팅 로그·host context·언어 설정을 바꿔도 저장 입력만 재사용: 통과
- system prompt hash가 바뀐 스냅샷은 provider 호출 전 거부: 통과
- 스냅샷 저장 실패가 정상 최초 평론가 provider 호출을 막지 않음: 통과
- 현재 턴 원문 전체와 저장 언어 설정 보존: 통과
- 최초 schema-invalid job 자동 실행 제외: 통과
- 수동 재시도 후 같은 schema 오류 provider 호출 1회에서 종료: 통과
- world-rule audit 실패 시 전체 derivation 성공 저장 금지: 통과
- MariaDB 스냅샷 동일 hash idempotency와 다른 hash 덮어쓰기 거부: 통과
- 위 핵심 표적 테스트 `-count=5`: 5회 모두 통과
- `go test ./internal/httpapi ./internal/store ./cmd/mariadb-schema`: 통과
- 번들 Node를 지정한 `go test ./...`: 전체 통과
- 번들 Node `node --check Archive Center.js`: 통과
- `git diff --check`: 통과

실행 중이던 테스트 빌드의 Go backend, MariaDB, ChromaDB를 종료한 뒤 기존
`_test-builds/3.9.5-1st-test`를 같은 이름으로 갱신했다. Windows 폴더를 잡고 있던 종료된 launcher의
directory handle 때문에 폴더 자체 삭제는 실패했지만, 별도 임시 출력에서 Windows package를 완성한 뒤
기존 빈 폴더의 내용만 교체했다. 사용자 `.runtime`과 `.env.full.local`은 보관 후 원위치 복원했다.

- POSIX 5종과 Windows 1종, 총 6개 package 폴더·ZIP 생성
- 6개 package의 `PACKAGE_FILE_MANIFEST.json` size·SHA-256 오류 0건
- 6개 package와 6개 ZIP에 `migrations/009_critic_input_snapshot.sql` 포함
- 6개 backend binary에 `critic_reprocessing_input.v1` 포함
- 외부 `SHA256SUMS-3.9.5.txt` 6개 ZIP 기록 및 hash 대조
- Windows `.env.full.local` 복원 hash 일치
- 빌드 종료 뒤 28080·3307·8000 listener 없음

자동 검증은 동일 입력 보존, 반복 호출 중단, package 포함을 증명한다. 실제 provider가 schema 오류를
반환한 뒤 자동 호출이 더 생기지 않는지와 HUD 수동 재시도가 1회만 실행되는지는 갱신된 테스트 빌드의
live 확인 항목이다.

## 27. 모델 입력의 불투명 ID 제거와 출판사 LLM 표시 명칭 통일

### 27.1 모델 입력 정리

본문 모델에 전달되는 장기 기억 문자열에서 의미 없는 내부 식별자만 제거했다.

- `[character-memory:<SHA-256>]` 접두어 제거
- `source_ref=precise_memory:<UUID>` 제거
- 인물 이름, 개체 종류, 관계 방향, privacy guard, `source_turn`은 의미 있는 문맥이므로 유지

DB의 stable entity ID, precise-memory unit ID, source revision과 source-ref는 삭제하지 않았다. 이 값들은
동일 개체 연결, 중복 방지, 전달 lineage와 출판사 LLM 결과 검증에 계속 사용된다. 변경점은 최종 본문
모델 입력에 식별자 문자열을 표시하지 않는 것뿐이다.

character-memory 전달 여부가 과거에는 렌더링된 `[source_ref]` 문자열 검색에 의존했으므로, 이를 같은
delivery plan 안에서 실제 전달된 문장 key와 정확히 대조하도록 바꿨다. 새 fallback, 차단 조건, 저장
경로 또는 DB 필드는 추가하지 않았다.

### 27.2 출판사 LLM 표시 명칭

설정, HUD, 상태 테스트, 경고, 투명성 화면과 본문 guidance 제목의 외부 표시를 `출판사 LLM`으로
통일했다. 한국어·영어·일본어 표시를 함께 정리했다. 내부 호환 계약은 변경하지 않았다.

- 유지: `supervisor_*` 설정 키
- 유지: `/supervisor` API 경로
- 유지: `supervisor_result`, `supervisor_scene_proposal` JSON 필드
- 유지: Go/JavaScript 내부 함수와 구조체 이름

내부 이름까지 바꾸면 기존 pluginStorage, 설정 동기화와 프론트엔드-백엔드 계약이 깨질 수 있으므로
이번 범위에서 제외했다. `Archive Center.js`는 표시 문자열 64줄을 일대일 교체했으며 새 함수·상태·API는
추가하지 않았다.

### 27.3 검증과 `3.9.5-1st-test` 갱신

- character-memory ID 비노출, 내부 delivered source-ref 유지, precise-memory UUID 비노출 표적 테스트
  `-count=5`: 5회 모두 통과
- `go test ./internal/httpapi -count=1`: 통과
- 번들 Node `node --check Archive Center.js`: 통과
- `git diff --check`: 통과
- 기존 사용자 `.runtime`과 `.env.full.local` 보관 후 원위치 복원, env SHA-256 일치
- POSIX 5종과 Windows 1종, 총 6개 package 폴더·ZIP을 기존
  `_test-builds/3.9.5-1st-test` 이름으로 재생성
- 6개 폴더와 6개 ZIP의 `PACKAGE_FILE_MANIFEST.json` size·SHA-256 오류 0건
- 외부 `SHA256SUMS-3.9.5.txt`에 6개 ZIP 기록 후 hash 대조
- 6개 package의 JavaScript와 출판사 시스템 프롬프트 hash가 각각 동일함을 확인
- 빌드 종료 뒤 28080·3307·8000 listener 없음

자동 검증은 소스·패키지 포함과 내부 참조 연속성을 확인한다. 실제 RisuAI 본문 입력에서 해시·UUID가
사라지고 설정·HUD 명칭이 출판사 LLM으로 보이는지는 갱신된 테스트 빌드의 live 확인 항목이다.

## 28. Ollama Cloud 출판사 빈 응답 수정과 1.0 방향 비교

### 28.1 빈 응답의 직접 원인

출판사 설정 UI의 기본 추론값은 `none`이지만 실제 턴의 `runSupervisorLLM`은 이미 저장된
`reasoning_preset`, `reasoning_effort`, reasoning budget과 max-completion 값을 공급자 요청에 전달하지 않았다.
공용 OpenAI 호환 호출부도 `reasoning_effort=none`을 모든 공급자에서 생략했다. Ollama의 OpenAI 호환
`/v1/chat/completions`는 `reasoning_effort: "none"`을 정식으로 지원하므로, 생략은 "추론 끄기"가 아니라
모델 기본값 사용이었다. DeepSeek 계열이 기본 추론을 수행하면 제한된 출력 토큰을 reasoning에 사용하고
최종 `message.content` 없이 HTTP 200으로 끝날 수 있었다.

### 28.2 제한된 수정

- 실제 출판사 턴 호출이 기존 runtime의 max-completion·reasoning 설정을 최초 공급자 요청에 전달한다.
- Ollama에 저장값이 `none`이면 최초 요청에 `reasoning_effort: "none"`을 그대로 보낸다.
- 설정 화면의 출판사 호출 테스트도 Ollama + `none` 조합에서 같은 요청을 보낸다.
- 빈 `message.content`나 `reasoning_content`를 출판사 지시문으로 승인하지 않는다.
- 자동 재시도, fallback, 별도 호출 경로, 새 상태, 기억 주입 변경은 추가하지 않았다.

### 28.3 1.0 출판사와 현재판 비교

1.0의 출판사는 최근 대화와 서사 문맥을 읽고 JSON의 `book_author`와 `director`를 반환했다.
`book_author`는 현재 arc·근접 목표·다음 beat·guardrail을, `director`는 현재 장면 mandate·필수 결과·금지
행동·pressure를 담당했다. 해당 직접 호출은 max token과 reasoning 옵션을 강제로 넣지 않았다. 실패 시
기본 지시문을 대신 반환하는 동작도 있었지만, 이는 실제 오류를 숨기는 fallback이므로 이번 수정에
되살리지 않았다.

현재판 출판사는 DB에서 선택·전달된 자료를 바탕으로 `supervisor_scene_proposal`의 짧은 source-linked
표현 지시만 반환한다. 따라서 외부 이름만 출판사 LLM으로 바뀐 상태이며 1.0의 `book_author + director`
역할 전체가 복구된 것은 아니다. 1.0 방향 복구는 별도 작업 경계에서 다음 세 부분을 함께 바꿔야 한다.

1. 출판사 프롬프트 출력 계약을 `book_author + director` 중심으로 복구
2. 기존 출판사 결과 파서와 본문 guidance 조립이 그 두 역할을 실제 메인 모델 입력에 전달
3. 현재 DB 검색·기억 주입·턴 저장은 그대로 유지하고 1.0의 기본값 fallback은 복구하지 않음

이번 절에서는 빈 응답의 최초 요청만 수정했으며 위 역할 복구는 아직 적용하지 않았다.

### 28.4 자동 검증

- Ollama 출판사 최초 요청이 `reasoning_effort: "none"`, 설정된 `max_tokens`를 보내고 호출은 정확히 1회: 통과
- 빈 최종 content는 기존대로 `publisher_llm_empty_content`로 분류: 통과
- `go test ./internal/httpapi -count=1`: 통과
- 번들 Node 경로를 지정한 `go test ./... -count=1`: 전체 통과
- 번들 Node `node --check Archive Center.js`: 통과
- 명칭 smoke의 한국어·영어·일본어 기대값을 현재 `출판사 LLM` 표시와 일치시킴
- 기존 `_test-builds/3.9.5-1st-test`를 같은 이름으로 POSIX 5종·Windows 1종 재생성
- Windows `.runtime` 3개 파일과 `.env.full.local`을 보관 후 원위치 복원, env SHA-256 일치
- 6개 package 폴더와 6개 ZIP의 managed file size·SHA-256 오류 0건
- 외부 `SHA256SUMS-3.9.5.txt` 6개 ZIP hash 대조 통과
- 6개 package JavaScript에 Ollama `reasoning_effort: none` 전달 코드 포함
- 빌드 종료 후 28080·3307·8000 listener와 임시 보관 폴더 없음

## 29. 현재 핵심 작업 경계 A-C 통합 정리

기준일: 2026-08-09
이 절은 최초 작업 프롬프트에서 구분한 A·B·C의 현재 작업 지도다. 과거 절의 구현 기록을 다시
구현 완료로 선언하지 않으며, source·자동 회귀·테스트 패키지·실사용 증거는 계속 구분한다.
출판사의 1.0 `book_author + director` 역할 복구는 이 목록의 D가 아니며 4.0 정밀 회상·최종
주입 로드맵으로 이관한다.

### 29.1 A — 평론가 입력·재처리·재시도

#### A-1. 평론가 입력

- 현재 턴의 실제 visible user/assistant 원문은 축약하지 않고 유지한다.
- 과거 active chat 전체를 provider에 반복 전달하지 않는다.
- 직전 canonical 턴과 현재 턴에 관련된 DB 기억·근거·관계·상태·세계 규칙만 기존 Go 선택·예산
  owner가 조립한다.
- 고정된 새 context cap을 추가하지 않고 기존 모델 문맥 정보와 입력 예산을 사용한다.
- 선택·제외·절단·최종 prompt 크기를 기존 trace에 기록한다.
- 현재 source에는 이 입력 축소와 평론가 고정 프롬프트 중복 정리가 반영됐고 자동 회귀가 통과했다.
  Luna 실사용 토큰 측정도 기록됐지만 장기 사용 품질과 여러 provider 실측은 계속 확인 대상이다.

#### A-2. 동일 입력 재처리

- 최초 평론가 호출에서 실제 사용한 bounded input snapshot과 language override를 source revision에
  보존하고, worker가 동일한 versioned input contract를 재현한다.
- stale revision 결과는 current 결과를 덮지 않는다.
- 동일한 `CRITIC_SCHEMA_INVALID` 입력을 자동으로 반복 호출하지 않으며 HUD 수동 재시도는 실제
  새 시도로 연결한다.
- source와 자동 회귀는 반영됐고, 실제 실패·재처리 상황에서의 장기 피드백은 남아 있다.

#### A-3. `retry_after` timer

- 기존 worker가 가장 가까운 `retry_after`를 기준으로 단일 timer를 예약하는 원래 계약이다.
- 새 worker·polling loop·별도 queue·새 backoff 수치는 만들지 않는다.
- 현재 우선순위에서는 보류했으며 구현 완료로 표시하지 않는다.

### 29.2 B — Pending confirmation·Historical Queue·HUD·복구

- B-1은 pending confirmation과 transport retry가 각각 자기 기존 용량을 사용하고, 같은 identity를
  새 행으로 누적하지 않고 기존 상태를 갱신하는 계약이다.
- B-2는 Historical Queue를 `queue_kind + state + reason_code`로 집계하고, terminal 또는 기존
  lifecycle상 stale/superseded인 항목만 `종료 항목 정리` 대상으로 삼는다.
- B-3은 복구 버튼이 기존 backend recovery API를 실제 호출하고 requested·running·success/failure를
  기존 HUD snapshot으로 표시하는 계약이다.
- B-4는 현재 턴 workflow와 과거 queue 오류를 분리하고, 기본 HUD에는 짧은 stable error를,
  provider·model·HTTP status·raw preview는 상세 진단에 표시하는 계약이다.
- 출판사 오류 수정은 pending storage, queue, recovery API, HUD state owner를 변경하지 않는다. B 전체의
  loaded artifact·실제 pluginStorage·실제 MariaDB 동시 검증은 별도 증거로 남긴다.

### 29.3 C — Canonical visible raw·부분 수정·streaming·삭제·리롤

- C-1·C-2는 RisuAI가 실제 표시·저장한 visible user/assistant 원문을 canonical raw와 hash의 기준으로
  삼고, 평론가 sanitize 결과나 provider hidden reasoning을 원문으로 대체하지 않는 계약이다.
- C-3의 직전 완료 턴 부분 수정은 visible raw hash 차이를 먼저 판정한 뒤 persistence fence를 적용하는
  계약이지만, 사용자가 별도 집중 작업으로 미뤘으므로 현재 우선순위에서 제외한다.
- C-4의 streaming 중간 chunk 실시간 원문 인식은 공식 host API가 없어 `unsupported`다. 공식 final
  lifecycle로 확인된 최종 출력만 기존 complete-turn 경로에서 처리하며 DOM watcher·polling·고정 대기
  fallback을 만들지 않는다.
- C-5에서 RisuAI 화면 기록 정리(`/del`, `/cut`, 휴지통)는 Archive Center 기억 삭제와 분리한다.
  공식 즉시 삭제 lifecycle이 없는 host에서 추정 observer를 다시 만들지 않으며, 실패한 즉시 HUD
  구현은 제거된 상태다.
- C-6 리롤은 기존 host observation과 backend logical-turn owner를 사용하며 출판사 오류 수정에서 변경하지 않는다.

### 29.4 3.9 출판사 종료선과 4.0 이관

3.9 계열의 출판사 작업은 현재 bounded response hint 계약을 유지하면서 재현된 호출 오류가 나지 않게
수정하는 데서 끝낸다.

- 저장된 endpoint·model·max-completion·reasoning 설정을 기존 provider adapter가 해당 provider의
  지원 형태로 전달한다.
- 문자열 `message.content`, 배열형 text content part, `choice.text`처럼 실제 지원 provider에서
  관찰되는 정상 응답을 공통 텍스트로 해석한다.
- 빈 content·잘못된 JSON·HTTP 오류·timeout은 stable failure로 구분하고 본문 요청은 계속한다.
- 특정 provider나 모델명에 묶인 출판사 본체 분기, 자동 retry, 기본 directive fallback, 병렬 호출
  경로를 추가하지 않는다.
- 3.9에서 `book_author + director` 역할, 단일 coherent narrative-plan block, 턴 간 arc/beat carry를
  추가하지 않는다.

1.0 출판사 역할 복구는
[`4.0-precision-recall-injection-roadmap.md`](4.0-precision-recall-injection-roadmap.md)가 소유한다.
4.0은 현재 기억 선별·delivery lineage·비밀 경계·명시적 실패를 유지한 채 기존 단일 출판사 owner를
`book_author + director`와 단일 본문 계획 block으로 확장한다. 4.1은 그 plan의
edit/delete/reroll/branch lifecycle을 검증하고, 4.9는 별도 owner를 만들지 않고 같은 계약 위에
Recall Auditor·Director·Reviewer 다단계 검토만 확장한다.

## 30. POSIX 재실행 시 이미 준비된 런타임 즉시 사용

기준일: 2026-08-09

Windows BAT 실행기와 같은 기준으로 Linux·Ubuntu·macOS·Termux의 기존 공통 POSIX 실행기를
정리했다. MariaDB server/client, Python, 그리고 선택한 local vector mode에 필요한 ChromaDB
런타임이 이미 준비되어 있으면 OS package manager와 Python package bootstrap을 호출하지 않고
기존 MariaDB·ChromaDB를 바로 시작하거나 이미 열린 port를 그대로 사용한다.

- Linux·Ubuntu: 준비 완료 시 `apt-get`·`dnf`·`yum`·`pacman`·`zypper`·`apk`를 건너뛴다.
- macOS: Homebrew 환경만 로드하고 준비 완료 시 `brew update/install`을 건너뛴다.
- Termux: 준비 완료 시 `pkg update/install`을 건너뛴다.
- Linux·macOS: 관리형 venv의 `chromadb==1.5.9`가 정상이면 반복적인 pip upgrade/install을
  건너뛴다.
- macOS 한 줄 설치 후 stable launcher도 내부 wrapper를 직접 호출하지 않고 공개
  `Start Archive Center macOS.command`를 사용하여 `full_local + local_native`를 유지한다.
- 필요한 구성요소가 없으면 기존 OS별 설치 경로를 그대로 사용한다. 별도 fallback이나 병렬 실행
  경로는 추가하지 않았다.

검증:

- `sh -n ops/full-package-posix/start-full-posix.sh`: 통과
- Linux·macOS·Termux 준비 완료 fixture에서 OS package manager 호출 0회: 통과
- 준비된 Linux·macOS ChromaDB venv에서 pip bootstrap 호출 0회: 통과
- Linux ChromaDB 런타임 누락 fixture에서 기존 `apt-get` 설치 경로 진입: 통과
- macOS 최초 설치와 갱신 후 재실행에서 `--profile full_local --vector-mode local_native` 전달: 통과
- `scripts/test-simple-fresh-install.sh`: 통과

## 31. 3.9.9 이후 UI 직접 업데이트 계약

기준일: 2026-08-09

기존 업데이트 확인은 GitHub에 더 높은 버전과 현재 OS용 asset·SHA-256이 있는지를 먼저 보고
`업데이트 가능`을 표시했다. 실제 ZIP의 관리 파일 전체, 누적 DB migration, 실행기 적용 권한,
직접 버전 점프 가능 여부는 버튼을 누른 뒤에야 확인되는 구조였다. 따라서 기존 UI의
`업데이트 가능`은 설치 가능 보장이 아니라 새 버전 알림에 가까웠고, 3.9.9에서 4.0 또는 4.2로
직접 올라가도 호환되어야 한다는 현재 요구와 맞지 않았다.

피드백 완성판의 버전을 3.9.9로 확정하면서 다음 계약으로 바꿨다.

- 자동 업데이트 기준선은 피드백 완성판 3.9.9다. 피드백 이전 3.9.0과 테스트 빌드 3.9.5는 자동 업데이트
  출발점으로 인정하지 않는다.
- `/update/check`가 정확한 현재 OS·architecture용 후보 ZIP을 임시로 받아 SHA-256과 패키지 내부를
  실제 검증한 뒤에만 `update_available=true`를 반환한다.
- 후보는 `PACKAGE_FILE_MANIFEST.json`, 검증 완료된 `PACKAGE_RELEASE_STATUS.json`, 직접 업데이트
  v2 migration 계약을 모두 포함해야 한다.
- v2 migration 계약은 `minimum_source_version=3.9.9`, `direct_update_supported=true`,
  `migration_inventory=cumulative_complete`, `managed_files=complete_manifest`,
  `database_policy=expand_first_old_backend_compatible`를 요구한다.
- 3.9.9에서 4.2로 중간 버전을 건너뛰더라도 후보가 001부터 최신 번호까지 빠짐없는 누적 migration을
  포함하면 직접 업데이트할 수 있다. 기존 번호의 SQL이 바뀌었거나 중간 번호가 비면 UI에서
  업데이트 가능으로 표시하지 않는다.
- 관리 파일 추가·변경·삭제를 전체 manifest 기준으로 적용한다. POSIX에서도 실제 삭제 경로의
  대소문자를 보존한다.
- 업데이트 확인, 다운로드, 적용 단계에서 후보를 다시 검증한다. 적용 후 새 백엔드의 `/ready`와
  `/version`을 확인한 다음 commit하며, 실패하면 관리 파일을 rollback한다. 사용자 DB와 로컬 설정은
  rollback 대상으로 삼지 않는다.
- 백엔드가 환경 변수만으로 자신을 관리형 실행기라고 주장할 수 없도록, Windows와 POSIX 실행기가
  백엔드 시작 직전에 일회용 launcher session을 만들고 백엔드가 이를 한 번 소비하도록 했다.
- POSIX 교차 빌드는 실제 대상 기기 검증 전에는 `release_ready=false`다. 이 패키지는 신규 설치에는
  사용할 수 있지만 자동 업데이트 후보로는 표시되지 않는다. 실제 릴리스 빌드는 대상 OS 검증 후
  `VerifiedReleaseTarget`을 명시해야 한다.

검증 결과:

- updater core·HTTP·JavaScript 범위 회귀 테스트: 통과
- Windows 실제 updater CLI의 apply/status/commit, 명시적 rollback, 중단 복구, readiness 실패 rollback,
  변조 ZIP 사전 거부: 통과
- Windows 패키지 백엔드 `noop` 기동·`/ready` 확인·종료: 통과, 잔류 프로세스 없음
- Windows PowerShell 빌더·실행기·검사 스크립트 구문: 통과
- POSIX 공통·Linux·macOS·Termux 실행기 shell 구문: 통과
- 6개 패키지 폴더와 6개 ZIP의 모든 관리 파일 크기·SHA-256 대조: 통과
- `SHA256SUMS-3.9.5.txt`의 6개 ZIP 해시 대조: 통과

전체 Go 저장소 회귀에서는 업데이터와 무관한 기존
`TestArchiveCenterJSSameTurnOverlayInjectionAndTraceRuntime`의
`backend-off supervisor fallback missing` 한 건이 실패했다. 업데이터 범위 테스트와
`internal/httpapi` 전체 테스트는 통과했다. 실제 MariaDB·ChromaDB를 사용한 업데이트와 실제
Linux·macOS·Termux 장치 릴리스 검증은 이번 Windows 소스 검증으로 완료됐다고 주장하지 않는다.

## 32. 피드백 완성판 버전 3.9.9 확정

기준일: 2026-08-09

기존 `3.9.5-1st-test`는 개발 중간 테스트 빌드로 남기고, 피드백 완성판의 제품 버전과 최초 자동 업데이트
기준선을 `3.9.9`로 올렸다. 과거 3.9.5 테스트 결과와 토큰 실측 기록은 당시 증거이므로 이전 절에서
이름을 바꾸거나 삭제하지 않는다.

이번 버전 전환에서 변경한 범위:

- RisuAI plugin metadata, HUD build ID, build notes: `3.9.9`
- 소스 직접 실행 시 Go 기본 개발 버전: `3.9.9-dev`
- Windows·POSIX 패키지 빌더 기본 package version: `3.9.9`
- 직접 UI 업데이트의 최소 source version: `3.9.9`
- 업데이트·plugin version 회귀 기대값: `3.9.9`

이번 변경은 버전 정체성과 업데이트 기준만 바꿨다. 턴 인식, 원문 저장, 삭제·리롤, 평론가, 출판사,
기억 선택·주입 정책은 변경하지 않았다. `3.9.9` 최종 패키지는 남은 비실사용 작업을 닫은 뒤 새 폴더로
빌드해야 하며, 현재 `_test-builds/3.9.5-1st-test`를 3.9.9 완성판으로 간주하지 않는다.

검증:

- `node --check "Archive Center.js"`: 통과
- `go test ./internal/config ./internal/packageupdate -count=1`: 통과
- 업데이트 HTTP 범위 회귀: 통과
- plugin version·build identity 회귀: 통과

## 33. `character_deltas.name` 계약 복구

기준일: 2026-08-09

평론가 결과의 `character_deltas`는 backend에서 인물 상태로 저장될 때 `name`이 필요하지만, 평론가 시스템
프롬프트의 JSON 예시가 이 필수를 분명하게 보여 주지 못하면 이름 없는 delta가 생성되어 `missing_name`으로
폐기될 수 있었다.

수정 범위는 다음 두 항목으로 제한했다.

- `critic_system.txt`의 `character_deltas` 예시에 `{"name": ""}`를 명시한다.
- 해당 예시를 사용해 이름이 있는 인물 상태가 실제 `character_states` 저장 경로에 들어가고
  `missing_name`으로 폐기되지 않는 회귀 검사를 둔다.

변경하지 않은 범위:

- 직접 근거 선택·원문 일치·저장·폐기 정책
- 관계·KG·개체 ID 계약
- 이름이 없는 결과를 임의 이름으로 보정하는 fallback
- 값 폐기 조건이나 별도 보호 장치

회귀 검사:

- `TestCriticCharacterDeltaNameContractPersistsState`
- 2026-08-09 현재 source에서 `-count=5`: 5회 모두 통과
- 시스템 프롬프트 JSON parse
- `character_deltas[0].name` 존재
- 인물 이름 `Mina`와 상태가 1건 저장됨
- `missing_name` skip reason이 생성되지 않음

## 34. Host message index와 canonical DB turn 분리

기준일: 2026-08-09

장기 세션에서 HUD의 Host turn과 Backend turn이 크게 어긋나면서
`source_acceptance_session_tail_conflict`, `pending_confirmation_persistence_failed`, raw save 실패가 이어진
사례를 확인했다. RisuAI의 message index를 Archive Center의 영구 turn 번호와 같은 값으로 사용하면, 화면에서
메시지가 잘리거나 시작 위치가 달라진 세션에서 낮은 Host index가 이미 존재하는 DB turn과 충돌할 수 있다.

현재 source의 해석 기준은 다음과 같다.

- RisuAI message index는 어느 Host 메시지를 관찰했는지 나타내는 참조값이다.
- Archive Center의 신규 canonical turn 번호는 해당 세션에 저장된 DB tail 다음 값이다.
- 같은 `logical_turn_id`의 활성 턴이 이미 있으면 DB tail이 앞서 있어도 그 기존 canonical turn을 유지한다.
- 같은 logical turn에 새 generation·swipe가 오면 신규 turn append가 아니라 기존 turn의 리롤 교체로 처리한다.
- 새 저장이 실제 persistence owner에서 다른 turn으로 확정되면 source acceptance ledger도 그 canonical turn으로
  rebind한다.

회귀 검사:

- 낮은 Host index가 들어와도 신규 raw가 DB tail+1에 저장됨
- DB tail이 앞선 상태에서도 기존 logical turn 리롤은 원래 canonical turn을 유지함
- 리롤 교체 시 이전 source revision과 vector 결과가 supersede·정리됨
- Host index와 DB turn을 같은 절대 숫자로 강제하는 별도 mapping revision은 추가하지 않음

2026-08-09 현재 위 Host/DB turn 표적 회귀를 다시 실행해 모두 통과했다. 같은 문서의 Historical Queue
집계·terminal/retryable 분리·pending recovery 표시 회귀와 pluginStorage 종료 이력 정리 회귀도 함께 통과했다.
