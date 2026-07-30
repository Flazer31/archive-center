# Provider Request Overrides and Vertex Flex PayGo Contract

Status: Archive Center backend and adapter implemented; live provider verification pending

Last updated: 2026-07-31

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

## OpenAI-Compatible Gateways and Service Tiers

Archive Center exposes LLM Gateway and Vercel AI Gateway as independent
generation providers. Both use an OpenAI-compatible Chat Completions transport,
but they are not stored or reported as `openai` or `custom`.

Default endpoints:

```text
LLM Gateway:      https://api.llmgateway.io/v1
Vercel AI Gateway: https://ai-gateway.vercel.sh/v1
```

Role-specific settings:

```json
{
  "pluginMainLlmGatewayServiceTier": "standard",
  "subLlmLlmGatewayServiceTier": "standard"
}
```

Backend request field:

```json
{
  "llm_gateway_service_tier": "flex"
}
```

The field name remains backward-compatible with the earlier LLM Gateway-only
setting. The Go provider owner normalizes it and writes the upstream
OpenAI-compatible `service_tier` field for provider `openai`, `llmgateway`,
`vercel`, or `custom`:

| UI value | Upstream value |
|---|---|
| `standard` | `default` |
| `flex` | `flex` |
| `priority` | `priority` |

Rules:

- The typed tier is accepted only with provider `openai`, `llmgateway`,
  `vercel`, or `custom`.
- `standard` is omitted so the provider keeps its normal default. Only an
  explicit Flex or Priority selection is forwarded. Provider applicability,
  normalization, and conflicts are owned by Go; JavaScript uses its provider
  list only to present the relevant settings row.
- Invalid values and a conflicting `extra_body_json.service_tier` fail before
  an upstream request.
- Existing untyped `extra_body_json.service_tier` remains usable when the
  typed setting is absent.
- An upstream `unsupported_service_tier` response is returned as an error. It
  is never retried after removing the tier and never silently downgraded.
- Trace records the requested and applied tier. It also records the response
  `service_tier` as the served tier, or `not_reported` when the gateway omits
  it.

OpenAI documents `service_tier:flex` as lower-cost, slower, best-effort
processing with limited model availability. LLM Gateway documents `flex`,
`priority`, and `default`/`auto`, but only for provider/model mappings that
advertise the selected tier. Vercel AI Gateway forwards OpenAI service tiers
for supported OpenAI models. Custom OpenAI-compatible endpoints receive the
field only when the user explicitly selects Flex or Priority; Archive Center
does not claim that every custom server supports it.

Official reference:

- https://developers.openai.com/api/docs/guides/flex-processing
- https://docs.llmgateway.io/features/service-tiers
- https://vercel.com/docs/ai-gateway/capabilities/service-tiers

Implementation status:

- source and automated regression: implemented for OpenAI, LLM Gateway,
  Vercel, and Custom request construction;
- real paid provider/model calls: unverified;
- release status: `implemented_unverified`, not a live-provider acceptance
  result.

## Prompt Caching Across Supported Providers

Archive Center does not expose one fake universal cache switch because the
provider contracts are different:

- OpenAI prompt caching is automatic for eligible requests. Recent model
  families also expose explicit breakpoints and `prompt_cache_key`; these can
  be sent through Extra Body JSON when the selected endpoint supports them.
- Gemini API and Vertex Gemini implicit caching are automatic. Explicit context
  caching requires creating a provider cache resource first; an existing
  `cachedContent` resource reference can be sent through Extra Body JSON.
- LLM Gateway provider caching is automatic for most mappings and injects
  Anthropic/Bedrock cache markers when required. Gateway-level byte-identical
  response caching remains a project setting, not a per-request Archive Center
  toggle.
- Vercel AI Gateway automatic provider-aware caching is available through:

```json
{
  "providerOptions": {
    "gateway": {
      "caching": "auto"
    }
  }
}
```

- Custom providers have no portable cache field. Extra Headers JSON and Extra
  Body JSON are therefore the explicit compatibility path; unsupported
  provider errors are returned rather than hidden.

Gemini and Vertex normalized responses preserve the provider's complete
`usageMetadata`. The trace copies `promptTokenCount`,
`candidatesTokenCount`, `totalTokenCount`, `cachedContentTokenCount`, and
`trafficType` when they are actually returned. Archive Center never invents a
cache hit.

Official reference:

- https://developers.openai.com/api/docs/guides/prompt-caching
- https://docs.llmgateway.io/features/caching/provider-cache-control
- https://vercel.com/docs/ai-gateway/models-and-providers/provider-options
- https://cloud.google.com/vertex-ai/generative-ai/docs/context-cache/context-cache-overview

## Critic JSON Response Enforcement

The Critic/extraction path requests structured JSON at the provider request
boundary. Ordinary narrative generation is not forced into JSON.

- Gemini and Vertex receive
  `generationConfig.responseMimeType=application/json`.
- OpenAI, OpenRouter, and LLM Gateway receive
  `response_format.type=json_object`.
- Vercel receives its documented `response_format.type=json_schema` with a
  Critic top-level object schema. User-supplied `json_schema` and Vercel's legacy
  `type=json` are preserved.
- Claude receives `output_config.format.type=json_schema` with the same Critic
  top-level schema. This is applied only to Critic/extraction calls; ordinary
  Claude narrative calls are unchanged.
- Custom, Ollama, and Copilot do not share one verified native structured-output
  field. Archive Center therefore does not invent `response_format` for them.
  A user-supplied provider-native Extra Body JSON setting is preserved and
  checked for conflict.
- A matching user-supplied structured-output setting is preserved. A
  conflicting setting fails before the upstream call.
- The provider adapter does not retry by silently deleting the JSON request.
  A provider/model that does not support structured output returns a visible
  provider error, while the already accepted raw turn remains durable and the
  derived Critic work remains retryable.

Official reference:

- https://developers.openai.com/api/docs/guides/structured-outputs
- https://vercel.com/docs/ai-gateway/sdks-and-apis/openai-chat-completions/structured-outputs
- https://platform.claude.com/docs/en/build-with-claude/structured-outputs

## Anthropic Claude Automatic Prompt Caching

Archive Center 3.6 exposes Anthropic's automatic prompt caching only for the
direct Claude Messages API provider ID `claude`. The UI stores independent
Publisher and Critic choices; the Supervisor inherits the Publisher choice.
The default is `off`.

Role-specific settings:

```json
{
  "pluginMainClaudePromptCacheMode": "off",
  "subLlmClaudePromptCacheMode": "off"
}
```

Backend request field:

```json
{
  "claude_prompt_cache_mode": "ephemeral_5m"
}
```

The Go provider owner validates the typed mode and writes the top-level
Anthropic `cache_control` object:

| Typed mode | Claude Messages API request |
|---|---|
| `off` | no typed `cache_control` is added |
| `ephemeral_5m` | `{"cache_control":{"type":"ephemeral"}}` |
| `ephemeral_1h` | `{"cache_control":{"type":"ephemeral","ttl":"1h"}}` |

Anthropic documents `ephemeral` as the current cache type. Its default
lifetime is five minutes; `ttl:"1h"` selects the one-hour duration. An
explicit manual `ttl:"5m"` is equivalent to the omitted five-minute TTL for
typed/manual matching, although Archive Center's typed five-minute mapping
continues to omit `ttl`.

Anthropic's official pricing uses multipliers relative to base input tokens:
five-minute cache writes cost `1.25x`, one-hour cache writes cost `2x`, and
cache reads/hits cost `0.1x`. The one-hour option therefore has a higher write
cost and should be selected intentionally.

Rules:

- A non-`off` typed mode is accepted only with provider `claude`.
- Invalid values and conflicting `extra_body_json.cache_control` values fail
  before any upstream request.
- Matching typed and manual values are accepted and traced as
  `typed_and_extra_body_json`.
- When the typed field is absent or `off`, an existing manual
  `extra_body_json.cache_control` remains unchanged.
- Manual `extra_body_json.cache_control` is backend request compatibility for
  callers that explicitly send that field. The typed mode above remains the
  normal Claude path; the generic Extra Body JSON field is available only for
  advanced provider-specific options and is checked for conflicts.
- The normalized response preserves Anthropic's raw `usage` object when it is
  present. Trace copies `cache_creation_input_tokens`,
  `cache_read_input_tokens`, and `usage.service_tier` only when Anthropic
  returned those fields; missing usage is never synthesized.
- Anthropic can process a prompt without caching when it is below the
  model-specific minimum cacheable length. The usage cache counters are the
  source of truth for whether a cache write or read occurred.

Official reference:

- https://platform.claude.com/docs/en/build-with-claude/prompt-caching
- https://platform.claude.com/docs/en/about-claude/pricing

Implementation status:

- `source_implemented`: typed mapping, conflict handling, role propagation,
  response/trace preservation, and automated regression are implemented;
- live Anthropic account/model call: unverified;
- acceptance status: `live Anthropic call unverified`, not live-provider proof.

Scope exclusions:

- no Claude Batch API;
- no priority or service-tier selector;
- no Claude Flex mode;
- no changes to retry policy or HUD design.

## Non-Goals

- Do not auto-enable Flex for every user.
- Do not hide cost-related routing changes.
- Do not allow custom JSON to overwrite the actual chat messages.
- Do not implement prompt-specific or model-name-specific hardcoded behavior.
- Do not move Output Quality Layer to Archive Center backend just for this
  feature.
