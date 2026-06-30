# Proxy Security Contract ? Archive Center 2.0 R0

> Status: **R0 shadow design contract**  
> Live proxy implementation is **explicitly banned** in R0/R1. This document defines the security envelope that any future live proxy must satisfy.

---

## 1. 0.8 Reference Behavior Analysis

### 1.1 Route
- `POST /proxy/plugin-main` in `backend/routes/proxy_plugin_main.py`
- Guarded by `check_proxy_access(request)` in `backend/services/route_guard.py`

### 1.2 Request Shape (`ProxyPluginMainRequest`)
| Field | Type | Default | Notes |
|-------|------|---------|-------|
| messages | list | ? | Chat messages array |
| model | str | "" | Resolved from `PROJECT_MAIN_MODEL` if empty |
| max_tokens | int | 1024 | |
| temperature | float | 0.7 | |
| provider | str | null | Resolved from `PROJECT_MAIN_PROVIDER`; falls back to `"openai"` |
| endpoint | str | "" | Resolved from `PROJECT_MAIN_ENDPOINT` if empty |
| api_key | str | "" | Resolved from `PROJECT_MAIN_API_KEY` if empty |
| timeout_ms | int | null | Resolved from `MAIN_TIMEOUT` (seconds) ? 1000; default 60000 |
| reasoning_preset | str | null | `"auto"` ? family detection |
| reasoning_effort | str | null | |
| reasoning_budget_tokens | int | null | |
| budget_tokens | int | null | |
| max_completion_tokens | int | null | |

### 1.3 0.8 Guard Layers
1. **Network origin** ? `_is_local_request()` blocks non-local unless operator token matches.
2. **Rate limit** ? Fixed-window 120 RPM (`_PROXY_DEFAULT_RPM`).
3. **Endpoint validation** ? `_proxy_validate_endpoint()` rejects private/local/metadata endpoints.
4. **Timeout clamp** ? `max(1, min(300000, timeout_ms))`.
5. **API key sanitization** ? `_proxy_sanitize_error_detail()` scrubs keys from error responses.

---

## 2. R0 Security Contract (Go Shadow)

### 2.1 Scope
- The Go handler for `POST /proxy/plugin-main` is an **R2 write guard** in R0/R1.
- It must return `503 shadow_guard` and not perform any upstream call.
- This contract defines the security envelope that a future **live** implementation must implement.

### 2.2 Live Implementation Ban
> **Rule**: Until explicit approval, no live proxy upstream call is permitted. The `proxy_plugin_main_payload` equivalent in Go must remain a shadow placeholder.

---

## 3. Provider Allowlist

Future live proxy MUST only route to providers in this allowlist:

| Provider | Endpoint Family | Auth Header | Notes |
|----------|-----------------|-------------|-------|
| `openai` | OpenAI-compatible | `Authorization: Bearer {api_key}` | Default fallback |
| `claude` | Anthropic Messages | `x-api-key: {api_key}` + `anthropic-version` | |
| `gemini` | Google GenAI | `x-goog-api-key: {api_key}` | |
| `vertex` | Google Vertex AI | `Authorization: Bearer {access_token}` | Requires OAuth token exchange |
| `openrouter` | OpenRouter | `Authorization: Bearer {api_key}` | |
| `copilot` | GitHub Copilot | `Authorization: Bearer {copilot_token}` | Requires `_COPILOT_TOKEN_URL` token fetch |
| `custom` | OpenAI-compatible | `Authorization: Bearer {api_key}` | GLM-like models; detected by `glm-` prefix or `bigmodel.cn` endpoint |

**Rejection**: Any provider string not in this set MUST return `400` with `"unsupported provider"`.

---

## 4. Endpoint Rejection Rules

Future live proxy MUST reject the endpoint before any upstream call if any of the following match:

### 4.1 Scheme
- Only `http` and `https` are permitted.
- Reject: `ftp://`, `file://`, `gopher://`, `data://`, `javascript:`, etc.

### 4.2 Hostname Blocklist
Exact matches (case-insensitive):
- `localhost`
- `localhost.localdomain`
- `ip6-localhost`
- `ip6-loopback`
- `metadata`
- `metadata.google.internal`
- `metadata.aws`
- `metadata.azure`
- `instance-data`
- `instance-data.ec2.internal`
- Any hostname starting with `localhost.`

### 4.3 IP Address Rejection
If the hostname parses as an IP address, reject if ANY of the following is true:
- `is_loopback`
- `is_private`
- `is_link_local`
- `is_multicast`
- `is_unspecified`
- `is_reserved`

**Rationale**: Prevents SSRF to internal services (EC2 metadata, local DB, k8s API, etc.).

### 4.4 Port Considerations
- Empty port ? default for scheme (80/443).
- Explicit ports are allowed **only if** the host passes the above checks.
- No additional port blocklist is required in R0.

---

## 5. Timeout / Rate / Cost Guards

### 5.1 Timeout Guard
- **Input**: `timeout_ms` from request, or `AC_MAIN_TIMEOUT` env (seconds) ? 1000.
- **Clamp**: `max(1, min(300000, timeout_ms))` ? 1 ms ? 5 minutes.
- **Default**: 60000 ms (60 s).
- **Enforcement**: HTTP client timeout MUST be set to `clamped_ms / 1000.0` seconds.
- **Timeout response**: `504 Gateway Timeout` with `{"error": "upstream timeout"}`.

### 5.2 Rate Guard
- **Algorithm**: Fixed-window rate limiter per process.
- **Default limit**: 120 requests / 60 seconds window (`PROXY_DEFAULT_RPM`).
- **Burst**: No burst allowance in R0 (strict count).
- **Exceeded response**: `429 Too Many Requests` with `{"error": "Rate limit exceeded"}`.
- **Config update limit**: 30 RPM (`CONFIG_DEFAULT_RPM`) for `/config/update`.

### 5.3 Cost Guard (R0 Skeleton Only)
> **Note**: Cost guard is an R2 concern. R0 contract records the 0.8 behavior for future reference.

- 0.8 does not implement a token-budget or cost-cap guard.
- Future R2 live proxy MAY add:
  - Max tokens per request clamp (already in `max_tokens`).
  - Daily/weekly spend ceiling via env var `AC_PROXY_SPEND_CAP_USD`.
  - Per-session token accumulator (not implemented in 0.8).
- R0 shadow handler MUST NOT implement cost logic.

---

## 6. Error Mapping Contract

Future live proxy MUST map upstream errors as follows:

| Upstream Condition | HTTP Status | Response Shape | Sanitized? |
|--------------------|-------------|----------------|------------|
| Timeout | 504 | `{"error": "upstream timeout"}` | N/A |
| HTTPStatusError (upstream 4xx/5xx) | upstream code | `{"error": "<detail>"}` | **YES** |
| Generic Exception | 502 | `{"error": "<detail>"}` | **YES** |
| Invalid endpoint | 400 | `{"error": "endpoint host is not allowed."}` | N/A |
| Unsupported provider | 400 | `{"error": "unsupported provider"}` | N/A |
| Rate limit exceeded | 429 | `{"error": "Rate limit exceeded"}` | N/A |
| Forbidden (non-local, no token) | 403 | `{"error": "Forbidden: local/operator access required"}` | N/A |
| Forbidden (invalid token) | 403 | `{"error": "Forbidden: invalid or missing operator token"}` | N/A |

### 6.1 API Key Sanitization Rules
Any error detail string MUST be sanitized before inclusion in the JSON response:
1. Replace the literal `api_key` value with `***`.
2. Replace `Authorization: Bearer <token>` with `Authorization: Bearer ***` (case-insensitive).
3. Replace standalone `Bearer <token>` with `Bearer ***`.

---

## 7. Operator / Local Access Guard

Future live proxy MUST preserve the operator-token gate from 0.8:

1. **Local request detection**:
   - `client.host` is loopback (`127.0.0.0/8`, `::1`), OR
   - Hostname is in `{"localhost", "localhost.localdomain", "ip6-localhost", "ip6-loopback", "testclient"}`.

2. **Non-local request**:
   - If `AC_OPERATOR_TOKEN` env is unset ? `403`.
   - If header `X-RisuAI-Operator-Token` or `Authorization: Bearer <token>` does not match ? `403`.

3. **R0 shadow behavior**: The Go handler is an R2 guard and does not need to implement this gate until live proxy is approved.

---

## 8. Request ? DTO Mapping

Future live proxy Go DTO MUST include at minimum these fields (mirrors `internal/dto`):

```go
type ProxyPluginMainRequest struct {
    Messages               []ProxyMessage `json:"messages"`
    Model                  string         `json:"model"`
    MaxTokens              int            `json:"max_tokens"`
    Temperature            float64        `json:"temperature"`
    Provider               string         `json:"provider,omitempty"`
    Endpoint               string         `json:"endpoint,omitempty"`
    APIKey                 string         `json:"api_key,omitempty"`
    TimeoutMs              int            `json:"timeout_ms,omitempty"`
    MaxCompletionTokens    int            `json:"max_completion_tokens,omitempty"`
    ReasoningPreset        string         `json:"reasoning_preset,omitempty"`
    ReasoningEffort        string         `json:"reasoning_effort,omitempty"`
    ReasoningBudgetTokens  int            `json:"reasoning_budget_tokens,omitempty"`
    BudgetTokens           int            `json:"budget_tokens,omitempty"`
    GLMThinkingType        string         `json:"glm_thinking_type,omitempty"`
}
```

**Validation**:
- `endpoint` non-empty after default resolution.
- `api_key` non-empty after default resolution.
- `model` non-empty after default resolution.

---

## 9. Go Handler Classification

| Route | Current Tier | Current Behavior | Future Tier |
|-------|-------------|------------------|-------------|
| `POST /proxy/plugin-main` | R2 (write) | `writeShadowGuard` ? 503 | R2 (live) |

The handler is registered in `group_proxy.go` and MUST remain a `writeShadowGuard` until live proxy approval.

---

## 10. Trace / Audit Placeholder

Future live proxy MUST emit trace events (no-op in R0):
- `proxy.request` ? model, provider, endpoint host (not full URL for privacy).
- `proxy.upstream.latency_ms` ? round-trip time.
- `proxy.upstream.status` ? upstream HTTP status.
- `proxy.guard.block` ? reason (rate, endpoint, auth).

The `trace.go` file in `internal/httpapi` defines the vocabulary; actual emission is R2.

---

## 11. Verification Checklist

Before any live proxy implementation is approved, the following MUST be true:

- [ ] Provider allowlist is enforced (hard reject on unknown).
- [ ] Endpoint validation rejects all private/local/metadata hosts.
- [ ] Timeout clamp is enforced (1 ms ? 300000 ms).
- [ ] Rate limiter is active (120 RPM default).
- [ ] Operator/local access gate is preserved.
- [ ] API key sanitization is applied to all error paths.
- [ ] Error status codes match the mapping table.
- [ ] DTO decoder uses `DecodeWithDefaults` with validation.
- [ ] No live upstream call is made in shadow mode.
- [ ] H-4e release hygiene scan passes (no secrets in source tree).

---

*Contract version: R0-2026-05-21*  
*Reference: `Archive Center Beta 0.8(fix)/backend/services/proxy_plugin_main.py`, `route_guard.py`*
