# Archive Center 코드 출처·라이선스 감사

- 감사 기준일: 2026-07-17
- 대상: 활성 `source` 트리의 Archive Center 본체, Go 서비스, 배포 스크립트,
  표준 Windows 배포 실행 파일 및 번들 ChromaDB 런타임
- 제외: 별도 플러그인 프로젝트인 `Risu Recomposer.js`, 과거 `_dist-*`, 백업,
  임시 검사 폴더
- 성격: 공개 배포 준비를 위한 공학적 감사이며 법률 자문이 아니다.

## 결론

현재 상태는 **조건부 보류**다. 알려진 외부 코드와 런타임의 라이선스
종류는 식별됐고, 표준 Go 실행 파일에서 GPL 코드가 링크된 흔적은 확인되지
않았다. MariaDB도 Archive Center 패키지에서 분리되어 공식 배포본을 별도로
설치하는 구조다.

다만 다음 네 항목을 완료하기 전에는 Archive Center 전체에 오픈소스
라이선스를 부여하거나 공개 소스 릴리스를 완결된 것으로 선언하면 안 된다.

1. 2.3 이전 코드에 대한 권리 확인
2. 프로젝트 최상위 `LICENSE` 확정
3. 제3자 원작명을 사용한 Go 테스트 픽스처의 중립화
4. 번들 Python 패키지 2개의 누락 라이선스 파일 보완

## 1. 자체 코드 출처

### 확인된 사실

- 현재 Git 이력의 작성자 표시는 모두 `Flazer31`이다.
- 저장소의 유일한 루트 커밋은 `2633b6f`이며 메시지는
  `Archive Center 2.3 source`다.
- `Archive Center.js`, `go-service/go.mod`, Go 서비스 본체와 마이그레이션은
  모두 이 루트 커밋에서 처음 나타난다.
- 활성 소스에서 외부 프로젝트 코드를 복사·포크·개작했다고 명시한 주석,
  헤더 또는 import는 발견되지 않았다.
- `Archive Center.js`에는 npm 의존성이나 외부 JavaScript 라이브러리 import가
  없으며, Python 보조 도구는 표준 라이브러리와 로컬 모듈만 사용한다.
- 저장소에 추적되는 사전 빌드 EXE, DLL, ZIP, 폰트 또는 이미지 벤더 파일은
  발견되지 않았다.

### 증명되지 않은 부분

현재 이력은 2.3 소스 전체를 한 번에 반입한 이력이다. 따라서 Git만으로는
2.3 이전 각 파일의 최초 작성자, 외부 기여자, 이전 저장소의 라이선스를
독립적으로 증명할 수 없다. 현 이력으로 증명 가능한 범위는 **2.3 반입 이후의
관리와 변경**까지다.

공개 라이선스 적용 전에 관리자는 다음 중 하나를 남겨야 한다.

- 2.3 반입 코드 전부를 본인이 작성했거나 재라이선스할 권리를 보유한다는
  서면 확인
- 또는 2.3 이전 저장소·백업 이력을 제시하고 작성자 및 라이선스를 재감사한
  기록

논문이나 Fugu/Sakana 오케스트레이터의 아이디어를 참고했다는 사실만으로
코드 복제가 되지는 않는다. 그러나 표현 코드, 프롬프트 문구, 표, 그림을
실제로 옮겼다면 별도 출처와 해당 라이선스가 필요하다. 이번 검색에서는
Fugu 코드의 import나 복사 표시는 발견되지 않았고, 문서상의 개념적 참고와
논문 링크만 확인됐다.

`Risu Recomposer.js`는 Archive Center의 구성 요소가 아닌 별도 플러그인이다.
Archive Center 공개 저장소와 배포물에서 제외하고 별도 출처·라이선스로
관리해야 한다.

## 2. Go 의존성

현재 표준 Windows 배포 실행 파일 세 개가 사용하는 외부 모듈은 다음과 같다.

| 모듈 | 버전 | 라이선스 |
| --- | --- | --- |
| `filippo.io/edwards25519` | v1.1.0 | BSD 3-Clause |
| `github.com/go-ole/go-ole` | v1.2.6 | MIT |
| `github.com/go-sql-driver/mysql` | v1.8.1 | MPL-2.0 |
| `github.com/shirou/gopsutil/v3` | v3.23.12 | BSD 3-Clause |
| `github.com/yusufpapurcu/wmi` | v1.2.3 | MIT |
| `golang.org/x/sys` | v0.40.0 | BSD 3-Clause |

여섯 모듈 모두 감사에 사용한 Go 모듈 캐시에 상위 라이선스 파일이 있었고
`THIRD_PARTY_NOTICES.md`에도 기록되어 있다. 제외된 레거시 마이그레이션 도구의
의존성 10개도 같은 고지 표에 포함되어 있다. `go-sqlmock`은 테스트 전용이다.

기존 고지의 제목은 모든 표 항목이 모든 실행 파일에 컴파일된 것처럼 읽힐 수
있어, 이번 감사에서 소스·릴리스 도구 전체 인벤토리와 표준 실행 파일의 실제
부분집합을 구분하도록 수정했다.

## 3. 번들 런타임

### MariaDB

MariaDB Community Server는 GPLv2로 배포된다. 현재 Archive Center 표준 패키지는
서버 바이너리를 포함하지 않고, 공식 MariaDB 배포본을 별도 런타임으로 내려받아
검증한 뒤 설치한다. Archive Center는 MPL-2.0인 Go MySQL 드라이버를 통해 별도
프로세스와 통신한다.

따라서 Archive Center ZIP이 MariaDB 서버 자체를 재배포하는 구조에서 발생하던
고지·소스 제공 부담은 분리됐다. 공식 MariaDB 런타임 내부의 `COPYING`,
`THIRDPARTY`, `CREDITS`는 제거하지 않아야 한다.

### ChromaDB·CPython

- ChromaDB 1.5.9: Apache License 2.0
- CPython: PSF License와 포함된 역사적 라이선스 묶음
- 감사한 Windows 런타임: Python distribution 79개
- 패키지 수준 라이선스 파일이 확인된 distribution: 77개

다음 두 distribution은 설치 메타데이터상 Apache License 2.0이지만 해당
`*.dist-info` 안에 패키지별 라이선스 파일이 없었다.

| 패키지 | 버전 | 조치 |
| --- | --- | --- |
| `flatbuffers` | 25.12.19 | 해당 버전의 upstream Apache 2.0 라이선스 추가 |
| `tokenizers` | 0.23.1 | 해당 버전의 upstream Apache 2.0 라이선스 추가 |

ChromaDB 자체 라이선스와 CPython `LICENSE.txt`는 런타임에 존재했다. 다음 공개
패키지 생성기는 위 두 라이선스도 자동 수집하고, 누락 시 빌드를 실패시켜야 한다.

## 4. 콘텐츠·테스트 데이터 출처

다음 Go 테스트 파일에는 `HUNTR/X`, `Rumi`, `Mira` 등 실제 제3자 작품의 고유명과
설정 문구가 픽스처로 포함되어 있다.

- `go-service/internal/store/mariadb_reference_library_test.go`
- `go-service/internal/httpapi/group_reference_coverage_apply_test.go`
- `go-service/internal/httpapi/group_reference_coverage_index_test.go`
- `go-service/internal/httpapi/group_reference_library_test.go`
- `go-service/internal/httpapi/group_reference_recall_test.go`
- `go-service/internal/httpapi/group_reference_relations_test.go`

이는 런타임 원작 DB가 아니라 테스트 데이터지만, 공개 소스에는 제3자 작품의
고정 설정을 그대로 넣을 이유가 없다. 동작 의미를 유지하는 중립적인 가상 작품,
인물, 집단명으로 교체하고 사실관계 자체를 검증하는 테스트가 아니라 관계·검색
계약을 검증하도록 바꿔야 한다.

사용자가 직접 입력한 원작 자료는 사용자의 로컬 DB 데이터이며 프로젝트
라이선스의 대상이 아니다. `.env`, MariaDB 데이터, Chroma 컬렉션, 사용자 입력
원문은 공개 저장소와 배포용 소스 아카이브에 포함하지 않아야 한다.

## 5. 프로젝트 라이선스 상태

현재 `NOTICE`는 프로젝트 전체에 오픈소스 권한을 부여하지 않는다고 명시하며,
최상위 `LICENSE` 파일도 없다. 따라서 지금 공개된 코드가 있더라도 제3자가
복제·수정·재배포할 수 있는 오픈소스 프로젝트로 간주할 수 없다.

2.3 이전 권리 확인이 끝난 뒤 프로젝트 라이선스를 선택해야 한다. 현재 구조에는
파일 단위 copyleft로 수정 공개를 요구하면서 다른 구성 요소와의 결합을 비교적
허용하는 MPL-2.0이 실용적인 후보지만, 이는 소유권 확인 후 관리자가 확정해야
한다. 선택 후에는 최상위 `LICENSE`, `NOTICE`, README의 라이선스 절을 함께
갱신해야 한다.

## 6. 공개 전 필수 체크리스트

- [ ] 2.3 이전 코드 권리 확인서 또는 역사 감사 기록 작성
- [ ] 프로젝트 `LICENSE` 확정 및 추가
- [ ] 제3자 작품 기반 Go 테스트 픽스처 중립화
- [ ] `flatbuffers`·`tokenizers` 라이선스 파일을 번들에 포함
- [ ] `.env`, DB, 사용자 자료, 개인 경로와 키가 Git·릴리스에 없는지 재검사
- [ ] `Risu Recomposer.js`, `AGENTS.md`, 임시·복구·감사 산출물을 공개 범위에서 제외
- [ ] 각 OS 패키지의 실제 포함 파일을 기준으로 제3자 고지를 재생성
- [ ] 패키지 SHA-256 및 라이선스 인벤토리를 릴리스와 함께 게시

위 항목이 완료되기 전의 올바른 표현은 "오픈소스 공개 완료"가 아니라
"오픈소스 전환 준비 중"이다.
