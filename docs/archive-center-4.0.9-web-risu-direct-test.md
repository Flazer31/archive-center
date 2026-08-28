# Archive Center 4.0.9 Web Risu 직접 연결 실험

상태: `implemented_unverified`  
작성일: 2026-08-28

## 목적

공식 Web Risu에서 RisuAI의 전역 직접 요청 설정을 바꾸지 않고 Archive
Center의 Go 백엔드 요청만 브라우저가 직접 전송할 수 있는지 확인한다.
이번 버전은 정식 Web Risu 지원 선언이 아니라 실사용 가능성을 판단하기
위한 Windows 테스트 빌드다.

## 공식 RisuAI 확인 기준

- upstream: `kwaroran/RisuAI`
- inspected branch: `main`
- inspected commit: `e565563a288ebe4c65b6099a1645ba477d1c84b4`
- inspected date: 2026-08-28
- Plugin V3 sandbox: `src/ts/plugins/apiV3/factory.ts`
  - iframe CSP의 `connect-src 'none'`
- Plugin V3 network bridge: `src/ts/plugins/apiV3/v3.svelte.ts`
  - `risuFetch`는 deprecated 호환 API이며 `globalFetch`로 전달됨
  - `nativeFetch`는 `fetchNative`로 전달됨
- Web transport owner: `src/ts/globalApi.svelte.ts`
  - `globalFetch`의 요청별 `plainFetchForce`
  - Web의 localhost/127.0.0.1 명시적 거부
  - `fetchNative`의 Web proxy 기본 경로와 Web `local_network` 거부

## 4.0.9 변경

- 설정에 `Web Risu 직접 연결 (실험)` 체크박스를 추가했다.
- 체크한 경우 Archive Center의 일반 JSON Bridge 요청만
  `Risuai.risuFetch(..., { plainFetchForce: true })`로 전달한다.
- RisuAI의 전역 직접 요청 설정과 LLM Provider 요청은 변경하지 않는다.
- 체크하지 않은 경우 기존 `Risuai.nativeFetch` 경로를 그대로 사용한다.
- 숨겨진 `nativeFetch` fallback이나 자동 재시도를 추가하지 않았다.
- 응답은 raw bytes로 받은 뒤 기존 `bridgeFetch`의 JSON 처리 경로로
  되돌린다.

## 실험 제한

- 공식 Web Risu는 localhost/127.0.0.1을 차단하므로 사용할 수 없다.
- 브라우저에서 직접 접근 가능한 HTTPS Bridge URL이 필요하다.
- 우선 검증 대상은 Tailscale Serve의 Tailnet 전용 HTTPS URL이다.
- Canon Pack ZIP 미리보기·설치처럼 binary request body를 사용하는 작업은
  이 실험 경로에서 `web_direct_raw_body_unsupported`로 종료한다.
- RisuAI의 `risuFetch`가 스트리밍 Response를 노출하지 않으므로 실시간
  Turn Workflow HUD event stream은 이 모드에서 시작하지 않는다. 일반
  Bridge 응답에 포함된 최종 상태 표시는 유지된다.
- deprecated RisuAI 호환 API를 사용하므로 upstream에서 제거되면 이 경로는
  다시 검토해야 한다.

## 사용자 테스트 순서

1. 백엔드 서버와 Web Risu를 여는 기기를 같은 Tailnet에 연결한다.
2. 백엔드 서버에서 `127.0.0.1:28080`을 Tailscale Serve HTTPS로 노출한다.
3. Web Risu의 Archive Center 설정에서 HTTPS 주소를 Backend URL에 넣는다.
4. `Web Risu 직접 연결 (실험)`을 켠다.
5. 먼저 연결 테스트에서 `/ready`가 성공하는지 확인한다.
6. 한 턴을 진행하여 prepare, complete-turn, 기억 저장이 완료되는지 확인한다.
7. Chrome/Edge/Firefox별 결과와 HUD의 transport detail을 기록한다.

## 완료 판정

소스 회귀와 패키지 검증만으로 Web Risu 지원을 완료했다고 판정하지 않는다.
공식 `risuai.xyz`에서 실제 HTTPS Bridge URL로 `/ready`, 한 턴 완료, MariaDB
저장까지 확인되기 전에는 `implemented_unverified` 상태를 유지한다.

## 소스 및 테스트 패키지 검증

- `Archive Center.js` Node 구문 검사: 통과
- Web 직접 연결 production `bridgeFetch` 회귀: 통과
  - 실험 모드에서 `risuFetch + plainFetchForce` 사용
  - 실험 모드에서 `nativeFetch` 미사용
  - 옵션을 끄면 기존 `nativeFetch` 사용
  - binary body는 숨겨진 fallback 없이 typed failure 기록
- 원작 DB 검색 설정 전용 저장 버튼 및 기존 설정 저장 경로 회귀: 통과
- Go 전체 회귀 `go test ./... -count=1`: 통과
- Windows package status: `green`
- managed files: 46
- missing / size mismatch / SHA-256 mismatch: 0 / 0 / 0
- source/package `Archive Center.js` SHA-256:
  `bb4e69b5b226373eb5b0d455de4ab0157652337f599342c94e92e08c10ebeb0f`
- test package root:
  `_test-builds/Archive-Center-4.0.9-web-risu-direct-windows-test`
- ZIP:
  `Archive Center 4.0.9 Windows Auto Install Package.zip`
- ZIP size: `12,134,442 bytes`
- ZIP SHA-256:
  `ce0c40920a68901fec0a46a3831ea513645da246667a83a933f578a165ba7f25`
- package contract: target `4.0.9`, `release_ready=true`,
  `automatic_update_apply=true`, `direct_update_supported=true`

위 검증은 소스와 Windows 패키지 무결성 증거다. 공식 Web Risu의 실제
브라우저 연결 성공 증거는 아직 아니다.
