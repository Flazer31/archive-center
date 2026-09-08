# Archive Center 4.3.0-test.2

2026-09-06 로컬 테스트 빌드. 공개 GitHub 릴리스가 아니다.

기존 피드백 수정에 5개 담당의 선택형 전처리를 더했다.
구현·분량/실패 처리·검증 경계는 [전처리 작업 기록](archive-center-4.3-preprocessing-work-log.md)을 따른다.

## 패키지 및 기존 설치 교체

- 새 ZIP: `_test-builds/4.3.0-test.2/Archive Center 4.3.0-test.2 Windows Auto Install Package.zip`
- ZIP SHA-256: `fee87ee936dfc0d118c4fa33daeaefa9e4ab3b08e32d486cb672742fbe07359c`
- 플러그인 SHA-256: `61EAC61A6833DAEA0AB5B335F03B04E5C832E5689A96BE6887DF2BA56B7DFB0C`
- 백엔드 SHA-256: `83A40BDA7305715FA890529FB210271323290DC6B48A62ACC1A8C9FF72E3BA30`
- 기존 실행 위치인 `_test-builds/4.3.0-test.1/Archive Center 4.3.0-test.1 Windows Auto Install Package`
  폴더에 새 패키지 56개 파일을 덮어썼다. **폴더명은 유지하며 내용은 test.2다.**
- 교체 전·후 패키지 명세의 53개 파일 SHA-256을 확인했다. 기존 `.env.full.local`의
  SHA-256은 `126D305D25479C927FE5E986006729CC382D6DD2CC95F84C859B3F528BF4E73D`로 같았다.
  기존 DB/벡터 데이터 디렉터리를 복사·삭제·교체하지 않았다.
- 사용자가 승인한 기존 백엔드 PID 16004를 종료하고 기존 관리 실행기로 재시작한다.
  제한된 실행 권한에서는 Chroma 환경 검사가 실패해, 승인된 권한으로 같은 실행기를
  다시 시작했다. 상태 판정은 아래 실제 응답을 기준으로 한다.

## 검사

- 전체 `go test ./...` 통과. 이후 변경한 전처리 검색/파싱 시험도 별도로 통과했다.
- JavaScript 회귀 패키지 및 `node --check "Archive Center.js"` 통과.
- 최종 소스에서 HTTP/JavaScript 회귀 패키지 재검사와 `go vet ./...` 통과.
- HTTP 대역 제공자로 1차 병렬, 한 번의 보충, 부분 실패, 빈 추천의 Go 선정, 원문/순서
  보존, 실제 주입문 및 출판사 support packet 전달을 확인했다. 보충 질문을 **새로 임베딩**하는
  생산 경로도 확인했다. 실제 유료 제공자 호출/응답 품질로 해석하지 않는다.
- 이번 플러그인 변경량은 기존 test.1 패키지 대비 **75줄 추가, 7줄 삭제**다.
  용도는 추가 기능 화면, 설정 전달, 결과 표시와 테스트 버전 표시다. 정책은 Go에 있다.

실제 RisuAI에 설치된 플러그인은 파일 교체만으로 재등록되지 않는다. 새 `Archive Center.js`를
다시 불러온 뒤 추가 기능 → 전처리 다중 에이전트에서 설정한다. 기본 OFF이며 화면을 열거나
프롬프트를 편집하는 것만으로 AI를 호출하지 않는다. 실제 제공자·로드된 UI·비밀 범위·최종 표시
확인은 `implemented_unverified`의 남은 실사용 단계다. 다른 OS의 이 빌드는 아직 실행하지 않았다.

## 실제 재시작 확인

2026-09-06 20:35 KST 이후 기존 설치 위치의 새 백엔드 PID 28344에서 다음을 확인했다.

- `/version`: `4.3.0-test.2`
- `/health`: `ok`, `/ready`: `ready=true`
- `/config/memory-preprocessing`: `memory_preprocessing.v1`, 담당 5개, `enabled=false`
- 실제 저장소: 기존 `%LOCALAPPDATA%/ArchiveCenter/data/mariadb`와 `chromadb`,
  `mariadb_authority`, `full_local`, `bundled` 실행.
- 설치된 JS와 활성 소스 JS의 SHA-256 일치.

자동 승인 검토가 `0.0.0.0`으로 지속 실행하는 요청을 LAN 공개 승인 부재를 이유로 거부했다.
제시된 안전한 대안으로 **이번 실행만 `127.0.0.1:28080`**에 바인딩했다. 기존 `.env.full.local`은
수정하지 않았다. LAN 접속은 이번 실행의 검증 범위 밖이며 현재 사용할 수 없다.
관리 실행기는 Codex의 지속 실행 세션에서 동작한다.
