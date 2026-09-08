# Archive Center 4.3.0-test.1 — Windows 테스트 빌드

2026-09-06 사용자 요청으로 만든 기본 작동 확인용 빌드입니다. 기존 4.2 기능에
#5/#6/#4/#10/#7 피드백 대응, RisuAI·PocketRisu 분기/재분기 호환성,
Gemini 3.8 Flash medium 선택·전송 수정을 포함합니다.
다중 AI 전처리와 상속 기억 전체의 독립 복사·편집 체크박스는 포함하지 않습니다.

## 실행 방법

1. 기존 Archive Center 실행 창에서 `Ctrl+C` → `Y`로 백엔드를 종료합니다.
2. 테스트 ZIP을 풀고 `01_start_archive_center_windows.bat`를 실행합니다.
   이미 압축 해제해 둔 같은 이름의 폴더에서도 바로 실행할 수 있습니다.
3. RisuAI/PocketRisu에 그 폴더의 `Archive Center.js`를 임포트해 기존 AC를
   교체합니다. AC 플러그인 두 개를 동시에 활성화하지 않습니다.
4. 설정 화면의 `4.3.0-test.1` 표시와 백엔드 연결을 확인합니다.
   같은 PC의 기본 Bridge URL은 `http://127.0.0.1:28080`입니다.
   원격 접속을 사용하던 경우에는 기존 서버 주소를 유지합니다.

기본 실행은 `%LOCALAPPDATA%\ArchiveCenter\data`의 기존 기억 데이터를 사용합니다.
별도 데이터 경로를 지정하던 설치는 기존 `ARCHIVE_CENTER_DATA_DIR`를 유지해야 합니다.
이 테스트 빌드를 새 폴더에 푸는 것만으로 시험용 DB가 분리되지는 않습니다.
지워도 되는 시험용 채팅/세션에서 삭제와 분기 동작을 확인하세요.

필요한 MariaDB·Python·ChromaDB 런타임이 없으면 기존 관리형 실행기가 설치합니다.
사용자 DB·API 키·설정은 ZIP에 넣지 않았습니다. 첫 실행 시 포함된 migrations를
적용하며, `013_precise_memory_text_fields.sql`의 긴 기억 필드 확장을 포함합니다.

## 사용자가 확인할 순서

| 순서 | 확인할 동작 |
|---|---|
| 1 | 새 시험용 채팅에서 일반 대화 후 원문·기억 저장 및 다음 턴 기억 조회 |
| 2 | 같은 사용자 메시지의 reroll, assistant 삭제 후 같은 사용자 메시지 편집·재생성 |
| 3 | 동일한 문장을 새 사용자 메시지로 보냈을 때 새 턴으로 기록되는지 |
| 4 | 분기 후 부모 연결, 상속 구간에서 재분기 후 더 이른 분기점에 맞는 기억 범위 |
| 5 | HUD 반복 표시·닫기, 실패 복구 뒤 오래된 카드가 반복되는지 |
| 6 | Gemini 3.8 Flash를 쓴다면 medium 선택·저장·재열기와 실제 호출 |

## 산출물

위치는 활성 소스의 `_test-builds/4.3.0-test.1/`입니다.

- `Archive Center 4.3.0-test.1 Windows Auto Install Package.zip`
  — 17,973,389바이트, 약 18 MB.
- 같은 이름의 압축 해제 폴더 — 플러그인, Go 백엔드·업데이터·스키마 도구,
  실행기, migrations, prompts, 시작 안내 포함.
- `SHA256SUMS-4.3.0-test.1.txt` — ZIP 확인용 체크섬.

ZIP SHA-256:
`CC8AF8B1A21BFB23EE3FC93E222CDE7A6CC1D1F743914650D8135A163419B7B9`

플러그인 SHA-256:
`63D387E52CEE235BCAD171815FD52D9833707104FD0F4AAEA7D2F8783F83AD43`

소스 기준은 `work/4.2.0`의 HEAD
`3e5e0dc99b3daafce5040418436b671f9a84ac78`와 미커밋 피드백 수정분입니다.
manifest에도 `source_dirty=true`가 기록되어 있습니다. 공식 4.3 릴리스나 GitHub
게시물은 생성하지 않았으며, 이번 산출물은 Windows x64용입니다.

## 빌드와 검사 결과

생산 빌더 `ops/build-full-package.ps1`에 `-PackageKind managed`,
`-PackageVersion 4.3.0-test.1`, `-Zip`을 지정했습니다.
복사된 실행 코드·manifest·체크섬을 직접 편집하지 않고 활성 소스에서 재생성했습니다.

- 활성 플러그인과 패키지 플러그인의 해시 일치. 관리 대상 53개 파일의 해시·크기와
  ZIP 외부 체크섬 확인. 사용자 DB·런타임 데이터·설정·로그 없음.
- 활성/패키지 JS 구문 검사, 패키지 PowerShell 구문 검사 통과.
- 메타데이터 수정 후 JS 회귀 패키지 전체 통과(13.207초).
  피드백 실행 로직에 대한 직전 전체 Go·실제 DB 시험은
  [사전 테스트 기록](archive-center-4.3-pretest-20260906.md)을 따른다.
- 최종 ZIP을 별도 검증 폴더에 풀어 실제 `scripts/start-full-windows.ps1` 실행.
  `ARCHIVE_CENTER_DATA_DIR`는 전용 시험 경로, 포트는 백엔드 28087·MariaDB 33187·
  Chroma 8187을 사용했다. 이미 설치된 MariaDB 11.4.10·Chroma 1.5.9 런타임을 사용했고
  새 DB 초기화, 계정 준비, migrations 및 호환성 경로의 적용을 확인했다.
- `/version`은 `4.3.0-test.1`, `/health`는 `ok`, `/ready`는 `ready=true`,
  `store_ready=true`, `vector_ready=true`, `reference_vector_ready=true`,
  `degraded=false`였다. `/version`의 commit/Go version은 기존 구현대로 `unknown`이며,
  소스 식별은 패키지 manifest, 파일 해시, 실제 빌드 기록으로 확인했다.
- 최종 패키지의 `smoke-live.ps1`: 원문 저장 `save_ok=true`, prepare/search 응답,
  rollback 및 시험 세션 삭제 성공. 실제 AI 설정을 제공하지 않아
  `critic_config_missing`, `critic_triggered=false`, 검색 결과 0개였다.
  prepare 응답은 기존 진단 경로의 `source=shadow`였다. 이를 실제 Host의 기억
  주입·Critic 추출·답변 품질 검증으로 계산하지 않는다.
- 패키지로 기동한 실제 DB에서 `memory_subtype`, `relationship_key`,
  `reveal_condition` 세 필드가 모두 `longtext`인 것을 조회했다.
- 시험 종료 후 시험 실행기와 그 자식 서비스가 종료됐으며 시험 포트가 닫혔다.
  기존 8000번 Chroma 서비스는 유지됐다. 배포 폴더에는 검증 중 생성한 데이터를 넣지 않았다.

이번 빌드를 준비하면서 동봉 진단 스크립트의 세션 삭제가
`session_delete_requires_manual_action`으로 실패하는 것을 재현했다. 기존 생산
API가 요구하는 `req_source=timeline_manual_delete`와 진단 사유를 요청에 넣도록
스크립트 한 줄만 수정했고, 재생성한 최종 ZIP의 전체 진단 요청이 끝까지 실행됐다.
백엔드의 삭제 정책이나 수락 조건은 바꾸지 않았다.

JS 변경은 버전·빌드 정보 **7줄 추가 / 7줄 삭제**이며 Host 동작 변경은 없다.
이전 피드백까지 포함한 HEAD 대비 JS 누적 차이는 **123줄 추가 / 24줄 삭제**다.
기존 버전 표시 회귀를 새 메타데이터에 맞추고, Claude 캐시 시험에서 관계없는
릴리스 소개 문구의 고정값 검사를 제거했다. Windows 시작 안내에는 테스트 절차와
현재의 안정된 데이터 경로를 반영했다.

실제 RisuAI/PocketRisu에 수정본을 로드한 검증과 실제 공급자 호출은 남아 있다.
따라서 피드백의 전체 상태는 `implemented_unverified`다. 패키지 제작·기동 검증
완료와 4.3 전체 기능 완료는 구분한다. 새 PC의 런타임 다운로드 설치, 공개 자동
업데이트 적용, 다른 OS의 실행도 이번 빌드 검증 범위에는 포함하지 않는다.

로컬 증거는 작업 루트 `.tmp-feedback-43/test-build-1-verification.json`,
`test-build-1-package-final.log`, `test-build-1-js-regression.log`,
`test-build-1-smoke/http-readiness-final.json`,
`test-build-1-smoke/basic-http-smoke-final.log`,
`test-build-1-smoke/schema-fields-final.txt`에 보관했다.
