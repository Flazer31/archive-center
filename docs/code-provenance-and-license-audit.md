# Archive Center 코드 출처·라이선스 감사

- 감사일: 2026-07-17
- 범위: 활성 `source` 트리, Go 서비스, 배포 스크립트, 표준 배포 실행
  파일과 번들 ChromaDB 런타임
- 제외: 별도 플러그인인 `Risu Recomposer.js`, 사용자 `.env`·DB·원작 자료,
  과거 배포본, 백업과 임시 검사 폴더
- 성격: 공개 배포 준비를 위한 공학적 감사이며 법률 자문이 아니다.

## 결론

확인된 기술적 라이선스 보완은 완료했다.

- 프로젝트 라이선스: Mozilla Public License Version 2.0
- 최상위 `LICENSE`, `NOTICE`, README 라이선스 안내 추가
- 특정 제3자 작품을 사용하던 원작 DB 테스트 픽스처 제거
- `flatbuffers` 25.12.19와 `tokenizers` 0.23.1의 Apache-2.0
  라이선스 포함 및 Windows 패키징 차단 검사 추가
- Windows와 POSIX 패키지에 프로젝트 및 제3자 라이선스 파일 포함

실제 공개 직전에는 비밀정보 검사, 릴리스 파일 인벤토리, 각 OS 패키지
SHA-256 검증을 다시 수행해야 한다.

## 1. 자체 코드 출처

- 현재 Git 이력의 작성자 표시는 모두 `Flazer31`이다.
- 유일한 루트 커밋 `2633b6f`에서 `Archive Center 2.3 source`가 일괄
  반입됐다.
- 활성 소스에서 외부 프로젝트 코드를 복사·포크·개작했다고 표시한 헤더,
  import 또는 출처 주석은 발견되지 않았다.
- `Archive Center.js`에는 npm 의존성이나 외부 JavaScript 라이브러리
  import가 없다.
- Fugu/Sakana는 문서상의 개념적 참고만 확인됐고 코드 복사 흔적은
  발견되지 않았다.
- 관리자의 기존 권리 확인과 이번 라이선스 적용 지시에 따라 프로젝트
  소유 코드를 MPL-2.0으로 공개하는 상태로 정리했다.

현재 Git 이력만으로는 2.3 이전 파일별 작성 과정을 독립적으로 재현할 수
없다. 추후 외부 기여 또는 복사 코드가 발견되면 해당 파일의 원저작자와
라이선스를 별도로 기록해야 한다.

`Risu Recomposer.js`는 Archive Center 구성 요소가 아니므로 공개 저장소와
배포 범위에서 제외하고 별도로 관리한다.

## 2. 프로젝트 라이선스

Archive Center 자체 소스는 최상위 `LICENSE`의 MPL-2.0을 적용한다.
`NOTICE`는 제3자 구성 요소, 사용자 데이터, 프로젝트 이름과 로고에는
별도 권리가 적용된다는 경계를 명시한다.

MPL-2.0은 파일 단위 copyleft다. Archive Center의 MPL 적용 파일을 수정해
배포하면 해당 수정 소스를 MPL 조건으로 제공해야 한다. 독립된 다른 파일이나
프로그램 전체를 MPL로 변경하도록 요구하지는 않는다.

바이너리 릴리스는 동일 버전의 GitHub 소스 태그를 함께 제공해야 한다.

## 3. Go 의존성

표준 Windows 실행 파일에서 확인된 외부 모듈은 다음과 같다.

| 모듈 | 버전 | 라이선스 |
| --- | --- | --- |
| `filippo.io/edwards25519` | v1.1.0 | BSD 3-Clause |
| `github.com/go-ole/go-ole` | v1.2.6 | MIT |
| `github.com/go-sql-driver/mysql` | v1.8.1 | MPL-2.0 |
| `github.com/shirou/gopsutil/v3` | v3.23.12 | BSD 3-Clause |
| `github.com/yusufpapurcu/wmi` | v1.2.3 | MIT |
| `golang.org/x/sys` | v0.40.0 | BSD 3-Clause |

모두 라이선스 파일과 `THIRD_PARTY_NOTICES.md` 기록이 확인됐다. MariaDB
서버는 Archive Center 패키지에 링크되거나 포함되지 않고 공식 배포본을 별도
런타임으로 설치한다.

## 4. ChromaDB·CPython 런타임

- ChromaDB 1.5.9: Apache License 2.0
- CPython: PSF License와 포함된 역사적 라이선스
- 감사한 Windows 런타임: Python distribution 79개
- 기존 패키지 수준 라이선스 파일 확인: 77개

기존 upstream 런타임에서 패키지별 라이선스 파일이 빠졌던 두 항목은 다음과
같이 보완했다.

| 패키지 | 버전 | 처리 |
| --- | --- | --- |
| `flatbuffers` | 25.12.19 | Apache-2.0 원문을 해당 dist-info에 복사 |
| `tokenizers` | 0.23.1 | Apache-2.0 원문을 해당 dist-info에 복사 |

공식 Apache-2.0 원문은 `licenses/Apache-2.0.txt`에 보존한다. Windows
패키지 빌더는 정확한 두 버전의 dist-info 디렉터리를 확인한 뒤 라이선스를
설치하며, 파일이나 버전이 없으면 패키징을 실패시킨다.

## 5. 테스트와 사용자 콘텐츠

원작 DB의 관계·검색·예산 계약 테스트는 유지하되 `HUNTR/X`, `Rumi`,
`Mira`, `Zoey` 등 특정 작품의 고유 설정은 가상의 `Aster Unit`, `Arin`,
`Bera`, `Ciel` 데이터로 대체했다. 이 값은 테스트 전용이며 런타임 원작 DB나
기본 입력에 포함되지 않는다.

사용자가 입력한 원작 자료는 사용자의 로컬 데이터다. `.env`, MariaDB 데이터,
Chroma 컬렉션, 사용자 원문과 비밀정보는 프로젝트 라이선스나 공개 소스
릴리스에 포함하지 않는다.

## 6. 공개 직전 체크리스트

- [x] 코드 출처와 Git 계보 감사
- [x] MPL-2.0 `LICENSE`, `NOTICE`, README 적용
- [x] 특정 작품 기반 테스트 픽스처 제거
- [x] 누락된 Python 패키지 라이선스 보완과 패키징 차단
- [x] Windows·POSIX 패키지에 라이선스 파일 포함
- [ ] `.env`, DB, 키, 사용자 자료와 개인 경로 최종 검사
- [ ] `Risu Recomposer.js`, `AGENTS.md`, 임시·복구 산출물 공개 제외 확인
- [ ] 각 OS 패키지 파일 인벤토리 및 SHA-256 재생성
- [ ] 새 패키지 설치·시작·자동 업데이트 회귀 검사
