# Archive Center 4.2 작업 기록

기준일: 2026-09-03

기준선:

- 공개 4.1 parent: `574c2d5b2295b0d50630474fb695055d5417777c`
- 로컬 4.2 계획 기준점: `48a63711e9acc79ab514a4bb0a1ea183dd5b65b3`
- 로컬 4.2 구현 기준점: `43bc20a19698d8e85c910c1b04f33b557eca42c7`
- 활성 작업 branch: `work/4.2.0`

이 문서는 4.2-A부터 4.2-G까지의 구현과 증거 단계를 기록한다. 소스·자동 회귀·패키지·
로드된 RisuAI·실제 MariaDB/ChromaDB·provider payload·displayed-final은 서로 대신하지 않는다.

## 1. 4.2-A — 저장→점수→payload 기준선

기존 production 조립 경로에서 저장 source, 검색 결과, lane 후보, 최종 예산, 렌더링,
`payload_application_plan.v1`과 output-fidelity 관찰을 이어 확인했다. 4.1 경로는 후보가 lane용
문자열로 평탄화된 뒤 일부 저장 점수와 source identity가 최종 예산까지 유지되지 않고,
objective event K만 전역 기억 상한처럼 사용되는 제한이 있었다.

4.2는 별도 검색기나 JavaScript selector를 만들지 않고 같은 Go 조립 경로에
`memory_delivery_plan.v2`를 연결한다. 현재 입력, 직접 근거, explicit correction, 비밀·관점,
branch/revision 권한은 점수 경쟁 밖의 기존 권한으로 유지한다.

## 2. 4.2-B/C — Priority Score와 사실 단위 후보

Go의 `prepare_turn_priority_memory.go`가 다음 값을 후보에서 최종 선택까지 보존한다.

- `canonical_fact_id`, canonical key, source ref/table/row/occurrence, lane
- 중간에서 자르지 않은 자연어 사실과 문자 수
- relevance, 저장 importance, recency, continuity bonus
- `priority_score.static.v1` final score와 결정적 rank
- selected/deferred/superseded 상태, reason, `superseded_by`

점수는 다음 정적 시작식이다.

```text
final_score = relevance * 0.60
            + importance * 0.25
            + recency * 0.15
            + continuity_bonus
```

Memory의 importance/emotional/narrative 신호, pending thread priority, storyline/canonical
confidence, Persona·private memory의 importance를 source metadata로 연결했다. 큰 JSON은
경로별 완전한 사실로 투영하되 원본 row·JSON은 바꾸거나 삭제하지 않는다. 별도 LLM 호출이나
DB migration은 추가하지 않았다.

## 3. 4.2-D/E — current resolution, 전역 순위와 핵심 기억 K

요청 단위 projection에서 structured current-state source는 canonical field identity를 사용해
현재 revision을 primary로 고른다. 검토된 인물 alias는 character-state identity에만 사용한다.
자연어 사건·private/perspective occurrence는 같은 문구라는 이유로 합치지 않으며, 안정적인 같은
occurrence에서 계획·시작·완료가 확인될 때만 현재/완료 표현을 primary로 둔다. 과거 자료는
삭제하지 않고 trace의 historical/superseded 후보로 남는다.

모든 eligible scored-memory 사실은 lane 선착순 대신 전역 `final_score` 순서로 경쟁한다.
`core_objective_memory_max_items`는 이제 objective event 전용 상한이 아니라 기본값 5의 전역
핵심 기억 K다. K 또는 문자 예산 중 먼저 닿는 곳에서 멈추며, K 뒤 남은 공간을 저득점 기억으로
채우지 않는다. 상위 K의 한 oversized 사실이 defer되어도 다른 상위 K 사실은 검사한다.

명시적인 legacy custom lane-budget mode는 기존 사용자의 설정 의미를 바꾸지 않기 위해 4.1
경로를 유지한다. 4.2 전역 priority 경로는 기본 auto budget mode의 owner이며, custom mode를
몰래 재해석하거나 사용자의 명시 설정을 삭제하지 않는다.

## 4. 4.2-F — Priority Memory Pack과 Publisher·전송 연결

선택된 사실만 source ref와 함께 짧은 자연어 pack으로 렌더한다. 기존
`publisher_plan.v2`는 새 Publisher를 만들지 않고 실제 selected/delivered ref를 사용한다.
Persona/private 후보가 선택된 경우 기존 해석·privacy guidance를 K를 소비하는 별도 기억으로
만들지 않고 해당 후보의 렌더링 guidance로 붙인다.

Text, Google AI Studio/Vertex PDF, LLM Gateway PDF, Provider Manager PDF는 모두 동일한 완성
`payload_application_plan.v1`의 selected fact ID·순서·렌더링 hash를 소비한다. PDF 경로가 별도
검색이나 순위를 만들지 않는다.

## 5. 4.2-G — 사용자 선택형 저장 확정 시점

고급 설정에 `저장 확정 시점`을 추가했다.

- `응답 직후` (`immediate_after_response`): 기본값이며 4.1 `afterRequest → /complete-turn`
  경로를 그대로 사용한다.
- `다음 사용자 입력 시` (`next_user_input`): 직전 표시 후보의 최소 Host 좌표와 hash만 pending
  marker로 보관하고, 실제 새 사용자 행의 `beforeRequest`에서 그 직전 최종 pair를 기존
  `/complete-turn`으로 보낸다.

Go가 `turn_finalization_policy.v1`로 mode를 확정한다. JavaScript는 Go 정책이 없으면 4.1 기본
동작을 사용하며 mode를 독자적으로 판정하지 않는다. 같은 user row의 리롤·편집 재생성은 pending
marker만 최종 assistant로 교체하고 저장하지 않는다. 내용이 같더라도 새 user row이면 직전 pair를
확정한다. 다른 branch/session은 marker를 소비하지 않는다.

직전 complete-turn은 현재 prepare-turn과 같은 시간대에 시작하지만 await하지 않는다. 느리거나
실패한 직전 Critic은 현재 본문 요청을 막지 않으며 두 번째 Critic, 저장 API, scheduler, hidden retry,
종료·세션 전환 auto-save를 만들지 않았다. 안정적인 직전 행을 확인할 수 없을 때는 marker를
남기고 현재 요청을 계속한다. 이 확인은 정상 출력 거부 조건이 아니라 잘못된 과거 pair 저장을
피하기 위한 pending 대상 identity 확인이다.

## 6. 자동 회귀 결과

2026-09-03 현재 다음 검사가 통과했다.

- `node --check "Archive Center.js"`
- `go test ./internal/httpapi -count=1`
- `go test ./cmd/js-route-variant-smoke -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `scripts/test-simple-fresh-install.ps1`의 Windows checksum/install-pointer와
  Linux x64/arm64·macOS Intel/Apple Silicon·Termux arm64 한 줄 설치 계약

검증 범위에는 stored-score 전파, structured fact 분해, current resolution, 검토된 alias,
occurrence 보존, importance 우선순위, 무관 importance takeover 방지, global K, no-fill,
oversized top-K, direct-evidence/privacy 보존, Text/PDF plan 일치, 기본 즉시 저장, 같은 행 리롤,
편집 재생성, 동일 문장 새 행, 비대기 직전 Critic, 재시작 marker 복원, 다른 branch 격리가 포함된다.

## 7. 완료 증거 단계

| 단계 | 상태 | 증거 |
| --- | --- | --- |
| `SOURCE_IMPLEMENTED` | 완료 | 활성 Go/JavaScript/DTO contract 구현 |
| `REGRESSION_VERIFIED` | 완료 | 위 대상·전체 회귀 통과 |
| `PACKAGE_BUILT` | 완료 | 공식 full-package 빌더의 green package, manifest/hash 검증 |
| `LOADED_RISU_VERIFIED` | 미확인 | 실제 RisuAI에서 새 plugin hash 로드 필요 |
| `REAL_MARIADB_CHROMA_VERIFIED` | 미확인 | 기존 4.1 DB를 사용하는 package stack 확인 필요 |
| `PROVIDER_PAYLOAD_VERIFIED` | 미확인 | 실제 provider payload의 selected fact/hash 확인 필요 |
| `DISPLAYED_FINAL_EFFECT_VERIFIED` | 미확인 | 동일 DB/input 4.1/4.2 A-B 결과 확인 필요 |

마지막 네 실환경 단계가 끝나기 전에는 기억 품질 개선이나 4.2 릴리스 완료를 주장하지 않는다.

## 8. Windows 4.2 테스트 패키지

공식 `ops/build-full-package.ps1`로 다음 단일 테스트 패키지를 생성했다.

- 폴더: `_test-builds/Archive-Center-4.2.0-windows-test-20260903`
- 실행: `Archive Center 4.2.0 Windows Test Package/01_start_archive_center_windows.bat`
- package status: `green`, `release_ready=true`, automatic update apply 활성
- 관리 파일: 52개, 크기·SHA-256 불일치 0개
- package source: `43bc20a19698d8e85c910c1b04f33b557eca42c7`, `source_dirty=false`
- ZIP 크기: `17,891,262 bytes`
- ZIP SHA-256: `e1c3f405d786399f6596f08b850705e2b250365b3d12ccf5e8b2856f770cfca2`
- `SHA256SUMS-4.2.0.txt` 일치
- source/package `Archive Center.js` SHA-256:
  `796d3f9d53829f57478ea2aa7c8702a1a0748b1201a4f5be1fec4bbd950d9b91`
- packaged `archive-center-go.exe` SHA-256:
  `83e967d09362f2f5d7f71fe710fbeca57fb434d7e51b3d6067fb03731937f296`
- package JavaScript 문법 검사 통과

ZIP은 `.env.full.local`, `.runtime`, `.updates`, DB와 사용자 자료를 포함하지 않는다. 로컬 테스트
폴더에는 기존 4.1 테스트 폴더의 `.env.full.local`, `.runtime`, `.updates`를 별도로 복원했다.
이전 4.1 테스트 빌드 폴더는 제거해 `_test-builds`에는 이 4.2 폴더 하나만 남겼다.

공개 BAT 실행은 정상적으로 4.2 banner와 runtime 점검 단계까지 진입했다. 현재 PC에는 요구되는
ChromaDB 1.5.9 Python runtime이 없어 공식 Python 다운로드를 시도했으나, 이 작업 환경의 외부
네트워크가 차단돼 실제 서비스 기동 전에 중단됐다. 따라서 이 기록은 one-click 진입과 installer
연결의 source/package 증거이며, package-launched `/ready`, 기존 4.1 DB, 실제 MariaDB/ChromaDB
증거는 아니다. 사용자가 네트워크가 가능한 환경에서 BAT를 한 번 실행해야 남은 runtime과 live
gate를 닫을 수 있다.

## 9. 4.2에서 다루지 않은 범위

- 전체 cross-surface 의미 통합: 4.3
- 범용 source-linked bundle: 4.4
- 물건·계획·약속의 영구 lifecycle: 4.5
- 모델/context별 adaptive 점수·K·예산: 4.6
- 인간형 기시감·부분·완전 회상: 5.x
- 기억 Review/Undo·quarantine 관리 UI: 5.8
