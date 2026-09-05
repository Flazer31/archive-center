# Archive Center 4.2.0 공개 및 업데이트 검증

2026-09-05에 [v4.2.0 정식 릴리스](https://github.com/Flazer31/archive-center/releases/tag/v4.2.0)를
최신 안정판으로 공개했다. 릴리스 태그의 소스는
`4257081c217e57b7e570592fb1090b484255c013`이며, 이후 이 검증 기록을 추가한
문서 커밋은 배포 바이너리의 소스 커밋과 구분한다.

## 공개 파일과 설치 진입점

- Windows x64 Auto Install / Update, Linux x64 / arm64, macOS Intel / Apple Silicon,
  Termux arm64의 ZIP 7개와 `SHA256SUMS-4.2.0.txt`를 공개했다.
- 최종 패키지는 위 커밋의 깨끗한 소스에서 공식 release builder로 생성했다.
  GitHub에 업로드된 파일 8개의 크기와 SHA-256 digest가 로컬 최종 파일과 일치했다.
- 각 ZIP의 파일 manifest, 원본 JS·migration 일치, CPU 형식, POSIX 실행 권한,
  사용자 데이터·키 미포함을 검사했다. 모든 패키지에서 실제 4.1 패키지를 입력으로 한
  production update preflight가 통과했다.
- 신규 설치는 Windows ZIP의 `01_start_archive_center_windows.bat` 또는
  [README의 OS별 한 줄 명령](../README.md#github-fresh-install)을 사용한다.
  기존 설치는 설정의 **업데이트 확인 → 지금 업데이트**를 사용한다.

## 자동 검사

[릴리스 소스 커밋의 GitHub CI](https://github.com/Flazer31/archive-center/actions/runs/33963356406)는
아래 네 작업이 모두 성공했다.

| 작업 | 확인 범위 |
| --- | --- |
| verify | 전체 Go 테스트·vet, JavaScript 구문·회귀, 실제 ChromaDB 1.5.9 API v2 통합, 빌드·취약점 검사 |
| Windows fresh install | 실제 신규 설치 진입점과 기존 설치 보존, 다운로드 검증, 생성된 POSIX 실행기 계약 |
| Ubuntu fresh install | POSIX 신규 설치 진입점 계약과 네이티브 updater 트랜잭션 회귀 |
| macOS fresh install | POSIX 신규 설치 진입점 계약과 네이티브 updater 트랜잭션 회귀 |

설치 계약 검사는 외부 다운로드·시작 경계에 fixture를 사용한다. 각 OS에서 모든
런타임을 새로 다운로드하고 실제 장시간 운용한 증거로 확대하지 않는다.
Windows 최종 패키지의 별도 `core_lite`/vector `off` 시작 검사도 warning/failure 0이었다.

공개 4.1 Windows updater 바이너리로 두 Windows 패키지의 적용 → 복구 → 재적용 →
확정을 실행했다. 관리 파일 복구와 테스트용 설정·runtime sentinel 4종 보존을 확인했다.
이는 아래 실제 DB 업데이트 검증과 별도의 파일 트랜잭션 검사이다.

## 실제 공개 4.1 → 4.2 관리형 업데이트

사용자 데이터와 분리한 Windows 테스트 폴더에서 공개 4.1 패키지를 실행했다.
실제 MariaDB 11.4.10 및 ChromaDB 1.5.9를 사용했고, 테스트 전용 포트는 각각
백엔드 28184, MariaDB 33184, ChromaDB 8184였다. 기존에 설치된 런타임 바이너리를
이용했으므로 런타임 최초 다운로드 검증에 해당하지 않는다.

1. 4.1의 `/version`과 `/ready`를 확인하고 MariaDB에 사용자·AI 원문 2행과 기억 1행,
   ChromaDB에 문서·embedding 1개를 저장했다.
2. 공개 후 UI가 사용하는 것과 같은 `POST /update/apply` 요청을 실제 4.1 백엔드에
   보냈다. 실제 GitHub latest에서 4.2 Windows Update ZIP을 선택·다운로드했고,
   `accepted`, 종료 코드 75, 관리형 실행기의 파일 교체·재시작으로 이어졌다.
3. 새 `/version`은 `4.2.0`, `/ready`의 backend·store·vector 상태는 모두 정상,
   `/update/status`와 update-state는 `committed`였다.
4. 설치 manifest는 4.2.0이고, 설치된 JS·소스 JS·공개 main JS의 SHA-256은 모두
   `4707f9196540af76b17a5cc02791b0bb7e7a4d6295f07d3c1f305f7ab6be0f65`로 일치했다.
5. `.env.full.local`의 SHA-256이 그대로였고, MariaDB의 원문·기억 내용 및 중요도,
   ChromaDB의 문서·embedding·개수가 유지됐으며 동일 벡터 질의도 성공했다.
6. 검증을 마친 테스트 실행기만 종료하고 전용 포트 3개가 닫힌 것을 확인했다.

최종 보존 검증 시각은 `2026-09-05T11:43:32Z`이다. 이는 실제 DB를 사용하는
격리 Windows 환경의 공개 업데이트 검증이다. 실제 RisuAI 화면에서 버튼을 누르거나
플러그인을 재가져온 검증, 사용자 본래 데이터 전체에 대한 검증은 아니다.

## 남아 있는 별도 검증

- Linux arm64·Termux 및 모든 macOS/Windows 기기의 전체 의존성 신규 설치·복구 실기기 확인.
- RisuAI에 설치된 4.2 JS의 실제 로딩, provider payload와 최종 출력, 장기 RP 기억 품질.

백엔드 패키지를 업데이트해도 RisuAI에 이미 설치된 코드는 자동 교체되지 않는다.
RisuAI 플러그인 업데이트 또는 새 `Archive Center.js` 가져오기를 함께 수행해야 한다.
4.3의 선택형 다중 AI 전처리 및 향후 출력 후처리는 이번 릴리스 기능에 포함하지 않는다.
