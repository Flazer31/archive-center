# Archive Center 4.3.0-test.18 — 기억 보정과 HUD 정리

날짜: 2026-09-08. 상태: `implemented_unverified`.
활성 소스의 미커밋 변경까지 포함한 Windows 테스트 패키지다.

## 포함한 변경

- HUD 폭 224px, 준비 시간과 생성·저장 항목의 두 열 박스, 상세 펼치기 유지.
- 정상 완료 HUD는 본체나 X로 닫고 펼치기·접기는 창을 유지한다. 경고·오류는 X 전용이다.
- `priority_score.static.v4`: 원본 Memory의 의미 검색 점수를 전체 요약 선정에 반영하고,
  저장 중요도와 최근성의 이중 감점을 제거한다. 개별 사실의 관련성과 AI 추천 순서는 유지한다.
- 인물별 지식·정체·공개 상태의 연결과 기존 보호 안내 정리 경로를 보정한다.
- 하이파 원문 요약 하나를 기억 하나로 가져오며, 원문 보존·반복 가져오기·이력 조회를 개선한다.

세부 변경과 한계는 [피드백 기록](archive-center-4.3-feedback-work-log.md)과
[전처리·HUD 기록](archive-center-4.3-preprocessing-work-log.md)을 따른다.
이번 포장 단계의 JS 변경은 빌드 식별자 **4줄 추가 / 4줄 삭제**다.
이전 수정이 모두 포함되며 **백엔드도 test.17과 다른 새 실행 파일**이다.

## 패키지

- [Windows ZIP](../_test-builds/4.3.0-test.18/Archive%20Center%204.3.0-test.18%20Windows%20Auto%20Install%20Package.zip)
- [플러그인](../_test-builds/4.3.0-test.18/Archive%20Center%204.3.0-test.18%20Windows%20Auto%20Install%20Package/Archive%20Center.js)
- [시작 파일](../_test-builds/4.3.0-test.18/Archive%20Center%204.3.0-test.18%20Windows%20Auto%20Install%20Package/01_start_archive_center_windows.bat)

ZIP 크기 **18,079,560바이트**. Go **1.26.6**으로 활성 소스의 백엔드·업데이터·스키마
도구를 빌드했다. 관리 파일 **53개**의 크기·디스크/ZIP 해시와 외부 체크섬이 일치한다.
플러그인, 두 프롬프트, SQL 13개가 활성 소스와 일치한다. 시작 스크립트와 실행용
`.env.full.example`은 test.18이다. `.env.source.example`은 원본 소스 예제 사본이며
실행용 설정이 아니다. 사용자 설정·DB를 포장하지 않았다.

| 파일 | SHA-256 |
| --- | --- |
| ZIP | `e35d571ecec795f14b4ffca1c7ffdea8cff92864fbd6d94f7a7191f37cfabbc7` |
| 백엔드 | `9a93088b78df0c98871f678567187fbd6597ffb00c5750c0e269eac97a7ab809` |
| 플러그인 | `0f2a111ff3c7567cc3230bbf2eb67813ca5e1362de77e14aa00416c125e6fc1d` |

## 검증과 적용 범위

전체 Go 검사에서 일반 시험 3,846개와 하위 시험 783개가 통과했고, 이전 test.17 번호를
기대하는 플러그인 버전 검사 하나가 실패했다. 해당 기대값을 test.18로 갱신한 뒤 JS
생산 경로 422개 전체를 재검사했다. 합산 범위는 **35개 패키지, 일반 시험 3,847개 및
하위 시험 783개**다. MariaDB 8개, Chroma 2개, 실제 제공자 1개, POSIX 실행 비트 1개는
환경상 생략했다. 소스와 패키지 플러그인의 JS 구문 검사도 통과했다.

앞선 HUD 격리 Edge 검사에서 좁은 화면·확대·0개/큰 수·펼치기·접기·X·일반 영역
클릭·이전 저장 동시 표시를 확인했다. 기록은 `_diagnostics/20260908-hud-storage-cards/`,
이번 빌드·검사·해시 영수증은 `_diagnostics/20260908-test18-build/`에 있다.
소스 HEAD는 `3e5e0dc99b3daafce5040418436b671f9a84ac78`, dirty 상태를 보존했다.

플러그인과 백엔드를 함께 교체하여 사용한다. 실행은 사용자가 직접 한다.
기존 패키지·사용자 데이터는 덮어쓰지 않았으며, GitHub 게시·실제 RisuAI 설치·백엔드
기동은 수행하지 않았다. 실제 RP 기억 효과와 사용자 환경에서의 최종 작동은 별도 확인 대상이다.

## 2026-09-08 문서 정정

후속 Markdown 점검에서 동봉 `00_README_FIRST_WINDOWS.md`의 첫 안내가 test.1과
“다중 AI 전처리 미포함”으로 남은 것을 확인했다. 이는 안내 문구 오류이며, 이 빌드에는
위에 기록한 전처리 기능과 최신 백엔드가 포함되어 있다. 버전·내용 판단은 이 빌드 기록과
[현재 4.3 현황](archive-center-4.3-status-summary.md)을 따른다.

[소스 시작 안내 템플릿](../ops/full-package/00_README_FIRST_WINDOWS.md)은 빌더가 실제
버전을 채우도록 정정했다. 이미 전달한 ZIP·실행 파일·매니페스트·체크섬은 변경하지 않았다.
따라서 기존 test.18 ZIP의 오래된 첫 문구는 남아 있으며, 위 정정으로 읽는다.
