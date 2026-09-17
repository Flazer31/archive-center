# PocketRisu + Tailscale HTTPS Termux 실행 설명서

기준일: 2026-08-09

이 문서는 Android Termux에서 PocketRisu와 Tailscale HTTPS 연결 프로그램을
각각 별도 탭으로 실행하는 방법을 고정한다. 내부 구현을 교체하더라도 아래
사용자 경로와 실행 명령은 변경하지 않는다.

## 고정 실행 계약

| 구분 | 고정 경로 | 고정 실행 명령 |
|---|---|---|
| PocketRisu | `~/PocketRisu` | `pnpm run runserver` |
| Tailscale HTTPS 연결 | `~/pocketrisu-tsnet` | `./pocketrisu-tsnet` |

다음 이름은 사용자 실행 경로로 새로 만들거나 안내하지 않는다.

- `~/ts-proxy`
- 다른 이름의 Tailscale 실행 폴더
- 다른 이름의 최종 실행 파일
- 새 Tailscale 장치명 또는 새 HTTPS 주소

Android 대응 라이브러리나 프록시 구현을 교체해야 한다면
`~/pocketrisu-tsnet/pocketrisu-tsnet`의 내부 구현만 교체한다. 기존 Tailscale
장치명, 인증 상태 저장 위치, HTTPS 주소를 그대로 승계해야 한다.

## 전체 구조

두 Termux 탭을 계속 실행 상태로 유지한다.

```text
Termux 탭 1
└─ ~/PocketRisu
   └─ pnpm run runserver

Termux 탭 2
└─ ~/pocketrisu-tsnet
   └─ ./pocketrisu-tsnet
```

PocketRisu 프로세스만 실행 중이면 로컬 서버만 살아 있는 상태다.
`pocketrisu-tsnet` 프로세스도 함께 실행되어야 Tailscale HTTPS 접속 경로가
유지된다.

## 실행 설명서 1: PocketRisu

첫 번째 Termux 탭을 열고 다음 명령을 실행한다.

```bash
termux-wake-lock
cd ~/PocketRisu
pnpm run runserver
```

정상 상태에서는 서버 시작 메시지가 출력된 뒤 명령 프롬프트로 돌아오지
않는다. 이 탭을 닫거나 `Ctrl+C`를 누르면 PocketRisu가 종료된다.

서버가 실제로 실행 중인지 확인할 때는 다른 Termux 탭에서 다음 명령을
사용한다.

```bash
pgrep -af 'server.cjs'
```

다음과 같이 `server/node/server.cjs` 프로세스가 표시되면 PocketRisu 프로세스는
실행 중이다.

```text
node ... server/node/server.cjs
```

로컬 HTTP 또는 HTTPS 확인 주소는 시작 로그에 출력된 주소를 그대로 사용한다.
인증서 파일의 유무에 따라 PocketRisu가 표시하는 프로토콜이 달라질 수 있으므로
로그를 무시하고 주소를 임의로 바꾸지 않는다.

## 실행 설명서 2: Tailscale HTTPS 연결

두 번째 Termux 탭을 열고 다음 명령을 실행한다.

```bash
cd ~/pocketrisu-tsnet
./pocketrisu-tsnet
```

정상 상태에서는 프로그램이 Tailscale 연결과 HTTPS 전달을 유지하며 명령
프롬프트로 돌아오지 않는다. 이 탭을 닫거나 `Ctrl+C`를 누르면 Tailscale HTTPS
접속 경로가 종료된다.

실행 직후 프롬프트가 다시 표시되면 정상 실행이 아니다. 출력된 오류를 그대로
확인해야 한다.

현재 확인된 구형 바이너리의 오류는 다음과 같다.

```text
tsnet.Up: tsnet: route ip+net: netlinkrib: permission denied
```

이 오류가 나오면 `pocketrisu-tsnet` 프로세스는 실행되지 않은 상태다. Android
호환 구현으로 교체하더라도 최종 폴더와 실행 파일은 반드시 아래 이름을
유지한다.

```text
~/pocketrisu-tsnet/pocketrisu-tsnet
```

새 구현체의 저장소명이나 라이브러리명을 사용자 실행 명령으로 노출하지 않는다.

## 두 프로세스 동시 확인

PocketRisu와 Tailscale HTTPS 연결을 모두 실행한 뒤 다음 명령을 사용한다.

```bash
pgrep -af 'server.cjs|pocketrisu-tsnet'
```

정상 상태에서는 다음 두 종류의 프로세스가 모두 표시되어야 한다.

```text
node ... server/node/server.cjs
.../pocketrisu-tsnet
```

하나만 표시되면 전체 연결이 완성된 상태가 아니다.

## 종료 순서

1. 두 번째 탭의 `pocketrisu-tsnet`을 `Ctrl+C`로 종료한다.
2. 첫 번째 탭의 PocketRisu를 `Ctrl+C`로 종료한다.
3. 더 이상 백그라운드 실행이 필요하지 않을 때만 다음 명령을 실행한다.

```bash
termux-wake-unlock
```

PocketRisu는 정상 종료 과정에서 저장 중인 데이터를 마무리할 수 있으므로 앱을
강제로 종료하기보다 `Ctrl+C`를 사용한다.

## Android가 프로세스를 종료한 경우

Termux 또는 Tailscale 연결이 Android에 의해 종료되었다면 새 이름이나 새
폴더를 만들지 않고 다음 두 명령만 다시 실행한다.

첫 번째 탭:

```bash
cd ~/PocketRisu
pnpm run runserver
```

두 번째 탭:

```bash
cd ~/pocketrisu-tsnet
./pocketrisu-tsnet
```

장시간 사용할 때는 Android 설정에서 Termux의 배터리 최적화를 제외하고
`termux-wake-lock`을 유지한다.

## 변경 금지 항목

다음 항목은 명시적인 사용자 결정 없이는 변경하지 않는다.

- `~/PocketRisu` 폴더명
- `~/pocketrisu-tsnet` 폴더명
- `pnpm run runserver` 실행 명령
- `./pocketrisu-tsnet` 실행 명령
- 기존 Tailscale 장치명
- 기존 `.ts.net` HTTPS 주소
- 기존 Tailscale 인증 상태

내부 결함 수정은 위 실행 계약을 보존하는 방식으로만 진행한다.
