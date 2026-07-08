# Provider Request Overrides and Vertex Flex PayGo Contract

Status: design contract; Risu Output Quality Layer standalone JS implemented; Archive Center backend pending

Last updated: 2026-07-07

## Purpose

Archive Center and `Risu Output Quality Layer 2.5.js` both call auxiliary LLMs.
Users who use Vertex AI need a way to send provider-specific request options
without editing code. The immediate request is Vertex Flex PayGo support, but
the contract must also cover general custom JSON body options used by
OpenAI-compatible, Gemini, Vertex, and similar providers.

The design goal is:

- let users add provider-specific options safely;
- avoid hardcoding one model or one prompt style;
- keep core chat content protected;
- expose enough trace/debug information to confirm whether the option was used.

## Key Finding: Vertex Flex PayGo Is Header-Based

Google's Flex PayGo documentation says Flex is selected by HTTP headers.
It is not only a JSON body field.

Provisioned Throughput first, then Flex PayGo fallback:

```http
X-Vertex-AI-LLM-Shared-Request-Type: flex
```

Flex PayGo only:

```http
X-Vertex-AI-LLM-Request-Type: shared
X-Vertex-AI-LLM-Shared-Request-Type: flex
```

So a "custom JSON body" box is useful, but it is not sufficient for Vertex
Flex PayGo. We need both:

- extra HTTP headers JSON;
- extra request body JSON.

Official references:

- https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/flex-paygo
- https://cloud.google.com/vertex-ai/generative-ai/pricing

## Where This Should Be Implemented

### Archive Center

Archive Center should apply these values in the Go backend, not only in
`Archive Center.js`.

Reason:

- Archive Center already routes Publisher/Supervisor/Critic calls through the
  backend `/proxy/plugin-main` path.
- Vertex service-account token exchange is backend-owned.
- Flex PayGo needs HTTP headers on the outgoing Vertex request.
- Keeping this in the backend avoids making `Archive Center.js` larger and
  avoids duplicating provider-specific request logic in the plugin UI.

Expected JS responsibility:

- show settings fields;
- validate that JSON text is at least parseable before saving when possible;
- sync the values to backend runtime config;
- show trace/debug summary.

Expected Go backend responsibility:

- parse and validate extra header/body JSON;
- apply headers/body at the final provider request boundary;
- protect core fields from being overwritten;
- report applied keys and failures in trace.

### Risu Output Quality Layer 2.5.js

The standalone Output Quality Layer does not depend on the Archive Center
backend. Therefore it must implement the same contract inside its own JS
provider caller.

This does not contradict the Archive Center backend decision. It means:

- Archive Center: backend-owned implementation.
- Output Quality Layer: standalone JS-owned implementation.
- Both share the same field names and safety rules.

## Shared Settings Shape

Use these setting names in both projects where possible:

```json
{
  "extra_headers_json": "",
  "extra_body_json": "",
  "vertex_flex_mode": "off"
}
```

`vertex_flex_mode` values:

```text
off
provisioned_then_flex
flex_only
```

Meaning:

- `off`: do not add Flex PayGo headers.
- `provisioned_then_flex`: use Provisioned Throughput quota if available, then
  Flex PayGo.
- `flex_only`: force shared Flex PayGo traffic.

## Header Contract

User-provided `extra_headers_json` must be a JSON object.

Example:

```json
{
  "X-Vertex-AI-LLM-Shared-Request-Type": "flex"
}
```

Protected headers must not be overridden by user JSON:

- `Authorization`
- `Content-Type`
- `Accept`
- provider auth headers such as `x-goog-api-key` or `x-api-key`

Archive Center and Output Quality Layer may add Vertex Flex headers from
`vertex_flex_mode` without requiring the user to type them manually.

Mapping:

```text
off
  no extra Flex headers

provisioned_then_flex
  X-Vertex-AI-LLM-Shared-Request-Type: flex

flex_only
  X-Vertex-AI-LLM-Request-Type: shared
  X-Vertex-AI-LLM-Shared-Request-Type: flex
```

## Body Contract

User-provided `extra_body_json` must be a JSON object.

Example for a provider-specific option:

```json
{
  "providerOptions": {
    "gateway": {
      "caching": "auto"
    }
  }
}
```

Example for native Gemini/Vertex generation config:

```json
{
  "generationConfig": {
    "topP": 0.9
  }
}
```

Protected body fields must not be overwritten:

- OpenAI-compatible:
  - `messages`
  - `model`
- Gemini/Vertex native:
  - `contents`
  - `systemInstruction`
- Anthropic-like:
  - `messages`
  - `system`

Recommended behavior:

- deep-merge normal JSON object fields;
- skip protected fields and record them in trace;
- reject non-object JSON;
- do not silently clear an existing provider payload when parsing fails.

## Trace and Debug Contract

Every LLM call using this feature should expose a compact trace:

```json
{
  "request_overrides": {
    "extra_headers_applied": true,
    "extra_header_keys": ["X-Vertex-AI-LLM-Shared-Request-Type"],
    "extra_body_applied": true,
    "extra_body_keys": ["generationConfig"],
    "protected_body_keys_skipped": [],
    "protected_header_keys_skipped": [],
    "vertex_flex_mode": "provisioned_then_flex"
  }
}
```

For Vertex Flex PayGo, response diagnostics should also keep the traffic type
if the provider returns it:

```json
{
  "vertex_traffic_type": "ON_DEMAND_FLEX"
}
```

If the provider does not return a traffic type, the trace should say:

```json
{
  "vertex_traffic_type": "not_reported"
}
```

## Archive Center Implementation Notes

Relevant current backend surfaces:

- `go-service/internal/httpapi/proxy_provider.go`
  - owns `/proxy/plugin-main` provider calls;
  - owns Vertex access token exchange;
  - should apply Flex headers and extra body at the final request boundary.
- `go-service/internal/httpapi/runtime_config.go`
  - should receive and store runtime config fields from `Archive Center.js`;
  - should expose override trace in runtime config/debug output.
- `Archive Center.js`
  - should add UI fields and send the settings to backend;
  - should not directly own provider-specific request mutation for Archive
    Center backend calls.

Suggested role-specific fields:

```json
{
  "supervisorExtraHeadersJson": "",
  "supervisorExtraBodyJson": "",
  "supervisorVertexFlexMode": "off",
  "criticExtraHeadersJson": "",
  "criticExtraBodyJson": "",
  "criticVertexFlexMode": "off"
}
```

Embedding calls may use the same header/body contract later, but should be
handled carefully because embedding endpoints can differ from generation
endpoints.

## Risu Output Quality Layer 2.5.js Implementation Notes

The standalone plugin should add the same fields to each role profile and
fallback profile:

```json
{
  "extra_headers_json": "",
  "extra_body_json": "",
  "vertex_flex_mode": "off"
}
```

Apply order:

1. Build the normal provider request.
2. Apply generated Vertex Flex headers from `vertex_flex_mode`.
3. Parse and apply `extra_headers_json`.
4. Parse and deep-merge `extra_body_json`.
5. Skip protected fields.
6. Record applied/skipped keys in the role call trace.
7. Send the final request.

Implementation note:

- Implemented in `source/Risu Output Quality Layer 2.5.js` build
  `0.1.73 / DEV-BUILD-20260707-ROQL-0.1.73-vertex-flex-vertex-only-ui`.
- Role profiles and fallback profiles expose `extra_headers_json`,
  `extra_body_json`, and `vertex_flex_mode`.
- Vertex native and Vertex OpenAI-compatible calls apply Flex headers from
  `vertex_flex_mode`.
- `vertex_flex_mode` is available only when the role provider or fallback
  provider is `vertex`; GPT, Claude, Gemini API, Ollama, OpenRouter, and custom
  providers do not use Vertex Flex headers.
- OpenAI-compatible, Gemini, Anthropic, and Vertex calls support protected
  extra header/body overrides with trace reporting.

The Output Quality Layer already has provider profile and trace concepts, so
the setting should be attached to role model profiles rather than as one
global-only toggle. A global default can exist, but per-role override is needed
because some roles can tolerate Flex latency and some cannot.

Suggested default:

- Character/style/continuity readers: `provisioned_then_flex` or `flex_only`
  can be acceptable if the user prioritizes cost.
- Final output enhancement or user-visible rewrite: keep `off` by default,
  because Flex can increase latency.

## User-Facing Explanation

Recommended UI wording:

```text
Vertex Flex PayGo
Cheaper Vertex Gemini traffic for latency-tolerant helper calls.
Flex can be slower or throttled more often than Standard PayGo.
Use it for critic/reviewer/helper calls, not for latency-sensitive main output.
```

Recommended modes:

```text
Off
  Use normal provider behavior.

Provisioned then Flex
  Use reserved throughput first if available, then cheaper Flex traffic.

Flex only
  Force cheaper shared Flex traffic. May be slower.
```

## Non-Goals

- Do not auto-enable Flex for every user.
- Do not hide cost-related routing changes.
- Do not allow custom JSON to overwrite the actual chat messages.
- Do not implement prompt-specific or model-name-specific hardcoded behavior.
- Do not move Output Quality Layer to Archive Center backend just for this
  feature.
