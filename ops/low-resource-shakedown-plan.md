# Low-Resource Shakedown Plan

> Status: **R0/R1 measurement methodology** — MariaDB/Milvus live inclusion is banned until explicit approval.
> This document defines how to measure Go service resource consumption without claiming cutover readiness.

> 2.0.1 update: the current low-resource product direction is profile-based
> runtime splitting, not a single mandatory full local stack. Use
> `docs/2.0.1-lightweight-runtime-profiles.md` as the follow-up contract for
> Windows, Linux, macOS, Termux, NAS/Docker, and remote RisuAI client profiles.
> The older Milvus measurement notes below remain historical methodology
> evidence and should not define the 2.0.1 default runtime.

---

## 1. Purpose

Archive Center 2.0 targets low-resource environments (single workstation, co-running RisuAI + Ollama). Before any R2 performance claim, we need reproducible measurement methodology for:

1. **Go service idle RSS**
2. **Go service startup time**
3. **Go service request latency**
4. **Combined stack idle RSS** (MariaDB + Milvus Lite + Go) — **plan only until approval**

---

## 2. Measurement Environment

| Parameter | Value | Note |
|-----------|-------|------|
| OS | Windows 10/11 | Primary dev target |
| Go version | 1.22+ | `go version` |
| Build flags | `-ldflags="-s -w"` | Stripped binary for realistic size |
| GOCACHE | outside workspace | Already `.gocache` inside `go-service/` |
| Test mode | shadow | `AC_MODE=shadow` only |

Before each measurement run:
```powershell
# Clean build cache
Remove-Item -Recurse -Force "M:\risulongmemory\Archive Center 2.0\go-service\.gocache"
# Build with release-like flags
cd "M:\risulongmemory\Archive Center 2.0\go-service"
go build -ldflags="-s -w" -o archive-center-go.exe ./cmd/archive-center-go/
```

---

## 3. Metric 1 — Go Service Idle RSS

### Definition
Resident Set Size of the Go process **after** it has started and served at least one `GET /health` request, with no concurrent load.

### Method A: External Process Probe (Windows)
```powershell
$proc = Start-Process -FilePath "..\go-service\archive-center-go.exe" `
  -ArgumentList "-bind", "127.0.0.1:28080" `
  -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 3
# Prime the runtime
Invoke-RestMethod -Uri "http://127.0.0.1:28080/health" -Method GET
Start-Sleep -Seconds 2
# Sample RSS (WorkingSet64 in bytes)
$ws = (Get-Process -Id $proc.Id).WorkingSet64
Write-Output ("Idle RSS: {0:N0} bytes ({1:N2} MB)" -f $ws, ($ws/1MB))
Stop-Process -Id $proc.Id -Force
```

### Method B: In-Process Exposure (Go)
Add an internal endpoint or startup log that reports `runtime.ReadMemStats()`:

```go
import "runtime"

var m runtime.MemStats
runtime.ReadMemStats(&m)
// m.Sys is total bytes of memory obtained from the OS
// m.HeapSys is heap bytes obtained from OS
// m.HeapAlloc is bytes allocated and not yet freed
```

This is already available via the `internal/bench` introspection helpers if needed.

### Acceptance Threshold (Provisional)
- **≤ 150 MB** idle RSS for Go service alone (no MariaDB, no Milvus).
- Measured on the reference workstation after a warm `GET /health`.

---

## 4. Metric 2 — Go Service Startup Time

### Definition
Wall-clock time from process start until the first successful `GET /health` response.

### Method
```powershell
$start = Get-Date
$proc = Start-Process -FilePath "..\go-service\archive-center-go.exe" `
  -ArgumentList "-bind", "127.0.0.1:28081" `
  -PassThru -WindowStyle Hidden
# Poll until ready
while ($true) {
    try {
        $resp = Invoke-RestMethod -Uri "http://127.0.0.1:28081/health" -Method GET -TimeoutSec 1
        if ($resp.status -eq "ok") { break }
    } catch { }
    Start-Sleep -Milliseconds 100
}
$elapsed = (Get-Date) - $start
Write-Output ("Startup time: {0:N3} s" -f $elapsed.TotalSeconds)
Stop-Process -Id $proc.Id -Force
```

### Acceptance Threshold (Provisional)
- **≤ 5 seconds** from binary start to first ready response.
- This includes module initialization, router registration, and first listener accept.

---

## 5. Metric 3 — Go Service Request Latency

### Definition
HTTP round-trip latency for safe, read-only routes against the Go shadow service.

### Method
Reuse the existing `cmd/baseline-capture` CLI, but point it at the Go service instead of the 0.8 backend:

```powershell
cd "M:\risulongmemory\Archive Center 2.0\go-service"
# Start Go service in background
Start-Process go -ArgumentList "run","./cmd/archive-center-go" -WindowStyle Hidden
# Wait for listen
Start-Sleep -Seconds 3
# Capture against Go service
go run ./cmd/baseline-capture `
  -base "http://127.0.0.1:28080" `
  -paths "/health","/ready","/version" `
  -n 20 -timeout 2 -json
```

### Routes to Measure (R1 Shadow)
| Route | Expected p95 |
|-------|-------------|
| `GET /health` | ≤ 10 ms |
| `GET /ready` | ≤ 10 ms |
| `GET /version` | ≤ 10 ms |
| `GET /sessions` | ≤ 50 ms (fake store) |
| `POST /search` | ≤ 100 ms (fake vector) |

> All R2 mutating routes are blocked by `shadow_guard` and must not be benchmarked.

### Acceptance Threshold (Provisional)
- **p95 ≤ 100 ms** for read-only shadow routes with fake backends.
- **p95 ≤ 2 s** for `/prepare-turn` and `/complete-turn` once R2 live backends are wired (requires approval).

---

## 6. Metric 4 — Combined Stack Idle RSS

### Scope
MariaDB + Milvus Lite + Go service running simultaneously on the reference workstation.

### Measured Disposable Runtime (R2 Evidence)
The combined stack has now been measured in disposable mode. This does not switch user authority, does not enable product Milvus reads, and does not persist Go as the default runtime.

Evidence: `benchmarks/combined-stack-low-resource-r2-115.json`.

1. Start MariaDB (local instance, default config, `innodb_buffer_pool_size=256M`).
2. Start Milvus Lite (local `.db` file, no cluster mode).
3. Start Go service with `AC_MARIADB_DSN` and `AC_MILVUS_ENDPOINT` set.
4. Wait for all readiness probes to pass.
5. Sample idle RSS of each process via `Get-Process`.
6. Sum: `RSS_go + RSS_mariadb + RSS_milvus`.

### Planned Acceptance Threshold (Provisional)
- **≤ 1.2 GB** combined idle RSS for MariaDB + Milvus Lite + Go.
- **≤ 1.8 GB** total non-Ollama steady-state RAM (including RisuAI host adapter process).

### Latest Result
- Combined stack status: **ok**.
- MariaDB startup: `1234.521 ms`.
- Milvus Lite startup: `7907.236 ms`.
- Go startup: `719.398 ms`.
- Combined idle RSS: `179.715 MB`.
- Component RSS: MariaDB `105.789 MB`, Milvus Lite `3.297 MB`, Go `70.629 MB`.
- Safe route p95 latency: `/health 5.469 ms`, `/ready 1.523 ms`, `/version 6.015 ms`.

---

## 7. Measurement Cadence

| Phase | Trigger | What Is Measured |
|-------|---------|------------------|
| R0 (now) | Every significant internal package change | Go idle RSS, startup, baseline-capture against Go shadow |
| R1 | After config/auth/health route stabilization | Same as R0 + session/read-only latency |
| R2 (approval required) | After MariaDB + Milvus live wiring | Combined stack idle RSS + full route latency |

---

## 8. Blockers & Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Go binary bloat from debug symbols | RSS inflated | Build with `-ldflags="-s -w"` |
| Windows Defender real-time scan skews latency | P95 outliers | Exclude workspace from real-time scan during measurement |
| GOCACHE inside workspace skews packaging size | `.gocache` is excluded by `.gitignore` and scan tool |
| Accidentally measuring against 0.8 backend | Wrong baseline | Use port 28080 for Go, 8000 for 0.8 |
| MariaDB/Milvus not available in R0 | Combined stack unmeasured | Closed by r2-115 disposable managed-provider probe; rerun if provider/runtime layout changes |

---

## 9. Evidence Checklist

- [x] Idle RSS measurement methodology defined (external probe + in-process memstats).
- [x] Startup time measurement methodology defined (poll-until-ready).
- [x] Request latency measurement methodology defined (reuse `baseline-capture` against Go service).
- [x] Provisional acceptance thresholds documented.
- [x] Combined stack (MariaDB + Milvus + Go) measured in disposable managed mode.
- [x] Actual measurement numbers against Go service (recorded in `benchmarks/low-resource-go-only-r2-114.json`: startup `760.044 ms`, idle RSS `20.293 MB`, safe-route p95 <= `19.249 ms`).
- [x] Combined stack measurement (recorded in `benchmarks/combined-stack-low-resource-r2-115.json`: combined RSS `179.715 MB`, safe-route p95 <= `6.015 ms`).

---

*Contract version: R0-2026-05-22*  
*Reference: `benchmarks/baseline-capture-plan.md`, `go-service/cmd/baseline-capture`, `go-service/internal/bench`*
