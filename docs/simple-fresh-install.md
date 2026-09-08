# Archive Center 설치 안내

2026-09-08 저장소의 공개 배포 기준은 [4.2.0](archive-center-4.2.0-release-verification.md)이다.
아래 명령은 GitHub의 공개 릴리스를 설치한다. 로컬 개발용
[4.3.0-test.18](archive-center-4.3-test-build-18.md)은 해당 테스트 ZIP으로 적용한다.

Archive Center 백엔드는 운영체제에 맞는 명령어 한 줄로 설치합니다. 설치기가
운영체제와 CPU를 확인하고, GitHub Release에서 맞는 패키지를 내려받아 SHA-256을
검증한 뒤 MariaDB·ChromaDB·DB 구조·실행기를 준비하고 바로 시작합니다.

이 문서의 첫 두 명령어는 **처음 설치하는 사용자 전용**입니다. 기존 설치 폴더가
있으면 아무 파일도 덮어쓰지 않고 중단합니다. 3.9.0 사용자는 아래의
`3.9.0에서 4.1.0으로 이전` 절차로 DB를 먼저 보존하십시오.

## 처음 설치하는 사용자

### Windows x64

GitHub Release의 Windows Auto Install Package ZIP을 빈 폴더에 풀어 설치하는
경우에는 `01_start_archive_center_windows.bat`를 실행하면 됩니다. 이 파일이
MariaDB·ChromaDB 준비, DB 구조 적용과 백엔드 시작을 한 번에 담당합니다.

PowerShell을 열고 다음 한 줄을 실행합니다. 관리자 권한은 필요하지 않습니다.

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

설치 위치는 `%LOCALAPPDATA%\ArchiveCenter`입니다. 설치가 끝나면 백엔드 실행
창이 열립니다. 처음 실행할 때 필요한 MariaDB와 Python·ChromaDB 실행 환경을
자동으로 준비하므로 인터넷 연결이 필요합니다.

설치된 `Archive Center.js`가 있는 폴더를 열려면 PowerShell에서 다음 명령을
사용할 수 있습니다.

```powershell
$package = (Get-Content "$env:LOCALAPPDATA\ArchiveCenter\current-package.txt" -Raw).Trim()
explorer.exe $package
```

### Linux·Ubuntu·macOS·Termux

터미널에서 다음 한 줄을 실행합니다.

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

- systemd를 사용하는 Linux·Ubuntu: `/opt/archive-center`에 설치하고
  `archive-center.service`를 등록·시작합니다.
- systemd를 사용하지 않는 Linux와 macOS: `$HOME/.archive-center`에 설치하고
  설치된 실행기를 바로 시작합니다.
- Termux: `$HOME/.archive-center`에 설치합니다. arm64 기기만 지원합니다.
- Linux는 x64·arm64, macOS는 Intel·Apple Silicon을 자동 구분합니다.

설치된 플러그인 파일은 systemd Linux에서는
`/opt/archive-center/current/Archive Center.js`, 그 외 POSIX 환경에서는
`$HOME/.archive-center/current/Archive Center.js`에 있습니다.

## 기존 관리형 설치 업데이트

4.1 관리형 설치에서 공개 4.2로의 업데이트는 설정의 **업데이트 확인 → 지금 업데이트**
경로를 사용한다. 검증된 범위는 [4.2 릴리스 기록](archive-center-4.2.0-release-verification.md)을
따른다. RisuAI에 설치한 `Archive Center.js`도 별도로 같은 버전으로 갱신한다.
한 줄 신규 설치 명령으로 기존 설치를 덮어쓰지 않는다.

## RisuAI·PocketRisu 연결

1. 설치된 패키지의 `Archive Center.js`를 RisuAI 또는 PocketRisu의
   스크립트·플러그인 항목에 불러오고 활성화합니다.
2. 같은 기기에서 사용한다면 백엔드 주소는 `http://127.0.0.1:28080`입니다.
3. 다른 기기에서 연결한다면 Archive Center 기기의 내부망·Tailscale·HTTPS
   주소를 사용합니다.
4. 메인 모델, 출판사 LLM, 평론가와 임베딩 설정을 저장합니다.
5. 연결·준비 상태·DB 통계·모델 시험을 확인한 뒤 새 채팅에서 한 턴을
   진행합니다.
6. HUD에서 응답과 저장 결과를 확인합니다. **이전 턴** 확정 모드에서는 다음 새
   입력에서 직전 응답의 평론가·저장을 확인하며, 현재 생성 카드에는 저장 목록이 없습니다.

## 3.9.0에서 4.1.0으로 이전

다음은 당시 3.9 → 4.1 이전 절차의 보존 기록이다. 4.1 이후 관리형 업데이트나
로컬 test.18 적용을 이 절차와 동일한 것으로 취급하지 않는다.

3.9.0은 4.1.0 자동 업데이트의 출발점이 아닙니다. 기존 프로그램 파일은 새
설치에 섞지 않고, MariaDB·ChromaDB·로컬 설정만 백업한 뒤 4.1.0을 신규
설치합니다. 이전이 확인될 때까지 기존 프로그램 폴더와 DB 원본을 삭제하지
마십시오.

### 공통 백업 원칙

1. Archive Center 실행기와 백엔드, MariaDB, ChromaDB를 모두 종료합니다.
2. 기존 설치 폴더의 `.runtime`과 `.env.full.local`을 다른 폴더에 복사합니다.
3. 복사본에 `mariadb-data/mysql`이 존재하는지 확인합니다.
4. ChromaDB를 사용했다면 `chromadb-data`도 복사되었는지 확인합니다.
5. 백업 확인이 끝난 뒤에만 4.1.0 설치를 시작합니다.

MariaDB가 실행 중인 상태에서 데이터 폴더를 복사하면 정상적인 백업이 되지
않을 수 있습니다.

### Windows 기존 사용자

1. 모든 Archive Center 관련 창과 프로세스를 종료합니다.
2. 기존 패키지의 `.runtime`과 `.env.full.local`을 별도 폴더에 복사합니다.
3. GitHub Release의 4.1.0 Windows 패키지 ZIP을 기존 3.9.0과 다른 빈 폴더에
   풉니다.
4. 백업한 `.runtime`과 `.env.full.local`을 새 패키지 최상위 폴더에 넣습니다.
5. `01_start_archive_center_windows.bat`를 실행합니다.
6. 실행기가 기존 데이터를 `%LOCALAPPDATA%\ArchiveCenter\data`로 검증 복사하고
   필요한 DB 구조를 적용합니다. 원본 `.runtime`은 그대로 남습니다.
7. 기존 세션과 기억을 확인한 뒤 RisuAI의 `Archive Center.js`를 4.1.0 파일로
   교체합니다.

MariaDB 후보가 둘 이상이면 실행기가 임의로 고르지 않고 중단합니다. 이 경우
사용할 `.runtime`만 새 패키지에 두고 다시 실행하십시오.

### macOS·Termux·systemd를 사용하지 않는 Linux

기존 데이터 위치를 찾습니다. 구 압축 패키지는 보통 패키지 안의 `.runtime`,
구 Termux 기본 설치는 `$HOME/.archive-center-2.0`입니다.

```sh
find "$HOME" -type d \( -name mariadb-data -o -name chromadb-data \) 2>/dev/null
```

모든 서버를 종료한 뒤 첫 줄의 경로만 실제 기존 데이터 경로로 바꿔 실행합니다.

```sh
OLD_DATA="/기존/패키지/.runtime"
BACKUP_DATA="$HOME/archive-center-preserved-data"
mkdir -p "$BACKUP_DATA"
cp -a "$OLD_DATA/mariadb-data" "$BACKUP_DATA/"
if [ -d "$OLD_DATA/chromadb-data" ]; then cp -a "$OLD_DATA/chromadb-data" "$BACKUP_DATA/"; fi
test -d "$BACKUP_DATA/mariadb-data/mysql" || { echo "MariaDB 백업 실패"; exit 1; }
du -sh "$BACKUP_DATA/mariadb-data"
```

Termux 구 기본 설치라면 `OLD_DATA="$HOME/.archive-center-2.0"`을 사용합니다.
새 설치 위치가 이미 있으면 삭제하지 말고 이름을 바꿔 보관한 뒤, 보존 폴더를
고정 데이터 위치로 지정하여 설치합니다.

```sh
if [ -e "$HOME/.archive-center" ]; then mv "$HOME/.archive-center" "$HOME/.archive-center-before-4.1.0"; fi
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | ARCHIVE_CENTER_DATA_DIR="$BACKUP_DATA" sh
```

### systemd를 사용하는 Linux·Ubuntu

서비스와 DB를 모두 종료하고 위 POSIX 절차와 같은 방법으로 데이터를
`$HOME/archive-center-preserved-data`에 복사한 뒤 확인합니다. 그다음 다음 순서로
신규 설치가 만든 빈 데이터 폴더를 보관하고 기존 DB 복사본을 넣습니다.

```sh
BACKUP_DATA="$HOME/archive-center-preserved-data"
test -d "$BACKUP_DATA/mariadb-data/mysql" || { echo "MariaDB 백업 실패"; exit 1; }
sudo systemctl stop archive-center.service 2>/dev/null || true
test ! -e /opt/archive-center-before-4.1.0 || { echo "기존 보관 폴더가 이미 있습니다"; exit 1; }
if [ -e /opt/archive-center ]; then sudo mv /opt/archive-center /opt/archive-center-before-4.1.0; fi
if [ -e "$HOME/.archive-center" ]; then mv "$HOME/.archive-center" "$HOME/.archive-center-before-4.1.0"; fi
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
sudo systemctl stop archive-center.service
sudo mv /opt/archive-center/data /opt/archive-center/data-new-empty
sudo mkdir -p /opt/archive-center/data
sudo cp -a "$BACKUP_DATA/." /opt/archive-center/data/
sudo chown -R "$(id -un):$(id -gn)" /opt/archive-center/data
sudo systemctl start archive-center.service
```

## 이전 완료 확인

1. `/ready`가 정상이고 `/version`이 4.1.0인지 확인합니다.
2. 기존 세션과 기존 턴의 요약·직접 근거·인물·관계가 조회되는지 확인합니다.
3. 새 턴을 한 번 진행해 원문·최종 출력·평론가 결과·기억·벡터가 저장되는지
   확인합니다.
4. 확인이 끝난 뒤 RisuAI 또는 PocketRisu의 `Archive Center.js`를 4.1.0으로
   교체합니다.
5. 구 프로그램 폴더와 DB 백업은 며칠간 더 보관한 뒤 정리하는 것을 권장합니다.

## 3.9.9 이후 업데이트

3.9.9 이후 버전은 Archive Center 설정의 `업데이트 확인`에서 새 버전을 확인한
뒤 `지금 업데이트`를 누릅니다. 업데이터가 OS·CPU에 맞는 패키지를 선택하고,
SHA-256 검증, 전체 관리 파일 교체, 누적 migration, 새 백엔드 준비·버전 확인을
자동으로 수행합니다. 사용자 DB·API 키·로컬 설정은 관리 파일 교체 대상이
아닙니다.

현재 GitHub Release에서 내려받은 3.9.9·3.9.10·3.9.11·4.0.0·4.0.1·4.0.2·4.0.3·4.0.4·4.0.5·4.0.6·4.0.7·4.0.8·4.0.9는 같은 버튼으로 4.1.0에
업데이트하는 대상입니다. 3.9.9 최초 배포 직후의 교체 전 패키지는 지원 대상이
아니며, 이 경우에는 위 신규 설치·DB 보존 절차를 사용합니다.

신규 설치 명령은 업데이트 명령이 아닙니다. 이미 설치된 경로에 다시 실행하면
덮어쓰지 않고 중단하는 것이 정상입니다.
