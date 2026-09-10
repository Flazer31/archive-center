# Archive Center 4.3.1

4.3.1은 저장된 과거 기억이 검색·선정 과정에서 너무 일찍 빠지는 문제를 보완합니다.
**전처리·출판사를 끈 기본 기억에도 적용**되며, 이후 기억 개선의 기준 버전으로 삼습니다.

- 현재 입력과 최근 대화에서 찾은 결과를 합칠 때 같은 개수 제한으로 다시 잘라내지 않고, 검색 질문별 관련 근거를 선정까지 전달합니다.
- 공개 범위와 출처가 유효한 기억에서 먼저 사실 후보를 만듭니다. 화면에 표시할 요약의 선정 결과만으로 후보가 제한되던 경로를 수정했습니다.
- `스트레칭`·`스트레칭을`처럼 조사가 붙은 단서와 짧은 현재 입력의 회수 경로를 보완했습니다.
- 평론가의 정리 힌트가 요약 전체의 저장 중요도를 낮춰, 함께 있던 습관·약속까지 감점하던 동작을 수정했습니다.
- 기본 기억의 Top K 입력을 제거하고 Go가 기억 문자 예산에 맞춰 검색 후보 수를 계산합니다. 최종 주입은 기존 문자 예산 안에서 선정합니다.
- 전처리도 같은 후보를 받습니다. 정상 AI 추천과 순서를 유지하고, 추천이 없는 분류에는 Go 기본 선정을 사용합니다.
- 긴 입력의 단어 중복 확인을 개선해 입력 맥락 조립 지연을 줄였습니다.
- **ChromaDB·MariaDB·Go 백엔드 포트를 변경**할 수 있습니다. 포트 입력을 비우고 Enter를 누르면 선택한 서비스의 기본 포트로 돌아갑니다.

## 설치와 업데이트

기존 관리형 설치는 Archive Center UI에서 업데이트를 확인하고 적용합니다.
추가된 포트 설정 파일도 패키지와 함께 설치됩니다. **RisuAI에 등록한 플러그인도
플러그인 업데이트 또는 새 `Archive Center.js` 가져오기로 4.3.1에 맞춰주세요.**
사용자 DB·API 키·개인 설정은 관리 파일 교체 대상에서 제외합니다.

신규 Windows 사용자는 ZIP을 풀고 `01_start_archive_center_windows.bat`를 실행하거나,
PowerShell에서 아래 명령을 실행합니다.

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

Linux / macOS / Termux:

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

설치 명령은 필요한 파일 설치와 백엔드 시작을 이어서 진행합니다.
기존 설치에는 신규 설치 명령 대신 UI 업데이트를 사용합니다.
RisuAI 플러그인 등록과 백엔드 주소 설정은 별도입니다.

배포 파일: Windows x64 설치·업데이트, Linux x64·arm64, macOS Intel·Apple Silicon, Termux arm64.

## 포트 변경

- Windows: `06_change_port_windows.bat`에서 서비스를 선택합니다.
- Termux 기본 설치: `sh ~/.archive-center/start.sh --configure-ports`.
- Linux/macOS: 기존 실행 명령에 `--configure-ports`를 추가합니다.

기본 포트는 ChromaDB 8000, MariaDB 3307, Go 28080입니다.
저장 후 평소 실행기로 재시작하면 로컬 DB 포트와 백엔드 연결 설정이 함께 적용됩니다.
Go 포트를 변경했다면 RisuAI의 백엔드 URL도 맞춰주세요. DB 위치와 데이터는 유지됩니다.

[설치·업데이트 검증](archive-center-4.3.1-install-update-verification.md) ·
[배포 기록](archive-center-4.3.1-release-verification.md) ·
[기억 검증 범위](archive-center-4.3.1-memory-revalidation.md)

기억이 전달되는 경로의 개선이며, 모든 세션의 회상이나 본문 AI의 활용을 보장한다는 의미는 아닙니다.
