# 4.3.1 OCI 환경의 본문 요청 준비 중단 제보

기록일: 2026-09-12

상태: **제보 접수 / 1차 소스 비교 완료 / 원인 미확정 / 추가 답변 대기**.
현재 제보자의 추가 답변과 브라우저 실행 자료는 없다. 재현 또는 수정 완료로 분류하지 않는다.

## 제보 내용

- 환경: OCI에서 PocketRisu를 Docker로, Archive Center 백엔드를 systemd로 실행.
- 사용 이력: 4.0부터 4.2.0까지 정상 사용하다가 4.3.1 업데이트 직후 문제 발생.
- 증상: 제보자는 `prepare-turn` 호출과 응답을 확인했지만 메인 모델로 최종 프롬프트가 전달되지 않는다고 설명했다. HUD는 **본문 요청 준비** 단계에서 멈춤.
- 함께 사용하는 플러그인: Yumi 번역기, 프로바이더 매니저. 각 버전과 등록 순서는 미확인.
- 비교 관측: Archive Center를 끈 요청에서는 정상 응답을 받았다고 보고했다.
- 메인 제공자·모델, PocketRisu 버전, 실제 로드된 JS/실행 중인 백엔드 버전, 브라우저 오류, HTTP 응답 원문은 미확인.

위 내용은 사용자에게 전달받은 제보이며 해당 OCI 환경에 접속해 확인한 결과가 아니다.
`prepare-turn` 응답 확인이 서버 로그·HTTP 헤더·브라우저의 완전한 JSON 수신 중 어느 수준인지도 아직 모른다.

## 확인한 소스 근거

비교 기준은 저장소의 정식 태그 `v4.2.0`과 `v4.3.1`이다. 현재 개발 소스만 보고 공개 버전의 동작을 추정하지 않았다.

1. `Archive Center.js`의 `bridgeFetch`, `orchestrateTurnHelpers`, `applyContextInjection`, `injectAuxiliaryBlock`, `attachFinalPayloadParityTrace`, `onMemoryTransportBodyInterceptor`는 해당 두 태그 비교에서 동일했다.
2. `onBeforeRequest`에는 HUD의 준비 완료/본문 시작 시간 관측이 추가됐다. `tryPrepareTurn`에서는 기존 `top_k` 전달이 제거됐다. 기억 회수·전달 자료와 HUD가 달라졌으므로 함수 일부가 같다는 이유로 AC 측 회귀를 배제하지 않는다.
3. 연결 관련 기본값 `bridgeUrl`, `requestTimeoutMs`, `webDirectBridgeEnabled`, `memoryTransportMode`는 동일했다. 이 비교에서 업데이트 후 새로 설정해야 하는 필수 연결 옵션은 확인되지 않았다.
4. `v4.3.1:go-service/internal/httpapi/group_turn_prepare.go`는 최종 주입 계획 조립 전에 `turnWorkflowStagePayload`를 시작하고, 정상 응답 구성 과정에서 해당 단계를 완료한 뒤 `awaitFinal`을 호출한다. 응답에 실리는 HUD는 그 이후의 snapshot이다.
5. 따라서 **본문 요청 준비** 표시는 메인 제공자 호출 완료/진입의 증거가 아니다. 정상 JSON의 HUD가 `awaiting_final_output`인데 화면만 이전 단계인지, 응답 본문 수신/처리가 끝나지 않은 것인지 구분해야 한다.
6. [RisuAI 요청 처리 소스](https://github.com/kwaroran/RisuAI/blob/main/src/ts/process/request/request.ts)는 등록된 `beforeRequest`들을 순서대로 `await`한 후 요청 트리거와 메인 모델 처리를 실행한다. AC의 백엔드 응답 성공만으로 이후 플러그인 완료까지 보장할 수 없다. 이는 확인한 upstream 구조이며 제보자의 PocketRisu 빌드와 완전히 같다고 단정하지 않는다.
7. 로컬 참조본 Yumi v1.4.2에는 별도의 `beforeRequest`와 채팅/번역 기록 처리 경로가 있다. 이 파일이 제보자가 사용한 버전인지는 모른다. 기존 AC의 Yumi 원문 읽기 시험은 전체 플러그인 조합의 정상 동작을 증명하지 않는다.

## 아직 확정하지 않은 부분

- 사용자 설정 누락, OCI/Docker/systemd 구성 오류 또는 Yumi/프로바이더 매니저 충돌 중 하나로 단정하지 않는다.
- AC가 받은 자료의 크기·형태 변화, 브라우저 응답 읽기/주입, 이후 플러그인 처리, HUD 갱신 중 실제 중단 위치가 확인되지 않았다.
- 오류가 발생한 실제 요청을 재현하지 않았다. 이번 제보에 대한 런타임 수정·빌드·배포는 하지 않았다.
- 별도로 만든 4.4.0-test.7 전처리 효율 개선을 이 OCI 제보의 해결 증거로 사용하지 않는다.

## 추가 답변을 받으면 이어갈 확인

1. 브라우저에서 수신한 해당 `/prepare-turn` 응답의 `turn_workflow_hud.status`, `turn_workflow_hud.current_stage.key`와 응답 수신 완료 여부.
2. 중단 직후 Console의 첫 오류 문구, 메인 제공자·모델명.
3. AC 편집 확인에 **해당 요청**의 최종 입력이 있는지, `capturedBeforeRequestReturn` 등 요청 반환 직전 관측이 남았는지. 이전 요청의 표시와 구분하며, 이 관측도 최종 제공자 전송 완료와 구분한다.
4. 필요하면 AC와 프로바이더 매니저를 유지하고 Yumi만 잠시 끈 요청 한 번으로 처리 위치를 좁힌다. 아직 시행 결과는 없으며 Yumi가 원인이라는 전제도 아니다.

추가 자료가 도착하면 같은 제보에 이어 기록한다. 현재는 응답 대기로 보관하며 별도 자동 점검이나 설정 변경을 예약하지 않는다.

문서만 변경했다. JavaScript +0/-0, Go·설정·사용자 DB·실행 중인 서비스 변경 없음.
