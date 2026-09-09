# Archive Center 4.3.0 배포 검증

검사일: 2026-09-09. **[4.3.0 정식 릴리스](https://github.com/Flazer31/archive-center/releases/tag/v4.3.0) 공개 완료**.
공개 시각은 `2026-09-09T08:54:13Z`이며 GitHub latest와 main을 4.3으로 반영했다.
바이너리 소스는 `51d901bda8173c2d668f64362213375f7eecc028` / `v4.3.0`이다.
배포 후 검증을 기록한 문서 커밋은 바이너리 소스 커밋과 구분한다.
처음 main 갱신이 자동 승인 검토에서 차단됐으나, 사용자의 명시적 추가 승인 후 반영했다.

## 소스·제공자 검사

- Go 1.26.6 전체 테스트·vet, JavaScript 구문 검사 통과.
- [최종 소스 CI](https://github.com/Flazer31/archive-center/actions/runs/34328928187):
  verify, Windows 신규 설치, Ubuntu 신규 설치, macOS 신규 설치 모두 통과.
  verify는 실제 ChromaDB 1.5.9 통합과 취약점 검사도 포함한다.
- 저장소 스키마에 마지막으로 영향을 준 준비 커밋의
  [MariaDB migration CI](https://github.com/Flazer31/archive-center/actions/runs/34327297089) 통과.
  이후 Go 제공자 추가 커밋은 스키마를 바꾸지 않았다.
- OpenCode Zen·Go / OpenRouter 51개 조합(17 경로 × 3개 목적)의 실제 요청 생성·응답 정규화 검사.
  외부 HTTP만 fixture로 대체하며, 실제 구독·유료 API 호출 증거는 아니다.
- Go의 동일 세션 목적별 호출·직접 프록시·5개 담당의 1/2차 호출에서 세션 헤더 유지,
  다른 세션 구분, 사용자 지정 헤더 보존, 비활성 Flex 설정 제외를 확인했다.
- 비-Claude Messages 모델에 Claude 전용 구조화 출력 필드를 자동 부착하지 않는다.
- JS 변경량: 첫 정식 준비 커밋 +26/-15행(기존 test.22~23 포함),
  Go 추가 커밋 +17/-8행. Go 추가분은 제공자 UI와 기존 세션 관찰값의 프록시 전달이다.

## 패키지와 이전 버전 업데이트

공식 builder로 깨끗한 위 소스에서 7개 ZIP을 만들었다.
Windows x64 Auto Install / Update, Linux x64·arm64, macOS Intel·Apple Silicon,
Termux arm64 및 `SHA256SUMS-4.3.0.txt`를 정식 릴리스에 공개했다.
GitHub의 파일별 SHA-256과 로컬 파일을 8개 모두 대조했다.

- 관리 파일 manifest의 SHA-256, 현재 JS와의 일치, 013까지의 migration,
  Noto 폰트 라이선스, 바이너리 CPU, POSIX 실행 권한 확인.
- Windows manifest source_commit은 위 소스와 일치하고 source_dirty=false다.
  POSIX manifest는 commit 필드가 없는 기존 형식이며, 동일 builder 실행과 파일 일치로 확인했다.
- 개인 .env, 데이터, 런타임, update 상태는 배포 ZIP에 포함하지 않는다.
- 공개 4.1·4.2 설치 폴더와 새 OS별 ZIP을 생산 `PreflightCandidate`로 검사: 14개 조합 통과.
- 공개 4.1·4.2 Windows updater 실행 파일 각각으로 새 ZIP 적용→복구→재적용→확정 통과.
  테스트용 개인 설정·MariaDB 디렉터리·Chroma 디렉터리·전처리 설정 sentinel 4종 보존.
- 위 Windows 검사는 실제 등록된 `POST /update/apply` 생산 경로와 이전 updater를 쓴다.
  외부 GitHub 응답·다운로드는 새 ZIP 바이트 fixture다.
  이 fixture 검사와 실제 공개 GitHub latest 다운로드 검사는 구분하며, 후자는 아래에 별도로 기록했다.
- 업데이트 생산 코드와 Windows 실행기는 공개 4.2에서 변경되지 않았다.

## Windows 신규 런타임과 관리형 실행

사용자 데이터와 분리한 임시 폴더에 공식 MariaDB 12.3.2 ZIP을 다운로드·SHA 검증·설치했다.
ChromaDB 1.5.9도 별도 venv에 설치했다. 서명된 기존 Python 3.10.11을 bootstrap으로
사용했으며 Python 자체 신규 다운로드 증거는 아니다.

실제 `01_start_archive_center_windows.bat` 진입도 실행했다.
기본 포트에 사용자 test.22가 실행 중이어서 임시 DB와의 경로 불일치로 진행을 멈췄다.
사용자 백엔드를 종료하거나 데이터에 쓰지 않고,
동일 관리 실행기를 별도 포트(28183·33183·8183)로 실행해 신규 DB 초기화와 준비를 검증했다.

- `/version`: 4.3.0.
- `/ready`: ready/store_ready/vector_ready=true, full_local, bundled.
- MariaDB 관리 계정·임시 데이터 경로와 migration 적용 정상.
- 실행 검증 종료 후 해당 임시 실행기만 종료하고 전용 포트 3개가 닫힌 것을 확인했다.
- 사용자의 원래 28080 백엔드는 test.22 그대로 유지했다.
- 초기 샌드박스 Python 실행 제한은 일반 권한 실행 검사와 구분했다.

## 실제 공개 GitHub 업데이트

사용자 데이터와 분리한 Windows 환경에서 공개 4.2와 4.1 패키지를 각각 실행했다.
각 실행은 실제 MariaDB 12.3.2와 ChromaDB 1.5.9, 테스트 전용 포트
28183·33183·8183을 사용했다. 아래 검사는 외부 GitHub 응답을 대체하지 않았다.

1. 실제 이전 백엔드의 버전을 확인하고, UI가 사용하는 `POST /update/apply`에 요청했다.
   공개 GitHub latest의 `Archive.Center.4.3.0.Windows.Update.Package.zip` 다운로드,
   `accepted`, 종료 코드 75, 관리 실행기 적용과 재시작이 이어졌다.
2. 4.2→4.3과 4.1→4.3 모두 `/version=4.3.0`, `/ready`의 backend/store/vector 정상,
   업데이트 확정 완료를 확인했다. 개인 `.env.full.local`의 SHA-256도 각각 유지됐다.
3. 4.1 검사에서는 **업데이트 전에** MariaDB 원문 2행·기억 1행과 중요도,
   ChromaDB 문서·embedding 1개를 저장했다. 업데이트 후 내용과 개수·metadata·embedding이
   모두 일치했고 같은 벡터 질의로 같은 문서를 다시 찾았다.
4. 4.2의 첫 검사는 설정과 기존 임시 DB 준비 상태를 확인한 범위다.
   원문·기억 보존에 대한 실제 레코드 전후 대조는 별도의 4.1 검사에서 수행했다.
   이를 4.2에서도 레코드 대조를 한 증거로 확대하지 않는다.
5. 검사 종료 후 임시 실행기만 종료했다. 사용자 본래 test.22 백엔드는 유지했다.
6. 공개 main의 JS blob `cff2400c990a221097ad85dc858948812564f8fe`는
   릴리스 태그의 JS blob과 일치한다. ZIP의 JS도 패키지 검사에서 같은 소스와 대조했다.

이는 실제 공개 업데이트·DB 보존 검증이며, RisuAI 화면에서 직접 버튼을 누른
증거와는 구분한다. 4.1 이전 버전의 실제 공개 업데이트 실행은 이번 검사에 포함하지 않았다.

## 아직 남은 확인

- 실제 RisuAI에 4.3 JS를 가져온 뒤의 제공자 호출·화면·최종 출력 검증.
- OpenCode Go의 RP 사용에 대한 서비스 수용 여부 및 실제 구독 계정 호출.
  공식 Go 안내는 코딩 에이전트 트래픽을 대상으로 하며 AC는 자체 User-Agent를 보낸다.
- 모든 OS·CPU 실기기의 전체 런타임 신규 다운로드 및 장시간 운용.
  OS별 CI 설치 검사는 외부 다운로드·시작 경계에 fixture를 쓰며,
  이를 모든 실기기의 완전한 설치 증거로 확대하지 않는다.

백엔드 패키지 업데이트와 RisuAI에 설치된 JS 교체는 별개다.
RisuAI 플러그인 업데이트 또는 새 Archive Center.js 가져오기도 필요하다.

정식 공개 후 검증 기록 갱신 단계의 JavaScript 변경: +0/-0행.
