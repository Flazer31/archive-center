# Archive Center v4.0.8 — 백그라운드 후처리 중 Go 메모리 급증 및 다음 턴 중첩 시 서버 메모리 고갈

## 요약

Archive Center v4.0.8에서 채팅 저장 및 memory/vector 후처리 과정 중 `archive-center-go`의 메모리 사용량이 수 GiB 단위로 급증하는 현상을 확인했습니다.

최초에는 v3.9.11 → v4.0.8 업데이트 과정에서 `Failed to fetch`가 발생했던 이력이 있어 업데이트 실패 또는 혼합 설치 상태를 의심했습니다.

그러나 기존 Archive Center 설치 및 데이터를 완전히 격리한 뒤 **공식 Fresh Install로 v4.0.8을 새로 설치하고 MariaDB/ChromaDB를 빈 상태에서 테스트했음에도 동일 현상이 재현**되었습니다.

특히 HUD에 `저장 완료`가 표시된 이후에도 상당한 background processing이 계속되고 있었으며, 이 상태에서 사용자가 다음 채팅을 전송할 경우 메모리 사용량이 더욱 증가하여 다음 상태까지 도달했습니다.

```text
archive-center-go RSS      ≈ 9.0 GiB
archive-center-go RssAnon  ≈ 9.0 GiB
archive-center-go VmSwap   ≈ 5.6 GiB

AC cgroup RAM              ≈ 9.5 GiB
AC cgroup swap             ≈ 6.0 GiB

memory.high                7063회
OOM                        0
```

테스트 서버는 약 12GiB RAM 환경이며, 과거 동일 증상으로 VM 전체가 두 차례 응답 불능 상태에 빠진 이력이 있습니다.

**심각도: High**

저메모리 서버에서는 Archive Center뿐 아니라 SSH/Tailscale 등 서버 전체 서비스까지 응답 불능 상태가 될 수 있습니다.

---

## 1. 테스트 환경

```text
Archive Center: v4.0.8
OS: Ubuntu 24.04
Architecture: ARM64
Server: Oracle Cloud VM.Standard.A1.Flex
OCPU: 2
RAM: 12GB 설정 / Linux 표시 약 11GiB
Swap: 8GiB

Runtime Profile: full_local
Vector Mode: local_native

Storage:
- MariaDB
- ChromaDB local

Go backend:
/opt/archive-center/releases/v4.0.8/bin/archive-center-go
```

Archive Center는 systemd로 관리하고 있습니다.

서버 전체 장애를 방지하기 위해 최종 테스트 시 다음 cgroup 제한을 사용했습니다.

```ini
MemoryHigh=9728M
MemoryMax=10G
OOMPolicy=kill
```

따라서 이후 관측된 5~9GiB 메모리 증가는 기존의 `MemoryHigh=7G` 제한 때문에 발생한 현상이 아닙니다.

---

## 2. 과거 업데이트 이력

기존 정상 사용 버전은 v3.9.11이었습니다.

플러그인 UI의 내장 업데이트 기능으로 v4.0.8 업데이트를 시도했으며 당시:

```text
Failed to fetch
```

오류가 발생했습니다.

오류 이후 UI 및 backend 표시 버전은 4.0.8이었지만 실제 실행 경로는:

```text
/opt/archive-center/releases/v3.9.11/bin/archive-center-go
```

였고 backend 로그에서는:

```text
Version=4.0.8
```

로 표시되는 혼합 상태가 확인되었습니다.

이 때문에 최초에는 업데이트 실패를 원인으로 의심했습니다.

그러나 후술하는 Fresh Install에서도 동일한 메모리 문제가 재현되었으므로 **업데이트 실패 또는 기존 설치 오염만으로 본 현상을 설명할 수 없습니다.**

---

## 3. 완전 Fresh Install 검증

기존 설치는 삭제하지 않고 완전히 격리했습니다.

```text
/opt/archive-center
→ /opt/archive-center.preclean-20260830-063537
```

이후 공식 설치 방식:

```bash
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

으로 신규 설치했습니다.

Fresh Install 후에는 정상적으로:

```text
/opt/archive-center/current
→ /opt/archive-center/releases/v4.0.8
```

상태였습니다.

MariaDB 역시 신규 생성되었으며 초기 상태는:

```text
memory_vector_outbox       0
memory_reprocessing_jobs   0
source_discovery_jobs      0
```

이었습니다.

따라서 기존 MariaDB/ChromaDB/queue 데이터는 테스트에 포함되지 않았습니다.

Fresh Install 직후 Archive Center 전체 메모리는 약 **180~300MiB**였습니다.

---

## 4. 93턴 기존 채팅 Cold Start 결과

기존 Archive Center 데이터는 복원하지 않고, Risu 측에 남아 있는 약 93턴의 기존 채팅을 Cold Start했습니다.

Cold Start 종료 후:

```text
MemoryCurrent ≈ 641 MiB
MemoryPeak    ≈ 4.26 GiB
Swap          0
memory.high   0
OOM           0
Restart       0
```

이었습니다.

즉 Cold Start 자체는 일시적으로 약 4.3GiB까지 메모리를 사용했으나, 완료 후 약 640MiB까지 정상적으로 감소했습니다.

따라서 단순 Cold Start만으로 영구적인 고메모리 상태가 발생하는 것은 아니었습니다.

---

## 5. 실제 채팅 처리 후 메모리 급증

Cold Start 이후 실제 채팅을 처리하면서 다시 메모리 급증이 발생했습니다.

한 시점에서:

```text
AC MemoryCurrent ≈ 6.77 GiB
AC MemoryPeak    ≈ 7.0 GiB

archive-center-go RSS ≈ 6.3 GiB
MariaDB RSS           ≈ 367 MiB
ChromaDB RSS           ≈ 164 MiB
```

로 확인되었습니다.

메모리 사용량 대부분이 MariaDB 또는 ChromaDB가 아닌 **`archive-center-go` 자체**였습니다.

---

## 6. 메모리 종류 확인

고메모리 상태에서 `/proc/<pid>/status`와 `smaps_rollup`을 확인했습니다.

대표 측정값:

```text
VmRSS:         5,334,832 kB
RssAnon:       5,318,636 kB
RssFile:          16,196 kB
VmSwap:                0

Private_Dirty:  5,318,636 kB
Anonymous:      5,318,636 kB
```

즉 약 5GiB가 거의 전부 `private anonymous memory`였습니다.

따라서 단순 파일 cache, ChromaDB mmap 또는 MariaDB buffer pool 문제로 보기는 어렵습니다.

---

## 7. Go GC 추적 결과

다음 설정으로 재현했습니다.

```ini
Environment="GODEBUG=gctrace=1,scavtrace=1"
```

문제 작업이 시작되기 전에는 GC 후 live heap이 약 10~15MB 수준이었습니다.

그러나 특정 backend processing이 시작된 이후 GC 후에도 살아남는 heap이 연속적으로 증가했습니다.

```text
15 MB
→ 62 MB
→ 87 MB
→ 121 MB
→ 170 MB
→ 238 MB
→ 343 MB
→ 492 MB
→ 717 MB
→ 1,028 MB
→ 1,564 MB
→ 2,260 MB
→ 2,951 MB
→ 3,246 MB
```

대표 GC:

```text
5238 -> 5365 -> 3246 MB
5903 MB goal
```

즉 단순히 GC가 실행되지 않아 dead object가 쌓인 상태는 아니며, **GC 이후에도 실제 reachable/live heap이 3GiB 이상 존재**했습니다.

이후 시간이 지나면서:

```text
5365 -> 5365 -> 1849 MB
1850 -> 1850 -> 1849 MB
```

로 live heap이 약 1.85GiB까지 감소했고 scavenger가 수 GiB 규모의 메모리를 단계적으로 OS에 반환했습니다.

따라서 현재 증상은 단순한 영구 memory leak보다는:

> 특정 후처리 과정에서 매우 큰 live working set이 생성되고 상당 시간 유지되며, 이후 GC/scavenger에 의해 천천히 감소하는 현상

에 더 가깝습니다.

다만 단일 채팅 처리에서 live heap이 3GiB 이상, 프로세스 RSS가 5~9GiB까지 상승하는 규모는 매우 큽니다.

---

## 8. Vector Outbox 대량 생성

Fresh Install 환경에서 `memory_vector_outbox`가 대량 생성되는 것도 확인했습니다.

한 시점의 상태:

```text
completed       3,073
pending         9,652
stale_rejected    154

총 약 12,879건
서로 다른 document 약 12,725개
```

`document_json` payload 합계는 약 575MiB, 테이블 데이터 크기는 약 661.5MiB였습니다.

이전 측정에서는 pending 약 9,014건이 거의 전부 동일한 `chat_session_id`에서 생성되었으며:

```text
attempts=0
embedding_ready=1
```

이었습니다.

따라서 재시도 실패가 반복되어 queue가 폭증한 것이 아니라, **아직 worker가 처리하지 않은 정상 생성 작업**으로 보였습니다.

---

## 9. Pending queue 자체가 고메모리의 직접 원인은 아님

pending 약 9,000건이 존재하는 상태에서 Archive Center를 restart했습니다.

재시작 직후:

```text
MemoryCurrent ≈ 300 MiB
pending        ≈ 9,007
```

였고 durable worker는 restart 후에도 기존 queue를 정상적으로 이어서 처리했습니다.

즉 DB에 pending 9천 건이 존재하는 것 자체만으로 Go backend가 5~7GiB 메모리를 사용하지는 않습니다.

고메모리 상태는 **특정 새로운 채팅/파생 작업을 처리하는 동안 생성되는 working set과 더 직접적으로 연관된 것으로 보입니다.**

---

## 10. HUD의 `저장 완료` 이후에도 backend 작업 지속

소스 조사 및 실제 동작 관찰 결과, HUD의 `저장 완료`는 현재 턴의 동기 저장 파이프라인 완료를 의미하며 전체 background processing 완료를 의미하지 않는 것으로 보입니다.

그 이후에도 별도 worker가 다음 작업을 수행할 수 있습니다.

- memory reprocessing
- embedding 생성
- vector outbox 처리
- ChromaDB indexing
- 실패 작업 재처리

실제로 UI에 `저장 완료`가 표시된 이후에도 `memory_vector_outbox` 전체 row가 계속 증가했습니다.

예:

```text
이전:
completed       2734
pending         9007

이후:
completed       3062
pending         9663
stale_rejected   154
```

즉 UI 완료 이후에도 약 1,100건 이상의 새로운 vector 작업이 추가 생성되었습니다.

또한 `저장 완료` 상태에서 Archive Center daemon을 restart하자 Risu plugin에서 연결 종료 경고가 발생했습니다.

따라서 사용자 관점의 완료 상태와 backend가 실제로 idle 상태가 되는 시점 사이에 상당한 차이가 있습니다.

---

## 11. 가장 심각한 재현 — `저장 완료` 직후 다음 채팅 전송

문제 확인을 위해 이전 턴의 HUD에 `저장 완료`가 표시된 직후 다음 채팅을 전송했습니다.

그 결과 메모리가 다음 수준까지 상승했습니다.

### Go 프로세스

```text
VmRSS:   9,431,892 kB
RssAnon: 9,415,836 kB
VmSwap:  5,910,268 kB
Threads: 9
```

즉 RSS 약 9.0GiB, RssAnon 약 9.0GiB, VmSwap 약 5.6GiB였습니다.

### Archive Center cgroup

```text
MemoryCurrent = 10,200,145,920
MemoryPeak    = 10,201,280,512
```

약 9.5GiB이며 설정된 `MemoryHigh=9.5GiB`에 사실상 정확히 도달했습니다.

### cgroup memory events

```text
high           7063
max               0
oom               0
oom_kill          0
oom_group_kill    0
```

### Archive Center swap

```text
6,433,906,688 bytes
≈ 5.99 GiB
```

즉 OOM은 발생하지 않았지만 **물리 RAM 약 9.5GiB + swap 약 6GiB**를 Archive Center가 사용하며 심한 reclaim/throttling 상태에 진입했습니다.

`MemoryHigh`가 9.5GiB이므로 실제 제한이 없었다면 추가 증가 가능성도 배제할 수 없습니다.

---

## 12. 다음 턴 전송 시 queue도 추가 증가

위 테스트 전:

```text
completed       3062
pending         9663
stale_rejected   154
총             12879
```

테스트 후:

```text
completed       3214
pending        10108
stale_rejected   375
총             13697
```

으로 변했습니다.

즉 짧은 시간 동안 총 vector 작업이 818건 추가됐습니다.

기존 worker가 queue를 소비하고 있는 동안에도 새로운 작업 생성량이 더 많아 pending은 오히려 증가했습니다.

이 결과는:

> 이전 턴의 background processing이 아직 끝나지 않은 상태에서 다음 턴의 처리 작업이 추가되고, 두 작업이 동시에 메모리 및 queue 부하를 증가시키는 상황

과 일치합니다.

단, 실제 goroutine 단위의 병렬 실행 여부는 아직 소스 레벨에서 확인하지 못했습니다.

---

## 13. Memory Reprocessing 상태

추가 조사 시 `memory_reprocessing_jobs`에서 다음 상태가 확인되었습니다.

```text
pending: 39
- critic_config_missing

permanent: 16
- CRITIC_INPUT_SNAPSHOT_INVALID
- critic_input_snapshot_missing

completed: 1
```

이 항목들이 본 메모리 문제의 직접 원인이라는 증거는 아직 없습니다.

다만 클린 설치 및 Cold Start 이후 발생한 상태이므로 정상적인 lifecycle인지 확인이 필요합니다.

---

## 14. 현재 판단

### 확실하게 확인된 사항

1. Fresh Install한 순수 v4.0.8에서도 현상이 재현됩니다.
2. 기존 v3.9.11 → v4.0.8 업데이트 실패 잔재만의 문제는 아닙니다.
3. 메모리 대부분은 `archive-center-go`의 private anonymous memory입니다.
4. 단순 file cache 또는 MariaDB/ChromaDB 메모리 문제는 아닙니다.
5. GC는 정상적으로 실행되고 있습니다.
6. 특정 처리 중 GC 후에도 실제 live heap이 3GiB 이상까지 증가합니다.
7. 이후 일부 heap은 해제되지만 상당 시간 높은 메모리 상태가 유지됩니다.
8. `저장 완료` 이후에도 background memory/vector 작업이 계속 생성·처리됩니다.
9. pending queue가 DB에 많이 존재하는 것 자체만으로는 고메모리가 발생하지 않습니다.
10. 기존 후처리가 남은 상태에서 다음 턴을 전송하면 메모리 사용량이 9.5GiB RAM + 약 6GiB swap 수준까지 증가할 수 있습니다.
11. 약 12GiB RAM 서버에서는 서버 전체 장애를 유발할 수 있습니다.

---

## 15. 잠정적으로 의심되는 영역

### A. 매우 큰 transient working set

memory/direct evidence/vector 파생 과정에서 수백 MiB의 JSON 또는 대량 document를 Go object로 materialize하면서 수 GiB 규모의 working set이 발생할 가능성.

### B. 대형 객체의 장시간 retention

처리 일부가 종료된 후에도 해당 객체나 cache가 reference를 유지해 GC 이후 live heap으로 남아 있을 가능성.

### C. background 작업의 backpressure 부족

이전 턴의 heavy processing이 완료되기 전에 다음 턴의 heavy processing을 시작할 수 있어 working set이 중첩될 가능성.

### D. 과도한 vector outbox 생성

약 93턴 규모의 채팅에서 1만 건 이상의 vector document가 생성되는 것이 의도된 규모인지 확인 필요.

### E. UI 완료 상태와 실제 backend 상태 불일치

사용자가 `저장 완료`를 전체 처리 완료로 인식하고 다음 턴을 전송할 수 있으나 backend에서는 아직 수 GiB 규모의 작업이 진행 중일 수 있음.

---

## 16. 기대 동작

백그라운드 vector/memory 후처리 자체가 의도된 구조라면 다음 중 하나 이상의 보호가 있을 것으로 기대합니다.

- heavy derivation 동시 실행 수 제한
- 이전 heavy job 완료 전 다음 heavy job enqueue
- queue/backlog 기반 backpressure
- 전체 작업을 소규모 batch 단위로 materialize
- 서버 메모리 상황에 따른 worker concurrency 제한
- `저장 완료`와 `백그라운드 후처리 중` 상태 UI 분리
- optional `GOMEMLIMIT` 또는 application-level memory limit 지원

---

## 17. 제작자 확인 요청

1. Cold Start 또는 최초 채팅 처리 시 기존 채팅 전체를 자동 백필/재색인하는 코드 경로가 존재하는지.
2. 약 90턴의 채팅에서 `memory_vector_outbox`가 1만 건 이상 생성되는 것이 정상적인 예상치인지.
3. `memory_vector_outbox.document_json` 전체 또는 대량 row를 한 번에 읽어 메모리에 materialize하는 경로가 있는지.
4. Go live heap이 3GiB 이상까지 증가하는 주요 allocation 지점.
5. GC 후 약 1.8GiB 이상이 일정 시간 live 상태로 유지되는 객체/cache.
6. 이전 턴의 background derivation이 진행 중일 때 다음 턴 derivation이 동시에 실행될 수 있는지.
7. background worker의 concurrency 및 batch size.
8. queue backlog 또는 시스템 RAM에 따른 backpressure가 존재하는지.
9. `critic_config_missing` job을 pending 상태로 계속 보존하는 것이 의도된 동작인지.
10. `critic_input_snapshot_missing` / `CRITIC_INPUT_SNAPSHOT_INVALID`의 정상적인 정리 lifecycle.
11. HUD의 `저장 완료`와 전체 background processing 상태를 별도 표시할 필요가 있는지.
12. production build에서 pprof 또는 기타 heap profiling을 활성화할 수 있는 방법이 있는지.
13. `GOMEMLIMIT` 또는 Archive Center 자체 memory limit 설정 지원 가능 여부.

---

## 결론

본 현상은 기존 업데이트 실패나 오래된 DB의 잔재만으로 발생하는 문제가 아니며, **Archive Center v4.0.8 Fresh Install + 신규 MariaDB/ChromaDB 환경에서도 재현됩니다.**

현재 가장 중요한 현상은 다음입니다.

> 특정 memory/vector 후처리 과정에서 Go backend의 live heap이 수 GiB까지 증가하고, 사용자 UI에 `저장 완료`가 표시된 이후에도 해당 후처리가 지속됩니다.

그리고 해당 상태에서 사용자가 정상적으로 다음 메시지를 전송하면:

> 이전 후처리 + 새로운 처리의 메모리 요구가 중첩되는 것으로 보이며, 12GiB 서버에서 Archive Center가 약 9.5GiB RAM과 약 6GiB swap을 사용하여 서버 전체를 위험한 상태로 만들 수 있습니다.

따라서 단순 memory leak 여부뿐 아니라 **대량 vector 작업 생성 경로, working-set 크기, job overlap 및 backpressure 설계**를 함께 확인할 필요가 있어 보입니다.

### Issue 제목 제안

**`v4.0.8: 저장 완료 이후 background memory/vector 처리 중 Go 메모리 급증 및 다음 턴에서 9GiB+ 사용`**

영문:

**`v4.0.8: Multi-GB Go heap growth during background memory/vector processing, amplified by subsequent turns`**
