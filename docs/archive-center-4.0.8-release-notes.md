# Archive Center 4.0.8

4.0.8은 4.0.2 이후 실제 사용 중 제보된 저장·재색인, 삭제·리롤,
콜드 스타트, 출판사·평론가 Provider와 기억 의미 문제를 기존 구조 안에서
정리한 안정화 업데이트입니다.

## 주요 수정

### 저장과 재색인

- 문자열 `bookVersion` 때문에 로어북 snapshot이 HTTP 400으로 거절되던
  요청을 숫자·숫자 문자열 모두 처리하도록 수정했습니다.
- JSON 저장 전 `[]string`과 재로딩 뒤 `[]any`의 표현 차이 때문에
  `committed_derived_result_hash_mismatch`로 색인이 막히던 문제를 동일한
  canonical JSON 해시 기준으로 수정했습니다.
- ChromaDB readback 뒤 MariaDB의 embedding·model과 outbox 완료 상태가 함께
  수렴하도록 했습니다.
- 같은 문서의 삭제 작업이 수십만 건까지 중복되던 경로를 정리하고, 안전 조건이
  같은 미완료 삭제만 종료하며 제한된 batch로 처리합니다.

### 삭제·리롤·콜드 스타트

- 사용자 입력만 삭제되고 assistant 출력이 남은 턴은 유지합니다. assistant
  출력이 실제로 삭제된 경우에는 canonical rollback을 수행합니다.
- 리롤은 기존 logical turn 교체 경로를 사용하며 이전 revision은
  `superseded`, 새 출력만 `active_final`로 유지합니다.
- rollback 요청은 Vector 정리를 무한히 기다리지 않고 durable queue 등록 뒤
  종료하며, worker가 제한된 양씩 이어서 처리합니다.
- 시험 중 추가돼 정상 콜드 스타트까지 무효화하던 `replacement_pending` 보호
  상태와 관련 판정을 제거했습니다.
- 콜드 스타트 첫 관측을 삭제 신호로 오인해 복원 직후 턴을 다시 rollback하던
  문제를 수정했습니다.
- 저장된 평론가 결과의 `character_name`·`character` 별칭 때문에 개체 정보가
  0건으로 남던 경우, canonical 결과를 다시 쓰지 않고 누락된 파생 투영만
  재개합니다.

### 출판사·평론가와 Provider

- Publisher 모델 입력의 중복 출력 요구, 들여쓰기 JSON과 감사용 metadata를
  줄였습니다. 실제 전달 텍스트, source reference, privacy·authority guard,
  단일 호출과 fail-open 계약은 유지합니다.
- Custom/OpenAI 호환 endpoint의 문자열 content, text block 배열, legacy text와
  명시적인 Responses API 응답을 처리합니다.
- reasoning만 있고 최종 텍스트가 없는 응답은 최종 JSON으로 오인하지 않으며,
  finish reason과 사용량·reasoning token 진단을 보존합니다.
- 출판사·평론가에 NeuralWatt Standard/Flex Provider를 추가했습니다. Flex는
  Go 백엔드가 SSE의 본문·reasoning·usage·finish reason을 조립합니다.
- UI의 출판사·평론가 timeout 설정이 backend 실제 Provider 호출 제한에 그대로
  적용됩니다.
- HTTP 429와 상류 Provider timeout은 JSON 형식 실패와 구분합니다. 다른 모델로
  숨겨서 전환하거나 재시도하지 않습니다.

### 기억 의미와 장기 사용 보완

- belief 기반 주관 기억에도 평론가가 반환한 항목별 중요도와 감정 가중치를
  전달합니다. 점수가 없는 항목만 기존 기본값을 사용합니다.
- 종료 시점이 없는 KG를 `현재 유효`가 아니라 `종료 미기록` 이력으로 표시하고,
  현재 상태 권위와 분리합니다. 오래된 약속·관계는 단순히 오래됐다는 이유로
  삭제하지 않습니다.
- 세계 규칙을 조립 초기에 180자로 잘라 문장 중간이 손상되던 처리를 제거하고,
  완전한 항목을 최종 예산 선택기로 전달합니다.
- Android Firefox를 포함한 브라우저 공통 설정 저장·복원 순서를 정리했습니다.
- 사용자가 직접 고정한 열린 약속은 기존 일반 기억 예산 안에서 전달 후보로
  유지합니다.
- 사용자가 명시한 이름 대응표가 있을 때 given name과 surname을 평론가 identity
  지침에서 보존합니다.

## 새 기본값

새 설치, 설정 초기화 또는 해당 필드가 없는 경우 다음 기본값을 사용합니다.
이미 저장된 사용자 설정은 자동으로 덮어쓰지 않습니다.

- 출판사 timeout: 120초
- 평론가 timeout: 120초
- 출판사 completion: 30,000 tokens
- 평론가 completion: 30,000 tokens
- 일반 기억 예산: 18,000 chars
- 원작 DB 예산: 3,000 chars
- 로어북 보조 참조 예산: 3,000 chars

## 설치와 업데이트

- 3.9.9 이상 정상 관리형 패키지는 Archive Center 설정의 업데이트 기능으로
  4.0.8을 직접 적용할 수 있습니다.
- Windows 기존 설치의 자동 업데이트는 `Windows.Update.Package.zip`을 우선
  사용합니다.
- 신규 사용자는 운영체제에 맞는 `Auto.Install.Package.zip`을 사용합니다.
- 지원 패키지: Windows x64, Linux x64/arm64, macOS Intel/Apple Silicon,
  Termux arm64.

신규 설치 명령:

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

신규 설치 명령은 기존 설치 업데이트용이 아닙니다. 기존 관리형 설치는 UI의
업데이트 기능을 사용하십시오.

## 남아 있는 경계

- 열린 KG끼리의 의미를 추측해 완전한 최신 사실 하나로 자동 통합하는 작업은
  이번 버전의 범위가 아닙니다. 4.0.8은 과거 이력을 현재 사실로 단정하던 표시와
  전달 의미를 먼저 분리합니다.
- 반복 약속의 주기·이행·누락·취소를 자동 판정하는 전면 개선은 후속 기억
  단계의 범위입니다.
- 기존 DB의 주관 기억 기본 점수나 과거 KG를 근거 없이 일괄 재작성하지 않습니다.

## 검증

- JavaScript 구문 검사
- Go 전체 테스트와 `go vet`
- Windows 신규 설치·업데이트 패키지 구조 및 manifest 검사
- Linux x64/arm64, macOS Intel/Apple Silicon, Termux arm64 cross-build와
  관리형 패키지 구조 검사
- ZIP별 SHA-256 기록

